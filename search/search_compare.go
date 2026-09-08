package search

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// SearchCompareQuery is one row from a recall CSV file.
type SearchCompareQuery struct {
	Language string
	Query    string
}

// JsonDiff describes a single difference at a JSON path.
type JsonDiff struct {
	Path string
	Base interface{}
	Exp  interface{}
	Kind string // "only_in_base", "only_in_exp", "different"
}

func (d JsonDiff) String() string {
	switch d.Kind {
	case "only_in_base":
		return fmt.Sprintf("%-40s  only in base: %v", d.Path, d.Base)
	case "only_in_exp":
		return fmt.Sprintf("%-40s  only in exp:  %v", d.Path, d.Exp)
	default:
		return fmt.Sprintf("%-40s  base: %v  ≠  exp: %v", d.Path, d.Base, d.Exp)
	}
}

// SearchCompareResult holds the per-query comparison outcome.
type SearchCompareResult struct {
	Language     string
	Query        string
	Identical    bool
	IdenticalTop bool // true when all diffs are in tail hits (index ≥ topHits) or hits.total
	BaseError    string
	ExpError     string
	ElasticErr   string // set when ES was unhealthy at query time
	Diffs        []JsonDiff
	BaseRaw      string // normalized JSON (for display)
	ExpRaw       string
}

// SearchCompareSummary aggregates results across all queries.
type SearchCompareSummary struct {
	Total        int
	Identical    int
	IdenticalTop int // identical in top hits; tail diffs are ES page-boundary noise
	Different    int
	Errors       int
	Skipped      int // skipped while ES was unhealthy
	Results      []SearchCompareResult
	ElasticPings []ElasticPing // health check timeline
}

// ElasticPing records one ES health check event.
type ElasticPing struct {
	At      time.Time
	Healthy bool
	Detail  string // status or error text
}

// ---------------------------------------------------------------------------
// Elasticsearch health checker
// ---------------------------------------------------------------------------

type esHealth struct {
	healthy int32 // atomic: 1=healthy, 0=unhealthy
	pings   []ElasticPing
	mu      sync.Mutex
}

func newESHealth() *esHealth { h := &esHealth{}; atomic.StoreInt32(&h.healthy, 1); return h }

func (h *esHealth) IsHealthy() bool { return atomic.LoadInt32(&h.healthy) == 1 }

func (h *esHealth) record(ok bool, detail string) {
	h.mu.Lock()
	h.pings = append(h.pings, ElasticPing{At: time.Now(), Healthy: ok, Detail: detail})
	h.mu.Unlock()
}

func (h *esHealth) Pings() []ElasticPing {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]ElasticPing, len(h.pings))
	copy(out, h.pings)
	return out
}

// runESHealthChecker pings elasticURL/_cluster/health every interval.
// It calls wakeAll whenever health flips so blocked goroutines can re-check.
func runESHealthChecker(elasticURL string, h *esHealth, wakeAll func(), stop <-chan struct{}) {
	checkURL := strings.TrimRight(elasticURL, "/") + "/_cluster/health"
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	ping := func() {
		resp, err := http.Get(checkURL)
		if err != nil {
			wasHealthy := h.IsHealthy()
			atomic.StoreInt32(&h.healthy, 0)
			detail := err.Error()
			h.record(false, detail)
			if wasHealthy {
				log.Warnf("ES health: DOWN — %s", detail)
				wakeAll() // wake throttled goroutines so they re-check
			}
			return
		}
		defer resp.Body.Close()

		ok := resp.StatusCode < 400
		wasHealthy := h.IsHealthy()
		if ok {
			atomic.StoreInt32(&h.healthy, 1)
		} else {
			atomic.StoreInt32(&h.healthy, 0)
		}
		body, _ := io.ReadAll(resp.Body)
		detail := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if len(body) > 0 {
			// Extract just the status field from the health JSON
			var m map[string]interface{}
			if json.Unmarshal(body, &m) == nil {
				if s, ok := m["status"]; ok {
					detail = fmt.Sprintf("status=%v", s)
				}
			}
		}
		h.record(ok, detail)
		if wasHealthy && !ok {
			log.Warnf("ES health: DEGRADED — %s", detail)
			wakeAll()
		} else if !wasHealthy && ok {
			log.Infof("ES health: RECOVERED — %s", detail)
			wakeAll()
		}
	}

	ping() // immediate first check
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			ping()
		}
	}
}

// ---------------------------------------------------------------------------
// Adaptive limiter
// ---------------------------------------------------------------------------
// Starts at max concurrency. Halves on each error (min 1).
// Increments by 1 every successStep consecutive successes, up to max.
// Also blocks when ES is unhealthy (if a health checker is active).

const successStep = 5 // ramp up 1 slot after this many consecutive clean queries

type adaptiveLimiter struct {
	mu          sync.Mutex
	cond        *sync.Cond
	current     int
	max         int
	active      int
	lastError   time.Time
	consecutive int // consecutive successes since last error
	health      *esHealth
}

func newAdaptiveLimiter(max int, h *esHealth) *adaptiveLimiter {
	l := &adaptiveLimiter{current: max, max: max, health: h}
	l.cond = sync.NewCond(&l.mu)
	return l
}

// Acquire blocks until a slot is available AND ES is healthy.
func (l *adaptiveLimiter) Acquire() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for {
		if l.health != nil && !l.health.IsHealthy() {
			l.cond.Wait()
			continue
		}
		if l.active < l.current {
			l.active++
			return
		}
		l.cond.Wait()
	}
}

// Release returns a slot. hadError drives the adaptive logic.
func (l *adaptiveLimiter) Release(hadError bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.active--
	if hadError {
		l.lastError = time.Now()
		l.consecutive = 0
		if l.current > 1 {
			l.current = l.current / 2
			log.Warnf("Query error detected — throttling concurrency to %d", l.current)
		}
	} else {
		l.consecutive++
		if l.consecutive >= successStep && l.current < l.max && time.Since(l.lastError) > 5*time.Second {
			l.current++
			l.consecutive = 0
			log.Infof("Recovered — raising concurrency to %d", l.current)
		}
	}
	l.cond.Broadcast()
}

// WakeAll is called by the health checker when ES state changes.
func (l *adaptiveLimiter) WakeAll() {
	l.mu.Lock()
	l.cond.Broadcast()
	l.mu.Unlock()
}

// ---------------------------------------------------------------------------
// CSV reading
// ---------------------------------------------------------------------------

// ReadSearchCompareQueries reads Language+Query columns from a recall CSV.
func ReadSearchCompareQueries(path string) ([]SearchCompareQuery, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv %s: %w", path, err)
	}

	var out []SearchCompareQuery
	for i, rec := range records {
		if i == 0 {
			continue // skip header
		}
		if len(rec) < 2 {
			continue
		}
		lang := strings.TrimSpace(rec[0])
		query := strings.TrimSpace(rec[1])
		if lang == "" || query == "" || strings.HasPrefix(lang, "//") {
			continue
		}
		out = append(out, SearchCompareQuery{Language: lang, Query: query})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// HTTP fetch
// ---------------------------------------------------------------------------

func fetchSearchJSON(serverURL, lang, query string, pageSize int) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/search?q=%s&language=%s&page_no=1&page_size=%d&sort_by=relevance",
		serverURL, url.QueryEscape(query), url.QueryEscape(lang), pageSize)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("GET: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Normalization
// ---------------------------------------------------------------------------

func normalizeFields(m map[string]interface{}, ignoreScores bool) {
	delete(m, "execution_time_log")
	// Strip ES metadata fields not present in the new backend's SearchResult struct.
	if sr, ok := m["search_result"].(map[string]interface{}); ok {
		delete(sr, "took")
		delete(sr, "timed_out")
		delete(sr, "_shards")
	}
	if ignoreScores {
		deleteRecursive(m, "_score")
		deleteRecursive(m, "max_score")
	}
	sortFilterValues(m)
	sortHitsStable(m)
	stripEmptyValues(m)
}

// sortHitsStable sorts hits.hits by _score desc, breaking ties by _id asc,
// so that equal-score results don't produce spurious diffs between base and exp.
func sortHitsStable(m map[string]interface{}) {
	sr, ok := m["search_result"].(map[string]interface{})
	if !ok {
		return
	}
	outer, ok := sr["hits"].(map[string]interface{})
	if !ok {
		return
	}
	hits, ok := outer["hits"].([]interface{})
	if !ok || len(hits) < 2 {
		return
	}
	sort.SliceStable(hits, func(i, j int) bool {
		hi, _ := hits[i].(map[string]interface{})
		hj, _ := hits[j].(map[string]interface{})
		si, _ := hi["_score"].(float64)
		sj, _ := hj["_score"].(float64)
		if si != sj {
			return si > sj // higher score first
		}
		return hitTiebreakKey(hi) < hitTiebreakKey(hj)
	})
}

// hitTiebreakKey returns the _id field of a hit, falling back to landing_page
// from _source when _id is absent, for stable sort tiebreaking.
func hitTiebreakKey(hit map[string]interface{}) string {
	if id, _ := hit["_id"].(string); id != "" {
		return id
	}
	if src, ok := hit["_source"].(map[string]interface{}); ok {
		if lp, _ := src["landing_page"].(string); lp != "" {
			return lp
		}
		if uid, _ := src["mdb_uid"].(string); uid != "" {
			return uid
		}
	}
	return ""
}

// sortFilterValues sorts _source.filter_values arrays within each hit by name then value,
// so that reordering of filter values between base and exp does not produce spurious diffs.
func sortFilterValues(m map[string]interface{}) {
	sr, ok := m["search_result"].(map[string]interface{})
	if !ok {
		return
	}
	outer, ok := sr["hits"].(map[string]interface{})
	if !ok {
		return
	}
	hits, ok := outer["hits"].([]interface{})
	if !ok {
		return
	}
	for _, h := range hits {
		hit, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		source, ok := hit["_source"].(map[string]interface{})
		if !ok {
			continue
		}
		fv, ok := source["filter_values"].([]interface{})
		if !ok || len(fv) < 2 {
			continue
		}
		sort.SliceStable(fv, func(i, j int) bool {
			fi, _ := fv[i].(map[string]interface{})
			fj, _ := fv[j].(map[string]interface{})
			ni, _ := fi["name"].(string)
			nj, _ := fj["name"].(string)
			if ni != nj {
				return ni < nj
			}
			vi, _ := fi["value"].(string)
			vj, _ := fj["value"].(string)
			return vi < vj
		})
	}
}

// stripEmptyValues removes null, empty-string, and empty-object values recursively.
// Booleans, numbers, and empty arrays are kept.
func stripEmptyValues(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, child := range t {
			cleaned := stripEmptyValues(child)
			if isEmptyValue(cleaned) {
				delete(t, k)
			} else {
				t[k] = cleaned
			}
		}
	case []interface{}:
		for i, child := range t {
			t[i] = stripEmptyValues(child)
		}
	}
	return v
}

// isEmptyValue returns true for nil, empty string, and empty map.
// Does NOT return true for false, 0, or empty slice.
func isEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return s == ""
	}
	if m, ok := v.(map[string]interface{}); ok {
		return len(m) == 0
	}
	return false
}

func deleteRecursive(v interface{}, key string) {
	switch t := v.(type) {
	case map[string]interface{}:
		delete(t, key)
		for _, child := range t {
			deleteRecursive(child, key)
		}
	case []interface{}:
		for _, child := range t {
			deleteRecursive(child, key)
		}
	}
}

func toNormalizedJSON(v interface{}) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ---------------------------------------------------------------------------
// JSON deep diff
// ---------------------------------------------------------------------------

func diffJSON(path string, base, exp interface{}, out *[]JsonDiff) {
	if base == nil && exp == nil {
		return
	}

	baseMap, baseIsMap := base.(map[string]interface{})
	expMap, expIsMap := exp.(map[string]interface{})

	if baseIsMap && expIsMap {
		keys := make(map[string]struct{})
		for k := range baseMap {
			keys[k] = struct{}{}
		}
		for k := range expMap {
			keys[k] = struct{}{}
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)

		for _, k := range sorted {
			childPath := path + "." + k
			if path == "" {
				childPath = k
			}
			bv, bOk := baseMap[k]
			ev, eOk := expMap[k]
			if bOk && !eOk {
				*out = append(*out, JsonDiff{Path: childPath, Base: bv, Kind: "only_in_base"})
			} else if !bOk && eOk {
				*out = append(*out, JsonDiff{Path: childPath, Exp: ev, Kind: "only_in_exp"})
			} else {
				diffJSON(childPath, bv, ev, out)
			}
		}
		return
	}

	baseArr, baseIsArr := base.([]interface{})
	expArr, expIsArr := exp.([]interface{})

	if baseIsArr && expIsArr {
		if len(baseArr) != len(expArr) {
			*out = append(*out, JsonDiff{
				Path: path,
				Base: fmt.Sprintf("[%d items]", len(baseArr)),
				Exp:  fmt.Sprintf("[%d items]", len(expArr)),
				Kind: "different",
			})
			min := len(baseArr)
			if len(expArr) < min {
				min = len(expArr)
			}
			for i := 0; i < min; i++ {
				diffJSON(fmt.Sprintf("%s[%d]", path, i), baseArr[i], expArr[i], out)
			}
			return
		}
		for i := range baseArr {
			diffJSON(fmt.Sprintf("%s[%d]", path, i), baseArr[i], expArr[i], out)
		}
		return
	}

	baseStr := fmt.Sprintf("%v", base)
	expStr := fmt.Sprintf("%v", exp)
	if baseStr != expStr {
		*out = append(*out, JsonDiff{Path: path, Base: base, Exp: exp, Kind: "different"})
	}
}

// ---------------------------------------------------------------------------
// Compare one query
// ---------------------------------------------------------------------------

func CompareSearchQuery(q SearchCompareQuery, baseURL, expURL string, pageSize, topHits int, ignoreScores bool) SearchCompareResult {
	res := SearchCompareResult{Language: q.Language, Query: q.Query}

	var (
		baseMap map[string]interface{}
		expMap  map[string]interface{}
		baseErr error
		expErr  error
		wg      sync.WaitGroup
	)
	wg.Add(2)

	go func() {
		defer wg.Done()
		baseMap, baseErr = fetchSearchJSON(baseURL, q.Language, q.Query, pageSize)
	}()
	go func() {
		defer wg.Done()
		expMap, expErr = fetchSearchJSON(expURL, q.Language, q.Query, pageSize)
	}()
	wg.Wait()

	if baseErr != nil {
		res.BaseError = baseErr.Error()
		log.Warnf("[%s] %q base error: %v", q.Language, q.Query, baseErr)
	}
	if expErr != nil {
		res.ExpError = expErr.Error()
		log.Warnf("[%s] %q exp error: %v", q.Language, q.Query, expErr)
	}
	if res.BaseError != "" || res.ExpError != "" {
		return res
	}

	normalizeFields(baseMap, ignoreScores)
	normalizeFields(expMap, ignoreScores)

	var diffs []JsonDiff
	diffJSON("", baseMap, expMap, &diffs)
	res.Diffs = diffs
	res.Identical = len(diffs) == 0
	if !res.Identical {
		res.IdenticalTop = topHits > 0 && allDiffsInTail(diffs, topHits)
		log.Debugf("[%s] %q — %d diff(s) identical_top=%v", q.Language, q.Query, len(diffs), res.IdenticalTop)
	}

	if baseJSON, err := toNormalizedJSON(baseMap); err == nil {
		res.BaseRaw = baseJSON
	}
	if expJSON, err := toNormalizedJSON(expMap); err == nil {
		res.ExpRaw = expJSON
	}

	return res
}

// allDiffsInTail returns true when every diff path is either:
//   - a tail hit (search_result.hits.hits[N]... where N >= topHits), or
//   - search_result.hits.total (page-boundary noise).
func allDiffsInTail(diffs []JsonDiff, topHits int) bool {
	prefix := "search_result.hits.hits["
	for _, d := range diffs {
		if d.Path == "search_result.hits.total" {
			continue
		}
		if strings.HasPrefix(d.Path, prefix) {
			rest := d.Path[len(prefix):]
			end := strings.IndexByte(rest, ']')
			if end > 0 {
				var idx int
				fmt.Sscanf(rest[:end], "%d", &idx)
				if idx >= topHits {
					continue
				}
			}
		}
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Run full comparison (with health check + adaptive throttle)
// ---------------------------------------------------------------------------

// RunSearchComparison compares both servers for every query.
// If elasticURL is non-empty, an ES health checker runs in parallel:
//   - Queries block while ES is unhealthy.
//   - Concurrency halves on each error, ramps back up on sustained success.
func RunSearchComparison(
	queries []SearchCompareQuery,
	baseURL, expURL, elasticURL string,
	pageSize, concurrency, topHits int,
	ignoreScores bool,
) SearchCompareSummary {
	if concurrency <= 0 {
		concurrency = 5
	}

	health := newESHealth()
	limiter := newAdaptiveLimiter(concurrency, health)

	// Start ES health checker if a URL was provided.
	stop := make(chan struct{})
	if elasticURL != "" {
		log.Infof("ES health checker: %s", elasticURL)
		go runESHealthChecker(elasticURL, health, limiter.WakeAll, stop)
		// Give the first ping a moment to land before we start querying.
		time.Sleep(300 * time.Millisecond)
	} else {
		log.Info("No --elastic_url provided — skipping ES health checks")
	}

	results := make([]SearchCompareResult, len(queries))
	var wg sync.WaitGroup
	done := make(chan int, len(queries)) // signals completed index for progress logging

	for i, q := range queries {
		wg.Add(1)
		go func(idx int, query SearchCompareQuery) {
			defer wg.Done()

			// Block here until ES is healthy AND a concurrency slot is free.
			limiter.Acquire()

			r := CompareSearchQuery(query, baseURL, expURL, pageSize, topHits, ignoreScores)
			hadError := r.BaseError != "" || r.ExpError != ""
			if hadError && !health.IsHealthy() {
				r.ElasticErr = "ES unhealthy at query time"
				log.Warnf("[%s] %q skipped — ES unhealthy at query time", query.Language, query.Query)
			}
			limiter.Release(hadError)

			results[idx] = r
			done <- idx
		}(i, q)
	}

	// Progress logger: prints a line every 20 completed queries.
	go func() {
		completed := 0
		for range done {
			completed++
			if completed%20 == 0 || completed == len(queries) {
				log.Infof("Progress: %d/%d queries done", completed, len(queries))
			}
		}
	}()

	wg.Wait()
	close(stop)  // stop health checker
	close(done)

	summary := SearchCompareSummary{
		Results:      results,
		ElasticPings: health.Pings(),
	}
	for _, r := range results {
		summary.Total++
		switch {
		case r.ElasticErr != "":
			summary.Skipped++
		case r.BaseError != "" || r.ExpError != "":
			summary.Errors++
		case r.Identical:
			summary.Identical++
		case r.IdenticalTop:
			summary.IdenticalTop++
		default:
			summary.Different++
		}
	}
	return summary
}

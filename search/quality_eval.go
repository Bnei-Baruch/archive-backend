package search

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

const (
	EVAL_SET_EXPECTATION_FIRST_COLUMN = 4
	EVAL_SET_EXPECTATION_LAST_COLUMN  = 8
)

// Search quality enum. Order important, the lower (higher integer) the better.
const (
	SQ_SERVER_ERROR   = iota
	SQ_NO_EXPECTATION = iota
	SQ_UNKNOWN        = iota
	SQ_REGULAR        = iota
	SQ_GOOD           = iota
)

var SEARCH_QUALITY_NAME = map[int]string{
	SQ_GOOD:           "Good",
	SQ_REGULAR:        "Regular",
	SQ_UNKNOWN:        "Unknown",
	SQ_NO_EXPECTATION: "NoExpectation",
	SQ_SERVER_ERROR:   "ServerError",
}

// Compare results classification.
const (
	CR_WIN            = iota
	CR_LOSS           = iota
	CR_SAME           = iota
	CR_NO_EXPECTATION = iota
	CR_ERROR          = iota
)

var COMPARE_RESULTS_NAME = map[int]string{
	CR_WIN:   "Win",
	CR_LOSS:  "Loss",
	CR_SAME:  "Same",
	CR_ERROR: "Error",
}

const (
	ET_CONTENT_UNITS = iota
	ET_COLLECTIONS   = iota
	ET_LESSONS       = iota
	ET_PROGRAMS      = iota
	ET_SOURCES       = iota
	ET_EMPTY         = iota
	ET_FAILED_PARSE  = iota
	ET_BAD_STRUCTURE = iota
)

var GOOD_EXPECTATION = map[int]bool{
	ET_CONTENT_UNITS: true,
	ET_COLLECTIONS:   true,
	ET_LESSONS:       true,
	ET_PROGRAMS:      true,
	ET_SOURCES:       true,
	ET_EMPTY:         false,
	ET_FAILED_PARSE:  false,
	ET_BAD_STRUCTURE: false,
}

var EXPECTATION_TO_NAME = map[int]string{
	ET_CONTENT_UNITS: "cu",
	ET_COLLECTIONS:   "c",
	ET_LESSONS:       "l",
	ET_PROGRAMS:      "p",
	ET_SOURCES:       "s",
	ET_EMPTY:         "e",
	ET_FAILED_PARSE:  "fp",
	ET_BAD_STRUCTURE: "bs",
}

var EXPECTATION_URL_PATH = map[int]string{
	ET_CONTENT_UNITS: "cu",
	ET_COLLECTIONS:   "c",
	ET_LESSONS:       "lessons",
	ET_PROGRAMS:      "programs",
	ET_SOURCES:       "sources",
}

var EXPECTATION_HIT_TYPE = map[int]string{
	ET_CONTENT_UNITS: "content_units",
	ET_COLLECTIONS:   "collections",
	ET_LESSONS:       consts.INTENT_HIT_TYPE_LESSONS,
	ET_PROGRAMS:      consts.INTENT_HIT_TYPE_PROGRAMS,
	ET_SOURCES:       "sources",
}

const (
	FILTER_NAME_SOURCE = "source"
	FILTER_NAME_TOPIC  = "topic"
)

type Filter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Expectation struct {
	Type    int      `json:"type"`
	Uid     string   `json:"uid,omitempty"`
	Filters []Filter `json:"filters,omitempty"`
	Source  string   `json:"source"`
}

type Loss struct {
	Expectation Expectation `json:"expectation,omitempty"`
	Query       EvalQuery   `json:"query,omitempty"`
	Unique      float64     `json:"unique,omitempty"`
	Weighted    float64     `json:"weighted,omitempty"`
}

type EvalQuery struct {
	Language     string        `json:"language"`
	Query        string        `json:"query"`
	Weight       uint64        `json:"weight,omitempty"`
	Bucket       string        `json:"bucket,omitempty"`
	Expectations []Expectation `json:"expectations"`
	Comment      string        `json:"comment,omitempty"`
}

type EvalResults struct {
	Results       []EvalResult    `json:"results"`
	TotalUnique   uint64          `json:"total_unique"`
	TotalWeighted uint64          `json:"total_weighted"`
	TotalErrors   uint64          `json:"total_errors"`
	UniqueMap     map[int]float64 `json:"unique_map"`
	WeightedMap   map[int]float64 `json:"weighted_map"`
}

type EvalResult struct {
	SearchQuality []int `json:"search_quality"`
	Rank          []int `json:"rank"`
	err           error `json:"error"`
}

// Returns compare results classification constant.
func CompareResults(base int, exp int) int {
	if base == SQ_NO_EXPECTATION || exp == SQ_NO_EXPECTATION {
		return CR_NO_EXPECTATION
	} else if base == SQ_SERVER_ERROR || exp == SQ_SERVER_ERROR {
		return CR_ERROR
	} else if base == exp {
		return CR_SAME
	} else if base < exp {
		return CR_WIN // Experiment is better
	} else {
		return CR_LOSS // Base is better
	}
}

func GoodExpectations(expectations []Expectation) int {
	ret := 0
	for i := range expectations {
		if GOOD_EXPECTATION[expectations[i].Type] {
			ret++
		}
	}
	return ret
}

func ReadEvalSet(evalSetPath string) ([]EvalQuery, error) {
	f, err := os.Open(evalSetPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read File into a Variable
	r := csv.NewReader(f)
	lines, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	expectationsCount := 0
	queriesWithExpectationsCount := 0
	var ret []EvalQuery
	for _, line := range lines {
		w, err := strconv.ParseUint(line[2], 10, 64)
		if err != nil {
			log.Warnf("Failed parsing query [%s] weight [%s].", line[1], line[2])
			continue
		}
		var expectations []Expectation
		hasGoodExpectations := false
		for i := EVAL_SET_EXPECTATION_FIRST_COLUMN; i <= EVAL_SET_EXPECTATION_LAST_COLUMN; i++ {
			e := ParseExpectation(line[i])
			expectations = append(expectations, e)
			if GOOD_EXPECTATION[e.Type] {
				expectationsCount++
				hasGoodExpectations = true
			}
		}
		if hasGoodExpectations {
			queriesWithExpectationsCount++
		}
		ret = append(ret, EvalQuery{
			Language:     line[0],
			Query:        line[1],
			Weight:       w,
			Bucket:       line[3],
			Expectations: expectations,
			Comment:      line[EVAL_SET_EXPECTATION_LAST_COLUMN+1],
		})
	}
	log.Infof("Read %d queries, with total %d expectations. %d Queries had expectations.",
		len(lines), expectationsCount, queriesWithExpectationsCount)
	return ret, nil
}

type HitSource struct {
	MdbUid string `json:"mdb_uid"`
}

// Parses expectation described by result URL and converts
// to type (collections or content_units) and uid.
// Examples:
// https://archive.kbb1.com/he/programs/cu/AsNLozeK ==> (content_units, AsNLozeK)
// https://archive.kbb1.com/he/programs/c/fLWpcUjQ  ==> (collections  , fLWpcUjQ)
// https://archive.kbb1.com/he/lessons?source=bs_L2jMWyce_kB3eD83I       ==> (lessons,  nil, source=bs_L2jMWyce_kB3eD83I)
// https://archive.kbb1.com/he/programs?topic=g3ml0jum_1nyptSIo_RWqjxgkj ==> (programs, nil, topic=g3ml0jum_1nyptSIo_RWqjxgkj)
// https://archive.kbb1.com/he/sources/kB3eD83I ==> (sources, kB3eD83I)
// All events sub pages and years:
// https://archive.kbb1.com/he/events/meals
// https://archive.kbb1.com/he/events/friends-gatherings
// https://archive.kbb1.com/he/events?year=2013
func ParseExpectation(e string) Expectation {
	if e == "" {
		return Expectation{ET_EMPTY, "", nil, e}
	}
	u, err := url.Parse(e)
	if err != nil {
		return Expectation{ET_FAILED_PARSE, "", nil, e}
	}
	p := u.RequestURI()
	idx := strings.Index(p, "?")
	q := "" // The query part, i.e., .../he/lessons?source=bs_L2jMWyce_kB3eD83I => source=bs_L2jMWyce_kB3eD83I
	if idx >= 0 {
		q = p[idx+1:]
		p = p[:idx]
	}
	// Last part .../he/programs/cu/AsNLozeK => AsNLozeK   or   /he/lessons => lessons
	uidOrSection := path.Base(p)
	// One before last part .../he/programs/cu/AsNLozeK => cu
	contentUnitOrCollection := path.Base(path.Dir(p))
	t := -1
	switch uidOrSection {
	case EXPECTATION_URL_PATH[ET_LESSONS]:
		t = ET_LESSONS
	case EXPECTATION_URL_PATH[ET_PROGRAMS]:
		t = ET_PROGRAMS
	}
	if t != -1 {
		queryParts := strings.Split(q, ",")
		filters := make([]Filter, len(queryParts))
		for i, qp := range queryParts {
			nameValue := strings.Split(qp, "=")
			if len(nameValue) > 0 {
				filters[i].Name = nameValue[0]
				if len(nameValue) > 1 {
					filters[i].Value = nameValue[1]
				}
			}
		}
		return Expectation{t, "", filters, e}
	}
	switch contentUnitOrCollection {
	case EXPECTATION_URL_PATH[ET_CONTENT_UNITS]:
		t = ET_CONTENT_UNITS
	case EXPECTATION_URL_PATH[ET_COLLECTIONS]:
		t = ET_COLLECTIONS
	case EXPECTATION_URL_PATH[ET_SOURCES]:
		t = ET_SOURCES
	default:
		return Expectation{ET_BAD_STRUCTURE, "", nil, e}
	}
	return Expectation{t, uidOrSection, nil, e}
}

func FilterValueToUid(value string) string {
	sl := strings.Split(value, "_")
	if len(sl) == 0 {
		return ""
	}
	return sl[len(sl)-1]
}

func HitMatchesExpectation(hit *elastic.SearchHit, hitSource HitSource, e Expectation) bool {
	if hit.Type != EXPECTATION_HIT_TYPE[e.Type] {
		return false
	}

	if e.Type == ET_LESSONS || e.Type == ET_PROGRAMS {
		// For now we support only one filter (zero also means not match).
		if len(e.Filters) == 0 || len(e.Filters) > 1 {
			return false
		}
		// Match all filters
		filter := e.Filters[0]
		return ((filter.Name == FILTER_NAME_TOPIC && hit.Index == consts.INTENT_INDEX_TAG) ||
			(filter.Name == FILTER_NAME_SOURCE && hit.Index == consts.INTENT_INDEX_SOURCE)) &&
			FilterValueToUid(filter.Value) == hitSource.MdbUid
	} else {
		return hitSource.MdbUid == e.Uid
	}
}

func EvaluateQuery(q EvalQuery, serverUrl string) EvalResult {
	r := EvalResult{
		SearchQuality: make([]int, len(q.Expectations)),
		Rank:          make([]int, len(q.Expectations)),
		err:           nil,
	}

	hasGoodExpectation := false
	for i := range q.Expectations {
		r.SearchQuality[i] = SQ_NO_EXPECTATION
		r.Rank[i] = -1
		if GOOD_EXPECTATION[q.Expectations[i].Type] {
			hasGoodExpectation = true
		}
	}
	// Optimization, don't fetch query if no good expectations.
	if !hasGoodExpectation {
		return r
	}

	urlTemplate := "%s/search?q=%s&language=%s&page_no=1&page_size=10&sort_by=relevance&deb=true"
	url := fmt.Sprintf(urlTemplate, serverUrl, url.QueryEscape(q.Query), q.Language)
	resp, err := http.Get(url)
	if err != nil {
		log.Warnf("Error %+v", err)
		for i := range q.Expectations {
			if GOOD_EXPECTATION[q.Expectations[i].Type] {
				r.SearchQuality[i] = SQ_SERVER_ERROR
			}
		}
		r.err = err
		return r
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			r.err = errors.Wrapf(err, "Status not ok (%d), failed reading body.", resp.StatusCode)
		}
		errMsg := fmt.Sprintf("Status not ok (%d), body: %s", resp.StatusCode, string(bodyBytes))
		log.Warn(errMsg)
		for i := range q.Expectations {
			if GOOD_EXPECTATION[q.Expectations[i].Type] {
				r.SearchQuality[i] = SQ_SERVER_ERROR
			}
		}
		r.err = errors.New(errMsg)
		return r
	}
	queryResult := QueryResult{}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&queryResult); err != nil {
		log.Warnf("Error decoding %+v", err)
		for i := range q.Expectations {
			if GOOD_EXPECTATION[q.Expectations[i].Type] {
				r.SearchQuality[i] = SQ_SERVER_ERROR
			}
		}
		r.err = err
		return r
	}
	for i := range q.Expectations {
		if GOOD_EXPECTATION[q.Expectations[i].Type] {
			sq := SQ_UNKNOWN
			rank := -1
			for j, hit := range queryResult.SearchResult.Hits.Hits {
				hitSource := HitSource{}
				if err := json.Unmarshal(*hit.Source, &hitSource); err != nil {
					log.Warnf("Error unmarshling source %+v", err)
					sq = SQ_SERVER_ERROR
					rank = -1
					r.err = err
					break
				}
				if HitMatchesExpectation(hit, hitSource, q.Expectations[i]) {
					rank = j + 1
					if j <= 2 {
						sq = SQ_GOOD
					} else {
						sq = SQ_REGULAR
					}
					break
				}
			}
			r.SearchQuality[i] = sq
			r.Rank[i] = rank
		}
	}

	return r
}

func Eval(queries []EvalQuery, serverUrl string) (EvalResults, map[int][]Loss, error) {
	log.Infof("Evaluating %d queries on %s.", len(queries), serverUrl)
	ret := EvalResults{}
	ret.UniqueMap = make(map[int]float64)
	ret.WeightedMap = make(map[int]float64)
	for _, q := range queries {
		r := EvaluateQuery(q, serverUrl)
		goodExpectations := GoodExpectations(q.Expectations)
		if goodExpectations > 0 {
			for i, sq := range r.SearchQuality {
				if GOOD_EXPECTATION[q.Expectations[i].Type] {
					ret.UniqueMap[sq] += 1 / float64(goodExpectations)
					// Each expectation has equal weight for the query.
					ret.WeightedMap[sq] += float64(q.Weight) / float64(goodExpectations)
				}
			}
		} else {
			// Meaning that the query has not any good expectation.
			ret.UniqueMap[SQ_NO_EXPECTATION]++
			ret.WeightedMap[SQ_NO_EXPECTATION] += float64(q.Weight)
		}
		ret.TotalUnique++
		ret.TotalWeighted += q.Weight
		if r.err != nil {
			ret.TotalErrors++
		}
		ret.Results = append(ret.Results, r)
	}
	for k, v := range ret.UniqueMap {
		ret.UniqueMap[k] = v / float64(ret.TotalUnique)
	}
	for k, v := range ret.WeightedMap {
		ret.WeightedMap[k] = v / float64(ret.TotalWeighted)
	}

	// Print detailed loss (Unknown) analysis
	losses := make(map[int][]Loss)
	for i, q := range queries {
		for j, sq := range ret.Results[i].SearchQuality {
			e := q.Expectations[j]
			if sq == SQ_UNKNOWN {
				if _, ok := losses[e.Type]; !ok {
					losses[e.Type] = make([]Loss, 0)
				}
				loss := Loss{e, q, 1 / float64(len(q.Expectations)), float64(q.Weight) / float64(len(q.Expectations))}
				losses[e.Type] = append(losses[e.Type], loss)
			}
		}
	}

	return ret, losses, nil
}

func ExpectationToString(e Expectation) string {
	filters := make([]string, len(e.Filters))
	for i, f := range e.Filters {
		filters[i] = fmt.Sprintf("%s - %s", f.Name, f.Value)
	}
	return fmt.Sprintf("%s|%s|%s", EXPECTATION_TO_NAME[e.Type], e.Uid, strings.Join(filters, ":"))
}

func WriteResults(path string, queries []EvalQuery, results EvalResults) error {
	records := [][]string{{"Language", "Query", "Weight", "Bucket", "Comment"}}
	for i := 0; i < EVAL_SET_EXPECTATION_LAST_COLUMN-EVAL_SET_EXPECTATION_FIRST_COLUMN+1; i++ {
		records[0] = append(records[0], fmt.Sprintf("#%d", i+1))
		records[0] = append(records[0], fmt.Sprintf("#%d Parsed", i+1))
		records[0] = append(records[0], fmt.Sprintf("#%d SQ", i+1))
		records[0] = append(records[0], fmt.Sprintf("#%d Rank", i+1))
	}
	for i, q := range queries {
		record := []string{q.Language, q.Query, fmt.Sprintf("%d", q.Weight), q.Bucket, q.Comment}
		for j, sq := range results.Results[i].SearchQuality {
			record = append(record, q.Expectations[j].Source)
			record = append(record, ExpectationToString(q.Expectations[j]))
			record = append(record, SEARCH_QUALITY_NAME[sq])
			record = append(record, fmt.Sprintf("%d", results.Results[i].Rank[j]))
		}
		records = append(records, record)
	}

	return WriteToCsv(path, records)
}

func WriteToCsv(path string, records [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	w := csv.NewWriter(file)

	for _, record := range records {
		if err := w.Write(record); err != nil {
			return err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return nil
}

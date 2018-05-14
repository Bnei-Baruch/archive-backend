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
)

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
	ET_LESSONS:       "intent-lessons",  // Special hit type. Should be handled as intent.
	ET_PROGRAMS:      "intent-programs", // Special hit type. Should be handled as intent.
	ET_SOURCES:       "sources",
}

type Filter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Expectation struct {
	Type    int      `json:"type"`
	Uid     string   `json:"uid,omitempty"`
	Filters []Filter `json:"filters,omitempty"`
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
		for i := 4; i <= 8; i++ {
			e, err := ParseExpectation(line[i])
			if err == nil {
				expectations = append(expectations, e)
				expectationsCount++
			}
		}
		if len(expectations) > 0 {
			queriesWithExpectationsCount++
		}
		ret = append(ret, EvalQuery{
			Language:     line[0],
			Query:        line[1],
			Weight:       w,
			Bucket:       line[3],
			Expectations: expectations,
			Comment:      line[5],
		})
	}
	log.Infof("Read %d queries, with total %d expectations. %d Queries had expectations.",
		len(lines), expectationsCount, queriesWithExpectationsCount)
	for _, q := range ret {
		if len(q.Expectations) > 0 {
			log.Infof("[%s]", q.Query)
			for _, e := range q.Expectations {
				log.Infof("\t(%s, %s)", EXPECTATION_HIT_TYPE[e.Type], e.Uid)
			}
		}
	}
	return ret, nil
}

type MdbUid struct {
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
func ParseExpectation(e string) (Expectation, error) {
	u, err := url.Parse(e)
	if err != nil {
		return Expectation{}, err
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
		return Expectation{t, "", filters}, nil
	}
	switch contentUnitOrCollection {
	case EXPECTATION_URL_PATH[ET_CONTENT_UNITS]:
		t = ET_CONTENT_UNITS
	case EXPECTATION_URL_PATH[ET_COLLECTIONS]:
		t = ET_COLLECTIONS
	case EXPECTATION_URL_PATH[ET_SOURCES]:
		t = ET_SOURCES
	default:
		return Expectation{}, errors.New("ParseExpectation - Could not parse expectation.")
	}
	return Expectation{t, uidOrSection, nil}, nil
}

func EvaluateQuery(q EvalQuery, serverUrl string) EvalResult {
	r := EvalResult{}

	if len(q.Expectations) == 0 {
		return r
	}

	urlTemplate := "%s/search?q=%s&language=%s&page_no=1&page_size=10&sort_by=relevance&deb=true"
	url := fmt.Sprintf(urlTemplate, serverUrl, url.QueryEscape(q.Query), q.Language)
	log.Infof("Url: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		log.Warnf("Error %+v", err)
		for _ = range q.Expectations {
			r.SearchQuality = append(r.SearchQuality, SQ_SERVER_ERROR)
			r.Rank = append(r.Rank, -1)
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
		for _ = range q.Expectations {
			r.SearchQuality = append(r.SearchQuality, SQ_SERVER_ERROR)
			r.Rank = append(r.Rank, -1)
		}
		r.err = errors.New(errMsg)
		return r
	}
	queryResult := QueryResult{}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&queryResult); err != nil {
		log.Warnf("Error decoding %+v", err)
		for _ = range q.Expectations {
			r.SearchQuality = append(r.SearchQuality, SQ_SERVER_ERROR)
			r.Rank = append(r.Rank, -1)
		}
		r.err = err
		return r
	}
	log.Infof("EvaluateQuery - searchResult: %+v", queryResult)
	for j, e := range q.Expectations {
		sq := SQ_UNKNOWN
		rank := -1
		for i, hit := range queryResult.SearchResult.Hits.Hits {
			mdbUid := MdbUid{}
			if err := json.Unmarshal(*hit.Source, &mdbUid); err != nil {
				log.Warnf("Error unmarshling source %+v", err)
				sq = SQ_SERVER_ERROR
				rank = -1
				r.err = err
				break
			}
			if j == 0 {
				log.Infof("[%s] [%s] type: %s mdb_uid: %s @%d", serverUrl, q.Query, hit.Type, mdbUid.MdbUid, i+1)
			}
			if mdbUid.MdbUid == e.Uid && hit.Type == EXPECTATION_HIT_TYPE[e.Type] {
				rank = i + 1
				if i <= 2 {
					sq = SQ_GOOD
				} else {
					sq = SQ_REGULAR
				}
				break
			}
		}
		// We need to add program/lesson to intent before validating expectation.
		// Check intents
		// for _, intent := range queryResult.Intents {
		//     if classificationIntent, ok := intent.Value.(es.ClassificationIntent); ok {
		//         intent.Type == consts.INTENT_SOURCE && e.Type == == consts.SOURCE_CLASSIFICATION_TYPE
		//     }
		//     if e.Type == ET_LESSONS && intent.Type == consts.INTENT_TAG {
		//     }
		// }
		r.SearchQuality = append(r.SearchQuality, sq)
		r.Rank = append(r.Rank, rank)
	}

	return r
}

func Eval(queries []EvalQuery, serverUrl string) (EvalResults, map[int][]Loss, error) {
	log.Infof("Evaluating %d queries.", len(queries))
	ret := EvalResults{}
	ret.UniqueMap = make(map[int]float64)
	ret.WeightedMap = make(map[int]float64)
	for _, q := range queries {
		r := EvaluateQuery(q, serverUrl)
		if len(r.SearchQuality) == 0 {
			ret.UniqueMap[SQ_NO_EXPECTATION]++
			ret.WeightedMap[SQ_NO_EXPECTATION] += float64(q.Weight)
		}
		for _, sq := range r.SearchQuality {
			ret.UniqueMap[sq] += 1 / float64(len(q.Expectations))
			// Each expectation has equal weight for the query.
			ret.WeightedMap[sq] += float64(q.Weight) / float64(len(q.Expectations))
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

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

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"
)

type EvalQuery struct {
	Language    string `json:"language"`
	Query       string `json:"query"`
	Weight      uint64 `json:"weight,omitempty"`
	Bucket      string `json:"bucket,omitempty"`
	Expectation string `json:"expectation"`
	Comment     string `json:"comment,omitempty"`
}

type EvalResults struct {
	Results             []EvalResult `json:"results"`
	TotalUnique         uint64       `json:"total_unique"`
	TotalWeighted       uint64       `json:"total_weighted"`
	TotalErrors         uint64       `json:"total_errors"`
	RecallUnique        float64      `json:"recall_unique"`
	RecallWeighted      float64      `json:"recall_weighted"`
	RegularUnique       float64      `json:"regular_unique"`
	RegularWeighted     float64      `json:"regular_weighted"`
	UnknownUnique       float64      `json:"unknown_unique"`
	UnknownWeighted     float64      `json:"unknown_weighted"`
	ServerErrorUnique   float64      `json:"server_error_unique"`
	ServerErrorWeighted float64      `json:"server_error_weighted"`
}

// Search quality enum. Order important, the lower (higher integer) the better.
const (
    SQ_SERVER_ERROR = iota
    SQ_UNKNOWN      = iota
    SQ_REGULAR      = iota
    SQ_GOOD         = iota
)

var SEARCH_QUALITY_NAME = map[uint32]string{
	SQ_GOOD:         "Good",
	SQ_REGULAR:      "Regular",
	SQ_UNKNOWN:      "Unknown",
    SQ_SERVER_ERROR: "ServerError",
}

// Compare results classification.
const (
    CR_WIN   = iota
    CR_LOSS  = iota
    CR_SAME  = iota
    CR_ERROR = iota
)

var COMPARE_RESULTS_NAME = map[uint32]string {
    CR_WIN:   "Win",
    CR_LOSS:  "Loss",
    CR_SAME:  "Same",
    CR_ERROR: "Error",
}

type EvalResult struct {
	SearchQuality uint32 `json:"search_quality"`
	Rank          uint64 `json:"rank"`
	err           error  `json:"error"`
}

// Returns compare results classification constant.
func CompareResults(base EvalResult, exp EvalResult) uint32 {
    if base.SearchQuality == SQ_SERVER_ERROR || exp.SearchQuality == SQ_SERVER_ERROR {
        return CR_ERROR
    } else if base.SearchQuality == exp.SearchQuality {
        return CR_SAME
    } else if base.SearchQuality < exp.SearchQuality {
        return CR_WIN  // Experiment is better
    } else {
        return CR_LOSS  // Base is better
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

	var ret []EvalQuery
	for _, line := range lines {
		w, err := strconv.ParseUint(line[2], 10, 64)
		if err != nil {
			log.Warnf("Failed parsing query [%s] weight [%s].", line[1], line[2])
			continue
		}
		ret = append(ret, EvalQuery{
			Language:    line[0],
			Query:       line[1],
			Weight:      w,
			Bucket:      line[3],
			Expectation: line[4],
			Comment:     line[5],
		})
	}
	return ret, nil
}

type MdbUid struct {
	MdbUid string `json:"mdb_uid"`
}

func ParseUidExpectation(e string) (string, error) {
	u, err := url.Parse(e)
	if err != nil {
		return "", err
	}
	return path.Base(u.RequestURI()), nil
}

func EvaluateQuery(q EvalQuery, serverUrl string) EvalResult {
	r := EvalResult{}
	r.SearchQuality = SQ_UNKNOWN

	uid, err := ParseUidExpectation(q.Expectation)
	if err != nil || uid == "" {
		log.Warnf("Bad Expectation %+v, [%s]", err, uid)
		r.SearchQuality = SQ_UNKNOWN
		r.err = err
		return r
	}

	urlTemplate := "%s/search?q=%s&language=%s&page_no=1&page_size=10&sort_by=relevance&deb=true"
	url := fmt.Sprintf(urlTemplate, serverUrl, url.QueryEscape(q.Query), q.Language)
	log.Infof("Url: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		log.Warnf("Error %+v", err)
		r.SearchQuality = SQ_SERVER_ERROR
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
		r.SearchQuality = SQ_SERVER_ERROR
		r.err = errors.New(errMsg)
		return r
	}
	searchResult := elastic.SearchResult{}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		log.Warnf("Error decoding %+v", err)
		r.SearchQuality = SQ_SERVER_ERROR
		r.err = err
		return r
	}
	for i, hit := range searchResult.Hits.Hits {
		mdbUid := MdbUid{}
		if err := json.Unmarshal(*hit.Source, &mdbUid); err != nil {
			log.Warnf("Error unmarshling source %+v", err)
			r.SearchQuality = SQ_SERVER_ERROR
			r.err = err
			return r
		}
		log.Infof("mdb_uid: %s @%d", mdbUid.MdbUid, i+1)
		if mdbUid.MdbUid == uid {
			r.Rank = uint64(i + 1)
			if i <= 2 {
				r.SearchQuality = SQ_GOOD
			} else {
				r.SearchQuality = SQ_REGULAR
			}
			break
		}
	}

	return r
}

func Eval(queries []EvalQuery, serverUrl string) (EvalResults, error) {
	log.Infof("Evaluating %d queries.", len(queries))
	ret := EvalResults{}
	for _, q := range queries {
		r := EvaluateQuery(q, serverUrl)
		if r.SearchQuality == SQ_GOOD {
			ret.RecallUnique++
			ret.RecallWeighted += float64(q.Weight)
		} else if r.SearchQuality == SQ_REGULAR {
			ret.RegularUnique++
			ret.RegularWeighted += float64(q.Weight)
		} else if r.SearchQuality == SQ_UNKNOWN {
			ret.UnknownUnique++
			ret.UnknownWeighted += float64(q.Weight)
		} else if r.SearchQuality == SQ_SERVER_ERROR {
			ret.ServerErrorUnique++
			ret.ServerErrorWeighted += float64(q.Weight)
		}
		ret.TotalUnique++
		ret.TotalWeighted += q.Weight
		if r.err != nil {
			ret.TotalErrors++
		}
		ret.Results = append(ret.Results, r)
	}
	ret.RecallUnique = ret.RecallUnique / float64(ret.TotalUnique)
	ret.RecallWeighted = ret.RecallWeighted / float64(ret.TotalWeighted)
	ret.RegularUnique = ret.RegularUnique / float64(ret.TotalUnique)
	ret.RegularWeighted = ret.RegularWeighted / float64(ret.TotalWeighted)
	ret.UnknownUnique = ret.UnknownUnique / float64(ret.TotalUnique)
	ret.UnknownWeighted = ret.UnknownWeighted / float64(ret.TotalWeighted)
	ret.ServerErrorUnique = ret.ServerErrorUnique / float64(ret.TotalUnique)
	ret.ServerErrorWeighted = ret.ServerErrorWeighted / float64(ret.TotalWeighted)
	return ret, nil
}

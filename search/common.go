package search

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/volatiletech/null/v8"
)

func (query *Query) ToString() string {
	queryToPrint := *query
	for i := range queryToPrint.Intents {
		if value, ok := queryToPrint.Intents[i].Value.(ClassificationIntent); ok {
			value.Explanation = elastic.SearchExplanation{0.0, "Don't print.", nil}
			value.MaxExplanation = value.Explanation
			queryToPrint.Intents[i].Value = value
		}
	}
	return fmt.Sprintf("%+v", queryToPrint)
}

func (query *Query) ToSimpleString() string {
	return query.ToFullSimpleString(consts.SORT_BY_RELEVANCE, 0, 10)
}

func (query *Query) ToFullSimpleString(sortBy string, from int, size int) string {
	page := ""
	if from != 0 || size != 10 {
		page = fmt.Sprintf(" (%d-%d)", from, from+size)
	}
	exact := []string{}
	for _, e := range query.ExactTerms {
		exact = append(exact, fmt.Sprintf("\"%s\"", e))
	}
	exactStr := ""
	if len(exact) > 0 {
		exactStr = fmt.Sprintf(" %s", strings.Join(exact, ","))
	}
	filters := []string{}
	for k, v := range query.Filters {
		if len(v) > 0 {
			filters = append(filters, fmt.Sprintf("%s=%s", k, strings.Join(v, ",")))
		}
	}
	filtersStr := ""
	if len(filters) > 0 {
		filtersStr = fmt.Sprintf(" %s", strings.Join(filters, ","))
	}
	language := ""
	if len(query.LanguageOrder) > 0 {
		language = fmt.Sprintf("%s ", query.LanguageOrder[0])
	}
	deb := ""
	if query.Deb {
		deb = " deb"
	}
	sortStr := ""
	if sortBy != consts.SORT_BY_RELEVANCE {
		sortStr = fmt.Sprintf(" %s", sortBy)
	}
	return fmt.Sprintf("%s[%s%s]%s%s%s%s", language, query.Term, exactStr, filtersStr, page, sortStr, deb)
}

func CombineBySourceOrTag(byLang map[string]*elastic.SearchResult, typeNameForGroupBySource null.String, typeNameForGroupByTag null.String) map[string]*elastic.SearchResult {
	for l, r := range byLang {
		hitBySource := make(map[string]*elastic.SearchHit)
		hitByTag := make(map[string]*elastic.SearchHit)
		hitsWithoutSourceOrTag := []*elastic.SearchHit{}
		var maxScore *float64
		for _, h := range r.Hits.Hits {
			suid, tuid := GetHitSourceAndTag(h)
			if typeNameForGroupBySource.Valid && suid != "" {
				if val, hasKey := hitBySource[suid]; !hasKey || (h.Score != nil && *h.Score > *val.Score) {
					hitBySource[suid] = h
				}
			} else if typeNameForGroupByTag.Valid && tuid != "" {
				if val, hasKey := hitByTag[tuid]; !hasKey || (h.Score != nil && *h.Score > *val.Score) {
					hitByTag[tuid] = h
				}
			} else {
				hitsWithoutSourceOrTag = append(hitsWithoutSourceOrTag, h)
				if maxScore == nil || *h.Score > *maxScore {
					maxScore = h.Score
				}
			}
		}

		byLang[l].Hits = new(elastic.SearchHits)
		byLang[l].Hits.MaxScore = maxScore
		byLang[l].Hits.Hits = hitsWithoutSourceOrTag
		for k, h := range hitBySource {
			newH := &elastic.SearchHit{
				Source:      h.Source,
				Type:        typeNameForGroupBySource.String,
				Score:       h.Score,
				Uid:         k,
				Explanation: h.Explanation,
			}
			if byLang[l].Hits.MaxScore == nil || *h.Score > *byLang[l].Hits.MaxScore {
				byLang[l].Hits.MaxScore = h.Score
			}
			byLang[l].Hits.TotalHits++
			byLang[l].Hits.Hits = append(byLang[l].Hits.Hits, newH)
		}
		for k, h := range hitByTag {
			newH := &elastic.SearchHit{
				Source:      h.Source,
				Type:        typeNameForGroupByTag.String,
				Score:       h.Score,
				Uid:         k,
				Explanation: h.Explanation,
			}
			if byLang[l].Hits.MaxScore == nil || *h.Score > *byLang[l].Hits.MaxScore {
				byLang[l].Hits.MaxScore = h.Score
			}
			byLang[l].Hits.TotalHits++
			byLang[l].Hits.Hits = append(byLang[l].Hits.Hits, newH)
		}
		if byLang[l].Hits == nil {
			delete(byLang, l)
		}
	}
	return byLang
}

func GetHitSourceAndTag(hit *elastic.SearchHit) (string, string) {
	var res es.Result
	if err := json.Unmarshal(*hit.Source, &res); err != nil {
		return "", ""
	}
	tagKeys := make(map[string]bool)
	srcKeys := make(map[string]bool)

	if res.ResultType != consts.ES_RESULT_TYPE_COLLECTIONS {
		return "", ""
	}
	for _, tId := range res.TypedUids {
		l := strings.Split(tId, ":")
		if l[0] == consts.ES_UID_TYPE_TAG && l[1] != "" {
			if _, ok := tagKeys[l[1]]; !ok { // avoid duplicates
				tagKeys[l[1]] = true
			}
		}
		if l[0] == consts.ES_UID_TYPE_SOURCE && l[1] != "" {
			if _, ok := srcKeys[l[1]]; !ok { // avoid duplicates
				srcKeys[l[1]] = true
			}
		}
	}
	var srcList []string
	for k := range srcKeys {
		srcList = append(srcList, k)
	}
	var tagList []string
	for k := range tagKeys {
		tagList = append(tagList, k)
	}
	src := strings.Join(srcList, "_")
	tag := strings.Join(tagList, "_")

	return src, tag
}

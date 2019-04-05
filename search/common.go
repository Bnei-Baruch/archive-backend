package search

import (
	"fmt"
	"strings"

	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
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

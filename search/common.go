package search

import (
	"fmt"

	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/es"
)

func (query *Query) ToString() string {
	queryToPrint := query
	for i := range queryToPrint.Intents {
		if value, ok := queryToPrint.Intents[i].Value.(es.ClassificationIntent); ok {
			value.Explanation = elastic.SearchExplanation{0.0, "Don't print.", nil}
			value.MaxExplanation = value.Explanation
			queryToPrint.Intents[i].Value = value
		}
	}
	return fmt.Sprintf("%+v", queryToPrint)
}

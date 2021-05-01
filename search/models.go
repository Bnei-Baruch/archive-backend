package search

import (
	"context"

	"gopkg.in/olivere/elastic.v6"
	null "gopkg.in/volatiletech/null.v6"
)

type Intent struct {
	Type     string      `json:"type"`
	Language string      `json:"language"`
	Value    interface{} `json:"value,omitempty"`
}

type QueryResult struct {
	SearchResult *elastic.SearchResult `json:"search_result,omitempty"`
	TypoSuggest  null.String           `json:"typo_suggest"`
	Language     string                `json:"language"`
}

type Engine interface {
	GetSuggestions(ctx context.Context, query Query) (interface{}, error)
	DoSearch(ctx context.Context, query Query, from int, size int, preference string) (interface{}, error)
}

type SearchRequestOptions struct {
	resultTypes          []string
	docIds               []string
	index                string
	query                Query
	sortBy               string
	from                 int
	size                 int
	preference           string
	useHighlight         bool
	highlightFullContent bool
	// Following field comes to solve elastic bug with highlight.
	// Just removed the analyzed fields and uses only standard fields
	// for highlighting. Only happens with intents.
	partialHighlight bool
	// Following field comes to reduce results duplication.
	// If we have classification intent (carousel) by source, filter out this results from the main search.
	filterOutCUSources []string
	// Setting the following field to 'true' will ignore the search of content and in some cases also description.
	// Description is considered as subtitle in sources,
	//  so the 'description' field will be included only when this field is true and resultTypes contains only 'sources'.
	// This field is used for classification intents (carousel) search and grammar filter for 'books'.
	titlesOnly bool
	// Setting the following field to 'true' will include 'typed_uids' values for results of type 'content units'.
	// We use this data for a further filtering out of hits recieved from 'grammar filter' search that duplicates the carousel items.
	// Since the search for 'grammar filter' is async. to  classification intents (carousel) search, we don't have yet the data for filterOutCUSources field.
	includeTypedUidsFromContentUnits bool
	// If not nil, set how long a search is allowed to take, e.g. "1s" or "500ms". Note: Not always respected by ES.
	Timeout *string
	// Collection UID by which the content will be filtered out.
	filterOutByCollection *string
}

package compare

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

// TagsComparator compares tags between ES6 and ES9
type TagsComparator struct {
	es6Client    *elastic.Client
	es9Client    *elasticsearch.Client
	es9IndexBase string
}

// NewTagsComparator creates a new tags comparator
func NewTagsComparator(es6Client *elastic.Client, es9Client *elasticsearch.Client, es9IndexBase string) *TagsComparator {
	return &TagsComparator{
		es6Client:    es6Client,
		es9Client:    es9Client,
		es9IndexBase: es9IndexBase,
	}
}

// GetResultType returns the result type
func (c *TagsComparator) GetResultType() string {
	return consts.ES_RESULT_TYPE_TAGS
}

// GetES6IndexName returns ES6 index name for a language
func (c *TagsComparator) GetES6IndexName(lang string) string {
	return fmt.Sprintf("prod_%s_%s", consts.ES_RESULTS_INDEX, lang)
}

// GetES9IndexName returns ES9 index name for a language
func (c *TagsComparator) GetES9IndexName(lang string) string {
	return fmt.Sprintf("%s_%s", c.es9IndexBase, lang)
}

// GetES6Count returns total document count in ES6 for this result type
func (c *TagsComparator) GetES6Count(ctx context.Context, lang string) (int64, error) {
	indexName := c.GetES6IndexName(lang)
	query := elastic.NewBoolQuery().
		Filter(elastic.NewTermQuery("result_type", c.GetResultType()))

	count, err := c.es6Client.Count().
		Index(indexName).
		Query(query).
		Do(ctx)

	if err != nil {
		return 0, errors.Wrap(err, "count ES6 documents")
	}

	return count, nil
}

// GetES9Count returns total document count in ES9 for this result type
func (c *TagsComparator) GetES9Count(ctx context.Context, lang string) (int64, error) {
	indexName := c.GetES9IndexName(lang)

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"result_type": c.GetResultType(),
			},
		},
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return 0, errors.Wrap(err, "marshal ES9 count query")
	}

	res, err := c.es9Client.Count(
		c.es9Client.Count.WithContext(ctx),
		c.es9Client.Count.WithIndex(indexName),
		c.es9Client.Count.WithBody(strings.NewReader(string(queryBytes))),
	)
	if err != nil {
		return 0, errors.Wrap(err, "count ES9 documents")
	}
	defer res.Body.Close()

	if res.IsError() {
		return 0, fmt.Errorf("ES9 count error: %s", res.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, errors.Wrap(err, "decode ES9 count response")
	}

	count, ok := result["count"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid count in ES9 response")
	}

	return int64(count), nil
}

// Sample retrieves random document UIDs from ES6
func (c *TagsComparator) Sample(ctx context.Context, lang string, size int) ([]string, error) {
	indexName := c.GetES6IndexName(lang)

	query := elastic.NewBoolQuery().
		Filter(elastic.NewTermQuery("result_type", c.GetResultType()))

	// First, get the total count of documents
	countResult, err := c.es6Client.Count().
		Index(indexName).
		Query(query).
		Do(ctx)

	if err != nil {
		return nil, errors.Wrap(err, "count documents in ES6")
	}

	totalDocs := int(countResult)
	if totalDocs == 0 {
		return nil, fmt.Errorf("no documents found in ES6 index")
	}

	// If we have fewer docs than requested, adjust size
	if totalDocs < size {
		size = totalDocs
	}

	// Generate random offsets to fetch documents from different positions
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	uids := make([]string, 0, size)
	usedOffsets := make(map[int]bool)

	// Fetch documents at random positions
	for len(uids) < size {
		// Generate a random offset
		offset := rng.Intn(totalDocs)

		// Skip if we already used this offset
		if usedOffsets[offset] {
			continue
		}
		usedOffsets[offset] = true

		// Fetch a single document at this offset
		fetchResult, err := c.es6Client.Search().
			Index(indexName).
			Query(query).
			From(offset).
			Size(1).
			FetchSourceContext(elastic.NewFetchSourceContext(true).Include("mdb_uid")).
			Do(ctx)

		if err != nil {
			continue
		}

		if len(fetchResult.Hits.Hits) == 0 {
			continue
		}

		var doc map[string]interface{}
		if err := json.Unmarshal(*fetchResult.Hits.Hits[0].Source, &doc); err != nil {
			continue
		}

		if uid, ok := doc["mdb_uid"].(string); ok {
			uids = append(uids, uid)
		}
	}

	return uids, nil
}

// FetchES6Document retrieves a document from ES6
func (c *TagsComparator) FetchES6Document(ctx context.Context, lang string, uid string) (map[string]interface{}, error) {
	indexName := c.GetES6IndexName(lang)

	query := elastic.NewBoolQuery().
		Filter(elastic.NewTermQuery("result_type", c.GetResultType())).
		Filter(elastic.NewTermQuery("mdb_uid", uid))

	searchResult, err := c.es6Client.Search().
		Index(indexName).
		Query(query).
		Size(1).
		Do(ctx)

	if err != nil {
		return nil, errors.Wrap(err, "search ES6")
	}

	if len(searchResult.Hits.Hits) == 0 {
		return nil, fmt.Errorf("document not found in ES6: %s", uid)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(*searchResult.Hits.Hits[0].Source, &doc); err != nil {
		return nil, errors.Wrap(err, "unmarshal ES6 document")
	}

	return doc, nil
}

// FetchES9Document retrieves a document from ES9
func (c *TagsComparator) FetchES9Document(ctx context.Context, lang string, uid string) (map[string]interface{}, error) {
	indexName := c.GetES9IndexName(lang)

	// Build ES9 query
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"result_type": c.GetResultType()}},
					{"term": map[string]interface{}{"mdb_uid": uid}},
				},
			},
		},
		"size": 1,
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, errors.Wrap(err, "marshal ES9 query")
	}

	res, err := c.es9Client.Search(
		c.es9Client.Search.WithContext(ctx),
		c.es9Client.Search.WithIndex(indexName),
		c.es9Client.Search.WithBody(strings.NewReader(string(queryBytes))),
	)
	if err != nil {
		return nil, errors.Wrap(err, "search ES9")
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ES9 search error: %s", res.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, errors.Wrap(err, "decode ES9 response")
	}

	hits, ok := result["hits"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid ES9 response: no hits")
	}

	hitsArray, ok := hits["hits"].([]interface{})
	if !ok || len(hitsArray) == 0 {
		return nil, fmt.Errorf("document not found in ES9: %s", uid)
	}

	firstHit, ok := hitsArray[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid hit format")
	}

	source, ok := firstHit["_source"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no _source in hit")
	}

	return source, nil
}

// Compare performs field-by-field comparison
func (c *TagsComparator) Compare(es6Doc, es9Doc map[string]interface{}) *ComparisonResult {
	return CompareDocuments(es6Doc, es9Doc, c.GetCriticalFields(), c.GetIgnoredFields())
}

// GetCriticalFields returns fields that must match exactly
func (c *TagsComparator) GetCriticalFields() []string {
	return []string{
		"mdb_uid",
		"result_type",
		"title",
		"full_title",
	}
}

// GetIgnoredFields returns fields that are expected to differ
func (c *TagsComparator) GetIgnoredFields() []string {
	return []string{
		"index_date",
		"_id",
		"_index",
		"_type",
		"_score",
	}
}

package search

import (
	"bytes"
	"context"
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esapi"
	"github.com/pkg/errors"

	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es9"
	es9common "github.com/Bnei-Baruch/archive-backend/es9/common"
)

// DeleteGrammarIndexES9 deletes the grammar indices for all languages on ES9.
func DeleteGrammarIndexES9(mgr *es9common.ES9Manager, indexDate string) error {
	ctx := context.TODO()
	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := GrammarIndexName(lang, indexDate)
		exists, err := mgr.IndexExists(ctx, name)
		if err != nil {
			return errors.Wrapf(err, "IndexExists %s", name)
		}
		if exists {
			if err := mgr.DeleteIndex(ctx, name); err != nil {
				return errors.Wrapf(err, "DeleteIndex %s", name)
			}
		}
	}
	return nil
}

// CreateGrammarIndexES9 creates the grammar indices for all languages on ES9
// using the standalone ES9 grammar mappings (es9.GrammarMapping). Existing
// indices are left untouched.
func CreateGrammarIndexES9(mgr *es9common.ES9Manager, indexDate string) error {
	ctx := context.TODO()
	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := GrammarIndexName(lang, indexDate)
		exists, err := mgr.IndexExists(ctx, name)
		if err != nil {
			return errors.Wrapf(err, "IndexExists %s", name)
		}
		if exists {
			log.Debugf("ES9 grammar index already exists (%s), skipping.", name)
			continue
		}
		mapping, err := es9.GrammarMapping(lang)
		if err != nil {
			return errors.Wrapf(err, "GrammarMapping %s", lang)
		}
		if err := mgr.CreateIndex(ctx, name, mapping); err != nil {
			return errors.Wrapf(err, "CreateIndex %s", name)
		}
		log.Debugf("Created ES9 grammar index: %s", name)
	}
	return nil
}

// IndexGrammarsES9 (re)builds the ES9 grammar indices for all languages. It
// mirrors IndexGrammars but writes to ES9: docs are built by the shared
// buildGrammarDocsForLang so the ES6 and ES9 grammar indices are identical.
func IndexGrammarsES9(mgr *es9common.ES9Manager, indexDate string, grammars GrammarsV2, variables VariablesV2, cm cache.CacheManager) error {
	if err := DeleteGrammarIndexES9(mgr, indexDate); err != nil {
		return err
	}
	if err := CreateGrammarIndexES9(mgr, indexDate); err != nil {
		return err
	}

	client, err := mgr.GetClient()
	if err != nil {
		return errors.Wrap(err, "GetClient")
	}

	log.Infof("Indexing %d grammars to ES9.", len(grammars))
	for lang, grammarsByIntent := range grammars {
		name := GrammarIndexName(lang, indexDate)
		docs, err := buildGrammarDocsForLang(lang, grammarsByIntent, variables, cm)
		if err != nil {
			return err
		}
		if err := es9BulkIndexGrammarDocs(client, name, docs); err != nil {
			return errors.Wrapf(err, "bulk index %s", name)
		}
		log.Infof("Indexed %d grammar docs to %s.", len(docs), name)
	}
	return nil
}

// es9BulkIndexGrammarDocs bulk-indexes grammar docs into a single ES9 index in
// one request (matching the ES6 indexer's one-bulk-per-language behaviour). No
// explicit _id is set, so ES auto-generates ids as ES6 did.
func es9BulkIndexGrammarDocs(client *elasticsearch.Client, name string, docs []GrammarRuleWithPercolatorQuery) error {
	if len(docs) == 0 {
		return nil
	}
	action := map[string]interface{}{"index": map[string]interface{}{"_index": name}}
	var buf bytes.Buffer
	for i := range docs {
		if err := json.NewEncoder(&buf).Encode(action); err != nil {
			return errors.Wrap(err, "encode action")
		}
		if err := json.NewEncoder(&buf).Encode(docs[i]); err != nil {
			return errors.Wrap(err, "encode doc")
		}
	}

	req := esapi.BulkRequest{Body: bytes.NewReader(buf.Bytes())}
	res, err := req.Do(context.TODO(), client)
	if err != nil {
		return errors.Wrap(err, "bulk request")
	}
	defer res.Body.Close()
	if res.IsError() {
		return errors.Errorf("bulk request failed: %s", res.Status())
	}

	var bulkRes struct {
		Errors bool `json:"errors"`
		Items  []struct {
			Index struct {
				Status int         `json:"status"`
				Error  interface{} `json:"error"`
			} `json:"index"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&bulkRes); err != nil {
		return errors.Wrap(err, "decode bulk response")
	}
	if bulkRes.Errors {
		fails := 0
		for _, it := range bulkRes.Items {
			if it.Index.Status < 200 || it.Index.Status >= 300 {
				fails++
				if fails <= 3 {
					log.Errorf("ES9 grammar bulk item failed (status %d): %v", it.Index.Status, it.Index.Error)
				}
			}
		}
		return errors.Errorf("%d/%d grammar docs failed to index into %s", fails, len(docs), name)
	}
	return nil
}

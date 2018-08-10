package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/bindata"
	"github.com/Bnei-Baruch/archive-backend/consts"
)

type Scope struct {
	ContentUnitUID string
	FileUID        string
	CollectionUID  string
	TagUID         string
	SourceUID      string
	PersonUID      string
	PublisherUID   string
	TweetTID       string
	BlogPostWPID   string
}

type Index interface {
	ReindexAll() error
	Update(scope Scope) error
	CreateIndex() error
	DeleteIndex() error
	RefreshIndex() error
	ResultType() string
}

type BaseIndex struct {
	resultType string
	namespace  string
	baseName   string
	indexDate  string
	db         *sql.DB
	esc        *elastic.Client
}

func IndexAliasName(namespace string, name string, lang string) string {
	if namespace == "" || name == "" || lang == "" {
		panic(fmt.Sprintf("Not expecting empty parameter for IndexName, provided: (%s, %s, %s)", namespace, name, lang))
	}
	return fmt.Sprintf("%s_%s_%s", namespace, name, lang)
}

func IndexName(namespace string, name string, lang string, date string) string {
	if date == "" {
		panic(fmt.Sprintf("Not expecting empty parameter for IndexName, provided: (%s, %s, %s, %s)", namespace, name, lang, date))
	}
	return fmt.Sprintf("%s_%s", IndexAliasName(namespace, name, lang), date)
}

func (index *BaseIndex) ResultType() string {
	return index.resultType
}

func (index *BaseIndex) indexName(lang string) string {
	if index.namespace == "" || index.baseName == "" || index.indexDate == "" {
		panic("Index namespace, baseName and indexDate should be set.")
	}
	return IndexName(index.namespace, index.baseName, lang, index.indexDate)
}

func (index *BaseIndex) indexAliasName(lang string) string {
	if index.namespace == "" || index.baseName == "" {
		panic("Index namespace and baseName should be set.")
	}
	return IndexAliasName(index.namespace, index.baseName, lang)
}

func (index *BaseIndex) CreateIndex() error {
	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := index.indexName(lang)
		// Do nothing if index already exists.
		exists, err := index.esc.IndexExists(name).Do(context.TODO())
		log.Debugf("Create index, exists: %t.", exists)
		if err != nil {
			return err
		}
		if exists {
			log.Debugf("Index already exists (%+v), skipping.", name)
			continue
		}

		definition := fmt.Sprintf("data/es/mappings/%s/%s-%s.json", index.baseName, index.baseName, lang)
		// Read mappings and create index
		mappings, err := bindata.Asset(definition)
		if err != nil {
			return errors.Wrapf(err, "Failed loading mapping %s", definition)
		}
		var bodyJson map[string]interface{}
		if err = json.Unmarshal(mappings, &bodyJson); err != nil {
			return errors.Wrap(err, "json.Unmarshal")
		}

		// Create index.
		res, err := index.esc.CreateIndex(name).BodyJson(bodyJson).Do(context.TODO())
		if err != nil {
			return errors.Wrap(err, "Create index")
		}
		if !res.Acknowledged {
			return errors.Errorf("Index creation wasn't acknowledged: %s", name)
		}
		log.Debugf("Created index: %+v", name)
	}
	return nil
}

func (index *BaseIndex) DeleteIndex() error {
	for _, lang := range consts.ALL_KNOWN_LANGS {
		if err := index.deleteIndexByLang(lang); err != nil {
			return err
		}
	}
	return nil
}

func (index *BaseIndex) deleteIndexByLang(lang string) error {
	i18nName := index.indexName(lang)
	exists, err := index.esc.IndexExists(i18nName).Do(context.TODO())
	if err != nil {
		return err
	}
	if exists {
		res, err := index.esc.DeleteIndex(i18nName).Do(context.TODO())
		if err != nil {
			return errors.Wrap(err, "Delete index")
		}
		if !res.Acknowledged {
			return errors.Errorf("Index deletion wasn't acknowledged: %s", i18nName)
		}
	}
	return nil
}

func (index *BaseIndex) RefreshIndex() error {
	for _, lang := range consts.ALL_KNOWN_LANGS {
		if err := index.RefreshIndexByLang(lang); err != nil {
			return err
		}
	}
	return nil
}

func (index *BaseIndex) RefreshIndexByLang(lang string) error {
	_, err := index.esc.Refresh(index.indexName(lang)).Do(context.TODO())
	// fmt.Printf("\n\n\nShards: %+v \n\n\n", shards)
	return err
}

func (index *BaseIndex) FilterByResultTypeQuery(resultType string) *elastic.BoolQuery {
	return elastic.NewBoolQuery().Filter(elastic.NewTermsQuery(consts.ES_RESULT_TYPE, resultType))
}

func (index *BaseIndex) RemoveFromIndexQuery(elasticScope elastic.Query) ([]string, error) {
	source, err := elasticScope.Source()
	if err != nil {
		return []string{}, err
	}
	jsonBytes, err := json.Marshal(source)
	if err != nil {
		return []string{}, err
	}
	log.Infof("Results Index - Removing from index. Scope: %s", string(jsonBytes))
	removed := make(map[string]bool)
	for _, lang := range consts.ALL_KNOWN_LANGS {
		indexName := index.indexName(lang)
		searchRes, err := index.esc.Search(indexName).Query(elasticScope).Do(context.TODO())
		if err != nil {
			return []string{}, err
		}
		for _, h := range searchRes.Hits.Hits {
			var cu ContentUnit
			err := json.Unmarshal(*h.Source, &cu)
			if err != nil {
				return []string{}, err
			}
			removed[cu.MDB_UID] = true
		}
		delRes, err := index.esc.DeleteByQuery(indexName).
			Query(elasticScope).
			Do(context.TODO())
		if err != nil {
			return []string{}, errors.Wrapf(err, "Results Index - Remove from index %s %+v\n", indexName, elasticScope)
		}
		if delRes.Deleted > 0 {
			fmt.Printf("Results Index - Deleted %d documents from %s.\n", delRes.Deleted, indexName)
		}
	}
	if len(removed) == 0 {
		fmt.Println("Results Index - Nothing was delete.")
		return []string{}, nil
	}
	keys := make([]string, 0)
	for k := range removed {
		keys = append(keys, k)
	}
	return keys, nil
}

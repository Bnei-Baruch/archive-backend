package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"

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
}

type Index interface {
	ReindexAll() error
	Add(scope Scope) error
	Update(scope Scope) error
	Delete(scope Scope) error
	CreateIndex() error
	DeleteIndex() error
	RefreshIndex() error
}

type BaseIndex struct {
	namespace string
	baseName  string
	db        *sql.DB
	esc       *elastic.Client
}

func IndexName(namespace string, name string, lang string) string {
	return fmt.Sprintf("%s_%s_%s", namespace, name, lang)
}

func (index *BaseIndex) indexName(lang string) string {
	if index.namespace == "" || index.baseName == "" {
		panic("Index namespace and baseName should be set.")
	}
	return IndexName(index.namespace, index.baseName, lang)
}

func (index *BaseIndex) CreateIndex() error {
	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := index.indexName(lang)
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

		// Delete index if it's already exists.
		if err = index.deleteIndexByLang(lang); err != nil {
			return err
		}

		// Create index.
		res, err := index.esc.CreateIndex(name).BodyJson(bodyJson).Do(context.TODO())
		if err != nil {
			return errors.Wrap(err, "Create index")
		}
		if !res.Acknowledged {
			return errors.Errorf("Index creation wasn't acknowledged: %s", name)
		}
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

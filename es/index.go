package es

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/Bnei-Baruch/archive-backend/bindata"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
)

type Scope struct {
	ContentUnitUID string
}

type Index interface {
	ReindexAll() error
	AddToIndex(scope Scope) error
	RemoveFromIndex(scope Scope) error
	CreateIndex() error
	DeleteIndex() error
	RefreshIndex() error
}

type BaseIndex struct {
	namespace string
	baseName  string
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
		definition := fmt.Sprintf("data/es/mappings/units/units-%s.json", lang)
		// read mappings and create index
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
		res, err := mdb.ESC.CreateIndex(name).BodyJson(bodyJson).Do(context.TODO())
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
	exists, err := mdb.ESC.IndexExists(i18nName).Do(context.TODO())
	if err != nil {
		return err
	}
	if exists {
		res, err := mdb.ESC.DeleteIndex(i18nName).Do(context.TODO())
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
	_, err := mdb.ESC.Refresh(index.indexName(lang)).Do(context.TODO())
	return err
}

package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

func MakeCollectionsIndex(namespace string, db *sql.DB, esc *elastic.Client) *CollectionsIndex {
	ci := new(CollectionsIndex)
	ci.baseName = consts.ES_COLLECTIONS_INDEX
	ci.namespace = namespace
	ci.db = db
	ci.esc = esc
	return ci
}

type CollectionsIndex struct {
	BaseIndex
	indexData *IndexData
}

func defaultCollectionsSql() string {
	return fmt.Sprintf("c.secure = 0 AND c.published IS TRUE AND c.type_id NOT IN (%d, %d, %d, %d, %d, %d)",
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_DAILY_LESSON].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SPECIAL_LESSON].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_CLIPS].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSONS_SERIES].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SONGS].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BOOKS].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_UNKNOWN].ID,
	)
}

func (index *CollectionsIndex) ReindexAll() error {
	log.Infof("Collections Index - Reindex all.")
	if _, err := index.removeFromIndexQuery(elastic.NewMatchAllQuery()); err != nil {
		return err
	}
	return index.addToIndexSql(defaultCollectionsSql())
}

func (index *CollectionsIndex) Update(scope Scope) error {
	log.Infof("Collections Index - Update. Scope: %+v.", scope)
	removed, err := index.removeFromIndex(scope)
	if err != nil {
		return err
	}
	return index.addToIndex(scope, removed)
}

func (index *CollectionsIndex) addToIndex(scope Scope, removedUIDs []string) error {
	sqlScope := defaultCollectionsSql()
	uids := removedUIDs
	if scope.CollectionUID != "" {
		uids = append(uids, scope.CollectionUID)
	}
	if scope.ContentUnitUID != "" {
		moreUIDs, err := CollectionsScopeByContentUnit(index.db, scope.ContentUnitUID)
		if err != nil {
			return errors.Wrap(err, "collections index addToIndex collectionsScopeByContentUnit")
		}
		uids = append(uids, moreUIDs...)
	}
	if len(uids) == 0 {
		return nil
	}
	quoted := make([]string, len(uids))
	for i, uid := range uids {
		quoted[i] = fmt.Sprintf("'%s'", uid)
	}
	sqlScope = fmt.Sprintf("%s AND c.uid IN (%s)", sqlScope, strings.Join(quoted, ","))
	if err := index.addToIndexSql(sqlScope); err != nil {
		return errors.Wrap(err, "collections index addToIndex addToIndexSql")
	}
	return nil
}

func (index *CollectionsIndex) removeFromIndex(scope Scope) ([]string, error) {
	typedUIDs := make([]string, 0)
	if scope.CollectionUID != "" {
		typedUIDs = append(typedUIDs, uidToTypedUID("collection", scope.CollectionUID))
	}
	if scope.FileUID != "" {
		typedUIDs = append(typedUIDs, uidToTypedUID("file", scope.FileUID))
	}
	if scope.ContentUnitUID != "" {
		typedUIDs = append(typedUIDs, uidToTypedUID("content_unit", scope.ContentUnitUID))
		moreUIDs, err := CollectionsScopeByContentUnit(index.db, scope.ContentUnitUID)
		if err != nil {
			return []string{}, err
		}
		typedUIDs = append(typedUIDs, uidsToTypedUIDs("content_unit", moreUIDs)...)
	}
	if scope.TagUID != "" {
		typedUIDs = append(typedUIDs, uidToTypedUID("tag", scope.TagUID))
	}
	if scope.SourceUID != "" {
		typedUIDs = append(typedUIDs, uidToTypedUID("source", scope.SourceUID))
	}
	if len(typedUIDs) > 0 {
		typedUIDsI := make([]interface{}, len(typedUIDs))
		for i, typedUID := range typedUIDs {
			typedUIDsI[i] = typedUID
		}
		elasticScope := elastic.NewTermsQuery("typed_uids", typedUIDsI...)
		return index.removeFromIndexQuery(elasticScope)
	} else {
		// Nothing to remove.
		return []string{}, nil
	}
}

func (index *CollectionsIndex) addToIndexSql(sqlScope string) error {
	var count int64
	if err := mdbmodels.NewQuery(index.db,
		qm.Select("count(*)"),
		qm.From("collections as c"),
		qm.Where(sqlScope)).QueryRow().Scan(&count); err != nil {
		return err
	}
	log.Infof("Collections Index - Adding %d collections. Scope: %s.", count, sqlScope)
	offset := 0
	limit := 10
	for offset < int(count) {
		var collections []*mdbmodels.Collection
		err := mdbmodels.NewQuery(index.db,
			qm.From("collections as c"),
			qm.Load("CollectionI18ns"),
			qm.Load("CollectionsContentUnits"),
			qm.Load("CollectionsContentUnits.ContentUnit"),
			// qm.Load("CollectionsContentUnits.ContentUnit.ContentUnitI18ns"),
			qm.Where(sqlScope),
			qm.Offset(offset),
			qm.Limit(limit)).Bind(&collections)
		if err != nil {
			return errors.Wrap(err, "Fetch collections from mdb.")
		}
		log.Infof("Adding %d collections (offset %d).", len(collections), offset)

		cuUIDs := make([]string, 0)
		for _, c := range collections {
			for _, ccu := range c.R.CollectionsContentUnits {
				cuUIDs = append(cuUIDs, fmt.Sprintf("'%s'", ccu.R.ContentUnit.UID))
			}
		}
		contentUnitsSqlScope := defaultContentUnitSql()
		if len(cuUIDs) > 0 {
			contentUnitsSqlScope = fmt.Sprintf(
				"%s AND cu.uid in (%s)", contentUnitsSqlScope, strings.Join(cuUIDs, ","))
		}

		index.indexData, err = MakeIndexData(index.db, contentUnitsSqlScope)
		if err != nil {
			return err
		}
		for _, collection := range collections {
			if err := index.indexCollection(collection); err != nil {
				return err
			}
		}
		offset += limit
	}
	return nil
}

func (index *CollectionsIndex) removeFromIndexQuery(elasticScope elastic.Query) ([]string, error) {
	source, err := elasticScope.Source()
	if err != nil {
		return []string{}, err
	}
	jsonBytes, err := json.Marshal(source)
	if err != nil {
		return []string{}, err
	}
	log.Infof("Collections Index - Removing from index. Scope: %s", string(jsonBytes))
	removed := make(map[string]bool)
	for _, lang := range consts.ALL_KNOWN_LANGS {
		indexName := index.indexName(lang)
		searchRes, err := index.esc.Search(indexName).Query(elasticScope).Do(context.TODO())
		if err != nil {
			return []string{}, err
		}
		for _, h := range searchRes.Hits.Hits {
			var c Collection
			err := json.Unmarshal(*h.Source, &c)
			if err != nil {
				return []string{}, err
			}
			removed[c.MDB_UID] = true
		}
		delRes, err := index.esc.DeleteByQuery(indexName).
			Query(elasticScope).
			Do(context.TODO())
		if err != nil {
			return []string{}, errors.Wrapf(err, "Remove from index %s %+v\n", indexName, elasticScope)
		}
		if delRes.Deleted > 0 {
			log.Infof("Deleted %d documents from %s.\n", delRes.Deleted, indexName)
		}
		if delRes.Deleted != int64(len(searchRes.Hits.Hits)) {
			return []string{}, errors.New(fmt.Sprintf("Expected to remove %d documents, removed only %d",
				len(searchRes.Hits.Hits), delRes.Deleted))
		}
	}
	if len(removed) == 0 {
		log.Info("Collections Index - Nothing was delete.")
		return []string{}, nil
	}
	keys := make([]string, 0)
	for k := range removed {
		keys = append(keys, k)
	}
	return keys, nil
}

func contentUnitsContentTypes(collectionsContentUnits mdbmodels.CollectionsContentUnitSlice) []string {
	m := make(map[string]bool)
	for _, ccu := range collectionsContentUnits {
		if defaultContentUnit(ccu.R.ContentUnit) {
			m[mdb.CONTENT_TYPE_REGISTRY.ByID[ccu.R.ContentUnit.TypeID].Name] = true
		}
	}
	keys := make([]string, 0)
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func contentUnitsTypedUIDs(collectionsContentUnits mdbmodels.CollectionsContentUnitSlice) []string {
	ret := make([]string, len(collectionsContentUnits))
	for i, ccu := range collectionsContentUnits {
		ret[i] = uidToTypedUID("content_unit", ccu.R.ContentUnit.UID)
	}
	return ret
}

func (index *CollectionsIndex) indexCollection(c *mdbmodels.Collection) error {
	// Create documents in each language with available translation
	i18nMap := make(map[string]Collection)
	for _, i18n := range c.R.CollectionI18ns {
		if i18n.Name.Valid && i18n.Name.String != "" {
			typedUIDs := append([]string{uidToTypedUID("collection", c.UID)},
				contentUnitsTypedUIDs(c.R.CollectionsContentUnits)...)
			collection := Collection{
				MDB_UID:                  c.UID,
				TypedUIDs:                typedUIDs,
				Name:                     i18n.Name.String,
				ContentType:              mdb.CONTENT_TYPE_REGISTRY.ByID[c.TypeID].Name,
				ContentUnitsContentTypes: contentUnitsContentTypes(c.R.CollectionsContentUnits),
			}

			if i18n.Description.Valid && i18n.Description.String != "" {
				collection.Description = i18n.Description.String
			}

			if c.Properties.Valid {
				var props map[string]interface{}
				err := json.Unmarshal(c.Properties.JSON, &props)
				if err != nil {
					return errors.Wrapf(err, "json.Unmarshal properties %s", c.UID)
				}

				if startDate, ok := props["start_date"]; ok {
					val, err := time.Parse("2006-01-02", startDate.(string))
					if err != nil {
						val, err = time.Parse("2006-01-02T15:04:05Z", startDate.(string))
						if err != nil {
							return errors.Wrapf(err, "time.Parse start_date %s", c.UID)
						}
					}
					collection.EffectiveDate = &utils.Date{Time: val}
				}

				if originalLanguage, ok := props["original_language"]; ok {
					collection.OriginalLanguage = originalLanguage.(string)
				}
			}

			i18nMap[i18n.Language] = collection
		}
	}

	// Index each document in its language index
	for k, v := range i18nMap {
		name := index.indexName(k)
		vBytes, err := json.Marshal(v)
		if err != nil {
			return err
		}
		log.Infof("Collections Index - Add collection %s to index %s", string(vBytes), name)
		resp, err := index.esc.Index().
			Index(name).
			Type("collections").
			BodyJson(v).
			Do(context.TODO())
		if err != nil {
			return errors.Wrapf(err, "Index collection %s %s", name, c.UID)
		}
		if !resp.Created {
			return errors.Errorf("Not created: collection %s %s", name, c.UID)
		}
	}

	return nil
}

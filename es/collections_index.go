package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

func MakeCollectionsIndex(namespace string, indexDate string, db *sql.DB, esc *elastic.Client) *CollectionsIndex {
	ci := new(CollectionsIndex)
	ci.resultType = consts.ES_RESULT_TYPE_COLLECTIONS
	ci.baseName = consts.ES_RESULTS_INDEX
	ci.namespace = namespace
	ci.db = db
	ci.esc = esc
	ci.indexDate = indexDate
	return ci
}

type CollectionsIndex struct {
	BaseIndex
}

func defaultCollectionsSql() string {
	return fmt.Sprintf("c.secure = 0 AND c.published IS TRUE AND c.type_id NOT IN (%d, %d, %d, %d, %d, %d, %d)",
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
	log.Info("Collections Index - Reindex all.")
	if _, err := index.RemoveFromIndexQuery(index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_COLLECTIONS)); err != nil {
		return err
	}
	return index.addToIndexSql(defaultCollectionsSql())
}

func (index *CollectionsIndex) Update(scope Scope) error {
	log.Debugf("Collections Index - Update. Scope: %+v.", scope)
	removed, err := index.removeFromIndex(scope)
	// We want to run addToIndex anyway and return joint error.
	return utils.JoinErrors(err, index.addToIndex(scope, removed))
}

func (index *CollectionsIndex) addToIndex(scope Scope, removedUIDs []string) error {
	sqlScope := defaultCollectionsSql()
	uids := removedUIDs
	if scope.CollectionUID != "" {
		uids = append(uids, scope.CollectionUID)
	}
	err := (error)(nil)
	if scope.ContentUnitUID != "" {
		moreUIDs, e := CollectionsScopeByContentUnit(index.db, scope.ContentUnitUID)
		if e != nil {
			err = utils.JoinErrors(err, errors.Wrap(e, "collections index addToIndex collectionsScopeByContentUnit"))
		}
		uids = append(uids, moreUIDs...)
	}
	if len(uids) == 0 {
		return err
	}
	quoted := make([]string, len(uids))
	for i, uid := range uids {
		quoted[i] = fmt.Sprintf("'%s'", uid)
	}
	sqlScope = fmt.Sprintf("%s AND c.uid IN (%s)", sqlScope, strings.Join(quoted, ","))
	if e := index.addToIndexSql(sqlScope); e != nil {
		err = utils.JoinErrors(err, errors.Wrap(e, "collections index addToIndex addToIndexSql"))
	}
	return err
}

func (index *CollectionsIndex) removeFromIndex(scope Scope) ([]string, error) {
	typedUIDs := make([]string, 0)
	if scope.CollectionUID != "" {
		typedUIDs = append(typedUIDs, keyValue("collection", scope.CollectionUID))
	}
	if scope.FileUID != "" {
		typedUIDs = append(typedUIDs, keyValue("file", scope.FileUID))
	}
	err := (error)(nil)
	if scope.ContentUnitUID != "" {
		typedUIDs = append(typedUIDs, keyValue("content_unit", scope.ContentUnitUID))
		moreUIDs, e := CollectionsScopeByContentUnit(index.db, scope.ContentUnitUID)
		err = utils.JoinErrors(err, e)
		typedUIDs = append(typedUIDs, KeyValues("content_unit", moreUIDs)...)
	}
	if scope.TagUID != "" {
		typedUIDs = append(typedUIDs, keyValue("tag", scope.TagUID))
	}
	if scope.SourceUID != "" {
		typedUIDs = append(typedUIDs, keyValue("source", scope.SourceUID))
	}
	if len(typedUIDs) > 0 {
		typedUIDsI := make([]interface{}, len(typedUIDs))
		for i, typedUID := range typedUIDs {
			typedUIDsI[i] = typedUID
		}
		elasticScope := elastic.NewTermsQuery("typed_uids", typedUIDsI...)
		uids, e := index.RemoveFromIndexQuery(elasticScope)
		return uids, utils.JoinErrors(err, e)
	} else {
		// Nothing to remove.
		return []string{}, err
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
	err := (error)(nil)
	for offset < int(count) {
		var collections []*mdbmodels.Collection
		e := mdbmodels.NewQuery(index.db,
			qm.From("collections as c"),
			qm.Load("CollectionI18ns"),
			qm.Load("CollectionsContentUnits"),
			qm.Load("CollectionsContentUnits.ContentUnit"),
			qm.Where(sqlScope),
			qm.Offset(offset),
			qm.Limit(limit)).Bind(&collections)
		if e != nil {
			err = utils.JoinErrors(err, errors.Wrap(err, "Fetch collections from mdb."))
			continue
		}
		log.Debugf("Adding %d collections (offset %d).", len(collections), offset)

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

		for _, collection := range collections {
			err = utils.JoinErrors(err, index.indexCollection(collection))
		}
		offset += limit
	}
	return err
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
		ret[i] = keyValue("content_unit", ccu.R.ContentUnit.UID)
	}
	return ret
}

func (index *CollectionsIndex) indexCollection(c *mdbmodels.Collection) error {
	// Create documents in each language with available translation
	i18nMap := make(map[string]Result)
	for _, i18n := range c.R.CollectionI18ns {
		if i18n.Name.Valid && i18n.Name.String != "" {
			typedUIDs := append([]string{keyValue("collection", c.UID)},
				contentUnitsTypedUIDs(c.R.CollectionsContentUnits)...)
			filterValues := append([]string{keyValue("content_type", mdb.CONTENT_TYPE_REGISTRY.ByID[c.TypeID].Name)},
				KeyValues("collections_content_type", contentUnitsContentTypes(c.R.CollectionsContentUnits))...)
			collection := Result{
				ResultType:   consts.ES_RESULT_TYPE_COLLECTIONS,
				MDB_UID:      c.UID,
				TypedUids:    typedUIDs,
				FilterValues: filterValues,
				Title:        i18n.Name.String,
				TitleSuggest: Suffixes(i18n.Name.String),
			}

			if i18n.Description.Valid && i18n.Description.String != "" {
				collection.Description = i18n.Description.String
			}

			// if c.Properties.Valid {
			// 	var props map[string]interface{}
			// 	err := json.Unmarshal(c.Properties.JSON, &props)
			// 	if err != nil {
			// 		return errors.Wrapf(err, "json.Unmarshal properties %s", c.UID)
			// 	}

			// 	if startDate, ok := props["start_date"]; ok {
			// 		val, err := time.Parse("2006-01-02", startDate.(string))
			// 		if err != nil {
			// 			val, err = time.Parse("2006-01-02T15:04:05Z", startDate.(string))
			// 			if err != nil {
			// 				return errors.Wrapf(err, "time.Parse start_date %s", c.UID)
			// 			}
			// 		}
			// 		collection.EffectiveDate = &utils.Date{Time: val}
			// 	}

			// 	// No use for OriginalLanguage
			// 	/*if originalLanguage, ok := props["original_language"]; ok {
			// 		collection.OriginalLanguage = originalLanguage.(string)
			// 	}*/
			// }

			i18nMap[i18n.Language] = collection
		}
	}

	// Index each document in its language index
	err := (error)(nil)
	for k, v := range i18nMap {
		name := index.indexName(k)
		vBytes, e := json.Marshal(v)
		if e != nil {
			err = utils.JoinErrors(err, e)
			continue
		}
		log.Debugf("Collections Index - Add collection %s to index %s", string(vBytes), name)
		resp, e := index.esc.Index().
			Index(name).
			Type("result").
			BodyJson(v).
			Do(context.TODO())
		if e != nil {
			utils.JoinErrors(err, errors.Wrapf(err, "Index collection %s %s", name, c.UID))
			continue
		}
		if resp.Result != "created" {
			utils.JoinErrors(err, errors.Errorf("Not created: collection %s %s", name, c.UID))
			continue
		}
	}

	return nil
}

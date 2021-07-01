package es

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

func MakeLikutimIndex(namespace string, indexDate string, db *sql.DB, esc *elastic.Client) *LikutimIndex {
	li := new(LikutimIndex)
	li.resultType = consts.ES_RESULT_TYPE_LIKUTIM
	li.baseName = consts.ES_RESULTS_INDEX
	li.namespace = namespace
	li.indexDate = indexDate
	li.db = db
	li.esc = esc
	return li
}

type LikutimIndex struct {
	BaseIndex
	Progress uint64
}

func (index *LikutimIndex) ReindexAll() error {
	log.Info("LikutimIndex.Reindex All.")
	_, indexErrors := index.RemoveFromIndexQuery(index.FilterByResultTypeQuery(index.resultType))
	if err := indexErrors.CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "LikutimIndex"); err != nil {
		return err
	}
	// SQL to always match any likutim
	sqlScope := fmt.Sprintf("cu.secure = 0 AND cu.published IS TRUE AND cu.type_id = %d", mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LIKUTIM].ID)
	return indexErrors.Join(index.addToIndexSql(sqlScope), "").CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "LikutimIndex")
}

func (index *LikutimIndex) RemoveFromIndex(scope Scope) (map[string][]string, error) {
	log.Debugf("LikutimIndex.Update - Scope: %+v.", scope)
	removed, indexErrors := index.removeFromIndex(scope)
	return removed, indexErrors.CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "LikutimIndex")
}

func (index *LikutimIndex) AddToIndex(scope Scope, removedUIDs []string) error {
	log.Debugf("LikutimIndex.AddToIndex - Scope: %+v, removedUIDs: %+v.", scope, removedUIDs)
	return index.addToIndex(scope, removedUIDs).CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "LikutimIndex")
}

func (index *LikutimIndex) addToIndex(scope Scope, removedUIDs []string) *IndexErrors {
	uids := removedUIDs
	if scope.ContentUnitUID != "" {
		uids = append(uids, scope.ContentUnitUID)
	}
	indexErrors := MakeIndexErrors()
	if scope.FileUID != "" {
		moreUIDs, err := contentUnitsScopeByFile(index.db, scope.FileUID)
		indexErrors.SetError(err)
		uids = append(uids, moreUIDs...)
	}
	if scope.TagUID != "" {
		moreUIDs, err := contentUnitsScopeByTag(index.db, scope.TagUID)
		indexErrors.SetError(err)
		uids = append(uids, moreUIDs...)
	}
	if len(uids) == 0 {
		return indexErrors
	}
	quoted := make([]string, len(uids))
	for i, uid := range uids {
		quoted[i] = fmt.Sprintf("'%s'", uid)
	}
	sqlScope := fmt.Sprintf("cu.secure = 0 AND cu.published IS TRUE AND cu.type_id = %d", mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LIKUTIM].ID)
	return indexErrors.Join(index.addToIndexSql(sqlScope), fmt.Sprintf("Failed adding to index: %+v", sqlScope))
}

func (index *LikutimIndex) addToIndexSql(sqlScope string) *IndexErrors {
	indexErrors := MakeIndexErrors()
	var count int
	err := mdbmodels.NewQuery(index.db,
		qm.Select("COUNT(1)"),
		qm.From("content_units as cu"),
		qm.Where(sqlScope)).QueryRow().Scan(&count)
	if err != nil {
		return indexErrors.SetError(errors.Wrapf(err, "Failed fetching content_units with sql scope: %s", sqlScope))
	}

	log.Debugf("Content Units Index - Adding %d units. Scope: %s", count, sqlScope)

	tasks := make(chan OffsetLimitJob, 300)
	errors := make(chan *IndexErrors, 300)
	doneAdding := make(chan bool)

	tasksCount := 0
	go func() {
		offset := 0
		limit := utils.MaxInt(10, utils.MinInt(250, (int)(count/10)))
		for offset < count {
			tasks <- OffsetLimitJob{offset, limit, count}
			tasksCount += 1
			offset += limit
		}
		close(tasks)
		doneAdding <- true
	}()

	for w := 1; w <= 10; w++ {
		go func(tasks <-chan OffsetLimitJob, errors chan<- *IndexErrors) {
			for task := range tasks {
				errors <- index.bulkIndexUnits(task, sqlScope)
			}
		}(tasks, errors)
	}

	<-doneAdding
	for a := 1; a <= tasksCount; a++ {
		indexErrors.Join(<-errors, "")
	}

	return indexErrors
}

func (index *LikutimIndex) removeFromIndex(scope Scope) (map[string][]string, *IndexErrors) {
	typedUids := make([]string, 0)
	if scope.ContentUnitUID != "" {
		typedUids = append(typedUids, KeyValue(consts.ES_UID_TYPE_CONTENT_UNIT, scope.ContentUnitUID))
	}
	indexErrors := MakeIndexErrors()
	if scope.FileUID != "" {
		typedUids = append(typedUids, KeyValue(consts.ES_UID_TYPE_FILE, scope.FileUID))
		moreUIDs, err := contentUnitsScopeByFile(index.db, scope.FileUID)
		indexErrors.SetError(err)
		typedUids = append(typedUids, KeyValues(consts.ES_UID_TYPE_CONTENT_UNIT, moreUIDs)...)
	}
	if scope.TagUID != "" {
		typedUids = append(typedUids, KeyValue(consts.ES_UID_TYPE_TAG, scope.TagUID))
	}
	if len(typedUids) > 0 {
		typedUidsI := make([]interface{}, len(typedUids))
		for i, typedUID := range typedUids {
			typedUidsI[i] = typedUID
		}
		elasticScope := index.FilterByResultTypeQuery(index.resultType).
			Filter(elastic.NewTermsQuery("typed_uids", typedUidsI...))
		uids, removeIndexErrors := index.RemoveFromIndexQuery(elasticScope)
		return uids, indexErrors.Join(removeIndexErrors, fmt.Sprintf("Failed removing from index: %+v", elasticScope))
	} else {
		// Nothing to remove, or error.
		return make(map[string][]string), indexErrors
	}
}

func (index *LikutimIndex) bulkIndexUnits(bulk OffsetLimitJob, sqlScope string) *IndexErrors {
	indexErrors := MakeIndexErrors()
	var units []*mdbmodels.ContentUnit
	if err := mdbmodels.NewQuery(index.db,
		qm.From("content_units as cu"),
		qm.Load("ContentUnitI18ns"),
		qm.Load("Tags"),
		qm.Where(sqlScope),
		qm.Offset(bulk.Offset),
		qm.Limit(bulk.Limit)).Bind(&units); err != nil {
		return indexErrors.SetError(errors.Wrap(err, "Fetch units from mdb"))
	}
	log.Infof("Content Units Index - Adding %d units (offset: %d total: %d).", len(units), bulk.Offset, bulk.Total)

	indexData, err := MakeIndexDataLikutim(index.db, sqlScope)
	if err != nil {
		return indexErrors.SetError(errors.Wrap(err, "Failed making index data."))
	}
	i18nMap := make(map[string][]Result)
	for _, unit := range units {
		i18nUnit, unitIndexErrors := index.prepareIndexUnit(unit, indexData)
		for lang, result := range i18nUnit {
			i18nMap[lang] = append(i18nMap[lang], result)
		}
		indexErrors.Join(unitIndexErrors, "")
	}

	// Index each document in its language index
	for lang, results := range i18nMap {
		indexName := index.IndexName(lang)

		bulkService := elastic.NewBulkService(index.esc).Index(indexName)
		for _, result := range results {
			indexErrors.ShouldIndex(lang)
			bulkService.Add(elastic.NewBulkIndexRequest().Index(indexName).Type("result").Doc(result))
		}
		bulkRes, e := bulkService.Do(context.TODO())
		if e != nil {
			indexErrors.LanguageError(lang, e, fmt.Sprintf("Results Index - bulkIndexUnits %s %+v.", indexName, sqlScope))
			continue
		}
		for _, itemMap := range bulkRes.Items {
			for _, res := range itemMap {
				if res.Result == "created" {
					indexErrors.Indexed(lang)
				}
			}
		}
	}
	indexErrors.PrintIndexCounts(fmt.Sprintf("ContentUnitIndex %d - %d / %d", bulk.Offset, bulk.Offset+bulk.Limit, bulk.Total))
	return indexErrors
}

func (index *LikutimIndex) prepareIndexUnit(cu *mdbmodels.ContentUnit, indexData *IndexData) (map[string]Result, *IndexErrors) {
	indexErrors := MakeIndexErrors()
	// Create documents in each language with available translation
	i18nMap := make(map[string]Result)
	for _, i18n := range cu.R.ContentUnitI18ns {
		if i18n.Name.Valid && strings.TrimSpace(i18n.Name.String) != "" {
			typedUids := []string{KeyValue(consts.ES_UID_TYPE_LIKUTIM, cu.UID)}

			unit := Result{
				ResultType: index.resultType,
				IndexDate:  &utils.Date{Time: time.Now()},
				MDB_UID:    cu.UID,
				TypedUids:  typedUids,
				Title:      i18n.Name.String,
			}

			if val, ok := indexData.Tags[cu.UID]; ok {
				unit.FilterValues = append(unit.FilterValues, KeyValues(consts.ES_UID_TYPE_TAG, val)...)
				unit.TypedUids = append(unit.TypedUids, KeyValues(consts.ES_UID_TYPE_TAG, val)...)
			}

			i18nMap[i18n.Language] = unit
		}
	}

	return i18nMap, indexErrors
}

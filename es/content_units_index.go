package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

func MakeContentUnitsIndex(namespace string, indexDate string, db *sql.DB, esc *elastic.Client) *ContentUnitsIndex {
	cui := new(ContentUnitsIndex)
	cui.resultType = consts.ES_RESULT_TYPE_UNITS
	cui.baseName = consts.ES_RESULTS_INDEX
	cui.namespace = namespace
	cui.indexDate = indexDate
	cui.db = db
	cui.esc = esc
	return cui
}

type ContentUnitsIndex struct {
	BaseIndex
	Progress uint64
}

func defaultContentUnit(cu *mdbmodels.ContentUnit) bool {
	return cu.Secure == 0 && cu.Published && !utils.Int64InSlice(cu.TypeID, []int64{
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_CLIP].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LELO_MIKUD].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_PUBLICATION].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SONG].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BOOK].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BLOG_POST].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_KITEI_MAKOR].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_UNKNOWN].ID,
	})
}

func defaultContentUnitSql() string {
	return fmt.Sprintf("cu.secure = 0 AND cu.published IS TRUE AND cu.type_id NOT IN (%d, %d, %d, %d, %d, %d, %d, %d)",
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_CLIP].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LELO_MIKUD].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_PUBLICATION].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SONG].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BOOK].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BLOG_POST].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_KITEI_MAKOR].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_UNKNOWN].ID,
	)
}

func (index *ContentUnitsIndex) ReindexAll() error {
	log.Infof("Content Units Index - Reindex all.")
	if _, err := index.RemoveFromIndexQuery(index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_UNITS)); err != nil {
		return err
	}
	return index.addToIndexSql(defaultContentUnitSql())
}

func (index *ContentUnitsIndex) Update(scope Scope) error {
	log.Infof("Content Units Index - Update. Scope: %+v.", scope)
	removed, err := index.removeFromIndex(scope)
	if err != nil {
		return err
	}
	return index.addToIndex(scope, removed)
}

func (index *ContentUnitsIndex) addToIndex(scope Scope, removedUIDs []string) error {
	// TODO: Missing tag scope handling.
	sqlScope := defaultContentUnitSql()
	uids := removedUIDs
	if scope.ContentUnitUID != "" {
		uids = append(uids, scope.ContentUnitUID)
	}
	if scope.FileUID != "" {
		moreUIDs, err := contentUnitsScopeByFile(index.db, scope.FileUID)
		if err != nil {
			return err
		}
		uids = append(uids, moreUIDs...)
	}
	if scope.CollectionUID != "" {
		moreUIDs, err := contentUnitsScopeByCollection(index.db, scope.CollectionUID)
		if err != nil {
			return err
		}
		uids = append(uids, moreUIDs...)
	}
	if scope.SourceUID != "" {
		moreUIDs, err := contentUnitsScopeBySource(index.db, scope.SourceUID)
		if err != nil {
			return err
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
	sqlScope = fmt.Sprintf("%s AND cu.uid IN (%s)", sqlScope, strings.Join(quoted, ","))
	return index.addToIndexSql(sqlScope)
}

func (index *ContentUnitsIndex) removeFromIndex(scope Scope) ([]string, error) {
	typedUids := make([]string, 0)
	if scope.ContentUnitUID != "" {
		typedUids = append(typedUids, keyValue("content_unit", scope.ContentUnitUID))
	}
	if scope.FileUID != "" {
		typedUids = append(typedUids, keyValue("file", scope.FileUID))
		moreUIDs, err := contentUnitsScopeByFile(index.db, scope.FileUID)
		if err != nil {
			return []string{}, err
		}
		typedUids = append(typedUids, KeyValues("content_unit", moreUIDs)...)
	}
	if scope.CollectionUID != "" {
		typedUids = append(typedUids, keyValue("collection", scope.CollectionUID))
		moreUIDs, err := contentUnitsScopeByCollection(index.db, scope.CollectionUID)
		if err != nil {
			return []string{}, err
		}
		typedUids = append(typedUids, KeyValues("content_unit", moreUIDs)...)
	}
	if scope.TagUID != "" {
		typedUids = append(typedUids, keyValue("tag", scope.TagUID))
	}
	if scope.SourceUID != "" {
		typedUids = append(typedUids, keyValue("source", scope.SourceUID))
		moreUIDs, err := contentUnitsScopeBySource(index.db, scope.SourceUID)
		if err != nil {
			return []string{}, err
		}
		typedUids = append(typedUids, KeyValues("content_unit", moreUIDs)...)
	}
	// if scope.PersonUID != "" {
	// 	typedUids = append(typedUids, keyValue("person", scope.PersonUID))
	// }
	// if scope.PublisherUID != "" {
	// 	typedUids = append(typedUids, keyValue("publisher", scope.PublisherUID))
	// }
	if len(typedUids) > 0 {
		typedUidsI := make([]interface{}, len(typedUids))
		for i, typedUID := range typedUids {
			typedUidsI[i] = typedUID
		}
		elasticScope := index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_UNITS).
			Filter(elastic.NewTermsQuery("typed_uids", typedUidsI...))
		return index.RemoveFromIndexQuery(elasticScope)
	} else {
		// Nothing to remove.
		return []string{}, nil
	}
}

func (index *ContentUnitsIndex) bulkIndexUnits(offset int, limit int, sqlScope string) error {
	var units []*mdbmodels.ContentUnit

	sqlScope = "cu.uid = 'CT5qya6m'"

	err := mdbmodels.NewQuery(index.db,
		qm.From("content_units as cu"),
		qm.Load("ContentUnitI18ns"),
		qm.Load("CollectionsContentUnits"),
		qm.Load("CollectionsContentUnits.Collection"),
		qm.Where(sqlScope),
		qm.Offset(offset),
		qm.Limit(limit)).Bind(&units)
	if err != nil {
		return errors.Wrap(err, "Fetch units from mdb")
	}
	log.Infof("Content Units Index - Adding %d units (offset: %d).", len(units), offset)

	indexData, err := MakeIndexData(index.db, sqlScope)
	if err != nil {
		return err
	}
	for _, unit := range units {
		if err := index.indexUnit(unit, indexData); err != nil {
			return err
		}
	}
	return nil
}

type OffsetLimitJob struct {
	Offset int
	Limit  int
}

func (index *ContentUnitsIndex) addToIndexSql(sqlScope string) error {
	var count int64
	err := mdbmodels.NewQuery(index.db,
		qm.Select("COUNT(1)"),
		qm.From("content_units as cu"),
		qm.Where(sqlScope)).QueryRow().Scan(&count)
	if err != nil {
		return err
	}

	log.Infof("Content Units Index - Adding %d units. Scope: %s", count, sqlScope)

	tasks := make(chan OffsetLimitJob, 300)
	errors := make(chan error, 300)
	doneAdding := make(chan bool)

	count = 1

	tasksCount := 0
	go func() {
		offset := 0
		// limit := 1000
		limit := 1
		for offset < int(count) {
			tasks <- OffsetLimitJob{offset, limit}
			tasksCount += 1
			offset += limit
		}
		close(tasks)
		doneAdding <- true
	}()

	for w := 1; w <= 10; w++ {
		go func(tasks <-chan OffsetLimitJob, errors chan<- error) {
			for task := range tasks {
				errors <- index.bulkIndexUnits(task.Offset, task.Limit, sqlScope)
			}
		}(tasks, errors)
	}

	<-doneAdding
	for a := 1; a <= tasksCount; a++ {
		e := <-errors
		if e != nil {
			return e
		}
	}

	return nil
}

func collectionsContentTypes(collectionsContentUnits mdbmodels.CollectionsContentUnitSlice) []string {
	ret := make([]string, len(collectionsContentUnits))
	for i, ccu := range collectionsContentUnits {
		ret[i] = mdb.CONTENT_TYPE_REGISTRY.ByID[ccu.R.Collection.TypeID].Name
	}
	return ret
}

func collectionsTypedUids(collectionsContentUnits mdbmodels.CollectionsContentUnitSlice) []string {
	ret := make([]string, len(collectionsContentUnits))
	for i, ccu := range collectionsContentUnits {
		ret[i] = keyValue("collection", ccu.R.Collection.UID)
	}
	return ret
}

func (index *ContentUnitsIndex) indexUnit(cu *mdbmodels.ContentUnit, indexData *IndexData) error {
	// Create documents in each language with available translation
	i18nMap := make(map[string]Result)
	for _, i18n := range cu.R.ContentUnitI18ns {
		if i18n.Name.Valid && strings.TrimSpace(i18n.Name.String) != "" {
			typedUids := append([]string{keyValue("content_unit", cu.UID)},
				collectionsTypedUids(cu.R.CollectionsContentUnits)...)
			filterValues := append([]string{keyValue("content_type", mdb.CONTENT_TYPE_REGISTRY.ByID[cu.TypeID].Name)},
				KeyValues("collections_content_type", collectionsContentTypes(cu.R.CollectionsContentUnits))...)

			unit := Result{
				ResultType:   consts.ES_RESULT_TYPE_UNITS,
				MDB_UID:      cu.UID,
				TypedUids:    typedUids,
				FilterValues: filterValues,
				Title:        i18n.Name.String,
				TitleSuggest: Suffixes(i18n.Name.String),
			}

			if i18n.Description.Valid && i18n.Description.String != "" {
				unit.Description = i18n.Description.String
			}

			if cu.Properties.Valid {
				var props map[string]interface{}
				err := json.Unmarshal(cu.Properties.JSON, &props)
				if err != nil {
					return errors.Wrapf(err, "json.Unmarshal properties %s", cu.UID)
				}

				if filmDate, ok := props["film_date"]; ok {
					val, err := time.Parse("2006-01-02", filmDate.(string))
					if err != nil {
						return errors.Wrapf(err, "time.Parse film_date %s", cu.UID)
					}
					unit.EffectiveDate = &utils.Date{Time: val}
				}
			}

			if val, ok := indexData.Sources[cu.UID]; ok {
				unit.FilterValues = append(unit.FilterValues, KeyValues("source", val)...)
				unit.TypedUids = append(unit.TypedUids, KeyValues("source", val)...)
			}
			if val, ok := indexData.Tags[cu.UID]; ok {
				unit.FilterValues = append(unit.FilterValues, KeyValues("tag", val)...)
				unit.TypedUids = append(unit.TypedUids, KeyValues("tag", val)...)
			}
			// if val, ok := indexData.Persons[cu.UID]; ok {
			// 	unit.Persons = val
			// 	unit.TypedUids = append(unit.TypedUids, KeyValues("person", val)...)
			// }
			// if val, ok := indexData.Translations[cu.UID]; ok {
			// 	unit.Translations = val[1]
			// 	unit.TypedUids = append(unit.TypedUids, KeyValues("file", val[0])...)
			// }
			log.Infof("11111")
			if byLang, ok := indexData.Transcripts[cu.UID]; ok {
				log.Infof("22222")
				if val, ok := byLang[i18n.Language]; ok {
					log.Infof("3333")
					var err error
					fileName, err := LoadDocFilename(index.db, val[0])
					if err != nil {
						log.Errorf("Content Units Index - Error retrieving doc from DB: %s. Error: %+v", val[0], err)
					} else if fileName == "" {
						log.Warnf("Content Units Index - Could not get transcript filename for %s, maybe it is not published or not secure.  Skipping.", val[0])
					} else {
						log.Infof("3333.55555 %s %s", val[0], fileName)
						err = DownloadAndConvert([][]string{{val[0], fileName}})
						if err != nil {
							log.Errorf("Content Units Index - Error downloading or converting doc: %s", val[0])
							log.Errorf("Content Units Index - Error %+v", err)
						} else {
							docxFilename := fmt.Sprintf("%s.docx", val[0])
							folder, err := DocFolder()
							if err != nil {
								return err
							}
							docxPath := path.Join(folder, docxFilename)
							unit.Content, err = ParseDocx(docxPath)
							if unit.Content == "" {
								log.Warnf("Content Units Index - Transcript empty: %s", val[0])
							} else {
								log.Infof("44444 content length %d", len(unit.Content))
							}
							if err != nil {
								log.Errorf("Content Units Index - Error parsing docx: %s", val[0])
							} else {
								unit.TypedUids = append(unit.TypedUids, keyValue("file", val[0]))
							}
						}
					}
				}
			}

			i18nMap[i18n.Language] = unit
		}
	}

	// Index each document in its language index
	for k, v := range i18nMap {
		name := index.indexName(k)

		log.Infof("Indexing %s content length: %d", k, len(v.Content))
		if len(v.Content) > 1000 {
			log.Infof("Content: [%s]", v.Content[0:1000])
		}

		b, err := json.Marshal(v)
		if err != nil {
			fmt.Printf("Error: %s", err)
			return errors.Wrapf(err, "BLAH!")
		}
		fmt.Println(string(b))

		log.Infof("Content Units Index - Add content unit %s to index %s", v.ToString(), name)
		resp, err := index.esc.Index().
			Index(name).
			Type("result").
			// BodyJson(v).
			BodyString(string(b)).
			Do(context.TODO())
		if err != nil {
			return errors.Wrapf(err, "Content Units Index - Index unit %s %s", name, cu.UID)
		}
		if resp.Result != "created" {
			return errors.Errorf("Content Units Index - Not created: unit %s %s %+v", name, cu.UID, resp)
		}
	}

	atomic.AddUint64(&index.Progress, 1)
	progress := atomic.LoadUint64(&index.Progress)
	if progress%100 == 0 {
		log.Infof("Progress units %d", progress)
	}

	return nil
}

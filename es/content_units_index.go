package es

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

func MakeContentUnitsIndex(namespace string) *ContentUnitsIndex {
	cui := new(ContentUnitsIndex)
	cui.baseName = consts.ES_UNITS_INDEX
	cui.namespace = namespace
	return cui
}

type ContentUnitsIndex struct {
	BaseIndex
	indexData *IndexData
}

func defaultContentUnit(cu *mdbmodels.ContentUnit) bool {
	return cu.Secure == 0 && cu.Published && !utils.Int64InSlice(cu.TypeID, []int64{
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_CLIP].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LELO_MIKUD].ID,
	})
}

func defaultContentUnitSql() string {
	return fmt.Sprintf("cu.secure = 0 AND cu.published IS TRUE AND cu.type_id NOT IN (%d, %d)",
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_CLIP].ID,
		mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LELO_MIKUD].ID)
}

func (index *ContentUnitsIndex) ReindexAll() error {
	log.Infof("Content Units Index - Reindex all.")
	if _, err := index.removeFromIndexQuery(elastic.NewMatchAllQuery()); err != nil {
		return err
	}
	return index.addToIndexSql(defaultContentUnitSql())
}

func (index *ContentUnitsIndex) Add(scope Scope) error {
	log.Infof("Content Units Index - Add. Scope: %+v.", scope)
	// We only add content units when the scope is content unit, otherwise we need to update.
	if scope.ContentUnitUID != "" {
		if err := index.addToIndex(Scope{ContentUnitUID: scope.ContentUnitUID}, []string{}); err != nil {
			return err
		}
		scope.ContentUnitUID = ""
	}
	emptyScope := Scope{}
	if scope != emptyScope {
		return index.Update(scope)
	}
	return nil
}

func (index *ContentUnitsIndex) Update(scope Scope) error {
	log.Infof("Content Units Index - Update. Scope: %+v.", scope)
	removed, err := index.removeFromIndex(scope)
	if err != nil {
		return err
	}
	return index.addToIndex(scope, removed)
}

func (index *ContentUnitsIndex) Delete(scope Scope) error {
	log.Infof("Content Units Index - Delete. Scope: %+v.", scope)
	// We only delete content units when content unit is deleted, otherwise we just update.
	if scope.ContentUnitUID != "" {
		if _, err := index.removeFromIndex(Scope{ContentUnitUID: scope.ContentUnitUID}); err != nil {
			return err
		}
		scope.ContentUnitUID = ""
	}
	emptyScope := Scope{}
	if scope != emptyScope {
		return index.Update(scope)
	}
	return nil
}

func (index *ContentUnitsIndex) addToIndex(scope Scope, removedUIDs []string) error {
	// TODO: Work not done! Missing tags and sources scopes!
	sqlScope := defaultContentUnitSql()
	uids := removedUIDs
	if scope.ContentUnitUID != "" {
		uids = append(uids, scope.ContentUnitUID)
	}
	if scope.FileUID != "" {
		moreUIDs, err := contentUnitsScopeByFile(scope.FileUID)
		if err != nil {
			return err
		}
		uids = append(uids, moreUIDs...)
	}
	if scope.CollectionUID != "" {
		moreUIDs, err := contentUnitsScopeByCollection(scope.CollectionUID)
		if err != nil {
			return err
		}
		uids = append(uids, moreUIDs...)
	}
	if scope.SourceUID != "" {
		moreUIDs, err := contentUnitsScopeBySource(scope.SourceUID)
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
	var typedUIDs []string
	if scope.ContentUnitUID != "" {
		typedUIDs = append(typedUIDs, uidToTypedUID("content_unit", scope.ContentUnitUID))
	}
	if scope.FileUID != "" {
		typedUIDs = append(typedUIDs, uidToTypedUID("file", scope.FileUID))
	}
	if scope.CollectionUID != "" {
		typedUIDs = append(typedUIDs, uidToTypedUID("collection", scope.CollectionUID))
		moreUIDs, err := contentUnitsScopeByCollection(scope.CollectionUID)
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
	if scope.PersonUID != "" {
		typedUIDs = append(typedUIDs, uidToTypedUID("person", scope.PersonUID))
	}
	if scope.PublisherUID != "" {
		typedUIDs = append(typedUIDs, uidToTypedUID("publisher", scope.PublisherUID))
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

func (index *ContentUnitsIndex) addToIndexSql(sqlScope string) error {
	var count int64
	err := mdbmodels.NewQuery(mdb.DB,
		qm.Select("COUNT(1)"),
		qm.From("content_units as cu"),
		qm.Where(sqlScope)).QueryRow().Scan(&count)
	if err != nil {
		return err
	}

	log.Infof("Content Units Index - Adding %d units. Scope: %s", count, sqlScope)

	offset := 0
	limit := 1000
	for offset < int(count) {
		var units []*mdbmodels.ContentUnit
		err := mdbmodels.NewQuery(mdb.DB,
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
		log.Infof("Adding %d units (offset: %d).", len(units), offset)

		index.indexData = new(IndexData)
		err = index.indexData.Load(sqlScope)
		if err != nil {
			return err
		}
		for _, unit := range units {
			if err := index.indexUnit(unit); err != nil {
				return err
			}
		}
		offset += limit
	}

	return nil
}

func (index *ContentUnitsIndex) removeFromIndexQuery(elasticScope elastic.Query) ([]string, error) {
	source, err := elasticScope.Source()
	if err != nil {
		return []string{}, err
	}
	jsonBytes, err := json.Marshal(source)
	if err != nil {
		return []string{}, err
	}
	log.Infof("Content Untis Index - Removing from index. Scope: %s", string(jsonBytes))
	removed := make(map[string]bool)
	for _, lang := range consts.ALL_KNOWN_LANGS {
		indexName := index.indexName(lang)
		searchRes, err := mdb.ESC.Search(indexName).Query(elasticScope).Do(context.TODO())
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
		delRes, err := mdb.ESC.DeleteByQuery(indexName).
			Query(elasticScope).
			Do(context.TODO())
		if err != nil {
			return []string{}, errors.Wrapf(err, "Remove from index %s %+v\n", indexName, elasticScope)
		}
		if delRes.Deleted > 0 {
			fmt.Printf("Deleted %d documents from %s.\n", delRes.Deleted, indexName)
		}
	}
	if len(removed) == 0 {
		fmt.Println("Content Units Index - Nothing was delete.")
		return []string{}, nil
	}
	keys := make([]string, len(removed))
	for k := range removed {
		keys = append(keys, k)
	}
	return keys, nil
}

func collectionsContentTypes(collectionsContentUnits mdbmodels.CollectionsContentUnitSlice) []string {
	ret := make([]string, len(collectionsContentUnits))
	for i, ccu := range collectionsContentUnits {
		ret[i] = mdb.CONTENT_TYPE_REGISTRY.ByID[ccu.R.Collection.TypeID].Name
	}
	return ret
}

func collectionsTypedUIDs(collectionsContentUnits mdbmodels.CollectionsContentUnitSlice) []string {
	ret := make([]string, len(collectionsContentUnits))
	for i, ccu := range collectionsContentUnits {
		ret[i] = uidToTypedUID("collection", ccu.R.Collection.UID)
	}
	return ret
}

func (index *ContentUnitsIndex) indexUnit(cu *mdbmodels.ContentUnit) error {
	// Create documents in each language with available translation
	i18nMap := make(map[string]ContentUnit)
	for _, i18n := range cu.R.ContentUnitI18ns {
		if i18n.Name.Valid && i18n.Name.String != "" {
			typedUIDs := append([]string{uidToTypedUID("content_unit", cu.UID)},
				collectionsTypedUIDs(cu.R.CollectionsContentUnits)...)
			unit := ContentUnit{
				MDB_UID:                 cu.UID,
				TypedUIDs:               typedUIDs,
				Name:                    i18n.Name.String,
				ContentType:             mdb.CONTENT_TYPE_REGISTRY.ByID[cu.TypeID].Name,
				CollectionsContentTypes: collectionsContentTypes(cu.R.CollectionsContentUnits),
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

				if duration, ok := props["duration"]; ok {
					unit.Duration = uint16(math.Max(0, duration.(float64)))
				}

				if originalLanguage, ok := props["original_language"]; ok {
					unit.OriginalLanguage = originalLanguage.(string)
				}
			}

			if val, ok := index.indexData.Sources[cu.UID]; ok {
				unit.Sources = val
				unit.TypedUIDs = append(unit.TypedUIDs, uidsToTypedUIDs("source", val)...)
			}
			if val, ok := index.indexData.Tags[cu.UID]; ok {
				unit.Tags = val
				unit.TypedUIDs = append(unit.TypedUIDs, uidsToTypedUIDs("tag", val)...)
			}
			if val, ok := index.indexData.Persons[cu.UID]; ok {
				unit.Persons = val
				unit.TypedUIDs = append(unit.TypedUIDs, uidsToTypedUIDs("person", val)...)
			}
			if val, ok := index.indexData.Translations[cu.UID]; ok {
				unit.Translations = val[1]
				unit.TypedUIDs = append(unit.TypedUIDs, uidsToTypedUIDs("file", val[0])...)
			}
			if byLang, ok := index.indexData.Transcripts[cu.UID]; ok {
				if val, ok := byLang[i18n.Language]; ok {
					var err error
					fileName, err := LoadDocFilename(val[0])
					if err != nil {
						log.Errorf("Error retrieving doc from DB: %s", val[0])
					} else {
						err = DownloadAndConvert([][]string{{val[0], fileName}})
						if err != nil {
							log.Errorf("Error downloading or converting doc: %s", val[0])
							log.Errorf("Error %+v", err)
						} else {
							docxFilename := fmt.Sprintf("%s.docx", val[0])
							docxPath := path.Join(mdb.DocFolder, docxFilename)
							unit.Transcript, err = index.ParseDocx(docxPath)
							if err != nil {
								log.Errorf("Error parsing docx: %s", val[0])
							} else {
								unit.TypedUIDs = append(unit.TypedUIDs, uidToTypedUID("file", val[0]))
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
		vCopy := v
		if len(vCopy.Transcript) > 30 {
			vCopy.Transcript = fmt.Sprintf("%s...", vCopy.Transcript[:30])
		}
		vBytes, err := json.Marshal(vCopy)
		if err != nil {
			return err
		}
		log.Infof("Content Units Index - Add content unit %s to index %s", string(vBytes), name)
		resp, err := mdb.ESC.Index().
			Index(name).
			Type("content_units").
			BodyJson(v).
			Do(context.TODO())
		if err != nil {
			return errors.Wrapf(err, "Index unit %s %s", name, cu.UID)
		}
		if !resp.Created {
			return errors.Errorf("Not created: unit %s %s", name, cu.UID)
		}
	}

	return nil
}

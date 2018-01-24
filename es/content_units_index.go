package es

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/queries"
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
	cui.docFolder = path.Join(viper.GetString("elasticsearch.docx-folder"))
	return cui
}

type ContentUnitsIndex struct {
	BaseIndex
	indexData *IndexData
	docFolder string
}

func (index *ContentUnitsIndex) ReindexAll() error {
	if err := index.removeFromIndex(elastic.NewMatchAllQuery()); err != nil {
		return err
	}
	return index.addToIndex("cu.secure = 0 AND cu.published IS TRUE")
}

func contentUnitsScopeByFile(fileUID string) ([]string, error) {
	units, err := mdbmodels.ContentUnits(mdb.DB,
		qm.Select("uid"),
		qm.InnerJoin("files AS f on f.content_unit_id = content_unit.id"),
		qm.Where("f.uid = ?", fileUID)).All()
	if err != nil {
		return nil, err
	}
	uids := make([]string, len(units))
	for i, unit := range units {
		uids[i] = unit.UID
	}
	return uids, nil
}

func (index *ContentUnitsIndex) AddToIndex(scope Scope) error {
	sqlScope := "cu.secure = 0 AND cu.published IS TRUE"
	var uids []string
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
	if len(uids) > 0 {
		quoted := make([]string, len(uids))
		for i, uid := range uids {
			quoted[i] = fmt.Sprintf("'%s'", uid)
		}
		sqlScope = fmt.Sprintf("%s AND cu.uid IN (%s)", sqlScope, strings.Join(quoted, ","))
	}
	return index.addToIndex(sqlScope)
}

func (index *ContentUnitsIndex) RemoveFromIndex(scope Scope) error {
	var typedUIDs []string
	if scope.ContentUnitUID != "" {
		typedUIDs = append(typedUIDs, fmt.Sprintf("content_unit:%s", scope.ContentUnitUID))
	}
	if scope.FileUID != "" {
		typedUIDs = append(typedUIDs, fmt.Sprintf("file:%s", scope.FileUID))
	}
	if scope.CollectionUID != "" {
		typedUIDs = append(typedUIDs, fmt.Sprintf("collection:%s", scope.CollectionUID))
	}
	if scope.TagUID != "" {
		typedUIDs = append(typedUIDs, fmt.Sprintf("tag:%s", scope.TagUID))
	}
	if scope.SourceUID != "" {
		typedUIDs = append(typedUIDs, fmt.Sprintf("source:%s", scope.SourceUID))
	}
	if scope.PersonUID != "" {
		typedUIDs = append(typedUIDs, fmt.Sprintf("person:%s", scope.PersonUID))
	}
	if scope.PublisherUID != "" {
		typedUIDs = append(typedUIDs, fmt.Sprintf("publisher:%s", scope.PublisherUID))
	}
	var elasticScope elastic.Query
	if len(typedUIDs) > 0 {
		typedUIDsI := make([]interface{}, len(typedUIDs))
		for i, typedUID := range typedUIDs {
			typedUIDsI[i] = typedUID
		}
		elasticScope = elastic.NewTermsQuery("typed_uids", typedUIDsI...)
		return index.removeFromIndex(elasticScope)
	} else {
		// Nothing to remove.
		return nil
	}
}

func (index *ContentUnitsIndex) addToIndex(sqlScope string) error {
	var units []*mdbmodels.ContentUnit
	// Note: I have noticed that Load("ContentUnitI18ns") uses following SQL:
	// select * from "content_unit_i18n" where "content_unit_id" in ($1,$2,$3,$4,$5,$ ...
	// Which is bad as there is a limit on X in [...list...]. We should really do inner join.
	// This is a problem for reindexing all elements ofcourse.
	err := mdbmodels.NewQuery(mdb.DB,
		qm.From("content_units as cu"),
		qm.Load("ContentUnitI18ns"),
		qm.Load("CollectionsContentUnits"),
		qm.Load("CollectionsContentUnits.Collection"),
		qm.Where(sqlScope)).Bind(&units)
	if err != nil {
		return errors.Wrap(err, "Fetch units from mdb")
	}
	log.Infof("Adding %d units.", len(units))

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
	return nil
}

func (index *ContentUnitsIndex) removeFromIndex(elasticScope elastic.Query) error {
	for _, lang := range consts.ALL_KNOWN_LANGS {
		indexName := index.indexName(lang)
		res, err := mdb.ESC.DeleteByQuery(indexName).
			Query(elasticScope).
			Do(context.TODO())
		if err != nil {
			return errors.Wrapf(err, "Remove from index %s %+v\n", indexName, elasticScope)
		}
		if res.Deleted > 0 {
			fmt.Printf("Deleted %d documents from %s.\n", res.Deleted, indexName)
		}
		// If not exists Deleted will be 0.
		// if resp.Deleted != int64(len(uids)) {
		// 	return errors.Errorf("Not deleted: %s %+v\n", indexName, uids)
		// }
	}
	return nil
}

func (index *ContentUnitsIndex) parseDocx(uid string) (string, error) {
	docxFilename := fmt.Sprintf("%s.docx", uid)
	docxPath := path.Join(index.docFolder, docxFilename)
	if _, err := os.Stat(docxPath); os.IsNotExist(err) {
		return "", nil
	}
	cmd := exec.Command("es/parse_docs.py", docxPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Warnf("parse_docs.py %s\nstdout: %s\nstderr: %s", docxPath, stdout.String(), stderr.String())
		return "", errors.Wrapf(err, "cmd.Run %s", uid)
	}
	return stdout.String(), nil
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
		ret[i] = fmt.Sprintf("collection:%s", ccu.R.Collection.UID)
	}
	return ret
}

func uidsToTypedUIDs(t string, uids []string) []string {
	ret := make([]string, len(uids))
	for i, uid := range uids {
		ret[i] = fmt.Sprintf("%s:%s", t, uid)
	}
	return ret
}

func (index *ContentUnitsIndex) indexUnit(cu *mdbmodels.ContentUnit) error {
	fmt.Printf("indexUnit: %+v\n", cu)
	// Create documents in each language with available translation
	i18nMap := make(map[string]ContentUnit)
	for i := range cu.R.ContentUnitI18ns {
		i18n := cu.R.ContentUnitI18ns[i]
		if i18n.Name.Valid && i18n.Name.String != "" {
			typedUIDs := append([]string{fmt.Sprintf("content_unit:%s", cu.UID)},
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
					unit.FilmDate = &utils.Date{Time: val}
				}

				if duration, ok := props["duration"]; ok {
					unit.Duration = uint16(math.Max(0, duration.(float64)))
				}

				if originalLanguage, ok := props["original_language"]; ok {
					unit.OriginalLanguage = originalLanguage.(string)
				}
			}

			if val, ok := index.indexData.Sources[cu.ID]; ok {
				unit.Sources = val
				unit.TypedUIDs = append(unit.TypedUIDs, uidsToTypedUIDs("source", val)...)
			}
			if val, ok := index.indexData.Tags[cu.ID]; ok {
				unit.Tags = val
				unit.TypedUIDs = append(unit.TypedUIDs, uidsToTypedUIDs("tag", val)...)
			}
			if val, ok := index.indexData.Persons[cu.ID]; ok {
				unit.Persons = val
				unit.TypedUIDs = append(unit.TypedUIDs, uidsToTypedUIDs("person", val)...)
			}
			if val, ok := index.indexData.Translations[cu.ID]; ok {
				unit.Translations = val[1]
				unit.TypedUIDs = append(unit.TypedUIDs, uidsToTypedUIDs("file:", val[0])...)
			}
			if byLang, ok := index.indexData.Transcripts[cu.ID]; ok {
				if val, ok := byLang[i18n.Language]; ok {
					var err error
					unit.Transcript, err = index.parseDocx(val[0])
					unit.TypedUIDs = append(unit.TypedUIDs, fmt.Sprintf("file:%s", val[0]))
					if err != nil {
						log.Warnf("Error parsing docx: %s", val[0])
					}
					// if err == nil && unit.Transcript != "" {
					// 	atomic.AddUint64(&withTranscript, 1)
					// }
				}
			}

			i18nMap[i18n.Language] = unit
		}
	}

	fmt.Printf("i18nMap: %+v\n", i18nMap)

	// Index each document in its language index
	for k, v := range i18nMap {
		name := index.indexName(k)
		fmt.Printf("Indexing to %s: %+v\n", name, v)
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

type IndexData struct {
	Sources      map[int64][]string
	Tags         map[int64][]string
	Persons      map[int64][]string
	Translations map[int64][][]string
	Transcripts  map[int64]map[string][]string
}

func (cm *IndexData) Load(sqlScope string) error {
	var err error

	cm.Sources, err = cm.loadSources(sqlScope)
	if err != nil {
		return err
	}

	cm.Tags, err = cm.loadTags(sqlScope)
	if err != nil {
		return err
	}

	cm.Persons, err = cm.loadPersons(sqlScope)
	if err != nil {
		return err
	}

	cm.Translations, err = cm.loadTranslations(sqlScope)
	if err != nil {
		return err
	}

	cm.Transcripts, err = cm.loadTranscripts(sqlScope)
	if err != nil {
		return err
	}

	return nil
}

func (cm *IndexData) loadSources(sqlScope string) (map[int64][]string, error) {
	rows, err := queries.Raw(mdb.DB, fmt.Sprintf(`
WITH RECURSIVE rec_sources AS (
  SELECT
    s.id,
    s.uid,
    s.position,
    ARRAY [a.code, s.uid] "path"
  FROM sources s INNER JOIN authors_sources aas ON s.id = aas.source_id
    INNER JOIN authors a ON a.id = aas.author_id
  UNION
  SELECT
    s.id,
    s.uid,
    s.position,
    rs.path || s.uid
  FROM sources s INNER JOIN rec_sources rs ON s.parent_id = rs.id
)
SELECT
  cus.content_unit_id,
  array_agg(DISTINCT item)
FROM content_units_sources cus
    INNER JOIN rec_sources AS rs ON cus.source_id = rs.id
    INNER JOIN content_units AS cu ON cus.content_unit_id = cu.id
    , unnest(rs.path) item
WHERE %s
GROUP BY cus.content_unit_id;`, sqlScope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load sources")
	}
	defer rows.Close()

	return cm.rowsToIdToValues(rows)
}

func (cm *IndexData) loadTags(sqlScope string) (map[int64][]string, error) {
	rows, err := queries.Raw(mdb.DB, fmt.Sprintf(`
WITH RECURSIVE rec_tags AS (
  SELECT
    t.id,
    t.uid,
    ARRAY [t.uid] :: CHAR(8) [] "path"
  FROM tags t
  WHERE parent_id IS NULL
  UNION
  SELECT
    t.id,
    t.uid,
    (rt.path || t.uid) :: CHAR(8) []
  FROM tags t INNER JOIN rec_tags rt ON t.parent_id = rt.id
)
SELECT
  cut.content_unit_id,
  array_agg(DISTINCT item)
FROM content_units_tags cut
    INNER JOIN rec_tags AS rt ON cut.tag_id = rt.id
    INNER JOIN content_units AS cu ON cut.content_unit_id = cu.id
    , unnest(rt.path) item
WHERE %s
GROUP BY cut.content_unit_id;`, sqlScope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load tags")
	}
	defer rows.Close()

	return cm.rowsToIdToValues(rows)
}

func (cm *IndexData) loadPersons(sqlScope string) (map[int64][]string, error) {
	rows, err := queries.Raw(mdb.DB, fmt.Sprintf(`
SELECT
  cup.content_unit_id,
  array_agg(p.uid)
FROM content_units_persons cup
    INNER JOIN persons p ON cup.person_id = p.id
    INNER JOIN content_units AS cu ON cup.content_unit_id = cu.id
WHERE %s
GROUP BY cup.content_unit_id;`, sqlScope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load persons")
	}
	defer rows.Close()

	return cm.rowsToIdToValues(rows)
}

func (cm *IndexData) loadTranslations(sqlScope string) (map[int64][][]string, error) {
	rows, err := queries.Raw(mdb.DB, fmt.Sprintf(`
SELECT
  files.content_unit_id,
  array_agg(DISTINCT files.uid),
  array_agg(DISTINCT files.language)
FROM files
    INNER JOIN content_units AS cu ON files.content_unit_id = cu.id
WHERE language NOT IN ('zz', 'xx') AND content_unit_id IS NOT NULL AND %s
GROUP BY content_unit_id;`, sqlScope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load translations")
	}
	defer rows.Close()

	return cm.rowsToIdToUIDsAndValues(rows)
}

func (cm *IndexData) rowsToIdToValues(rows *sql.Rows) (map[int64][]string, error) {
	m := make(map[int64][]string)

	for rows.Next() {
		var cuid int64
		var values pq.StringArray
		err := rows.Scan(&cuid, &values)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		m[cuid] = values
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}

	return m, nil
}

func (cm *IndexData) rowsToIdToUIDsAndValues(rows *sql.Rows) (map[int64][][]string, error) {
	m := make(map[int64][][]string)

	for rows.Next() {
		var cuid int64
		var values pq.StringArray
		var uids pq.StringArray
		err := rows.Scan(&cuid, &uids, &values)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		m[cuid] = [][]string{uids, values}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}

	return m, nil
}

func (cm *IndexData) loadTranscripts(sqlScope string) (map[int64]map[string][]string, error) {
	rows, err := queries.Raw(mdb.DB, fmt.Sprintf(`
SELECT
    f.uid,
    f.name,
    f.language,
    cu.id
FROM files AS f
    INNER JOIN content_units AS cu ON f.content_unit_id = cu.id
WHERE name ~ '.docx?' AND
    f.language NOT IN ('zz', 'xx') AND
    f.content_unit_id IS NOT NULL AND
    cu.type_id != 31 AND
    %s;`, sqlScope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load transcripts")
	}
	defer rows.Close()

	return loadTranscriptsMap(rows)
}

func loadTranscriptsMap(rows *sql.Rows) (map[int64]map[string][]string, error) {
	m := make(map[int64]map[string][]string)

	for rows.Next() {
		var uid string
		var name string
		var language string
		var cuID int64
		err := rows.Scan(&uid, &name, &language, &cuID)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		if _, ok := m[cuID]; !ok {
			m[cuID] = make(map[string][]string)
		}
		m[cuID][language] = []string{uid, name}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}

	return m, nil
}

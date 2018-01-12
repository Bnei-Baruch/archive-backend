package es

import (
	"bytes"
	"context"
    "fmt"
	"database/sql"
	"encoding/json"
    "math"
	"os"
	"os/exec"
	"path"
    "time"

	log "github.com/Sirupsen/logrus"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/queries"
	"github.com/volatiletech/sqlboiler/queries/qm"

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
    return index.Reindex("cu.secure = 0 AND cu.published IS TRUE")
}

// After scoping
func (index *ContentUnitsIndex) Reindex(scope string) error {
    units, err := mdbmodels.ContentUnits(db,
        qm.Load("ContentUnitI18ns"),
        qm.Load("CollectionsContentUnits"),
        qm.Load("CollectionsContentUnits.Collection"),
        qm.Where(scope)).
        All()
    if err != nil {
        return errors.Wrap(err, "Fetch units from mdb")
    }
    log.Infof("Reindexing %d units (secure and published).", len(units))

    index.indexData = new(IndexData)
    err = index.indexData.Load(scope)
    if err != nil {
        return err
    }

    for _, unit := range units {
        if err = index.RemoveFromIndex(unit); err != nil {
            return err
        }
        if err = index.IndexUnit(unit); err != nil {
            return err
        }
    }
    return errors.New("Not implemented.")
}

func (index* ContentUnitsIndex) RemoveFromIndex(cu *mdbmodels.ContentUnit) error {
    return errors.New("Not implemented.")
}

func (index* ContentUnitsIndex) ParseDocx(uid string) (string, error) {
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

func (index* ContentUnitsIndex) collectionsContentTypes(collectionsContentUnits mdbmodels.CollectionsContentUnitSlice) []string {
	ret := make([]string, len(collectionsContentUnits))
	for i, ccu := range collectionsContentUnits {
		ret[i] = mdb.CONTENT_TYPE_REGISTRY.ByID[ccu.R.Collection.TypeID].Name
	}
	return ret
}

func (index* ContentUnitsIndex) IndexUnit(cu *mdbmodels.ContentUnit) error {
	// create documents in each language with available translation
	i18nMap := make(map[string]ContentUnit)
	for i := range cu.R.ContentUnitI18ns {
		i18n := cu.R.ContentUnitI18ns[i]
		if i18n.Name.Valid && i18n.Name.String != "" {
			unit := ContentUnit{
				MDB_UID:                 cu.UID,
				Name:                    i18n.Name.String,
				ContentType:             mdb.CONTENT_TYPE_REGISTRY.ByID[cu.TypeID].Name,
				CollectionsContentTypes: index.collectionsContentTypes(cu.R.CollectionsContentUnits),
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
			}
			if val, ok := index.indexData.Tags[cu.ID]; ok {
				unit.Tags = val
			}
			if val, ok := index.indexData.Persons[cu.ID]; ok {
				unit.Persons = val
			}
			if val, ok := index.indexData.Translations[cu.ID]; ok {
				unit.Translations = val
			}
			if byLang, ok := index.indexData.Transcripts[cu.ID]; ok {
				if val, ok := byLang[i18n.Language]; ok {
					var err error
					unit.Transcript, err = index.ParseDocx(val[0])
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

	// Index each document in its language index
	for k, v := range i18nMap {
		name := index.indexName(k)
		resp, err := esc.Index().
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
	Translations map[int64][]string
	Transcripts  map[int64]map[string][]string
}

func (cm *IndexData) Load(scope string) error {
	var err error

	cm.Sources, err = cm.loadSources(scope)
	if err != nil {
		return err
	}

	cm.Tags, err = cm.loadTags(scope)
	if err != nil {
		return err
	}

	cm.Persons, err = cm.loadPersons(scope)
	if err != nil {
		return err
	}

	cm.Translations, err = cm.loadTranslations(scope)
	if err != nil {
		return err
	}

	cm.Transcripts, err = cm.loadTranscripts(scope)
	if err != nil {
		return err
	}

	return nil
}

func (cm *IndexData) loadSources(scope string) (map[int64][]string, error) {
	rows, err := queries.Raw(db, fmt.Sprintf(`
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
    , unnest(rs.path) item
    INNER JOIN content_units AS cu ON cus.content_unit_id = cu.id
WHERE %s
GROUP BY cus.content_unit_id;`, scope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load sources")
	}
	defer rows.Close()

	return cm.loadMap(rows)
}

func (cm *IndexData) loadTags(scope string) (map[int64][]string, error) {
	rows, err := queries.Raw(db, fmt.Sprintf(`
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
    , unnest(rt.path) item
    INNER JOIN content_units AS cu ON cut.content_unit_id = cu.id
WHERE %s
GROUP BY cut.content_unit_id;`, scope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load tags")
	}
	defer rows.Close()

	return cm.loadMap(rows)
}

func (cm *IndexData) loadPersons(scope string) (map[int64][]string, error) {
	rows, err := queries.Raw(db, fmt.Sprintf(`
SELECT
  cup.content_unit_id,
  array_agg(p.uid)
FROM content_units_persons cup
    INNER JOIN persons p ON cup.person_id = p.id
    INNER JOIN content_units AS cu ON cup.content_unit_id = cu.id
WHERE %s
GROUP BY cup.content_unit_id;`, scope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load persons")
	}
	defer rows.Close()

	return cm.loadMap(rows)
}

func (cm *IndexData) loadTranslations(scope string) (map[int64][]string, error) {
	rows, err := queries.Raw(db, fmt.Sprintf(`
SELECT
  content_unit_id,
  array_agg(DISTINCT language)
FROM files
    INNER JOIN content_units AS cu ON files.content_unit_id = cu.id
WHERE language NOT IN ('zz', 'xx') AND content_unit_id IS NOT NULL AND %s
GROUP BY content_unit_id;`, scope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load translations")
	}
	defer rows.Close()

	return cm.loadMap(rows)
}

func (cm *IndexData) loadMap(rows *sql.Rows) (map[int64][]string, error) {
	m := make(map[int64][]string)

	for rows.Next() {
		var cuid int64
		var sources pq.StringArray
		err := rows.Scan(&cuid, &sources)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		m[cuid] = sources
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}

	return m, nil
}

func (cm *IndexData) loadTranscripts(scope string) (map[int64]map[string][]string, error) {
	rows, err := queries.Raw(db, fmt.Sprintf(`
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
    %s;`, scope)).Query()

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

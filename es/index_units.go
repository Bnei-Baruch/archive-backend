package es

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/sync/errgroup"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/vattle/sqlboiler/queries"
	"github.com/vattle/sqlboiler/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var CLASSIFICATIONS_MANAGER = new(ClassificationsManager)

func IndexUnits() {
	clock := Init()

	for i := range consts.ALL_KNOWN_LANGS {
		lang := consts.ALL_KNOWN_LANGS[i]
		name := IndexName(consts.ES_UNITS_INDEX, lang)
		mappings := fmt.Sprintf("data/es/mappings/units/units-%s.json", lang)
		utils.Must(recreateIndex(name, mappings))
	}

	log.Info("Loading content units classifications")
	utils.Must(CLASSIFICATIONS_MANAGER.Load())

	docFolder = path.Join(viper.GetString("elasticsearch.docx-folder"))

	ctx := context.Background()
	utils.Must(indexUnits(ctx))

	for i := range consts.ALL_KNOWN_LANGS {
		lang := consts.ALL_KNOWN_LANGS[i]
		name := IndexName(consts.ES_UNITS_INDEX, lang)
		utils.Must(finishIndexing(name))
	}

	Shutdown()
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}

var total uint64 = 0
var indexed uint64 = 0
var withTranscript uint64 = 0

func indexUnits(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	unitsCH := make(chan *mdbmodels.ContentUnit)
	g.Go(func() error {
		defer close(unitsCH)

		count, err := mdbmodels.ContentUnits(db).Count()
		if err != nil {
			return errors.Wrap(err, "Count units in mdb")
		}
		log.Infof("%d units in MDB", count)
		atomic.AddUint64(&total, uint64(count))

		units, err := mdbmodels.ContentUnits(db,
			qm.Load("ContentUnitI18ns")).
			All()
		if err != nil {
			return errors.Wrap(err, "Fetch units from mdb")
		}

		for i := range units {
			unit := units[i]

			select {
			case unitsCH <- unit:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	})

	for i := 1; i <= 5; i++ {
		g.Go(func() error {
			for unit := range unitsCH {
				if err := indexUnit(unit); err != nil {
					log.Errorf("Index unit error: %s", err.Error())
					return err
				}
				atomic.AddUint64(&indexed, 1)

				if atomic.LoadUint64(&indexed)%100 == 0 {
					log.Infof("Indexed %d / %d. With transcripts %d.", indexed, total, withTranscript)
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func ParseDocx(uid string) (string, error) {
	docxFilename := fmt.Sprintf("%s.docx", uid)
	docxPath := path.Join(docFolder, docxFilename)
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

func indexUnit(cu *mdbmodels.ContentUnit) error {
	//// create documents in each language with available translation
	i18nMap := make(map[string]ContentUnit)
	for i := range cu.R.ContentUnitI18ns {
		i18n := cu.R.ContentUnitI18ns[i]
		if i18n.Name.Valid && i18n.Name.String != "" {
			unit := ContentUnit{
				MDB_UID:     cu.UID,
				Name:        i18n.Name.String,
				ContentType: mdb.CONTENT_TYPE_REGISTRY.ByID[cu.TypeID].Name,
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
					unit.Duration = int16(duration.(float64))
				}

				if originalLanguage, ok := props["original_language"]; ok {
					unit.OriginalLanguage = originalLanguage.(string)
				}
			}

			if val, ok := CLASSIFICATIONS_MANAGER.Sources[cu.ID]; ok {
				unit.Sources = val
			}
			if val, ok := CLASSIFICATIONS_MANAGER.Tags[cu.ID]; ok {
				unit.Tags = val
			}
			if val, ok := CLASSIFICATIONS_MANAGER.Persons[cu.ID]; ok {
				unit.Persons = val
			}
			if val, ok := CLASSIFICATIONS_MANAGER.Translations[cu.ID]; ok {
				unit.Translations = val
			}
			if byLang, ok := CLASSIFICATIONS_MANAGER.Transcripts[cu.ID]; ok {
				if val, ok := byLang[i18n.Language]; ok {
					var err error
					unit.Transcript, err = ParseDocx(val[0])
					if err != nil && unit.Transcript != "" {
						atomic.AddUint64(&withTranscript, 1)
					}
				}
			}

			i18nMap[i18n.Language] = unit
		}
	}

	// index each document in its language index
	for k, v := range i18nMap {
		name := IndexName(consts.ES_UNITS_INDEX, k)
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

type ClassificationsManager struct {
	Sources      map[int64][]string
	Tags         map[int64][]string
	Persons      map[int64][]string
	Translations map[int64][]string
	Transcripts  map[int64]map[string][]string
}

func (cm *ClassificationsManager) Load() error {
	var err error

	cm.Sources, err = cm.loadSources()
	if err != nil {
		return err
	}

	cm.Tags, err = cm.loadTags()
	if err != nil {
		return err
	}

	cm.Persons, err = cm.loadPersons()
	if err != nil {
		return err
	}

	cm.Translations, err = cm.loadTranslations()
	if err != nil {
		return err
	}

	cm.Transcripts, err = cm.loadTranscripts()
	if err != nil {
		return err
	}

	return nil
}

func (cm *ClassificationsManager) loadSources() (map[int64][]string, error) {
	rows, err := queries.Raw(db, `
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
FROM content_units_sources cus INNER JOIN rec_sources AS rs ON cus.source_id = rs.id
  , unnest(rs.path) item
GROUP BY cus.content_unit_id;`).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load sources")
	}
	defer rows.Close()

	return cm.loadMap(rows)
}

func (cm *ClassificationsManager) loadTags() (map[int64][]string, error) {
	rows, err := queries.Raw(db, `
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
FROM content_units_tags cut INNER JOIN rec_tags AS rt ON cut.tag_id = rt.id
  , unnest(rt.path) item
GROUP BY cut.content_unit_id;`).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load tags")
	}
	defer rows.Close()

	return cm.loadMap(rows)
}

func (cm *ClassificationsManager) loadPersons() (map[int64][]string, error) {
	rows, err := queries.Raw(db, `
SELECT
  cup.content_unit_id,
  array_agg(p.uid)
FROM content_units_persons cup INNER JOIN persons p ON cup.person_id = p.id
GROUP BY cup.content_unit_id;`).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load persons")
	}
	defer rows.Close()

	return cm.loadMap(rows)
}

func (cm *ClassificationsManager) loadTranslations() (map[int64][]string, error) {
	rows, err := queries.Raw(db, `
SELECT
  content_unit_id,
  array_agg(DISTINCT language)
FROM files
WHERE language NOT IN ('zz', 'xx') AND content_unit_id IS NOT NULL AND secure=0 AND published IS TRUE
GROUP BY content_unit_id;`).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load translations")
	}
	defer rows.Close()

	return cm.loadMap(rows)
}

func (cm *ClassificationsManager) loadMap(rows *sql.Rows) (map[int64][]string, error) {
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

func (cm *ClassificationsManager) loadTranscripts() (map[int64]map[string][]string, error) {
	rows, err := queries.Raw(db, `
SELECT
    f.uid,
    f.name,
    f.language,
    cu.id
FROM files as f, content_units as cu
WHERE name ~ '.docx?' AND
    f.language NOT IN ('zz', 'xx') AND
    f.content_unit_id IS NOT NULL AND
    f.secure=0 AND f.published IS TRUE AND
    f.content_unit_id = cu.id AND cu.type_id != 31;`).Query()

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
		var content_unit_id int64
		err := rows.Scan(&uid, &name, &language, &content_unit_id)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		if _, ok := m[content_unit_id]; !ok {
			m[content_unit_id] = make(map[string][]string)
		}
		m[content_unit_id][language] = []string{uid, name}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}

	return m, nil
}

package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/sync/errgroup"
	"github.com/lib/pq"
	"github.com/pkg/errors"
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
		mappings := fmt.Sprintf("es/mappings/units/units-%s.json", lang)
		utils.Must(recreateIndex(name, mappings))
	}

	log.Info("Loading content units classifications")
	utils.Must(CLASSIFICATIONS_MANAGER.Load())

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

func indexUnit(cu *mdbmodels.ContentUnit) error {
	log.Infof("Indexing unit %s", cu.UID)

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
					unit.Duration = uint16(duration.(float64))
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

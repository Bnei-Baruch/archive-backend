package es

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/queries"

	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/consts"
)

type IndexData struct {
	Sources      map[string][]string
	Tags         map[string][]string
	Persons      map[string][]string
	Translations map[string][][]string
	Transcripts  map[string]map[string][]string
}

func (indexData *IndexData) Load(sqlScope string) error {
	var err error

	indexData.Sources, err = indexData.loadSources(sqlScope)
	if err != nil {
		return err
	}

	indexData.Tags, err = indexData.loadTags(sqlScope)
	if err != nil {
		return err
	}

	indexData.Persons, err = indexData.loadPersons(sqlScope)
	if err != nil {
		return err
	}

	indexData.Translations, err = indexData.loadTranslations(sqlScope)
	if err != nil {
		return err
	}

	indexData.Transcripts, err = indexData.loadTranscripts(sqlScope)
	if err != nil {
		return err
	}

	return nil
}

func (indexData *IndexData) loadSources(sqlScope string) (map[string][]string, error) {
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
  cu.uid,
  array_agg(DISTINCT item)
FROM content_units_sources cus
    INNER JOIN rec_sources AS rs ON cus.source_id = rs.id
    INNER JOIN content_units AS cu ON cus.content_unit_id = cu.id
    , unnest(rs.path) item
WHERE %s
GROUP BY cu.uid;`, sqlScope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load sources")
	}
	defer rows.Close()

	return indexData.rowsToUIDToValues(rows)
}

func (indexData *IndexData) loadTags(sqlScope string) (map[string][]string, error) {
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
  cu.uid,
  array_agg(DISTINCT item)
FROM content_units_tags cut
    INNER JOIN rec_tags AS rt ON cut.tag_id = rt.id
    INNER JOIN content_units AS cu ON cut.content_unit_id = cu.id
    , unnest(rt.path) item
WHERE %s
GROUP BY cu.uid;`, sqlScope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load tags")
	}
	defer rows.Close()

	return indexData.rowsToUIDToValues(rows)
}

func (indexData *IndexData) loadPersons(sqlScope string) (map[string][]string, error) {
	rows, err := queries.Raw(mdb.DB, fmt.Sprintf(`
SELECT
  cu.uid,
  array_agg(p.uid)
FROM content_units_persons cup
    INNER JOIN persons p ON cup.person_id = p.id
    INNER JOIN content_units AS cu ON cup.content_unit_id = cu.id
WHERE %s
GROUP BY cu.uid;`, sqlScope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load persons")
	}
	defer rows.Close()

	return indexData.rowsToUIDToValues(rows)
}

func (indexData *IndexData) loadTranslations(sqlScope string) (map[string][][]string, error) {
	rows, err := queries.Raw(mdb.DB, fmt.Sprintf(`
SELECT
  cu.uid,
  array_agg(DISTINCT files.uid),
  array_agg(DISTINCT files.language)
FROM files
    INNER JOIN content_units AS cu ON files.content_unit_id = cu.id
WHERE language NOT IN ('zz', 'xx') AND content_unit_id IS NOT NULL AND %s
GROUP BY cu.uid;`, sqlScope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load translations")
	}
	defer rows.Close()

	return indexData.rowsToIdToUIDsAndValues(rows)
}

func (indexData *IndexData) rowsToUIDToValues(rows *sql.Rows) (map[string][]string, error) {
	m := make(map[string][]string)

	for rows.Next() {
		var cuUID string
		var values pq.StringArray
		err := rows.Scan(&cuUID, &values)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		m[cuUID] = values
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}

	return m, nil
}

func (indexData *IndexData) rowsToIdToUIDsAndValues(rows *sql.Rows) (map[string][][]string, error) {
	m := make(map[string][][]string)

	for rows.Next() {
		var cuid string
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

func (indexData *IndexData) loadTranscripts(sqlScope string) (map[string]map[string][]string, error) {
    kmID := mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_KITEI_MAKOR].ID
	rows, err := queries.Raw(mdb.DB, fmt.Sprintf(`
SELECT
    f.uid,
    f.name,
    f.language,
    cu.uid
FROM files AS f
    INNER JOIN content_units AS cu ON f.content_unit_id = cu.id
WHERE name ~ '.docx?' AND
    f.language NOT IN ('zz', 'xx') AND
    f.content_unit_id IS NOT NULL AND
    cu.type_id != %d AND
    %s;`, kmID, sqlScope)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load transcripts")
	}
	defer rows.Close()

	return loadTranscriptsMap(rows)
}

func loadTranscriptsMap(rows *sql.Rows) (map[string]map[string][]string, error) {
	m := make(map[string]map[string][]string)

	for rows.Next() {
		var fUID string
		var name string
		var language string
		var cuUID string
		err := rows.Scan(&fUID, &name, &language, &cuUID)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		if _, ok := m[cuUID]; !ok {
			m[cuUID] = make(map[string][]string)
		}
		m[cuUID][language] = []string{fUID, name}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}

	return m, nil
}

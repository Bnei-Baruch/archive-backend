package es

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/queries"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
)

// TranscriptFile holds metadata for a single transcript file
type TranscriptFile struct {
	UID        string
	Name       string
	CreatedAt  time.Time
	InsertType string
}

type IndexData struct {
	DB      *sql.DB
	Sources map[string][]string
	Tags    map[string][]string
	// Persons      map[string][]string
	// Translations map[string][][]string
	MediaLanguages map[string][]string
	Transcripts    map[string]map[string][]TranscriptFile
}

func MakeIndexData(db *sql.DB, sqlScope string) (*IndexData, error) {
	indexData := &IndexData{DB: db}
	err := indexData.load(sqlScope)
	return indexData, err
}

func (indexData *IndexData) load(sqlScope string) error {
	var err error

	indexData.Sources, err = indexData.loadSources(sqlScope)
	if err != nil {
		return err
	}

	indexData.Tags, err = indexData.loadTags(sqlScope)
	if err != nil {
		return err
	}

	// indexData.Persons, err = indexData.loadPersons(sqlScope)
	// if err != nil {
	// 	return err
	// }
	//
	// indexData.Translations, err = indexData.loadTranslations(sqlScope)
	// if err != nil {
	// 	return err
	// }

	indexData.Transcripts, err = indexData.loadTranscripts(sqlScope)
	if err != nil {
		return err
	}

	indexData.MediaLanguages, err = indexData.loadMediaLanguages(sqlScope)
	if err != nil {
		return err
	}

	return nil
}

func (indexData *IndexData) loadSources(sqlScope string) (map[string][]string, error) {
	rows, err := queries.Raw(fmt.Sprintf(`
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
  array_agg(DISTINCT item) FILTER (WHERE item IS NOT NULL AND item <> '')
FROM content_units_sources cus
    INNER JOIN rec_sources AS rs ON cus.source_id = rs.id
    INNER JOIN content_units AS cu ON cus.content_unit_id = cu.id
    , unnest(rs.path) item
WHERE %s
GROUP BY cu.uid;`, sqlScope)).Query(indexData.DB)

	if err != nil {
		return nil, errors.Wrap(err, "Load sources")
	}
	defer rows.Close()

	return indexData.rowsToUIDToValues(rows)
}

func (indexData *IndexData) loadTags(sqlScope string) (map[string][]string, error) {
	rows, err := queries.Raw(fmt.Sprintf(`
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
GROUP BY cu.uid;`, sqlScope)).Query(indexData.DB)

	if err != nil {
		return nil, errors.Wrap(err, "Load tags")
	}
	defer rows.Close()

	return indexData.rowsToUIDToValues(rows)
}

/*func (indexData *IndexData) loadPersons(sqlScope string) (map[string][]string, error) {
	rows, err := queries.Raw(indexData.DB, fmt.Sprintf(`
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
}*/

/*func (indexData *IndexData) loadTranslations(sqlScope string) (map[string][][]string, error) {
	rows, err := queries.Raw(indexData.DB, fmt.Sprintf(`
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
}*/

func (indexData *IndexData) loadTranscripts(sqlScope string) (map[string]map[string][]TranscriptFile, error) {
	// Exclude content types that aren't transcripts
	kmID := mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_KITEI_MAKOR].ID        // Sources - not transcripts
	bookID := mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BOOK].ID             // Books - actual content, not transcripts
	booksID := mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_BOOKS].ID           // Book collections
	songID := mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SONG].ID             // Songs - actual content, not transcripts
	songsID := mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SONGS].ID           // Song collections
	rows, err := queries.Raw(fmt.Sprintf(`
SELECT
    f.uid,
    f.name,
    f.language,
    cu.uid,
    f.created_at,
    COALESCE(f.properties->>'insert_type', '') as insert_type
FROM files AS f
    INNER JOIN content_units AS cu ON f.content_unit_id = cu.id
WHERE f.secure = 0
    AND f.published = true
    AND f.removed_at IS NULL
    AND f.type NOT IN ('audio', 'video', 'subtitles')
    AND (
        f.type = 'text'
        OR f.mime_type = 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
        OR f.mime_type = 'application/msword'
    )
    AND f.language NOT IN ('zz', 'xx')
    AND f.content_unit_id IS NOT NULL
    AND cu.type_id NOT IN (%d, %d, %d, %d, %d)
    AND %s
ORDER BY
    cu.uid,
    f.language,
    CASE
        WHEN f.properties->>'insert_type' = 'tamlil' THEN 1
        WHEN f.properties->>'insert_type' = 'akladot' THEN 2
        WHEN f.properties->>'insert_type' IS NOT NULL THEN 3
        ELSE 4
    END,
    f.created_at DESC,
    f.name;`, kmID, bookID, booksID, songID, songsID, sqlScope)).Query(indexData.DB)

	if err != nil {
		return nil, errors.Wrap(err, "Load transcripts")
	}
	defer rows.Close()

	return loadTranscriptsMap(rows)
}

func loadTranscriptsMap(rows *sql.Rows) (map[string]map[string][]TranscriptFile, error) {
	m := make(map[string]map[string][]TranscriptFile)

	for rows.Next() {
		var fUID string
		var name string
		var language string
		var cuUID string
		var createdAt time.Time
		var insertType string
		err := rows.Scan(&fUID, &name, &language, &cuUID, &createdAt, &insertType)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		if _, ok := m[cuUID]; !ok {
			m[cuUID] = make(map[string][]TranscriptFile)
		}
		m[cuUID][language] = append(m[cuUID][language], TranscriptFile{
			UID:        fUID,
			Name:       name,
			CreatedAt:  createdAt,
			InsertType: insertType,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}

	return m, nil
}

func (indexData *IndexData) loadMediaLanguages(sqlScope string) (map[string][]string, error) {

	rows, err := queries.Raw(fmt.Sprintf(`SELECT cu.uid, array_agg(DISTINCT f.language) 
		FROM files f
			INNER JOIN content_units AS cu ON f.content_unit_id = cu.id
		WHERE f.secure = 0  and f.published = true
		AND (f.type = 'audio' or f.type = 'video')
		AND f.language NOT IN ('zz', 'xx')
		AND f.content_unit_id IS NOT NULL
		AND %s
		GROUP BY cu.uid`, sqlScope)).Query(indexData.DB)

	if err != nil {
		return nil, errors.Wrap(err, "Load media languages")
	}
	defer rows.Close()

	return indexData.rowsToUIDToValues(rows)
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

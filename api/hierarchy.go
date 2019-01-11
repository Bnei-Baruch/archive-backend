package api

import (
	"database/sql"
	"fmt"
	"math"

	"github.com/Bnei-Baruch/sqlboiler/queries"
	"github.com/lib/pq"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
)

// args:
// 0,1,3,4 language
// 2 roots clause, "parent_id is NULL", "id = 8392", etc...
// 5 depth (int)
const SOURCE_HIERARCHY_SQL = `
WITH RECURSIVE rec_sources AS (
  SELECT
    s.id, s.uid, s.parent_id, s.position, s.type_id,
    coalesce((SELECT name FROM source_i18n WHERE source_id = s.id AND language = '%s'),
			 (SELECT name FROM source_i18n WHERE source_id = s.id AND language = 'en'),
             (SELECT name FROM source_i18n WHERE source_id = s.id AND language = 'he'),
			  '') "name",
    coalesce((SELECT description FROM source_i18n WHERE source_id = s.id AND language = '%s'),
			 (SELECT description FROM source_i18n WHERE source_id = s.id AND language = 'en'),
             (SELECT description FROM source_i18n WHERE source_id = s.id AND language = 'he'),
			  '') "description",
	coalesce(properties->>'year', '') "year",
	coalesce(properties->>'number', '') "number",
    1 "depth"
  FROM sources s
  WHERE s.%s
  UNION
  SELECT
    s.id, s.uid, s.parent_id, s.position, s.type_id,
    coalesce((SELECT name FROM source_i18n WHERE source_id = s.id AND language = '%s'),
			 (SELECT name FROM source_i18n WHERE source_id = s.id AND language = 'en'),
             (SELECT name FROM source_i18n WHERE source_id = s.id AND language = 'he'),
			  '') "name",
    coalesce((SELECT description FROM source_i18n WHERE source_id = s.id AND language = '%s'),
			 (SELECT description FROM source_i18n WHERE source_id = s.id AND language = 'en'),
             (SELECT description FROM source_i18n WHERE source_id = s.id AND language = 'he'),
 			  '') "description",
	coalesce(properties->>'year', '') "year",
	coalesce(properties->>'number', '') "number",
    depth + 1
  FROM sources s INNER JOIN rec_sources rs ON s.parent_id = rs.id
  WHERE rs.depth < %d
)
SELECT * FROM rec_sources
ORDER by depth, parent_id, position, name;
`

const AUTHORS_SOURCES_SQL = `
SELECT
  a.code,
  coalesce((SELECT name FROM author_i18n WHERE author_id = a.id AND language = '%s'),
		   (SELECT name FROM author_i18n WHERE author_id = a.id AND language = 'en'),
           (SELECT name FROM author_i18n WHERE author_id = a.id AND language = 'he')) "name",
  coalesce((SELECT full_name FROM author_i18n WHERE author_id = a.id AND language = '%s'),
		   (SELECT full_name FROM author_i18n WHERE author_id = a.id AND language = 'en'),
           (SELECT full_name FROM author_i18n WHERE author_id = a.id AND language = 'he')) "full_name",
  (SELECT array_agg(source_id) FROM authors_sources WHERE author_id = a.id GROUP BY author_id) "sources"
FROM authors a
ORDER BY id;
`

// args:
// 0,2 language
// 1 roots clause, "parent_id is NULL", "id = 8392", etc...
// 3 depth (int)
const TAG_HIERARCHY_SQL = `
WITH RECURSIVE rec_tags AS (
  SELECT
    t.id, t.uid, t.parent_id,
    coalesce((SELECT label FROM tag_i18n WHERE tag_id = t.id AND language = '%s'),
			 (SELECT label FROM tag_i18n WHERE tag_id = t.id AND language = 'en'),
             (SELECT label FROM tag_i18n WHERE tag_id = t.id AND language = 'he')) "label",
    1 "depth"
  FROM tags t
  WHERE t.%s
  UNION
  SELECT
    t.id, t.uid, t.parent_id,
    coalesce((SELECT label FROM tag_i18n WHERE tag_id = t.id AND language = '%s'),
			 (SELECT label FROM tag_i18n WHERE tag_id = t.id AND language = 'en'),
             (SELECT label FROM tag_i18n WHERE tag_id = t.id AND language = 'he')) "label",
    depth + 1
  FROM tags t INNER JOIN rec_tags rt ON t.parent_id = rt.id
  WHERE rt.depth < %d
)
SELECT * FROM rec_tags
ORDER BY depth, parent_id, label;
`

func SourcesHierarchyHandler(c *gin.Context) {
	var r HierarchyRequest
	if c.Bind(&r) != nil {
		return
	}

	resp, err := handleSources(c.MustGet("MDB_DB").(*sql.DB), r)
	concludeRequest(c, resp, err)
}

func TagsHierarchyHandler(c *gin.Context) {
	var r HierarchyRequest
	if c.Bind(&r) != nil {
		return
	}

	resp, err := handleTags(c.MustGet("MDB_DB").(*sql.DB), r)
	concludeRequest(c, resp, err)
}

func handleSources(db *sql.DB, r HierarchyRequest) (interface{}, *HttpError) {
	var l string
	if r.Language == "" {
		l = consts.LANG_HEBREW
	} else {
		l = r.Language
	}

	var depth int
	if r.Depth == 0 {
		depth = math.MaxInt8
	} else {
		depth = r.Depth
	}

	var rootClause string
	if r.RootUID == "" {
		rootClause = "parent_id IS NULL"
	} else {
		rootClause = fmt.Sprintf("uid = '%s'", r.RootUID)
	}

	// Execute query
	rsql := fmt.Sprintf(SOURCE_HIERARCHY_SQL, l, l, rootClause, l, l, depth)
	rows, err := queries.Raw(db, rsql).Query()
	if err != nil {
		return nil, NewInternalError(err)
	}
	defer rows.Close()

	// Iterate rows, build tree
	sources := make(map[int64]*Source)
	roots := make([]*Source, 0)
	for rows.Next() {
		// Scan source
		s := new(Source)
		var typeID, d int64
		err := rows.Scan(&s.ID, &s.UID, &s.ParentID, &s.Position, &typeID, &s.Name, &s.Description, &s.Year, &s.Number, &d)
		if err != nil {
			return nil, NewInternalError(err)
		}
		s.Type = mdb.SOURCE_TYPE_REGISTRY.ByID[typeID].Name

		// Attach source to tree
		sources[s.ID] = s
		if s.ParentID.Valid {
			p, ok := sources[s.ParentID.Int64]
			if ok {
				if p.Children == nil {
					p.Children = make([]*Source, 0)
				}
				p.Children = append(p.Children, s)
				s.ParentUID = p.UID
			} else {
				roots = append(roots, s)
			}
		} else {
			roots = append(roots, s)
		}
	}
	err = rows.Err()
	if err != nil {
		return nil, NewInternalError(err)
	}

	if r.RootUID == "" {
		rsql = fmt.Sprintf(AUTHORS_SOURCES_SQL, l, l)
		rows, err := queries.Raw(db, rsql).Query()
		if err != nil {
			return nil, NewInternalError(err)
		}
		defer rows.Close()

		authors := make([]*Author, 0)
		for rows.Next() {
			a := new(Author)
			var sids pq.Int64Array
			err := rows.Scan(&a.Code, &a.Name, &a.FullName, &sids)
			if err != nil {
				return nil, NewInternalError(err)
			}

			// Associate sources
			a.Children = make([]*Source, len(sids))
			for i, x := range sids {
				a.Children[i] = sources[x]
				a.Children[i].ParentUID = a.Code
			}
			authors = append(authors, a)
		}
		err = rows.Err()
		if err == nil {
			return authors, nil
		} else {
			return nil, NewInternalError(err)
		}
	}

	return roots, nil
}

func handleTags(db *sql.DB, r HierarchyRequest) (interface{}, *HttpError) {
	var l string
	if r.Language == "" {
		l = consts.LANG_HEBREW
	} else {
		l = r.Language
	}

	var depth int
	if r.Depth == 0 {
		depth = math.MaxInt8
	} else {
		depth = r.Depth
	}

	var rootClause string
	if r.RootUID == "" {
		rootClause = "parent_id IS NULL"
	} else {
		rootClause = fmt.Sprintf("uid = '%s'", r.RootUID)
	}

	// Execute query
	rsql := fmt.Sprintf(TAG_HIERARCHY_SQL, l, rootClause, l, depth)
	rows, err := queries.Raw(db, rsql).Query()
	if err != nil {
		return nil, NewInternalError(err)
	}
	defer rows.Close()

	// Iterate rows, build tree
	tags := make(map[int64]*Tag)
	roots := make([]*Tag, 0)
	for rows.Next() {
		// Scan tag
		t := new(Tag)
		var d int64
		err := rows.Scan(&t.ID, &t.UID, &t.ParentID, &t.Label, &d)
		if err != nil {
			return nil, NewInternalError(err)
		}

		// Attach tag to tree
		tags[t.ID] = t
		if t.ParentID.Valid {
			p, ok := tags[t.ParentID.Int64]
			if ok {
				if p.Children == nil {
					p.Children = make([]*Tag, 0)
				}
				p.Children = append(p.Children, t)
				t.ParentUID = p.UID
			} else {
				roots = append(roots, t)
			}
		} else {
			roots = append(roots, t)
		}
	}
	err = rows.Err()
	if err == nil {
		return roots, nil
	} else {
		return nil, NewInternalError(err)
	}
}

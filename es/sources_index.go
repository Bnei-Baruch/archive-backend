package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/Bnei-Baruch/sqlboiler/queries"
	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

func MakeSourcesIndex(namespace string, db *sql.DB, esc *elastic.Client) *SourcesIndex {
	si := new(SourcesIndex)
	si.baseName = consts.ES_SOURCES_INDEX
	si.namespace = namespace
	si.db = db
	si.esc = esc
	return si
}

type SourcesIndex struct {
	BaseIndex
}

func (index *SourcesIndex) ReindexAll() error {
	log.Infof("SourcesIndex.Reindex All.")
	if _, err := index.removeFromIndexQuery(elastic.NewMatchAllQuery()); err != nil {
		return err
	}
	return index.addToIndexSql("1=1") // SQL to always match any source
}

func (index *SourcesIndex) Update(scope Scope) error {
	log.Infof("SourcesIndex.Update - Scope: %+v.", scope)
	removed, err := index.removeFromIndex(scope)
	if err != nil {
		return err
	}
	return index.addToIndex(scope, removed)
}

func (index *SourcesIndex) addToIndex(scope Scope, removedUIDs []string) error {
	uids := removedUIDs
	if scope.SourceUID != "" {
		uids = append(uids, scope.SourceUID)
	}
	quoted := make([]string, len(uids))
	for i, uid := range uids {
		quoted[i] = fmt.Sprintf("'%s'", uid)
	}
	sqlScope := fmt.Sprintf("source.uid IN (%s)", strings.Join(quoted, ","))
	return index.addToIndexSql(sqlScope)
}

func (index *SourcesIndex) removeFromIndex(scope Scope) ([]string, error) {
	if scope.SourceUID != "" {
		return index.removeFromIndexQuery(elastic.NewTermsQuery("mdb_uid", scope.SourceUID))
	}

	// Nothing to remove.
	return []string{}, nil
}

// Note: scope usage is limited to source.uid only (e.g. source.uid='L2jMWyce')
func (index *SourcesIndex) addToIndexSql(sqlScope string) error {
	var count int64
	err := mdbmodels.NewQuery(index.db,
		qm.Select("COUNT(1)"),
		qm.From("sources as source"),
		qm.Where(sqlScope)).QueryRow().Scan(&count)
	if err != nil {
		return err
	}

	log.Infof("SourcesIndex.addToIndexSql - Sources Index - Adding %d sources. Scope: %s", count, sqlScope)

	// Loading all sources path in all languages.
	// codesMap from uid => codes
	// idsMap from uid => id
	// authorsByLanguageMap from uid => lang => authors
	codesMap, idsMap, authorsByLanguageMap, err := index.loadSources(index.db, sqlScope)
	if err != nil {
		return errors.Wrap(err, "SourcesIndex.addToIndexSql - Fetch sources parents from mdb.")
	}

	offset := 0
	limit := 1000
	for offset < int(count) {
		var sources []*mdbmodels.Source
		err := mdbmodels.NewQuery(index.db,
			qm.From("sources as source"),
			qm.Load("SourceI18ns"),
			qm.Load("Authors"),
			qm.Where(sqlScope),
			qm.Offset(offset),
			qm.Limit(limit)).Bind(&sources)
		if err != nil {
			return errors.Wrap(err, "SourcesIndex.addToIndexSql - Fetch sources from mdb")
		}

		log.Infof("SourcesIndex.addToIndexSql - Adding %d sources (offset: %d).", len(sources), offset)

		for _, source := range sources {
			if parents, ok := codesMap[source.UID]; !ok {
				log.Warnf("SourcesIndex.addToIndexSql - Source %s not found in codesMap: %+v", source.UID, codesMap)
			} else if parentIds, ok := idsMap[source.UID]; !ok {
				log.Warnf("SourcesIndex.addToIndexSql - Source %s not found in idsMap: %+v", source.UID, idsMap)
			} else if authors, ok := authorsByLanguageMap[source.UID]; !ok {
				log.Warnf("SourcesIndex.addToIndexSql - Source %s not found in authorsByLanguageMap: %+v", source.UID, idsMap)
			} else if err := index.indexSource(source, parents, parentIds, authors); err != nil {
				log.Warnf("SourcesIndex.addToIndexSql - Unable to index source '%s' (uid: %s). Error is: %v.", source.Name, source.UID, err)
			}
		}
		offset += limit
	}

	return nil
}

func (index *SourcesIndex) loadSources(db *sql.DB, sqlScope string) (map[string][]string, map[string][]int64, map[string]map[string][]string, error) {
	rows, err := queries.Raw(db, fmt.Sprintf(`
		WITH recursive rec_sources AS
        (
               SELECT s.id,
                      s.uid,
                      array [s.uid] "path",
                      array [s.id] :: bigint [] "idspath",
                      authors,
                      aun.language,
                      aun.author_names
               FROM   sources s
               JOIN
                      (
                               SELECT   source_id,
                                        array_agg(a.id),
                                        array_agg(a.code) AS authors
                               FROM     authors_sources aas
                               JOIN     authors a
                               ON       a.id = aas.author_id
                               GROUP BY source_id) au
               ON     au.source_id = s.id
               JOIN
                      (
                                SELECT    source_id,
                                          an.language,
                                          CASE
                                                    WHEN an.full_name IS NULL THEN array [an.name]
                                                    ELSE array [an.name, an.full_name]
                                          END AS author_names
                                FROM      authors_sources aas
                                JOIN      authors a
                                ON        a.id = aas.author_id
                                LEFT JOIN author_i18n an
                                ON        an.author_id=a.id ) aun
               ON     aun.source_id = s.id
               UNION
               SELECT     s.id,
                          s.uid,
                          (rs.path
                                     || s.uid) :: character(8)[],
                          (rs.idspath
                                     || s.id) :: bigint [],
                          rs.authors,
                          rs.language,
                          rs.author_names
               FROM       sources s
               INNER JOIN rec_sources rs
               ON         s.parent_id = rs.id )
        SELECT uid,
               array_cat(path, authors) "path",
               idspath,
               language,
               author_names
		FROM   rec_sources source 
		where %s;`, sqlScope)).Query()

	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "SourcesIndex.loadSources - Query failed.")
	}
	defer rows.Close()

	codeMap := make(map[string][]string)
	idMap := make(map[string][]int64)
	authorsByLanguageMap := make(map[string]map[string][]string)

	for rows.Next() {
		var uid string
		var lang string
		var authors pq.StringArray
		var codeValues pq.StringArray
		var idValues pq.Int64Array
		err := rows.Scan(&uid, &codeValues, &idValues, &lang, &authors)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "rows.Scan")
		}
		if authorsByLanguageMap[uid] == nil {
			authorsByLanguageMap[uid] = make(map[string][]string)
		}
		stringAuthors := make([]string, 0)
		for _, ns := range authors {
			stringAuthors = append(stringAuthors, ns)
		}
		stringCodeValues := make([]string, 0)
		for _, ns := range codeValues {
			stringCodeValues = append(stringCodeValues, ns)
		}
		authorsByLanguageMap[uid][lang] = stringAuthors
		if _, ok := codeMap[uid]; !ok {
			codeMap[uid] = stringCodeValues
			idMap[uid] = idValues
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, errors.Wrap(err, "SourcesIndex.loadSources - rows.Err()")
	}

	return codeMap, idMap, authorsByLanguageMap, nil
}

func (index *SourcesIndex) removeFromIndexQuery(elasticScope elastic.Query) ([]string, error) {
	source, err := elasticScope.Source()
	if err != nil {
		return []string{}, err
	}
	jsonBytes, err := json.Marshal(source)
	if err != nil {
		return []string{}, err
	}
	log.Infof("SourcesIndex.removeFromIndexQuery - Removing from index. Scope: %s", string(jsonBytes))
	removed := make(map[string]bool)
	for _, lang := range consts.ALL_KNOWN_LANGS {
		indexName := index.indexName(lang)
		searchRes, err := index.esc.Search(indexName).Query(elasticScope).Do(context.TODO())
		if err != nil {
			return []string{}, err
		}
		for _, h := range searchRes.Hits.Hits {
			var source Source
			err := json.Unmarshal(*h.Source, &source)
			if err != nil {
				return []string{}, err
			}
			removed[source.MDB_UID] = true
		}
		delRes, err := index.esc.DeleteByQuery(indexName).
			Query(elasticScope).
			Do(context.TODO())
		if err != nil {
			return []string{}, errors.Wrapf(err, "SourcesIndex.removeFromIndexQuery - Remove from index %s %+v\n", indexName, elasticScope)
		}
		if delRes.Deleted > 0 {
			fmt.Printf("SourcesIndex.removeFromIndexQuery - Deleted %d documents from %s.\n", delRes.Deleted, indexName)
		}
	}
	if len(removed) == 0 {
		fmt.Println("SourcesIndex.removeFromIndexQuery - Nothing was delete.")
		return []string{}, nil
	}
	keys := make([]string, 0)
	for k := range removed {
		keys = append(keys, k)
	}
	return keys, nil
}

func (index *SourcesIndex) getDocxPath(uid string, lang string) (string, error) {
	folder, err := SourcesFolder()
	if err != nil {
		return "", err
	}
	uidPath := path.Join(folder, uid)
	jsonPath := path.Join(uidPath, "index.json")
	jsonCnt, err := ioutil.ReadFile(jsonPath)
	if err != nil {
		return "", fmt.Errorf("SourcesIndex.getDocxPath - Unable to read from file %s. Error: %+v", jsonPath, err)
	}
	var m map[string]map[string]string
	err = json.Unmarshal(jsonCnt, &m)
	if err != nil {
		return "", err
	}
	if val, ok := m[lang]; ok {
		docxPath := path.Join(uidPath, val["docx"])
		if _, err := os.Stat(docxPath); err == nil {
			return path.Join(docxPath), nil
		}
	}
	return "", errors.New("SourcesIndex.getDocxPath - Docx not found in index.json.")
}

func (index *SourcesIndex) indexSource(mdbSource *mdbmodels.Source, parents []string, parentIds []int64, authorsByLanguage map[string][]string) error {
	// Create documents in each language with available translation
	hasDocxForSomeLanguage := false
	i18nMap := make(map[string]Source)
	for _, i18n := range mdbSource.R.SourceI18ns {
		if i18n.Name.Valid && i18n.Name.String != "" {
			source := Source{
				MDB_UID: mdbSource.UID,
				Name:    i18n.Name.String,
				Sources: parents,
			}
			if i18n.Description.Valid && i18n.Description.String != "" {
				source.Description = i18n.Description.String
			}
			fPath, err := index.getDocxPath(mdbSource.UID, i18n.Language)
			if err != nil {
				log.Warnf("SourcesIndex.indexSource - Unable to retrieving docx path for source %s with language %s.  Skipping indexing.", mdbSource.UID, i18n.Language)
			} else {
				// Found docx.
				content, err := ParseDocx(fPath)
				if err == nil {
					source.Content = content
					hasDocxForSomeLanguage = true
				} else {
					log.Warnf("SourcesIndex.indexSource - Error parsing docx for source %s and language %s. Skipping indexing.", mdbSource.UID, i18n.Language)
				}
			}
			for _, i := range parentIds {
				ni18n, err := mdbmodels.FindSourceI18n(index.db, i, i18n.Language)
				if err != nil {
					if err == sql.ErrNoRows {
						continue
					}
					return err
				}
				if ni18n.Name.Valid && ni18n.Name.String != "" {
					source.PathNames = append(source.PathNames, ni18n.Name.String)
				}
				if ni18n.Description.Valid && ni18n.Description.String != "" {
					source.PathNames = append(source.PathNames, ni18n.Description.String)
				}
			}
			source.Authors = authorsByLanguage[i18n.Language]
			source.FullName = append(source.Authors, source.PathNames...)
			i18nMap[i18n.Language] = source
		}
	}

	if !hasDocxForSomeLanguage {
		log.Warnf("SourcesIndex.indexSource - No docx files found for source %s", mdbSource.UID)
	}

	// Index each document in its language index
	for k, v := range i18nMap {
		name := index.indexName(k)
		log.Infof("Sources Index - Add source %s to index %s", mdbSource.UID, name)
		resp, err := index.esc.Index().
			Index(name).
			Type("sources").
			BodyJson(v).
			Do(context.TODO())
		if err != nil {
			return errors.Wrapf(err, "Sources Index - Source %s %s", name, mdbSource.UID)
		}
		if !resp.Created {
			return errors.Errorf("Sources Index - Not created: source %s %s", name, mdbSource.UID)
		}
	}

	return nil
}

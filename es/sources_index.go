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

		parentsMap, err := index.loadSources(index.db) // Note: getting all sources path, not by scope.
		if err != nil {
			return errors.Wrap(err, "SourcesIndex.addToIndexSql - Fetch sources parents from mdb")
		}

		log.Infof("SourcesIndex.addToIndexSql - Adding %d sources (offset: %d).", len(sources), offset)

		for _, source := range sources {
			if parents, ok := parentsMap[source.UID]; !ok {
				log.Warnf("SourcesIndex.addToIndexSql - Source %s not found in parentsMap: %+v", source.UID, parents)
			} else if err := index.indexSource(source, parents); err != nil {
				log.Warnf("SourcesIndex.addToIndexSql - Unable to index source '%s' (uid: %s). Error is: %v.", source.Name, source.UID, err)
			}
		}
		offset += limit
	}

	return nil
}

func (index *SourcesIndex) loadSources(db *sql.DB) (map[string][]string, error) {
	rows, err := queries.Raw(db, fmt.Sprintf(`
		WITH RECURSIVE rec_sources AS (
			SELECT
			  s.id,
			  s.uid,
			  s.position,
			  array_append(au.authors, s.uid) "path"
			FROM sources s JOIN
    
    		(select source_id,  array_agg(a.code) as authors 
    		from authors_sources aas 
				INNER JOIN authors a ON a.id = aas.author_id
   			 group by source_id ) au ON au.source_id = s.id
    
			UNION
			SELECT
			  s.id,
			  s.uid,
			  s.position,
			  rs.path || s.uid
			FROM sources s INNER JOIN rec_sources rs ON s.parent_id = rs.id
		  )
		  select uid, path from rec_sources;`)).Query()

	if err != nil {
		return nil, errors.Wrap(err, "SourcesIndex.loadSources - Query failed.")
	}
	defer rows.Close()

	m := make(map[string][]string)

	for rows.Next() {
		var uid string
		var values pq.StringArray
		err := rows.Scan(&uid, &values)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		m[uid] = values
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "SourcesIndex.loadSources - rows.Err()")
	}

	return m, nil
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
	err, folder := SourcesFolder()
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

func (index *SourcesIndex) indexSource(mdbSource *mdbmodels.Source, parentsMap []string) error {
	// Create documents in each language with available translation
	hasDocxForSomeLanguage := false
	i18nMap := make(map[string]Source)
	for _, i18n := range mdbSource.R.SourceI18ns {
		if i18n.Name.Valid && i18n.Name.String != "" {
			source := Source{
				MDB_UID: mdbSource.UID,
				Name:    i18n.Name.String,
				Sources: parentsMap,
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

			for _, a := range mdbSource.R.Authors {
				ai18n, err := mdbmodels.FindAuthorI18n(index.db, a.ID, i18n.Language)
				if err != nil {
					if err == sql.ErrNoRows {
						continue
					}
					return err
				}
				if ai18n.Name.Valid && ai18n.Name.String != "" {
					source.Authors = append(source.Authors, ai18n.Name.String)
				}
				if ai18n.FullName.Valid && ai18n.FullName.String != "" {
					source.Authors = append(source.Authors, ai18n.FullName.String)
				}
			}
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

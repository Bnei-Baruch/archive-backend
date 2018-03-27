package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/queries/qm"
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
	log.Infof("Sources Index - Reindex all.")
	if _, err := index.removeFromIndexQuery(elastic.NewMatchAllQuery()); err != nil {
		return err
	}
	return index.addToIndexSql("1=1")
}

func (index *SourcesIndex) Add(scope Scope) error {
	log.Infof("Sources Index - Add. Scope: %+v.", scope)
	// We only add sources when the scope is source, otherwise we need to update.
	if scope.SourceUID != "" {
		sqlScope := fmt.Sprintf("source.uid = '%s'", scope.SourceUID)
		if err := index.addToIndexSql(sqlScope); err != nil {
			return errors.Wrap(err, "Sources index addToIndexSql")
		}
		scope.SourceUID = ""
	}
	emptyScope := Scope{}
	if scope != emptyScope {
		return index.Update(scope)
	}
	return nil
}

func (index *SourcesIndex) Update(scope Scope) error {
	log.Infof("Sources Index - Update. Scope: %+v.", scope)
	removed, err := index.removeFromIndexQuery(elastic.NewTermsQuery("mdb_uid", scope.SourceUID))
	if err != nil {
		return err
	}
	if len(removed) > 0 && removed[0] == scope.SourceUID {
		sqlScope := fmt.Sprintf("source.uid = '%s'", scope.SourceUID)
		if err := index.addToIndexSql(sqlScope); err != nil {
			return errors.Wrap(err, "Sources index addToIndexSql")
		}
	}
	return nil
}

func (index *SourcesIndex) Delete(scope Scope) error {
	log.Infof("Sources Index - Delete. Scope: %+v.", scope)
	// We only delete sources when source is deleted, otherwise we just update.
	if scope.SourceUID != "" {
		if _, err := index.removeFromIndexQuery(elastic.NewTermsQuery("mdb_uid", scope.SourceUID)); err != nil {
			return err
		}
		scope.SourceUID = ""
	}
	emptyScope := Scope{}
	if scope != emptyScope {
		return index.Update(scope)
	}
	return nil
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

	log.Infof("Sources Index - Adding %d sources. Scope: %s", count, sqlScope)

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
			return errors.Wrap(err, "Fetch sources from mdb")
		}
		log.Infof("Adding %d sources (offset: %d).", len(sources), offset)

		for _, source := range sources {
			if err := index.indexSource(source); err != nil {
				return err
			}
		}
		offset += limit
	}

	return nil
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
	log.Infof("Sources Index - Removing from index. Scope: %s", string(jsonBytes))
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
			return []string{}, errors.Wrapf(err, "Remove from index %s %+v\n", indexName, elasticScope)
		}
		if delRes.Deleted > 0 {
			fmt.Printf("Deleted %d documents from %s.\n", delRes.Deleted, indexName)
		}
	}
	if len(removed) == 0 {
		fmt.Println("Sources Index - Nothing was delete.")
		return []string{}, nil
	}
	keys := make([]string, 0)
	for k := range removed {
		keys = append(keys, k)
	}
	return keys, nil
}

func (index *SourcesIndex) getDocxPath(uid string, lang string) (string, error) {
	uidPath := path.Join(SourcesFolder, uid)
	jsonPath := path.Join(uidPath, "index.json")
	jsonCnt, err := ioutil.ReadFile(jsonPath)
	if err != nil {
		return "", fmt.Errorf("Unable to read from file %s. Error: %+v", jsonPath, err)
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
	return "", errors.New("Docx not found in index.json")
}

func (index *SourcesIndex) indexSource(mdbSource *mdbmodels.Source) error {
	// Create documents in each language with available translation
	i18nMap := make(map[string]Source)
	for _, i18n := range mdbSource.R.SourceI18ns {
		if i18n.Name.Valid && i18n.Name.String != "" {

			source := Source{
				MDB_UID: mdbSource.UID,
				Name:    i18n.Name.String,
			}

			if i18n.Description.Valid && i18n.Description.String != "" {
				source.Description = i18n.Description.String
			}

			fPath, err := index.getDocxPath(mdbSource.UID, i18n.Language)
			if err != nil {
				log.Errorf("Error retrieving docx path for source %s and language %s", mdbSource.UID, i18n.Language)
				//return errors.Errorf("Error retrieving docx path for source %s and language %s", mdbSource.UID, i18n.Language)
			} else {
				content, err := index.ParseDocx(fPath)
				if err != nil {
					return errors.Errorf("Error parsing docx for source %s and language %s", mdbSource.UID, i18n.Language)
				}
				source.Content = content
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

	// Index each document in its language index
	for k, v := range i18nMap {
		name := index.indexName(k)
		vBytes, err := json.Marshal(v)
		if err != nil {
			return err
		}
		log.Infof("Sources Index - Add source %s to index %s", string(vBytes), name)
		resp, err := index.esc.Index().
			Index(name).
			Type("sources").
			BodyJson(v).
			Do(context.TODO())
		if err != nil {
			return errors.Wrapf(err, "Source %s %s", name, mdbSource.UID)
		}
		if !resp.Created {
			return errors.Errorf("Not created: source %s %s", name, mdbSource.UID)
		}
	}

	return nil
}

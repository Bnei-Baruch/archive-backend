package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

func MakeClassificationsIndex(namespace string, db *sql.DB, esc *elastic.Client) *ClassificationsIndex {
	ci := new(ClassificationsIndex)
	ci.baseName = consts.ES_CLASSIFICATIONS_INDEX
	ci.namespace = namespace
	ci.db = db
	ci.esc = esc
	return ci
}

type ClassificationsIndex struct {
	BaseIndex
}

func (index *ClassificationsIndex) ReindexAll() error {
	log.Info("Classifications Index - Reindexing all.")
	if err := index.removeFromIndexQuery(elastic.NewMatchAllQuery()); err != nil {
		return err
	}
	if err := index.addTagsToIndexSql("TRUE"); err != nil {
		return err
	}
	return index.addSourcesToIndexSql("TRUE")
}

func (index *ClassificationsIndex) Update(scope Scope) error {
	log.Infof("Classifications Index - Update. Scope: %+v.", scope)
	if err := index.removeFromIndex(scope); err != nil {
		return err
	}
	return index.addToIndex(scope)
}

func (index *ClassificationsIndex) removeFromIndex(scope Scope) error {
	log.Infof("Classifications Index - removeFromIndex. Scope: %+v.", scope)
	if scope.TagUID != "" {
		if err := index.removeFromIndexQuery(elastic.NewTermsQuery("mdb_uid", scope.TagUID)); err != nil {
			return err
		}
	}
	if scope.SourceUID != "" {
		if err := index.removeFromIndexQuery(elastic.NewTermsQuery("mdb_uid", scope.SourceUID)); err != nil {
			return err
		}
	}
	return nil
}

func (index *ClassificationsIndex) addToIndex(scope Scope) error {
	if scope.TagUID != "" {
		return index.addTagsToIndexSql(fmt.Sprintf("uid = '%s'", scope.TagUID))
	}
	if scope.SourceUID != "" {
		return index.addSourcesToIndexSql(fmt.Sprintf("uid = '%s'", scope.SourceUID))
	}
	return nil
}

func (index *ClassificationsIndex) addTagsToIndexSql(sqlScope string) error {
	tags, err := mdbmodels.Tags(index.db,
		qm.Load("TagI18ns"),
		qm.Where(sqlScope)).All()
	if err != nil {
		return errors.Wrap(err, "Classifications Index - Fetch tags from mdb.")
	}
	log.Infof("Classifications Index - Adding %d tags. Scope: %s.", len(tags), sqlScope)

	for _, tag := range tags {
		if !tag.ParentID.Valid {
			log.Infof("Classifications Index - Skipping root tag [%s].", tag.UID)
			continue
		}
		if err := index.indexTag(tag); err != nil {
			return err
		}
	}
	return nil
}

func (index *ClassificationsIndex) addSourcesToIndexSql(sqlScope string) error {
	sources, err := mdbmodels.Sources(index.db,
		qm.Load("SourceI18ns"),
		qm.Where(sqlScope)).All()
	if err != nil {
		return errors.Wrap(err, "Classifications Index - Fetch sources from mdb.")
	}
	log.Infof("Classifications Index - Adding %d sources. Scope: %s.", len(sources), sqlScope)

	for _, source := range sources {
		if err := index.indexSource(source); err != nil {
			return err
		}
	}
	return nil
}

func (index *ClassificationsIndex) removeFromIndexQuery(elasticScope elastic.Query) error {
	source, err := elasticScope.Source()
	if err != nil {
		return err
	}
	jsonBytes, err := json.Marshal(source)
	if err != nil {
		return err
	}
	log.Infof("Classifications Index - Removing from index. Scope: %s", string(jsonBytes))
	for _, lang := range consts.ALL_KNOWN_LANGS {
		indexName := index.indexName(lang)
		res, err := index.esc.DeleteByQuery(indexName).
			Query(elasticScope).
			Do(context.TODO())
		if err != nil {
			return errors.Wrapf(err, "Classifications Index - Remove from index %s %+v\n", indexName, elasticScope)
		}
		if res.Deleted > 0 {
			fmt.Printf("Classifications Index - Deleted %d documents from %s.\n", res.Deleted, indexName)
		}
		// If not exists Deleted will be 0.
		// if resp.Deleted != int64(len(uids)) {
		// 	return errors.Errorf("Not deleted: %s %+v\n", indexName, uids)
		// }
	}
	return nil
}

func (index *ClassificationsIndex) indexTag(t *mdbmodels.Tag) error {
	for i := range t.R.TagI18ns {
		i18n := t.R.TagI18ns[i]
		if i18n.Label.Valid && i18n.Label.String != "" {
			c := Classification{
				MDB_UID:     t.UID,
				Type:        "tag",
				Name:        i18n.Label.String,
				NameSuggest: i18n.Label.String,
			}
			name := index.indexName(i18n.Language)
			cBytes, err := json.Marshal(c)
			if err != nil {
				return err
			}
			log.Infof("Classifications Index - Add tag %s to index %s", string(cBytes), name)
			resp, err := index.esc.Index().
				Index(name).
				Type("tags").
				BodyJson(c).
				Do(context.TODO())
			if err != nil {
				return errors.Wrapf(err, "Classifications Index - Index tag %s %s", name, t.UID)
			}
			if !resp.Created {
				return errors.Errorf("Classifications Index - Not created: tag %s %s", name, t.UID)
			}
		}
	}
	return nil
}

func (index *ClassificationsIndex) indexSource(s *mdbmodels.Source) error {
	for i := range s.R.SourceI18ns {
		i18n := s.R.SourceI18ns[i]
		if i18n.Name.Valid && i18n.Name.String != "" {
			c := Classification{
				MDB_UID:     s.UID,
				Type:        "source",
				Name:        i18n.Name.String,
				NameSuggest: i18n.Name.String,
			}
			if i18n.Description.Valid && i18n.Description.String != "" {
				c.Description = i18n.Description.String
				c.DescriptionSuggest = i18n.Description.String
			}
			name := index.indexName(i18n.Language)
			cBytes, err := json.Marshal(c)
			if err != nil {
				return err
			}
			log.Infof("Classifications Index - Add source %s to index %s", string(cBytes), name)
			resp, err := index.esc.Index().
				Index(name).
				Type("sources").
				BodyJson(c).
				Do(context.TODO())
			if err != nil {
				return errors.Wrapf(err, "Classifications Index - Index source %s %s.", name, s.UID)
			}
			if !resp.Created {
				return errors.Errorf("Classifications Index - Not created: source %s %s.", name, s.UID)
			}
		}
	}
	return nil
}

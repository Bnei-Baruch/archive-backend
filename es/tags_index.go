package es

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

func MakeTagsIndex(namespace string, db *sql.DB, esc *elastic.Client) *TagsIndex {
	tagsIndex := new(TagsIndex)
	tagsIndex.baseName = consts.ES_RESULTS_INDEX
	tagsIndex.namespace = namespace
	tagsIndex.db = db
	tagsIndex.esc = esc
	return tagsIndex
}

type TagsIndex struct {
	BaseIndex
}

func (index *TagsIndex) ReindexAll() error {
	log.Info("Tags Index - Reindexing all.")
	if _, err := index.RemoveFromIndexQuery(index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_TAGS)); err != nil {
		return err
	}
	return index.addToIndexSql("TRUE")
}

func (index *TagsIndex) Update(scope Scope) error {
	log.Infof("Tags Index - Update. Scope: %+v.", scope)
	removed, err := index.removeFromIndex(scope)
	if err != nil {
		return err
	}
	return index.addToIndex(scope, removed)
}

func (index *TagsIndex) addToIndex(scope Scope, removedUIDs []string) error {
	uids := removedUIDs
	if scope.TagUID != "" {
		uids = append(uids, scope.TagUID)
	}
	if len(uids) == 0 {
		return nil
	}
	quoted := make([]string, len(uids))
	for i, uid := range uids {
		quoted[i] = fmt.Sprintf("'%s'", uid)
	}
	return index.addToIndexSql(fmt.Sprintf("uid IN (%s)", strings.Join(quoted, ",")))
}

func (index *TagsIndex) removeFromIndex(scope Scope) ([]string, error) {
	log.Infof("Tags Index - removeFromIndex. Scope: %+v.", scope)
	if scope.TagUID != "" {
		elasticScope := index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_UNITS).
			Filter(elastic.NewTermsQuery("typed_uids", keyValue("tag", scope.TagUID)))
		return index.RemoveFromIndexQuery(elasticScope)
	}
	// Nothing to remove.
	return []string{}, nil
}

func (index *TagsIndex) addToIndexSql(sqlScope string) error {
	tags, err := mdbmodels.Tags(index.db,
		qm.Load("TagI18ns"),
		qm.Where(sqlScope)).All()
	if err != nil {
		return errors.Wrap(err, "Tags Index - Fetch tags from mdb.")
	}
	log.Infof("Tags Index - Adding %d tags. Scope: %s.", len(tags), sqlScope)

	for _, tag := range tags {
		if !tag.ParentID.Valid {
			log.Infof("Tags Index - Skipping root tag [%s].", tag.UID)
			continue
		}
		if err := index.indexTag(tag); err != nil {
			return err
		}
	}
	return nil
}

func (index *TagsIndex) indexTag(t *mdbmodels.Tag) error {
	for i := range t.R.TagI18ns {
		i18n := t.R.TagI18ns[i]
		if i18n.Label.Valid && strings.TrimSpace(i18n.Label.String) != "" {
			r := Result{
				ResultType:   consts.ES_RESULT_TYPE_TAGS,
				MDB_UID:      t.UID,
				TypedUids:    []string{keyValue("tag", t.UID)},
				FilterValues: []string{keyValue("tag", t.UID)},
				Title:        i18n.Label.String,
				TitleSuggest: Suffixes(i18n.Label.String),
			}
			name := index.indexName(i18n.Language)
			log.Infof("Tags Index - Add tag %s to index %s", r.ToString(), name)
			resp, err := index.esc.Index().
				Index(name).
				Type("result").
				BodyJson(r).
				Do(context.TODO())
			if err != nil {
				return errors.Wrapf(err, "Tags Index - Index tag %s %s", name, t.UID)
			}
			if !resp.Created {
				return errors.Errorf("Tags Index - Not created: tag %s %s", name, t.UID)
			}
		}
	}
	return nil
}

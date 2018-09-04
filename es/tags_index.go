package es

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

func MakeTagsIndex(namespace string, indexDate string, db *sql.DB, esc *elastic.Client) *TagsIndex {
	tagsIndex := new(TagsIndex)
	tagsIndex.resultType = consts.ES_RESULT_TYPE_TAGS
	tagsIndex.baseName = consts.ES_RESULTS_INDEX
	tagsIndex.namespace = namespace
	tagsIndex.indexDate = indexDate
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
		elasticScope := index.FilterByResultTypeQuery(consts.ES_RESULT_TYPE_TAGS).
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
			parentTag := t
			parentI18n := i18n
			pathNames := []string{i18n.Label.String}
			parentUids := []string{t.UID}
			for parentTag.ParentID.Valid {
				var err error
				parentTag, err = mdbmodels.Tags(index.db,
					qm.Load("TagI18ns"),
					qm.Where(fmt.Sprintf("id = %d", parentTag.ParentID.Int64))).One()
				if err != nil {
					return err
				}
				found := false
				for _, pI18n := range parentTag.R.TagI18ns {
					if pI18n.Language == parentI18n.Language {
						parentI18n = pI18n
						found = true
					}
				}
				if !found || !parentI18n.Label.Valid {
                    log.Warnf("Tag I18n not found or invalid label. Tag UID: %s Label: %s Language: %s.", t.UID, i18n.Label.String, i18n.Language)
                    continue
				}
				pathNames = append([]string{parentI18n.Label.String}, pathNames...)
				parentUids = append([]string{parentTag.UID}, parentUids...)
			}

			r := Result{
				ResultType:   consts.ES_RESULT_TYPE_TAGS,
				MDB_UID:      t.UID,
				FilterValues: KeyValues("tag", parentUids),
				TypedUids:    []string{keyValue("tag", t.UID)},
				Title:        strings.Join(pathNames, " - "),
				TitleSuggest: Suffixes(strings.Join(pathNames, " ")),
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
			if resp.Result != "created" {
				return errors.Errorf("Tags Index - Not created: tag %s %s", name, t.UID)
			}
		}
	}
	return nil
}

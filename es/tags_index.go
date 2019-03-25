package es

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
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
	_, indexErrors := index.RemoveFromIndexQuery(index.FilterByResultTypeQuery(index.resultType))
	if err := indexErrors.CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "TagsIndex"); err != nil {
		return err
	}
	return indexErrors.Join(index.addToIndexSql("TRUE"), "").CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "TagsIndex")
}

func (index *TagsIndex) RemoveFromIndex(scope Scope) (map[string][]string, error) {
	log.Debugf("Tags Index - RemoveFromIndex. Scope: %+v.", scope)
	removed, indexErrors := index.removeFromIndex(scope)
	return removed, indexErrors.CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "TagsIndex")
}

func (index *TagsIndex) AddToIndex(scope Scope, removedUIDs []string) error {
	log.Debugf("Tags Index - AddToIndex. Scope: %+v, removedUIDs: %+v.", scope, removedUIDs)
	return index.addToIndex(scope, removedUIDs).CheckErrors(LANGUAGES_MAX_FAILURE, DOCUMENT_MAX_FAILIRE_RATIO, "TagsIndex")
}

func (index *TagsIndex) addToIndex(scope Scope, removedUIDs []string) *IndexErrors {
	uids := removedUIDs
	if scope.TagUID != "" {
		uids = append(uids, scope.TagUID)
	}
	if len(uids) == 0 {
		return MakeIndexErrors()
	}
	quoted := make([]string, len(uids))
	for i, uid := range uids {
		quoted[i] = fmt.Sprintf("'%s'", uid)
	}
	return index.addToIndexSql(fmt.Sprintf("uid IN (%s)", strings.Join(quoted, ",")))
}

func (index *TagsIndex) removeFromIndex(scope Scope) (map[string][]string, *IndexErrors) {
	log.Debugf("Tags Index - removeFromIndex. Scope: %+v.", scope)
	if scope.TagUID != "" {
		elasticScope := index.FilterByResultTypeQuery(index.resultType).
			Filter(elastic.NewTermsQuery("typed_uids", keyValue(consts.ES_UID_TYPE_TAG, scope.TagUID)))
		return index.RemoveFromIndexQuery(elasticScope)
	}
	// Nothing to remove.
	return make(map[string][]string), MakeIndexErrors()
}

func (index *TagsIndex) addToIndexSql(sqlScope string) *IndexErrors {
	tags, err := mdbmodels.Tags(index.db,
		qm.Load("TagI18ns"),
		qm.Where(sqlScope)).All()
	if err != nil {
		return MakeIndexErrors().SetError(err).Wrap("Tags Index - Fetch tags from mdb.")
	}
	log.Infof("Tags Index - Adding %d tags. Scope: %s.", len(tags), sqlScope)
	indexErrors := MakeIndexErrors()
	for _, tag := range tags {
		if !tag.ParentID.Valid {
			log.Debugf("Tags Index - Skipping root tag [%s].", tag.UID)
			continue
		}
		indexErrors.Join(index.indexTag(tag), "")
	}
	return indexErrors
}

func (index *TagsIndex) indexTag(t *mdbmodels.Tag) *IndexErrors {
	indexErrors := MakeIndexErrors()
	for i := range t.R.TagI18ns {
		i18n := t.R.TagI18ns[i]
		if i18n.Label.Valid && strings.TrimSpace(i18n.Label.String) != "" {
			parentTag := t
			parentI18n := i18n
			pathNames := []string{i18n.Label.String}
			parentUids := []string{t.UID}
			found := false
			errFetching := (error)(nil)
			for parentTag.ParentID.Valid {
				parentTag, errFetching = mdbmodels.Tags(index.db,
					qm.Load("TagI18ns"),
					qm.Where(fmt.Sprintf("id = %d", parentTag.ParentID.Int64))).One()
				if errFetching != nil {
					break
				}
				for _, pI18n := range parentTag.R.TagI18ns {
					if pI18n.Language == parentI18n.Language {
						parentI18n = pI18n
						found = true
					}
				}
				if !found || !parentI18n.Label.Valid {
					found = false
					break
				}
				pathNames = append([]string{parentI18n.Label.String}, pathNames...)
				parentUids = append([]string{parentTag.UID}, parentUids...)
			}
			if errFetching != nil {
				indexErrors.DocumentError(i18n.Language, errFetching, fmt.Sprintf("Tag I18n failed fetching tags. Tag UID: %s Label: %s Language: %s. Skipping language.", t.UID, i18n.Label.String, i18n.Language))
				continue
			}

			if !found {
				// Don't log this, this is very common.
				// log.Warnf("Tag I18n not found or invalid label. Tag UID: %s Label: %s Language: %s. Skipping language.", t.UID, i18n.Label.String, i18n.Language)
				continue
			}

			r := Result{
				ResultType:   index.resultType,
				MDB_UID:      t.UID,
				FilterValues: KeyValues(consts.ES_UID_TYPE_TAG, parentUids),
				TypedUids:    []string{keyValue(consts.ES_UID_TYPE_TAG, t.UID)},
				Title:        strings.Join(pathNames, " - "),
				TitleSuggest: Suffixes(strings.Join(pathNames, " ")),
			}
			name := index.IndexName(i18n.Language)
			log.Debugf("Tags Index - Add tag %s to index %s", r.ToDebugString(), name)
			resp, err := index.esc.Index().
				Index(name).
				Type("result").
				BodyJson(r).
				Do(context.TODO())
			if err != nil {
				indexErrors.DocumentError(i18n.Language, err, fmt.Sprintf("Tags Index - Index tag %s %s", name, t.UID))
			} else if resp.Result != "created" {
				indexErrors.DocumentError(i18n.Language, err, fmt.Sprintf("Tags Index - Not created: tag %s %s", name, t.UID))
			}
		}
	}
	return indexErrors
}

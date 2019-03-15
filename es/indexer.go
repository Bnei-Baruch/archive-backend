package es

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type Indexer struct {
	indices []Index
}

func MakeProdIndexer(date string, mdb *sql.DB, esc *elastic.Client) (*Indexer, error) {
	return MakeIndexer("prod", date, consts.ES_ALL_RESULT_TYPES, mdb, esc)
}

func MakeFakeIndexer(mdb *sql.DB, esc *elastic.Client) (*Indexer, error) {
	return MakeIndexer("fake", "fake-date", []string{}, mdb, esc)
}

// Receives namespace and list of indexes names.
func MakeIndexer(namespace string, date string, names []string, mdb *sql.DB, esc *elastic.Client) (*Indexer, error) {
	log.Infof("Indexer - Make indexer - %s - %s", namespace, strings.Join(names, ", "))
	indexer := new(Indexer)
	indexer.indices = make([]Index, len(names))
	for i, name := range names {
		if name == consts.ES_RESULT_TYPE_UNITS {
			indexer.indices[i] = MakeContentUnitsIndex(namespace, date, mdb, esc)
		} else if name == consts.ES_RESULT_TYPE_SOURCES {
			indexer.indices[i] = MakeSourcesIndex(namespace, date, mdb, esc)
		} else if name == consts.ES_RESULT_TYPE_TAGS {
			indexer.indices[i] = MakeTagsIndex(namespace, date, mdb, esc)
		} else if name == consts.ES_RESULT_TYPE_COLLECTIONS {
			indexer.indices[i] = MakeCollectionsIndex(namespace, date, mdb, esc)
		} else if name == consts.ES_RESULT_TYPE_BLOG_POSTS {
			indexer.indices[i] = MakeBlogIndex(namespace, date, mdb, esc)
		} else if name == consts.ES_RESULT_TYPE_TWEETS {
			indexer.indices[i] = MakeTweeterIndex(namespace, date, mdb, esc)
		} else {
			return nil, errors.New(fmt.Sprintf("MakeIndexer - Invalid index name: %+v", name))
		}
	}
	return indexer, nil
}

func ProdAliasedIndexDate(esc *elastic.Client) (error, string) {
	return aliasedIndexDate(esc, "prod", consts.ES_RESULTS_INDEX)
}

func aliasedIndexDate(esc *elastic.Client, namespace string, name string) (error, string) {
	aliasesService := elastic.NewAliasesService(esc)
	prevIndicesByAlias := make(map[string]string)
	aliasesRes, err := aliasesService.Do(context.TODO())
	if err != nil {
		return err, ""
	}
	for indexName, indexResult := range aliasesRes.Indices {
		matched, err := regexp.MatchString(IndexName(namespace, name, ".*", ".*"), indexName)
		if err != nil {
			return err, ""
		}
		if matched {
			if len(indexResult.Aliases) > 1 {
				return errors.New(fmt.Sprintf("Expected no more then one alias for %s, got %d", indexName, len(indexResult.Aliases))), ""
			}
			if len(indexResult.Aliases) == 1 {
				prevIndicesByAlias[indexResult.Aliases[0].AliasName] = indexName
			}
		}
	}

	date := ""
	indicesExist := false
	for _, lang := range consts.ALL_KNOWN_LANGS {
		alias := IndexAliasName(namespace, name, lang)
		prevIndex, ok := prevIndicesByAlias[alias]
		if ok {
			indicesExist = true
			parts := strings.Split(prevIndex, "_")
			if len(parts) != 4 {
				return errors.New(fmt.Sprintf("Expected 4 parts in index name %s, got %d.", prevIndex, len(parts))), ""
			}
			if date == "" {
				date = parts[len(parts)-1]
			}
			if date != parts[len(parts)-1] {
				return errors.New(fmt.Sprintf("Expected index date to be %s got %s at index %s", date, parts[len(parts)], prevIndex)), ""
			}
		} else {
			if indicesExist {
				log.Warnf("Indexer - Did not find index name for %s", alias)
			}
		}
	}

	if date == "" && indicesExist {
		return errors.New("At least one aliased index should have date specified."), ""
	}

	return nil, date
}

func SwitchProdAliasToCurrentIndex(date string, esc *elastic.Client) error {
	return SwitchAliasToCurrentIndex("prod", consts.ES_RESULTS_INDEX, date, esc)
}

func SwitchAliasToCurrentIndex(namespace string, name string, date string, esc *elastic.Client) error {
	err, prevDate := aliasedIndexDate(esc, namespace, name)
	if err != nil {
		return err
	}
	aliasService := elastic.NewAliasService(esc)
	for _, lang := range consts.ALL_KNOWN_LANGS {
		indexName := IndexName(namespace, name, lang, date)
		alias := IndexAliasName(namespace, name, lang)
		if prevDate != "" {
			prevIndex := IndexName(namespace, name, lang, prevDate)
			aliasService = aliasService.Remove(prevIndex, alias)
		}
		aliasService.Add(indexName, alias)
	}
	res, err := aliasService.Do(context.TODO())
	if err != nil || !res.Acknowledged {
		return errors.Wrap(err, "Failed due to error or Acknowledged is false.")
	}
	return nil
}

func (indexer *Indexer) ReindexAll() error {
	log.Info("Indexer - Re-Indexing everything")
	if err := indexer.CreateIndexes(); err != nil {
		return err
	}
	done := make(chan string)
	errs := make([]error, len(indexer.indices))
	for i := range indexer.indices {
		go func(i int) {
			errs[i] = indexer.indices[i].ReindexAll()
			done <- indexer.indices[i].ResultType()
		}(i)
	}
	for _ = range indexer.indices {
		name := <-done
		log.Infof("Finished: %s", name)
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) RefreshAll() error {
	log.Info("Indexer - Refresh (sync new indexed documents) all indices.")
	for _, index := range indexer.indices {
		index.RefreshIndex()
	}
	return nil
}

func (indexer *Indexer) CreateIndexes() error {
	log.Infof("Indexer - Create new indices in elastic: %+v", indexer.indices)
	for _, index := range indexer.indices {
		if err := index.CreateIndex(); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) DeleteIndexes() error {
	log.Info("Indexer - Delete indices from elastic.")
	for _, index := range indexer.indices {
		if err := index.DeleteIndex(); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) Update(scope Scope) error {
	err := (error)(nil)
	for _, index := range indexer.indices {
		err = utils.JoinErrors(err, index.Update(scope))
	}
	return err
}

// Set of MDB event handlers to incrementally change all indexes.
func (indexer *Indexer) CollectionUpdate(uid string) error {
	log.Infof("Indexer - Index collection upadate event: %s", uid)
	return indexer.Update(Scope{CollectionUID: uid})
}

func (indexer *Indexer) ContentUnitUpdate(uid string) error {
	log.Infof("Indexer - Index content unit update  event: %s", uid)
	return indexer.Update(Scope{ContentUnitUID: uid})
}

func (indexer *Indexer) FileUpdate(uid string) error {
	log.Infof("Indexer - Index file update event: %s", uid)
	return indexer.Update(Scope{FileUID: uid})
}

func (indexer *Indexer) SourceUpdate(uid string) error {
	log.Infof("Indexer - Index source update event: %s", uid)
	return indexer.Update(Scope{SourceUID: uid})
}

func (indexer *Indexer) TagUpdate(uid string) error {
	log.Infof("Indexer - Index tag update  event: %s", uid)
	return indexer.Update(Scope{TagUID: uid})
}

func (indexer *Indexer) PersonUpdate(uid string) error {
	log.Infof("Indexer - Index person update  event: %s", uid)
	return indexer.Update(Scope{PersonUID: uid})
}

func (indexer *Indexer) PublisherUpdate(uid string) error {
	log.Infof("Indexer - Index publisher update event: %s", uid)
	return indexer.Update(Scope{PublisherUID: uid})
}

func (indexer *Indexer) BlogPostUpdate(id string) error {
	log.Infof("Indexer - Index blog post update event: %v", id)
	return indexer.Update(Scope{BlogPostWPID: id})
}

func (indexer *Indexer) TweetUpdate(tid string) error {
	log.Infof("Indexer - Index tweet update event: %v", tid)
	return indexer.Update(Scope{TweetTID: tid})
}

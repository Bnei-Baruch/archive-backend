package es

import (
	"database/sql"
	"strings"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

type Indexer struct {
	indices []Index
}

func MakeProdIndexer(mdb *sql.DB, esc *elastic.Client) *Indexer {
	return MakeIndexer("prod", []string{
		consts.ES_CLASSIFICATIONS_INDEX,
		consts.ES_UNITS_INDEX,
		consts.ES_COLLECTIONS_INDEX}, mdb, esc)
}

func MakeFakeIndexer(mdb *sql.DB, esc *elastic.Client) *Indexer {
	return MakeIndexer("fake", []string{}, mdb, esc)
}

// Receives namespace and list of indexes names.
func MakeIndexer(namespace string, names []string, mdb *sql.DB, esc *elastic.Client) *Indexer {
	log.Infof("Indexer - Make indexer - %s - %s", namespace, strings.Join(names, ", "))
	indexer := new(Indexer)
	indexer.indices = make([]Index, len(names))
	for i, name := range names {
		if name == consts.ES_CLASSIFICATIONS_INDEX {
			indexer.indices[i] = MakeClassificationsIndex(namespace, mdb, esc)
		} else if name == consts.ES_UNITS_INDEX {
			indexer.indices[i] = MakeContentUnitsIndex(namespace, mdb, esc)
		} else if name == consts.ES_COLLECTIONS_INDEX {
			indexer.indices[i] = MakeCollectionsIndex(namespace, mdb, esc)
		} else if name == consts.ES_SOURCES_INDEX {
			indexer.indices[i] = MakeSourcesIndex(namespace, mdb, esc)
		}
	}
	return indexer
}

func (indexer *Indexer) ReindexAll() error {
	log.Info("Indexer - Re-Indexing everything")
    if err := indexer.CreateIndexes(); err != nil {
        return err
    }
	for _, index := range indexer.indices {
		if err := index.ReindexAll(); err != nil {
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
	log.Info("Indexer - Create new indices in elastic.")
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

// Set of MDB event handlers to incrementally change all indexes.
func (indexer *Indexer) CollectionUpdate(uid string) error {
	log.Infof("Indexer - Index collection upadate event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{CollectionUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) ContentUnitUpdate(uid string) error {
	log.Infof("Indexer - Index content unit update  event: %s", uid)
	for _, index := range indexer.indices {
		// TODO: Optimize update to update elastic and not delete and then
		// add. It might be a problem on bulk updates, i.e., of someone added
		// some kind of tag for 1000 documents.
		// In that case removeing and adding will be much slower then updating
		// existing documents in elastic.
		// Decicded to not optimize prematurly.
		if err := index.Update(Scope{ContentUnitUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) FileUpdate(uid string) error {
	log.Infof("Indexer - Index file update event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{FileUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) SourceUpdate(uid string) error {
	log.Infof("Indexer - Index source update event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{SourceUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) TagUpdate(uid string) error {
	log.Infof("Indexer - Index tag update  event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{TagUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) PersonUpdate(uid string) error {
	log.Infof("Indexer - Index person update  event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{PersonUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) PublisherUpdate(uid string) error {
	log.Infof("Indexer - Index publisher update event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{PublisherUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

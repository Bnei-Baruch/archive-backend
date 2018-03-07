package es

import (
    "strings"

	log "github.com/Sirupsen/logrus"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

type Indexer struct {
	indices []Index
}

func MakeProdIndexer() *Indexer {
	return MakeIndexer("prod", []string{
		consts.ES_CLASSIFICATIONS_INDEX,
		consts.ES_UNITS_INDEX,
		consts.ES_COLLECTIONS_INDEX})
}

func MakeFakeIndexer() *Indexer {
	return MakeIndexer("fake", []string{})
}

// Receives namespace and list of indexes names.
func MakeIndexer(namespace string, names []string) *Indexer {
    log.Infof("Indexer - Make indexer - %s - %s", namespace, strings.Join(names, ", "))
	indexer := new(Indexer)
	indexer.indices = make([]Index, len(names))
	for i, name := range names {
		if name == consts.ES_CLASSIFICATIONS_INDEX {
			indexer.indices[i] = MakeClassificationsIndex(namespace)
		} else if name == consts.ES_UNITS_INDEX {
			indexer.indices[i] = MakeContentUnitsIndex(namespace)
		} else if name == consts.ES_COLLECTIONS_INDEX {
			indexer.indices[i] = MakeCollectionsIndex(namespace)
		}
	}
	return indexer
}

func (indexer *Indexer) ReindexAll() error {
	log.Info("Indexer - Re-Indexing everything")
	for _, index := range indexer.indices {
		// TODO: Check if indexing things in parallel will make things faster?
		log.Info("Indexer - Deleting index.")
		if err := index.DeleteIndex(); err != nil {
			return err
		}
		log.Info("Indexer - Creating index.")
		if err := index.CreateIndex(); err != nil {
			return err
		}
		log.Info("Indexer - Reindexing")
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
func (indexer *Indexer) CollectionAdd(uid string) error {
    log.Info("Indexer - Index collection add event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Add(Scope{CollectionUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) CollectionUpdate(uid string) error {
    log.Info("Indexer - Index collection upadate event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{CollectionUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) CollectionDelete(uid string) error {
    log.Info("Indexer - Index collection delete event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Delete(Scope{CollectionUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) ContentUnitAdd(uid string) error {
    log.Info("Indexer - Index content unit add event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Add(Scope{ContentUnitUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) ContentUnitUpdate(uid string) error {
    log.Info("Indexer - Index content unit update  event: %s", uid)
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

func (indexer *Indexer) ContentUnitDelete(uid string) error {
    log.Info("Indexer - Index content unit delete event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Delete(Scope{ContentUnitUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) FileAdd(uid string) error {
    log.Info("Indexer - Index file add event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Add(Scope{FileUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) FileUpdate(uid string) error {
    log.Info("Indexer - Index file update event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{FileUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) FileDelete(uid string) error {
    log.Info("Indexer - Index file delete event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Delete(Scope{FileUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) SourceAdd(uid string) error {
    log.Info("Indexer - Index source add event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Add(Scope{SourceUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) SourceUpdate(uid string) error {
    log.Info("Indexer - Index source update event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{SourceUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) TagAdd(uid string) error {
    log.Info("Indexer - Index tag add event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Add(Scope{TagUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) TagUpdate(uid string) error {
    log.Info("Indexer - Index tag update  event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{TagUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) PersonAdd(uid string) error {
    log.Info("Indexer - Index person add event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Add(Scope{PersonUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) PersonUpdate(uid string) error {
    log.Info("Indexer - Index person update  event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{PersonUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) PersonDelete(uid string) error {
    log.Info("Indexer - Index person delete event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Delete(Scope{PersonUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) PublisherAdd(uid string) error {
    log.Info("Indexer - Index publisher add event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Add(Scope{PublisherUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) PublisherUpdate(uid string) error {
    log.Info("Indexer - Index publisher update event: %s", uid)
	for _, index := range indexer.indices {
		if err := index.Update(Scope{PublisherUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

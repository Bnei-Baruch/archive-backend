package es

import (
	"github.com/Bnei-Baruch/archive-backend/consts"
)

type Indexer struct {
	indices []Index
}

func MakeProdIndexer() *Indexer {
	return MakeIndexer("prod", []string{consts.ES_CLASSIFICATIONS_INDEX, consts.ES_UNITS_INDEX})
}

// Receives namespace and list of indexes names.
func MakeIndexer(namespace string, names []string) *Indexer {
	indexer := new(Indexer)
	indexer.indices = make([]Index, len(names))
	for i, name := range names {
		if name == consts.ES_CLASSIFICATIONS_INDEX {
			// indexer.indices[i] = MakeCollectionsIndex(namespace)
		} else if name == consts.ES_UNITS_INDEX {
			indexer.indices[i] = MakeContentUnitsIndex(namespace)
		}
	}
	return indexer
}

func (indexer *Indexer) ReindexAll() error {
	for _, index := range indexer.indices {
		// TODO: Check if indexing things in parallel will make things faster?
		if err := index.DeleteIndex(); err != nil {
			return err
		}
		if err := index.CreateIndex(); err != nil {
			return err
		}
		if err := index.ReindexAll(); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) RefreshAll() error {
	for _, index := range indexer.indices {
		index.RefreshIndex()
	}
	return nil
}

func (indexer *Indexer) CreateIndexes() error {
	for _, index := range indexer.indices {
		if err := index.CreateIndex(); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) DeleteIndexes() error {
	for _, index := range indexer.indices {
		if err := index.DeleteIndex(); err != nil {
			return err
		}
	}
	return nil
}

// Set of MDB event handlers to incrementally change all indexes.
func (indexer *Indexer) CollectionAdd(uid string) error {
	return nil
}

func (indexer *Indexer) CollectionUpdate(uid string) error {
	return nil
}

func (indexer *Indexer) CollectionDelete(uid string) error {
	return nil
}

func (indexer *Indexer) ContentUnitAdd(uid string) error {
	for _, index := range indexer.indices {
		if err := index.AddToIndex(Scope{ContentUnitUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) ContentUnitUpdate(uid string) error {
	for _, index := range indexer.indices {
		if err := index.RemoveFromIndex(Scope{ContentUnitUID: uid}); err != nil {
			return err
		}
		if err := index.AddToIndex(Scope{ContentUnitUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) ContentUnitDelete(uid string) error {
	for _, index := range indexer.indices {
		if err := index.RemoveFromIndex(Scope{ContentUnitUID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (indexer *Indexer) FileAdd(uid string) error {
	return nil
}

func (indexer *Indexer) FileUpdate(uid string) error {
	return nil
}

func (indexer *Indexer) FileDelete(uid string) error {
	return nil
}

func (indexer *Indexer) SourceAdd(uid string) error {
	return nil
}

func (indexer *Indexer) SourceUpdate(uid string) error {
	return nil
}

func (indexer *Indexer) TagAdd(uid string) error {
	return nil
}

func (indexer *Indexer) TagUpdate(uid string) error {
	return nil
}

func (indexer *Indexer) PersonAdd(uid string) error {
	return nil
}

func (indexer *Indexer) PersonUpdate(uid string) error {
	return nil
}

func (indexer *Indexer) PublisherAdd(uid string) error {
	return nil
}

func (indexer *Indexer) PublisherUpdate(uid string) error {
	return nil
}

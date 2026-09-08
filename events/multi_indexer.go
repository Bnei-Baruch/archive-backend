package events

// MultiEventIndexer fans each MDB-change event to several EventIndexers. Used as an
// intermediate migration step: run ES9 (primary/served) while also keeping ES6 fresh,
// so a rollback to ES6 needs no catch-up reindex. Returns the last error encountered.
type MultiEventIndexer []EventIndexer

func (m MultiEventIndexer) each(f func(EventIndexer) error) error {
	var result error
	for _, i := range m {
		if err := f(i); err != nil {
			result = err
		}
	}
	return result
}

func (m MultiEventIndexer) CollectionUpdate(uid string) error {
	return m.each(func(i EventIndexer) error { return i.CollectionUpdate(uid) })
}
func (m MultiEventIndexer) ContentUnitUpdate(uid string) error {
	return m.each(func(i EventIndexer) error { return i.ContentUnitUpdate(uid) })
}
func (m MultiEventIndexer) FileUpdate(uid string) error {
	return m.each(func(i EventIndexer) error { return i.FileUpdate(uid) })
}
func (m MultiEventIndexer) SourceUpdate(uid string) error {
	return m.each(func(i EventIndexer) error { return i.SourceUpdate(uid) })
}
func (m MultiEventIndexer) TagUpdate(uid string) error {
	return m.each(func(i EventIndexer) error { return i.TagUpdate(uid) })
}
func (m MultiEventIndexer) PersonUpdate(uid string) error {
	return m.each(func(i EventIndexer) error { return i.PersonUpdate(uid) })
}
func (m MultiEventIndexer) PublisherUpdate(uid string) error {
	return m.each(func(i EventIndexer) error { return i.PublisherUpdate(uid) })
}
func (m MultiEventIndexer) BlogPostUpdate(id string) error {
	return m.each(func(i EventIndexer) error { return i.BlogPostUpdate(id) })
}
func (m MultiEventIndexer) TweetUpdate(tid string) error {
	return m.each(func(i EventIndexer) error { return i.TweetUpdate(tid) })
}

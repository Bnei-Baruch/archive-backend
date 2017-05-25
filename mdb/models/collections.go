package mdbmodels

import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vattle/sqlboiler/boil"
	"github.com/vattle/sqlboiler/queries"
	"github.com/vattle/sqlboiler/queries/qm"
	"github.com/vattle/sqlboiler/strmangle"
	"gopkg.in/nullbio/null.v6"
)

// Collection is an object representing the database table.
type Collection struct {
	ID         int64     `boil:"id" json:"id" toml:"id" yaml:"id"`
	UID        string    `boil:"uid" json:"uid" toml:"uid" yaml:"uid"`
	TypeID     int64     `boil:"type_id" json:"type_id" toml:"type_id" yaml:"type_id"`
	CreatedAt  time.Time `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	Properties null.JSON `boil:"properties" json:"properties,omitempty" toml:"properties" yaml:"properties,omitempty"`
	Secure     int16     `boil:"secure" json:"secure" toml:"secure" yaml:"secure"`
	Published  bool      `boil:"published" json:"published" toml:"published" yaml:"published"`

	R *collectionR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L collectionL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// collectionR is where relationships are stored.
type collectionR struct {
	Type                    *ContentType
	CollectionsContentUnits CollectionsContentUnitSlice
	CollectionI18ns         CollectionI18nSlice
}

// collectionL is where Load methods for each relationship are stored.
type collectionL struct{}

var (
	collectionColumns               = []string{"id", "uid", "type_id", "created_at", "properties", "secure", "published"}
	collectionColumnsWithoutDefault = []string{"uid", "type_id", "properties"}
	collectionColumnsWithDefault    = []string{"id", "created_at", "secure", "published"}
	collectionPrimaryKeyColumns     = []string{"id"}
)

type (
	// CollectionSlice is an alias for a slice of pointers to Collection.
	// This should generally be used opposed to []Collection.
	CollectionSlice []*Collection

	collectionQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	collectionType                 = reflect.TypeOf(&Collection{})
	collectionMapping              = queries.MakeStructMapping(collectionType)
	collectionPrimaryKeyMapping, _ = queries.BindMapping(collectionType, collectionMapping, collectionPrimaryKeyColumns)
	collectionInsertCacheMut       sync.RWMutex
	collectionInsertCache          = make(map[string]insertCache)
	collectionUpdateCacheMut       sync.RWMutex
	collectionUpdateCache          = make(map[string]updateCache)
	collectionUpsertCacheMut       sync.RWMutex
	collectionUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single collection record from the query, and panics on error.
func (q collectionQuery) OneP() *Collection {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single collection record from the query.
func (q collectionQuery) One() (*Collection, error) {
	o := &Collection{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: failed to execute a one query for collections")
	}

	return o, nil
}

// AllP returns all Collection records from the query, and panics on error.
func (q collectionQuery) AllP() CollectionSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all Collection records from the query.
func (q collectionQuery) All() (CollectionSlice, error) {
	var o CollectionSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "mdbmodels: failed to assign all query results to Collection slice")
	}

	return o, nil
}

// CountP returns the count of all Collection records in the query, and panics on error.
func (q collectionQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all Collection records in the query.
func (q collectionQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to count collections rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q collectionQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q collectionQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: failed to check if collections exists")
	}

	return count > 0, nil
}

// TypeG pointed to by the foreign key.
func (o *Collection) TypeG(mods ...qm.QueryMod) contentTypeQuery {
	return o.Type(boil.GetDB(), mods...)
}

// Type pointed to by the foreign key.
func (o *Collection) Type(exec boil.Executor, mods ...qm.QueryMod) contentTypeQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.TypeID),
	}

	queryMods = append(queryMods, mods...)

	query := ContentTypes(exec, queryMods...)
	queries.SetFrom(query.Query, "\"content_types\"")

	return query
}

// CollectionsContentUnitsG retrieves all the collections_content_unit's collections content units.
func (o *Collection) CollectionsContentUnitsG(mods ...qm.QueryMod) collectionsContentUnitQuery {
	return o.CollectionsContentUnits(boil.GetDB(), mods...)
}

// CollectionsContentUnits retrieves all the collections_content_unit's collections content units with an executor.
func (o *Collection) CollectionsContentUnits(exec boil.Executor, mods ...qm.QueryMod) collectionsContentUnitQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"collection_id\"=?", o.ID),
	)

	query := CollectionsContentUnits(exec, queryMods...)
	queries.SetFrom(query.Query, "\"collections_content_units\" as \"a\"")
	return query
}

// CollectionI18nsG retrieves all the collection_i18n's collection i18n.
func (o *Collection) CollectionI18nsG(mods ...qm.QueryMod) collectionI18nQuery {
	return o.CollectionI18ns(boil.GetDB(), mods...)
}

// CollectionI18ns retrieves all the collection_i18n's collection i18n with an executor.
func (o *Collection) CollectionI18ns(exec boil.Executor, mods ...qm.QueryMod) collectionI18nQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"collection_id\"=?", o.ID),
	)

	query := CollectionI18ns(exec, queryMods...)
	queries.SetFrom(query.Query, "\"collection_i18n\" as \"a\"")
	return query
}

// LoadType allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (collectionL) LoadType(e boil.Executor, singular bool, maybeCollection interface{}) error {
	var slice []*Collection
	var object *Collection

	count := 1
	if singular {
		object = maybeCollection.(*Collection)
	} else {
		slice = *maybeCollection.(*CollectionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &collectionR{}
		}
		args[0] = object.TypeID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &collectionR{}
			}
			args[i] = obj.TypeID
		}
	}

	query := fmt.Sprintf(
		"select * from \"content_types\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load ContentType")
	}
	defer results.Close()

	var resultSlice []*ContentType
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice ContentType")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Type = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.TypeID == foreign.ID {
				local.R.Type = foreign
				break
			}
		}
	}

	return nil
}

// LoadCollectionsContentUnits allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (collectionL) LoadCollectionsContentUnits(e boil.Executor, singular bool, maybeCollection interface{}) error {
	var slice []*Collection
	var object *Collection

	count := 1
	if singular {
		object = maybeCollection.(*Collection)
	} else {
		slice = *maybeCollection.(*CollectionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &collectionR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &collectionR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"collections_content_units\" where \"collection_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load collections_content_units")
	}
	defer results.Close()

	var resultSlice []*CollectionsContentUnit
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice collections_content_units")
	}

	if singular {
		object.R.CollectionsContentUnits = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.CollectionID {
				local.R.CollectionsContentUnits = append(local.R.CollectionsContentUnits, foreign)
				break
			}
		}
	}

	return nil
}

// LoadCollectionI18ns allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (collectionL) LoadCollectionI18ns(e boil.Executor, singular bool, maybeCollection interface{}) error {
	var slice []*Collection
	var object *Collection

	count := 1
	if singular {
		object = maybeCollection.(*Collection)
	} else {
		slice = *maybeCollection.(*CollectionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &collectionR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &collectionR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"collection_i18n\" where \"collection_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load collection_i18n")
	}
	defer results.Close()

	var resultSlice []*CollectionI18n
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice collection_i18n")
	}

	if singular {
		object.R.CollectionI18ns = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.CollectionID {
				local.R.CollectionI18ns = append(local.R.CollectionI18ns, foreign)
				break
			}
		}
	}

	return nil
}

// SetTypeG of the collection to the related item.
// Sets o.R.Type to related.
// Adds o to related.R.TypeCollections.
// Uses the global database handle.
func (o *Collection) SetTypeG(insert bool, related *ContentType) error {
	return o.SetType(boil.GetDB(), insert, related)
}

// SetTypeP of the collection to the related item.
// Sets o.R.Type to related.
// Adds o to related.R.TypeCollections.
// Panics on error.
func (o *Collection) SetTypeP(exec boil.Executor, insert bool, related *ContentType) {
	if err := o.SetType(exec, insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetTypeGP of the collection to the related item.
// Sets o.R.Type to related.
// Adds o to related.R.TypeCollections.
// Uses the global database handle and panics on error.
func (o *Collection) SetTypeGP(insert bool, related *ContentType) {
	if err := o.SetType(boil.GetDB(), insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetType of the collection to the related item.
// Sets o.R.Type to related.
// Adds o to related.R.TypeCollections.
func (o *Collection) SetType(exec boil.Executor, insert bool, related *ContentType) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"collections\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"type_id"}),
		strmangle.WhereClause("\"", "\"", 2, collectionPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.TypeID = related.ID

	if o.R == nil {
		o.R = &collectionR{
			Type: related,
		}
	} else {
		o.R.Type = related
	}

	if related.R == nil {
		related.R = &contentTypeR{
			TypeCollections: CollectionSlice{o},
		}
	} else {
		related.R.TypeCollections = append(related.R.TypeCollections, o)
	}

	return nil
}

// AddCollectionsContentUnitsG adds the given related objects to the existing relationships
// of the collection, optionally inserting them as new records.
// Appends related to o.R.CollectionsContentUnits.
// Sets related.R.Collection appropriately.
// Uses the global database handle.
func (o *Collection) AddCollectionsContentUnitsG(insert bool, related ...*CollectionsContentUnit) error {
	return o.AddCollectionsContentUnits(boil.GetDB(), insert, related...)
}

// AddCollectionsContentUnitsP adds the given related objects to the existing relationships
// of the collection, optionally inserting them as new records.
// Appends related to o.R.CollectionsContentUnits.
// Sets related.R.Collection appropriately.
// Panics on error.
func (o *Collection) AddCollectionsContentUnitsP(exec boil.Executor, insert bool, related ...*CollectionsContentUnit) {
	if err := o.AddCollectionsContentUnits(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddCollectionsContentUnitsGP adds the given related objects to the existing relationships
// of the collection, optionally inserting them as new records.
// Appends related to o.R.CollectionsContentUnits.
// Sets related.R.Collection appropriately.
// Uses the global database handle and panics on error.
func (o *Collection) AddCollectionsContentUnitsGP(insert bool, related ...*CollectionsContentUnit) {
	if err := o.AddCollectionsContentUnits(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddCollectionsContentUnits adds the given related objects to the existing relationships
// of the collection, optionally inserting them as new records.
// Appends related to o.R.CollectionsContentUnits.
// Sets related.R.Collection appropriately.
func (o *Collection) AddCollectionsContentUnits(exec boil.Executor, insert bool, related ...*CollectionsContentUnit) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.CollectionID = o.ID
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"collections_content_units\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"collection_id"}),
				strmangle.WhereClause("\"", "\"", 2, collectionsContentUnitPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.CollectionID, rel.ContentUnitID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.CollectionID = o.ID
		}
	}

	if o.R == nil {
		o.R = &collectionR{
			CollectionsContentUnits: related,
		}
	} else {
		o.R.CollectionsContentUnits = append(o.R.CollectionsContentUnits, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &collectionsContentUnitR{
				Collection: o,
			}
		} else {
			rel.R.Collection = o
		}
	}
	return nil
}

// AddCollectionI18nsG adds the given related objects to the existing relationships
// of the collection, optionally inserting them as new records.
// Appends related to o.R.CollectionI18ns.
// Sets related.R.Collection appropriately.
// Uses the global database handle.
func (o *Collection) AddCollectionI18nsG(insert bool, related ...*CollectionI18n) error {
	return o.AddCollectionI18ns(boil.GetDB(), insert, related...)
}

// AddCollectionI18nsP adds the given related objects to the existing relationships
// of the collection, optionally inserting them as new records.
// Appends related to o.R.CollectionI18ns.
// Sets related.R.Collection appropriately.
// Panics on error.
func (o *Collection) AddCollectionI18nsP(exec boil.Executor, insert bool, related ...*CollectionI18n) {
	if err := o.AddCollectionI18ns(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddCollectionI18nsGP adds the given related objects to the existing relationships
// of the collection, optionally inserting them as new records.
// Appends related to o.R.CollectionI18ns.
// Sets related.R.Collection appropriately.
// Uses the global database handle and panics on error.
func (o *Collection) AddCollectionI18nsGP(insert bool, related ...*CollectionI18n) {
	if err := o.AddCollectionI18ns(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddCollectionI18ns adds the given related objects to the existing relationships
// of the collection, optionally inserting them as new records.
// Appends related to o.R.CollectionI18ns.
// Sets related.R.Collection appropriately.
func (o *Collection) AddCollectionI18ns(exec boil.Executor, insert bool, related ...*CollectionI18n) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.CollectionID = o.ID
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"collection_i18n\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"collection_id"}),
				strmangle.WhereClause("\"", "\"", 2, collectionI18nPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.CollectionID, rel.Language}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.CollectionID = o.ID
		}
	}

	if o.R == nil {
		o.R = &collectionR{
			CollectionI18ns: related,
		}
	} else {
		o.R.CollectionI18ns = append(o.R.CollectionI18ns, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &collectionI18nR{
				Collection: o,
			}
		} else {
			rel.R.Collection = o
		}
	}
	return nil
}

// CollectionsG retrieves all records.
func CollectionsG(mods ...qm.QueryMod) collectionQuery {
	return Collections(boil.GetDB(), mods...)
}

// Collections retrieves all the records using an executor.
func Collections(exec boil.Executor, mods ...qm.QueryMod) collectionQuery {
	mods = append(mods, qm.From("\"collections\""))
	return collectionQuery{NewQuery(exec, mods...)}
}

// FindCollectionG retrieves a single record by ID.
func FindCollectionG(id int64, selectCols ...string) (*Collection, error) {
	return FindCollection(boil.GetDB(), id, selectCols...)
}

// FindCollectionGP retrieves a single record by ID, and panics on error.
func FindCollectionGP(id int64, selectCols ...string) *Collection {
	retobj, err := FindCollection(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindCollection retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindCollection(exec boil.Executor, id int64, selectCols ...string) (*Collection, error) {
	collectionObj := &Collection{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"collections\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(collectionObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: unable to select from collections")
	}

	return collectionObj, nil
}

// FindCollectionP retrieves a single record by ID with an executor, and panics on error.
func FindCollectionP(exec boil.Executor, id int64, selectCols ...string) *Collection {
	retobj, err := FindCollection(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *Collection) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *Collection) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *Collection) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *Collection) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no collections provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(collectionColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	collectionInsertCacheMut.RLock()
	cache, cached := collectionInsertCache[key]
	collectionInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			collectionColumns,
			collectionColumnsWithDefault,
			collectionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(collectionType, collectionMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(collectionType, collectionMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"collections\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

		if len(cache.retMapping) != 0 {
			cache.query += fmt.Sprintf(" RETURNING \"%s\"", strings.Join(returnColumns, "\",\""))
		}
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRow(cache.query, vals...).Scan(queries.PtrsFromMapping(value, cache.retMapping)...)
	} else {
		_, err = exec.Exec(cache.query, vals...)
	}

	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to insert into collections")
	}

	if !cached {
		collectionInsertCacheMut.Lock()
		collectionInsertCache[key] = cache
		collectionInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single Collection record. See Update for
// whitelist behavior description.
func (o *Collection) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single Collection record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *Collection) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the Collection, and panics on error.
// See Update for whitelist behavior description.
func (o *Collection) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the Collection.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *Collection) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	collectionUpdateCacheMut.RLock()
	cache, cached := collectionUpdateCache[key]
	collectionUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(collectionColumns, collectionPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("mdbmodels: unable to update collections, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"collections\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, collectionPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(collectionType, collectionMapping, append(wl, collectionPrimaryKeyColumns...))
		if err != nil {
			return err
		}
	}

	values := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err = exec.Exec(cache.query, values...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update collections row")
	}

	if !cached {
		collectionUpdateCacheMut.Lock()
		collectionUpdateCache[key] = cache
		collectionUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q collectionQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q collectionQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all for collections")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o CollectionSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o CollectionSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o CollectionSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o CollectionSlice) UpdateAll(exec boil.Executor, cols M) error {
	ln := int64(len(o))
	if ln == 0 {
		return nil
	}

	if len(cols) == 0 {
		return errors.New("mdbmodels: update all requires at least one column argument")
	}

	colNames := make([]string, len(cols))
	args := make([]interface{}, len(cols))

	i := 0
	for name, value := range cols {
		colNames[i] = name
		args[i] = value
		i++
	}

	// Append all of the primary key values for each column
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), collectionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"collections\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(collectionPrimaryKeyColumns), len(colNames)+1, len(collectionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all in collection slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *Collection) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *Collection) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *Collection) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *Collection) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no collections provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(collectionColumnsWithDefault, o)

	// Build cache key in-line uglily - mysql vs postgres problems
	buf := strmangle.GetBuffer()
	if updateOnConflict {
		buf.WriteByte('t')
	} else {
		buf.WriteByte('f')
	}
	buf.WriteByte('.')
	for _, c := range conflictColumns {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range updateColumns {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range whitelist {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range nzDefaults {
		buf.WriteString(c)
	}
	key := buf.String()
	strmangle.PutBuffer(buf)

	collectionUpsertCacheMut.RLock()
	cache, cached := collectionUpsertCache[key]
	collectionUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			collectionColumns,
			collectionColumnsWithDefault,
			collectionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			collectionColumns,
			collectionPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("mdbmodels: unable to upsert collections, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(collectionPrimaryKeyColumns))
			copy(conflict, collectionPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"collections\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(collectionType, collectionMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(collectionType, collectionMapping, ret)
			if err != nil {
				return err
			}
		}
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)
	var returns []interface{}
	if len(cache.retMapping) != 0 {
		returns = queries.PtrsFromMapping(value, cache.retMapping)
	}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRow(cache.query, vals...).Scan(returns...)
		if err == sql.ErrNoRows {
			err = nil // Postgres doesn't return anything when there's no update
		}
	} else {
		_, err = exec.Exec(cache.query, vals...)
	}
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to upsert collections")
	}

	if !cached {
		collectionUpsertCacheMut.Lock()
		collectionUpsertCache[key] = cache
		collectionUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single Collection record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Collection) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single Collection record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *Collection) DeleteG() error {
	if o == nil {
		return errors.New("mdbmodels: no Collection provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single Collection record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Collection) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single Collection record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Collection) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no Collection provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), collectionPrimaryKeyMapping)
	sql := "DELETE FROM \"collections\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete from collections")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q collectionQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q collectionQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("mdbmodels: no collectionQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from collections")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o CollectionSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o CollectionSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("mdbmodels: no Collection slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o CollectionSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o CollectionSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no Collection slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), collectionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"collections\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, collectionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(collectionPrimaryKeyColumns), 1, len(collectionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from collection slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *Collection) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *Collection) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *Collection) ReloadG() error {
	if o == nil {
		return errors.New("mdbmodels: no Collection provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Collection) Reload(exec boil.Executor) error {
	ret, err := FindCollection(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *CollectionSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *CollectionSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *CollectionSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("mdbmodels: empty CollectionSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *CollectionSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	collections := CollectionSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), collectionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"collections\".* FROM \"collections\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, collectionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(collectionPrimaryKeyColumns), 1, len(collectionPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&collections)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to reload all in CollectionSlice")
	}

	*o = collections

	return nil
}

// CollectionExists checks if the Collection row exists.
func CollectionExists(exec boil.Executor, id int64) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"collections\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: unable to check if collections exists")
	}

	return exists, nil
}

// CollectionExistsG checks if the Collection row exists.
func CollectionExistsG(id int64) (bool, error) {
	return CollectionExists(boil.GetDB(), id)
}

// CollectionExistsGP checks if the Collection row exists. Panics on error.
func CollectionExistsGP(id int64) bool {
	e, err := CollectionExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// CollectionExistsP checks if the Collection row exists. Panics on error.
func CollectionExistsP(exec boil.Executor, id int64) bool {
	e, err := CollectionExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

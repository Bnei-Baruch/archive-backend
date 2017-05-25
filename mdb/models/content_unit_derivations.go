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
)

// ContentUnitDerivation is an object representing the database table.
type ContentUnitDerivation struct {
	SourceID  int64  `boil:"source_id" json:"source_id" toml:"source_id" yaml:"source_id"`
	DerivedID int64  `boil:"derived_id" json:"derived_id" toml:"derived_id" yaml:"derived_id"`
	Name      string `boil:"name" json:"name" toml:"name" yaml:"name"`

	R *contentUnitDerivationR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L contentUnitDerivationL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// contentUnitDerivationR is where relationships are stored.
type contentUnitDerivationR struct {
	Derived *ContentUnit
	Source  *ContentUnit
}

// contentUnitDerivationL is where Load methods for each relationship are stored.
type contentUnitDerivationL struct{}

var (
	contentUnitDerivationColumns               = []string{"source_id", "derived_id", "name"}
	contentUnitDerivationColumnsWithoutDefault = []string{"source_id", "derived_id", "name"}
	contentUnitDerivationColumnsWithDefault    = []string{}
	contentUnitDerivationPrimaryKeyColumns     = []string{"source_id", "derived_id"}
)

type (
	// ContentUnitDerivationSlice is an alias for a slice of pointers to ContentUnitDerivation.
	// This should generally be used opposed to []ContentUnitDerivation.
	ContentUnitDerivationSlice []*ContentUnitDerivation

	contentUnitDerivationQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	contentUnitDerivationType                 = reflect.TypeOf(&ContentUnitDerivation{})
	contentUnitDerivationMapping              = queries.MakeStructMapping(contentUnitDerivationType)
	contentUnitDerivationPrimaryKeyMapping, _ = queries.BindMapping(contentUnitDerivationType, contentUnitDerivationMapping, contentUnitDerivationPrimaryKeyColumns)
	contentUnitDerivationInsertCacheMut       sync.RWMutex
	contentUnitDerivationInsertCache          = make(map[string]insertCache)
	contentUnitDerivationUpdateCacheMut       sync.RWMutex
	contentUnitDerivationUpdateCache          = make(map[string]updateCache)
	contentUnitDerivationUpsertCacheMut       sync.RWMutex
	contentUnitDerivationUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single contentUnitDerivation record from the query, and panics on error.
func (q contentUnitDerivationQuery) OneP() *ContentUnitDerivation {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single contentUnitDerivation record from the query.
func (q contentUnitDerivationQuery) One() (*ContentUnitDerivation, error) {
	o := &ContentUnitDerivation{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: failed to execute a one query for content_unit_derivations")
	}

	return o, nil
}

// AllP returns all ContentUnitDerivation records from the query, and panics on error.
func (q contentUnitDerivationQuery) AllP() ContentUnitDerivationSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all ContentUnitDerivation records from the query.
func (q contentUnitDerivationQuery) All() (ContentUnitDerivationSlice, error) {
	var o ContentUnitDerivationSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "mdbmodels: failed to assign all query results to ContentUnitDerivation slice")
	}

	return o, nil
}

// CountP returns the count of all ContentUnitDerivation records in the query, and panics on error.
func (q contentUnitDerivationQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all ContentUnitDerivation records in the query.
func (q contentUnitDerivationQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to count content_unit_derivations rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q contentUnitDerivationQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q contentUnitDerivationQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: failed to check if content_unit_derivations exists")
	}

	return count > 0, nil
}

// DerivedG pointed to by the foreign key.
func (o *ContentUnitDerivation) DerivedG(mods ...qm.QueryMod) contentUnitQuery {
	return o.Derived(boil.GetDB(), mods...)
}

// Derived pointed to by the foreign key.
func (o *ContentUnitDerivation) Derived(exec boil.Executor, mods ...qm.QueryMod) contentUnitQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.DerivedID),
	}

	queryMods = append(queryMods, mods...)

	query := ContentUnits(exec, queryMods...)
	queries.SetFrom(query.Query, "\"content_units\"")

	return query
}

// SourceG pointed to by the foreign key.
func (o *ContentUnitDerivation) SourceG(mods ...qm.QueryMod) contentUnitQuery {
	return o.Source(boil.GetDB(), mods...)
}

// Source pointed to by the foreign key.
func (o *ContentUnitDerivation) Source(exec boil.Executor, mods ...qm.QueryMod) contentUnitQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.SourceID),
	}

	queryMods = append(queryMods, mods...)

	query := ContentUnits(exec, queryMods...)
	queries.SetFrom(query.Query, "\"content_units\"")

	return query
}

// LoadDerived allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (contentUnitDerivationL) LoadDerived(e boil.Executor, singular bool, maybeContentUnitDerivation interface{}) error {
	var slice []*ContentUnitDerivation
	var object *ContentUnitDerivation

	count := 1
	if singular {
		object = maybeContentUnitDerivation.(*ContentUnitDerivation)
	} else {
		slice = *maybeContentUnitDerivation.(*ContentUnitDerivationSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &contentUnitDerivationR{}
		}
		args[0] = object.DerivedID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &contentUnitDerivationR{}
			}
			args[i] = obj.DerivedID
		}
	}

	query := fmt.Sprintf(
		"select * from \"content_units\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load ContentUnit")
	}
	defer results.Close()

	var resultSlice []*ContentUnit
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice ContentUnit")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Derived = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.DerivedID == foreign.ID {
				local.R.Derived = foreign
				break
			}
		}
	}

	return nil
}

// LoadSource allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (contentUnitDerivationL) LoadSource(e boil.Executor, singular bool, maybeContentUnitDerivation interface{}) error {
	var slice []*ContentUnitDerivation
	var object *ContentUnitDerivation

	count := 1
	if singular {
		object = maybeContentUnitDerivation.(*ContentUnitDerivation)
	} else {
		slice = *maybeContentUnitDerivation.(*ContentUnitDerivationSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &contentUnitDerivationR{}
		}
		args[0] = object.SourceID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &contentUnitDerivationR{}
			}
			args[i] = obj.SourceID
		}
	}

	query := fmt.Sprintf(
		"select * from \"content_units\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load ContentUnit")
	}
	defer results.Close()

	var resultSlice []*ContentUnit
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice ContentUnit")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Source = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.SourceID == foreign.ID {
				local.R.Source = foreign
				break
			}
		}
	}

	return nil
}

// SetDerivedG of the content_unit_derivation to the related item.
// Sets o.R.Derived to related.
// Adds o to related.R.DerivedContentUnitDerivations.
// Uses the global database handle.
func (o *ContentUnitDerivation) SetDerivedG(insert bool, related *ContentUnit) error {
	return o.SetDerived(boil.GetDB(), insert, related)
}

// SetDerivedP of the content_unit_derivation to the related item.
// Sets o.R.Derived to related.
// Adds o to related.R.DerivedContentUnitDerivations.
// Panics on error.
func (o *ContentUnitDerivation) SetDerivedP(exec boil.Executor, insert bool, related *ContentUnit) {
	if err := o.SetDerived(exec, insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetDerivedGP of the content_unit_derivation to the related item.
// Sets o.R.Derived to related.
// Adds o to related.R.DerivedContentUnitDerivations.
// Uses the global database handle and panics on error.
func (o *ContentUnitDerivation) SetDerivedGP(insert bool, related *ContentUnit) {
	if err := o.SetDerived(boil.GetDB(), insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetDerived of the content_unit_derivation to the related item.
// Sets o.R.Derived to related.
// Adds o to related.R.DerivedContentUnitDerivations.
func (o *ContentUnitDerivation) SetDerived(exec boil.Executor, insert bool, related *ContentUnit) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"content_unit_derivations\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"derived_id"}),
		strmangle.WhereClause("\"", "\"", 2, contentUnitDerivationPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.SourceID, o.DerivedID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.DerivedID = related.ID

	if o.R == nil {
		o.R = &contentUnitDerivationR{
			Derived: related,
		}
	} else {
		o.R.Derived = related
	}

	if related.R == nil {
		related.R = &contentUnitR{
			DerivedContentUnitDerivations: ContentUnitDerivationSlice{o},
		}
	} else {
		related.R.DerivedContentUnitDerivations = append(related.R.DerivedContentUnitDerivations, o)
	}

	return nil
}

// SetSourceG of the content_unit_derivation to the related item.
// Sets o.R.Source to related.
// Adds o to related.R.SourceContentUnitDerivations.
// Uses the global database handle.
func (o *ContentUnitDerivation) SetSourceG(insert bool, related *ContentUnit) error {
	return o.SetSource(boil.GetDB(), insert, related)
}

// SetSourceP of the content_unit_derivation to the related item.
// Sets o.R.Source to related.
// Adds o to related.R.SourceContentUnitDerivations.
// Panics on error.
func (o *ContentUnitDerivation) SetSourceP(exec boil.Executor, insert bool, related *ContentUnit) {
	if err := o.SetSource(exec, insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetSourceGP of the content_unit_derivation to the related item.
// Sets o.R.Source to related.
// Adds o to related.R.SourceContentUnitDerivations.
// Uses the global database handle and panics on error.
func (o *ContentUnitDerivation) SetSourceGP(insert bool, related *ContentUnit) {
	if err := o.SetSource(boil.GetDB(), insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetSource of the content_unit_derivation to the related item.
// Sets o.R.Source to related.
// Adds o to related.R.SourceContentUnitDerivations.
func (o *ContentUnitDerivation) SetSource(exec boil.Executor, insert bool, related *ContentUnit) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"content_unit_derivations\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"source_id"}),
		strmangle.WhereClause("\"", "\"", 2, contentUnitDerivationPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.SourceID, o.DerivedID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.SourceID = related.ID

	if o.R == nil {
		o.R = &contentUnitDerivationR{
			Source: related,
		}
	} else {
		o.R.Source = related
	}

	if related.R == nil {
		related.R = &contentUnitR{
			SourceContentUnitDerivations: ContentUnitDerivationSlice{o},
		}
	} else {
		related.R.SourceContentUnitDerivations = append(related.R.SourceContentUnitDerivations, o)
	}

	return nil
}

// ContentUnitDerivationsG retrieves all records.
func ContentUnitDerivationsG(mods ...qm.QueryMod) contentUnitDerivationQuery {
	return ContentUnitDerivations(boil.GetDB(), mods...)
}

// ContentUnitDerivations retrieves all the records using an executor.
func ContentUnitDerivations(exec boil.Executor, mods ...qm.QueryMod) contentUnitDerivationQuery {
	mods = append(mods, qm.From("\"content_unit_derivations\""))
	return contentUnitDerivationQuery{NewQuery(exec, mods...)}
}

// FindContentUnitDerivationG retrieves a single record by ID.
func FindContentUnitDerivationG(sourceID int64, derivedID int64, selectCols ...string) (*ContentUnitDerivation, error) {
	return FindContentUnitDerivation(boil.GetDB(), sourceID, derivedID, selectCols...)
}

// FindContentUnitDerivationGP retrieves a single record by ID, and panics on error.
func FindContentUnitDerivationGP(sourceID int64, derivedID int64, selectCols ...string) *ContentUnitDerivation {
	retobj, err := FindContentUnitDerivation(boil.GetDB(), sourceID, derivedID, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindContentUnitDerivation retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindContentUnitDerivation(exec boil.Executor, sourceID int64, derivedID int64, selectCols ...string) (*ContentUnitDerivation, error) {
	contentUnitDerivationObj := &ContentUnitDerivation{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"content_unit_derivations\" where \"source_id\"=$1 AND \"derived_id\"=$2", sel,
	)

	q := queries.Raw(exec, query, sourceID, derivedID)

	err := q.Bind(contentUnitDerivationObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: unable to select from content_unit_derivations")
	}

	return contentUnitDerivationObj, nil
}

// FindContentUnitDerivationP retrieves a single record by ID with an executor, and panics on error.
func FindContentUnitDerivationP(exec boil.Executor, sourceID int64, derivedID int64, selectCols ...string) *ContentUnitDerivation {
	retobj, err := FindContentUnitDerivation(exec, sourceID, derivedID, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *ContentUnitDerivation) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *ContentUnitDerivation) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *ContentUnitDerivation) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *ContentUnitDerivation) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no content_unit_derivations provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(contentUnitDerivationColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	contentUnitDerivationInsertCacheMut.RLock()
	cache, cached := contentUnitDerivationInsertCache[key]
	contentUnitDerivationInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			contentUnitDerivationColumns,
			contentUnitDerivationColumnsWithDefault,
			contentUnitDerivationColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(contentUnitDerivationType, contentUnitDerivationMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(contentUnitDerivationType, contentUnitDerivationMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"content_unit_derivations\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "mdbmodels: unable to insert into content_unit_derivations")
	}

	if !cached {
		contentUnitDerivationInsertCacheMut.Lock()
		contentUnitDerivationInsertCache[key] = cache
		contentUnitDerivationInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single ContentUnitDerivation record. See Update for
// whitelist behavior description.
func (o *ContentUnitDerivation) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single ContentUnitDerivation record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *ContentUnitDerivation) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the ContentUnitDerivation, and panics on error.
// See Update for whitelist behavior description.
func (o *ContentUnitDerivation) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the ContentUnitDerivation.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *ContentUnitDerivation) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	contentUnitDerivationUpdateCacheMut.RLock()
	cache, cached := contentUnitDerivationUpdateCache[key]
	contentUnitDerivationUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(contentUnitDerivationColumns, contentUnitDerivationPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("mdbmodels: unable to update content_unit_derivations, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"content_unit_derivations\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, contentUnitDerivationPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(contentUnitDerivationType, contentUnitDerivationMapping, append(wl, contentUnitDerivationPrimaryKeyColumns...))
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
		return errors.Wrap(err, "mdbmodels: unable to update content_unit_derivations row")
	}

	if !cached {
		contentUnitDerivationUpdateCacheMut.Lock()
		contentUnitDerivationUpdateCache[key] = cache
		contentUnitDerivationUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q contentUnitDerivationQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q contentUnitDerivationQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all for content_unit_derivations")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ContentUnitDerivationSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o ContentUnitDerivationSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o ContentUnitDerivationSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ContentUnitDerivationSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentUnitDerivationPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"content_unit_derivations\" SET %s WHERE (\"source_id\",\"derived_id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(contentUnitDerivationPrimaryKeyColumns), len(colNames)+1, len(contentUnitDerivationPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all in contentUnitDerivation slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *ContentUnitDerivation) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *ContentUnitDerivation) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *ContentUnitDerivation) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *ContentUnitDerivation) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no content_unit_derivations provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(contentUnitDerivationColumnsWithDefault, o)

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

	contentUnitDerivationUpsertCacheMut.RLock()
	cache, cached := contentUnitDerivationUpsertCache[key]
	contentUnitDerivationUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			contentUnitDerivationColumns,
			contentUnitDerivationColumnsWithDefault,
			contentUnitDerivationColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			contentUnitDerivationColumns,
			contentUnitDerivationPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("mdbmodels: unable to upsert content_unit_derivations, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(contentUnitDerivationPrimaryKeyColumns))
			copy(conflict, contentUnitDerivationPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"content_unit_derivations\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(contentUnitDerivationType, contentUnitDerivationMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(contentUnitDerivationType, contentUnitDerivationMapping, ret)
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
		return errors.Wrap(err, "mdbmodels: unable to upsert content_unit_derivations")
	}

	if !cached {
		contentUnitDerivationUpsertCacheMut.Lock()
		contentUnitDerivationUpsertCache[key] = cache
		contentUnitDerivationUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single ContentUnitDerivation record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContentUnitDerivation) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single ContentUnitDerivation record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *ContentUnitDerivation) DeleteG() error {
	if o == nil {
		return errors.New("mdbmodels: no ContentUnitDerivation provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single ContentUnitDerivation record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContentUnitDerivation) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single ContentUnitDerivation record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *ContentUnitDerivation) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no ContentUnitDerivation provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), contentUnitDerivationPrimaryKeyMapping)
	sql := "DELETE FROM \"content_unit_derivations\" WHERE \"source_id\"=$1 AND \"derived_id\"=$2"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete from content_unit_derivations")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q contentUnitDerivationQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q contentUnitDerivationQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("mdbmodels: no contentUnitDerivationQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from content_unit_derivations")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o ContentUnitDerivationSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o ContentUnitDerivationSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("mdbmodels: no ContentUnitDerivation slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o ContentUnitDerivationSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ContentUnitDerivationSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no ContentUnitDerivation slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentUnitDerivationPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"content_unit_derivations\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, contentUnitDerivationPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(contentUnitDerivationPrimaryKeyColumns), 1, len(contentUnitDerivationPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from contentUnitDerivation slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *ContentUnitDerivation) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *ContentUnitDerivation) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *ContentUnitDerivation) ReloadG() error {
	if o == nil {
		return errors.New("mdbmodels: no ContentUnitDerivation provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *ContentUnitDerivation) Reload(exec boil.Executor) error {
	ret, err := FindContentUnitDerivation(exec, o.SourceID, o.DerivedID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContentUnitDerivationSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContentUnitDerivationSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContentUnitDerivationSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("mdbmodels: empty ContentUnitDerivationSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContentUnitDerivationSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	contentUnitDerivations := ContentUnitDerivationSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentUnitDerivationPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"content_unit_derivations\".* FROM \"content_unit_derivations\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, contentUnitDerivationPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(contentUnitDerivationPrimaryKeyColumns), 1, len(contentUnitDerivationPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&contentUnitDerivations)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to reload all in ContentUnitDerivationSlice")
	}

	*o = contentUnitDerivations

	return nil
}

// ContentUnitDerivationExists checks if the ContentUnitDerivation row exists.
func ContentUnitDerivationExists(exec boil.Executor, sourceID int64, derivedID int64) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"content_unit_derivations\" where \"source_id\"=$1 AND \"derived_id\"=$2 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, sourceID, derivedID)
	}

	row := exec.QueryRow(sql, sourceID, derivedID)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: unable to check if content_unit_derivations exists")
	}

	return exists, nil
}

// ContentUnitDerivationExistsG checks if the ContentUnitDerivation row exists.
func ContentUnitDerivationExistsG(sourceID int64, derivedID int64) (bool, error) {
	return ContentUnitDerivationExists(boil.GetDB(), sourceID, derivedID)
}

// ContentUnitDerivationExistsGP checks if the ContentUnitDerivation row exists. Panics on error.
func ContentUnitDerivationExistsGP(sourceID int64, derivedID int64) bool {
	e, err := ContentUnitDerivationExists(boil.GetDB(), sourceID, derivedID)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// ContentUnitDerivationExistsP checks if the ContentUnitDerivation row exists. Panics on error.
func ContentUnitDerivationExistsP(exec boil.Executor, sourceID int64, derivedID int64) bool {
	e, err := ContentUnitDerivationExists(exec, sourceID, derivedID)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

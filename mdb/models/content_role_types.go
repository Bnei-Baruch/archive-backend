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

// ContentRoleType is an object representing the database table.
type ContentRoleType struct {
	ID          int64       `boil:"id" json:"id" toml:"id" yaml:"id"`
	Name        string      `boil:"name" json:"name" toml:"name" yaml:"name"`
	Description null.String `boil:"description" json:"description,omitempty" toml:"description" yaml:"description,omitempty"`

	R *contentRoleTypeR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L contentRoleTypeL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// contentRoleTypeR is where relationships are stored.
type contentRoleTypeR struct {
	RoleContentUnitsPersons ContentUnitsPersonSlice
}

// contentRoleTypeL is where Load methods for each relationship are stored.
type contentRoleTypeL struct{}

var (
	contentRoleTypeColumns               = []string{"id", "name", "description"}
	contentRoleTypeColumnsWithoutDefault = []string{"name", "description"}
	contentRoleTypeColumnsWithDefault    = []string{"id"}
	contentRoleTypePrimaryKeyColumns     = []string{"id"}
)

type (
	// ContentRoleTypeSlice is an alias for a slice of pointers to ContentRoleType.
	// This should generally be used opposed to []ContentRoleType.
	ContentRoleTypeSlice []*ContentRoleType

	contentRoleTypeQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	contentRoleTypeType                 = reflect.TypeOf(&ContentRoleType{})
	contentRoleTypeMapping              = queries.MakeStructMapping(contentRoleTypeType)
	contentRoleTypePrimaryKeyMapping, _ = queries.BindMapping(contentRoleTypeType, contentRoleTypeMapping, contentRoleTypePrimaryKeyColumns)
	contentRoleTypeInsertCacheMut       sync.RWMutex
	contentRoleTypeInsertCache          = make(map[string]insertCache)
	contentRoleTypeUpdateCacheMut       sync.RWMutex
	contentRoleTypeUpdateCache          = make(map[string]updateCache)
	contentRoleTypeUpsertCacheMut       sync.RWMutex
	contentRoleTypeUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single contentRoleType record from the query, and panics on error.
func (q contentRoleTypeQuery) OneP() *ContentRoleType {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single contentRoleType record from the query.
func (q contentRoleTypeQuery) One() (*ContentRoleType, error) {
	o := &ContentRoleType{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: failed to execute a one query for content_role_types")
	}

	return o, nil
}

// AllP returns all ContentRoleType records from the query, and panics on error.
func (q contentRoleTypeQuery) AllP() ContentRoleTypeSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all ContentRoleType records from the query.
func (q contentRoleTypeQuery) All() (ContentRoleTypeSlice, error) {
	var o ContentRoleTypeSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "mdbmodels: failed to assign all query results to ContentRoleType slice")
	}

	return o, nil
}

// CountP returns the count of all ContentRoleType records in the query, and panics on error.
func (q contentRoleTypeQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all ContentRoleType records in the query.
func (q contentRoleTypeQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to count content_role_types rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q contentRoleTypeQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q contentRoleTypeQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: failed to check if content_role_types exists")
	}

	return count > 0, nil
}

// RoleContentUnitsPersonsG retrieves all the content_units_person's content units persons via role_id column.
func (o *ContentRoleType) RoleContentUnitsPersonsG(mods ...qm.QueryMod) contentUnitsPersonQuery {
	return o.RoleContentUnitsPersons(boil.GetDB(), mods...)
}

// RoleContentUnitsPersons retrieves all the content_units_person's content units persons with an executor via role_id column.
func (o *ContentRoleType) RoleContentUnitsPersons(exec boil.Executor, mods ...qm.QueryMod) contentUnitsPersonQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"role_id\"=?", o.ID),
	)

	query := ContentUnitsPersons(exec, queryMods...)
	queries.SetFrom(query.Query, "\"content_units_persons\" as \"a\"")
	return query
}

// LoadRoleContentUnitsPersons allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (contentRoleTypeL) LoadRoleContentUnitsPersons(e boil.Executor, singular bool, maybeContentRoleType interface{}) error {
	var slice []*ContentRoleType
	var object *ContentRoleType

	count := 1
	if singular {
		object = maybeContentRoleType.(*ContentRoleType)
	} else {
		slice = *maybeContentRoleType.(*ContentRoleTypeSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &contentRoleTypeR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &contentRoleTypeR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"content_units_persons\" where \"role_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load content_units_persons")
	}
	defer results.Close()

	var resultSlice []*ContentUnitsPerson
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice content_units_persons")
	}

	if singular {
		object.R.RoleContentUnitsPersons = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.RoleID {
				local.R.RoleContentUnitsPersons = append(local.R.RoleContentUnitsPersons, foreign)
				break
			}
		}
	}

	return nil
}

// AddRoleContentUnitsPersonsG adds the given related objects to the existing relationships
// of the content_role_type, optionally inserting them as new records.
// Appends related to o.R.RoleContentUnitsPersons.
// Sets related.R.Role appropriately.
// Uses the global database handle.
func (o *ContentRoleType) AddRoleContentUnitsPersonsG(insert bool, related ...*ContentUnitsPerson) error {
	return o.AddRoleContentUnitsPersons(boil.GetDB(), insert, related...)
}

// AddRoleContentUnitsPersonsP adds the given related objects to the existing relationships
// of the content_role_type, optionally inserting them as new records.
// Appends related to o.R.RoleContentUnitsPersons.
// Sets related.R.Role appropriately.
// Panics on error.
func (o *ContentRoleType) AddRoleContentUnitsPersonsP(exec boil.Executor, insert bool, related ...*ContentUnitsPerson) {
	if err := o.AddRoleContentUnitsPersons(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddRoleContentUnitsPersonsGP adds the given related objects to the existing relationships
// of the content_role_type, optionally inserting them as new records.
// Appends related to o.R.RoleContentUnitsPersons.
// Sets related.R.Role appropriately.
// Uses the global database handle and panics on error.
func (o *ContentRoleType) AddRoleContentUnitsPersonsGP(insert bool, related ...*ContentUnitsPerson) {
	if err := o.AddRoleContentUnitsPersons(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddRoleContentUnitsPersons adds the given related objects to the existing relationships
// of the content_role_type, optionally inserting them as new records.
// Appends related to o.R.RoleContentUnitsPersons.
// Sets related.R.Role appropriately.
func (o *ContentRoleType) AddRoleContentUnitsPersons(exec boil.Executor, insert bool, related ...*ContentUnitsPerson) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.RoleID = o.ID
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"content_units_persons\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"role_id"}),
				strmangle.WhereClause("\"", "\"", 2, contentUnitsPersonPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ContentUnitID, rel.PersonID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.RoleID = o.ID
		}
	}

	if o.R == nil {
		o.R = &contentRoleTypeR{
			RoleContentUnitsPersons: related,
		}
	} else {
		o.R.RoleContentUnitsPersons = append(o.R.RoleContentUnitsPersons, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &contentUnitsPersonR{
				Role: o,
			}
		} else {
			rel.R.Role = o
		}
	}
	return nil
}

// ContentRoleTypesG retrieves all records.
func ContentRoleTypesG(mods ...qm.QueryMod) contentRoleTypeQuery {
	return ContentRoleTypes(boil.GetDB(), mods...)
}

// ContentRoleTypes retrieves all the records using an executor.
func ContentRoleTypes(exec boil.Executor, mods ...qm.QueryMod) contentRoleTypeQuery {
	mods = append(mods, qm.From("\"content_role_types\""))
	return contentRoleTypeQuery{NewQuery(exec, mods...)}
}

// FindContentRoleTypeG retrieves a single record by ID.
func FindContentRoleTypeG(id int64, selectCols ...string) (*ContentRoleType, error) {
	return FindContentRoleType(boil.GetDB(), id, selectCols...)
}

// FindContentRoleTypeGP retrieves a single record by ID, and panics on error.
func FindContentRoleTypeGP(id int64, selectCols ...string) *ContentRoleType {
	retobj, err := FindContentRoleType(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindContentRoleType retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindContentRoleType(exec boil.Executor, id int64, selectCols ...string) (*ContentRoleType, error) {
	contentRoleTypeObj := &ContentRoleType{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"content_role_types\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(contentRoleTypeObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: unable to select from content_role_types")
	}

	return contentRoleTypeObj, nil
}

// FindContentRoleTypeP retrieves a single record by ID with an executor, and panics on error.
func FindContentRoleTypeP(exec boil.Executor, id int64, selectCols ...string) *ContentRoleType {
	retobj, err := FindContentRoleType(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *ContentRoleType) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *ContentRoleType) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *ContentRoleType) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *ContentRoleType) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no content_role_types provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(contentRoleTypeColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	contentRoleTypeInsertCacheMut.RLock()
	cache, cached := contentRoleTypeInsertCache[key]
	contentRoleTypeInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			contentRoleTypeColumns,
			contentRoleTypeColumnsWithDefault,
			contentRoleTypeColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(contentRoleTypeType, contentRoleTypeMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(contentRoleTypeType, contentRoleTypeMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"content_role_types\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "mdbmodels: unable to insert into content_role_types")
	}

	if !cached {
		contentRoleTypeInsertCacheMut.Lock()
		contentRoleTypeInsertCache[key] = cache
		contentRoleTypeInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single ContentRoleType record. See Update for
// whitelist behavior description.
func (o *ContentRoleType) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single ContentRoleType record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *ContentRoleType) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the ContentRoleType, and panics on error.
// See Update for whitelist behavior description.
func (o *ContentRoleType) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the ContentRoleType.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *ContentRoleType) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	contentRoleTypeUpdateCacheMut.RLock()
	cache, cached := contentRoleTypeUpdateCache[key]
	contentRoleTypeUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(contentRoleTypeColumns, contentRoleTypePrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("mdbmodels: unable to update content_role_types, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"content_role_types\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, contentRoleTypePrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(contentRoleTypeType, contentRoleTypeMapping, append(wl, contentRoleTypePrimaryKeyColumns...))
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
		return errors.Wrap(err, "mdbmodels: unable to update content_role_types row")
	}

	if !cached {
		contentRoleTypeUpdateCacheMut.Lock()
		contentRoleTypeUpdateCache[key] = cache
		contentRoleTypeUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q contentRoleTypeQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q contentRoleTypeQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all for content_role_types")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ContentRoleTypeSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o ContentRoleTypeSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o ContentRoleTypeSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ContentRoleTypeSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentRoleTypePrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"content_role_types\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(contentRoleTypePrimaryKeyColumns), len(colNames)+1, len(contentRoleTypePrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all in contentRoleType slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *ContentRoleType) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *ContentRoleType) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *ContentRoleType) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *ContentRoleType) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no content_role_types provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(contentRoleTypeColumnsWithDefault, o)

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

	contentRoleTypeUpsertCacheMut.RLock()
	cache, cached := contentRoleTypeUpsertCache[key]
	contentRoleTypeUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			contentRoleTypeColumns,
			contentRoleTypeColumnsWithDefault,
			contentRoleTypeColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			contentRoleTypeColumns,
			contentRoleTypePrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("mdbmodels: unable to upsert content_role_types, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(contentRoleTypePrimaryKeyColumns))
			copy(conflict, contentRoleTypePrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"content_role_types\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(contentRoleTypeType, contentRoleTypeMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(contentRoleTypeType, contentRoleTypeMapping, ret)
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
		return errors.Wrap(err, "mdbmodels: unable to upsert content_role_types")
	}

	if !cached {
		contentRoleTypeUpsertCacheMut.Lock()
		contentRoleTypeUpsertCache[key] = cache
		contentRoleTypeUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single ContentRoleType record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContentRoleType) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single ContentRoleType record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *ContentRoleType) DeleteG() error {
	if o == nil {
		return errors.New("mdbmodels: no ContentRoleType provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single ContentRoleType record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContentRoleType) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single ContentRoleType record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *ContentRoleType) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no ContentRoleType provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), contentRoleTypePrimaryKeyMapping)
	sql := "DELETE FROM \"content_role_types\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete from content_role_types")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q contentRoleTypeQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q contentRoleTypeQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("mdbmodels: no contentRoleTypeQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from content_role_types")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o ContentRoleTypeSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o ContentRoleTypeSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("mdbmodels: no ContentRoleType slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o ContentRoleTypeSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ContentRoleTypeSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no ContentRoleType slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentRoleTypePrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"content_role_types\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, contentRoleTypePrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(contentRoleTypePrimaryKeyColumns), 1, len(contentRoleTypePrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from contentRoleType slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *ContentRoleType) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *ContentRoleType) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *ContentRoleType) ReloadG() error {
	if o == nil {
		return errors.New("mdbmodels: no ContentRoleType provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *ContentRoleType) Reload(exec boil.Executor) error {
	ret, err := FindContentRoleType(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContentRoleTypeSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContentRoleTypeSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContentRoleTypeSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("mdbmodels: empty ContentRoleTypeSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContentRoleTypeSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	contentRoleTypes := ContentRoleTypeSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentRoleTypePrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"content_role_types\".* FROM \"content_role_types\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, contentRoleTypePrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(contentRoleTypePrimaryKeyColumns), 1, len(contentRoleTypePrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&contentRoleTypes)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to reload all in ContentRoleTypeSlice")
	}

	*o = contentRoleTypes

	return nil
}

// ContentRoleTypeExists checks if the ContentRoleType row exists.
func ContentRoleTypeExists(exec boil.Executor, id int64) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"content_role_types\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: unable to check if content_role_types exists")
	}

	return exists, nil
}

// ContentRoleTypeExistsG checks if the ContentRoleType row exists.
func ContentRoleTypeExistsG(id int64) (bool, error) {
	return ContentRoleTypeExists(boil.GetDB(), id)
}

// ContentRoleTypeExistsGP checks if the ContentRoleType row exists. Panics on error.
func ContentRoleTypeExistsGP(id int64) bool {
	e, err := ContentRoleTypeExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// ContentRoleTypeExistsP checks if the ContentRoleType row exists. Panics on error.
func ContentRoleTypeExistsP(exec boil.Executor, id int64) bool {
	e, err := ContentRoleTypeExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

package kmedia

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

// ContentType is an object representing the database table.
type ContentType struct {
	ID      int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	Name    null.String `boil:"name" json:"name,omitempty" toml:"name" yaml:"name,omitempty"`
	Pattern null.String `boil:"pattern" json:"pattern,omitempty" toml:"pattern" yaml:"pattern,omitempty"`
	Secure  null.Int    `boil:"secure" json:"secure,omitempty" toml:"secure" yaml:"secure,omitempty"`

	R *contentTypeR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L contentTypeL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// contentTypeR is where relationships are stored.
type contentTypeR struct {
	Containers ContainerSlice
}

// contentTypeL is where Load methods for each relationship are stored.
type contentTypeL struct{}

var (
	contentTypeColumns               = []string{"id", "name", "pattern", "secure"}
	contentTypeColumnsWithoutDefault = []string{"name", "pattern"}
	contentTypeColumnsWithDefault    = []string{"id", "secure"}
	contentTypePrimaryKeyColumns     = []string{"id"}
)

type (
	// ContentTypeSlice is an alias for a slice of pointers to ContentType.
	// This should generally be used opposed to []ContentType.
	ContentTypeSlice []*ContentType

	contentTypeQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	contentTypeType                 = reflect.TypeOf(&ContentType{})
	contentTypeMapping              = queries.MakeStructMapping(contentTypeType)
	contentTypePrimaryKeyMapping, _ = queries.BindMapping(contentTypeType, contentTypeMapping, contentTypePrimaryKeyColumns)
	contentTypeInsertCacheMut       sync.RWMutex
	contentTypeInsertCache          = make(map[string]insertCache)
	contentTypeUpdateCacheMut       sync.RWMutex
	contentTypeUpdateCache          = make(map[string]updateCache)
	contentTypeUpsertCacheMut       sync.RWMutex
	contentTypeUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single contentType record from the query, and panics on error.
func (q contentTypeQuery) OneP() *ContentType {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single contentType record from the query.
func (q contentTypeQuery) One() (*ContentType, error) {
	o := &ContentType{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for content_types")
	}

	return o, nil
}

// AllP returns all ContentType records from the query, and panics on error.
func (q contentTypeQuery) AllP() ContentTypeSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all ContentType records from the query.
func (q contentTypeQuery) All() (ContentTypeSlice, error) {
	var o ContentTypeSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to ContentType slice")
	}

	return o, nil
}

// CountP returns the count of all ContentType records in the query, and panics on error.
func (q contentTypeQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all ContentType records in the query.
func (q contentTypeQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count content_types rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q contentTypeQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q contentTypeQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if content_types exists")
	}

	return count > 0, nil
}

// ContainersG retrieves all the container's containers.
func (o *ContentType) ContainersG(mods ...qm.QueryMod) containerQuery {
	return o.Containers(boil.GetDB(), mods...)
}

// Containers retrieves all the container's containers with an executor.
func (o *ContentType) Containers(exec boil.Executor, mods ...qm.QueryMod) containerQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"content_type_id\"=?", o.ID),
	)

	query := Containers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"containers\" as \"a\"")
	return query
}

// LoadContainers allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (contentTypeL) LoadContainers(e boil.Executor, singular bool, maybeContentType interface{}) error {
	var slice []*ContentType
	var object *ContentType

	count := 1
	if singular {
		object = maybeContentType.(*ContentType)
	} else {
		slice = *maybeContentType.(*ContentTypeSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &contentTypeR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &contentTypeR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"containers\" where \"content_type_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load containers")
	}
	defer results.Close()

	var resultSlice []*Container
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice containers")
	}

	if singular {
		object.R.Containers = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.ContentTypeID.Int {
				local.R.Containers = append(local.R.Containers, foreign)
				break
			}
		}
	}

	return nil
}

// AddContainers adds the given related objects to the existing relationships
// of the content_type, optionally inserting them as new records.
// Appends related to o.R.Containers.
// Sets related.R.ContentType appropriately.
func (o *ContentType) AddContainers(exec boil.Executor, insert bool, related ...*Container) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.ContentTypeID.Int = o.ID
			rel.ContentTypeID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"containers\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"content_type_id"}),
				strmangle.WhereClause("\"", "\"", 2, containerPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.ContentTypeID.Int = o.ID
			rel.ContentTypeID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &contentTypeR{
			Containers: related,
		}
	} else {
		o.R.Containers = append(o.R.Containers, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerR{
				ContentType: o,
			}
		} else {
			rel.R.ContentType = o
		}
	}
	return nil
}

// SetContainers removes all previously related items of the
// content_type replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.ContentType's Containers accordingly.
// Replaces o.R.Containers with related.
// Sets related.R.ContentType's Containers accordingly.
func (o *ContentType) SetContainers(exec boil.Executor, insert bool, related ...*Container) error {
	query := "update \"containers\" set \"content_type_id\" = null where \"content_type_id\" = $1"
	values := []interface{}{o.ID}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.Containers {
			rel.ContentTypeID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.ContentType = nil
		}

		o.R.Containers = nil
	}
	return o.AddContainers(exec, insert, related...)
}

// RemoveContainers relationships from objects passed in.
// Removes related items from R.Containers (uses pointer comparison, removal does not keep order)
// Sets related.R.ContentType.
func (o *ContentType) RemoveContainers(exec boil.Executor, related ...*Container) error {
	var err error
	for _, rel := range related {
		rel.ContentTypeID.Valid = false
		if rel.R != nil {
			rel.R.ContentType = nil
		}
		if err = rel.Update(exec, "content_type_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.Containers {
			if rel != ri {
				continue
			}

			ln := len(o.R.Containers)
			if ln > 1 && i < ln-1 {
				o.R.Containers[i] = o.R.Containers[ln-1]
			}
			o.R.Containers = o.R.Containers[:ln-1]
			break
		}
	}

	return nil
}

// ContentTypesG retrieves all records.
func ContentTypesG(mods ...qm.QueryMod) contentTypeQuery {
	return ContentTypes(boil.GetDB(), mods...)
}

// ContentTypes retrieves all the records using an executor.
func ContentTypes(exec boil.Executor, mods ...qm.QueryMod) contentTypeQuery {
	mods = append(mods, qm.From("\"content_types\""))
	return contentTypeQuery{NewQuery(exec, mods...)}
}

// FindContentTypeG retrieves a single record by ID.
func FindContentTypeG(id int, selectCols ...string) (*ContentType, error) {
	return FindContentType(boil.GetDB(), id, selectCols...)
}

// FindContentTypeGP retrieves a single record by ID, and panics on error.
func FindContentTypeGP(id int, selectCols ...string) *ContentType {
	retobj, err := FindContentType(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindContentType retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindContentType(exec boil.Executor, id int, selectCols ...string) (*ContentType, error) {
	contentTypeObj := &ContentType{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"content_types\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(contentTypeObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from content_types")
	}

	return contentTypeObj, nil
}

// FindContentTypeP retrieves a single record by ID with an executor, and panics on error.
func FindContentTypeP(exec boil.Executor, id int, selectCols ...string) *ContentType {
	retobj, err := FindContentType(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *ContentType) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *ContentType) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *ContentType) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *ContentType) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no content_types provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(contentTypeColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	contentTypeInsertCacheMut.RLock()
	cache, cached := contentTypeInsertCache[key]
	contentTypeInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			contentTypeColumns,
			contentTypeColumnsWithDefault,
			contentTypeColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(contentTypeType, contentTypeMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(contentTypeType, contentTypeMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"content_types\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into content_types")
	}

	if !cached {
		contentTypeInsertCacheMut.Lock()
		contentTypeInsertCache[key] = cache
		contentTypeInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single ContentType record. See Update for
// whitelist behavior description.
func (o *ContentType) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single ContentType record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *ContentType) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the ContentType, and panics on error.
// See Update for whitelist behavior description.
func (o *ContentType) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the ContentType.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *ContentType) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	contentTypeUpdateCacheMut.RLock()
	cache, cached := contentTypeUpdateCache[key]
	contentTypeUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(contentTypeColumns, contentTypePrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update content_types, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"content_types\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, contentTypePrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(contentTypeType, contentTypeMapping, append(wl, contentTypePrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update content_types row")
	}

	if !cached {
		contentTypeUpdateCacheMut.Lock()
		contentTypeUpdateCache[key] = cache
		contentTypeUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q contentTypeQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q contentTypeQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for content_types")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ContentTypeSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o ContentTypeSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o ContentTypeSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ContentTypeSlice) UpdateAll(exec boil.Executor, cols M) error {
	ln := int64(len(o))
	if ln == 0 {
		return nil
	}

	if len(cols) == 0 {
		return errors.New("kmedia: update all requires at least one column argument")
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentTypePrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"content_types\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(contentTypePrimaryKeyColumns), len(colNames)+1, len(contentTypePrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in contentType slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *ContentType) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *ContentType) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *ContentType) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *ContentType) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no content_types provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(contentTypeColumnsWithDefault, o)

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

	contentTypeUpsertCacheMut.RLock()
	cache, cached := contentTypeUpsertCache[key]
	contentTypeUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			contentTypeColumns,
			contentTypeColumnsWithDefault,
			contentTypeColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			contentTypeColumns,
			contentTypePrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert content_types, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(contentTypePrimaryKeyColumns))
			copy(conflict, contentTypePrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"content_types\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(contentTypeType, contentTypeMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(contentTypeType, contentTypeMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert content_types")
	}

	if !cached {
		contentTypeUpsertCacheMut.Lock()
		contentTypeUpsertCache[key] = cache
		contentTypeUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single ContentType record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContentType) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single ContentType record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *ContentType) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no ContentType provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single ContentType record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContentType) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single ContentType record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *ContentType) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no ContentType provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), contentTypePrimaryKeyMapping)
	sql := "DELETE FROM \"content_types\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from content_types")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q contentTypeQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q contentTypeQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no contentTypeQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from content_types")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o ContentTypeSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o ContentTypeSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no ContentType slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o ContentTypeSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ContentTypeSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no ContentType slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentTypePrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"content_types\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, contentTypePrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(contentTypePrimaryKeyColumns), 1, len(contentTypePrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from contentType slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *ContentType) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *ContentType) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *ContentType) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no ContentType provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *ContentType) Reload(exec boil.Executor) error {
	ret, err := FindContentType(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContentTypeSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContentTypeSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContentTypeSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty ContentTypeSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContentTypeSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	contentTypes := ContentTypeSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentTypePrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"content_types\".* FROM \"content_types\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, contentTypePrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(contentTypePrimaryKeyColumns), 1, len(contentTypePrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&contentTypes)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in ContentTypeSlice")
	}

	*o = contentTypes

	return nil
}

// ContentTypeExists checks if the ContentType row exists.
func ContentTypeExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"content_types\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if content_types exists")
	}

	return exists, nil
}

// ContentTypeExistsG checks if the ContentType row exists.
func ContentTypeExistsG(id int) (bool, error) {
	return ContentTypeExists(boil.GetDB(), id)
}

// ContentTypeExistsGP checks if the ContentType row exists. Panics on error.
func ContentTypeExistsGP(id int) bool {
	e, err := ContentTypeExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// ContentTypeExistsP checks if the ContentType row exists. Panics on error.
func ContentTypeExistsP(exec boil.Executor, id int) bool {
	e, err := ContentTypeExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

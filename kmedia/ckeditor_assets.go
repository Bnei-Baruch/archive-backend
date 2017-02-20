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

// CkeditorAsset is an object representing the database table.
type CkeditorAsset struct {
	ID              int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	DataFileName    string      `boil:"data_file_name" json:"data_file_name" toml:"data_file_name" yaml:"data_file_name"`
	DataContentType null.String `boil:"data_content_type" json:"data_content_type,omitempty" toml:"data_content_type" yaml:"data_content_type,omitempty"`
	DataFileSize    null.Int    `boil:"data_file_size" json:"data_file_size,omitempty" toml:"data_file_size" yaml:"data_file_size,omitempty"`
	AssetableID     null.Int    `boil:"assetable_id" json:"assetable_id,omitempty" toml:"assetable_id" yaml:"assetable_id,omitempty"`
	AssetableType   null.String `boil:"assetable_type" json:"assetable_type,omitempty" toml:"assetable_type" yaml:"assetable_type,omitempty"`
	Type            null.String `boil:"type" json:"type,omitempty" toml:"type" yaml:"type,omitempty"`
	CreatedAt       time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	UpdatedAt       time.Time   `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`

	R *ckeditorAssetR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L ckeditorAssetL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// ckeditorAssetR is where relationships are stored.
type ckeditorAssetR struct {
}

// ckeditorAssetL is where Load methods for each relationship are stored.
type ckeditorAssetL struct{}

var (
	ckeditorAssetColumns               = []string{"id", "data_file_name", "data_content_type", "data_file_size", "assetable_id", "assetable_type", "type", "created_at", "updated_at"}
	ckeditorAssetColumnsWithoutDefault = []string{"data_file_name", "data_content_type", "data_file_size", "assetable_id", "assetable_type", "type", "created_at", "updated_at"}
	ckeditorAssetColumnsWithDefault    = []string{"id"}
	ckeditorAssetPrimaryKeyColumns     = []string{"id"}
)

type (
	// CkeditorAssetSlice is an alias for a slice of pointers to CkeditorAsset.
	// This should generally be used opposed to []CkeditorAsset.
	CkeditorAssetSlice []*CkeditorAsset

	ckeditorAssetQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	ckeditorAssetType                 = reflect.TypeOf(&CkeditorAsset{})
	ckeditorAssetMapping              = queries.MakeStructMapping(ckeditorAssetType)
	ckeditorAssetPrimaryKeyMapping, _ = queries.BindMapping(ckeditorAssetType, ckeditorAssetMapping, ckeditorAssetPrimaryKeyColumns)
	ckeditorAssetInsertCacheMut       sync.RWMutex
	ckeditorAssetInsertCache          = make(map[string]insertCache)
	ckeditorAssetUpdateCacheMut       sync.RWMutex
	ckeditorAssetUpdateCache          = make(map[string]updateCache)
	ckeditorAssetUpsertCacheMut       sync.RWMutex
	ckeditorAssetUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single ckeditorAsset record from the query, and panics on error.
func (q ckeditorAssetQuery) OneP() *CkeditorAsset {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single ckeditorAsset record from the query.
func (q ckeditorAssetQuery) One() (*CkeditorAsset, error) {
	o := &CkeditorAsset{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for ckeditor_assets")
	}

	return o, nil
}

// AllP returns all CkeditorAsset records from the query, and panics on error.
func (q ckeditorAssetQuery) AllP() CkeditorAssetSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all CkeditorAsset records from the query.
func (q ckeditorAssetQuery) All() (CkeditorAssetSlice, error) {
	var o CkeditorAssetSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to CkeditorAsset slice")
	}

	return o, nil
}

// CountP returns the count of all CkeditorAsset records in the query, and panics on error.
func (q ckeditorAssetQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all CkeditorAsset records in the query.
func (q ckeditorAssetQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count ckeditor_assets rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q ckeditorAssetQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q ckeditorAssetQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if ckeditor_assets exists")
	}

	return count > 0, nil
}

// CkeditorAssetsG retrieves all records.
func CkeditorAssetsG(mods ...qm.QueryMod) ckeditorAssetQuery {
	return CkeditorAssets(boil.GetDB(), mods...)
}

// CkeditorAssets retrieves all the records using an executor.
func CkeditorAssets(exec boil.Executor, mods ...qm.QueryMod) ckeditorAssetQuery {
	mods = append(mods, qm.From("\"ckeditor_assets\""))
	return ckeditorAssetQuery{NewQuery(exec, mods...)}
}

// FindCkeditorAssetG retrieves a single record by ID.
func FindCkeditorAssetG(id int, selectCols ...string) (*CkeditorAsset, error) {
	return FindCkeditorAsset(boil.GetDB(), id, selectCols...)
}

// FindCkeditorAssetGP retrieves a single record by ID, and panics on error.
func FindCkeditorAssetGP(id int, selectCols ...string) *CkeditorAsset {
	retobj, err := FindCkeditorAsset(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindCkeditorAsset retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindCkeditorAsset(exec boil.Executor, id int, selectCols ...string) (*CkeditorAsset, error) {
	ckeditorAssetObj := &CkeditorAsset{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"ckeditor_assets\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(ckeditorAssetObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from ckeditor_assets")
	}

	return ckeditorAssetObj, nil
}

// FindCkeditorAssetP retrieves a single record by ID with an executor, and panics on error.
func FindCkeditorAssetP(exec boil.Executor, id int, selectCols ...string) *CkeditorAsset {
	retobj, err := FindCkeditorAsset(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *CkeditorAsset) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *CkeditorAsset) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *CkeditorAsset) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *CkeditorAsset) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no ckeditor_assets provided for insertion")
	}

	var err error
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = currTime
	}

	nzDefaults := queries.NonZeroDefaultSet(ckeditorAssetColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	ckeditorAssetInsertCacheMut.RLock()
	cache, cached := ckeditorAssetInsertCache[key]
	ckeditorAssetInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			ckeditorAssetColumns,
			ckeditorAssetColumnsWithDefault,
			ckeditorAssetColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(ckeditorAssetType, ckeditorAssetMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(ckeditorAssetType, ckeditorAssetMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"ckeditor_assets\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into ckeditor_assets")
	}

	if !cached {
		ckeditorAssetInsertCacheMut.Lock()
		ckeditorAssetInsertCache[key] = cache
		ckeditorAssetInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single CkeditorAsset record. See Update for
// whitelist behavior description.
func (o *CkeditorAsset) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single CkeditorAsset record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *CkeditorAsset) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the CkeditorAsset, and panics on error.
// See Update for whitelist behavior description.
func (o *CkeditorAsset) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the CkeditorAsset.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *CkeditorAsset) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt = currTime

	var err error
	key := makeCacheKey(whitelist, nil)
	ckeditorAssetUpdateCacheMut.RLock()
	cache, cached := ckeditorAssetUpdateCache[key]
	ckeditorAssetUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(ckeditorAssetColumns, ckeditorAssetPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update ckeditor_assets, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"ckeditor_assets\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, ckeditorAssetPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(ckeditorAssetType, ckeditorAssetMapping, append(wl, ckeditorAssetPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update ckeditor_assets row")
	}

	if !cached {
		ckeditorAssetUpdateCacheMut.Lock()
		ckeditorAssetUpdateCache[key] = cache
		ckeditorAssetUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q ckeditorAssetQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q ckeditorAssetQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for ckeditor_assets")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o CkeditorAssetSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o CkeditorAssetSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o CkeditorAssetSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o CkeditorAssetSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), ckeditorAssetPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"ckeditor_assets\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(ckeditorAssetPrimaryKeyColumns), len(colNames)+1, len(ckeditorAssetPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in ckeditorAsset slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *CkeditorAsset) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *CkeditorAsset) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *CkeditorAsset) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *CkeditorAsset) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no ckeditor_assets provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	o.UpdatedAt = currTime

	nzDefaults := queries.NonZeroDefaultSet(ckeditorAssetColumnsWithDefault, o)

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

	ckeditorAssetUpsertCacheMut.RLock()
	cache, cached := ckeditorAssetUpsertCache[key]
	ckeditorAssetUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			ckeditorAssetColumns,
			ckeditorAssetColumnsWithDefault,
			ckeditorAssetColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			ckeditorAssetColumns,
			ckeditorAssetPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert ckeditor_assets, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(ckeditorAssetPrimaryKeyColumns))
			copy(conflict, ckeditorAssetPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"ckeditor_assets\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(ckeditorAssetType, ckeditorAssetMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(ckeditorAssetType, ckeditorAssetMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert ckeditor_assets")
	}

	if !cached {
		ckeditorAssetUpsertCacheMut.Lock()
		ckeditorAssetUpsertCache[key] = cache
		ckeditorAssetUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single CkeditorAsset record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *CkeditorAsset) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single CkeditorAsset record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *CkeditorAsset) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no CkeditorAsset provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single CkeditorAsset record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *CkeditorAsset) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single CkeditorAsset record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *CkeditorAsset) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no CkeditorAsset provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), ckeditorAssetPrimaryKeyMapping)
	sql := "DELETE FROM \"ckeditor_assets\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from ckeditor_assets")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q ckeditorAssetQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q ckeditorAssetQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no ckeditorAssetQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from ckeditor_assets")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o CkeditorAssetSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o CkeditorAssetSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no CkeditorAsset slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o CkeditorAssetSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o CkeditorAssetSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no CkeditorAsset slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), ckeditorAssetPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"ckeditor_assets\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, ckeditorAssetPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(ckeditorAssetPrimaryKeyColumns), 1, len(ckeditorAssetPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from ckeditorAsset slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *CkeditorAsset) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *CkeditorAsset) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *CkeditorAsset) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no CkeditorAsset provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *CkeditorAsset) Reload(exec boil.Executor) error {
	ret, err := FindCkeditorAsset(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *CkeditorAssetSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *CkeditorAssetSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *CkeditorAssetSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty CkeditorAssetSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *CkeditorAssetSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	ckeditorAssets := CkeditorAssetSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), ckeditorAssetPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"ckeditor_assets\".* FROM \"ckeditor_assets\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, ckeditorAssetPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(ckeditorAssetPrimaryKeyColumns), 1, len(ckeditorAssetPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&ckeditorAssets)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in CkeditorAssetSlice")
	}

	*o = ckeditorAssets

	return nil
}

// CkeditorAssetExists checks if the CkeditorAsset row exists.
func CkeditorAssetExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"ckeditor_assets\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if ckeditor_assets exists")
	}

	return exists, nil
}

// CkeditorAssetExistsG checks if the CkeditorAsset row exists.
func CkeditorAssetExistsG(id int) (bool, error) {
	return CkeditorAssetExists(boil.GetDB(), id)
}

// CkeditorAssetExistsGP checks if the CkeditorAsset row exists. Panics on error.
func CkeditorAssetExistsGP(id int) bool {
	e, err := CkeditorAssetExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// CkeditorAssetExistsP checks if the CkeditorAsset row exists. Panics on error.
func CkeditorAssetExistsP(exec boil.Executor, id int) bool {
	e, err := CkeditorAssetExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

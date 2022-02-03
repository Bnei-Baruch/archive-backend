// Code generated by SQLBoiler (https://github.com/Bnei-Baruch/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package mdbmodels

import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/boil"
	"github.com/Bnei-Baruch/sqlboiler/queries"
	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	"github.com/Bnei-Baruch/sqlboiler/strmangle"
	"github.com/pkg/errors"
	"gopkg.in/volatiletech/null.v6"
)

// LabelI18n is an object representing the database table.
type LabelI18n struct {
	LabelID     int64       `boil:"label_id" json:"label_id" toml:"label_id" yaml:"label_id"`
	Language    string      `boil:"language" json:"language" toml:"language" yaml:"language"`
	Name        null.String `boil:"name" json:"name,omitempty" toml:"name" yaml:"name,omitempty"`
	CreatedWith string      `boil:"created_with" json:"created_with" toml:"created_with" yaml:"created_with"`
	CreatedAt   time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`

	R *labelI18nR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L labelI18nL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

var LabelI18nColumns = struct {
	LabelID     string
	Language    string
	Name        string
	CreatedWith string
	CreatedAt   string
}{
	LabelID:     "label_id",
	Language:    "language",
	Name:        "name",
	CreatedWith: "created_with",
	CreatedAt:   "created_at",
}

// labelI18nR is where relationships are stored.
type labelI18nR struct {
	Label *Label
}

// labelI18nL is where Load methods for each relationship are stored.
type labelI18nL struct{}

var (
	labelI18nColumns               = []string{"label_id", "language", "name", "created_with", "created_at"}
	labelI18nColumnsWithoutDefault = []string{"label_id", "language", "name", "created_with"}
	labelI18nColumnsWithDefault    = []string{"created_at"}
	labelI18nPrimaryKeyColumns     = []string{"label_id", "language"}
)

type (
	// LabelI18nSlice is an alias for a slice of pointers to LabelI18n.
	// This should generally be used opposed to []LabelI18n.
	LabelI18nSlice []*LabelI18n

	labelI18nQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	labelI18nType                 = reflect.TypeOf(&LabelI18n{})
	labelI18nMapping              = queries.MakeStructMapping(labelI18nType)
	labelI18nPrimaryKeyMapping, _ = queries.BindMapping(labelI18nType, labelI18nMapping, labelI18nPrimaryKeyColumns)
	labelI18nInsertCacheMut       sync.RWMutex
	labelI18nInsertCache          = make(map[string]insertCache)
	labelI18nUpdateCacheMut       sync.RWMutex
	labelI18nUpdateCache          = make(map[string]updateCache)
	labelI18nUpsertCacheMut       sync.RWMutex
	labelI18nUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single labelI18n record from the query, and panics on error.
func (q labelI18nQuery) OneP() *LabelI18n {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single labelI18n record from the query.
func (q labelI18nQuery) One() (*LabelI18n, error) {
	o := &LabelI18n{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: failed to execute a one query for label_i18n")
	}

	return o, nil
}

// AllP returns all LabelI18n records from the query, and panics on error.
func (q labelI18nQuery) AllP() LabelI18nSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all LabelI18n records from the query.
func (q labelI18nQuery) All() (LabelI18nSlice, error) {
	var o []*LabelI18n

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "models: failed to assign all query results to LabelI18n slice")
	}

	return o, nil
}

// CountP returns the count of all LabelI18n records in the query, and panics on error.
func (q labelI18nQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all LabelI18n records in the query.
func (q labelI18nQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to count label_i18n rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q labelI18nQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q labelI18nQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "models: failed to check if label_i18n exists")
	}

	return count > 0, nil
}

// LabelG pointed to by the foreign key.
func (o *LabelI18n) LabelG(mods ...qm.QueryMod) labelQuery {
	return o.Label(boil.GetDB(), mods...)
}

// Label pointed to by the foreign key.
func (o *LabelI18n) Label(exec boil.Executor, mods ...qm.QueryMod) labelQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.LabelID),
	}

	queryMods = append(queryMods, mods...)

	query := Labels(exec, queryMods...)
	queries.SetFrom(query.Query, "\"labels\"")

	return query
} // LoadLabel allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (labelI18nL) LoadLabel(e boil.Executor, singular bool, maybeLabelI18n interface{}) error {
	var slice []*LabelI18n
	var object *LabelI18n

	count := 1
	if singular {
		object = maybeLabelI18n.(*LabelI18n)
	} else {
		slice = *maybeLabelI18n.(*[]*LabelI18n)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &labelI18nR{}
		}
		args[0] = object.LabelID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &labelI18nR{}
			}
			args[i] = obj.LabelID
		}
	}

	query := fmt.Sprintf(
		"select * from \"labels\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Label")
	}
	defer results.Close()

	var resultSlice []*Label
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Label")
	}

	if len(resultSlice) == 0 {
		return nil
	}

	if singular {
		object.R.Label = resultSlice[0]
		return nil
	}

	for _, local := range slice {
		for _, foreign := range resultSlice {
			if local.LabelID == foreign.ID {
				local.R.Label = foreign
				break
			}
		}
	}

	return nil
}

// SetLabelG of the label_i18n to the related item.
// Sets o.R.Label to related.
// Adds o to related.R.LabelI18ns.
// Uses the global database handle.
func (o *LabelI18n) SetLabelG(insert bool, related *Label) error {
	return o.SetLabel(boil.GetDB(), insert, related)
}

// SetLabelP of the label_i18n to the related item.
// Sets o.R.Label to related.
// Adds o to related.R.LabelI18ns.
// Panics on error.
func (o *LabelI18n) SetLabelP(exec boil.Executor, insert bool, related *Label) {
	if err := o.SetLabel(exec, insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetLabelGP of the label_i18n to the related item.
// Sets o.R.Label to related.
// Adds o to related.R.LabelI18ns.
// Uses the global database handle and panics on error.
func (o *LabelI18n) SetLabelGP(insert bool, related *Label) {
	if err := o.SetLabel(boil.GetDB(), insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetLabel of the label_i18n to the related item.
// Sets o.R.Label to related.
// Adds o to related.R.LabelI18ns.
func (o *LabelI18n) SetLabel(exec boil.Executor, insert bool, related *Label) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"label_i18n\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"label_id"}),
		strmangle.WhereClause("\"", "\"", 2, labelI18nPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.LabelID, o.Language}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.LabelID = related.ID

	if o.R == nil {
		o.R = &labelI18nR{
			Label: related,
		}
	} else {
		o.R.Label = related
	}

	if related.R == nil {
		related.R = &labelR{
			LabelI18ns: LabelI18nSlice{o},
		}
	} else {
		related.R.LabelI18ns = append(related.R.LabelI18ns, o)
	}

	return nil
}

// LabelI18nsG retrieves all records.
func LabelI18nsG(mods ...qm.QueryMod) labelI18nQuery {
	return LabelI18ns(boil.GetDB(), mods...)
}

// LabelI18ns retrieves all the records using an executor.
func LabelI18ns(exec boil.Executor, mods ...qm.QueryMod) labelI18nQuery {
	mods = append(mods, qm.From("\"label_i18n\""))
	return labelI18nQuery{NewQuery(exec, mods...)}
}

// FindLabelI18nG retrieves a single record by ID.
func FindLabelI18nG(labelID int64, language string, selectCols ...string) (*LabelI18n, error) {
	return FindLabelI18n(boil.GetDB(), labelID, language, selectCols...)
}

// FindLabelI18nGP retrieves a single record by ID, and panics on error.
func FindLabelI18nGP(labelID int64, language string, selectCols ...string) *LabelI18n {
	retobj, err := FindLabelI18n(boil.GetDB(), labelID, language, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindLabelI18n retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindLabelI18n(exec boil.Executor, labelID int64, language string, selectCols ...string) (*LabelI18n, error) {
	labelI18nObj := &LabelI18n{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"label_i18n\" where \"label_id\"=$1 AND \"language\"=$2", sel,
	)

	q := queries.Raw(exec, query, labelID, language)

	err := q.Bind(labelI18nObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: unable to select from label_i18n")
	}

	return labelI18nObj, nil
}

// FindLabelI18nP retrieves a single record by ID with an executor, and panics on error.
func FindLabelI18nP(exec boil.Executor, labelID int64, language string, selectCols ...string) *LabelI18n {
	retobj, err := FindLabelI18n(exec, labelID, language, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *LabelI18n) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *LabelI18n) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *LabelI18n) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *LabelI18n) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("models: no label_i18n provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(labelI18nColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	labelI18nInsertCacheMut.RLock()
	cache, cached := labelI18nInsertCache[key]
	labelI18nInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			labelI18nColumns,
			labelI18nColumnsWithDefault,
			labelI18nColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(labelI18nType, labelI18nMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(labelI18nType, labelI18nMapping, returnColumns)
		if err != nil {
			return err
		}
		if len(wl) != 0 {
			cache.query = fmt.Sprintf("INSERT INTO \"label_i18n\" (\"%s\") %%sVALUES (%s)%%s", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))
		} else {
			cache.query = "INSERT INTO \"label_i18n\" DEFAULT VALUES"
		}

		var queryOutput, queryReturning string

		if len(cache.retMapping) != 0 {
			queryReturning = fmt.Sprintf(" RETURNING \"%s\"", strings.Join(returnColumns, "\",\""))
		}

		if len(wl) != 0 {
			cache.query = fmt.Sprintf(cache.query, queryOutput, queryReturning)
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
		return errors.Wrap(err, "models: unable to insert into label_i18n")
	}

	if !cached {
		labelI18nInsertCacheMut.Lock()
		labelI18nInsertCache[key] = cache
		labelI18nInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single LabelI18n record. See Update for
// whitelist behavior description.
func (o *LabelI18n) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single LabelI18n record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *LabelI18n) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the LabelI18n, and panics on error.
// See Update for whitelist behavior description.
func (o *LabelI18n) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the LabelI18n.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *LabelI18n) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	labelI18nUpdateCacheMut.RLock()
	cache, cached := labelI18nUpdateCache[key]
	labelI18nUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(
			labelI18nColumns,
			labelI18nPrimaryKeyColumns,
			whitelist,
		)

		if len(wl) == 0 {
			return errors.New("models: unable to update label_i18n, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"label_i18n\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, labelI18nPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(labelI18nType, labelI18nMapping, append(wl, labelI18nPrimaryKeyColumns...))
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
		return errors.Wrap(err, "models: unable to update label_i18n row")
	}

	if !cached {
		labelI18nUpdateCacheMut.Lock()
		labelI18nUpdateCache[key] = cache
		labelI18nUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q labelI18nQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q labelI18nQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "models: unable to update all for label_i18n")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o LabelI18nSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o LabelI18nSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o LabelI18nSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o LabelI18nSlice) UpdateAll(exec boil.Executor, cols M) error {
	ln := int64(len(o))
	if ln == 0 {
		return nil
	}

	if len(cols) == 0 {
		return errors.New("models: update all requires at least one column argument")
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), labelI18nPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf("UPDATE \"label_i18n\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), len(colNames)+1, labelI18nPrimaryKeyColumns, len(o)))

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "models: unable to update all in labelI18n slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *LabelI18n) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *LabelI18n) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *LabelI18n) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *LabelI18n) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("models: no label_i18n provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(labelI18nColumnsWithDefault, o)

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

	labelI18nUpsertCacheMut.RLock()
	cache, cached := labelI18nUpsertCache[key]
	labelI18nUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		insert, ret := strmangle.InsertColumnSet(
			labelI18nColumns,
			labelI18nColumnsWithDefault,
			labelI18nColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		update := strmangle.UpdateColumnSet(
			labelI18nColumns,
			labelI18nPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("models: unable to upsert label_i18n, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(labelI18nPrimaryKeyColumns))
			copy(conflict, labelI18nPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"label_i18n\"", updateOnConflict, ret, update, conflict, insert)

		cache.valueMapping, err = queries.BindMapping(labelI18nType, labelI18nMapping, insert)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(labelI18nType, labelI18nMapping, ret)
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
		return errors.Wrap(err, "models: unable to upsert label_i18n")
	}

	if !cached {
		labelI18nUpsertCacheMut.Lock()
		labelI18nUpsertCache[key] = cache
		labelI18nUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single LabelI18n record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *LabelI18n) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single LabelI18n record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *LabelI18n) DeleteG() error {
	if o == nil {
		return errors.New("models: no LabelI18n provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single LabelI18n record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *LabelI18n) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single LabelI18n record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *LabelI18n) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("models: no LabelI18n provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), labelI18nPrimaryKeyMapping)
	sql := "DELETE FROM \"label_i18n\" WHERE \"label_id\"=$1 AND \"language\"=$2"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "models: unable to delete from label_i18n")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q labelI18nQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q labelI18nQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("models: no labelI18nQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "models: unable to delete all from label_i18n")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o LabelI18nSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o LabelI18nSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("models: no LabelI18n slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o LabelI18nSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o LabelI18nSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("models: no LabelI18n slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), labelI18nPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "DELETE FROM \"label_i18n\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, labelI18nPrimaryKeyColumns, len(o))

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "models: unable to delete all from labelI18n slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *LabelI18n) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *LabelI18n) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *LabelI18n) ReloadG() error {
	if o == nil {
		return errors.New("models: no LabelI18n provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *LabelI18n) Reload(exec boil.Executor) error {
	ret, err := FindLabelI18n(exec, o.LabelID, o.Language)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *LabelI18nSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *LabelI18nSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *LabelI18nSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("models: empty LabelI18nSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *LabelI18nSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	labelI18ns := LabelI18nSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), labelI18nPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "SELECT \"label_i18n\".* FROM \"label_i18n\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, labelI18nPrimaryKeyColumns, len(*o))

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&labelI18ns)
	if err != nil {
		return errors.Wrap(err, "models: unable to reload all in LabelI18nSlice")
	}

	*o = labelI18ns

	return nil
}

// LabelI18nExists checks if the LabelI18n row exists.
func LabelI18nExists(exec boil.Executor, labelID int64, language string) (bool, error) {
	var exists bool
	sql := "select exists(select 1 from \"label_i18n\" where \"label_id\"=$1 AND \"language\"=$2 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, labelID, language)
	}

	row := exec.QueryRow(sql, labelID, language)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "models: unable to check if label_i18n exists")
	}

	return exists, nil
}

// LabelI18nExistsG checks if the LabelI18n row exists.
func LabelI18nExistsG(labelID int64, language string) (bool, error) {
	return LabelI18nExists(boil.GetDB(), labelID, language)
}

// LabelI18nExistsGP checks if the LabelI18n row exists. Panics on error.
func LabelI18nExistsGP(labelID int64, language string) bool {
	e, err := LabelI18nExists(boil.GetDB(), labelID, language)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// LabelI18nExistsP checks if the LabelI18n row exists. Panics on error.
func LabelI18nExistsP(exec boil.Executor, labelID int64, language string) bool {
	e, err := LabelI18nExists(exec, labelID, language)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

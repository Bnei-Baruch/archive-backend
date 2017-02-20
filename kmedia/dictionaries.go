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

// Dictionary is an object representing the database table.
type Dictionary struct {
	ID        int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	Suid      null.String `boil:"suid" json:"suid,omitempty" toml:"suid" yaml:"suid,omitempty"`
	CreatedAt time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	UpdatedAt time.Time   `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`

	R *dictionaryR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L dictionaryL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// dictionaryR is where relationships are stored.
type dictionaryR struct {
	DictionaryDescriptions DictionaryDescriptionSlice
}

// dictionaryL is where Load methods for each relationship are stored.
type dictionaryL struct{}

var (
	dictionaryColumns               = []string{"id", "suid", "created_at", "updated_at"}
	dictionaryColumnsWithoutDefault = []string{"suid", "created_at", "updated_at"}
	dictionaryColumnsWithDefault    = []string{"id"}
	dictionaryPrimaryKeyColumns     = []string{"id"}
)

type (
	// DictionarySlice is an alias for a slice of pointers to Dictionary.
	// This should generally be used opposed to []Dictionary.
	DictionarySlice []*Dictionary

	dictionaryQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	dictionaryType                 = reflect.TypeOf(&Dictionary{})
	dictionaryMapping              = queries.MakeStructMapping(dictionaryType)
	dictionaryPrimaryKeyMapping, _ = queries.BindMapping(dictionaryType, dictionaryMapping, dictionaryPrimaryKeyColumns)
	dictionaryInsertCacheMut       sync.RWMutex
	dictionaryInsertCache          = make(map[string]insertCache)
	dictionaryUpdateCacheMut       sync.RWMutex
	dictionaryUpdateCache          = make(map[string]updateCache)
	dictionaryUpsertCacheMut       sync.RWMutex
	dictionaryUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single dictionary record from the query, and panics on error.
func (q dictionaryQuery) OneP() *Dictionary {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single dictionary record from the query.
func (q dictionaryQuery) One() (*Dictionary, error) {
	o := &Dictionary{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for dictionaries")
	}

	return o, nil
}

// AllP returns all Dictionary records from the query, and panics on error.
func (q dictionaryQuery) AllP() DictionarySlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all Dictionary records from the query.
func (q dictionaryQuery) All() (DictionarySlice, error) {
	var o DictionarySlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to Dictionary slice")
	}

	return o, nil
}

// CountP returns the count of all Dictionary records in the query, and panics on error.
func (q dictionaryQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all Dictionary records in the query.
func (q dictionaryQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count dictionaries rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q dictionaryQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q dictionaryQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if dictionaries exists")
	}

	return count > 0, nil
}

// DictionaryDescriptionsG retrieves all the dictionary_description's dictionary descriptions.
func (o *Dictionary) DictionaryDescriptionsG(mods ...qm.QueryMod) dictionaryDescriptionQuery {
	return o.DictionaryDescriptions(boil.GetDB(), mods...)
}

// DictionaryDescriptions retrieves all the dictionary_description's dictionary descriptions with an executor.
func (o *Dictionary) DictionaryDescriptions(exec boil.Executor, mods ...qm.QueryMod) dictionaryDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"dictionary_id\"=?", o.ID),
	)

	query := DictionaryDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"dictionary_descriptions\" as \"a\"")
	return query
}

// LoadDictionaryDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (dictionaryL) LoadDictionaryDescriptions(e boil.Executor, singular bool, maybeDictionary interface{}) error {
	var slice []*Dictionary
	var object *Dictionary

	count := 1
	if singular {
		object = maybeDictionary.(*Dictionary)
	} else {
		slice = *maybeDictionary.(*DictionarySlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &dictionaryR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &dictionaryR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"dictionary_descriptions\" where \"dictionary_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load dictionary_descriptions")
	}
	defer results.Close()

	var resultSlice []*DictionaryDescription
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice dictionary_descriptions")
	}

	if singular {
		object.R.DictionaryDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.DictionaryID.Int {
				local.R.DictionaryDescriptions = append(local.R.DictionaryDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// AddDictionaryDescriptions adds the given related objects to the existing relationships
// of the dictionary, optionally inserting them as new records.
// Appends related to o.R.DictionaryDescriptions.
// Sets related.R.Dictionary appropriately.
func (o *Dictionary) AddDictionaryDescriptions(exec boil.Executor, insert bool, related ...*DictionaryDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.DictionaryID.Int = o.ID
			rel.DictionaryID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"dictionary_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"dictionary_id"}),
				strmangle.WhereClause("\"", "\"", 2, dictionaryDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.DictionaryID.Int = o.ID
			rel.DictionaryID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &dictionaryR{
			DictionaryDescriptions: related,
		}
	} else {
		o.R.DictionaryDescriptions = append(o.R.DictionaryDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &dictionaryDescriptionR{
				Dictionary: o,
			}
		} else {
			rel.R.Dictionary = o
		}
	}
	return nil
}

// SetDictionaryDescriptions removes all previously related items of the
// dictionary replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Dictionary's DictionaryDescriptions accordingly.
// Replaces o.R.DictionaryDescriptions with related.
// Sets related.R.Dictionary's DictionaryDescriptions accordingly.
func (o *Dictionary) SetDictionaryDescriptions(exec boil.Executor, insert bool, related ...*DictionaryDescription) error {
	query := "update \"dictionary_descriptions\" set \"dictionary_id\" = null where \"dictionary_id\" = $1"
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
		for _, rel := range o.R.DictionaryDescriptions {
			rel.DictionaryID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Dictionary = nil
		}

		o.R.DictionaryDescriptions = nil
	}
	return o.AddDictionaryDescriptions(exec, insert, related...)
}

// RemoveDictionaryDescriptions relationships from objects passed in.
// Removes related items from R.DictionaryDescriptions (uses pointer comparison, removal does not keep order)
// Sets related.R.Dictionary.
func (o *Dictionary) RemoveDictionaryDescriptions(exec boil.Executor, related ...*DictionaryDescription) error {
	var err error
	for _, rel := range related {
		rel.DictionaryID.Valid = false
		if rel.R != nil {
			rel.R.Dictionary = nil
		}
		if err = rel.Update(exec, "dictionary_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.DictionaryDescriptions {
			if rel != ri {
				continue
			}

			ln := len(o.R.DictionaryDescriptions)
			if ln > 1 && i < ln-1 {
				o.R.DictionaryDescriptions[i] = o.R.DictionaryDescriptions[ln-1]
			}
			o.R.DictionaryDescriptions = o.R.DictionaryDescriptions[:ln-1]
			break
		}
	}

	return nil
}

// DictionariesG retrieves all records.
func DictionariesG(mods ...qm.QueryMod) dictionaryQuery {
	return Dictionaries(boil.GetDB(), mods...)
}

// Dictionaries retrieves all the records using an executor.
func Dictionaries(exec boil.Executor, mods ...qm.QueryMod) dictionaryQuery {
	mods = append(mods, qm.From("\"dictionaries\""))
	return dictionaryQuery{NewQuery(exec, mods...)}
}

// FindDictionaryG retrieves a single record by ID.
func FindDictionaryG(id int, selectCols ...string) (*Dictionary, error) {
	return FindDictionary(boil.GetDB(), id, selectCols...)
}

// FindDictionaryGP retrieves a single record by ID, and panics on error.
func FindDictionaryGP(id int, selectCols ...string) *Dictionary {
	retobj, err := FindDictionary(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindDictionary retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindDictionary(exec boil.Executor, id int, selectCols ...string) (*Dictionary, error) {
	dictionaryObj := &Dictionary{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"dictionaries\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(dictionaryObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from dictionaries")
	}

	return dictionaryObj, nil
}

// FindDictionaryP retrieves a single record by ID with an executor, and panics on error.
func FindDictionaryP(exec boil.Executor, id int, selectCols ...string) *Dictionary {
	retobj, err := FindDictionary(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *Dictionary) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *Dictionary) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *Dictionary) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *Dictionary) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no dictionaries provided for insertion")
	}

	var err error
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = currTime
	}

	nzDefaults := queries.NonZeroDefaultSet(dictionaryColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	dictionaryInsertCacheMut.RLock()
	cache, cached := dictionaryInsertCache[key]
	dictionaryInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			dictionaryColumns,
			dictionaryColumnsWithDefault,
			dictionaryColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(dictionaryType, dictionaryMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(dictionaryType, dictionaryMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"dictionaries\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into dictionaries")
	}

	if !cached {
		dictionaryInsertCacheMut.Lock()
		dictionaryInsertCache[key] = cache
		dictionaryInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single Dictionary record. See Update for
// whitelist behavior description.
func (o *Dictionary) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single Dictionary record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *Dictionary) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the Dictionary, and panics on error.
// See Update for whitelist behavior description.
func (o *Dictionary) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the Dictionary.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *Dictionary) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt = currTime

	var err error
	key := makeCacheKey(whitelist, nil)
	dictionaryUpdateCacheMut.RLock()
	cache, cached := dictionaryUpdateCache[key]
	dictionaryUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(dictionaryColumns, dictionaryPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update dictionaries, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"dictionaries\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, dictionaryPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(dictionaryType, dictionaryMapping, append(wl, dictionaryPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update dictionaries row")
	}

	if !cached {
		dictionaryUpdateCacheMut.Lock()
		dictionaryUpdateCache[key] = cache
		dictionaryUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q dictionaryQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q dictionaryQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for dictionaries")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o DictionarySlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o DictionarySlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o DictionarySlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o DictionarySlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), dictionaryPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"dictionaries\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(dictionaryPrimaryKeyColumns), len(colNames)+1, len(dictionaryPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in dictionary slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *Dictionary) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *Dictionary) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *Dictionary) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *Dictionary) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no dictionaries provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	o.UpdatedAt = currTime

	nzDefaults := queries.NonZeroDefaultSet(dictionaryColumnsWithDefault, o)

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

	dictionaryUpsertCacheMut.RLock()
	cache, cached := dictionaryUpsertCache[key]
	dictionaryUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			dictionaryColumns,
			dictionaryColumnsWithDefault,
			dictionaryColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			dictionaryColumns,
			dictionaryPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert dictionaries, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(dictionaryPrimaryKeyColumns))
			copy(conflict, dictionaryPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"dictionaries\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(dictionaryType, dictionaryMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(dictionaryType, dictionaryMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert dictionaries")
	}

	if !cached {
		dictionaryUpsertCacheMut.Lock()
		dictionaryUpsertCache[key] = cache
		dictionaryUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single Dictionary record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Dictionary) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single Dictionary record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *Dictionary) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no Dictionary provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single Dictionary record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Dictionary) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single Dictionary record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Dictionary) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Dictionary provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), dictionaryPrimaryKeyMapping)
	sql := "DELETE FROM \"dictionaries\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from dictionaries")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q dictionaryQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q dictionaryQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no dictionaryQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from dictionaries")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o DictionarySlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o DictionarySlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no Dictionary slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o DictionarySlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o DictionarySlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Dictionary slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), dictionaryPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"dictionaries\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, dictionaryPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(dictionaryPrimaryKeyColumns), 1, len(dictionaryPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from dictionary slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *Dictionary) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *Dictionary) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *Dictionary) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no Dictionary provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Dictionary) Reload(exec boil.Executor) error {
	ret, err := FindDictionary(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *DictionarySlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *DictionarySlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *DictionarySlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty DictionarySlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *DictionarySlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	dictionaries := DictionarySlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), dictionaryPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"dictionaries\".* FROM \"dictionaries\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, dictionaryPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(dictionaryPrimaryKeyColumns), 1, len(dictionaryPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&dictionaries)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in DictionarySlice")
	}

	*o = dictionaries

	return nil
}

// DictionaryExists checks if the Dictionary row exists.
func DictionaryExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"dictionaries\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if dictionaries exists")
	}

	return exists, nil
}

// DictionaryExistsG checks if the Dictionary row exists.
func DictionaryExistsG(id int) (bool, error) {
	return DictionaryExists(boil.GetDB(), id)
}

// DictionaryExistsGP checks if the Dictionary row exists. Panics on error.
func DictionaryExistsGP(id int) bool {
	e, err := DictionaryExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// DictionaryExistsP checks if the Dictionary row exists. Panics on error.
func DictionaryExistsP(exec boil.Executor, id int) bool {
	e, err := DictionaryExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

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

// DictionaryDescription is an object representing the database table.
type DictionaryDescription struct {
	ID           int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	DictionaryID null.Int    `boil:"dictionary_id" json:"dictionary_id,omitempty" toml:"dictionary_id" yaml:"dictionary_id,omitempty"`
	Topic        null.String `boil:"topic" json:"topic,omitempty" toml:"topic" yaml:"topic,omitempty"`
	LangID       null.String `boil:"lang_id" json:"lang_id,omitempty" toml:"lang_id" yaml:"lang_id,omitempty"`
	CreatedAt    time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	UpdatedAt    time.Time   `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`

	R *dictionaryDescriptionR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L dictionaryDescriptionL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// dictionaryDescriptionR is where relationships are stored.
type dictionaryDescriptionR struct {
	Dictionary *Dictionary
	Lang       *Language
}

// dictionaryDescriptionL is where Load methods for each relationship are stored.
type dictionaryDescriptionL struct{}

var (
	dictionaryDescriptionColumns               = []string{"id", "dictionary_id", "topic", "lang_id", "created_at", "updated_at"}
	dictionaryDescriptionColumnsWithoutDefault = []string{"dictionary_id", "topic", "created_at", "updated_at"}
	dictionaryDescriptionColumnsWithDefault    = []string{"id", "lang_id"}
	dictionaryDescriptionPrimaryKeyColumns     = []string{"id"}
)

type (
	// DictionaryDescriptionSlice is an alias for a slice of pointers to DictionaryDescription.
	// This should generally be used opposed to []DictionaryDescription.
	DictionaryDescriptionSlice []*DictionaryDescription

	dictionaryDescriptionQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	dictionaryDescriptionType                 = reflect.TypeOf(&DictionaryDescription{})
	dictionaryDescriptionMapping              = queries.MakeStructMapping(dictionaryDescriptionType)
	dictionaryDescriptionPrimaryKeyMapping, _ = queries.BindMapping(dictionaryDescriptionType, dictionaryDescriptionMapping, dictionaryDescriptionPrimaryKeyColumns)
	dictionaryDescriptionInsertCacheMut       sync.RWMutex
	dictionaryDescriptionInsertCache          = make(map[string]insertCache)
	dictionaryDescriptionUpdateCacheMut       sync.RWMutex
	dictionaryDescriptionUpdateCache          = make(map[string]updateCache)
	dictionaryDescriptionUpsertCacheMut       sync.RWMutex
	dictionaryDescriptionUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single dictionaryDescription record from the query, and panics on error.
func (q dictionaryDescriptionQuery) OneP() *DictionaryDescription {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single dictionaryDescription record from the query.
func (q dictionaryDescriptionQuery) One() (*DictionaryDescription, error) {
	o := &DictionaryDescription{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for dictionary_descriptions")
	}

	return o, nil
}

// AllP returns all DictionaryDescription records from the query, and panics on error.
func (q dictionaryDescriptionQuery) AllP() DictionaryDescriptionSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all DictionaryDescription records from the query.
func (q dictionaryDescriptionQuery) All() (DictionaryDescriptionSlice, error) {
	var o DictionaryDescriptionSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to DictionaryDescription slice")
	}

	return o, nil
}

// CountP returns the count of all DictionaryDescription records in the query, and panics on error.
func (q dictionaryDescriptionQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all DictionaryDescription records in the query.
func (q dictionaryDescriptionQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count dictionary_descriptions rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q dictionaryDescriptionQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q dictionaryDescriptionQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if dictionary_descriptions exists")
	}

	return count > 0, nil
}

// DictionaryG pointed to by the foreign key.
func (o *DictionaryDescription) DictionaryG(mods ...qm.QueryMod) dictionaryQuery {
	return o.Dictionary(boil.GetDB(), mods...)
}

// Dictionary pointed to by the foreign key.
func (o *DictionaryDescription) Dictionary(exec boil.Executor, mods ...qm.QueryMod) dictionaryQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.DictionaryID),
	}

	queryMods = append(queryMods, mods...)

	query := Dictionaries(exec, queryMods...)
	queries.SetFrom(query.Query, "\"dictionaries\"")

	return query
}

// LangG pointed to by the foreign key.
func (o *DictionaryDescription) LangG(mods ...qm.QueryMod) languageQuery {
	return o.Lang(boil.GetDB(), mods...)
}

// Lang pointed to by the foreign key.
func (o *DictionaryDescription) Lang(exec boil.Executor, mods ...qm.QueryMod) languageQuery {
	queryMods := []qm.QueryMod{
		qm.Where("code3=?", o.LangID),
	}

	queryMods = append(queryMods, mods...)

	query := Languages(exec, queryMods...)
	queries.SetFrom(query.Query, "\"languages\"")

	return query
}

// LoadDictionary allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (dictionaryDescriptionL) LoadDictionary(e boil.Executor, singular bool, maybeDictionaryDescription interface{}) error {
	var slice []*DictionaryDescription
	var object *DictionaryDescription

	count := 1
	if singular {
		object = maybeDictionaryDescription.(*DictionaryDescription)
	} else {
		slice = *maybeDictionaryDescription.(*DictionaryDescriptionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &dictionaryDescriptionR{}
		}
		args[0] = object.DictionaryID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &dictionaryDescriptionR{}
			}
			args[i] = obj.DictionaryID
		}
	}

	query := fmt.Sprintf(
		"select * from \"dictionaries\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Dictionary")
	}
	defer results.Close()

	var resultSlice []*Dictionary
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Dictionary")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Dictionary = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.DictionaryID.Int == foreign.ID {
				local.R.Dictionary = foreign
				break
			}
		}
	}

	return nil
}

// LoadLang allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (dictionaryDescriptionL) LoadLang(e boil.Executor, singular bool, maybeDictionaryDescription interface{}) error {
	var slice []*DictionaryDescription
	var object *DictionaryDescription

	count := 1
	if singular {
		object = maybeDictionaryDescription.(*DictionaryDescription)
	} else {
		slice = *maybeDictionaryDescription.(*DictionaryDescriptionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &dictionaryDescriptionR{}
		}
		args[0] = object.LangID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &dictionaryDescriptionR{}
			}
			args[i] = obj.LangID
		}
	}

	query := fmt.Sprintf(
		"select * from \"languages\" where \"code3\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Language")
	}
	defer results.Close()

	var resultSlice []*Language
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Language")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Lang = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.LangID.String == foreign.Code3.String {
				local.R.Lang = foreign
				break
			}
		}
	}

	return nil
}

// SetDictionary of the dictionary_description to the related item.
// Sets o.R.Dictionary to related.
// Adds o to related.R.DictionaryDescriptions.
func (o *DictionaryDescription) SetDictionary(exec boil.Executor, insert bool, related *Dictionary) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"dictionary_descriptions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"dictionary_id"}),
		strmangle.WhereClause("\"", "\"", 2, dictionaryDescriptionPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.DictionaryID.Int = related.ID
	o.DictionaryID.Valid = true

	if o.R == nil {
		o.R = &dictionaryDescriptionR{
			Dictionary: related,
		}
	} else {
		o.R.Dictionary = related
	}

	if related.R == nil {
		related.R = &dictionaryR{
			DictionaryDescriptions: DictionaryDescriptionSlice{o},
		}
	} else {
		related.R.DictionaryDescriptions = append(related.R.DictionaryDescriptions, o)
	}

	return nil
}

// RemoveDictionary relationship.
// Sets o.R.Dictionary to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *DictionaryDescription) RemoveDictionary(exec boil.Executor, related *Dictionary) error {
	var err error

	o.DictionaryID.Valid = false
	if err = o.Update(exec, "dictionary_id"); err != nil {
		o.DictionaryID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.Dictionary = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.DictionaryDescriptions {
		if o.DictionaryID.Int != ri.DictionaryID.Int {
			continue
		}

		ln := len(related.R.DictionaryDescriptions)
		if ln > 1 && i < ln-1 {
			related.R.DictionaryDescriptions[i] = related.R.DictionaryDescriptions[ln-1]
		}
		related.R.DictionaryDescriptions = related.R.DictionaryDescriptions[:ln-1]
		break
	}
	return nil
}

// SetLang of the dictionary_description to the related item.
// Sets o.R.Lang to related.
// Adds o to related.R.LangDictionaryDescriptions.
func (o *DictionaryDescription) SetLang(exec boil.Executor, insert bool, related *Language) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"dictionary_descriptions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
		strmangle.WhereClause("\"", "\"", 2, dictionaryDescriptionPrimaryKeyColumns),
	)
	values := []interface{}{related.Code3, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.LangID.String = related.Code3.String
	o.LangID.Valid = true

	if o.R == nil {
		o.R = &dictionaryDescriptionR{
			Lang: related,
		}
	} else {
		o.R.Lang = related
	}

	if related.R == nil {
		related.R = &languageR{
			LangDictionaryDescriptions: DictionaryDescriptionSlice{o},
		}
	} else {
		related.R.LangDictionaryDescriptions = append(related.R.LangDictionaryDescriptions, o)
	}

	return nil
}

// RemoveLang relationship.
// Sets o.R.Lang to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *DictionaryDescription) RemoveLang(exec boil.Executor, related *Language) error {
	var err error

	o.LangID.Valid = false
	if err = o.Update(exec, "lang_id"); err != nil {
		o.LangID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.Lang = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.LangDictionaryDescriptions {
		if o.LangID.String != ri.LangID.String {
			continue
		}

		ln := len(related.R.LangDictionaryDescriptions)
		if ln > 1 && i < ln-1 {
			related.R.LangDictionaryDescriptions[i] = related.R.LangDictionaryDescriptions[ln-1]
		}
		related.R.LangDictionaryDescriptions = related.R.LangDictionaryDescriptions[:ln-1]
		break
	}
	return nil
}

// DictionaryDescriptionsG retrieves all records.
func DictionaryDescriptionsG(mods ...qm.QueryMod) dictionaryDescriptionQuery {
	return DictionaryDescriptions(boil.GetDB(), mods...)
}

// DictionaryDescriptions retrieves all the records using an executor.
func DictionaryDescriptions(exec boil.Executor, mods ...qm.QueryMod) dictionaryDescriptionQuery {
	mods = append(mods, qm.From("\"dictionary_descriptions\""))
	return dictionaryDescriptionQuery{NewQuery(exec, mods...)}
}

// FindDictionaryDescriptionG retrieves a single record by ID.
func FindDictionaryDescriptionG(id int, selectCols ...string) (*DictionaryDescription, error) {
	return FindDictionaryDescription(boil.GetDB(), id, selectCols...)
}

// FindDictionaryDescriptionGP retrieves a single record by ID, and panics on error.
func FindDictionaryDescriptionGP(id int, selectCols ...string) *DictionaryDescription {
	retobj, err := FindDictionaryDescription(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindDictionaryDescription retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindDictionaryDescription(exec boil.Executor, id int, selectCols ...string) (*DictionaryDescription, error) {
	dictionaryDescriptionObj := &DictionaryDescription{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"dictionary_descriptions\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(dictionaryDescriptionObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from dictionary_descriptions")
	}

	return dictionaryDescriptionObj, nil
}

// FindDictionaryDescriptionP retrieves a single record by ID with an executor, and panics on error.
func FindDictionaryDescriptionP(exec boil.Executor, id int, selectCols ...string) *DictionaryDescription {
	retobj, err := FindDictionaryDescription(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *DictionaryDescription) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *DictionaryDescription) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *DictionaryDescription) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *DictionaryDescription) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no dictionary_descriptions provided for insertion")
	}

	var err error
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = currTime
	}

	nzDefaults := queries.NonZeroDefaultSet(dictionaryDescriptionColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	dictionaryDescriptionInsertCacheMut.RLock()
	cache, cached := dictionaryDescriptionInsertCache[key]
	dictionaryDescriptionInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			dictionaryDescriptionColumns,
			dictionaryDescriptionColumnsWithDefault,
			dictionaryDescriptionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(dictionaryDescriptionType, dictionaryDescriptionMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(dictionaryDescriptionType, dictionaryDescriptionMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"dictionary_descriptions\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into dictionary_descriptions")
	}

	if !cached {
		dictionaryDescriptionInsertCacheMut.Lock()
		dictionaryDescriptionInsertCache[key] = cache
		dictionaryDescriptionInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single DictionaryDescription record. See Update for
// whitelist behavior description.
func (o *DictionaryDescription) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single DictionaryDescription record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *DictionaryDescription) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the DictionaryDescription, and panics on error.
// See Update for whitelist behavior description.
func (o *DictionaryDescription) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the DictionaryDescription.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *DictionaryDescription) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt = currTime

	var err error
	key := makeCacheKey(whitelist, nil)
	dictionaryDescriptionUpdateCacheMut.RLock()
	cache, cached := dictionaryDescriptionUpdateCache[key]
	dictionaryDescriptionUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(dictionaryDescriptionColumns, dictionaryDescriptionPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update dictionary_descriptions, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"dictionary_descriptions\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, dictionaryDescriptionPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(dictionaryDescriptionType, dictionaryDescriptionMapping, append(wl, dictionaryDescriptionPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update dictionary_descriptions row")
	}

	if !cached {
		dictionaryDescriptionUpdateCacheMut.Lock()
		dictionaryDescriptionUpdateCache[key] = cache
		dictionaryDescriptionUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q dictionaryDescriptionQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q dictionaryDescriptionQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for dictionary_descriptions")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o DictionaryDescriptionSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o DictionaryDescriptionSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o DictionaryDescriptionSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o DictionaryDescriptionSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), dictionaryDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"dictionary_descriptions\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(dictionaryDescriptionPrimaryKeyColumns), len(colNames)+1, len(dictionaryDescriptionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in dictionaryDescription slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *DictionaryDescription) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *DictionaryDescription) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *DictionaryDescription) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *DictionaryDescription) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no dictionary_descriptions provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	o.UpdatedAt = currTime

	nzDefaults := queries.NonZeroDefaultSet(dictionaryDescriptionColumnsWithDefault, o)

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

	dictionaryDescriptionUpsertCacheMut.RLock()
	cache, cached := dictionaryDescriptionUpsertCache[key]
	dictionaryDescriptionUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			dictionaryDescriptionColumns,
			dictionaryDescriptionColumnsWithDefault,
			dictionaryDescriptionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			dictionaryDescriptionColumns,
			dictionaryDescriptionPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert dictionary_descriptions, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(dictionaryDescriptionPrimaryKeyColumns))
			copy(conflict, dictionaryDescriptionPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"dictionary_descriptions\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(dictionaryDescriptionType, dictionaryDescriptionMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(dictionaryDescriptionType, dictionaryDescriptionMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert dictionary_descriptions")
	}

	if !cached {
		dictionaryDescriptionUpsertCacheMut.Lock()
		dictionaryDescriptionUpsertCache[key] = cache
		dictionaryDescriptionUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single DictionaryDescription record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *DictionaryDescription) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single DictionaryDescription record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *DictionaryDescription) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no DictionaryDescription provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single DictionaryDescription record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *DictionaryDescription) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single DictionaryDescription record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *DictionaryDescription) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no DictionaryDescription provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), dictionaryDescriptionPrimaryKeyMapping)
	sql := "DELETE FROM \"dictionary_descriptions\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from dictionary_descriptions")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q dictionaryDescriptionQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q dictionaryDescriptionQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no dictionaryDescriptionQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from dictionary_descriptions")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o DictionaryDescriptionSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o DictionaryDescriptionSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no DictionaryDescription slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o DictionaryDescriptionSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o DictionaryDescriptionSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no DictionaryDescription slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), dictionaryDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"dictionary_descriptions\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, dictionaryDescriptionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(dictionaryDescriptionPrimaryKeyColumns), 1, len(dictionaryDescriptionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from dictionaryDescription slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *DictionaryDescription) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *DictionaryDescription) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *DictionaryDescription) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no DictionaryDescription provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *DictionaryDescription) Reload(exec boil.Executor) error {
	ret, err := FindDictionaryDescription(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *DictionaryDescriptionSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *DictionaryDescriptionSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *DictionaryDescriptionSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty DictionaryDescriptionSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *DictionaryDescriptionSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	dictionaryDescriptions := DictionaryDescriptionSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), dictionaryDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"dictionary_descriptions\".* FROM \"dictionary_descriptions\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, dictionaryDescriptionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(dictionaryDescriptionPrimaryKeyColumns), 1, len(dictionaryDescriptionPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&dictionaryDescriptions)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in DictionaryDescriptionSlice")
	}

	*o = dictionaryDescriptions

	return nil
}

// DictionaryDescriptionExists checks if the DictionaryDescription row exists.
func DictionaryDescriptionExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"dictionary_descriptions\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if dictionary_descriptions exists")
	}

	return exists, nil
}

// DictionaryDescriptionExistsG checks if the DictionaryDescription row exists.
func DictionaryDescriptionExistsG(id int) (bool, error) {
	return DictionaryDescriptionExists(boil.GetDB(), id)
}

// DictionaryDescriptionExistsGP checks if the DictionaryDescription row exists. Panics on error.
func DictionaryDescriptionExistsGP(id int) bool {
	e, err := DictionaryDescriptionExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// DictionaryDescriptionExistsP checks if the DictionaryDescription row exists. Panics on error.
func DictionaryDescriptionExistsP(exec boil.Executor, id int) bool {
	e, err := DictionaryDescriptionExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

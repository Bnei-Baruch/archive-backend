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

// LecturerDescription is an object representing the database table.
type LecturerDescription struct {
	ID         int       `boil:"id" json:"id" toml:"id" yaml:"id"`
	LecturerID int       `boil:"lecturer_id" json:"lecturer_id" toml:"lecturer_id" yaml:"lecturer_id"`
	Desc       string    `boil:"desc" json:"desc" toml:"desc" yaml:"desc"`
	LangID     string    `boil:"lang_id" json:"lang_id" toml:"lang_id" yaml:"lang_id"`
	CreatedAt  null.Time `boil:"created_at" json:"created_at,omitempty" toml:"created_at" yaml:"created_at,omitempty"`
	UpdatedAt  null.Time `boil:"updated_at" json:"updated_at,omitempty" toml:"updated_at" yaml:"updated_at,omitempty"`

	R *lecturerDescriptionR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L lecturerDescriptionL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// lecturerDescriptionR is where relationships are stored.
type lecturerDescriptionR struct {
	Lecturer *Lecturer
	Lang     *Language
}

// lecturerDescriptionL is where Load methods for each relationship are stored.
type lecturerDescriptionL struct{}

var (
	lecturerDescriptionColumns               = []string{"id", "lecturer_id", "desc", "lang_id", "created_at", "updated_at"}
	lecturerDescriptionColumnsWithoutDefault = []string{"created_at", "updated_at"}
	lecturerDescriptionColumnsWithDefault    = []string{"id", "lecturer_id", "desc", "lang_id"}
	lecturerDescriptionPrimaryKeyColumns     = []string{"id"}
)

type (
	// LecturerDescriptionSlice is an alias for a slice of pointers to LecturerDescription.
	// This should generally be used opposed to []LecturerDescription.
	LecturerDescriptionSlice []*LecturerDescription

	lecturerDescriptionQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	lecturerDescriptionType                 = reflect.TypeOf(&LecturerDescription{})
	lecturerDescriptionMapping              = queries.MakeStructMapping(lecturerDescriptionType)
	lecturerDescriptionPrimaryKeyMapping, _ = queries.BindMapping(lecturerDescriptionType, lecturerDescriptionMapping, lecturerDescriptionPrimaryKeyColumns)
	lecturerDescriptionInsertCacheMut       sync.RWMutex
	lecturerDescriptionInsertCache          = make(map[string]insertCache)
	lecturerDescriptionUpdateCacheMut       sync.RWMutex
	lecturerDescriptionUpdateCache          = make(map[string]updateCache)
	lecturerDescriptionUpsertCacheMut       sync.RWMutex
	lecturerDescriptionUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single lecturerDescription record from the query, and panics on error.
func (q lecturerDescriptionQuery) OneP() *LecturerDescription {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single lecturerDescription record from the query.
func (q lecturerDescriptionQuery) One() (*LecturerDescription, error) {
	o := &LecturerDescription{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for lecturer_descriptions")
	}

	return o, nil
}

// AllP returns all LecturerDescription records from the query, and panics on error.
func (q lecturerDescriptionQuery) AllP() LecturerDescriptionSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all LecturerDescription records from the query.
func (q lecturerDescriptionQuery) All() (LecturerDescriptionSlice, error) {
	var o LecturerDescriptionSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to LecturerDescription slice")
	}

	return o, nil
}

// CountP returns the count of all LecturerDescription records in the query, and panics on error.
func (q lecturerDescriptionQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all LecturerDescription records in the query.
func (q lecturerDescriptionQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count lecturer_descriptions rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q lecturerDescriptionQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q lecturerDescriptionQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if lecturer_descriptions exists")
	}

	return count > 0, nil
}

// LecturerG pointed to by the foreign key.
func (o *LecturerDescription) LecturerG(mods ...qm.QueryMod) lecturerQuery {
	return o.Lecturer(boil.GetDB(), mods...)
}

// Lecturer pointed to by the foreign key.
func (o *LecturerDescription) Lecturer(exec boil.Executor, mods ...qm.QueryMod) lecturerQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.LecturerID),
	}

	queryMods = append(queryMods, mods...)

	query := Lecturers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"lecturers\"")

	return query
}

// LangG pointed to by the foreign key.
func (o *LecturerDescription) LangG(mods ...qm.QueryMod) languageQuery {
	return o.Lang(boil.GetDB(), mods...)
}

// Lang pointed to by the foreign key.
func (o *LecturerDescription) Lang(exec boil.Executor, mods ...qm.QueryMod) languageQuery {
	queryMods := []qm.QueryMod{
		qm.Where("code3=?", o.LangID),
	}

	queryMods = append(queryMods, mods...)

	query := Languages(exec, queryMods...)
	queries.SetFrom(query.Query, "\"languages\"")

	return query
}

// LoadLecturer allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (lecturerDescriptionL) LoadLecturer(e boil.Executor, singular bool, maybeLecturerDescription interface{}) error {
	var slice []*LecturerDescription
	var object *LecturerDescription

	count := 1
	if singular {
		object = maybeLecturerDescription.(*LecturerDescription)
	} else {
		slice = *maybeLecturerDescription.(*LecturerDescriptionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &lecturerDescriptionR{}
		}
		args[0] = object.LecturerID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &lecturerDescriptionR{}
			}
			args[i] = obj.LecturerID
		}
	}

	query := fmt.Sprintf(
		"select * from \"lecturers\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Lecturer")
	}
	defer results.Close()

	var resultSlice []*Lecturer
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Lecturer")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Lecturer = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.LecturerID == foreign.ID {
				local.R.Lecturer = foreign
				break
			}
		}
	}

	return nil
}

// LoadLang allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (lecturerDescriptionL) LoadLang(e boil.Executor, singular bool, maybeLecturerDescription interface{}) error {
	var slice []*LecturerDescription
	var object *LecturerDescription

	count := 1
	if singular {
		object = maybeLecturerDescription.(*LecturerDescription)
	} else {
		slice = *maybeLecturerDescription.(*LecturerDescriptionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &lecturerDescriptionR{}
		}
		args[0] = object.LangID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &lecturerDescriptionR{}
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
			if local.LangID == foreign.Code3.String {
				local.R.Lang = foreign
				break
			}
		}
	}

	return nil
}

// SetLecturer of the lecturer_description to the related item.
// Sets o.R.Lecturer to related.
// Adds o to related.R.LecturerDescriptions.
func (o *LecturerDescription) SetLecturer(exec boil.Executor, insert bool, related *Lecturer) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"lecturer_descriptions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"lecturer_id"}),
		strmangle.WhereClause("\"", "\"", 2, lecturerDescriptionPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.LecturerID = related.ID

	if o.R == nil {
		o.R = &lecturerDescriptionR{
			Lecturer: related,
		}
	} else {
		o.R.Lecturer = related
	}

	if related.R == nil {
		related.R = &lecturerR{
			LecturerDescriptions: LecturerDescriptionSlice{o},
		}
	} else {
		related.R.LecturerDescriptions = append(related.R.LecturerDescriptions, o)
	}

	return nil
}

// SetLang of the lecturer_description to the related item.
// Sets o.R.Lang to related.
// Adds o to related.R.LangLecturerDescriptions.
func (o *LecturerDescription) SetLang(exec boil.Executor, insert bool, related *Language) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"lecturer_descriptions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
		strmangle.WhereClause("\"", "\"", 2, lecturerDescriptionPrimaryKeyColumns),
	)
	values := []interface{}{related.Code3, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.LangID = related.Code3.String

	if o.R == nil {
		o.R = &lecturerDescriptionR{
			Lang: related,
		}
	} else {
		o.R.Lang = related
	}

	if related.R == nil {
		related.R = &languageR{
			LangLecturerDescriptions: LecturerDescriptionSlice{o},
		}
	} else {
		related.R.LangLecturerDescriptions = append(related.R.LangLecturerDescriptions, o)
	}

	return nil
}

// LecturerDescriptionsG retrieves all records.
func LecturerDescriptionsG(mods ...qm.QueryMod) lecturerDescriptionQuery {
	return LecturerDescriptions(boil.GetDB(), mods...)
}

// LecturerDescriptions retrieves all the records using an executor.
func LecturerDescriptions(exec boil.Executor, mods ...qm.QueryMod) lecturerDescriptionQuery {
	mods = append(mods, qm.From("\"lecturer_descriptions\""))
	return lecturerDescriptionQuery{NewQuery(exec, mods...)}
}

// FindLecturerDescriptionG retrieves a single record by ID.
func FindLecturerDescriptionG(id int, selectCols ...string) (*LecturerDescription, error) {
	return FindLecturerDescription(boil.GetDB(), id, selectCols...)
}

// FindLecturerDescriptionGP retrieves a single record by ID, and panics on error.
func FindLecturerDescriptionGP(id int, selectCols ...string) *LecturerDescription {
	retobj, err := FindLecturerDescription(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindLecturerDescription retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindLecturerDescription(exec boil.Executor, id int, selectCols ...string) (*LecturerDescription, error) {
	lecturerDescriptionObj := &LecturerDescription{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"lecturer_descriptions\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(lecturerDescriptionObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from lecturer_descriptions")
	}

	return lecturerDescriptionObj, nil
}

// FindLecturerDescriptionP retrieves a single record by ID with an executor, and panics on error.
func FindLecturerDescriptionP(exec boil.Executor, id int, selectCols ...string) *LecturerDescription {
	retobj, err := FindLecturerDescription(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *LecturerDescription) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *LecturerDescription) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *LecturerDescription) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *LecturerDescription) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no lecturer_descriptions provided for insertion")
	}

	var err error
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.Time.IsZero() {
		o.CreatedAt.Time = currTime
		o.CreatedAt.Valid = true
	}
	if o.UpdatedAt.Time.IsZero() {
		o.UpdatedAt.Time = currTime
		o.UpdatedAt.Valid = true
	}

	nzDefaults := queries.NonZeroDefaultSet(lecturerDescriptionColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	lecturerDescriptionInsertCacheMut.RLock()
	cache, cached := lecturerDescriptionInsertCache[key]
	lecturerDescriptionInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			lecturerDescriptionColumns,
			lecturerDescriptionColumnsWithDefault,
			lecturerDescriptionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(lecturerDescriptionType, lecturerDescriptionMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(lecturerDescriptionType, lecturerDescriptionMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"lecturer_descriptions\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into lecturer_descriptions")
	}

	if !cached {
		lecturerDescriptionInsertCacheMut.Lock()
		lecturerDescriptionInsertCache[key] = cache
		lecturerDescriptionInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single LecturerDescription record. See Update for
// whitelist behavior description.
func (o *LecturerDescription) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single LecturerDescription record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *LecturerDescription) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the LecturerDescription, and panics on error.
// See Update for whitelist behavior description.
func (o *LecturerDescription) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the LecturerDescription.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *LecturerDescription) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	var err error
	key := makeCacheKey(whitelist, nil)
	lecturerDescriptionUpdateCacheMut.RLock()
	cache, cached := lecturerDescriptionUpdateCache[key]
	lecturerDescriptionUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(lecturerDescriptionColumns, lecturerDescriptionPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update lecturer_descriptions, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"lecturer_descriptions\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, lecturerDescriptionPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(lecturerDescriptionType, lecturerDescriptionMapping, append(wl, lecturerDescriptionPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update lecturer_descriptions row")
	}

	if !cached {
		lecturerDescriptionUpdateCacheMut.Lock()
		lecturerDescriptionUpdateCache[key] = cache
		lecturerDescriptionUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q lecturerDescriptionQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q lecturerDescriptionQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for lecturer_descriptions")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o LecturerDescriptionSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o LecturerDescriptionSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o LecturerDescriptionSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o LecturerDescriptionSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), lecturerDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"lecturer_descriptions\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(lecturerDescriptionPrimaryKeyColumns), len(colNames)+1, len(lecturerDescriptionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in lecturerDescription slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *LecturerDescription) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *LecturerDescription) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *LecturerDescription) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *LecturerDescription) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no lecturer_descriptions provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.Time.IsZero() {
		o.CreatedAt.Time = currTime
		o.CreatedAt.Valid = true
	}
	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	nzDefaults := queries.NonZeroDefaultSet(lecturerDescriptionColumnsWithDefault, o)

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

	lecturerDescriptionUpsertCacheMut.RLock()
	cache, cached := lecturerDescriptionUpsertCache[key]
	lecturerDescriptionUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			lecturerDescriptionColumns,
			lecturerDescriptionColumnsWithDefault,
			lecturerDescriptionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			lecturerDescriptionColumns,
			lecturerDescriptionPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert lecturer_descriptions, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(lecturerDescriptionPrimaryKeyColumns))
			copy(conflict, lecturerDescriptionPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"lecturer_descriptions\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(lecturerDescriptionType, lecturerDescriptionMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(lecturerDescriptionType, lecturerDescriptionMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert lecturer_descriptions")
	}

	if !cached {
		lecturerDescriptionUpsertCacheMut.Lock()
		lecturerDescriptionUpsertCache[key] = cache
		lecturerDescriptionUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single LecturerDescription record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *LecturerDescription) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single LecturerDescription record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *LecturerDescription) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no LecturerDescription provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single LecturerDescription record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *LecturerDescription) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single LecturerDescription record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *LecturerDescription) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no LecturerDescription provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), lecturerDescriptionPrimaryKeyMapping)
	sql := "DELETE FROM \"lecturer_descriptions\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from lecturer_descriptions")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q lecturerDescriptionQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q lecturerDescriptionQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no lecturerDescriptionQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from lecturer_descriptions")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o LecturerDescriptionSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o LecturerDescriptionSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no LecturerDescription slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o LecturerDescriptionSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o LecturerDescriptionSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no LecturerDescription slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), lecturerDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"lecturer_descriptions\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, lecturerDescriptionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(lecturerDescriptionPrimaryKeyColumns), 1, len(lecturerDescriptionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from lecturerDescription slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *LecturerDescription) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *LecturerDescription) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *LecturerDescription) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no LecturerDescription provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *LecturerDescription) Reload(exec boil.Executor) error {
	ret, err := FindLecturerDescription(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *LecturerDescriptionSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *LecturerDescriptionSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *LecturerDescriptionSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty LecturerDescriptionSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *LecturerDescriptionSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	lecturerDescriptions := LecturerDescriptionSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), lecturerDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"lecturer_descriptions\".* FROM \"lecturer_descriptions\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, lecturerDescriptionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(lecturerDescriptionPrimaryKeyColumns), 1, len(lecturerDescriptionPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&lecturerDescriptions)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in LecturerDescriptionSlice")
	}

	*o = lecturerDescriptions

	return nil
}

// LecturerDescriptionExists checks if the LecturerDescription row exists.
func LecturerDescriptionExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"lecturer_descriptions\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if lecturer_descriptions exists")
	}

	return exists, nil
}

// LecturerDescriptionExistsG checks if the LecturerDescription row exists.
func LecturerDescriptionExistsG(id int) (bool, error) {
	return LecturerDescriptionExists(boil.GetDB(), id)
}

// LecturerDescriptionExistsGP checks if the LecturerDescription row exists. Panics on error.
func LecturerDescriptionExistsGP(id int) bool {
	e, err := LecturerDescriptionExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// LecturerDescriptionExistsP checks if the LecturerDescription row exists. Panics on error.
func LecturerDescriptionExistsP(exec boil.Executor, id int) bool {
	e, err := LecturerDescriptionExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

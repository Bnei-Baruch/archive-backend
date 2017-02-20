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

// Lecturer is an object representing the database table.
type Lecturer struct {
	ID        int       `boil:"id" json:"id" toml:"id" yaml:"id"`
	Name      string    `boil:"name" json:"name" toml:"name" yaml:"name"`
	CreatedAt null.Time `boil:"created_at" json:"created_at,omitempty" toml:"created_at" yaml:"created_at,omitempty"`
	UpdatedAt null.Time `boil:"updated_at" json:"updated_at,omitempty" toml:"updated_at" yaml:"updated_at,omitempty"`
	Ordnum    int       `boil:"ordnum" json:"ordnum" toml:"ordnum" yaml:"ordnum"`

	R *lecturerR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L lecturerL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// lecturerR is where relationships are stored.
type lecturerR struct {
	LecturerDescriptions LecturerDescriptionSlice
	Containers           ContainerSlice
}

// lecturerL is where Load methods for each relationship are stored.
type lecturerL struct{}

var (
	lecturerColumns               = []string{"id", "name", "created_at", "updated_at", "ordnum"}
	lecturerColumnsWithoutDefault = []string{"created_at", "updated_at"}
	lecturerColumnsWithDefault    = []string{"id", "name", "ordnum"}
	lecturerPrimaryKeyColumns     = []string{"id"}
)

type (
	// LecturerSlice is an alias for a slice of pointers to Lecturer.
	// This should generally be used opposed to []Lecturer.
	LecturerSlice []*Lecturer

	lecturerQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	lecturerType                 = reflect.TypeOf(&Lecturer{})
	lecturerMapping              = queries.MakeStructMapping(lecturerType)
	lecturerPrimaryKeyMapping, _ = queries.BindMapping(lecturerType, lecturerMapping, lecturerPrimaryKeyColumns)
	lecturerInsertCacheMut       sync.RWMutex
	lecturerInsertCache          = make(map[string]insertCache)
	lecturerUpdateCacheMut       sync.RWMutex
	lecturerUpdateCache          = make(map[string]updateCache)
	lecturerUpsertCacheMut       sync.RWMutex
	lecturerUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single lecturer record from the query, and panics on error.
func (q lecturerQuery) OneP() *Lecturer {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single lecturer record from the query.
func (q lecturerQuery) One() (*Lecturer, error) {
	o := &Lecturer{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for lecturers")
	}

	return o, nil
}

// AllP returns all Lecturer records from the query, and panics on error.
func (q lecturerQuery) AllP() LecturerSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all Lecturer records from the query.
func (q lecturerQuery) All() (LecturerSlice, error) {
	var o LecturerSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to Lecturer slice")
	}

	return o, nil
}

// CountP returns the count of all Lecturer records in the query, and panics on error.
func (q lecturerQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all Lecturer records in the query.
func (q lecturerQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count lecturers rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q lecturerQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q lecturerQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if lecturers exists")
	}

	return count > 0, nil
}

// LecturerDescriptionsG retrieves all the lecturer_description's lecturer descriptions.
func (o *Lecturer) LecturerDescriptionsG(mods ...qm.QueryMod) lecturerDescriptionQuery {
	return o.LecturerDescriptions(boil.GetDB(), mods...)
}

// LecturerDescriptions retrieves all the lecturer_description's lecturer descriptions with an executor.
func (o *Lecturer) LecturerDescriptions(exec boil.Executor, mods ...qm.QueryMod) lecturerDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lecturer_id\"=?", o.ID),
	)

	query := LecturerDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"lecturer_descriptions\" as \"a\"")
	return query
}

// ContainersG retrieves all the container's containers.
func (o *Lecturer) ContainersG(mods ...qm.QueryMod) containerQuery {
	return o.Containers(boil.GetDB(), mods...)
}

// Containers retrieves all the container's containers with an executor.
func (o *Lecturer) Containers(exec boil.Executor, mods ...qm.QueryMod) containerQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lecturer_id\"=?", o.ID),
	)

	query := Containers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"containers\" as \"a\"")
	return query
}

// LoadLecturerDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (lecturerL) LoadLecturerDescriptions(e boil.Executor, singular bool, maybeLecturer interface{}) error {
	var slice []*Lecturer
	var object *Lecturer

	count := 1
	if singular {
		object = maybeLecturer.(*Lecturer)
	} else {
		slice = *maybeLecturer.(*LecturerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &lecturerR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &lecturerR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"lecturer_descriptions\" where \"lecturer_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load lecturer_descriptions")
	}
	defer results.Close()

	var resultSlice []*LecturerDescription
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice lecturer_descriptions")
	}

	if singular {
		object.R.LecturerDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.LecturerID {
				local.R.LecturerDescriptions = append(local.R.LecturerDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// LoadContainers allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (lecturerL) LoadContainers(e boil.Executor, singular bool, maybeLecturer interface{}) error {
	var slice []*Lecturer
	var object *Lecturer

	count := 1
	if singular {
		object = maybeLecturer.(*Lecturer)
	} else {
		slice = *maybeLecturer.(*LecturerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &lecturerR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &lecturerR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"containers\" where \"lecturer_id\" in (%s)",
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
			if local.ID == foreign.LecturerID.Int {
				local.R.Containers = append(local.R.Containers, foreign)
				break
			}
		}
	}

	return nil
}

// AddLecturerDescriptions adds the given related objects to the existing relationships
// of the lecturer, optionally inserting them as new records.
// Appends related to o.R.LecturerDescriptions.
// Sets related.R.Lecturer appropriately.
func (o *Lecturer) AddLecturerDescriptions(exec boil.Executor, insert bool, related ...*LecturerDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LecturerID = o.ID
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"lecturer_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lecturer_id"}),
				strmangle.WhereClause("\"", "\"", 2, lecturerDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LecturerID = o.ID
		}
	}

	if o.R == nil {
		o.R = &lecturerR{
			LecturerDescriptions: related,
		}
	} else {
		o.R.LecturerDescriptions = append(o.R.LecturerDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &lecturerDescriptionR{
				Lecturer: o,
			}
		} else {
			rel.R.Lecturer = o
		}
	}
	return nil
}

// AddContainers adds the given related objects to the existing relationships
// of the lecturer, optionally inserting them as new records.
// Appends related to o.R.Containers.
// Sets related.R.Lecturer appropriately.
func (o *Lecturer) AddContainers(exec boil.Executor, insert bool, related ...*Container) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LecturerID.Int = o.ID
			rel.LecturerID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"containers\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lecturer_id"}),
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

			rel.LecturerID.Int = o.ID
			rel.LecturerID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &lecturerR{
			Containers: related,
		}
	} else {
		o.R.Containers = append(o.R.Containers, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerR{
				Lecturer: o,
			}
		} else {
			rel.R.Lecturer = o
		}
	}
	return nil
}

// SetContainers removes all previously related items of the
// lecturer replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Lecturer's Containers accordingly.
// Replaces o.R.Containers with related.
// Sets related.R.Lecturer's Containers accordingly.
func (o *Lecturer) SetContainers(exec boil.Executor, insert bool, related ...*Container) error {
	query := "update \"containers\" set \"lecturer_id\" = null where \"lecturer_id\" = $1"
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
			rel.LecturerID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Lecturer = nil
		}

		o.R.Containers = nil
	}
	return o.AddContainers(exec, insert, related...)
}

// RemoveContainers relationships from objects passed in.
// Removes related items from R.Containers (uses pointer comparison, removal does not keep order)
// Sets related.R.Lecturer.
func (o *Lecturer) RemoveContainers(exec boil.Executor, related ...*Container) error {
	var err error
	for _, rel := range related {
		rel.LecturerID.Valid = false
		if rel.R != nil {
			rel.R.Lecturer = nil
		}
		if err = rel.Update(exec, "lecturer_id"); err != nil {
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

// LecturersG retrieves all records.
func LecturersG(mods ...qm.QueryMod) lecturerQuery {
	return Lecturers(boil.GetDB(), mods...)
}

// Lecturers retrieves all the records using an executor.
func Lecturers(exec boil.Executor, mods ...qm.QueryMod) lecturerQuery {
	mods = append(mods, qm.From("\"lecturers\""))
	return lecturerQuery{NewQuery(exec, mods...)}
}

// FindLecturerG retrieves a single record by ID.
func FindLecturerG(id int, selectCols ...string) (*Lecturer, error) {
	return FindLecturer(boil.GetDB(), id, selectCols...)
}

// FindLecturerGP retrieves a single record by ID, and panics on error.
func FindLecturerGP(id int, selectCols ...string) *Lecturer {
	retobj, err := FindLecturer(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindLecturer retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindLecturer(exec boil.Executor, id int, selectCols ...string) (*Lecturer, error) {
	lecturerObj := &Lecturer{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"lecturers\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(lecturerObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from lecturers")
	}

	return lecturerObj, nil
}

// FindLecturerP retrieves a single record by ID with an executor, and panics on error.
func FindLecturerP(exec boil.Executor, id int, selectCols ...string) *Lecturer {
	retobj, err := FindLecturer(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *Lecturer) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *Lecturer) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *Lecturer) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *Lecturer) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no lecturers provided for insertion")
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

	nzDefaults := queries.NonZeroDefaultSet(lecturerColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	lecturerInsertCacheMut.RLock()
	cache, cached := lecturerInsertCache[key]
	lecturerInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			lecturerColumns,
			lecturerColumnsWithDefault,
			lecturerColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(lecturerType, lecturerMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(lecturerType, lecturerMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"lecturers\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into lecturers")
	}

	if !cached {
		lecturerInsertCacheMut.Lock()
		lecturerInsertCache[key] = cache
		lecturerInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single Lecturer record. See Update for
// whitelist behavior description.
func (o *Lecturer) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single Lecturer record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *Lecturer) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the Lecturer, and panics on error.
// See Update for whitelist behavior description.
func (o *Lecturer) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the Lecturer.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *Lecturer) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	var err error
	key := makeCacheKey(whitelist, nil)
	lecturerUpdateCacheMut.RLock()
	cache, cached := lecturerUpdateCache[key]
	lecturerUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(lecturerColumns, lecturerPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update lecturers, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"lecturers\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, lecturerPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(lecturerType, lecturerMapping, append(wl, lecturerPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update lecturers row")
	}

	if !cached {
		lecturerUpdateCacheMut.Lock()
		lecturerUpdateCache[key] = cache
		lecturerUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q lecturerQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q lecturerQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for lecturers")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o LecturerSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o LecturerSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o LecturerSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o LecturerSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), lecturerPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"lecturers\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(lecturerPrimaryKeyColumns), len(colNames)+1, len(lecturerPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in lecturer slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *Lecturer) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *Lecturer) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *Lecturer) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *Lecturer) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no lecturers provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.Time.IsZero() {
		o.CreatedAt.Time = currTime
		o.CreatedAt.Valid = true
	}
	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	nzDefaults := queries.NonZeroDefaultSet(lecturerColumnsWithDefault, o)

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

	lecturerUpsertCacheMut.RLock()
	cache, cached := lecturerUpsertCache[key]
	lecturerUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			lecturerColumns,
			lecturerColumnsWithDefault,
			lecturerColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			lecturerColumns,
			lecturerPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert lecturers, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(lecturerPrimaryKeyColumns))
			copy(conflict, lecturerPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"lecturers\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(lecturerType, lecturerMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(lecturerType, lecturerMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert lecturers")
	}

	if !cached {
		lecturerUpsertCacheMut.Lock()
		lecturerUpsertCache[key] = cache
		lecturerUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single Lecturer record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Lecturer) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single Lecturer record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *Lecturer) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no Lecturer provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single Lecturer record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Lecturer) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single Lecturer record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Lecturer) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Lecturer provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), lecturerPrimaryKeyMapping)
	sql := "DELETE FROM \"lecturers\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from lecturers")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q lecturerQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q lecturerQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no lecturerQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from lecturers")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o LecturerSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o LecturerSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no Lecturer slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o LecturerSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o LecturerSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Lecturer slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), lecturerPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"lecturers\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, lecturerPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(lecturerPrimaryKeyColumns), 1, len(lecturerPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from lecturer slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *Lecturer) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *Lecturer) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *Lecturer) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no Lecturer provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Lecturer) Reload(exec boil.Executor) error {
	ret, err := FindLecturer(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *LecturerSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *LecturerSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *LecturerSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty LecturerSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *LecturerSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	lecturers := LecturerSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), lecturerPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"lecturers\".* FROM \"lecturers\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, lecturerPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(lecturerPrimaryKeyColumns), 1, len(lecturerPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&lecturers)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in LecturerSlice")
	}

	*o = lecturers

	return nil
}

// LecturerExists checks if the Lecturer row exists.
func LecturerExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"lecturers\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if lecturers exists")
	}

	return exists, nil
}

// LecturerExistsG checks if the Lecturer row exists.
func LecturerExistsG(id int) (bool, error) {
	return LecturerExists(boil.GetDB(), id)
}

// LecturerExistsGP checks if the Lecturer row exists. Panics on error.
func LecturerExistsGP(id int) bool {
	e, err := LecturerExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// LecturerExistsP checks if the Lecturer row exists. Panics on error.
func LecturerExistsP(exec boil.Executor, id int) bool {
	e, err := LecturerExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

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

// VirtualLesson is an object representing the database table.
type VirtualLesson struct {
	ID        int       `boil:"id" json:"id" toml:"id" yaml:"id"`
	FilmDate  null.Time `boil:"film_date" json:"film_date,omitempty" toml:"film_date" yaml:"film_date,omitempty"`
	CreatedAt time.Time `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	UpdatedAt time.Time `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`
	UserID    null.Int  `boil:"user_id" json:"user_id,omitempty" toml:"user_id" yaml:"user_id,omitempty"`

	R *virtualLessonR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L virtualLessonL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// virtualLessonR is where relationships are stored.
type virtualLessonR struct {
	User       *User
	Containers ContainerSlice
}

// virtualLessonL is where Load methods for each relationship are stored.
type virtualLessonL struct{}

var (
	virtualLessonColumns               = []string{"id", "film_date", "created_at", "updated_at", "user_id"}
	virtualLessonColumnsWithoutDefault = []string{"film_date", "created_at", "updated_at", "user_id"}
	virtualLessonColumnsWithDefault    = []string{"id"}
	virtualLessonPrimaryKeyColumns     = []string{"id"}
)

type (
	// VirtualLessonSlice is an alias for a slice of pointers to VirtualLesson.
	// This should generally be used opposed to []VirtualLesson.
	VirtualLessonSlice []*VirtualLesson

	virtualLessonQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	virtualLessonType                 = reflect.TypeOf(&VirtualLesson{})
	virtualLessonMapping              = queries.MakeStructMapping(virtualLessonType)
	virtualLessonPrimaryKeyMapping, _ = queries.BindMapping(virtualLessonType, virtualLessonMapping, virtualLessonPrimaryKeyColumns)
	virtualLessonInsertCacheMut       sync.RWMutex
	virtualLessonInsertCache          = make(map[string]insertCache)
	virtualLessonUpdateCacheMut       sync.RWMutex
	virtualLessonUpdateCache          = make(map[string]updateCache)
	virtualLessonUpsertCacheMut       sync.RWMutex
	virtualLessonUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single virtualLesson record from the query, and panics on error.
func (q virtualLessonQuery) OneP() *VirtualLesson {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single virtualLesson record from the query.
func (q virtualLessonQuery) One() (*VirtualLesson, error) {
	o := &VirtualLesson{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for virtual_lessons")
	}

	return o, nil
}

// AllP returns all VirtualLesson records from the query, and panics on error.
func (q virtualLessonQuery) AllP() VirtualLessonSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all VirtualLesson records from the query.
func (q virtualLessonQuery) All() (VirtualLessonSlice, error) {
	var o VirtualLessonSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to VirtualLesson slice")
	}

	return o, nil
}

// CountP returns the count of all VirtualLesson records in the query, and panics on error.
func (q virtualLessonQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all VirtualLesson records in the query.
func (q virtualLessonQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count virtual_lessons rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q virtualLessonQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q virtualLessonQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if virtual_lessons exists")
	}

	return count > 0, nil
}

// UserG pointed to by the foreign key.
func (o *VirtualLesson) UserG(mods ...qm.QueryMod) userQuery {
	return o.User(boil.GetDB(), mods...)
}

// User pointed to by the foreign key.
func (o *VirtualLesson) User(exec boil.Executor, mods ...qm.QueryMod) userQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.UserID),
	}

	queryMods = append(queryMods, mods...)

	query := Users(exec, queryMods...)
	queries.SetFrom(query.Query, "\"users\"")

	return query
}

// ContainersG retrieves all the container's containers.
func (o *VirtualLesson) ContainersG(mods ...qm.QueryMod) containerQuery {
	return o.Containers(boil.GetDB(), mods...)
}

// Containers retrieves all the container's containers with an executor.
func (o *VirtualLesson) Containers(exec boil.Executor, mods ...qm.QueryMod) containerQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"virtual_lesson_id\"=?", o.ID),
	)

	query := Containers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"containers\" as \"a\"")
	return query
}

// LoadUser allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (virtualLessonL) LoadUser(e boil.Executor, singular bool, maybeVirtualLesson interface{}) error {
	var slice []*VirtualLesson
	var object *VirtualLesson

	count := 1
	if singular {
		object = maybeVirtualLesson.(*VirtualLesson)
	} else {
		slice = *maybeVirtualLesson.(*VirtualLessonSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &virtualLessonR{}
		}
		args[0] = object.UserID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &virtualLessonR{}
			}
			args[i] = obj.UserID
		}
	}

	query := fmt.Sprintf(
		"select * from \"users\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load User")
	}
	defer results.Close()

	var resultSlice []*User
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice User")
	}

	if singular && len(resultSlice) != 0 {
		object.R.User = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.UserID.Int == foreign.ID {
				local.R.User = foreign
				break
			}
		}
	}

	return nil
}

// LoadContainers allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (virtualLessonL) LoadContainers(e boil.Executor, singular bool, maybeVirtualLesson interface{}) error {
	var slice []*VirtualLesson
	var object *VirtualLesson

	count := 1
	if singular {
		object = maybeVirtualLesson.(*VirtualLesson)
	} else {
		slice = *maybeVirtualLesson.(*VirtualLessonSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &virtualLessonR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &virtualLessonR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"containers\" where \"virtual_lesson_id\" in (%s)",
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
			if local.ID == foreign.VirtualLessonID.Int {
				local.R.Containers = append(local.R.Containers, foreign)
				break
			}
		}
	}

	return nil
}

// SetUser of the virtual_lesson to the related item.
// Sets o.R.User to related.
// Adds o to related.R.VirtualLessons.
func (o *VirtualLesson) SetUser(exec boil.Executor, insert bool, related *User) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"virtual_lessons\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
		strmangle.WhereClause("\"", "\"", 2, virtualLessonPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.UserID.Int = related.ID
	o.UserID.Valid = true

	if o.R == nil {
		o.R = &virtualLessonR{
			User: related,
		}
	} else {
		o.R.User = related
	}

	if related.R == nil {
		related.R = &userR{
			VirtualLessons: VirtualLessonSlice{o},
		}
	} else {
		related.R.VirtualLessons = append(related.R.VirtualLessons, o)
	}

	return nil
}

// RemoveUser relationship.
// Sets o.R.User to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *VirtualLesson) RemoveUser(exec boil.Executor, related *User) error {
	var err error

	o.UserID.Valid = false
	if err = o.Update(exec, "user_id"); err != nil {
		o.UserID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.User = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.VirtualLessons {
		if o.UserID.Int != ri.UserID.Int {
			continue
		}

		ln := len(related.R.VirtualLessons)
		if ln > 1 && i < ln-1 {
			related.R.VirtualLessons[i] = related.R.VirtualLessons[ln-1]
		}
		related.R.VirtualLessons = related.R.VirtualLessons[:ln-1]
		break
	}
	return nil
}

// AddContainers adds the given related objects to the existing relationships
// of the virtual_lesson, optionally inserting them as new records.
// Appends related to o.R.Containers.
// Sets related.R.VirtualLesson appropriately.
func (o *VirtualLesson) AddContainers(exec boil.Executor, insert bool, related ...*Container) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.VirtualLessonID.Int = o.ID
			rel.VirtualLessonID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"containers\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"virtual_lesson_id"}),
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

			rel.VirtualLessonID.Int = o.ID
			rel.VirtualLessonID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &virtualLessonR{
			Containers: related,
		}
	} else {
		o.R.Containers = append(o.R.Containers, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerR{
				VirtualLesson: o,
			}
		} else {
			rel.R.VirtualLesson = o
		}
	}
	return nil
}

// SetContainers removes all previously related items of the
// virtual_lesson replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.VirtualLesson's Containers accordingly.
// Replaces o.R.Containers with related.
// Sets related.R.VirtualLesson's Containers accordingly.
func (o *VirtualLesson) SetContainers(exec boil.Executor, insert bool, related ...*Container) error {
	query := "update \"containers\" set \"virtual_lesson_id\" = null where \"virtual_lesson_id\" = $1"
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
			rel.VirtualLessonID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.VirtualLesson = nil
		}

		o.R.Containers = nil
	}
	return o.AddContainers(exec, insert, related...)
}

// RemoveContainers relationships from objects passed in.
// Removes related items from R.Containers (uses pointer comparison, removal does not keep order)
// Sets related.R.VirtualLesson.
func (o *VirtualLesson) RemoveContainers(exec boil.Executor, related ...*Container) error {
	var err error
	for _, rel := range related {
		rel.VirtualLessonID.Valid = false
		if rel.R != nil {
			rel.R.VirtualLesson = nil
		}
		if err = rel.Update(exec, "virtual_lesson_id"); err != nil {
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

// VirtualLessonsG retrieves all records.
func VirtualLessonsG(mods ...qm.QueryMod) virtualLessonQuery {
	return VirtualLessons(boil.GetDB(), mods...)
}

// VirtualLessons retrieves all the records using an executor.
func VirtualLessons(exec boil.Executor, mods ...qm.QueryMod) virtualLessonQuery {
	mods = append(mods, qm.From("\"virtual_lessons\""))
	return virtualLessonQuery{NewQuery(exec, mods...)}
}

// FindVirtualLessonG retrieves a single record by ID.
func FindVirtualLessonG(id int, selectCols ...string) (*VirtualLesson, error) {
	return FindVirtualLesson(boil.GetDB(), id, selectCols...)
}

// FindVirtualLessonGP retrieves a single record by ID, and panics on error.
func FindVirtualLessonGP(id int, selectCols ...string) *VirtualLesson {
	retobj, err := FindVirtualLesson(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindVirtualLesson retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindVirtualLesson(exec boil.Executor, id int, selectCols ...string) (*VirtualLesson, error) {
	virtualLessonObj := &VirtualLesson{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"virtual_lessons\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(virtualLessonObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from virtual_lessons")
	}

	return virtualLessonObj, nil
}

// FindVirtualLessonP retrieves a single record by ID with an executor, and panics on error.
func FindVirtualLessonP(exec boil.Executor, id int, selectCols ...string) *VirtualLesson {
	retobj, err := FindVirtualLesson(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *VirtualLesson) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *VirtualLesson) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *VirtualLesson) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *VirtualLesson) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no virtual_lessons provided for insertion")
	}

	var err error
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = currTime
	}

	nzDefaults := queries.NonZeroDefaultSet(virtualLessonColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	virtualLessonInsertCacheMut.RLock()
	cache, cached := virtualLessonInsertCache[key]
	virtualLessonInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			virtualLessonColumns,
			virtualLessonColumnsWithDefault,
			virtualLessonColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(virtualLessonType, virtualLessonMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(virtualLessonType, virtualLessonMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"virtual_lessons\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into virtual_lessons")
	}

	if !cached {
		virtualLessonInsertCacheMut.Lock()
		virtualLessonInsertCache[key] = cache
		virtualLessonInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single VirtualLesson record. See Update for
// whitelist behavior description.
func (o *VirtualLesson) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single VirtualLesson record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *VirtualLesson) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the VirtualLesson, and panics on error.
// See Update for whitelist behavior description.
func (o *VirtualLesson) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the VirtualLesson.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *VirtualLesson) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt = currTime

	var err error
	key := makeCacheKey(whitelist, nil)
	virtualLessonUpdateCacheMut.RLock()
	cache, cached := virtualLessonUpdateCache[key]
	virtualLessonUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(virtualLessonColumns, virtualLessonPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update virtual_lessons, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"virtual_lessons\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, virtualLessonPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(virtualLessonType, virtualLessonMapping, append(wl, virtualLessonPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update virtual_lessons row")
	}

	if !cached {
		virtualLessonUpdateCacheMut.Lock()
		virtualLessonUpdateCache[key] = cache
		virtualLessonUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q virtualLessonQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q virtualLessonQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for virtual_lessons")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o VirtualLessonSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o VirtualLessonSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o VirtualLessonSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o VirtualLessonSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), virtualLessonPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"virtual_lessons\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(virtualLessonPrimaryKeyColumns), len(colNames)+1, len(virtualLessonPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in virtualLesson slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *VirtualLesson) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *VirtualLesson) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *VirtualLesson) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *VirtualLesson) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no virtual_lessons provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	o.UpdatedAt = currTime

	nzDefaults := queries.NonZeroDefaultSet(virtualLessonColumnsWithDefault, o)

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

	virtualLessonUpsertCacheMut.RLock()
	cache, cached := virtualLessonUpsertCache[key]
	virtualLessonUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			virtualLessonColumns,
			virtualLessonColumnsWithDefault,
			virtualLessonColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			virtualLessonColumns,
			virtualLessonPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert virtual_lessons, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(virtualLessonPrimaryKeyColumns))
			copy(conflict, virtualLessonPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"virtual_lessons\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(virtualLessonType, virtualLessonMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(virtualLessonType, virtualLessonMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert virtual_lessons")
	}

	if !cached {
		virtualLessonUpsertCacheMut.Lock()
		virtualLessonUpsertCache[key] = cache
		virtualLessonUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single VirtualLesson record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *VirtualLesson) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single VirtualLesson record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *VirtualLesson) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no VirtualLesson provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single VirtualLesson record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *VirtualLesson) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single VirtualLesson record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *VirtualLesson) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no VirtualLesson provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), virtualLessonPrimaryKeyMapping)
	sql := "DELETE FROM \"virtual_lessons\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from virtual_lessons")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q virtualLessonQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q virtualLessonQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no virtualLessonQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from virtual_lessons")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o VirtualLessonSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o VirtualLessonSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no VirtualLesson slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o VirtualLessonSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o VirtualLessonSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no VirtualLesson slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), virtualLessonPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"virtual_lessons\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, virtualLessonPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(virtualLessonPrimaryKeyColumns), 1, len(virtualLessonPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from virtualLesson slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *VirtualLesson) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *VirtualLesson) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *VirtualLesson) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no VirtualLesson provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *VirtualLesson) Reload(exec boil.Executor) error {
	ret, err := FindVirtualLesson(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *VirtualLessonSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *VirtualLessonSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *VirtualLessonSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty VirtualLessonSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *VirtualLessonSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	virtualLessons := VirtualLessonSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), virtualLessonPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"virtual_lessons\".* FROM \"virtual_lessons\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, virtualLessonPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(virtualLessonPrimaryKeyColumns), 1, len(virtualLessonPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&virtualLessons)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in VirtualLessonSlice")
	}

	*o = virtualLessons

	return nil
}

// VirtualLessonExists checks if the VirtualLesson row exists.
func VirtualLessonExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"virtual_lessons\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if virtual_lessons exists")
	}

	return exists, nil
}

// VirtualLessonExistsG checks if the VirtualLesson row exists.
func VirtualLessonExistsG(id int) (bool, error) {
	return VirtualLessonExists(boil.GetDB(), id)
}

// VirtualLessonExistsGP checks if the VirtualLesson row exists. Panics on error.
func VirtualLessonExistsGP(id int) bool {
	e, err := VirtualLessonExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// VirtualLessonExistsP checks if the VirtualLesson row exists. Panics on error.
func VirtualLessonExistsP(exec boil.Executor, id int) bool {
	e, err := VirtualLessonExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

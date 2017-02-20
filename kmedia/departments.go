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

// Department is an object representing the database table.
type Department struct {
	ID        int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	Name      null.String `boil:"name" json:"name,omitempty" toml:"name" yaml:"name,omitempty"`
	CreatedAt time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	UpdatedAt time.Time   `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`

	R *departmentR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L departmentL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// departmentR is where relationships are stored.
type departmentR struct {
	Users UserSlice
}

// departmentL is where Load methods for each relationship are stored.
type departmentL struct{}

var (
	departmentColumns               = []string{"id", "name", "created_at", "updated_at"}
	departmentColumnsWithoutDefault = []string{"name", "created_at", "updated_at"}
	departmentColumnsWithDefault    = []string{"id"}
	departmentPrimaryKeyColumns     = []string{"id"}
)

type (
	// DepartmentSlice is an alias for a slice of pointers to Department.
	// This should generally be used opposed to []Department.
	DepartmentSlice []*Department

	departmentQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	departmentType                 = reflect.TypeOf(&Department{})
	departmentMapping              = queries.MakeStructMapping(departmentType)
	departmentPrimaryKeyMapping, _ = queries.BindMapping(departmentType, departmentMapping, departmentPrimaryKeyColumns)
	departmentInsertCacheMut       sync.RWMutex
	departmentInsertCache          = make(map[string]insertCache)
	departmentUpdateCacheMut       sync.RWMutex
	departmentUpdateCache          = make(map[string]updateCache)
	departmentUpsertCacheMut       sync.RWMutex
	departmentUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single department record from the query, and panics on error.
func (q departmentQuery) OneP() *Department {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single department record from the query.
func (q departmentQuery) One() (*Department, error) {
	o := &Department{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for departments")
	}

	return o, nil
}

// AllP returns all Department records from the query, and panics on error.
func (q departmentQuery) AllP() DepartmentSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all Department records from the query.
func (q departmentQuery) All() (DepartmentSlice, error) {
	var o DepartmentSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to Department slice")
	}

	return o, nil
}

// CountP returns the count of all Department records in the query, and panics on error.
func (q departmentQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all Department records in the query.
func (q departmentQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count departments rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q departmentQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q departmentQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if departments exists")
	}

	return count > 0, nil
}

// UsersG retrieves all the user's users.
func (o *Department) UsersG(mods ...qm.QueryMod) userQuery {
	return o.Users(boil.GetDB(), mods...)
}

// Users retrieves all the user's users with an executor.
func (o *Department) Users(exec boil.Executor, mods ...qm.QueryMod) userQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"department_id\"=?", o.ID),
	)

	query := Users(exec, queryMods...)
	queries.SetFrom(query.Query, "\"users\" as \"a\"")
	return query
}

// LoadUsers allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (departmentL) LoadUsers(e boil.Executor, singular bool, maybeDepartment interface{}) error {
	var slice []*Department
	var object *Department

	count := 1
	if singular {
		object = maybeDepartment.(*Department)
	} else {
		slice = *maybeDepartment.(*DepartmentSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &departmentR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &departmentR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"users\" where \"department_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load users")
	}
	defer results.Close()

	var resultSlice []*User
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice users")
	}

	if singular {
		object.R.Users = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.DepartmentID.Int {
				local.R.Users = append(local.R.Users, foreign)
				break
			}
		}
	}

	return nil
}

// AddUsers adds the given related objects to the existing relationships
// of the department, optionally inserting them as new records.
// Appends related to o.R.Users.
// Sets related.R.Department appropriately.
func (o *Department) AddUsers(exec boil.Executor, insert bool, related ...*User) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.DepartmentID.Int = o.ID
			rel.DepartmentID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"users\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"department_id"}),
				strmangle.WhereClause("\"", "\"", 2, userPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.DepartmentID.Int = o.ID
			rel.DepartmentID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &departmentR{
			Users: related,
		}
	} else {
		o.R.Users = append(o.R.Users, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &userR{
				Department: o,
			}
		} else {
			rel.R.Department = o
		}
	}
	return nil
}

// SetUsers removes all previously related items of the
// department replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Department's Users accordingly.
// Replaces o.R.Users with related.
// Sets related.R.Department's Users accordingly.
func (o *Department) SetUsers(exec boil.Executor, insert bool, related ...*User) error {
	query := "update \"users\" set \"department_id\" = null where \"department_id\" = $1"
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
		for _, rel := range o.R.Users {
			rel.DepartmentID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Department = nil
		}

		o.R.Users = nil
	}
	return o.AddUsers(exec, insert, related...)
}

// RemoveUsers relationships from objects passed in.
// Removes related items from R.Users (uses pointer comparison, removal does not keep order)
// Sets related.R.Department.
func (o *Department) RemoveUsers(exec boil.Executor, related ...*User) error {
	var err error
	for _, rel := range related {
		rel.DepartmentID.Valid = false
		if rel.R != nil {
			rel.R.Department = nil
		}
		if err = rel.Update(exec, "department_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.Users {
			if rel != ri {
				continue
			}

			ln := len(o.R.Users)
			if ln > 1 && i < ln-1 {
				o.R.Users[i] = o.R.Users[ln-1]
			}
			o.R.Users = o.R.Users[:ln-1]
			break
		}
	}

	return nil
}

// DepartmentsG retrieves all records.
func DepartmentsG(mods ...qm.QueryMod) departmentQuery {
	return Departments(boil.GetDB(), mods...)
}

// Departments retrieves all the records using an executor.
func Departments(exec boil.Executor, mods ...qm.QueryMod) departmentQuery {
	mods = append(mods, qm.From("\"departments\""))
	return departmentQuery{NewQuery(exec, mods...)}
}

// FindDepartmentG retrieves a single record by ID.
func FindDepartmentG(id int, selectCols ...string) (*Department, error) {
	return FindDepartment(boil.GetDB(), id, selectCols...)
}

// FindDepartmentGP retrieves a single record by ID, and panics on error.
func FindDepartmentGP(id int, selectCols ...string) *Department {
	retobj, err := FindDepartment(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindDepartment retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindDepartment(exec boil.Executor, id int, selectCols ...string) (*Department, error) {
	departmentObj := &Department{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"departments\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(departmentObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from departments")
	}

	return departmentObj, nil
}

// FindDepartmentP retrieves a single record by ID with an executor, and panics on error.
func FindDepartmentP(exec boil.Executor, id int, selectCols ...string) *Department {
	retobj, err := FindDepartment(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *Department) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *Department) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *Department) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *Department) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no departments provided for insertion")
	}

	var err error
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = currTime
	}

	nzDefaults := queries.NonZeroDefaultSet(departmentColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	departmentInsertCacheMut.RLock()
	cache, cached := departmentInsertCache[key]
	departmentInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			departmentColumns,
			departmentColumnsWithDefault,
			departmentColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(departmentType, departmentMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(departmentType, departmentMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"departments\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into departments")
	}

	if !cached {
		departmentInsertCacheMut.Lock()
		departmentInsertCache[key] = cache
		departmentInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single Department record. See Update for
// whitelist behavior description.
func (o *Department) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single Department record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *Department) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the Department, and panics on error.
// See Update for whitelist behavior description.
func (o *Department) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the Department.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *Department) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt = currTime

	var err error
	key := makeCacheKey(whitelist, nil)
	departmentUpdateCacheMut.RLock()
	cache, cached := departmentUpdateCache[key]
	departmentUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(departmentColumns, departmentPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update departments, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"departments\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, departmentPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(departmentType, departmentMapping, append(wl, departmentPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update departments row")
	}

	if !cached {
		departmentUpdateCacheMut.Lock()
		departmentUpdateCache[key] = cache
		departmentUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q departmentQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q departmentQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for departments")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o DepartmentSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o DepartmentSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o DepartmentSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o DepartmentSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), departmentPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"departments\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(departmentPrimaryKeyColumns), len(colNames)+1, len(departmentPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in department slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *Department) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *Department) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *Department) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *Department) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no departments provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	o.UpdatedAt = currTime

	nzDefaults := queries.NonZeroDefaultSet(departmentColumnsWithDefault, o)

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

	departmentUpsertCacheMut.RLock()
	cache, cached := departmentUpsertCache[key]
	departmentUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			departmentColumns,
			departmentColumnsWithDefault,
			departmentColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			departmentColumns,
			departmentPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert departments, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(departmentPrimaryKeyColumns))
			copy(conflict, departmentPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"departments\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(departmentType, departmentMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(departmentType, departmentMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert departments")
	}

	if !cached {
		departmentUpsertCacheMut.Lock()
		departmentUpsertCache[key] = cache
		departmentUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single Department record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Department) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single Department record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *Department) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no Department provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single Department record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Department) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single Department record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Department) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Department provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), departmentPrimaryKeyMapping)
	sql := "DELETE FROM \"departments\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from departments")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q departmentQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q departmentQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no departmentQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from departments")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o DepartmentSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o DepartmentSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no Department slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o DepartmentSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o DepartmentSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Department slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), departmentPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"departments\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, departmentPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(departmentPrimaryKeyColumns), 1, len(departmentPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from department slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *Department) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *Department) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *Department) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no Department provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Department) Reload(exec boil.Executor) error {
	ret, err := FindDepartment(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *DepartmentSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *DepartmentSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *DepartmentSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty DepartmentSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *DepartmentSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	departments := DepartmentSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), departmentPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"departments\".* FROM \"departments\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, departmentPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(departmentPrimaryKeyColumns), 1, len(departmentPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&departments)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in DepartmentSlice")
	}

	*o = departments

	return nil
}

// DepartmentExists checks if the Department row exists.
func DepartmentExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"departments\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if departments exists")
	}

	return exists, nil
}

// DepartmentExistsG checks if the Department row exists.
func DepartmentExistsG(id int) (bool, error) {
	return DepartmentExists(boil.GetDB(), id)
}

// DepartmentExistsGP checks if the Department row exists. Panics on error.
func DepartmentExistsGP(id int) bool {
	e, err := DepartmentExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// DepartmentExistsP checks if the Department row exists. Panics on error.
func DepartmentExistsP(exec boil.Executor, id int) bool {
	e, err := DepartmentExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

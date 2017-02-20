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
)

// RolesUser is an object representing the database table.
type RolesUser struct {
	RoleID int `boil:"role_id" json:"role_id" toml:"role_id" yaml:"role_id"`
	UserID int `boil:"user_id" json:"user_id" toml:"user_id" yaml:"user_id"`

	R *rolesUserR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L rolesUserL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// rolesUserR is where relationships are stored.
type rolesUserR struct {
	User *User
}

// rolesUserL is where Load methods for each relationship are stored.
type rolesUserL struct{}

var (
	rolesUserColumns               = []string{"role_id", "user_id"}
	rolesUserColumnsWithoutDefault = []string{"role_id", "user_id"}
	rolesUserColumnsWithDefault    = []string{}
	rolesUserPrimaryKeyColumns     = []string{"role_id", "user_id"}
)

type (
	// RolesUserSlice is an alias for a slice of pointers to RolesUser.
	// This should generally be used opposed to []RolesUser.
	RolesUserSlice []*RolesUser

	rolesUserQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	rolesUserType                 = reflect.TypeOf(&RolesUser{})
	rolesUserMapping              = queries.MakeStructMapping(rolesUserType)
	rolesUserPrimaryKeyMapping, _ = queries.BindMapping(rolesUserType, rolesUserMapping, rolesUserPrimaryKeyColumns)
	rolesUserInsertCacheMut       sync.RWMutex
	rolesUserInsertCache          = make(map[string]insertCache)
	rolesUserUpdateCacheMut       sync.RWMutex
	rolesUserUpdateCache          = make(map[string]updateCache)
	rolesUserUpsertCacheMut       sync.RWMutex
	rolesUserUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single rolesUser record from the query, and panics on error.
func (q rolesUserQuery) OneP() *RolesUser {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single rolesUser record from the query.
func (q rolesUserQuery) One() (*RolesUser, error) {
	o := &RolesUser{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for roles_users")
	}

	return o, nil
}

// AllP returns all RolesUser records from the query, and panics on error.
func (q rolesUserQuery) AllP() RolesUserSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all RolesUser records from the query.
func (q rolesUserQuery) All() (RolesUserSlice, error) {
	var o RolesUserSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to RolesUser slice")
	}

	return o, nil
}

// CountP returns the count of all RolesUser records in the query, and panics on error.
func (q rolesUserQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all RolesUser records in the query.
func (q rolesUserQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count roles_users rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q rolesUserQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q rolesUserQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if roles_users exists")
	}

	return count > 0, nil
}

// UserG pointed to by the foreign key.
func (o *RolesUser) UserG(mods ...qm.QueryMod) userQuery {
	return o.User(boil.GetDB(), mods...)
}

// User pointed to by the foreign key.
func (o *RolesUser) User(exec boil.Executor, mods ...qm.QueryMod) userQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.UserID),
	}

	queryMods = append(queryMods, mods...)

	query := Users(exec, queryMods...)
	queries.SetFrom(query.Query, "\"users\"")

	return query
}

// LoadUser allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (rolesUserL) LoadUser(e boil.Executor, singular bool, maybeRolesUser interface{}) error {
	var slice []*RolesUser
	var object *RolesUser

	count := 1
	if singular {
		object = maybeRolesUser.(*RolesUser)
	} else {
		slice = *maybeRolesUser.(*RolesUserSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &rolesUserR{}
		}
		args[0] = object.UserID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &rolesUserR{}
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
			if local.UserID == foreign.ID {
				local.R.User = foreign
				break
			}
		}
	}

	return nil
}

// SetUser of the roles_user to the related item.
// Sets o.R.User to related.
// Adds o to related.R.RolesUsers.
func (o *RolesUser) SetUser(exec boil.Executor, insert bool, related *User) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"roles_users\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
		strmangle.WhereClause("\"", "\"", 2, rolesUserPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.RoleID, o.UserID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.UserID = related.ID

	if o.R == nil {
		o.R = &rolesUserR{
			User: related,
		}
	} else {
		o.R.User = related
	}

	if related.R == nil {
		related.R = &userR{
			RolesUsers: RolesUserSlice{o},
		}
	} else {
		related.R.RolesUsers = append(related.R.RolesUsers, o)
	}

	return nil
}

// RolesUsersG retrieves all records.
func RolesUsersG(mods ...qm.QueryMod) rolesUserQuery {
	return RolesUsers(boil.GetDB(), mods...)
}

// RolesUsers retrieves all the records using an executor.
func RolesUsers(exec boil.Executor, mods ...qm.QueryMod) rolesUserQuery {
	mods = append(mods, qm.From("\"roles_users\""))
	return rolesUserQuery{NewQuery(exec, mods...)}
}

// FindRolesUserG retrieves a single record by ID.
func FindRolesUserG(roleID int, userID int, selectCols ...string) (*RolesUser, error) {
	return FindRolesUser(boil.GetDB(), roleID, userID, selectCols...)
}

// FindRolesUserGP retrieves a single record by ID, and panics on error.
func FindRolesUserGP(roleID int, userID int, selectCols ...string) *RolesUser {
	retobj, err := FindRolesUser(boil.GetDB(), roleID, userID, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindRolesUser retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindRolesUser(exec boil.Executor, roleID int, userID int, selectCols ...string) (*RolesUser, error) {
	rolesUserObj := &RolesUser{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"roles_users\" where \"role_id\"=$1 AND \"user_id\"=$2", sel,
	)

	q := queries.Raw(exec, query, roleID, userID)

	err := q.Bind(rolesUserObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from roles_users")
	}

	return rolesUserObj, nil
}

// FindRolesUserP retrieves a single record by ID with an executor, and panics on error.
func FindRolesUserP(exec boil.Executor, roleID int, userID int, selectCols ...string) *RolesUser {
	retobj, err := FindRolesUser(exec, roleID, userID, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *RolesUser) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *RolesUser) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *RolesUser) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *RolesUser) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no roles_users provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(rolesUserColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	rolesUserInsertCacheMut.RLock()
	cache, cached := rolesUserInsertCache[key]
	rolesUserInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			rolesUserColumns,
			rolesUserColumnsWithDefault,
			rolesUserColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(rolesUserType, rolesUserMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(rolesUserType, rolesUserMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"roles_users\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into roles_users")
	}

	if !cached {
		rolesUserInsertCacheMut.Lock()
		rolesUserInsertCache[key] = cache
		rolesUserInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single RolesUser record. See Update for
// whitelist behavior description.
func (o *RolesUser) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single RolesUser record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *RolesUser) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the RolesUser, and panics on error.
// See Update for whitelist behavior description.
func (o *RolesUser) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the RolesUser.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *RolesUser) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	rolesUserUpdateCacheMut.RLock()
	cache, cached := rolesUserUpdateCache[key]
	rolesUserUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(rolesUserColumns, rolesUserPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update roles_users, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"roles_users\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, rolesUserPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(rolesUserType, rolesUserMapping, append(wl, rolesUserPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update roles_users row")
	}

	if !cached {
		rolesUserUpdateCacheMut.Lock()
		rolesUserUpdateCache[key] = cache
		rolesUserUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q rolesUserQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q rolesUserQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for roles_users")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o RolesUserSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o RolesUserSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o RolesUserSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o RolesUserSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), rolesUserPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"roles_users\" SET %s WHERE (\"role_id\",\"user_id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(rolesUserPrimaryKeyColumns), len(colNames)+1, len(rolesUserPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in rolesUser slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *RolesUser) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *RolesUser) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *RolesUser) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *RolesUser) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no roles_users provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(rolesUserColumnsWithDefault, o)

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

	rolesUserUpsertCacheMut.RLock()
	cache, cached := rolesUserUpsertCache[key]
	rolesUserUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			rolesUserColumns,
			rolesUserColumnsWithDefault,
			rolesUserColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			rolesUserColumns,
			rolesUserPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert roles_users, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(rolesUserPrimaryKeyColumns))
			copy(conflict, rolesUserPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"roles_users\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(rolesUserType, rolesUserMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(rolesUserType, rolesUserMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert roles_users")
	}

	if !cached {
		rolesUserUpsertCacheMut.Lock()
		rolesUserUpsertCache[key] = cache
		rolesUserUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single RolesUser record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *RolesUser) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single RolesUser record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *RolesUser) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no RolesUser provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single RolesUser record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *RolesUser) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single RolesUser record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *RolesUser) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no RolesUser provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), rolesUserPrimaryKeyMapping)
	sql := "DELETE FROM \"roles_users\" WHERE \"role_id\"=$1 AND \"user_id\"=$2"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from roles_users")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q rolesUserQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q rolesUserQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no rolesUserQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from roles_users")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o RolesUserSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o RolesUserSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no RolesUser slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o RolesUserSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o RolesUserSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no RolesUser slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), rolesUserPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"roles_users\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, rolesUserPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(rolesUserPrimaryKeyColumns), 1, len(rolesUserPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from rolesUser slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *RolesUser) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *RolesUser) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *RolesUser) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no RolesUser provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *RolesUser) Reload(exec boil.Executor) error {
	ret, err := FindRolesUser(exec, o.RoleID, o.UserID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *RolesUserSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *RolesUserSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *RolesUserSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty RolesUserSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *RolesUserSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	rolesUsers := RolesUserSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), rolesUserPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"roles_users\".* FROM \"roles_users\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, rolesUserPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(rolesUserPrimaryKeyColumns), 1, len(rolesUserPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&rolesUsers)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in RolesUserSlice")
	}

	*o = rolesUsers

	return nil
}

// RolesUserExists checks if the RolesUser row exists.
func RolesUserExists(exec boil.Executor, roleID int, userID int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"roles_users\" where \"role_id\"=$1 AND \"user_id\"=$2 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, roleID, userID)
	}

	row := exec.QueryRow(sql, roleID, userID)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if roles_users exists")
	}

	return exists, nil
}

// RolesUserExistsG checks if the RolesUser row exists.
func RolesUserExistsG(roleID int, userID int) (bool, error) {
	return RolesUserExists(boil.GetDB(), roleID, userID)
}

// RolesUserExistsGP checks if the RolesUser row exists. Panics on error.
func RolesUserExistsGP(roleID int, userID int) bool {
	e, err := RolesUserExists(boil.GetDB(), roleID, userID)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// RolesUserExistsP checks if the RolesUser row exists. Panics on error.
func RolesUserExistsP(exec boil.Executor, roleID int, userID int) bool {
	e, err := RolesUserExists(exec, roleID, userID)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

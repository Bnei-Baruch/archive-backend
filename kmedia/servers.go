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

// Server is an object representing the database table.
type Server struct {
	Servername string      `boil:"servername" json:"servername" toml:"servername" yaml:"servername"`
	Httpurl    null.String `boil:"httpurl" json:"httpurl,omitempty" toml:"httpurl" yaml:"httpurl,omitempty"`
	Created    null.Time   `boil:"created" json:"created,omitempty" toml:"created" yaml:"created,omitempty"`
	Updated    null.Time   `boil:"updated" json:"updated,omitempty" toml:"updated" yaml:"updated,omitempty"`
	Lastuser   null.String `boil:"lastuser" json:"lastuser,omitempty" toml:"lastuser" yaml:"lastuser,omitempty"`
	Path       null.String `boil:"path" json:"path,omitempty" toml:"path" yaml:"path,omitempty"`
	ID         int         `boil:"id" json:"id" toml:"id" yaml:"id"`

	R *serverR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L serverL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// serverR is where relationships are stored.
type serverR struct {
	ServerNameFileAssets FileAssetSlice
}

// serverL is where Load methods for each relationship are stored.
type serverL struct{}

var (
	serverColumns               = []string{"servername", "httpurl", "created", "updated", "lastuser", "path", "id"}
	serverColumnsWithoutDefault = []string{"httpurl", "created", "updated", "lastuser", "path"}
	serverColumnsWithDefault    = []string{"servername", "id"}
	serverPrimaryKeyColumns     = []string{"id"}
)

type (
	// ServerSlice is an alias for a slice of pointers to Server.
	// This should generally be used opposed to []Server.
	ServerSlice []*Server

	serverQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	serverType                 = reflect.TypeOf(&Server{})
	serverMapping              = queries.MakeStructMapping(serverType)
	serverPrimaryKeyMapping, _ = queries.BindMapping(serverType, serverMapping, serverPrimaryKeyColumns)
	serverInsertCacheMut       sync.RWMutex
	serverInsertCache          = make(map[string]insertCache)
	serverUpdateCacheMut       sync.RWMutex
	serverUpdateCache          = make(map[string]updateCache)
	serverUpsertCacheMut       sync.RWMutex
	serverUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single server record from the query, and panics on error.
func (q serverQuery) OneP() *Server {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single server record from the query.
func (q serverQuery) One() (*Server, error) {
	o := &Server{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for servers")
	}

	return o, nil
}

// AllP returns all Server records from the query, and panics on error.
func (q serverQuery) AllP() ServerSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all Server records from the query.
func (q serverQuery) All() (ServerSlice, error) {
	var o ServerSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to Server slice")
	}

	return o, nil
}

// CountP returns the count of all Server records in the query, and panics on error.
func (q serverQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all Server records in the query.
func (q serverQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count servers rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q serverQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q serverQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if servers exists")
	}

	return count > 0, nil
}

// ServerNameFileAssetsG retrieves all the file_asset's file assets via server_name_id column.
func (o *Server) ServerNameFileAssetsG(mods ...qm.QueryMod) fileAssetQuery {
	return o.ServerNameFileAssets(boil.GetDB(), mods...)
}

// ServerNameFileAssets retrieves all the file_asset's file assets with an executor via server_name_id column.
func (o *Server) ServerNameFileAssets(exec boil.Executor, mods ...qm.QueryMod) fileAssetQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"server_name_id\"=?", o.Servername),
	)

	query := FileAssets(exec, queryMods...)
	queries.SetFrom(query.Query, "\"file_assets\" as \"a\"")
	return query
}

// LoadServerNameFileAssets allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (serverL) LoadServerNameFileAssets(e boil.Executor, singular bool, maybeServer interface{}) error {
	var slice []*Server
	var object *Server

	count := 1
	if singular {
		object = maybeServer.(*Server)
	} else {
		slice = *maybeServer.(*ServerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &serverR{}
		}
		args[0] = object.Servername
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &serverR{}
			}
			args[i] = obj.Servername
		}
	}

	query := fmt.Sprintf(
		"select * from \"file_assets\" where \"server_name_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load file_assets")
	}
	defer results.Close()

	var resultSlice []*FileAsset
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice file_assets")
	}

	if singular {
		object.R.ServerNameFileAssets = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Servername == foreign.ServerNameID.String {
				local.R.ServerNameFileAssets = append(local.R.ServerNameFileAssets, foreign)
				break
			}
		}
	}

	return nil
}

// AddServerNameFileAssets adds the given related objects to the existing relationships
// of the server, optionally inserting them as new records.
// Appends related to o.R.ServerNameFileAssets.
// Sets related.R.ServerName appropriately.
func (o *Server) AddServerNameFileAssets(exec boil.Executor, insert bool, related ...*FileAsset) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.ServerNameID.String = o.Servername
			rel.ServerNameID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"file_assets\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"server_name_id"}),
				strmangle.WhereClause("\"", "\"", 2, fileAssetPrimaryKeyColumns),
			)
			values := []interface{}{o.Servername, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.ServerNameID.String = o.Servername
			rel.ServerNameID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &serverR{
			ServerNameFileAssets: related,
		}
	} else {
		o.R.ServerNameFileAssets = append(o.R.ServerNameFileAssets, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &fileAssetR{
				ServerName: o,
			}
		} else {
			rel.R.ServerName = o
		}
	}
	return nil
}

// SetServerNameFileAssets removes all previously related items of the
// server replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.ServerName's ServerNameFileAssets accordingly.
// Replaces o.R.ServerNameFileAssets with related.
// Sets related.R.ServerName's ServerNameFileAssets accordingly.
func (o *Server) SetServerNameFileAssets(exec boil.Executor, insert bool, related ...*FileAsset) error {
	query := "update \"file_assets\" set \"server_name_id\" = null where \"server_name_id\" = $1"
	values := []interface{}{o.Servername}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.ServerNameFileAssets {
			rel.ServerNameID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.ServerName = nil
		}

		o.R.ServerNameFileAssets = nil
	}
	return o.AddServerNameFileAssets(exec, insert, related...)
}

// RemoveServerNameFileAssets relationships from objects passed in.
// Removes related items from R.ServerNameFileAssets (uses pointer comparison, removal does not keep order)
// Sets related.R.ServerName.
func (o *Server) RemoveServerNameFileAssets(exec boil.Executor, related ...*FileAsset) error {
	var err error
	for _, rel := range related {
		rel.ServerNameID.Valid = false
		if rel.R != nil {
			rel.R.ServerName = nil
		}
		if err = rel.Update(exec, "server_name_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.ServerNameFileAssets {
			if rel != ri {
				continue
			}

			ln := len(o.R.ServerNameFileAssets)
			if ln > 1 && i < ln-1 {
				o.R.ServerNameFileAssets[i] = o.R.ServerNameFileAssets[ln-1]
			}
			o.R.ServerNameFileAssets = o.R.ServerNameFileAssets[:ln-1]
			break
		}
	}

	return nil
}

// ServersG retrieves all records.
func ServersG(mods ...qm.QueryMod) serverQuery {
	return Servers(boil.GetDB(), mods...)
}

// Servers retrieves all the records using an executor.
func Servers(exec boil.Executor, mods ...qm.QueryMod) serverQuery {
	mods = append(mods, qm.From("\"servers\""))
	return serverQuery{NewQuery(exec, mods...)}
}

// FindServerG retrieves a single record by ID.
func FindServerG(id int, selectCols ...string) (*Server, error) {
	return FindServer(boil.GetDB(), id, selectCols...)
}

// FindServerGP retrieves a single record by ID, and panics on error.
func FindServerGP(id int, selectCols ...string) *Server {
	retobj, err := FindServer(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindServer retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindServer(exec boil.Executor, id int, selectCols ...string) (*Server, error) {
	serverObj := &Server{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"servers\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(serverObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from servers")
	}

	return serverObj, nil
}

// FindServerP retrieves a single record by ID with an executor, and panics on error.
func FindServerP(exec boil.Executor, id int, selectCols ...string) *Server {
	retobj, err := FindServer(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *Server) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *Server) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *Server) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *Server) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no servers provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(serverColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	serverInsertCacheMut.RLock()
	cache, cached := serverInsertCache[key]
	serverInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			serverColumns,
			serverColumnsWithDefault,
			serverColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(serverType, serverMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(serverType, serverMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"servers\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into servers")
	}

	if !cached {
		serverInsertCacheMut.Lock()
		serverInsertCache[key] = cache
		serverInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single Server record. See Update for
// whitelist behavior description.
func (o *Server) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single Server record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *Server) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the Server, and panics on error.
// See Update for whitelist behavior description.
func (o *Server) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the Server.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *Server) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	serverUpdateCacheMut.RLock()
	cache, cached := serverUpdateCache[key]
	serverUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(serverColumns, serverPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update servers, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"servers\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, serverPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(serverType, serverMapping, append(wl, serverPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update servers row")
	}

	if !cached {
		serverUpdateCacheMut.Lock()
		serverUpdateCache[key] = cache
		serverUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q serverQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q serverQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for servers")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ServerSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o ServerSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o ServerSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ServerSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), serverPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"servers\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(serverPrimaryKeyColumns), len(colNames)+1, len(serverPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in server slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *Server) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *Server) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *Server) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *Server) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no servers provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(serverColumnsWithDefault, o)

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

	serverUpsertCacheMut.RLock()
	cache, cached := serverUpsertCache[key]
	serverUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			serverColumns,
			serverColumnsWithDefault,
			serverColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			serverColumns,
			serverPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert servers, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(serverPrimaryKeyColumns))
			copy(conflict, serverPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"servers\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(serverType, serverMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(serverType, serverMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert servers")
	}

	if !cached {
		serverUpsertCacheMut.Lock()
		serverUpsertCache[key] = cache
		serverUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single Server record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Server) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single Server record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *Server) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no Server provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single Server record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Server) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single Server record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Server) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Server provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), serverPrimaryKeyMapping)
	sql := "DELETE FROM \"servers\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from servers")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q serverQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q serverQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no serverQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from servers")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o ServerSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o ServerSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no Server slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o ServerSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ServerSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Server slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), serverPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"servers\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, serverPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(serverPrimaryKeyColumns), 1, len(serverPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from server slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *Server) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *Server) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *Server) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no Server provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Server) Reload(exec boil.Executor) error {
	ret, err := FindServer(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ServerSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ServerSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ServerSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty ServerSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ServerSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	servers := ServerSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), serverPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"servers\".* FROM \"servers\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, serverPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(serverPrimaryKeyColumns), 1, len(serverPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&servers)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in ServerSlice")
	}

	*o = servers

	return nil
}

// ServerExists checks if the Server row exists.
func ServerExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"servers\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if servers exists")
	}

	return exists, nil
}

// ServerExistsG checks if the Server row exists.
func ServerExistsG(id int) (bool, error) {
	return ServerExists(boil.GetDB(), id)
}

// ServerExistsGP checks if the Server row exists. Panics on error.
func ServerExistsGP(id int) bool {
	e, err := ServerExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// ServerExistsP checks if the Server row exists. Panics on error.
func ServerExistsP(exec boil.Executor, id int) bool {
	e, err := ServerExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

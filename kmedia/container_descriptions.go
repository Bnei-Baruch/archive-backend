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

// ContainerDescription is an object representing the database table.
type ContainerDescription struct {
	ID            int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	ContainerID   int         `boil:"container_id" json:"container_id" toml:"container_id" yaml:"container_id"`
	ContainerDesc null.String `boil:"container_desc" json:"container_desc,omitempty" toml:"container_desc" yaml:"container_desc,omitempty"`
	LangID        null.String `boil:"lang_id" json:"lang_id,omitempty" toml:"lang_id" yaml:"lang_id,omitempty"`
	CreatedAt     null.Time   `boil:"created_at" json:"created_at,omitempty" toml:"created_at" yaml:"created_at,omitempty"`
	UpdatedAt     null.Time   `boil:"updated_at" json:"updated_at,omitempty" toml:"updated_at" yaml:"updated_at,omitempty"`
	Descr         null.String `boil:"descr" json:"descr,omitempty" toml:"descr" yaml:"descr,omitempty"`

	R *containerDescriptionR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L containerDescriptionL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// containerDescriptionR is where relationships are stored.
type containerDescriptionR struct {
	Container *Container
	Lang      *Language
}

// containerDescriptionL is where Load methods for each relationship are stored.
type containerDescriptionL struct{}

var (
	containerDescriptionColumns               = []string{"id", "container_id", "container_desc", "lang_id", "created_at", "updated_at", "descr"}
	containerDescriptionColumnsWithoutDefault = []string{"container_desc", "lang_id", "created_at", "updated_at", "descr"}
	containerDescriptionColumnsWithDefault    = []string{"id", "container_id"}
	containerDescriptionPrimaryKeyColumns     = []string{"id"}
)

type (
	// ContainerDescriptionSlice is an alias for a slice of pointers to ContainerDescription.
	// This should generally be used opposed to []ContainerDescription.
	ContainerDescriptionSlice []*ContainerDescription

	containerDescriptionQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	containerDescriptionType                 = reflect.TypeOf(&ContainerDescription{})
	containerDescriptionMapping              = queries.MakeStructMapping(containerDescriptionType)
	containerDescriptionPrimaryKeyMapping, _ = queries.BindMapping(containerDescriptionType, containerDescriptionMapping, containerDescriptionPrimaryKeyColumns)
	containerDescriptionInsertCacheMut       sync.RWMutex
	containerDescriptionInsertCache          = make(map[string]insertCache)
	containerDescriptionUpdateCacheMut       sync.RWMutex
	containerDescriptionUpdateCache          = make(map[string]updateCache)
	containerDescriptionUpsertCacheMut       sync.RWMutex
	containerDescriptionUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single containerDescription record from the query, and panics on error.
func (q containerDescriptionQuery) OneP() *ContainerDescription {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single containerDescription record from the query.
func (q containerDescriptionQuery) One() (*ContainerDescription, error) {
	o := &ContainerDescription{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for container_descriptions")
	}

	return o, nil
}

// AllP returns all ContainerDescription records from the query, and panics on error.
func (q containerDescriptionQuery) AllP() ContainerDescriptionSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all ContainerDescription records from the query.
func (q containerDescriptionQuery) All() (ContainerDescriptionSlice, error) {
	var o ContainerDescriptionSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to ContainerDescription slice")
	}

	return o, nil
}

// CountP returns the count of all ContainerDescription records in the query, and panics on error.
func (q containerDescriptionQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all ContainerDescription records in the query.
func (q containerDescriptionQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count container_descriptions rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q containerDescriptionQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q containerDescriptionQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if container_descriptions exists")
	}

	return count > 0, nil
}

// ContainerG pointed to by the foreign key.
func (o *ContainerDescription) ContainerG(mods ...qm.QueryMod) containerQuery {
	return o.Container(boil.GetDB(), mods...)
}

// Container pointed to by the foreign key.
func (o *ContainerDescription) Container(exec boil.Executor, mods ...qm.QueryMod) containerQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.ContainerID),
	}

	queryMods = append(queryMods, mods...)

	query := Containers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"containers\"")

	return query
}

// LangG pointed to by the foreign key.
func (o *ContainerDescription) LangG(mods ...qm.QueryMod) languageQuery {
	return o.Lang(boil.GetDB(), mods...)
}

// Lang pointed to by the foreign key.
func (o *ContainerDescription) Lang(exec boil.Executor, mods ...qm.QueryMod) languageQuery {
	queryMods := []qm.QueryMod{
		qm.Where("code3=?", o.LangID),
	}

	queryMods = append(queryMods, mods...)

	query := Languages(exec, queryMods...)
	queries.SetFrom(query.Query, "\"languages\"")

	return query
}

// LoadContainer allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerDescriptionL) LoadContainer(e boil.Executor, singular bool, maybeContainerDescription interface{}) error {
	var slice []*ContainerDescription
	var object *ContainerDescription

	count := 1
	if singular {
		object = maybeContainerDescription.(*ContainerDescription)
	} else {
		slice = *maybeContainerDescription.(*ContainerDescriptionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerDescriptionR{}
		}
		args[0] = object.ContainerID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerDescriptionR{}
			}
			args[i] = obj.ContainerID
		}
	}

	query := fmt.Sprintf(
		"select * from \"containers\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Container")
	}
	defer results.Close()

	var resultSlice []*Container
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Container")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Container = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ContainerID == foreign.ID {
				local.R.Container = foreign
				break
			}
		}
	}

	return nil
}

// LoadLang allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerDescriptionL) LoadLang(e boil.Executor, singular bool, maybeContainerDescription interface{}) error {
	var slice []*ContainerDescription
	var object *ContainerDescription

	count := 1
	if singular {
		object = maybeContainerDescription.(*ContainerDescription)
	} else {
		slice = *maybeContainerDescription.(*ContainerDescriptionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerDescriptionR{}
		}
		args[0] = object.LangID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerDescriptionR{}
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

// SetContainer of the container_description to the related item.
// Sets o.R.Container to related.
// Adds o to related.R.ContainerDescriptions.
func (o *ContainerDescription) SetContainer(exec boil.Executor, insert bool, related *Container) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"container_descriptions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"container_id"}),
		strmangle.WhereClause("\"", "\"", 2, containerDescriptionPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.ContainerID = related.ID

	if o.R == nil {
		o.R = &containerDescriptionR{
			Container: related,
		}
	} else {
		o.R.Container = related
	}

	if related.R == nil {
		related.R = &containerR{
			ContainerDescriptions: ContainerDescriptionSlice{o},
		}
	} else {
		related.R.ContainerDescriptions = append(related.R.ContainerDescriptions, o)
	}

	return nil
}

// SetLang of the container_description to the related item.
// Sets o.R.Lang to related.
// Adds o to related.R.LangContainerDescriptions.
func (o *ContainerDescription) SetLang(exec boil.Executor, insert bool, related *Language) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"container_descriptions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
		strmangle.WhereClause("\"", "\"", 2, containerDescriptionPrimaryKeyColumns),
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
		o.R = &containerDescriptionR{
			Lang: related,
		}
	} else {
		o.R.Lang = related
	}

	if related.R == nil {
		related.R = &languageR{
			LangContainerDescriptions: ContainerDescriptionSlice{o},
		}
	} else {
		related.R.LangContainerDescriptions = append(related.R.LangContainerDescriptions, o)
	}

	return nil
}

// RemoveLang relationship.
// Sets o.R.Lang to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *ContainerDescription) RemoveLang(exec boil.Executor, related *Language) error {
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

	for i, ri := range related.R.LangContainerDescriptions {
		if o.LangID.String != ri.LangID.String {
			continue
		}

		ln := len(related.R.LangContainerDescriptions)
		if ln > 1 && i < ln-1 {
			related.R.LangContainerDescriptions[i] = related.R.LangContainerDescriptions[ln-1]
		}
		related.R.LangContainerDescriptions = related.R.LangContainerDescriptions[:ln-1]
		break
	}
	return nil
}

// ContainerDescriptionsG retrieves all records.
func ContainerDescriptionsG(mods ...qm.QueryMod) containerDescriptionQuery {
	return ContainerDescriptions(boil.GetDB(), mods...)
}

// ContainerDescriptions retrieves all the records using an executor.
func ContainerDescriptions(exec boil.Executor, mods ...qm.QueryMod) containerDescriptionQuery {
	mods = append(mods, qm.From("\"container_descriptions\""))
	return containerDescriptionQuery{NewQuery(exec, mods...)}
}

// FindContainerDescriptionG retrieves a single record by ID.
func FindContainerDescriptionG(id int, selectCols ...string) (*ContainerDescription, error) {
	return FindContainerDescription(boil.GetDB(), id, selectCols...)
}

// FindContainerDescriptionGP retrieves a single record by ID, and panics on error.
func FindContainerDescriptionGP(id int, selectCols ...string) *ContainerDescription {
	retobj, err := FindContainerDescription(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindContainerDescription retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindContainerDescription(exec boil.Executor, id int, selectCols ...string) (*ContainerDescription, error) {
	containerDescriptionObj := &ContainerDescription{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"container_descriptions\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(containerDescriptionObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from container_descriptions")
	}

	return containerDescriptionObj, nil
}

// FindContainerDescriptionP retrieves a single record by ID with an executor, and panics on error.
func FindContainerDescriptionP(exec boil.Executor, id int, selectCols ...string) *ContainerDescription {
	retobj, err := FindContainerDescription(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *ContainerDescription) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *ContainerDescription) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *ContainerDescription) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *ContainerDescription) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no container_descriptions provided for insertion")
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

	nzDefaults := queries.NonZeroDefaultSet(containerDescriptionColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	containerDescriptionInsertCacheMut.RLock()
	cache, cached := containerDescriptionInsertCache[key]
	containerDescriptionInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			containerDescriptionColumns,
			containerDescriptionColumnsWithDefault,
			containerDescriptionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(containerDescriptionType, containerDescriptionMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(containerDescriptionType, containerDescriptionMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"container_descriptions\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into container_descriptions")
	}

	if !cached {
		containerDescriptionInsertCacheMut.Lock()
		containerDescriptionInsertCache[key] = cache
		containerDescriptionInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single ContainerDescription record. See Update for
// whitelist behavior description.
func (o *ContainerDescription) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single ContainerDescription record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *ContainerDescription) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the ContainerDescription, and panics on error.
// See Update for whitelist behavior description.
func (o *ContainerDescription) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the ContainerDescription.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *ContainerDescription) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	var err error
	key := makeCacheKey(whitelist, nil)
	containerDescriptionUpdateCacheMut.RLock()
	cache, cached := containerDescriptionUpdateCache[key]
	containerDescriptionUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(containerDescriptionColumns, containerDescriptionPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update container_descriptions, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"container_descriptions\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, containerDescriptionPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(containerDescriptionType, containerDescriptionMapping, append(wl, containerDescriptionPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update container_descriptions row")
	}

	if !cached {
		containerDescriptionUpdateCacheMut.Lock()
		containerDescriptionUpdateCache[key] = cache
		containerDescriptionUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q containerDescriptionQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q containerDescriptionQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for container_descriptions")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ContainerDescriptionSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o ContainerDescriptionSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o ContainerDescriptionSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ContainerDescriptionSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), containerDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"container_descriptions\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(containerDescriptionPrimaryKeyColumns), len(colNames)+1, len(containerDescriptionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in containerDescription slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *ContainerDescription) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *ContainerDescription) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *ContainerDescription) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *ContainerDescription) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no container_descriptions provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.Time.IsZero() {
		o.CreatedAt.Time = currTime
		o.CreatedAt.Valid = true
	}
	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	nzDefaults := queries.NonZeroDefaultSet(containerDescriptionColumnsWithDefault, o)

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

	containerDescriptionUpsertCacheMut.RLock()
	cache, cached := containerDescriptionUpsertCache[key]
	containerDescriptionUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			containerDescriptionColumns,
			containerDescriptionColumnsWithDefault,
			containerDescriptionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			containerDescriptionColumns,
			containerDescriptionPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert container_descriptions, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(containerDescriptionPrimaryKeyColumns))
			copy(conflict, containerDescriptionPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"container_descriptions\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(containerDescriptionType, containerDescriptionMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(containerDescriptionType, containerDescriptionMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert container_descriptions")
	}

	if !cached {
		containerDescriptionUpsertCacheMut.Lock()
		containerDescriptionUpsertCache[key] = cache
		containerDescriptionUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single ContainerDescription record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContainerDescription) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single ContainerDescription record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *ContainerDescription) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no ContainerDescription provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single ContainerDescription record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContainerDescription) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single ContainerDescription record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *ContainerDescription) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no ContainerDescription provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), containerDescriptionPrimaryKeyMapping)
	sql := "DELETE FROM \"container_descriptions\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from container_descriptions")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q containerDescriptionQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q containerDescriptionQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no containerDescriptionQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from container_descriptions")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o ContainerDescriptionSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o ContainerDescriptionSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no ContainerDescription slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o ContainerDescriptionSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ContainerDescriptionSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no ContainerDescription slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), containerDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"container_descriptions\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, containerDescriptionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(containerDescriptionPrimaryKeyColumns), 1, len(containerDescriptionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from containerDescription slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *ContainerDescription) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *ContainerDescription) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *ContainerDescription) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no ContainerDescription provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *ContainerDescription) Reload(exec boil.Executor) error {
	ret, err := FindContainerDescription(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContainerDescriptionSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContainerDescriptionSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContainerDescriptionSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty ContainerDescriptionSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContainerDescriptionSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	containerDescriptions := ContainerDescriptionSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), containerDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"container_descriptions\".* FROM \"container_descriptions\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, containerDescriptionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(containerDescriptionPrimaryKeyColumns), 1, len(containerDescriptionPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&containerDescriptions)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in ContainerDescriptionSlice")
	}

	*o = containerDescriptions

	return nil
}

// ContainerDescriptionExists checks if the ContainerDescription row exists.
func ContainerDescriptionExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"container_descriptions\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if container_descriptions exists")
	}

	return exists, nil
}

// ContainerDescriptionExistsG checks if the ContainerDescription row exists.
func ContainerDescriptionExistsG(id int) (bool, error) {
	return ContainerDescriptionExists(boil.GetDB(), id)
}

// ContainerDescriptionExistsGP checks if the ContainerDescription row exists. Panics on error.
func ContainerDescriptionExistsGP(id int) bool {
	e, err := ContainerDescriptionExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// ContainerDescriptionExistsP checks if the ContainerDescription row exists. Panics on error.
func ContainerDescriptionExistsP(exec boil.Executor, id int) bool {
	e, err := ContainerDescriptionExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

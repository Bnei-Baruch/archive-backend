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

// CatalogDescription is an object representing the database table.
type CatalogDescription struct {
	ID        int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	CatalogID int         `boil:"catalog_id" json:"catalog_id" toml:"catalog_id" yaml:"catalog_id"`
	Name      null.String `boil:"name" json:"name,omitempty" toml:"name" yaml:"name,omitempty"`
	LangID    null.String `boil:"lang_id" json:"lang_id,omitempty" toml:"lang_id" yaml:"lang_id,omitempty"`
	CreatedAt null.Time   `boil:"created_at" json:"created_at,omitempty" toml:"created_at" yaml:"created_at,omitempty"`
	UpdatedAt null.Time   `boil:"updated_at" json:"updated_at,omitempty" toml:"updated_at" yaml:"updated_at,omitempty"`

	R *catalogDescriptionR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L catalogDescriptionL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// catalogDescriptionR is where relationships are stored.
type catalogDescriptionR struct {
	Catalog *Catalog
	Lang    *Language
}

// catalogDescriptionL is where Load methods for each relationship are stored.
type catalogDescriptionL struct{}

var (
	catalogDescriptionColumns               = []string{"id", "catalog_id", "name", "lang_id", "created_at", "updated_at"}
	catalogDescriptionColumnsWithoutDefault = []string{"name", "lang_id", "created_at", "updated_at"}
	catalogDescriptionColumnsWithDefault    = []string{"id", "catalog_id"}
	catalogDescriptionPrimaryKeyColumns     = []string{"id"}
)

type (
	// CatalogDescriptionSlice is an alias for a slice of pointers to CatalogDescription.
	// This should generally be used opposed to []CatalogDescription.
	CatalogDescriptionSlice []*CatalogDescription

	catalogDescriptionQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	catalogDescriptionType                 = reflect.TypeOf(&CatalogDescription{})
	catalogDescriptionMapping              = queries.MakeStructMapping(catalogDescriptionType)
	catalogDescriptionPrimaryKeyMapping, _ = queries.BindMapping(catalogDescriptionType, catalogDescriptionMapping, catalogDescriptionPrimaryKeyColumns)
	catalogDescriptionInsertCacheMut       sync.RWMutex
	catalogDescriptionInsertCache          = make(map[string]insertCache)
	catalogDescriptionUpdateCacheMut       sync.RWMutex
	catalogDescriptionUpdateCache          = make(map[string]updateCache)
	catalogDescriptionUpsertCacheMut       sync.RWMutex
	catalogDescriptionUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single catalogDescription record from the query, and panics on error.
func (q catalogDescriptionQuery) OneP() *CatalogDescription {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single catalogDescription record from the query.
func (q catalogDescriptionQuery) One() (*CatalogDescription, error) {
	o := &CatalogDescription{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for catalog_descriptions")
	}

	return o, nil
}

// AllP returns all CatalogDescription records from the query, and panics on error.
func (q catalogDescriptionQuery) AllP() CatalogDescriptionSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all CatalogDescription records from the query.
func (q catalogDescriptionQuery) All() (CatalogDescriptionSlice, error) {
	var o CatalogDescriptionSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to CatalogDescription slice")
	}

	return o, nil
}

// CountP returns the count of all CatalogDescription records in the query, and panics on error.
func (q catalogDescriptionQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all CatalogDescription records in the query.
func (q catalogDescriptionQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count catalog_descriptions rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q catalogDescriptionQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q catalogDescriptionQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if catalog_descriptions exists")
	}

	return count > 0, nil
}

// CatalogG pointed to by the foreign key.
func (o *CatalogDescription) CatalogG(mods ...qm.QueryMod) catalogQuery {
	return o.Catalog(boil.GetDB(), mods...)
}

// Catalog pointed to by the foreign key.
func (o *CatalogDescription) Catalog(exec boil.Executor, mods ...qm.QueryMod) catalogQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.CatalogID),
	}

	queryMods = append(queryMods, mods...)

	query := Catalogs(exec, queryMods...)
	queries.SetFrom(query.Query, "\"catalogs\"")

	return query
}

// LangG pointed to by the foreign key.
func (o *CatalogDescription) LangG(mods ...qm.QueryMod) languageQuery {
	return o.Lang(boil.GetDB(), mods...)
}

// Lang pointed to by the foreign key.
func (o *CatalogDescription) Lang(exec boil.Executor, mods ...qm.QueryMod) languageQuery {
	queryMods := []qm.QueryMod{
		qm.Where("code3=?", o.LangID),
	}

	queryMods = append(queryMods, mods...)

	query := Languages(exec, queryMods...)
	queries.SetFrom(query.Query, "\"languages\"")

	return query
}

// LoadCatalog allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (catalogDescriptionL) LoadCatalog(e boil.Executor, singular bool, maybeCatalogDescription interface{}) error {
	var slice []*CatalogDescription
	var object *CatalogDescription

	count := 1
	if singular {
		object = maybeCatalogDescription.(*CatalogDescription)
	} else {
		slice = *maybeCatalogDescription.(*CatalogDescriptionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &catalogDescriptionR{}
		}
		args[0] = object.CatalogID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &catalogDescriptionR{}
			}
			args[i] = obj.CatalogID
		}
	}

	query := fmt.Sprintf(
		"select * from \"catalogs\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Catalog")
	}
	defer results.Close()

	var resultSlice []*Catalog
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Catalog")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Catalog = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.CatalogID == foreign.ID {
				local.R.Catalog = foreign
				break
			}
		}
	}

	return nil
}

// LoadLang allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (catalogDescriptionL) LoadLang(e boil.Executor, singular bool, maybeCatalogDescription interface{}) error {
	var slice []*CatalogDescription
	var object *CatalogDescription

	count := 1
	if singular {
		object = maybeCatalogDescription.(*CatalogDescription)
	} else {
		slice = *maybeCatalogDescription.(*CatalogDescriptionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &catalogDescriptionR{}
		}
		args[0] = object.LangID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &catalogDescriptionR{}
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

// SetCatalog of the catalog_description to the related item.
// Sets o.R.Catalog to related.
// Adds o to related.R.CatalogDescriptions.
func (o *CatalogDescription) SetCatalog(exec boil.Executor, insert bool, related *Catalog) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"catalog_descriptions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"catalog_id"}),
		strmangle.WhereClause("\"", "\"", 2, catalogDescriptionPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.CatalogID = related.ID

	if o.R == nil {
		o.R = &catalogDescriptionR{
			Catalog: related,
		}
	} else {
		o.R.Catalog = related
	}

	if related.R == nil {
		related.R = &catalogR{
			CatalogDescriptions: CatalogDescriptionSlice{o},
		}
	} else {
		related.R.CatalogDescriptions = append(related.R.CatalogDescriptions, o)
	}

	return nil
}

// SetLang of the catalog_description to the related item.
// Sets o.R.Lang to related.
// Adds o to related.R.LangCatalogDescriptions.
func (o *CatalogDescription) SetLang(exec boil.Executor, insert bool, related *Language) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"catalog_descriptions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
		strmangle.WhereClause("\"", "\"", 2, catalogDescriptionPrimaryKeyColumns),
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
		o.R = &catalogDescriptionR{
			Lang: related,
		}
	} else {
		o.R.Lang = related
	}

	if related.R == nil {
		related.R = &languageR{
			LangCatalogDescriptions: CatalogDescriptionSlice{o},
		}
	} else {
		related.R.LangCatalogDescriptions = append(related.R.LangCatalogDescriptions, o)
	}

	return nil
}

// RemoveLang relationship.
// Sets o.R.Lang to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *CatalogDescription) RemoveLang(exec boil.Executor, related *Language) error {
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

	for i, ri := range related.R.LangCatalogDescriptions {
		if o.LangID.String != ri.LangID.String {
			continue
		}

		ln := len(related.R.LangCatalogDescriptions)
		if ln > 1 && i < ln-1 {
			related.R.LangCatalogDescriptions[i] = related.R.LangCatalogDescriptions[ln-1]
		}
		related.R.LangCatalogDescriptions = related.R.LangCatalogDescriptions[:ln-1]
		break
	}
	return nil
}

// CatalogDescriptionsG retrieves all records.
func CatalogDescriptionsG(mods ...qm.QueryMod) catalogDescriptionQuery {
	return CatalogDescriptions(boil.GetDB(), mods...)
}

// CatalogDescriptions retrieves all the records using an executor.
func CatalogDescriptions(exec boil.Executor, mods ...qm.QueryMod) catalogDescriptionQuery {
	mods = append(mods, qm.From("\"catalog_descriptions\""))
	return catalogDescriptionQuery{NewQuery(exec, mods...)}
}

// FindCatalogDescriptionG retrieves a single record by ID.
func FindCatalogDescriptionG(id int, selectCols ...string) (*CatalogDescription, error) {
	return FindCatalogDescription(boil.GetDB(), id, selectCols...)
}

// FindCatalogDescriptionGP retrieves a single record by ID, and panics on error.
func FindCatalogDescriptionGP(id int, selectCols ...string) *CatalogDescription {
	retobj, err := FindCatalogDescription(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindCatalogDescription retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindCatalogDescription(exec boil.Executor, id int, selectCols ...string) (*CatalogDescription, error) {
	catalogDescriptionObj := &CatalogDescription{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"catalog_descriptions\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(catalogDescriptionObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from catalog_descriptions")
	}

	return catalogDescriptionObj, nil
}

// FindCatalogDescriptionP retrieves a single record by ID with an executor, and panics on error.
func FindCatalogDescriptionP(exec boil.Executor, id int, selectCols ...string) *CatalogDescription {
	retobj, err := FindCatalogDescription(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *CatalogDescription) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *CatalogDescription) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *CatalogDescription) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *CatalogDescription) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no catalog_descriptions provided for insertion")
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

	nzDefaults := queries.NonZeroDefaultSet(catalogDescriptionColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	catalogDescriptionInsertCacheMut.RLock()
	cache, cached := catalogDescriptionInsertCache[key]
	catalogDescriptionInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			catalogDescriptionColumns,
			catalogDescriptionColumnsWithDefault,
			catalogDescriptionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(catalogDescriptionType, catalogDescriptionMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(catalogDescriptionType, catalogDescriptionMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"catalog_descriptions\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into catalog_descriptions")
	}

	if !cached {
		catalogDescriptionInsertCacheMut.Lock()
		catalogDescriptionInsertCache[key] = cache
		catalogDescriptionInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single CatalogDescription record. See Update for
// whitelist behavior description.
func (o *CatalogDescription) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single CatalogDescription record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *CatalogDescription) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the CatalogDescription, and panics on error.
// See Update for whitelist behavior description.
func (o *CatalogDescription) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the CatalogDescription.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *CatalogDescription) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	var err error
	key := makeCacheKey(whitelist, nil)
	catalogDescriptionUpdateCacheMut.RLock()
	cache, cached := catalogDescriptionUpdateCache[key]
	catalogDescriptionUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(catalogDescriptionColumns, catalogDescriptionPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update catalog_descriptions, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"catalog_descriptions\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, catalogDescriptionPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(catalogDescriptionType, catalogDescriptionMapping, append(wl, catalogDescriptionPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update catalog_descriptions row")
	}

	if !cached {
		catalogDescriptionUpdateCacheMut.Lock()
		catalogDescriptionUpdateCache[key] = cache
		catalogDescriptionUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q catalogDescriptionQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q catalogDescriptionQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for catalog_descriptions")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o CatalogDescriptionSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o CatalogDescriptionSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o CatalogDescriptionSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o CatalogDescriptionSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), catalogDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"catalog_descriptions\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(catalogDescriptionPrimaryKeyColumns), len(colNames)+1, len(catalogDescriptionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in catalogDescription slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *CatalogDescription) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *CatalogDescription) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *CatalogDescription) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *CatalogDescription) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no catalog_descriptions provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.Time.IsZero() {
		o.CreatedAt.Time = currTime
		o.CreatedAt.Valid = true
	}
	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	nzDefaults := queries.NonZeroDefaultSet(catalogDescriptionColumnsWithDefault, o)

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

	catalogDescriptionUpsertCacheMut.RLock()
	cache, cached := catalogDescriptionUpsertCache[key]
	catalogDescriptionUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			catalogDescriptionColumns,
			catalogDescriptionColumnsWithDefault,
			catalogDescriptionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			catalogDescriptionColumns,
			catalogDescriptionPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert catalog_descriptions, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(catalogDescriptionPrimaryKeyColumns))
			copy(conflict, catalogDescriptionPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"catalog_descriptions\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(catalogDescriptionType, catalogDescriptionMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(catalogDescriptionType, catalogDescriptionMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert catalog_descriptions")
	}

	if !cached {
		catalogDescriptionUpsertCacheMut.Lock()
		catalogDescriptionUpsertCache[key] = cache
		catalogDescriptionUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single CatalogDescription record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *CatalogDescription) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single CatalogDescription record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *CatalogDescription) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no CatalogDescription provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single CatalogDescription record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *CatalogDescription) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single CatalogDescription record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *CatalogDescription) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no CatalogDescription provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), catalogDescriptionPrimaryKeyMapping)
	sql := "DELETE FROM \"catalog_descriptions\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from catalog_descriptions")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q catalogDescriptionQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q catalogDescriptionQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no catalogDescriptionQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from catalog_descriptions")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o CatalogDescriptionSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o CatalogDescriptionSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no CatalogDescription slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o CatalogDescriptionSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o CatalogDescriptionSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no CatalogDescription slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), catalogDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"catalog_descriptions\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, catalogDescriptionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(catalogDescriptionPrimaryKeyColumns), 1, len(catalogDescriptionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from catalogDescription slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *CatalogDescription) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *CatalogDescription) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *CatalogDescription) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no CatalogDescription provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *CatalogDescription) Reload(exec boil.Executor) error {
	ret, err := FindCatalogDescription(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *CatalogDescriptionSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *CatalogDescriptionSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *CatalogDescriptionSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty CatalogDescriptionSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *CatalogDescriptionSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	catalogDescriptions := CatalogDescriptionSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), catalogDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"catalog_descriptions\".* FROM \"catalog_descriptions\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, catalogDescriptionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(catalogDescriptionPrimaryKeyColumns), 1, len(catalogDescriptionPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&catalogDescriptions)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in CatalogDescriptionSlice")
	}

	*o = catalogDescriptions

	return nil
}

// CatalogDescriptionExists checks if the CatalogDescription row exists.
func CatalogDescriptionExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"catalog_descriptions\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if catalog_descriptions exists")
	}

	return exists, nil
}

// CatalogDescriptionExistsG checks if the CatalogDescription row exists.
func CatalogDescriptionExistsG(id int) (bool, error) {
	return CatalogDescriptionExists(boil.GetDB(), id)
}

// CatalogDescriptionExistsGP checks if the CatalogDescription row exists. Panics on error.
func CatalogDescriptionExistsGP(id int) bool {
	e, err := CatalogDescriptionExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// CatalogDescriptionExistsP checks if the CatalogDescription row exists. Panics on error.
func CatalogDescriptionExistsP(exec boil.Executor, id int) bool {
	e, err := CatalogDescriptionExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

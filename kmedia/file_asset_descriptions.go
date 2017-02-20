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

// FileAssetDescription is an object representing the database table.
type FileAssetDescription struct {
	ID        int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	FileID    int         `boil:"file_id" json:"file_id" toml:"file_id" yaml:"file_id"`
	Filedesc  null.String `boil:"filedesc" json:"filedesc,omitempty" toml:"filedesc" yaml:"filedesc,omitempty"`
	LangID    null.String `boil:"lang_id" json:"lang_id,omitempty" toml:"lang_id" yaml:"lang_id,omitempty"`
	CreatedAt null.Time   `boil:"created_at" json:"created_at,omitempty" toml:"created_at" yaml:"created_at,omitempty"`
	UpdatedAt null.Time   `boil:"updated_at" json:"updated_at,omitempty" toml:"updated_at" yaml:"updated_at,omitempty"`

	R *fileAssetDescriptionR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L fileAssetDescriptionL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// fileAssetDescriptionR is where relationships are stored.
type fileAssetDescriptionR struct {
	File *FileAsset
	Lang *Language
}

// fileAssetDescriptionL is where Load methods for each relationship are stored.
type fileAssetDescriptionL struct{}

var (
	fileAssetDescriptionColumns               = []string{"id", "file_id", "filedesc", "lang_id", "created_at", "updated_at"}
	fileAssetDescriptionColumnsWithoutDefault = []string{"filedesc", "lang_id", "created_at", "updated_at"}
	fileAssetDescriptionColumnsWithDefault    = []string{"id", "file_id"}
	fileAssetDescriptionPrimaryKeyColumns     = []string{"id"}
)

type (
	// FileAssetDescriptionSlice is an alias for a slice of pointers to FileAssetDescription.
	// This should generally be used opposed to []FileAssetDescription.
	FileAssetDescriptionSlice []*FileAssetDescription

	fileAssetDescriptionQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	fileAssetDescriptionType                 = reflect.TypeOf(&FileAssetDescription{})
	fileAssetDescriptionMapping              = queries.MakeStructMapping(fileAssetDescriptionType)
	fileAssetDescriptionPrimaryKeyMapping, _ = queries.BindMapping(fileAssetDescriptionType, fileAssetDescriptionMapping, fileAssetDescriptionPrimaryKeyColumns)
	fileAssetDescriptionInsertCacheMut       sync.RWMutex
	fileAssetDescriptionInsertCache          = make(map[string]insertCache)
	fileAssetDescriptionUpdateCacheMut       sync.RWMutex
	fileAssetDescriptionUpdateCache          = make(map[string]updateCache)
	fileAssetDescriptionUpsertCacheMut       sync.RWMutex
	fileAssetDescriptionUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single fileAssetDescription record from the query, and panics on error.
func (q fileAssetDescriptionQuery) OneP() *FileAssetDescription {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single fileAssetDescription record from the query.
func (q fileAssetDescriptionQuery) One() (*FileAssetDescription, error) {
	o := &FileAssetDescription{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for file_asset_descriptions")
	}

	return o, nil
}

// AllP returns all FileAssetDescription records from the query, and panics on error.
func (q fileAssetDescriptionQuery) AllP() FileAssetDescriptionSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all FileAssetDescription records from the query.
func (q fileAssetDescriptionQuery) All() (FileAssetDescriptionSlice, error) {
	var o FileAssetDescriptionSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to FileAssetDescription slice")
	}

	return o, nil
}

// CountP returns the count of all FileAssetDescription records in the query, and panics on error.
func (q fileAssetDescriptionQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all FileAssetDescription records in the query.
func (q fileAssetDescriptionQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count file_asset_descriptions rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q fileAssetDescriptionQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q fileAssetDescriptionQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if file_asset_descriptions exists")
	}

	return count > 0, nil
}

// FileG pointed to by the foreign key.
func (o *FileAssetDescription) FileG(mods ...qm.QueryMod) fileAssetQuery {
	return o.File(boil.GetDB(), mods...)
}

// File pointed to by the foreign key.
func (o *FileAssetDescription) File(exec boil.Executor, mods ...qm.QueryMod) fileAssetQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.FileID),
	}

	queryMods = append(queryMods, mods...)

	query := FileAssets(exec, queryMods...)
	queries.SetFrom(query.Query, "\"file_assets\"")

	return query
}

// LangG pointed to by the foreign key.
func (o *FileAssetDescription) LangG(mods ...qm.QueryMod) languageQuery {
	return o.Lang(boil.GetDB(), mods...)
}

// Lang pointed to by the foreign key.
func (o *FileAssetDescription) Lang(exec boil.Executor, mods ...qm.QueryMod) languageQuery {
	queryMods := []qm.QueryMod{
		qm.Where("code3=?", o.LangID),
	}

	queryMods = append(queryMods, mods...)

	query := Languages(exec, queryMods...)
	queries.SetFrom(query.Query, "\"languages\"")

	return query
}

// LoadFile allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (fileAssetDescriptionL) LoadFile(e boil.Executor, singular bool, maybeFileAssetDescription interface{}) error {
	var slice []*FileAssetDescription
	var object *FileAssetDescription

	count := 1
	if singular {
		object = maybeFileAssetDescription.(*FileAssetDescription)
	} else {
		slice = *maybeFileAssetDescription.(*FileAssetDescriptionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &fileAssetDescriptionR{}
		}
		args[0] = object.FileID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &fileAssetDescriptionR{}
			}
			args[i] = obj.FileID
		}
	}

	query := fmt.Sprintf(
		"select * from \"file_assets\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load FileAsset")
	}
	defer results.Close()

	var resultSlice []*FileAsset
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice FileAsset")
	}

	if singular && len(resultSlice) != 0 {
		object.R.File = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.FileID == foreign.ID {
				local.R.File = foreign
				break
			}
		}
	}

	return nil
}

// LoadLang allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (fileAssetDescriptionL) LoadLang(e boil.Executor, singular bool, maybeFileAssetDescription interface{}) error {
	var slice []*FileAssetDescription
	var object *FileAssetDescription

	count := 1
	if singular {
		object = maybeFileAssetDescription.(*FileAssetDescription)
	} else {
		slice = *maybeFileAssetDescription.(*FileAssetDescriptionSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &fileAssetDescriptionR{}
		}
		args[0] = object.LangID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &fileAssetDescriptionR{}
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

// SetFile of the file_asset_description to the related item.
// Sets o.R.File to related.
// Adds o to related.R.FileFileAssetDescriptions.
func (o *FileAssetDescription) SetFile(exec boil.Executor, insert bool, related *FileAsset) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"file_asset_descriptions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"file_id"}),
		strmangle.WhereClause("\"", "\"", 2, fileAssetDescriptionPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.FileID = related.ID

	if o.R == nil {
		o.R = &fileAssetDescriptionR{
			File: related,
		}
	} else {
		o.R.File = related
	}

	if related.R == nil {
		related.R = &fileAssetR{
			FileFileAssetDescriptions: FileAssetDescriptionSlice{o},
		}
	} else {
		related.R.FileFileAssetDescriptions = append(related.R.FileFileAssetDescriptions, o)
	}

	return nil
}

// SetLang of the file_asset_description to the related item.
// Sets o.R.Lang to related.
// Adds o to related.R.LangFileAssetDescriptions.
func (o *FileAssetDescription) SetLang(exec boil.Executor, insert bool, related *Language) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"file_asset_descriptions\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
		strmangle.WhereClause("\"", "\"", 2, fileAssetDescriptionPrimaryKeyColumns),
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
		o.R = &fileAssetDescriptionR{
			Lang: related,
		}
	} else {
		o.R.Lang = related
	}

	if related.R == nil {
		related.R = &languageR{
			LangFileAssetDescriptions: FileAssetDescriptionSlice{o},
		}
	} else {
		related.R.LangFileAssetDescriptions = append(related.R.LangFileAssetDescriptions, o)
	}

	return nil
}

// RemoveLang relationship.
// Sets o.R.Lang to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *FileAssetDescription) RemoveLang(exec boil.Executor, related *Language) error {
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

	for i, ri := range related.R.LangFileAssetDescriptions {
		if o.LangID.String != ri.LangID.String {
			continue
		}

		ln := len(related.R.LangFileAssetDescriptions)
		if ln > 1 && i < ln-1 {
			related.R.LangFileAssetDescriptions[i] = related.R.LangFileAssetDescriptions[ln-1]
		}
		related.R.LangFileAssetDescriptions = related.R.LangFileAssetDescriptions[:ln-1]
		break
	}
	return nil
}

// FileAssetDescriptionsG retrieves all records.
func FileAssetDescriptionsG(mods ...qm.QueryMod) fileAssetDescriptionQuery {
	return FileAssetDescriptions(boil.GetDB(), mods...)
}

// FileAssetDescriptions retrieves all the records using an executor.
func FileAssetDescriptions(exec boil.Executor, mods ...qm.QueryMod) fileAssetDescriptionQuery {
	mods = append(mods, qm.From("\"file_asset_descriptions\""))
	return fileAssetDescriptionQuery{NewQuery(exec, mods...)}
}

// FindFileAssetDescriptionG retrieves a single record by ID.
func FindFileAssetDescriptionG(id int, selectCols ...string) (*FileAssetDescription, error) {
	return FindFileAssetDescription(boil.GetDB(), id, selectCols...)
}

// FindFileAssetDescriptionGP retrieves a single record by ID, and panics on error.
func FindFileAssetDescriptionGP(id int, selectCols ...string) *FileAssetDescription {
	retobj, err := FindFileAssetDescription(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindFileAssetDescription retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindFileAssetDescription(exec boil.Executor, id int, selectCols ...string) (*FileAssetDescription, error) {
	fileAssetDescriptionObj := &FileAssetDescription{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"file_asset_descriptions\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(fileAssetDescriptionObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from file_asset_descriptions")
	}

	return fileAssetDescriptionObj, nil
}

// FindFileAssetDescriptionP retrieves a single record by ID with an executor, and panics on error.
func FindFileAssetDescriptionP(exec boil.Executor, id int, selectCols ...string) *FileAssetDescription {
	retobj, err := FindFileAssetDescription(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *FileAssetDescription) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *FileAssetDescription) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *FileAssetDescription) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *FileAssetDescription) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no file_asset_descriptions provided for insertion")
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

	nzDefaults := queries.NonZeroDefaultSet(fileAssetDescriptionColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	fileAssetDescriptionInsertCacheMut.RLock()
	cache, cached := fileAssetDescriptionInsertCache[key]
	fileAssetDescriptionInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			fileAssetDescriptionColumns,
			fileAssetDescriptionColumnsWithDefault,
			fileAssetDescriptionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(fileAssetDescriptionType, fileAssetDescriptionMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(fileAssetDescriptionType, fileAssetDescriptionMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"file_asset_descriptions\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into file_asset_descriptions")
	}

	if !cached {
		fileAssetDescriptionInsertCacheMut.Lock()
		fileAssetDescriptionInsertCache[key] = cache
		fileAssetDescriptionInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single FileAssetDescription record. See Update for
// whitelist behavior description.
func (o *FileAssetDescription) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single FileAssetDescription record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *FileAssetDescription) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the FileAssetDescription, and panics on error.
// See Update for whitelist behavior description.
func (o *FileAssetDescription) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the FileAssetDescription.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *FileAssetDescription) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	var err error
	key := makeCacheKey(whitelist, nil)
	fileAssetDescriptionUpdateCacheMut.RLock()
	cache, cached := fileAssetDescriptionUpdateCache[key]
	fileAssetDescriptionUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(fileAssetDescriptionColumns, fileAssetDescriptionPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update file_asset_descriptions, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"file_asset_descriptions\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, fileAssetDescriptionPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(fileAssetDescriptionType, fileAssetDescriptionMapping, append(wl, fileAssetDescriptionPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update file_asset_descriptions row")
	}

	if !cached {
		fileAssetDescriptionUpdateCacheMut.Lock()
		fileAssetDescriptionUpdateCache[key] = cache
		fileAssetDescriptionUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q fileAssetDescriptionQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q fileAssetDescriptionQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for file_asset_descriptions")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o FileAssetDescriptionSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o FileAssetDescriptionSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o FileAssetDescriptionSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o FileAssetDescriptionSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), fileAssetDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"file_asset_descriptions\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(fileAssetDescriptionPrimaryKeyColumns), len(colNames)+1, len(fileAssetDescriptionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in fileAssetDescription slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *FileAssetDescription) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *FileAssetDescription) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *FileAssetDescription) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *FileAssetDescription) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no file_asset_descriptions provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.Time.IsZero() {
		o.CreatedAt.Time = currTime
		o.CreatedAt.Valid = true
	}
	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	nzDefaults := queries.NonZeroDefaultSet(fileAssetDescriptionColumnsWithDefault, o)

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

	fileAssetDescriptionUpsertCacheMut.RLock()
	cache, cached := fileAssetDescriptionUpsertCache[key]
	fileAssetDescriptionUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			fileAssetDescriptionColumns,
			fileAssetDescriptionColumnsWithDefault,
			fileAssetDescriptionColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			fileAssetDescriptionColumns,
			fileAssetDescriptionPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert file_asset_descriptions, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(fileAssetDescriptionPrimaryKeyColumns))
			copy(conflict, fileAssetDescriptionPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"file_asset_descriptions\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(fileAssetDescriptionType, fileAssetDescriptionMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(fileAssetDescriptionType, fileAssetDescriptionMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert file_asset_descriptions")
	}

	if !cached {
		fileAssetDescriptionUpsertCacheMut.Lock()
		fileAssetDescriptionUpsertCache[key] = cache
		fileAssetDescriptionUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single FileAssetDescription record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *FileAssetDescription) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single FileAssetDescription record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *FileAssetDescription) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no FileAssetDescription provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single FileAssetDescription record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *FileAssetDescription) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single FileAssetDescription record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *FileAssetDescription) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no FileAssetDescription provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), fileAssetDescriptionPrimaryKeyMapping)
	sql := "DELETE FROM \"file_asset_descriptions\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from file_asset_descriptions")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q fileAssetDescriptionQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q fileAssetDescriptionQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no fileAssetDescriptionQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from file_asset_descriptions")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o FileAssetDescriptionSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o FileAssetDescriptionSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no FileAssetDescription slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o FileAssetDescriptionSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o FileAssetDescriptionSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no FileAssetDescription slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), fileAssetDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"file_asset_descriptions\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, fileAssetDescriptionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(fileAssetDescriptionPrimaryKeyColumns), 1, len(fileAssetDescriptionPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from fileAssetDescription slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *FileAssetDescription) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *FileAssetDescription) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *FileAssetDescription) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no FileAssetDescription provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *FileAssetDescription) Reload(exec boil.Executor) error {
	ret, err := FindFileAssetDescription(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *FileAssetDescriptionSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *FileAssetDescriptionSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *FileAssetDescriptionSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty FileAssetDescriptionSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *FileAssetDescriptionSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	fileAssetDescriptions := FileAssetDescriptionSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), fileAssetDescriptionPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"file_asset_descriptions\".* FROM \"file_asset_descriptions\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, fileAssetDescriptionPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(fileAssetDescriptionPrimaryKeyColumns), 1, len(fileAssetDescriptionPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&fileAssetDescriptions)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in FileAssetDescriptionSlice")
	}

	*o = fileAssetDescriptions

	return nil
}

// FileAssetDescriptionExists checks if the FileAssetDescription row exists.
func FileAssetDescriptionExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"file_asset_descriptions\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if file_asset_descriptions exists")
	}

	return exists, nil
}

// FileAssetDescriptionExistsG checks if the FileAssetDescription row exists.
func FileAssetDescriptionExistsG(id int) (bool, error) {
	return FileAssetDescriptionExists(boil.GetDB(), id)
}

// FileAssetDescriptionExistsGP checks if the FileAssetDescription row exists. Panics on error.
func FileAssetDescriptionExistsGP(id int) bool {
	e, err := FileAssetDescriptionExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// FileAssetDescriptionExistsP checks if the FileAssetDescription row exists. Panics on error.
func FileAssetDescriptionExistsP(exec boil.Executor, id int) bool {
	e, err := FileAssetDescriptionExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

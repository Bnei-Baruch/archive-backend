package mdbmodels

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

// TagsI18n is an object representing the database table.
type TagsI18n struct {
	TagID            int64       `boil:"tag_id" json:"tag_id" toml:"tag_id" yaml:"tag_id"`
	Language         string      `boil:"language" json:"language" toml:"language" yaml:"language"`
	OriginalLanguage null.String `boil:"original_language" json:"original_language,omitempty" toml:"original_language" yaml:"original_language,omitempty"`
	Label            null.String `boil:"label" json:"label,omitempty" toml:"label" yaml:"label,omitempty"`
	UserID           null.Int64  `boil:"user_id" json:"user_id,omitempty" toml:"user_id" yaml:"user_id,omitempty"`
	CreatedAt        time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`

	R *tagsI18nR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L tagsI18nL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// tagsI18nR is where relationships are stored.
type tagsI18nR struct {
	Tag  *Tag
	User *User
}

// tagsI18nL is where Load methods for each relationship are stored.
type tagsI18nL struct{}

var (
	tagsI18nColumns               = []string{"tag_id", "language", "original_language", "label", "user_id", "created_at"}
	tagsI18nColumnsWithoutDefault = []string{"tag_id", "language", "original_language", "label", "user_id"}
	tagsI18nColumnsWithDefault    = []string{"created_at"}
	tagsI18nPrimaryKeyColumns     = []string{"tag_id", "language"}
)

type (
	// TagsI18nSlice is an alias for a slice of pointers to TagsI18n.
	// This should generally be used opposed to []TagsI18n.
	TagsI18nSlice []*TagsI18n

	tagsI18nQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	tagsI18nType                 = reflect.TypeOf(&TagsI18n{})
	tagsI18nMapping              = queries.MakeStructMapping(tagsI18nType)
	tagsI18nPrimaryKeyMapping, _ = queries.BindMapping(tagsI18nType, tagsI18nMapping, tagsI18nPrimaryKeyColumns)
	tagsI18nInsertCacheMut       sync.RWMutex
	tagsI18nInsertCache          = make(map[string]insertCache)
	tagsI18nUpdateCacheMut       sync.RWMutex
	tagsI18nUpdateCache          = make(map[string]updateCache)
	tagsI18nUpsertCacheMut       sync.RWMutex
	tagsI18nUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single tagsI18n record from the query, and panics on error.
func (q tagsI18nQuery) OneP() *TagsI18n {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single tagsI18n record from the query.
func (q tagsI18nQuery) One() (*TagsI18n, error) {
	o := &TagsI18n{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: failed to execute a one query for tags_i18n")
	}

	return o, nil
}

// AllP returns all TagsI18n records from the query, and panics on error.
func (q tagsI18nQuery) AllP() TagsI18nSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all TagsI18n records from the query.
func (q tagsI18nQuery) All() (TagsI18nSlice, error) {
	var o TagsI18nSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "mdbmodels: failed to assign all query results to TagsI18n slice")
	}

	return o, nil
}

// CountP returns the count of all TagsI18n records in the query, and panics on error.
func (q tagsI18nQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all TagsI18n records in the query.
func (q tagsI18nQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to count tags_i18n rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q tagsI18nQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q tagsI18nQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: failed to check if tags_i18n exists")
	}

	return count > 0, nil
}

// TagG pointed to by the foreign key.
func (o *TagsI18n) TagG(mods ...qm.QueryMod) tagQuery {
	return o.Tag(boil.GetDB(), mods...)
}

// Tag pointed to by the foreign key.
func (o *TagsI18n) Tag(exec boil.Executor, mods ...qm.QueryMod) tagQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.TagID),
	}

	queryMods = append(queryMods, mods...)

	query := Tags(exec, queryMods...)
	queries.SetFrom(query.Query, "\"tags\"")

	return query
}

// UserG pointed to by the foreign key.
func (o *TagsI18n) UserG(mods ...qm.QueryMod) userQuery {
	return o.User(boil.GetDB(), mods...)
}

// User pointed to by the foreign key.
func (o *TagsI18n) User(exec boil.Executor, mods ...qm.QueryMod) userQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.UserID),
	}

	queryMods = append(queryMods, mods...)

	query := Users(exec, queryMods...)
	queries.SetFrom(query.Query, "\"users\"")

	return query
}

// LoadTag allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (tagsI18nL) LoadTag(e boil.Executor, singular bool, maybeTagsI18n interface{}) error {
	var slice []*TagsI18n
	var object *TagsI18n

	count := 1
	if singular {
		object = maybeTagsI18n.(*TagsI18n)
	} else {
		slice = *maybeTagsI18n.(*TagsI18nSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &tagsI18nR{}
		}
		args[0] = object.TagID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &tagsI18nR{}
			}
			args[i] = obj.TagID
		}
	}

	query := fmt.Sprintf(
		"select * from \"tags\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Tag")
	}
	defer results.Close()

	var resultSlice []*Tag
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Tag")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Tag = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.TagID == foreign.ID {
				local.R.Tag = foreign
				break
			}
		}
	}

	return nil
}

// LoadUser allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (tagsI18nL) LoadUser(e boil.Executor, singular bool, maybeTagsI18n interface{}) error {
	var slice []*TagsI18n
	var object *TagsI18n

	count := 1
	if singular {
		object = maybeTagsI18n.(*TagsI18n)
	} else {
		slice = *maybeTagsI18n.(*TagsI18nSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &tagsI18nR{}
		}
		args[0] = object.UserID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &tagsI18nR{}
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
			if local.UserID.Int64 == foreign.ID {
				local.R.User = foreign
				break
			}
		}
	}

	return nil
}

// SetTagG of the tags_i18n to the related item.
// Sets o.R.Tag to related.
// Adds o to related.R.TagsI18ns.
// Uses the global database handle.
func (o *TagsI18n) SetTagG(insert bool, related *Tag) error {
	return o.SetTag(boil.GetDB(), insert, related)
}

// SetTagP of the tags_i18n to the related item.
// Sets o.R.Tag to related.
// Adds o to related.R.TagsI18ns.
// Panics on error.
func (o *TagsI18n) SetTagP(exec boil.Executor, insert bool, related *Tag) {
	if err := o.SetTag(exec, insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetTagGP of the tags_i18n to the related item.
// Sets o.R.Tag to related.
// Adds o to related.R.TagsI18ns.
// Uses the global database handle and panics on error.
func (o *TagsI18n) SetTagGP(insert bool, related *Tag) {
	if err := o.SetTag(boil.GetDB(), insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetTag of the tags_i18n to the related item.
// Sets o.R.Tag to related.
// Adds o to related.R.TagsI18ns.
func (o *TagsI18n) SetTag(exec boil.Executor, insert bool, related *Tag) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"tags_i18n\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"tag_id"}),
		strmangle.WhereClause("\"", "\"", 2, tagsI18nPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.TagID, o.Language}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.TagID = related.ID

	if o.R == nil {
		o.R = &tagsI18nR{
			Tag: related,
		}
	} else {
		o.R.Tag = related
	}

	if related.R == nil {
		related.R = &tagR{
			TagsI18ns: TagsI18nSlice{o},
		}
	} else {
		related.R.TagsI18ns = append(related.R.TagsI18ns, o)
	}

	return nil
}

// SetUserG of the tags_i18n to the related item.
// Sets o.R.User to related.
// Adds o to related.R.TagsI18ns.
// Uses the global database handle.
func (o *TagsI18n) SetUserG(insert bool, related *User) error {
	return o.SetUser(boil.GetDB(), insert, related)
}

// SetUserP of the tags_i18n to the related item.
// Sets o.R.User to related.
// Adds o to related.R.TagsI18ns.
// Panics on error.
func (o *TagsI18n) SetUserP(exec boil.Executor, insert bool, related *User) {
	if err := o.SetUser(exec, insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetUserGP of the tags_i18n to the related item.
// Sets o.R.User to related.
// Adds o to related.R.TagsI18ns.
// Uses the global database handle and panics on error.
func (o *TagsI18n) SetUserGP(insert bool, related *User) {
	if err := o.SetUser(boil.GetDB(), insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetUser of the tags_i18n to the related item.
// Sets o.R.User to related.
// Adds o to related.R.TagsI18ns.
func (o *TagsI18n) SetUser(exec boil.Executor, insert bool, related *User) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"tags_i18n\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
		strmangle.WhereClause("\"", "\"", 2, tagsI18nPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.TagID, o.Language}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.UserID.Int64 = related.ID
	o.UserID.Valid = true

	if o.R == nil {
		o.R = &tagsI18nR{
			User: related,
		}
	} else {
		o.R.User = related
	}

	if related.R == nil {
		related.R = &userR{
			TagsI18ns: TagsI18nSlice{o},
		}
	} else {
		related.R.TagsI18ns = append(related.R.TagsI18ns, o)
	}

	return nil
}

// RemoveUserG relationship.
// Sets o.R.User to nil.
// Removes o from all passed in related items' relationships struct (Optional).
// Uses the global database handle.
func (o *TagsI18n) RemoveUserG(related *User) error {
	return o.RemoveUser(boil.GetDB(), related)
}

// RemoveUserP relationship.
// Sets o.R.User to nil.
// Removes o from all passed in related items' relationships struct (Optional).
// Panics on error.
func (o *TagsI18n) RemoveUserP(exec boil.Executor, related *User) {
	if err := o.RemoveUser(exec, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// RemoveUserGP relationship.
// Sets o.R.User to nil.
// Removes o from all passed in related items' relationships struct (Optional).
// Uses the global database handle and panics on error.
func (o *TagsI18n) RemoveUserGP(related *User) {
	if err := o.RemoveUser(boil.GetDB(), related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// RemoveUser relationship.
// Sets o.R.User to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *TagsI18n) RemoveUser(exec boil.Executor, related *User) error {
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

	for i, ri := range related.R.TagsI18ns {
		if o.UserID.Int64 != ri.UserID.Int64 {
			continue
		}

		ln := len(related.R.TagsI18ns)
		if ln > 1 && i < ln-1 {
			related.R.TagsI18ns[i] = related.R.TagsI18ns[ln-1]
		}
		related.R.TagsI18ns = related.R.TagsI18ns[:ln-1]
		break
	}
	return nil
}

// TagsI18nsG retrieves all records.
func TagsI18nsG(mods ...qm.QueryMod) tagsI18nQuery {
	return TagsI18ns(boil.GetDB(), mods...)
}

// TagsI18ns retrieves all the records using an executor.
func TagsI18ns(exec boil.Executor, mods ...qm.QueryMod) tagsI18nQuery {
	mods = append(mods, qm.From("\"tags_i18n\""))
	return tagsI18nQuery{NewQuery(exec, mods...)}
}

// FindTagsI18nG retrieves a single record by ID.
func FindTagsI18nG(tagID int64, language string, selectCols ...string) (*TagsI18n, error) {
	return FindTagsI18n(boil.GetDB(), tagID, language, selectCols...)
}

// FindTagsI18nGP retrieves a single record by ID, and panics on error.
func FindTagsI18nGP(tagID int64, language string, selectCols ...string) *TagsI18n {
	retobj, err := FindTagsI18n(boil.GetDB(), tagID, language, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindTagsI18n retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindTagsI18n(exec boil.Executor, tagID int64, language string, selectCols ...string) (*TagsI18n, error) {
	tagsI18nObj := &TagsI18n{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"tags_i18n\" where \"tag_id\"=$1 AND \"language\"=$2", sel,
	)

	q := queries.Raw(exec, query, tagID, language)

	err := q.Bind(tagsI18nObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: unable to select from tags_i18n")
	}

	return tagsI18nObj, nil
}

// FindTagsI18nP retrieves a single record by ID with an executor, and panics on error.
func FindTagsI18nP(exec boil.Executor, tagID int64, language string, selectCols ...string) *TagsI18n {
	retobj, err := FindTagsI18n(exec, tagID, language, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *TagsI18n) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *TagsI18n) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *TagsI18n) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *TagsI18n) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no tags_i18n provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(tagsI18nColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	tagsI18nInsertCacheMut.RLock()
	cache, cached := tagsI18nInsertCache[key]
	tagsI18nInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			tagsI18nColumns,
			tagsI18nColumnsWithDefault,
			tagsI18nColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(tagsI18nType, tagsI18nMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(tagsI18nType, tagsI18nMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"tags_i18n\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "mdbmodels: unable to insert into tags_i18n")
	}

	if !cached {
		tagsI18nInsertCacheMut.Lock()
		tagsI18nInsertCache[key] = cache
		tagsI18nInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single TagsI18n record. See Update for
// whitelist behavior description.
func (o *TagsI18n) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single TagsI18n record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *TagsI18n) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the TagsI18n, and panics on error.
// See Update for whitelist behavior description.
func (o *TagsI18n) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the TagsI18n.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *TagsI18n) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	tagsI18nUpdateCacheMut.RLock()
	cache, cached := tagsI18nUpdateCache[key]
	tagsI18nUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(tagsI18nColumns, tagsI18nPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("mdbmodels: unable to update tags_i18n, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"tags_i18n\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, tagsI18nPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(tagsI18nType, tagsI18nMapping, append(wl, tagsI18nPrimaryKeyColumns...))
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
		return errors.Wrap(err, "mdbmodels: unable to update tags_i18n row")
	}

	if !cached {
		tagsI18nUpdateCacheMut.Lock()
		tagsI18nUpdateCache[key] = cache
		tagsI18nUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q tagsI18nQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q tagsI18nQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all for tags_i18n")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o TagsI18nSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o TagsI18nSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o TagsI18nSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o TagsI18nSlice) UpdateAll(exec boil.Executor, cols M) error {
	ln := int64(len(o))
	if ln == 0 {
		return nil
	}

	if len(cols) == 0 {
		return errors.New("mdbmodels: update all requires at least one column argument")
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), tagsI18nPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"tags_i18n\" SET %s WHERE (\"tag_id\",\"language\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(tagsI18nPrimaryKeyColumns), len(colNames)+1, len(tagsI18nPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all in tagsI18n slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *TagsI18n) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *TagsI18n) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *TagsI18n) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *TagsI18n) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no tags_i18n provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(tagsI18nColumnsWithDefault, o)

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

	tagsI18nUpsertCacheMut.RLock()
	cache, cached := tagsI18nUpsertCache[key]
	tagsI18nUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			tagsI18nColumns,
			tagsI18nColumnsWithDefault,
			tagsI18nColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			tagsI18nColumns,
			tagsI18nPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("mdbmodels: unable to upsert tags_i18n, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(tagsI18nPrimaryKeyColumns))
			copy(conflict, tagsI18nPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"tags_i18n\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(tagsI18nType, tagsI18nMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(tagsI18nType, tagsI18nMapping, ret)
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
		return errors.Wrap(err, "mdbmodels: unable to upsert tags_i18n")
	}

	if !cached {
		tagsI18nUpsertCacheMut.Lock()
		tagsI18nUpsertCache[key] = cache
		tagsI18nUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single TagsI18n record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *TagsI18n) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single TagsI18n record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *TagsI18n) DeleteG() error {
	if o == nil {
		return errors.New("mdbmodels: no TagsI18n provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single TagsI18n record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *TagsI18n) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single TagsI18n record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *TagsI18n) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no TagsI18n provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), tagsI18nPrimaryKeyMapping)
	sql := "DELETE FROM \"tags_i18n\" WHERE \"tag_id\"=$1 AND \"language\"=$2"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete from tags_i18n")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q tagsI18nQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q tagsI18nQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("mdbmodels: no tagsI18nQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from tags_i18n")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o TagsI18nSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o TagsI18nSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("mdbmodels: no TagsI18n slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o TagsI18nSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o TagsI18nSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no TagsI18n slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), tagsI18nPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"tags_i18n\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, tagsI18nPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(tagsI18nPrimaryKeyColumns), 1, len(tagsI18nPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from tagsI18n slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *TagsI18n) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *TagsI18n) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *TagsI18n) ReloadG() error {
	if o == nil {
		return errors.New("mdbmodels: no TagsI18n provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *TagsI18n) Reload(exec boil.Executor) error {
	ret, err := FindTagsI18n(exec, o.TagID, o.Language)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *TagsI18nSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *TagsI18nSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *TagsI18nSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("mdbmodels: empty TagsI18nSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *TagsI18nSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	tagsI18ns := TagsI18nSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), tagsI18nPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"tags_i18n\".* FROM \"tags_i18n\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, tagsI18nPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(tagsI18nPrimaryKeyColumns), 1, len(tagsI18nPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&tagsI18ns)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to reload all in TagsI18nSlice")
	}

	*o = tagsI18ns

	return nil
}

// TagsI18nExists checks if the TagsI18n row exists.
func TagsI18nExists(exec boil.Executor, tagID int64, language string) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"tags_i18n\" where \"tag_id\"=$1 AND \"language\"=$2 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, tagID, language)
	}

	row := exec.QueryRow(sql, tagID, language)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: unable to check if tags_i18n exists")
	}

	return exists, nil
}

// TagsI18nExistsG checks if the TagsI18n row exists.
func TagsI18nExistsG(tagID int64, language string) (bool, error) {
	return TagsI18nExists(boil.GetDB(), tagID, language)
}

// TagsI18nExistsGP checks if the TagsI18n row exists. Panics on error.
func TagsI18nExistsGP(tagID int64, language string) bool {
	e, err := TagsI18nExists(boil.GetDB(), tagID, language)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// TagsI18nExistsP checks if the TagsI18n row exists. Panics on error.
func TagsI18nExistsP(exec boil.Executor, tagID int64, language string) bool {
	e, err := TagsI18nExists(exec, tagID, language)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

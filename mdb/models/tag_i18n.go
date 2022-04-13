// Code generated by SQLBoiler 4.8.6 (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package mdbmodels

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"github.com/volatiletech/sqlboiler/v4/queries/qmhelper"
	"github.com/volatiletech/strmangle"
)

// TagI18n is an object representing the database table.
type TagI18n struct {
	TagID            int64       `boil:"tag_id" json:"tag_id" toml:"tag_id" yaml:"tag_id"`
	Language         string      `boil:"language" json:"language" toml:"language" yaml:"language"`
	OriginalLanguage null.String `boil:"original_language" json:"original_language,omitempty" toml:"original_language" yaml:"original_language,omitempty"`
	Label            null.String `boil:"label" json:"label,omitempty" toml:"label" yaml:"label,omitempty"`
	UserID           null.Int64  `boil:"user_id" json:"user_id,omitempty" toml:"user_id" yaml:"user_id,omitempty"`
	CreatedAt        time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`

	R *tagI18nR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L tagI18nL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

var TagI18nColumns = struct {
	TagID            string
	Language         string
	OriginalLanguage string
	Label            string
	UserID           string
	CreatedAt        string
}{
	TagID:            "tag_id",
	Language:         "language",
	OriginalLanguage: "original_language",
	Label:            "label",
	UserID:           "user_id",
	CreatedAt:        "created_at",
}

var TagI18nTableColumns = struct {
	TagID            string
	Language         string
	OriginalLanguage string
	Label            string
	UserID           string
	CreatedAt        string
}{
	TagID:            "tag_i18n.tag_id",
	Language:         "tag_i18n.language",
	OriginalLanguage: "tag_i18n.original_language",
	Label:            "tag_i18n.label",
	UserID:           "tag_i18n.user_id",
	CreatedAt:        "tag_i18n.created_at",
}

// Generated where

var TagI18nWhere = struct {
	TagID            whereHelperint64
	Language         whereHelperstring
	OriginalLanguage whereHelpernull_String
	Label            whereHelpernull_String
	UserID           whereHelpernull_Int64
	CreatedAt        whereHelpertime_Time
}{
	TagID:            whereHelperint64{field: "\"tag_i18n\".\"tag_id\""},
	Language:         whereHelperstring{field: "\"tag_i18n\".\"language\""},
	OriginalLanguage: whereHelpernull_String{field: "\"tag_i18n\".\"original_language\""},
	Label:            whereHelpernull_String{field: "\"tag_i18n\".\"label\""},
	UserID:           whereHelpernull_Int64{field: "\"tag_i18n\".\"user_id\""},
	CreatedAt:        whereHelpertime_Time{field: "\"tag_i18n\".\"created_at\""},
}

// TagI18nRels is where relationship names are stored.
var TagI18nRels = struct {
	Tag  string
	User string
}{
	Tag:  "Tag",
	User: "User",
}

// tagI18nR is where relationships are stored.
type tagI18nR struct {
	Tag  *Tag  `boil:"Tag" json:"Tag" toml:"Tag" yaml:"Tag"`
	User *User `boil:"User" json:"User" toml:"User" yaml:"User"`
}

// NewStruct creates a new relationship struct
func (*tagI18nR) NewStruct() *tagI18nR {
	return &tagI18nR{}
}

// tagI18nL is where Load methods for each relationship are stored.
type tagI18nL struct{}

var (
	tagI18nAllColumns            = []string{"tag_id", "language", "original_language", "label", "user_id", "created_at"}
	tagI18nColumnsWithoutDefault = []string{"tag_id", "language"}
	tagI18nColumnsWithDefault    = []string{"original_language", "label", "user_id", "created_at"}
	tagI18nPrimaryKeyColumns     = []string{"tag_id", "language"}
	tagI18nGeneratedColumns      = []string{}
)

type (
	// TagI18nSlice is an alias for a slice of pointers to TagI18n.
	// This should almost always be used instead of []TagI18n.
	TagI18nSlice []*TagI18n

	tagI18nQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	tagI18nType                 = reflect.TypeOf(&TagI18n{})
	tagI18nMapping              = queries.MakeStructMapping(tagI18nType)
	tagI18nPrimaryKeyMapping, _ = queries.BindMapping(tagI18nType, tagI18nMapping, tagI18nPrimaryKeyColumns)
	tagI18nInsertCacheMut       sync.RWMutex
	tagI18nInsertCache          = make(map[string]insertCache)
	tagI18nUpdateCacheMut       sync.RWMutex
	tagI18nUpdateCache          = make(map[string]updateCache)
	tagI18nUpsertCacheMut       sync.RWMutex
	tagI18nUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force qmhelper dependency for where clause generation (which doesn't
	// always happen)
	_ = qmhelper.Where
)

// One returns a single tagI18n record from the query.
func (q tagI18nQuery) One(exec boil.Executor) (*TagI18n, error) {
	o := &TagI18n{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(nil, exec, o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: failed to execute a one query for tag_i18n")
	}

	return o, nil
}

// All returns all TagI18n records from the query.
func (q tagI18nQuery) All(exec boil.Executor) (TagI18nSlice, error) {
	var o []*TagI18n

	err := q.Bind(nil, exec, &o)
	if err != nil {
		return nil, errors.Wrap(err, "mdbmodels: failed to assign all query results to TagI18n slice")
	}

	return o, nil
}

// Count returns the count of all TagI18n records in the query.
func (q tagI18nQuery) Count(exec boil.Executor) (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow(exec).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to count tag_i18n rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table.
func (q tagI18nQuery) Exists(exec boil.Executor) (bool, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow(exec).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: failed to check if tag_i18n exists")
	}

	return count > 0, nil
}

// Tag pointed to by the foreign key.
func (o *TagI18n) Tag(mods ...qm.QueryMod) tagQuery {
	queryMods := []qm.QueryMod{
		qm.Where("\"id\" = ?", o.TagID),
	}

	queryMods = append(queryMods, mods...)

	query := Tags(queryMods...)
	queries.SetFrom(query.Query, "\"tags\"")

	return query
}

// User pointed to by the foreign key.
func (o *TagI18n) User(mods ...qm.QueryMod) userQuery {
	queryMods := []qm.QueryMod{
		qm.Where("\"id\" = ?", o.UserID),
	}

	queryMods = append(queryMods, mods...)

	query := Users(queryMods...)
	queries.SetFrom(query.Query, "\"users\"")

	return query
}

// LoadTag allows an eager lookup of values, cached into the
// loaded structs of the objects. This is for an N-1 relationship.
func (tagI18nL) LoadTag(e boil.Executor, singular bool, maybeTagI18n interface{}, mods queries.Applicator) error {
	var slice []*TagI18n
	var object *TagI18n

	if singular {
		object = maybeTagI18n.(*TagI18n)
	} else {
		slice = *maybeTagI18n.(*[]*TagI18n)
	}

	args := make([]interface{}, 0, 1)
	if singular {
		if object.R == nil {
			object.R = &tagI18nR{}
		}
		args = append(args, object.TagID)

	} else {
	Outer:
		for _, obj := range slice {
			if obj.R == nil {
				obj.R = &tagI18nR{}
			}

			for _, a := range args {
				if a == obj.TagID {
					continue Outer
				}
			}

			args = append(args, obj.TagID)

		}
	}

	if len(args) == 0 {
		return nil
	}

	query := NewQuery(
		qm.From(`tags`),
		qm.WhereIn(`tags.id in ?`, args...),
	)
	if mods != nil {
		mods.Apply(query)
	}

	results, err := query.Query(e)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Tag")
	}

	var resultSlice []*Tag
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Tag")
	}

	if err = results.Close(); err != nil {
		return errors.Wrap(err, "failed to close results of eager load for tags")
	}
	if err = results.Err(); err != nil {
		return errors.Wrap(err, "error occurred during iteration of eager loaded relations for tags")
	}

	if len(resultSlice) == 0 {
		return nil
	}

	if singular {
		foreign := resultSlice[0]
		object.R.Tag = foreign
		if foreign.R == nil {
			foreign.R = &tagR{}
		}
		foreign.R.TagI18ns = append(foreign.R.TagI18ns, object)
		return nil
	}

	for _, local := range slice {
		for _, foreign := range resultSlice {
			if local.TagID == foreign.ID {
				local.R.Tag = foreign
				if foreign.R == nil {
					foreign.R = &tagR{}
				}
				foreign.R.TagI18ns = append(foreign.R.TagI18ns, local)
				break
			}
		}
	}

	return nil
}

// LoadUser allows an eager lookup of values, cached into the
// loaded structs of the objects. This is for an N-1 relationship.
func (tagI18nL) LoadUser(e boil.Executor, singular bool, maybeTagI18n interface{}, mods queries.Applicator) error {
	var slice []*TagI18n
	var object *TagI18n

	if singular {
		object = maybeTagI18n.(*TagI18n)
	} else {
		slice = *maybeTagI18n.(*[]*TagI18n)
	}

	args := make([]interface{}, 0, 1)
	if singular {
		if object.R == nil {
			object.R = &tagI18nR{}
		}
		if !queries.IsNil(object.UserID) {
			args = append(args, object.UserID)
		}

	} else {
	Outer:
		for _, obj := range slice {
			if obj.R == nil {
				obj.R = &tagI18nR{}
			}

			for _, a := range args {
				if queries.Equal(a, obj.UserID) {
					continue Outer
				}
			}

			if !queries.IsNil(obj.UserID) {
				args = append(args, obj.UserID)
			}

		}
	}

	if len(args) == 0 {
		return nil
	}

	query := NewQuery(
		qm.From(`users`),
		qm.WhereIn(`users.id in ?`, args...),
	)
	if mods != nil {
		mods.Apply(query)
	}

	results, err := query.Query(e)
	if err != nil {
		return errors.Wrap(err, "failed to eager load User")
	}

	var resultSlice []*User
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice User")
	}

	if err = results.Close(); err != nil {
		return errors.Wrap(err, "failed to close results of eager load for users")
	}
	if err = results.Err(); err != nil {
		return errors.Wrap(err, "error occurred during iteration of eager loaded relations for users")
	}

	if len(resultSlice) == 0 {
		return nil
	}

	if singular {
		foreign := resultSlice[0]
		object.R.User = foreign
		if foreign.R == nil {
			foreign.R = &userR{}
		}
		foreign.R.TagI18ns = append(foreign.R.TagI18ns, object)
		return nil
	}

	for _, local := range slice {
		for _, foreign := range resultSlice {
			if queries.Equal(local.UserID, foreign.ID) {
				local.R.User = foreign
				if foreign.R == nil {
					foreign.R = &userR{}
				}
				foreign.R.TagI18ns = append(foreign.R.TagI18ns, local)
				break
			}
		}
	}

	return nil
}

// SetTag of the tagI18n to the related item.
// Sets o.R.Tag to related.
// Adds o to related.R.TagI18ns.
func (o *TagI18n) SetTag(exec boil.Executor, insert bool, related *Tag) error {
	var err error
	if insert {
		if err = related.Insert(exec, boil.Infer()); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"tag_i18n\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"tag_id"}),
		strmangle.WhereClause("\"", "\"", 2, tagI18nPrimaryKeyColumns),
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
		o.R = &tagI18nR{
			Tag: related,
		}
	} else {
		o.R.Tag = related
	}

	if related.R == nil {
		related.R = &tagR{
			TagI18ns: TagI18nSlice{o},
		}
	} else {
		related.R.TagI18ns = append(related.R.TagI18ns, o)
	}

	return nil
}

// SetUser of the tagI18n to the related item.
// Sets o.R.User to related.
// Adds o to related.R.TagI18ns.
func (o *TagI18n) SetUser(exec boil.Executor, insert bool, related *User) error {
	var err error
	if insert {
		if err = related.Insert(exec, boil.Infer()); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"tag_i18n\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
		strmangle.WhereClause("\"", "\"", 2, tagI18nPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.TagID, o.Language}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}
	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	queries.Assign(&o.UserID, related.ID)
	if o.R == nil {
		o.R = &tagI18nR{
			User: related,
		}
	} else {
		o.R.User = related
	}

	if related.R == nil {
		related.R = &userR{
			TagI18ns: TagI18nSlice{o},
		}
	} else {
		related.R.TagI18ns = append(related.R.TagI18ns, o)
	}

	return nil
}

// RemoveUser relationship.
// Sets o.R.User to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *TagI18n) RemoveUser(exec boil.Executor, related *User) error {
	var err error

	queries.SetScanner(&o.UserID, nil)
	if _, err = o.Update(exec, boil.Whitelist("user_id")); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	if o.R != nil {
		o.R.User = nil
	}
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.TagI18ns {
		if queries.Equal(o.UserID, ri.UserID) {
			continue
		}

		ln := len(related.R.TagI18ns)
		if ln > 1 && i < ln-1 {
			related.R.TagI18ns[i] = related.R.TagI18ns[ln-1]
		}
		related.R.TagI18ns = related.R.TagI18ns[:ln-1]
		break
	}
	return nil
}

// TagI18ns retrieves all the records using an executor.
func TagI18ns(mods ...qm.QueryMod) tagI18nQuery {
	mods = append(mods, qm.From("\"tag_i18n\""))
	return tagI18nQuery{NewQuery(mods...)}
}

// FindTagI18n retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindTagI18n(exec boil.Executor, tagID int64, language string, selectCols ...string) (*TagI18n, error) {
	tagI18nObj := &TagI18n{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"tag_i18n\" where \"tag_id\"=$1 AND \"language\"=$2", sel,
	)

	q := queries.Raw(query, tagID, language)

	err := q.Bind(nil, exec, tagI18nObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: unable to select from tag_i18n")
	}

	return tagI18nObj, nil
}

// Insert a single record using an executor.
// See boil.Columns.InsertColumnSet documentation to understand column list inference for inserts.
func (o *TagI18n) Insert(exec boil.Executor, columns boil.Columns) error {
	if o == nil {
		return errors.New("mdbmodels: no tag_i18n provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(tagI18nColumnsWithDefault, o)

	key := makeCacheKey(columns, nzDefaults)
	tagI18nInsertCacheMut.RLock()
	cache, cached := tagI18nInsertCache[key]
	tagI18nInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := columns.InsertColumnSet(
			tagI18nAllColumns,
			tagI18nColumnsWithDefault,
			tagI18nColumnsWithoutDefault,
			nzDefaults,
		)

		cache.valueMapping, err = queries.BindMapping(tagI18nType, tagI18nMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(tagI18nType, tagI18nMapping, returnColumns)
		if err != nil {
			return err
		}
		if len(wl) != 0 {
			cache.query = fmt.Sprintf("INSERT INTO \"tag_i18n\" (\"%s\") %%sVALUES (%s)%%s", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.UseIndexPlaceholders, len(wl), 1, 1))
		} else {
			cache.query = "INSERT INTO \"tag_i18n\" %sDEFAULT VALUES%s"
		}

		var queryOutput, queryReturning string

		if len(cache.retMapping) != 0 {
			queryReturning = fmt.Sprintf(" RETURNING \"%s\"", strings.Join(returnColumns, "\",\""))
		}

		cache.query = fmt.Sprintf(cache.query, queryOutput, queryReturning)
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
		return errors.Wrap(err, "mdbmodels: unable to insert into tag_i18n")
	}

	if !cached {
		tagI18nInsertCacheMut.Lock()
		tagI18nInsertCache[key] = cache
		tagI18nInsertCacheMut.Unlock()
	}

	return nil
}

// Update uses an executor to update the TagI18n.
// See boil.Columns.UpdateColumnSet documentation to understand column list inference for updates.
// Update does not automatically update the record in case of default values. Use .Reload() to refresh the records.
func (o *TagI18n) Update(exec boil.Executor, columns boil.Columns) (int64, error) {
	var err error
	key := makeCacheKey(columns, nil)
	tagI18nUpdateCacheMut.RLock()
	cache, cached := tagI18nUpdateCache[key]
	tagI18nUpdateCacheMut.RUnlock()

	if !cached {
		wl := columns.UpdateColumnSet(
			tagI18nAllColumns,
			tagI18nPrimaryKeyColumns,
		)
		if len(wl) == 0 {
			return 0, errors.New("mdbmodels: unable to update tag_i18n, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"tag_i18n\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, tagI18nPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(tagI18nType, tagI18nMapping, append(wl, tagI18nPrimaryKeyColumns...))
		if err != nil {
			return 0, err
		}
	}

	values := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, values)
	}
	var result sql.Result
	result, err = exec.Exec(cache.query, values...)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: unable to update tag_i18n row")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to get rows affected by update for tag_i18n")
	}

	if !cached {
		tagI18nUpdateCacheMut.Lock()
		tagI18nUpdateCache[key] = cache
		tagI18nUpdateCacheMut.Unlock()
	}

	return rowsAff, nil
}

// UpdateAll updates all rows with the specified column values.
func (q tagI18nQuery) UpdateAll(exec boil.Executor, cols M) (int64, error) {
	queries.SetUpdate(q.Query, cols)

	result, err := q.Query.Exec(exec)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: unable to update all for tag_i18n")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: unable to retrieve rows affected for tag_i18n")
	}

	return rowsAff, nil
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o TagI18nSlice) UpdateAll(exec boil.Executor, cols M) (int64, error) {
	ln := int64(len(o))
	if ln == 0 {
		return 0, nil
	}

	if len(cols) == 0 {
		return 0, errors.New("mdbmodels: update all requires at least one column argument")
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), tagI18nPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf("UPDATE \"tag_i18n\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), len(colNames)+1, tagI18nPrimaryKeyColumns, len(o)))

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}
	result, err := exec.Exec(sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: unable to update all in tagI18n slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: unable to retrieve rows affected all in update all tagI18n")
	}
	return rowsAff, nil
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
// See boil.Columns documentation for how to properly use updateColumns and insertColumns.
func (o *TagI18n) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	if o == nil {
		return errors.New("mdbmodels: no tag_i18n provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(tagI18nColumnsWithDefault, o)

	// Build cache key in-line uglily - mysql vs psql problems
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
	buf.WriteString(strconv.Itoa(updateColumns.Kind))
	for _, c := range updateColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(insertColumns.Kind))
	for _, c := range insertColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range nzDefaults {
		buf.WriteString(c)
	}
	key := buf.String()
	strmangle.PutBuffer(buf)

	tagI18nUpsertCacheMut.RLock()
	cache, cached := tagI18nUpsertCache[key]
	tagI18nUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		insert, ret := insertColumns.InsertColumnSet(
			tagI18nAllColumns,
			tagI18nColumnsWithDefault,
			tagI18nColumnsWithoutDefault,
			nzDefaults,
		)

		update := updateColumns.UpdateColumnSet(
			tagI18nAllColumns,
			tagI18nPrimaryKeyColumns,
		)

		if updateOnConflict && len(update) == 0 {
			return errors.New("mdbmodels: unable to upsert tag_i18n, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(tagI18nPrimaryKeyColumns))
			copy(conflict, tagI18nPrimaryKeyColumns)
		}
		cache.query = buildUpsertQueryPostgres(dialect, "\"tag_i18n\"", updateOnConflict, ret, update, conflict, insert)

		cache.valueMapping, err = queries.BindMapping(tagI18nType, tagI18nMapping, insert)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(tagI18nType, tagI18nMapping, ret)
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
		return errors.Wrap(err, "mdbmodels: unable to upsert tag_i18n")
	}

	if !cached {
		tagI18nUpsertCacheMut.Lock()
		tagI18nUpsertCache[key] = cache
		tagI18nUpsertCacheMut.Unlock()
	}

	return nil
}

// Delete deletes a single TagI18n record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *TagI18n) Delete(exec boil.Executor) (int64, error) {
	if o == nil {
		return 0, errors.New("mdbmodels: no TagI18n provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), tagI18nPrimaryKeyMapping)
	sql := "DELETE FROM \"tag_i18n\" WHERE \"tag_id\"=$1 AND \"language\"=$2"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}
	result, err := exec.Exec(sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: unable to delete from tag_i18n")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to get rows affected by delete for tag_i18n")
	}

	return rowsAff, nil
}

// DeleteAll deletes all matching rows.
func (q tagI18nQuery) DeleteAll(exec boil.Executor) (int64, error) {
	if q.Query == nil {
		return 0, errors.New("mdbmodels: no tagI18nQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	result, err := q.Query.Exec(exec)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: unable to delete all from tag_i18n")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to get rows affected by deleteall for tag_i18n")
	}

	return rowsAff, nil
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o TagI18nSlice) DeleteAll(exec boil.Executor) (int64, error) {
	if len(o) == 0 {
		return 0, nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), tagI18nPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "DELETE FROM \"tag_i18n\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, tagI18nPrimaryKeyColumns, len(o))

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}
	result, err := exec.Exec(sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: unable to delete all from tagI18n slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to get rows affected by deleteall for tag_i18n")
	}

	return rowsAff, nil
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *TagI18n) Reload(exec boil.Executor) error {
	ret, err := FindTagI18n(exec, o.TagID, o.Language)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *TagI18nSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	slice := TagI18nSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), tagI18nPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "SELECT \"tag_i18n\".* FROM \"tag_i18n\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, tagI18nPrimaryKeyColumns, len(*o))

	q := queries.Raw(sql, args...)

	err := q.Bind(nil, exec, &slice)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to reload all in TagI18nSlice")
	}

	*o = slice

	return nil
}

// TagI18nExists checks if the TagI18n row exists.
func TagI18nExists(exec boil.Executor, tagID int64, language string) (bool, error) {
	var exists bool
	sql := "select exists(select 1 from \"tag_i18n\" where \"tag_id\"=$1 AND \"language\"=$2 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, tagID, language)
	}
	row := exec.QueryRow(sql, tagID, language)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: unable to check if tag_i18n exists")
	}

	return exists, nil
}

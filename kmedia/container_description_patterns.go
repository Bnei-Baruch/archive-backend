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

// ContainerDescriptionPattern is an object representing the database table.
type ContainerDescriptionPattern struct {
	ID          int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	Pattern     null.String `boil:"pattern" json:"pattern,omitempty" toml:"pattern" yaml:"pattern,omitempty"`
	Description null.String `boil:"description" json:"description,omitempty" toml:"description" yaml:"description,omitempty"`
	LangID      null.String `boil:"lang_id" json:"lang_id,omitempty" toml:"lang_id" yaml:"lang_id,omitempty"`
	CreatedAt   null.Time   `boil:"created_at" json:"created_at,omitempty" toml:"created_at" yaml:"created_at,omitempty"`
	UpdatedAt   null.Time   `boil:"updated_at" json:"updated_at,omitempty" toml:"updated_at" yaml:"updated_at,omitempty"`
	UserID      null.Int    `boil:"user_id" json:"user_id,omitempty" toml:"user_id" yaml:"user_id,omitempty"`

	R *containerDescriptionPatternR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L containerDescriptionPatternL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// containerDescriptionPatternR is where relationships are stored.
type containerDescriptionPatternR struct {
	User     *User
	Lang     *Language
	Catalogs CatalogSlice
}

// containerDescriptionPatternL is where Load methods for each relationship are stored.
type containerDescriptionPatternL struct{}

var (
	containerDescriptionPatternColumns               = []string{"id", "pattern", "description", "lang_id", "created_at", "updated_at", "user_id"}
	containerDescriptionPatternColumnsWithoutDefault = []string{"pattern", "description", "lang_id", "created_at", "updated_at", "user_id"}
	containerDescriptionPatternColumnsWithDefault    = []string{"id"}
	containerDescriptionPatternPrimaryKeyColumns     = []string{"id"}
)

type (
	// ContainerDescriptionPatternSlice is an alias for a slice of pointers to ContainerDescriptionPattern.
	// This should generally be used opposed to []ContainerDescriptionPattern.
	ContainerDescriptionPatternSlice []*ContainerDescriptionPattern

	containerDescriptionPatternQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	containerDescriptionPatternType                 = reflect.TypeOf(&ContainerDescriptionPattern{})
	containerDescriptionPatternMapping              = queries.MakeStructMapping(containerDescriptionPatternType)
	containerDescriptionPatternPrimaryKeyMapping, _ = queries.BindMapping(containerDescriptionPatternType, containerDescriptionPatternMapping, containerDescriptionPatternPrimaryKeyColumns)
	containerDescriptionPatternInsertCacheMut       sync.RWMutex
	containerDescriptionPatternInsertCache          = make(map[string]insertCache)
	containerDescriptionPatternUpdateCacheMut       sync.RWMutex
	containerDescriptionPatternUpdateCache          = make(map[string]updateCache)
	containerDescriptionPatternUpsertCacheMut       sync.RWMutex
	containerDescriptionPatternUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single containerDescriptionPattern record from the query, and panics on error.
func (q containerDescriptionPatternQuery) OneP() *ContainerDescriptionPattern {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single containerDescriptionPattern record from the query.
func (q containerDescriptionPatternQuery) One() (*ContainerDescriptionPattern, error) {
	o := &ContainerDescriptionPattern{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for container_description_patterns")
	}

	return o, nil
}

// AllP returns all ContainerDescriptionPattern records from the query, and panics on error.
func (q containerDescriptionPatternQuery) AllP() ContainerDescriptionPatternSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all ContainerDescriptionPattern records from the query.
func (q containerDescriptionPatternQuery) All() (ContainerDescriptionPatternSlice, error) {
	var o ContainerDescriptionPatternSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to ContainerDescriptionPattern slice")
	}

	return o, nil
}

// CountP returns the count of all ContainerDescriptionPattern records in the query, and panics on error.
func (q containerDescriptionPatternQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all ContainerDescriptionPattern records in the query.
func (q containerDescriptionPatternQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count container_description_patterns rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q containerDescriptionPatternQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q containerDescriptionPatternQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if container_description_patterns exists")
	}

	return count > 0, nil
}

// UserG pointed to by the foreign key.
func (o *ContainerDescriptionPattern) UserG(mods ...qm.QueryMod) userQuery {
	return o.User(boil.GetDB(), mods...)
}

// User pointed to by the foreign key.
func (o *ContainerDescriptionPattern) User(exec boil.Executor, mods ...qm.QueryMod) userQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.UserID),
	}

	queryMods = append(queryMods, mods...)

	query := Users(exec, queryMods...)
	queries.SetFrom(query.Query, "\"users\"")

	return query
}

// LangG pointed to by the foreign key.
func (o *ContainerDescriptionPattern) LangG(mods ...qm.QueryMod) languageQuery {
	return o.Lang(boil.GetDB(), mods...)
}

// Lang pointed to by the foreign key.
func (o *ContainerDescriptionPattern) Lang(exec boil.Executor, mods ...qm.QueryMod) languageQuery {
	queryMods := []qm.QueryMod{
		qm.Where("code3=?", o.LangID),
	}

	queryMods = append(queryMods, mods...)

	query := Languages(exec, queryMods...)
	queries.SetFrom(query.Query, "\"languages\"")

	return query
}

// CatalogsG retrieves all the catalog's catalogs.
func (o *ContainerDescriptionPattern) CatalogsG(mods ...qm.QueryMod) catalogQuery {
	return o.Catalogs(boil.GetDB(), mods...)
}

// Catalogs retrieves all the catalog's catalogs with an executor.
func (o *ContainerDescriptionPattern) Catalogs(exec boil.Executor, mods ...qm.QueryMod) catalogQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.InnerJoin("\"catalogs_container_description_patterns\" as \"b\" on \"a\".\"id\" = \"b\".\"catalog_id\""),
		qm.Where("\"b\".\"container_description_pattern_id\"=?", o.ID),
	)

	query := Catalogs(exec, queryMods...)
	queries.SetFrom(query.Query, "\"catalogs\" as \"a\"")
	return query
}

// LoadUser allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerDescriptionPatternL) LoadUser(e boil.Executor, singular bool, maybeContainerDescriptionPattern interface{}) error {
	var slice []*ContainerDescriptionPattern
	var object *ContainerDescriptionPattern

	count := 1
	if singular {
		object = maybeContainerDescriptionPattern.(*ContainerDescriptionPattern)
	} else {
		slice = *maybeContainerDescriptionPattern.(*ContainerDescriptionPatternSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerDescriptionPatternR{}
		}
		args[0] = object.UserID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerDescriptionPatternR{}
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

// LoadLang allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerDescriptionPatternL) LoadLang(e boil.Executor, singular bool, maybeContainerDescriptionPattern interface{}) error {
	var slice []*ContainerDescriptionPattern
	var object *ContainerDescriptionPattern

	count := 1
	if singular {
		object = maybeContainerDescriptionPattern.(*ContainerDescriptionPattern)
	} else {
		slice = *maybeContainerDescriptionPattern.(*ContainerDescriptionPatternSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerDescriptionPatternR{}
		}
		args[0] = object.LangID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerDescriptionPatternR{}
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

// LoadCatalogs allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerDescriptionPatternL) LoadCatalogs(e boil.Executor, singular bool, maybeContainerDescriptionPattern interface{}) error {
	var slice []*ContainerDescriptionPattern
	var object *ContainerDescriptionPattern

	count := 1
	if singular {
		object = maybeContainerDescriptionPattern.(*ContainerDescriptionPattern)
	} else {
		slice = *maybeContainerDescriptionPattern.(*ContainerDescriptionPatternSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerDescriptionPatternR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerDescriptionPatternR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select \"a\".*, \"b\".\"container_description_pattern_id\" from \"catalogs\" as \"a\" inner join \"catalogs_container_description_patterns\" as \"b\" on \"a\".\"id\" = \"b\".\"catalog_id\" where \"b\".\"container_description_pattern_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load catalogs")
	}
	defer results.Close()

	var resultSlice []*Catalog

	var localJoinCols []int
	for results.Next() {
		one := new(Catalog)
		var localJoinCol int

		err = results.Scan(&one.ID, &one.Name, &one.ParentID, &one.CreatedAt, &one.UpdatedAt, &one.Catorder, &one.Secure, &one.Visible, &one.Open, &one.Label, &one.SelectedCatalog, &one.UserID, &one.BooksCatalog, &localJoinCol)
		if err = results.Err(); err != nil {
			return errors.Wrap(err, "failed to plebian-bind eager loaded slice catalogs")
		}

		resultSlice = append(resultSlice, one)
		localJoinCols = append(localJoinCols, localJoinCol)
	}

	if err = results.Err(); err != nil {
		return errors.Wrap(err, "failed to plebian-bind eager loaded slice catalogs")
	}

	if singular {
		object.R.Catalogs = resultSlice
		return nil
	}

	for i, foreign := range resultSlice {
		localJoinCol := localJoinCols[i]
		for _, local := range slice {
			if local.ID == localJoinCol {
				local.R.Catalogs = append(local.R.Catalogs, foreign)
				break
			}
		}
	}

	return nil
}

// SetUser of the container_description_pattern to the related item.
// Sets o.R.User to related.
// Adds o to related.R.ContainerDescriptionPatterns.
func (o *ContainerDescriptionPattern) SetUser(exec boil.Executor, insert bool, related *User) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"container_description_patterns\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
		strmangle.WhereClause("\"", "\"", 2, containerDescriptionPatternPrimaryKeyColumns),
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
		o.R = &containerDescriptionPatternR{
			User: related,
		}
	} else {
		o.R.User = related
	}

	if related.R == nil {
		related.R = &userR{
			ContainerDescriptionPatterns: ContainerDescriptionPatternSlice{o},
		}
	} else {
		related.R.ContainerDescriptionPatterns = append(related.R.ContainerDescriptionPatterns, o)
	}

	return nil
}

// RemoveUser relationship.
// Sets o.R.User to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *ContainerDescriptionPattern) RemoveUser(exec boil.Executor, related *User) error {
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

	for i, ri := range related.R.ContainerDescriptionPatterns {
		if o.UserID.Int != ri.UserID.Int {
			continue
		}

		ln := len(related.R.ContainerDescriptionPatterns)
		if ln > 1 && i < ln-1 {
			related.R.ContainerDescriptionPatterns[i] = related.R.ContainerDescriptionPatterns[ln-1]
		}
		related.R.ContainerDescriptionPatterns = related.R.ContainerDescriptionPatterns[:ln-1]
		break
	}
	return nil
}

// SetLang of the container_description_pattern to the related item.
// Sets o.R.Lang to related.
// Adds o to related.R.LangContainerDescriptionPatterns.
func (o *ContainerDescriptionPattern) SetLang(exec boil.Executor, insert bool, related *Language) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"container_description_patterns\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
		strmangle.WhereClause("\"", "\"", 2, containerDescriptionPatternPrimaryKeyColumns),
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
		o.R = &containerDescriptionPatternR{
			Lang: related,
		}
	} else {
		o.R.Lang = related
	}

	if related.R == nil {
		related.R = &languageR{
			LangContainerDescriptionPatterns: ContainerDescriptionPatternSlice{o},
		}
	} else {
		related.R.LangContainerDescriptionPatterns = append(related.R.LangContainerDescriptionPatterns, o)
	}

	return nil
}

// RemoveLang relationship.
// Sets o.R.Lang to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *ContainerDescriptionPattern) RemoveLang(exec boil.Executor, related *Language) error {
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

	for i, ri := range related.R.LangContainerDescriptionPatterns {
		if o.LangID.String != ri.LangID.String {
			continue
		}

		ln := len(related.R.LangContainerDescriptionPatterns)
		if ln > 1 && i < ln-1 {
			related.R.LangContainerDescriptionPatterns[i] = related.R.LangContainerDescriptionPatterns[ln-1]
		}
		related.R.LangContainerDescriptionPatterns = related.R.LangContainerDescriptionPatterns[:ln-1]
		break
	}
	return nil
}

// AddCatalogs adds the given related objects to the existing relationships
// of the container_description_pattern, optionally inserting them as new records.
// Appends related to o.R.Catalogs.
// Sets related.R.ContainerDescriptionPatterns appropriately.
func (o *ContainerDescriptionPattern) AddCatalogs(exec boil.Executor, insert bool, related ...*Catalog) error {
	var err error
	for _, rel := range related {
		if insert {
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		}
	}

	for _, rel := range related {
		query := "insert into \"catalogs_container_description_patterns\" (\"container_description_pattern_id\", \"catalog_id\") values ($1, $2)"
		values := []interface{}{o.ID, rel.ID}

		if boil.DebugMode {
			fmt.Fprintln(boil.DebugWriter, query)
			fmt.Fprintln(boil.DebugWriter, values)
		}

		_, err = exec.Exec(query, values...)
		if err != nil {
			return errors.Wrap(err, "failed to insert into join table")
		}
	}
	if o.R == nil {
		o.R = &containerDescriptionPatternR{
			Catalogs: related,
		}
	} else {
		o.R.Catalogs = append(o.R.Catalogs, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &catalogR{
				ContainerDescriptionPatterns: ContainerDescriptionPatternSlice{o},
			}
		} else {
			rel.R.ContainerDescriptionPatterns = append(rel.R.ContainerDescriptionPatterns, o)
		}
	}
	return nil
}

// SetCatalogs removes all previously related items of the
// container_description_pattern replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.ContainerDescriptionPatterns's Catalogs accordingly.
// Replaces o.R.Catalogs with related.
// Sets related.R.ContainerDescriptionPatterns's Catalogs accordingly.
func (o *ContainerDescriptionPattern) SetCatalogs(exec boil.Executor, insert bool, related ...*Catalog) error {
	query := "delete from \"catalogs_container_description_patterns\" where \"container_description_pattern_id\" = $1"
	values := []interface{}{o.ID}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	removeCatalogsFromContainerDescriptionPatternsSlice(o, related)
	o.R.Catalogs = nil
	return o.AddCatalogs(exec, insert, related...)
}

// RemoveCatalogs relationships from objects passed in.
// Removes related items from R.Catalogs (uses pointer comparison, removal does not keep order)
// Sets related.R.ContainerDescriptionPatterns.
func (o *ContainerDescriptionPattern) RemoveCatalogs(exec boil.Executor, related ...*Catalog) error {
	var err error
	query := fmt.Sprintf(
		"delete from \"catalogs_container_description_patterns\" where \"container_description_pattern_id\" = $1 and \"catalog_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, len(related), 1, 1),
	)
	values := []interface{}{o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err = exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}
	removeCatalogsFromContainerDescriptionPatternsSlice(o, related)
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.Catalogs {
			if rel != ri {
				continue
			}

			ln := len(o.R.Catalogs)
			if ln > 1 && i < ln-1 {
				o.R.Catalogs[i] = o.R.Catalogs[ln-1]
			}
			o.R.Catalogs = o.R.Catalogs[:ln-1]
			break
		}
	}

	return nil
}

func removeCatalogsFromContainerDescriptionPatternsSlice(o *ContainerDescriptionPattern, related []*Catalog) {
	for _, rel := range related {
		if rel.R == nil {
			continue
		}
		for i, ri := range rel.R.ContainerDescriptionPatterns {
			if o.ID != ri.ID {
				continue
			}

			ln := len(rel.R.ContainerDescriptionPatterns)
			if ln > 1 && i < ln-1 {
				rel.R.ContainerDescriptionPatterns[i] = rel.R.ContainerDescriptionPatterns[ln-1]
			}
			rel.R.ContainerDescriptionPatterns = rel.R.ContainerDescriptionPatterns[:ln-1]
			break
		}
	}
}

// ContainerDescriptionPatternsG retrieves all records.
func ContainerDescriptionPatternsG(mods ...qm.QueryMod) containerDescriptionPatternQuery {
	return ContainerDescriptionPatterns(boil.GetDB(), mods...)
}

// ContainerDescriptionPatterns retrieves all the records using an executor.
func ContainerDescriptionPatterns(exec boil.Executor, mods ...qm.QueryMod) containerDescriptionPatternQuery {
	mods = append(mods, qm.From("\"container_description_patterns\""))
	return containerDescriptionPatternQuery{NewQuery(exec, mods...)}
}

// FindContainerDescriptionPatternG retrieves a single record by ID.
func FindContainerDescriptionPatternG(id int, selectCols ...string) (*ContainerDescriptionPattern, error) {
	return FindContainerDescriptionPattern(boil.GetDB(), id, selectCols...)
}

// FindContainerDescriptionPatternGP retrieves a single record by ID, and panics on error.
func FindContainerDescriptionPatternGP(id int, selectCols ...string) *ContainerDescriptionPattern {
	retobj, err := FindContainerDescriptionPattern(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindContainerDescriptionPattern retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindContainerDescriptionPattern(exec boil.Executor, id int, selectCols ...string) (*ContainerDescriptionPattern, error) {
	containerDescriptionPatternObj := &ContainerDescriptionPattern{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"container_description_patterns\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(containerDescriptionPatternObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from container_description_patterns")
	}

	return containerDescriptionPatternObj, nil
}

// FindContainerDescriptionPatternP retrieves a single record by ID with an executor, and panics on error.
func FindContainerDescriptionPatternP(exec boil.Executor, id int, selectCols ...string) *ContainerDescriptionPattern {
	retobj, err := FindContainerDescriptionPattern(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *ContainerDescriptionPattern) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *ContainerDescriptionPattern) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *ContainerDescriptionPattern) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *ContainerDescriptionPattern) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no container_description_patterns provided for insertion")
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

	nzDefaults := queries.NonZeroDefaultSet(containerDescriptionPatternColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	containerDescriptionPatternInsertCacheMut.RLock()
	cache, cached := containerDescriptionPatternInsertCache[key]
	containerDescriptionPatternInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			containerDescriptionPatternColumns,
			containerDescriptionPatternColumnsWithDefault,
			containerDescriptionPatternColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(containerDescriptionPatternType, containerDescriptionPatternMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(containerDescriptionPatternType, containerDescriptionPatternMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"container_description_patterns\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into container_description_patterns")
	}

	if !cached {
		containerDescriptionPatternInsertCacheMut.Lock()
		containerDescriptionPatternInsertCache[key] = cache
		containerDescriptionPatternInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single ContainerDescriptionPattern record. See Update for
// whitelist behavior description.
func (o *ContainerDescriptionPattern) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single ContainerDescriptionPattern record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *ContainerDescriptionPattern) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the ContainerDescriptionPattern, and panics on error.
// See Update for whitelist behavior description.
func (o *ContainerDescriptionPattern) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the ContainerDescriptionPattern.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *ContainerDescriptionPattern) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	var err error
	key := makeCacheKey(whitelist, nil)
	containerDescriptionPatternUpdateCacheMut.RLock()
	cache, cached := containerDescriptionPatternUpdateCache[key]
	containerDescriptionPatternUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(containerDescriptionPatternColumns, containerDescriptionPatternPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update container_description_patterns, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"container_description_patterns\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, containerDescriptionPatternPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(containerDescriptionPatternType, containerDescriptionPatternMapping, append(wl, containerDescriptionPatternPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update container_description_patterns row")
	}

	if !cached {
		containerDescriptionPatternUpdateCacheMut.Lock()
		containerDescriptionPatternUpdateCache[key] = cache
		containerDescriptionPatternUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q containerDescriptionPatternQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q containerDescriptionPatternQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for container_description_patterns")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ContainerDescriptionPatternSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o ContainerDescriptionPatternSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o ContainerDescriptionPatternSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ContainerDescriptionPatternSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), containerDescriptionPatternPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"container_description_patterns\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(containerDescriptionPatternPrimaryKeyColumns), len(colNames)+1, len(containerDescriptionPatternPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in containerDescriptionPattern slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *ContainerDescriptionPattern) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *ContainerDescriptionPattern) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *ContainerDescriptionPattern) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *ContainerDescriptionPattern) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no container_description_patterns provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.Time.IsZero() {
		o.CreatedAt.Time = currTime
		o.CreatedAt.Valid = true
	}
	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	nzDefaults := queries.NonZeroDefaultSet(containerDescriptionPatternColumnsWithDefault, o)

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

	containerDescriptionPatternUpsertCacheMut.RLock()
	cache, cached := containerDescriptionPatternUpsertCache[key]
	containerDescriptionPatternUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			containerDescriptionPatternColumns,
			containerDescriptionPatternColumnsWithDefault,
			containerDescriptionPatternColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			containerDescriptionPatternColumns,
			containerDescriptionPatternPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert container_description_patterns, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(containerDescriptionPatternPrimaryKeyColumns))
			copy(conflict, containerDescriptionPatternPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"container_description_patterns\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(containerDescriptionPatternType, containerDescriptionPatternMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(containerDescriptionPatternType, containerDescriptionPatternMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert container_description_patterns")
	}

	if !cached {
		containerDescriptionPatternUpsertCacheMut.Lock()
		containerDescriptionPatternUpsertCache[key] = cache
		containerDescriptionPatternUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single ContainerDescriptionPattern record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContainerDescriptionPattern) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single ContainerDescriptionPattern record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *ContainerDescriptionPattern) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no ContainerDescriptionPattern provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single ContainerDescriptionPattern record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContainerDescriptionPattern) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single ContainerDescriptionPattern record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *ContainerDescriptionPattern) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no ContainerDescriptionPattern provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), containerDescriptionPatternPrimaryKeyMapping)
	sql := "DELETE FROM \"container_description_patterns\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from container_description_patterns")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q containerDescriptionPatternQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q containerDescriptionPatternQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no containerDescriptionPatternQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from container_description_patterns")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o ContainerDescriptionPatternSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o ContainerDescriptionPatternSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no ContainerDescriptionPattern slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o ContainerDescriptionPatternSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ContainerDescriptionPatternSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no ContainerDescriptionPattern slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), containerDescriptionPatternPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"container_description_patterns\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, containerDescriptionPatternPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(containerDescriptionPatternPrimaryKeyColumns), 1, len(containerDescriptionPatternPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from containerDescriptionPattern slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *ContainerDescriptionPattern) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *ContainerDescriptionPattern) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *ContainerDescriptionPattern) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no ContainerDescriptionPattern provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *ContainerDescriptionPattern) Reload(exec boil.Executor) error {
	ret, err := FindContainerDescriptionPattern(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContainerDescriptionPatternSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContainerDescriptionPatternSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContainerDescriptionPatternSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty ContainerDescriptionPatternSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContainerDescriptionPatternSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	containerDescriptionPatterns := ContainerDescriptionPatternSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), containerDescriptionPatternPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"container_description_patterns\".* FROM \"container_description_patterns\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, containerDescriptionPatternPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(containerDescriptionPatternPrimaryKeyColumns), 1, len(containerDescriptionPatternPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&containerDescriptionPatterns)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in ContainerDescriptionPatternSlice")
	}

	*o = containerDescriptionPatterns

	return nil
}

// ContainerDescriptionPatternExists checks if the ContainerDescriptionPattern row exists.
func ContainerDescriptionPatternExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"container_description_patterns\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if container_description_patterns exists")
	}

	return exists, nil
}

// ContainerDescriptionPatternExistsG checks if the ContainerDescriptionPattern row exists.
func ContainerDescriptionPatternExistsG(id int) (bool, error) {
	return ContainerDescriptionPatternExists(boil.GetDB(), id)
}

// ContainerDescriptionPatternExistsGP checks if the ContainerDescriptionPattern row exists. Panics on error.
func ContainerDescriptionPatternExistsGP(id int) bool {
	e, err := ContainerDescriptionPatternExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// ContainerDescriptionPatternExistsP checks if the ContainerDescriptionPattern row exists. Panics on error.
func ContainerDescriptionPatternExistsP(exec boil.Executor, id int) bool {
	e, err := ContainerDescriptionPatternExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

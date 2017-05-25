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

// Tag is an object representing the database table.
type Tag struct {
	ID          int64       `boil:"id" json:"id" toml:"id" yaml:"id"`
	Description null.String `boil:"description" json:"description,omitempty" toml:"description" yaml:"description,omitempty"`
	ParentID    null.Int64  `boil:"parent_id" json:"parent_id,omitempty" toml:"parent_id" yaml:"parent_id,omitempty"`
	UID         string      `boil:"uid" json:"uid" toml:"uid" yaml:"uid"`
	Pattern     null.String `boil:"pattern" json:"pattern,omitempty" toml:"pattern" yaml:"pattern,omitempty"`

	R *tagR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L tagL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// tagR is where relationships are stored.
type tagR struct {
	Parent       *Tag
	ContentUnits ContentUnitSlice
	TagI18ns     TagI18nSlice
	ParentTags   TagSlice
}

// tagL is where Load methods for each relationship are stored.
type tagL struct{}

var (
	tagColumns               = []string{"id", "description", "parent_id", "uid", "pattern"}
	tagColumnsWithoutDefault = []string{"description", "parent_id", "uid", "pattern"}
	tagColumnsWithDefault    = []string{"id"}
	tagPrimaryKeyColumns     = []string{"id"}
)

type (
	// TagSlice is an alias for a slice of pointers to Tag.
	// This should generally be used opposed to []Tag.
	TagSlice []*Tag

	tagQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	tagType                 = reflect.TypeOf(&Tag{})
	tagMapping              = queries.MakeStructMapping(tagType)
	tagPrimaryKeyMapping, _ = queries.BindMapping(tagType, tagMapping, tagPrimaryKeyColumns)
	tagInsertCacheMut       sync.RWMutex
	tagInsertCache          = make(map[string]insertCache)
	tagUpdateCacheMut       sync.RWMutex
	tagUpdateCache          = make(map[string]updateCache)
	tagUpsertCacheMut       sync.RWMutex
	tagUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single tag record from the query, and panics on error.
func (q tagQuery) OneP() *Tag {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single tag record from the query.
func (q tagQuery) One() (*Tag, error) {
	o := &Tag{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: failed to execute a one query for tags")
	}

	return o, nil
}

// AllP returns all Tag records from the query, and panics on error.
func (q tagQuery) AllP() TagSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all Tag records from the query.
func (q tagQuery) All() (TagSlice, error) {
	var o TagSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "mdbmodels: failed to assign all query results to Tag slice")
	}

	return o, nil
}

// CountP returns the count of all Tag records in the query, and panics on error.
func (q tagQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all Tag records in the query.
func (q tagQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to count tags rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q tagQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q tagQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: failed to check if tags exists")
	}

	return count > 0, nil
}

// ParentG pointed to by the foreign key.
func (o *Tag) ParentG(mods ...qm.QueryMod) tagQuery {
	return o.Parent(boil.GetDB(), mods...)
}

// Parent pointed to by the foreign key.
func (o *Tag) Parent(exec boil.Executor, mods ...qm.QueryMod) tagQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.ParentID),
	}

	queryMods = append(queryMods, mods...)

	query := Tags(exec, queryMods...)
	queries.SetFrom(query.Query, "\"tags\"")

	return query
}

// ContentUnitsG retrieves all the content_unit's content units.
func (o *Tag) ContentUnitsG(mods ...qm.QueryMod) contentUnitQuery {
	return o.ContentUnits(boil.GetDB(), mods...)
}

// ContentUnits retrieves all the content_unit's content units with an executor.
func (o *Tag) ContentUnits(exec boil.Executor, mods ...qm.QueryMod) contentUnitQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.InnerJoin("\"content_units_tags\" as \"b\" on \"a\".\"id\" = \"b\".\"content_unit_id\""),
		qm.Where("\"b\".\"tag_id\"=?", o.ID),
	)

	query := ContentUnits(exec, queryMods...)
	queries.SetFrom(query.Query, "\"content_units\" as \"a\"")
	return query
}

// TagI18nsG retrieves all the tag_i18n's tag i18n.
func (o *Tag) TagI18nsG(mods ...qm.QueryMod) tagI18nQuery {
	return o.TagI18ns(boil.GetDB(), mods...)
}

// TagI18ns retrieves all the tag_i18n's tag i18n with an executor.
func (o *Tag) TagI18ns(exec boil.Executor, mods ...qm.QueryMod) tagI18nQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"tag_id\"=?", o.ID),
	)

	query := TagI18ns(exec, queryMods...)
	queries.SetFrom(query.Query, "\"tag_i18n\" as \"a\"")
	return query
}

// ParentTagsG retrieves all the tag's tags via parent_id column.
func (o *Tag) ParentTagsG(mods ...qm.QueryMod) tagQuery {
	return o.ParentTags(boil.GetDB(), mods...)
}

// ParentTags retrieves all the tag's tags with an executor via parent_id column.
func (o *Tag) ParentTags(exec boil.Executor, mods ...qm.QueryMod) tagQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"parent_id\"=?", o.ID),
	)

	query := Tags(exec, queryMods...)
	queries.SetFrom(query.Query, "\"tags\" as \"a\"")
	return query
}

// LoadParent allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (tagL) LoadParent(e boil.Executor, singular bool, maybeTag interface{}) error {
	var slice []*Tag
	var object *Tag

	count := 1
	if singular {
		object = maybeTag.(*Tag)
	} else {
		slice = *maybeTag.(*TagSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &tagR{}
		}
		args[0] = object.ParentID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &tagR{}
			}
			args[i] = obj.ParentID
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
		object.R.Parent = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ParentID.Int64 == foreign.ID {
				local.R.Parent = foreign
				break
			}
		}
	}

	return nil
}

// LoadContentUnits allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (tagL) LoadContentUnits(e boil.Executor, singular bool, maybeTag interface{}) error {
	var slice []*Tag
	var object *Tag

	count := 1
	if singular {
		object = maybeTag.(*Tag)
	} else {
		slice = *maybeTag.(*TagSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &tagR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &tagR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select \"a\".*, \"b\".\"tag_id\" from \"content_units\" as \"a\" inner join \"content_units_tags\" as \"b\" on \"a\".\"id\" = \"b\".\"content_unit_id\" where \"b\".\"tag_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load content_units")
	}
	defer results.Close()

	var resultSlice []*ContentUnit

	var localJoinCols []int64
	for results.Next() {
		one := new(ContentUnit)
		var localJoinCol int64

		err = results.Scan(&one.ID, &one.UID, &one.TypeID, &one.CreatedAt, &one.Properties, &one.Secure, &one.Published, &localJoinCol)
		if err = results.Err(); err != nil {
			return errors.Wrap(err, "failed to plebian-bind eager loaded slice content_units")
		}

		resultSlice = append(resultSlice, one)
		localJoinCols = append(localJoinCols, localJoinCol)
	}

	if err = results.Err(); err != nil {
		return errors.Wrap(err, "failed to plebian-bind eager loaded slice content_units")
	}

	if singular {
		object.R.ContentUnits = resultSlice
		return nil
	}

	for i, foreign := range resultSlice {
		localJoinCol := localJoinCols[i]
		for _, local := range slice {
			if local.ID == localJoinCol {
				local.R.ContentUnits = append(local.R.ContentUnits, foreign)
				break
			}
		}
	}

	return nil
}

// LoadTagI18ns allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (tagL) LoadTagI18ns(e boil.Executor, singular bool, maybeTag interface{}) error {
	var slice []*Tag
	var object *Tag

	count := 1
	if singular {
		object = maybeTag.(*Tag)
	} else {
		slice = *maybeTag.(*TagSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &tagR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &tagR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"tag_i18n\" where \"tag_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load tag_i18n")
	}
	defer results.Close()

	var resultSlice []*TagI18n
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice tag_i18n")
	}

	if singular {
		object.R.TagI18ns = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.TagID {
				local.R.TagI18ns = append(local.R.TagI18ns, foreign)
				break
			}
		}
	}

	return nil
}

// LoadParentTags allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (tagL) LoadParentTags(e boil.Executor, singular bool, maybeTag interface{}) error {
	var slice []*Tag
	var object *Tag

	count := 1
	if singular {
		object = maybeTag.(*Tag)
	} else {
		slice = *maybeTag.(*TagSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &tagR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &tagR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"tags\" where \"parent_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load tags")
	}
	defer results.Close()

	var resultSlice []*Tag
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice tags")
	}

	if singular {
		object.R.ParentTags = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.ParentID.Int64 {
				local.R.ParentTags = append(local.R.ParentTags, foreign)
				break
			}
		}
	}

	return nil
}

// SetParentG of the tag to the related item.
// Sets o.R.Parent to related.
// Adds o to related.R.ParentTags.
// Uses the global database handle.
func (o *Tag) SetParentG(insert bool, related *Tag) error {
	return o.SetParent(boil.GetDB(), insert, related)
}

// SetParentP of the tag to the related item.
// Sets o.R.Parent to related.
// Adds o to related.R.ParentTags.
// Panics on error.
func (o *Tag) SetParentP(exec boil.Executor, insert bool, related *Tag) {
	if err := o.SetParent(exec, insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetParentGP of the tag to the related item.
// Sets o.R.Parent to related.
// Adds o to related.R.ParentTags.
// Uses the global database handle and panics on error.
func (o *Tag) SetParentGP(insert bool, related *Tag) {
	if err := o.SetParent(boil.GetDB(), insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetParent of the tag to the related item.
// Sets o.R.Parent to related.
// Adds o to related.R.ParentTags.
func (o *Tag) SetParent(exec boil.Executor, insert bool, related *Tag) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"tags\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"parent_id"}),
		strmangle.WhereClause("\"", "\"", 2, tagPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.ParentID.Int64 = related.ID
	o.ParentID.Valid = true

	if o.R == nil {
		o.R = &tagR{
			Parent: related,
		}
	} else {
		o.R.Parent = related
	}

	if related.R == nil {
		related.R = &tagR{
			ParentTags: TagSlice{o},
		}
	} else {
		related.R.ParentTags = append(related.R.ParentTags, o)
	}

	return nil
}

// RemoveParentG relationship.
// Sets o.R.Parent to nil.
// Removes o from all passed in related items' relationships struct (Optional).
// Uses the global database handle.
func (o *Tag) RemoveParentG(related *Tag) error {
	return o.RemoveParent(boil.GetDB(), related)
}

// RemoveParentP relationship.
// Sets o.R.Parent to nil.
// Removes o from all passed in related items' relationships struct (Optional).
// Panics on error.
func (o *Tag) RemoveParentP(exec boil.Executor, related *Tag) {
	if err := o.RemoveParent(exec, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// RemoveParentGP relationship.
// Sets o.R.Parent to nil.
// Removes o from all passed in related items' relationships struct (Optional).
// Uses the global database handle and panics on error.
func (o *Tag) RemoveParentGP(related *Tag) {
	if err := o.RemoveParent(boil.GetDB(), related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// RemoveParent relationship.
// Sets o.R.Parent to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *Tag) RemoveParent(exec boil.Executor, related *Tag) error {
	var err error

	o.ParentID.Valid = false
	if err = o.Update(exec, "parent_id"); err != nil {
		o.ParentID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.Parent = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.ParentTags {
		if o.ParentID.Int64 != ri.ParentID.Int64 {
			continue
		}

		ln := len(related.R.ParentTags)
		if ln > 1 && i < ln-1 {
			related.R.ParentTags[i] = related.R.ParentTags[ln-1]
		}
		related.R.ParentTags = related.R.ParentTags[:ln-1]
		break
	}
	return nil
}

// AddContentUnitsG adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.ContentUnits.
// Sets related.R.Tags appropriately.
// Uses the global database handle.
func (o *Tag) AddContentUnitsG(insert bool, related ...*ContentUnit) error {
	return o.AddContentUnits(boil.GetDB(), insert, related...)
}

// AddContentUnitsP adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.ContentUnits.
// Sets related.R.Tags appropriately.
// Panics on error.
func (o *Tag) AddContentUnitsP(exec boil.Executor, insert bool, related ...*ContentUnit) {
	if err := o.AddContentUnits(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddContentUnitsGP adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.ContentUnits.
// Sets related.R.Tags appropriately.
// Uses the global database handle and panics on error.
func (o *Tag) AddContentUnitsGP(insert bool, related ...*ContentUnit) {
	if err := o.AddContentUnits(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddContentUnits adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.ContentUnits.
// Sets related.R.Tags appropriately.
func (o *Tag) AddContentUnits(exec boil.Executor, insert bool, related ...*ContentUnit) error {
	var err error
	for _, rel := range related {
		if insert {
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		}
	}

	for _, rel := range related {
		query := "insert into \"content_units_tags\" (\"tag_id\", \"content_unit_id\") values ($1, $2)"
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
		o.R = &tagR{
			ContentUnits: related,
		}
	} else {
		o.R.ContentUnits = append(o.R.ContentUnits, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &contentUnitR{
				Tags: TagSlice{o},
			}
		} else {
			rel.R.Tags = append(rel.R.Tags, o)
		}
	}
	return nil
}

// SetContentUnitsG removes all previously related items of the
// tag replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Tags's ContentUnits accordingly.
// Replaces o.R.ContentUnits with related.
// Sets related.R.Tags's ContentUnits accordingly.
// Uses the global database handle.
func (o *Tag) SetContentUnitsG(insert bool, related ...*ContentUnit) error {
	return o.SetContentUnits(boil.GetDB(), insert, related...)
}

// SetContentUnitsP removes all previously related items of the
// tag replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Tags's ContentUnits accordingly.
// Replaces o.R.ContentUnits with related.
// Sets related.R.Tags's ContentUnits accordingly.
// Panics on error.
func (o *Tag) SetContentUnitsP(exec boil.Executor, insert bool, related ...*ContentUnit) {
	if err := o.SetContentUnits(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetContentUnitsGP removes all previously related items of the
// tag replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Tags's ContentUnits accordingly.
// Replaces o.R.ContentUnits with related.
// Sets related.R.Tags's ContentUnits accordingly.
// Uses the global database handle and panics on error.
func (o *Tag) SetContentUnitsGP(insert bool, related ...*ContentUnit) {
	if err := o.SetContentUnits(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetContentUnits removes all previously related items of the
// tag replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Tags's ContentUnits accordingly.
// Replaces o.R.ContentUnits with related.
// Sets related.R.Tags's ContentUnits accordingly.
func (o *Tag) SetContentUnits(exec boil.Executor, insert bool, related ...*ContentUnit) error {
	query := "delete from \"content_units_tags\" where \"tag_id\" = $1"
	values := []interface{}{o.ID}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	removeContentUnitsFromTagsSlice(o, related)
	o.R.ContentUnits = nil
	return o.AddContentUnits(exec, insert, related...)
}

// RemoveContentUnitsG relationships from objects passed in.
// Removes related items from R.ContentUnits (uses pointer comparison, removal does not keep order)
// Sets related.R.Tags.
// Uses the global database handle.
func (o *Tag) RemoveContentUnitsG(related ...*ContentUnit) error {
	return o.RemoveContentUnits(boil.GetDB(), related...)
}

// RemoveContentUnitsP relationships from objects passed in.
// Removes related items from R.ContentUnits (uses pointer comparison, removal does not keep order)
// Sets related.R.Tags.
// Panics on error.
func (o *Tag) RemoveContentUnitsP(exec boil.Executor, related ...*ContentUnit) {
	if err := o.RemoveContentUnits(exec, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// RemoveContentUnitsGP relationships from objects passed in.
// Removes related items from R.ContentUnits (uses pointer comparison, removal does not keep order)
// Sets related.R.Tags.
// Uses the global database handle and panics on error.
func (o *Tag) RemoveContentUnitsGP(related ...*ContentUnit) {
	if err := o.RemoveContentUnits(boil.GetDB(), related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// RemoveContentUnits relationships from objects passed in.
// Removes related items from R.ContentUnits (uses pointer comparison, removal does not keep order)
// Sets related.R.Tags.
func (o *Tag) RemoveContentUnits(exec boil.Executor, related ...*ContentUnit) error {
	var err error
	query := fmt.Sprintf(
		"delete from \"content_units_tags\" where \"tag_id\" = $1 and \"content_unit_id\" in (%s)",
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
	removeContentUnitsFromTagsSlice(o, related)
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.ContentUnits {
			if rel != ri {
				continue
			}

			ln := len(o.R.ContentUnits)
			if ln > 1 && i < ln-1 {
				o.R.ContentUnits[i] = o.R.ContentUnits[ln-1]
			}
			o.R.ContentUnits = o.R.ContentUnits[:ln-1]
			break
		}
	}

	return nil
}

func removeContentUnitsFromTagsSlice(o *Tag, related []*ContentUnit) {
	for _, rel := range related {
		if rel.R == nil {
			continue
		}
		for i, ri := range rel.R.Tags {
			if o.ID != ri.ID {
				continue
			}

			ln := len(rel.R.Tags)
			if ln > 1 && i < ln-1 {
				rel.R.Tags[i] = rel.R.Tags[ln-1]
			}
			rel.R.Tags = rel.R.Tags[:ln-1]
			break
		}
	}
}

// AddTagI18nsG adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.TagI18ns.
// Sets related.R.Tag appropriately.
// Uses the global database handle.
func (o *Tag) AddTagI18nsG(insert bool, related ...*TagI18n) error {
	return o.AddTagI18ns(boil.GetDB(), insert, related...)
}

// AddTagI18nsP adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.TagI18ns.
// Sets related.R.Tag appropriately.
// Panics on error.
func (o *Tag) AddTagI18nsP(exec boil.Executor, insert bool, related ...*TagI18n) {
	if err := o.AddTagI18ns(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddTagI18nsGP adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.TagI18ns.
// Sets related.R.Tag appropriately.
// Uses the global database handle and panics on error.
func (o *Tag) AddTagI18nsGP(insert bool, related ...*TagI18n) {
	if err := o.AddTagI18ns(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddTagI18ns adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.TagI18ns.
// Sets related.R.Tag appropriately.
func (o *Tag) AddTagI18ns(exec boil.Executor, insert bool, related ...*TagI18n) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.TagID = o.ID
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"tag_i18n\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"tag_id"}),
				strmangle.WhereClause("\"", "\"", 2, tagI18nPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.TagID, rel.Language}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.TagID = o.ID
		}
	}

	if o.R == nil {
		o.R = &tagR{
			TagI18ns: related,
		}
	} else {
		o.R.TagI18ns = append(o.R.TagI18ns, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &tagI18nR{
				Tag: o,
			}
		} else {
			rel.R.Tag = o
		}
	}
	return nil
}

// AddParentTagsG adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.ParentTags.
// Sets related.R.Parent appropriately.
// Uses the global database handle.
func (o *Tag) AddParentTagsG(insert bool, related ...*Tag) error {
	return o.AddParentTags(boil.GetDB(), insert, related...)
}

// AddParentTagsP adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.ParentTags.
// Sets related.R.Parent appropriately.
// Panics on error.
func (o *Tag) AddParentTagsP(exec boil.Executor, insert bool, related ...*Tag) {
	if err := o.AddParentTags(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddParentTagsGP adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.ParentTags.
// Sets related.R.Parent appropriately.
// Uses the global database handle and panics on error.
func (o *Tag) AddParentTagsGP(insert bool, related ...*Tag) {
	if err := o.AddParentTags(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddParentTags adds the given related objects to the existing relationships
// of the tag, optionally inserting them as new records.
// Appends related to o.R.ParentTags.
// Sets related.R.Parent appropriately.
func (o *Tag) AddParentTags(exec boil.Executor, insert bool, related ...*Tag) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.ParentID.Int64 = o.ID
			rel.ParentID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"tags\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"parent_id"}),
				strmangle.WhereClause("\"", "\"", 2, tagPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.ParentID.Int64 = o.ID
			rel.ParentID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &tagR{
			ParentTags: related,
		}
	} else {
		o.R.ParentTags = append(o.R.ParentTags, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &tagR{
				Parent: o,
			}
		} else {
			rel.R.Parent = o
		}
	}
	return nil
}

// SetParentTagsG removes all previously related items of the
// tag replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Parent's ParentTags accordingly.
// Replaces o.R.ParentTags with related.
// Sets related.R.Parent's ParentTags accordingly.
// Uses the global database handle.
func (o *Tag) SetParentTagsG(insert bool, related ...*Tag) error {
	return o.SetParentTags(boil.GetDB(), insert, related...)
}

// SetParentTagsP removes all previously related items of the
// tag replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Parent's ParentTags accordingly.
// Replaces o.R.ParentTags with related.
// Sets related.R.Parent's ParentTags accordingly.
// Panics on error.
func (o *Tag) SetParentTagsP(exec boil.Executor, insert bool, related ...*Tag) {
	if err := o.SetParentTags(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetParentTagsGP removes all previously related items of the
// tag replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Parent's ParentTags accordingly.
// Replaces o.R.ParentTags with related.
// Sets related.R.Parent's ParentTags accordingly.
// Uses the global database handle and panics on error.
func (o *Tag) SetParentTagsGP(insert bool, related ...*Tag) {
	if err := o.SetParentTags(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetParentTags removes all previously related items of the
// tag replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Parent's ParentTags accordingly.
// Replaces o.R.ParentTags with related.
// Sets related.R.Parent's ParentTags accordingly.
func (o *Tag) SetParentTags(exec boil.Executor, insert bool, related ...*Tag) error {
	query := "update \"tags\" set \"parent_id\" = null where \"parent_id\" = $1"
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
		for _, rel := range o.R.ParentTags {
			rel.ParentID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Parent = nil
		}

		o.R.ParentTags = nil
	}
	return o.AddParentTags(exec, insert, related...)
}

// RemoveParentTagsG relationships from objects passed in.
// Removes related items from R.ParentTags (uses pointer comparison, removal does not keep order)
// Sets related.R.Parent.
// Uses the global database handle.
func (o *Tag) RemoveParentTagsG(related ...*Tag) error {
	return o.RemoveParentTags(boil.GetDB(), related...)
}

// RemoveParentTagsP relationships from objects passed in.
// Removes related items from R.ParentTags (uses pointer comparison, removal does not keep order)
// Sets related.R.Parent.
// Panics on error.
func (o *Tag) RemoveParentTagsP(exec boil.Executor, related ...*Tag) {
	if err := o.RemoveParentTags(exec, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// RemoveParentTagsGP relationships from objects passed in.
// Removes related items from R.ParentTags (uses pointer comparison, removal does not keep order)
// Sets related.R.Parent.
// Uses the global database handle and panics on error.
func (o *Tag) RemoveParentTagsGP(related ...*Tag) {
	if err := o.RemoveParentTags(boil.GetDB(), related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// RemoveParentTags relationships from objects passed in.
// Removes related items from R.ParentTags (uses pointer comparison, removal does not keep order)
// Sets related.R.Parent.
func (o *Tag) RemoveParentTags(exec boil.Executor, related ...*Tag) error {
	var err error
	for _, rel := range related {
		rel.ParentID.Valid = false
		if rel.R != nil {
			rel.R.Parent = nil
		}
		if err = rel.Update(exec, "parent_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.ParentTags {
			if rel != ri {
				continue
			}

			ln := len(o.R.ParentTags)
			if ln > 1 && i < ln-1 {
				o.R.ParentTags[i] = o.R.ParentTags[ln-1]
			}
			o.R.ParentTags = o.R.ParentTags[:ln-1]
			break
		}
	}

	return nil
}

// TagsG retrieves all records.
func TagsG(mods ...qm.QueryMod) tagQuery {
	return Tags(boil.GetDB(), mods...)
}

// Tags retrieves all the records using an executor.
func Tags(exec boil.Executor, mods ...qm.QueryMod) tagQuery {
	mods = append(mods, qm.From("\"tags\""))
	return tagQuery{NewQuery(exec, mods...)}
}

// FindTagG retrieves a single record by ID.
func FindTagG(id int64, selectCols ...string) (*Tag, error) {
	return FindTag(boil.GetDB(), id, selectCols...)
}

// FindTagGP retrieves a single record by ID, and panics on error.
func FindTagGP(id int64, selectCols ...string) *Tag {
	retobj, err := FindTag(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindTag retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindTag(exec boil.Executor, id int64, selectCols ...string) (*Tag, error) {
	tagObj := &Tag{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"tags\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(tagObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: unable to select from tags")
	}

	return tagObj, nil
}

// FindTagP retrieves a single record by ID with an executor, and panics on error.
func FindTagP(exec boil.Executor, id int64, selectCols ...string) *Tag {
	retobj, err := FindTag(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *Tag) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *Tag) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *Tag) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *Tag) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no tags provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(tagColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	tagInsertCacheMut.RLock()
	cache, cached := tagInsertCache[key]
	tagInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			tagColumns,
			tagColumnsWithDefault,
			tagColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(tagType, tagMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(tagType, tagMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"tags\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "mdbmodels: unable to insert into tags")
	}

	if !cached {
		tagInsertCacheMut.Lock()
		tagInsertCache[key] = cache
		tagInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single Tag record. See Update for
// whitelist behavior description.
func (o *Tag) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single Tag record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *Tag) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the Tag, and panics on error.
// See Update for whitelist behavior description.
func (o *Tag) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the Tag.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *Tag) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	tagUpdateCacheMut.RLock()
	cache, cached := tagUpdateCache[key]
	tagUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(tagColumns, tagPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("mdbmodels: unable to update tags, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"tags\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, tagPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(tagType, tagMapping, append(wl, tagPrimaryKeyColumns...))
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
		return errors.Wrap(err, "mdbmodels: unable to update tags row")
	}

	if !cached {
		tagUpdateCacheMut.Lock()
		tagUpdateCache[key] = cache
		tagUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q tagQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q tagQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all for tags")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o TagSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o TagSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o TagSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o TagSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), tagPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"tags\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(tagPrimaryKeyColumns), len(colNames)+1, len(tagPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all in tag slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *Tag) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *Tag) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *Tag) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *Tag) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no tags provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(tagColumnsWithDefault, o)

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

	tagUpsertCacheMut.RLock()
	cache, cached := tagUpsertCache[key]
	tagUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			tagColumns,
			tagColumnsWithDefault,
			tagColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			tagColumns,
			tagPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("mdbmodels: unable to upsert tags, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(tagPrimaryKeyColumns))
			copy(conflict, tagPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"tags\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(tagType, tagMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(tagType, tagMapping, ret)
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
		return errors.Wrap(err, "mdbmodels: unable to upsert tags")
	}

	if !cached {
		tagUpsertCacheMut.Lock()
		tagUpsertCache[key] = cache
		tagUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single Tag record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Tag) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single Tag record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *Tag) DeleteG() error {
	if o == nil {
		return errors.New("mdbmodels: no Tag provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single Tag record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Tag) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single Tag record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Tag) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no Tag provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), tagPrimaryKeyMapping)
	sql := "DELETE FROM \"tags\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete from tags")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q tagQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q tagQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("mdbmodels: no tagQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from tags")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o TagSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o TagSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("mdbmodels: no Tag slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o TagSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o TagSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no Tag slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), tagPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"tags\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, tagPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(tagPrimaryKeyColumns), 1, len(tagPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from tag slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *Tag) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *Tag) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *Tag) ReloadG() error {
	if o == nil {
		return errors.New("mdbmodels: no Tag provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Tag) Reload(exec boil.Executor) error {
	ret, err := FindTag(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *TagSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *TagSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *TagSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("mdbmodels: empty TagSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *TagSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	tags := TagSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), tagPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"tags\".* FROM \"tags\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, tagPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(tagPrimaryKeyColumns), 1, len(tagPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&tags)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to reload all in TagSlice")
	}

	*o = tags

	return nil
}

// TagExists checks if the Tag row exists.
func TagExists(exec boil.Executor, id int64) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"tags\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: unable to check if tags exists")
	}

	return exists, nil
}

// TagExistsG checks if the Tag row exists.
func TagExistsG(id int64) (bool, error) {
	return TagExists(boil.GetDB(), id)
}

// TagExistsGP checks if the Tag row exists. Panics on error.
func TagExistsGP(id int64) bool {
	e, err := TagExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// TagExistsP checks if the Tag row exists. Panics on error.
func TagExistsP(exec boil.Executor, id int64) bool {
	e, err := TagExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

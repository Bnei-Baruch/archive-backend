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

// ContentUnit is an object representing the database table.
type ContentUnit struct {
	ID         int64     `boil:"id" json:"id" toml:"id" yaml:"id"`
	UID        string    `boil:"uid" json:"uid" toml:"uid" yaml:"uid"`
	TypeID     int64     `boil:"type_id" json:"type_id" toml:"type_id" yaml:"type_id"`
	CreatedAt  time.Time `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	Properties null.JSON `boil:"properties" json:"properties,omitempty" toml:"properties" yaml:"properties,omitempty"`

	R *contentUnitR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L contentUnitL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// contentUnitR is where relationships are stored.
type contentUnitR struct {
	Type                    *ContentType
	Files                   FileSlice
	CollectionsContentUnits CollectionsContentUnitSlice
	ContentUnitI18ns        ContentUnitI18nSlice
}

// contentUnitL is where Load methods for each relationship are stored.
type contentUnitL struct{}

var (
	contentUnitColumns               = []string{"id", "uid", "type_id", "created_at", "properties"}
	contentUnitColumnsWithoutDefault = []string{"uid", "type_id", "properties"}
	contentUnitColumnsWithDefault    = []string{"id", "created_at"}
	contentUnitPrimaryKeyColumns     = []string{"id"}
)

type (
	// ContentUnitSlice is an alias for a slice of pointers to ContentUnit.
	// This should generally be used opposed to []ContentUnit.
	ContentUnitSlice []*ContentUnit

	contentUnitQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	contentUnitType                 = reflect.TypeOf(&ContentUnit{})
	contentUnitMapping              = queries.MakeStructMapping(contentUnitType)
	contentUnitPrimaryKeyMapping, _ = queries.BindMapping(contentUnitType, contentUnitMapping, contentUnitPrimaryKeyColumns)
	contentUnitInsertCacheMut       sync.RWMutex
	contentUnitInsertCache          = make(map[string]insertCache)
	contentUnitUpdateCacheMut       sync.RWMutex
	contentUnitUpdateCache          = make(map[string]updateCache)
	contentUnitUpsertCacheMut       sync.RWMutex
	contentUnitUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single contentUnit record from the query, and panics on error.
func (q contentUnitQuery) OneP() *ContentUnit {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single contentUnit record from the query.
func (q contentUnitQuery) One() (*ContentUnit, error) {
	o := &ContentUnit{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: failed to execute a one query for content_units")
	}

	return o, nil
}

// AllP returns all ContentUnit records from the query, and panics on error.
func (q contentUnitQuery) AllP() ContentUnitSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all ContentUnit records from the query.
func (q contentUnitQuery) All() (ContentUnitSlice, error) {
	var o ContentUnitSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "mdbmodels: failed to assign all query results to ContentUnit slice")
	}

	return o, nil
}

// CountP returns the count of all ContentUnit records in the query, and panics on error.
func (q contentUnitQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all ContentUnit records in the query.
func (q contentUnitQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "mdbmodels: failed to count content_units rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q contentUnitQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q contentUnitQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: failed to check if content_units exists")
	}

	return count > 0, nil
}

// TypeG pointed to by the foreign key.
func (o *ContentUnit) TypeG(mods ...qm.QueryMod) contentTypeQuery {
	return o.Type(boil.GetDB(), mods...)
}

// Type pointed to by the foreign key.
func (o *ContentUnit) Type(exec boil.Executor, mods ...qm.QueryMod) contentTypeQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.TypeID),
	}

	queryMods = append(queryMods, mods...)

	query := ContentTypes(exec, queryMods...)
	queries.SetFrom(query.Query, "\"content_types\"")

	return query
}

// FilesG retrieves all the file's files.
func (o *ContentUnit) FilesG(mods ...qm.QueryMod) fileQuery {
	return o.Files(boil.GetDB(), mods...)
}

// Files retrieves all the file's files with an executor.
func (o *ContentUnit) Files(exec boil.Executor, mods ...qm.QueryMod) fileQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"content_unit_id\"=?", o.ID),
	)

	query := Files(exec, queryMods...)
	queries.SetFrom(query.Query, "\"files\" as \"a\"")
	return query
}

// CollectionsContentUnitsG retrieves all the collections_content_unit's collections content units.
func (o *ContentUnit) CollectionsContentUnitsG(mods ...qm.QueryMod) collectionsContentUnitQuery {
	return o.CollectionsContentUnits(boil.GetDB(), mods...)
}

// CollectionsContentUnits retrieves all the collections_content_unit's collections content units with an executor.
func (o *ContentUnit) CollectionsContentUnits(exec boil.Executor, mods ...qm.QueryMod) collectionsContentUnitQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"content_unit_id\"=?", o.ID),
	)

	query := CollectionsContentUnits(exec, queryMods...)
	queries.SetFrom(query.Query, "\"collections_content_units\" as \"a\"")
	return query
}

// ContentUnitI18nsG retrieves all the content_unit_i18n's content unit i18n.
func (o *ContentUnit) ContentUnitI18nsG(mods ...qm.QueryMod) contentUnitI18nQuery {
	return o.ContentUnitI18ns(boil.GetDB(), mods...)
}

// ContentUnitI18ns retrieves all the content_unit_i18n's content unit i18n with an executor.
func (o *ContentUnit) ContentUnitI18ns(exec boil.Executor, mods ...qm.QueryMod) contentUnitI18nQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"content_unit_id\"=?", o.ID),
	)

	query := ContentUnitI18ns(exec, queryMods...)
	queries.SetFrom(query.Query, "\"content_unit_i18n\" as \"a\"")
	return query
}

// LoadType allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (contentUnitL) LoadType(e boil.Executor, singular bool, maybeContentUnit interface{}) error {
	var slice []*ContentUnit
	var object *ContentUnit

	count := 1
	if singular {
		object = maybeContentUnit.(*ContentUnit)
	} else {
		slice = *maybeContentUnit.(*ContentUnitSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &contentUnitR{}
		}
		args[0] = object.TypeID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &contentUnitR{}
			}
			args[i] = obj.TypeID
		}
	}

	query := fmt.Sprintf(
		"select * from \"content_types\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load ContentType")
	}
	defer results.Close()

	var resultSlice []*ContentType
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice ContentType")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Type = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.TypeID == foreign.ID {
				local.R.Type = foreign
				break
			}
		}
	}

	return nil
}

// LoadFiles allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (contentUnitL) LoadFiles(e boil.Executor, singular bool, maybeContentUnit interface{}) error {
	var slice []*ContentUnit
	var object *ContentUnit

	count := 1
	if singular {
		object = maybeContentUnit.(*ContentUnit)
	} else {
		slice = *maybeContentUnit.(*ContentUnitSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &contentUnitR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &contentUnitR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"files\" where \"content_unit_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load files")
	}
	defer results.Close()

	var resultSlice []*File
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice files")
	}

	if singular {
		object.R.Files = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.ContentUnitID.Int64 {
				local.R.Files = append(local.R.Files, foreign)
				break
			}
		}
	}

	return nil
}

// LoadCollectionsContentUnits allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (contentUnitL) LoadCollectionsContentUnits(e boil.Executor, singular bool, maybeContentUnit interface{}) error {
	var slice []*ContentUnit
	var object *ContentUnit

	count := 1
	if singular {
		object = maybeContentUnit.(*ContentUnit)
	} else {
		slice = *maybeContentUnit.(*ContentUnitSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &contentUnitR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &contentUnitR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"collections_content_units\" where \"content_unit_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load collections_content_units")
	}
	defer results.Close()

	var resultSlice []*CollectionsContentUnit
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice collections_content_units")
	}

	if singular {
		object.R.CollectionsContentUnits = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.ContentUnitID {
				local.R.CollectionsContentUnits = append(local.R.CollectionsContentUnits, foreign)
				break
			}
		}
	}

	return nil
}

// LoadContentUnitI18ns allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (contentUnitL) LoadContentUnitI18ns(e boil.Executor, singular bool, maybeContentUnit interface{}) error {
	var slice []*ContentUnit
	var object *ContentUnit

	count := 1
	if singular {
		object = maybeContentUnit.(*ContentUnit)
	} else {
		slice = *maybeContentUnit.(*ContentUnitSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &contentUnitR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &contentUnitR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"content_unit_i18n\" where \"content_unit_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load content_unit_i18n")
	}
	defer results.Close()

	var resultSlice []*ContentUnitI18n
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice content_unit_i18n")
	}

	if singular {
		object.R.ContentUnitI18ns = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.ContentUnitID {
				local.R.ContentUnitI18ns = append(local.R.ContentUnitI18ns, foreign)
				break
			}
		}
	}

	return nil
}

// SetTypeG of the content_unit to the related item.
// Sets o.R.Type to related.
// Adds o to related.R.TypeContentUnits.
// Uses the global database handle.
func (o *ContentUnit) SetTypeG(insert bool, related *ContentType) error {
	return o.SetType(boil.GetDB(), insert, related)
}

// SetTypeP of the content_unit to the related item.
// Sets o.R.Type to related.
// Adds o to related.R.TypeContentUnits.
// Panics on error.
func (o *ContentUnit) SetTypeP(exec boil.Executor, insert bool, related *ContentType) {
	if err := o.SetType(exec, insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetTypeGP of the content_unit to the related item.
// Sets o.R.Type to related.
// Adds o to related.R.TypeContentUnits.
// Uses the global database handle and panics on error.
func (o *ContentUnit) SetTypeGP(insert bool, related *ContentType) {
	if err := o.SetType(boil.GetDB(), insert, related); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetType of the content_unit to the related item.
// Sets o.R.Type to related.
// Adds o to related.R.TypeContentUnits.
func (o *ContentUnit) SetType(exec boil.Executor, insert bool, related *ContentType) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"content_units\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"type_id"}),
		strmangle.WhereClause("\"", "\"", 2, contentUnitPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.TypeID = related.ID

	if o.R == nil {
		o.R = &contentUnitR{
			Type: related,
		}
	} else {
		o.R.Type = related
	}

	if related.R == nil {
		related.R = &contentTypeR{
			TypeContentUnits: ContentUnitSlice{o},
		}
	} else {
		related.R.TypeContentUnits = append(related.R.TypeContentUnits, o)
	}

	return nil
}

// AddFilesG adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.Files.
// Sets related.R.ContentUnit appropriately.
// Uses the global database handle.
func (o *ContentUnit) AddFilesG(insert bool, related ...*File) error {
	return o.AddFiles(boil.GetDB(), insert, related...)
}

// AddFilesP adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.Files.
// Sets related.R.ContentUnit appropriately.
// Panics on error.
func (o *ContentUnit) AddFilesP(exec boil.Executor, insert bool, related ...*File) {
	if err := o.AddFiles(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddFilesGP adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.Files.
// Sets related.R.ContentUnit appropriately.
// Uses the global database handle and panics on error.
func (o *ContentUnit) AddFilesGP(insert bool, related ...*File) {
	if err := o.AddFiles(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddFiles adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.Files.
// Sets related.R.ContentUnit appropriately.
func (o *ContentUnit) AddFiles(exec boil.Executor, insert bool, related ...*File) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.ContentUnitID.Int64 = o.ID
			rel.ContentUnitID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"files\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"content_unit_id"}),
				strmangle.WhereClause("\"", "\"", 2, filePrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.ContentUnitID.Int64 = o.ID
			rel.ContentUnitID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &contentUnitR{
			Files: related,
		}
	} else {
		o.R.Files = append(o.R.Files, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &fileR{
				ContentUnit: o,
			}
		} else {
			rel.R.ContentUnit = o
		}
	}
	return nil
}

// SetFilesG removes all previously related items of the
// content_unit replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.ContentUnit's Files accordingly.
// Replaces o.R.Files with related.
// Sets related.R.ContentUnit's Files accordingly.
// Uses the global database handle.
func (o *ContentUnit) SetFilesG(insert bool, related ...*File) error {
	return o.SetFiles(boil.GetDB(), insert, related...)
}

// SetFilesP removes all previously related items of the
// content_unit replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.ContentUnit's Files accordingly.
// Replaces o.R.Files with related.
// Sets related.R.ContentUnit's Files accordingly.
// Panics on error.
func (o *ContentUnit) SetFilesP(exec boil.Executor, insert bool, related ...*File) {
	if err := o.SetFiles(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetFilesGP removes all previously related items of the
// content_unit replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.ContentUnit's Files accordingly.
// Replaces o.R.Files with related.
// Sets related.R.ContentUnit's Files accordingly.
// Uses the global database handle and panics on error.
func (o *ContentUnit) SetFilesGP(insert bool, related ...*File) {
	if err := o.SetFiles(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// SetFiles removes all previously related items of the
// content_unit replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.ContentUnit's Files accordingly.
// Replaces o.R.Files with related.
// Sets related.R.ContentUnit's Files accordingly.
func (o *ContentUnit) SetFiles(exec boil.Executor, insert bool, related ...*File) error {
	query := "update \"files\" set \"content_unit_id\" = null where \"content_unit_id\" = $1"
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
		for _, rel := range o.R.Files {
			rel.ContentUnitID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.ContentUnit = nil
		}

		o.R.Files = nil
	}
	return o.AddFiles(exec, insert, related...)
}

// RemoveFilesG relationships from objects passed in.
// Removes related items from R.Files (uses pointer comparison, removal does not keep order)
// Sets related.R.ContentUnit.
// Uses the global database handle.
func (o *ContentUnit) RemoveFilesG(related ...*File) error {
	return o.RemoveFiles(boil.GetDB(), related...)
}

// RemoveFilesP relationships from objects passed in.
// Removes related items from R.Files (uses pointer comparison, removal does not keep order)
// Sets related.R.ContentUnit.
// Panics on error.
func (o *ContentUnit) RemoveFilesP(exec boil.Executor, related ...*File) {
	if err := o.RemoveFiles(exec, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// RemoveFilesGP relationships from objects passed in.
// Removes related items from R.Files (uses pointer comparison, removal does not keep order)
// Sets related.R.ContentUnit.
// Uses the global database handle and panics on error.
func (o *ContentUnit) RemoveFilesGP(related ...*File) {
	if err := o.RemoveFiles(boil.GetDB(), related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// RemoveFiles relationships from objects passed in.
// Removes related items from R.Files (uses pointer comparison, removal does not keep order)
// Sets related.R.ContentUnit.
func (o *ContentUnit) RemoveFiles(exec boil.Executor, related ...*File) error {
	var err error
	for _, rel := range related {
		rel.ContentUnitID.Valid = false
		if rel.R != nil {
			rel.R.ContentUnit = nil
		}
		if err = rel.Update(exec, "content_unit_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.Files {
			if rel != ri {
				continue
			}

			ln := len(o.R.Files)
			if ln > 1 && i < ln-1 {
				o.R.Files[i] = o.R.Files[ln-1]
			}
			o.R.Files = o.R.Files[:ln-1]
			break
		}
	}

	return nil
}

// AddCollectionsContentUnitsG adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.CollectionsContentUnits.
// Sets related.R.ContentUnit appropriately.
// Uses the global database handle.
func (o *ContentUnit) AddCollectionsContentUnitsG(insert bool, related ...*CollectionsContentUnit) error {
	return o.AddCollectionsContentUnits(boil.GetDB(), insert, related...)
}

// AddCollectionsContentUnitsP adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.CollectionsContentUnits.
// Sets related.R.ContentUnit appropriately.
// Panics on error.
func (o *ContentUnit) AddCollectionsContentUnitsP(exec boil.Executor, insert bool, related ...*CollectionsContentUnit) {
	if err := o.AddCollectionsContentUnits(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddCollectionsContentUnitsGP adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.CollectionsContentUnits.
// Sets related.R.ContentUnit appropriately.
// Uses the global database handle and panics on error.
func (o *ContentUnit) AddCollectionsContentUnitsGP(insert bool, related ...*CollectionsContentUnit) {
	if err := o.AddCollectionsContentUnits(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddCollectionsContentUnits adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.CollectionsContentUnits.
// Sets related.R.ContentUnit appropriately.
func (o *ContentUnit) AddCollectionsContentUnits(exec boil.Executor, insert bool, related ...*CollectionsContentUnit) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.ContentUnitID = o.ID
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"collections_content_units\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"content_unit_id"}),
				strmangle.WhereClause("\"", "\"", 2, collectionsContentUnitPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.CollectionID, rel.ContentUnitID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.ContentUnitID = o.ID
		}
	}

	if o.R == nil {
		o.R = &contentUnitR{
			CollectionsContentUnits: related,
		}
	} else {
		o.R.CollectionsContentUnits = append(o.R.CollectionsContentUnits, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &collectionsContentUnitR{
				ContentUnit: o,
			}
		} else {
			rel.R.ContentUnit = o
		}
	}
	return nil
}

// AddContentUnitI18nsG adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.ContentUnitI18ns.
// Sets related.R.ContentUnit appropriately.
// Uses the global database handle.
func (o *ContentUnit) AddContentUnitI18nsG(insert bool, related ...*ContentUnitI18n) error {
	return o.AddContentUnitI18ns(boil.GetDB(), insert, related...)
}

// AddContentUnitI18nsP adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.ContentUnitI18ns.
// Sets related.R.ContentUnit appropriately.
// Panics on error.
func (o *ContentUnit) AddContentUnitI18nsP(exec boil.Executor, insert bool, related ...*ContentUnitI18n) {
	if err := o.AddContentUnitI18ns(exec, insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddContentUnitI18nsGP adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.ContentUnitI18ns.
// Sets related.R.ContentUnit appropriately.
// Uses the global database handle and panics on error.
func (o *ContentUnit) AddContentUnitI18nsGP(insert bool, related ...*ContentUnitI18n) {
	if err := o.AddContentUnitI18ns(boil.GetDB(), insert, related...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// AddContentUnitI18ns adds the given related objects to the existing relationships
// of the content_unit, optionally inserting them as new records.
// Appends related to o.R.ContentUnitI18ns.
// Sets related.R.ContentUnit appropriately.
func (o *ContentUnit) AddContentUnitI18ns(exec boil.Executor, insert bool, related ...*ContentUnitI18n) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.ContentUnitID = o.ID
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"content_unit_i18n\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"content_unit_id"}),
				strmangle.WhereClause("\"", "\"", 2, contentUnitI18nPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ContentUnitID, rel.Language}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.ContentUnitID = o.ID
		}
	}

	if o.R == nil {
		o.R = &contentUnitR{
			ContentUnitI18ns: related,
		}
	} else {
		o.R.ContentUnitI18ns = append(o.R.ContentUnitI18ns, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &contentUnitI18nR{
				ContentUnit: o,
			}
		} else {
			rel.R.ContentUnit = o
		}
	}
	return nil
}

// ContentUnitsG retrieves all records.
func ContentUnitsG(mods ...qm.QueryMod) contentUnitQuery {
	return ContentUnits(boil.GetDB(), mods...)
}

// ContentUnits retrieves all the records using an executor.
func ContentUnits(exec boil.Executor, mods ...qm.QueryMod) contentUnitQuery {
	mods = append(mods, qm.From("\"content_units\""))
	return contentUnitQuery{NewQuery(exec, mods...)}
}

// FindContentUnitG retrieves a single record by ID.
func FindContentUnitG(id int64, selectCols ...string) (*ContentUnit, error) {
	return FindContentUnit(boil.GetDB(), id, selectCols...)
}

// FindContentUnitGP retrieves a single record by ID, and panics on error.
func FindContentUnitGP(id int64, selectCols ...string) *ContentUnit {
	retobj, err := FindContentUnit(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindContentUnit retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindContentUnit(exec boil.Executor, id int64, selectCols ...string) (*ContentUnit, error) {
	contentUnitObj := &ContentUnit{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"content_units\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(contentUnitObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "mdbmodels: unable to select from content_units")
	}

	return contentUnitObj, nil
}

// FindContentUnitP retrieves a single record by ID with an executor, and panics on error.
func FindContentUnitP(exec boil.Executor, id int64, selectCols ...string) *ContentUnit {
	retobj, err := FindContentUnit(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *ContentUnit) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *ContentUnit) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *ContentUnit) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *ContentUnit) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no content_units provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(contentUnitColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	contentUnitInsertCacheMut.RLock()
	cache, cached := contentUnitInsertCache[key]
	contentUnitInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			contentUnitColumns,
			contentUnitColumnsWithDefault,
			contentUnitColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(contentUnitType, contentUnitMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(contentUnitType, contentUnitMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"content_units\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "mdbmodels: unable to insert into content_units")
	}

	if !cached {
		contentUnitInsertCacheMut.Lock()
		contentUnitInsertCache[key] = cache
		contentUnitInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single ContentUnit record. See Update for
// whitelist behavior description.
func (o *ContentUnit) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single ContentUnit record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *ContentUnit) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the ContentUnit, and panics on error.
// See Update for whitelist behavior description.
func (o *ContentUnit) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the ContentUnit.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *ContentUnit) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	contentUnitUpdateCacheMut.RLock()
	cache, cached := contentUnitUpdateCache[key]
	contentUnitUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(contentUnitColumns, contentUnitPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("mdbmodels: unable to update content_units, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"content_units\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, contentUnitPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(contentUnitType, contentUnitMapping, append(wl, contentUnitPrimaryKeyColumns...))
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
		return errors.Wrap(err, "mdbmodels: unable to update content_units row")
	}

	if !cached {
		contentUnitUpdateCacheMut.Lock()
		contentUnitUpdateCache[key] = cache
		contentUnitUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q contentUnitQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q contentUnitQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all for content_units")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ContentUnitSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o ContentUnitSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o ContentUnitSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ContentUnitSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentUnitPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"content_units\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(contentUnitPrimaryKeyColumns), len(colNames)+1, len(contentUnitPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to update all in contentUnit slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *ContentUnit) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *ContentUnit) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *ContentUnit) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *ContentUnit) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("mdbmodels: no content_units provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(contentUnitColumnsWithDefault, o)

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

	contentUnitUpsertCacheMut.RLock()
	cache, cached := contentUnitUpsertCache[key]
	contentUnitUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			contentUnitColumns,
			contentUnitColumnsWithDefault,
			contentUnitColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			contentUnitColumns,
			contentUnitPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("mdbmodels: unable to upsert content_units, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(contentUnitPrimaryKeyColumns))
			copy(conflict, contentUnitPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"content_units\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(contentUnitType, contentUnitMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(contentUnitType, contentUnitMapping, ret)
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
		return errors.Wrap(err, "mdbmodels: unable to upsert content_units")
	}

	if !cached {
		contentUnitUpsertCacheMut.Lock()
		contentUnitUpsertCache[key] = cache
		contentUnitUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single ContentUnit record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContentUnit) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single ContentUnit record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *ContentUnit) DeleteG() error {
	if o == nil {
		return errors.New("mdbmodels: no ContentUnit provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single ContentUnit record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ContentUnit) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single ContentUnit record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *ContentUnit) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no ContentUnit provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), contentUnitPrimaryKeyMapping)
	sql := "DELETE FROM \"content_units\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete from content_units")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q contentUnitQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q contentUnitQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("mdbmodels: no contentUnitQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from content_units")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o ContentUnitSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o ContentUnitSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("mdbmodels: no ContentUnit slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o ContentUnitSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ContentUnitSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("mdbmodels: no ContentUnit slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentUnitPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"content_units\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, contentUnitPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(contentUnitPrimaryKeyColumns), 1, len(contentUnitPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to delete all from contentUnit slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *ContentUnit) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *ContentUnit) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *ContentUnit) ReloadG() error {
	if o == nil {
		return errors.New("mdbmodels: no ContentUnit provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *ContentUnit) Reload(exec boil.Executor) error {
	ret, err := FindContentUnit(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContentUnitSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContentUnitSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContentUnitSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("mdbmodels: empty ContentUnitSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContentUnitSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	contentUnits := ContentUnitSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), contentUnitPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"content_units\".* FROM \"content_units\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, contentUnitPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(contentUnitPrimaryKeyColumns), 1, len(contentUnitPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&contentUnits)
	if err != nil {
		return errors.Wrap(err, "mdbmodels: unable to reload all in ContentUnitSlice")
	}

	*o = contentUnits

	return nil
}

// ContentUnitExists checks if the ContentUnit row exists.
func ContentUnitExists(exec boil.Executor, id int64) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"content_units\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "mdbmodels: unable to check if content_units exists")
	}

	return exists, nil
}

// ContentUnitExistsG checks if the ContentUnit row exists.
func ContentUnitExistsG(id int64) (bool, error) {
	return ContentUnitExists(boil.GetDB(), id)
}

// ContentUnitExistsGP checks if the ContentUnit row exists. Panics on error.
func ContentUnitExistsGP(id int64) bool {
	e, err := ContentUnitExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// ContentUnitExistsP checks if the ContentUnit row exists. Panics on error.
func ContentUnitExistsP(exec boil.Executor, id int64) bool {
	e, err := ContentUnitExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

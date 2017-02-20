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

// FileAsset is an object representing the database table.
type FileAsset struct {
	ID           int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	Name         null.String `boil:"name" json:"name,omitempty" toml:"name" yaml:"name,omitempty"`
	LangID       null.String `boil:"lang_id" json:"lang_id,omitempty" toml:"lang_id" yaml:"lang_id,omitempty"`
	AssetTypeID  null.String `boil:"asset_type_id" json:"asset_type_id,omitempty" toml:"asset_type_id" yaml:"asset_type_id,omitempty"`
	Date         null.Time   `boil:"date" json:"date,omitempty" toml:"date" yaml:"date,omitempty"`
	Size         null.Int    `boil:"size" json:"size,omitempty" toml:"size" yaml:"size,omitempty"`
	ServerNameID null.String `boil:"server_name_id" json:"server_name_id,omitempty" toml:"server_name_id" yaml:"server_name_id,omitempty"`
	Status       null.String `boil:"status" json:"status,omitempty" toml:"status" yaml:"status,omitempty"`
	CreatedAt    null.Time   `boil:"created_at" json:"created_at,omitempty" toml:"created_at" yaml:"created_at,omitempty"`
	UpdatedAt    null.Time   `boil:"updated_at" json:"updated_at,omitempty" toml:"updated_at" yaml:"updated_at,omitempty"`
	Lastuser     null.String `boil:"lastuser" json:"lastuser,omitempty" toml:"lastuser" yaml:"lastuser,omitempty"`
	Clicks       null.Int    `boil:"clicks" json:"clicks,omitempty" toml:"clicks" yaml:"clicks,omitempty"`
	Secure       null.Int    `boil:"secure" json:"secure,omitempty" toml:"secure" yaml:"secure,omitempty"`
	PlaytimeSecs null.Int    `boil:"playtime_secs" json:"playtime_secs,omitempty" toml:"playtime_secs" yaml:"playtime_secs,omitempty"`
	UserID       null.Int    `boil:"user_id" json:"user_id,omitempty" toml:"user_id" yaml:"user_id,omitempty"`

	R *fileAssetR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L fileAssetL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// fileAssetR is where relationships are stored.
type fileAssetR struct {
	AssetType                 *FileType
	User                      *User
	ServerName                *Server
	Lang                      *Language
	Containers                ContainerSlice
	FileFileAssetDescriptions FileAssetDescriptionSlice
}

// fileAssetL is where Load methods for each relationship are stored.
type fileAssetL struct{}

var (
	fileAssetColumns               = []string{"id", "name", "lang_id", "asset_type_id", "date", "size", "server_name_id", "status", "created_at", "updated_at", "lastuser", "clicks", "secure", "playtime_secs", "user_id"}
	fileAssetColumnsWithoutDefault = []string{"name", "lang_id", "asset_type_id", "date", "size", "status", "created_at", "updated_at", "lastuser", "playtime_secs", "user_id"}
	fileAssetColumnsWithDefault    = []string{"id", "server_name_id", "clicks", "secure"}
	fileAssetPrimaryKeyColumns     = []string{"id"}
)

type (
	// FileAssetSlice is an alias for a slice of pointers to FileAsset.
	// This should generally be used opposed to []FileAsset.
	FileAssetSlice []*FileAsset

	fileAssetQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	fileAssetType                 = reflect.TypeOf(&FileAsset{})
	fileAssetMapping              = queries.MakeStructMapping(fileAssetType)
	fileAssetPrimaryKeyMapping, _ = queries.BindMapping(fileAssetType, fileAssetMapping, fileAssetPrimaryKeyColumns)
	fileAssetInsertCacheMut       sync.RWMutex
	fileAssetInsertCache          = make(map[string]insertCache)
	fileAssetUpdateCacheMut       sync.RWMutex
	fileAssetUpdateCache          = make(map[string]updateCache)
	fileAssetUpsertCacheMut       sync.RWMutex
	fileAssetUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single fileAsset record from the query, and panics on error.
func (q fileAssetQuery) OneP() *FileAsset {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single fileAsset record from the query.
func (q fileAssetQuery) One() (*FileAsset, error) {
	o := &FileAsset{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for file_assets")
	}

	return o, nil
}

// AllP returns all FileAsset records from the query, and panics on error.
func (q fileAssetQuery) AllP() FileAssetSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all FileAsset records from the query.
func (q fileAssetQuery) All() (FileAssetSlice, error) {
	var o FileAssetSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to FileAsset slice")
	}

	return o, nil
}

// CountP returns the count of all FileAsset records in the query, and panics on error.
func (q fileAssetQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all FileAsset records in the query.
func (q fileAssetQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count file_assets rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q fileAssetQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q fileAssetQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if file_assets exists")
	}

	return count > 0, nil
}

// AssetTypeG pointed to by the foreign key.
func (o *FileAsset) AssetTypeG(mods ...qm.QueryMod) fileTypeQuery {
	return o.AssetType(boil.GetDB(), mods...)
}

// AssetType pointed to by the foreign key.
func (o *FileAsset) AssetType(exec boil.Executor, mods ...qm.QueryMod) fileTypeQuery {
	queryMods := []qm.QueryMod{
		qm.Where("name=?", o.AssetTypeID),
	}

	queryMods = append(queryMods, mods...)

	query := FileTypes(exec, queryMods...)
	queries.SetFrom(query.Query, "\"file_types\"")

	return query
}

// UserG pointed to by the foreign key.
func (o *FileAsset) UserG(mods ...qm.QueryMod) userQuery {
	return o.User(boil.GetDB(), mods...)
}

// User pointed to by the foreign key.
func (o *FileAsset) User(exec boil.Executor, mods ...qm.QueryMod) userQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.UserID),
	}

	queryMods = append(queryMods, mods...)

	query := Users(exec, queryMods...)
	queries.SetFrom(query.Query, "\"users\"")

	return query
}

// ServerNameG pointed to by the foreign key.
func (o *FileAsset) ServerNameG(mods ...qm.QueryMod) serverQuery {
	return o.ServerName(boil.GetDB(), mods...)
}

// ServerName pointed to by the foreign key.
func (o *FileAsset) ServerName(exec boil.Executor, mods ...qm.QueryMod) serverQuery {
	queryMods := []qm.QueryMod{
		qm.Where("servername=?", o.ServerNameID),
	}

	queryMods = append(queryMods, mods...)

	query := Servers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"servers\"")

	return query
}

// LangG pointed to by the foreign key.
func (o *FileAsset) LangG(mods ...qm.QueryMod) languageQuery {
	return o.Lang(boil.GetDB(), mods...)
}

// Lang pointed to by the foreign key.
func (o *FileAsset) Lang(exec boil.Executor, mods ...qm.QueryMod) languageQuery {
	queryMods := []qm.QueryMod{
		qm.Where("code3=?", o.LangID),
	}

	queryMods = append(queryMods, mods...)

	query := Languages(exec, queryMods...)
	queries.SetFrom(query.Query, "\"languages\"")

	return query
}

// ContainersG retrieves all the container's containers.
func (o *FileAsset) ContainersG(mods ...qm.QueryMod) containerQuery {
	return o.Containers(boil.GetDB(), mods...)
}

// Containers retrieves all the container's containers with an executor.
func (o *FileAsset) Containers(exec boil.Executor, mods ...qm.QueryMod) containerQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.InnerJoin("\"containers_file_assets\" as \"b\" on \"a\".\"id\" = \"b\".\"container_id\""),
		qm.Where("\"b\".\"file_asset_id\"=?", o.ID),
	)

	query := Containers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"containers\" as \"a\"")
	return query
}

// FileFileAssetDescriptionsG retrieves all the file_asset_description's file asset descriptions via file_id column.
func (o *FileAsset) FileFileAssetDescriptionsG(mods ...qm.QueryMod) fileAssetDescriptionQuery {
	return o.FileFileAssetDescriptions(boil.GetDB(), mods...)
}

// FileFileAssetDescriptions retrieves all the file_asset_description's file asset descriptions with an executor via file_id column.
func (o *FileAsset) FileFileAssetDescriptions(exec boil.Executor, mods ...qm.QueryMod) fileAssetDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"file_id\"=?", o.ID),
	)

	query := FileAssetDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"file_asset_descriptions\" as \"a\"")
	return query
}

// LoadAssetType allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (fileAssetL) LoadAssetType(e boil.Executor, singular bool, maybeFileAsset interface{}) error {
	var slice []*FileAsset
	var object *FileAsset

	count := 1
	if singular {
		object = maybeFileAsset.(*FileAsset)
	} else {
		slice = *maybeFileAsset.(*FileAssetSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &fileAssetR{}
		}
		args[0] = object.AssetTypeID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &fileAssetR{}
			}
			args[i] = obj.AssetTypeID
		}
	}

	query := fmt.Sprintf(
		"select * from \"file_types\" where \"name\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load FileType")
	}
	defer results.Close()

	var resultSlice []*FileType
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice FileType")
	}

	if singular && len(resultSlice) != 0 {
		object.R.AssetType = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.AssetTypeID.String == foreign.Name {
				local.R.AssetType = foreign
				break
			}
		}
	}

	return nil
}

// LoadUser allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (fileAssetL) LoadUser(e boil.Executor, singular bool, maybeFileAsset interface{}) error {
	var slice []*FileAsset
	var object *FileAsset

	count := 1
	if singular {
		object = maybeFileAsset.(*FileAsset)
	} else {
		slice = *maybeFileAsset.(*FileAssetSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &fileAssetR{}
		}
		args[0] = object.UserID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &fileAssetR{}
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

// LoadServerName allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (fileAssetL) LoadServerName(e boil.Executor, singular bool, maybeFileAsset interface{}) error {
	var slice []*FileAsset
	var object *FileAsset

	count := 1
	if singular {
		object = maybeFileAsset.(*FileAsset)
	} else {
		slice = *maybeFileAsset.(*FileAssetSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &fileAssetR{}
		}
		args[0] = object.ServerNameID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &fileAssetR{}
			}
			args[i] = obj.ServerNameID
		}
	}

	query := fmt.Sprintf(
		"select * from \"servers\" where \"servername\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Server")
	}
	defer results.Close()

	var resultSlice []*Server
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Server")
	}

	if singular && len(resultSlice) != 0 {
		object.R.ServerName = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ServerNameID.String == foreign.Servername {
				local.R.ServerName = foreign
				break
			}
		}
	}

	return nil
}

// LoadLang allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (fileAssetL) LoadLang(e boil.Executor, singular bool, maybeFileAsset interface{}) error {
	var slice []*FileAsset
	var object *FileAsset

	count := 1
	if singular {
		object = maybeFileAsset.(*FileAsset)
	} else {
		slice = *maybeFileAsset.(*FileAssetSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &fileAssetR{}
		}
		args[0] = object.LangID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &fileAssetR{}
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

// LoadContainers allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (fileAssetL) LoadContainers(e boil.Executor, singular bool, maybeFileAsset interface{}) error {
	var slice []*FileAsset
	var object *FileAsset

	count := 1
	if singular {
		object = maybeFileAsset.(*FileAsset)
	} else {
		slice = *maybeFileAsset.(*FileAssetSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &fileAssetR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &fileAssetR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select \"a\".*, \"b\".\"file_asset_id\" from \"containers\" as \"a\" inner join \"containers_file_assets\" as \"b\" on \"a\".\"id\" = \"b\".\"container_id\" where \"b\".\"file_asset_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load containers")
	}
	defer results.Close()

	var resultSlice []*Container

	var localJoinCols []int
	for results.Next() {
		one := new(Container)
		var localJoinCol int

		err = results.Scan(&one.ID, &one.Name, &one.CreatedAt, &one.UpdatedAt, &one.Filmdate, &one.LangID, &one.LecturerID, &one.Secure, &one.ContentTypeID, &one.MarkedForMerge, &one.SecureChanged, &one.AutoParsed, &one.VirtualLessonID, &one.PlaytimeSecs, &one.UserID, &one.ForCensorship, &one.OpenedByCensor, &one.ClosedByCensor, &one.CensorID, &one.Position, &localJoinCol)
		if err = results.Err(); err != nil {
			return errors.Wrap(err, "failed to plebian-bind eager loaded slice containers")
		}

		resultSlice = append(resultSlice, one)
		localJoinCols = append(localJoinCols, localJoinCol)
	}

	if err = results.Err(); err != nil {
		return errors.Wrap(err, "failed to plebian-bind eager loaded slice containers")
	}

	if singular {
		object.R.Containers = resultSlice
		return nil
	}

	for i, foreign := range resultSlice {
		localJoinCol := localJoinCols[i]
		for _, local := range slice {
			if local.ID == localJoinCol {
				local.R.Containers = append(local.R.Containers, foreign)
				break
			}
		}
	}

	return nil
}

// LoadFileFileAssetDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (fileAssetL) LoadFileFileAssetDescriptions(e boil.Executor, singular bool, maybeFileAsset interface{}) error {
	var slice []*FileAsset
	var object *FileAsset

	count := 1
	if singular {
		object = maybeFileAsset.(*FileAsset)
	} else {
		slice = *maybeFileAsset.(*FileAssetSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &fileAssetR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &fileAssetR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"file_asset_descriptions\" where \"file_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load file_asset_descriptions")
	}
	defer results.Close()

	var resultSlice []*FileAssetDescription
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice file_asset_descriptions")
	}

	if singular {
		object.R.FileFileAssetDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.FileID {
				local.R.FileFileAssetDescriptions = append(local.R.FileFileAssetDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// SetAssetType of the file_asset to the related item.
// Sets o.R.AssetType to related.
// Adds o to related.R.AssetTypeFileAssets.
func (o *FileAsset) SetAssetType(exec boil.Executor, insert bool, related *FileType) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"file_assets\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"asset_type_id"}),
		strmangle.WhereClause("\"", "\"", 2, fileAssetPrimaryKeyColumns),
	)
	values := []interface{}{related.Name, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.AssetTypeID.String = related.Name
	o.AssetTypeID.Valid = true

	if o.R == nil {
		o.R = &fileAssetR{
			AssetType: related,
		}
	} else {
		o.R.AssetType = related
	}

	if related.R == nil {
		related.R = &fileTypeR{
			AssetTypeFileAssets: FileAssetSlice{o},
		}
	} else {
		related.R.AssetTypeFileAssets = append(related.R.AssetTypeFileAssets, o)
	}

	return nil
}

// RemoveAssetType relationship.
// Sets o.R.AssetType to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *FileAsset) RemoveAssetType(exec boil.Executor, related *FileType) error {
	var err error

	o.AssetTypeID.Valid = false
	if err = o.Update(exec, "asset_type_id"); err != nil {
		o.AssetTypeID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.AssetType = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.AssetTypeFileAssets {
		if o.AssetTypeID.String != ri.AssetTypeID.String {
			continue
		}

		ln := len(related.R.AssetTypeFileAssets)
		if ln > 1 && i < ln-1 {
			related.R.AssetTypeFileAssets[i] = related.R.AssetTypeFileAssets[ln-1]
		}
		related.R.AssetTypeFileAssets = related.R.AssetTypeFileAssets[:ln-1]
		break
	}
	return nil
}

// SetUser of the file_asset to the related item.
// Sets o.R.User to related.
// Adds o to related.R.FileAssets.
func (o *FileAsset) SetUser(exec boil.Executor, insert bool, related *User) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"file_assets\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
		strmangle.WhereClause("\"", "\"", 2, fileAssetPrimaryKeyColumns),
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
		o.R = &fileAssetR{
			User: related,
		}
	} else {
		o.R.User = related
	}

	if related.R == nil {
		related.R = &userR{
			FileAssets: FileAssetSlice{o},
		}
	} else {
		related.R.FileAssets = append(related.R.FileAssets, o)
	}

	return nil
}

// RemoveUser relationship.
// Sets o.R.User to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *FileAsset) RemoveUser(exec boil.Executor, related *User) error {
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

	for i, ri := range related.R.FileAssets {
		if o.UserID.Int != ri.UserID.Int {
			continue
		}

		ln := len(related.R.FileAssets)
		if ln > 1 && i < ln-1 {
			related.R.FileAssets[i] = related.R.FileAssets[ln-1]
		}
		related.R.FileAssets = related.R.FileAssets[:ln-1]
		break
	}
	return nil
}

// SetServerName of the file_asset to the related item.
// Sets o.R.ServerName to related.
// Adds o to related.R.ServerNameFileAssets.
func (o *FileAsset) SetServerName(exec boil.Executor, insert bool, related *Server) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"file_assets\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"server_name_id"}),
		strmangle.WhereClause("\"", "\"", 2, fileAssetPrimaryKeyColumns),
	)
	values := []interface{}{related.Servername, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.ServerNameID.String = related.Servername
	o.ServerNameID.Valid = true

	if o.R == nil {
		o.R = &fileAssetR{
			ServerName: related,
		}
	} else {
		o.R.ServerName = related
	}

	if related.R == nil {
		related.R = &serverR{
			ServerNameFileAssets: FileAssetSlice{o},
		}
	} else {
		related.R.ServerNameFileAssets = append(related.R.ServerNameFileAssets, o)
	}

	return nil
}

// RemoveServerName relationship.
// Sets o.R.ServerName to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *FileAsset) RemoveServerName(exec boil.Executor, related *Server) error {
	var err error

	o.ServerNameID.Valid = false
	if err = o.Update(exec, "server_name_id"); err != nil {
		o.ServerNameID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.ServerName = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.ServerNameFileAssets {
		if o.ServerNameID.String != ri.ServerNameID.String {
			continue
		}

		ln := len(related.R.ServerNameFileAssets)
		if ln > 1 && i < ln-1 {
			related.R.ServerNameFileAssets[i] = related.R.ServerNameFileAssets[ln-1]
		}
		related.R.ServerNameFileAssets = related.R.ServerNameFileAssets[:ln-1]
		break
	}
	return nil
}

// SetLang of the file_asset to the related item.
// Sets o.R.Lang to related.
// Adds o to related.R.LangFileAssets.
func (o *FileAsset) SetLang(exec boil.Executor, insert bool, related *Language) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"file_assets\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
		strmangle.WhereClause("\"", "\"", 2, fileAssetPrimaryKeyColumns),
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
		o.R = &fileAssetR{
			Lang: related,
		}
	} else {
		o.R.Lang = related
	}

	if related.R == nil {
		related.R = &languageR{
			LangFileAssets: FileAssetSlice{o},
		}
	} else {
		related.R.LangFileAssets = append(related.R.LangFileAssets, o)
	}

	return nil
}

// RemoveLang relationship.
// Sets o.R.Lang to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *FileAsset) RemoveLang(exec boil.Executor, related *Language) error {
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

	for i, ri := range related.R.LangFileAssets {
		if o.LangID.String != ri.LangID.String {
			continue
		}

		ln := len(related.R.LangFileAssets)
		if ln > 1 && i < ln-1 {
			related.R.LangFileAssets[i] = related.R.LangFileAssets[ln-1]
		}
		related.R.LangFileAssets = related.R.LangFileAssets[:ln-1]
		break
	}
	return nil
}

// AddContainers adds the given related objects to the existing relationships
// of the file_asset, optionally inserting them as new records.
// Appends related to o.R.Containers.
// Sets related.R.FileAssets appropriately.
func (o *FileAsset) AddContainers(exec boil.Executor, insert bool, related ...*Container) error {
	var err error
	for _, rel := range related {
		if insert {
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		}
	}

	for _, rel := range related {
		query := "insert into \"containers_file_assets\" (\"file_asset_id\", \"container_id\") values ($1, $2)"
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
		o.R = &fileAssetR{
			Containers: related,
		}
	} else {
		o.R.Containers = append(o.R.Containers, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerR{
				FileAssets: FileAssetSlice{o},
			}
		} else {
			rel.R.FileAssets = append(rel.R.FileAssets, o)
		}
	}
	return nil
}

// SetContainers removes all previously related items of the
// file_asset replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.FileAssets's Containers accordingly.
// Replaces o.R.Containers with related.
// Sets related.R.FileAssets's Containers accordingly.
func (o *FileAsset) SetContainers(exec boil.Executor, insert bool, related ...*Container) error {
	query := "delete from \"containers_file_assets\" where \"file_asset_id\" = $1"
	values := []interface{}{o.ID}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	removeContainersFromFileAssetsSlice(o, related)
	o.R.Containers = nil
	return o.AddContainers(exec, insert, related...)
}

// RemoveContainers relationships from objects passed in.
// Removes related items from R.Containers (uses pointer comparison, removal does not keep order)
// Sets related.R.FileAssets.
func (o *FileAsset) RemoveContainers(exec boil.Executor, related ...*Container) error {
	var err error
	query := fmt.Sprintf(
		"delete from \"containers_file_assets\" where \"file_asset_id\" = $1 and \"container_id\" in (%s)",
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
	removeContainersFromFileAssetsSlice(o, related)
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.Containers {
			if rel != ri {
				continue
			}

			ln := len(o.R.Containers)
			if ln > 1 && i < ln-1 {
				o.R.Containers[i] = o.R.Containers[ln-1]
			}
			o.R.Containers = o.R.Containers[:ln-1]
			break
		}
	}

	return nil
}

func removeContainersFromFileAssetsSlice(o *FileAsset, related []*Container) {
	for _, rel := range related {
		if rel.R == nil {
			continue
		}
		for i, ri := range rel.R.FileAssets {
			if o.ID != ri.ID {
				continue
			}

			ln := len(rel.R.FileAssets)
			if ln > 1 && i < ln-1 {
				rel.R.FileAssets[i] = rel.R.FileAssets[ln-1]
			}
			rel.R.FileAssets = rel.R.FileAssets[:ln-1]
			break
		}
	}
}

// AddFileFileAssetDescriptions adds the given related objects to the existing relationships
// of the file_asset, optionally inserting them as new records.
// Appends related to o.R.FileFileAssetDescriptions.
// Sets related.R.File appropriately.
func (o *FileAsset) AddFileFileAssetDescriptions(exec boil.Executor, insert bool, related ...*FileAssetDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.FileID = o.ID
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"file_asset_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"file_id"}),
				strmangle.WhereClause("\"", "\"", 2, fileAssetDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.FileID = o.ID
		}
	}

	if o.R == nil {
		o.R = &fileAssetR{
			FileFileAssetDescriptions: related,
		}
	} else {
		o.R.FileFileAssetDescriptions = append(o.R.FileFileAssetDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &fileAssetDescriptionR{
				File: o,
			}
		} else {
			rel.R.File = o
		}
	}
	return nil
}

// FileAssetsG retrieves all records.
func FileAssetsG(mods ...qm.QueryMod) fileAssetQuery {
	return FileAssets(boil.GetDB(), mods...)
}

// FileAssets retrieves all the records using an executor.
func FileAssets(exec boil.Executor, mods ...qm.QueryMod) fileAssetQuery {
	mods = append(mods, qm.From("\"file_assets\""))
	return fileAssetQuery{NewQuery(exec, mods...)}
}

// FindFileAssetG retrieves a single record by ID.
func FindFileAssetG(id int, selectCols ...string) (*FileAsset, error) {
	return FindFileAsset(boil.GetDB(), id, selectCols...)
}

// FindFileAssetGP retrieves a single record by ID, and panics on error.
func FindFileAssetGP(id int, selectCols ...string) *FileAsset {
	retobj, err := FindFileAsset(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindFileAsset retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindFileAsset(exec boil.Executor, id int, selectCols ...string) (*FileAsset, error) {
	fileAssetObj := &FileAsset{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"file_assets\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(fileAssetObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from file_assets")
	}

	return fileAssetObj, nil
}

// FindFileAssetP retrieves a single record by ID with an executor, and panics on error.
func FindFileAssetP(exec boil.Executor, id int, selectCols ...string) *FileAsset {
	retobj, err := FindFileAsset(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *FileAsset) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *FileAsset) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *FileAsset) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *FileAsset) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no file_assets provided for insertion")
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

	nzDefaults := queries.NonZeroDefaultSet(fileAssetColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	fileAssetInsertCacheMut.RLock()
	cache, cached := fileAssetInsertCache[key]
	fileAssetInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			fileAssetColumns,
			fileAssetColumnsWithDefault,
			fileAssetColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(fileAssetType, fileAssetMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(fileAssetType, fileAssetMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"file_assets\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into file_assets")
	}

	if !cached {
		fileAssetInsertCacheMut.Lock()
		fileAssetInsertCache[key] = cache
		fileAssetInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single FileAsset record. See Update for
// whitelist behavior description.
func (o *FileAsset) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single FileAsset record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *FileAsset) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the FileAsset, and panics on error.
// See Update for whitelist behavior description.
func (o *FileAsset) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the FileAsset.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *FileAsset) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	var err error
	key := makeCacheKey(whitelist, nil)
	fileAssetUpdateCacheMut.RLock()
	cache, cached := fileAssetUpdateCache[key]
	fileAssetUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(fileAssetColumns, fileAssetPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update file_assets, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"file_assets\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, fileAssetPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(fileAssetType, fileAssetMapping, append(wl, fileAssetPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update file_assets row")
	}

	if !cached {
		fileAssetUpdateCacheMut.Lock()
		fileAssetUpdateCache[key] = cache
		fileAssetUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q fileAssetQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q fileAssetQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for file_assets")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o FileAssetSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o FileAssetSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o FileAssetSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o FileAssetSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), fileAssetPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"file_assets\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(fileAssetPrimaryKeyColumns), len(colNames)+1, len(fileAssetPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in fileAsset slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *FileAsset) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *FileAsset) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *FileAsset) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *FileAsset) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no file_assets provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.Time.IsZero() {
		o.CreatedAt.Time = currTime
		o.CreatedAt.Valid = true
	}
	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	nzDefaults := queries.NonZeroDefaultSet(fileAssetColumnsWithDefault, o)

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

	fileAssetUpsertCacheMut.RLock()
	cache, cached := fileAssetUpsertCache[key]
	fileAssetUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			fileAssetColumns,
			fileAssetColumnsWithDefault,
			fileAssetColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			fileAssetColumns,
			fileAssetPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert file_assets, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(fileAssetPrimaryKeyColumns))
			copy(conflict, fileAssetPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"file_assets\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(fileAssetType, fileAssetMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(fileAssetType, fileAssetMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert file_assets")
	}

	if !cached {
		fileAssetUpsertCacheMut.Lock()
		fileAssetUpsertCache[key] = cache
		fileAssetUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single FileAsset record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *FileAsset) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single FileAsset record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *FileAsset) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no FileAsset provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single FileAsset record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *FileAsset) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single FileAsset record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *FileAsset) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no FileAsset provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), fileAssetPrimaryKeyMapping)
	sql := "DELETE FROM \"file_assets\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from file_assets")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q fileAssetQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q fileAssetQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no fileAssetQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from file_assets")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o FileAssetSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o FileAssetSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no FileAsset slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o FileAssetSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o FileAssetSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no FileAsset slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), fileAssetPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"file_assets\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, fileAssetPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(fileAssetPrimaryKeyColumns), 1, len(fileAssetPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from fileAsset slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *FileAsset) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *FileAsset) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *FileAsset) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no FileAsset provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *FileAsset) Reload(exec boil.Executor) error {
	ret, err := FindFileAsset(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *FileAssetSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *FileAssetSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *FileAssetSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty FileAssetSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *FileAssetSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	fileAssets := FileAssetSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), fileAssetPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"file_assets\".* FROM \"file_assets\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, fileAssetPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(fileAssetPrimaryKeyColumns), 1, len(fileAssetPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&fileAssets)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in FileAssetSlice")
	}

	*o = fileAssets

	return nil
}

// FileAssetExists checks if the FileAsset row exists.
func FileAssetExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"file_assets\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if file_assets exists")
	}

	return exists, nil
}

// FileAssetExistsG checks if the FileAsset row exists.
func FileAssetExistsG(id int) (bool, error) {
	return FileAssetExists(boil.GetDB(), id)
}

// FileAssetExistsGP checks if the FileAsset row exists. Panics on error.
func FileAssetExistsGP(id int) bool {
	e, err := FileAssetExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// FileAssetExistsP checks if the FileAsset row exists. Panics on error.
func FileAssetExistsP(exec boil.Executor, id int) bool {
	e, err := FileAssetExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

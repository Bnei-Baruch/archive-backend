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

// Container is an object representing the database table.
type Container struct {
	ID              int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	Name            null.String `boil:"name" json:"name,omitempty" toml:"name" yaml:"name,omitempty"`
	CreatedAt       null.Time   `boil:"created_at" json:"created_at,omitempty" toml:"created_at" yaml:"created_at,omitempty"`
	UpdatedAt       null.Time   `boil:"updated_at" json:"updated_at,omitempty" toml:"updated_at" yaml:"updated_at,omitempty"`
	Filmdate        null.Time   `boil:"filmdate" json:"filmdate,omitempty" toml:"filmdate" yaml:"filmdate,omitempty"`
	LangID          null.String `boil:"lang_id" json:"lang_id,omitempty" toml:"lang_id" yaml:"lang_id,omitempty"`
	LecturerID      null.Int    `boil:"lecturer_id" json:"lecturer_id,omitempty" toml:"lecturer_id" yaml:"lecturer_id,omitempty"`
	Secure          int         `boil:"secure" json:"secure" toml:"secure" yaml:"secure"`
	ContentTypeID   null.Int    `boil:"content_type_id" json:"content_type_id,omitempty" toml:"content_type_id" yaml:"content_type_id,omitempty"`
	MarkedForMerge  null.Bool   `boil:"marked_for_merge" json:"marked_for_merge,omitempty" toml:"marked_for_merge" yaml:"marked_for_merge,omitempty"`
	SecureChanged   null.Bool   `boil:"secure_changed" json:"secure_changed,omitempty" toml:"secure_changed" yaml:"secure_changed,omitempty"`
	AutoParsed      null.Bool   `boil:"auto_parsed" json:"auto_parsed,omitempty" toml:"auto_parsed" yaml:"auto_parsed,omitempty"`
	VirtualLessonID null.Int    `boil:"virtual_lesson_id" json:"virtual_lesson_id,omitempty" toml:"virtual_lesson_id" yaml:"virtual_lesson_id,omitempty"`
	PlaytimeSecs    null.Int    `boil:"playtime_secs" json:"playtime_secs,omitempty" toml:"playtime_secs" yaml:"playtime_secs,omitempty"`
	UserID          null.Int    `boil:"user_id" json:"user_id,omitempty" toml:"user_id" yaml:"user_id,omitempty"`
	ForCensorship   null.Bool   `boil:"for_censorship" json:"for_censorship,omitempty" toml:"for_censorship" yaml:"for_censorship,omitempty"`
	OpenedByCensor  null.Bool   `boil:"opened_by_censor" json:"opened_by_censor,omitempty" toml:"opened_by_censor" yaml:"opened_by_censor,omitempty"`
	ClosedByCensor  null.Bool   `boil:"closed_by_censor" json:"closed_by_censor,omitempty" toml:"closed_by_censor" yaml:"closed_by_censor,omitempty"`
	CensorID        null.Int    `boil:"censor_id" json:"censor_id,omitempty" toml:"censor_id" yaml:"censor_id,omitempty"`
	Position        null.Int    `boil:"position" json:"position,omitempty" toml:"position" yaml:"position,omitempty"`

	R *containerR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L containerL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// containerR is where relationships are stored.
type containerR struct {
	Lecturer              *Lecturer
	ContentType           *ContentType
	VirtualLesson         *VirtualLesson
	User                  *User
	Censor                *User
	Lang                  *Language
	ContainerTranscripts  ContainerTranscriptSlice
	Catalogs              CatalogSlice
	Labels                LabelSlice
	FileAssets            FileAssetSlice
	ContainerDescriptions ContainerDescriptionSlice
}

// containerL is where Load methods for each relationship are stored.
type containerL struct{}

var (
	containerColumns               = []string{"id", "name", "created_at", "updated_at", "filmdate", "lang_id", "lecturer_id", "secure", "content_type_id", "marked_for_merge", "secure_changed", "auto_parsed", "virtual_lesson_id", "playtime_secs", "user_id", "for_censorship", "opened_by_censor", "closed_by_censor", "censor_id", "position"}
	containerColumnsWithoutDefault = []string{"name", "created_at", "updated_at", "filmdate", "lang_id", "lecturer_id", "content_type_id", "marked_for_merge", "virtual_lesson_id", "playtime_secs", "user_id", "censor_id", "position"}
	containerColumnsWithDefault    = []string{"id", "secure", "secure_changed", "auto_parsed", "for_censorship", "opened_by_censor", "closed_by_censor"}
	containerPrimaryKeyColumns     = []string{"id"}
)

type (
	// ContainerSlice is an alias for a slice of pointers to Container.
	// This should generally be used opposed to []Container.
	ContainerSlice []*Container

	containerQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	containerType                 = reflect.TypeOf(&Container{})
	containerMapping              = queries.MakeStructMapping(containerType)
	containerPrimaryKeyMapping, _ = queries.BindMapping(containerType, containerMapping, containerPrimaryKeyColumns)
	containerInsertCacheMut       sync.RWMutex
	containerInsertCache          = make(map[string]insertCache)
	containerUpdateCacheMut       sync.RWMutex
	containerUpdateCache          = make(map[string]updateCache)
	containerUpsertCacheMut       sync.RWMutex
	containerUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single container record from the query, and panics on error.
func (q containerQuery) OneP() *Container {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single container record from the query.
func (q containerQuery) One() (*Container, error) {
	o := &Container{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for containers")
	}

	return o, nil
}

// AllP returns all Container records from the query, and panics on error.
func (q containerQuery) AllP() ContainerSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all Container records from the query.
func (q containerQuery) All() (ContainerSlice, error) {
	var o ContainerSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to Container slice")
	}

	return o, nil
}

// CountP returns the count of all Container records in the query, and panics on error.
func (q containerQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all Container records in the query.
func (q containerQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count containers rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q containerQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q containerQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if containers exists")
	}

	return count > 0, nil
}

// LecturerG pointed to by the foreign key.
func (o *Container) LecturerG(mods ...qm.QueryMod) lecturerQuery {
	return o.Lecturer(boil.GetDB(), mods...)
}

// Lecturer pointed to by the foreign key.
func (o *Container) Lecturer(exec boil.Executor, mods ...qm.QueryMod) lecturerQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.LecturerID),
	}

	queryMods = append(queryMods, mods...)

	query := Lecturers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"lecturers\"")

	return query
}

// ContentTypeG pointed to by the foreign key.
func (o *Container) ContentTypeG(mods ...qm.QueryMod) contentTypeQuery {
	return o.ContentType(boil.GetDB(), mods...)
}

// ContentType pointed to by the foreign key.
func (o *Container) ContentType(exec boil.Executor, mods ...qm.QueryMod) contentTypeQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.ContentTypeID),
	}

	queryMods = append(queryMods, mods...)

	query := ContentTypes(exec, queryMods...)
	queries.SetFrom(query.Query, "\"content_types\"")

	return query
}

// VirtualLessonG pointed to by the foreign key.
func (o *Container) VirtualLessonG(mods ...qm.QueryMod) virtualLessonQuery {
	return o.VirtualLesson(boil.GetDB(), mods...)
}

// VirtualLesson pointed to by the foreign key.
func (o *Container) VirtualLesson(exec boil.Executor, mods ...qm.QueryMod) virtualLessonQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.VirtualLessonID),
	}

	queryMods = append(queryMods, mods...)

	query := VirtualLessons(exec, queryMods...)
	queries.SetFrom(query.Query, "\"virtual_lessons\"")

	return query
}

// UserG pointed to by the foreign key.
func (o *Container) UserG(mods ...qm.QueryMod) userQuery {
	return o.User(boil.GetDB(), mods...)
}

// User pointed to by the foreign key.
func (o *Container) User(exec boil.Executor, mods ...qm.QueryMod) userQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.UserID),
	}

	queryMods = append(queryMods, mods...)

	query := Users(exec, queryMods...)
	queries.SetFrom(query.Query, "\"users\"")

	return query
}

// CensorG pointed to by the foreign key.
func (o *Container) CensorG(mods ...qm.QueryMod) userQuery {
	return o.Censor(boil.GetDB(), mods...)
}

// Censor pointed to by the foreign key.
func (o *Container) Censor(exec boil.Executor, mods ...qm.QueryMod) userQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.CensorID),
	}

	queryMods = append(queryMods, mods...)

	query := Users(exec, queryMods...)
	queries.SetFrom(query.Query, "\"users\"")

	return query
}

// LangG pointed to by the foreign key.
func (o *Container) LangG(mods ...qm.QueryMod) languageQuery {
	return o.Lang(boil.GetDB(), mods...)
}

// Lang pointed to by the foreign key.
func (o *Container) Lang(exec boil.Executor, mods ...qm.QueryMod) languageQuery {
	queryMods := []qm.QueryMod{
		qm.Where("code3=?", o.LangID),
	}

	queryMods = append(queryMods, mods...)

	query := Languages(exec, queryMods...)
	queries.SetFrom(query.Query, "\"languages\"")

	return query
}

// ContainerTranscriptsG retrieves all the container_transcript's container transcripts.
func (o *Container) ContainerTranscriptsG(mods ...qm.QueryMod) containerTranscriptQuery {
	return o.ContainerTranscripts(boil.GetDB(), mods...)
}

// ContainerTranscripts retrieves all the container_transcript's container transcripts with an executor.
func (o *Container) ContainerTranscripts(exec boil.Executor, mods ...qm.QueryMod) containerTranscriptQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"container_id\"=?", o.ID),
	)

	query := ContainerTranscripts(exec, queryMods...)
	queries.SetFrom(query.Query, "\"container_transcripts\" as \"a\"")
	return query
}

// CatalogsG retrieves all the catalog's catalogs.
func (o *Container) CatalogsG(mods ...qm.QueryMod) catalogQuery {
	return o.Catalogs(boil.GetDB(), mods...)
}

// Catalogs retrieves all the catalog's catalogs with an executor.
func (o *Container) Catalogs(exec boil.Executor, mods ...qm.QueryMod) catalogQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.InnerJoin("\"catalogs_containers\" as \"b\" on \"a\".\"id\" = \"b\".\"catalog_id\""),
		qm.Where("\"b\".\"container_id\"=?", o.ID),
	)

	query := Catalogs(exec, queryMods...)
	queries.SetFrom(query.Query, "\"catalogs\" as \"a\"")
	return query
}

// LabelsG retrieves all the label's labels.
func (o *Container) LabelsG(mods ...qm.QueryMod) labelQuery {
	return o.Labels(boil.GetDB(), mods...)
}

// Labels retrieves all the label's labels with an executor.
func (o *Container) Labels(exec boil.Executor, mods ...qm.QueryMod) labelQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.InnerJoin("\"containers_labels\" as \"b\" on \"a\".\"id\" = \"b\".\"label_id\""),
		qm.Where("\"b\".\"container_id\"=?", o.ID),
	)

	query := Labels(exec, queryMods...)
	queries.SetFrom(query.Query, "\"labels\" as \"a\"")
	return query
}

// FileAssetsG retrieves all the file_asset's file assets.
func (o *Container) FileAssetsG(mods ...qm.QueryMod) fileAssetQuery {
	return o.FileAssets(boil.GetDB(), mods...)
}

// FileAssets retrieves all the file_asset's file assets with an executor.
func (o *Container) FileAssets(exec boil.Executor, mods ...qm.QueryMod) fileAssetQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.InnerJoin("\"containers_file_assets\" as \"b\" on \"a\".\"id\" = \"b\".\"file_asset_id\""),
		qm.Where("\"b\".\"container_id\"=?", o.ID),
	)

	query := FileAssets(exec, queryMods...)
	queries.SetFrom(query.Query, "\"file_assets\" as \"a\"")
	return query
}

// ContainerDescriptionsG retrieves all the container_description's container descriptions.
func (o *Container) ContainerDescriptionsG(mods ...qm.QueryMod) containerDescriptionQuery {
	return o.ContainerDescriptions(boil.GetDB(), mods...)
}

// ContainerDescriptions retrieves all the container_description's container descriptions with an executor.
func (o *Container) ContainerDescriptions(exec boil.Executor, mods ...qm.QueryMod) containerDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"container_id\"=?", o.ID),
	)

	query := ContainerDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"container_descriptions\" as \"a\"")
	return query
}

// LoadLecturer allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadLecturer(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.LecturerID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
			}
			args[i] = obj.LecturerID
		}
	}

	query := fmt.Sprintf(
		"select * from \"lecturers\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Lecturer")
	}
	defer results.Close()

	var resultSlice []*Lecturer
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Lecturer")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Lecturer = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.LecturerID.Int == foreign.ID {
				local.R.Lecturer = foreign
				break
			}
		}
	}

	return nil
}

// LoadContentType allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadContentType(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.ContentTypeID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
			}
			args[i] = obj.ContentTypeID
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
		object.R.ContentType = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ContentTypeID.Int == foreign.ID {
				local.R.ContentType = foreign
				break
			}
		}
	}

	return nil
}

// LoadVirtualLesson allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadVirtualLesson(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.VirtualLessonID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
			}
			args[i] = obj.VirtualLessonID
		}
	}

	query := fmt.Sprintf(
		"select * from \"virtual_lessons\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load VirtualLesson")
	}
	defer results.Close()

	var resultSlice []*VirtualLesson
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice VirtualLesson")
	}

	if singular && len(resultSlice) != 0 {
		object.R.VirtualLesson = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.VirtualLessonID.Int == foreign.ID {
				local.R.VirtualLesson = foreign
				break
			}
		}
	}

	return nil
}

// LoadUser allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadUser(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.UserID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
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

// LoadCensor allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadCensor(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.CensorID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
			}
			args[i] = obj.CensorID
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
		object.R.Censor = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.CensorID.Int == foreign.ID {
				local.R.Censor = foreign
				break
			}
		}
	}

	return nil
}

// LoadLang allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadLang(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.LangID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
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

// LoadContainerTranscripts allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadContainerTranscripts(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"container_transcripts\" where \"container_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load container_transcripts")
	}
	defer results.Close()

	var resultSlice []*ContainerTranscript
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice container_transcripts")
	}

	if singular {
		object.R.ContainerTranscripts = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.ContainerID.Int {
				local.R.ContainerTranscripts = append(local.R.ContainerTranscripts, foreign)
				break
			}
		}
	}

	return nil
}

// LoadCatalogs allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadCatalogs(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select \"a\".*, \"b\".\"container_id\" from \"catalogs\" as \"a\" inner join \"catalogs_containers\" as \"b\" on \"a\".\"id\" = \"b\".\"catalog_id\" where \"b\".\"container_id\" in (%s)",
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

// LoadLabels allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadLabels(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select \"a\".*, \"b\".\"container_id\" from \"labels\" as \"a\" inner join \"containers_labels\" as \"b\" on \"a\".\"id\" = \"b\".\"label_id\" where \"b\".\"container_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load labels")
	}
	defer results.Close()

	var resultSlice []*Label

	var localJoinCols []int
	for results.Next() {
		one := new(Label)
		var localJoinCol int

		err = results.Scan(&one.ID, &one.DictionaryID, &one.Suid, &one.CreatedAt, &one.UpdatedAt, &localJoinCol)
		if err = results.Err(); err != nil {
			return errors.Wrap(err, "failed to plebian-bind eager loaded slice labels")
		}

		resultSlice = append(resultSlice, one)
		localJoinCols = append(localJoinCols, localJoinCol)
	}

	if err = results.Err(); err != nil {
		return errors.Wrap(err, "failed to plebian-bind eager loaded slice labels")
	}

	if singular {
		object.R.Labels = resultSlice
		return nil
	}

	for i, foreign := range resultSlice {
		localJoinCol := localJoinCols[i]
		for _, local := range slice {
			if local.ID == localJoinCol {
				local.R.Labels = append(local.R.Labels, foreign)
				break
			}
		}
	}

	return nil
}

// LoadFileAssets allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadFileAssets(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select \"a\".*, \"b\".\"container_id\" from \"file_assets\" as \"a\" inner join \"containers_file_assets\" as \"b\" on \"a\".\"id\" = \"b\".\"file_asset_id\" where \"b\".\"container_id\" in (%s)",
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

	var localJoinCols []int
	for results.Next() {
		one := new(FileAsset)
		var localJoinCol int

		err = results.Scan(&one.ID, &one.Name, &one.LangID, &one.AssetTypeID, &one.Date, &one.Size, &one.ServerNameID, &one.Status, &one.CreatedAt, &one.UpdatedAt, &one.Lastuser, &one.Clicks, &one.Secure, &one.PlaytimeSecs, &one.UserID, &localJoinCol)
		if err = results.Err(); err != nil {
			return errors.Wrap(err, "failed to plebian-bind eager loaded slice file_assets")
		}

		resultSlice = append(resultSlice, one)
		localJoinCols = append(localJoinCols, localJoinCol)
	}

	if err = results.Err(); err != nil {
		return errors.Wrap(err, "failed to plebian-bind eager loaded slice file_assets")
	}

	if singular {
		object.R.FileAssets = resultSlice
		return nil
	}

	for i, foreign := range resultSlice {
		localJoinCol := localJoinCols[i]
		for _, local := range slice {
			if local.ID == localJoinCol {
				local.R.FileAssets = append(local.R.FileAssets, foreign)
				break
			}
		}
	}

	return nil
}

// LoadContainerDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (containerL) LoadContainerDescriptions(e boil.Executor, singular bool, maybeContainer interface{}) error {
	var slice []*Container
	var object *Container

	count := 1
	if singular {
		object = maybeContainer.(*Container)
	} else {
		slice = *maybeContainer.(*ContainerSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &containerR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &containerR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"container_descriptions\" where \"container_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load container_descriptions")
	}
	defer results.Close()

	var resultSlice []*ContainerDescription
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice container_descriptions")
	}

	if singular {
		object.R.ContainerDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.ContainerID {
				local.R.ContainerDescriptions = append(local.R.ContainerDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// SetLecturer of the container to the related item.
// Sets o.R.Lecturer to related.
// Adds o to related.R.Containers.
func (o *Container) SetLecturer(exec boil.Executor, insert bool, related *Lecturer) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"containers\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"lecturer_id"}),
		strmangle.WhereClause("\"", "\"", 2, containerPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.LecturerID.Int = related.ID
	o.LecturerID.Valid = true

	if o.R == nil {
		o.R = &containerR{
			Lecturer: related,
		}
	} else {
		o.R.Lecturer = related
	}

	if related.R == nil {
		related.R = &lecturerR{
			Containers: ContainerSlice{o},
		}
	} else {
		related.R.Containers = append(related.R.Containers, o)
	}

	return nil
}

// RemoveLecturer relationship.
// Sets o.R.Lecturer to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *Container) RemoveLecturer(exec boil.Executor, related *Lecturer) error {
	var err error

	o.LecturerID.Valid = false
	if err = o.Update(exec, "lecturer_id"); err != nil {
		o.LecturerID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.Lecturer = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.Containers {
		if o.LecturerID.Int != ri.LecturerID.Int {
			continue
		}

		ln := len(related.R.Containers)
		if ln > 1 && i < ln-1 {
			related.R.Containers[i] = related.R.Containers[ln-1]
		}
		related.R.Containers = related.R.Containers[:ln-1]
		break
	}
	return nil
}

// SetContentType of the container to the related item.
// Sets o.R.ContentType to related.
// Adds o to related.R.Containers.
func (o *Container) SetContentType(exec boil.Executor, insert bool, related *ContentType) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"containers\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"content_type_id"}),
		strmangle.WhereClause("\"", "\"", 2, containerPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.ContentTypeID.Int = related.ID
	o.ContentTypeID.Valid = true

	if o.R == nil {
		o.R = &containerR{
			ContentType: related,
		}
	} else {
		o.R.ContentType = related
	}

	if related.R == nil {
		related.R = &contentTypeR{
			Containers: ContainerSlice{o},
		}
	} else {
		related.R.Containers = append(related.R.Containers, o)
	}

	return nil
}

// RemoveContentType relationship.
// Sets o.R.ContentType to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *Container) RemoveContentType(exec boil.Executor, related *ContentType) error {
	var err error

	o.ContentTypeID.Valid = false
	if err = o.Update(exec, "content_type_id"); err != nil {
		o.ContentTypeID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.ContentType = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.Containers {
		if o.ContentTypeID.Int != ri.ContentTypeID.Int {
			continue
		}

		ln := len(related.R.Containers)
		if ln > 1 && i < ln-1 {
			related.R.Containers[i] = related.R.Containers[ln-1]
		}
		related.R.Containers = related.R.Containers[:ln-1]
		break
	}
	return nil
}

// SetVirtualLesson of the container to the related item.
// Sets o.R.VirtualLesson to related.
// Adds o to related.R.Containers.
func (o *Container) SetVirtualLesson(exec boil.Executor, insert bool, related *VirtualLesson) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"containers\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"virtual_lesson_id"}),
		strmangle.WhereClause("\"", "\"", 2, containerPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.VirtualLessonID.Int = related.ID
	o.VirtualLessonID.Valid = true

	if o.R == nil {
		o.R = &containerR{
			VirtualLesson: related,
		}
	} else {
		o.R.VirtualLesson = related
	}

	if related.R == nil {
		related.R = &virtualLessonR{
			Containers: ContainerSlice{o},
		}
	} else {
		related.R.Containers = append(related.R.Containers, o)
	}

	return nil
}

// RemoveVirtualLesson relationship.
// Sets o.R.VirtualLesson to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *Container) RemoveVirtualLesson(exec boil.Executor, related *VirtualLesson) error {
	var err error

	o.VirtualLessonID.Valid = false
	if err = o.Update(exec, "virtual_lesson_id"); err != nil {
		o.VirtualLessonID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.VirtualLesson = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.Containers {
		if o.VirtualLessonID.Int != ri.VirtualLessonID.Int {
			continue
		}

		ln := len(related.R.Containers)
		if ln > 1 && i < ln-1 {
			related.R.Containers[i] = related.R.Containers[ln-1]
		}
		related.R.Containers = related.R.Containers[:ln-1]
		break
	}
	return nil
}

// SetUser of the container to the related item.
// Sets o.R.User to related.
// Adds o to related.R.Containers.
func (o *Container) SetUser(exec boil.Executor, insert bool, related *User) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"containers\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
		strmangle.WhereClause("\"", "\"", 2, containerPrimaryKeyColumns),
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
		o.R = &containerR{
			User: related,
		}
	} else {
		o.R.User = related
	}

	if related.R == nil {
		related.R = &userR{
			Containers: ContainerSlice{o},
		}
	} else {
		related.R.Containers = append(related.R.Containers, o)
	}

	return nil
}

// RemoveUser relationship.
// Sets o.R.User to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *Container) RemoveUser(exec boil.Executor, related *User) error {
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

	for i, ri := range related.R.Containers {
		if o.UserID.Int != ri.UserID.Int {
			continue
		}

		ln := len(related.R.Containers)
		if ln > 1 && i < ln-1 {
			related.R.Containers[i] = related.R.Containers[ln-1]
		}
		related.R.Containers = related.R.Containers[:ln-1]
		break
	}
	return nil
}

// SetCensor of the container to the related item.
// Sets o.R.Censor to related.
// Adds o to related.R.CensorContainers.
func (o *Container) SetCensor(exec boil.Executor, insert bool, related *User) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"containers\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"censor_id"}),
		strmangle.WhereClause("\"", "\"", 2, containerPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.CensorID.Int = related.ID
	o.CensorID.Valid = true

	if o.R == nil {
		o.R = &containerR{
			Censor: related,
		}
	} else {
		o.R.Censor = related
	}

	if related.R == nil {
		related.R = &userR{
			CensorContainers: ContainerSlice{o},
		}
	} else {
		related.R.CensorContainers = append(related.R.CensorContainers, o)
	}

	return nil
}

// RemoveCensor relationship.
// Sets o.R.Censor to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *Container) RemoveCensor(exec boil.Executor, related *User) error {
	var err error

	o.CensorID.Valid = false
	if err = o.Update(exec, "censor_id"); err != nil {
		o.CensorID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.Censor = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.CensorContainers {
		if o.CensorID.Int != ri.CensorID.Int {
			continue
		}

		ln := len(related.R.CensorContainers)
		if ln > 1 && i < ln-1 {
			related.R.CensorContainers[i] = related.R.CensorContainers[ln-1]
		}
		related.R.CensorContainers = related.R.CensorContainers[:ln-1]
		break
	}
	return nil
}

// SetLang of the container to the related item.
// Sets o.R.Lang to related.
// Adds o to related.R.LangContainers.
func (o *Container) SetLang(exec boil.Executor, insert bool, related *Language) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"containers\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
		strmangle.WhereClause("\"", "\"", 2, containerPrimaryKeyColumns),
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
		o.R = &containerR{
			Lang: related,
		}
	} else {
		o.R.Lang = related
	}

	if related.R == nil {
		related.R = &languageR{
			LangContainers: ContainerSlice{o},
		}
	} else {
		related.R.LangContainers = append(related.R.LangContainers, o)
	}

	return nil
}

// RemoveLang relationship.
// Sets o.R.Lang to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *Container) RemoveLang(exec boil.Executor, related *Language) error {
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

	for i, ri := range related.R.LangContainers {
		if o.LangID.String != ri.LangID.String {
			continue
		}

		ln := len(related.R.LangContainers)
		if ln > 1 && i < ln-1 {
			related.R.LangContainers[i] = related.R.LangContainers[ln-1]
		}
		related.R.LangContainers = related.R.LangContainers[:ln-1]
		break
	}
	return nil
}

// AddContainerTranscripts adds the given related objects to the existing relationships
// of the container, optionally inserting them as new records.
// Appends related to o.R.ContainerTranscripts.
// Sets related.R.Container appropriately.
func (o *Container) AddContainerTranscripts(exec boil.Executor, insert bool, related ...*ContainerTranscript) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.ContainerID.Int = o.ID
			rel.ContainerID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"container_transcripts\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"container_id"}),
				strmangle.WhereClause("\"", "\"", 2, containerTranscriptPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.ContainerID.Int = o.ID
			rel.ContainerID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &containerR{
			ContainerTranscripts: related,
		}
	} else {
		o.R.ContainerTranscripts = append(o.R.ContainerTranscripts, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerTranscriptR{
				Container: o,
			}
		} else {
			rel.R.Container = o
		}
	}
	return nil
}

// SetContainerTranscripts removes all previously related items of the
// container replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Container's ContainerTranscripts accordingly.
// Replaces o.R.ContainerTranscripts with related.
// Sets related.R.Container's ContainerTranscripts accordingly.
func (o *Container) SetContainerTranscripts(exec boil.Executor, insert bool, related ...*ContainerTranscript) error {
	query := "update \"container_transcripts\" set \"container_id\" = null where \"container_id\" = $1"
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
		for _, rel := range o.R.ContainerTranscripts {
			rel.ContainerID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Container = nil
		}

		o.R.ContainerTranscripts = nil
	}
	return o.AddContainerTranscripts(exec, insert, related...)
}

// RemoveContainerTranscripts relationships from objects passed in.
// Removes related items from R.ContainerTranscripts (uses pointer comparison, removal does not keep order)
// Sets related.R.Container.
func (o *Container) RemoveContainerTranscripts(exec boil.Executor, related ...*ContainerTranscript) error {
	var err error
	for _, rel := range related {
		rel.ContainerID.Valid = false
		if rel.R != nil {
			rel.R.Container = nil
		}
		if err = rel.Update(exec, "container_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.ContainerTranscripts {
			if rel != ri {
				continue
			}

			ln := len(o.R.ContainerTranscripts)
			if ln > 1 && i < ln-1 {
				o.R.ContainerTranscripts[i] = o.R.ContainerTranscripts[ln-1]
			}
			o.R.ContainerTranscripts = o.R.ContainerTranscripts[:ln-1]
			break
		}
	}

	return nil
}

// AddCatalogs adds the given related objects to the existing relationships
// of the container, optionally inserting them as new records.
// Appends related to o.R.Catalogs.
// Sets related.R.Containers appropriately.
func (o *Container) AddCatalogs(exec boil.Executor, insert bool, related ...*Catalog) error {
	var err error
	for _, rel := range related {
		if insert {
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		}
	}

	for _, rel := range related {
		query := "insert into \"catalogs_containers\" (\"container_id\", \"catalog_id\") values ($1, $2)"
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
		o.R = &containerR{
			Catalogs: related,
		}
	} else {
		o.R.Catalogs = append(o.R.Catalogs, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &catalogR{
				Containers: ContainerSlice{o},
			}
		} else {
			rel.R.Containers = append(rel.R.Containers, o)
		}
	}
	return nil
}

// SetCatalogs removes all previously related items of the
// container replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Containers's Catalogs accordingly.
// Replaces o.R.Catalogs with related.
// Sets related.R.Containers's Catalogs accordingly.
func (o *Container) SetCatalogs(exec boil.Executor, insert bool, related ...*Catalog) error {
	query := "delete from \"catalogs_containers\" where \"container_id\" = $1"
	values := []interface{}{o.ID}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	removeCatalogsFromContainersSlice(o, related)
	o.R.Catalogs = nil
	return o.AddCatalogs(exec, insert, related...)
}

// RemoveCatalogs relationships from objects passed in.
// Removes related items from R.Catalogs (uses pointer comparison, removal does not keep order)
// Sets related.R.Containers.
func (o *Container) RemoveCatalogs(exec boil.Executor, related ...*Catalog) error {
	var err error
	query := fmt.Sprintf(
		"delete from \"catalogs_containers\" where \"container_id\" = $1 and \"catalog_id\" in (%s)",
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
	removeCatalogsFromContainersSlice(o, related)
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

func removeCatalogsFromContainersSlice(o *Container, related []*Catalog) {
	for _, rel := range related {
		if rel.R == nil {
			continue
		}
		for i, ri := range rel.R.Containers {
			if o.ID != ri.ID {
				continue
			}

			ln := len(rel.R.Containers)
			if ln > 1 && i < ln-1 {
				rel.R.Containers[i] = rel.R.Containers[ln-1]
			}
			rel.R.Containers = rel.R.Containers[:ln-1]
			break
		}
	}
}

// AddLabels adds the given related objects to the existing relationships
// of the container, optionally inserting them as new records.
// Appends related to o.R.Labels.
// Sets related.R.Containers appropriately.
func (o *Container) AddLabels(exec boil.Executor, insert bool, related ...*Label) error {
	var err error
	for _, rel := range related {
		if insert {
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		}
	}

	for _, rel := range related {
		query := "insert into \"containers_labels\" (\"container_id\", \"label_id\") values ($1, $2)"
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
		o.R = &containerR{
			Labels: related,
		}
	} else {
		o.R.Labels = append(o.R.Labels, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &labelR{
				Containers: ContainerSlice{o},
			}
		} else {
			rel.R.Containers = append(rel.R.Containers, o)
		}
	}
	return nil
}

// SetLabels removes all previously related items of the
// container replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Containers's Labels accordingly.
// Replaces o.R.Labels with related.
// Sets related.R.Containers's Labels accordingly.
func (o *Container) SetLabels(exec boil.Executor, insert bool, related ...*Label) error {
	query := "delete from \"containers_labels\" where \"container_id\" = $1"
	values := []interface{}{o.ID}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	removeLabelsFromContainersSlice(o, related)
	o.R.Labels = nil
	return o.AddLabels(exec, insert, related...)
}

// RemoveLabels relationships from objects passed in.
// Removes related items from R.Labels (uses pointer comparison, removal does not keep order)
// Sets related.R.Containers.
func (o *Container) RemoveLabels(exec boil.Executor, related ...*Label) error {
	var err error
	query := fmt.Sprintf(
		"delete from \"containers_labels\" where \"container_id\" = $1 and \"label_id\" in (%s)",
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
	removeLabelsFromContainersSlice(o, related)
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.Labels {
			if rel != ri {
				continue
			}

			ln := len(o.R.Labels)
			if ln > 1 && i < ln-1 {
				o.R.Labels[i] = o.R.Labels[ln-1]
			}
			o.R.Labels = o.R.Labels[:ln-1]
			break
		}
	}

	return nil
}

func removeLabelsFromContainersSlice(o *Container, related []*Label) {
	for _, rel := range related {
		if rel.R == nil {
			continue
		}
		for i, ri := range rel.R.Containers {
			if o.ID != ri.ID {
				continue
			}

			ln := len(rel.R.Containers)
			if ln > 1 && i < ln-1 {
				rel.R.Containers[i] = rel.R.Containers[ln-1]
			}
			rel.R.Containers = rel.R.Containers[:ln-1]
			break
		}
	}
}

// AddFileAssets adds the given related objects to the existing relationships
// of the container, optionally inserting them as new records.
// Appends related to o.R.FileAssets.
// Sets related.R.Containers appropriately.
func (o *Container) AddFileAssets(exec boil.Executor, insert bool, related ...*FileAsset) error {
	var err error
	for _, rel := range related {
		if insert {
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		}
	}

	for _, rel := range related {
		query := "insert into \"containers_file_assets\" (\"container_id\", \"file_asset_id\") values ($1, $2)"
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
		o.R = &containerR{
			FileAssets: related,
		}
	} else {
		o.R.FileAssets = append(o.R.FileAssets, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &fileAssetR{
				Containers: ContainerSlice{o},
			}
		} else {
			rel.R.Containers = append(rel.R.Containers, o)
		}
	}
	return nil
}

// SetFileAssets removes all previously related items of the
// container replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Containers's FileAssets accordingly.
// Replaces o.R.FileAssets with related.
// Sets related.R.Containers's FileAssets accordingly.
func (o *Container) SetFileAssets(exec boil.Executor, insert bool, related ...*FileAsset) error {
	query := "delete from \"containers_file_assets\" where \"container_id\" = $1"
	values := []interface{}{o.ID}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	removeFileAssetsFromContainersSlice(o, related)
	o.R.FileAssets = nil
	return o.AddFileAssets(exec, insert, related...)
}

// RemoveFileAssets relationships from objects passed in.
// Removes related items from R.FileAssets (uses pointer comparison, removal does not keep order)
// Sets related.R.Containers.
func (o *Container) RemoveFileAssets(exec boil.Executor, related ...*FileAsset) error {
	var err error
	query := fmt.Sprintf(
		"delete from \"containers_file_assets\" where \"container_id\" = $1 and \"file_asset_id\" in (%s)",
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
	removeFileAssetsFromContainersSlice(o, related)
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.FileAssets {
			if rel != ri {
				continue
			}

			ln := len(o.R.FileAssets)
			if ln > 1 && i < ln-1 {
				o.R.FileAssets[i] = o.R.FileAssets[ln-1]
			}
			o.R.FileAssets = o.R.FileAssets[:ln-1]
			break
		}
	}

	return nil
}

func removeFileAssetsFromContainersSlice(o *Container, related []*FileAsset) {
	for _, rel := range related {
		if rel.R == nil {
			continue
		}
		for i, ri := range rel.R.Containers {
			if o.ID != ri.ID {
				continue
			}

			ln := len(rel.R.Containers)
			if ln > 1 && i < ln-1 {
				rel.R.Containers[i] = rel.R.Containers[ln-1]
			}
			rel.R.Containers = rel.R.Containers[:ln-1]
			break
		}
	}
}

// AddContainerDescriptions adds the given related objects to the existing relationships
// of the container, optionally inserting them as new records.
// Appends related to o.R.ContainerDescriptions.
// Sets related.R.Container appropriately.
func (o *Container) AddContainerDescriptions(exec boil.Executor, insert bool, related ...*ContainerDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.ContainerID = o.ID
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"container_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"container_id"}),
				strmangle.WhereClause("\"", "\"", 2, containerDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.ContainerID = o.ID
		}
	}

	if o.R == nil {
		o.R = &containerR{
			ContainerDescriptions: related,
		}
	} else {
		o.R.ContainerDescriptions = append(o.R.ContainerDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerDescriptionR{
				Container: o,
			}
		} else {
			rel.R.Container = o
		}
	}
	return nil
}

// ContainersG retrieves all records.
func ContainersG(mods ...qm.QueryMod) containerQuery {
	return Containers(boil.GetDB(), mods...)
}

// Containers retrieves all the records using an executor.
func Containers(exec boil.Executor, mods ...qm.QueryMod) containerQuery {
	mods = append(mods, qm.From("\"containers\""))
	return containerQuery{NewQuery(exec, mods...)}
}

// FindContainerG retrieves a single record by ID.
func FindContainerG(id int, selectCols ...string) (*Container, error) {
	return FindContainer(boil.GetDB(), id, selectCols...)
}

// FindContainerGP retrieves a single record by ID, and panics on error.
func FindContainerGP(id int, selectCols ...string) *Container {
	retobj, err := FindContainer(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindContainer retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindContainer(exec boil.Executor, id int, selectCols ...string) (*Container, error) {
	containerObj := &Container{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"containers\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(containerObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from containers")
	}

	return containerObj, nil
}

// FindContainerP retrieves a single record by ID with an executor, and panics on error.
func FindContainerP(exec boil.Executor, id int, selectCols ...string) *Container {
	retobj, err := FindContainer(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *Container) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *Container) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *Container) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *Container) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no containers provided for insertion")
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

	nzDefaults := queries.NonZeroDefaultSet(containerColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	containerInsertCacheMut.RLock()
	cache, cached := containerInsertCache[key]
	containerInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			containerColumns,
			containerColumnsWithDefault,
			containerColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(containerType, containerMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(containerType, containerMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"containers\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into containers")
	}

	if !cached {
		containerInsertCacheMut.Lock()
		containerInsertCache[key] = cache
		containerInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single Container record. See Update for
// whitelist behavior description.
func (o *Container) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single Container record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *Container) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the Container, and panics on error.
// See Update for whitelist behavior description.
func (o *Container) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the Container.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *Container) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	var err error
	key := makeCacheKey(whitelist, nil)
	containerUpdateCacheMut.RLock()
	cache, cached := containerUpdateCache[key]
	containerUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(containerColumns, containerPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update containers, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"containers\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, containerPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(containerType, containerMapping, append(wl, containerPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update containers row")
	}

	if !cached {
		containerUpdateCacheMut.Lock()
		containerUpdateCache[key] = cache
		containerUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q containerQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q containerQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for containers")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ContainerSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o ContainerSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o ContainerSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ContainerSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), containerPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"containers\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(containerPrimaryKeyColumns), len(colNames)+1, len(containerPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in container slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *Container) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *Container) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *Container) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *Container) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no containers provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.Time.IsZero() {
		o.CreatedAt.Time = currTime
		o.CreatedAt.Valid = true
	}
	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	nzDefaults := queries.NonZeroDefaultSet(containerColumnsWithDefault, o)

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

	containerUpsertCacheMut.RLock()
	cache, cached := containerUpsertCache[key]
	containerUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			containerColumns,
			containerColumnsWithDefault,
			containerColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			containerColumns,
			containerPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert containers, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(containerPrimaryKeyColumns))
			copy(conflict, containerPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"containers\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(containerType, containerMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(containerType, containerMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert containers")
	}

	if !cached {
		containerUpsertCacheMut.Lock()
		containerUpsertCache[key] = cache
		containerUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single Container record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Container) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single Container record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *Container) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no Container provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single Container record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Container) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single Container record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Container) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Container provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), containerPrimaryKeyMapping)
	sql := "DELETE FROM \"containers\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from containers")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q containerQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q containerQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no containerQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from containers")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o ContainerSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o ContainerSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no Container slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o ContainerSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ContainerSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Container slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), containerPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"containers\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, containerPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(containerPrimaryKeyColumns), 1, len(containerPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from container slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *Container) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *Container) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *Container) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no Container provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Container) Reload(exec boil.Executor) error {
	ret, err := FindContainer(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContainerSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ContainerSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContainerSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty ContainerSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ContainerSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	containers := ContainerSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), containerPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"containers\".* FROM \"containers\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, containerPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(containerPrimaryKeyColumns), 1, len(containerPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&containers)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in ContainerSlice")
	}

	*o = containers

	return nil
}

// ContainerExists checks if the Container row exists.
func ContainerExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"containers\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if containers exists")
	}

	return exists, nil
}

// ContainerExistsG checks if the Container row exists.
func ContainerExistsG(id int) (bool, error) {
	return ContainerExists(boil.GetDB(), id)
}

// ContainerExistsGP checks if the Container row exists. Panics on error.
func ContainerExistsGP(id int) bool {
	e, err := ContainerExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// ContainerExistsP checks if the Container row exists. Panics on error.
func ContainerExistsP(exec boil.Executor, id int) bool {
	e, err := ContainerExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

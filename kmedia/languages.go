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

// Language is an object representing the database table.
type Language struct {
	ID       int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	Locale   null.String `boil:"locale" json:"locale,omitempty" toml:"locale" yaml:"locale,omitempty"`
	Code3    null.String `boil:"code3" json:"code3,omitempty" toml:"code3" yaml:"code3,omitempty"`
	Language null.String `boil:"language" json:"language,omitempty" toml:"language" yaml:"language,omitempty"`

	R *languageR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L languageL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// languageR is where relationships are stored.
type languageR struct {
	LangDictionaryDescriptions       DictionaryDescriptionSlice
	LangContainerDescriptionPatterns ContainerDescriptionPatternSlice
	LangContainerTranscripts         ContainerTranscriptSlice
	LangLabelDescriptions            LabelDescriptionSlice
	LangLecturerDescriptions         LecturerDescriptionSlice
	LangFileAssets                   FileAssetSlice
	LangFileAssetDescriptions        FileAssetDescriptionSlice
	LangCatalogDescriptions          CatalogDescriptionSlice
	LangContainers                   ContainerSlice
	LangContainerDescriptions        ContainerDescriptionSlice
}

// languageL is where Load methods for each relationship are stored.
type languageL struct{}

var (
	languageColumns               = []string{"id", "locale", "code3", "language"}
	languageColumnsWithoutDefault = []string{"locale", "code3", "language"}
	languageColumnsWithDefault    = []string{"id"}
	languagePrimaryKeyColumns     = []string{"id"}
)

type (
	// LanguageSlice is an alias for a slice of pointers to Language.
	// This should generally be used opposed to []Language.
	LanguageSlice []*Language

	languageQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	languageType                 = reflect.TypeOf(&Language{})
	languageMapping              = queries.MakeStructMapping(languageType)
	languagePrimaryKeyMapping, _ = queries.BindMapping(languageType, languageMapping, languagePrimaryKeyColumns)
	languageInsertCacheMut       sync.RWMutex
	languageInsertCache          = make(map[string]insertCache)
	languageUpdateCacheMut       sync.RWMutex
	languageUpdateCache          = make(map[string]updateCache)
	languageUpsertCacheMut       sync.RWMutex
	languageUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single language record from the query, and panics on error.
func (q languageQuery) OneP() *Language {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single language record from the query.
func (q languageQuery) One() (*Language, error) {
	o := &Language{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for languages")
	}

	return o, nil
}

// AllP returns all Language records from the query, and panics on error.
func (q languageQuery) AllP() LanguageSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all Language records from the query.
func (q languageQuery) All() (LanguageSlice, error) {
	var o LanguageSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to Language slice")
	}

	return o, nil
}

// CountP returns the count of all Language records in the query, and panics on error.
func (q languageQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all Language records in the query.
func (q languageQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count languages rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q languageQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q languageQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if languages exists")
	}

	return count > 0, nil
}

// LangDictionaryDescriptionsG retrieves all the dictionary_description's dictionary descriptions via lang_id column.
func (o *Language) LangDictionaryDescriptionsG(mods ...qm.QueryMod) dictionaryDescriptionQuery {
	return o.LangDictionaryDescriptions(boil.GetDB(), mods...)
}

// LangDictionaryDescriptions retrieves all the dictionary_description's dictionary descriptions with an executor via lang_id column.
func (o *Language) LangDictionaryDescriptions(exec boil.Executor, mods ...qm.QueryMod) dictionaryDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lang_id\"=?", o.Code3),
	)

	query := DictionaryDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"dictionary_descriptions\" as \"a\"")
	return query
}

// LangContainerDescriptionPatternsG retrieves all the container_description_pattern's container description patterns via lang_id column.
func (o *Language) LangContainerDescriptionPatternsG(mods ...qm.QueryMod) containerDescriptionPatternQuery {
	return o.LangContainerDescriptionPatterns(boil.GetDB(), mods...)
}

// LangContainerDescriptionPatterns retrieves all the container_description_pattern's container description patterns with an executor via lang_id column.
func (o *Language) LangContainerDescriptionPatterns(exec boil.Executor, mods ...qm.QueryMod) containerDescriptionPatternQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lang_id\"=?", o.Code3),
	)

	query := ContainerDescriptionPatterns(exec, queryMods...)
	queries.SetFrom(query.Query, "\"container_description_patterns\" as \"a\"")
	return query
}

// LangContainerTranscriptsG retrieves all the container_transcript's container transcripts via lang_id column.
func (o *Language) LangContainerTranscriptsG(mods ...qm.QueryMod) containerTranscriptQuery {
	return o.LangContainerTranscripts(boil.GetDB(), mods...)
}

// LangContainerTranscripts retrieves all the container_transcript's container transcripts with an executor via lang_id column.
func (o *Language) LangContainerTranscripts(exec boil.Executor, mods ...qm.QueryMod) containerTranscriptQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lang_id\"=?", o.Code3),
	)

	query := ContainerTranscripts(exec, queryMods...)
	queries.SetFrom(query.Query, "\"container_transcripts\" as \"a\"")
	return query
}

// LangLabelDescriptionsG retrieves all the label_description's label descriptions via lang_id column.
func (o *Language) LangLabelDescriptionsG(mods ...qm.QueryMod) labelDescriptionQuery {
	return o.LangLabelDescriptions(boil.GetDB(), mods...)
}

// LangLabelDescriptions retrieves all the label_description's label descriptions with an executor via lang_id column.
func (o *Language) LangLabelDescriptions(exec boil.Executor, mods ...qm.QueryMod) labelDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lang_id\"=?", o.Code3),
	)

	query := LabelDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"label_descriptions\" as \"a\"")
	return query
}

// LangLecturerDescriptionsG retrieves all the lecturer_description's lecturer descriptions via lang_id column.
func (o *Language) LangLecturerDescriptionsG(mods ...qm.QueryMod) lecturerDescriptionQuery {
	return o.LangLecturerDescriptions(boil.GetDB(), mods...)
}

// LangLecturerDescriptions retrieves all the lecturer_description's lecturer descriptions with an executor via lang_id column.
func (o *Language) LangLecturerDescriptions(exec boil.Executor, mods ...qm.QueryMod) lecturerDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lang_id\"=?", o.Code3),
	)

	query := LecturerDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"lecturer_descriptions\" as \"a\"")
	return query
}

// LangFileAssetsG retrieves all the file_asset's file assets via lang_id column.
func (o *Language) LangFileAssetsG(mods ...qm.QueryMod) fileAssetQuery {
	return o.LangFileAssets(boil.GetDB(), mods...)
}

// LangFileAssets retrieves all the file_asset's file assets with an executor via lang_id column.
func (o *Language) LangFileAssets(exec boil.Executor, mods ...qm.QueryMod) fileAssetQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lang_id\"=?", o.Code3),
	)

	query := FileAssets(exec, queryMods...)
	queries.SetFrom(query.Query, "\"file_assets\" as \"a\"")
	return query
}

// LangFileAssetDescriptionsG retrieves all the file_asset_description's file asset descriptions via lang_id column.
func (o *Language) LangFileAssetDescriptionsG(mods ...qm.QueryMod) fileAssetDescriptionQuery {
	return o.LangFileAssetDescriptions(boil.GetDB(), mods...)
}

// LangFileAssetDescriptions retrieves all the file_asset_description's file asset descriptions with an executor via lang_id column.
func (o *Language) LangFileAssetDescriptions(exec boil.Executor, mods ...qm.QueryMod) fileAssetDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lang_id\"=?", o.Code3),
	)

	query := FileAssetDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"file_asset_descriptions\" as \"a\"")
	return query
}

// LangCatalogDescriptionsG retrieves all the catalog_description's catalog descriptions via lang_id column.
func (o *Language) LangCatalogDescriptionsG(mods ...qm.QueryMod) catalogDescriptionQuery {
	return o.LangCatalogDescriptions(boil.GetDB(), mods...)
}

// LangCatalogDescriptions retrieves all the catalog_description's catalog descriptions with an executor via lang_id column.
func (o *Language) LangCatalogDescriptions(exec boil.Executor, mods ...qm.QueryMod) catalogDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lang_id\"=?", o.Code3),
	)

	query := CatalogDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"catalog_descriptions\" as \"a\"")
	return query
}

// LangContainersG retrieves all the container's containers via lang_id column.
func (o *Language) LangContainersG(mods ...qm.QueryMod) containerQuery {
	return o.LangContainers(boil.GetDB(), mods...)
}

// LangContainers retrieves all the container's containers with an executor via lang_id column.
func (o *Language) LangContainers(exec boil.Executor, mods ...qm.QueryMod) containerQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lang_id\"=?", o.Code3),
	)

	query := Containers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"containers\" as \"a\"")
	return query
}

// LangContainerDescriptionsG retrieves all the container_description's container descriptions via lang_id column.
func (o *Language) LangContainerDescriptionsG(mods ...qm.QueryMod) containerDescriptionQuery {
	return o.LangContainerDescriptions(boil.GetDB(), mods...)
}

// LangContainerDescriptions retrieves all the container_description's container descriptions with an executor via lang_id column.
func (o *Language) LangContainerDescriptions(exec boil.Executor, mods ...qm.QueryMod) containerDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"lang_id\"=?", o.Code3),
	)

	query := ContainerDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"container_descriptions\" as \"a\"")
	return query
}

// LoadLangDictionaryDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (languageL) LoadLangDictionaryDescriptions(e boil.Executor, singular bool, maybeLanguage interface{}) error {
	var slice []*Language
	var object *Language

	count := 1
	if singular {
		object = maybeLanguage.(*Language)
	} else {
		slice = *maybeLanguage.(*LanguageSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &languageR{}
		}
		args[0] = object.Code3
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &languageR{}
			}
			args[i] = obj.Code3
		}
	}

	query := fmt.Sprintf(
		"select * from \"dictionary_descriptions\" where \"lang_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load dictionary_descriptions")
	}
	defer results.Close()

	var resultSlice []*DictionaryDescription
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice dictionary_descriptions")
	}

	if singular {
		object.R.LangDictionaryDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Code3.String == foreign.LangID.String {
				local.R.LangDictionaryDescriptions = append(local.R.LangDictionaryDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// LoadLangContainerDescriptionPatterns allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (languageL) LoadLangContainerDescriptionPatterns(e boil.Executor, singular bool, maybeLanguage interface{}) error {
	var slice []*Language
	var object *Language

	count := 1
	if singular {
		object = maybeLanguage.(*Language)
	} else {
		slice = *maybeLanguage.(*LanguageSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &languageR{}
		}
		args[0] = object.Code3
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &languageR{}
			}
			args[i] = obj.Code3
		}
	}

	query := fmt.Sprintf(
		"select * from \"container_description_patterns\" where \"lang_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load container_description_patterns")
	}
	defer results.Close()

	var resultSlice []*ContainerDescriptionPattern
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice container_description_patterns")
	}

	if singular {
		object.R.LangContainerDescriptionPatterns = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Code3.String == foreign.LangID.String {
				local.R.LangContainerDescriptionPatterns = append(local.R.LangContainerDescriptionPatterns, foreign)
				break
			}
		}
	}

	return nil
}

// LoadLangContainerTranscripts allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (languageL) LoadLangContainerTranscripts(e boil.Executor, singular bool, maybeLanguage interface{}) error {
	var slice []*Language
	var object *Language

	count := 1
	if singular {
		object = maybeLanguage.(*Language)
	} else {
		slice = *maybeLanguage.(*LanguageSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &languageR{}
		}
		args[0] = object.Code3
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &languageR{}
			}
			args[i] = obj.Code3
		}
	}

	query := fmt.Sprintf(
		"select * from \"container_transcripts\" where \"lang_id\" in (%s)",
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
		object.R.LangContainerTranscripts = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Code3.String == foreign.LangID.String {
				local.R.LangContainerTranscripts = append(local.R.LangContainerTranscripts, foreign)
				break
			}
		}
	}

	return nil
}

// LoadLangLabelDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (languageL) LoadLangLabelDescriptions(e boil.Executor, singular bool, maybeLanguage interface{}) error {
	var slice []*Language
	var object *Language

	count := 1
	if singular {
		object = maybeLanguage.(*Language)
	} else {
		slice = *maybeLanguage.(*LanguageSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &languageR{}
		}
		args[0] = object.Code3
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &languageR{}
			}
			args[i] = obj.Code3
		}
	}

	query := fmt.Sprintf(
		"select * from \"label_descriptions\" where \"lang_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load label_descriptions")
	}
	defer results.Close()

	var resultSlice []*LabelDescription
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice label_descriptions")
	}

	if singular {
		object.R.LangLabelDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Code3.String == foreign.LangID.String {
				local.R.LangLabelDescriptions = append(local.R.LangLabelDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// LoadLangLecturerDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (languageL) LoadLangLecturerDescriptions(e boil.Executor, singular bool, maybeLanguage interface{}) error {
	var slice []*Language
	var object *Language

	count := 1
	if singular {
		object = maybeLanguage.(*Language)
	} else {
		slice = *maybeLanguage.(*LanguageSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &languageR{}
		}
		args[0] = object.Code3
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &languageR{}
			}
			args[i] = obj.Code3
		}
	}

	query := fmt.Sprintf(
		"select * from \"lecturer_descriptions\" where \"lang_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load lecturer_descriptions")
	}
	defer results.Close()

	var resultSlice []*LecturerDescription
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice lecturer_descriptions")
	}

	if singular {
		object.R.LangLecturerDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Code3.String == foreign.LangID {
				local.R.LangLecturerDescriptions = append(local.R.LangLecturerDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// LoadLangFileAssets allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (languageL) LoadLangFileAssets(e boil.Executor, singular bool, maybeLanguage interface{}) error {
	var slice []*Language
	var object *Language

	count := 1
	if singular {
		object = maybeLanguage.(*Language)
	} else {
		slice = *maybeLanguage.(*LanguageSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &languageR{}
		}
		args[0] = object.Code3
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &languageR{}
			}
			args[i] = obj.Code3
		}
	}

	query := fmt.Sprintf(
		"select * from \"file_assets\" where \"lang_id\" in (%s)",
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
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice file_assets")
	}

	if singular {
		object.R.LangFileAssets = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Code3.String == foreign.LangID.String {
				local.R.LangFileAssets = append(local.R.LangFileAssets, foreign)
				break
			}
		}
	}

	return nil
}

// LoadLangFileAssetDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (languageL) LoadLangFileAssetDescriptions(e boil.Executor, singular bool, maybeLanguage interface{}) error {
	var slice []*Language
	var object *Language

	count := 1
	if singular {
		object = maybeLanguage.(*Language)
	} else {
		slice = *maybeLanguage.(*LanguageSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &languageR{}
		}
		args[0] = object.Code3
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &languageR{}
			}
			args[i] = obj.Code3
		}
	}

	query := fmt.Sprintf(
		"select * from \"file_asset_descriptions\" where \"lang_id\" in (%s)",
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
		object.R.LangFileAssetDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Code3.String == foreign.LangID.String {
				local.R.LangFileAssetDescriptions = append(local.R.LangFileAssetDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// LoadLangCatalogDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (languageL) LoadLangCatalogDescriptions(e boil.Executor, singular bool, maybeLanguage interface{}) error {
	var slice []*Language
	var object *Language

	count := 1
	if singular {
		object = maybeLanguage.(*Language)
	} else {
		slice = *maybeLanguage.(*LanguageSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &languageR{}
		}
		args[0] = object.Code3
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &languageR{}
			}
			args[i] = obj.Code3
		}
	}

	query := fmt.Sprintf(
		"select * from \"catalog_descriptions\" where \"lang_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load catalog_descriptions")
	}
	defer results.Close()

	var resultSlice []*CatalogDescription
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice catalog_descriptions")
	}

	if singular {
		object.R.LangCatalogDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Code3.String == foreign.LangID.String {
				local.R.LangCatalogDescriptions = append(local.R.LangCatalogDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// LoadLangContainers allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (languageL) LoadLangContainers(e boil.Executor, singular bool, maybeLanguage interface{}) error {
	var slice []*Language
	var object *Language

	count := 1
	if singular {
		object = maybeLanguage.(*Language)
	} else {
		slice = *maybeLanguage.(*LanguageSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &languageR{}
		}
		args[0] = object.Code3
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &languageR{}
			}
			args[i] = obj.Code3
		}
	}

	query := fmt.Sprintf(
		"select * from \"containers\" where \"lang_id\" in (%s)",
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
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice containers")
	}

	if singular {
		object.R.LangContainers = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Code3.String == foreign.LangID.String {
				local.R.LangContainers = append(local.R.LangContainers, foreign)
				break
			}
		}
	}

	return nil
}

// LoadLangContainerDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (languageL) LoadLangContainerDescriptions(e boil.Executor, singular bool, maybeLanguage interface{}) error {
	var slice []*Language
	var object *Language

	count := 1
	if singular {
		object = maybeLanguage.(*Language)
	} else {
		slice = *maybeLanguage.(*LanguageSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &languageR{}
		}
		args[0] = object.Code3
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &languageR{}
			}
			args[i] = obj.Code3
		}
	}

	query := fmt.Sprintf(
		"select * from \"container_descriptions\" where \"lang_id\" in (%s)",
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
		object.R.LangContainerDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.Code3.String == foreign.LangID.String {
				local.R.LangContainerDescriptions = append(local.R.LangContainerDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// AddLangDictionaryDescriptions adds the given related objects to the existing relationships
// of the language, optionally inserting them as new records.
// Appends related to o.R.LangDictionaryDescriptions.
// Sets related.R.Lang appropriately.
func (o *Language) AddLangDictionaryDescriptions(exec boil.Executor, insert bool, related ...*DictionaryDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"dictionary_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
				strmangle.WhereClause("\"", "\"", 2, dictionaryDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.Code3, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &languageR{
			LangDictionaryDescriptions: related,
		}
	} else {
		o.R.LangDictionaryDescriptions = append(o.R.LangDictionaryDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &dictionaryDescriptionR{
				Lang: o,
			}
		} else {
			rel.R.Lang = o
		}
	}
	return nil
}

// SetLangDictionaryDescriptions removes all previously related items of the
// language replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Lang's LangDictionaryDescriptions accordingly.
// Replaces o.R.LangDictionaryDescriptions with related.
// Sets related.R.Lang's LangDictionaryDescriptions accordingly.
func (o *Language) SetLangDictionaryDescriptions(exec boil.Executor, insert bool, related ...*DictionaryDescription) error {
	query := "update \"dictionary_descriptions\" set \"lang_id\" = null where \"lang_id\" = $1"
	values := []interface{}{o.Code3}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.LangDictionaryDescriptions {
			rel.LangID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Lang = nil
		}

		o.R.LangDictionaryDescriptions = nil
	}
	return o.AddLangDictionaryDescriptions(exec, insert, related...)
}

// RemoveLangDictionaryDescriptions relationships from objects passed in.
// Removes related items from R.LangDictionaryDescriptions (uses pointer comparison, removal does not keep order)
// Sets related.R.Lang.
func (o *Language) RemoveLangDictionaryDescriptions(exec boil.Executor, related ...*DictionaryDescription) error {
	var err error
	for _, rel := range related {
		rel.LangID.Valid = false
		if rel.R != nil {
			rel.R.Lang = nil
		}
		if err = rel.Update(exec, "lang_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.LangDictionaryDescriptions {
			if rel != ri {
				continue
			}

			ln := len(o.R.LangDictionaryDescriptions)
			if ln > 1 && i < ln-1 {
				o.R.LangDictionaryDescriptions[i] = o.R.LangDictionaryDescriptions[ln-1]
			}
			o.R.LangDictionaryDescriptions = o.R.LangDictionaryDescriptions[:ln-1]
			break
		}
	}

	return nil
}

// AddLangContainerDescriptionPatterns adds the given related objects to the existing relationships
// of the language, optionally inserting them as new records.
// Appends related to o.R.LangContainerDescriptionPatterns.
// Sets related.R.Lang appropriately.
func (o *Language) AddLangContainerDescriptionPatterns(exec boil.Executor, insert bool, related ...*ContainerDescriptionPattern) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"container_description_patterns\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
				strmangle.WhereClause("\"", "\"", 2, containerDescriptionPatternPrimaryKeyColumns),
			)
			values := []interface{}{o.Code3, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &languageR{
			LangContainerDescriptionPatterns: related,
		}
	} else {
		o.R.LangContainerDescriptionPatterns = append(o.R.LangContainerDescriptionPatterns, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerDescriptionPatternR{
				Lang: o,
			}
		} else {
			rel.R.Lang = o
		}
	}
	return nil
}

// SetLangContainerDescriptionPatterns removes all previously related items of the
// language replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Lang's LangContainerDescriptionPatterns accordingly.
// Replaces o.R.LangContainerDescriptionPatterns with related.
// Sets related.R.Lang's LangContainerDescriptionPatterns accordingly.
func (o *Language) SetLangContainerDescriptionPatterns(exec boil.Executor, insert bool, related ...*ContainerDescriptionPattern) error {
	query := "update \"container_description_patterns\" set \"lang_id\" = null where \"lang_id\" = $1"
	values := []interface{}{o.Code3}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.LangContainerDescriptionPatterns {
			rel.LangID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Lang = nil
		}

		o.R.LangContainerDescriptionPatterns = nil
	}
	return o.AddLangContainerDescriptionPatterns(exec, insert, related...)
}

// RemoveLangContainerDescriptionPatterns relationships from objects passed in.
// Removes related items from R.LangContainerDescriptionPatterns (uses pointer comparison, removal does not keep order)
// Sets related.R.Lang.
func (o *Language) RemoveLangContainerDescriptionPatterns(exec boil.Executor, related ...*ContainerDescriptionPattern) error {
	var err error
	for _, rel := range related {
		rel.LangID.Valid = false
		if rel.R != nil {
			rel.R.Lang = nil
		}
		if err = rel.Update(exec, "lang_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.LangContainerDescriptionPatterns {
			if rel != ri {
				continue
			}

			ln := len(o.R.LangContainerDescriptionPatterns)
			if ln > 1 && i < ln-1 {
				o.R.LangContainerDescriptionPatterns[i] = o.R.LangContainerDescriptionPatterns[ln-1]
			}
			o.R.LangContainerDescriptionPatterns = o.R.LangContainerDescriptionPatterns[:ln-1]
			break
		}
	}

	return nil
}

// AddLangContainerTranscripts adds the given related objects to the existing relationships
// of the language, optionally inserting them as new records.
// Appends related to o.R.LangContainerTranscripts.
// Sets related.R.Lang appropriately.
func (o *Language) AddLangContainerTranscripts(exec boil.Executor, insert bool, related ...*ContainerTranscript) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"container_transcripts\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
				strmangle.WhereClause("\"", "\"", 2, containerTranscriptPrimaryKeyColumns),
			)
			values := []interface{}{o.Code3, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &languageR{
			LangContainerTranscripts: related,
		}
	} else {
		o.R.LangContainerTranscripts = append(o.R.LangContainerTranscripts, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerTranscriptR{
				Lang: o,
			}
		} else {
			rel.R.Lang = o
		}
	}
	return nil
}

// SetLangContainerTranscripts removes all previously related items of the
// language replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Lang's LangContainerTranscripts accordingly.
// Replaces o.R.LangContainerTranscripts with related.
// Sets related.R.Lang's LangContainerTranscripts accordingly.
func (o *Language) SetLangContainerTranscripts(exec boil.Executor, insert bool, related ...*ContainerTranscript) error {
	query := "update \"container_transcripts\" set \"lang_id\" = null where \"lang_id\" = $1"
	values := []interface{}{o.Code3}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.LangContainerTranscripts {
			rel.LangID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Lang = nil
		}

		o.R.LangContainerTranscripts = nil
	}
	return o.AddLangContainerTranscripts(exec, insert, related...)
}

// RemoveLangContainerTranscripts relationships from objects passed in.
// Removes related items from R.LangContainerTranscripts (uses pointer comparison, removal does not keep order)
// Sets related.R.Lang.
func (o *Language) RemoveLangContainerTranscripts(exec boil.Executor, related ...*ContainerTranscript) error {
	var err error
	for _, rel := range related {
		rel.LangID.Valid = false
		if rel.R != nil {
			rel.R.Lang = nil
		}
		if err = rel.Update(exec, "lang_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.LangContainerTranscripts {
			if rel != ri {
				continue
			}

			ln := len(o.R.LangContainerTranscripts)
			if ln > 1 && i < ln-1 {
				o.R.LangContainerTranscripts[i] = o.R.LangContainerTranscripts[ln-1]
			}
			o.R.LangContainerTranscripts = o.R.LangContainerTranscripts[:ln-1]
			break
		}
	}

	return nil
}

// AddLangLabelDescriptions adds the given related objects to the existing relationships
// of the language, optionally inserting them as new records.
// Appends related to o.R.LangLabelDescriptions.
// Sets related.R.Lang appropriately.
func (o *Language) AddLangLabelDescriptions(exec boil.Executor, insert bool, related ...*LabelDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"label_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
				strmangle.WhereClause("\"", "\"", 2, labelDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.Code3, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &languageR{
			LangLabelDescriptions: related,
		}
	} else {
		o.R.LangLabelDescriptions = append(o.R.LangLabelDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &labelDescriptionR{
				Lang: o,
			}
		} else {
			rel.R.Lang = o
		}
	}
	return nil
}

// SetLangLabelDescriptions removes all previously related items of the
// language replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Lang's LangLabelDescriptions accordingly.
// Replaces o.R.LangLabelDescriptions with related.
// Sets related.R.Lang's LangLabelDescriptions accordingly.
func (o *Language) SetLangLabelDescriptions(exec boil.Executor, insert bool, related ...*LabelDescription) error {
	query := "update \"label_descriptions\" set \"lang_id\" = null where \"lang_id\" = $1"
	values := []interface{}{o.Code3}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.LangLabelDescriptions {
			rel.LangID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Lang = nil
		}

		o.R.LangLabelDescriptions = nil
	}
	return o.AddLangLabelDescriptions(exec, insert, related...)
}

// RemoveLangLabelDescriptions relationships from objects passed in.
// Removes related items from R.LangLabelDescriptions (uses pointer comparison, removal does not keep order)
// Sets related.R.Lang.
func (o *Language) RemoveLangLabelDescriptions(exec boil.Executor, related ...*LabelDescription) error {
	var err error
	for _, rel := range related {
		rel.LangID.Valid = false
		if rel.R != nil {
			rel.R.Lang = nil
		}
		if err = rel.Update(exec, "lang_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.LangLabelDescriptions {
			if rel != ri {
				continue
			}

			ln := len(o.R.LangLabelDescriptions)
			if ln > 1 && i < ln-1 {
				o.R.LangLabelDescriptions[i] = o.R.LangLabelDescriptions[ln-1]
			}
			o.R.LangLabelDescriptions = o.R.LangLabelDescriptions[:ln-1]
			break
		}
	}

	return nil
}

// AddLangLecturerDescriptions adds the given related objects to the existing relationships
// of the language, optionally inserting them as new records.
// Appends related to o.R.LangLecturerDescriptions.
// Sets related.R.Lang appropriately.
func (o *Language) AddLangLecturerDescriptions(exec boil.Executor, insert bool, related ...*LecturerDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LangID = o.Code3.String
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"lecturer_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
				strmangle.WhereClause("\"", "\"", 2, lecturerDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.Code3, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LangID = o.Code3.String
		}
	}

	if o.R == nil {
		o.R = &languageR{
			LangLecturerDescriptions: related,
		}
	} else {
		o.R.LangLecturerDescriptions = append(o.R.LangLecturerDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &lecturerDescriptionR{
				Lang: o,
			}
		} else {
			rel.R.Lang = o
		}
	}
	return nil
}

// AddLangFileAssets adds the given related objects to the existing relationships
// of the language, optionally inserting them as new records.
// Appends related to o.R.LangFileAssets.
// Sets related.R.Lang appropriately.
func (o *Language) AddLangFileAssets(exec boil.Executor, insert bool, related ...*FileAsset) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"file_assets\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
				strmangle.WhereClause("\"", "\"", 2, fileAssetPrimaryKeyColumns),
			)
			values := []interface{}{o.Code3, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &languageR{
			LangFileAssets: related,
		}
	} else {
		o.R.LangFileAssets = append(o.R.LangFileAssets, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &fileAssetR{
				Lang: o,
			}
		} else {
			rel.R.Lang = o
		}
	}
	return nil
}

// SetLangFileAssets removes all previously related items of the
// language replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Lang's LangFileAssets accordingly.
// Replaces o.R.LangFileAssets with related.
// Sets related.R.Lang's LangFileAssets accordingly.
func (o *Language) SetLangFileAssets(exec boil.Executor, insert bool, related ...*FileAsset) error {
	query := "update \"file_assets\" set \"lang_id\" = null where \"lang_id\" = $1"
	values := []interface{}{o.Code3}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.LangFileAssets {
			rel.LangID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Lang = nil
		}

		o.R.LangFileAssets = nil
	}
	return o.AddLangFileAssets(exec, insert, related...)
}

// RemoveLangFileAssets relationships from objects passed in.
// Removes related items from R.LangFileAssets (uses pointer comparison, removal does not keep order)
// Sets related.R.Lang.
func (o *Language) RemoveLangFileAssets(exec boil.Executor, related ...*FileAsset) error {
	var err error
	for _, rel := range related {
		rel.LangID.Valid = false
		if rel.R != nil {
			rel.R.Lang = nil
		}
		if err = rel.Update(exec, "lang_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.LangFileAssets {
			if rel != ri {
				continue
			}

			ln := len(o.R.LangFileAssets)
			if ln > 1 && i < ln-1 {
				o.R.LangFileAssets[i] = o.R.LangFileAssets[ln-1]
			}
			o.R.LangFileAssets = o.R.LangFileAssets[:ln-1]
			break
		}
	}

	return nil
}

// AddLangFileAssetDescriptions adds the given related objects to the existing relationships
// of the language, optionally inserting them as new records.
// Appends related to o.R.LangFileAssetDescriptions.
// Sets related.R.Lang appropriately.
func (o *Language) AddLangFileAssetDescriptions(exec boil.Executor, insert bool, related ...*FileAssetDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"file_asset_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
				strmangle.WhereClause("\"", "\"", 2, fileAssetDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.Code3, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &languageR{
			LangFileAssetDescriptions: related,
		}
	} else {
		o.R.LangFileAssetDescriptions = append(o.R.LangFileAssetDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &fileAssetDescriptionR{
				Lang: o,
			}
		} else {
			rel.R.Lang = o
		}
	}
	return nil
}

// SetLangFileAssetDescriptions removes all previously related items of the
// language replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Lang's LangFileAssetDescriptions accordingly.
// Replaces o.R.LangFileAssetDescriptions with related.
// Sets related.R.Lang's LangFileAssetDescriptions accordingly.
func (o *Language) SetLangFileAssetDescriptions(exec boil.Executor, insert bool, related ...*FileAssetDescription) error {
	query := "update \"file_asset_descriptions\" set \"lang_id\" = null where \"lang_id\" = $1"
	values := []interface{}{o.Code3}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.LangFileAssetDescriptions {
			rel.LangID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Lang = nil
		}

		o.R.LangFileAssetDescriptions = nil
	}
	return o.AddLangFileAssetDescriptions(exec, insert, related...)
}

// RemoveLangFileAssetDescriptions relationships from objects passed in.
// Removes related items from R.LangFileAssetDescriptions (uses pointer comparison, removal does not keep order)
// Sets related.R.Lang.
func (o *Language) RemoveLangFileAssetDescriptions(exec boil.Executor, related ...*FileAssetDescription) error {
	var err error
	for _, rel := range related {
		rel.LangID.Valid = false
		if rel.R != nil {
			rel.R.Lang = nil
		}
		if err = rel.Update(exec, "lang_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.LangFileAssetDescriptions {
			if rel != ri {
				continue
			}

			ln := len(o.R.LangFileAssetDescriptions)
			if ln > 1 && i < ln-1 {
				o.R.LangFileAssetDescriptions[i] = o.R.LangFileAssetDescriptions[ln-1]
			}
			o.R.LangFileAssetDescriptions = o.R.LangFileAssetDescriptions[:ln-1]
			break
		}
	}

	return nil
}

// AddLangCatalogDescriptions adds the given related objects to the existing relationships
// of the language, optionally inserting them as new records.
// Appends related to o.R.LangCatalogDescriptions.
// Sets related.R.Lang appropriately.
func (o *Language) AddLangCatalogDescriptions(exec boil.Executor, insert bool, related ...*CatalogDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"catalog_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
				strmangle.WhereClause("\"", "\"", 2, catalogDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.Code3, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &languageR{
			LangCatalogDescriptions: related,
		}
	} else {
		o.R.LangCatalogDescriptions = append(o.R.LangCatalogDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &catalogDescriptionR{
				Lang: o,
			}
		} else {
			rel.R.Lang = o
		}
	}
	return nil
}

// SetLangCatalogDescriptions removes all previously related items of the
// language replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Lang's LangCatalogDescriptions accordingly.
// Replaces o.R.LangCatalogDescriptions with related.
// Sets related.R.Lang's LangCatalogDescriptions accordingly.
func (o *Language) SetLangCatalogDescriptions(exec boil.Executor, insert bool, related ...*CatalogDescription) error {
	query := "update \"catalog_descriptions\" set \"lang_id\" = null where \"lang_id\" = $1"
	values := []interface{}{o.Code3}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.LangCatalogDescriptions {
			rel.LangID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Lang = nil
		}

		o.R.LangCatalogDescriptions = nil
	}
	return o.AddLangCatalogDescriptions(exec, insert, related...)
}

// RemoveLangCatalogDescriptions relationships from objects passed in.
// Removes related items from R.LangCatalogDescriptions (uses pointer comparison, removal does not keep order)
// Sets related.R.Lang.
func (o *Language) RemoveLangCatalogDescriptions(exec boil.Executor, related ...*CatalogDescription) error {
	var err error
	for _, rel := range related {
		rel.LangID.Valid = false
		if rel.R != nil {
			rel.R.Lang = nil
		}
		if err = rel.Update(exec, "lang_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.LangCatalogDescriptions {
			if rel != ri {
				continue
			}

			ln := len(o.R.LangCatalogDescriptions)
			if ln > 1 && i < ln-1 {
				o.R.LangCatalogDescriptions[i] = o.R.LangCatalogDescriptions[ln-1]
			}
			o.R.LangCatalogDescriptions = o.R.LangCatalogDescriptions[:ln-1]
			break
		}
	}

	return nil
}

// AddLangContainers adds the given related objects to the existing relationships
// of the language, optionally inserting them as new records.
// Appends related to o.R.LangContainers.
// Sets related.R.Lang appropriately.
func (o *Language) AddLangContainers(exec boil.Executor, insert bool, related ...*Container) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"containers\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
				strmangle.WhereClause("\"", "\"", 2, containerPrimaryKeyColumns),
			)
			values := []interface{}{o.Code3, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &languageR{
			LangContainers: related,
		}
	} else {
		o.R.LangContainers = append(o.R.LangContainers, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerR{
				Lang: o,
			}
		} else {
			rel.R.Lang = o
		}
	}
	return nil
}

// SetLangContainers removes all previously related items of the
// language replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Lang's LangContainers accordingly.
// Replaces o.R.LangContainers with related.
// Sets related.R.Lang's LangContainers accordingly.
func (o *Language) SetLangContainers(exec boil.Executor, insert bool, related ...*Container) error {
	query := "update \"containers\" set \"lang_id\" = null where \"lang_id\" = $1"
	values := []interface{}{o.Code3}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.LangContainers {
			rel.LangID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Lang = nil
		}

		o.R.LangContainers = nil
	}
	return o.AddLangContainers(exec, insert, related...)
}

// RemoveLangContainers relationships from objects passed in.
// Removes related items from R.LangContainers (uses pointer comparison, removal does not keep order)
// Sets related.R.Lang.
func (o *Language) RemoveLangContainers(exec boil.Executor, related ...*Container) error {
	var err error
	for _, rel := range related {
		rel.LangID.Valid = false
		if rel.R != nil {
			rel.R.Lang = nil
		}
		if err = rel.Update(exec, "lang_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.LangContainers {
			if rel != ri {
				continue
			}

			ln := len(o.R.LangContainers)
			if ln > 1 && i < ln-1 {
				o.R.LangContainers[i] = o.R.LangContainers[ln-1]
			}
			o.R.LangContainers = o.R.LangContainers[:ln-1]
			break
		}
	}

	return nil
}

// AddLangContainerDescriptions adds the given related objects to the existing relationships
// of the language, optionally inserting them as new records.
// Appends related to o.R.LangContainerDescriptions.
// Sets related.R.Lang appropriately.
func (o *Language) AddLangContainerDescriptions(exec boil.Executor, insert bool, related ...*ContainerDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"container_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"lang_id"}),
				strmangle.WhereClause("\"", "\"", 2, containerDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.Code3, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LangID.String = o.Code3.String
			rel.LangID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &languageR{
			LangContainerDescriptions: related,
		}
	} else {
		o.R.LangContainerDescriptions = append(o.R.LangContainerDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerDescriptionR{
				Lang: o,
			}
		} else {
			rel.R.Lang = o
		}
	}
	return nil
}

// SetLangContainerDescriptions removes all previously related items of the
// language replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Lang's LangContainerDescriptions accordingly.
// Replaces o.R.LangContainerDescriptions with related.
// Sets related.R.Lang's LangContainerDescriptions accordingly.
func (o *Language) SetLangContainerDescriptions(exec boil.Executor, insert bool, related ...*ContainerDescription) error {
	query := "update \"container_descriptions\" set \"lang_id\" = null where \"lang_id\" = $1"
	values := []interface{}{o.Code3}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.LangContainerDescriptions {
			rel.LangID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Lang = nil
		}

		o.R.LangContainerDescriptions = nil
	}
	return o.AddLangContainerDescriptions(exec, insert, related...)
}

// RemoveLangContainerDescriptions relationships from objects passed in.
// Removes related items from R.LangContainerDescriptions (uses pointer comparison, removal does not keep order)
// Sets related.R.Lang.
func (o *Language) RemoveLangContainerDescriptions(exec boil.Executor, related ...*ContainerDescription) error {
	var err error
	for _, rel := range related {
		rel.LangID.Valid = false
		if rel.R != nil {
			rel.R.Lang = nil
		}
		if err = rel.Update(exec, "lang_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.LangContainerDescriptions {
			if rel != ri {
				continue
			}

			ln := len(o.R.LangContainerDescriptions)
			if ln > 1 && i < ln-1 {
				o.R.LangContainerDescriptions[i] = o.R.LangContainerDescriptions[ln-1]
			}
			o.R.LangContainerDescriptions = o.R.LangContainerDescriptions[:ln-1]
			break
		}
	}

	return nil
}

// LanguagesG retrieves all records.
func LanguagesG(mods ...qm.QueryMod) languageQuery {
	return Languages(boil.GetDB(), mods...)
}

// Languages retrieves all the records using an executor.
func Languages(exec boil.Executor, mods ...qm.QueryMod) languageQuery {
	mods = append(mods, qm.From("\"languages\""))
	return languageQuery{NewQuery(exec, mods...)}
}

// FindLanguageG retrieves a single record by ID.
func FindLanguageG(id int, selectCols ...string) (*Language, error) {
	return FindLanguage(boil.GetDB(), id, selectCols...)
}

// FindLanguageGP retrieves a single record by ID, and panics on error.
func FindLanguageGP(id int, selectCols ...string) *Language {
	retobj, err := FindLanguage(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindLanguage retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindLanguage(exec boil.Executor, id int, selectCols ...string) (*Language, error) {
	languageObj := &Language{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"languages\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(languageObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from languages")
	}

	return languageObj, nil
}

// FindLanguageP retrieves a single record by ID with an executor, and panics on error.
func FindLanguageP(exec boil.Executor, id int, selectCols ...string) *Language {
	retobj, err := FindLanguage(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *Language) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *Language) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *Language) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *Language) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no languages provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(languageColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	languageInsertCacheMut.RLock()
	cache, cached := languageInsertCache[key]
	languageInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			languageColumns,
			languageColumnsWithDefault,
			languageColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(languageType, languageMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(languageType, languageMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"languages\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into languages")
	}

	if !cached {
		languageInsertCacheMut.Lock()
		languageInsertCache[key] = cache
		languageInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single Language record. See Update for
// whitelist behavior description.
func (o *Language) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single Language record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *Language) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the Language, and panics on error.
// See Update for whitelist behavior description.
func (o *Language) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the Language.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *Language) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	languageUpdateCacheMut.RLock()
	cache, cached := languageUpdateCache[key]
	languageUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(languageColumns, languagePrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update languages, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"languages\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, languagePrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(languageType, languageMapping, append(wl, languagePrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update languages row")
	}

	if !cached {
		languageUpdateCacheMut.Lock()
		languageUpdateCache[key] = cache
		languageUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q languageQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q languageQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for languages")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o LanguageSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o LanguageSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o LanguageSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o LanguageSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), languagePrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"languages\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(languagePrimaryKeyColumns), len(colNames)+1, len(languagePrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in language slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *Language) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *Language) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *Language) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *Language) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no languages provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(languageColumnsWithDefault, o)

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

	languageUpsertCacheMut.RLock()
	cache, cached := languageUpsertCache[key]
	languageUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			languageColumns,
			languageColumnsWithDefault,
			languageColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			languageColumns,
			languagePrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert languages, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(languagePrimaryKeyColumns))
			copy(conflict, languagePrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"languages\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(languageType, languageMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(languageType, languageMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert languages")
	}

	if !cached {
		languageUpsertCacheMut.Lock()
		languageUpsertCache[key] = cache
		languageUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single Language record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Language) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single Language record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *Language) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no Language provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single Language record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Language) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single Language record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Language) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Language provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), languagePrimaryKeyMapping)
	sql := "DELETE FROM \"languages\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from languages")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q languageQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q languageQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no languageQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from languages")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o LanguageSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o LanguageSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no Language slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o LanguageSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o LanguageSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Language slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), languagePrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"languages\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, languagePrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(languagePrimaryKeyColumns), 1, len(languagePrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from language slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *Language) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *Language) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *Language) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no Language provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Language) Reload(exec boil.Executor) error {
	ret, err := FindLanguage(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *LanguageSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *LanguageSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *LanguageSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty LanguageSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *LanguageSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	languages := LanguageSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), languagePrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"languages\".* FROM \"languages\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, languagePrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(languagePrimaryKeyColumns), 1, len(languagePrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&languages)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in LanguageSlice")
	}

	*o = languages

	return nil
}

// LanguageExists checks if the Language row exists.
func LanguageExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"languages\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if languages exists")
	}

	return exists, nil
}

// LanguageExistsG checks if the Language row exists.
func LanguageExistsG(id int) (bool, error) {
	return LanguageExists(boil.GetDB(), id)
}

// LanguageExistsGP checks if the Language row exists. Panics on error.
func LanguageExistsGP(id int) bool {
	e, err := LanguageExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// LanguageExistsP checks if the Language row exists. Panics on error.
func LanguageExistsP(exec boil.Executor, id int) bool {
	e, err := LanguageExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

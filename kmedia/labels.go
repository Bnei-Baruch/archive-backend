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

// Label is an object representing the database table.
type Label struct {
	ID           int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	DictionaryID null.Int    `boil:"dictionary_id" json:"dictionary_id,omitempty" toml:"dictionary_id" yaml:"dictionary_id,omitempty"`
	Suid         null.String `boil:"suid" json:"suid,omitempty" toml:"suid" yaml:"suid,omitempty"`
	CreatedAt    time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	UpdatedAt    time.Time   `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`

	R *labelR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L labelL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// labelR is where relationships are stored.
type labelR struct {
	Containers        ContainerSlice
	LabelDescriptions LabelDescriptionSlice
}

// labelL is where Load methods for each relationship are stored.
type labelL struct{}

var (
	labelColumns               = []string{"id", "dictionary_id", "suid", "created_at", "updated_at"}
	labelColumnsWithoutDefault = []string{"dictionary_id", "suid", "created_at", "updated_at"}
	labelColumnsWithDefault    = []string{"id"}
	labelPrimaryKeyColumns     = []string{"id"}
)

type (
	// LabelSlice is an alias for a slice of pointers to Label.
	// This should generally be used opposed to []Label.
	LabelSlice []*Label

	labelQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	labelType                 = reflect.TypeOf(&Label{})
	labelMapping              = queries.MakeStructMapping(labelType)
	labelPrimaryKeyMapping, _ = queries.BindMapping(labelType, labelMapping, labelPrimaryKeyColumns)
	labelInsertCacheMut       sync.RWMutex
	labelInsertCache          = make(map[string]insertCache)
	labelUpdateCacheMut       sync.RWMutex
	labelUpdateCache          = make(map[string]updateCache)
	labelUpsertCacheMut       sync.RWMutex
	labelUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single label record from the query, and panics on error.
func (q labelQuery) OneP() *Label {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single label record from the query.
func (q labelQuery) One() (*Label, error) {
	o := &Label{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for labels")
	}

	return o, nil
}

// AllP returns all Label records from the query, and panics on error.
func (q labelQuery) AllP() LabelSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all Label records from the query.
func (q labelQuery) All() (LabelSlice, error) {
	var o LabelSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to Label slice")
	}

	return o, nil
}

// CountP returns the count of all Label records in the query, and panics on error.
func (q labelQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all Label records in the query.
func (q labelQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count labels rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q labelQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q labelQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if labels exists")
	}

	return count > 0, nil
}

// ContainersG retrieves all the container's containers.
func (o *Label) ContainersG(mods ...qm.QueryMod) containerQuery {
	return o.Containers(boil.GetDB(), mods...)
}

// Containers retrieves all the container's containers with an executor.
func (o *Label) Containers(exec boil.Executor, mods ...qm.QueryMod) containerQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.InnerJoin("\"containers_labels\" as \"b\" on \"a\".\"id\" = \"b\".\"container_id\""),
		qm.Where("\"b\".\"label_id\"=?", o.ID),
	)

	query := Containers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"containers\" as \"a\"")
	return query
}

// LabelDescriptionsG retrieves all the label_description's label descriptions.
func (o *Label) LabelDescriptionsG(mods ...qm.QueryMod) labelDescriptionQuery {
	return o.LabelDescriptions(boil.GetDB(), mods...)
}

// LabelDescriptions retrieves all the label_description's label descriptions with an executor.
func (o *Label) LabelDescriptions(exec boil.Executor, mods ...qm.QueryMod) labelDescriptionQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"label_id\"=?", o.ID),
	)

	query := LabelDescriptions(exec, queryMods...)
	queries.SetFrom(query.Query, "\"label_descriptions\" as \"a\"")
	return query
}

// LoadContainers allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (labelL) LoadContainers(e boil.Executor, singular bool, maybeLabel interface{}) error {
	var slice []*Label
	var object *Label

	count := 1
	if singular {
		object = maybeLabel.(*Label)
	} else {
		slice = *maybeLabel.(*LabelSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &labelR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &labelR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select \"a\".*, \"b\".\"label_id\" from \"containers\" as \"a\" inner join \"containers_labels\" as \"b\" on \"a\".\"id\" = \"b\".\"container_id\" where \"b\".\"label_id\" in (%s)",
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

// LoadLabelDescriptions allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (labelL) LoadLabelDescriptions(e boil.Executor, singular bool, maybeLabel interface{}) error {
	var slice []*Label
	var object *Label

	count := 1
	if singular {
		object = maybeLabel.(*Label)
	} else {
		slice = *maybeLabel.(*LabelSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &labelR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &labelR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"label_descriptions\" where \"label_id\" in (%s)",
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
		object.R.LabelDescriptions = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.LabelID.Int {
				local.R.LabelDescriptions = append(local.R.LabelDescriptions, foreign)
				break
			}
		}
	}

	return nil
}

// AddContainers adds the given related objects to the existing relationships
// of the label, optionally inserting them as new records.
// Appends related to o.R.Containers.
// Sets related.R.Labels appropriately.
func (o *Label) AddContainers(exec boil.Executor, insert bool, related ...*Container) error {
	var err error
	for _, rel := range related {
		if insert {
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		}
	}

	for _, rel := range related {
		query := "insert into \"containers_labels\" (\"label_id\", \"container_id\") values ($1, $2)"
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
		o.R = &labelR{
			Containers: related,
		}
	} else {
		o.R.Containers = append(o.R.Containers, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerR{
				Labels: LabelSlice{o},
			}
		} else {
			rel.R.Labels = append(rel.R.Labels, o)
		}
	}
	return nil
}

// SetContainers removes all previously related items of the
// label replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Labels's Containers accordingly.
// Replaces o.R.Containers with related.
// Sets related.R.Labels's Containers accordingly.
func (o *Label) SetContainers(exec boil.Executor, insert bool, related ...*Container) error {
	query := "delete from \"containers_labels\" where \"label_id\" = $1"
	values := []interface{}{o.ID}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	removeContainersFromLabelsSlice(o, related)
	o.R.Containers = nil
	return o.AddContainers(exec, insert, related...)
}

// RemoveContainers relationships from objects passed in.
// Removes related items from R.Containers (uses pointer comparison, removal does not keep order)
// Sets related.R.Labels.
func (o *Label) RemoveContainers(exec boil.Executor, related ...*Container) error {
	var err error
	query := fmt.Sprintf(
		"delete from \"containers_labels\" where \"label_id\" = $1 and \"container_id\" in (%s)",
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
	removeContainersFromLabelsSlice(o, related)
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

func removeContainersFromLabelsSlice(o *Label, related []*Container) {
	for _, rel := range related {
		if rel.R == nil {
			continue
		}
		for i, ri := range rel.R.Labels {
			if o.ID != ri.ID {
				continue
			}

			ln := len(rel.R.Labels)
			if ln > 1 && i < ln-1 {
				rel.R.Labels[i] = rel.R.Labels[ln-1]
			}
			rel.R.Labels = rel.R.Labels[:ln-1]
			break
		}
	}
}

// AddLabelDescriptions adds the given related objects to the existing relationships
// of the label, optionally inserting them as new records.
// Appends related to o.R.LabelDescriptions.
// Sets related.R.Label appropriately.
func (o *Label) AddLabelDescriptions(exec boil.Executor, insert bool, related ...*LabelDescription) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.LabelID.Int = o.ID
			rel.LabelID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"label_descriptions\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"label_id"}),
				strmangle.WhereClause("\"", "\"", 2, labelDescriptionPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.LabelID.Int = o.ID
			rel.LabelID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &labelR{
			LabelDescriptions: related,
		}
	} else {
		o.R.LabelDescriptions = append(o.R.LabelDescriptions, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &labelDescriptionR{
				Label: o,
			}
		} else {
			rel.R.Label = o
		}
	}
	return nil
}

// SetLabelDescriptions removes all previously related items of the
// label replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Label's LabelDescriptions accordingly.
// Replaces o.R.LabelDescriptions with related.
// Sets related.R.Label's LabelDescriptions accordingly.
func (o *Label) SetLabelDescriptions(exec boil.Executor, insert bool, related ...*LabelDescription) error {
	query := "update \"label_descriptions\" set \"label_id\" = null where \"label_id\" = $1"
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
		for _, rel := range o.R.LabelDescriptions {
			rel.LabelID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Label = nil
		}

		o.R.LabelDescriptions = nil
	}
	return o.AddLabelDescriptions(exec, insert, related...)
}

// RemoveLabelDescriptions relationships from objects passed in.
// Removes related items from R.LabelDescriptions (uses pointer comparison, removal does not keep order)
// Sets related.R.Label.
func (o *Label) RemoveLabelDescriptions(exec boil.Executor, related ...*LabelDescription) error {
	var err error
	for _, rel := range related {
		rel.LabelID.Valid = false
		if rel.R != nil {
			rel.R.Label = nil
		}
		if err = rel.Update(exec, "label_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.LabelDescriptions {
			if rel != ri {
				continue
			}

			ln := len(o.R.LabelDescriptions)
			if ln > 1 && i < ln-1 {
				o.R.LabelDescriptions[i] = o.R.LabelDescriptions[ln-1]
			}
			o.R.LabelDescriptions = o.R.LabelDescriptions[:ln-1]
			break
		}
	}

	return nil
}

// LabelsG retrieves all records.
func LabelsG(mods ...qm.QueryMod) labelQuery {
	return Labels(boil.GetDB(), mods...)
}

// Labels retrieves all the records using an executor.
func Labels(exec boil.Executor, mods ...qm.QueryMod) labelQuery {
	mods = append(mods, qm.From("\"labels\""))
	return labelQuery{NewQuery(exec, mods...)}
}

// FindLabelG retrieves a single record by ID.
func FindLabelG(id int, selectCols ...string) (*Label, error) {
	return FindLabel(boil.GetDB(), id, selectCols...)
}

// FindLabelGP retrieves a single record by ID, and panics on error.
func FindLabelGP(id int, selectCols ...string) *Label {
	retobj, err := FindLabel(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindLabel retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindLabel(exec boil.Executor, id int, selectCols ...string) (*Label, error) {
	labelObj := &Label{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"labels\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(labelObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from labels")
	}

	return labelObj, nil
}

// FindLabelP retrieves a single record by ID with an executor, and panics on error.
func FindLabelP(exec boil.Executor, id int, selectCols ...string) *Label {
	retobj, err := FindLabel(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *Label) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *Label) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *Label) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *Label) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no labels provided for insertion")
	}

	var err error
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = currTime
	}

	nzDefaults := queries.NonZeroDefaultSet(labelColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	labelInsertCacheMut.RLock()
	cache, cached := labelInsertCache[key]
	labelInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			labelColumns,
			labelColumnsWithDefault,
			labelColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(labelType, labelMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(labelType, labelMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"labels\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into labels")
	}

	if !cached {
		labelInsertCacheMut.Lock()
		labelInsertCache[key] = cache
		labelInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single Label record. See Update for
// whitelist behavior description.
func (o *Label) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single Label record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *Label) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the Label, and panics on error.
// See Update for whitelist behavior description.
func (o *Label) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the Label.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *Label) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt = currTime

	var err error
	key := makeCacheKey(whitelist, nil)
	labelUpdateCacheMut.RLock()
	cache, cached := labelUpdateCache[key]
	labelUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(labelColumns, labelPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update labels, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"labels\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, labelPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(labelType, labelMapping, append(wl, labelPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update labels row")
	}

	if !cached {
		labelUpdateCacheMut.Lock()
		labelUpdateCache[key] = cache
		labelUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q labelQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q labelQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for labels")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o LabelSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o LabelSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o LabelSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o LabelSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), labelPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"labels\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(labelPrimaryKeyColumns), len(colNames)+1, len(labelPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in label slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *Label) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *Label) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *Label) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *Label) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no labels provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	o.UpdatedAt = currTime

	nzDefaults := queries.NonZeroDefaultSet(labelColumnsWithDefault, o)

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

	labelUpsertCacheMut.RLock()
	cache, cached := labelUpsertCache[key]
	labelUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			labelColumns,
			labelColumnsWithDefault,
			labelColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			labelColumns,
			labelPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert labels, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(labelPrimaryKeyColumns))
			copy(conflict, labelPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"labels\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(labelType, labelMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(labelType, labelMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert labels")
	}

	if !cached {
		labelUpsertCacheMut.Lock()
		labelUpsertCache[key] = cache
		labelUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single Label record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Label) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single Label record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *Label) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no Label provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single Label record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *Label) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single Label record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Label) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Label provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), labelPrimaryKeyMapping)
	sql := "DELETE FROM \"labels\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from labels")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q labelQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q labelQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no labelQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from labels")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o LabelSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o LabelSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no Label slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o LabelSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o LabelSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no Label slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), labelPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"labels\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, labelPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(labelPrimaryKeyColumns), 1, len(labelPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from label slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *Label) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *Label) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *Label) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no Label provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Label) Reload(exec boil.Executor) error {
	ret, err := FindLabel(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *LabelSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *LabelSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *LabelSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty LabelSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *LabelSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	labels := LabelSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), labelPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"labels\".* FROM \"labels\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, labelPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(labelPrimaryKeyColumns), 1, len(labelPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&labels)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in LabelSlice")
	}

	*o = labels

	return nil
}

// LabelExists checks if the Label row exists.
func LabelExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"labels\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if labels exists")
	}

	return exists, nil
}

// LabelExistsG checks if the Label row exists.
func LabelExistsG(id int) (bool, error) {
	return LabelExists(boil.GetDB(), id)
}

// LabelExistsGP checks if the Label row exists. Panics on error.
func LabelExistsGP(id int) bool {
	e, err := LabelExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// LabelExistsP checks if the Label row exists. Panics on error.
func LabelExistsP(exec boil.Executor, id int) bool {
	e, err := LabelExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

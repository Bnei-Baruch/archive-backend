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

// User is an object representing the database table.
type User struct {
	ID                  int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	Email               string      `boil:"email" json:"email" toml:"email" yaml:"email"`
	EncryptedPassword   string      `boil:"encrypted_password" json:"encrypted_password" toml:"encrypted_password" yaml:"encrypted_password"`
	ResetPasswordToken  null.String `boil:"reset_password_token" json:"reset_password_token,omitempty" toml:"reset_password_token" yaml:"reset_password_token,omitempty"`
	RememberCreatedAt   null.Time   `boil:"remember_created_at" json:"remember_created_at,omitempty" toml:"remember_created_at" yaml:"remember_created_at,omitempty"`
	SignInCount         null.Int    `boil:"sign_in_count" json:"sign_in_count,omitempty" toml:"sign_in_count" yaml:"sign_in_count,omitempty"`
	CurrentSignInAt     null.Time   `boil:"current_sign_in_at" json:"current_sign_in_at,omitempty" toml:"current_sign_in_at" yaml:"current_sign_in_at,omitempty"`
	LastSignInAt        null.Time   `boil:"last_sign_in_at" json:"last_sign_in_at,omitempty" toml:"last_sign_in_at" yaml:"last_sign_in_at,omitempty"`
	CurrentSignInIP     null.String `boil:"current_sign_in_ip" json:"current_sign_in_ip,omitempty" toml:"current_sign_in_ip" yaml:"current_sign_in_ip,omitempty"`
	LastSignInIP        null.String `boil:"last_sign_in_ip" json:"last_sign_in_ip,omitempty" toml:"last_sign_in_ip" yaml:"last_sign_in_ip,omitempty"`
	CreatedAt           null.Time   `boil:"created_at" json:"created_at,omitempty" toml:"created_at" yaml:"created_at,omitempty"`
	UpdatedAt           null.Time   `boil:"updated_at" json:"updated_at,omitempty" toml:"updated_at" yaml:"updated_at,omitempty"`
	FirstName           null.String `boil:"first_name" json:"first_name,omitempty" toml:"first_name" yaml:"first_name,omitempty"`
	LastName            null.String `boil:"last_name" json:"last_name,omitempty" toml:"last_name" yaml:"last_name,omitempty"`
	AuthenticationToken null.String `boil:"authentication_token" json:"authentication_token,omitempty" toml:"authentication_token" yaml:"authentication_token,omitempty"`
	ResetPasswordSentAt null.Time   `boil:"reset_password_sent_at" json:"reset_password_sent_at,omitempty" toml:"reset_password_sent_at" yaml:"reset_password_sent_at,omitempty"`
	DepartmentID        null.Int    `boil:"department_id" json:"department_id,omitempty" toml:"department_id" yaml:"department_id,omitempty"`

	R *userR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L userL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// userR is where relationships are stored.
type userR struct {
	Department                   *Department
	RolesUsers                   RolesUserSlice
	ContainerDescriptionPatterns ContainerDescriptionPatternSlice
	Catalogs                     CatalogSlice
	FileAssets                   FileAssetSlice
	VirtualLessons               VirtualLessonSlice
	Containers                   ContainerSlice
	CensorContainers             ContainerSlice
}

// userL is where Load methods for each relationship are stored.
type userL struct{}

var (
	userColumns               = []string{"id", "email", "encrypted_password", "reset_password_token", "remember_created_at", "sign_in_count", "current_sign_in_at", "last_sign_in_at", "current_sign_in_ip", "last_sign_in_ip", "created_at", "updated_at", "first_name", "last_name", "authentication_token", "reset_password_sent_at", "department_id"}
	userColumnsWithoutDefault = []string{"reset_password_token", "remember_created_at", "current_sign_in_at", "last_sign_in_at", "current_sign_in_ip", "last_sign_in_ip", "created_at", "updated_at", "authentication_token", "reset_password_sent_at", "department_id"}
	userColumnsWithDefault    = []string{"id", "email", "encrypted_password", "sign_in_count", "first_name", "last_name"}
	userPrimaryKeyColumns     = []string{"id"}
)

type (
	// UserSlice is an alias for a slice of pointers to User.
	// This should generally be used opposed to []User.
	UserSlice []*User

	userQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	userType                 = reflect.TypeOf(&User{})
	userMapping              = queries.MakeStructMapping(userType)
	userPrimaryKeyMapping, _ = queries.BindMapping(userType, userMapping, userPrimaryKeyColumns)
	userInsertCacheMut       sync.RWMutex
	userInsertCache          = make(map[string]insertCache)
	userUpdateCacheMut       sync.RWMutex
	userUpdateCache          = make(map[string]updateCache)
	userUpsertCacheMut       sync.RWMutex
	userUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single user record from the query, and panics on error.
func (q userQuery) OneP() *User {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single user record from the query.
func (q userQuery) One() (*User, error) {
	o := &User{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: failed to execute a one query for users")
	}

	return o, nil
}

// AllP returns all User records from the query, and panics on error.
func (q userQuery) AllP() UserSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all User records from the query.
func (q userQuery) All() (UserSlice, error) {
	var o UserSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "kmedia: failed to assign all query results to User slice")
	}

	return o, nil
}

// CountP returns the count of all User records in the query, and panics on error.
func (q userQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all User records in the query.
func (q userQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "kmedia: failed to count users rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q userQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q userQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: failed to check if users exists")
	}

	return count > 0, nil
}

// DepartmentG pointed to by the foreign key.
func (o *User) DepartmentG(mods ...qm.QueryMod) departmentQuery {
	return o.Department(boil.GetDB(), mods...)
}

// Department pointed to by the foreign key.
func (o *User) Department(exec boil.Executor, mods ...qm.QueryMod) departmentQuery {
	queryMods := []qm.QueryMod{
		qm.Where("id=?", o.DepartmentID),
	}

	queryMods = append(queryMods, mods...)

	query := Departments(exec, queryMods...)
	queries.SetFrom(query.Query, "\"departments\"")

	return query
}

// RolesUsersG retrieves all the roles_user's roles users.
func (o *User) RolesUsersG(mods ...qm.QueryMod) rolesUserQuery {
	return o.RolesUsers(boil.GetDB(), mods...)
}

// RolesUsers retrieves all the roles_user's roles users with an executor.
func (o *User) RolesUsers(exec boil.Executor, mods ...qm.QueryMod) rolesUserQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"user_id\"=?", o.ID),
	)

	query := RolesUsers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"roles_users\" as \"a\"")
	return query
}

// ContainerDescriptionPatternsG retrieves all the container_description_pattern's container description patterns.
func (o *User) ContainerDescriptionPatternsG(mods ...qm.QueryMod) containerDescriptionPatternQuery {
	return o.ContainerDescriptionPatterns(boil.GetDB(), mods...)
}

// ContainerDescriptionPatterns retrieves all the container_description_pattern's container description patterns with an executor.
func (o *User) ContainerDescriptionPatterns(exec boil.Executor, mods ...qm.QueryMod) containerDescriptionPatternQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"user_id\"=?", o.ID),
	)

	query := ContainerDescriptionPatterns(exec, queryMods...)
	queries.SetFrom(query.Query, "\"container_description_patterns\" as \"a\"")
	return query
}

// CatalogsG retrieves all the catalog's catalogs.
func (o *User) CatalogsG(mods ...qm.QueryMod) catalogQuery {
	return o.Catalogs(boil.GetDB(), mods...)
}

// Catalogs retrieves all the catalog's catalogs with an executor.
func (o *User) Catalogs(exec boil.Executor, mods ...qm.QueryMod) catalogQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"user_id\"=?", o.ID),
	)

	query := Catalogs(exec, queryMods...)
	queries.SetFrom(query.Query, "\"catalogs\" as \"a\"")
	return query
}

// FileAssetsG retrieves all the file_asset's file assets.
func (o *User) FileAssetsG(mods ...qm.QueryMod) fileAssetQuery {
	return o.FileAssets(boil.GetDB(), mods...)
}

// FileAssets retrieves all the file_asset's file assets with an executor.
func (o *User) FileAssets(exec boil.Executor, mods ...qm.QueryMod) fileAssetQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"user_id\"=?", o.ID),
	)

	query := FileAssets(exec, queryMods...)
	queries.SetFrom(query.Query, "\"file_assets\" as \"a\"")
	return query
}

// VirtualLessonsG retrieves all the virtual_lesson's virtual lessons.
func (o *User) VirtualLessonsG(mods ...qm.QueryMod) virtualLessonQuery {
	return o.VirtualLessons(boil.GetDB(), mods...)
}

// VirtualLessons retrieves all the virtual_lesson's virtual lessons with an executor.
func (o *User) VirtualLessons(exec boil.Executor, mods ...qm.QueryMod) virtualLessonQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"user_id\"=?", o.ID),
	)

	query := VirtualLessons(exec, queryMods...)
	queries.SetFrom(query.Query, "\"virtual_lessons\" as \"a\"")
	return query
}

// ContainersG retrieves all the container's containers.
func (o *User) ContainersG(mods ...qm.QueryMod) containerQuery {
	return o.Containers(boil.GetDB(), mods...)
}

// Containers retrieves all the container's containers with an executor.
func (o *User) Containers(exec boil.Executor, mods ...qm.QueryMod) containerQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"user_id\"=?", o.ID),
	)

	query := Containers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"containers\" as \"a\"")
	return query
}

// CensorContainersG retrieves all the container's containers via censor_id column.
func (o *User) CensorContainersG(mods ...qm.QueryMod) containerQuery {
	return o.CensorContainers(boil.GetDB(), mods...)
}

// CensorContainers retrieves all the container's containers with an executor via censor_id column.
func (o *User) CensorContainers(exec boil.Executor, mods ...qm.QueryMod) containerQuery {
	queryMods := []qm.QueryMod{
		qm.Select("\"a\".*"),
	}

	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"a\".\"censor_id\"=?", o.ID),
	)

	query := Containers(exec, queryMods...)
	queries.SetFrom(query.Query, "\"containers\" as \"a\"")
	return query
}

// LoadDepartment allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (userL) LoadDepartment(e boil.Executor, singular bool, maybeUser interface{}) error {
	var slice []*User
	var object *User

	count := 1
	if singular {
		object = maybeUser.(*User)
	} else {
		slice = *maybeUser.(*UserSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &userR{}
		}
		args[0] = object.DepartmentID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &userR{}
			}
			args[i] = obj.DepartmentID
		}
	}

	query := fmt.Sprintf(
		"select * from \"departments\" where \"id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)

	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load Department")
	}
	defer results.Close()

	var resultSlice []*Department
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice Department")
	}

	if singular && len(resultSlice) != 0 {
		object.R.Department = resultSlice[0]
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.DepartmentID.Int == foreign.ID {
				local.R.Department = foreign
				break
			}
		}
	}

	return nil
}

// LoadRolesUsers allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (userL) LoadRolesUsers(e boil.Executor, singular bool, maybeUser interface{}) error {
	var slice []*User
	var object *User

	count := 1
	if singular {
		object = maybeUser.(*User)
	} else {
		slice = *maybeUser.(*UserSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &userR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &userR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"roles_users\" where \"user_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load roles_users")
	}
	defer results.Close()

	var resultSlice []*RolesUser
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice roles_users")
	}

	if singular {
		object.R.RolesUsers = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.UserID {
				local.R.RolesUsers = append(local.R.RolesUsers, foreign)
				break
			}
		}
	}

	return nil
}

// LoadContainerDescriptionPatterns allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (userL) LoadContainerDescriptionPatterns(e boil.Executor, singular bool, maybeUser interface{}) error {
	var slice []*User
	var object *User

	count := 1
	if singular {
		object = maybeUser.(*User)
	} else {
		slice = *maybeUser.(*UserSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &userR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &userR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"container_description_patterns\" where \"user_id\" in (%s)",
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
		object.R.ContainerDescriptionPatterns = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.UserID.Int {
				local.R.ContainerDescriptionPatterns = append(local.R.ContainerDescriptionPatterns, foreign)
				break
			}
		}
	}

	return nil
}

// LoadCatalogs allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (userL) LoadCatalogs(e boil.Executor, singular bool, maybeUser interface{}) error {
	var slice []*User
	var object *User

	count := 1
	if singular {
		object = maybeUser.(*User)
	} else {
		slice = *maybeUser.(*UserSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &userR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &userR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"catalogs\" where \"user_id\" in (%s)",
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
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice catalogs")
	}

	if singular {
		object.R.Catalogs = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.UserID.Int {
				local.R.Catalogs = append(local.R.Catalogs, foreign)
				break
			}
		}
	}

	return nil
}

// LoadFileAssets allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (userL) LoadFileAssets(e boil.Executor, singular bool, maybeUser interface{}) error {
	var slice []*User
	var object *User

	count := 1
	if singular {
		object = maybeUser.(*User)
	} else {
		slice = *maybeUser.(*UserSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &userR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &userR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"file_assets\" where \"user_id\" in (%s)",
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
		object.R.FileAssets = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.UserID.Int {
				local.R.FileAssets = append(local.R.FileAssets, foreign)
				break
			}
		}
	}

	return nil
}

// LoadVirtualLessons allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (userL) LoadVirtualLessons(e boil.Executor, singular bool, maybeUser interface{}) error {
	var slice []*User
	var object *User

	count := 1
	if singular {
		object = maybeUser.(*User)
	} else {
		slice = *maybeUser.(*UserSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &userR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &userR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"virtual_lessons\" where \"user_id\" in (%s)",
		strmangle.Placeholders(dialect.IndexPlaceholders, count, 1, 1),
	)
	if boil.DebugMode {
		fmt.Fprintf(boil.DebugWriter, "%s\n%v\n", query, args)
	}

	results, err := e.Query(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to eager load virtual_lessons")
	}
	defer results.Close()

	var resultSlice []*VirtualLesson
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice virtual_lessons")
	}

	if singular {
		object.R.VirtualLessons = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.UserID.Int {
				local.R.VirtualLessons = append(local.R.VirtualLessons, foreign)
				break
			}
		}
	}

	return nil
}

// LoadContainers allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (userL) LoadContainers(e boil.Executor, singular bool, maybeUser interface{}) error {
	var slice []*User
	var object *User

	count := 1
	if singular {
		object = maybeUser.(*User)
	} else {
		slice = *maybeUser.(*UserSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &userR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &userR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"containers\" where \"user_id\" in (%s)",
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
		object.R.Containers = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.UserID.Int {
				local.R.Containers = append(local.R.Containers, foreign)
				break
			}
		}
	}

	return nil
}

// LoadCensorContainers allows an eager lookup of values, cached into the
// loaded structs of the objects.
func (userL) LoadCensorContainers(e boil.Executor, singular bool, maybeUser interface{}) error {
	var slice []*User
	var object *User

	count := 1
	if singular {
		object = maybeUser.(*User)
	} else {
		slice = *maybeUser.(*UserSlice)
		count = len(slice)
	}

	args := make([]interface{}, count)
	if singular {
		if object.R == nil {
			object.R = &userR{}
		}
		args[0] = object.ID
	} else {
		for i, obj := range slice {
			if obj.R == nil {
				obj.R = &userR{}
			}
			args[i] = obj.ID
		}
	}

	query := fmt.Sprintf(
		"select * from \"containers\" where \"censor_id\" in (%s)",
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
		object.R.CensorContainers = resultSlice
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if local.ID == foreign.CensorID.Int {
				local.R.CensorContainers = append(local.R.CensorContainers, foreign)
				break
			}
		}
	}

	return nil
}

// SetDepartment of the user to the related item.
// Sets o.R.Department to related.
// Adds o to related.R.Users.
func (o *User) SetDepartment(exec boil.Executor, insert bool, related *Department) error {
	var err error
	if insert {
		if err = related.Insert(exec); err != nil {
			return errors.Wrap(err, "failed to insert into foreign table")
		}
	}

	updateQuery := fmt.Sprintf(
		"UPDATE \"users\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, []string{"department_id"}),
		strmangle.WhereClause("\"", "\"", 2, userPrimaryKeyColumns),
	)
	values := []interface{}{related.ID, o.ID}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, updateQuery)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	if _, err = exec.Exec(updateQuery, values...); err != nil {
		return errors.Wrap(err, "failed to update local table")
	}

	o.DepartmentID.Int = related.ID
	o.DepartmentID.Valid = true

	if o.R == nil {
		o.R = &userR{
			Department: related,
		}
	} else {
		o.R.Department = related
	}

	if related.R == nil {
		related.R = &departmentR{
			Users: UserSlice{o},
		}
	} else {
		related.R.Users = append(related.R.Users, o)
	}

	return nil
}

// RemoveDepartment relationship.
// Sets o.R.Department to nil.
// Removes o from all passed in related items' relationships struct (Optional).
func (o *User) RemoveDepartment(exec boil.Executor, related *Department) error {
	var err error

	o.DepartmentID.Valid = false
	if err = o.Update(exec, "department_id"); err != nil {
		o.DepartmentID.Valid = true
		return errors.Wrap(err, "failed to update local table")
	}

	o.R.Department = nil
	if related == nil || related.R == nil {
		return nil
	}

	for i, ri := range related.R.Users {
		if o.DepartmentID.Int != ri.DepartmentID.Int {
			continue
		}

		ln := len(related.R.Users)
		if ln > 1 && i < ln-1 {
			related.R.Users[i] = related.R.Users[ln-1]
		}
		related.R.Users = related.R.Users[:ln-1]
		break
	}
	return nil
}

// AddRolesUsers adds the given related objects to the existing relationships
// of the user, optionally inserting them as new records.
// Appends related to o.R.RolesUsers.
// Sets related.R.User appropriately.
func (o *User) AddRolesUsers(exec boil.Executor, insert bool, related ...*RolesUser) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.UserID = o.ID
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"roles_users\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
				strmangle.WhereClause("\"", "\"", 2, rolesUserPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.RoleID, rel.UserID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.UserID = o.ID
		}
	}

	if o.R == nil {
		o.R = &userR{
			RolesUsers: related,
		}
	} else {
		o.R.RolesUsers = append(o.R.RolesUsers, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &rolesUserR{
				User: o,
			}
		} else {
			rel.R.User = o
		}
	}
	return nil
}

// AddContainerDescriptionPatterns adds the given related objects to the existing relationships
// of the user, optionally inserting them as new records.
// Appends related to o.R.ContainerDescriptionPatterns.
// Sets related.R.User appropriately.
func (o *User) AddContainerDescriptionPatterns(exec boil.Executor, insert bool, related ...*ContainerDescriptionPattern) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.UserID.Int = o.ID
			rel.UserID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"container_description_patterns\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
				strmangle.WhereClause("\"", "\"", 2, containerDescriptionPatternPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.UserID.Int = o.ID
			rel.UserID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &userR{
			ContainerDescriptionPatterns: related,
		}
	} else {
		o.R.ContainerDescriptionPatterns = append(o.R.ContainerDescriptionPatterns, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerDescriptionPatternR{
				User: o,
			}
		} else {
			rel.R.User = o
		}
	}
	return nil
}

// SetContainerDescriptionPatterns removes all previously related items of the
// user replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.User's ContainerDescriptionPatterns accordingly.
// Replaces o.R.ContainerDescriptionPatterns with related.
// Sets related.R.User's ContainerDescriptionPatterns accordingly.
func (o *User) SetContainerDescriptionPatterns(exec boil.Executor, insert bool, related ...*ContainerDescriptionPattern) error {
	query := "update \"container_description_patterns\" set \"user_id\" = null where \"user_id\" = $1"
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
		for _, rel := range o.R.ContainerDescriptionPatterns {
			rel.UserID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.User = nil
		}

		o.R.ContainerDescriptionPatterns = nil
	}
	return o.AddContainerDescriptionPatterns(exec, insert, related...)
}

// RemoveContainerDescriptionPatterns relationships from objects passed in.
// Removes related items from R.ContainerDescriptionPatterns (uses pointer comparison, removal does not keep order)
// Sets related.R.User.
func (o *User) RemoveContainerDescriptionPatterns(exec boil.Executor, related ...*ContainerDescriptionPattern) error {
	var err error
	for _, rel := range related {
		rel.UserID.Valid = false
		if rel.R != nil {
			rel.R.User = nil
		}
		if err = rel.Update(exec, "user_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.ContainerDescriptionPatterns {
			if rel != ri {
				continue
			}

			ln := len(o.R.ContainerDescriptionPatterns)
			if ln > 1 && i < ln-1 {
				o.R.ContainerDescriptionPatterns[i] = o.R.ContainerDescriptionPatterns[ln-1]
			}
			o.R.ContainerDescriptionPatterns = o.R.ContainerDescriptionPatterns[:ln-1]
			break
		}
	}

	return nil
}

// AddCatalogs adds the given related objects to the existing relationships
// of the user, optionally inserting them as new records.
// Appends related to o.R.Catalogs.
// Sets related.R.User appropriately.
func (o *User) AddCatalogs(exec boil.Executor, insert bool, related ...*Catalog) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.UserID.Int = o.ID
			rel.UserID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"catalogs\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
				strmangle.WhereClause("\"", "\"", 2, catalogPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.UserID.Int = o.ID
			rel.UserID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &userR{
			Catalogs: related,
		}
	} else {
		o.R.Catalogs = append(o.R.Catalogs, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &catalogR{
				User: o,
			}
		} else {
			rel.R.User = o
		}
	}
	return nil
}

// SetCatalogs removes all previously related items of the
// user replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.User's Catalogs accordingly.
// Replaces o.R.Catalogs with related.
// Sets related.R.User's Catalogs accordingly.
func (o *User) SetCatalogs(exec boil.Executor, insert bool, related ...*Catalog) error {
	query := "update \"catalogs\" set \"user_id\" = null where \"user_id\" = $1"
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
		for _, rel := range o.R.Catalogs {
			rel.UserID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.User = nil
		}

		o.R.Catalogs = nil
	}
	return o.AddCatalogs(exec, insert, related...)
}

// RemoveCatalogs relationships from objects passed in.
// Removes related items from R.Catalogs (uses pointer comparison, removal does not keep order)
// Sets related.R.User.
func (o *User) RemoveCatalogs(exec boil.Executor, related ...*Catalog) error {
	var err error
	for _, rel := range related {
		rel.UserID.Valid = false
		if rel.R != nil {
			rel.R.User = nil
		}
		if err = rel.Update(exec, "user_id"); err != nil {
			return err
		}
	}
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

// AddFileAssets adds the given related objects to the existing relationships
// of the user, optionally inserting them as new records.
// Appends related to o.R.FileAssets.
// Sets related.R.User appropriately.
func (o *User) AddFileAssets(exec boil.Executor, insert bool, related ...*FileAsset) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.UserID.Int = o.ID
			rel.UserID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"file_assets\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
				strmangle.WhereClause("\"", "\"", 2, fileAssetPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.UserID.Int = o.ID
			rel.UserID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &userR{
			FileAssets: related,
		}
	} else {
		o.R.FileAssets = append(o.R.FileAssets, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &fileAssetR{
				User: o,
			}
		} else {
			rel.R.User = o
		}
	}
	return nil
}

// SetFileAssets removes all previously related items of the
// user replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.User's FileAssets accordingly.
// Replaces o.R.FileAssets with related.
// Sets related.R.User's FileAssets accordingly.
func (o *User) SetFileAssets(exec boil.Executor, insert bool, related ...*FileAsset) error {
	query := "update \"file_assets\" set \"user_id\" = null where \"user_id\" = $1"
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
		for _, rel := range o.R.FileAssets {
			rel.UserID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.User = nil
		}

		o.R.FileAssets = nil
	}
	return o.AddFileAssets(exec, insert, related...)
}

// RemoveFileAssets relationships from objects passed in.
// Removes related items from R.FileAssets (uses pointer comparison, removal does not keep order)
// Sets related.R.User.
func (o *User) RemoveFileAssets(exec boil.Executor, related ...*FileAsset) error {
	var err error
	for _, rel := range related {
		rel.UserID.Valid = false
		if rel.R != nil {
			rel.R.User = nil
		}
		if err = rel.Update(exec, "user_id"); err != nil {
			return err
		}
	}
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

// AddVirtualLessons adds the given related objects to the existing relationships
// of the user, optionally inserting them as new records.
// Appends related to o.R.VirtualLessons.
// Sets related.R.User appropriately.
func (o *User) AddVirtualLessons(exec boil.Executor, insert bool, related ...*VirtualLesson) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.UserID.Int = o.ID
			rel.UserID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"virtual_lessons\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
				strmangle.WhereClause("\"", "\"", 2, virtualLessonPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.UserID.Int = o.ID
			rel.UserID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &userR{
			VirtualLessons: related,
		}
	} else {
		o.R.VirtualLessons = append(o.R.VirtualLessons, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &virtualLessonR{
				User: o,
			}
		} else {
			rel.R.User = o
		}
	}
	return nil
}

// SetVirtualLessons removes all previously related items of the
// user replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.User's VirtualLessons accordingly.
// Replaces o.R.VirtualLessons with related.
// Sets related.R.User's VirtualLessons accordingly.
func (o *User) SetVirtualLessons(exec boil.Executor, insert bool, related ...*VirtualLesson) error {
	query := "update \"virtual_lessons\" set \"user_id\" = null where \"user_id\" = $1"
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
		for _, rel := range o.R.VirtualLessons {
			rel.UserID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.User = nil
		}

		o.R.VirtualLessons = nil
	}
	return o.AddVirtualLessons(exec, insert, related...)
}

// RemoveVirtualLessons relationships from objects passed in.
// Removes related items from R.VirtualLessons (uses pointer comparison, removal does not keep order)
// Sets related.R.User.
func (o *User) RemoveVirtualLessons(exec boil.Executor, related ...*VirtualLesson) error {
	var err error
	for _, rel := range related {
		rel.UserID.Valid = false
		if rel.R != nil {
			rel.R.User = nil
		}
		if err = rel.Update(exec, "user_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.VirtualLessons {
			if rel != ri {
				continue
			}

			ln := len(o.R.VirtualLessons)
			if ln > 1 && i < ln-1 {
				o.R.VirtualLessons[i] = o.R.VirtualLessons[ln-1]
			}
			o.R.VirtualLessons = o.R.VirtualLessons[:ln-1]
			break
		}
	}

	return nil
}

// AddContainers adds the given related objects to the existing relationships
// of the user, optionally inserting them as new records.
// Appends related to o.R.Containers.
// Sets related.R.User appropriately.
func (o *User) AddContainers(exec boil.Executor, insert bool, related ...*Container) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.UserID.Int = o.ID
			rel.UserID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"containers\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"user_id"}),
				strmangle.WhereClause("\"", "\"", 2, containerPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.UserID.Int = o.ID
			rel.UserID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &userR{
			Containers: related,
		}
	} else {
		o.R.Containers = append(o.R.Containers, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerR{
				User: o,
			}
		} else {
			rel.R.User = o
		}
	}
	return nil
}

// SetContainers removes all previously related items of the
// user replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.User's Containers accordingly.
// Replaces o.R.Containers with related.
// Sets related.R.User's Containers accordingly.
func (o *User) SetContainers(exec boil.Executor, insert bool, related ...*Container) error {
	query := "update \"containers\" set \"user_id\" = null where \"user_id\" = $1"
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
		for _, rel := range o.R.Containers {
			rel.UserID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.User = nil
		}

		o.R.Containers = nil
	}
	return o.AddContainers(exec, insert, related...)
}

// RemoveContainers relationships from objects passed in.
// Removes related items from R.Containers (uses pointer comparison, removal does not keep order)
// Sets related.R.User.
func (o *User) RemoveContainers(exec boil.Executor, related ...*Container) error {
	var err error
	for _, rel := range related {
		rel.UserID.Valid = false
		if rel.R != nil {
			rel.R.User = nil
		}
		if err = rel.Update(exec, "user_id"); err != nil {
			return err
		}
	}
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

// AddCensorContainers adds the given related objects to the existing relationships
// of the user, optionally inserting them as new records.
// Appends related to o.R.CensorContainers.
// Sets related.R.Censor appropriately.
func (o *User) AddCensorContainers(exec boil.Executor, insert bool, related ...*Container) error {
	var err error
	for _, rel := range related {
		if insert {
			rel.CensorID.Int = o.ID
			rel.CensorID.Valid = true
			if err = rel.Insert(exec); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"containers\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"censor_id"}),
				strmangle.WhereClause("\"", "\"", 2, containerPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}

			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			rel.CensorID.Int = o.ID
			rel.CensorID.Valid = true
		}
	}

	if o.R == nil {
		o.R = &userR{
			CensorContainers: related,
		}
	} else {
		o.R.CensorContainers = append(o.R.CensorContainers, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &containerR{
				Censor: o,
			}
		} else {
			rel.R.Censor = o
		}
	}
	return nil
}

// SetCensorContainers removes all previously related items of the
// user replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.Censor's CensorContainers accordingly.
// Replaces o.R.CensorContainers with related.
// Sets related.R.Censor's CensorContainers accordingly.
func (o *User) SetCensorContainers(exec boil.Executor, insert bool, related ...*Container) error {
	query := "update \"containers\" set \"censor_id\" = null where \"censor_id\" = $1"
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
		for _, rel := range o.R.CensorContainers {
			rel.CensorID.Valid = false
			if rel.R == nil {
				continue
			}

			rel.R.Censor = nil
		}

		o.R.CensorContainers = nil
	}
	return o.AddCensorContainers(exec, insert, related...)
}

// RemoveCensorContainers relationships from objects passed in.
// Removes related items from R.CensorContainers (uses pointer comparison, removal does not keep order)
// Sets related.R.Censor.
func (o *User) RemoveCensorContainers(exec boil.Executor, related ...*Container) error {
	var err error
	for _, rel := range related {
		rel.CensorID.Valid = false
		if rel.R != nil {
			rel.R.Censor = nil
		}
		if err = rel.Update(exec, "censor_id"); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.CensorContainers {
			if rel != ri {
				continue
			}

			ln := len(o.R.CensorContainers)
			if ln > 1 && i < ln-1 {
				o.R.CensorContainers[i] = o.R.CensorContainers[ln-1]
			}
			o.R.CensorContainers = o.R.CensorContainers[:ln-1]
			break
		}
	}

	return nil
}

// UsersG retrieves all records.
func UsersG(mods ...qm.QueryMod) userQuery {
	return Users(boil.GetDB(), mods...)
}

// Users retrieves all the records using an executor.
func Users(exec boil.Executor, mods ...qm.QueryMod) userQuery {
	mods = append(mods, qm.From("\"users\""))
	return userQuery{NewQuery(exec, mods...)}
}

// FindUserG retrieves a single record by ID.
func FindUserG(id int, selectCols ...string) (*User, error) {
	return FindUser(boil.GetDB(), id, selectCols...)
}

// FindUserGP retrieves a single record by ID, and panics on error.
func FindUserGP(id int, selectCols ...string) *User {
	retobj, err := FindUser(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindUser retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindUser(exec boil.Executor, id int, selectCols ...string) (*User, error) {
	userObj := &User{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"users\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(userObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "kmedia: unable to select from users")
	}

	return userObj, nil
}

// FindUserP retrieves a single record by ID with an executor, and panics on error.
func FindUserP(exec boil.Executor, id int, selectCols ...string) *User {
	retobj, err := FindUser(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *User) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *User) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *User) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *User) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no users provided for insertion")
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

	nzDefaults := queries.NonZeroDefaultSet(userColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	userInsertCacheMut.RLock()
	cache, cached := userInsertCache[key]
	userInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			userColumns,
			userColumnsWithDefault,
			userColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(userType, userMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(userType, userMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"users\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "kmedia: unable to insert into users")
	}

	if !cached {
		userInsertCacheMut.Lock()
		userInsertCache[key] = cache
		userInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single User record. See Update for
// whitelist behavior description.
func (o *User) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single User record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *User) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the User, and panics on error.
// See Update for whitelist behavior description.
func (o *User) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the User.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *User) Update(exec boil.Executor, whitelist ...string) error {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	var err error
	key := makeCacheKey(whitelist, nil)
	userUpdateCacheMut.RLock()
	cache, cached := userUpdateCache[key]
	userUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(userColumns, userPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("kmedia: unable to update users, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"users\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, userPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(userType, userMapping, append(wl, userPrimaryKeyColumns...))
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
		return errors.Wrap(err, "kmedia: unable to update users row")
	}

	if !cached {
		userUpdateCacheMut.Lock()
		userUpdateCache[key] = cache
		userUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q userQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q userQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all for users")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o UserSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o UserSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o UserSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o UserSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), userPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"users\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(userPrimaryKeyColumns), len(colNames)+1, len(userPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to update all in user slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *User) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *User) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *User) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *User) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("kmedia: no users provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.Time.IsZero() {
		o.CreatedAt.Time = currTime
		o.CreatedAt.Valid = true
	}
	o.UpdatedAt.Time = currTime
	o.UpdatedAt.Valid = true

	nzDefaults := queries.NonZeroDefaultSet(userColumnsWithDefault, o)

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

	userUpsertCacheMut.RLock()
	cache, cached := userUpsertCache[key]
	userUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			userColumns,
			userColumnsWithDefault,
			userColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			userColumns,
			userPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("kmedia: unable to upsert users, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(userPrimaryKeyColumns))
			copy(conflict, userPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"users\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(userType, userMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(userType, userMapping, ret)
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
		return errors.Wrap(err, "kmedia: unable to upsert users")
	}

	if !cached {
		userUpsertCacheMut.Lock()
		userUpsertCache[key] = cache
		userUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single User record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *User) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single User record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *User) DeleteG() error {
	if o == nil {
		return errors.New("kmedia: no User provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single User record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *User) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single User record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *User) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no User provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), userPrimaryKeyMapping)
	sql := "DELETE FROM \"users\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete from users")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q userQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q userQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("kmedia: no userQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from users")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o UserSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o UserSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("kmedia: no User slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o UserSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o UserSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("kmedia: no User slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), userPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"users\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, userPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(userPrimaryKeyColumns), 1, len(userPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to delete all from user slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *User) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *User) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *User) ReloadG() error {
	if o == nil {
		return errors.New("kmedia: no User provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *User) Reload(exec boil.Executor) error {
	ret, err := FindUser(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *UserSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *UserSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *UserSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("kmedia: empty UserSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *UserSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	users := UserSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), userPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"users\".* FROM \"users\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, userPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(userPrimaryKeyColumns), 1, len(userPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&users)
	if err != nil {
		return errors.Wrap(err, "kmedia: unable to reload all in UserSlice")
	}

	*o = users

	return nil
}

// UserExists checks if the User row exists.
func UserExists(exec boil.Executor, id int) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"users\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "kmedia: unable to check if users exists")
	}

	return exists, nil
}

// UserExistsG checks if the User row exists.
func UserExistsG(id int) (bool, error) {
	return UserExists(boil.GetDB(), id)
}

// UserExistsGP checks if the User row exists. Panics on error.
func UserExistsGP(id int) bool {
	e, err := UserExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// UserExistsP checks if the User row exists. Panics on error.
func UserExistsP(exec boil.Executor, id int) bool {
	e, err := UserExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

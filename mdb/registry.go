package mdb

/*
This is a modified version of the github.com/Bnei-Baruch/mdb/api/registry.go
 We take, manually, only what we need from there.
*/

import (
	"github.com/pkg/errors"
	"github.com/vattle/sqlboiler/boil"
	"github.com/vattle/sqlboiler/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

var (
	CONTENT_TYPE_REGISTRY      = &ContentTypeRegistry{}
	CONTENT_ROLE_TYPE_REGISTRY = &ContentRoleTypeRegistry{}
	PERSONS_REGISTRY           = &PersonsRegistry{}
	AUTHOR_REGISTRY            = &AuthorRegistry{}
	SOURCE_TYPE_REGISTRY       = &SourceTypeRegistry{}
)

func InitTypeRegistries(exec boil.Executor) error {
	registries := []TypeRegistry{CONTENT_TYPE_REGISTRY,
		CONTENT_ROLE_TYPE_REGISTRY,
		PERSONS_REGISTRY,
		AUTHOR_REGISTRY,
		SOURCE_TYPE_REGISTRY}

	for _, x := range registries {
		if err := x.Init(exec); err != nil {
			return err
		}
	}

	return nil
}

type TypeRegistry interface {
	Init(exec boil.Executor) error
}

type ContentTypeRegistry struct {
	ByName map[string]*mdbmodels.ContentType
	ByID   map[int64]*mdbmodels.ContentType
}

func (r *ContentTypeRegistry) Init(exec boil.Executor) error {
	types, err := mdbmodels.ContentTypes(exec).All()
	if err != nil {
		return errors.Wrap(err, "Load content_types from DB")
	}

	r.ByName = make(map[string]*mdbmodels.ContentType)
	r.ByID = make(map[int64]*mdbmodels.ContentType)
	for _, t := range types {
		r.ByName[t.Name] = t
		r.ByID[t.ID] = t
	}

	return nil
}

type ContentRoleTypeRegistry struct {
	ByName map[string]*mdbmodels.ContentRoleType
}

func (r *ContentRoleTypeRegistry) Init(exec boil.Executor) error {
	types, err := mdbmodels.ContentRoleTypes(exec).All()
	if err != nil {
		return errors.Wrap(err, "Load content_role_types from DB")
	}

	r.ByName = make(map[string]*mdbmodels.ContentRoleType)
	for _, t := range types {
		r.ByName[t.Name] = t
	}

	return nil
}

type PersonsRegistry struct {
	ByPattern map[string]*mdbmodels.Person
}

func (r *PersonsRegistry) Init(exec boil.Executor) error {
	types, err := mdbmodels.Persons(exec, qm.Where("pattern is not null")).All()
	if err != nil {
		return errors.Wrap(err, "Load persons from DB")
	}

	r.ByPattern = make(map[string]*mdbmodels.Person)
	for _, t := range types {
		r.ByPattern[t.Pattern.String] = t
	}

	return nil
}

type AuthorRegistry struct {
	ByCode map[string]*mdbmodels.Author
}

func (r *AuthorRegistry) Init(exec boil.Executor) error {
	authors, err := mdbmodels.Authors(exec).All()
	if err != nil {
		return errors.Wrap(err, "Load authors from DB")
	}

	r.ByCode = make(map[string]*mdbmodels.Author)
	for _, a := range authors {
		r.ByCode[a.Code] = a
	}

	return nil
}

type SourceTypeRegistry struct {
	ByName map[string]*mdbmodels.SourceType
	ByID   map[int64]*mdbmodels.SourceType
}

func (r *SourceTypeRegistry) Init(exec boil.Executor) error {
	types, err := mdbmodels.SourceTypes(exec).All()
	if err != nil {
		return errors.Wrap(err, "Load source_types from DB")
	}

	r.ByName = make(map[string]*mdbmodels.SourceType)
	r.ByID = make(map[int64]*mdbmodels.SourceType)
	for _, t := range types {
		r.ByName[t.Name] = t
		r.ByID[t.ID] = t
	}

	return nil
}

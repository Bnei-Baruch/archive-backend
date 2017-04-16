package mdb

/*
This is a modified version of the github.com/Bnei-Baruch/mdb/api/registry.go
 We take, manually, only what we need from there.
*/

import (
	"github.com/vattle/sqlboiler/boil"

	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

var (
	CONTENT_TYPE_REGISTRY = &ContentTypeRegistry{}
	SOURCE_TYPE_REGISTRY = &SourceTypeRegistry{}
)

type ContentTypeRegistry struct {
	ByName map[string]*mdbmodels.ContentType
	ByID   map[int64]*mdbmodels.ContentType
}

func (r *ContentTypeRegistry) Init(exec boil.Executor) error {
	types, err := mdbmodels.ContentTypes(exec).All()
	if err != nil {
		return err
	}

	r.ByName = make(map[string]*mdbmodels.ContentType)
	r.ByID = make(map[int64]*mdbmodels.ContentType)
	for _, t := range types {
		r.ByName[t.Name] = t
		r.ByID[t.ID] = t
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
		return err
	}

	r.ByName = make(map[string]*mdbmodels.SourceType)
	r.ByID = make(map[int64]*mdbmodels.SourceType)
	for _, t := range types {
		r.ByName[t.Name] = t
		r.ByID[t.ID] = t
	}

	return nil
}
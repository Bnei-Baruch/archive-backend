package mdb

import (
	"strconv"

	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

type ExtCCUSlice mdbmodels.CollectionsContentUnitSlice

func (s ExtCCUSlice) Len() int      { return len(s) }
func (s ExtCCUSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type InCollection struct{ ExtCCUSlice }

func (s InCollection) Less(i, j int) bool {
	a, b := s.ExtCCUSlice[i], s.ExtCCUSlice[j]

	// Lesson parts should be sorted by numerically
	ctlID := CONTENT_TYPE_REGISTRY.ByName[CT_LESSON_PART].ID
	if a.R.ContentUnit.TypeID == ctlID && b.R.ContentUnit.TypeID == ctlID {
		ai, err := strconv.Atoi(a.Name)
		if err != nil {
			bi, err := strconv.Atoi(b.Name)
			if err != nil {
				return ai < bi
			}
		}
	}

	return a.Name < b.Name
}

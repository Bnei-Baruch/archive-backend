// Code generated by SQLBoiler (https://github.com/Bnei-Baruch/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/Bnei-Baruch/sqlboiler/boil"
	"github.com/Bnei-Baruch/sqlboiler/randomize"
	"github.com/Bnei-Baruch/sqlboiler/strmangle"
)

func testHistories(t *testing.T) {
	t.Parallel()

	query := Histories(nil)

	if query.Query == nil {
		t.Error("expected a query, got nothing")
	}
}
func testHistoriesDelete(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = history.Delete(tx); err != nil {
		t.Error(err)
	}

	count, err := Histories(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testHistoriesQueryDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = Histories(tx).DeleteAll(); err != nil {
		t.Error(err)
	}

	count, err := Histories(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testHistoriesSliceDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	slice := HistorySlice{history}

	if err = slice.DeleteAll(tx); err != nil {
		t.Error(err)
	}

	count, err := Histories(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}
func testHistoriesExists(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	e, err := HistoryExists(tx, history.ID)
	if err != nil {
		t.Errorf("Unable to check if History exists: %s", err)
	}
	if !e {
		t.Errorf("Expected HistoryExistsG to return true, but got false.")
	}
}
func testHistoriesFind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	historyFound, err := FindHistory(tx, history.ID)
	if err != nil {
		t.Error(err)
	}

	if historyFound == nil {
		t.Error("want a record, got nil")
	}
}
func testHistoriesBind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = Histories(tx).Bind(history); err != nil {
		t.Error(err)
	}
}

func testHistoriesOne(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	if x, err := Histories(tx).One(); err != nil {
		t.Error(err)
	} else if x == nil {
		t.Error("expected to get a non nil record")
	}
}

func testHistoriesAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	historyOne := &History{}
	historyTwo := &History{}
	if err = randomize.Struct(seed, historyOne, historyDBTypes, false, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}
	if err = randomize.Struct(seed, historyTwo, historyDBTypes, false, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = historyOne.Insert(tx); err != nil {
		t.Error(err)
	}
	if err = historyTwo.Insert(tx); err != nil {
		t.Error(err)
	}

	slice, err := Histories(tx).All()
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 2 {
		t.Error("want 2 records, got:", len(slice))
	}
}

func testHistoriesCount(t *testing.T) {
	t.Parallel()

	var err error
	seed := randomize.NewSeed()
	historyOne := &History{}
	historyTwo := &History{}
	if err = randomize.Struct(seed, historyOne, historyDBTypes, false, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}
	if err = randomize.Struct(seed, historyTwo, historyDBTypes, false, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = historyOne.Insert(tx); err != nil {
		t.Error(err)
	}
	if err = historyTwo.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := Histories(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 2 {
		t.Error("want 2 records, got:", count)
	}
}

func testHistoriesInsert(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := Histories(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testHistoriesInsertWhitelist(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx, historyColumnsWithoutDefault...); err != nil {
		t.Error(err)
	}

	count, err := Histories(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testHistoriesReload(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = history.Reload(tx); err != nil {
		t.Error(err)
	}
}

func testHistoriesReloadAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	slice := HistorySlice{history}

	if err = slice.ReloadAll(tx); err != nil {
		t.Error(err)
	}
}
func testHistoriesSelect(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	slice, err := Histories(tx).All()
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 1 {
		t.Error("want one record, got:", len(slice))
	}
}

var (
	historyDBTypes = map[string]string{`AccountID`: `character varying`, `ChronicleID`: `character varying`, `CreatedAt`: `timestamp with time zone`, `Data`: `jsonb`, `ID`: `bigint`, `UnitUID`: `character varying`}
	_              = bytes.MinRead
)

func testHistoriesUpdate(t *testing.T) {
	t.Parallel()

	if len(historyColumns) == len(historyPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := Histories(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	if err = history.Update(tx); err != nil {
		t.Error(err)
	}
}

func testHistoriesSliceUpdateAll(t *testing.T) {
	t.Parallel()

	if len(historyColumns) == len(historyPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	history := &History{}
	if err = randomize.Struct(seed, history, historyDBTypes, true, historyColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := Histories(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, history, historyDBTypes, true, historyPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	// Remove Primary keys and unique columns from what we plan to update
	var fields []string
	if strmangle.StringSliceMatch(historyColumns, historyPrimaryKeyColumns) {
		fields = historyColumns
	} else {
		fields = strmangle.SetComplement(
			historyColumns,
			historyPrimaryKeyColumns,
		)
	}

	value := reflect.Indirect(reflect.ValueOf(history))
	updateMap := M{}
	for _, col := range fields {
		updateMap[col] = value.FieldByName(strmangle.TitleCase(col)).Interface()
	}

	slice := HistorySlice{history}
	if err = slice.UpdateAll(tx, updateMap); err != nil {
		t.Error(err)
	}
}
func testHistoriesUpsert(t *testing.T) {
	t.Parallel()

	if len(historyColumns) == len(historyPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	// Attempt the INSERT side of an UPSERT
	history := History{}
	if err = randomize.Struct(seed, &history, historyDBTypes, true); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = history.Upsert(tx, false, nil, nil); err != nil {
		t.Errorf("Unable to upsert History: %s", err)
	}

	count, err := Histories(tx).Count()
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}

	// Attempt the UPDATE side of an UPSERT
	if err = randomize.Struct(seed, &history, historyDBTypes, false, historyPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize History struct: %s", err)
	}

	if err = history.Upsert(tx, true, nil, nil); err != nil {
		t.Errorf("Unable to upsert History: %s", err)
	}

	count, err = Histories(tx).Count()
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

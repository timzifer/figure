package data_test

import (
	"testing"
	"time"
	"unsafe"

	"github.com/timzifer/figure/data"
)

// TestFloat64ColumnsBorrowsRatherThanCopies pins the zero-copy promise: the
// slice handed in is the slice handed back, not a duplicate of it.
func TestFloat64ColumnsBorrowsRatherThanCopies(t *testing.T) {
	xs := []float64{1, 2, 3}
	src := data.Float64Columns(map[string][]float64{"x": xs})

	got, ok := data.Float64Column(src, "x")
	if !ok {
		t.Fatal("column x is missing")
	}
	if unsafe.SliceData(got) != unsafe.SliceData(xs) {
		t.Fatal("Float64Columns copied the data instead of borrowing it")
	}
}

func TestFloat64ColumnsBasics(t *testing.T) {
	src := data.Float64Columns(map[string][]float64{"b": {1, 2}, "a": {3, 4}})
	if src.Len() != 2 {
		t.Errorf("Len() = %d, want 2", src.Len())
	}
	// Column order must not depend on map iteration order.
	cols := src.Columns()
	if len(cols) != 2 || cols[0] != "a" || cols[1] != "b" {
		t.Errorf("Columns() = %v, want [a b]", cols)
	}
	if _, ok := data.Float64Column(src, "nope"); ok {
		t.Error("an absent column reported ok")
	}
	if _, ok := data.TimeColumn(src, "a"); ok {
		t.Error("a numeric column must not answer TimeColumn")
	}
}

func TestFloat64ColumnsRejectsRaggedInput(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("mismatched column lengths must panic")
		}
	}()
	data.Float64Columns(map[string][]float64{"a": {1, 2}, "b": {1}})
}

func TestTableMixesNumericAndTimeColumns(t *testing.T) {
	now := time.Date(2026, time.March, 14, 9, 0, 0, 0, time.UTC)
	times := []time.Time{now, now.Add(time.Minute)}
	values := []float64{1.5, 2.5}

	tab := data.NewTable().Time("t", times).Float64("y", values)

	if tab.Len() != 2 {
		t.Errorf("Len() = %d, want 2", tab.Len())
	}
	if cols := tab.Columns(); len(cols) != 2 || cols[0] != "t" || cols[1] != "y" {
		t.Errorf("Columns() = %v, want insertion order [t y]", cols)
	}
	got, ok := data.TimeColumn(tab, "t")
	if !ok || unsafe.SliceData(got) != unsafe.SliceData(times) {
		t.Error("Table copied the time column instead of borrowing it")
	}
	if _, ok := data.Float64Column(tab, "t"); ok {
		t.Error("a time column must not answer Float64Column")
	}
}

func TestTableRejectsDuplicateAndRaggedColumns(t *testing.T) {
	t.Run("duplicate", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("a duplicate column name must panic")
			}
		}()
		data.NewTable().Float64("a", []float64{1}).Float64("a", []float64{2})
	})
	t.Run("ragged", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("a mismatched column length must panic")
			}
		}()
		data.NewTable().Float64("a", []float64{1, 2}).Float64("b", []float64{1})
	})
}

func TestTableCarriesStringColumns(t *testing.T) {
	tbl := data.NewTable().
		String("region", []string{"north", "south", "north"}).
		Float64("sales", []float64{3, 4, 5})

	got, ok := data.StringColumn(tbl, "region")
	if !ok {
		t.Fatal("StringColumn(region) reported the column missing")
	}
	if len(got) != 3 || got[1] != "south" {
		t.Errorf("StringColumn(region) = %v", got)
	}
	if _, ok := data.Float64Column(tbl, "region"); ok {
		t.Error("a string column must not answer to Float64Column")
	}
	if _, ok := data.StringColumn(tbl, "sales"); ok {
		t.Error("a numeric column must not answer to StringColumn")
	}
	if tbl.Len() != 3 {
		t.Errorf("Len() = %d, want 3", tbl.Len())
	}
}

func TestTableStringColumnIsBorrowed(t *testing.T) {
	src := []string{"a", "b"}
	tbl := data.NewTable().String("k", src)
	src[0] = "z"
	if got, _ := data.StringColumn(tbl, "k"); got[0] != "z" {
		t.Error("the slice was copied; the data layer borrows")
	}
}

func TestTableRejectsARaggedStringColumn(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("want a panic: a ragged table is a programming error")
		}
	}()
	data.NewTable().Float64("a", []float64{1, 2}).String("b", []string{"x"})
}

func TestFloat64SourceHasNoStringColumns(t *testing.T) {
	s := data.Float64Columns(map[string][]float64{"x": {1}})
	if _, ok := data.StringColumn(s, "x"); ok {
		t.Error("a numeric source must report no string columns")
	}
}

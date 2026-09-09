package arrow_test

import (
	"testing"

	aw "github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/timzifer/figure/arrow/v18"
	"github.com/timzifer/figure/data"
)

// An Arrow int64 column arrives exact rather than widened.
//
// Widening it into a float64 was the adapter's behaviour until the data layer
// could hold an exact integer, and it is invisible in a position — a pixel
// cannot show the difference — but it is not invisible in an identifier: past
// 2^53 two ids become one number, one key and one row. See ADR 0061.
func TestAnInt64ColumnStaysExact(t *testing.T) {
	const id = 9007199254740993 // 2^53+1
	rec := record(t, schemaOf(aw.Field{Name: "id", Type: aw.PrimitiveTypes.Int64}),
		func(b *array.RecordBuilder) {
			b.Field(0).(*array.Int64Builder).AppendValues([]int64{id, id + 2}, nil)
		})

	src := arrow.Source(rec)
	got, ok := data.Int64Column(src, "id")
	if !ok {
		t.Fatal("an Arrow int64 column did not arrive as exact integers")
	}
	if got[0] != id || got[1] != id+2 {
		t.Errorf("ids are %v, want %d and %d", got, id, id+2)
	}
	labels, _ := data.Labels(src, "id")
	if labels[0] == labels[1] {
		t.Errorf("two ids spell the same: %q", labels[0])
	}
}

// The borrow is kept where Arrow's memory is already what figure wants: one
// contiguous run of int64 with nothing to substitute.
func TestAnInt64ColumnWithoutNullsIsBorrowed(t *testing.T) {
	rec := record(t, schemaOf(aw.Field{Name: "n", Type: aw.PrimitiveTypes.Int64}),
		func(b *array.RecordBuilder) {
			b.Field(0).(*array.Int64Builder).AppendValues([]int64{1, 2, 3}, nil)
		})

	src := arrow.Source(rec)
	got, _ := data.Int64Column(src, "n")
	want := rec.Column(0).(*array.Int64).Int64Values()
	if len(got) != len(want) || &got[0] != &want[0] {
		t.Error("the int64 column was copied where it could have been borrowed")
	}
}

// A uint64 is not claimed as exact: its top half has no int64 to be exact in,
// and wrapping silently would be worse than the float it becomes.
func TestAUint64ColumnStaysNumeric(t *testing.T) {
	rec := record(t, schemaOf(aw.Field{Name: "u", Type: aw.PrimitiveTypes.Uint64}),
		func(b *array.RecordBuilder) {
			b.Field(0).(*array.Uint64Builder).AppendValues([]uint64{1, 2}, nil)
		})

	src := arrow.Source(rec)
	if _, ok := data.Int64Column(src, "u"); ok {
		t.Error("a uint64 column claimed to be exact int64")
	}
	if _, ok := data.Float64Column(src, "u"); !ok {
		t.Error("a uint64 column is not readable as numbers")
	}
}

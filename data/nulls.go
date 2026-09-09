package data

// # What a null means
//
// One rule, everywhere a column is read: **a null is a missing value.** On a
// position axis the row has no place, so
// [github.com/timzifer/figure/geom.OnMissing] decides whether the chart gaps,
// interpolates or errors — the same three answers it already gives a NaN. On a
// colour channel the row takes the scale's undefined colour. In a group or
// facet column the row belongs to no series and no panel, so it is not drawn: a
// category named "" is not a reading, and inventing one is how the zero time
// got onto an axis in the first place. See
// [ADR 0034](../docs/adr/0034-null-values.md).
//
// Absence is carried by [Column.Nulls]. It used to be an optional interface
// beside [Source], which meant every Source wrapping another had to remember to
// forward a second method for its rows to stay missing — and two in this
// package did not. ADR 0061 moved it onto the column, where it travels with the
// values it describes.

// NullMask reports which of a column's rows are absent, or ok == false when the
// column is not there or has nothing to say.
//
// It is the shared spelling of the question, the way [Labels] is for "what does
// this row say in that column". ok is false for a column whose mask marks
// nothing, which is what keeps the zero-copy path in [Float64Columns] intact: a
// reader asks first and copies only when the answer is yes, so a table without
// nulls costs exactly what it did.
func NullMask(src Source, name string) (null []bool, ok bool) {
	c, got := ColumnOf(src, name)
	if !got || len(c.Nulls) == 0 || !AnyNull(c.Nulls) {
		return nil, false
	}
	return c.Nulls, true
}

// IsNull reports whether row i of mask is absent, tolerating a nil or short
// mask.
//
// A mask is as long as the column it describes, so the bounds test is not
// defensive padding: [Rows] gathers a mask along with the column it belongs
// to, and a caller composing sources by hand may hand over one that stops
// early. A row past the end has a value, because that is what the column says.
// A negative row is not a row at all — an [Alignment] spells "this end has no
// such row" that way — and is not null either; the caller has already been
// told there is nothing there.
func IsNull(mask []bool, i int) bool { return i >= 0 && i < len(mask) && mask[i] }

// AnyNull reports whether mask marks any row at all.
//
// It is what lets a reader decide between the borrowed column and a copy: a
// mask that marks nothing changes no value, so there is nothing to write and
// the caller's slice is handed on untouched.
func AnyNull(mask []bool) bool {
	for _, null := range mask {
		if null {
			return true
		}
	}
	return false
}

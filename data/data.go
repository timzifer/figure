// Package data is figure's data layer: columnar, batch-oriented access to a
// table of values.
//
// The interface returns whole typed columns, never one value at a time. Scalar
// access is the single easiest way to make a plotting library slow, and a
// columnar shape is also what lets a []float64-backed source be borrowed
// instead of copied.
package data

import (
	"strconv"
	"time"
)

// Kind says what a [Column] holds.
//
// The set grows at the end. A reader that switches on it needs a default, and
// the default that is always right is [Column.Spell]: every column of every
// kind can say what a row reads as, so a kind added later is labelled rather
// than lost. See ADR 0061.
type Kind uint8

// The column kinds.
const (
	// KindFloat64 is a measured number, in [Column.Floats]. A missing value
	// is NaN or a marked row; see [Column.Nulls].
	KindFloat64 Kind = iota
	// KindInt64 is an exact integer, in [Column.Ints]. It exists because a
	// float64 stops counting exactly above 2^53, and an identifier that
	// collides there is a chart that draws two rows as one.
	KindInt64
	// KindString is a category name, in [Column.Strings].
	KindString
	// KindTime is an instant, in [Column.Times].
	KindTime
)

// Column is one column of a table: the values, which rows are absent, and how
// a row reads as text.
//
// # Why one type rather than one method per kind
//
// [Source] used to answer three questions — is this column numeric, temporal,
// categorical — and a fourth kind meant a fourth optional interface, a fourth
// branch at every reader, and a fourth thing every wrapping Source had to
// remember to forward. Two in this package had already forgotten to forward the
// third. One accessor returning a growable struct makes a new kind a field and
// a case in this package's own helpers, and leaves every Source that does not
// produce it untouched. ADR 0061 is the record.
//
// # What is borrowed
//
// The slices are lent, not given: a reader must not write to one, and a Source
// hands out its own memory rather than copying. That is what keeps
// [Float64Columns] zero-copy.
type Column struct {
	// Kind says which of the value slices below is the column's own. The
	// others are nil.
	Kind Kind

	// Floats holds the values of a [KindFloat64] column.
	Floats []float64
	// Ints holds the values of a [KindInt64] column.
	Ints []int64
	// Strings holds the values of a [KindString] column.
	Strings []string
	// Times holds the values of a [KindTime] column.
	Times []time.Time

	// Nulls is one flag per row, true where the column has no value, or nil
	// for a column with nothing absent.
	//
	// It is a field rather than the optional interface it used to be because
	// absence belongs to the column: a Source that wraps another — a facet's
	// cut, a stream's snapshot, a transition's blend — carries it along with
	// the values instead of having to remember a second method. Forgetting
	// that method silently un-marked the missing rows, which is a chart
	// drawing something nobody measured.
	//
	// A numeric column may use it or use NaN, and the two spellings agree: a
	// NaN and a marked row are both missing and neither outranks the other. A
	// text or temporal column has no NaN to be missing with — "" is a string
	// somebody may have measured and the zero time is the year 1 — so for
	// those it is the only way to say it. See
	// [ADR 0034](../docs/adr/0034-null-values.md).
	Nulls []bool

	// Text spells one row, and is what makes a column readable without
	// knowing its kind: a label, a facet's strip, a legend entry, the key
	// [github.com/timzifer/figure/geom.KeyBy] identifies a row by.
	//
	// It is optional. A nil Text means [Column.Spell] derives the spelling
	// from the values, which is what every column in this package does. A
	// Source whose values are richer than the slice it can offer fills it in:
	// a column of physical quantities plots as float64 positions and spells
	// as "2.5 bar", and figure needs to know nothing about units to draw and
	// label it.
	//
	// It is a function rather than a slice of strings because most columns
	// are never spelled. Materialising a million labels for a scatter nobody
	// hovers over is a cost with no reader.
	//
	// It does not survive the JSON spec: a document records the values, and a
	// Source rebuilt from one spells them the default way.
	Text func(i int) string
}

// Len reports the number of rows in the column.
func (c Column) Len() int {
	switch c.Kind {
	case KindInt64:
		return len(c.Ints)
	case KindString:
		return len(c.Strings)
	case KindTime:
		return len(c.Times)
	default:
		return len(c.Floats)
	}
}

// Spell reports what row i reads as.
//
// It is [Column.Text] where the Source supplied one, and otherwise the default
// spelling of the kind: an exact integer as digits, a number as Go's shortest
// round-tripping form, an instant as RFC 3339, a category as itself. A row
// outside the column, and a kind this build does not know, spell as "".
//
// Two rows with the same value spell the same, which is what lets a spelling
// be an identity: [Labels] groups by it and a categorical axis encodes by it,
// so the two agree about what counts as one category.
func (c Column) Spell(i int) string {
	if i < 0 || i >= c.Len() {
		return ""
	}
	if c.Text != nil {
		return c.Text(i)
	}
	switch c.Kind {
	case KindInt64:
		return strconv.FormatInt(c.Ints[i], 10)
	case KindString:
		return c.Strings[i]
	case KindTime:
		return c.Times[i].Format(time.RFC3339)
	case KindFloat64:
		return FormatNumber(c.Floats[i])
	default:
		return ""
	}
}

// Numbers reports the column as float64, which is what a position on an axis
// is made of.
//
// A [KindFloat64] column is handed back borrowed. A [KindInt64] column is
// converted, which allocates and which loses exactness above 2^53 — that is
// the honest trade and it is the right way round: a pixel cannot show the
// difference, and the exactness that matters is in [Column.Spell], where an
// identifier is compared. Any other kind reports ok == false; a category and
// an instant reach an axis through [github.com/timzifer/figure/scale], not
// through here.
func (c Column) Numbers() ([]float64, bool) {
	switch c.Kind {
	case KindFloat64:
		return c.Floats, true
	case KindInt64:
		out := make([]float64, len(c.Ints))
		for i, v := range c.Ints {
			out[i] = float64(v)
		}
		return out, true
	default:
		return nil, false
	}
}

// Source exposes columnar, batch access to a table.
//
// Implementations return read-only views: the caller must not mutate a
// returned slice, and figure never does. An implementation that already holds
// its data as a Go slice should return that slice directly rather than
// copying.
//
// # Stability
//
// Source is implemented outside this module, so it never gains a method, and
// its one column accessor takes and returns growable values rather than a list
// of typed alternatives. A kind of column this package does not yet know is a
// [Kind] and a field on [Column], not another method here and not an optional
// interface beside it. See ADR 0060 and ADR 0061.
type Source interface {
	// Len reports the number of rows. Every column has this length.
	Len() int

	// Columns lists the available column names. The order is stable across
	// calls on the same Source.
	Columns() []string

	// Column returns a column by name. ok is false if the column does not
	// exist.
	Column(name string) (Column, bool)
}

// ColumnOf returns a column of src, or the zero Column when src is nil, the
// name is empty or the column is not there.
//
// It is the nil-tolerant spelling every reader in figure uses, so that "no
// source" and "no such column" are one answer rather than two guards at each
// call.
func ColumnOf(src Source, name string) (Column, bool) {
	if src == nil || name == "" {
		return Column{}, false
	}
	return src.Column(name)
}

// Float64Column returns a column as numbers: [Column.Numbers] of the named
// column.
func Float64Column(src Source, name string) ([]float64, bool) {
	c, ok := ColumnOf(src, name)
	if !ok {
		return nil, false
	}
	return c.Numbers()
}

// Int64Column returns an exact integer column, or ok == false for a column of
// any other kind. A caller wanting a number rather than an identifier asks
// [Float64Column], which converts.
func Int64Column(src Source, name string) ([]int64, bool) {
	c, ok := ColumnOf(src, name)
	if !ok || c.Kind != KindInt64 {
		return nil, false
	}
	return c.Ints, true
}

// StringColumn returns a category column, or ok == false for a column of any
// other kind. A caller that wants any column *as* text asks [Labels], which
// spells every kind.
func StringColumn(src Source, name string) ([]string, bool) {
	c, ok := ColumnOf(src, name)
	if !ok || c.Kind != KindString {
		return nil, false
	}
	return c.Strings, true
}

// TimeColumn returns a temporal column, or ok == false for a column of any
// other kind.
func TimeColumn(src Source, name string) ([]time.Time, bool) {
	c, ok := ColumnOf(src, name)
	if !ok || c.Kind != KindTime {
		return nil, false
	}
	return c.Times, true
}

// Float64Columns builds a Source over the given numeric columns.
//
// The slices are borrowed, not copied: the returned Source aliases the caller's
// memory, and mutating it afterwards mutates what figure will plot. All
// columns must have the same length; Float64Columns panics otherwise, because
// a ragged table is a programming error rather than a runtime condition.
func Float64Columns(cols map[string][]float64) Source {
	s := &float64Source{cols: cols, names: sortedKeys(cols)}
	for i, n := range s.names {
		if i == 0 {
			s.n = len(cols[n])
			continue
		}
		if len(cols[n]) != s.n {
			panic("figure/data: Float64Columns: column " + n + " has a different length than " + s.names[0])
		}
	}
	return s
}

type float64Source struct {
	cols  map[string][]float64
	names []string
	n     int
}

func (s *float64Source) Len() int          { return s.n }
func (s *float64Source) Columns() []string { return s.names }

func (s *float64Source) Column(name string) (Column, bool) {
	c, ok := s.cols[name]
	if !ok {
		return Column{}, false
	}
	return Column{Kind: KindFloat64, Floats: c}, true
}

// Table is a Source that mixes numeric, exact-integer, temporal and
// categorical columns.
//
// It is the general-purpose implementation: use it when a chart plots time or
// a category against values, which is the common case for the Time and Ordinal
// scales.
type Table struct {
	cols  map[string]Column
	names []string
	n     int
	fixed bool // true once the row count has been established
}

// NewTable returns an empty Table.
func NewTable() *Table {
	return &Table{cols: map[string]Column{}}
}

// Float64 adds a numeric column, borrowing the slice. It returns t so calls
// can be chained. It panics if the column length disagrees with columns
// already added, or if the name is already taken.
func (t *Table) Float64(name string, v []float64) *Table {
	t.claim(name, len(v))
	t.cols[name] = Column{Kind: KindFloat64, Floats: v}
	return t
}

// Int64 adds an exact integer column, borrowing the slice. It returns t so
// calls can be chained. It panics if the column length disagrees with columns
// already added, or if the name is already taken.
//
// Use it for a column that counts or identifies: a row id, an order number, an
// epoch in a unit of its own. It plots exactly as a numeric column does — the
// values are converted for the axis — and it *labels* exactly, which is the
// difference. A float64 stops counting at 2^53, and past that two ids become
// one row.
func (t *Table) Int64(name string, v []int64) *Table {
	t.claim(name, len(v))
	t.cols[name] = Column{Kind: KindInt64, Ints: v}
	return t
}

// Time adds a temporal column, borrowing the slice. It returns t so calls can
// be chained. It panics if the column length disagrees with columns already
// added, or if the name is already taken.
func (t *Table) Time(name string, v []time.Time) *Table {
	t.claim(name, len(v))
	t.cols[name] = Column{Kind: KindTime, Times: v}
	return t
}

// String adds a categorical column, borrowing the slice. It returns t so calls
// can be chained. It panics if the column length disagrees with columns
// already added, or if the name is already taken.
//
// Plot such a column against a [scale.Ordinal] axis; a continuous scale has no
// position for a category name and a geom says so rather than guessing one.
func (t *Table) String(name string, v []string) *Table {
	t.claim(name, len(v))
	t.cols[name] = Column{Kind: KindString, Strings: v}
	return t
}

func (t *Table) claim(name string, n int) {
	if _, dup := t.cols[name]; dup {
		panic("figure/data: duplicate column " + name)
	}
	if t.fixed && n != t.n {
		panic("figure/data: column " + name + " has a different length than the existing columns")
	}
	t.n, t.fixed = n, true
	t.names = append(t.names, name)
}

// Len reports the number of rows.
func (t *Table) Len() int { return t.n }

// Columns lists the column names in insertion order.
func (t *Table) Columns() []string { return t.names }

// Column returns a column by name.
func (t *Table) Column(name string) (Column, bool) {
	c, ok := t.cols[name]
	return c, ok
}

// WithNulls marks rows of an existing column as absent, borrowing the mask. It
// returns t so calls can be chained. It panics if the column does not exist or
// the mask is not one flag per row.
//
// A text or temporal column needs this because it has no NaN to be missing
// with: "" is a string somebody may have measured and the zero time is an
// instant, so absence has to be said beside the values rather than inside
// them. A numeric column may use it too, and the two spellings agree — a NaN
// and a marked row are both missing, and neither outranks the other.
//
// A mask that marks nothing is not stored: a reader sees a column with no
// nulls either way, and would otherwise copy a column to change none of it.
func (t *Table) WithNulls(name string, null []bool) *Table {
	c, ok := t.cols[name]
	if !ok {
		panic("figure/data: no column " + name + " to mark null")
	}
	if len(null) != t.n {
		panic("figure/data: null mask for column " + name + " has a different length than the columns")
	}
	if !AnyNull(null) {
		return t
	}
	c.Nulls = null
	t.cols[name] = c
	return t
}

// WithText gives a column a spelling of its own, borrowing the function. It
// returns t so calls can be chained. It panics if the column does not exist.
//
// It is how a caller whose values mean more than their type says so: a column
// of exact integers that are really order numbers, a numeric column whose rows
// carry a unit. See [Column.Text].
func (t *Table) WithText(name string, text func(i int) string) *Table {
	c, ok := t.cols[name]
	if !ok {
		panic("figure/data: no column " + name + " to spell")
	}
	c.Text = text
	t.cols[name] = c
	return t
}

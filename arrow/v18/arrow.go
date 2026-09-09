// Package arrow adapts Apache Arrow data to figure's data layer.
//
// It is a module of its own so that the core stays what it claims to be: a
// chart library with no dependencies. Arrow is a large dependency and most
// charts have nothing to do with it, so it enters only for the programs that
// already hold Arrow data — and for them it is the shortest possible path,
// because Arrow's columnar layout is the layout figure already wants.
//
// The import path carries a major version, and it is Arrow's rather than
// figure's: this module adapts apache/arrow-go/v18, and its own major version
// follows its upstream's so that the two can never disagree about what an
// Arrow record is. The package name is still arrow.
//
//	import "github.com/timzifer/figure/arrow/v18"
//
//	rec := reader.Record()
//	src := arrow.Source(rec)
//
//	p := figure.New(figure.Title("Latency"))
//	p.Add(geom.Line(src, geom.X("t"), geom.Y("p99")))
//
// # What is borrowed and what is copied
//
// A float64 column with no nulls is borrowed: [data.Source.Float64Column]
// returns Arrow's own buffer, with no copy and no conversion. That is the case
// the two libraries agree about exactly — IEEE-754 doubles, contiguous, one
// per row.
//
// Everything else is converted once, on first use, and cached: an integer or
// float32 column becomes float64, a timestamp becomes time.Time, a string view
// becomes a Go string. A column the chart never reads is never converted, so a
// record with forty columns and a chart that plots two pays for two.
//
// # Nulls
//
// An Arrow null becomes NaN in a numeric column, the zero time in a temporal
// one and the empty string in a categorical one — and the validity bitmap is
// offered beside the values through [data.Nulls], which is what tells the
// second two apart from a value somebody measured. NaN is what figure's
// missing-data policies are written against, so a null row is gapped,
// interpolated or rejected by the same [geom.OnMissing] setting that handles a
// NaN coming from anywhere else; with the mask that is true of a timestamp and
// a category as well, where before an absent instant was the year 1 and an
// absent category was a band of its own. A float64 column that has nulls is
// copied rather than borrowed: the nulls have to become NaN somewhere, and
// writing them into Arrow's buffer is not this package's memory to write. A
// column with no nulls offers no mask, so the borrowed path is untouched.
//
// # Lifetime and concurrency
//
// A Source borrows the record; it does not retain a reference to it beyond
// what it needs and does not release it. Keep the record alive — and its
// memory unreleased — for as long as the chart may be rendered.
//
// Resolving a column for the first time fills a cache, so a Source is not safe
// for concurrent first use. Render once before sharing one across goroutines,
// or use [Materialize], which converts everything up front and returns a Source
// that is read-only thereafter.
package arrow

import (
	"time"

	arrow "github.com/apache/arrow-go/v18/arrow"
	"github.com/timzifer/figure/data"
)

// Source returns a [data.Source] over one Arrow record batch.
//
// A nil record gives an empty Source rather than a panic: an empty batch is a
// normal thing for a reader to hand back, and a chart over no rows is a chart
// with no marks.
func Source(rec arrow.Record) data.Source {
	if rec == nil {
		return &source{}
	}
	s := &source{n: int(rec.NumRows())}
	for i := range int(rec.NumCols()) {
		s.names = append(s.names, rec.ColumnName(i))
		s.cols = append(s.cols, []arrow.Array{rec.Column(i)})
	}
	return s
}

// TableSource returns a [data.Source] over an Arrow table.
//
// A table holds each column as a list of chunks, and figure's data layer is
// one slice per column — so a chunked column is concatenated on first use.
// A single-chunk column takes the same path a record does, borrowing where it
// can.
func TableSource(t arrow.Table) data.Source {
	if t == nil {
		return &source{}
	}
	s := &source{n: int(t.NumRows())}
	for i := range int(t.NumCols()) {
		col := t.Column(i)
		s.names = append(s.names, col.Name())
		s.cols = append(s.cols, col.Data().Chunks())
	}
	return s
}

// Materialize converts every column src can offer and returns a plain
// [data.Table] holding the results.
//
// It is the escape hatch from the lazy path: the conversions happen once, here,
// and the result shares nothing with Arrow — so the record can be released, and
// the table can be read from as many goroutines as like. The cost is a copy of
// every column, including the float64 ones a lazy Source would have borrowed.
func Materialize(src data.Source) *data.Table {
	t := data.NewTable()
	if src == nil {
		return t
	}
	for _, name := range src.Columns() {
		switch v, ok := data.Float64Column(src, name); {
		case ok:
			t.Float64(name, append([]float64(nil), v...))
		default:
			if v, ok := data.TimeColumn(src, name); ok {
				t.Time(name, append([]time.Time(nil), v...))
				break
			}
			if v, ok := data.StringColumn(src, name); ok {
				t.String(name, append([]string(nil), v...))
			}
		}
		// The mask travels with the column. Materializing without it would
		// turn every text and temporal null back into "" and the zero time —
		// which is the whole failure [source.Nulls] exists to end, reappearing
		// in the one call whose purpose is to preserve the data.
		if null, ok := data.NullMask(src, name); ok {
			t.WithNulls(name, append([]bool(nil), null...))
		}
	}
	return t
}

type source struct {
	names []string
	// One entry per column, holding its chunks: a record batch has exactly
	// one, a table may have many. Keeping the same shape for both is what lets
	// the converters below not care which they came from.
	cols [][]arrow.Array
	n    int

	nums  map[string][]float64
	times map[string][]time.Time
	strs  map[string][]string
	nulls map[string][]bool

	// cols2 caches the converted columns by name.
	cols2 map[string]data.Column
}

func (s *source) Len() int          { return s.n }
func (s *source) Columns() []string { return s.names }

func (s *source) at(name string) ([]arrow.Array, bool) {
	for i, n := range s.names {
		if n == name {
			return s.cols[i], true
		}
	}
	return nil, false
}

// Column implements [data.Source]. Each kind of Arrow array is tried in the
// order that loses the least: an exact integer stays exact, a timestamp stays
// an instant, a string stays itself, and everything else numeric widens into
// float64.
//
// The result is cached, because a chart reads a column more than once and
// converting an Arrow chunk list is the expensive half of this adapter.
func (s *source) Column(name string) (data.Column, bool) {
	if c, ok := s.cols2[name]; ok {
		return c, c.Kind != kindAbsent
	}
	c, ok := s.build(name)
	if s.cols2 == nil {
		s.cols2 = map[string]data.Column{}
	}
	if !ok {
		s.cols2[name] = data.Column{Kind: kindAbsent}
		return data.Column{}, false
	}
	s.cols2[name] = c
	return c, true
}

// kindAbsent marks a name this source does not have, so that asking twice
// costs one lookup. It is out of range of every real [data.Kind].
const kindAbsent data.Kind = 255

func (s *source) build(name string) (data.Column, bool) {
	chunks, ok := s.at(name)
	if !ok {
		return data.Column{}, false
	}
	// The validity bitmap is the one place a null survives the conversion. It
	// is what closes the half of the null story the stand-in values cannot
	// tell: a numeric column loses nothing by becoming NaN, but "" is a string
	// somebody may have measured and the zero time is an instant — so a text
	// or temporal null would otherwise be a value like any other, a category
	// of its own on an ordinal axis or the year 1 stretching a time domain
	// across two millennia.
	null, _ := nullMask(chunks, s.n)

	if v, ok := intColumn(chunks, s.n); ok {
		return data.Column{Kind: data.KindInt64, Ints: v, Nulls: null}, true
	}
	if v, ok := numericColumn(chunks, s.n); ok {
		return data.Column{Kind: data.KindFloat64, Floats: v, Nulls: null}, true
	}
	if v, ok := timeColumn(chunks, s.n); ok {
		return data.Column{Kind: data.KindTime, Times: v, Nulls: null}, true
	}
	if v, ok := stringColumn(chunks, s.n); ok {
		return data.Column{Kind: data.KindString, Strings: v, Nulls: null}, true
	}
	return data.Column{}, false
}

var _ data.Source = (*source)(nil)

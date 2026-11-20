package geom

import (
	"errors"
	"fmt"
	"math"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/scale"
)

// series is a resolved set of columns, already converted to float64.
//
// y2, c and sz are optional: y2 carries the second bound of a band, c the
// values a colour scale is read from, sz the values a size scale is read from.
// Each is nil when the geom was not given one.
//
// off and rows say where the elements came from, for the caller that asked to
// know — see [Rows]. A segment cut out of the source is contiguous, so off
// alone answers it; an interpolated one is not, and carries a row per element.
type series struct {
	x, y  []float64
	y2, c []float64
	// c2 is the second reading a bivariate colour scale paints from, parallel
	// to c, and nil for every layer that named none. See [UncertaintyBy].
	c2   []float64
	sz   []float64
	off  int
	rows []int

	// origin maps this series' rows onto the rows of the table its source was
	// cut from, when its source is a cut. Faceting makes one per panel, so
	// without this a faceted chart would report row numbers relative to a
	// table the caller never sees. See [data.Subset].
	origin []int
}

// ErrNoColumn reports a column named by an option that the source does not
// have. It is returned rather than panicking because the column name usually
// comes from user input or a config file.
var ErrNoColumn = errors.New("figure/geom: column not found")

// ErrCategorical reports a text column mapped onto an axis that has no
// position for a name.
var ErrCategorical = errors.New("figure/geom: categorical column on a continuous scale")

// ErrRampOnPath reports a continuous colour ramp bound to a layer that draws a
// path rather than a mark.
//
// A stroke carries one colour and no stops, so a path colours in stretches: it
// can say "this part is over the limit" and it cannot say "this part is
// slightly further over it than that part". The scales that cut a continuum
// into stretches are [scale.Threshold], [scale.Quantize] and [scale.Quantile];
// the mark that can paint a continuum is a point.
var ErrRampOnPath = errors.New("figure/geom: continuous colour ramp on a path")

// resolve reads the configured columns out of src.
//
// The scales are passed in because reading a column is not independent of the
// axis it feeds: a text column only has numbers at all once a categorical
// scale has assigned them, and doing that anywhere else would mean two layers
// on one axis disagreeing about which category is which.
func resolve(src data.Source, c config, x, y scale.Scale) (series, error) {
	if src == nil {
		return series{}, errors.New("figure/geom: nil data source")
	}
	xs, err := column(src, c.xcol, x)
	if err != nil {
		return series{}, err
	}
	ys, err := column(src, c.ycol, y)
	if err != nil {
		return series{}, err
	}
	if len(xs) != len(ys) {
		return series{}, fmt.Errorf("figure/geom: columns %q and %q differ in length (%d vs %d)", c.xcol, c.ycol, len(xs), len(ys))
	}
	s := series{x: xs, y: ys, origin: data.Origins(src)}
	if c.y2col != "" {
		v, err := column(src, c.y2col, y)
		if err != nil {
			return series{}, err
		}
		if len(v) != len(xs) {
			return series{}, fmt.Errorf("figure/geom: columns %q and %q differ in length (%d vs %d)", c.xcol, c.y2col, len(xs), len(v))
		}
		s.y2 = v
	}
	if c.colorCol != "" && c.dirTo != "" {
		return series{}, ErrColorAndDirection
	}
	if c.colorCol != "" {
		v, err := colorColumn(src, c)
		if err != nil {
			return series{}, err
		}
		if len(v) != len(xs) {
			return series{}, fmt.Errorf("figure/geom: columns %q and %q differ in length (%d vs %d)", c.xcol, c.colorCol, len(xs), len(v))
		}
		s.c = v
	}
	if c.dirTo != "" {
		v, err := c.directionCodes(src, len(xs))
		if err != nil {
			return series{}, err
		}
		s.c = v
	}
	if c.secondCol != "" {
		v, err := c.secondColumn(src, len(xs))
		if err != nil {
			return series{}, err
		}
		s.c2 = v
	}
	if c.sizeCol != "" && c.sizeScale != nil {
		v, err := column(src, c.sizeCol, nil)
		if err != nil {
			return series{}, err
		}
		if len(v) != len(xs) {
			return series{}, fmt.Errorf("figure/geom: columns %q and %q differ in length (%d vs %d)", c.xcol, c.sizeCol, len(xs), len(v))
		}
		s.sz = v
	}
	dropNulls(src, &s, c.groupCol, c.sizeCol, c.widthCol)
	return s, nil
}

// resolveOne reads a layer that summarises a single column.
//
// A [Histogram] and an [ECDF] read X and compute what goes on Y, so [resolve]'s
// insistence on both columns would refuse them for not naming a column they do
// not have. The Y column of the series is the X column again rather than nil,
// so that every traversal written over a series — plottable, the missing-data
// policy, the group index — reads the observation on both axes and answers the
// one question there is: does this value have a position.
func resolveOne(src data.Source, c config, x scale.Scale) (series, error) {
	if src == nil {
		return series{}, errors.New("figure/geom: nil data source")
	}
	xs, err := column(src, c.xcol, x)
	if err != nil {
		return series{}, err
	}
	s := series{x: xs, y: xs, origin: data.Origins(src)}
	dropNulls(src, &s, c.groupCol)
	return s, nil
}

// checkPathColor rejects a colour scale a path cannot draw. See [ErrRampOnPath].
func (c config) checkPathColor(s series) error {
	if !c.varying(s) {
		return nil
	}
	if s.c2 != nil {
		return fmt.Errorf("%w: column %q with the second reading %q; a path changes colour where one reading crosses a boundary, and two readings cross theirs in different places",
			ErrRampOnPath, c.colorCol, c.secondCol)
	}
	if _, classed := scale.Classed(c.colorScale); classed {
		return nil
	}
	if _, discrete := scale.Discrete(c.colorScale); discrete {
		return nil
	}
	return fmt.Errorf("%w: column %q; cut it into classes with scale.Threshold, scale.Quantize or scale.Quantile, or draw it with geom.Scatter",
		ErrRampOnPath, c.colorCol)
}

// colorColumn reads the column a colour scale paints from.
//
// A discrete scale reads categories, so a text column is encoded through it —
// the same arrangement a categorical *axis* has, where the scale assigns the
// numbers and the column is read in the scale's own space. A continuous scale
// reads numbers and a text column is an error: there is no position on a ramp
// for a name.
func colorColumn(src data.Source, c config) ([]float64, error) {
	if d, discrete := scale.Discrete(c.colorScale); discrete {
		// Whatever the column is stored as, it is read as one label per row —
		// [data.Labels] is the shared spelling of that, so a colour category
		// and an axis category for the same value are the same string.
		labels, ok := data.Labels(src, c.colorCol)
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrNoColumn, c.colorCol)
		}
		// A row with no category is not given one. NaN is the value every
		// colour scale answers with its undefined colour, so an absent
		// category reads as "not one of these" rather than as a category
		// named "" holding a slot in the palette.
		null, _ := data.NullMask(src, c.colorCol)
		out := make([]float64, len(labels))
		for i, l := range labels {
			if data.IsNull(null, i) {
				out[i] = math.NaN()
				continue
			}
			out[i] = d.Encode(l)
		}
		return out, nil
	}
	if _, text := data.StringColumn(src, c.colorCol); text {
		return nil, fmt.Errorf("%w: column %q holds category names and a colour ramp reads numbers; give it a scale.Qualitative",
			ErrCategorical, c.colorCol)
	}
	return column(src, c.colorCol, nil)
}

// column reads one column as float64 for the axis it feeds.
//
// A numeric column on a continuous scale is returned as it lies — that is the
// zero-copy path the data layer exists for. A time column becomes a number in
// the axis's own time space through [scale.ValueOf], so a time scale sees the
// same numeric domain as any other — nanoseconds since the scale's origin,
// which is the Unix epoch unless [scale.Origin] moved it. A text column is
// encoded through the scale, which must be categorical.
//
// A numeric or time column on a *categorical* scale is encoded too, one
// category per distinct formatted value: asking for an ordinal axis over
// numbers means asking for equally spaced slots rather than a numeric line.
func column(src data.Source, name string, s scale.Scale) ([]float64, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: no column selected (use geom.X/geom.Y)", ErrNoColumn)
	}
	cat, _ := s.(scale.Categorical)
	// A row the source calls absent becomes NaN, whatever the column is stored
	// as. That is the whole of the null rule on a position axis: no scale
	// places a NaN, so plottable rejects the row and [OnMissing] decides what
	// happens to it — the same three answers a numeric column's own NaN
	// already gets. Doing it here rather than per geom is what makes it true
	// of every mark, including the ones that do not exist yet. See
	// [data.Column].
	null, _ := data.NullMask(src, name)

	col, ok := data.ColumnOf(src, name)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNoColumn, name)
	}
	// A category is encoded by what it reads as, whatever it is stored as, so
	// an ordinal axis over exact integers puts them in the order the scale was
	// given and labels them as digits rather than as floats.
	if cat != nil {
		out := make([]float64, col.Len())
		for i := range out {
			if data.IsNull(null, i) {
				out[i] = math.NaN()
				continue
			}
			out[i] = cat.Encode(col.Spell(i))
		}
		return out, nil
	}
	if col.Kind == data.KindString {
		return nil, fmt.Errorf("%w: column %q holds category names; give that axis a scale.Ordinal", ErrCategorical, name)
	}
	if t, ok := data.TimeColumn(src, name); ok {
		out := make([]float64, len(t))
		for i, tv := range t {
			if data.IsNull(null, i) {
				out[i] = math.NaN()
				continue
			}
			out[i] = scale.ValueOf(s, tv)
		}
		return out, nil
	}
	v, ok := col.Numbers()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNoColumn, name)
	}
	if !masks(v, null) {
		return v, nil
	}
	out := make([]float64, len(v))
	copy(out, v)
	for i := range out {
		if data.IsNull(null, i) {
			out[i] = math.NaN()
		}
	}
	return out, nil
}

// masks reports whether applying null to vs would change any of them.
//
// It is what keeps the zero-copy path in [data.Float64Columns] intact for the
// source that needs the mask least: an Arrow null is already NaN in a numeric
// column, so the mask agrees with the values and the caller's slice is handed
// on untouched. A source that says a row is absent while leaving a number in
// it — which is the whole point of the interface for text and time, and is
// permitted for numbers — forces the copy, and only then.
func masks(vs []float64, null []bool) bool {
	for i := range vs {
		if data.IsNull(null, i) && finite(vs[i]) {
			return true
		}
	}
	return false
}

// encode maps a column through a categorical scale, naming each value with
// label.
//
// A null row is not encoded at all. Registering it would give the scale a
// category for a value that is not there — "" for a text column, the zero
// time for a temporal one — and an ordinal axis would grow a band for it,
// between the categories somebody did measure.
func encode[T any](cat scale.Categorical, vs []T, null []bool, label func(T) string) []float64 {
	out := make([]float64, len(vs))
	for i, v := range vs {
		if data.IsNull(null, i) {
			out[i] = math.NaN()
			continue
		}
		out[i] = cat.Encode(label(v))
	}
	return out
}

// dropNulls marks a row missing when a column the layer reads *beside* its
// position has no value there.
//
// The position columns need none of this: [column] has already turned their
// nulls into NaN, and a NaN has no position. These are the others — the group
// column, the size column, the width column — where the value is absent but
// the row still has somewhere to go, and where drawing it would mean drawing a
// mark whose series, or whose size, nobody measured. The colour column is
// deliberately not among them: a colour scale has an answer for a value it
// cannot place ([scale.ColorUndefined]), so a row with an unknown category is
// still a reading and is painted in the undefined colour. That colour is
// transparent unless the scale was given one, so such a row is invisible by
// default — which is the answer every unmappable colour value already got,
// reached without a rule of its own.
//
// The copy is made on the first row it has to change and not before, because
// s.x and s.y may be the caller's own slices — the zero-copy path — and
// writing a NaN into those would edit the table rather than the chart.
func dropNulls(src data.Source, s *series, cols ...string) {
	owned := false
	for _, col := range cols {
		if col == "" {
			continue
		}
		null, ok := data.NullMask(src, col)
		if !ok {
			continue
		}
		for i := range s.x {
			if !data.IsNull(null, i) {
				continue
			}
			if !owned {
				s.detach()
				owned = true
			}
			s.x[i], s.y[i] = math.NaN(), math.NaN()
		}
	}
}

// detach replaces the series' position columns with copies it owns.
//
// x and y are the same slice for a layer that summarises one column — see
// [resolveOne] — so the aliasing is preserved rather than broken into two
// copies that happen to hold the same numbers.
func (s *series) detach() {
	alias := len(s.x) > 0 && len(s.y) == len(s.x) && &s.x[0] == &s.y[0]
	s.x = append([]float64(nil), s.x...)
	if alias {
		s.y = s.x
		return
	}
	s.y = append([]float64(nil), s.y...)
}

// finite reports whether v can be plotted.
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// defined reports whether s has a position for v. A scale that excludes part
// of the real line — a log scale, an ordinal one — says so through
// [scale.Definite]; every other scale takes any finite value.
func defined(s scale.Scale, v float64) bool {
	if !finite(v) {
		return false
	}
	if d, ok := s.(scale.Definite); ok {
		return d.Defined(v)
	}
	return true
}

// trainColumn feeds a whole column into a scale in one call.
//
// [scale.Scale.Train] is documented to ignore NaN and infinities, and every
// scale here does — so filtering row by row here was doing the scale's job
// twice, and doing it through a variadic interface method allocated the
// argument slice once per row. On a million-row column that was a million
// allocations to establish two numbers.
func trainColumn(s scale.Scale, vs []float64) { s.Train(vs...) }

// plottable marks the rows a geom can draw: finite values that both scales
// have a position for.
//
// It is computed once per build rather than tested per use, because the
// answer feeds three different traversals — segmenting, interpolating and
// projecting — and they must agree on it exactly.
func (s series) plottable(x, y scale.Scale) []bool {
	ok := make([]bool, len(s.x))
	for i := range s.x {
		ok[i] = defined(x, s.x[i]) && defined(y, s.y[i])
		if ok[i] && s.y2 != nil {
			ok[i] = defined(y, s.y2[i])
		}
	}
	return ok
}

// checkMissing enforces the Error policy. It runs against the scales, so a
// negative reading on a log axis is caught by the same policy that catches a
// NaN — from the chart's point of view they are the same failure.
func (s series) checkMissing(c config, x, y scale.Scale) error {
	if c.missing != Error {
		return nil
	}
	for i, ok := range s.plottable(x, y) {
		if !ok {
			return fmt.Errorf("figure/geom: row %d has no position on the scales and OnMissing(Error) is set", i)
		}
	}
	return nil
}

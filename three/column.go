package three

import (
	"errors"
	"fmt"
	"math"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/scale"
)

// ErrNoColumn reports a layer that was not told which column to read.
var ErrNoColumn = errors.New("figure/three: no column selected")

// ErrCategorical reports a text column mapped onto a continuous axis.
var ErrCategorical = errors.New("figure/three: categorical column on a continuous scale")

// column reads one column as positions on a scale.
//
// It is geom's rule spelled again for this package rather than shared, because
// geom's is unexported: a category is encoded by what it reads as, whatever it
// is stored as; an instant is nanoseconds; a row the source calls absent is a
// NaN, which no scale places, so the layer skips it.
func column(src data.Source, name string, s scale.Scale) ([]float64, error) {
	if name == "" {
		return nil, fmt.Errorf("%w (use geom.X, geom.Y or geom.Z)", ErrNoColumn)
	}
	col, ok := data.ColumnOf(src, name)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNoColumn, name)
	}
	null, _ := data.NullMask(src, name)

	if cat, isCat := s.(scale.Categorical); isCat {
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
		return nil, fmt.Errorf("%w: column %q holds category names; give that axis a scale.Ordinal",
			ErrCategorical, name)
	}
	if t, isTime := data.TimeColumn(src, name); isTime {
		out := make([]float64, len(t))
		for i, tv := range t {
			if data.IsNull(null, i) {
				out[i] = math.NaN()
				continue
			}
			out[i] = scale.Nanos(tv)
		}
		return out, nil
	}
	vs, ok := col.Numbers()
	if !ok {
		return nil, fmt.Errorf("%w: column %q is not a position", ErrCategorical, name)
	}
	if null == nil {
		return vs, nil
	}
	out := make([]float64, len(vs))
	copy(out, vs)
	for i := range out {
		if data.IsNull(null, i) {
			out[i] = math.NaN()
		}
	}
	return out, nil
}

// defined reports whether a scale has a position for v — which is not the same
// question as whether v is finite, because a log scale has no position for
// zero. A layer asks once and skips the rows the answer is no for.
func defined(s scale.Scale, v float64) bool {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return false
	}
	if d, ok := s.(scale.Definite); ok {
		return d.Defined(v)
	}
	return true
}

// at maps a value into the unit cube, and is the one place this package turns
// data into geometry.
func at(s scale.Scale, v float64) float32 { return clamp01(s.Map(v)) }

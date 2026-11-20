package geom

import (
	"fmt"
	"math"
	"sort"

	"github.com/timzifer/figure/scale"
)

// errLength reports two columns of a layer that disagree about how many rows
// there are.
func errLength(a, b string, na, nb int) error {
	return fmt.Errorf("figure/geom: columns %q and %q differ in length (%d vs %d)", a, b, na, nb)
}

// extent is the range of the finite values in a column.
func extent(vs []float64) (lo, hi float64, ok bool) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, v := range vs {
		if finite(v) {
			lo, hi, ok = math.Min(lo, v), math.Max(hi, v), true
		}
	}
	return lo, hi, ok
}

// smallestGap is the spacing between adjacent positions in data units: the
// smallest distance between distinct values. Using the smallest rather than
// the average means marks never overlap on irregularly spaced data.
//
// It sorts, so it needs somewhere to sort: buf is the caller's own buffer,
// which the plottable values are gathered into and which is handed back grown
// for the next call. That is the same bargain every buffer sized by the data
// makes here, and it is not decoration — a layer measures its slot on every
// Train, Train runs on every frame, and a copy of the column per frame is most
// of what a hundred-thousand-row frame would otherwise allocate. The buffer
// lives on the layer rather than in the frame's pool for the reason the group
// index does: Train runs outside a Build, where there is no scratch to take.
func smallestGap(buf, vs []float64) (float64, []float64) {
	xs := buf[:0]
	for _, v := range vs {
		if finite(v) {
			xs = append(xs, v)
		}
	}
	if len(xs) < 2 {
		return 1, xs
	}
	sort.Float64s(xs)
	gap := math.Inf(1)
	for i := 1; i < len(xs); i++ {
		if d := xs[i] - xs[i-1]; d > 0 && d < gap {
			gap = d
		}
	}
	if math.IsInf(gap, 0) {
		return 1, xs
	}
	return gap, xs
}

// markSpan returns the mapped edges of a mark centred on data position x —
// device coordinates under a Cartesian coord, and an angle or a radius under
// another one, which is the coord's business rather than the geom's.
//
// A band scale knows the width of a slot and is asked for it. On a continuous
// axis the width is halfWidth in data units on each side, mapped through the
// scale — so a bar on a log axis is narrower on its high side, as it must be.
func markSpan(f Frame, x, halfWidth float64) (float32, float32) {
	return slotOn(f.X, x, halfWidth)
}

// slotOn is markSpan against a named scale, which is what a mark with width on
// the *vertical* axis needs — a ridgeline's slot is a row rather than a column.
func slotOn(s scale.Scale, v, halfWidth float64) (float32, float32) {
	if band, ok := s.(scale.Band); ok {
		c, w := s.Map(v), band.Bandwidth()
		return c - w/2, c + w/2
	}
	a, b := s.Map(v-halfWidth), s.Map(v+halfWidth)
	if b < a {
		a, b = b, a
	}
	return a, b
}

// baselineOn is [baselinePos] or [baselineAcross], whichever axis the value
// runs along.
func baselineOn(f Frame, c config, v float64) float32 {
	if c.orient == Horizontal {
		return baselineAcross(f, v)
	}
	return baselinePos(f, v)
}

// baselinePos maps the value a bar or area grows from.
//
// A log axis has no position for zero, which is the default baseline. Falling
// back to the bottom of the plot is the only honest answer: the bar still
// starts at the axis, and the axis still says where that is.
func baselinePos(f Frame, v float64) float32 {
	if defined(f.Y, v) {
		return f.Y.Map(v)
	}
	lo, hi := f.Y.Domain()
	if f.Y.Map(lo) > f.Y.Map(hi) {
		return f.Y.Map(lo)
	}
	return f.Y.Map(hi)
}

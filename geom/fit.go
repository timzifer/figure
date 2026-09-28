package geom

import (
	"math"

	"github.com/timzifer/figure/scale"
)

// The training a layer does for a value axis that fits its view: the three
// shapes [Training.Within] asks for, written once so that each mark states
// which shape it is rather than how to be one. Each falls back to training on
// every row when the window is nil, which is every panel that does not fit.
// See docs/adr/0089-a-value-axis-fits-what-its-time-axis-shows.md.

// trainPoints trains y on the rows whose position is in view: a scatter, a
// label.
func trainPoints(y scale.Scale, w *scale.Interval, xs []float64, ys ...[]float64) {
	if w == nil {
		for _, col := range ys {
			trainColumn(y, col)
		}
		return
	}
	for i, x := range xs {
		if x < w.Lo || x > w.Hi {
			continue
		}
		for _, col := range ys {
			if i < len(col) {
				y.Train(col[i])
			}
		}
	}
}

// trainSpans trains y on the rows whose extent along x overlaps the view, so
// a bar or a candle half in view still fits. span answers a row's extent.
func trainSpans(y scale.Scale, w *scale.Interval, n int, span func(i int) (float64, float64), ys ...[]float64) {
	if w == nil {
		for _, col := range ys {
			trainColumn(y, col)
		}
		return
	}
	for i := range n {
		a, b := span(i)
		if a > b {
			a, b = b, a
		}
		if b < w.Lo || a > w.Hi {
			continue
		}
		for _, col := range ys {
			if i < len(col) {
				y.Train(col[i])
			}
		}
	}
}

// trainConnected trains y on the rows in view and on the value where the path
// crosses each edge of it, because the stretch between the last row inside and
// the first outside is drawn and crosses the edge at a value no row holds.
//
// Rows are joined in row order within their group — of is each row's group, or
// nil for one — which is the order a path is drawn in. hold is a staircase,
// which crosses the edge at one of its two rows' values rather than between
// them; both are trained, which is at worst the height of one step looser.
func trainConnected(y scale.Scale, w *scale.Interval, xs, ys []float64, of []int, hold bool) {
	if w == nil {
		trainColumn(y, ys)
		return
	}
	n := min(len(xs), len(ys))
	groups := 1
	for _, g := range of {
		groups = max(groups, g+1)
	}
	last := make([]int, groups)
	for g := range last {
		last[g] = -1
	}
	for i := range n {
		x, v := xs[i], ys[i]
		if !finite(x) || !finite(v) {
			continue
		}
		if x >= w.Lo && x <= w.Hi {
			y.Train(v)
		}
		g := 0
		if of != nil && i < len(of) {
			g = of[i]
		}
		if j := last[g]; j >= 0 {
			edgeValues(y, w, xs[j], ys[j], x, v, hold)
		}
		last[g] = i
	}
}

// edgeValues trains the values at which the segment from (x0, v0) to (x1, v1)
// crosses the edges of the view.
func edgeValues(y scale.Scale, w *scale.Interval, x0, v0, x1, v1 float64, hold bool) {
	lo, hi := math.Min(x0, x1), math.Max(x0, x1)
	for _, e := range [2]float64{w.Lo, w.Hi} {
		if e <= lo || e >= hi {
			continue
		}
		if hold {
			y.Train(v0, v1)
			continue
		}
		y.Train(v0 + (v1-v0)*(e-x0)/(x1-x0))
	}
}

// viewPadded is the view widened by pad on each side, for a stack whose segments
// have width or whose neighbours the path reaches: trained on the rows in the
// wider window it fits loosely, which is the direction a fit may err in.
func viewPadded(w *scale.Interval, pad float64) *scale.Interval {
	if w == nil {
		return nil
	}
	if !(pad > 0) || math.IsInf(pad, 0) {
		pad = 0
	}
	return &scale.Interval{Lo: w.Lo - pad, Hi: w.Hi + pad}
}

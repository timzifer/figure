package stat

import (
	"math"
	"sort"
)

// The named indicators. Each is here because the obvious composition of the
// functions beside it gives the wrong numbers — docs/adr/0092 states the rule
// — and each takes the contract the trailing windows take: xs ascending, the
// columns finite, the shorter one wins, and an Append form that allocates
// nothing into a warm slice.

// RSI is Wilder's relative strength index over a window of rows, in [0, 100].
//
// It is not a composition of [EMA]. Its averages of gains and of losses are
// Wilder's: seeded with the simple mean of the first window changes, then
// moved 1/window of the way towards each new change — not the 2/(window+1)
// an EMA moves. A reader told "RSI 14" expects that definition, and an RSI
// built from EMA is a curve close enough to be trusted and wrong enough to be
// noticed.
//
// The first row has no change and is NaN. Until a window of changes has
// arrived, the averages are the simple mean of the changes so far, as the
// trailing windows compute their first rows over what exists. A stretch with
// no losses is 100, one with no gains 0, and one with neither 50.
func RSI(xs, ys []float64, window int) []Point { return AppendRSI(nil, xs, ys, window) }

// AppendRSI is [RSI] writing into a caller-owned slice. dst is truncated first.
func AppendRSI(dst []Point, xs, ys []float64, window int) []Point {
	dst = dst[:0]
	n := min(len(xs), len(ys))
	if n == 0 {
		return dst
	}
	w := trailingWindow(window, n)
	dst = append(dst, Point{X: xs[0], Y: math.NaN()})
	var gain, loss float64
	for i := 1; i < n; i++ {
		d := ys[i] - ys[i-1]
		g, l := math.Max(d, 0), math.Max(-d, 0)
		if k := float64(i); i <= w {
			// The seed: the simple mean of the changes so far.
			gain += (g - gain) / k
			loss += (l - loss) / k
		} else {
			gain += (g - gain) / float64(w)
			loss += (l - loss) / float64(w)
		}
		dst = append(dst, Point{X: xs[i], Y: rsiOf(gain, loss)})
	}
	return dst
}

func rsiOf(gain, loss float64) float64 {
	switch {
	case gain == 0 && loss == 0:
		return 50
	case loss == 0:
		return 100
	}
	return 100 - 100/(1+gain/loss)
}

// VWAP is the volume-weighted average price of each session so far: at each
// row, the sum of price times volume over the sum of volume since the session
// the row falls in began.
//
// It is two cumulative sums divided, and it is here because of the one thing
// that composition forgets: the sums start again at every session. A VWAP
// that runs across the overnight is not the number any platform shows.
// edges are the session starts — a calendar's opens,
// [github.com/timzifer/figure/scale.Opens], in the axis's units, as
// [OHLCAt] takes them — ascending; rows before the first edge are NaN, and a
// nil edges is one session from the first row. A session that has traded no
// volume yet is NaN.
func VWAP(ts, ps, vs, edges []float64) []Point { return AppendVWAP(nil, ts, ps, vs, edges) }

// AppendVWAP is [VWAP] writing into a caller-owned slice. dst is truncated
// first.
func AppendVWAP(dst []Point, ts, ps, vs, edges []float64) []Point {
	dst = dst[:0]
	n := min(len(ts), len(ps), len(vs))
	session := -2
	var pv, v float64
	for i := range n {
		k := -1
		if len(edges) > 0 {
			k = sort.SearchFloat64s(edges, ts[i])
			if k == len(edges) || edges[k] != ts[i] {
				k--
			}
			if k < 0 {
				dst = append(dst, Point{X: ts[i], Y: math.NaN()})
				continue
			}
		}
		if k != session {
			session, pv, v = k, 0, 0
		}
		if finite(ps[i]) && finite(vs[i]) {
			pv += ps[i] * vs[i]
			v += vs[i]
		}
		y := math.NaN()
		if v > 0 {
			y = pv / v
		}
		dst = append(dst, Point{X: ts[i], Y: y})
	}
	return dst
}

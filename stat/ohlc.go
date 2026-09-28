package stat

import (
	"math"
	"sort"
)

// Candle is one interval of a price column: where it opened and closed, how
// far it ranged, and how much traded in it.
//
// Start is the interval's own left edge, not the time of its first trade: a
// candle is drawn at the interval it summarises, and two candles for adjacent
// minutes should sit a minute apart whichever second their first trade fell in.
type Candle struct {
	Start float64
	// End is where the interval stops: the start of the next one for [OHLC]
	// and [OHLCAt], and for the last candle of OHLCAt — whose period the data
	// has not closed — the time of its last row.
	End                    float64
	Open, High, Low, Close float64
	// Volume is the sum of the weights the rows in it carried, or their count
	// when the call had no weights.
	Volume float64
	// Count is how many rows fell in the interval.
	Count int
}

// Up reports whether the candle closed at or above where it opened — the
// question its colour answers. A candle that closed where it opened is up, so
// that every candle is one of the two colours rather than a third nobody
// assigned.
func (c Candle) Up() bool { return c.Close >= c.Open }

// OHLC summarises a price column into candles of a fixed width.
//
// ts are the times of the rows, ascending, in whatever unit the caller's axis
// uses — nanoseconds for a [github.com/timzifer/figure/scale.Time] axis, which
// is what [github.com/timzifer/figure/scale.Nanos] converts to. ps are the
// prices and vs the volumes; pass nil for vs to have each candle's Volume be
// its row count. The shortest column wins.
//
// The intervals are [origin + k·width, origin + (k+1)·width). origin is what
// aligns them to something a reader recognises: the Unix epoch aligns an hour
// to the clock hour, but a day to UTC midnight, and a market that opens at
// 09:30 in New York wants its days to start there. Pass the session's first
// open and every candle starts where a session does.
//
// An interval nobody traded in has no candle. Inventing one — at the previous
// close, with no range — would be a price the market never printed, and a gap
// in the candles is also the gap a reader expects to see over a halt.
//
// Rows that are not finite are skipped. A column with rows out of order is
// summarised in the order given, which puts a late row into a candle of its
// own; sorting would need a copy of the column, for the reason [Loess] gives.
func OHLC(ts, ps, vs []float64, origin, width float64) []Candle {
	return AppendOHLC(nil, ts, ps, vs, origin, width)
}

// AppendOHLC is [OHLC] writing into a caller-owned slice. dst is truncated
// first, so a live chart resampling every frame does it in the same memory.
func AppendOHLC(dst []Candle, ts, ps, vs []float64, origin, width float64) []Candle {
	dst = dst[:0]
	if !(width > 0) || !finite(width) || !finite(origin) {
		return dst
	}
	n := min(len(ts), len(ps))
	if vs != nil {
		n = min(n, len(vs))
	}
	var cur Candle
	open := false
	for i := range n {
		t, p := ts[i], ps[i]
		if !finite(t) || !finite(p) {
			continue
		}
		v := 1.0
		if vs != nil {
			if v = vs[i]; !finite(v) {
				continue
			}
		}
		start := origin + math.Floor((t-origin)/width)*width
		if !open || start != cur.Start {
			if open {
				dst = append(dst, cur)
			}
			cur, open = Candle{Start: start, End: start + width, Open: p, High: p, Low: p}, true
		}
		cur.High = max(cur.High, p)
		cur.Low = min(cur.Low, p)
		cur.Close = p
		cur.Volume += v
		cur.Count++
	}
	if open {
		dst = append(dst, cur)
	}
	return dst
}

// OHLCAt summarises a price column into candles between consecutive edges:
// the k-th candle covers [edges[k], edges[k+1]) and the last one everything
// from the last edge on.
//
// It is [OHLC] for periods that are not a fixed width — a session, a shift,
// a day across a daylight saving change, which is 23 or 25 hours long. The
// edges are the caller's, typically the opens of a calendar:
//
//	opens, _ := scale.Opens(cal, from, to)
//	edges := make([]float64, len(opens))
//	for i, o := range opens {
//		edges[i] = scale.Nanos(o)
//	}
//	candles := stat.OHLCAt(ts, ps, vs, edges)
//
// edges must be ascending; rows before the first edge are not counted. Each
// candle's Start is its edge. As in OHLC, a period nobody traded in has no
// candle, rows that are not finite are skipped, and vs may be nil.
func OHLCAt(ts, ps, vs, edges []float64) []Candle {
	return AppendOHLCAt(nil, ts, ps, vs, edges)
}

// AppendOHLCAt is [OHLCAt] writing into a caller-owned slice. dst is truncated
// first.
//
// Each row finds its period by a binary search over the edges, so a column out
// of order is still bucketed correctly, one candle per run of rows in the
// same period.
func AppendOHLCAt(dst []Candle, ts, ps, vs, edges []float64) []Candle {
	dst = dst[:0]
	if len(edges) == 0 {
		return dst
	}
	n := min(len(ts), len(ps))
	if vs != nil {
		n = min(n, len(vs))
	}
	var cur Candle
	at, open := -1, false
	for i := range n {
		t, p := ts[i], ps[i]
		if !finite(t) || !finite(p) {
			continue
		}
		v := 1.0
		if vs != nil {
			if v = vs[i]; !finite(v) {
				continue
			}
		}
		k := sort.SearchFloat64s(edges, t)
		if k == len(edges) || edges[k] != t {
			k--
		}
		if k < 0 {
			continue
		}
		if !open || k != at {
			if open {
				dst = append(dst, cur)
			}
			end := t
			if k+1 < len(edges) {
				end = edges[k+1]
			}
			cur, at, open = Candle{Start: edges[k], End: end, Open: p, High: p, Low: p}, k, true
		}
		if k+1 == len(edges) {
			cur.End = max(cur.End, t)
		}
		cur.High = max(cur.High, p)
		cur.Low = min(cur.Low, p)
		cur.Close = p
		cur.Volume += v
		cur.Count++
	}
	if open {
		dst = append(dst, cur)
	}
	return dst
}

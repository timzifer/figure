package stat

import (
	"errors"
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

// Resampler is [OHLC] for a feed that arrives a tick at a time: each tick
// either revises the open candle or opens the next, and Add says which.
//
//	r := stat.Resampler{Origin: scale.Nanos(open), Width: float64(time.Minute)}
//	for tick := range ticks {
//		c, fresh, err := r.Add(scale.Nanos(tick.At), tick.Price, tick.Size)
//		if err != nil {
//			continue // a tick for a candle that has already closed
//		}
//		row := []float64{c.Start, c.End, c.Open, c.High, c.Low, c.Close, c.Volume}
//		if fresh {
//			stream.Append(row...)
//		} else {
//			stream.ReplaceLast(row...)
//		}
//	}
//
// The periods are Width wide from Origin, as OHLC's are, or — when Edges is
// set — between consecutive edges, as [OHLCAt]'s are; Edges wins. Over the
// same ticks in the same order, the candles it produces are exactly those
// functions'. The zero value has no width and no edges and refuses every
// tick.
//
// It is a struct with state because the state is the point: the open candle.
// [Resampler.Reset] forgets it, so a chart restarting its feed reuses the
// value. See docs/adr/0089-a-value-axis-fits-what-its-time-axis-shows.md.
type Resampler struct {
	Origin, Width float64
	Edges         []float64

	cur  Candle
	at   int // the open candle's period: its index among Edges, or its multiple of Width
	open bool
}

// ErrLateTick is returned by [Resampler.Add] for a tick that belongs before
// the open candle — to a period that has closed.
var ErrLateTick = errors.New("figure/stat: tick belongs to a candle that has closed")

// ErrNoPeriod is returned by [Resampler.Add] for a tick with no period: not
// finite, before the first edge, or on a resampler with no width and no edges.
var ErrNoPeriod = errors.New("figure/stat: tick has no period")

// Add folds one tick into the candles and returns the candle it is now part
// of, and whether that candle is new. A tick for a period that has closed is
// [ErrLateTick] and changes nothing: that candle is history. v is the tick's
// volume; pass 1 to count ticks.
func (r *Resampler) Add(t, p, v float64) (Candle, bool, error) {
	if !finite(t) || !finite(p) || !finite(v) {
		return r.cur, false, ErrNoPeriod
	}
	k, start, end, ok := r.period(t)
	if !ok {
		return r.cur, false, ErrNoPeriod
	}
	if r.open && k < r.at {
		return r.cur, false, ErrLateTick
	}
	fresh := !r.open || k != r.at
	if fresh {
		r.cur = Candle{Start: start, End: end, Open: p, High: p, Low: p}
		r.at, r.open = k, true
	}
	r.cur.High = max(r.cur.High, p)
	r.cur.Low = min(r.cur.Low, p)
	r.cur.Close = p
	r.cur.Volume += v
	r.cur.Count++
	if len(r.Edges) > 0 && k == len(r.Edges)-1 {
		// The last period has no edge after it, so it ends where its data
		// does — OHLCAt's rule.
		r.cur.End = max(r.cur.End, t)
	}
	return r.cur, fresh, nil
}

// Current is the open candle, and false before the first tick.
func (r *Resampler) Current() (Candle, bool) { return r.cur, r.open }

// Reset forgets the open candle, keeping the periods.
func (r *Resampler) Reset() { r.cur, r.at, r.open = Candle{}, 0, false }

// period is the period t falls in: an index that orders periods, and its
// start and end.
func (r *Resampler) period(t float64) (int, float64, float64, bool) {
	if n := len(r.Edges); n > 0 {
		k := sort.SearchFloat64s(r.Edges, t)
		if k == n || r.Edges[k] != t {
			k--
		}
		if k < 0 {
			return 0, 0, 0, false
		}
		end := t
		if k+1 < n {
			end = r.Edges[k+1]
		}
		return k, r.Edges[k], end, true
	}
	if !(r.Width > 0) || !finite(r.Width) || !finite(r.Origin) {
		return 0, 0, 0, false
	}
	m := math.Floor((t - r.Origin) / r.Width)
	start := r.Origin + m*r.Width
	return int(m), start, start + r.Width, true
}

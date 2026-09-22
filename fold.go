package figure

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/scale"
)

// ErrSpanColumn is returned by [SpansWhere] and its siblings when a column
// they were told to read is not in the table, or is not of the kind the
// helper reads.
var ErrSpanColumn = errors.New("figure: span column missing or of the wrong kind")

// SpansWhere reads the stretches of time a table's rows cover where column
// col spells value — the periods a machine's state column says "Inaktiv" —
// ready to be folded out of a time axis:
//
//	idle, err := figure.SpansWhere(states, "start", "end", "state", "Inaktiv")
//	p.X(scale.Time(scale.TimeFold(idle...)))
//
// from and to are time columns. A row missing either bound is skipped, and
// the spans come back sorted and with overlapping or touching ones merged, so
// the list is the axis's rather than the table's.
//
// It lives here rather than in scale because a scale never reads a table and
// a table never knows what an axis is: the chart is the one place that holds
// both. The document a chart is written into holds the spans, not the query.
// See docs/adr/0083-an-axis-break-is-marked-or-not-drawn.md.
func SpansWhere(src data.Source, from, to, col, value string) ([]scale.TimeSpan, error) {
	c, ok := data.ColumnOf(src, col)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrSpanColumn, col)
	}
	return SpansFunc(src, from, to, func(i int) bool { return !null(c, i) && c.Spell(i) == value })
}

// SpansFunc is [SpansWhere] for any test: keep is asked about each row by its
// index in src.
func SpansFunc(src data.Source, from, to string, keep func(i int) bool) ([]scale.TimeSpan, error) {
	f, err := timeColumn(src, from)
	if err != nil {
		return nil, err
	}
	t, err := timeColumn(src, to)
	if err != nil {
		return nil, err
	}
	var out []scale.TimeSpan
	for i := range f.Times {
		if i >= len(t.Times) || null(f, i) || null(t, i) || !keep(i) {
			continue
		}
		a, z := f.Times[i], t.Times[i]
		if z.Before(a) {
			a, z = z, a
		}
		if !a.Before(z) {
			continue
		}
		out = append(out, scale.TimeSpan{From: a, To: z})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].From.Before(out[j].From) })
	merged := out[:0]
	for _, s := range out {
		if n := len(merged); n > 0 && !s.From.After(merged[n-1].To) {
			if s.To.After(merged[n-1].To) {
				merged[n-1].To = s.To
			}
			continue
		}
		merged = append(merged, s)
	}
	return merged, nil
}

// IntervalsWhere is [SpansWhere] for a numeric axis: from and to are number
// columns, and the intervals are ready for [scale.Fold].
func IntervalsWhere(src data.Source, from, to, col, value string) ([]scale.Interval, error) {
	c, ok := data.ColumnOf(src, col)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrSpanColumn, col)
	}
	return IntervalsFunc(src, from, to, func(i int) bool { return !null(c, i) && c.Spell(i) == value })
}

// IntervalsFunc is [IntervalsWhere] for any test.
func IntervalsFunc(src data.Source, from, to string, keep func(i int) bool) ([]scale.Interval, error) {
	f, fok := numberColumn(src, from)
	t, tok := numberColumn(src, to)
	if !fok {
		return nil, fmt.Errorf("%w: %q", ErrSpanColumn, from)
	}
	if !tok {
		return nil, fmt.Errorf("%w: %q", ErrSpanColumn, to)
	}
	var out []scale.Interval
	for i := range f {
		if i >= len(t) || math.IsNaN(f[i]) || math.IsNaN(t[i]) || !keep(i) {
			continue
		}
		a, z := f[i], t[i]
		if z < a {
			a, z = z, a
		}
		if !(a < z) {
			continue
		}
		out = append(out, scale.Interval{Lo: a, Hi: z})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Lo < out[j].Lo })
	merged := out[:0]
	for _, s := range out {
		if n := len(merged); n > 0 && s.Lo <= merged[n-1].Hi {
			if s.Hi > merged[n-1].Hi {
				merged[n-1].Hi = s.Hi
			}
			continue
		}
		merged = append(merged, s)
	}
	return merged, nil
}

func null(c data.Column, i int) bool { return i < len(c.Nulls) && c.Nulls[i] }

func timeColumn(src data.Source, name string) (data.Column, error) {
	c, ok := data.ColumnOf(src, name)
	if !ok || c.Kind != data.KindTime {
		return data.Column{}, fmt.Errorf("%w: %q", ErrSpanColumn, name)
	}
	return c, nil
}

// numberColumn reads a numeric column with its nulls written as NaN, which is
// how every reader in figure sees a missing number.
func numberColumn(src data.Source, name string) ([]float64, bool) {
	c, ok := data.ColumnOf(src, name)
	if !ok {
		return nil, false
	}
	vs, ok := c.Numbers()
	if !ok {
		return nil, false
	}
	if len(c.Nulls) == 0 {
		return vs, true
	}
	out := append([]float64(nil), vs...)
	for i, n := range c.Nulls {
		if n && i < len(out) {
			out[i] = math.NaN()
		}
	}
	return out, true
}

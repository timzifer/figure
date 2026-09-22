package figure_test

// Axis breaks and folds, end to end: a break cuts the panel's clip and splits
// its axis and grid lines, a mark is drawn over the data at each gap, a fold
// read out of a table folds the idle periods of a machine off a time axis, and
// under a coord that cannot mark a break the axis is drawn whole. See
// docs/adr/0083-an-axis-break-is-marked-or-not-drawn.md.

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/backend/svg"
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/facet"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// outlier is the chart a break exists for: one bar thirty times the rest.
func outlier(opts ...figure.Option) *figure.Plot {
	src := figure.NewTable().
		String("site", []string{"A", "B", "C", "D", "E"}).
		Float64("load", []float64{4, 7, 3, 96, 6})
	p := figure.New(append([]figure.Option{figure.Size(560, 360), figure.YTitle("load")}, opts...)...)
	p.X(scale.Ordinal())
	p.Y(scale.Linear(scale.Domain(0, 100), scale.Break(10, 88)))
	p.Add(geom.Bar(src, geom.X("site"), geom.Y("load"), geom.Color(palette.SkyBlue)))
	return p
}

// idle is a machine's state log over one day, with three idle periods.
func idle() *data.Table {
	t0 := time.Date(2026, 3, 2, 6, 0, 0, 0, time.UTC)
	at := func(h float64) time.Time { return t0.Add(time.Duration(h * float64(time.Hour))) }
	return figure.NewTable().
		Time("start", []time.Time{at(0), at(2), at(5), at(6), at(9), at(10.5), at(14)}).
		Time("end", []time.Time{at(2), at(5), at(6), at(9), at(10.5), at(14), at(16)}).
		String("state", []string{"Aktiv", "Inaktiv", "Aktiv", "Störung", "Inaktiv", "Aktiv", "Inaktiv"})
}

func folded(t *testing.T) *figure.Plot {
	t.Helper()
	tbl := idle()
	spans, err := figure.SpansWhere(tbl, "start", "end", "state", "Inaktiv")
	if err != nil {
		t.Fatal(err)
	}
	p := figure.New(figure.Size(640, 220))
	p.X(scale.Time(scale.TimeFold(spans...)))
	p.Y(scale.Ordinal())
	p.Add(geom.Rect(tbl,
		geom.X("start"), geom.X2("end"), geom.Y("state"),
		geom.ColorBy("state", scale.Qualitative(palette.Default)),
	))
	return p
}

func TestGoldenAxisBreakSlash(t *testing.T) { golden(t, "axis-break-slash", outlier()) }

func TestGoldenAxisBreakZigzag(t *testing.T) {
	golden(t, "axis-break-zigzag", outlier(figure.Theme(theme.Light.With(theme.AxisBreaks(theme.BreakZigzag, 0, 0)))))
}

func TestGoldenAxisBreakTime(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var ts []time.Time
	var vs []float64
	for d := 0; d < 60; d++ {
		if d >= 12 && d < 45 {
			continue // a gap in the record
		}
		ts = append(ts, t0.AddDate(0, 0, d))
		vs = append(vs, float64(d%7))
	}
	src := figure.NewTable().Time("t", ts).Float64("v", vs)
	p := figure.New(figure.Size(640, 300))
	p.X(scale.Time(scale.TimeBreak(t0.AddDate(0, 0, 13), t0.AddDate(0, 0, 44))))
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Line(src, geom.X("t"), geom.Y("v")))
	golden(t, "axis-break-time", p)
}

func TestGoldenAxisFolds(t *testing.T) { golden(t, "axis-folds", folded(t)) }

// render draws p to an SVG string.
func renderSVG(t *testing.T, p *figure.Plot) string {
	t.Helper()
	var buf bytes.Buffer
	if err := p.Render(figure.SVGWriter(&buf, svg.Pretty())); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestABreakUnderAPolarCoordIsNotDrawn is the claim the record is named
// after: a coord that cannot mark a break does not get one, so a radial axis
// with a break draws exactly what it draws without.
func TestABreakUnderAPolarCoordIsNotDrawn(t *testing.T) {
	build := func(s scale.Scale) *figure.Plot {
		src := figure.NewTable().String("k", []string{"a", "b", "c"}).Float64("v", []float64{3, 5, 90})
		p := figure.New(figure.Size(400, 400), figure.Coord(coord.Polar()))
		p.X(scale.Ordinal())
		p.Y(s)
		p.Add(geom.Bar(src, geom.X("k"), geom.Y("v")))
		return p
	}
	with := renderSVG(t, build(scale.Linear(scale.Domain(0, 100), scale.Break(10, 80))))
	without := renderSVG(t, build(scale.Linear(scale.Domain(0, 100))))
	if with != without {
		t.Error("a break under a polar coord changed the drawing")
	}
}

// TestAChartWithoutBreaksIsUnchanged guards the golden files by another road:
// a break wholly outside the domain resolves to nothing, and the chart is the
// chart without it, byte for byte.
func TestAChartWithoutBreaksIsUnchanged(t *testing.T) {
	src := figure.Float64Columns(map[string][]float64{"x": {0, 1, 2}, "y": {1, 3, 2}})
	build := func(s scale.Scale) *figure.Plot {
		p := figure.New(figure.Size(400, 300))
		p.X(scale.Linear(scale.Nice())).Y(s)
		p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))
		return p
	}
	with := renderSVG(t, build(scale.Linear(scale.Nice(), scale.Break(50, 60))))
	without := renderSVG(t, build(scale.Linear(scale.Nice())))
	if with != without {
		t.Error("a break outside the domain changed the drawing")
	}
}

// TestABreakCutsTheClip checks that the data is clipped by one rectangle per
// kept piece rather than by the panel, so a bar crossing the break is drawn
// with its middle missing.
func TestABreakCutsTheClip(t *testing.T) {
	out := renderSVG(t, outlier())
	i := strings.Index(out, "<clipPath")
	if i < 0 {
		t.Fatal("no clip path in the drawing")
	}
	clip := out[i:]
	clip = clip[:strings.Index(clip, "</clipPath>")]
	if n := strings.Count(clip, "M"); n != 2 {
		t.Errorf("the clip has %d subpaths, want 2:\n%s", n, clip)
	}
}

func TestSpansWhereMergesAndSkips(t *testing.T) {
	tbl := idle()
	spans, err := figure.SpansWhere(tbl, "start", "end", "state", "Inaktiv")
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 3 {
		t.Fatalf("got %d spans, want 3: %v", len(spans), spans)
	}
	for i := 1; i < len(spans); i++ {
		if !spans[i-1].To.Before(spans[i].From) {
			t.Errorf("spans %d and %d are not sorted and disjoint", i-1, i)
		}
	}
	// Two touching rows are one span.
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Hour) }
	touch := figure.NewTable().
		Time("a", []time.Time{h(3), h(0), h(1)}).
		Time("b", []time.Time{h(4), h(1), h(2)}).
		String("s", []string{"x", "x", "x"})
	got, err := figure.SpansWhere(touch, "a", "b", "s", "x")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].From.Equal(h(0)) || !got[0].To.Equal(h(2)) {
		t.Errorf("got %v, want 0..2 and 3..4", got)
	}
	if _, err := figure.SpansWhere(touch, "a", "nope", "s", "x"); !errors.Is(err, figure.ErrSpanColumn) {
		t.Errorf("a missing column gave %v", err)
	}
	if _, err := figure.SpansWhere(touch, "s", "b", "s", "x"); !errors.Is(err, figure.ErrSpanColumn) {
		t.Errorf("a text bound column gave %v", err)
	}
}

// TestABrokenFacetDrawsTheSameInParallel: panels sharing a broken axis share
// its scale object, and on the parallel path each draws against a snapshot.
// The cuts are resolved on the stack of every call, so the two paths must
// produce the same bytes — ADR 0012's rule, with a break in it.
func TestABrokenFacetDrawsTheSameInParallel(t *testing.T) {
	build := func(parallel bool) *figure.Plot {
		src := figure.NewTable().
			String("g", []string{"a", "a", "a", "b", "b", "b", "c", "c", "c"}).
			String("k", []string{"x", "y", "z", "x", "y", "z", "x", "y", "z"}).
			Float64("v", []float64{3, 95, 5, 4, 6, 91, 97, 2, 7})
		p := figure.New(figure.Size(700, 300), figure.Parallel(parallel))
		p.X(scale.Ordinal())
		p.Y(scale.Linear(scale.Domain(0, 100), scale.Break(10, 88)))
		p.Add(geom.Bar(src, geom.X("k"), geom.Y("v")))
		p.Facet(facet.Wrap("g", facet.Columns(3)))
		return p
	}
	if renderSVG(t, build(true)) != renderSVG(t, build(false)) {
		t.Error("a broken facet drew differently on the parallel path")
	}
}

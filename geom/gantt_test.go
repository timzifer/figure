package geom_test

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// plan is a three-task schedule on a numeric time axis, so that a test can say
// where an edge is without converting an instant.
//
//	a: 0 → 10 on lane 0, half done
//	b: 10 → 20 on lane 1, not started
//	c: 30 → 40 on lane 2, finished
func plan() *data.Table {
	return data.NewTable().
		String("id", []string{"a", "b", "c"}).
		Float64("start", []float64{0, 10, 30}).
		Float64("end", []float64{10, 20, 40}).
		Float64("lane", []float64{0, 1, 2}).
		Float64("done", []float64{0.5, 0, 1})
}

func links(from, to []string, opts ...func(*data.Table)) *data.Table {
	t := data.NewTable().String("before", from).String("after", to)
	for _, o := range opts {
		o(t)
	}
	return t
}

// planFrame is [frame] over a panel wide enough that a ten-pixel stub is small
// against the spans, which is what the routing tests depend on.
func planFrame(t *testing.T, g geom.Geom) (*irtest.Recorder, geom.Frame) {
	t.Helper()
	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("Train: %v", err)
	}
	area := ir.R(0, 0, 400, 300)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)
	return irtest.New(), geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}
}

// --- progress ------------------------------------------------------------

// TestAProgressLayerPaintsEveryCellTwice. The whole span is drawn at the
// unfinished alpha and the finished part over it at full strength, which is
// what makes the pale part and the solid part read as one bar.
func TestAProgressLayerPaintsEveryCellTwice(t *testing.T) {
	g := geom.Rect(plan(), geom.X("start"), geom.X2("end"), geom.Y("lane"),
		geom.ProgressBy("done"), geom.Color(palette.Blue))
	rec, f := planFrame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	fills := rec.Filter("FillPath")
	if len(fills) != 2 {
		t.Fatalf("got %d fills, want 2 — the span and the part of it that is done", len(fills))
	}
	pale, solid := fills[0].Fill.Color, fills[1].Fill.Color
	if pale.A >= solid.A {
		t.Errorf("the unfinished pass is not fainter: alpha %d against %d", pale.A, solid.A)
	}
	if pale.R != solid.R || pale.G != solid.G || pale.B != solid.B {
		t.Errorf("the two passes are different colours (%v, %v); a bar's two parts are one task", pale, solid)
	}
}

// TestTheFinishedPartOfACellStopsAtItsFraction. Half of a span from 0 to 10 is
// the span from 0 to 5, and nothing about the panel changes that.
func TestTheFinishedPartOfACellStopsAtItsFraction(t *testing.T) {
	g := geom.Rect(plan(), geom.X("start"), geom.X2("end"), geom.Y("lane"),
		geom.ProgressBy("done"), geom.Color(palette.Blue))
	rec, f := planFrame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	fills := rec.Filter("FillPath")
	whole, done := fills[0].Path.Bounds(), fills[1].Path.Bounds()

	// Row a is the leftmost cell, and it is the one that is half done. Its
	// finished part therefore starts where the whole span starts and reaches
	// the middle of it.
	if math.Abs(float64(done.Min.X-whole.Min.X)) > 0.01 {
		t.Errorf("the finished part starts at %v, not at the start of the span (%v)", done.Min.X, whole.Min.X)
	}
	x0, x1 := float64(f.X.Map(0)), float64(f.X.Map(10))
	if got, want := float64(doneEdgeOfFirstCell(t, fills[1])), (x0+x1)/2; math.Abs(got-want) > 0.01 {
		t.Errorf("half of a span from 0 to 10 reaches %v, want %v", got, want)
	}
}

// doneEdgeOfFirstCell reads the far edge of the first subpath of a fill, which
// is the first cell the layer drew.
func doneEdgeOfFirstCell(t *testing.T, c irtest.Call) float32 {
	t.Helper()
	var first []ir.Point
	c.Path.Walk(func(op ir.PathOp, pts []ir.Point) {
		if op == ir.OpMoveTo && first == nil {
			first = []ir.Point{}
		}
		if first != nil && len(first) < 4 {
			first = append(first, pts...)
		}
	})
	max := first[0].X
	for _, p := range first {
		if p.X > max {
			max = p.X
		}
	}
	return max
}

// TestATaskThatHasNotStartedDrawsNoFinishedPart. A subpath of no area is still
// a mark a pointer could be told it was inside, which is a hit on a bar that
// has nothing in it.
func TestATaskThatHasNotStartedDrawsNoFinishedPart(t *testing.T) {
	src := data.NewTable().
		Float64("start", []float64{0}).
		Float64("end", []float64{10}).
		Float64("lane", []float64{0}).
		Float64("done", []float64{0})
	g := geom.Rect(src, geom.X("start"), geom.X2("end"), geom.Y("lane"),
		geom.ProgressBy("done"), geom.Color(palette.Blue))
	rec, f := planFrame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, c := range rec.Filter("FillPath") {
		if c.Path.Empty() {
			continue
		}
		if b := c.Path.Bounds(); b.Min.X == b.Max.X {
			t.Fatal("a zero-width cell was drawn; an unstarted task has no finished part")
		}
	}
}

// TestAnUnknownProgressIsAWholeBar. A null in the column is a task whose
// progress nobody reported, and a bar drawn empty would report that it had not
// started — which is a reading of the data rather than of its absence.
func TestAnUnknownProgressIsAWholeBar(t *testing.T) {
	src := data.NewTable().
		Float64("start", []float64{0}).
		Float64("end", []float64{10}).
		Float64("lane", []float64{0}).
		Float64("done", []float64{math.NaN()})
	g := geom.Rect(src, geom.X("start"), geom.X2("end"), geom.Y("lane"),
		geom.ProgressBy("done"), geom.Color(palette.Blue))
	rec, f := planFrame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	fills := rec.Filter("FillPath")
	if len(fills) != 2 {
		t.Fatalf("got %d fills, want 2", len(fills))
	}
	if fills[0].Path.Bounds() != fills[1].Path.Bounds() {
		t.Errorf("an unknown progress drew %v over %v; it should cover the whole bar",
			fills[1].Path.Bounds(), fills[0].Path.Bounds())
	}
}

// TestACellBoundedOnBothAxesHasNoAxisForItsProgress.
func TestACellBoundedOnBothAxesHasNoAxisForItsProgress(t *testing.T) {
	src := data.NewTable().
		Float64("x0", []float64{0}).Float64("x1", []float64{1}).
		Float64("y0", []float64{0}).Float64("y1", []float64{1}).
		Float64("done", []float64{0.5})
	g := geom.Rect(src, geom.X("x0"), geom.X2("x1"), geom.Y("y0"), geom.Y2("y1"),
		geom.ProgressBy("done"))
	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrProgressAxis) {
		t.Fatalf("Train: %v, want ErrProgressAxis", err)
	}
}

// TestAVerticalSpanFillsUpwards. A gantt drawn down the page is the same chart
// turned a quarter turn, and the fraction still grows from the edge the row
// named first.
func TestAVerticalSpanFillsUpwards(t *testing.T) {
	src := data.NewTable().
		Float64("lane", []float64{0}).
		Float64("start", []float64{0}).
		Float64("end", []float64{10}).
		Float64("done", []float64{0.5})
	g := geom.Rect(src, geom.X("lane"), geom.Y("start"), geom.Y2("end"),
		geom.ProgressBy("done"), geom.Color(palette.Blue))
	rec, f := planFrame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	fills := rec.Filter("FillPath")
	whole, done := fills[0].Path.Bounds(), fills[1].Path.Bounds()
	if math.Abs(float64(done.Max.Y-whole.Max.Y)) > 0.01 {
		t.Errorf("the finished part is anchored at %v, not at the start of the span (%v)", done.Max.Y, whole.Max.Y)
	}
	if h, want := whole.Max.Y-done.Min.Y, (whole.Max.Y-whole.Min.Y)/2; math.Abs(float64(h-want)) > 0.01 {
		t.Errorf("half a vertical span is %v tall, want %v", h, want)
	}
}

// --- linkage -------------------------------------------------------------

// TestALinkageRoundTripsThroughItsName. The two-letter form is what a document
// writes and what a column holds, so the two spellings have to agree.
func TestALinkageRoundTripsThroughItsName(t *testing.T) {
	for _, k := range []geom.Linkage{
		geom.FinishToStart, geom.StartToStart, geom.FinishToFinish, geom.StartToFinish,
	} {
		back, ok := geom.LinkageNamed(k.String())
		if !ok || back != k {
			t.Errorf("%q read back as %v (known: %v)", k.String(), back, ok)
		}
	}
	if k, ok := geom.LinkageNamed("later-than"); ok || k != geom.FinishToStart {
		t.Errorf("an unknown name gave %v (known: %v); it should be the default", k, ok)
	}
}

// --- dependencies --------------------------------------------------------

// depends is the layer under test over the standard plan.
func depends(l *data.Table, opts ...geom.Option) geom.Geom {
	return geom.Depends(plan(), l, append([]geom.Option{
		geom.X("start"), geom.X2("end"), geom.Y("lane"),
		geom.KeyBy("id"), geom.From("before"), geom.To("after"),
	}, opts...)...)
}

// TestADependencyLayerNeedsAnIdentity. A dependency names two tasks, and a task
// with no name cannot be one of them.
func TestADependencyLayerNeedsAnIdentity(t *testing.T) {
	g := geom.Depends(plan(), links([]string{"a"}, []string{"b"}),
		geom.X("start"), geom.X2("end"), geom.Y("lane"),
		geom.From("before"), geom.To("after"))
	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrNoKey) {
		t.Fatalf("Train: %v, want ErrNoKey", err)
	}
}

// TestAFinishToStartArrowLeavesTheFinishAndArrivesAtTheStart, which is the
// whole of what the default linkage means.
func TestAFinishToStartArrowLeavesTheFinishAndArrivesAtTheStart(t *testing.T) {
	g := depends(links([]string{"b"}, []string{"c"}))
	rec, f := planFrame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	pts := routeOf(t, rec)
	if got, want := pts[0].X, f.X.Map(20); math.Abs(float64(got-want)) > 0.01 {
		t.Errorf("the arrow leaves at %v, want b's finish at %v", got, want)
	}
	last := pts[len(pts)-1]
	if got, want := last.X, f.X.Map(30); math.Abs(float64(got-want)) > 0.01 {
		t.Errorf("the arrow arrives at %v, want c's start at %v", got, want)
	}
	// Room between the two spans, so the elbow turns once: out, across, in.
	if len(pts) != 4 {
		t.Errorf("the route has %d corners, want 4 — one turn where there is room for one", len(pts))
	}
}

// TestAFinishToFinishArrowJoinsTheFarEdges.
func TestAFinishToFinishArrowJoinsTheFarEdges(t *testing.T) {
	g := depends(links([]string{"b"}, []string{"c"}), geom.Link(geom.FinishToFinish))
	rec, f := planFrame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	pts := routeOf(t, rec)
	if got, want := pts[0].X, f.X.Map(20); math.Abs(float64(got-want)) > 0.01 {
		t.Errorf("the arrow leaves at %v, want b's finish at %v", got, want)
	}
	if got, want := pts[len(pts)-1].X, f.X.Map(40); math.Abs(float64(got-want)) > 0.01 {
		t.Errorf("the arrow arrives at %v, want c's finish at %v", got, want)
	}
}

// TestAnArrowHeadPointsTheWayTheArrowTravels. It is a filled triangle whose tip
// is the arrival point, and it enters a start edge travelling the way the span
// runs.
func TestAnArrowHeadPointsTheWayTheArrowTravels(t *testing.T) {
	g := depends(links([]string{"b"}, []string{"c"}))
	rec, f := planFrame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	fills := rec.Filter("FillPath")
	if len(fills) != 1 {
		t.Fatalf("got %d filled paths, want 1 batch of arrow heads", len(fills))
	}
	b := fills[0].Path.Bounds()
	tip := f.X.Map(30)
	if math.Abs(float64(b.Max.X-tip)) > 0.01 {
		t.Errorf("the head reaches %v, want its tip on c's start at %v", b.Max.X, tip)
	}
	if b.Min.X >= tip {
		t.Error("the head has no body behind its tip; it is not pointing anywhere")
	}
}

// TestALinkToATaskThatIsNotThereIsDropped. Faceting cuts the task table, and a
// constraint whose other end is in the next panel has nothing to point at.
func TestALinkToATaskThatIsNotThereIsDropped(t *testing.T) {
	g := depends(links([]string{"a", "a"}, []string{"b", "elsewhere"}))
	rec, f := planFrame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	strokes := rec.Filter("StrokePath")
	if len(strokes) != 1 {
		t.Fatalf("got %d stroked paths, want 1", len(strokes))
	}
	if n := routeCount(strokes[0].Path); n != 1 {
		t.Errorf("drew %d routes, want 1 — the link to a task nothing is called should be dropped", n)
	}
}

// TestLinksAreBatchedByColourAndKeepOneSubpathEach. A layer of a thousand
// arrows is a call per distinct colour, and a pointer still lands on an arrow
// rather than on the sheet of them.
func TestLinksAreBatchedByColourAndKeepOneSubpathEach(t *testing.T) {
	l := links([]string{"a", "b"}, []string{"b", "c"}, func(tb *data.Table) {
		tb.String("path", []string{"critical", "slack"})
	})
	g := depends(l, geom.ColorBy("path", scale.Named(map[string]ir.Color{
		"critical": palette.Red,
		"slack":    palette.Gray,
	})))
	rec, f := planFrame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	strokes := rec.Filter("StrokePath")
	if len(strokes) != 2 {
		t.Fatalf("got %d stroked paths, want one per colour", len(strokes))
	}
	for i, c := range strokes {
		if n := routeCount(c.Path); n != 1 {
			t.Errorf("colour %d drew %d subpaths, want 1", i, n)
		}
	}
	if strokes[0].Stroke.Color == strokes[1].Stroke.Color {
		t.Error("both batches are the same colour; the link table's column was not read")
	}
}

// TestADependencyLayerReportsTheLinkTableToItsDocument, which is what lets a
// chart with constraints in it be written down and read back.
func TestADependencyLayerReportsTheLinkTableToItsDocument(t *testing.T) {
	l := links([]string{"a"}, []string{"b"})
	g := depends(l, geom.Link(geom.StartToStart))
	d, ok := geom.Describe(g)
	if !ok {
		t.Fatal("a dependency layer cannot describe itself")
	}
	if d.Mark != geom.MarkDepends {
		t.Errorf("mark is %q", d.Mark)
	}
	if d.Links == nil || d.Links.Len() != 1 {
		t.Errorf("the link table did not survive: %v", d.Links)
	}
	if d.Linkage != geom.StartToStart {
		t.Errorf("linkage is %v, want start-to-start", d.Linkage)
	}
	back, err := geom.FromDesc(d)
	if err != nil {
		t.Fatalf("FromDesc: %v", err)
	}
	if _, _, err := buildBoth(t, g, back); err != nil {
		t.Fatal(err)
	}
}

// buildBoth draws two layers and reports whether they emitted the same calls.
func buildBoth(t *testing.T, a, b geom.Geom) (*irtest.Recorder, *irtest.Recorder, error) {
	t.Helper()
	ra, fa := planFrame(t, a)
	if err := a.Build(ra, fa); err != nil {
		return nil, nil, err
	}
	rb, fb := planFrame(t, b)
	if err := b.Build(rb, fb); err != nil {
		return nil, nil, err
	}
	if ra.String() != rb.String() {
		t.Errorf("the rebuilt layer draws something else:\n%s\nagainst\n%s", ra, rb)
	}
	return ra, rb, nil
}

// routeOf returns the corners of the single route the layer drew.
func routeOf(t *testing.T, rec *irtest.Recorder) []ir.Point {
	t.Helper()
	strokes := rec.Filter("StrokePath")
	if len(strokes) != 1 {
		t.Fatalf("got %d stroked paths, want 1", len(strokes))
	}
	var pts []ir.Point
	strokes[0].Path.Walk(func(op ir.PathOp, p []ir.Point) { pts = append(pts, p...) })
	if len(pts) < 2 {
		t.Fatalf("the route has %d points", len(pts))
	}
	return pts
}

// routeCount is how many separate routes a batched path holds.
func routeCount(p *ir.Path) int {
	n := 0
	p.Walk(func(op ir.PathOp, _ []ir.Point) {
		if op == ir.OpMoveTo {
			n++
		}
	})
	return n
}

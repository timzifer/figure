package geom_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// pie is a ring of three slices round a hole, with only the thin one labelled:
// 22..25 of 100 is a slice of eleven degrees just short of three o'clock, where
// its two straight edges run nearly level and a label's height is what it has
// no room for. wider is how much further the thin slice reaches.
func pie(label string, wider float64) *data.Table {
	return data.NewTable().
		Float64("r0", []float64{0, 0, 0}).
		Float64("r1", []float64{1, 1, 1}).
		Float64("lo", []float64{0, 22, 25 + wider}).
		Float64("hi", []float64{22, 25 + wider, 100}).
		String("label", []string{"", label, ""})
}

// pieCentre is where the disc of [pieFrame] is centred, and pieRadius how far
// its rim is from there.
var (
	pieCentre = ir.Point{X: 200, Y: 100}
	pieRadius = float32(90) // the default radius is nine tenths of the half-width
)

func pieLabels(src data.Source, opts ...geom.Option) geom.Geom {
	return geom.Text(src, append([]geom.Option{
		geom.X("r0"), geom.X2("r1"), geom.Y("lo"), geom.Y2("hi"), geom.TextBy("label"),
		geom.FontSize(12),
	}, opts...)...)
}

// pieFrame trains a layer and frames it under a polar coord taking its angle
// from Y, the way render does for a pie, in a panel twice as wide as it is
// high so that there is room beside the pie to call a label out into.
func pieFrame(t *testing.T, g geom.Geom) geom.Frame {
	t.Helper()
	return pieFrameIn(t, g, ir.R(0, 0, 400, 200))
}

func pieFrameIn(t *testing.T, g geom.Geom, area ir.Rect) geom.Frame {
	t.Helper()
	x := scale.Linear(scale.Domain(0, 1))
	y := scale.Linear(scale.Domain(0, 100))
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("Train: %v", err)
	}
	c := coord.Polar(coord.Theta(coord.FromY), coord.Hole(0.4)).Frame(coord.Framing{Area: area, X: x, Y: y})
	return geom.Frame{Area: area, X: x, Y: y, Coord: c, Theme: theme.Light}
}

// The bug: a slice's room was the chord across its middle, which is a width,
// and a label is a width *and* a height. A narrow label in a slice that runs
// nearly level fits the chord and overruns both edges.
func TestALabelMustFitItsSliceAndNotOnlyItsChord(t *testing.T) {
	g := pieLabels(pie("8", 0), geom.MinFontSize(12))
	f := pieFrame(t, g)
	if runs := draw(t, f, g).Filter("Text"); len(runs) != 0 {
		t.Errorf("drew %q across the edges of a slice with no height for it", runs[0].Text.Text)
	}
}

func TestALabelIsShrunkToFitBeforeItIsDropped(t *testing.T) {
	g := pieLabels(pie("8", 0))
	f := pieFrame(t, g)
	runs := draw(t, f, g).Filter("Text")
	if len(runs) != 1 {
		t.Fatalf("drew %d labels, want the one shrunk into its slice", len(runs))
	}
	if s := runs[0].Text.Font.Size; s >= 12 || s < 9 {
		t.Errorf("drawn at %v, want smaller than 12 and no smaller than the default floor of 9", s)
	}
}

func TestALabelIsNotShrunkBelowItsFloor(t *testing.T) {
	g := pieLabels(pie("8", -0.5))
	f := pieFrame(t, g)
	if runs := draw(t, f, g).Filter("Text"); len(runs) != 0 {
		t.Errorf("drew %q at %v, below the floor a label is shrunk to", runs[0].Text.Text, runs[0].Text.Font.Size)
	}
}

// A slice is wider towards its rim, and a label that has no room across its
// middle is moved along it to where it has — and wherever it goes, every
// corner of it is inside the slice.
func TestALabelMovesAlongItsSliceToWhereItFits(t *testing.T) {
	g := pieLabels(pie("8", 0.5))
	f := pieFrame(t, g)
	runs := draw(t, f, g).Filter("Text")
	if len(runs) != 1 || runs[0].Text.Font.Size != 12 {
		t.Fatalf("drew %v, want the label at full size", runs)
	}
	run := runs[0].Text
	mid := float64(pieRadius) * 0.7 // halfway between the hole at 0.4 and the rim
	if d := dist(run.At, pieCentre); d <= mid {
		t.Errorf("label at %.1f from the centre, want moved out past the middle of the slice at %.1f", d, mid)
	}
	m := irtest.New().Measure(run)
	w, h := float64(m.Advance), float64(m.Height())
	for _, c := range [4][2]float64{{-w / 2, -h / 2}, {w / 2, -h / 2}, {-w / 2, h / 2}, {w / 2, h / 2}} {
		x, y := float64(run.At.X)+c[0]-float64(pieCentre.X), float64(run.At.Y)+c[1]-float64(pieCentre.Y)
		r := math.Hypot(x, y)
		turn := math.Atan2(x, -y) / (2 * math.Pi) * 100 // clockwise from twelve, in the data's units
		if r < float64(pieRadius)*0.4 || r > float64(pieRadius) || turn < 22 || turn > 25.5 {
			t.Errorf("corner %v of the label is at radius %.1f and %.2f of the turn, outside the slice", c, r, turn)
		}
	}
}

// Slide(false) keeps a label in the middle of its slice: it is shrunk there
// rather than moved out to where it would fit at full size.
func TestALabelThatMayNotSlideStaysInTheMiddle(t *testing.T) {
	mid := float64(pieRadius) * 0.7
	g := pieLabels(pie("8", 0.25), geom.Slide(false))
	runs := draw(t, pieFrame(t, g), g).Filter("Text")
	if len(runs) != 1 {
		t.Fatalf("drew %d labels, want the one shrunk in the middle of its slice", len(runs))
	}
	if d := dist(runs[0].Text.At, pieCentre); math.Abs(d-mid) > 0.01 || runs[0].Text.Font.Size >= 12 {
		t.Errorf("label %.1f from the centre at %v, want smaller than 12 in the middle at %.1f", d, runs[0].Text.Font.Size, mid)
	}

	// The same label allowed to slide goes out to where it fits at full size.
	g = pieLabels(pie("8", 0.25))
	runs = draw(t, pieFrame(t, g), g).Filter("Text")
	if len(runs) != 1 || runs[0].Text.Font.Size != 12 || dist(runs[0].Text.At, pieCentre) <= mid {
		t.Errorf("drew %v, want the label moved outwards at full size", runs)
	}
}

// With Slide(false) a label the middle of its slice cannot hold is called out
// rather than moved, where the layer allows callouts at all.
func TestALabelThatMayNotSlideIsCalledOut(t *testing.T) {
	g := pieLabels(pie("8", 0), geom.Slide(false), geom.Callout(true))
	r := draw(t, pieFrame(t, g), g)
	runs := r.Filter("Text")
	if len(runs) != 1 || dist(runs[0].Text.At, pieCentre) <= float64(pieRadius) || r.Count("Polyline") != 1 {
		t.Fatalf("drew %v, want the label called out of the pie on a leader", runs)
	}

	// Without Callout it is dropped.
	g = pieLabels(pie("8", 0), geom.Slide(false))
	if runs := draw(t, pieFrame(t, g), g).Filter("Text"); len(runs) != 0 {
		t.Errorf("drew %v, want nothing: the middle has no room and nothing else was allowed", runs)
	}
}

// A label that fits at the layer's size is drawn at it.
func TestALabelThatFitsIsNotShrunk(t *testing.T) {
	g := boxed(gantt(), geom.FontSize(12))
	f := textFrame(t, g, ir.R(0, 0, 400, 100))
	for _, r := range draw(t, f, g).Filter("Text") {
		if r.Text.Font.Size != 12 {
			t.Errorf("%q drawn at %v, want 12", r.Text.Text, r.Text.Font.Size)
		}
	}
}

func TestACalloutWritesTheLabelOutsideThePieWithALeader(t *testing.T) {
	const label = "8 % of all"
	g := pieLabels(pie(label, 0), geom.Callout(true))
	if o, ok := g.(geom.Overhanger); !ok || !o.Overhangs() {
		t.Fatal("a layer that calls labels out does not ask to draw past the coord's clip")
	}
	f := pieFrame(t, g)
	var marks []ir.Point
	f.Rows = rowsFunc(func(at []ir.Point, rows []int) { marks = append(marks, at...) })
	r := draw(t, f, g)

	runs := r.Filter("Text")
	if len(runs) != 1 || runs[0].Text.Text != label {
		t.Fatalf("drew %v, want the whole label once", runs)
	}
	at := runs[0].Text.At
	if d := dist(at, pieCentre); d <= float64(pieRadius) {
		t.Errorf("label at %v, %.1f from the centre, want outside the rim at %v", at, d, pieRadius)
	}
	// The slice is on the right of the pie, so the label reads away from it.
	if at.X <= pieCentre.X || runs[0].Text.H != ir.AlignStart {
		t.Errorf("label at %v aligned %v, want to the right of the pie and starting there", at, runs[0].Text.H)
	}

	lines := r.Filter("Polyline")
	if len(lines) != 1 || len(lines[0].Points) != 3 {
		t.Fatalf("drew %d leaders, want one of three points", len(lines))
	}
	pts := lines[0].Points
	if d := dist(pts[0], pieCentre); math.Abs(d-float64(pieRadius)) > 0.5 {
		t.Errorf("leader starts %.1f from the centre, want on the slice's rim at %v", d, pieRadius)
	}
	if end := pts[2]; end.Y != at.Y || end.X >= at.X {
		t.Errorf("leader ends at %v, want level with and just short of the label at %v", end, at)
	}
	if len(marks) != 1 || !samePoint(marks[0], at) {
		t.Errorf("reported marks %v, want the one label where it was written", marks)
	}
}

// Two thin slices side by side call their labels out onto one side, and the
// second is stacked under the first rather than on top of it.
func TestCalloutsOnOneSideAreStackedApart(t *testing.T) {
	src := data.NewTable().
		Float64("r0", []float64{0, 0, 0, 0}).
		Float64("r1", []float64{1, 1, 1, 1}).
		Float64("lo", []float64{0, 30, 31, 32}).
		Float64("hi", []float64{30, 31, 32, 100}).
		String("label", []string{"", "1 % of all", "1 % more", ""})
	g := pieLabels(src, geom.Callout(true))
	f := pieFrame(t, g)
	runs := draw(t, f, g).Filter("Text")
	if len(runs) != 2 || runs[0].Text.H != runs[1].Text.H {
		t.Fatalf("drew %v, want both called out onto one side", runs)
	}
	h := float32(irtest.New().Measure(ir.TextRun{Text: "0", Font: runs[0].Text.Font}).Height())
	if gap := runs[0].Text.At.Y - runs[1].Text.At.Y; math.Abs(float64(gap)) < float64(h) {
		t.Errorf("labels %v and %v are %.1f apart, want at least a line of %.1f", runs[0].Text.At, runs[1].Text.At, gap, h)
	}
}

// A label with no room beside the pie is dropped with its leader rather than
// cut by the edge of the panel.
func TestACalloutWithNoRoomInThePanelIsDropped(t *testing.T) {
	g := pieLabels(pie("8 % of all", 0), geom.Callout(true))
	f := pieFrameIn(t, g, ir.R(100, 0, 300, 200))
	r := draw(t, f, g)
	if n := len(r.Filter("Text")) + len(r.Filter("Polyline")); n != 0 {
		t.Errorf("drew %d runs and leaders past the edge of the panel", n)
	}
}

// A label that runs a little past the edge of the panel is drawn in on a
// shorter arm rather than dropped.
func TestACalloutJustPastThePanelIsDrawnInOnAShorterArm(t *testing.T) {
	g := pieLabels(pie("8 % of all", 0), geom.Callout(true))
	r := draw(t, pieFrame(t, g), g)
	run := r.Filter("Text")[0].Text
	edge := run.At.X + irtest.New().Measure(run).Advance

	// The same pie in a panel narrowed by the same amount on both sides, so
	// that the disc does not move and the label runs three pixels past it.
	d := 403 - edge
	g = pieLabels(pie("8 % of all", 0), geom.Callout(true))
	f := pieFrameIn(t, g, ir.R(d, 0, 400-d, 200))
	r = draw(t, f, g)
	runs, lines := r.Filter("Text"), r.Filter("Polyline")
	if len(runs) != 1 || len(lines) != 1 {
		t.Fatalf("drew %d labels and %d leaders, want the label kept", len(runs), len(lines))
	}
	if got := runs[0].Text.At.X; math.Abs(float64(got-(run.At.X-3))) > 0.01 {
		t.Errorf("label at x %.2f, want pulled in by three pixels to %.2f", got, run.At.X-3)
	}
	if end := lines[0].Points[2]; end.X >= runs[0].Text.At.X || end.X <= lines[0].Points[1].X {
		t.Errorf("leader %v, want an arm that is shorter but still runs out to the label", lines[0].Points)
	}
}

// narrowPie frames a pie labelled with label in a panel narrowed by the same
// amount on both sides, so that the disc does not move and the label, called
// out at the layer's size on one line, runs over pixels past the edge.
func narrowPie(t *testing.T, label string, over float32, opts ...geom.Option) (geom.Frame, geom.Geom) {
	t.Helper()
	g := pieLabels(pie(label, 0), append([]geom.Option{geom.Callout(true)}, opts...)...)
	runs := draw(t, pieFrame(t, g), g).Filter("Text")
	if len(runs) != 1 {
		t.Fatalf("drew %v in the wide panel, want the label called out on one line", runs)
	}
	edge := runs[0].Text.At.X + irtest.New().Measure(runs[0].Text).Advance
	d := 400 + over - edge
	g = pieLabels(pie(label, 0), append([]geom.Option{geom.Callout(true)}, opts...)...)
	return pieFrameIn(t, g, ir.R(d, 0, 400-d, 200)), g
}

// rightEdge is where the widest of runs ends.
func rightEdge(runs []irtest.Call) float32 {
	m := irtest.New()
	var edge float32
	for _, r := range runs {
		edge = max(edge, r.Text.At.X+m.Measure(r.Text).Advance)
	}
	return edge
}

// A long name out of a slice at three o'clock has only the margin beside the
// pie, and one that is too wide for it on one line is broken over lines where
// Wrap allows it — at the layer's size, before it is shrunk - rather than
// dropped. It was dropped: the name most worth calling out was the one that
// never appeared.
func TestACalloutTooWideForTheMarginIsBrokenOverLines(t *testing.T) {
	const label = "Pause overrun"
	f, g := narrowPie(t, label, 30, geom.Wrap(true))
	r := draw(t, f, g)
	runs, lines := r.Filter("Text"), r.Filter("Polyline")
	if len(runs) < 2 || len(lines) != 1 {
		t.Fatalf("drew %d runs and %d leaders, want the label broken over lines on one leader", len(runs), len(lines))
	}
	for _, run := range runs {
		if run.Text.Font.Size != 12 || run.Text.H != ir.AlignStart {
			t.Errorf("%q drawn at %v aligned %v, want the layer's 12 starting beside the pie", run.Text.Text, run.Text.Font.Size, run.Text.H)
		}
	}
	if edge := rightEdge(runs); edge > f.Area.Max.X+0.01 {
		t.Errorf("block ends at %.2f, past the edge of the panel at %.2f", edge, f.Area.Max.X)
	}
	// The leader meets the block in the middle, not its first line.
	mid := (runs[0].Text.At.Y + runs[len(runs)-1].Text.At.Y) / 2
	if end := lines[0].Points[2]; math.Abs(float64(end.Y-mid)) > 0.01 {
		t.Errorf("leader ends at y %.2f, want the middle of the block at %.2f", end.Y, mid)
	}
}

// Without Wrap the same label is shrunk to the margin, down to the floor.
func TestACalloutTooWideForTheMarginIsShrunk(t *testing.T) {
	f, g := narrowPie(t, "Pause overrun", 15)
	runs := draw(t, f, g).Filter("Text")
	if len(runs) != 1 {
		t.Fatalf("drew %v, want the label shrunk onto one line", runs)
	}
	if s := runs[0].Text.Font.Size; s >= 12 || s < 9 {
		t.Errorf("drawn at %v, want smaller than 12 and no smaller than the default floor of 9", s)
	}
	if edge := rightEdge(runs); edge > f.Area.Max.X+0.01 {
		t.Errorf("label ends at %.2f, past the edge of the panel at %.2f", edge, f.Area.Max.X)
	}

	// A floor at the layer's size leaves nothing to shrink, and the label is
	// dropped with its leader, as before.
	f, g = narrowPie(t, "Pause overrun", 15, geom.MinFontSize(12))
	r := draw(t, f, g)
	if n := len(r.Filter("Text")) + len(r.Filter("Polyline")); n != 0 {
		t.Errorf("drew %d runs and leaders for a label that fits neither at its size nor above its floor", n)
	}
}

// A label broken over lines takes as many lines of the column it is stacked
// in, and its neighbour is pushed along by the whole block.
func TestAWrappedCalloutIsStackedByItsWholeHeight(t *testing.T) {
	src := data.NewTable().
		Float64("r0", []float64{0, 0, 0, 0}).
		Float64("r1", []float64{1, 1, 1, 1}).
		Float64("lo", []float64{0, 30, 31, 32}).
		Float64("hi", []float64{30, 31, 32, 100}).
		String("label", []string{"", "a label broken over lines", "1 %", ""})
	g := pieLabels(src, geom.Callout(true), geom.Wrap(true))
	f := pieFrame(t, g)
	runs := draw(t, f, g).Filter("Text")
	if len(runs) < 3 {
		t.Fatalf("drew %v, want the first label broken and the second beside it", runs)
	}
	m := irtest.New()
	h := m.Measure(ir.TextRun{Text: "0", Font: runs[0].Text.Font}).Height()
	short := runs[len(runs)-1]
	if short.Text.Text != "1 %" {
		t.Fatalf("last run is %q, want the short label stacked under the block", short.Text.Text)
	}
	block := runs[:len(runs)-1]
	bottom := block[len(block)-1].Text.At.Y + h/2
	if top := short.Text.At.Y - h/2; top < bottom {
		t.Errorf("short label's top at %.2f is inside the block ending at %.2f", top, bottom)
	}
}

// Under Cartesian there is no outside to call a label out to, so the option
// changes nothing: the label is dropped, as it would have been.
func TestACalloutNeedsACoordWithAMiddle(t *testing.T) {
	g := boxed(oneRow("Wareneingang Halle 3"), geom.Callout(true))
	f := textFrame(t, g, ir.R(0, 0, 4, 100))
	r := draw(t, f, g)
	if n := len(r.Filter("Text")) + len(r.Filter("Polyline")); n != 0 {
		t.Errorf("drew %d runs and leaders under a coord with no outside", n)
	}
}

func TestCalloutAndFloorDescribeThemselves(t *testing.T) {
	g := boxed(gantt(), geom.Callout(true), geom.MinFontSize(7), geom.Wrap(true), geom.Slide(false))
	d, _ := geom.Describe(g)
	if !d.Callout || d.MinFontSize != 7 || !d.Wrap || !d.Pinned {
		t.Fatalf("described as callout %v, floor %v", d.Callout, d.MinFontSize)
	}
	back, err := geom.FromDesc(d)
	if err != nil {
		t.Fatalf("FromDesc: %v", err)
	}
	if bd, _ := geom.Describe(back); !bd.Callout || bd.MinFontSize != 7 || !bd.Wrap || !bd.Pinned {
		t.Errorf("read back as callout %v, floor %v", bd.Callout, bd.MinFontSize)
	}
}

// A label too wide for its box on one line and short enough for it on two is
// broken at the space that makes the narrowest block, at the layer's size, and
// the lines are stacked about the middle of the box.
func TestWrapBreaksALabelBeforeShrinkingIt(t *testing.T) {
	const label = "Wareneingang Halle 3"
	// Wide enough for "Wareneingang" and not for the whole label; tall
	// enough for two lines and not three.
	m := irtest.New()
	font := theme.Light.Font(12)
	one := m.Measure(ir.TextRun{Text: label, Font: font}).Advance
	word := m.Measure(ir.TextRun{Text: "Wareneingang", Font: font}).Advance
	w := (one + word) / 2
	g := boxed(oneRow(label), geom.FontSize(12), geom.Wrap(true))
	f := textFrame(t, g, ir.R(0, 0, w, 40))
	runs := draw(t, f, g).Filter("Text")
	if len(runs) != 2 {
		t.Fatalf("drew %v, want the label on two lines", runs)
	}
	if runs[0].Text.Text != "Wareneingang" || runs[1].Text.Text != "Halle 3" {
		t.Errorf("broke it into %q and %q, want the break after the long word", runs[0].Text.Text, runs[1].Text.Text)
	}
	for _, r := range runs {
		if r.Text.Font.Size != 12 {
			t.Errorf("%q drawn at %v, want the layer's 12: breaking comes before shrinking", r.Text.Text, r.Text.Font.Size)
		}
	}
	h := m.Measure(ir.TextRun{Text: label, Font: font}).Height()
	if d := runs[1].Text.At.Y - runs[0].Text.At.Y; math.Abs(float64(d-h)) > 0.01 {
		t.Errorf("lines %.2f apart, want one line of %.2f", d, h)
	}
	if mid := (runs[0].Text.At.Y + runs[1].Text.At.Y) / 2; math.Abs(float64(mid-20)) > 0.01 {
		t.Errorf("block centred at %.2f, want the middle of the box at 20", mid)
	}

	// Without Wrap the same label in the same box is shrunk or dropped, and
	// never broken.
	g = boxed(oneRow(label), geom.FontSize(12))
	f = textFrame(t, g, ir.R(0, 0, w, 40))
	if runs := draw(t, f, g).Filter("Text"); len(runs) > 1 {
		t.Errorf("broke a label over %d lines without being asked to", len(runs))
	}
}

// A label with no space in it has nowhere to break and is one line whatever
// Wrap says.
func TestWrapNeedsASpace(t *testing.T) {
	g := boxed(oneRow("Wareneingangsprüfung"), geom.Wrap(true))
	f := textFrame(t, g, ir.R(0, 0, 40, 60))
	if runs := draw(t, f, g).Filter("Text"); len(runs) > 1 {
		t.Errorf("broke a label with no space into %d lines", len(runs))
	}
}

func dist(a, b ir.Point) float64 { return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)) }

package geom_test

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

// A labelled contour writes each level it draws, once per run: the number is in
// the picture rather than only in the colourbar beside it.
func TestALabelledContourWritesItsLevels(t *testing.T) {
	src := field(16, 16, func(x, y float64) float64 { return x*x + y*y })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.Levels(2, 4, 6), geom.LabelLevels(true))

	rec := drawContour(t, g)
	texts := rec.Texts()
	if len(texts) == 0 {
		t.Fatal("a labelled contour wrote nothing on its lines")
	}
	for _, s := range texts {
		switch s {
		case "2", "4", "6":
		default:
			t.Errorf("a contour of 2, 4 and 6 wrote %q", s)
		}
	}
}

// Nothing is written unless it was asked for: the option is the whole switch,
// and a chart that drew labels it was not asked for would move every golden
// file in the repository.
func TestAnUnlabelledContourWritesNothing(t *testing.T) {
	src := field(16, 16, func(x, y float64) float64 { return x*x + y*y })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Levels(2, 4))

	if got := drawContour(t, g).Count("Text"); got != 0 {
		t.Errorf("an unlabelled contour wrote %d runs, want none", got)
	}
}

// The line is gapped for its own label rather than drawn under it: a label over
// its own stroke is the thing this feature exists to avoid, and a halo is ink
// the IR has no word for.
func TestALabelledContourGapsTheLineItWritesOn(t *testing.T) {
	src := field(16, 16, func(x, y float64) float64 { return x*x + y*y })
	level := []float64{4}

	plain := drawContour(t, geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.Levels(level...)))
	labelled := drawContour(t, geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.Levels(level...), geom.LabelLevels(true)))

	if strokedRuns(labelled) <= strokedRuns(plain) {
		t.Errorf("the labelled ring is %d subpaths and the plain one %d; the gap should have split it",
			strokedRuns(labelled), strokedRuns(plain))
	}

	// The gap is where the label is: no stroked point may sit inside the box
	// the text covers.
	for _, run := range labelled.Filter("Text") {
		for _, c := range labelled.Filter("StrokePath") {
			for _, p := range c.Path.Pts {
				if insideLabel(labelled, run.Text, p) {
					t.Fatalf("the line runs through the label at (%.1f, %.1f)", p.X, p.Y)
				}
			}
		}
	}
}

// A label reads left to right whichever way its curve was traced. The turn is
// the chord's, and a chord pointing leftwards is turned a further half turn
// rather than written upside down.
func TestACurveLabelReadsUpright(t *testing.T) {
	src := field(24, 24, func(x, y float64) float64 { return math.Sin(x) * math.Cos(y) })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.LevelCount(5), geom.LabelLevels(true))

	for _, c := range drawContour(t, g).Filter("Text") {
		if a := c.Text.Rotation; a > math.Pi/2+1e-6 || a < -math.Pi/2-1e-6 {
			t.Errorf("the label %q is turned %.2f rad, which reads upside down", c.Text.Text, a)
		}
	}
}

// The same table at the same size draws the same labels in the same places. A
// candidate is chosen by a bounded scan over the run's own points, so nothing
// here depends on how a tie was broken or on how many times a loop ran.
func TestCurveLabelsAreDeterministic(t *testing.T) {
	src := field(20, 20, func(x, y float64) float64 { return math.Sin(x) * math.Cos(y) })
	build := func() []irtest.Call {
		g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
			geom.LevelCount(6), geom.LabelLevels(true))
		return drawContour(t, g).Filter("Text")
	}
	first, second := build(), build()
	if len(first) != len(second) {
		t.Fatalf("one render wrote %d labels and the next %d", len(first), len(second))
	}
	for i := range first {
		a, b := first[i].Text, second[i].Text
		if a.Text != b.Text || a.At != b.At || a.Rotation != b.Rotation {
			t.Errorf("label %d moved between renders: %q at %v turned %v, then %q at %v turned %v",
				i, a.Text, a.At, a.Rotation, b.Text, b.At, b.Rotation)
		}
	}
}

// A run with no room for its label keeps its line. The alternative — gapping a
// ring for a label that does not fit in it — is a chart with a hole in it and
// no explanation of the hole.
func TestATinyRunKeepsItsLineAndTakesNoLabel(t *testing.T) {
	// A single narrow peak: the level just under its summit is a ring a few
	// pixels across.
	src := field(24, 24, func(x, y float64) float64 { return math.Exp(-(x*x + y*y)) })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.Levels(0.96), geom.LabelLevels(true))

	rec := drawContour(t, g)
	if got := rec.Count("Text"); got != 0 {
		t.Errorf("a ring too small for its label wrote %d of them", got)
	}
	if rec.Count("StrokePath") == 0 {
		t.Error("the unlabelled ring was not drawn at all")
	}
}

// A label is refused rather than moved: the placer says yes or no, and a curve
// label that moved would name a level it is not on.
func TestACurveLabelIsRefusedRatherThanMoved(t *testing.T) {
	src := field(20, 20, func(x, y float64) float64 { return x*x + y*y })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.Levels(1, 2, 3, 4, 5, 6, 7, 8), geom.LabelLevels(true))

	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("training the contour: %v", err)
	}
	area := ir.R(0, 0, 300, 200)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)

	rec := irtest.New()
	p := &refusingPlacer{}
	f := geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light, Labels: p}
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("drawing the contour: %v", err)
	}
	if p.asked == 0 {
		t.Fatal("a labelled contour never asked the panel for its labels")
	}
	if got := rec.Count("Text"); got != 0 {
		t.Errorf("the placer refused every label and %d were written anyway", got)
	}
	if got, ok := g.(geom.LabelAvoider); !ok || !got.AvoidsLabels() {
		t.Error("a labelled contour does not ask for the panel's placer")
	}
}

// refusingPlacer is a panel with no room in it: every candidate is refused, so
// a mark that drew a label anyway is a mark that ignored the arbiter.
type refusingPlacer struct{ asked int }

func (p *refusingPlacer) PlaceLabel(run ir.TextRun, move bool) (ir.Point, bool) {
	p.asked++
	return run.At, false
}

// A locus labels its curves through the same helper, which is the point of the
// helper: by the time a run reaches it there is nothing left of where it came
// from.
func TestALabelledLocusWritesItsLevels(t *testing.T) {
	x := scale.Linear(scale.Domain(-360, 0))
	y := scale.Linear(scale.Domain(-40, 40))
	g := geom.Locus(stat.NicholsM, []float64{-6, 0, 6}, geom.LabelLevels(true))

	area := ir.R(0, 0, 400, 300)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)
	rec := irtest.New()
	if err := g.Build(rec, geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}); err != nil {
		t.Fatalf("drawing the locus: %v", err)
	}
	if len(rec.Texts()) == 0 {
		t.Fatal("a labelled locus wrote nothing on its contours")
	}
	for _, s := range rec.Texts() {
		if !strings.Contains("-6 0 6", s) {
			t.Errorf("a family of -6, 0 and 6 wrote %q", s)
		}
	}
}

// The format is the caller's where they gave one, and a plain decimal where
// they did not.
func TestLevelFormatWritesWhatItWasGiven(t *testing.T) {
	src := field(16, 16, func(x, y float64) float64 { return x*x + y*y })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.Levels(4), geom.LabelLevels(true),
		geom.LevelFormat(func(v float64) string { return "level " + strconv.FormatFloat(v, 'f', 1, 64) }))

	for _, s := range drawContour(t, g).Texts() {
		if !strings.HasPrefix(s, "level ") {
			t.Errorf("the layer wrote %q, which is not what the format says", s)
		}
	}
}

// strokedRuns is how many separate runs a recording strokes, which is what a
// gap adds one of.
func strokedRuns(rec *irtest.Recorder) int {
	n := 0
	for _, c := range rec.Filter("StrokePath") {
		for _, op := range c.Path.Ops {
			if op == ir.OpMoveTo {
				n++
			}
		}
	}
	return n
}

// insideLabel reports whether a device point falls inside the box a written
// run covers, measured in the label's own turned frame.
func insideLabel(rec *irtest.Recorder, run ir.TextRun, p ir.Point) bool {
	m := rec.Measure(run)
	s, c := math.Sincos(-float64(run.Rotation))
	dx, dy := float64(p.X-run.At.X), float64(p.Y-run.At.Y)
	u, v := dx*c-dy*s, dx*s+dy*c
	return math.Abs(u) < float64(m.Advance)/2 && math.Abs(v) < float64(m.Height())/2
}

package geom_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

// locusFrame is a panel with pinned domains, which is what a chart a family is
// printed under has: the extent of a Nichols diagram or a Smith chart is a
// convention of its field rather than an answer to the data.
func locusFrame(x, y scale.Scale, c coord.Coord, size float32) geom.Frame {
	area := ir.R(0, 0, size, size)
	x.SetRange(0, size)
	y.SetRange(size, 0)
	f := geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}
	if c != nil {
		f.Coord = c.Frame(coord.Framing{Area: area, X: x, Y: y})
	}
	return f
}

func nicholsAxes() (scale.Scale, scale.Scale) {
	return scale.Linear(scale.Domain(-360, 0)), scale.Linear(scale.Domain(-40, 40))
}

func smithAxes() (scale.Scale, scale.Scale) {
	return scale.Linear(scale.Domain(0, 50)), scale.Linear(scale.Domain(-50, 50))
}

// runs is every polyline a layer drew, whichever primitive the coord made it:
// straight edges reach the backend as a polyline, and curved ones as a path.
func runs(r *irtest.Recorder) [][]ir.Point {
	var out [][]ir.Point
	for _, c := range r.Calls {
		switch c.Op {
		case "Polyline":
			out = append(out, c.Points)
		case "StrokePath":
			var run []ir.Point
			c.Path.Walk(func(op ir.PathOp, pts []ir.Point) {
				if op == ir.OpMoveTo && len(run) > 0 {
					out, run = append(out, run), nil
				}
				if len(pts) > 0 {
					run = append(run, pts[len(pts)-1])
				}
			})
			if len(run) > 0 {
				out = append(out, run)
			}
		}
	}
	return out
}

// The mark's whole shape: one family, one level list, one run per curve — and
// a Nichols family repeating with the phase draws more runs than it has levels,
// which is the point of it being handed the extent.
func TestALocusDrawsACurvePerLevel(t *testing.T) {
	x, y := smithAxes()
	g := geom.Locus(stat.SmithVSWR, []float64{1.5, 2, 3})
	r := draw(t, locusFrame(x, y, nil, 200), g)
	if got := len(runs(r)); got != 3 {
		t.Errorf("three VSWR circles drew %d runs", got)
	}

	x, y = nicholsAxes()
	g = geom.Locus(stat.NicholsM, []float64{-3, 0, 3})
	if got := len(runs(draw(t, locusFrame(x, y, nil, 200), g))); got < 3 {
		t.Errorf("three M contours drew %d runs over a whole turn of phase", got)
	}
}

// The consequence the record turns on: a family emits impedances, and the coord
// is what makes them the circle a Smith chart prints. Nothing in the mark, and
// nothing in the family, knows this chart exists.
func TestAVSWRLocusIsACircleOnTheDisc(t *testing.T) {
	x, y := smithAxes()
	f := locusFrame(x, y, coord.Smith(), 200)
	r := draw(t, f, geom.Locus(stat.SmithVSWR, []float64{2}))

	got := runs(r)
	if len(got) != 1 {
		t.Fatalf("one VSWR circle drew %d runs", len(got))
	}
	// The middle of the disc is a matched load, and a constant-reflection locus
	// is at a constant distance from it.
	mid := f.Coord.Point(x.Map(1), y.Map(0))
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range got[0] {
		d := math.Hypot(float64(p.X-mid.X), float64(p.Y-mid.Y))
		lo, hi = math.Min(lo, d), math.Max(hi, d)
	}
	if lo <= 0 || (hi-lo)/hi > 0.01 {
		t.Errorf("the circle's radius runs from %v to %v, want one radius", lo, hi)
	}
	// |Γ| = (s−1)/(s+1) = 1/3 of the way out, and the disc is the unit circle.
	rim := math.Hypot(float64(f.Coord.Point(x.Map(0), y.Map(0)).X-mid.X),
		float64(f.Coord.Point(x.Map(0), y.Map(0)).Y-mid.Y))
	if want := rim / 3; math.Abs(hi-want)/want > 0.01 {
		t.Errorf("the VSWR 2 circle sits %v from the middle, want %v — a third of the way to the rim", hi, want)
	}
}

// A locus says what the region of the plane means, so the region is whatever
// the axes already show. The 0 dB M contour runs to −∞ dB, and an axis trained
// on that has no ticks left anywhere a reader is looking.
func TestALocusDoesNotTrainTheAxes(t *testing.T) {
	x, y := scale.Linear(), scale.Linear()
	data := geom.Line(src(map[string][]float64{"x": {-180, -90}, "y": {-6, 6}}), geom.X("x"), geom.Y("y"))
	if err := data.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	x0, x1 := x.Domain()
	y0, y1 := y.Domain()
	if err := geom.Locus(stat.NicholsM, []float64{0, -3}).Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	if a, b := x.Domain(); a != x0 || b != x1 {
		t.Errorf("a locus moved the X domain from [%v, %v] to [%v, %v]", x0, x1, a, b)
	}
	if a, b := y.Domain(); a != y0 || b != y1 {
		t.Errorf("a locus moved the Y domain from [%v, %v] to [%v, %v]", y0, y1, a, b)
	}
}

// The default is reversed rather than removed: a caller drawing a bounded
// family can still ask for the behaviour every other annotation has.
func TestExtendMakesALocusTrain(t *testing.T) {
	x, y := scale.Linear(), scale.Linear()
	g := geom.Locus(stat.SmithVSWR, []float64{3}, geom.Extend(true))
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	// The VSWR 3 circle runs from r = 1/3 to r = 3.
	if lo, hi := x.Domain(); lo > 1.0/3+1e-9 || hi < 3-1e-9 {
		t.Errorf("the X domain is [%v, %v], want it to cover the circle", lo, hi)
	}
	if lo, hi := y.Domain(); !(lo < 0) || !(hi > 0) {
		t.Errorf("the Y domain is [%v, %v], want it to cover both halves of the circle", lo, hi)
	}
}

// The other annotations keep the default they have had since v0.3. Adding a
// flag beside Extend is the kind of change that quietly moves a threshold line.
func TestTheOtherAnnotationsStillExtendByDefault(t *testing.T) {
	x, y := scale.Linear(), scale.Linear()
	if err := geom.HLine(9).Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	if _, hi := y.Domain(); hi < 9 {
		t.Errorf("a reference line stopped extending the domain: it tops out at %v", hi)
	}
}

// A gap in a family is a gap in the chart. It is how a curve that leaves the
// panel and comes back is drawn, and how the two rays of a Q arc stay two.
func TestALocusBreaksItsRunWhereTheFamilyDoes(t *testing.T) {
	x, y := scale.Linear(scale.Domain(0, 10)), scale.Linear(scale.Domain(0, 10))
	g := geom.Locus(broken{}, []float64{1})
	got := runs(draw(t, locusFrame(x, y, nil, 100), g))
	if len(got) != 2 {
		t.Fatalf("a family with one gap in it drew %d runs, want 2", len(got))
	}
	for i, run := range got {
		if len(run) != 2 {
			t.Errorf("run %d has %d points, want 2", i, len(run))
		}
	}
}

// A point no scale can place is the same thing to a mark as a gap, which is the
// missing-value policy every layer has followed since v0.1: the run ends and
// the next one starts after it, rather than a NaN reaching a backend.
func TestALocusBreaksWhereAScaleCannotPlaceAPoint(t *testing.T) {
	x, y := scale.Log(scale.LogDomain(1, 100)), scale.Linear(scale.Domain(0, 10))
	g := geom.Locus(straddling{}, []float64{1})
	got := runs(draw(t, locusFrame(x, y, nil, 100), g))
	if len(got) != 2 {
		t.Fatalf("a curve crossing a value the log axis has no place for drew %d runs, want 2", len(got))
	}
	for _, run := range got {
		for _, p := range run {
			if math.IsNaN(float64(p.X)) || math.IsNaN(float64(p.Y)) {
				t.Errorf("a NaN coordinate reached the backend at %v", p)
			}
		}
	}
}

func TestALocusIsInTheLegendOnlyWhenItIsNamed(t *testing.T) {
	x, y := nicholsAxes()
	f := locusFrame(x, y, nil, 200)
	if _, ok := geom.Locus(stat.NicholsM, []float64{0}).Legend(f); ok {
		t.Error("an unlabelled locus asked for a legend row")
	}
	e, ok := geom.Locus(stat.NicholsM, []float64{0}, geom.Label("M contours")).Legend(f)
	if !ok || e.Label != "M contours" || e.Kind != geom.SwatchLine {
		t.Errorf("a labelled locus contributed %+v, ok = %v", e, ok)
	}
}

// A family nobody named still draws. It is only writing it down that it cannot
// survive, which is the spec's business rather than this package's.
func TestALocusOfACallersOwnFamilyDraws(t *testing.T) {
	x, y := scale.Linear(scale.Domain(0, 10)), scale.Linear(scale.Domain(0, 10))
	if got := len(runs(draw(t, locusFrame(x, y, nil, 100), geom.Locus(broken{}, []float64{1})))); got == 0 {
		t.Error("a family written outside figure drew nothing")
	}
}

func TestALocusWithNoFamilyAndNoLevelsDrawsNothing(t *testing.T) {
	x, y := scale.Linear(scale.Domain(0, 10)), scale.Linear(scale.Domain(0, 10))
	f := locusFrame(x, y, nil, 100)
	if got := len(runs(draw(t, f, geom.Locus(nil, []float64{1})))); got != 0 {
		t.Errorf("a locus with no family drew %d runs", got)
	}
	if got := len(runs(draw(t, f, geom.Locus(stat.NicholsM, nil)))); got != 0 {
		t.Errorf("a locus with no levels drew %d runs", got)
	}
}

// broken is a family of one straight segment, a gap, and another.
type broken struct{}

func (broken) Locus(xs, ys []float64, level float64, ext stat.Extent) ([]float64, []float64) {
	nan := math.NaN()
	return append(xs, 1, 2, nan, 3, 4), append(ys, level, level, nan, level, level)
}

// straddling walks across zero, which a log axis has no position for.
type straddling struct{}

func (straddling) Locus(xs, ys []float64, level float64, ext stat.Extent) ([]float64, []float64) {
	return append(xs, 2, 4, 0, 8, 16), append(ys, level, level, level, level, level)
}

package geom_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
)

// curvePath builds a line with the given options and returns the one stroked
// path it drew.
func curvePath(t *testing.T, xs, ys []float64, opts ...geom.Option) (*ir.Path, geom.Frame) {
	t.Helper()
	opts = append([]geom.Option{geom.X("x"), geom.Y("y")}, opts...)
	g := geom.Line(src(map[string][]float64{"x": xs, "y": ys}), opts...)
	rec, f := frame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	paths := rec.Filter("StrokePath")
	if len(paths) != 1 {
		t.Fatalf("got %d stroked paths, want 1: %v", len(paths), rec.Ops())
	}
	return paths[0].Path, f
}

// onCurve collects the points the path actually visits — the start and the end
// of every segment, which for a cubic is where it touches the data and not
// where its control points are.
func onCurve(p *ir.Path) []ir.Point {
	var out []ir.Point
	p.Walk(func(op ir.PathOp, pts []ir.Point) {
		switch op {
		case ir.OpMoveTo, ir.OpLineTo:
			out = append(out, pts[0])
		case ir.OpCubicTo:
			out = append(out, pts[2])
		}
	})
	return out
}

// allPoints collects every point in the path, control points included. It is
// what an overshoot has to be looked for in: a cubic leaves the box its ends
// bound only if a control point does, and it is bounded by the box all four of
// them bound.
func allPoints(p *ir.Path) []ir.Point {
	var out []ir.Point
	p.Walk(func(op ir.PathOp, pts []ir.Point) { out = append(out, pts...) })
	return out
}

func pathOps(p *ir.Path) []ir.PathOp {
	var out []ir.PathOp
	p.Walk(func(op ir.PathOp, _ []ir.Point) { out = append(out, op) })
	return out
}

// A monotone fit is the family that exists because the cardinal one overshoots,
// so the test is the overshoot: over a rising series no part of the curve —
// control points included — may dip below the reading it just left.
func TestMonotoneCurveNeverOvershoots(t *testing.T) {
	// A step-like rise: flat, jump, flat. This is where a Catmull-Rom spline
	// swings furthest past the data.
	xs := []float64{0, 1, 2, 3, 4, 5}
	ys := []float64{0, 0, 0, 10, 10, 10}

	mono, f := curvePath(t, xs, ys, geom.Curve(geom.CurveMonotone))
	lo, hi := f.Y.Map(0), f.Y.Map(10)
	if lo < hi {
		lo, hi = hi, lo
	}
	for i, q := range allPoints(mono) {
		if q.Y > lo+1e-3 || q.Y < hi-1e-3 {
			t.Errorf("point %d at y=%v leaves the band [%v, %v] the data bounds", i, q.Y, hi, lo)
		}
	}

	// And the guard on the guard: the cardinal family on the same data does
	// leave it, so the assertion above is testing something.
	card, _ := curvePath(t, xs, ys, geom.Curve(geom.CurveCardinal))
	over := false
	for _, q := range allPoints(card) {
		if q.Y > lo+1e-3 || q.Y < hi-1e-3 {
			over = true
		}
	}
	if !over {
		t.Error("the cardinal family did not overshoot this data, so the monotone assertion proves nothing")
	}
}

// Every interpolating family has to touch every row. This is the property the
// approximating ones give up, and the reason they are documented apart.
func TestInterpolatingCurvesPassThroughEveryRow(t *testing.T) {
	xs := []float64{0, 1, 2, 3, 4}
	ys := []float64{0, 2, 1, 3, 2}
	for _, k := range []geom.CurveKind{geom.CurveCardinal, geom.CurveMonotone, geom.CurveNatural} {
		if !k.Interpolating() {
			t.Fatalf("curve %d reports itself as approximating", k)
		}
		p, f := curvePath(t, xs, ys, geom.Curve(k))
		pts := onCurve(p)
		if len(pts) != len(xs) {
			t.Fatalf("curve %d visits %d points, want %d", k, len(pts), len(xs))
		}
		for i := range xs {
			wx, wy := f.X.Map(xs[i]), f.Y.Map(ys[i])
			if math.Abs(float64(pts[i].X-wx)) > 1e-3 || math.Abs(float64(pts[i].Y-wy)) > 1e-3 {
				t.Errorf("curve %d point %d is at (%v, %v), want (%v, %v)", k, i, pts[i].X, pts[i].Y, wx, wy)
			}
		}
	}
}

// A basis spline is a smoothing of the sequence rather than a reading of it.
// It must stay inside the hull of the control points — it may not invent a
// value outside the data — and it must *not* touch the interior ones, because
// a family that did would be an interpolating one under another name.
func TestBasisCurveApproximatesRatherThanInterpolates(t *testing.T) {
	xs := []float64{0, 1, 2, 3, 4}
	ys := []float64{0, 4, 0, 4, 0}
	if geom.CurveBasis.Interpolating() {
		t.Fatal("CurveBasis reports itself as interpolating")
	}
	p, f := curvePath(t, xs, ys, geom.Curve(geom.CurveBasis))

	lo, hi := f.Y.Map(4), f.Y.Map(0)
	if lo > hi {
		lo, hi = hi, lo
	}
	for i, q := range allPoints(p) {
		if q.Y < lo-1e-3 || q.Y > hi+1e-3 {
			t.Errorf("point %d at y=%v leaves the hull [%v, %v] of the control points", i, q.Y, lo, hi)
		}
	}
	// The peak at x=1 is a control point, and the curve is pulled towards it
	// rather than through it.
	peak := f.Y.Map(4)
	for _, q := range onCurve(p) {
		if math.Abs(float64(q.Y-peak)) < 1e-3 {
			t.Error("the basis curve passes through an interior vertex; it is meant to approximate them")
		}
	}
}

// The plain basis family still starts and ends on the data, because it repeats
// the end points; the open one does not, which is the whole difference.
func TestBasisVariantsDifferOnlyAtTheEnds(t *testing.T) {
	xs := []float64{0, 1, 2, 3, 4}
	ys := []float64{0, 4, 0, 4, 0}

	p, f := curvePath(t, xs, ys, geom.Curve(geom.CurveBasis))
	pts := onCurve(p)
	first, last := pts[0], pts[len(pts)-1]
	if math.Abs(float64(first.X-f.X.Map(0))) > 1e-3 || math.Abs(float64(last.X-f.X.Map(4))) > 1e-3 {
		t.Errorf("a plain basis curve runs from x=%v to x=%v, want the first and last row", first.X, last.X)
	}

	q, _ := curvePath(t, xs, ys, geom.Curve(geom.CurveBasisOpen))
	open := onCurve(q)
	if math.Abs(float64(open[0].X-first.X)) < 1e-3 {
		t.Error("an open basis curve started on the first row; it is meant to start short of it")
	}
}

// A closed family wraps its own neighbours and closes the path itself. The
// point of the test is that it does not also get the straight join back to the
// start that Closed adds, which would be a second, different closing.
func TestClosedCurvesCloseThemselves(t *testing.T) {
	xs := []float64{0, 1, 2, 3}
	ys := []float64{0, 1, 0, 1}
	for _, k := range []geom.CurveKind{geom.CurveCardinalClosed, geom.CurveBasisClosed} {
		p, _ := curvePath(t, xs, ys, geom.Curve(k))
		var closes, lines int
		for _, op := range pathOps(p) {
			switch op {
			case ir.OpClose:
				closes++
			case ir.OpLineTo:
				lines++
			}
		}
		if closes != 1 {
			t.Errorf("curve %d emitted %d closes, want exactly 1", k, closes)
		}
		if lines != 0 {
			t.Errorf("curve %d emitted %d straight segments, want none — it closes with its own curve", k, lines)
		}
	}
}

// Bundle blends towards the chord, so at beta 0 it is the chord: a smoothing
// that has smoothed everything away. It is the end of the family's range and
// the cheapest thing about it to assert exactly.
func TestBundleAtZeroBetaIsTheChord(t *testing.T) {
	xs := []float64{0, 1, 2, 3, 4}
	ys := []float64{0, 4, 0, 4, 0}
	p, f := curvePath(t, xs, ys, geom.Curve(geom.CurveBundle), geom.Tension(0))

	// Tension(0) is not "no tension" for a bundle, it is beta zero — but the
	// option cannot tell zero from unset, so the default applies and the curve
	// is not yet the chord. Beta has to be small rather than absent.
	q, _ := curvePath(t, xs, ys, geom.Curve(geom.CurveBundle), geom.Tension(1e-6))
	for _, pt := range allPoints(q) {
		// The chord of a series whose first and last rows are both y=0 is flat.
		if math.Abs(float64(pt.Y-f.Y.Map(0))) > 1e-2 {
			t.Errorf("a bundle at beta≈0 has a point at y=%v, want the flat chord at %v", pt.Y, f.Y.Map(0))
		}
	}
	if len(allPoints(p)) == 0 {
		t.Error("a bundle at the default beta drew nothing")
	}
}

// A monotone fit needs an axis to be a function of. A path that doubles back is
// a function of neither, and the family says so by drawing the polyline rather
// than quietly fitting a different curve.
func TestMonotoneFallsBackOnASelfIntersectingPath(t *testing.T) {
	// Both columns go out and come back, so neither device axis is monotone.
	// One of them being monotone would be enough — a line plotted sideways is
	// a perfectly good function of y — which is why both have to double back
	// for this to be the case the fallback is for.
	p, _ := curvePath(t, []float64{0, 2, 1, 3}, []float64{0, 2, 1, 3}, geom.Curve(geom.CurveMonotone))
	for _, op := range pathOps(p) {
		if op == ir.OpCubicTo {
			t.Fatal("a monotone fit on a self-intersecting path emitted a curve; it has none to emit")
		}
	}
}

// A line plotted sideways is a function of y rather than of x, and a monotone
// fit has to notice: taking x as the independent axis there divides by a step
// of zero.
func TestMonotoneTakesTheAxisTheDataIsAFunctionOf(t *testing.T) {
	p, _ := curvePath(t, []float64{0, 0, 0, 0}, []float64{0, 1, 2, 3}, geom.Curve(geom.CurveMonotone))
	for _, q := range allPoints(p) {
		if math.IsNaN(float64(q.X)) || math.IsNaN(float64(q.Y)) {
			t.Fatalf("a vertical line produced a non-finite point %v", q)
		}
	}
}

// Naming a tension and no family is what the option meant before there were
// families, and it still has to mean it.
func TestTensionWithoutACurveIsStillCardinal(t *testing.T) {
	xs := []float64{0, 1, 2, 3}
	ys := []float64{0, 2, 1, 3}
	old, _ := curvePath(t, xs, ys, geom.Tension(0.5))
	new, _ := curvePath(t, xs, ys, geom.Curve(geom.CurveCardinal), geom.Tension(0.5))
	a, b := allPoints(old), allPoints(new)
	if len(a) != len(b) {
		t.Fatalf("the two spellings drew %d and %d points", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("point %d differs: %v vs %v", i, a[i], b[i])
		}
	}
}

// Saying CurveLinear out loud must reach the polyline fast path, even beside a
// tension — otherwise the family could not be turned off again once a shared
// option set had turned it on.
func TestExplicitLinearCurveBeatsATension(t *testing.T) {
	g := geom.Line(src(map[string][]float64{"x": {0, 1, 2}, "y": {0, 1, 0}}),
		geom.X("x"), geom.Y("y"), geom.Tension(0.8), geom.Curve(geom.CurveLinear))
	rec, f := frame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if rec.Count("Polyline") != 1 || rec.Count("StrokePath") != 0 {
		t.Fatalf("an explicit linear curve should take the Polyline fast path, got %v", rec.Ops())
	}
}

// An area shares the option, and its two edges have to be fitted with the same
// family or the band would be smooth along the top and straight underneath.
func TestAreaCurvesBothEdges(t *testing.T) {
	g := geom.Area(src(map[string][]float64{
		"x": {0, 1, 2, 3},
		"y": {1, 3, 2, 4},
		"z": {0, 1, 0, 2},
	}), geom.X("x"), geom.Y("y"), geom.Y2("z"), geom.Curve(geom.CurveNatural))
	rec, f := frame(t, g)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, call := range rec.Filter("StrokePath") {
		if !hasCubic(pathOps(call.Path)) {
			t.Error("an edge of a curved area was drawn straight")
		}
	}
	if rec.Count("StrokePath") != 2 {
		t.Fatalf("got %d stroked edges, want 2", rec.Count("StrokePath"))
	}
}

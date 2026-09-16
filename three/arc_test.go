package three

import (
	"math"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

// sphereFrame is a spherical scene's frame, with the scales the sphere pins.
func sphereFrame(t *testing.T, opts ...SphereOption) Frame {
	t.Helper()
	sc := NewScene(Spherical(opts...)).
		X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear(scale.Domain(0, 1)))
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatal(err)
	}
	return Frame{X: scales[axisX], Y: scales[axisY], Z: scales[axisZ], space: sc.sphere}
}

// In a box a segment is the straight line it always was, so nothing that drew
// a trajectory in a cube draws anything different.
func TestASegmentInABoxIsOneStraightStep(t *testing.T) {
	var f Frame
	f.X, f.Y, f.Z = unitScales()[0], unitScales()[1], unitScales()[2]
	a := f.Arc(1, 2, 3, 9, 8, 7)
	if a.Steps() != 1 {
		t.Errorf("a straight segment is drawn in %d steps, want one", a.Steps())
	}
	nearVec(t, a.At(0), f.Point(1, 2, 3), "the near end")
	nearVec(t, a.At(1), f.Point(9, 8, 7), "the far end")
	nearVec(t, a.At(0.5), f.Point(5, 5, 5), "the middle")
}

// Two directions a quarter turn apart are joined by the arc between them, not
// by the chord through the inside of the ball. That is the whole of this
// feature: a coarse sweep on a Bloch sphere used to be drawn through the
// middle of it.
func TestASegmentOnASphereFollowsTheGreatCircle(t *testing.T) {
	f := sphereFrame(t)
	// Two points on the equator, ninety degrees apart, both on the surface.
	a := f.Arc(0, 90, 1, 90, 90, 1)
	if a.Steps() < 10 {
		t.Fatalf("a quarter turn is drawn in %d steps, want it subdivided", a.Steps())
	}
	for k := 0; k <= a.Steps(); k++ {
		p := a.At(float32(k) / float32(a.Steps()))
		r := p.Sub(centre)
		if d := math.Sqrt(r.Dot(r)); math.Abs(d-radius) > 1e-5 {
			t.Errorf("step %d is at radius %v, want it on the ball at %v", k, d, radius)
		}
	}
	// The midpoint is the direction halfway between the two, which the chord
	// would have put a factor of √2 too far in.
	nearVec(t, a.At(0.5), f.Point(45, 90, 1), "the middle of the arc")
}

// The radius runs linearly along the arc, so a sweep that grows as it turns
// grows evenly rather than jumping at its ends.
func TestASegmentOnASphereRunsItsRadiusAlongTheArc(t *testing.T) {
	f := sphereFrame(t)
	a := f.Arc(0, 90, 0.4, 90, 90, 1)
	mid := a.At(0.5).Sub(centre)
	want := radius * (0.4 + 1) / 2
	if got := math.Sqrt(mid.Dot(mid)); math.Abs(got-want) > 1e-5 {
		t.Errorf("the middle of the arc is at radius %v, want %v", got, want)
	}
}

// Two segments have no arc to be drawn as, and are chords: one whose ends are
// antipodal, because every meridian between them is a great circle, and one
// with an end at the centre, which has no direction there.
func TestASegmentWithNoUniqueArcIsDrawnAsAChord(t *testing.T) {
	f := sphereFrame(t)
	for name, a := range map[string]Arc{
		"antipodal":  f.Arc(0, 0, 1, 0, 180, 1),
		"the centre": f.Arc(0, 90, 0, 90, 90, 1),
	} {
		if a.Steps() != 1 {
			t.Errorf("%s: drawn in %d steps, want one chord", name, a.Steps())
		}
	}
}

// On a Smith sphere the path between two impedances is the image of the
// straight line between their reflection coefficients, which a stereographic
// projection takes to a circle on the ball — so it is on the surface, not a
// chord through the solid.
func TestASegmentOnASmithSphereFollowsACircleOnTheBall(t *testing.T) {
	sc := NewScene(Spherical(Smith())).
		X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear(scale.Domain(0, 1)))
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatal(err)
	}
	f := Frame{X: scales[axisX], Y: scales[axisY], Z: scales[axisZ], space: sc.sphere}

	// A series inductance swept from the match: r = 1, x from 0 to 2.
	a := f.Arc(1, 0, 1, 1, 2, 1)
	if a.Steps() < 2 {
		t.Fatalf("the sweep is drawn in %d steps, want it subdivided", a.Steps())
	}
	for k := 0; k <= a.Steps(); k++ {
		p := a.At(float32(k) / float32(a.Steps()))
		r := p.Sub(centre)
		if d := math.Sqrt(r.Dot(r)); math.Abs(d-radius) > 1e-5 {
			t.Errorf("step %d is at radius %v, want it on the ball at %v", k, d, radius)
		}
	}
	nearVec(t, a.At(0), f.Point(1, 0, 1), "the near end")
	nearVec(t, a.At(1), f.Point(1, 2, 1), "the far end")
}

// A line on a ball reaches the sink as one primitive per step of its path, so
// it interleaves with everything it crosses at every scale a reader can see.
func TestALineOnASphereEmitsOnePrimitivePerStep(t *testing.T) {
	src := figure.NewTable().
		Float64("az", []float64{0, 90}).
		Float64("pol", []float64{90, 90}).
		Float64("r", []float64{1, 1})
	sc := NewScene(Spherical()).
		X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear(scale.Domain(0, 1))).
		Add(Line3(src, geom.X("az"), geom.Y("pol"), geom.Z("r")))
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatal(err)
	}
	f := Frame{X: scales[axisX], Y: scales[axisY], Z: scales[axisZ],
		space: sc.sphere, Rows: noRows{}}
	s := new(Sink)
	s.openLayer(0, Home().Forward())
	if err := sc.layers[0].Emit(s, f); err != nil {
		t.Fatal(err)
	}
	if len(s.prims) < 10 {
		t.Fatalf("%d primitives over a quarter turn, want one per step", len(s.prims))
	}
	for i, p := range s.prims {
		if p.kind != kindLine {
			t.Errorf("primitive %d is kind %d, want a line", i, p.kind)
		}
		// Every step belongs to the row the segment ends at, whichever piece
		// of it the reader points at.
		if p.row != 1 {
			t.Errorf("primitive %d reports row %d, want 1", i, p.row)
		}
	}
	for i, v := range s.verts {
		r := v.Sub(centre)
		if d := math.Sqrt(r.Dot(r)); math.Abs(d-radius) > 1e-5 {
			t.Errorf("vertex %d is at radius %v, want it on the ball", i, d)
		}
	}
}

// A line in a box is one primitive per pair of rows, exactly as it was before
// there was an arc to walk.
func TestALineInABoxIsStillOnePrimitivePerSegment(t *testing.T) {
	src := figure.NewTable().
		Float64("x", []float64{0, 0.5, 1}).
		Float64("y", []float64{0, 0.5, 1}).
		Float64("z", []float64{0, 0.5, 1})
	sc := NewScene().
		X(scale.Linear(scale.Domain(0, 1))).Y(scale.Linear(scale.Domain(0, 1))).
		Z(scale.Linear(scale.Domain(0, 1))).
		Add(Line3(src, geom.X("x"), geom.Y("y"), geom.Z("z")))
	s, _ := emitOne(t, sc)
	if len(s.prims) != 2 {
		t.Errorf("%d primitives over three rows, want one per segment", len(s.prims))
	}
}

package three

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/scale"
)

func TestTheSmithSpherePutsTheLandmarksWhereTheRiemannSphereDoes(t *testing.T) {
	top, bottom := Vec3{0.5, 0.5, 1}, Vec3{0.5, 0.5, 0}
	nearVec(t, smithPoint(1, 0, 1), top, "the match, z = 1")
	nearVec(t, smithPoint(1e12, 0, 1), Vec3{1, 0.5, 0.5}, "the open, z → ∞")
	nearVec(t, smithPoint(0, 0, 1), Vec3{0, 0.5, 0.5}, "the short, z = 0")
	nearVec(t, smithPoint(-1, 0, 1), bottom, "z = −1, where Γ is infinite")

	// A pure reactance has |Γ| = 1, and every one lies on the equator.
	for _, x := range []float64{-5, -1, -0.2, 0.3, 1, 7} {
		if p := smithPoint(0, x, 1); math.Abs(float64(p.Z-0.5)) > 1e-6 {
			t.Errorf("z = j%v is at height %v, want the equator", x, p.Z)
		}
	}
	// A passive impedance is in the north and an active one in the south.
	if smithPoint(0.3, 0.7, 1).Z <= 0.5 || smithPoint(-0.3, 0.7, 1).Z >= 0.5 {
		t.Error("a passive impedance must be north of the equator and an active one south of it")
	}
	// Radius zero is the centre, whatever the impedance.
	nearVec(t, smithPoint(2, -3, 0), centre, "radius zero")
}

// A stereographic projection takes circles to circles, which is the property
// that lets the flat chart's grid be carried onto the sphere at all. The image
// of a constant-resistance circle must therefore lie in one plane.
func TestAConstantResistanceCircleStaysACircleOnTheSphere(t *testing.T) {
	for _, r := range []float64{0.5, 2, -0.5, -3} {
		c, a := r/(r+1), 1/math.Abs(r+1)
		var pts []Vec3
		for k := 0; k < 12; k++ {
			pts = append(pts, riemann(complex(c, 0)+cmplx.Rect(a, 2*math.Pi*float64(k)/12+0.1), 1))
		}
		n := pts[3].Sub(pts[0]).Cross(pts[7].Sub(pts[0])).Unit()
		for _, p := range pts {
			if d := math.Abs(float64(p.Sub(pts[0]).Dot(n))); d > 1e-4 {
				t.Errorf("r = %v: a point of its image is %v off the plane of the others", r, d)
				break
			}
		}
	}
}

func TestASmithSphereReadsImpedancesAndDrawsItsGrid(t *testing.T) {
	src := figure.NewTable().
		Float64("r", []float64{1, 0.5, -0.4}).Float64("x", []float64{0, 1, -0.8}).Float64("one", []float64{1, 1, 1})
	sc := NewScene(Spherical(Smith())).
		Add(Scatter3(src, geom.X("r"), geom.Y("x"), geom.Z("one"), geom.Droplines(false)))
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatal(err)
	}
	if lo, _ := scales[axisX].Domain(); lo > -0.4 {
		t.Errorf("the resistance scale was pinned to [%v, …]; a Smith sphere reads it raw", lo)
	}
	f := Frame{X: scales[0], Y: scales[1], Z: scales[2], space: sc.sphere}
	nearVec(t, f.Point(1, 0, 1), Vec3{0.5, 0.5, 1}, "the match through the frame")

	rec := irtest.New()
	if err := New(Size(400, 400)).Scene(sc).Render(rec.Target()); err != nil {
		t.Fatal(err)
	}
	labels := map[string]bool{}
	for _, c := range rec.Filter("Text") {
		labels[c.Text.Text] = true
	}
	for _, want := range []string{"match", "open", "short"} {
		if !labels[want] {
			t.Errorf("the Smith sphere did not write %q: %v", want, labels)
		}
	}
	// Some of the grid's values face the reader from any camera.
	values := 0
	for _, v := range []string{"0.2", "0.5", "1", "2", "5", "-0.2", "-0.5", "-2", "-5"} {
		if labels[v] {
			values++
		}
	}
	if values == 0 {
		t.Errorf("no grid values were written: %v", labels)
	}
	if n := len(rec.Filter("StrokePath")); n < 4 {
		t.Errorf("%d strokes, want the far and near halves of the grid and of the axes", n)
	}
}

func TestASmithSurfaceDoesNotCloseRoundAnAzimuth(t *testing.T) {
	var r, x, v []float64
	for _, xi := range []float64{-1, 0, 1} {
		for _, ri := range []float64{0, 1, 2} {
			r, x, v = append(r, ri), append(x, xi), append(v, 1)
		}
	}
	src := figure.NewTable().Float64("r", r).Float64("x", x).Float64("v", v)
	sc := NewScene(Spherical(Smith())).Z(scale.Linear(scale.Domain(0, 1))).
		Add(Surface(src, geom.X("r"), geom.Y("x"), geom.Z("v")))
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatal(err)
	}
	s := acquire()
	defer release(s)
	cam := Home()
	s.openLayer(0, cam.Forward())
	if err := sc.layers[0].Emit(s, Frame{X: scales[0], Y: scales[1], Z: scales[2], Forward: cam.Forward(), space: sc.sphere}); err != nil {
		t.Fatal(err)
	}
	if len(s.prims) != 4 {
		t.Errorf("%d faces over a 3×3 grid of impedances, want the four cells and no seam", len(s.prims))
	}
}

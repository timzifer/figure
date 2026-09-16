package three

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// alloys is three compositions and a temperature: the pure first component,
// the pure second, and the even mixture.
func alloys() figure.Source {
	return figure.NewTable().
		Float64("a", []float64{1, 0, 1.0 / 3}).
		Float64("b", []float64{0, 1, 1.0 / 3}).
		Float64("t", []float64{0, 1, 0.5})
}

func prismScene(opts ...PrismOption) *Scene {
	return NewScene(ZTitle("°C"), Prism(opts...)).
		X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear(scale.Domain(0, 1))).
		Add(Scatter3(alloys(), geom.X("a"), geom.Y("b"), geom.Z("t"),
			geom.Droplines(false)))
}

func prismFrame(t *testing.T, sc *Scene) Frame {
	t.Helper()
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatal(err)
	}
	return Frame{X: scales[axisX], Y: scales[axisY], Z: scales[axisZ], floor: sc.prism}
}

// The three pure compositions land on the three corners of the floor triangle,
// and the even mixture lands over its centroid. Everything else the prism does
// follows from that.
func TestAPrismPlacesTheComponentsAtTheCornersOfItsFloor(t *testing.T) {
	f := prismFrame(t, prismScene())
	if !f.Prismatic() {
		t.Fatal("a prism scene does not report itself prismatic")
	}
	nearVec(t, f.Point(1, 0, 0), prismFloor[0], "all of the first component")
	nearVec(t, f.Point(0, 1, 0), prismFloor[1], "all of the second")
	nearVec(t, f.Point(0, 0, 0), prismFloor[2], "all of the derived third")

	mid := prismFloor[0].Add(prismFloor[1]).Add(prismFloor[2]).Mul(1.0 / 3)
	nearVec(t, f.Point(1.0/3, 1.0/3, 0), mid, "the even mixture")
}

// The height is an ordinary depth axis: the fourth variable runs up the prism
// and the composition does not move with it.
func TestAPrismsHeightIsTheFourthVariable(t *testing.T) {
	f := prismFrame(t, prismScene())
	lo, hi := f.Point(0.5, 0.25, 0), f.Point(0.5, 0.25, 1)
	if lo.X != hi.X || lo.Y != hi.Y {
		t.Errorf("the composition moved with the height: %v then %v", lo, hi)
	}
	if lo.Z != 0 || hi.Z != 1 {
		t.Errorf("heights %v and %v, want the whole depth axis", lo.Z, hi.Z)
	}
}

// Both component domains are pinned to the whole simplex whatever the data
// covers, which is the sphere's rule for its angles: the extent is a fact
// about the coordinate system rather than about the rows.
func TestAPrismPinsItsComponentsToTheWholeSimplex(t *testing.T) {
	for name, tc := range map[string]struct {
		opts []PrismOption
		want [2]float64
	}{
		"fractions":  {nil, [2]float64{0, 1}},
		"percentage": {[]PrismOption{PrismSum(100)}, [2]float64{0, 100}},
	} {
		t.Run(name, func(t *testing.T) {
			sc := prismScene(tc.opts...)
			scales := sc.scales()
			if err := sc.train(scales); err != nil {
				t.Fatal(err)
			}
			for a, axis := range [2]int{axisX, axisY} {
				lo, hi := scales[axis].Domain()
				if lo != tc.want[0] || hi != tc.want[1] {
					t.Errorf("component %d domain [%v, %v], want %v", a, lo, hi, tc.want)
				}
			}
		})
	}

	ordinal := NewScene(Prism()).X(scale.Ordinal()).
		Add(Scatter3(alloys(), geom.X("a"), geom.Y("b"), geom.Z("t")))
	if err := ordinal.train(ordinal.scales()); err == nil {
		t.Error("an ordinal component was accepted; half a category is not a component")
	}
}

// A percentage prism reads the same triangle in hundredths.
func TestAPrismSumIsTheUnitTheComponentsAreIn(t *testing.T) {
	f := prismFrame(t, prismScene(PrismSum(100)))
	nearVec(t, f.Point(100, 0, 0), prismFloor[0], "100% of the first component")
	nearVec(t, f.Point(0, 0, 0), prismFloor[2], "100% of the derived third")
}

// A mark that stands on a rectangular floor has no meaning over a triangle
// whose two axes are one composition, and says so rather than drawing a
// skewed box.
func TestAMarkThatNeedsARectangularFloorRefusesAPrism(t *testing.T) {
	src := figure.NewTable().
		Float64("s", []float64{0.2, 0.6}).Float64("r", []float64{0.3, 0.1}).
		Float64("v", []float64{1, 2})
	for name, l := range map[string]Layer{
		"bar3": Bar3(src, geom.X("s"), geom.Y("r"), geom.Z("v")),
	} {
		sc := NewScene(Prism()).X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear()).Add(l)
		scales := sc.scales()
		if err := sc.train(scales); err != nil {
			t.Fatal(err)
		}
		f := Frame{X: scales[axisX], Y: scales[axisY], Z: scales[axisZ], floor: sc.prism}
		s := new(Sink)
		s.openLayer(0, Home().Forward())
		if err := l.Emit(s, f); !errors.Is(err, ErrNotPrismatic) {
			t.Errorf("%s in a prism: err = %v, want ErrNotPrismatic", name, err)
		}
	}
}

// The furniture is a triangular prism: three vertical edges rather than four,
// and the component names at the corners they belong to.
func TestAPrismDrawsATriangularSolidAndNamesItsCorners(t *testing.T) {
	rec := irtest.New()
	p := New(Size(400, 360)).Scene(prismScene(PrismCorners("Fe", "Cr", "Ni")))
	if err := p.Render(rec.Target()); err != nil {
		t.Fatal(err)
	}
	texts := map[string]bool{}
	for _, s := range rec.Texts() {
		texts[s] = true
	}
	for _, want := range []string{"Fe", "Cr", "Ni", "°C"} {
		if !texts[want] {
			t.Errorf("the prism does not write %q", want)
		}
	}
}

// Every side wall the camera is shown is one that points away from it, which
// is what makes the furniture exact rather than approximate: every datum is
// inside the prism, so it is in front of all of them.
func TestEveryWallAPrismFillsPointsAwayFromTheCamera(t *testing.T) {
	for _, cam := range octants() {
		f := newPrism(theme.Light, project(cam, ir.R(0, 0, 300, 300)), cam,
			&prism{sum: 1}, [3][]scale.Tick{}, [3]string{})
		shown := 0
		for k := range f.far {
			if !f.far[k] {
				continue
			}
			shown++
			if n := f.wallNormal(k); n.Dot(cam.Forward()) <= 0 {
				t.Errorf("camera %+v: wall %d is filled and faces it", cam, k)
			}
		}
		if shown == 0 || shown > 2 {
			t.Errorf("camera %+v: %d of three side walls shown, want one or two", cam, shown)
		}
	}
}

// The horizontal face the ternary grid is drawn on is the one on the far side
// of the camera, so a camera above the solid is given its floor and one below
// it is given its roof — a datum is in front of either.
func TestAPrismShowsTheHorizontalFaceOnTheFarSide(t *testing.T) {
	above := Home()
	below := LookAt(Azimuth(-0.6), Elevation(-math.Pi/6))
	for name, tc := range map[string]struct {
		cam  Camera
		want float32
	}{"above": {above, 0}, "below": {below, 1}} {
		f := newPrism(theme.Light, project(tc.cam, ir.R(0, 0, 300, 300)), tc.cam,
			&prism{sum: 1}, [3][]scale.Tick{}, [3]string{})
		if f.floor != tc.want {
			t.Errorf("from %s the far horizontal face is at z = %v, want %v", name, f.floor, tc.want)
		}
	}
}

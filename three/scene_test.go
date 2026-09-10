package three

import (
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// ridge is a small regular grid: a ripple whose height is not a function of
// either floor axis alone, so a picture of it is wrong in a visible way if the
// axes get swapped.
func ridge(nx, ny int) data.Source {
	xs := make([]float64, 0, nx*ny)
	ys := make([]float64, 0, nx*ny)
	zs := make([]float64, 0, nx*ny)
	for j := 0; j < ny; j++ {
		for i := 0; i < nx; i++ {
			x := float64(i) / float64(nx-1)
			y := float64(j) / float64(ny-1)
			xs = append(xs, x)
			ys = append(ys, y)
			zs = append(zs, (1-x)*(1-x)+0.5*y)
		}
	}
	return data.Float64Columns(map[string][]float64{"x": xs, "y": ys, "z": zs})
}

func surfaceScene(nx, ny int) *Scene {
	return NewScene(XTitle("x"), YTitle("y"), ZTitle("z")).
		Z(scale.Linear(scale.Nice())).
		Add(Surface(ridge(nx, ny), geom.X("x"), geom.Y("y"), geom.Z("z")))
}

// counting wraps a scale and counts what it was told, so that a test can pin
// how often a scene is trained.
type counting struct {
	scale.Scale
	trains int
}

func (c *counting) Train(vs ...float64) { c.trains++; c.Scale.Train(vs...) }

// One data repository, several ways of looking at it: a scene is trained once
// however many cameras are pointed at it. Training per view would give a layer
// as many times its weight as there are views, because a scale accumulates —
// and the domain would then depend on how many angles the author asked for.
func TestSeveralViewsTrainTheSceneOnce(t *testing.T) {
	z := &counting{Scale: scale.Linear()}
	sc := NewScene().Z(z).
		Add(Surface(ridge(4, 4), geom.X("x"), geom.Y("y"), geom.Z("z")))

	p := New(Size(600, 200), Columns(3)).Scene(sc).Add(
		View{Camera: Home(), Label: "three-quarter"},
		View{Camera: LookAt(Azimuth(0)), Label: "front"},
		View{Camera: LookAt(Elevation(1.2)), Label: "top"},
	)
	rec := irtest.New()
	if err := p.Render(rec.Target()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if z.trains != 1 {
		t.Errorf("the depth scale was trained %d times for three views, want once", z.trains)
	}
}

// Three cameras on one scene draw three cubes and three copies of the data,
// and the scene itself is unchanged by any of it.
func TestSeveralViewsDrawOneSceneSeveralTimes(t *testing.T) {
	sc := surfaceScene(4, 4)
	before := len(sc.Layers())

	one := irtest.New()
	if err := New(Size(300, 300)).Scene(sc).Render(one.Target()); err != nil {
		t.Fatalf("one view: %v", err)
	}
	three := irtest.New()
	err := New(Size(900, 300), Columns(3)).Scene(sc).Add(
		View{Camera: Home()},
		View{Camera: LookAt(Azimuth(0.9), Elevation(0.2))},
		View{Camera: LookAt(Azimuth(2.2), Elevation(0.6))},
	).Render(three.Target())
	if err != nil {
		t.Fatalf("three views: %v", err)
	}

	if got, want := three.Count("FillPath"), one.Count("FillPath"); got < want*3 {
		t.Errorf("three views made %d fill calls and one made %d; the scene was not drawn three times",
			got, want)
	}
	if got := len(sc.Layers()); got != before {
		t.Errorf("rendering changed the scene: %d layers, was %d", got, before)
	}
}

// A view may bring its own scene, which is how one figure compares two
// datasets from the same angle.
func TestAViewMayOverrideTheScene(t *testing.T) {
	rec := irtest.New()
	err := New(Size(600, 300), Columns(2)).Scene(surfaceScene(3, 3)).Add(
		View{Camera: Home(), Label: "small"},
		View{Camera: Home(), Label: "large", Scene: surfaceScene(6, 6)},
	).Render(rec.Target())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := strings.Join(rec.Texts(), " "); !strings.Contains(got, "small") || !strings.Contains(got, "large") {
		t.Errorf("the view labels are missing from %q", got)
	}
}

// A plot with nothing to draw says so rather than drawing an empty box.
func TestAPlotWithoutASceneIsAnError(t *testing.T) {
	rec := irtest.New()
	if err := New().Render(rec.Target()); err != ErrNoScene {
		t.Errorf("got %v, want ErrNoScene", err)
	}
}

// An error from a layer reaches the caller rather than being drawn around.
func TestALayerErrorSurfaces(t *testing.T) {
	src := data.Float64Columns(map[string][]float64{
		"x": {0, 1, 2}, "y": {0, 0, 1}, "z": {1, 2, 3},
	})
	sc := NewScene().Add(Surface(src, geom.X("x"), geom.Y("y"), geom.Z("z")))
	rec := irtest.New()
	err := New().Scene(sc).Render(rec.Target())
	if err == nil {
		t.Fatal("a table that is not a full grid drew a surface anyway")
	}
	if !strings.Contains(err.Error(), "one row per cell") {
		t.Errorf("the error is %q; it should say what a surface needs", err)
	}
}

// The three scales are ranged into the unit cube, not into pixels. Choosing
// what the interval means is what this stage does, and anything downstream
// that assumes a scale's range is in pixels is wrong here.
func TestTheScalesAreRangedIntoTheUnitCube(t *testing.T) {
	x, y, z := scale.Linear(), scale.Linear(), scale.Linear()
	sc := NewScene().X(x).Y(y).Z(z).
		Add(Surface(ridge(3, 3), geom.X("x"), geom.Y("y"), geom.Z("z")))
	rec := irtest.New()
	if err := New().Scene(sc).Render(rec.Target()); err != nil {
		t.Fatalf("render: %v", err)
	}
	for i, s := range []scale.Scale{x, y, z} {
		lo, hi := s.Map(mustMin(s)), s.Map(mustMax(s))
		if lo != 0 || hi != 1 {
			t.Errorf("axis %d maps its domain onto [%v, %v], want the unit cube's edge [0, 1]", i, lo, hi)
		}
	}
}

func mustMin(s scale.Scale) float64 { lo, _ := s.Domain(); return lo }
func mustMax(s scale.Scale) float64 { _, hi := s.Domain(); return hi }

// Nothing three-dimensional reaches the IR. That is the invariant ADR 0056
// actually defends: no ir.Point3, no depth on a drawing call, no Backend3 —
// which is what makes every backend draw a surface on the day this package
// compiles.
func TestNothingThreeDimensionalReachesTheIR(t *testing.T) {
	sc := surfaceScene(5, 5).Add(
		Line3(ridge(5, 5), geom.X("x"), geom.Y("y"), geom.Z("z")),
	)
	rec := irtest.New()
	if err := New(Size(400, 400), Theme(theme.Dark)).Scene(sc).Render(rec.Target()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(rec.Calls) == 0 {
		t.Fatal("nothing was drawn")
	}
	for i, c := range rec.Calls {
		switch c.Op {
		case "Polyline", "StrokePath", "FillPath", "Text", "Push", "Pop", "Markers", "Image":
		default:
			t.Errorf("call %d is a %q, which is not one of the calls the IR has always had", i, c.Op)
		}
		for _, p := range c.Points {
			if p != (ir.Point{X: p.X, Y: p.Y}) {
				t.Errorf("call %d carries a point with more than two coordinates", i)
			}
		}
	}
}

// The data is clipped to its own view, so a scene that overflows its cell
// cannot draw over the one beside it.
func TestEachViewClipsToItsOwnCell(t *testing.T) {
	rec := irtest.New()
	err := New(Size(600, 300), Columns(2)).Scene(surfaceScene(4, 4)).Add(
		View{Camera: Home()}, View{Camera: Home()},
	).Render(rec.Target())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if rec.MaxDepth < 1 {
		t.Error("nothing was clipped; a view's data can spill into the next cell")
	}
	if got := rec.Count("Push"); got != 2 {
		t.Errorf("got %d pushes for two views, want one clip each", got)
	}
}

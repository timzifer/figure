package three

import (
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

// rippleTable is a field with ridges and troughs, so that its isolines are
// closed rings rather than a single straight line.
func rippleTable(n int) data.Source {
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	zs := make([]float64, 0, n*n)
	for j := range n {
		for i := range n {
			x := -3 + 6*float64(i)/float64(n-1)
			y := -3 + 6*float64(j)/float64(n-1)
			xs, ys = append(xs, x), append(ys, y)
			zs = append(zs, math.Sin(x)*math.Cos(y))
		}
	}
	return data.Float64Columns(map[string][]float64{"x": xs, "y": ys, "z": zs})
}

// The lines lie flat on the floor: every point of every one of them is at the
// bottom of the cube, whatever the value it traces.
func TestAContourLiesOnTheFloor(t *testing.T) {
	sc := NewScene().Add(Contour(rippleTable(12),
		geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Levels(-0.5, 0, 0.5)))

	s, _ := emitOne(t, sc)
	if len(s.prims) == 0 {
		t.Fatal("the contour emitted nothing")
	}
	for _, p := range s.prims {
		if p.kind != kindLine {
			t.Fatalf("the contour emitted a %v, want lines only", p.kind)
		}
		for _, v := range s.verts[p.lo:p.hi] {
			if v.Z != 0 {
				t.Fatalf("a point of the floor contour is at z = %v", v.Z)
			}
		}
	}
}

// The ceiling is the other face it may lie on, for a scene a reader looks up
// into.
func TestAContourMayLieOnTheCeiling(t *testing.T) {
	sc := NewScene().Add(Contour(rippleTable(10),
		geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Levels(0), Plane(Ceiling)))

	s, _ := emitOne(t, sc)
	if len(s.prims) == 0 {
		t.Fatal("the contour emitted nothing")
	}
	for _, p := range s.prims {
		for _, v := range s.verts[p.lo:p.hi] {
			if v.Z != 1 {
				t.Fatalf("a point of the ceiling contour is at z = %v", v.Z)
			}
		}
	}
}

// One segment per primitive, which is the rule that lets a floor line interleave
// with the surface above it rather than being ordered wholly in front of it or
// wholly behind.
func TestAContourEmitsOneSegmentAtATime(t *testing.T) {
	sc := NewScene().Add(Contour(rippleTable(12),
		geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Levels(0)))

	s, _ := emitOne(t, sc)
	for _, p := range s.prims {
		if n := p.hi - p.lo; n != 2 {
			t.Fatalf("a primitive has %d points, want the two of one segment", n)
		}
	}
}

// The depth axis covers the values the lines trace, even though nothing is
// drawn at them: the lines are a reading of z, and an axis that did not reach
// them would label a cube the contour contradicts.
func TestAFloorContourTrainsTheDepthAxis(t *testing.T) {
	g := Contour(rippleTable(10), geom.X("x"), geom.Y("y"), geom.Z("z"))
	z := scale.Linear()
	if err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear(), Z: z}); err != nil {
		t.Fatal(err)
	}
	lo, hi := z.Domain()
	if lo > -0.9 || hi < 0.9 {
		t.Errorf("the depth axis reports [%v, %v], want it to cover the field", lo, hi)
	}
}

// The load-bearing one: the scene's floor and a flat chart of the same table
// are the same lines, because they are one tracing rather than two. A reading
// taken off the plan and one taken off the floor have to agree.
func TestAFloorContourIsTheSameLinesAsAFlatOne(t *testing.T) {
	src := rippleTable(14)
	levels := []float64{-0.6, -0.2, 0.2, 0.6}

	flat := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Levels(levels...))
	if err := flat.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); err != nil {
		t.Fatal(err)
	}
	deep := Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Levels(levels...))
	if err := deep.Train(geom.Training{X: scale.Linear(), Y: scale.Linear(), Z: scale.Linear()}); err != nil {
		t.Fatal(err)
	}

	a, ok := flat.(interface{ Levels() []float64 })
	if !ok {
		t.Fatal("the flat contour does not report its levels")
	}
	b, ok := deep.(interface{ Levels() []float64 })
	if !ok {
		t.Fatal("the floor contour does not report its levels")
	}
	if len(a.Levels()) != len(b.Levels()) {
		t.Fatalf("the two traced %d levels and %d", len(a.Levels()), len(b.Levels()))
	}
	for i := range a.Levels() {
		if a.Levels()[i] != b.Levels()[i] {
			t.Fatalf("level %d is %v flat and %v on the floor", i, a.Levels()[i], b.Levels()[i])
		}
	}

	// And the same geometry, which is the claim that matters: two tracings of
	// one field are the same vertices, so the two pictures cannot disagree at a
	// saddle. It is counted through what each actually draws — the flat one's
	// subpaths and the scene's segments — because that is what a reader
	// compares.
	rec := irtest.New()
	x, y := scale.Linear(), scale.Linear()
	if err := flat.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	area := ir.R(0, 0, 300, 200)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)
	if err := flat.Build(rec, geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}); err != nil {
		t.Fatal(err)
	}
	flatSegments := 0
	for _, c := range rec.Filter("StrokePath") {
		// One subpath per run: a MoveTo opens one, and every other op
		// continues it, so the segments are the points less the runs.
		runs, pts := 0, 0
		for _, op := range c.Path.Ops {
			if op == ir.OpMoveTo {
				runs++
			}
			if op == ir.OpMoveTo || op == ir.OpLineTo {
				pts++
			}
		}
		flatSegments += pts - runs
	}

	s, _ := emitOne(t, NewScene().Add(Contour(src,
		geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Levels(levels...))))
	if flatSegments == 0 {
		t.Fatal("the flat contour drew nothing, so the comparison says nothing")
	}
	if len(s.prims) != flatSegments {
		t.Errorf("the floor drew %d segments and the flat chart %d; they are not the same lines",
			len(s.prims), flatSegments)
	}
}

// Each level is drawn in the ramp's reading of it, so that one colour scale
// makes this chart and a surface of the same table comparable.
func TestEachFloorLevelTakesItsColourFromTheRamp(t *testing.T) {
	ramp := scale.Sequential(palette.Viridis, scale.ColorDomain(-1, 1))
	sc := NewScene().Add(Contour(rippleTable(12),
		geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.Levels(-0.5, 0, 0.5), geom.ColorBy("z", ramp)))

	s, _ := emitOne(t, sc)
	seen := map[ir.Color]bool{}
	for _, p := range s.prims {
		seen[p.style.Stroke] = true
	}
	if len(seen) != 3 {
		t.Errorf("three levels were drawn in %d colours", len(seen))
	}
	for _, level := range []float64{-0.5, 0, 0.5} {
		if !seen[ramp.Color(level)] {
			t.Errorf("no line is drawn in the ramp's colour for %v", level)
		}
	}
}

// A table that is not a grid is an error naming the mark and the rows, not a
// picture with holes in it.
func TestAFloorContourRefusesATableThatIsNotAGrid(t *testing.T) {
	src := data.Float64Columns(map[string][]float64{
		"x": {0, 1, 0, 0}, "y": {0, 0, 1, 1}, "z": {1, 2, 3, 4},
	})
	g := Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"))
	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear(), Z: scale.Linear()})
	if err == nil {
		t.Fatal("a table with two rows at one node was accepted")
	}
}

// emitOne trains a scene and emits its one layer into a sink, which is how a
// test reads what a layer put into the scene rather than what the painter made
// of it.
func emitOne(t *testing.T, sc *Scene) (*Sink, Frame) {
	t.Helper()
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatalf("training the scene: %v", err)
	}
	f := Frame{
		X: scales[axisX], Y: scales[axisY], Z: scales[axisZ],
		Theme: theme.Light, Forward: Home().Forward(),
	}
	s := new(Sink)
	s.openLayer(0, f.Forward)
	if err := sc.layers[0].Emit(s, f); err != nil {
		t.Fatalf("emitting the layer: %v", err)
	}
	return s, f
}

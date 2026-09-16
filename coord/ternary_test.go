package coord

import (
	"math"
	"testing"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// simplex is a ternary coord framed in a square panel, with both scales the
// linear ones a chart would give it.
func simplex(t *testing.T, opts ...TernaryOption) *ternary {
	t.Helper()
	c := Ternary(opts...).Frame(Framing{
		Area: ir.R(0, 0, 100, 100),
		X:    scale.Linear(),
		Y:    scale.Linear(),
	})
	tc, ok := c.(*ternary)
	if !ok {
		t.Fatalf("Frame returned %T, want a ternary", c)
	}
	return tc
}

// The three pure compositions land on the three corners, and the even mixture
// lands at the centroid. Everything else this coord does follows from that.
func TestATernaryCoordPlacesThePureComponentsAtTheCorners(t *testing.T) {
	c := simplex(t)
	for _, tc := range []struct {
		name string
		x, y float32
		want ir.Point
	}{
		{"all of the first", 1, 0, c.a},
		{"all of the second", 0, 1, c.b},
		{"all of the third", 0, 0, c.c},
	} {
		if got := c.Point(tc.x, tc.y); !samePoint(got, tc.want) {
			t.Errorf("%s is at %v, want %v", tc.name, got, tc.want)
		}
	}
	mid := ir.Point{X: (c.a.X + c.b.X + c.c.X) / 3, Y: (c.a.Y + c.b.Y + c.c.Y) / 3}
	if got := c.Point(1.0/3, 1.0/3); !samePoint(got, mid) {
		t.Errorf("the even mixture is at %v, want the centroid %v", got, mid)
	}
}

// The triangle is equilateral and inscribed in the panel, so a chart drawn in
// a square cell is the simplex rather than a sheared picture of it.
func TestATernarysTriangleIsEquilateralAndInscribed(t *testing.T) {
	c := simplex(t)
	ab := math.Hypot(float64(c.b.X-c.a.X), float64(c.b.Y-c.a.Y))
	bc := math.Hypot(float64(c.c.X-c.b.X), float64(c.c.Y-c.b.Y))
	ca := math.Hypot(float64(c.a.X-c.c.X), float64(c.a.Y-c.c.Y))
	if math.Abs(ab-bc) > 1e-3 || math.Abs(bc-ca) > 1e-3 {
		t.Errorf("sides %v, %v, %v, want them equal", ab, bc, ca)
	}
	for _, p := range []ir.Point{c.a, c.b, c.c} {
		if p.X < -0.01 || p.X > 100.01 || p.Y < -0.01 || p.Y > 100.01 {
			t.Errorf("corner %v is outside the panel", p)
		}
	}
}

// A percentage chart is the same triangle read in hundredths, which is what
// most tables in these fields come in.
func TestATernarySumIsTheUnitTheComponentsAreIn(t *testing.T) {
	c := simplex(t, TernarySum(100))
	if got := c.Point(100, 0); !samePoint(got, c.a) {
		t.Errorf("100%% of the first component is at %v, want the corner %v", got, c.a)
	}
	if x0, x1, _, _ := c.Extent(); x0 != 0 || x1 != 100 {
		t.Errorf("extent [%v, %v], want the whole simplex in percent", x0, x1)
	}
}

// Both domains are pinned to the whole simplex whatever the data did, because
// the chart's extent is a fact about the coordinate system. An axis that
// autoscaled to a cluster of compositions would draw a triangle that is not
// one.
func TestATernaryPinsItsDomainsAndMapsTheCompositionItself(t *testing.T) {
	x := scale.Linear(scale.Domain(0.4, 0.6))
	y := scale.Linear(scale.Domain(0.1, 0.2))
	Ternary().Frame(Framing{Area: ir.R(0, 0, 100, 100), X: x, Y: y})
	for name, sc := range map[string]scale.Scale{"x": x, "y": y} {
		if lo, hi := sc.Domain(); lo != 0 || hi != 1 {
			t.Errorf("%s domain [%v, %v], want the whole simplex", name, lo, hi)
		}
		// The range is the domain, so Map is the identity and the pair
		// reaching Point is the composition.
		if got := sc.Map(0.25); math.Abs(float64(got)-0.25) > 1e-6 {
			t.Errorf("%s maps 0.25 to %v, want the value itself", name, got)
		}
	}
	if Steerable(Ternary()) {
		t.Error("a ternary coord reports itself steerable; the triangle does not move")
	}
}

// Invert is the inverse of the map, so a tooltip reports the composition the
// reader is pointing at.
func TestATernaryInvertsToTheCompositionItPlaced(t *testing.T) {
	c := simplex(t)
	for _, want := range [][2]float32{{1, 0}, {0, 1}, {0, 0}, {0.2, 0.5}, {1.0 / 3, 1.0 / 3}} {
		x, y := c.Invert(c.Point(want[0], want[1]))
		if math.Abs(float64(x-want[0])) > 1e-4 || math.Abs(float64(y-want[1])) > 1e-4 {
			t.Errorf("(%v, %v) inverted to (%v, %v)", want[0], want[1], x, y)
		}
	}
}

// An affine map takes a straight segment to a straight segment, which is why
// every geom draws under this coord what it drew under Cartesian.
func TestATernaryIsStraightAndDoesNotDecimate(t *testing.T) {
	c := simplex(t)
	if !c.Straight() {
		t.Error("a ternary edge is not straight, and the map is affine")
	}
	// Three collinear compositions stay collinear on screen.
	p, q, r := c.Point(0, 0), c.Point(0.25, 0.25), c.Point(0.5, 0.5)
	area := (q.X-p.X)*(r.Y-p.Y) - (q.Y-p.Y)*(r.X-p.X)
	if math.Abs(float64(area)) > 1e-3 {
		t.Errorf("three points on one data-space line bend by %v on screen", area)
	}
	if c.Decimates() {
		t.Error("a ternary coord accepts decimation; a screen column is a band of constant b − a")
	}
}

// The clip is the triangle rather than the panel: a point outside the simplex
// is a composition with a part less than nothing or more than everything.
func TestATernaryClipsToItsTriangle(t *testing.T) {
	c := simplex(t)
	var p ir.Path
	c.Clip(&p, ir.R(0, 0, 100, 100))
	if p.Empty() {
		t.Fatal("the clip path is empty")
	}
	var rect ir.Path
	rect.Rect(ir.R(0, 0, 100, 100))
	if len(p.Ops) == len(rect.Ops) && len(p.Pts) == len(rect.Pts) {
		// A triangle and a rectangle differ in their point count; if they did
		// not, the clip would be the panel and the chart would show marks
		// outside the simplex.
		t.Error("the clip path looks like the panel rectangle")
	}
}

// A ternary chart has three grid families and a panel has two tick lists. The
// third is drawn as a second subpath inside the X tick's own shape, which is
// what keeps Furniture from gaining a field.
func TestATernarysThirdGridFamilyRidesOnTheFirstsTicks(t *testing.T) {
	c := simplex(t)
	var fur Furniture
	ticks := []scale.Tick{{Pos: 0.25, Label: "0.25"}, {Pos: 0.5, Label: "0.5"}}
	c.Furniture(&fur, FurnitureRequest{
		Area:    ir.R(0, 0, 100, 100),
		Metrics: Metrics{TickLen: 4, LabelPad: 2},
		XTicks:  ticks, YTicks: ticks,
	})
	if len(fur.GridX) != 2 || len(fur.GridY) != 2 {
		t.Fatalf("%d X and %d Y grid shapes, want one per tick", len(fur.GridX), len(fur.GridY))
	}
	// Two subpaths on X — the constant-a line and the derived constant-c one —
	// and one on Y.
	if got := moves(&fur.GridX[0].Path); got != 2 {
		t.Errorf("the first X tick's grid has %d subpaths, want the constant-a line and the third family", got)
	}
	if got := moves(&fur.GridY[0].Path); got != 1 {
		t.Errorf("the first Y tick's grid has %d subpaths, want one", got)
	}
	// Between them the two axis lines stroke all three sides of the triangle.
	if len(fur.AxisX.Pts) != 2 || len(fur.AxisY.Pts) != 3 {
		t.Errorf("axis runs of %d and %d points, want the triangle's three sides",
			len(fur.AxisX.Pts), len(fur.AxisY.Pts))
	}
}

// A tick outside the simplex is not on the chart, and says so rather than
// being placed somewhere outside the triangle.
func TestATernaryCullsATickOutsideTheSimplex(t *testing.T) {
	c := simplex(t)
	var fur Furniture
	c.Furniture(&fur, FurnitureRequest{
		Area:    ir.R(0, 0, 100, 100),
		Metrics: Metrics{TickLen: 4, LabelPad: 2},
		XTicks:  []scale.Tick{{Pos: -0.5}, {Pos: 0.5}, {Pos: 1.5}},
	})
	want := []bool{false, true, false}
	for i, w := range want {
		if fur.InX[i] != w {
			t.Errorf("tick %d reported in = %v, want %v", i, fur.InX[i], w)
		}
	}
}

// The coord writes itself down and reads back the same triangle.
func TestATernaryRoundTripsThroughItsDesc(t *testing.T) {
	d, ok := Describe(Ternary(TernarySum(100)))
	if !ok {
		t.Fatal("a ternary coord cannot describe itself")
	}
	if d.Type != TypeTernary || d.Sum != 100 {
		t.Fatalf("Describe = %+v, want a ternary summing to 100", d)
	}
	back, err := FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Describe(back)
	if got != d {
		t.Errorf("round trip = %+v, want %+v", got, d)
	}
}

// samePoint compares two device points at the slack a float32 map leaves.
func samePoint(a, b ir.Point) bool {
	return math.Abs(float64(a.X-b.X)) < 1e-3 && math.Abs(float64(a.Y-b.Y)) < 1e-3
}

// moves counts the subpaths of a path, which is how many separate runs of ink
// one piece of furniture holds.
func moves(p *ir.Path) int {
	n := 0
	for _, v := range p.Ops {
		if v == ir.OpMoveTo {
			n++
		}
	}
	return n
}

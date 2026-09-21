package coord

import (
	"math"
	"reflect"
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
// third is a family of the coord's own, which is the field ADR 0070 added so
// that a ladder with no tick behind it can carry its own labels.
func TestATernarysThirdGridFamilyIsItsOwnAndIsLabelled(t *testing.T) {
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
	// One line per tick on each axis, and nothing smuggled in beside it.
	if got := moves(&fur.GridX[0].Path); got != 1 {
		t.Errorf("the first X tick's grid has %d subpaths, want the constant-a line alone", got)
	}
	if got := moves(&fur.GridY[0].Path); got != 1 {
		t.Errorf("the first Y tick's grid has %d subpaths, want one", got)
	}
	if len(fur.Families) != 1 {
		t.Fatalf("%d families, want the derived component's", len(fur.Families))
	}
	fam := fur.Families[0]
	if len(fam.Lines) != 2 || len(fam.Labels) != 2 || len(fam.Text) != 2 {
		t.Fatalf("family has %d lines, %d labels and %d strings, want one of each per level",
			len(fam.Lines), len(fam.Labels), len(fam.Text))
	}
	// It reads the same sequence the first component's ticks do, which is what
	// makes all three ladders one chart rather than three.
	if fam.Text[0] != "0.25" || fam.Text[1] != "0.5" {
		t.Errorf("family labelled %q, want the X ticks' own strings", fam.Text)
	}
	// The level at v is the diagonal a + b = k - v: it meets the b = 0 edge at
	// a = k - v, which is where it is read.
	if got, want := fam.Labels[0].At, c.Point(0.75, 0); !near(got, want, 12) {
		t.Errorf("the first level is labelled at %v, want it beyond %v on the b = 0 edge", got, want)
	}
}

// Each component is read along its own edge, cyclically, which is the
// arrangement every printed ternary chart uses and the one the third family
// needs: the two edges it crosses are the two the other ladders label.
func TestATernaryReadsEachComponentAlongItsOwnEdge(t *testing.T) {
	c := simplex(t)
	var fur Furniture
	ticks := []scale.Tick{{Pos: 0.5, Label: "0.5"}}
	c.Furniture(&fur, FurnitureRequest{
		Area:    ir.R(0, 0, 100, 100),
		Metrics: Metrics{TickLen: 4, LabelPad: 2},
		XTicks:  ticks, YTicks: ticks,
	})
	// The first component is read along the base, between the corner where it
	// is everything and the one where the second is.
	if got, want := fur.LabelX[0].At, c.Point(0.5, 0.5); !near(got, want, 12) {
		t.Errorf("the first component is labelled at %v, want it beyond %v on the base", got, want)
	}
	// The second along the edge where the first is nothing.
	if got, want := fur.LabelY[0].At, c.Point(0, 0.5); !near(got, want, 12) {
		t.Errorf("the second component is labelled at %v, want it beyond %v on the a = 0 edge", got, want)
	}
	// And the three anchors are three different edges, so no two ladders share
	// one and the numbers do not pile up.
	third := fur.Families[0].Labels[0].At
	for _, p := range []ir.Point{fur.LabelY[0].At, third} {
		if near(fur.LabelX[0].At, p, 1) {
			t.Errorf("two ladders are labelled at the same point %v", p)
		}
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
	// Compared with reflect rather than with ==: Desc holds the dimensions of
	// a coord that has more axes than two, so it is no longer comparable.
	if !reflect.DeepEqual(got, d) {
		t.Errorf("round trip = %+v, want %+v", got, d)
	}
}

// samePoint compares two device points at the slack a float32 map leaves.
func samePoint(a, b ir.Point) bool {
	return math.Abs(float64(a.X-b.X)) < 1e-3 && math.Abs(float64(a.Y-b.Y)) < 1e-3
}

// near reports whether a is within d of b, which is how a test says "just
// outside this edge" without restating the label gap.
func near(a, b ir.Point, d float64) bool {
	return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)) <= d
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

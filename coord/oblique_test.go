package coord_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/ir"
)

func TestAnObliqueCoordFramesItsFrontPlaneInsideThePanel(t *testing.T) {
	x, y := linear(0, 10), linear(0, 10)
	area := ir.R(0, 0, 400, 300)
	cd := coord.Oblique(coord.Depth(0.1)).Frame(coord.Framing{Area: area, X: x, Y: y})

	e, ok := cd.(coord.Extruder)
	if !ok {
		t.Fatal("a framed oblique coord does not report a depth vector")
	}
	dx, dy := e.Extrude()
	// A tenth of the shorter side, 30 px, up and to the right.
	d := float32(30 / math.Sqrt2)
	near(t, dx, d, "dx")
	near(t, dy, -d, "dy")

	// The scales map into the panel less the depth, on the sides the depth
	// points to: the volume comes out of the plot area, not out of the margin.
	x0, x1, y0, y1 := cd.Extent()
	near(t, x0, 0, "x0")
	near(t, x1, 400-d, "x1")
	near(t, y0, 300, "y0")
	near(t, y1, d, "y1")
	near(t, x.Map(10), 400-d, "X.Map(max)")
	near(t, y.Map(10), d, "Y.Map(max)")

	// Everything else is Cartesian.
	if p := cd.Point(12, 34); p.X != 12 || p.Y != 34 {
		t.Errorf("Point is not the identity: %v", p)
	}
	if !cd.Straight() || !cd.Decimates() {
		t.Error("an oblique coord must keep Cartesian's straight edges and its decimation")
	}
}

func TestCartesianAndPolarHaveNoDepth(t *testing.T) {
	area := ir.R(0, 0, 400, 300)
	for name, cd := range map[string]coord.Coord{
		"cartesian": coord.Cartesian().Frame(coord.Framing{Area: area}),
		"polar":     coord.Polar().Frame(coord.Framing{Area: area}),
	} {
		if _, ok := cd.(coord.Extruder); ok {
			t.Errorf("%s implements Extruder; a layer asking for volume there would invent it", name)
		}
	}
}

func TestAnObliqueCoordSurvivesItsDesc(t *testing.T) {
	area := ir.R(0, 0, 400, 300)
	for name, c := range map[string]coord.Coord{
		"default":  coord.Oblique(),
		"deep":     coord.Oblique(coord.Depth(0.2), coord.DepthAngle(-2.5)),
		"straight": coord.Oblique(coord.DepthAngle(0)),
		"clamped":  coord.Oblique(coord.Depth(9)),
	} {
		d, ok := coord.Describe(c)
		if !ok || d.Type != coord.TypeOblique {
			t.Fatalf("%s: Desc = %+v", name, d)
		}
		back, err := coord.FromDesc(d)
		if err != nil {
			t.Fatal(err)
		}
		dx1, dy1 := c.Frame(coord.Framing{Area: area}).(coord.Extruder).Extrude()
		dx2, dy2 := back.Frame(coord.Framing{Area: area}).(coord.Extruder).Extrude()
		near(t, dx2, dx1, name+" dx")
		near(t, dy2, dy1, name+" dy")
	}

	// A Desc that names the type and nothing else draws what Oblique draws.
	zero, _ := coord.FromDesc(coord.Desc{Type: coord.TypeOblique})
	dx1, dy1 := zero.Frame(coord.Framing{Area: area}).(coord.Extruder).Extrude()
	dx2, dy2 := coord.Oblique().Frame(coord.Framing{Area: area}).(coord.Extruder).Extrude()
	near(t, dx1, dx2, "zero Desc dx")
	near(t, dy1, dy2, "zero Desc dy")

	// A quarter of the shorter side is as deep as it goes.
	dx, _ := coord.Oblique(coord.Depth(9), coord.DepthAngle(0)).Frame(coord.Framing{Area: area}).(coord.Extruder).Extrude()
	near(t, dx, 75, "clamped depth")
}

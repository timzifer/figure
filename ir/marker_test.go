package ir_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/ir"
)

// The downward triangle is the upward one mirrored through its centroid, so a
// buy and a sell drawn at the same point occupy the same box turned over.
func TestTheDownwardTriangleMirrorsTheUpwardOne(t *testing.T) {
	var up, down ir.Path
	ir.MarkerPath(&up, ir.MarkerTriangle, 10)
	ir.MarkerPath(&down, ir.MarkerTriangleDown, 10)

	u, d := up.Bounds(), down.Bounds()
	const eps = 1e-5
	near := func(a, b float32) bool { return math.Abs(float64(a-b)) < eps }
	if !near(u.Min.X, d.Min.X) || !near(u.Max.X, d.Max.X) {
		t.Errorf("widths differ: up %v, down %v", u, d)
	}
	if !near(u.Min.Y, -d.Max.Y) || !near(u.Max.Y, -d.Min.Y) {
		t.Errorf("down %v is not up %v mirrored through the centre", d, u)
	}

	// The apex is the point that tells the two apart: above the centre for
	// one, below it for the other, in a y-down device space.
	var upApex, downApex ir.Point
	up.Walk(func(op ir.PathOp, pts []ir.Point) {
		if op == ir.OpMoveTo {
			upApex = pts[0]
		}
	})
	down.Walk(func(op ir.PathOp, pts []ir.Point) {
		if op == ir.OpMoveTo {
			downApex = pts[0]
		}
	})
	if upApex.Y >= 0 || downApex.Y <= 0 {
		t.Errorf("apexes at %v and %v, want one above and one below the centre", upApex, downApex)
	}
}

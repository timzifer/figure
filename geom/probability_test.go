package geom_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

// ADR 0052: an ECDF on a probit axis is a normal probability plot, with no
// mark of its own.
func TestAnECDFOnAProbabilityAxisIsAProbabilityPlot(t *testing.T) {
	g := geom.ECDF(src(map[string][]float64{"x": {3, 1, 4, 1, 5}}), geom.X("x"))
	y := scale.Probability(scale.Probit)
	rec := build(t, g, scale.Linear(), y)

	lines := rec.Filter("Polyline")
	if len(lines) != 1 {
		t.Fatalf("got %d polylines, want one unbroken staircase", len(lines))
	}
	for _, p := range lines[0].Points {
		if math.IsNaN(float64(p.X)) || math.IsNaN(float64(p.Y)) {
			t.Fatal("a NaN coordinate reached the backend: the 0 and 1 of the staircase have no place on this axis")
		}
	}
	// Distinct values 1, 3, 4, 5 reach 0.4, 0.6, 0.8 and 1: the 0 and the 1
	// are dropped, which leaves six of the eight vertices.
	if n := len(lines[0].Points); n != 6 {
		t.Errorf("%d vertices, want 6", n)
	}
	if lo, hi := y.Domain(); lo != 0.4 || hi != 0.8 {
		t.Errorf("Y domain = [%v, %v], want the fractions the curve reaches, [0.4, 0.8]", lo, hi)
	}
}

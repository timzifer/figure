package geom_test

import (
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

// fitted trains g with the view [lo, hi] and returns the value axis's extent.
func fitted(t *testing.T, g geom.Geom, lo, hi float64) (float64, float64) {
	t.Helper()
	y := scale.Linear()
	if err := g.Train(geom.Training{X: scale.Linear(), Y: y, Within: &scale.Interval{Lo: lo, Hi: hi}}); err != nil {
		t.Fatal(err)
	}
	return y.Domain()
}

func stairs() *data.Table {
	return data.NewTable().
		Float64("x", []float64{0, 10, 20, 30}).
		Float64("y", []float64{1, 5, 9, 2}).
		String("g", []string{"a", "a", "b", "b"})
}

func TestAPointMarkFitsTheRowsInView(t *testing.T) {
	if lo, hi := fitted(t, geom.Scatter(stairs(), geom.X("x"), geom.Y("y")), 5, 25); lo != 5 || hi != 9 {
		t.Errorf("[%v, %v], want the two points in view", lo, hi)
	}
}

// A staircase holds its value to the edge, so it fits both values of the step
// it crosses on rather than one between them.
func TestAStepFitsTheStepsItCrossesOn(t *testing.T) {
	if lo, hi := fitted(t, geom.Step(stairs(), geom.X("x"), geom.Y("y")), 12, 18); lo != 5 || hi != 9 {
		t.Errorf("[%v, %v], want the step from 5 to 9 it crosses", lo, hi)
	}
}

// A line joins rows within their group only, so no value is interpolated
// between the last row of one series and the first of the next.
func TestALineInterpolatesWithinItsGroupOnly(t *testing.T) {
	g := geom.Line(stairs(), geom.X("x"), geom.Y("y"), geom.GroupBy("g"))
	// Joined across the groups, the rows at 10 and 20 would put 5.8 and 7.4
	// into the axis; an axis nothing trained reports its default instead.
	if lo, hi := fitted(t, g, 12, 18); hi >= 5 {
		t.Errorf("[%v, %v], want nothing in view — the gap between the series is not a line", lo, hi)
	}
}

// A bar is in view when its width is.
func TestABarHalfInViewFits(t *testing.T) {
	if lo, hi := fitted(t, geom.Bar(stairs(), geom.X("x"), geom.Y("y"), geom.BarWidth(0.8)), 23, 26); lo != 0 || hi != 9 {
		t.Errorf("[%v, %v], want the bar at 20 — reaching to 24 — and the baseline", lo, hi)
	}
}

// A stack fits its totals in view.
func TestAStackFitsItsTotalsInView(t *testing.T) {
	tb := data.NewTable().
		Float64("x", []float64{0, 0, 10, 10}).
		Float64("y", []float64{1, 2, 50, 50}).
		String("s", []string{"a", "b", "a", "b"})
	if lo, hi := fitted(t, geom.Bar(tb, geom.X("x"), geom.Y("y"), geom.GroupBy("s")), -2, 2); lo != 0 || hi != 3 {
		t.Errorf("[%v, %v], want the first stack's total of 3", lo, hi)
	}
}

package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// panel is a Nichols chart's own extent: a turn of phase, eighty decibels.
var panel = stat.Extent{X0: -360, X1: 0, Y0: -40, Y1: 40, Steps: 360}

// disc is a Smith chart's: the domains a paper chart's grid is pinned to.
var disc = stat.Extent{X0: 0, X1: 50, Y0: -50, Y1: 50, Steps: 360}

func curve(t *testing.T, f stat.Family, level float64, ext stat.Extent) (xs, ys []float64) {
	t.Helper()
	xs, ys = f.Locus(nil, nil, level, ext)
	if len(xs) != len(ys) {
		t.Fatalf("%v at %v appended %d x values and %d y values", f, level, len(xs), len(ys))
	}
	if len(xs) == 0 {
		t.Fatalf("%v at %v drew nothing", f, level)
	}
	return xs, ys
}

// The determinism rule this package is under: a parallel render has to be byte
// identical to a serial one, so every curve is the same numbers twice.
func TestAFamilyIsDeterministic(t *testing.T) {
	for _, c := range []struct {
		f     stat.Family
		level float64
		ext   stat.Extent
	}{
		{stat.NicholsM, -3, panel},
		{stat.NicholsM, 0, panel},
		{stat.NicholsN, -45, panel},
		{stat.SmithVSWR, 2, disc},
		{stat.SmithQ, 3, disc},
	} {
		xs, ys := c.f.Locus(nil, nil, c.level, c.ext)
		again, alsoAgain := c.f.Locus(nil, nil, c.level, c.ext)
		if len(xs) != len(again) {
			t.Fatalf("%v at %v drew %d points and then %d", c.f, c.level, len(xs), len(again))
		}
		for i := range xs {
			if !same(xs[i], again[i]) || !same(ys[i], alsoAgain[i]) {
				t.Fatalf("%v at %v moved point %d from (%v, %v) to (%v, %v)",
					c.f, c.level, i, xs[i], ys[i], again[i], alsoAgain[i])
			}
		}
	}
}

func same(a, b float64) bool { return a == b || (math.IsNaN(a) && math.IsNaN(b)) }

// The M contour is what it says it is: every point on it is an open loop whose
// closed loop has the magnitude the level names.
func TestAnMContourHoldsItsClosedLoopMagnitude(t *testing.T) {
	for _, db := range []float64{-12, -6, -3, -1, 0, 1, 3, 6, 12} {
		xs, ys := curve(t, stat.NicholsM, db, panel)
		want := math.Pow(10, db/20)
		for i := range xs {
			if math.IsNaN(xs[i]) {
				continue
			}
			l := open(xs[i], ys[i])
			got := cabs(l / (1 + l))
			if math.Abs(20*math.Log10(got)-db) > 1e-6 {
				t.Fatalf("the %v dB contour passes through L = %v, where |T| is %v and not %v",
					db, l, got, want)
			}
		}
	}
}

func TestAnNContourHoldsItsClosedLoopPhase(t *testing.T) {
	for _, deg := range []float64{-1, -5, -10, -20, -45, -90, -150} {
		xs, ys := curve(t, stat.NicholsN, deg, panel)
		for i := range xs {
			if math.IsNaN(xs[i]) {
				continue
			}
			l := open(xs[i], ys[i])
			if cabs(1+l) < 1e-9 {
				// The critical point itself, which every N contour passes
				// through and where T is infinite: the closed loop has no phase
				// there, and the chart draws it as the point they converge on.
				continue
			}
			got := math.Atan2(imag(l/(1+l)), real(l/(1+l))) * 180 / math.Pi
			// Modulo half a turn: ∠T = α and ∠T = α ± 180° are the two halves
			// of one circle, which is the curve the level names.
			d := got - deg
			d -= 180 * math.Round(d/180)
			if math.Abs(d) > 1e-6 {
				t.Fatalf("the %v° contour passes through L = %v, where ∠T is %v°", deg, l, got)
			}
		}
	}
}

// The regression this mark's sampling exists for. An N contour passes through
// L = 0, so it plunges to −∞ dB, and a uniform walk round its circle resolves
// that arc with one sample in a thousand: the −1° contour used to stop dead at
// −6 dB, which draws a contour hanging in mid-air a long way above the bottom
// of the panel.
func TestEveryNContourLeavesTheBottomOfThePanel(t *testing.T) {
	for _, deg := range []float64{-1, -5, -10, -20, -45, -90, -150, -270, -359} {
		_, ys := curve(t, stat.NicholsN, deg, panel)
		lo := math.Inf(1)
		for _, y := range ys {
			if !math.IsNaN(y) {
				lo = math.Min(lo, y)
			}
		}
		if lo > panel.Y0 {
			t.Errorf("the %v° contour stops at %.1f dB, above the panel's own %.0f dB", deg, lo, panel.Y0)
		}
	}
}

// Its other end is the critical point — 0 dB at half a turn — which is where
// every N contour converges and what the chart is read against. Close in the
// chart rather than close in the L-plane, because what a reader has to see is
// the contours meeting: a curve that stopped a degree short would be a family
// that frays where it matters most.
func TestEveryNContourReachesTheCriticalPoint(t *testing.T) {
	for _, deg := range []float64{-1, -45, -90, -150, -270} {
		xs, ys := curve(t, stat.NicholsN, deg, panel)
		phase, gain := math.Inf(1), math.Inf(1)
		for i := range xs {
			if math.IsNaN(xs[i]) {
				continue
			}
			d := xs[i] - 180
			d -= 360 * math.Round(d/360)
			if math.Abs(d) < phase {
				phase, gain = math.Abs(d), math.Abs(ys[i])
			}
		}
		if phase > 2 || gain > 0.5 {
			t.Errorf("the %v° contour comes no closer to the critical point than %.2f° and %.2f dB",
				deg, phase, gain)
		}
	}
}

// A Nichols family repeats every 360° of phase, so a panel showing a whole turn
// shows the whole family wherever the turn happens to start.
func TestANicholsFamilyRepeatsWithThePhase(t *testing.T) {
	here := stat.Extent{X0: -360, X1: 0, Y0: -40, Y1: 40, Steps: 360}
	there := stat.Extent{X0: 0, X1: 360, Y0: -40, Y1: 40, Steps: 360}
	for _, f := range []stat.Family{stat.NicholsM, stat.NicholsN} {
		a, _ := curve(t, f, -3, here)
		b, _ := curve(t, f, -3, there)
		if !covers(a, here) {
			t.Errorf("%v drew nothing in [%v, %v]", f, here.X0, here.X1)
		}
		if !covers(b, there) {
			t.Errorf("%v drew nothing in [%v, %v]", f, there.X0, there.X1)
		}
	}
}

func covers(xs []float64, ext stat.Extent) bool {
	for _, x := range xs {
		if !math.IsNaN(x) && x >= ext.X0 && x <= ext.X1 {
			return true
		}
	}
	return false
}

// The one degenerate member of either family: |T| = 1 is the perpendicular
// bisector of the origin and the critical point, which is a line and not a
// circle. Drawn as a circle it would be an arc of radius infinity through the
// wrong points.
func TestTheZeroDecibelMContourIsALine(t *testing.T) {
	xs, ys := curve(t, stat.NicholsM, 0, panel)
	for i := range xs {
		if math.IsNaN(xs[i]) {
			continue
		}
		if got := real(open(xs[i], ys[i])); math.Abs(got+0.5) > 1e-9 {
			t.Fatalf("the 0 dB contour passes through an L whose real part is %v, want −1/2", got)
		}
	}
	if len(ys) < 2 {
		t.Fatal("the 0 dB contour is a line and was drawn as a point")
	}
}

// A VSWR circle crosses the resistance axis at the ratio and at its reciprocal,
// which is the arithmetic every RF textbook states it by.
func TestAVSWRCircleCrossesTheAxisAtTheRatioAndItsReciprocal(t *testing.T) {
	for _, s := range []float64{1.5, 2, 3, 5} {
		xs, ys := curve(t, stat.SmithVSWR, s, disc)
		lo, hi := math.Inf(1), math.Inf(-1)
		for i := range xs {
			lo, hi = math.Min(lo, xs[i]), math.Max(hi, xs[i])
			if math.IsNaN(ys[i]) {
				t.Fatalf("the VSWR %v circle is broken, and a circle is one run", s)
			}
		}
		if math.Abs(lo-1/s) > 1e-9 || math.Abs(hi-s) > 1e-9 {
			t.Errorf("the VSWR %v circle runs from r = %v to r = %v, want %v to %v", s, lo, hi, 1/s, s)
		}
		if xs[0] != xs[len(xs)-1] || ys[0] != ys[len(ys)-1] {
			t.Errorf("the VSWR %v circle does not close: it starts at (%v, %v) and ends at (%v, %v)",
				s, xs[0], ys[0], xs[len(xs)-1], ys[len(ys)-1])
		}
	}
}

// A matched load is the middle of the chart: a point, which no stroke draws.
func TestAVSWROfOneDrawsNothing(t *testing.T) {
	if xs, _ := stat.SmithVSWR.Locus(nil, nil, 1, disc); len(xs) != 0 {
		t.Errorf("VSWR 1 drew %d points, want none", len(xs))
	}
}

// A Q arc is the locus |x| = Q·r: two rays in impedance, which is what the disc
// turns into two arcs. Both halves are drawn, because a reactance of either
// sign has the same Q.
func TestAQArcHoldsItsRatioInBothHalves(t *testing.T) {
	for _, q := range []float64{1, 2, 5} {
		xs, ys := curve(t, stat.SmithQ, q, disc)
		up, down, gaps := 0, 0, 0
		for i := range xs {
			if math.IsNaN(xs[i]) {
				gaps++
				continue
			}
			if xs[i] < 1e-12 {
				continue
			}
			if got := math.Abs(ys[i]) / xs[i]; math.Abs(got-q) > 1e-9 {
				t.Fatalf("the Q = %v arc passes through z = %v + j%v, whose ratio is %v", q, xs[i], ys[i], got)
			}
			if ys[i] > 0 {
				up++
			} else if ys[i] < 0 {
				down++
			}
		}
		if up == 0 || down == 0 {
			t.Errorf("the Q = %v arc drew %d points above the axis and %d below", q, up, down)
		}
		if gaps != 1 {
			t.Errorf("the Q = %v arc has %d gaps in it, want one between its two rays", q, gaps)
		}
	}
}

// Every family answers an extent it has never been told anything about, because
// that is what it is handed before the domains exist — which is how a locus
// trains an axis when a caller asks it to.
func TestAFamilyAnswersAnEmptyExtent(t *testing.T) {
	for _, c := range []struct {
		f     stat.Family
		level float64
	}{{stat.NicholsM, -3}, {stat.NicholsN, -45}, {stat.SmithVSWR, 2}, {stat.SmithQ, 3}} {
		xs, ys := curve(t, c.f, c.level, stat.Extent{})
		for i := range xs {
			if math.IsInf(xs[i], 0) || math.IsInf(ys[i], 0) {
				t.Fatalf("%v drew the point (%v, %v), which is not a position", c.f, xs[i], ys[i])
			}
		}
	}
}

// A named family is written down as its name and read back by it. A family a
// caller wrote in Go is a perfectly good family and has neither.
func TestOnlyTheFamiliesThisPackageNamesHaveNames(t *testing.T) {
	for _, f := range []stat.Family{stat.NicholsM, stat.NicholsN, stat.SmithVSWR, stat.SmithQ} {
		name, ok := stat.FamilyName(f)
		if !ok {
			t.Fatalf("%v has no name", f)
		}
		back, ok := stat.FamilyNamed(name)
		if !ok || back != f {
			t.Errorf("%q read back as %v", name, back)
		}
	}
	if _, ok := stat.FamilyName(mine{}); ok {
		t.Error("a family written outside this package claimed a name")
	}
	if _, ok := stat.FamilyNamed("nichols-p"); ok {
		t.Error("a name no family answers to resolved anyway")
	}
}

type mine struct{}

func (mine) Locus(xs, ys []float64, level float64, ext stat.Extent) ([]float64, []float64) {
	return append(xs, 0, 1), append(ys, level, level)
}

// The append forms are what a chart redrawn every frame calls: a family writes
// into the caller's buffers and keeps none of its own.
func TestAFamilyAppendsIntoTheCallersBuffers(t *testing.T) {
	xs, ys := make([]float64, 0, 4096), make([]float64, 0, 4096)
	n := int(testing.AllocsPerRun(20, func() {
		xs, ys = stat.NicholsM.Locus(xs[:0], ys[:0], -3, panel)
	}))
	if n > 1 {
		t.Errorf("a redrawn curve allocated %d times, want none", n)
	}
}

// open is the point of the L-plane a Nichols sample names: the phase in degrees
// and the magnitude in decibels are polar coordinates of it.
func open(phase, db float64) complex128 {
	r, th := math.Pow(10, db/20), phase*math.Pi/180
	return complex(r*math.Cos(th), r*math.Sin(th))
}

func cabs(z complex128) float64 { return math.Hypot(real(z), imag(z)) }

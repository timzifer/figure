package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// smoothRamp builds n evenly spaced abscissae and the ys the given function gives.
func smoothRamp(n int, f func(x float64) float64) (xs, ys []float64) {
	xs = make([]float64, n)
	ys = make([]float64, n)
	for i := range n {
		xs[i] = float64(i)
		ys[i] = f(xs[i])
	}
	return xs, ys
}

// A mean of one value repeated is that value, at every row including the ends
// — which is the assertion that the partial windows at the edges are averaged
// over what is there rather than padded with zeroes.
func TestMovingAverageOfAConstantIsThatConstant(t *testing.T) {
	xs, ys := smoothRamp(20, func(float64) float64 { return 7 })
	for _, w := range []int{1, 3, 5, 9, 21} {
		for i, p := range stat.MovingAverage(xs, ys, w) {
			if math.Abs(p.Y-7) > 1e-12 {
				t.Fatalf("window %d, row %d: got %v, want 7", w, i, p.Y)
			}
		}
	}
}

// The running sum is carried rather than recomputed, so this is also the test
// that carrying it has not drifted: every window is compared against the mean
// taken the slow way.
func TestMovingAverageMatchesTheDirectMean(t *testing.T) {
	xs, ys := smoothRamp(31, func(x float64) float64 { return math.Sin(x/3) * 10 })
	const w = 7
	got := stat.MovingAverage(xs, ys, w)
	for i := range xs {
		lo, hi := max(i-w/2, 0), min(i+w/2+1, len(xs))
		sum := 0.0
		for j := lo; j < hi; j++ {
			sum += ys[j]
		}
		want := sum / float64(hi-lo)
		if math.Abs(got[i].Y-want) > 1e-9 {
			t.Fatalf("row %d: got %v, want %v", i, got[i].Y, want)
		}
	}
}

// The property that separates the two smoothers. A quadratic is in the space
// an order-two fit can represent, so Savitzky-Golay returns it untouched —
// which means it returns a peak at its real height. A running mean over the
// same window does not, and the second half of the test says so, because
// otherwise the first half would pass for a smoother that did nothing at all.
func TestSavitzkyGolayReproducesItsOwnDegree(t *testing.T) {
	q := func(x float64) float64 { return 3*x*x - 5*x + 2 }
	xs, ys := smoothRamp(25, q)

	for i, p := range stat.SavitzkyGolay(xs, ys, 9, 2) {
		if math.Abs(p.Y-q(xs[i])) > 1e-6 {
			t.Fatalf("row %d: got %v, want the quadratic's own %v", i, p.Y, q(xs[i]))
		}
	}

	mean := stat.MovingAverage(xs, ys, 9)
	off := false
	for i := range xs {
		if math.Abs(mean[i].Y-q(xs[i])) > 1e-6 {
			off = true
		}
	}
	if !off {
		t.Error("a running mean reproduced the quadratic too, so the test proves nothing about the fit")
	}
}

// A cubic is outside an order-two fit's reach and inside an order-three one's.
// That is the only thing the order knob does, so it is the thing to assert
// about it.
func TestSavitzkyGolayOrderDecidesWhatSurvives(t *testing.T) {
	c := func(x float64) float64 { return x*x*x/50 - x*x + 4*x }
	xs, ys := smoothRamp(25, c)

	exact := stat.SavitzkyGolay(xs, ys, 11, 3)
	for i, p := range exact {
		if math.Abs(p.Y-c(xs[i])) > 1e-6 {
			t.Fatalf("order 3, row %d: got %v, want the cubic's own %v", i, p.Y, c(xs[i]))
		}
	}
	rough := stat.SavitzkyGolay(xs, ys, 11, 2)
	off := false
	for i := range xs {
		if math.Abs(rough[i].Y-c(xs[i])) > 1e-6 {
			off = true
		}
	}
	if !off {
		t.Error("an order-two fit reproduced a cubic exactly, which it cannot do")
	}
}

// Unevenly spaced rows are the case the tabulated Savitzky-Golay coefficients
// cannot take and this one can, so it is worth an assertion of its own: the
// fit is against the real abscissae, and a quadratic still comes back whole.
func TestSavitzkyGolayTakesUnevenAbscissae(t *testing.T) {
	q := func(x float64) float64 { return x*x - x }
	xs := []float64{0, 0.5, 0.7, 2, 2.1, 5, 9, 9.5, 12, 20}
	ys := make([]float64, len(xs))
	for i, x := range xs {
		ys[i] = q(x)
	}
	for i, p := range stat.SavitzkyGolay(xs, ys, 5, 2) {
		if math.Abs(p.Y-q(xs[i])) > 1e-6 {
			t.Fatalf("row %d at x=%v: got %v, want %v", i, xs[i], p.Y, q(xs[i]))
		}
	}
}

// Both smoothers take the narrower contract [stat.Loess] takes, and both have
// to survive the degenerate ends of it rather than panicking on them.
func TestSmoothersSurviveShortAndRaggedColumns(t *testing.T) {
	cases := []struct{ xs, ys []float64 }{
		{nil, nil},
		{[]float64{1}, []float64{2}},
		{[]float64{1, 2}, []float64{3}},          // the shorter column wins
		{[]float64{1, 1, 1}, []float64{1, 2, 3}}, // no distinct abscissae
		{[]float64{0, 1, 2}, []float64{0, 1, 2}},
	}
	for i, c := range cases {
		n := min(len(c.xs), len(c.ys))
		if got := stat.MovingAverage(c.xs, c.ys, 5); len(got) != n {
			t.Errorf("case %d: moving average returned %d points, want %d", i, len(got), n)
		}
		if got := stat.SavitzkyGolay(c.xs, c.ys, 5, 2); len(got) != n {
			t.Errorf("case %d: savitzky-golay returned %d points, want %d", i, len(got), n)
		}
	}
}

// dst is caller-owned so a chart redrawn every frame fits into the same
// memory. The test is that a second pass over the same buffer allocates
// nothing at all.
func TestSmoothersReuseTheCallersBuffer(t *testing.T) {
	xs, ys := smoothRamp(256, func(x float64) float64 { return math.Sin(x / 5) })
	dst := make([]stat.Point, 0, len(xs))

	if n := testing.AllocsPerRun(20, func() {
		dst = stat.AppendMovingAverage(dst, xs, ys, 11)
	}); n != 0 {
		t.Errorf("moving average allocated %v times per call", n)
	}
	if n := testing.AllocsPerRun(20, func() {
		dst = stat.AppendSavitzkyGolay(dst, xs, ys, 11, 2)
	}); n != 0 {
		t.Errorf("savitzky-golay allocated %v times per call", n)
	}
}

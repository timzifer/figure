package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// The question an image asks that a contour does not: are the cells the same
// size. A grid that is not a grid of equal cells cannot be blitted.
func TestStepReportsAnEvenAxisAndRefusesAnUnevenOne(t *testing.T) {
	cases := []struct {
		name string
		vs   []float64
		want float64
		ok   bool
	}{
		{"unit", []float64{0, 1, 2, 3}, 1, true},
		{"fractional", []float64{0, 0.1, 0.2, 0.3, 0.4}, 0.1, true},
		{"negative through zero", []float64{-2, -1, 0, 1}, 1, true},
		{"two values", []float64{4, 9}, 5, true},
		{"a gap", []float64{1, 2, 10}, 0, false},
		{"logarithmic", []float64{1, 2, 4, 8}, 0, false},
		{"one value", []float64{3}, 0, false},
		{"none", nil, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := stat.Step(c.vs)
			if ok != c.ok {
				t.Fatalf("Step(%v) = %v, %v; want ok = %v", c.vs, got, ok, c.ok)
			}
			if ok && math.Abs(got-c.want) > 1e-12 {
				t.Errorf("Step(%v) = %v, want %v", c.vs, got, c.want)
			}
		})
	}
}

// A lattice of timestamps is even in the clock's units and not in the
// machine's: a second either side of 1.7e9 is a number whose last bits are
// already spent, and an axis refused for that would be an axis refused for
// being large.
func TestStepAllowsTheErrorALargeAxisCarries(t *testing.T) {
	vs := make([]float64, 2048)
	for i := range vs {
		vs[i] = 1.7e9 + float64(i)/44100
	}
	if _, ok := stat.Step(vs); !ok {
		t.Error("an evenly sampled second of audio, timestamped, was called uneven")
	}
}

// grid is a lattice over a table whose value at (i, j) is f(i, j).
func grid(t *testing.T, nx, ny int, f func(i, j int) float64) *stat.Lattice {
	t.Helper()
	xs := make([]float64, 0, nx*ny)
	ys := make([]float64, 0, nx*ny)
	vs := make([]float64, 0, nx*ny)
	for j := range ny {
		for i := range nx {
			xs, ys = append(xs, float64(i)), append(ys, float64(j))
			vs = append(vs, f(i, j))
		}
	}
	var l stat.Lattice
	if fault := l.Reset(xs, ys, vs); fault != stat.LatticeOK {
		t.Fatalf("the table is not a lattice: fault %v", fault)
	}
	return &l
}

// The two reductions over the cells one pixel covers. Nearest is a block of
// one, which is why there is no third.
func TestMeanAndMaxReduceABlock(t *testing.T) {
	l := grid(t, 4, 4, func(i, j int) float64 { return float64(4*j + i) })
	b := stat.Block{X0: 1, X1: 3, Y0: 0, Y1: 2}
	// The cells 1, 2, 5, 6.
	if got := l.Mean(b); got != 3.5 {
		t.Errorf("the mean of the block is %v, want 3.5", got)
	}
	if got := l.Max(b); got != 6 {
		t.Errorf("the largest of the block is %v, want 6", got)
	}
	if got := l.Mean(stat.Block{X0: 2, X1: 3, Y0: 2, Y1: 3}); got != 10 {
		t.Errorf("a block of one cell reads %v, want that cell", got)
	}
}

// A hole is skipped rather than counted as a zero, which is the lattice's own
// rule: nothing was measured there, and a mean that counted it would report a
// reading nobody took.
func TestAHoleIsSkippedRatherThanCountedAsZero(t *testing.T) {
	l := grid(t, 2, 2, func(i, j int) float64 {
		if i == 0 && j == 0 {
			return math.NaN()
		}
		return 6
	})
	whole := stat.Block{X0: 0, X1: 2, Y0: 0, Y1: 2}
	if got := l.Mean(whole); got != 6 {
		t.Errorf("the mean over a hole is %v, want the mean of what was measured", got)
	}
	if got := l.Max(whole); got != 6 {
		t.Errorf("the largest over a hole is %v, want 6", got)
	}
}

// A block with nothing in it is NaN rather than zero, so that the mark drawing
// it leaves the pixel transparent instead of painting the bottom of the ramp.
func TestAnEmptyBlockIsNotANumber(t *testing.T) {
	l := grid(t, 2, 2, func(i, j int) float64 { return math.NaN() })
	for _, b := range []stat.Block{
		{X0: 0, X1: 2, Y0: 0, Y1: 2}, // every cell a hole
		{X0: 5, X1: 7, Y0: 0, Y1: 1}, // off the lattice entirely
		{X0: 1, X1: 1, Y0: 0, Y1: 1}, // no cells at all
	} {
		if got := l.Mean(b); !math.IsNaN(got) {
			t.Errorf("the mean of %+v is %v, want NaN", b, got)
		}
		if got := l.Max(b); !math.IsNaN(got) {
			t.Errorf("the largest of %+v is %v, want NaN", b, got)
		}
	}
}

// A block reaching past the lattice reads the cells that are there, which is
// what the pixels at the edge of a panel ask for.
func TestABlockIsClampedToTheLattice(t *testing.T) {
	l := grid(t, 3, 3, func(i, j int) float64 { return 2 })
	if got := l.Mean(stat.Block{X0: -4, X1: 40, Y0: -4, Y1: 40}); got != 2 {
		t.Errorf("a block past both edges reads %v, want the field's own value", got)
	}
}

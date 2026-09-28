package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// trailing returns the rows of the trailing window of width w ending at row i,
// the slow way, for the tests to compare the carried forms against.
func trailing(ys []float64, i, w int) []float64 { return ys[max(i-w+1, 0) : i+1] }

func wobble(n int) (xs, ys []float64) {
	return smoothRamp(n, func(x float64) float64 { return 100 + math.Sin(x/3)*10 + math.Cos(x/7)*4 })
}

// Every trailing form is compared against the window taken the slow way, at
// every row including the partial windows at the start.
func TestTrailingWindowsMatchTheDirectForm(t *testing.T) {
	xs, ys := wobble(53)
	direct := map[string]func(win []float64) float64{
		"mean": func(win []float64) float64 {
			s := 0.0
			for _, v := range win {
				s += v
			}
			return s / float64(len(win))
		},
		"sd": func(win []float64) float64 {
			m := 0.0
			for _, v := range win {
				m += v
			}
			m /= float64(len(win))
			s := 0.0
			for _, v := range win {
				s += (v - m) * (v - m)
			}
			return math.Sqrt(s / float64(len(win)))
		},
		"min": func(win []float64) float64 {
			m := win[0]
			for _, v := range win {
				m = math.Min(m, v)
			}
			return m
		},
		"max": func(win []float64) float64 {
			m := win[0]
			for _, v := range win {
				m = math.Max(m, v)
			}
			return m
		},
	}
	fast := map[string]func(xs, ys []float64, w int) []stat.Point{
		"mean": stat.TrailingMean,
		"sd":   stat.RollingStdDev,
		"min":  stat.RollingMin,
		"max":  stat.RollingMax,
	}
	for name, f := range fast {
		for _, w := range []int{1, 2, 5, 20, 100} {
			got := f(xs, ys, w)
			if len(got) != len(xs) {
				t.Fatalf("%s window %d: %d points for %d rows", name, w, len(got), len(xs))
			}
			for i := range xs {
				want := direct[name](trailing(ys, i, min(w, len(xs))))
				if math.Abs(got[i].Y-want) > 1e-9 || got[i].X != xs[i] {
					t.Fatalf("%s window %d, row %d: got %v, want (%v, %v)", name, w, i, got[i], xs[i], want)
				}
			}
		}
	}
}

// The property the trailing forms exist for: a row appended to the column
// changes no value already computed. A centred mean fails it at its last rows,
// and the second half of the test says so.
func TestAppendingARowChangesNothingAlreadyDrawn(t *testing.T) {
	xs, ys := wobble(40)
	for name, f := range map[string]func(xs, ys []float64, w int) []stat.Point{
		"mean": stat.TrailingMean,
		"ema":  stat.EMA,
		"sd":   stat.RollingStdDev,
		"min":  stat.RollingMin,
		"max":  stat.RollingMax,
		"sum":  func(xs, ys []float64, _ int) []stat.Point { return stat.Cumsum(xs, ys) },
	} {
		before := f(xs[:39], ys[:39], 9)
		after := f(xs, ys, 9)
		for i := range before {
			if before[i] != after[i] {
				t.Fatalf("%s: row %d moved from %v to %v when row 39 arrived", name, i, before[i], after[i])
			}
		}
	}

	before := stat.MovingAverage(xs[:39], ys[:39], 9)
	after := stat.MovingAverage(xs, ys, 9)
	if before[38] == after[38] {
		t.Error("a centred mean's last row did not move either, so the test proves nothing about the trailing forms")
	}
}

// The EMA's fraction is the market's 2/(n+1), started at the first row.
func TestTheEMAMovesTwoOverNPlusOneOfTheWay(t *testing.T) {
	xs := []float64{0, 1, 2, 3}
	ys := []float64{10, 20, 20, 20}
	got := stat.EMA(xs, ys, 3) // alpha = 0.5
	want := []float64{10, 15, 17.5, 18.75}
	for i, w := range want {
		if math.Abs(got[i].Y-w) > 1e-12 {
			t.Fatalf("row %d: got %v, want %v", i, got[i].Y, w)
		}
	}
}

// The case the carried extreme is slowest in, a column falling for its whole
// length under RollingMax, is still right.
func TestARollingMaxOverAFallingColumnRescansCorrectly(t *testing.T) {
	xs, ys := smoothRamp(30, func(x float64) float64 { return -x })
	for i, p := range stat.RollingMax(xs, ys, 4) {
		if want := ys[max(i-3, 0)]; p.Y != want {
			t.Fatalf("row %d: got %v, want %v", i, p.Y, want)
		}
	}
}

func TestCumsumEndsAtTheTotal(t *testing.T) {
	xs, ys := smoothRamp(10, func(x float64) float64 { return x + 1 })
	got := stat.Cumsum(xs, ys)
	if got[len(got)-1].Y != 55 {
		t.Errorf("last value %v, want the column's total 55", got[len(got)-1].Y)
	}
}

// The spread is taken in two passes per window, so a price column far from
// zero with a tiny spread still has one — the case the running sum of squares
// cancels on.
func TestTheRollingSpreadSurvivesALargeOffset(t *testing.T) {
	xs, ys := smoothRamp(50, func(x float64) float64 { return 1e9 + math.Mod(x, 2) })
	for i, p := range stat.RollingStdDev(xs, ys, 10) {
		if i < 9 {
			continue
		}
		if math.Abs(p.Y-0.5) > 1e-6 {
			t.Fatalf("row %d: spread %v, want 0.5", i, p.Y)
		}
	}
}

func TestEmptyColumnsGiveNoPoints(t *testing.T) {
	for name, got := range map[string][]stat.Point{
		"mean": stat.TrailingMean(nil, nil, 5),
		"ema":  stat.EMA(nil, nil, 5),
		"sd":   stat.RollingStdDev(nil, nil, 5),
		"min":  stat.RollingMin(nil, nil, 5),
		"sum":  stat.Cumsum(nil, nil),
	} {
		if len(got) != 0 {
			t.Errorf("%s: %v from no rows", name, got)
		}
	}
}

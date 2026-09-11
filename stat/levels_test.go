package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// Levels picks round numbers, and only ones a reader can read: an isoline at
// the minimum of the data is a point and one at the maximum is the boundary.
func TestLevelsAreRoundNumbersStrictlyInsideTheRange(t *testing.T) {
	for _, tc := range []struct {
		lo, hi float64
		n      int
		want   []float64
	}{
		{0, 10, 5, []float64{2, 4, 6, 8}},
		{0, 1, 4, []float64{0.2, 0.4, 0.6, 0.8}},
		{-3, 3, 6, []float64{-2, -1, 0, 1, 2}},
		{0, 100, 4, []float64{20, 40, 60, 80}},
		{1.2, 4.7, 4, []float64{2, 3, 4}},
	} {
		got := stat.Levels(tc.lo, tc.hi, tc.n)
		if len(got) != len(tc.want) {
			t.Errorf("Levels(%v, %v, %d) = %v, want %v", tc.lo, tc.hi, tc.n, got, tc.want)
			continue
		}
		for i := range got {
			if math.Abs(got[i]-tc.want[i]) > 1e-9 {
				t.Errorf("Levels(%v, %v, %d) = %v, want %v", tc.lo, tc.hi, tc.n, got, tc.want)
				break
			}
		}
		for _, v := range got {
			if v <= tc.lo || v >= tc.hi {
				t.Errorf("Levels(%v, %v, %d) returned %v, which is not inside the range",
					tc.lo, tc.hi, tc.n, v)
			}
		}
	}
}

// A range nothing can be said about is an empty list rather than an error: a
// chart over a column with one value in it draws no isolines and is not broken.
func TestLevelsOfAnEmptyRangeIsEmpty(t *testing.T) {
	for _, tc := range []struct {
		lo, hi float64
		n      int
	}{
		{5, 5, 4},
		{5, 1, 4},
		{0, 1, 0},
		{math.Inf(-1), 1, 4},
	} {
		if got := stat.Levels(tc.lo, tc.hi, tc.n); len(got) != 0 {
			t.Errorf("Levels(%v, %v, %d) = %v, want nothing", tc.lo, tc.hi, tc.n, got)
		}
	}
}

// The count is a hint and the step is the promise: a round step giving about n
// intervals is worth more than exactly n awkward ones.
//
// n counts intervals and the answer counts levels, and a range whose ends are
// not multiples of the step has one fewer level than it has intervals — so the
// interval count is between got+1 and got+2, and that is what is compared. The
// band is a factor of two either way, which is the furthest apart two
// neighbouring rungs of the 1-2-5 ladder can be.
func TestLevelsGivesAboutAsManyAsAsked(t *testing.T) {
	for _, hi := range []float64{1, 3, 7.5, 250} {
		for n := 2; n <= 20; n++ {
			got := len(stat.Levels(0, hi, n))
			if lo, hiCount := got+1, got+2; lo > 2*n || hiCount*2 < n {
				t.Errorf("Levels(0, %v, %d) gave %d levels, so %d or %d intervals, which is not about %d",
					hi, n, got, lo, hiCount, n)
			}
		}
	}
}

// The Append form writes into a caller's slice rather than one of its own. How
// many times it allocates is checked in alloc_test.go, away from the race
// detector.
func TestAppendLevelsReusesTheCallersSlice(t *testing.T) {
	dst := make([]float64, 0, 16)
	dst = stat.AppendLevels(dst, 0, 10, 5)
	first := &dst[0]

	dst = stat.AppendLevels(dst[:0], 0, 10, 5)
	if &dst[0] != first {
		t.Error("AppendLevels replaced the slice it was given")
	}
}

// Levels are multiples of the step rather than a running sum, so that the same
// range gives the same numbers however many of them there are.
func TestLevelsDoNotDrift(t *testing.T) {
	got := stat.Levels(0, 1000, 1000)
	if len(got) == 0 {
		t.Fatal("a thousand levels over a thousand units gave none")
	}
	for _, v := range got {
		if v != math.Round(v) {
			t.Fatalf("a level of a unit step is %v, which is not a whole number", v)
		}
	}
	if last := got[len(got)-1]; last != 999 {
		t.Errorf("the last level is %v, want 999", last)
	}
}

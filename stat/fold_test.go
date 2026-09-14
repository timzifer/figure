package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// The fold is the whole claim of a horizon chart: a value is cut into bands of
// equal height, and reading the bands back has to give the value again. A test
// that only checked the ends would pass on a fold that had lost a band in the
// middle.
func TestFoldingAndAddingUpAgainIsTheValue(t *testing.T) {
	const origin, height, bands = 0.0, 25.0, 4
	for _, v := range []float64{0, 1, 24.9, 25, 25.1, 60, 99.9, -37, -80} {
		arm := stat.FoldAbove
		if v < origin {
			arm = stat.FoldBelow
		}
		sum := 0.0
		for band := range bands {
			sum += stat.Fold(v, origin, height, band, arm)
		}
		want := math.Abs(v-origin) / height
		if math.Abs(sum-want) > 1e-9 {
			t.Errorf("folding %v adds up to %v bands, want %v", v, sum, want)
		}
	}
}

// A value in one arm draws nothing in the other, which is what keeps the two
// halves of the chart from being painted over each other.
func TestTheOtherArmIsEmpty(t *testing.T) {
	for _, v := range []float64{5, 40, 120} {
		if got := stat.Fold(v, 0, 25, 0, stat.FoldBelow); got != 0 {
			t.Errorf("a value of %v fills %v of the band below the origin, want none", v, got)
		}
		if got := stat.Fold(-v, 0, 25, 0, stat.FoldAbove); got != 0 {
			t.Errorf("a value of %v fills %v of the band above the origin, want none", -v, got)
		}
	}
}

// A band the value passes right through is full, and one it never reaches is
// empty. Between them is the only band with a reading in it, which is what
// makes the chart legible: at most one boundary is visible per column.
func TestABandIsFullBeforeTheNextOneStarts(t *testing.T) {
	const h = 10.0
	if got := stat.Fold(35, 0, h, 0, stat.FoldAbove); got != 1 {
		t.Errorf("the first band of 35 is %v, want a full band", got)
	}
	if got := stat.Fold(35, 0, h, 2, stat.FoldAbove); got != 1 {
		t.Errorf("the third band of 35 is %v, want a full band", got)
	}
	if got := stat.Fold(35, 0, h, 3, stat.FoldAbove); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("the fourth band of 35 is %v, want half a band", got)
	}
	if got := stat.Fold(35, 0, h, 4, stat.FoldAbove); got != 0 {
		t.Errorf("the fifth band of 35 is %v, want an empty band", got)
	}
}

// A row nobody measured has no position inside a band either. It must stay a
// NaN rather than becoming a zero, or a hole in the data would be drawn as a
// series that touched its origin.
func TestAMissingValueStaysMissing(t *testing.T) {
	if got := stat.Fold(math.NaN(), 0, 10, 0, stat.FoldAbove); !math.IsNaN(got) {
		t.Errorf("a missing value folded to %v, want NaN", got)
	}
	got := stat.AppendFold(nil, []float64{1, math.NaN(), 3}, 0, 10, 0, stat.FoldAbove)
	if len(got) != 3 || !math.IsNaN(got[1]) {
		t.Errorf("AppendFold lost the hole: %v", got)
	}
}

// The origin is where the fold starts, not zero. A plant engineer measuring a
// deviation from a set point is folding about the set point.
func TestTheOriginIsWhereTheFoldStarts(t *testing.T) {
	if got := stat.Fold(105, 100, 10, 0, stat.FoldAbove); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("105 about an origin of 100 is %v of a band, want half", got)
	}
	if got := stat.Fold(95, 100, 10, 0, stat.FoldBelow); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("95 about an origin of 100 is %v of a band below, want half", got)
	}
}

// The height and the count are the same statement said two ways round, so
// asking for one and then the other has to come back where it started.
func TestHeightAndCountAreInverses(t *testing.T) {
	const lo, hi, origin = -30.0, 90.0, 0.0
	for _, n := range []int{1, 2, 3, 4, 8} {
		h := stat.FoldHeight(lo, hi, origin, n)
		if got := stat.FoldBands(lo, hi, origin, h); got != n {
			t.Errorf("%d bands are %v high, which is %d bands", n, h, got)
		}
	}
}

// The further end from the origin sets the height, because the two arms are
// drawn at one height: a band that meant one thing above the origin and
// another below it would make the chart's own halves incomparable.
func TestTheHeightComesFromTheFurtherEnd(t *testing.T) {
	if got := stat.FoldHeight(-100, 10, 0, 4); got != 25 {
		t.Errorf("the band height is %v, want 25 — the further end is the low one", got)
	}
	if got := stat.FoldHeight(-10, 100, 0, 4); got != 25 {
		t.Errorf("the band height is %v, want 25", got)
	}
}

// A series that never leaves its origin is still a chart. Neither call may
// answer with something a caller would divide by.
func TestAFlatSeriesHasNoReach(t *testing.T) {
	if got := stat.FoldHeight(5, 5, 5, 3); got != 0 {
		t.Errorf("a flat series has a band height of %v, want none", got)
	}
	if got := stat.FoldBands(5, 5, 5, 10); got != 1 {
		t.Errorf("a flat series needs %d bands, want one", got)
	}
	if got := stat.FoldBands(0, 100, 0, 0); got != 1 {
		t.Errorf("a band of no height needs %d bands, want one", got)
	}
}

// The Append form is what a chart redrawn every frame calls, so it must not
// allocate once it has a buffer — the property every reduction here is gated
// on.
func TestAppendFoldReusesItsBuffer(t *testing.T) {
	vs := make([]float64, 4096)
	for i := range vs {
		vs[i] = float64(i%97) - 40
	}
	buf := stat.AppendFold(nil, vs, 0, 10, 1, stat.FoldAbove)
	if n := testing.AllocsPerRun(20, func() {
		buf = stat.AppendFold(buf, vs, 0, 10, 1, stat.FoldAbove)
	}); n != 0 {
		t.Errorf("AppendFold allocated %v times per run, want none", n)
	}
}

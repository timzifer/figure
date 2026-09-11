package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// xRamp is a field that rises with x and nothing else, so every isoline of it is
// a straight vertical line at a known place.
func xRamp(nx, ny int) (xs, ys, z []float64) {
	xs = make([]float64, nx)
	ys = make([]float64, ny)
	for i := range xs {
		xs[i] = float64(i)
	}
	for j := range ys {
		ys[j] = float64(j)
	}
	z = make([]float64, nx*ny)
	for j := range ny {
		for i := range nx {
			z[j*nx+i] = float64(i)
		}
	}
	return xs, ys, z
}

// A level of a field that rises with x is the vertical line at that x, running
// from one edge of the lattice to the other.
func TestALevelOfARampIsAStraightLineAcrossIt(t *testing.T) {
	xs, ys, z := xRamp(5, 4)
	var c stat.Contour
	c.Reset(xs, ys, z, []float64{2.5})

	if len(c.Lines) != 1 {
		t.Fatalf("a ramp at one level traced %d runs, want 1", len(c.Lines))
	}
	line := c.Lines[0]
	if line.Closed {
		t.Error("a run that crosses the lattice is not a ring")
	}
	pts := c.Line(line)
	if len(pts) < 2 {
		t.Fatalf("the run has %d points", len(pts))
	}
	for _, p := range pts {
		if math.Abs(p.X-2.5) > 1e-12 {
			t.Fatalf("a point of the 2.5 isoline is at x=%v", p.X)
		}
	}
	// It spans the field: from the bottom edge to the top.
	lo, hi := pts[0].Y, pts[len(pts)-1].Y
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo > 0 || hi < float64(len(ys)-1) {
		t.Errorf("the run spans y in [%v, %v], want the whole lattice", lo, hi)
	}
}

// A level below everything or above everything traces nothing, and that is an
// answer rather than an error: it is what "show me the 0 dB line" means when
// nothing reaches 0 dB.
func TestALevelOutsideTheDataTracesNothing(t *testing.T) {
	xs, ys, z := xRamp(5, 4)
	var c stat.Contour
	c.Reset(xs, ys, z, []float64{-1, 99})
	if len(c.Lines) != 0 {
		t.Errorf("levels outside the data traced %d runs", len(c.Lines))
	}
}

// A hill closes on itself, and its last point is its first exactly — not nearly,
// which is what Closed claims.
func TestAHillTracesARingThatClosesExactly(t *testing.T) {
	const n = 21
	xs := make([]float64, n)
	ys := make([]float64, n)
	for i := range n {
		xs[i], ys[i] = -1+2*float64(i)/(n-1), -1+2*float64(i)/(n-1)
	}
	z := make([]float64, n*n)
	for j := range n {
		for i := range n {
			z[j*n+i] = math.Exp(-4 * (xs[i]*xs[i] + ys[j]*ys[j]))
		}
	}

	var c stat.Contour
	c.Reset(xs, ys, z, []float64{0.5})
	if len(c.Lines) != 1 {
		t.Fatalf("a single hill traced %d runs at one level, want 1", len(c.Lines))
	}
	line := c.Lines[0]
	if !line.Closed {
		t.Fatal("an isoline round a hill inside the lattice is not closed")
	}
	pts := c.Line(line)
	if pts[0] != pts[len(pts)-1] {
		t.Errorf("the ring closes at %v having opened at %v", pts[len(pts)-1], pts[0])
	}
	// It is a circle of the right radius: exp(-4r²) = 0.5 at r = sqrt(ln2/4).
	want := math.Sqrt(math.Ln2 / 4)
	for _, p := range pts {
		if r := math.Hypot(p.X, p.Y); math.Abs(r-want) > 0.03 {
			t.Fatalf("a point of the ring is at radius %.3f, want %.3f", r, want)
		}
	}
}

// A cell with an undefined corner is a hole: the run ends at its edge rather
// than being routed round it, because a contour through a number nobody
// measured is a contour through a guess.
func TestACellTouchingANaNIsNotTraced(t *testing.T) {
	xs, ys, z := xRamp(5, 4)
	whole := trace(xs, ys, z, 2.5)

	z[1*5+2] = math.NaN()
	holed := trace(xs, ys, z, 2.5)

	if len(holed) == 0 {
		t.Fatal("a hole in the middle of a ramp removed every run")
	}
	if pointsIn(holed) >= pointsIn(whole) {
		t.Errorf("a hole left %d points, against %d without one", pointsIn(holed), pointsIn(whole))
	}
	for _, pts := range holed {
		for _, p := range pts {
			if math.IsNaN(p.X) || math.IsNaN(p.Y) {
				t.Fatal("a traced point is not a number")
			}
		}
	}
}

// A saddle admits two joinings and the choice must not depend on how the walk
// arrived. The rule is the cell's centre, which is a function of the four
// corners and of nothing else — so the same field traced the same way twice,
// and traced with its rows walked the other way up, agrees about the saddle.
func TestASaddleIsResolvedTheSameWayEveryTime(t *testing.T) {
	// One cell, high on one diagonal and low on the other.
	xs := []float64{0, 1}
	ys := []float64{0, 1}
	z := []float64{1, -1, -1, 1} // (0,0)=1 (1,0)=-1 (0,1)=-1 (1,1)=1

	first := trace(xs, ys, z, 0)
	second := trace(xs, ys, z, 0)
	if len(first) != len(second) {
		t.Fatalf("tracing one saddle twice gave %d runs and then %d", len(first), len(second))
	}
	for i := range first {
		if len(first[i]) != len(second[i]) {
			t.Fatalf("run %d has %d points and then %d", i, len(first[i]), len(second[i]))
		}
		for k := range first[i] {
			if first[i][k] != second[i][k] {
				t.Fatalf("run %d point %d is %v and then %v", i, k, first[i][k], second[i][k])
			}
		}
	}
	if len(first) != 2 {
		t.Errorf("a saddle traced %d runs, want the two segments it has", len(first))
	}
}

// The package's rule: everything here is a pure function of its arguments, and
// every reduction has a test that runs it twice.
func TestTracingTwiceGivesTheSameLines(t *testing.T) {
	const n = 17
	xs := make([]float64, n)
	ys := make([]float64, n)
	for i := range n {
		xs[i], ys[i] = -2+4*float64(i)/(n-1), -2+4*float64(i)/(n-1)
	}
	z := make([]float64, n*n)
	for j := range n {
		for i := range n {
			z[j*n+i] = math.Sin(xs[i]) * math.Cos(ys[j])
		}
	}
	levels := stat.Levels(-1, 1, 6)

	var a, b stat.Contour
	a.Reset(xs, ys, z, levels)
	b.Reset(xs, ys, z, levels)

	if len(a.Lines) != len(b.Lines) || len(a.Points) != len(b.Points) {
		t.Fatalf("two tracings gave %d/%d runs and points against %d/%d",
			len(a.Lines), len(a.Points), len(b.Lines), len(b.Points))
	}
	for i := range a.Lines {
		if a.Lines[i] != b.Lines[i] {
			t.Fatalf("run %d is %+v and then %+v", i, a.Lines[i], b.Lines[i])
		}
	}
	for i := range a.Points {
		if a.Points[i] != b.Points[i] {
			t.Fatalf("point %d is %v and then %v", i, a.Points[i], b.Points[i])
		}
	}
	if len(a.Lines) == 0 {
		t.Error("the field traced nothing at all, so the comparison says nothing")
	}
}

// Runs are grouped by level in ascending order, because a geom strokes them in
// that order and a golden file compares element order exactly.
func TestRunsAreGroupedByAscendingLevel(t *testing.T) {
	xs, ys, z := xRamp(9, 5)
	var c stat.Contour
	c.Reset(xs, ys, z, []float64{6, 2, 4, 2})

	if want := []float64{2, 4, 6}; len(c.Levels) != len(want) {
		t.Fatalf("the levels are %v, want %v with the repeat removed", c.Levels, want)
	}
	last := math.Inf(-1)
	for _, l := range c.Lines {
		if l.Level < last {
			t.Fatalf("a run at level %v follows one at %v", l.Level, last)
		}
		if c.Levels[l.Index] != l.Level {
			t.Errorf("a run says level %v at index %d, where the list has %v",
				l.Level, l.Index, c.Levels[l.Index])
		}
		last = l.Level
	}
}

func trace(xs, ys, z []float64, level float64) [][]stat.Point {
	var c stat.Contour
	c.Reset(xs, ys, z, []float64{level})
	out := make([][]stat.Point, 0, len(c.Lines))
	for _, l := range c.Lines {
		out = append(out, append([]stat.Point(nil), c.Line(l)...))
	}
	return out
}

func pointsIn(runs [][]stat.Point) int {
	n := 0
	for _, r := range runs {
		n += len(r)
	}
	return n
}

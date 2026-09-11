package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// A long table of (x, y, v) rows becomes a grid, sorted on both axes whatever
// order the rows arrived in.
func TestALatticeSortsBothAxesAndPlacesEveryRow(t *testing.T) {
	// Deliberately out of order and with y varying fastest, which is the other
	// way round from the layout the lattice produces.
	xs := []float64{2, 2, 1, 1, 0, 0}
	ys := []float64{1, 0, 1, 0, 1, 0}
	vs := []float64{21, 20, 11, 10, 1, 0}

	var l stat.Lattice
	if f := l.Reset(xs, ys, vs); f != stat.LatticeOK {
		t.Fatalf("a product table faulted with %v", f)
	}
	if got := l.Xs; len(got) != 3 || got[0] != 0 || got[2] != 2 {
		t.Fatalf("the x axis is %v, want the sorted distinct values", got)
	}
	if got := l.Ys; len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("the y axis is %v", got)
	}
	// Row-major in y: V[j*nx+i] is the value at (Xs[i], Ys[j]).
	for i, wantRow := range [][2]float64{{0, 1}, {10, 11}, {20, 21}} {
		for j, want := range wantRow {
			if got := l.Value(i, j); got != want {
				t.Errorf("the cell at (%v, %v) is %v, want %v", l.Xs[i], l.Ys[j], got, want)
			}
		}
	}
	if l.Row[l.Index(0, 0)] != 5 {
		t.Errorf("the cell at the origin came from row %d, want 5", l.Row[l.Index(0, 0)])
	}
}

// A hole in the data is not a hole in the grid. NaN is a measurement that
// failed, and a mark decides for itself what to do with one.
func TestALatticeCarriesUndefinedValues(t *testing.T) {
	xs := []float64{0, 1, 0, 1}
	ys := []float64{0, 0, 1, 1}
	vs := []float64{1, math.NaN(), 3, 4}

	var l stat.Lattice
	if f := l.Reset(xs, ys, vs); f != stat.LatticeOK {
		t.Fatalf("a NaN value faulted with %v", f)
	}
	if !math.IsNaN(l.Value(1, 0)) {
		t.Errorf("the undefined cell is %v", l.Value(1, 0))
	}
}

// Each fault is reported for what it is, and the two that are about particular
// rows name them — so a caller can point its message at the data rather than at
// the table as a whole.
func TestALatticeReportsWhyATableIsNotOne(t *testing.T) {
	for _, tc := range []struct {
		name       string
		xs, ys, vs []float64
		want       stat.LatticeFault
	}{
		{"ragged", []float64{0, 1}, []float64{0}, []float64{1, 2}, stat.LatticeRagged},
		{"one column", []float64{0, 0}, []float64{0, 1}, []float64{1, 2}, stat.LatticeTooSmall},
		{
			"a cell short",
			[]float64{0, 1, 0, 1, 2}, []float64{0, 0, 1, 1, 0}, []float64{1, 2, 3, 4, 5},
			stat.LatticeWrongCount,
		},
		{
			"two rows at one node",
			[]float64{0, 1, 0, 0}, []float64{0, 0, 1, 1}, []float64{1, 2, 3, 4},
			stat.LatticeDuplicate,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var l stat.Lattice
			if got := l.Reset(tc.xs, tc.ys, tc.vs); got != tc.want {
				t.Errorf("Reset reported %v, want %v", got, tc.want)
			}
		})
	}

	// A duplicate names both rows, which is the message a caller writes.
	var l stat.Lattice
	l.Reset([]float64{0, 1, 0, 0}, []float64{0, 0, 1, 1}, []float64{1, 2, 3, 4})
	if l.At != 2 || l.With != 3 {
		t.Errorf("the duplicate is rows %d and %d, want 2 and 3", l.At, l.With)
	}

	// A position that is not a number names itself. It is the only way a row
	// can fail to be at a node: the axes are the distinct values of the columns
	// themselves, so everything finite is on the grid by construction.
	l.Reset([]float64{0, 1, math.NaN(), 1}, []float64{0, 0, 1, 1}, []float64{1, 2, 3, 4})
	if l.Fault != stat.LatticeBadPosition {
		t.Fatalf("a row at an undefined position faulted with %v", l.Fault)
	}
	if l.At != 2 {
		t.Errorf("the bad row is %d, want 2", l.At)
	}
}

// The type is a struct with a Reset so that a chart redrawn every frame
// resolves into the same memory.
func TestResolvingALatticeAgainDoesNotAllocate(t *testing.T) {
	const n = 24
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	vs := make([]float64, 0, n*n)
	for j := range n {
		for i := range n {
			xs, ys = append(xs, float64(i)), append(ys, float64(j))
			vs = append(vs, float64(i*j))
		}
	}

	var l stat.Lattice
	if f := l.Reset(xs, ys, vs); f != stat.LatticeOK {
		t.Fatal(f)
	}
	if got := testing.AllocsPerRun(10, func() { l.Reset(xs, ys, vs) }); got != 0 {
		t.Errorf("resolving again allocated %.0f times, want none", got)
	}
}

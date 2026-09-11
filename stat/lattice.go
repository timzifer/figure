package stat

import (
	"math"
	"slices"
)

// LatticeFault says why a table is not a product lattice.
//
// It is a code rather than an error because a useful message names a column and
// a mark, and this package knows about numbers and nothing else. The caller
// spells the sentence; this says which one. [Sankey.Cyclic] is the same trade.
type LatticeFault uint8

// The faults, in the order [Lattice.Reset] can find them.
const (
	// LatticeOK is a table that is a lattice.
	LatticeOK LatticeFault = iota
	// LatticeRagged is three columns of different lengths.
	LatticeRagged
	// LatticeTooSmall is fewer than two distinct values on one of the axes.
	// One value is a line rather than a grid, and no surface, contour or image
	// can be drawn over it.
	LatticeTooSmall
	// LatticeWrongCount is a table with a number of rows that is not the
	// product of the two axes' distinct values, which means cells are missing
	// or repeated whatever the positions say.
	LatticeWrongCount
	// LatticeBadPosition is a row whose x or y is not a finite number.
	// [Lattice.At] is the row.
	//
	// It is checked rather than discovered, because the axes are the distinct
	// values of the columns themselves — so every row is at a node of the
	// lattice by construction, and the only position that is not one is a
	// position that is not a number.
	LatticeBadPosition
	// LatticeDuplicate is two rows at one node. [Lattice.At] and [Lattice.With]
	// are the two.
	LatticeDuplicate
)

// Lattice turns a long table of (x, y, v) rows into a regular grid.
//
// It is the shape a surface, a contour and a raster of a measured field all
// need, and it is one implementation rather than one each because two
// resolvers that agree today disagree at the first duplicated position — and
// the symptom of that is a surface and its own contours that do not line up.
//
// The grid is required rather than guessed. The rows must be the full product
// of the distinct x and y values with each cell present exactly once, and
// anything else is a [LatticeFault] rather than a picture with holes in it: a
// field drawn over a scattered sample is a field over a triangulation nobody
// asked for. A cell whose value is NaN is a different matter and is allowed —
// that is a hole in the data, and a mark decides for itself what to do with
// one.
//
// The zero Lattice is unusable; call [Lattice.Reset] first. Reset keeps the
// buffers and clears the index maps rather than replacing them, so their
// buckets survive too — which is what keeps a chart redrawn every frame from
// allocating per frame.
type Lattice struct {
	// Xs and Ys are the sorted distinct values of the two axes.
	Xs, Ys []float64
	// V holds len(Ys) rows of len(Xs) values, row-major in y: the value at
	// (Xs[i], Ys[j]) is V[j*len(Xs)+i], which is [Lattice.Index].
	V []float64
	// Row is the source row behind each cell, in the same order, or -1 for a
	// cell no row reached — which happens only when Fault is not [LatticeOK].
	Row []int32

	// Fault says whether the table was a lattice, and At and With name the rows
	// a [LatticeOffGrid] or [LatticeDuplicate] is about so that a caller can
	// point its message at the data.
	Fault    LatticeFault
	At, With int

	xi, yi map[float64]int
	seen   map[float64]struct{}
}

// Reset fills the lattice from three parallel columns and reports whether they
// were one.
//
// On a fault the fields are left in whatever state the search reached and must
// not be read: a caller checks the return, or [Lattice.Fault], first.
func (l *Lattice) Reset(xs, ys, vs []float64) LatticeFault {
	l.At, l.With = -1, -1
	if len(xs) != len(ys) || len(ys) != len(vs) {
		return l.fault(LatticeRagged)
	}
	for r := range xs {
		if !finite(xs[r]) || !finite(ys[r]) {
			l.At = r
			return l.fault(LatticeBadPosition)
		}
	}

	l.Xs = l.distinctInto(l.Xs[:0], xs)
	l.Ys = l.distinctInto(l.Ys[:0], ys)
	nx, ny := len(l.Xs), len(l.Ys)
	if nx < 2 || ny < 2 {
		return l.fault(LatticeTooSmall)
	}
	if nx*ny != len(vs) {
		return l.fault(LatticeWrongCount)
	}

	l.V = grow(l.V, nx*ny)
	l.Row = grow(l.Row, nx*ny)
	for i := range l.Row {
		l.Row[i] = -1
	}
	l.xi, l.yi = indexInto(l.xi, l.Xs), indexInto(l.yi, l.Ys)

	for r := range vs {
		// Both lookups hit: the axes are the distinct values of these very
		// columns, and a position that is not a number was refused above.
		k := l.yi[ys[r]]*nx + l.xi[xs[r]]
		if l.Row[k] >= 0 {
			l.At, l.With = int(l.Row[k]), r
			return l.fault(LatticeDuplicate)
		}
		l.V[k], l.Row[k] = vs[r], int32(r)
	}
	l.Fault = LatticeOK
	return LatticeOK
}

func (l *Lattice) fault(f LatticeFault) LatticeFault { l.Fault = f; return f }

// Index is where the cell at (Xs[i], Ys[j]) is in [Lattice.V] and
// [Lattice.Row].
func (l *Lattice) Index(i, j int) int { return j*len(l.Xs) + i }

// Value is the value at (Xs[i], Ys[j]), or NaN for a position off the grid.
func (l *Lattice) Value(i, j int) float64 {
	if i < 0 || j < 0 || i >= len(l.Xs) || j >= len(l.Ys) {
		return math.NaN()
	}
	return l.V[l.Index(i, j)]
}

// distinctInto collects the distinct values of vs into dst, sorted.
//
// The map is cleared rather than replaced so that its buckets survive a redraw,
// which is the same reason the whole type has a Reset.
func (l *Lattice) distinctInto(dst, vs []float64) []float64 {
	if l.seen == nil {
		l.seen = make(map[float64]struct{}, len(vs))
	} else {
		clear(l.seen)
	}
	for _, v := range vs {
		if _, ok := l.seen[v]; ok {
			continue
		}
		l.seen[v] = struct{}{}
		dst = append(dst, v)
	}
	slices.Sort(dst)
	return dst
}

// indexInto rebuilds a value-to-position map over vs, reusing m's buckets.
func indexInto(m map[float64]int, vs []float64) map[float64]int {
	if m == nil {
		m = make(map[float64]int, len(vs))
	} else {
		clear(m)
	}
	for i, v := range vs {
		m[v] = i
	}
	return m
}

// grow returns a slice of length n, reusing buf's array when it is large
// enough.
func grow[T any](buf []T, n int) []T {
	if cap(buf) >= n {
		return buf[:n]
	}
	return make([]T, n)
}

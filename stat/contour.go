package stat

import (
	"math"
	"slices"
)

// Isoline is one connected run of one contour level.
//
// It is a range into the [Contour]'s point arena rather than a slice of its
// own, so a chart redrawn every frame refills one buffer instead of allocating
// a slice per line. [Sankey.Flows] has the same shape for the same reason.
type Isoline struct {
	// Level is the value the run traces, and Index its position in
	// [Contour.Levels] — so a caller colouring by level does not have to search
	// for it.
	Level float64
	Index int
	// Lo and Hi bound the run in [Contour.Points]. Reach for [Contour.Line].
	Lo, Hi int
	// Closed reports a ring: the run's last point is exactly its first. An open
	// run begins and ends on the edge of the lattice or at a hole, so this is a
	// statement about the points rather than about where the run stopped.
	Closed bool
}

// Contour traces the isolines of a value sampled on a regular lattice, by
// marching squares.
//
// It is one tracing serving two charts. A flat contour plot strokes what it
// returns through the panel's scales; a projected scene strokes the same runs
// on the floor of its cube. Handed one level list they are the same lines, and
// that is the point: a reading taken off the plan and a reading taken off the
// floor of the surface have to agree, and two implementations of marching
// squares would agree until the first saddle.
//
// It is a struct with a [Contour.Reset] rather than a pair of functions because
// its working state is the size of the data, which is [Hex]'s reason and
// [Sankey]'s. It is not generic over float32, and that is a decision rather
// than an omission: a reduction is generic so that a geom can run it on
// projected device coordinates, and a contour runs in Train, in data space,
// where there is no float32 caller.
//
// The zero Contour is unusable; call [Contour.Reset] first. Reset keeps the
// arena and the working tables.
type Contour struct {
	// Lines has one run per connected piece of an isoline: grouped by level in
	// ascending order, and within a level in the order the runs were found,
	// which is lattice order. Nothing here depends on a map's iteration order —
	// see the package documentation on determinism.
	Lines []Isoline
	// Points is the vertex arena every run indexes, in the coordinates the
	// lattice was given. A geom maps them through its scales exactly as it maps
	// a row.
	Points []Point
	// Levels is the list the runs were traced at, ascending and with any
	// repeats removed.
	Levels []float64

	xs, ys []float64
	z      []float64
	nx, ny int

	// seg holds each cell's marching-squares case for the level being traced,
	// and used which of its four edges a run has already left through. Both are
	// refilled per level rather than made per level.
	seg  []uint8
	used []uint8
	work []float64
}

// Edges of a cell, in the order marching squares numbers them: the bit set in
// [Contour.used] when a run has already crossed one.
const (
	edgeBottom uint8 = 1 << iota
	edgeRight
	edgeTop
	edgeLeft
)

// Reset traces z over the lattice xs × ys at the given levels.
//
// z holds len(ys) rows of len(xs) values, row-major in y — which is
// [Lattice.V]'s layout, and [Lattice] is how a long table becomes one.
//
// A cell with any non-finite corner is skipped whole. A contour drawn through a
// hole would be a contour through a number nobody measured, so a run that
// reaches one ends there rather than being routed round it. A level outside the
// data's range traces nothing, which is not an error: it is what "show me the
// 0 dB line" means when nothing reaches 0 dB.
func (c *Contour) Reset(xs, ys, z []float64, levels []float64) {
	c.Lines = c.Lines[:0]
	c.Points = c.Points[:0]
	c.Levels = c.Levels[:0]
	c.xs, c.ys, c.z = xs, ys, z
	c.nx, c.ny = len(xs), len(ys)
	if c.nx < 2 || c.ny < 2 || len(z) < c.nx*c.ny {
		return
	}

	c.Levels = appendSortedUnique(c.Levels, levels, c.work[:0])
	cells := (c.nx - 1) * (c.ny - 1)
	c.seg = grow(c.seg, cells)
	c.used = grow(c.used, cells)

	for i, level := range c.Levels {
		c.trace(level, i)
	}
}

// Line returns the points of one run, as a slice of the arena. It is lent
// rather than given: the next [Contour.Reset] refills it.
func (c *Contour) Line(l Isoline) []Point { return c.Points[l.Lo:l.Hi] }

// trace walks one level, in two passes and both of them in lattice order.
//
// The boundary first, so that every run which leaves the lattice is found from
// the edge it leaves through and is therefore open; then the interior, where
// everything still unvisited closes on itself. Both walks are ordered by the
// lattice rather than by anything discovered along the way, which is what makes
// the output a pure function of the input.
func (c *Contour) trace(level float64, index int) {
	for i := range c.used {
		c.used[i] = 0
	}
	for j := 0; j < c.ny-1; j++ {
		for i := 0; i < c.nx-1; i++ {
			c.seg[j*(c.nx-1)+i] = c.caseAt(i, j, level)
		}
	}

	// Pass one: runs that start on the edge of the lattice.
	for j := 0; j < c.ny-1; j++ {
		for i := 0; i < c.nx-1; i++ {
			if i != 0 && j != 0 && i != c.nx-2 && j != c.ny-2 {
				continue
			}
			for _, e := range [...]uint8{edgeBottom, edgeLeft, edgeTop, edgeRight} {
				if c.onBoundary(i, j, e) {
					c.follow(i, j, e, level, index)
				}
			}
		}
	}
	// Pass two: everything left closes on itself.
	for j := 0; j < c.ny-1; j++ {
		for i := 0; i < c.nx-1; i++ {
			for _, e := range [...]uint8{edgeBottom, edgeLeft, edgeTop, edgeRight} {
				c.follow(i, j, e, level, index)
			}
		}
	}
}

// onBoundary reports whether an edge of a cell is on the outside of the
// lattice, which is where an open run begins.
func (c *Contour) onBoundary(i, j int, e uint8) bool {
	switch e {
	case edgeBottom:
		return j == 0
	case edgeTop:
		return j == c.ny-2
	case edgeLeft:
		return i == 0
	default:
		return i == c.nx-2
	}
}

// caseAt is the marching-squares case of one cell: a bit per corner that is at
// or above the level, counting anticlockwise from the bottom left.
//
// A cell with a non-finite corner is 255, which every walk treats as a wall.
func (c *Contour) caseAt(i, j int, level float64) uint8 {
	v := [4]float64{c.at(i, j), c.at(i+1, j), c.at(i+1, j+1), c.at(i, j+1)}
	var out uint8
	for k, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 255
		}
		if x >= level {
			out |= 1 << uint(k)
		}
	}
	return out
}

func (c *Contour) at(i, j int) float64 { return c.z[j*c.nx+i] }

// crosses reports whether the isoline cuts one edge of a cell, given its case.
// An edge is cut when its two corners are on opposite sides of the level.
func crosses(cs, e uint8) bool {
	if cs == 255 || cs == 0 || cs == 15 {
		return false
	}
	var a, b uint8
	switch e {
	case edgeBottom:
		a, b = 0, 1
	case edgeRight:
		a, b = 1, 2
	case edgeTop:
		a, b = 3, 2
	default:
		a, b = 0, 3
	}
	return (cs>>a)&1 != (cs>>b)&1
}

// follow walks a run from one edge of one cell until it leaves the lattice,
// reaches a hole, or returns to where it started, appending it to Lines.
//
// It does nothing when that edge carries no crossing or has already been
// walked, which is what lets both passes simply try every edge of every cell.
func (c *Contour) follow(i, j int, entry uint8, level float64, index int) {
	cs := c.seg[j*(c.nx-1)+i]
	if !crosses(cs, entry) || c.used[j*(c.nx-1)+i]&entry != 0 {
		return
	}
	lo := len(c.Points)
	c.Points = append(c.Points, c.edgePoint(i, j, entry, level))

	ci, cj, in := i, j, entry
	for {
		k := cj*(c.nx-1) + ci
		cs := c.seg[k]
		if cs == 255 || c.used[k]&in != 0 {
			break
		}
		out := exitOf(cs, in, c.centreAbove(ci, cj, level))
		if out == 0 {
			break
		}
		c.used[k] |= in | out
		c.Points = append(c.Points, c.edgePoint(ci, cj, out, level))

		ni, nj, next := neighbour(ci, cj, out)
		if ni < 0 || nj < 0 || ni >= c.nx-1 || nj >= c.ny-1 {
			break
		}
		if ni == i && nj == j && next == entry {
			// Back where it began. The closing vertex is the opening one by
			// assignment rather than by arithmetic, so that Closed is exact
			// whatever the compiler did with the interpolation.
			c.Points = append(c.Points, c.Points[lo])
			c.Lines = append(c.Lines, Isoline{
				Level: level, Index: index, Lo: lo, Hi: len(c.Points), Closed: true,
			})
			return
		}
		ci, cj, in = ni, nj, next
	}
	if len(c.Points)-lo < 2 {
		c.Points = c.Points[:lo]
		return
	}
	c.Lines = append(c.Lines, Isoline{Level: level, Index: index, Lo: lo, Hi: len(c.Points)})
}

// exitOf is which edge a run leaves a cell by, having entered by another.
//
// Twelve of the sixteen cases have exactly two crossed edges and no choice to
// make. The two saddles — the diagonal cases 5 and 10 — have four, and the rule
// that decides them is the cell's centre: the mean of its four corners, which
// is the bilinear value there rather than an approximation of it. If the centre
// is on the same side of the level as the two corners that are, those two are
// joined; otherwise the other pair is. A centre exactly at the level takes the
// first branch, so the rule is total.
//
// It depends on nothing but the four corner values, so it cannot depend on
// which way a walk arrived — which is what makes a flat contour and a
// projected one draw the same lines through a saddle.
func exitOf(cs, in uint8, centreAbove bool) uint8 {
	if cs == 5 || cs == 10 {
		// A saddle: two segments, and which pairs with which is the decision.
		joinHigh := centreAbove == (cs == 5)
		switch {
		case cs == 5 && joinHigh, cs == 10 && !joinHigh:
			// bottom joins left, top joins right.
			switch in {
			case edgeBottom:
				return edgeLeft
			case edgeLeft:
				return edgeBottom
			case edgeTop:
				return edgeRight
			case edgeRight:
				return edgeTop
			}
		default:
			// bottom joins right, top joins left.
			switch in {
			case edgeBottom:
				return edgeRight
			case edgeRight:
				return edgeBottom
			case edgeTop:
				return edgeLeft
			case edgeLeft:
				return edgeTop
			}
		}
		return 0
	}
	for _, e := range [...]uint8{edgeBottom, edgeRight, edgeTop, edgeLeft} {
		if e != in && crosses(cs, e) {
			return e
		}
	}
	return 0
}

// centreAbove reports whether the mean of a cell's four corners is at or above
// the level. See [exitOf].
func (c *Contour) centreAbove(i, j int, level float64) bool {
	m := (c.at(i, j) + c.at(i+1, j) + c.at(i+1, j+1) + c.at(i, j+1)) / 4
	return m >= level
}

// neighbour is the cell on the other side of an edge, and which of its own
// edges that is.
func neighbour(i, j int, e uint8) (int, int, uint8) {
	switch e {
	case edgeBottom:
		return i, j - 1, edgeTop
	case edgeTop:
		return i, j + 1, edgeBottom
	case edgeLeft:
		return i - 1, j, edgeRight
	default:
		return i + 1, j, edgeLeft
	}
}

// edgePoint is where the isoline cuts one edge of a cell, by linear
// interpolation between its two corners.
func (c *Contour) edgePoint(i, j int, e uint8, level float64) Point {
	var ax, ay, bx, by, va, vb = 0.0, 0.0, 0.0, 0.0, 0.0, 0.0
	switch e {
	case edgeBottom:
		ax, ay, bx, by = c.xs[i], c.ys[j], c.xs[i+1], c.ys[j]
		va, vb = c.at(i, j), c.at(i+1, j)
	case edgeTop:
		ax, ay, bx, by = c.xs[i], c.ys[j+1], c.xs[i+1], c.ys[j+1]
		va, vb = c.at(i, j+1), c.at(i+1, j+1)
	case edgeLeft:
		ax, ay, bx, by = c.xs[i], c.ys[j], c.xs[i], c.ys[j+1]
		va, vb = c.at(i, j), c.at(i, j+1)
	default:
		ax, ay, bx, by = c.xs[i+1], c.ys[j], c.xs[i+1], c.ys[j+1]
		va, vb = c.at(i+1, j), c.at(i+1, j+1)
	}
	return Point{X: lerpAt(ax, bx, va, vb, level), Y: lerpAt(ay, by, va, vb, level)}
}

// lerpAt is where a runs to b as its value runs from va to vb through level.
//
// The ends are handed back rather than computed, which is [LTTB]'s rule about
// arithmetic the compiler may contract: a fused multiply-add gives a different
// last bit on arm64 than on amd64, and a vertex that landed a bit past the
// corner would make the two architectures draw different pictures. It also
// makes the closing vertex of a ring exact.
func lerpAt(a, b, va, vb, level float64) float64 {
	if va == vb {
		return a
	}
	t := (level - va) / (vb - va)
	switch {
	case t <= 0:
		return a
	case t >= 1:
		return b
	}
	return a + t*(b-a)
}

// appendSortedUnique appends the finite values of vs to dst in ascending order
// with repeats removed, using scratch as its working buffer.
func appendSortedUnique(dst, vs, scratch []float64) []float64 {
	for _, v := range vs {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			scratch = append(scratch, v)
		}
	}
	slices.Sort(scratch)
	for i, v := range scratch {
		if i > 0 && v == scratch[i-1] {
			continue
		}
		dst = append(dst, v)
	}
	return dst
}

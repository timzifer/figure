package stat

// MaxVoronoiSites is the largest set of sites a [Voronoi] partitions.
//
// Two limits arrive at about the same number, which is why one constant serves
// for both — the same coincidence [MaxStressNodes] rests on. The picture gives
// out first: a cell has to be big enough to be told apart from its neighbours
// and pointed at, and a thousand cells in a 900×600 panel are about twenty
// pixels a side already. Past that the drawing is a field rather than a set of
// regions, and the answer to a field is a raster of the value
// (docs/adr/0066-a-raster-mark.md) rather than more cells. The arithmetic gives
// out just after: every cell is clipped against every other site, so the work
// grows faster than the data — measured on the machine this was written on, 250
// sites is about 1 ms, a thousand about 8, two thousand about 25 and four
// thousand about 83, which is a layout rather than a frame.
const MaxVoronoiSites = 1000

// Region is one site's cell: the run of [Voronoi.Verts] that is its boundary,
// as an offset and a length.
//
// A cell is a convex polygon of a few vertices and there is one per site, so
// the vertices of a whole diagram live in one slice and a region says where in
// it to look. A slice per cell would be an allocation per site per frame,
// which is what this package's buffers exist to avoid.
//
// A region of no vertices is a site with no cell: one that fell outside the
// rectangle, or the second of two sites at the same point.
type Region struct{ At, N int }

// Voronoi partitions a rectangle into the parts nearest each of a set of
// sites: the cell of site i is every point of the rectangle that is closer to
// site i than to any other site.
//
// It is the layout behind [github.com/timzifer/figure/geom.Voronoi] — a
// Thiessen polygon map, a nearest-station or nearest-depot diagram — and it is
// the drawing of *one* question: which site is nearest here. See
// docs/adr/0080-nearest-neighbour-cells.md.
//
// # How it is constructed, and why not with a sweep line
//
// A cell is an intersection of half-planes. For each other site, the points
// closer to this one are the side of the perpendicular bisector that this site
// is on, so the cell is the rectangle clipped by one bisector per other site —
// [Sutherland and Hodgman]'s clip, run once per pair. That is quadratic, which
// Fortune's sweep is not, and it is what this does anyway for three reasons:
// it is exact where a sweep needs circle-event predicates that decide the
// picture on a rounding; it has no degenerate cases to get wrong, because
// clipping a convex polygon by a line is the whole of it; and the counts it
// runs at are bounded by what a reader can read
// ([MaxVoronoiSites]) rather than by what a machine can hold.
//
// Most of the clipping is skipped rather than done. A cell is inside the
// circle of radius R about its site, where R is its farthest vertex so far, so
// a site more than 2R away cannot reach it — its bisector is more than R from
// the site. R shrinks as the cell is clipped, so the test gets stronger as it
// goes; what is left is a distance per pair, which is arithmetic without a
// polygon walk behind it.
//
// # It is a pure function of its input
//
// Nothing here iterates to a tolerance or reads a clock, and the answer does
// not depend on the order the sites arrive in: the cells are an intersection,
// and an intersection does not care in which order it was taken. The order
// decides only which of two coincident sites keeps the cell — the first — and
// the last bit of the float arithmetic, which is why the clipping is in site
// order rather than in an order this chose (docs/adr/0012-parallel-panels.md).
//
// It is a struct with a [Voronoi.Reset] rather than a function for the reason
// [Sankey], [Tidy] and [Stress] are: it keeps buffers the size of the data,
// and a chart redrawn every frame should reuse them.
//
// The zero Voronoi is empty; call [Voronoi.Reset] first.
//
// [Sutherland and Hodgman]: https://doi.org/10.1145/360767.360802
type Voronoi struct {
	// Cells has one region per site, in the order the sites were given, so a
	// caller reads the cell of the row it handed in without a second mapping.
	Cells []Region
	// Verts holds every cell's boundary, in order round the cell. Use
	// [Voronoi.Cell] rather than indexing it.
	Verts []Point
	// TooMany reports more than [MaxVoronoiSites] sites, in which case nothing
	// is partitioned at all: a diagram of cells too small to see is not what
	// the caller asked for, and half of one is not either.
	TooMany bool

	poly, next []Point
}

// Cell is site i's boundary, in order round it, and empty for a site with no
// cell. It is a view into [Voronoi.Verts] rather than a copy, so it is good
// until the next [Voronoi.Reset].
func (v *Voronoi) Cell(i int) []Point {
	if i < 0 || i >= len(v.Cells) {
		return nil
	}
	r := v.Cells[i]
	return v.Verts[r.At : r.At+r.N]
}

// Reset partitions the rectangle between the sites xs[i], ys[i].
//
// The two columns are read to the length of the shorter. A site whose position
// is not finite gets no cell, and neither does the second of two sites at
// exactly the same point: their bisector is undefined, and two cells drawn on
// top of each other would be two marks a reader cannot point at separately.
// Such a site still takes part in every other cell's clipping, because it is
// somewhere even if it is nowhere of its own.
//
// A site outside the rectangle is a site like any other: it cuts the cells
// inside and usually has none itself. That is what makes a zoomed or panned
// chart draw the same partition it drew before, less the part that has gone
// off the panel.
func (v *Voronoi) Reset(xs, ys []float64, x0, y0, x1, y1 float64) {
	v.Cells, v.Verts = v.Cells[:0], v.Verts[:0]
	v.TooMany = false
	n := min(len(xs), len(ys))
	if n == 0 {
		return
	}
	if n > MaxVoronoiSites {
		v.TooMany = true
		return
	}
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	for range n {
		v.Cells = append(v.Cells, Region{At: len(v.Verts)})
	}
	if !(x1 > x0) || !(y1 > y0) {
		return
	}
	for i := range n {
		if !finite(xs[i]) || !finite(ys[i]) || earlierAt(xs, ys, i) {
			continue
		}
		v.cell(xs, ys, i, n, x0, y0, x1, y1)
	}
}

// cell clips the rectangle down to site i's cell and appends it.
func (v *Voronoi) cell(xs, ys []float64, i, n int, x0, y0, x1, y1 float64) {
	sx, sy := xs[i], ys[i]
	v.poly = append(v.poly[:0],
		Point{X: x0, Y: y0}, Point{X: x1, Y: y0}, Point{X: x1, Y: y1}, Point{X: x0, Y: y1})
	reach := reachOf(v.poly, sx, sy)

	for j := range n {
		if j == i || !finite(xs[j]) || !finite(ys[j]) {
			continue
		}
		dx, dy := xs[j]-sx, ys[j]-sy
		d2 := dx*dx + dy*dy
		if d2 == 0 {
			// A site at this one's own point: the bisector between them is
			// every line through it, so there is no side to keep.
			continue
		}
		if d2 > 4*reach {
			// Farther than twice the cell's own reach, so its bisector misses
			// the cell altogether and the clip below would be a no-op.
			continue
		}
		// The bisector, as the points p with d·p = d·m for the midpoint m.
		// Keeping d·p ≤ d·m keeps the side this site is on.
		limit := dx*(sx+xs[j])/2 + dy*(sy+ys[j])/2
		v.next = clipHalf(v.next, v.poly, dx, dy, limit)
		if len(v.next) == 0 {
			// Clipped away to nothing: a site whose cell is outside the
			// rectangle, which has no cell rather than an empty one.
			v.poly = v.poly[:0]
			return
		}
		v.poly, v.next = v.next, v.poly
		reach = reachOf(v.poly, sx, sy)
	}

	if len(v.poly) < 3 {
		// Two vertices are an edge and one is a corner; neither is a cell, and
		// a subpath of no area is a mark a reader would be told they were
		// inside.
		return
	}
	v.Cells[i] = Region{At: len(v.Verts), N: len(v.poly)}
	v.Verts = append(v.Verts, v.poly...)
}

// reachOf is the square of the distance from (sx, sy) to the farthest vertex
// of poly: how far the cell can still reach from its own site.
func reachOf(poly []Point, sx, sy float64) float64 {
	worst := 0.0
	for _, p := range poly {
		dx, dy := p.X-sx, p.Y-sy
		if d2 := dx*dx + dy*dy; d2 > worst {
			worst = d2
		}
	}
	return worst
}

// clipHalf clips the convex polygon poly to the half-plane dx*x + dy*y ≤ limit,
// writing the result into dst.
//
// It is the Sutherland–Hodgman clip of one edge: each edge of the polygon
// contributes its start point where that is inside, and the crossing where the
// edge changes side. A convex polygon clipped by a line is convex, so the
// result is one ring and needs no reassembly.
func clipHalf(dst, poly []Point, dx, dy, limit float64) []Point {
	dst = dst[:0]
	if len(poly) == 0 {
		return dst
	}
	prev := poly[len(poly)-1]
	prevAt := dx*prev.X + dy*prev.Y - limit
	for _, p := range poly {
		at := dx*p.X + dy*p.Y - limit
		if (at <= 0) != (prevAt <= 0) {
			// The edge crosses the line: t is where, and the division is safe
			// because the two ends are strictly on opposite sides.
			t := prevAt / (prevAt - at)
			dst = append(dst, Point{
				X: prev.X + t*(p.X-prev.X),
				Y: prev.Y + t*(p.Y-prev.Y),
			})
		}
		if at <= 0 {
			dst = append(dst, p)
		}
		prev, prevAt = p, at
	}
	return dst
}

// earlierAt reports whether some site before i stands at exactly i's point.
//
// It is the quadratic scan rather than a map of positions because the sites are
// bounded by [MaxVoronoiSites] and a map keyed by two floats would allocate per
// frame for the one case it answers. The cell construction is quadratic anyway.
func earlierAt(xs, ys []float64, i int) bool {
	for j := range i {
		if xs[j] == xs[i] && ys[j] == ys[i] {
			return true
		}
	}
	return false
}

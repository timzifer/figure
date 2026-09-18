package geom

import (
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/ir"
)

// curveFit is the family a layer draws with and the parameter that family
// reads. It travels as one value because the two are only meaningful together:
// a tension without a family is a number nobody applies, and several families
// have no parameter at all.
type curveFit struct {
	kind    CurveKind
	tension float32
}

// defaultBeta is [CurveBundle]'s blend when nobody set a [Tension]. It is
// d3's, and it is not 1: a bundle at beta 1 is a plain [CurveBasis], so
// defaulting there would make the two families draw the same thing and hide
// the choice.
const defaultBeta = 0.85

// curveFit resolves the family from the options.
//
// A [Tension] with no [Curve] is [CurveCardinal] at that tension, which is
// what this option meant before there were families to choose between — so
// every chart written against the old spelling draws what it always drew, and
// the polyline fast path is still reached by saying nothing at all.
func (c config) curveFit() curveFit {
	if !c.curveSet {
		if c.tension <= 0 {
			return curveFit{}
		}
		return curveFit{kind: CurveCardinal, tension: float32(clamp01(c.tension))}
	}
	cv := curveFit{kind: c.curve, tension: float32(clamp01(c.tension))}
	if cv.tension > 0 {
		return cv
	}
	switch cv.kind {
	case CurveCardinal, CurveCardinalOpen, CurveCardinalClosed:
		cv.tension = 1
	case CurveBundle:
		cv.tension = defaultBeta
	}
	return cv
}

// appendCurve appends pts to p, straight under [CurveLinear] and as the chosen
// family's cubics otherwise, starting a new subpath when move is set.
//
// Continuing an existing subpath is what lets an area append its lower edge to
// its upper one and get a single closed shape rather than two.
//
// A coord that bends its edges takes them over from every family. Smoothing is
// a curve fitted through device positions, and under a polar transform the
// tangent it fits is not the tangent the data has — so an edge the coord
// already knows how to draw is the better answer than a spline through points
// it has moved.
func (sc *scratch) appendCurve(p *ir.Path, cd coord.Coord, pts []ir.Point, cv curveFit, move bool) {
	if len(pts) == 0 {
		return
	}
	if !cd.Straight() {
		appendEdges(p, cd, pts, move)
		return
	}
	if !cv.kind.Smoothed() || len(pts) < 3 {
		appendPolyline(p, pts, move)
		return
	}
	switch cv.kind {
	case CurveCardinal, CurveCardinalOpen, CurveCardinalClosed:
		cardinal(p, pts, cv.tension, cv.kind, move)
	case CurveMonotone:
		sc.monotone(p, pts, move)
	case CurveNatural:
		sc.natural(p, pts, move)
	case CurveBasis, CurveBasisOpen, CurveBasisClosed:
		basis(p, pts, cv.kind, move)
	case CurveBundle:
		basis(p, sc.bundled(pts, cv.tension), CurveBasis, move)
	default:
		appendPolyline(p, pts, move)
	}
}

// appendPolyline is the straight path through pts, and the fallback of every
// family that cannot describe the points it was given.
func appendPolyline(p *ir.Path, pts []ir.Point, move bool) {
	begin(p, pts[0], move)
	for _, q := range pts[1:] {
		p.LineTo(q.X, q.Y)
	}
}

// begin opens the run at q, either as a new subpath or as a segment of the one
// already being built.
func begin(p *ir.Path, q ir.Point, move bool) {
	if move {
		p.MoveTo(q.X, q.Y)
	} else {
		p.LineTo(q.X, q.Y)
	}
}

// cardinal appends a Catmull-Rom spline through pts as cubic Béziers.
//
// tension in (0, 1] scales the tangents: 1 gives the classic uniform
// Catmull-Rom curve, smaller values pull the curve back towards the polyline.
// The curve passes through every data point, which matters — a smoothing that
// misses the data would be drawing something that was never measured.
//
// The three variants differ only in what a vertex at the end of the run uses
// for its missing neighbour. The plain family repeats the end point, which is
// what makes the curve start and finish at the data; the open one declines to
// draw those two segments at all; the closed one takes the neighbour from the
// other end of the run and closes the loop.
func cardinal(p *ir.Path, pts []ir.Point, tension float32, kind CurveKind, move bool) {
	n := len(pts)
	k := tension / 6
	switch kind {
	case CurveCardinalClosed:
		begin(p, pts[0], move)
		for i := range n {
			cardinalSpan(p, pts[(i-1+n)%n], pts[i], pts[(i+1)%n], pts[(i+2)%n], k)
		}
		p.Close()
	case CurveCardinalOpen:
		// The first and last spans are the ones whose neighbour had to be
		// invented, so an open cardinal simply has no first and last span.
		// Three points leave nothing behind, which is why d3 draws nothing
		// there either.
		if n < 4 {
			return
		}
		begin(p, pts[1], move)
		for i := 1; i < n-2; i++ {
			cardinalSpan(p, pts[i-1], pts[i], pts[i+1], pts[i+2], k)
		}
	default:
		begin(p, pts[0], move)
		for i := 0; i < n-1; i++ {
			cardinalSpan(p, pts[max(i-1, 0)], pts[i], pts[i+1], pts[min(i+2, n-1)], k)
		}
	}
}

// cardinalSpan appends the cubic from p1 to p2, given the neighbours that set
// the two tangents.
func cardinalSpan(p *ir.Path, p0, p1, p2, p3 ir.Point, k float32) {
	p.CubicTo(
		p1.X+(p2.X-p0.X)*k, p1.Y+(p2.Y-p0.Y)*k,
		p2.X-(p3.X-p1.X)*k, p2.Y-(p3.Y-p1.Y)*k,
		p2.X, p2.Y,
	)
}

// monotone appends a Fritsch-Carlson monotone cubic through pts.
//
// It is the family to reach for when the data only ever goes one way, because
// it is the only interpolating one that cannot overshoot: a cardinal or
// natural spline through a rising series dips below its own previous reading
// between two samples, and that dip is a value nobody measured.
//
// It needs an independent axis to be monotone in, and takes whichever of the
// two device axes is — x for the usual series, y for one plotted sideways.
// When neither is, the run is not a function of either coordinate, no monotone
// fit is defined, and it falls back to the polyline rather than quietly
// fitting some other family.
func (sc *scratch) monotone(p *ir.Path, pts []ir.Point, move bool) {
	flip, ok := paramAxis(pts)
	if !ok {
		appendPolyline(p, pts, move)
		return
	}
	n := len(pts)
	m := grow(sc.slope, n)
	sc.slope = m

	// Secant slopes first, held in the tangent buffer: the tangent of a vertex
	// is a function of the two secants beside it, and both are read before any
	// is overwritten.
	for i := range n - 1 {
		hx, hy := span(pts[i], pts[i+1], flip)
		m[i] = hy / hx
	}
	// Walk backwards so that m[i]'s two secants, m[i-1] and m[i], are both
	// still secants when it is written.
	for i := n - 1; i > 0; i-- {
		if i == n-1 {
			m[i] = m[i-1]
			continue
		}
		d0, d1 := m[i-1], m[i]
		switch {
		case d0*d1 <= 0:
			// A turning point. A monotone fit is flat there; anything else
			// would leave the interval the two readings bound.
			m[i] = 0
		default:
			m[i] = clampSlope((d0+d1)/2, d0, d1)
		}
	}
	// The first vertex keeps its own secant, which cannot overshoot the only
	// span it bounds.

	begin(p, pts[0], move)
	for i := range n - 1 {
		h, _ := span(pts[i], pts[i+1], flip)
		hermiteSpan(p, pts[i], pts[i+1], m[i], m[i+1], h, flip)
	}
}

// clampSlope limits a tangent to three times the shallower of the two secants
// beside it, which is the Fritsch-Carlson condition for monotonicity.
func clampSlope(t, d0, d1 float32) float32 {
	lim := 3 * minAbs(d0, d1)
	if t > lim {
		return lim
	}
	if t < -lim {
		return -lim
	}
	return t
}

func minAbs(a, b float32) float32 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	return min(a, b)
}

// natural appends a natural cubic spline through pts: the C2-continuous
// interpolant whose curvature is zero at both ends.
//
// It is the smoothest curve that still passes through every vertex, and it
// pays for that with a global solve — moving one point changes the whole
// curve — and with the overshoot [CurveMonotone] exists to avoid.
func (sc *scratch) natural(p *ir.Path, pts []ir.Point, move bool) {
	flip, ok := paramAxis(pts)
	if !ok {
		appendPolyline(p, pts, move)
		return
	}
	n := len(pts)
	mm := grow(sc.slope, n)
	sc.slope = mm
	w := grow(sc.tri, 2*n)
	sc.tri = w
	cp, dp := w[:n], w[n:] // the Thomas algorithm's two swept rows

	// Second derivatives M, from h[i-1]·M[i-1] + 2(h[i-1]+h[i])·M[i] +
	// h[i]·M[i+1] = 6(δ[i] − δ[i-1]), with M at both ends pinned to zero.
	// Swept forward and substituted back in place.
	cp[0], dp[0] = 0, 0
	for i := 1; i < n-1; i++ {
		h0, r0 := span(pts[i-1], pts[i], flip)
		h1, r1 := span(pts[i], pts[i+1], flip)
		d0, d1 := r0/h0, r1/h1
		den := 2*(h0+h1) - h0*cp[i-1]
		cp[i] = h1 / den
		dp[i] = (6*(d1-d0) - h0*dp[i-1]) / den
	}
	mm[n-1] = 0
	for i := n - 2; i > 0; i-- {
		mm[i] = dp[i] - cp[i]*mm[i+1]
	}
	mm[0] = 0

	// Turn the second derivatives into the first derivative at each end of
	// each span, which is what the Bézier control points are built from.
	begin(p, pts[0], move)
	for i := range n - 1 {
		h, r := span(pts[i], pts[i+1], flip)
		d := r / h
		m0 := d - h*(2*mm[i]+mm[i+1])/6
		m1 := d + h*(mm[i]+2*mm[i+1])/6
		hermiteSpan(p, pts[i], pts[i+1], m0, m1, h, flip)
	}
}

// hermiteSpan appends the cubic from a to b with the given end slopes, in the
// parameterisation paramAxis chose. A cubic Hermite's Bézier controls sit a
// third of the span along each tangent.
func hermiteSpan(p *ir.Path, a, b ir.Point, m0, m1, h float32, flip bool) {
	t := h / 3
	if flip {
		p.CubicTo(a.X+m0*t, a.Y+t, b.X-m1*t, b.Y-t, b.X, b.Y)
		return
	}
	p.CubicTo(a.X+t, a.Y+m0*t, b.X-t, b.Y-m1*t, b.X, b.Y)
}

// paramAxis reports which device axis the run is a function of: false for x,
// true for y. It is not ok when neither is strictly monotone, which is a run
// that doubles back on itself.
func paramAxis(pts []ir.Point) (flip, ok bool) {
	if monotonic(pts, false) {
		return false, true
	}
	if monotonic(pts, true) {
		return true, true
	}
	return false, false
}

// monotonic reports whether the chosen coordinate is strictly monotone. Equal
// neighbours disqualify it: the span between them has no slope.
func monotonic(pts []ir.Point, flip bool) bool {
	up, down := true, true
	for i := range len(pts) - 1 {
		h, _ := span(pts[i], pts[i+1], flip)
		switch {
		case h > 0:
			down = false
		case h < 0:
			up = false
		default:
			return false
		}
	}
	return up || down
}

// span returns the step along the independent axis and the rise along the
// other one, for whichever way round paramAxis chose.
func span(a, b ir.Point, flip bool) (h, rise float32) {
	if flip {
		return b.Y - a.Y, b.X - a.X
	}
	return b.X - a.X, b.Y - a.Y
}

// basis appends a uniform cubic B-spline over pts.
//
// It does not pass through them. The vertices are control points that pull the
// curve towards themselves, so what is drawn is a smoothing of the sequence
// rather than a claim about any value between two rows — see [CurveKind]. Use
// it when the shape of a bundle of traces is the point; do not use it to draw
// a measurement.
//
// The plain family repeats each end twice so the curve still starts and
// finishes at the data; the open one does not, so it begins and ends short;
// the closed one wraps and closes.
func basis(p *ir.Path, pts []ir.Point, kind CurveKind, move bool) {
	n := len(pts)
	spans := n + 1
	switch kind {
	case CurveBasisClosed:
		spans = n
	case CurveBasisOpen:
		// Three points leave no span whose four control points are all data,
		// which is why d3 draws nothing there either.
		if n < 4 {
			return
		}
		spans = n - 3
	}
	begin(p, basisAt(basisPt(pts, kind, 0), basisPt(pts, kind, 1), basisPt(pts, kind, 2)), move)
	for i := range spans {
		c1, c2, c3 := basisPt(pts, kind, i+1), basisPt(pts, kind, i+2), basisPt(pts, kind, i+3)
		end := basisAt(c1, c2, c3)
		p.CubicTo(
			(2*c1.X+c2.X)/3, (2*c1.Y+c2.Y)/3,
			(c1.X+2*c2.X)/3, (c1.Y+2*c2.Y)/3,
			end.X, end.Y,
		)
	}
	if kind == CurveBasisClosed {
		p.Close()
	}
}

// basisPt maps a control index onto a vertex. The padding is the whole
// difference between the three variants: the plain family repeats each end,
// which is what keeps the curve's own ends on the data; the open one pads
// nothing and so begins and finishes short; the closed one wraps.
func basisPt(pts []ir.Point, kind CurveKind, i int) ir.Point {
	n := len(pts)
	switch kind {
	case CurveBasisClosed:
		return pts[((i%n)+n)%n]
	case CurveBasisOpen:
		return pts[i]
	default:
		return pts[min(max(i-2, 0), n-1)]
	}
}

// basisAt is a uniform cubic B-spline's value at a knot: the weighted average
// of the three control points around it.
func basisAt(a, b, c ir.Point) ir.Point {
	return ir.Point{X: (a.X + 4*b.X + c.X) / 6, Y: (a.Y + 4*b.Y + c.Y) / 6}
}

// bundled blends pts towards the straight chord from the first to the last,
// which is what turns a B-spline into a bundled edge: beta 1 leaves the points
// alone and beta 0 collapses them onto the chord.
func (sc *scratch) bundled(pts []ir.Point, beta float32) []ir.Point {
	n := len(pts)
	out := grow(sc.bpts, n)
	sc.bpts = out
	first, last := pts[0], pts[n-1]
	dx, dy := last.X-first.X, last.Y-first.Y
	for i, q := range pts {
		t := float32(i) / float32(n-1)
		out[i] = ir.Point{
			X: beta*q.X + (1-beta)*(first.X+t*dx),
			Y: beta*q.Y + (1-beta)*(first.Y+t*dy),
		}
	}
	return out
}

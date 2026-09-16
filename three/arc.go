package three

import (
	"math"
	"math/cmplx"
)

// Arc is how a layer joins two of its rows: the path the segment between them
// takes through the scene's own space.
//
// In a box it is the straight line, because a straight line in the data is a
// straight line in the picture. In a [Spherical] scene it is the great circle
// between the two directions, with the radius run linearly along it — the
// reason is `coord.Polar`'s for drawing an edge as an arc in two dimensions,
// and it matters more here. A dense sweep looks right joined by chords; a
// coarse one cuts through the inside of the ball, so two states a quarter turn
// apart on a Bloch sphere are joined by a line through the middle of it rather
// than by the arc the state actually travelled. Under [Smith] the path is the
// straight line between the two impedances' reflection coefficients, which a
// stereographic projection takes to a circle on the ball — so it is still a
// curve on the surface rather than a chord through the solid.
//
// A layer walks it with [Arc.Steps] and [Arc.At] and emits one primitive per
// step, which is [Line3]'s rule for the same reason it emits one per segment:
// a curve that spans the scene has no single depth.
//
// Two segments are drawn as chords however the scene is shaped, because
// neither has an arc to be drawn as. One whose endpoints are antipodal has no
// unique great circle between them — every meridian is one — and one with an
// end at the centre has no direction at that end.
type Arc struct {
	// from and to are the endpoints in scene space, and are all a straight
	// segment needs.
	from, to Vec3
	// ua and ub are the unit directions from the centre and ra, rb the radii,
	// for a segment on the ball; ang is the angle between the directions.
	ua, ub Vec3
	ra, rb float64
	ang    float64
	// ga and gb are the endpoints in the Γ-plane, for a Smith scene, and wa,
	// wb the two radii the path runs between.
	ga, gb complex128
	wa, wb float64

	kind  arcKind
	steps int
}

type arcKind uint8

const (
	arcStraight arcKind = iota
	arcGreatCircle
	arcGamma
)

// arcStep is how much of a turn one step of a curved segment covers: five
// degrees, which is the graticule's own sampling and finer than a reader can
// see a polygon in at any size a figure is drawn at.
const arcStep = 5 * math.Pi / 180

// maxArcSteps bounds the walk. A segment that runs most of the way round the
// ball is already a claim about a path nobody measured; drawing it in seventy
// pieces rather than seven hundred is not what makes it one.
const maxArcSteps = 72

// arcEps is how near a segment's two ends have to be to the same direction, or
// to opposite ones, before the great circle between them stops being decided.
const arcEps = 1e-6

// Arc returns the path between two rows, given both rows' three values.
func (f Frame) Arc(x0, y0, z0, x1, y1, z1 float64) Arc {
	a := Arc{from: f.Point(x0, y0, z0), to: f.Point(x1, y1, z1), steps: 1}
	if f.space == nil {
		return a
	}
	if f.space.smith {
		return a.gammaPath(gamma(x0, y0), gamma(x1, y1), float64(at(f.Z, z0)), float64(at(f.Z, z1)))
	}
	return a.greatCircle()
}

// greatCircle fills in the walk along the ball, or leaves the segment straight
// where there is no arc to draw it as.
func (a Arc) greatCircle() Arc {
	va, vb := a.from.Sub(centre), a.to.Sub(centre)
	a.ra, a.rb = math.Sqrt(va.Dot(va)), math.Sqrt(vb.Dot(vb))
	if a.ra == 0 || a.rb == 0 {
		return a
	}
	a.ua, a.ub = va.Unit(), vb.Unit()
	a.ang = math.Acos(clampMu(a.ua.Dot(a.ub)))
	if a.ang < arcEps || a.ang > math.Pi-arcEps {
		return a
	}
	a.kind, a.steps = arcGreatCircle, stepsFor(a.ang)
	return a
}

// gammaPath fills in the walk along the image of the straight line between two
// reflection coefficients.
func (a Arc) gammaPath(ga, gb complex128, wa, wb float64) Arc {
	if cmplx.IsInf(ga) || cmplx.IsInf(gb) || cmplx.IsNaN(ga) || cmplx.IsNaN(gb) {
		// An impedance of z = −1 is at the pole of the map and the line
		// through it has no image on the ball to follow.
		return a
	}
	va, vb := a.from.Sub(centre), a.to.Sub(centre)
	if va.Dot(va) == 0 || vb.Dot(vb) == 0 {
		return a
	}
	ang := math.Acos(clampMu(va.Unit().Dot(vb.Unit())))
	if ang < arcEps {
		return a
	}
	a.ga, a.gb, a.wa, a.wb = ga, gb, wa, wb
	a.kind, a.steps = arcGamma, stepsFor(ang)
	return a
}

// stepsFor is how many pieces a turn of the given size is drawn in.
func stepsFor(ang float64) int {
	n := int(math.Ceil(ang / arcStep))
	switch {
	case n < 1:
		return 1
	case n > maxArcSteps:
		return maxArcSteps
	}
	return n
}

// Steps is how many pieces the segment is drawn in: one for a straight
// segment, and enough that no piece turns more than a few degrees for a curved
// one.
func (a Arc) Steps() int { return a.steps }

// At is the point a fraction t of the way along the segment, with At(0) and
// At(1) the two rows themselves.
func (a Arc) At(t float32) Vec3 {
	switch a.kind {
	case arcGreatCircle:
		// Slerp along the great circle, with the radius run linearly, so a
		// path that spirals outward stays on the ball's surface only if it
		// started there.
		s := math.Sin(a.ang)
		f0 := math.Sin((1-float64(t))*a.ang) / s
		f1 := math.Sin(float64(t)*a.ang) / s
		dir := a.ua.Mul(float32(f0)).Add(a.ub.Mul(float32(f1)))
		r := a.ra + (a.rb-a.ra)*float64(t)
		return centre.Add(dir.Mul(float32(r)))
	case arcGamma:
		g := a.ga + complex(float64(t), 0)*(a.gb-a.ga)
		return riemann(g, a.wa+(a.wb-a.wa)*float64(t))
	}
	return a.from.Add(a.to.Sub(a.from).Mul(t))
}

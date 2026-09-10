package three

import "math"

// Vec3 is a point or a direction in scene space.
//
// Scene space is the unit cube: x and y span the floor, z is up, and every
// coordinate lies in [0, 1] because that is the interval the three scales are
// ranged into. A scale maps a value into an interval without caring what the
// interval means — the property ADR 0018 was built on — and here the interval
// is an edge of a box rather than an edge of a panel.
//
// The components are float32 because [github.com/timzifer/figure/ir.Point] is,
// so the projection converts nothing on the hot path.
type Vec3 struct{ X, Y, Z float32 }

// V is the short spelling of a Vec3, for the geometry that builds a lot of
// them.
func V(x, y, z float32) Vec3 { return Vec3{x, y, z} }

// Add returns v + w.
func (v Vec3) Add(w Vec3) Vec3 { return Vec3{v.X + w.X, v.Y + w.Y, v.Z + w.Z} }

// Sub returns v - w.
func (v Vec3) Sub(w Vec3) Vec3 { return Vec3{v.X - w.X, v.Y - w.Y, v.Z - w.Z} }

// Mul returns v scaled by f.
func (v Vec3) Mul(f float32) Vec3 { return Vec3{v.X * f, v.Y * f, v.Z * f} }

// Cross returns the vector perpendicular to both v and w, which is how a face
// gets the normal its shade is computed from.
func (v Vec3) Cross(w Vec3) Vec3 {
	return Vec3{
		v.Y*w.Z - v.Z*w.Y,
		v.Z*w.X - v.X*w.Z,
		v.X*w.Y - v.Y*w.X,
	}
}

// Dot returns the scalar product, in float64.
//
// The width is deliberate and it is not symmetry with the rest of the package:
// this is the arithmetic behind a depth key, a depth key feeds a comparison,
// and a comparison is a decision. AGENTS.md records what one float32 ulp did
// to a decision once already.
func (v Vec3) Dot(w Vec3) float64 {
	return float64(v.X)*float64(w.X) + float64(v.Y)*float64(w.Y) + float64(v.Z)*float64(w.Z)
}

// Unit returns v scaled to length one, or the zero vector for a zero v.
func (v Vec3) Unit() Vec3 {
	n := math.Sqrt(v.Dot(v))
	if n == 0 {
		return Vec3{}
	}
	return v.Mul(float32(1 / n))
}

// centre is the middle of the unit cube, which every camera looks at and every
// projection is measured from.
var centre = Vec3{0.5, 0.5, 0.5}

// diameter is the length of the unit cube's main diagonal.
//
// Fitting a scene to this rather than to the projected bounding box of the
// cube is what keeps a scene the same size while it turns: the box breathes
// with the camera and the sphere does not, and a measuring instrument whose
// scale changes under the reader's hand is not one. [Dolly] is how the reader
// reclaims the room this costs.
var diameter = float32(math.Sqrt(3))

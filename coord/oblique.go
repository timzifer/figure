package coord

import (
	"math"

	"github.com/timzifer/figure/ir"
)

// Extruder is implemented by a coord that sees its panel from an angle: a mark
// drawn under it has a back as well as a front.
//
// It is an optional interface, the shape [Exploder] established, and
// [Cartesian] deliberately does not implement it. A layer that asks to be
// extruded under a coord with no angle to see it from draws exactly what it
// always drew, and nothing is invented. A geom resolves the interface once per
// Build, never per mark. See docs/adr/0055-depth-without-a-third-axis.md.
type Extruder interface {
	// Extrude reports the device offset from a mark's front face to its back
	// one. It is the same vector for every mark in the panel.
	Extrude() (dx, dy float32)
}

// ObliqueOption configures an oblique coord.
type ObliqueOption func(*oblique)

// Depth sets how deep a mark's volume is drawn, as a fraction of the panel's
// shorter side. The default is 0.04; a value at or below zero is ignored, and
// one above a quarter is held at a quarter, past which the back faces take
// more of the panel than the data does.
//
// It is a fraction rather than a length in pixels for the reason a chart
// follows its surface by scaling its theme (docs/adr/0025-responsive-charts.md):
// a slab of fourteen pixels on a chart half the size is twice as deep a slab.
// A coord does not know what a theme is, and the panel rectangle it is framed
// in is the one length it does know.
func Depth(f float64) ObliqueOption {
	return func(o *oblique) {
		if f > 0 {
			o.depth = math.Min(f, maxDepth)
		}
	}
}

// DepthAngle sets the direction the back faces are offset in, in radians in
// device space: zero is to the right, and a negative angle is up, because a
// device's Y axis grows downwards. The default is −π/4 — up and to the right,
// so a bar shows its top and its right-hand side.
func DepthAngle(radians float64) ObliqueOption {
	return func(o *oblique) {
		if !math.IsNaN(radians) && !math.IsInf(radians, 0) {
			// Reduced to (−π, π], so that a full turn is the angle zero
			// exactly rather than one whose sine is a rounding error: a
			// depth vector with a vertical part of 1e-16 would still be a
			// vertical part, and give every mark a top face of no height.
			o.angle = math.Remainder(radians, FullTurn)
		}
	}
}

const (
	defaultDepth      = 0.04
	defaultDepthAngle = -math.Pi / 4
	maxDepth          = 0.25
)

// Oblique returns a Cartesian coord seen from a corner: the same axes, the same
// straight edges and the same inverse, with a depth vector — reported through
// [Extruder] — that an extruded mark reads its back faces from.
//
// It is depth as decoration and nothing more. The front plane is not
// foreshortened, so a bar is as tall in pixels as it would be on a flat chart
// and two equal bars are equal wherever they stand; there is no perspective,
// and there will not be one, because a vanishing point makes the same value
// taller at the front of a scene than at the back. No column can set the depth
// — a third variable gets an axis in docs/adr/0056-three-dimensional-charts.md,
// not a nameless displacement here.
//
// The volume comes out of the plot area rather than out of the margins: framing
// insets the rectangle the scales map into by the depth vector, so the axes
// still bound the data, the back faces stay inside the panel's clip, and the
// layout solver is asked nothing new.
func Oblique(opts ...ObliqueOption) Coord {
	o := oblique{depth: defaultDepth, angle: defaultDepthAngle}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// oblique is the unframed coord: a Cartesian one that knows how deep and at
// what angle its panels are seen.
type oblique struct {
	cartesian
	depth, angle float64
}

// vector is the device offset from front to back in a panel of the given
// rectangle.
func (o oblique) vector(area ir.Rect) (dx, dy float32) {
	side := math.Min(float64(area.Max.X-area.Min.X), float64(area.Max.Y-area.Min.Y))
	if side <= 0 {
		return 0, 0
	}
	d := o.depth * side
	return float32(d * math.Cos(o.angle)), float32(d * math.Sin(o.angle))
}

func (o oblique) Frame(f Framing) Coord {
	dx, dy := o.vector(f.Area)
	front := inset(f.Area, dx, dy)
	framed := cartesian{}.Frame(Framing{Area: front, X: f.X, Y: f.Y, Z: f.Z}).(framedCartesian)
	return framedOblique{framedCartesian: framed, spec: o, dx: dx, dy: dy}
}

// inset is the front plane of a panel whose back faces are offset by (dx, dy):
// the rectangle shrunk on the sides the offset points towards.
func inset(area ir.Rect, dx, dy float32) ir.Rect {
	r := area
	if dx > 0 {
		r.Max.X -= dx
	} else {
		r.Min.X -= dx
	}
	if dy > 0 {
		r.Max.Y -= dy
	} else {
		r.Min.Y -= dy
	}
	if r.Max.X < r.Min.X {
		r.Max.X = r.Min.X
	}
	if r.Max.Y < r.Min.Y {
		r.Max.Y = r.Min.Y
	}
	return r
}

// Describe writes an angle of exactly zero as a full turn, because a zero in a
// Desc is the default angle — see [Desc].
func (o oblique) Describe() Desc {
	angle := o.angle
	if angle == 0 {
		angle = FullTurn
	}
	return Desc{Type: TypeOblique, Depth: o.depth, DepthAngle: angle}
}

// framedOblique is an oblique coord positioned in a panel. Everything a
// Cartesian coord answers it answers about the front plane, which is where the
// scales map into.
type framedOblique struct {
	framedCartesian
	spec   oblique
	dx, dy float32
}

func (f framedOblique) Extrude() (dx, dy float32) { return f.dx, f.dy }

// Furniture places the axes and the grid on the front plane, whatever
// rectangle it is handed: the panel's rectangle includes the room the back
// faces take, and an axis drawn along its edge would stand a depth away from
// the data it measures.
func (f framedOblique) Furniture(dst *Furniture, req FurnitureRequest) {
	req.Area = f.area
	f.framedCartesian.Furniture(dst, req)
}

// FurnitureOpposite places a second axis on the front plane, for the reason
// [framedOblique.Furniture] does.
func (f framedOblique) FurnitureOpposite(dst *Furniture, req FurnitureRequest) {
	req.Area = f.area
	f.framedCartesian.FurnitureOpposite(dst, req)
}

func (f framedOblique) Frame(fr Framing) Coord { return f.spec.Frame(fr) }

func (f framedOblique) Describe() Desc { return f.spec.Describe() }

var (
	_ Coord     = oblique{}
	_ Coord     = framedOblique{}
	_ Describer = oblique{}
	_ Describer = framedOblique{}
	_ Extruder  = framedOblique{}
	_ Opposite  = framedOblique{}
)

package coord

import (
	"errors"
	"fmt"
	"math"

	"github.com/timzifer/figure/scale"
)

// Type names a coordinate system. It is the word a written-down chart carries
// in place of the constructor that built the coord.
type Type string

// The coord types.
const (
	TypeCartesian Type = "cartesian"
	TypePolar     Type = "polar"
	TypeSmith     Type = "smith"
	// TypeOblique is a Cartesian coord seen from a corner. See [Oblique].
	TypeOblique Type = "oblique"
	// TypeTernary is the barycentric coord: two components of a composition
	// on the axes and the third derived. See [Ternary].
	TypeTernary Type = "ternary"
	// TypeParallel is the coord with one vertical axis per dimension. See
	// [Parallel].
	TypeParallel Type = "parallel"
	// TypeGeo is the map projection: a longitude and a latitude placed on the
	// plane. See [Geo].
	TypeGeo Type = "geo"
)

// Desc is a coord reduced to what configures it.
//
// It is the bargain [github.com/timzifer/figure/scale.Desc] makes, for the
// same reason: a Coord is an interface over an unexported type, which is right
// for mapping positions and useless for writing one down. Nothing about a
// coord is a Go function, so unlike a scale, nothing here is lost.
type Desc struct {
	// Type is which coord this is.
	Type Type

	// Dims are the axes of a coord that has more than the panel's two, in the
	// order they are drawn: a [Parallel] coord's dimensions and nothing else
	// today.
	//
	// It is the first field here that is a list, and the reason is that it is
	// the first coord whose configuration is one. Every other coord in this
	// package is configured by a handful of numbers because it has a fixed
	// number of axes; a coordinate system whose axes are the thing being
	// configured cannot be written down as scalars without a limit on how many
	// of them there may be.
	Dims []DimDesc

	// Theta is the axis a polar coord sweeps around the circle.
	Theta Axis
	// Hole is the inner radius as a fraction of the outer one, zero for a
	// coord with no hole.
	Hole float64
	// Radius is how much of the panel's shorter half-side the circle fills.
	Radius float64
	// Start is where the angular scale begins, in radians clockwise from
	// twelve o'clock, and Sweep how much of the circle it covers.
	Start, Sweep float64
	// Counterclockwise reverses the direction the angular scale runs in.
	Counterclockwise bool
	// Chord reports a coord drawing an edge between two marks as the straight
	// line between them rather than as an arc. It is a polar coord's choice,
	// whose default is the arc.
	Chord bool

	// Arc is the same choice made by a coord whose default is the other one:
	// it reports a Smith coord drawing an edge as the true image of a straight
	// data-space edge rather than as the chord it draws by default.
	//
	// The two are separate fields rather than one because a zero Desc has to
	// mean each coord's own default, and the two coords default opposite ways
	// — a rose petal's side is an arc, a measured locus is a chord. One field
	// would make a document that named a type and nothing else draw something
	// its constructor does not.
	Arc bool

	// Admittance reports a Smith coord mirrored through its centre — Γ ↦ −Γ —
	// so that the pair reads as a conductance and a susceptance. See
	// [SmithAdmittance].
	Admittance bool

	// Sum is what a ternary coord's three components add up to: 1 for
	// fractions, 100 for percentages. Zero is the default, which is 1, so a
	// Desc that names the type and nothing else draws what [Ternary] draws.
	Sum float64

	// Projection is which map projection a [Geo] coord draws, and CenterLon
	// and CenterLat where it is centred, in degrees.
	//
	// The projection is a name rather than a function for the reason the
	// curve families and the locus families are names: a Go function cannot
	// be written down, and a coord that could not be written down would be a
	// chart the spec loses. The centre is two numbers because a projection's
	// configuration is a handful of numbers, which is what [Desc] is for.
	Projection           Projection
	CenterLon, CenterLat float64

	// Depth and DepthAngle are an oblique coord's depth vector: how deep, as
	// a fraction of the panel's shorter side, and in which direction, in
	// radians in device space. See [Depth] and [DepthAngle].
	//
	// Zero is each one's default, for the reason Chord and Arc are two
	// fields: a Desc that names the type and nothing else must draw what
	// [Oblique] draws. A depth vector pointing straight to the right is
	// therefore written as a full turn, 2π, which is the same direction.
	Depth, DepthAngle float64
}

// DimDesc is one axis of a coord that has more than the panel's two: what it
// is called, and the scale that places a value on it.
//
// The scale travels as a [github.com/timzifer/figure/scale.Desc] rather than
// as a scale, for that type's own reason — a scale is an interface over an
// unexported type, and a description is what can be written down. A scale
// nobody can describe describes as the zero Desc, which reads back as a
// linear one.
type DimDesc struct {
	Name  string
	Scale scale.Desc
}

// Describe reports c's configuration, or ok == false if c cannot describe
// itself. A third-party coord that does not implement [Describer] still draws;
// it is simply not serializable.
func Describe(c Coord) (Desc, bool) {
	d, ok := c.(Describer)
	if !ok {
		return Desc{}, false
	}
	return d.Describe(), true
}

// FromDesc rebuilds a coord from its description. A type this package does not
// define is built by whoever registered it — see [Register] — and one nobody
// did is [ErrUnknownType].
func FromDesc(d Desc) (Coord, error) {
	switch d.Type {
	case "", TypeCartesian:
		return Cartesian(), nil
	case TypePolar:
		opts := []PolarOption{Theta(d.Theta), Hole(d.Hole), Radius(d.Radius),
			Start(d.Start), Sweep(d.Sweep), Counterclockwise(d.Counterclockwise)}
		if d.Chord {
			opts = append(opts, Chord())
		}
		return Polar(opts...), nil

	case TypeOblique:
		opts := []ObliqueOption{Depth(d.Depth)}
		if d.DepthAngle != 0 {
			opts = append(opts, DepthAngle(d.DepthAngle))
		}
		return Oblique(opts...), nil

	case TypeSmith:
		opts := []SmithOption{SmithRadius(d.Radius), SmithAdmittance(d.Admittance)}
		if d.Arc {
			opts = append(opts, SmithArc())
		}
		return Smith(opts...), nil

	case TypeTernary:
		return Ternary(TernarySum(d.Sum)), nil

	case TypeGeo:
		p := d.Projection
		if p == "" {
			// The absent projection is the equal-area one: a map that
			// misleads about size is asked for by name. See
			// [DefaultProjection].
			p = DefaultProjection
		}
		if !p.Known() {
			return nil, fmt.Errorf("%w: %q", ErrUnknownProjection, p)
		}
		opts := []GeoOption{GeoCenter(d.CenterLon, d.CenterLat)}
		if d.Arc {
			opts = append(opts, GeoArc())
		}
		return Geo(p, opts...), nil

	case TypeParallel:
		dims := make([]ParallelDim, 0, len(d.Dims))
		for _, e := range d.Dims {
			s, err := scale.FromDesc(e.Scale)
			if err != nil {
				return nil, fmt.Errorf("figure/coord: the %q axis: %w", e.Name, err)
			}
			dims = append(dims, Dim(e.Name, s))
		}
		return Parallel(dims...), nil
	}
	if build, ok := registered(d.Type); ok {
		return build(d)
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownType, d.Type)
}

// ErrUnknownType reports a Desc naming a coord this package does not have and
// nobody registered.
var ErrUnknownType = errors.New("figure/coord: unknown coord type")

// ErrUnknownProjection reports a Desc naming a map projection this package
// does not draw. Unlike a coord type it cannot be registered: the set is
// closed so that a map can be written down, and a projection of one's own is a
// [Coord] of one's own. See [Projection].
var ErrUnknownProjection = errors.New("figure/coord: unknown map projection")

func (p *polar) Describe() Desc {
	return Desc{
		Type:             TypePolar,
		Theta:            p.theta,
		Hole:             p.hole,
		Radius:           p.radius,
		Start:            p.start,
		Sweep:            p.sweep,
		Counterclockwise: p.ccw,
		Chord:            p.edge == chordEdges,
	}
}

// Default reports whether d describes the coord a chart has when nobody chose
// one. A document does not carry a field for that: the absent coord and the
// Cartesian one draw the same chart, and writing `"coord": {"type":
// "cartesian"}` into every spec figure has ever produced would be noise.
func (d Desc) Default() bool { return d.Type == "" || d.Type == TypeCartesian }

// FullTurn is the default sweep: a whole circle.
const FullTurn = 2 * math.Pi

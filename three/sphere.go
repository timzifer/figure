package three

import (
	"errors"
	"math"
	"math/cmplx"
	"strconv"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// ErrNotSpherical reports a layer asked to draw in a spherical scene that has
// no meaning there: a field of bars standing on a floor, or a contour traced on
// one, where a sphere has no floor.
var ErrNotSpherical = errors.New("figure/three: this layer has no meaning in a spherical scene")

// SphereOption configures a spherical scene. See [Spherical].
type SphereOption func(*sphere)

// Spherical makes a scene's three scales a direction and a radius rather than
// three distances along the edges of a box.
//
//	sc := three.NewScene(three.Spherical(three.AxisEnds("x", "", "y", "", "z", "")))
//	sc.Add(three.Surface(pattern, geom.X("phi"), geom.Y("theta"), geom.Z("gain")))
//
// X is the azimuth, a full turn round the pole; Y is the polar angle from the
// north pole down to the south one, or, with [Latitude], the angle up from the
// equator; Z is the distance from the centre. Both angles are in degrees
// unless [Radians] says otherwise, and their domains are pinned to the whole
// sphere whatever the data covers — an azimuth scale trained on readings from
// 0° to 355° would otherwise stretch them round the full circle and close the
// five-degree gap by lying about every other angle. The radius runs from zero
// at the centre, so a radius is a length the reader can compare.
//
// It is the third coordinate system figure has, and it is here for the reason
// coord.Polar is: one piece of machinery, a whole family of charts. A
// [Surface] over (azimuth, polar angle) with the radius as its value is an
// antenna radiation pattern; a [Scatter3] of unit radii is a Bloch sphere's
// states or a Poincaré sphere's polarisations, and a [Line3] through them is a
// state or a polarisation moving; a scatter of directions is a stereonet's
// data before it was projected flat. See docs/adr/0058-what-3d-is-for.md.
//
// The furniture is a globe rather than a cube: a graticule every thirty
// degrees, the silhouette, the three axes through the centre and the labels
// [AxisEnds] puts on them. The far half is drawn before the data and the near
// half after it, which is exact rather than approximate for the reason the
// cube's back walls are: every datum is inside the ball, and a point inside a
// ball is in front of the far hemisphere and behind the near one along every
// ray through it.
//
// A [Bar3] or a [Contour] in a spherical scene is [ErrNotSpherical]: both
// stand on a floor, and a sphere has none.
func Spherical(opts ...SphereOption) SceneOption {
	sp := &sphere{}
	for _, o := range opts {
		o(sp)
	}
	if sp.smith && sp.ends == ([6]string{}) {
		// A Smith sphere says where its landmarks are unless told otherwise:
		// the open and the short on the real axis, the match at the top.
		sp.ends = [6]string{"open", "short", "", "", "match", ""}
	}
	return func(sc *Scene) { sc.sphere = sp }
}

// Radians reads both angles in radians rather than degrees: the azimuth over
// [0, 2π) and the polar angle over [0, π] — or the latitude over [−π/2, π/2].
func Radians() SphereOption { return func(sp *sphere) { sp.radians = true } }

// Latitude reads the second angle as the angle above the equator, from −90°
// at the south pole to +90° at the north one, rather than as the polar angle
// down from the north pole. It is the convention of a globe, a
// stereonet's plunge and a Poincaré sphere's 2χ; the polar angle is the
// convention of an antenna pattern's θ and a Bloch sphere's.
func Latitude() SphereOption { return func(sp *sphere) { sp.elevation = true } }

// AxisEnds labels the six ends of the three axes through the centre: +x, −x,
// +y, −y, +z and −z, in that order. An empty label leaves its end bare.
//
// It is how the sphere says what it is. A Bloch sphere writes |0⟩ and |1⟩ at
// the poles and |+⟩ and |+i⟩ on the equator; a Poincaré sphere writes S₁, S₂
// and S₃; an antenna pattern writes x, y and z. With no labels at all the axes
// are still drawn, because a direction is read against them.
func AxisEnds(px, nx, py, ny, pz, nz string) SphereOption {
	return func(sp *sphere) { sp.ends = [6]string{px, nx, py, ny, pz, nz} }
}

// Smith makes a spherical scene the three-dimensional Smith chart: X is a
// normalised resistance and Y a normalised reactance, and each impedance is
// placed where its reflection coefficient Γ = (z − 1)/(z + 1) lies on the
// Riemann sphere of the Γ-plane.
//
//	sc := three.NewScene(three.Spherical(three.Smith()))
//	sc.Add(three.Line3(sweep, geom.X("r"), geom.Y("x"), geom.Z("one")))
//
// The flat Smith chart is the unit disc of that plane, and everything with
// |Γ| > 1 — a negative resistance, an oscillator, the unstable region of an
// amplifier — is off the page. On the sphere the disc is the northern
// hemisphere, the unit circle is the equator, and the outside of the disc is
// the southern hemisphere: the infinite plane becomes finite, and the one class
// of circuit the flat chart cannot show becomes a place a reader can point at.
// The match, Γ = 0, is the north pole; the open circuit, Γ = 1, is on the
// equator at +x and the short, Γ = −1, opposite it; Γ = ∞, z = −1, is the south
// pole.
//
// The grid is the flat chart's, carried onto the sphere: the circles of
// constant resistance and of constant reactance, both whole — a
// constant-resistance circle of a negative resistance lies in the south,
// touching the equator only at the open circuit — and all of them still pass
// through the open circuit, because a stereographic projection takes circles
// to circles. The resistance circles
// are labelled where they cross the real axis and the reactance circles where
// they cross the equator, on the half of the ball facing the reader.
//
// It is here for the reason docs/adr/0058-what-3d-is-for.md gives: it is the
// test that the spherical scene is a coordinate system rather than a chart
// type. The azimuth of an impedance is the phase of Γ and its polar angle is
// 2·atan|Γ|, so a Smith sphere is the spherical scene with a different reading
// of its first two values and a different grid, and nothing else.
func Smith() SphereOption { return func(sp *sphere) { sp.smith = true } }

// sphere is a spherical scene's configuration.
type sphere struct {
	radians, elevation bool
	smith              bool
	ends               [6]string
}

// smithResistances and smithReactances are the circles a Smith sphere draws:
// the flat chart's printed values, and the same resistances negated for the
// southern hemisphere a flat chart has no room for.
var (
	smithResistances = []float64{0, 0.2, 0.5, 1, 2, 5, -0.2, -0.5, -2, -5}
	smithReactances  = []float64{0.2, 0.5, 1, 2, 5, -0.2, -0.5, -1, -2, -5}
)

// smithSamples is how many segments a Smith grid circle is drawn with. The
// circles are sampled evenly in the Γ-plane, where a circle of a small
// negative resistance is very large and lands on the sphere very small, so
// they get more than a graticule circle does.
const smithSamples = 180

// smithPoint places the impedance r + jx at radius w of the ball.
func smithPoint(r, x float64, w float32) Vec3 {
	return riemann(gamma(r, x), float64(w))
}

// gamma is the reflection coefficient of the normalised impedance r + jx.
// z = −1 is the one impedance whose Γ is infinite, and it is returned as such.
func gamma(r, x float64) complex128 {
	z := complex(r, x)
	if z == -1 {
		return cmplx.Inf()
	}
	return (z - 1) / (z + 1)
}

// riemann places a point of the Γ-plane on the Riemann sphere of radius w
// times the ball's: the inverse stereographic projection from the south pole,
// which puts Γ = 0 at the north pole, the unit circle on the equator and Γ = ∞
// at the south pole. The polar angle is 2·atan|Γ| and the azimuth is Γ's phase.
func riemann(g complex128, w float64) Vec3 {
	polar := math.Pi
	if !cmplx.IsInf(g) && !cmplx.IsNaN(g) {
		polar = 2 * math.Atan(cmplx.Abs(g))
	}
	az := 0.0
	if polar < math.Pi {
		az = cmplx.Phase(g)
	}
	r := radius * w
	sn := math.Sin(polar)
	return Vec3{
		X: centre.X + float32(r*sn*math.Cos(az)),
		Y: centre.Y + float32(r*sn*math.Sin(az)),
		Z: centre.Z + float32(r*math.Cos(polar)),
	}
}

// radius is how much of the unit cube the sphere of radius one fills: all of
// it, touching the middle of each face.
const radius = 0.5

// sphereFit is how much larger a spherical scene is drawn than a box would be
// in the same cell. A box is fitted by its main diagonal, √3, so that it keeps
// its size while it turns; a ball's every diameter is 1, so fitting it the same
// way would leave it filling little more than half its cell. The tenth left
// over is the room its axis labels stand in beyond the silhouette.
var sphereFit = diameter / 1.1

// pin fixes the angular domains to the whole sphere and trains the radius on
// zero, before and after the layers train respectively. It reports an error
// for an angular scale that cannot be pinned, which is an ordinal one: half a
// category is not an angle.
func (sp *sphere) pin(sc [3]scale.Scale) error {
	if sp.smith {
		// A resistance and a reactance are not angles and run to infinity;
		// their scales keep whatever the data trained, and nothing reads the
		// interval they map into.
		return nil
	}
	turn, half := 360.0, 180.0
	if sp.radians {
		turn, half = 2*math.Pi, math.Pi
	}
	lo, hi := 0.0, half
	if sp.elevation {
		lo, hi = -half/2, half/2
	}
	az, ok := sc[axisX].(scale.Zoomer)
	pol, ok2 := sc[axisY].(scale.Zoomer)
	if !ok || !ok2 {
		return errors.New("figure/three: a spherical scene's two angles need continuous scales")
	}
	az.SetDomain(0, turn)
	pol.SetDomain(lo, hi)
	return nil
}

// place turns three positions in [0, 1] — azimuth, polar angle or elevation,
// and radius, as the scales mapped them — into a scene point.
func (sp *sphere) place(u, v, w float32) Vec3 {
	az := 2 * math.Pi * float64(u)
	var polar float64
	if sp.elevation {
		polar = math.Pi * (1 - float64(v))
	} else {
		polar = math.Pi * float64(v)
	}
	r := radius * float64(w)
	s := math.Sin(polar)
	return Vec3{
		X: centre.X + float32(r*s*math.Cos(az)),
		Y: centre.Y + float32(r*s*math.Sin(az)),
		Z: centre.Z + float32(r*math.Cos(polar)),
	}
}

// Point turns one row's three values into a point in the scene, through the
// frame's scales and the scene's space.
//
// In an ordinary scene each value is mapped by its scale onto an edge of the
// unit cube. In a [Prism] the first two are two components of a composition
// and the point is over the floor triangle. In a [Spherical] one they are an
// azimuth, a polar angle and a radius, and the point is on or inside the ball. Under [Smith] the first two
// are a normalised resistance and reactance, placed where their reflection
// coefficient lies on the Riemann sphere — which is why this takes the values
// rather than positions a scale already mapped: a reactance runs to infinity
// both ways, and no interval a scale maps into can hold it.
//
// A layer that places its geometry through this draws in all three, which is
// the whole of what a layer needs to know about which space it is in.
func (f Frame) Point(x, y, z float64) Vec3 {
	switch {
	case f.floor != nil:
		return f.floor.place(at(f.X, x), at(f.Y, y), at(f.Z, z))
	case f.space == nil:
		return Vec3{at(f.X, x), at(f.Y, y), at(f.Z, z)}
	case f.space.smith:
		return smithPoint(x, y, at(f.Z, z))
	}
	// The azimuth is mapped without clamping, so that a surface closing round
	// the turn can name the column a full turn past its first.
	return f.space.place(f.X.Map(x), at(f.Y, y), at(f.Z, z))
}

// Place turns three positions already in [0, 1] into a point in the scene: the
// identity in a box, a point over the floor triangle in a [Prism], and a
// direction and a radius on the ball in a [Spherical] scene.
//
// It is [Frame.Point]'s counterpart for a layer whose geometry is not a row.
// A histogram's cell edge, a graticule's corner and a wall's midpoint are
// positions the layer worked out itself rather than values a scale has to map,
// and putting them through Point would mean inverting the scale first. Under
// [Smith] the two angles are still angles here, because a position is one —
// the impedance reading belongs to Point, which takes values.
func (f Frame) Place(u, v, w float32) Vec3 {
	switch {
	case f.floor != nil:
		return f.floor.place(u, v, w)
	case f.space == nil:
		return Vec3{u, v, w}
	}
	return f.space.place(u, v, w)
}

// Spherical reports whether the frame is of a [Spherical] scene, for a layer
// that has no meaning in one to say so with [ErrNotSpherical].
func (f Frame) Spherical() bool { return f.space != nil }

// globe is the furniture of one view of a spherical scene: the far half drawn
// before the data, the near half after it.
type globe struct {
	th   theme.Theme
	proj projector
	fwd  Vec3
	sp   *sphere
}

func newGlobe(th theme.Theme, pr projector, cam Camera, sp *sphere) globe {
	return globe{th: th, proj: pr, fwd: cam.Forward(), sp: sp}
}

// graticuleStep is the spacing of the graticule, and arcSamples how finely one
// of its circles is drawn: five degrees a segment, which is finer than a
// reader can see a polygon in at any size a figure is drawn at.
const (
	graticuleStep = 30
	arcSamples    = 72
	nearOpacity   = 0.35
)

// back fills the disc, strokes the far half of the graticule and of the axes,
// and outlines the silhouette.
func (g globe) back(b ir.Backend, path *ir.Path) {
	if g.th.CubeFill.A != 0 {
		path.Reset()
		g.silhouette(path)
		b.FillPath(path, ir.Solid(g.th.CubeFill), ir.NonZero)
	}
	g.lines(b, path, true)
	if g.th.CubeEdge.A != 0 {
		path.Reset()
		g.silhouette(path)
		b.StrokePath(path, ir.Stroke{Color: g.th.CubeEdge, Width: pickWidth(g.th.AxisWidth)})
	}
}

// front strokes the near half of the graticule and of the axes, fainter so the
// data under them stays the thing being read, and writes the axis labels.
func (g globe) front(b ir.Backend, path *ir.Path) {
	g.lines(b, path, false)
	if g.sp.smith {
		g.smithLabels(b)
	}
	g.labels(b)
}

// silhouette appends the outline of the ball as seen from the camera: the
// circle of radius 0.5 in the plane square to the view.
func (g globe) silhouette(path *ir.Path) {
	u, v := g.proj.right, g.proj.up
	for k := 0; k <= arcSamples; k++ {
		a := 2 * math.Pi * float64(k) / arcSamples
		p := centre.Add(u.Mul(float32(radius * math.Cos(a)))).Add(v.Mul(float32(radius * math.Sin(a))))
		q := g.proj.point(p)
		if k == 0 {
			path.MoveTo(q.X, q.Y)
			continue
		}
		path.LineTo(q.X, q.Y)
	}
	path.Close()
}

// lines strokes the graticule and the axes on one side of the ball: the far
// side when far is set, the near side otherwise. A segment belongs to the side
// its middle is on.
func (g globe) lines(b ir.Backend, path *ir.Path, far bool) {
	grid, axis := g.th.CubeGrid, g.th.AxisColor
	if !far {
		grid, axis = ir.Fade(grid, nearOpacity), ir.Fade(axis, nearOpacity)
	}
	if grid.A != 0 && g.sp.smith {
		g.smithGrid(b, path, far, grid)
	} else if grid.A != 0 {
		path.Reset()
		n := 0
		// Meridians, pole to pole.
		for d := 0; d < 360; d += graticuleStep {
			az := float64(d) * math.Pi / 180
			n += g.arc(path, far, func(t float64) Vec3 { return onSphere(az, math.Pi*t) }, arcSamples/2)
		}
		// Parallels, between the poles.
		for d := graticuleStep; d < 180; d += graticuleStep {
			polar := float64(d) * math.Pi / 180
			n += g.arc(path, far, func(t float64) Vec3 { return onSphere(2*math.Pi*t, polar) }, arcSamples)
		}
		if n > 0 {
			b.StrokePath(path, ir.Stroke{Color: grid, Width: pickWidth(g.th.GridWidth), Dash: g.th.GridDash})
		}
	}
	if axis.A != 0 {
		path.Reset()
		n := 0
		for a := 0; a < 3; a++ {
			end := Vec3{}.with(a, radius)
			for _, sign := range [2]float32{1, -1} {
				tip := centre.Add(end.Mul(sign))
				mid := centre.Add(end.Mul(sign / 2))
				if (g.depth(mid) > 0) != far {
					continue
				}
				p, q := g.proj.point(centre), g.proj.point(tip)
				path.MoveTo(p.X, p.Y)
				path.LineTo(q.X, q.Y)
				n++
			}
		}
		if n > 0 {
			b.StrokePath(path, ir.Stroke{Color: axis, Width: pickWidth(g.th.AxisWidth)})
		}
	}
}

// arc appends the segments of one curve on the ball that lie on the requested
// side, sampled steps times over t in [0, 1], and reports how many it added.
func (g globe) arc(path *ir.Path, far bool, at func(t float64) Vec3, steps int) int {
	n := 0
	prev := at(0)
	open := false
	for k := 1; k <= steps; k++ {
		next := at(float64(k) / float64(steps))
		mid := prev.Add(next).Mul(0.5)
		if (g.depth(mid) > 0) == far {
			p, q := g.proj.point(prev), g.proj.point(next)
			if !open {
				path.MoveTo(p.X, p.Y)
				open = true
			}
			path.LineTo(q.X, q.Y)
			n++
		} else {
			open = false
		}
		prev = next
	}
	return n
}

// smithGrid strokes the circles of constant resistance and of constant
// reactance on one side of the ball, and the real axis, which is the circle of
// zero reactance.
func (g globe) smithGrid(b ir.Backend, path *ir.Path, far bool, col ir.Color) {
	path.Reset()
	n := 0
	for _, r := range smithResistances {
		// The circle of constant r in the Γ-plane: centre r/(r+1) on the real
		// axis, radius 1/|r+1|. Every one passes through Γ = 1.
		c, a := r/(r+1), 1/math.Abs(r+1)
		n += g.arc(path, far, func(t float64) Vec3 {
			return riemann(complex(c, 0)+cmplx.Rect(a, 2*math.Pi*t), 1)
		}, smithSamples)
	}
	for _, x := range smithReactances {
		// The circle of constant x: centre 1 + j/x, radius 1/|x|.
		c, a := complex(1, 1/x), 1/math.Abs(x)
		n += g.arc(path, far, func(t float64) Vec3 {
			return riemann(c+cmplx.Rect(a, 2*math.Pi*t), 1)
		}, smithSamples)
	}
	// The real axis of the Γ-plane is a great circle through both poles.
	n += g.arc(path, far, func(t float64) Vec3 { return onSphere(0, 2*math.Pi*t) }, arcSamples)
	if n > 0 {
		b.StrokePath(path, ir.Stroke{Color: col, Width: pickWidth(g.th.GridWidth), Dash: g.th.GridDash})
	}
}

// smithLabels writes each grid circle's value where a flat Smith chart prints
// it — a resistance where its circle crosses the real axis, a reactance where
// its circle crosses the unit circle — on the half of the ball facing the
// reader, where it can be read against the curve it names.
func (g globe) smithLabels(b ir.Backend) {
	font := g.th.Font(g.th.TickSize)
	put := func(at Vec3, v float64) {
		if g.depth(at) > 0 {
			return
		}
		p := g.proj.point(at)
		b.Text(ir.TextRun{
			Text:  strconv.FormatFloat(v, 'g', -1, 64),
			Font:  font,
			At:    ir.Point{X: p.X + g.th.TickLabelPad, Y: p.Y - g.th.TickLabelPad},
			V:     ir.AlignBottom,
			Color: g.th.TickColor,
		})
	}
	for _, r := range smithResistances {
		// r = 1 crosses the real axis at the match and r = 0 at the short,
		// which are landmarks the axis ends already name.
		if r == 0 || r == 1 {
			continue
		}
		put(riemann(complex((r-1)/(r+1), 0), 1), r)
	}
	for _, x := range smithReactances {
		put(riemann(gamma(0, x), 1), x)
	}
}

// depth is how far a point is beyond the centre along the view direction:
// positive on the far half of the ball.
func (g globe) depth(v Vec3) float64 { return v.Sub(centre).Dot(g.fwd) }

// labels writes the axis ends, just beyond the tips, pushed outward from the
// centre of the projected ball so that none sits on its own axis line.
func (g globe) labels(b ir.Backend) {
	font := g.th.Font(g.th.LabelSize)
	mid := g.proj.point(centre)
	pad := g.th.TickLabelPad + g.th.TickLength
	for k, text := range g.sp.ends {
		if text == "" {
			continue
		}
		a, sign := k/2, float32(1)
		if k%2 == 1 {
			sign = -1
		}
		tip := g.proj.point(centre.Add(Vec3{}.with(a, radius*sign)))
		dx, dy := tip.X-mid.X, tip.Y-mid.Y
		if l := float32(math.Hypot(float64(dx), float64(dy))); l > 0 {
			dx, dy = dx/l, dy/l
		}
		h, v := alignFor(ir.Point{X: dx, Y: dy})
		b.Text(ir.TextRun{
			Text:  text,
			Font:  font,
			At:    ir.Point{X: tip.X + dx*pad, Y: tip.Y + dy*pad},
			H:     h,
			V:     v,
			Color: g.th.LabelColor,
		})
	}
}

// onSphere is the point of the unit ball's surface at an azimuth and a polar
// angle, in scene space.
func onSphere(az, polar float64) Vec3 {
	s := math.Sin(polar)
	return Vec3{
		X: centre.X + float32(radius*s*math.Cos(az)),
		Y: centre.Y + float32(radius*s*math.Sin(az)),
		Z: centre.Z + float32(radius*math.Cos(polar)),
	}
}

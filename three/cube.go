package three

import (
	"math"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// The three axes, in the order everything in this file indexes them.
const (
	axisX = iota
	axisY
	axisZ
)

// cube is the furniture of one view: the three faces of the box that point
// away from the camera, the grid on them, the ticks along their outer edges
// and the axis titles beyond those.
//
// It is drawn before the data and never enters the depth order, and that is a
// proof rather than a convention: every datum lies inside the box, and these
// three faces are the extreme faces on the far side of each axis, so every
// datum is in front of all three. Which three they are is recomputed every
// frame from the sign of the view direction, which is what keeps the picture
// readable as ADR 0057 turns it.
type cube struct {
	th    theme.Theme
	proj  projector
	fwd   Vec3
	far   [3]float32
	ticks [3][]scale.Tick
	title [3]string
	// mid is the projected middle of the box, which every label is placed
	// outward from.
	mid ir.Point
}

func newCube(th theme.Theme, pr projector, cam Camera, ticks [3][]scale.Tick, titles [3]string) cube {
	c := cube{th: th, proj: pr, fwd: cam.Forward(), ticks: ticks, title: titles, mid: pr.point(centre)}
	for a := range c.far {
		// The far side of an axis is the one the view direction points
		// toward: moving along it increases depth. With the camera above the
		// floor this makes the floor a "far" face in z, which is 0056's two
		// back walls and a floor falling out of the arithmetic rather than
		// being asserted.
		if c.fwd.at(a) > 0 {
			c.far[a] = 1
		}
	}
	return c
}

// ticksOf asks the three scales for their ticks, once per scene per frame.
//
// Once, rather than once per view and again for the measurement, because a
// tick is a string and a chart redrawn on every pointer move should not build
// the same three sets of them five times. It also makes the labels that are
// measured exactly the labels that are drawn, which is the kind of agreement
// that is easy to lose and hard to notice losing.
//
// The two floor axes take the horizontal hint and the depth axis the vertical
// one, because that is what each of them is: z is the up axis of the scene and
// asks for the density a vertical axis asks for. No theme field is added for a
// third count, because there is no third kind of axis here.
func ticksOf(th theme.Theme, sc [3]scale.Scale) [3][]scale.Tick {
	want := [3]int{th.TickCountHintX, th.TickCountHintX, th.TickCountHintY}
	var out [3][]scale.Tick
	for a, s := range sc {
		if s != nil {
			out[a] = s.Ticks(scale.TickRequest{Want: want[a]})
		}
	}
	return out
}

// at reads one component of a vector by axis index.
func (v Vec3) at(a int) float32 {
	switch a {
	case axisX:
		return v.X
	case axisY:
		return v.Y
	}
	return v.Z
}

// with returns v with one component replaced.
func (v Vec3) with(a int, f float32) Vec3 {
	switch a {
	case axisX:
		v.X = f
	case axisY:
		v.Y = f
	default:
		v.Z = f
	}
	return v
}

// corner builds a cube corner from the three axis positions.
func corner(x, y, z float32) Vec3 { return Vec3{x, y, z} }

// draw paints the walls, the grid, the ticks and the titles, in that order.
func (c cube) draw(b ir.Backend, path *ir.Path) {
	c.walls(b, path)
	c.grid(b, path)
	c.axes(b, path)
}

// walls fills the three faces pointing away from the camera and outlines them.
func (c cube) walls(b ir.Backend, path *ir.Path) {
	path.Reset()
	for a := 0; a < 3; a++ {
		pts := c.faceCorners(a, c.far[a])
		path.MoveTo(pts[0].X, pts[0].Y)
		for _, p := range pts[1:] {
			path.LineTo(p.X, p.Y)
		}
		path.Close()
	}
	if c.th.CubeFill.A != 0 {
		b.FillPath(path, ir.Solid(c.th.CubeFill), ir.NonZero)
	}
	if c.th.CubeEdge.A != 0 {
		b.StrokePath(path, ir.Stroke{Color: c.th.CubeEdge, Width: pickWidth(c.th.AxisWidth)})
	}
}

// faceCorners projects the four corners of the face perpendicular to axis a at
// the given side, in order around it.
func (c cube) faceCorners(a int, side float32) [4]ir.Point {
	b1, b2 := others(a)
	var out [4]ir.Point
	for i, uv := range [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		v := corner(0, 0, 0).with(a, side).with(b1, uv[0]).with(b2, uv[1])
		out[i] = c.proj.point(v)
	}
	return out
}

// grid strokes one line per tick on each of the two faces that axis lies in.
//
// A tick of the depth axis therefore appears on both back walls, which is what
// makes a height readable against either of them; a tick of a floor axis
// appears on the floor and on one wall.
func (c cube) grid(b ir.Backend, path *ir.Path) {
	if c.th.CubeGrid.A == 0 {
		return
	}
	path.Reset()
	n := 0
	for a := 0; a < 3; a++ {
		for _, t := range c.ticks[a] {
			p := clamp01(t.Pos)
			for _, face := range others2(a) {
				// The line runs across the face along the third axis, at the
				// tick's position along a and on the face's own side.
				run := third(a, face)
				from := corner(0, 0, 0).with(a, p).with(face, c.far[face]).with(run, 0)
				to := from.with(run, 1)
				f, t2 := c.proj.point(from), c.proj.point(to)
				path.MoveTo(f.X, f.Y)
				path.LineTo(t2.X, t2.Y)
				n++
			}
		}
	}
	if n > 0 {
		b.StrokePath(path, ir.Stroke{Color: c.th.CubeGrid, Width: pickWidth(c.th.GridWidth), Dash: c.th.GridDash})
	}
}

// axes strokes each axis's outer edge, its tick marks, its labels and its
// title.
func (c cube) axes(b ir.Backend, path *ir.Path) {
	tickFont := c.th.Font(c.th.TickSize)
	titleFont := c.th.Font(c.th.LabelSize)
	for a := 0; a < 3; a++ {
		if len(c.ticks[a]) == 0 {
			continue
		}
		e := c.edgeOf(a)
		from, to := c.proj.point(e.at(0)), c.proj.point(e.at(1))

		path.Reset()
		path.MoveTo(from.X, from.Y)
		path.LineTo(to.X, to.Y)
		for _, t := range c.ticks[a] {
			anchor := c.proj.point(e.at(clamp01(t.Pos)))
			out := c.outward(anchor)
			reach := c.th.TickLength
			if t.Minor {
				reach /= 2
			}
			path.MoveTo(anchor.X, anchor.Y)
			path.LineTo(anchor.X+out.X*reach, anchor.Y+out.Y*reach)
		}
		if c.th.CubeEdge.A != 0 {
			b.StrokePath(path, ir.Stroke{Color: c.th.AxisColor, Width: pickWidth(c.th.AxisWidth)})
		}

		for _, t := range c.ticks[a] {
			if t.Label == "" {
				continue
			}
			anchor := c.proj.point(e.at(clamp01(t.Pos)))
			out := c.outward(anchor)
			gap := c.th.TickLength + c.th.TickLabelPad
			h, v := alignFor(out)
			// The label is upright at a projected anchor and is never
			// sheared: ir.TextRun carries one rotation about its anchor and no
			// shear, deliberately, and a sheared tick label is harder to read
			// than an upright one anyway.
			b.Text(ir.TextRun{
				Text:  t.Label,
				Font:  tickFont,
				At:    ir.Point{X: anchor.X + out.X*gap, Y: anchor.Y + out.Y*gap},
				H:     h,
				V:     v,
				Color: c.th.TickColor,
			})
		}

		if c.title[a] == "" {
			continue
		}
		anchor := c.proj.point(e.at(0.5))
		out := c.outward(anchor)
		gap := c.th.TickLength + c.th.TickLabelPad + c.th.AxisTitlePad + float32(c.th.TickSize)*1.6
		// The title takes the screen angle of its own projected axis, which is
		// a rotation about the anchor and therefore something ir.TextRun can
		// express. Anything past a quarter turn is folded back so that the
		// title never reads upside down.
		b.Text(ir.TextRun{
			Text:     c.title[a],
			Font:     titleFont,
			At:       ir.Point{X: anchor.X + out.X*gap, Y: anchor.Y + out.Y*gap},
			H:        ir.AlignCenter,
			V:        ir.AlignMiddle,
			Rotation: uprightAngle(to.X-from.X, to.Y-from.Y),
			Color:    c.th.LabelColor,
		})
	}
}

// edge is the segment of the cube an axis writes its ticks along.
type edge struct {
	axis   int
	anchor Vec3
}

func (e edge) at(p float32) Vec3 { return e.anchor.with(e.axis, p) }

// edgeOf picks which of the cube's four parallel edges an axis is labelled
// along.
//
// A floor axis takes the one on the floor at the near side of the other floor
// axis, so its numbers sit at the front of the box where nothing is drawn over
// them. The depth axis takes whichever of the four verticals projects
// furthest left, which is on the box's silhouette by construction — so its
// numbers are outside the walls rather than written across them.
func (c cube) edgeOf(a int) edge {
	if a != axisZ {
		other := axisX
		if a == axisX {
			other = axisY
		}
		return edge{axis: a, anchor: corner(0, 0, 0).
			with(other, 1-c.far[other]).
			with(axisZ, c.far[axisZ])}
	}
	best := edge{axis: axisZ, anchor: corner(0, 0, 0)}
	bestX := float32(math.Inf(1))
	for _, x := range []float32{0, 1} {
		for _, y := range []float32{0, 1} {
			e := edge{axis: axisZ, anchor: Vec3{x, y, 0}}
			if px := c.proj.point(e.at(0.5)).X; px < bestX {
				best, bestX = e, px
			}
		}
	}
	return best
}

// outward is the unit screen direction from the middle of the projected box
// toward a point on its edge, which is where a label goes.
func (c cube) outward(at ir.Point) ir.Point {
	dx, dy := at.X-c.mid.X, at.Y-c.mid.Y
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n == 0 {
		return ir.Point{X: 0, Y: 1}
	}
	return ir.Point{X: dx / n, Y: dy / n}
}

// alignFor anchors a label on the side of its position the box is not on.
func alignFor(out ir.Point) (ir.HAlign, ir.VAlign) {
	if abs32(out.X) > abs32(out.Y) {
		if out.X < 0 {
			return ir.AlignEnd, ir.AlignMiddle
		}
		return ir.AlignStart, ir.AlignMiddle
	}
	// Device y grows downward, so a label below the box is anchored by its
	// top.
	if out.Y > 0 {
		return ir.AlignCenter, ir.AlignTop
	}
	return ir.AlignCenter, ir.AlignBottom
}

// uprightAngle is the screen angle of a direction, folded into the half turn
// that reads left to right.
func uprightAngle(dx, dy float32) float64 {
	a := math.Atan2(float64(dy), float64(dx))
	for a > math.Pi/2 {
		a -= math.Pi
	}
	for a <= -math.Pi/2 {
		a += math.Pi
	}
	return a
}

// others returns the two axes an axis is not.
func others(a int) (int, int) {
	switch a {
	case axisX:
		return axisY, axisZ
	case axisY:
		return axisX, axisZ
	}
	return axisX, axisY
}

// others2 is [others] as a slice, for the grid's loop.
func others2(a int) [2]int { b, c := others(a); return [2]int{b, c} }

// third names the axis that is neither a nor b.
func third(a, b int) int {
	return 3 - a - b
}

func clamp01(f float32) float32 {
	switch {
	case f < 0:
		return 0
	case f > 1:
		return 1
	}
	return f
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

// pickWidth keeps a hand-built theme from asking for a hairline nobody can
// see.
func pickWidth(w float32) float32 {
	if w <= 0 {
		return 1
	}
	return w
}

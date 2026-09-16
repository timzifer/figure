package three

import (
	"errors"
	"math"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// ErrNotPrismatic reports a layer asked to draw in a ternary prism that has no
// meaning there: one that stands on a rectangular floor, or that closes round
// an azimuth a composition does not have.
var ErrNotPrismatic = errors.New("figure/three: this layer has no meaning in a ternary prism")

// PrismOption configures a ternary prism. See [Prism].
type PrismOption func(*prism)

// Prism makes a scene's floor a ternary diagram and its up axis a fourth
// variable, which is the chart metallurgy and petrology draw when a
// composition's reading depends on one more number: temperature, pressure,
// depth, time.
//
//	sc := three.NewScene(three.Prism(three.PrismCorners("Fe", "Cr", "Ni")))
//	sc.Add(three.Scatter3(alloys, geom.X("fe"), geom.Y("cr"), geom.Z("degC")))
//
// X is the first component of the composition and Y the second; the third is
// Sum − x − y, exactly as [github.com/timzifer/figure/coord.Ternary] derives
// it, and for the same two reasons — a third column has nowhere to enter a
// stage that transforms a pair, and deriving it makes the constraint hold by
// construction rather than raising a normalise-or-refuse question about every
// row. Z is the fourth variable and is an ordinary depth axis.
//
// It is docs/adr/0058-what-3d-is-for.md's rank 4, beside the Smith sphere and
// at the same distance from the mainstream, and it is reachable by the same
// machinery: a scene option that places a layer's values through a different
// map, the way [Spherical] places them on the ball. Every layer that draws in
// a box draws here unchanged, because a layer places its geometry through
// [Frame.Point] and that is all it has to know about which space it is in.
//
// The two component domains are pinned to the whole simplex whatever the data
// does, for the reason a spherical scene's angles are pinned to the whole
// sphere: the chart's extent is a fact about the coordinate system, and an
// axis that autoscaled to a tight cluster of compositions would draw a
// triangle that is not one.
//
// The furniture is a triangular prism rather than a cube — the three faces
// pointing away from the camera, the ternary grid on the floor, the fourth
// variable's ladder up one vertical edge, and [PrismCorners] naming the three
// components at the corners they belong to. Naming them there rather than
// along a third numeric ladder is what a ternary chart does anyway: the three
// ladders read the same sequence in the same direction, so the numbers on the
// third one are redundant with the other two.
func Prism(opts ...PrismOption) SceneOption {
	p := &prism{sum: 1}
	for _, o := range opts {
		o(p)
	}
	return func(sc *Scene) { sc.prism = p }
}

// PrismSum sets what the three components add up to. The default is 1, and 100
// is the percentage spelling most tables in these fields come in.
func PrismSum(k float64) PrismOption {
	return func(p *prism) {
		if k > 0 {
			p.sum = k
		}
	}
}

// PrismCorners names the three components at the corners they belong to: the
// first component's corner, the second's, and the derived third's.
//
// It is how the prism says what it is, as [AxisEnds] is for a sphere. An empty
// name leaves its corner bare.
func PrismCorners(a, b, c string) PrismOption {
	return func(p *prism) { p.corners = [3]string{a, b, c} }
}

// prism is a ternary prism's configuration.
type prism struct {
	sum     float64
	corners [3]string
}

// The three corners of the floor triangle, in scene space: the corner where
// the first component is everything, where the second is, and where the
// derived third is.
//
// The triangle is the largest equilateral one inscribed in the unit square,
// centred in it, so a prism fills its cell the way a cube does and the two are
// drawn at the same scale.
var prismFloor = func() [3]Vec3 {
	h := float32(math.Sqrt(3) / 2)
	lo := (1 - h) / 2
	return [3]Vec3{{0, lo, 0}, {1, lo, 0}, {0.5, lo + h, 0}}
}()

// pin fixes the two component domains to the whole simplex, before anything
// trains them. It reports an error for a scale that cannot be pinned, which is
// an ordinal one: half a category is not a component.
func (p *prism) pin(sc [3]scale.Scale) error {
	a, ok := sc[axisX].(scale.Zoomer)
	b, ok2 := sc[axisY].(scale.Zoomer)
	if !ok || !ok2 {
		return errors.New("figure/three: a ternary prism's two components need continuous scales")
	}
	a.SetDomain(0, p.sum)
	b.SetDomain(0, p.sum)
	return nil
}

// place turns two component fractions and a height, each in [0, 1] as the
// scales mapped them, into a scene point: the barycentric mean of the floor's
// three corners, lifted to the height.
func (p *prism) place(u, v, w float32) Vec3 {
	c := 1 - u - v
	f := prismFloor[0].Mul(u).Add(prismFloor[1].Mul(v)).Add(prismFloor[2].Mul(c))
	return Vec3{f.X, f.Y, w}
}

// Prismatic reports whether the frame is of a [Prism] scene, for a layer that
// has no meaning in one to say so with [ErrNotPrismatic].
func (f Frame) Prismatic() bool { return f.floor != nil }

// prismFurniture is the furniture of one view of a ternary prism: the three faces
// pointing away from the camera, the grid on them, the ladders along their
// outer edges and the corner names beyond those.
//
// It is drawn before the data and never enters the depth order, for the
// reason a [cube]'s back walls are: every datum is inside the prism, and these
// are the extreme faces on the far side of it, so every datum is in front of
// all three.
type prismFurniture struct {
	th    theme.Theme
	proj  projector
	fwd   Vec3
	p     *prism
	ticks [3][]scale.Tick
	title [3]string
	mid   ir.Point
	// floor is which horizontal face is on the far side of the camera: the
	// floor at 0 for a camera above the prism, the roof at 1 for one below
	// it. It is [cube]'s far side of an axis, worked out the same way and for
	// the same reason — every datum is in front of it.
	floor float32
	// far reports, per side wall, whether it points away from the camera.
	far [3]bool
}

func newPrism(th theme.Theme, pr projector, cam Camera, p *prism, ticks [3][]scale.Tick, titles [3]string) prismFurniture {
	f := prismFurniture{
		th: th, proj: pr, fwd: cam.Forward(), p: p,
		ticks: ticks, title: titles, mid: pr.point(centre),
	}
	if f.fwd.Z > 0 {
		f.floor = 1
	}
	for k := range f.far {
		// A side wall is on the far side when its outward normal points the
		// way the camera looks. The wall is vertical, so the normal is the
		// floor edge turned a quarter turn in the floor plane, pointed away
		// from the third corner.
		f.far[k] = f.wallNormal(k).Dot(f.fwd) > 0
	}
	return f
}

// wallEdge is the two floor corners side wall k stands on: the wall opposite
// corner k, which is the one where component k is nothing.
func wallEdge(k int) (int, int) { return (k + 1) % 3, (k + 2) % 3 }

// wallNormal is the outward horizontal normal of side wall k.
func (f prismFurniture) wallNormal(k int) Vec3 {
	i, j := wallEdge(k)
	d := prismFloor[j].Sub(prismFloor[i])
	n := Vec3{-d.Y, d.X, 0}
	if n.Dot(prismFloor[k].Sub(prismFloor[i])) > 0 {
		n = n.Mul(-1)
	}
	return n.Unit()
}

// draw paints the far faces, the grid on them, and the ladders and names
// beyond them.
func (f prismFurniture) draw(b ir.Backend, path *ir.Path, boxes *[]ir.Rect) {
	f.walls(b, path)
	f.grid(b, path)
	f.ladders(b, path, boxes)
}

// walls fills the horizontal face on the far side of the prism and the side
// walls pointing away from the camera, and outlines the whole solid.
//
// Each face is filled on its own rather than as subpaths of one path, which is
// [cube.walls]'s reason: gg's GPU tier loses fills that follow one made of
// several large subpaths sharing edges. The outline is one stroke, because
// strokes were never the problem.
func (f prismFurniture) walls(b ir.Backend, path *ir.Path) {
	if f.th.CubeFill.A != 0 {
		path.Reset()
		f.triangle(path, f.floor)
		b.FillPath(path, ir.Solid(f.th.CubeFill), ir.NonZero)
		for k := range f.far {
			if !f.far[k] {
				continue
			}
			path.Reset()
			f.sideWall(path, k)
			b.FillPath(path, ir.Solid(f.th.CubeFill), ir.NonZero)
		}
	}
	if f.th.CubeEdge.A == 0 {
		return
	}
	path.Reset()
	// The whole wireframe: both triangles and the three verticals, so that the
	// solid reads as one however it is turned.
	f.triangle(path, 0)
	f.triangle(path, 1)
	for k := range prismFloor {
		lo, hi := f.proj.point(prismFloor[k]), f.proj.point(prismFloor[k].with(axisZ, 1))
		path.MoveTo(lo.X, lo.Y).LineTo(hi.X, hi.Y)
	}
	b.StrokePath(path, ir.Stroke{Color: f.th.CubeEdge, Width: pickWidth(f.th.AxisWidth)})
}

// triangle appends the floor or the roof as one closed subpath.
func (f prismFurniture) triangle(path *ir.Path, z float32) {
	for k, v := range prismFloor {
		p := f.proj.point(v.with(axisZ, z))
		if k == 0 {
			path.MoveTo(p.X, p.Y)
			continue
		}
		path.LineTo(p.X, p.Y)
	}
	path.Close()
}

// sideWall appends side wall k as one closed subpath.
func (f prismFurniture) sideWall(path *ir.Path, k int) {
	i, j := wallEdge(k)
	quad := [4]Vec3{
		prismFloor[i], prismFloor[j],
		prismFloor[j].with(axisZ, 1), prismFloor[i].with(axisZ, 1),
	}
	for n, v := range quad {
		p := f.proj.point(v)
		if n == 0 {
			path.MoveTo(p.X, p.Y)
			continue
		}
		path.LineTo(p.X, p.Y)
	}
	path.Close()
}

// grid strokes the ternary grid on the horizontal face and the fourth
// variable's ticks round the far side walls.
//
// The floor carries all three families — the two the component axes' own ticks
// are, and the derived third at the same levels — which is
// [github.com/timzifer/figure/coord.Ternary]'s arrangement carried into the
// scene: three ladders drawn from two tick lists.
func (f prismFurniture) grid(b ir.Backend, path *ir.Path) {
	if f.th.CubeGrid.A == 0 {
		return
	}
	path.Reset()
	n := 0
	line := func(a, c Vec3) {
		p, q := f.proj.point(a), f.proj.point(c)
		path.MoveTo(p.X, p.Y).LineTo(q.X, q.Y)
		n++
	}
	for _, t := range f.ticks[axisX] {
		if t.Minor {
			continue
		}
		v := clamp01(t.Pos)
		// Constant first component, and the derived third at the same level.
		line(f.on(v, 0), f.on(v, 1-v))
		line(f.on(1-v, 0), f.on(0, 1-v))
	}
	for _, t := range f.ticks[axisY] {
		if t.Minor {
			continue
		}
		v := clamp01(t.Pos)
		line(f.on(0, v), f.on(1-v, v))
	}
	// The fourth variable's grid runs round the far walls, so a height is
	// readable against whichever of them is behind the mark.
	for _, t := range f.ticks[axisZ] {
		if t.Minor {
			continue
		}
		z := clamp01(t.Pos)
		for k := range f.far {
			if !f.far[k] {
				continue
			}
			i, j := wallEdge(k)
			line(prismFloor[i].with(axisZ, z), prismFloor[j].with(axisZ, z))
		}
	}
	if n > 0 {
		b.StrokePath(path, ir.Stroke{Color: f.th.CubeGrid, Width: pickWidth(f.th.GridWidth), Dash: f.th.GridDash})
	}
}

// on is the floor point of a composition given as two fractions.
func (f prismFurniture) on(u, v float32) Vec3 { return f.p.place(u, v, f.floor) }

// ladders strokes the three edges a reading is taken along, their tick marks
// and labels, the corner names and the depth axis's title.
func (f prismFurniture) ladders(b ir.Backend, path *ir.Path, boxes *[]ir.Rect) {
	tickFont := f.th.Font(f.th.TickSize)
	nameFont := f.th.Font(f.th.LabelSize)

	*boxes = (*boxes)[:0]
	f.names(b, nameFont, boxes)

	for _, l := range f.edges() {
		if len(f.ticks[l.axis]) == 0 {
			continue
		}
		path.Reset()
		from, to := f.proj.point(l.at(0)), f.proj.point(l.at(1))
		path.MoveTo(from.X, from.Y).LineTo(to.X, to.Y)
		for _, t := range f.ticks[l.axis] {
			anchor := f.proj.point(l.at(clamp01(t.Pos)))
			out := f.outward(anchor)
			reach := f.th.TickLength
			if t.Minor {
				reach /= 2
			}
			path.MoveTo(anchor.X, anchor.Y).LineTo(anchor.X+out.X*reach, anchor.Y+out.Y*reach)
		}
		if f.th.CubeEdge.A != 0 {
			b.StrokePath(path, ir.Stroke{Color: f.th.AxisColor, Width: pickWidth(f.th.AxisWidth)})
		}
		// The same greedy pass a cube runs, and for the same reason: any of
		// the three ladders can be the one pointing at the reader, and an
		// axis whose labels all land in one place is a pile of numbers rather
		// than a scale. An axis that keeps fewer than two gives its boxes
		// back — one number names a position the reader cannot tell from any
		// other, which is a claim the picture does not support.
		mark := len(*boxes)
		if f.placeLabels(b, tickFont, l, boxes, false) < 2 {
			*boxes = (*boxes)[:mark]
			continue
		}
		*boxes = (*boxes)[:mark]
		f.placeLabels(b, tickFont, l, boxes, true)
	}
}

// segment is a run of the prism's wireframe one axis is read along, and which
// axis that is.
type segment struct {
	axis     int
	from, to Vec3
}

func (s segment) at(t float32) Vec3 {
	return s.from.Add(s.to.Sub(s.from).Mul(t))
}

// edges picks which run of the wireframe each axis writes its ticks along.
//
// The two components take the floor edges where the other one is nothing,
// which is where a flat ternary chart prints them. The fourth variable takes
// whichever of the three verticals projects furthest left, which is on the
// silhouette by construction — so its numbers are outside the walls rather
// than written across them.
func (f prismFurniture) edges() [3]segment {
	z := f.floor
	out := [3]segment{
		{axis: axisX, from: prismFloor[0].with(axisZ, z), to: prismFloor[2].with(axisZ, z)},
		{axis: axisY, from: prismFloor[1].with(axisZ, z), to: prismFloor[2].with(axisZ, z)},
	}
	best, bestX := 0, float32(math.Inf(1))
	for k := range prismFloor {
		if px := f.proj.point(prismFloor[k].with(axisZ, 0.5)).X; px < bestX {
			best, bestX = k, px
		}
	}
	out[2] = segment{axis: axisZ, from: prismFloor[best], to: prismFloor[best].with(axisZ, 1)}
	return out
}

// placeLabels runs the greedy pass over one ladder's tick labels, claiming a
// box for each one it keeps, and draws them when draw is set. It reports how
// many it kept, so the caller can run it twice — once to learn whether the
// axis has a scale to show, and again to put it on the page.
func (f prismFurniture) placeLabels(b ir.Backend, font ir.FontRef, l segment, boxes *[]ir.Rect, draw bool) int {
	kept := 0
	for _, t := range f.ticks[l.axis] {
		if t.Label == "" {
			continue
		}
		anchor := f.proj.point(l.at(clamp01(t.Pos)))
		out := f.outward(anchor)
		gap := f.th.TickLength + f.th.TickLabelPad
		h, v := alignFor(out)
		run := ir.TextRun{
			Text:  t.Label,
			Font:  font,
			At:    ir.Point{X: anchor.X + out.X*gap, Y: anchor.Y + out.Y*gap},
			H:     h,
			V:     v,
			Color: f.th.TickColor,
		}
		box := labelBox(run, b.Measure(run), f.th.TickLabelPad)
		if overlapsAny(box, *boxes) {
			continue
		}
		if draw {
			b.Text(run)
		}
		*boxes = append(*boxes, box)
		kept++
	}
	return kept
}

// names writes the three component names at the corners they belong to, and
// the fourth variable's title along its own edge.
//
// They go first and claim their boxes before any tick label does, which is
// [cube.axes]'s rule: a name says what the axis is and a tick label is one
// reading off it, so where the two cannot both fit the name is the one that
// survives.
func (f prismFurniture) names(b ir.Backend, font ir.FontRef, boxes *[]ir.Rect) {
	gap := f.th.TickLength + f.th.TickLabelPad + f.th.AxisTitlePad
	for k, name := range f.p.corners {
		if name == "" {
			continue
		}
		at := f.proj.point(prismFloor[k].with(axisZ, f.floor))
		out := f.outward(at)
		h, v := alignFor(out)
		run := ir.TextRun{
			Text:  name,
			Font:  font,
			At:    ir.Point{X: at.X + out.X*gap, Y: at.Y + out.Y*gap},
			H:     h,
			V:     v,
			Color: f.th.LabelColor,
		}
		b.Text(run)
		*boxes = append(*boxes, labelBox(run, b.Measure(run), f.th.TickLabelPad))
	}
	if f.title[axisZ] == "" {
		return
	}
	l := f.edges()[2]
	from, to := f.proj.point(l.at(0)), f.proj.point(l.at(1))
	anchor := f.proj.point(l.at(0.5))
	out := f.outward(anchor)
	h := b.Measure(ir.TextRun{Text: f.title[axisZ], Font: font})
	reach, _ := furnitureReach(f.th, f.widest(b, f.th.Font(f.th.TickSize), axisZ), h.Ascent+h.Descent)
	run := ir.TextRun{
		Text:     f.title[axisZ],
		Font:     font,
		At:       ir.Point{X: anchor.X + out.X*reach, Y: anchor.Y + out.Y*reach},
		H:        ir.AlignCenter,
		V:        ir.AlignMiddle,
		Rotation: uprightAngle(to.X-from.X, to.Y-from.Y),
		Color:    f.th.LabelColor,
	}
	b.Text(run)
	*boxes = append(*boxes, titleBox(run, b.Measure(run), f.th.TickLabelPad))
}

// widest is how far one axis's tick labels reach out of the prism.
func (f prismFurniture) widest(m ir.Backend, font ir.FontRef, a int) float32 {
	widest := float32(0)
	for _, t := range f.ticks[a] {
		if t.Label == "" {
			continue
		}
		if w := m.Measure(ir.TextRun{Text: t.Label, Font: font}).Advance; w > widest {
			widest = w
		}
	}
	return widest
}

// outward is the unit screen direction from the middle of the projected prism
// toward a point on its edge, which is where a label goes.
func (f prismFurniture) outward(at ir.Point) ir.Point {
	dx, dy := at.X-f.mid.X, at.Y-f.mid.Y
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n == 0 {
		return ir.Point{X: 0, Y: 1}
	}
	return ir.Point{X: dx / n, Y: dy / n}
}

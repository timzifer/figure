package geom

import (
	"slices"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/theme"
)

// Extrude draws a [Bar] or a [Rect] layer with volume: a front face where the
// flat mark would have been, and the top and side faces an oblique view of it
// shows.
//
//	p := figure.New(figure.Coord(coord.Oblique()))
//	p.Add(geom.Bar(src, geom.X("quarter"), geom.Y("revenue"), geom.Extrude(true)))
//
// The depth is the coord's, and there is no way to set it from a column — see
// [github.com/timzifer/figure/coord.Oblique] and
// docs/adr/0055-depth-without-a-third-axis.md. A layer that asks for it under a
// coord that does not see its panel from an angle, which is every coord but
// the oblique one, draws exactly what it always drew.
//
// The front face is the mark, unforeshortened: its height is the value, on the
// same axis a flat chart would have measured it on. The two other faces are
// the mark's colour mixed toward the theme's [theme.Theme.DepthTop] and
// [theme.Theme.DepthSide] — two fixed shades and nothing more, no light model.
// All three are separate shapes that a pointer can land on, and all three
// answer with the mark's row. A layer that names both a [Fill] and a [Color]
// outlines its marks here as it does flat, each mark's faces stroked with the
// mark rather than all the fronts at the end — a back mark's outline over a
// front mark's faces would be worse than none.
func Extrude(on bool) Option { return func(c *config) { c.extrude = on } }

// How far a face turned up and a face turned aside are mixed toward the
// theme's two depth shades. Enough to read as a different face of one solid,
// not so much that a face reads as a different colour of the palette.
const (
	extrudeTopMix  = 0.3
	extrudeSideMix = 0.35
)

// extrusion is a layer's extrusion resolved for one Build: the depth vector, or
// the zero value for a layer that asked for none or a coord that has none.
type extrusion struct {
	dx, dy float32
	on     bool
}

// extruding resolves c's extrusion against the coord the layer is drawn in,
// once per Build, as [config.breaking] does.
func (c config) extruding(cd coord.Coord) extrusion {
	if !c.extrude {
		return extrusion{}
	}
	e, ok := cd.(coord.Extruder)
	if !ok {
		return extrusion{}
	}
	dx, dy := e.Extrude()
	if dx == 0 && dy == 0 {
		return extrusion{}
	}
	return extrusion{dx: dx, dy: dy, on: true}
}

// top is the face a mark shows on the side its depth vector points towards
// vertically — its top when the back is up — as four corners.
func (e extrusion) top(r ir.Rect) [4]ir.Point {
	y := r.Min.Y
	if e.dy > 0 {
		y = r.Max.Y
	}
	return [4]ir.Point{
		{X: r.Min.X, Y: y}, {X: r.Max.X, Y: y},
		{X: r.Max.X + e.dx, Y: y + e.dy}, {X: r.Min.X + e.dx, Y: y + e.dy},
	}
}

// side is the face a mark shows on the side its depth vector points towards
// horizontally.
func (e extrusion) side(r ir.Rect) [4]ir.Point {
	x := r.Max.X
	if e.dx < 0 {
		x = r.Min.X
	}
	return [4]ir.Point{
		{X: x, Y: r.Min.Y}, {X: x + e.dx, Y: r.Min.Y + e.dy},
		{X: x + e.dx, Y: r.Max.Y + e.dy}, {X: x, Y: r.Max.Y},
	}
}

// drawExtruded paints extruded marks one at a time, in the order that lets a
// mark cover the faces its neighbours turn towards it.
//
// Every mark is in the same plane, so nothing is in front of anything in depth;
// what overlaps is the faces a mark shows beside it. A face is turned the way
// the depth vector points, so the neighbour lying that way is the one standing
// over it — and a layer draws its marks in ascending order of the projection of
// each mark's middle onto the depth vector, which paints the covered face
// before the mark that covers it. Ties are broken by the row, because ADR 0012
// requires that nothing a chart draws depends on how a sort felt about equal
// keys.
//
// It is a sort over marks rather than over pixels. There is no depth buffer
// here and cannot be one under a vector IR (docs/adr/0056-three-dimensional-charts.md),
// and for marks that share one plane none is needed.
//
// The outline is stroked per mark too, and that is the same argument a second
// time. A layer that names both a fill and a colour outlines its marks, and an
// extruded one used to lose the outline altogether — the flat path that
// carried it was never built. Stroking every front afterwards would put a back
// mark's outline over a front mark's faces, so each mark's three faces are
// stroked with the mark, before the neighbour that stands over it is drawn.
//
// rects are the marks in device space, rows the source row behind each and
// cols the colour of each. A mark whose colour is transparent draws nothing.
// stroke is the outline, and is invisible for a layer that asked for none.
func (sc *scratch) drawExtruded(b ir.Backend, th theme.Theme, e extrusion, rects []ir.Rect, rows []int, cols []ir.Color, stroke ir.Stroke, hatchOf func(i int) ir.Hatching) {
	order := grow(sc.order, len(rects))
	for i := range order {
		order[i] = i
	}
	key := func(i int) float32 {
		r := rects[i]
		return (r.Min.X+r.Max.X)/2*e.dx + (r.Min.Y+r.Max.Y)/2*e.dy
	}
	slices.SortFunc(order, func(a, b int) int {
		ka, kb := key(a), key(b)
		switch {
		case ka < kb:
			return -1
		case ka > kb:
			return 1
		}
		return rows[a] - rows[b]
	})
	sc.order = order

	outlined := stroke.Visible()
	// The outline of one mark, collected as the faces are filled and stroked
	// once the mark is whole. It is the scratch's own buffer, so a chart
	// redrawn every frame does not allocate one per mark.
	edge := &sc.line
	quad := func(p *ir.Path, pts [4]ir.Point) {
		p.MoveTo(pts[0].X, pts[0].Y).LineTo(pts[1].X, pts[1].Y).
			LineTo(pts[2].X, pts[2].Y).LineTo(pts[3].X, pts[3].Y).Close()
	}
	face := func(pts [4]ir.Point, c ir.Color) {
		sc.fill.Reset()
		quad(&sc.fill, pts)
		b.FillPath(&sc.fill, ir.Solid(c), ir.NonZero)
		if outlined {
			quad(edge, pts)
		}
	}
	for _, i := range order {
		col := cols[i]
		if col.A == 0 {
			continue
		}
		r := rects[i]
		edge.Reset()
		if e.dx != 0 {
			face(e.side(r), palette.Lerp(col, th.DepthSide, extrudeSideMix))
		}
		if e.dy != 0 {
			face(e.top(r), palette.Lerp(col, th.DepthTop, extrudeTopMix))
		}
		sc.fill.Reset()
		sc.fill.Rect(r)
		// Only the front face is hatched. A pattern on the top and the sides
		// would run at a different apparent angle on each of them — they are
		// the same plane seen sheared — and would fight the shading that is
		// already telling the reader which face is which.
		var h ir.Hatching
		if hatchOf != nil {
			h = hatchOf(i)
		}
		ir.FillHatched(b, &sc.fill, ir.Solid(col), ir.NonZero, h)
		if outlined {
			edge.Rect(r)
			b.StrokePath(edge, stroke)
		}
	}
}

// reportExtruded tells the frame where each extruded mark's row is: in the
// middle of each face it shows beside its front, so that a pointer on a top or
// a side face finds the row inside the face it landed on, and then at the
// position a flat mark would have reported. front is that flat position.
//
// The front comes last because a row reported more than once is located at
// the last position it was given — [github.com/timzifer/figure/interact.Index.Locate]
// — and the front is where a highlight or a leader line should point: the
// value, on the axis that measures it.
//
// A row therefore appears up to three times in what the layer reports, which
// is why a selection gathers its rows as a set — see
// [github.com/timzifer/figure.Live.Select].
func (sc *scratch) reportExtruded(f Frame, e extrusion, rects []ir.Rect, rows []int, front []ir.Point, s series) {
	pts := sc.edge[:0]
	faces := sc.faces[:0]
	for i, r := range rects {
		if e.dy != 0 {
			pts = append(pts, middle(e.top(r)))
			faces = append(faces, rows[i])
		}
		if e.dx != 0 {
			pts = append(pts, middle(e.side(r)))
			faces = append(faces, rows[i])
		}
		pts = append(pts, front[i])
		faces = append(faces, rows[i])
	}
	sc.edge, sc.faces = pts, faces
	f.Marks(MarkRows{At: pts, Rows: sc.sourceRows(s, faces)})
}

func middle(q [4]ir.Point) ir.Point {
	return ir.Point{X: (q[0].X + q[2].X) / 2, Y: (q[0].Y + q[2].Y) / 2}
}

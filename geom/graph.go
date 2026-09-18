package geom

import (
	"math"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/stat"
)

// Graph draws a directed graph in ranks: every edge that is not on a cycle runs
// from one rank to a later one, and a node sits in a box carrying its name.
//
//	geom.Graph(src, geom.From("state"), geom.To("next"))
//
// The table is one row per edge — where it comes from and where it goes. Nodes
// are not declared anywhere: a node exists because a row mentioned it, and the
// order they are first mentioned in is the order they take their colours and
// their places in. See [From].
//
// This is the mark a state chart, a dependency graph and a layered flowchart
// are drawn with. See docs/adr/0072-layered-graph-layout.md.
//
// # What it decides
//
// A node stands one rank past the deepest predecessor that reaches it, so an
// edge reads outwards and a state never sits before the one that leads to it.
// Within a rank the nodes are ordered by [stat.LayeredSweeps] passes of the
// barycentre heuristic, which reduces crossings without promising the fewest:
// the sort is stable, so two nodes it cannot tell apart keep the order their
// rows gave them, and the picture is the same on one goroutine or on eight.
// [Order] is how a caller asks for a different arrangement — by sorting its own
// rows.
//
// # Cycles and self-edges
//
// Unlike [Sankey], which refuses a cyclic edge list with [ErrCyclic], this mark
// breaks cycles and draws them: a state machine that cannot return to an
// earlier state is not a state machine. The edge that closes a cycle is drawn
// in its true direction, straight from its source to its target against the
// rank order, which is what makes a returning transition read as one. An edge
// whose two ends are the same node is drawn as a loop beside it.
//
// # The box is measured, the layout is not
//
// A node's box is sized from its label through the backend's own text metrics
// and centred on the position the layout chose, so the layout itself never
// depends on which font a backend happens to have — the same document renders
// to SVG and to PNG with the same geometry. The cost is that a long name can
// overlap its neighbour, because the layout spaced them without knowing how
// wide they would be; [Padding] widens the gap between ranks' slots and
// [FontSize] narrows the labels.
//
// Because the box is measured on screen it stays a rectangle under every coord,
// where a [Treemap] cell becomes an annular sector under a polar one. The
// *positions* still go through the coord, so under
// [github.com/timzifer/figure/coord.Polar] the ranks are concentric rings with
// the sources at the hub.
//
// Both axes describe the unit square, so give the chart a theme with no grid,
// no axis lines and no ticks.
func Graph(src data.Source, opts ...Option) Geom {
	return &graphGeom{src: src, cfg: newConfig(opts)}
}

// graphPad is how much room a node's box leaves round its label, as a fraction
// of the label's own height. It is a fraction rather than a length so that a
// chart drawn at twice the font size is the same picture twice the size.
const graphPad = 0.55

// graphLoop is how far a self-edge's loop stands off its node, as a multiple of
// the node's own height.
const graphLoop = 0.9

// graphMinBox is the smallest a node's box is drawn, in device units, so that a
// node whose name is empty is still something a pointer can land on.
const graphMinBox = 6

type graphGeom struct {
	src data.Source
	cfg config
	e   edges
	lay stat.Layered

	// The device geometry of each node: where its middle is and how big its
	// box is. Both are resolved in Build, because both need the backend.
	at   []ir.Point
	size []ir.Point
	// How far the layout is held off the edges of the panel, as a fraction of
	// the unit square, so that a box on the first or last rank is drawn whole
	// rather than clipped in half.
	insetX, insetY float64
	err            error
}

func (g *graphGeom) Train(t Training) error {
	if g.err = g.e.reset(g.src, g.cfg); g.err != nil {
		return g.err
	}
	if g.err = trainUnit(t.X, t.Y); g.err != nil {
		return g.err
	}
	g.lay.Reset(g.e.from, g.e.to, g.e.count())
	g.cfg.trainColors(g.e.s)
	return nil
}

// height turns a rank's place in the unit interval into the height the layout
// is drawn at.
//
// [Baseline] is what flips it, and it is the same knob an [Arc]'s rail moves
// on: the default puts rank zero at y = 0, which is the bottom of a Cartesian
// panel and the hub of a polar one, and Baseline(1) puts it at the top. There
// is no rankdir option, because the coord and this are what that option is in
// this grammar.
func (g *graphGeom) height(y float64) float64 {
	if g.cfg.baseline >= 1 {
		return 1 - y
	}
	return y
}

func (g *graphGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	sc := acquire(f)
	defer sc.release()
	cd := f.Coords()

	g.measure(b, f, cd)

	// The edges go down first and the nodes on top of them, which is both the
	// right picture — a box is a landmark and should not be hidden by what
	// passes it — and the right hit test: a pointer on a node reports the node,
	// because the later mark wins a tie. See docs/adr/0015-hit-testing.md.
	g.edges(b, sc, f)
	g.nodes(b, sc, f)
	return g.labels(b, f)
}

// measure resolves every node's middle and the size of the box round it.
//
// The middle comes from the layout through the coord, and the size comes from
// the backend's text metrics. That split is the whole of
// docs/adr/0072-layered-graph-layout.md's third claim.
func (g *graphGeom) measure(b ir.Backend, f Frame, cd coord.Coord) {
	run := ir.TextRun{Font: f.Theme.Font(g.fontSize(f))}
	line := b.Measure(ir.TextRun{Text: "Hg", Font: run.Font}).Height()
	pad := float32(math.Round(float64(line) * graphPad))

	g.at = grow(g.at, len(g.lay.Nodes))
	g.size = grow(g.size, len(g.lay.Nodes))
	var halfW, halfH float32
	for i := range g.lay.Nodes {
		run.Text = g.e.keys[i]
		m := b.Measure(run)
		g.size[i] = ir.Point{
			X: max(m.Advance+2*pad, graphMinBox),
			Y: max(line+2*pad, graphMinBox),
		}
		halfW = max(halfW, g.size[i].X/2)
		halfH = max(halfH, g.size[i].Y/2)
	}

	// The layout fills the unit square, so a node on the first or last rank
	// sits exactly on the panel's edge and half its box falls outside it. The
	// inset is what the layout does not know it needs: half the widest box, as
	// a fraction of the panel, taken off each end.
	g.insetX = inset(halfW, f.Area.Max.X-f.Area.Min.X)
	g.insetY = inset(halfH, f.Area.Max.Y-f.Area.Min.Y)
	for i, n := range g.lay.Nodes {
		g.at[i] = g.point(cd, f, n.X, n.Y)
	}
}

// point maps a pair of layout coordinates to a device point, held off the
// panel's edges and turned over if [Baseline] asked.
func (g *graphGeom) point(cd coord.Coord, f Frame, x, y float64) ir.Point {
	return cd.Point(
		f.X.Map(g.insetX+x*(1-2*g.insetX)),
		f.Y.Map(g.insetY+g.height(y)*(1-2*g.insetY)),
	)
}

// inset is half a box as a fraction of the panel, capped so that a chart too
// small for its labels still draws a layout rather than collapsing it to a
// point.
func inset(half, span float32) float64 {
	if !(span > 0) {
		return 0
	}
	return min(float64(half/span), 0.25)
}

func (g *graphGeom) fontSize(f Frame) float64 {
	if g.cfg.fontSize > 0 {
		return g.cfg.fontSize
	}
	return f.Theme.LabelSize
}

// box is the rectangle drawn round node i.
func (g *graphGeom) box(i int) ir.Rect {
	c, s := g.at[i], g.size[i]
	return ir.R(c.X-s.X/2, c.Y-s.Y/2, c.X+s.X/2, c.Y+s.Y/2)
}

// nodes fills every box, batched by colour and one subpath each so that a
// pointer lands on the node it is inside rather than on the sheet they were
// drawn as — docs/adr/0015-hit-testing.md.
//
// The boxes are device rectangles rather than areas handed to the coord, which
// is what keeps a node readable under a polar coord: a label cannot be set in
// an annular sector, and a box that became one would be a shape with a name
// lying across it.
func (g *graphGeom) nodes(b ir.Backend, sc *scratch, f Frame) {
	if len(g.lay.Nodes) == 0 {
		return
	}
	cols := sc.boxColors(g.cfg, f, series{}, g.e.keys, indexes(sc, len(g.lay.Nodes)))
	for _, run := range sc.groupByColorAt(cols) {
		if run.color.A == 0 {
			continue
		}
		sc.fill.Reset()
		for _, i := range run.idx {
			sc.fill.RoundRect(g.box(i), g.cfg.corner)
		}
		g.cfg.fillMark(b, &sc.fill, f, 0, run.color)
	}
}

// labels sets each node's name inside its box, in whichever of the theme's inks
// reads against the colour the box was painted.
func (g *graphGeom) labels(b ir.Backend, f Frame) error {
	run := ir.TextRun{
		Font: f.Theme.Font(g.fontSize(f)),
		H:    ir.AlignCenter,
		V:    ir.AlignMiddle,
	}
	for i := range g.lay.Nodes {
		run.Text = g.e.keys[i]
		if run.Text == "" {
			continue
		}
		run.Color = contrast(f.Theme, g.cfg.fillOf(g.cfg.nodeColor(f, g.e.keys, i), 1))
		if g.cfg.color != nil {
			run.Color = contrast(f.Theme, *g.cfg.color)
		}
		if run.Color.A == 0 {
			continue
		}
		run.At = g.at[i]
		b.Text(run)
	}
	return nil
}

// edges strokes every edge and fills its head, batched by colour.
func (g *graphGeom) edges(b ir.Backend, sc *scratch, f Frame) {
	rows := sc.rows[:0]
	for e, edge := range g.lay.Edges {
		if edge.OK {
			rows = append(rows, e)
		}
	}
	sc.rows = rows
	if len(rows) == 0 {
		return
	}

	stroke := g.cfg.dependStroke(f)
	size := pick(g.cfg.size, DefaultArrowSize)
	cols := sc.colorsFor(g.cfg, g.e.s, rows)
	if cols == nil {
		g.paint(b, sc, f, rows, stroke, stroke.Color, size)
		g.report(sc, f, rows)
		return
	}
	// One path per distinct colour, and one subpath per edge inside it, for
	// the same reason a dependency arrow is drawn that way.
	for _, run := range sc.groupByColorAt(cols) {
		if run.color.A == 0 {
			continue
		}
		sc.keep = sc.keep[:0]
		for _, k := range run.idx {
			sc.keep = append(sc.keep, rows[k])
		}
		st := stroke
		st.Color = run.color
		g.paint(b, sc, f, sc.keep, st, run.color, size)
	}
	g.report(sc, f, rows)
}

// paint strokes a set of edges and fills their heads, in two calls.
func (g *graphGeom) paint(b ir.Backend, sc *scratch, f Frame, es []int, st ir.Stroke, head ir.Color, size float32) {
	cd := f.Coords()
	drawHead := head.A != 0 && size > 0
	sc.line.Reset()
	sc.fill.Reset()
	for _, e := range es {
		tip, dir, ok := g.trace(&sc.line, sc, cd, f, e)
		if !ok || !drawHead {
			continue
		}
		arrowTip(&sc.fill, tip, dir, size)
	}
	if st.Visible() {
		b.StrokePath(&sc.line, st)
	}
	if drawHead && !sc.fill.Empty() {
		b.FillPath(&sc.fill, ir.Solid(head), ir.NonZero)
	}
}

// trace appends one edge's path and reports where its head goes: the point it
// arrives at on the target's box and the direction it arrives from.
func (g *graphGeom) trace(p *ir.Path, sc *scratch, cd coord.Coord, f Frame, e int) (ir.Point, ir.Point, bool) {
	edge := g.lay.Edges[e]
	src, dst := g.e.from[e], g.e.to[e]
	if edge.Self {
		return g.loop(p, src)
	}

	pts := sc.pts[:0]
	pts = append(pts, g.at[src])
	for _, bend := range g.lay.Bends[edge.Lo:edge.Hi] {
		pts = append(pts, g.point(cd, f, bend.X, bend.Y))
	}
	pts = append(pts, g.at[dst])
	if g.cfg.branch != Straight && len(pts) == 2 {
		// One rank to the next with nothing in between: the orthogonal reading
		// is out of the source, across at the halfway height, and in to the
		// target, which is the shape a flowchart is drawn with.
		mid := ir.Point{X: (pts[0].X + pts[1].X) / 2, Y: (pts[0].Y + pts[1].Y) / 2}
		pts = append(pts[:1], ir.Point{X: pts[0].X, Y: mid.Y}, ir.Point{X: pts[1].X, Y: mid.Y}, g.at[dst])
	}
	sc.pts = pts

	// Both ends are trimmed to the box they touch, so an edge meets a node's
	// edge rather than disappearing under it and coming out the other side.
	start, ok := exit(pts[0], pts[1], g.box(src))
	if !ok {
		return ir.Point{}, ir.Point{}, false
	}
	end, ok := exit(pts[len(pts)-1], pts[len(pts)-2], g.box(dst))
	if !ok {
		return ir.Point{}, ir.Point{}, false
	}
	pts[0], pts[len(pts)-1] = start, end
	p.Polyline(pts)
	return end, unit(ir.Point{X: end.X - pts[len(pts)-2].X, Y: end.Y - pts[len(pts)-2].Y}), true
}

// loop appends a self-edge: out of the node's right side, round, and back into
// its top. It never reaches the layering, so it is drawn against the node's own
// box and nothing else.
func (g *graphGeom) loop(p *ir.Path, i int) (ir.Point, ir.Point, bool) {
	r := g.box(i)
	h := (r.Max.Y - r.Min.Y) * graphLoop
	a := ir.Point{X: r.Max.X, Y: (r.Min.Y + r.Max.Y) / 2}
	d := ir.Point{X: (r.Min.X + r.Max.X) / 2, Y: r.Min.Y}
	p.MoveTo(a.X, a.Y)
	p.CubicTo(a.X+h, a.Y, d.X+h/2, d.Y-h, d.X, d.Y)
	return d, ir.Point{X: 0, Y: 1}, true
}

// report tells the frame which source row is behind each edge, at its middle.
//
// The edges are the rows: a node is what several rows have in common rather
// than a row of its own, so a pointer on one reports none — ADR 0039's rule,
// and this mark keeps it.
func (g *graphGeom) report(sc *scratch, f Frame, es []int) {
	if !sc.wantRows {
		return
	}
	// sc.pts held the last edge's polyline, which nothing reads once it has
	// been stroked, so the middles are gathered into the same buffer.
	pts := sc.pts[:0]
	mrows := sc.mrows[:0]
	for _, e := range es {
		src, dst := g.e.from[e], g.e.to[e]
		pts = append(pts, ir.Point{
			X: (g.at[src].X + g.at[dst].X) / 2,
			Y: (g.at[src].Y + g.at[dst].Y) / 2,
		})
		mrows = append(mrows, g.e.s.rowAt(e))
	}
	sc.pts, sc.mrows = pts, mrows
	f.Marks(MarkRows{At: pts, Rows: mrows})
}

// exit is where the segment from inside a box towards a point outside it
// crosses the box's edge.
//
// It is measured rather than assumed — the nearest corner, the nearest side —
// because an edge that leaves at the wrong place puts its head somewhere the
// reader can see is wrong. A target whose middle is inside the source's box has
// no crossing and is reported as such rather than drawn backwards.
func exit(from, to ir.Point, r ir.Rect) (ir.Point, bool) {
	dx, dy := to.X-from.X, to.Y-from.Y
	if dx == 0 && dy == 0 {
		return ir.Point{}, false
	}
	hw, hh := (r.Max.X-r.Min.X)/2, (r.Max.Y-r.Min.Y)/2
	t := float32(math.Inf(1))
	if dx != 0 {
		t = min(t, float32(math.Abs(float64(hw/dx))))
	}
	if dy != 0 {
		t = min(t, float32(math.Abs(float64(hh/dy))))
	}
	if t > 1 {
		// The far point is inside the box: there is nothing left to draw.
		return ir.Point{}, false
	}
	return ir.Point{X: from.X + dx*t, Y: from.Y + dy*t}, true
}

// arrowTip appends the triangle that marks where an edge arrives, pointing
// along dir.
//
// [arrowhead] in gantt.go draws the same shape along an axis, because a
// dependency link always arrives along one. An edge in a graph arrives from
// wherever its source happens to be, so this one is given a direction rather
// than an orientation.
func arrowTip(p *ir.Path, tip, dir ir.Point, size float32) {
	back := ir.Point{X: tip.X - dir.X*size, Y: tip.Y - dir.Y*size}
	// The normal of the direction, which is the direction with its components
	// swapped and one of them negated.
	nx, ny := -dir.Y*size/2, dir.X*size/2
	p.MoveTo(tip.X, tip.Y)
	p.LineTo(back.X+nx, back.Y+ny)
	p.LineTo(back.X-nx, back.Y-ny)
	p.Close()
}

func unit(v ir.Point) ir.Point {
	d := float32(math.Hypot(float64(v.X), float64(v.Y)))
	if d == 0 {
		return ir.Point{X: 0, Y: 1}
	}
	return ir.Point{X: v.X / d, Y: v.Y / d}
}

func (g *graphGeom) ColorGuide() (ColorGuide, bool) { return g.cfg.colorGuide(g.e.s, g.err) }

func (g *graphGeom) Legends(f Frame) []LegendEntry {
	if g.err != nil {
		return nil
	}
	return LegendsOr(g, f, g.cfg.nodeLegends(f, g.e.keys))
}

func (g *graphGeom) Legend(f Frame) (LegendEntry, bool) {
	return oneNodeLegend(g.cfg, f, g.err)
}

func (g *graphGeom) Source() data.Source { return g.src }

func (g *graphGeom) Subset(rows []int) Geom {
	return &graphGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *graphGeom) Describe() Desc {
	d := g.cfg.describe(MarkGraph)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*graphGeom)(nil)
	_ Faceter   = (*graphGeom)(nil)
	_ Guided    = (*graphGeom)(nil)
	_ Legender  = (*graphGeom)(nil)
)

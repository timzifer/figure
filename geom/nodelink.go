package geom

import (
	"fmt"
	"math"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/stat"
)

// ErrTooManyNodes reports a graph with more nodes than a layout can place.
var ErrTooManyNodes = fmt.Errorf("figure/geom: this mark cannot lay out that many nodes")

// NodeLinkRadius is the radius a node is drawn at when [Size] says nothing, as
// a fraction of the shorter side of the panel.
//
// A fraction rather than a length, so that the same chart drawn at twice the
// size is the same picture twice the size.
const NodeLinkRadius = 0.012

// NodeLink draws a graph as dots joined by lines, placed so that the distance
// between two dots is as close as it can be to the number of edges between
// them.
//
//	geom.NodeLink(src, geom.From("person"), geom.To("knows"))
//
// The table is one row per edge. Nodes are not declared anywhere: a node exists
// because a row mentioned it, and the order they are first mentioned in is the
// order they take their colours in — [From] and [To], the channels every
// relational mark reads.
//
// This is the mark [ADR 0039](docs/adr/0039-relational-layouts.md) refused and
// [ADR 0077](docs/adr/0077-a-node-link-layout.md) admitted. What it runs is
// [stat.Stress], which is a minimisation with a closed-form step rather than a
// simulation with a stopping rule: the same table gives the same picture, on
// one goroutine or on eight.
//
// # What it is for, and what it is not
//
// It reads the edges as undirected and shows no order, because there is none to
// show: two dots near each other are two things with a short path between them.
// A graph that has a direction — a state machine, a dependency, a pipeline — is
// [Graph], which draws the direction as the reading. A tree is [Tree].
//
// It refuses a graph of more than [stat.MaxStressNodes] nodes with
// [ErrTooManyNodes], rather than drawing a hairball: past a few hundred nodes
// this picture stops being one, whatever it costs to compute.
//
// # The drawing
//
// A node is a disc, sized by [Size] or by [NodeLinkRadius], carrying its name
// beside it; an edge is a straight line between two discs, clipped at their
// rims. The discs are device circles rather than areas handed to the coord —
// the reason [Graph]'s boxes are — so a node stays round and its name stays
// level. The *positions* go through the coord like every other mark's.
//
// Both axes describe the unit square, so give the chart a theme with no grid,
// no axis lines and no ticks. The drawing is fitted to the largest square the
// panel holds, because what the layout matched is a distance and a panel that
// stretched one axis would undo it.
func NodeLink(src data.Source, opts ...Option) Geom {
	return &nodeLinkGeom{src: src, cfg: newConfig(opts)}
}

type nodeLinkGeom struct {
	src data.Source
	cfg config
	e   edges
	lay stat.Stress

	// The device geometry, resolved in Build: where each node's middle is and
	// how big every disc is.
	at     []ir.Point
	radius float32
	err    error
}

func (g *nodeLinkGeom) Train(t Training) error {
	if g.err = g.e.reset(g.src, g.cfg); g.err != nil {
		return g.err
	}
	if g.err = trainUnit(t.X, t.Y); g.err != nil {
		return g.err
	}
	g.lay.Reset(g.e.from, g.e.to, g.e.count())
	if g.lay.TooMany {
		g.err = fmt.Errorf("%w: %d nodes, and this layout places %d; geom.Graph draws a graph that has a direction",
			ErrTooManyNodes, g.e.count(), stat.MaxStressNodes)
		return g.err
	}
	g.cfg.trainColors(g.e.s)
	return nil
}

func (g *nodeLinkGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	if len(g.lay.Nodes) == 0 {
		return nil
	}
	sc := acquire(f)
	defer sc.release()

	g.place(f, f.Coords())
	// The edges go down first and the discs on top of them: a dot is a landmark
	// and should not be hidden by what passes it, and a pointer on a dot
	// reports the dot, because the later mark wins a tie
	// (docs/adr/0015-hit-testing.md).
	g.edges(b, sc, f)
	g.nodes(b, sc, f)
	g.labels(b, f)
	return nil
}

// place resolves every node's middle in device space.
//
// The unit square is squeezed into the largest square the panel holds, and then
// held off the edges by one radius so that a disc is drawn whole rather than
// clipped in half. Both are fractions of the unit square rather than device
// offsets, so the positions still go through the scales and the coord.
func (g *nodeLinkGeom) place(f Frame, cd coord.Coord) {
	g.radius = g.radiusOf(f)
	x0, sx := fitUnit(f.Area.Dx(), f.Area.Dy(), g.radius)
	y0, sy := fitUnit(f.Area.Dy(), f.Area.Dx(), g.radius)
	g.at = grow(g.at, len(g.lay.Nodes))[:0]
	for _, n := range g.lay.Nodes {
		g.at = append(g.at, cd.Point(
			f.X.Map(x0+n.X*sx),
			f.Y.Map(y0+n.Y*sy),
		))
	}
}

// fitUnit is where one axis's unit interval starts and how much of it the
// drawing uses, given the panel's two sides and the room a disc needs.
//
// The square comes from the shorter side: on the longer axis the drawing takes
// the matching fraction of the interval and is centred in what is left. A panel
// too small for its discs falls back to using the whole interval rather than
// collapsing the layout to a point.
func fitUnit(along, across, radius float32) (start, span float64) {
	span = 1
	if along > 0 && across > 0 && across < along {
		span = float64(across / along)
	}
	if along > 0 {
		if pad := 2 * float64(radius/along); pad < span {
			span -= pad
		}
	}
	return (1 - span) / 2, span
}

// radiusOf is how big a disc is drawn, in device units.
func (g *nodeLinkGeom) radiusOf(f Frame) float32 {
	if g.cfg.size > 0 {
		return float32(g.cfg.size) / 2
	}
	return float32(NodeLinkRadius) * min(f.Area.Dx(), f.Area.Dy())
}

// nodes fills every disc, batched by colour and one subpath each so that a
// pointer lands on the node it is inside rather than on the sheet they were
// drawn as.
func (g *nodeLinkGeom) nodes(b ir.Backend, sc *scratch, f Frame) {
	cols := sc.boxColors(g.cfg, f, series{}, g.e.keys, indexes(sc, len(g.lay.Nodes)))
	for _, run := range sc.groupByColorAt(cols) {
		if run.color.A == 0 {
			continue
		}
		sc.fill.Reset()
		for _, i := range run.idx {
			sc.fill.Circle(g.at[i], g.radius)
		}
		g.cfg.fillMark(b, &sc.fill, f, 0, run.color)
	}
}

// edges strokes one straight line per row, clipped at the two rims so that a
// line meets a disc rather than disappearing under it.
func (g *nodeLinkGeom) edges(b ir.Backend, sc *scratch, f Frame) {
	st := g.cfg.dependStroke(f)
	if !st.Visible() {
		return
	}
	rows := sc.rows[:0]
	sc.fill.Reset()
	for e, edge := range g.lay.Edges {
		if !edge.OK || edge.Self {
			continue
		}
		a, z, ok := g.segment(g.e.from[e], g.e.to[e])
		if !ok {
			continue
		}
		sc.fill.MoveTo(a.X, a.Y)
		sc.fill.LineTo(z.X, z.Y)
		rows = append(rows, e)
	}
	sc.rows = rows
	if sc.fill.Empty() {
		return
	}
	b.StrokePath(&sc.fill, st)
	g.report(sc, f, rows)
}

// segment is the visible part of the line between two nodes: the straight
// between their middles, shortened by a radius at each end. Two discs closer
// together than their own rims have no visible line and report as much.
func (g *nodeLinkGeom) segment(i, j int) (ir.Point, ir.Point, bool) {
	a, z := g.at[i], g.at[j]
	dx, dy := float64(z.X-a.X), float64(z.Y-a.Y)
	d := math.Hypot(dx, dy)
	if d <= 2*float64(g.radius) {
		return a, z, false
	}
	ux, uy := float32(dx/d)*g.radius, float32(dy/d)*g.radius
	return ir.Point{X: a.X + ux, Y: a.Y + uy}, ir.Point{X: z.X - ux, Y: z.Y - uy}, true
}

// report hands the edges' middles to the frame, which is what a pointer over a
// line lands on. A node reports no row: a node is what several rows have in
// common rather than a row of its own — docs/adr/0039-relational-layouts.md.
func (g *nodeLinkGeom) report(sc *scratch, f Frame, es []int) {
	if !sc.wantRows {
		return
	}
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

// labels sets each node's name above its disc, in the theme's own ink: the
// label is outside the mark rather than inside it, so it reads against the
// panel rather than against the colour the disc was painted.
func (g *nodeLinkGeom) labels(b ir.Backend, f Frame) {
	if g.cfg.fontSize < 0 {
		return
	}
	run := ir.TextRun{
		Font:  f.Theme.Font(g.fontSize(f)),
		H:     ir.AlignCenter,
		V:     ir.AlignBottom,
		Color: f.Theme.LabelColor,
	}
	if run.Color.A == 0 {
		return
	}
	for i := range g.lay.Nodes {
		run.Text = g.e.keys[i]
		if run.Text == "" {
			continue
		}
		run.At = ir.Point{X: g.at[i].X, Y: g.at[i].Y - g.radius - 2}
		b.Text(run)
	}
}

func (g *nodeLinkGeom) fontSize(f Frame) float64 {
	if g.cfg.fontSize > 0 {
		return g.cfg.fontSize
	}
	return f.Theme.TickSize
}

func (g *nodeLinkGeom) ColorGuide() (ColorGuide, bool) { return g.cfg.colorGuide(g.e.s, g.err) }

func (g *nodeLinkGeom) Legends(f Frame) []LegendEntry {
	if g.err != nil {
		return nil
	}
	return LegendsOr(g, f, g.cfg.nodeLegends(f, g.e.keys))
}

func (g *nodeLinkGeom) Legend(f Frame) (LegendEntry, bool) {
	return oneNodeLegend(g.cfg, f, g.err)
}

func (g *nodeLinkGeom) Source() data.Source { return g.src }

func (g *nodeLinkGeom) Subset(rows []int) Geom {
	return &nodeLinkGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *nodeLinkGeom) Describe() Desc {
	d := g.cfg.describe(MarkNodeLink)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*nodeLinkGeom)(nil)
	_ Faceter   = (*nodeLinkGeom)(nil)
)

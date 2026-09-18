package geom

import (
	"fmt"
	"math"
	"strconv"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
)

// The Venn diagram of two or three sets, drawn from the same membership table
// an UpSet plot reads.
//
// [ADR 0039](docs/adr/0039-relational-layouts.md) refused Venn as "a
// circle-packing optimiser with its own failure modes", and that is true of the
// *area-proportional* diagram, where the radii and the distances have to be
// solved for so that every region's area is its count. It is not true of the
// diagram people draw: two or three circles of one size in a fixed arrangement,
// with the counts written in the regions. Nothing there is solved, so nothing
// there is a simulation. See docs/adr/0074-sets-are-counted.md.

// VennSteps is how many points one circle of a [Venn] is drawn with.
//
// A circle goes through the coordinate stage as a closed run of points like
// every other mark, so the number is a constant rather than something measured
// off the panel: a chart whose outline got smoother when it was resized would
// be a chart that redraws differently at two sizes.
const VennSteps = 96

// MaxVennSets is how many sets a [Venn] draws.
//
// Three, because three circles of one size have a symmetric arrangement whose
// seven regions all exist and are all big enough to write a number in, and four
// circles have no such arrangement at all — the four-set diagram that gets
// drawn uses ellipses, which is a different picture with different regions, and
// the five-set one is a packing problem. An [Intersections] plot is the answer
// past three, and it is the better reading well before that.
const MaxVennSets = 3

// Venn draws two or three overlapping circles and writes in each region how
// many elements are in exactly that combination of sets.
//
//	p.X(scale.Linear())
//	p.Y(scale.Linear())
//	p.Add(geom.Venn(src, geom.From("customer"), geom.To("product")))
//
// The table is the membership list [Intersections] reads — one row per
// (element, set) pair, the element named by [From] and the set by [To] — and
// the counts are the same counts, which is what makes the two charts of one
// table agree. A region says how many elements are in exactly its sets, so the
// numbers partition the elements and add up to how many there are; the common
// way of labelling a Venn, where a circle carries its own total, counts the
// overlapping elements twice and is the mislabelling this refuses to draw.
//
// # It fills the unit square
//
// Both axes describe the unit square, like every other mark that places its own
// geometry, so the chart wants a theme with no grid, no axis line and no ticks
// — see [github.com/timzifer/figure/theme.Grid] and the relational marks, whose
// convention this follows.
//
// # What it does not do
//
// It is not area-proportional: the circles are the same size whatever the
// counts are, because making the areas the counts is an optimiser with its own
// failure modes and no solution at all for most three-set tables. It refuses a
// fourth set with [ErrTooManySets] rather than drawing something that looks
// like a Venn diagram and is not one.
func Venn(src data.Source, opts ...Option) Geom {
	return &vennGeom{src: src, cfg: newConfig(opts)}
}

type vennGeom struct {
	src data.Source
	cfg config
	m   membership
	err error

	pts []ir.Point
}

func (g *vennGeom) Train(t Training) error {
	g.err = g.resolve(t)
	return g.err
}

func (g *vennGeom) resolve(t Training) error {
	if err := g.m.reset(g.src, g.cfg); err != nil {
		return err
	}
	if n := g.m.sets.count(); n > MaxVennSets {
		return fmt.Errorf("%w: %d sets, and a Venn diagram of circles holds %d; geom.Intersections draws any number",
			ErrTooManySets, n, MaxVennSets)
	}
	return trainUnit(t.X, t.Y)
}

// vennCircle is one set's disc in the unit square.
type vennCircle struct {
	cx, cy, r float64
}

// vennLayout is where the circles go, and it is a table rather than a solver.
//
// One set is a disc in the middle; two are side by side overlapping by about a
// third of their width; three are on the corners of an equilateral triangle,
// which is the arrangement every printed Venn diagram uses and the only one
// whose seven regions are all visible.
func vennLayout(n int) []vennCircle {
	switch n {
	case 1:
		return []vennCircle{{0.5, 0.5, 0.32}}
	case 2:
		const r, d = 0.30, 0.16
		return []vennCircle{{0.5 - d, 0.5, r}, {0.5 + d, 0.5, r}}
	default:
		const r, d = 0.27, 0.155
		out := make([]vennCircle, 0, 3)
		for k := range 3 {
			a := math.Pi/2 - float64(k)*2*math.Pi/3
			out = append(out, vennCircle{
				cx: 0.5 + d*math.Cos(a),
				cy: 0.48 + d*math.Sin(a),
				r:  r,
			})
		}
		return out
	}
}

func (g *vennGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	n := g.m.sets.count()
	if n == 0 {
		return nil
	}
	circles := vennLayout(n)
	cd := f.Coords()
	at := func(x, y float64) ir.Point { return cd.Point(f.X.Map(x), f.Y.Map(y)) }

	sc := acquire(f)
	defer sc.release()

	// One filled disc per set, each at the fill alpha a band is drawn at: the
	// overlaps are read off where two translucent discs cross, which is the
	// whole picture, so an opaque fill would hide the reading.
	alpha := f.Theme.AnnotationOpacity
	if g.cfg.opacity >= 0 {
		alpha = clamp01(g.cfg.opacity)
	}
	width := pick(g.cfg.width, f.Theme.LineWidth)
	for i, c := range circles {
		col := g.cfg.nodeColor(f, g.m.sets.keys, i)
		g.discPath(&sc.fill, cd, at, c)
		if a := ir.Fade(col, alpha); a.A != 0 {
			b.FillPath(&sc.fill, ir.Fill{Color: a}, ir.NonZero)
		}
		b.StrokePath(&sc.fill, ir.Stroke{Color: col, Width: width})
	}

	g.writeCounts(b, f, at, circles)
	return nil
}

// discPath builds one circle as a closed run of points through the coord, so
// that a Venn drawn under a coord that bends the plane bends with it.
func (g *vennGeom) discPath(p *ir.Path, cd coord.Coord, at func(x, y float64) ir.Point, c vennCircle) {
	g.pts = grow(g.pts, VennSteps)[:0]
	for k := range VennSteps {
		a := 2 * math.Pi * float64(k) / VennSteps
		g.pts = append(g.pts, at(c.cx+c.r*math.Cos(a), c.cy+c.r*math.Sin(a)))
	}
	p.Reset()
	appendEdges(p, cd, g.pts, true)
	closeLoop(p, cd, g.pts)
}

// writeCounts writes the number in every region, and every set's name outside
// its own circle.
//
// Every region, including an empty one: a region with nobody in it is a fact
// about the data, and leaving it blank would read as a region nobody counted.
func (g *vennGeom) writeCounts(b ir.Backend, f Frame, at func(x, y float64) ir.Point, circles []vennCircle) {
	run := ir.TextRun{
		Font: g.cfg.labelFont(f),
		H:    ir.AlignCenter, V: ir.AlignMiddle,
		Color: f.Theme.LabelColor,
	}
	for mask := uint64(1); mask < 1<<uint(len(circles)); mask++ {
		x, y := vennAnchor(circles, mask)
		run.Text = strconv.Itoa(g.m.x.CountOf(mask))
		run.At = at(x, y)
		b.Text(run)
	}
	for i := range circles {
		x, y := vennNameAnchor(circles, i)
		run.Text = g.m.sets.keys[i]
		run.Color = g.cfg.nodeColor(f, g.m.sets.keys, i)
		run.At = at(x, y)
		b.Text(run)
	}
}

// vennAnchor is where one region's number goes: the centroid of the circles in
// the combination, pushed away from the centroid of all of them by how few of
// them there are.
//
// It is arithmetic on the fixed arrangement rather than a search for the
// region's own centre. The arrangement is fixed, so the anchors are known, and
// a search would be the optimiser this record exists without.
func vennAnchor(circles []vennCircle, mask uint64) (float64, float64) {
	var mx, my, all float64
	var inx, iny, in float64
	for i, c := range circles {
		mx, my, all = mx+c.cx, my+c.cy, all+1
		if mask&(1<<uint(i)) != 0 {
			inx, iny, in = inx+c.cx, iny+c.cy, in+1
		}
	}
	mx, my = mx/all, my/all
	if in == 0 {
		return mx, my
	}
	inx, iny = inx/in, iny/in

	// How far out: a set on its own sits well outside the overlaps, a pair sits
	// in its lens, and the combination of everything sits in the middle.
	push := 1.0
	switch {
	case in == all:
		push = 0
	case in == 1 && all == 2:
		push = 1.45
	case in == 1:
		push = 2.05
	default:
		push = 1.85
	}
	return mx + (inx-mx)*push, my + (iny-my)*push
}

// vennNameAnchor is where a set's own name goes: outside its circle, on the
// line out from the middle of the arrangement.
//
// Two circles are side by side, so that line runs straight off the panel — a
// name there would be clipped by the plot rectangle rather than read. They take
// their names above themselves instead, which is where a two-set diagram is
// labelled on paper, and the three-set anchors are kept inside the unit square
// for the same reason.
func vennNameAnchor(circles []vennCircle, i int) (float64, float64) {
	c := circles[i]
	if len(circles) < 3 {
		return c.cx, c.cy + c.r + 0.05
	}
	var mx, my float64
	for _, o := range circles {
		mx, my = mx+o.cx, my+o.cy
	}
	mx, my = mx/float64(len(circles)), my/float64(len(circles))
	dx, dy := c.cx-mx, c.cy-my
	d := math.Hypot(dx, dy)
	if d == 0 {
		return c.cx, c.cy + c.r + 0.05
	}
	out := (d + c.r + 0.05) / d
	return clampUnit(mx + dx*out), clampUnit(my + dy*out)
}

// clampUnit keeps an anchor inside the unit square, a little in from the edge
// so that the text it anchors is inside the panel rather than half over it.
func clampUnit(v float64) float64 { return min(max(v, 0.03), 0.97) }

func (g *vennGeom) Legends(f Frame) []LegendEntry {
	if g.err != nil {
		return nil
	}
	return g.cfg.nodeLegends(f, g.m.sets.keys)
}

func (g *vennGeom) Legend(f Frame) (LegendEntry, bool) { return oneNodeLegend(g.cfg, f, g.err) }

func (g *vennGeom) Source() data.Source { return g.src }

func (g *vennGeom) Subset(rows []int) Geom {
	return &vennGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *vennGeom) Describe() Desc {
	d := g.cfg.describe(MarkVenn)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*vennGeom)(nil)
	_ Faceter   = (*vennGeom)(nil)
	_ Legender  = (*vennGeom)(nil)
)

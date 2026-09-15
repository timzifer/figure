package geom

import (
	"fmt"
	"math"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// Tree draws a hierarchy as a node-link tree: every node at a breadth across
// and a height out, joined to its parent by a branch.
//
//	geom.Tree(src, geom.ID("node"), geom.Parent("under"))                      // an org chart
//	geom.Tree(src, geom.ID("node"), geom.Parent("under"), geom.Value("height")) // a dendrogram
//
// The table is one row per node, read through the same three channels a
// [Treemap] and an [Icicle] read. What differs is what [Value] means: there it
// is a size, here it is where the node is drawn.
//
// # The height comes from a column or from the depth
//
// With [Value], a node's height is that column — a dendrogram's merge
// distance, a phylogram's branch length — and the Y axis is continuous and
// trained on it, so it can be read. Every leaf takes its own slot across, in
// the order a walk down the tree meets them, which is what keeps two leaves
// that are drawn at one height from being drawn at one place.
//
// Without it, a node's height is its depth, the Y axis is the integers, and
// the layout is the Reingold–Tilford tidy tree: subtrees pushed together
// until they would touch, so a shallow branch tucks in beside a deep one. See
// [stat.Tidy] for both.
//
// [Baseline] of 1 turns the height axis over within its extent: a depth tree
// grows down from a root at the top instead of up from one at the bottom, and
// a dendrogram hangs from its leaves.
//
// # A radial dendrogram is this mark under a polar coord
//
// The breadth is the X axis and the height the Y axis, so under
// [coord.Polar] the breadth goes round and the height goes out:
//
//	p := figure.New(figure.Theme(bare), figure.Coord(coord.Polar(coord.Hole(0.08))))
//	p.Add(geom.Tree(src, geom.ID("node"), geom.Parent("under")))
//
// A branch is drawn as data-space points through the coord, so the cross-piece
// of an elbow becomes the arc a radial dendrogram is printed with, and nothing
// here knows it.
//
// Give the coord a [coord.Hole] when the root is at the hub. The very centre of
// a polar coord has no angle, so a branch that ends exactly there has no way
// round to go and is drawn as a spiral; a small hole gives the root a ring to
// stand on, and the root's own cross-piece is drawn round it.
//
// # Beside a heatmap
//
// Given an ordinal X axis, the leaves are placed at their own names on it
// rather than at slots of their own, and each parent over the span of its
// children. A dendrogram in a [github.com/timzifer/figure.Track] above a
// heatmap therefore lines up with the heatmap's columns; an axis that is still
// discovering its categories learns them from the tree, in the tree's order. The height axis is never
// ordinal, and is refused with [ErrNotContinuous].
//
// A node is reported to a hit test at its own position. [Branches] chooses the
// shape of a branch; [ColorBy] colours each branch by the row of the node it
// leads to, which is how a clade is picked out.
func Tree(src data.Source, opts ...Option) Geom {
	return &treeGeom{src: src, cfg: newConfig(opts)}
}

// Branch is the shape a [Tree] joins a node to its parent with.
type Branch uint8

const (
	// Elbow goes out from the node, across at its parent's height, and in to
	// the parent: the dendrogram bracket. It is the default, because it keeps
	// a height readable along the whole of a branch.
	Elbow Branch = iota
	// Straight is one segment from the node to its parent, as a phylogram or
	// an org chart is often drawn.
	Straight
)

// Branches sets how a [Tree] draws a branch. The default is [Elbow].
func Branches(b Branch) Option { return func(c *config) { c.branch = b } }

type treeGeom struct {
	src data.Source
	cfg config
	t   tree
	lay stat.Tidy
	// bx and h are each node's breadth and height in data space. A node with
	// neither — a leaf whose name a fixed ordinal axis does not have — is NaN.
	bx, h []float64
	err   error
}

func (g *treeGeom) Train(t Training) error {
	x, y := t.X, t.Y
	if g.err = g.t.reset(g.src, g.cfg); g.err != nil {
		return g.err
	}
	if _, ordinal := y.(scale.Categorical); ordinal {
		g.err = fmt.Errorf("%w: a tree's height axis needs a scale.Linear", ErrNotContinuous)
		return g.err
	}
	g.t.depth = stat.AppendDepth(g.t.depth, g.t.parent)
	deepest := 0
	for i, d := range g.t.depth {
		if d < 0 {
			g.err = fmt.Errorf("%w: %q is its own ancestor", ErrCyclic, g.t.keys[i])
			return g.err
		}
		deepest = max(deepest, d)
	}

	n := len(g.t.parent)
	cat, ordinal := x.(scale.Categorical)
	if g.cfg.valCol != "" || ordinal {
		g.lay.ResetLeaves(g.t.parent, g.t.depth)
	} else {
		g.lay.Reset(g.t.parent, g.t.depth)
	}
	g.bx = grow(g.bx, n)
	if ordinal {
		g.breadthOn(cat, deepest)
	} else {
		copy(g.bx, g.lay.X)
		x.Train(0, 1)
	}

	if g.err = g.heights(y, deepest); g.err != nil {
		return g.err
	}
	g.cfg.trainColors(g.t.s)
	return nil
}

// breadthOn places the leaves at their own categories and every parent over
// the span of its children, deepest first so a child is placed before its
// parent reads it.
//
// The leaves are encoded in walk order, which is what gives an axis that is
// still discovering its categories the dendrogram's order rather than the
// table's.
func (g *treeGeom) breadthOn(cat scale.Categorical, deepest int) {
	lo, hi := grow(g.t.lo, len(g.bx)), grow(g.t.hi, len(g.bx))
	for i := range g.bx {
		g.bx[i], lo[i], hi[i] = math.NaN(), math.Inf(1), math.Inf(-1)
	}
	for _, leaf := range g.lay.Leaves {
		g.bx[leaf] = cat.Encode(g.t.keys[leaf])
	}
	for d := deepest; d >= 0; d-- {
		for i, di := range g.t.depth {
			if di != d {
				continue
			}
			if lo[i] <= hi[i] {
				g.bx[i] = (lo[i] + hi[i]) / 2
			}
			if p := g.t.parent[i]; d > 0 && !math.IsNaN(g.bx[i]) {
				lo[p], hi[p] = math.Min(lo[p], g.bx[i]), math.Max(hi[p], g.bx[i])
			}
		}
	}
	g.t.lo, g.t.hi = lo, hi
}

// heights resolves every node's height and trains the Y axis on them.
func (g *treeGeom) heights(y scale.Scale, deepest int) error {
	g.h = grow(g.h, len(g.bx))
	lo, hi := 0.0, float64(deepest)
	if g.cfg.valCol != "" {
		lo, hi = math.Inf(1), math.Inf(-1)
		for i, v := range g.t.val {
			if !finite(v) {
				return fmt.Errorf("figure/geom: %q has no height in %q, and a tree draws every node at one", g.t.keys[i], g.cfg.valCol)
			}
			g.h[i] = v
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	} else {
		for i, d := range g.t.depth {
			g.h[i] = float64(d)
		}
	}
	if g.cfg.baseline >= 0.5 {
		for i := range g.h {
			g.h[i] = lo + hi - g.h[i]
		}
	}
	if len(g.h) > 0 {
		y.Train(lo, hi)
	}
	return nil
}

func (g *treeGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	sc := acquire(f)
	defer sc.release()
	cd := f.Coords()
	at := func(v, h float64) ir.Point { return cd.Point(f.X.Map(v), f.Y.Map(h)) }

	op := 1.0
	if g.cfg.opacity >= 0 {
		op = clamp01(g.cfg.opacity)
	}
	stroke := ir.Stroke{
		Color: ir.Fade(g.cfg.colorFor(f), op),
		Width: pick(g.cfg.width, f.Theme.LineWidth),
		Cap:   ir.CapButt,
		Join:  ir.JoinMiter,
		Dash:  g.cfg.dashFor(f),
	}
	cols := sc.colorsFor(g.cfg, g.t.s, indexes(sc, len(g.bx)))

	// One stroke per run of branches that share a colour, in table order. A
	// clade is usually a run of rows, so a coloured tree is a handful of
	// calls, and an uncoloured one is one.
	path := &sc.line
	path.Reset()
	current := stroke.Color
	flush := func() {
		if len(path.Ops) > 0 && current.A > 0 {
			st := stroke
			st.Color = current
			b.StrokePath(path, st)
		}
		path.Reset()
	}
	for i, p := range g.t.parent {
		if p < 0 || !g.placed(i) || !g.placed(p) {
			continue
		}
		if cols != nil {
			if c := ir.Fade(cols[i], op); c != current {
				flush()
				current = c
			}
		}
		g.branch(path, cd, at, i, p)
	}
	flush()

	if f.tracking() {
		pts, rows := sc.pts[:0], sc.rows[:0]
		for i := range g.bx {
			if g.placed(i) {
				pts = append(pts, at(g.bx[i], g.h[i]))
				rows = append(rows, i)
			}
		}
		sc.pts, sc.rows = pts, rows
		f.Marks(MarkRows{At: pts, Rows: sc.sourceRows(g.t.s, rows)})
	}
	return nil
}

func (g *treeGeom) placed(i int) bool { return finite(g.bx[i]) && finite(g.h[i]) }

// branch appends the path from node i to its parent p.
func (g *treeGeom) branch(path *ir.Path, cd coord.Coord, at func(v, h float64) ir.Point, i, p int) {
	start := at(g.bx[i], g.h[i])
	path.MoveTo(start.X, start.Y)
	if g.cfg.branch == Straight {
		cd.Edge(path, start, at(g.bx[p], g.h[p]))
		return
	}
	corner := at(g.bx[i], g.h[p])
	cd.Edge(path, start, corner)
	// The cross-piece is split the way a relational layout's spans are, so
	// that under a polar coord it follows the circle the short way round.
	from, to := g.bx[i], g.bx[p]
	steps := arcSteps(math.Abs(to - from))
	prev := corner
	for k := 1; k <= steps; k++ {
		next := at(from+(to-from)*float64(k)/float64(steps), g.h[p])
		cd.Edge(path, prev, next)
		prev = next
	}
}

func (g *treeGeom) ColorGuide() (ColorGuide, bool) { return g.cfg.colorGuide(g.t.s, g.err) }

// Legend is one line swatch, and only for a layer that was named: a tree's
// nodes are its reading, and a swatch per node would be a legend as long as
// the tree.
func (g *treeGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil || g.cfg.label == "" {
		return LegendEntry{}, false
	}
	return LegendEntry{
		Label: g.cfg.label,
		Color: g.cfg.colorFor(f),
		Kind:  SwatchLine,
		Dash:  g.cfg.dashFor(f),
		Width: pick(g.cfg.width, f.Theme.LineWidth),
	}, true
}

func (g *treeGeom) Source() data.Source { return g.src }

func (g *treeGeom) Subset(rows []int) Geom {
	return &treeGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *treeGeom) Describe() Desc {
	d := g.cfg.describe(MarkTree)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*treeGeom)(nil)
	_ Faceter   = (*treeGeom)(nil)
	_ Guided    = (*treeGeom)(nil)
)

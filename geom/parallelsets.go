package geom

import (
	"errors"
	"fmt"
	"math"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// DimensionSeparator joins a dimension's name to one of its categories to make
// the name of a node a [ParallelSets] diagram stands on.
//
// A node there is a category *in a dimension*, which is why the name is
// qualified: a table with a "yes" in two columns has two nodes and not one,
// and a legend that said "yes" twice would be naming neither of them.
const DimensionSeparator = ": "

// ParallelSets draws a table of categorical columns as the flow between them:
// one column of category boxes per dimension, and a ribbon between two boxes
// as thick as the number of rows that hold both.
//
//	p.Add(geom.ParallelSets(passengers,
//		geom.Dims("class", "sex", "survived"),
//		geom.Value("people"),
//		geom.ColorBy("survived", scale.Qualitative(palette.OkabeIto))))
//
// It is the chart for a table whose columns are categories rather than
// quantities — where a parallel-coordinates plot ([Parallel]) has a line per
// row, this has a ribbon per *combination*, because a hundred rows that agree
// on every category are one line drawn a hundred times and no reading at all.
//
// # It is a count, and the layout is the flow's
//
// Nothing here is laid out that was not already: [stat.Crosstab] counts the
// pairs of categories neighbouring columns hold, and [stat.Sankey] stacks them
// exactly as it stacks a flow — which it can do because the count *is* a flow,
// with the same total passing through every column. See
// docs/adr/0079-parallel-sets.md.
//
// Every column is scaled alike, so a ribbon's thickness means the same thing
// anywhere in the diagram. Two axes end level to within their gaps: an axis of
// five categories spends four of them where an axis of two spends one, and
// [Padding] is what that costs.
//
// # A colour column subdivides the ribbons
//
// Without one, a ribbon takes the colour of the box it leaves. With one, rows
// that agree on both categories and disagree on the colour column are drawn as
// separate ribbons, stacked in the class's own order — which is what lets a
// reader follow one class the length of the diagram. A coloured diagram
// therefore has more ribbons than an uncoloured one, rather than the same
// ribbons in an average of their colours.
//
// The boxes stay out of it. They are landmarks rather than series, so a
// diagram whose ribbons are painted from a colour column draws its boxes in
// the theme's label ink and leaves the palette to the ribbons.
//
// # Order, missing values, and what a hit reports
//
// Categories stack in the order they first appear going down the table, which
// is every relational mark's rule here and is what [Order] means by the
// caller's order: sort the rows to change it. The ribbons against one box are
// stacked in the order of the boxes at their far end, so they cross only where
// the data makes them.
//
// A row with no value in one of the dimensions is counted in none of them.
// It is the one place in this library where an absent value costs a row rather
// than gapping what it is part of, and the arithmetic is the reason: a
// [Parallel] line can break and resume because a line is one row, where each
// column here is a partition of the same total, and a column that added up to
// less than its neighbour would make every thickness in the diagram mean
// something slightly different.
//
// A ribbon is a count over many rows, so — like a [Sankey]'s node and an
// [Intersections] bar — it reports no row to a hit test. There is no single
// row behind it to report.
//
// Both axes describe the unit square, so give the chart a theme with no grid,
// no axis lines and no ticks.
func ParallelSets(src data.Source, opts ...Option) Geom {
	return &parallelSetsGeom{src: src, cfg: newConfig(opts)}
}

type parallelSetsGeom struct {
	src data.Source
	cfg config
	err error

	// The resolved table: one category index per row per dimension, which
	// column of the diagram each node stands in, and what each row weighs.
	// All of them are kept on the layer so that a chart redrawn every frame
	// allocates none of them.
	nodes  interner
	cats   [][]int
	column []int
	weight []float64
	ones   []float64

	// class is the colour column read as one class per row, and classValue
	// the encoded value each class was read from — which is what the colour
	// scale is trained on and painted from.
	class      []int
	classValue []float64
	classAt    map[float64]int

	// seen finds a category again within one dimension, so that the node's
	// qualified name is built once per category rather than once per row. It
	// is cleared between dimensions rather than replaced, like every other map
	// on a drawing path here.
	seen map[string]int

	x   stat.Crosstab
	lay stat.Sankey
	s   series
	// cs is the colour value per ribbon, kept here rather than taken from the
	// frame's pool because it is filled in Train, where there is no scratch.
	cs []float64
}

func (g *parallelSetsGeom) Train(t Training) error {
	g.err = g.resolve(t)
	return g.err
}

func (g *parallelSetsGeom) resolve(t Training) error {
	if g.src == nil {
		return errors.New("figure/geom: nil data source")
	}
	if len(g.cfg.dims) == 0 {
		return fmt.Errorf("%w: this mark reads a table of categorical columns; name them with geom.Dims", ErrNoColumn)
	}
	if len(g.cfg.dims) < 2 {
		return fmt.Errorf("%w: %q alone, and a crossing needs two columns to be between",
			ErrDimensions, g.cfg.dims[0])
	}
	if err := trainUnit(t.X, t.Y); err != nil {
		return err
	}
	n, err := g.intern()
	if err != nil {
		return err
	}
	if g.weight, g.ones, err = magnitudes(g.src, g.cfg, n, g.weight, g.ones); err != nil {
		return err
	}
	if err := g.classes(n); err != nil {
		return err
	}

	g.x.Reset(g.cats, g.classColumn(), g.weight)
	g.lay.ResetColumns(g.x.From, g.x.To, g.x.Value, g.column, g.nodes.count(), g.cfg.padding)
	g.ribbonColors()
	g.cfg.trainColors(g.s)
	return nil
}

// intern reads the dimension columns and gives every category in every one of
// them a node.
//
// It reads a whole dimension before it starts the next, so a column's nodes are
// a run of consecutive indices and the categories inside one stack in the order
// a reader meets them going down the table. Reading row by row instead would
// interleave the dimensions, which changes nothing about the picture and makes
// every slice here harder to reason about than it needs to be.
func (g *parallelSetsGeom) intern() (int, error) {
	g.nodes.reset()
	if g.seen == nil {
		g.seen = make(map[string]int, 16)
	}
	g.cats = growColumnsInt(g.cats, len(g.cfg.dims))
	g.column = g.column[:0]
	n := 0
	for d, name := range g.cfg.dims {
		labels, ok := data.Labels(g.src, name)
		if !ok {
			return 0, fmt.Errorf("%w: %q", ErrNoColumn, name)
		}
		if d == 0 {
			n = len(labels)
		} else if len(labels) != n {
			return 0, errLength(g.cfg.dims[0], name, n, len(labels))
		}
		null, _ := data.NullMask(g.src, name)

		g.cats[d] = grow(g.cats[d], n)
		clear(g.seen)
		for i, l := range labels {
			if data.IsNull(null, i) || l == "" {
				// A row with no category here is in no crossing at all, which
				// is [stat.Crosstab]'s rule and this chart's arithmetic.
				g.cats[d][i] = -1
				continue
			}
			j, known := g.seen[l]
			if !known {
				j = g.nodes.of(name+DimensionSeparator+l, i)
				g.seen[l] = j
				for len(g.column) <= j {
					g.column = append(g.column, d)
				}
			}
			g.cats[d][i] = j
		}
	}
	return n, nil
}

// classes reads the colour column as one class per row, remembering the value
// each class was read from so that the scale can be trained on the classes
// rather than on the rows.
//
// A row whose colour value is missing is class −1: it keeps its ribbon and is
// painted whatever the scale answers for a value it has no category for, which
// is how every layer here draws a row it cannot classify.
func (g *parallelSetsGeom) classes(n int) error {
	g.class, g.classValue = g.class[:0], g.classValue[:0]
	if g.cfg.colorCol == "" || g.cfg.colorScale == nil {
		return nil
	}
	cs, err := colorColumn(g.src, g.cfg)
	if err != nil {
		return err
	}
	if len(cs) != n {
		return errLength(g.cfg.dims[0], g.cfg.colorCol, n, len(cs))
	}
	if g.classAt == nil {
		g.classAt = make(map[float64]int, 8)
	}
	clear(g.classAt)
	g.class = grow(g.class, n)
	for i, v := range cs {
		if math.IsNaN(v) {
			g.class[i] = -1
			continue
		}
		j, seen := g.classAt[v]
		if !seen {
			j = len(g.classValue)
			g.classValue = append(g.classValue, v)
			g.classAt[v] = j
		}
		g.class[i] = j
	}
	return nil
}

// classColumn is the class per row, and nil for a layer that named no colour
// column — which is what [stat.Crosstab] reads as "every row is the same
// class". An empty slice is not that: it is a class column with no rows in it,
// and it would count nothing.
func (g *parallelSetsGeom) classColumn() []int {
	if len(g.class) == 0 {
		return nil
	}
	return g.class
}

// ribbonColors gives every ribbon the value its class was read from, which is
// the series a colour scale is trained on and painted from here. A ribbon is
// not a row, so this is the one place a series' values are not a column.
func (g *parallelSetsGeom) ribbonColors() {
	g.s = series{origin: data.Origins(g.src)}
	if g.cfg.colorScale == nil || len(g.classValue) == 0 {
		return
	}
	g.cs = grow(g.cs, len(g.x.Class))
	for e, c := range g.x.Class {
		if c < 0 || c >= len(g.classValue) {
			g.cs[e] = math.NaN()
			continue
		}
		g.cs[e] = g.classValue[c]
	}
	g.s.c = g.cs
}

func (g *parallelSetsGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	sc := acquire(f)
	defer sc.release()
	cd := f.Coords()
	w := flowWidth(g.cfg.thickness, g.lay.Layers)

	// Ribbons first and boxes on top of them, exactly as a sankey draws: a box
	// is a landmark and should not be hidden by what passes it.
	g.ribbons(b, sc, cd, f, w)
	return g.boxes(b, sc, cd, f, w)
}

func (g *parallelSetsGeom) boxes(b ir.Backend, sc *scratch, cd coord.Coord, f Frame, w float64) error {
	rects := sc.rects[:0]
	rows := sc.rows[:0]
	for i, n := range g.lay.Nodes {
		if !(n.Hi > n.Lo) {
			continue
		}
		x := flowColumnAt(n.Layer, g.lay.Layers, w)
		rects = append(rects, ir.R(f.X.Map(x), f.Y.Map(n.Lo), f.X.Map(x+w), f.Y.Map(n.Hi)))
		rows = append(rows, i)
	}
	sc.rects, sc.rows = rects, rows
	if len(rects) == 0 {
		return nil
	}
	cols := sc.cols[:0]
	for _, i := range rows {
		cols = append(cols, g.cfg.fillOf(g.boxColor(f, i), 1))
	}
	sc.cols = cols
	for _, run := range sc.groupByRect(rects, cols, nil) {
		if run.color.A == 0 {
			continue
		}
		sc.fill.Reset()
		for _, r := range run.rects {
			areaRound(&sc.fill, cd, r, ir.Point{}, g.cfg.corner)
		}
		g.cfg.fillMark(b, &sc.fill, f, 0, run.color)
	}
	// Neither a box nor a ribbon is a row — a box is what a great many rows
	// have in common and a ribbon is a count over them — so this mark reports
	// no rows at all. See docs/adr/0015-hit-testing.md.
	return nil
}

// boxColor is the ink one category's box is drawn in.
//
// It is the palette walk every relational mark uses, except where the layer
// named a colour column: there the palette belongs to the ribbons, and a box
// drawn out of the same palette would read as a class it is not. The theme's
// label ink is what a landmark is drawn in.
func (g *parallelSetsGeom) boxColor(f Frame, i int) ir.Color {
	if g.cfg.color != nil {
		return *g.cfg.color
	}
	if g.cfg.fill != nil {
		return *g.cfg.fill
	}
	if g.cfg.colorCol != "" && g.cfg.colorScale != nil {
		return f.Theme.LabelColor
	}
	return g.cfg.nodeColor(f, g.nodes.keys, i)
}

func (g *parallelSetsGeom) ribbons(b ir.Backend, sc *scratch, cd coord.Coord, f Frame, w float64) {
	opacity := sankeyLinkOpacity
	if g.cfg.opacity >= 0 {
		opacity = g.cfg.opacity
	}
	cols := sc.colorsFor(g.cfg, g.s, indexes(sc, len(g.lay.Flows)))
	for e, fl := range g.lay.Flows {
		if !(fl.SrcHi > fl.SrcLo) {
			continue
		}
		src, dst := g.x.From[e], g.x.To[e]
		x0 := flowColumnAt(g.lay.Nodes[src].Layer, g.lay.Layers, w) + w
		x1 := flowColumnAt(g.lay.Nodes[dst].Layer, g.lay.Layers, w)

		// The node's colour is asked for only when there is no colour column,
		// because asking a discrete scale for a name it has never seen
		// registers that name — which would put every node into a legend that
		// is naming the colour column's categories, and shift the palette
		// under them.
		var col ir.Color
		if cols != nil {
			col = cols[e]
		} else {
			col = g.cfg.nodeColor(f, g.nodes.keys, src)
		}
		col = g.cfg.fillOf(col, opacity)
		if col.A == 0 {
			continue
		}
		sc.fill.Reset()
		ribbon(&sc.fill, cd, f, x0, x1, fl.SrcLo, fl.SrcHi, fl.DstLo, fl.DstHi)
		g.cfg.fillMark(b, &sc.fill, f, 0, col)
	}
}

func (g *parallelSetsGeom) ColorGuide() (ColorGuide, bool) { return g.cfg.colorGuide(g.s, g.err) }

// Legends is one entry per class where the ribbons are painted from a discrete
// colour column — the classes are what the colours mean — and one entry per
// category otherwise, which is the only naming an uncoloured diagram has.
func (g *parallelSetsGeom) Legends(f Frame) []LegendEntry {
	if g.err != nil {
		return nil
	}
	if d, discrete := scale.Discrete(g.cfg.colorScale); discrete && g.s.c != nil {
		labels := d.Labels()
		out := make([]LegendEntry, 0, len(labels))
		for _, l := range labels {
			out = append(out, g.cfg.boxSwatch(f, l, d.ColorOf(l)))
		}
		return LegendsOr(g, f, out)
	}
	return LegendsOr(g, f, g.cfg.nodeLegends(f, g.nodes.keys))
}

func (g *parallelSetsGeom) Legend(f Frame) (LegendEntry, bool) {
	return oneNodeLegend(g.cfg, f, g.err)
}

func (g *parallelSetsGeom) Source() data.Source { return g.src }

func (g *parallelSetsGeom) Subset(rows []int) Geom {
	return &parallelSetsGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *parallelSetsGeom) Describe() Desc {
	d := g.cfg.describe(MarkParallelSets)
	d.Source = g.src
	return d
}

// growColumnsInt is [growColumns] for the category indices: one slice per
// dimension, kept between frames so that a chart redrawn every frame reuses
// every one of them.
func growColumnsInt(cols [][]int, n int) [][]int {
	if cap(cols) < n {
		next := make([][]int, n)
		copy(next, cols)
		return next
	}
	return cols[:n]
}

var (
	_ Describer = (*parallelSetsGeom)(nil)
	_ Faceter   = (*parallelSetsGeom)(nil)
	_ Guided    = (*parallelSetsGeom)(nil)
	_ Legender  = (*parallelSetsGeom)(nil)
)

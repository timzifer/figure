package geom

import (
	"fmt"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// Contour strokes the level sets of a value sampled over a regular grid.
//
// It is the reading a heatmap of the same table cannot give and a surface
// hides. A heatmap says what every cell holds and nothing about the slope
// between two of them; a surface shows the shape and makes a number hard to
// read off. A contour answers the question a map answers: where does this
// quantity cross *this* value, and how steeply — close lines are a cliff and
// far ones are a plain.
//
//	p.Add(geom.Contour(src,
//		geom.X("bias"), geom.Y("drive"), geom.Z("gain"),
//		geom.LevelCount(8), geom.ColorBy("gain", ramp)))
//
// The grid is required rather than guessed, exactly as a surface's is: the rows
// must be the full product of the distinct x and y values with each cell
// present once, and anything else is an error out of Train rather than a
// picture with holes in it. A cell whose value is missing is a different
// matter — the lines stop at its edge, because a contour through a number
// nobody measured is a contour through a guess.
//
// # It is the same lines as a surface's floor
//
// [github.com/timzifer/figure/three.Contour] draws these runs on the floor of a
// projected cube. Handed the same [Levels] and the same
// [github.com/timzifer/figure/scale.ColorScale] the two are the same reading,
// which is the point of them being one tracing in [stat.Contour] rather than
// two.
//
// # What it does not do
//
// Filled bands between levels, and labels along a line. Both are worth having
// and neither is this: a band is a polygon where this is a path, and a label on
// a curve is a placement problem with its own record. Say so here rather than
// leaving a caller to discover it.
func Contour(src data.Source, opts ...Option) Geom {
	return &contourGeom{src: src, cfg: newConfig(opts)}
}

type contourGeom struct {
	src data.Source
	cfg config

	// grid and lines are rebuilt every Train into buffers the layer keeps,
	// which is what stops a chart redrawn per pointer move allocating per row.
	grid  stat.Lattice
	lines stat.Contour
	// levels is the list actually traced: the caller's, or the one chosen from
	// the data.
	levels []float64

	ramp scale.ColorScale
	err  error

	// pts is the device-space scratch one run is mapped into.
	pts []ir.Point
}

func (g *contourGeom) Train(t Training) error {
	g.err = g.resolve(t)
	return g.err
}

// resolve reads the three columns, lays them out as a grid and traces it.
//
// It runs in Train rather than in Build for ADR 0028's reason rather than ADR
// 0011's: what a contour computes *is* what the axes describe, so it has to
// happen before the domains are settled. It is also where an error can still be
// reported, and where the colour guide's scale has to be trained — a guide is
// measured before the plot rectangle exists, so a ramp trained in Build would
// be trained too late to draw a colourbar.
func (g *contourGeom) resolve(t Training) error {
	if g.src == nil {
		return fmt.Errorf("figure/geom: nil data source")
	}
	xs, err := column(g.src, g.cfg.xcol, t.X)
	if err != nil {
		return err
	}
	ys, err := column(g.src, g.cfg.ycol, t.Y)
	if err != nil {
		return err
	}
	if g.cfg.zcol == "" {
		return fmt.Errorf("%w: a contour needs the column it draws the levels of (use geom.Z)", ErrNoColumn)
	}
	zs, ok := data.Float64Column(g.src, g.cfg.zcol)
	if !ok {
		return fmt.Errorf("%w: %q, which has to be a number", ErrNoColumn, g.cfg.zcol)
	}

	switch f := g.grid.Reset(xs, ys, zs); f {
	case stat.LatticeOK:
	case stat.LatticeRagged:
		return fmt.Errorf("figure/geom: the contour's columns have %d, %d and %d rows",
			len(xs), len(ys), len(zs))
	case stat.LatticeTooSmall:
		return fmt.Errorf("figure/geom: a contour needs at least two distinct values on each axis, got %d and %d",
			len(g.grid.Xs), len(g.grid.Ys))
	case stat.LatticeWrongCount:
		nx, ny := len(g.grid.Xs), len(g.grid.Ys)
		return fmt.Errorf("figure/geom: a contour needs one row per cell of its grid: "+
			"%d by %d is %d cells and the table has %d rows", nx, ny, nx*ny, len(zs))
	case stat.LatticeBadPosition:
		r := g.grid.At
		return fmt.Errorf("figure/geom: row %d of the contour is at (%v, %v), which is not a position",
			r, xs[r], ys[r])
	default:
		a, b := g.grid.At, g.grid.With
		return fmt.Errorf("figure/geom: rows %d and %d are both at (%v, %v); a contour has one value per cell",
			a, b, xs[b], ys[b])
	}

	g.ramp = g.cfg.colorScale
	g.levels = g.levelsFor()
	g.lines.Reset(g.grid.Xs, g.grid.Ys, g.grid.V, g.levels)

	trainColumn(t.X, g.grid.Xs)
	trainColumn(t.Y, g.grid.Ys)
	if g.ramp != nil {
		g.ramp.Train(g.levelsOrValues()...)
	}
	return nil
}

// levelsFor is the levels to trace: the ones the caller named, or a round set
// chosen from the data.
//
// Chosen from the data and not from the chart, which is [stat.Levels]'s own
// argument: a chart whose number of isolines changed when it was resized would
// be a chart whose reading depended on its size.
func (g *contourGeom) levelsFor() []float64 {
	if len(g.cfg.levels) > 0 {
		return g.cfg.levels
	}
	lo, hi := fieldExtent(g.grid.V)
	return stat.AppendLevels(g.levels, lo, hi, orElseInt(g.cfg.levelCount, DefaultLevels))
}

// levelsOrValues is what the ramp is trained on: the levels when there are any,
// and the field itself when there are none.
//
// The levels rather than the values is the right default and is worth saying
// why. A contour draws its levels and nothing between them, so a ramp stretched
// over the whole field would give the outermost isoline a colour from the middle
// of it — and the colourbar beside the chart would run to values no line is
// drawn at. Pin the ends with
// [github.com/timzifer/figure/scale.ColorDomain] to say otherwise, which is
// also what makes this chart and a surface of the same table comparable.
func (g *contourGeom) levelsOrValues() []float64 {
	if len(g.levels) > 0 {
		return g.levels
	}
	return g.grid.V
}

// DefaultLevels is how many intervals a contour asks [stat.Levels] for when the
// caller named neither the levels nor a count.
//
// Six is enough to read a shape off and few enough to tell apart. More is what
// [LevelCount] is for.
const DefaultLevels = 6

func (g *contourGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	cd := f.Coords()
	width := pick(g.cfg.width, f.Theme.LineWidth)
	dash := g.cfg.dashFor(f)

	// One path per level, which is one path per colour: the runs come out of
	// stat grouped by level, so batching them is a walk rather than a sort.
	// Each run is its own subpath, so a hit index sees one mark per line.
	var path ir.Path
	level, open := 0.0, false
	flush := func() {
		if !open || path.Empty() {
			return
		}
		b.StrokePath(&path, ir.Stroke{Color: g.colorAt(f, level), Width: width, Dash: dash})
		path.Reset()
	}
	for _, run := range g.lines.Lines {
		if !open || run.Level != level {
			flush()
			level, open = run.Level, true
		}
		g.pts = g.pts[:0]
		for _, p := range g.lines.Line(run) {
			g.pts = append(g.pts, cd.Point(float32(f.X.Map(p.X)), float32(f.Y.Map(p.Y))))
		}
		if len(g.pts) < 2 {
			continue
		}
		path.Polyline(g.pts)
	}
	flush()
	return nil
}

// colorAt is the ink one level is drawn in: the ramp's reading of it, or the
// layer's own colour where there is no ramp.
func (g *contourGeom) colorAt(f Frame, level float64) ir.Color {
	if g.ramp != nil {
		return g.ramp.Color(level)
	}
	return g.cfg.colorFor(f)
}

// Levels reports the levels the layer traced, which is what a caller handing
// the same list to a projected scene reads back.
func (g *contourGeom) Levels() []float64 { return g.levels }

func (g *contourGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.cfg.label == "" {
		return LegendEntry{}, false
	}
	return LegendEntry{
		Label: g.cfg.label, Kind: SwatchLine,
		Color: g.colorAt(f, midOf(g.levels)),
		Width: pick(g.cfg.width, f.Theme.LineWidth),
		Dash:  g.cfg.dashFor(f),
	}, true
}

// ColorGuide implements [Guided]: a contour with a ramp gets a colourbar
// without being asked for one.
//
// It can, where [Hexbin] cannot, and the difference is when the numbers are
// known: a hexbin's counts depend on how large the plot rectangle turned out to
// be, and a contour's levels are settled in Train, before anything is measured.
func (g *contourGeom) ColorGuide() (ColorGuide, bool) {
	if g.ramp == nil {
		return ColorGuide{}, false
	}
	label := g.cfg.colorCol
	if label == "" {
		label = g.cfg.zcol
	}
	return ColorGuide{Label: label, Scale: g.ramp}, true
}

func (g *contourGeom) Source() data.Source { return g.src }

func (g *contourGeom) Describe() Desc {
	d := g.cfg.describe(MarkContour)
	d.Source = g.src
	// The levels the layer is actually tracing rather than the ones it was
	// given, so that a document written from a chart that chose its own reads
	// back as the same lines. Before Train there is nothing to say and the
	// configured list stands, which is what a description asked for on a layer
	// that has not drawn yet has always answered.
	if len(g.levels) > 0 {
		d.Levels = g.levels
	}
	return d
}

// fieldExtent is the smallest and largest finite value of a field, or an empty
// range when it has none.
func fieldExtent(vs []float64) (lo, hi float64) {
	first := true
	for _, v := range vs {
		if v != v || v-v != 0 { // NaN or infinite
			continue
		}
		if first {
			lo, hi, first = v, v, false
			continue
		}
		lo, hi = min(lo, v), max(hi, v)
	}
	return lo, hi
}

// midOf is the middle level, which is the one a legend swatch is drawn in.
func midOf(levels []float64) float64 {
	if len(levels) == 0 {
		return 0
	}
	return levels[len(levels)/2]
}

func orElseInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}

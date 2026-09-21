package geom

import (
	"errors"
	"fmt"
	"math"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// ErrDimensions reports a layer whose columns do not match the axes of the
// coord it is drawn in.
//
// A parallel-coordinates panel has one axis per dimension and the two lists
// are matched in order, so a layer naming four columns in a coord with three
// axes has a column nothing is drawn against — which is a chart with a reading
// missing rather than a chart to guess at.
var ErrDimensions = errors.New("figure/geom: this mark needs one column per axis of its coord")

// Parallel draws one line per row across a panel's axes, crossing each at that
// row's value on it.
//
// It is the parallel-coordinates plot, and it is the chart for a table with
// more measured quantities than a pair of axes can hold: every row is a line,
// and what a reader is after is the shape the lines make together — where they
// converge, where they cross, which few of them run against the rest.
//
//	p := figure.New(figure.Coord(coord.Parallel(
//		coord.Dim("mpg", scale.Linear()),
//		coord.Dim("power", scale.Linear()),
//		coord.Dim("weight", scale.Linear()),
//	)))
//	p.Add(geom.Parallel(cars,
//		geom.Dims("mpg", "power", "weight"),
//		geom.ColorBy("origin", scale.Qualitative(palette.OkabeIto))))
//
// # Each axis has its own domain, and that is the whole point
//
// A radar draws its spokes against one shared scale, which is right when the
// spokes are one quantity measured several times and wrong when they are
// different quantities. Here every axis is its own scale, held by the coord
// ([github.com/timzifer/figure/coord.Parallel]) and trained by this layer on
// the column it was matched with — so miles per gallon and kilograms sit on
// one panel without either being squashed into the other's range.
//
// The columns are named by [Dims] and matched to the coord's axes in order.
// A layer whose count does not match is [ErrDimensions].
//
// # A row is a line
//
// So a row is reported at every axis it crosses, and a pointer anywhere along
// a line finds that row — the rule an extruded bar already follows, where a
// row is reported once per face. A row missing a value on one axis breaks
// there and resumes after it, which is the missing-data policy every layer has
// followed since v0.1: a line drawn straight through the gap would assert a
// value nobody measured.
//
// # What it does not do
//
// It draws no categorical axis, no ribbons between axes — that is a parallel
// *sets* diagram, which is a flow layout rather than this — and it does not
// reorder the axes. Which axis stands where is the order the caller declared
// them in, because an order chosen by an optimiser is not a pure function of
// the input. See docs/adr/0078-a-coord-with-more-than-two-axes.md.
func Parallel(src data.Source, opts ...Option) Geom {
	return &parallelGeom{src: src, cfg: newConfig(opts)}
}

type parallelGeom struct {
	src data.Source
	cfg config
	err error

	// vals is one column per dimension, cols the colour column's encoded
	// values, and rows the scratch the build walks. All three are kept on the
	// layer so that a chart redrawn every frame allocates none of them.
	vals [][]float64
	cols []float64
	n    int

	// The per-frame scratch: the points of one row's line, one row index per
	// point, the colour runs and the index that finds one again. Kept on the
	// layer so that a chart redrawn every frame allocates none of them.
	pts  []ir.Point
	rows []int
	runs []lineRun
	at   map[ir.Color]int
}

func (g *parallelGeom) Train(t Training) error {
	g.err = g.resolve(t)
	return g.err
}

func (g *parallelGeom) resolve(t Training) error {
	if g.src == nil {
		return errors.New("figure/geom: nil data source")
	}
	if len(g.cfg.dims) == 0 {
		return fmt.Errorf("%w: name them with geom.Dims", ErrNoColumn)
	}
	if len(t.Dims) != len(g.cfg.dims) {
		return fmt.Errorf("%w: %d columns against %d axes",
			ErrDimensions, len(g.cfg.dims), len(t.Dims))
	}

	g.vals = growColumns(g.vals, len(g.cfg.dims))
	g.n = 0
	for i, name := range g.cfg.dims {
		vs, ok := data.Float64Column(g.src, name)
		if !ok {
			return fmt.Errorf("%w: %q, which has to be a number", ErrNoColumn, name)
		}
		if i == 0 {
			g.n = len(vs)
		} else if len(vs) != g.n {
			return errLength(g.cfg.dims[0], name, g.n, len(vs))
		}
		g.vals[i] = append(g.vals[i][:0], vs...)

		// Each axis is trained on its own column and on nothing else, which is
		// what makes the domains independent. The panel's own Y is pinned by
		// the coord and carries nothing.
		trainColumn(t.Dims[i], g.vals[i])
	}
	return g.resolveColors()
}

// resolveColors reads the colour column, which is one value per row: a row is
// a line here, so a layer coloured by a column paints whole lines rather than
// segments of one.
func (g *parallelGeom) resolveColors() error {
	g.cols = g.cols[:0]
	if g.cfg.colorCol == "" || g.cfg.colorScale == nil {
		return nil
	}
	cs, err := colorColumn(g.src, g.cfg)
	if err != nil {
		return err
	}
	if len(cs) != g.n {
		return errLength(g.cfg.dims[0], g.cfg.colorCol, g.n, len(cs))
	}
	g.cols = append(g.cols, cs...)
	g.cfg.colorScale.Train(g.cols...)
	return nil
}

func (g *parallelGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	cd := f.Coords()
	dims := coord.Dimensions(cd)
	if len(dims) != len(g.vals) || g.n == 0 {
		return nil
	}
	stroke := ir.Stroke{
		Color: g.cfg.colorFor(f),
		Width: pick(g.cfg.width, f.Theme.LineWidth),
		Cap:   ir.CapRound,
		Join:  ir.JoinRound,
		Dash:  g.cfg.dashFor(f),
	}
	if g.cfg.opacity >= 0 {
		stroke.Color = ir.Fade(stroke.Color, clamp01(g.cfg.opacity))
	}
	if !stroke.Visible() {
		return nil
	}

	sc := acquire(f)
	defer sc.release()

	// One path per colour and one subpath per row: a line is a row, so a
	// pointer lands on the line it is over rather than on the sheet the batch
	// was drawn as — the rule a bubble already follows, per ADR 0015.
	for _, run := range g.grouped(stroke.Color) {
		sc.line.Reset()
		for _, row := range run.rows {
			g.appendRow(&sc.line, cd, dims, f, row)
		}
		if sc.line.Empty() {
			continue
		}
		st := stroke
		st.Color = run.color
		b.StrokePath(&sc.line, st)
	}
	return nil
}

// appendRow adds one row's line, broken where the row has no value.
func (g *parallelGeom) appendRow(path *ir.Path, cd coord.Coord, dims []coord.ParallelDim, f Frame, row int) {
	g.pts = g.pts[:0]
	flush := func() {
		if len(g.pts) >= 2 {
			appendEdges(path, cd, g.pts, true)
		}
		if f.tracking() && len(g.pts) > 0 {
			// Every vertex a row was drawn at reports that row: a line is one
			// row, so a pointer anywhere along it finds the same one.
			f.Marks(MarkRows{At: g.pts, Rows: g.sameRow(row, len(g.pts))})
		}
		g.pts = g.pts[:0]
	}
	for i, dim := range dims {
		v := g.vals[i][row]
		if !finite(v) || dim.Scale == nil || !defined(dim.Scale, v) {
			flush()
			continue
		}
		at := float32(dim.Scale.Map(v))
		if math.IsNaN(float64(at)) {
			flush()
			continue
		}
		g.pts = append(g.pts, cd.Point(float32(i), at))
	}
	flush()
}

// sameRow is one row index per point, which is what a mark whose marks are all
// one row reports.
func (g *parallelGeom) sameRow(row, n int) []int {
	g.rows = grow(g.rows, n)[:0]
	for range n {
		g.rows = append(g.rows, row)
	}
	return g.rows
}

// lineRun is the rows drawn in one colour, in row order.
type lineRun struct {
	color ir.Color
	rows  []int
}

// runs groups the rows by the colour they are drawn in.
//
// Grouped rather than one call per row for the reason every other batched mark
// is: the IR carries one style per drawing call, so a thousand rows in three
// colours are three calls. Rows keep their order within a colour, and the
// colours come out in the order their first row appears — never a map's.
func (g *parallelGeom) grouped(base ir.Color) []lineRun {
	g.runs = resetRuns(g.runs)
	if len(g.cols) != g.n {
		g.runs = append(g.runs, lineRun{color: base})
		for i := range g.n {
			g.runs[0].rows = append(g.runs[0].rows, i)
		}
		return g.runs
	}
	if g.at == nil {
		g.at = make(map[ir.Color]int, 8)
	}
	clear(g.at)
	for i := range g.n {
		col := g.cfg.colorScale.Color(g.cols[i])
		if g.cfg.opacity >= 0 {
			col = ir.Fade(col, clamp01(g.cfg.opacity))
		}
		j, seen := g.at[col]
		if !seen {
			j = len(g.runs)
			g.at[col] = j
			g.runs = growRuns(g.runs)
			g.runs[j].color = col
		}
		g.runs[j].rows = append(g.runs[j].rows, i)
	}
	return g.runs
}

// growRuns lengthens the run list by one, reusing the row buffer the last
// frame left past the length.
func growRuns(runs []lineRun) []lineRun {
	if len(runs) < cap(runs) {
		runs = runs[:len(runs)+1]
		runs[len(runs)-1].rows = runs[len(runs)-1].rows[:0]
		return runs
	}
	return append(runs, lineRun{})
}

// resetRuns empties every run the slice has ever held, so that a row buffer
// past the current length does not come back holding the last frame's rows.
func resetRuns(runs []lineRun) []lineRun {
	for i := range runs[:cap(runs)] {
		runs[:cap(runs)][i].rows = runs[:cap(runs)][i].rows[:0]
	}
	return runs[:0]
}

// Legends is one entry per colour category, which is what a layer coloured by
// a discrete column has to say: the lines are rows and a row is not a series,
// so the categories are the only names this chart has.
func (g *parallelGeom) Legends(f Frame) []LegendEntry {
	if g.err != nil {
		return nil
	}
	d, discrete := scale.Discrete(g.cfg.colorScale)
	if !discrete || len(g.cols) == 0 {
		return LegendsOr(g, f, nil)
	}
	labels := d.Labels()
	out := make([]LegendEntry, 0, len(labels))
	for _, l := range labels {
		out = append(out, LegendEntry{
			Label: l, Kind: SwatchLine,
			Color: d.ColorOf(l),
			Width: pick(g.cfg.width, f.Theme.LineWidth),
			Dash:  g.cfg.dashFor(f),
		})
	}
	return out
}

func (g *parallelGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil || g.cfg.label == "" {
		return LegendEntry{}, false
	}
	return LegendEntry{
		Label: g.cfg.label, Kind: SwatchLine,
		Color: g.cfg.colorFor(f),
		Width: pick(g.cfg.width, f.Theme.LineWidth),
		Dash:  g.cfg.dashFor(f),
	}, true
}

// ColorGuide implements [Guided]: a layer whose lines come from a continuous
// ramp gets a colourbar, because a swatch per row would be a ladder of
// thousands.
func (g *parallelGeom) ColorGuide() (ColorGuide, bool) {
	if g.cfg.colorScale == nil || g.cfg.hideGuide || len(g.cols) == 0 {
		return ColorGuide{}, false
	}
	if _, discrete := scale.Discrete(g.cfg.colorScale); discrete {
		return ColorGuide{}, false
	}
	return ColorGuide{Label: g.cfg.colorCol, Scale: g.cfg.colorScale}, true
}

func (g *parallelGeom) Source() data.Source { return g.src }

func (g *parallelGeom) Subset(rows []int) Geom {
	return &parallelGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *parallelGeom) Describe() Desc {
	d := g.cfg.describe(MarkParallel)
	d.Source = g.src
	return d
}

// growColumns keeps one buffer per dimension between frames.
func growColumns(buf [][]float64, n int) [][]float64 {
	if cap(buf) < n {
		next := make([][]float64, n)
		copy(next, buf)
		return next
	}
	return buf[:n]
}

var (
	_ Describer = (*parallelGeom)(nil)
	_ Faceter   = (*parallelGeom)(nil)
	_ Guided    = (*parallelGeom)(nil)
	_ Legender  = (*parallelGeom)(nil)
)

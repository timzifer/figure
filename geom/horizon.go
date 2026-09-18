package geom

import (
	"errors"
	"fmt"
	"math"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// Horizon folds a series' own axis: it cuts the values into bands of equal
// height, draws every band at the panel's full height, and tells the bands
// apart by colour instead of by position.
//
//	p.Y(scale.Linear())
//	p.Add(geom.Horizon(src, geom.X("t"), geom.Y("kw"), geom.Bands(3)))
//
// A chart one quarter as tall then keeps the resolution a chart four times as
// tall would have, because the vertical space is spent four times over. Values
// below the fold's origin are mirrored back above it and coloured from the
// other arm of the ramp, so a deviation reads as a colour and its size reads as
// a height.
//
// It is the mark for a question about *how many series* rather than about how
// many rows. Decimation, the density raster and the hexbin all answer "too many
// rows"; forty sensors at a thousand samples each is not a big series but a lot
// of them, and the only other answer in this library is faceting, which divides
// the space instead of reusing it. Heer, Kong and Agrawala measured the trade
// in 2009: below about forty pixels of chart height a horizon chart is read
// faster and more accurately than the filled line chart of the same series.
//
// # The bands
//
// [Bands] cuts the trained extent into k bands. [BandHeight] gives the band in
// the data's own units and wins where both are set, and it is the spelling to
// prefer for anything measured: a band that means 50 kW is a band a plant
// engineer already has, and a band that means "a third of whatever today's
// maximum turned out to be" is a band that changes when tomorrow's data
// arrives — which makes two charts of the same quantity incomparable, and that
// is the one thing this form exists to avoid. The default is
// [DefaultBands] bands over the data's own reach, and it is a default rather
// than a recommendation.
//
// [Baseline] is the origin the fold is measured from. It is 0 unless it is set,
// and it is what a chart of a deviation from a set point moves.
//
// # The axis it gives up, and the key it gets instead
//
// A horizon chart has no vertical ladder, and this library does not let a mark
// invent one: nothing is labelled that a scale did not write. Nothing has to
// be. After the fold the Y scale describes *one band* and its ticks are correct
// for every band on screen — a mark two thirds of the way up the panel really
// is two thirds of a band above that band's floor, whichever band it is in.
// What a reader is missing is not the ladder but *which* band, and that is a
// colour: the layer contributes a classed colourbar whose boundaries are the
// fold's own, printed in the data's units. That is the reading the axis gave
// up, and it needs no new furniture.
//
// [ColorBy]'s scale is used if one is given and the column named in it is not
// read, exactly as [Hexbin] does: the quantity being coloured is the layer's
// own distance from the origin. Left alone the layer builds a
// [scale.Threshold] over [palette.BlueOrange] cut at the band boundaries, which
// is one diverging ramp read from both ends.
//
// # One layer draws one series
//
// [GroupBy] is [ErrGroupedHorizon] rather than a silent overlap. The whole
// premise of the form is that a series gets a strip of its own, and N series in
// one panel would be N chart-height bands painted over each other — not a
// degraded horizon chart but a solid rectangle. A wall of them is what this
// library already builds walls with: a [github.com/timzifer/figure/facet] over
// the series column, or one [github.com/timzifer/figure.Plot.Track] per series
// with `TrackSize` naming the strip height.
//
// [Curve] and [Tension] are ignored. A spline through a folded value
// overshoots the top and the bottom of its band, which is ink outside the only
// interval the band has.
//
// See docs/adr/0065-horizon-charts.md.
func Horizon(src data.Source, opts ...Option) Geom {
	return &horizonGeom{src: src, cfg: newConfig(opts)}
}

// DefaultBands is how many bands a horizon layer folds into when the caller
// named neither a count nor a height.
//
// Three is the count the form is usually published at: enough that a quiet
// stretch and an extreme one are different colours, and few enough that the
// colours are still told apart at the chart height this mark exists for.
const DefaultBands = 3

// maxHorizonBands caps what a [BandHeight] much smaller than the data's reach
// can ask for. Past a few dozen the bands are thinner than the difference
// between two colours of the ramp, so the chart is a solid block and the
// arithmetic to draw it is unbounded — a mistyped height should draw something
// wrong rather than take a minute doing it.
const maxHorizonBands = 64

// ErrGroupedHorizon reports [GroupBy] on a horizon layer.
//
// It is an error rather than an overlap because the two pictures are not a
// good one and a worse one: every band of every series occupies the whole
// panel, so N series drawn together are a filled rectangle carrying no reading
// at all.
var ErrGroupedHorizon = errors.New("figure/geom: a horizon layer draws one series")

type horizonGeom struct {
	src data.Source
	cfg config
	s   series

	// The fold, resolved in Train: where it is measured from, how tall one
	// band is in the data's own units, and how many of them there are.
	origin, height float64
	bands          int

	// ramp is the classed scale the bands are painted from and the colourbar
	// is drawn off. It is settled in Train because a guide is measured before
	// there is a plot rectangle — see [contourGeom.resolve] for the same
	// reason said about the same seam.
	ramp scale.ColorScale

	// frac is one band's fractions, rebuilt per band per frame into a buffer
	// the layer keeps. It is on the layer rather than in the frame's pool
	// because it is sized by the data and a chart redrawn every frame would
	// otherwise allocate a column per band per frame.
	frac []float64

	err error
}

func (g *horizonGeom) Train(t Training) error {
	g.err = g.resolve(t)
	return g.err
}

// resolve reads the columns and settles the fold.
//
// It runs in Train for ADR 0028's reason rather than ADR 0011's, and in the
// strongest form of it: a distribution stat's output is what the axis
// describes, and the fold does not merely describe the vertical axis — it
// replaces it. After folding, the domain is [0, 1] and nothing from the table
// is plotted against it.
func (g *horizonGeom) resolve(t Training) error {
	x, y := t.X, t.Y
	if g.cfg.groupCol != "" {
		return fmt.Errorf("%w: %q names series and every band of every one of them would be the whole panel; "+
			"facet over it, or give each series a figure.Track of its own", ErrGroupedHorizon, g.cfg.groupCol)
	}
	s, err := resolve(g.src, g.cfg, x, y)
	if err != nil {
		return err
	}
	g.s = s
	if err := s.checkMissing(g.cfg, x, y); err != nil {
		return err
	}
	trainColumn(x, s.x)

	g.origin = g.cfg.baseline
	lo, hi, ok := extent(s.y)
	if !ok {
		lo, hi = g.origin, g.origin
	}
	g.bands, g.height = g.foldFor(lo, hi)
	g.ramp = g.colorScale()

	// The fold replaces the vertical axis rather than describing it: what is
	// plotted against Y is how far into one band a value reached, and every
	// band is the whole panel.
	y.Train(0, 1)
	return nil
}

// foldFor settles how many bands there are and how tall one is.
//
// A pinned height wins, and the count then comes from the data — which is the
// point of pinning it: the bands mean the same thing in every chart drawn this
// way, and how many of them a given day needs is the reading.
func (g *horizonGeom) foldFor(lo, hi float64) (bands int, height float64) {
	if h := g.cfg.bandHeight; h > 0 {
		return min(stat.FoldBands(lo, hi, g.origin, h), maxHorizonBands), h
	}
	n := g.cfg.bands
	if n < 1 {
		n = DefaultBands
	}
	if n > maxHorizonBands {
		n = maxHorizonBands
	}
	h := stat.FoldHeight(lo, hi, g.origin, n)
	if !(h > 0) {
		// A column that never leaves its origin has no reach to divide. One
		// band of unit height draws the flat line it is, rather than dividing
		// by zero on the way to drawing nothing.
		return 1, 1
	}
	return n, h
}

// colorScale is the scale the bands are painted from.
//
// The caller's if they named one — the column in it is not read, because the
// quantity being coloured is the layer's own distance from the origin, which
// is [Hexbin]'s precedent exactly. Otherwise a threshold scale cut at the
// fold's own boundaries, over one diverging ramp: the two arms of a fold are
// the two arms of the ramp, which is what makes "well above" and "well below"
// different colours rather than the same depth twice.
func (g *horizonGeom) colorScale() scale.ColorScale {
	if g.cfg.colorScale != nil {
		return g.cfg.colorScale
	}
	cs := scale.Threshold(palette.BlueOrange, g.breaks())
	// The ends, so that the outermost class covers everything beyond the last
	// boundary rather than stopping where the boundaries do.
	reach := float64(g.bands) * g.height
	cs.Train(g.origin-reach, g.origin+reach)
	return cs
}

// breaks are the fold's boundaries in the data's own units: every band edge
// from the outermost below the origin to the outermost above it, the origin
// among them.
//
// They are the numbers printed down the side of the colourbar, and they are the
// ladder the chart gave up when it folded its axis.
func (g *horizonGeom) breaks() []float64 {
	out := make([]float64, 0, 2*g.bands-1)
	for j := -(g.bands - 1); j <= g.bands-1; j++ {
		out = append(out, g.origin+float64(j)*g.height)
	}
	return out
}

func (g *horizonGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	sc := acquire(f)
	defer sc.release()

	cd := f.Coords()
	floor := f.Y.Map(0)

	// Fold first, then decimate — the order ADR 0011 and ADR 0028 settle
	// between them, and the one whose reverse is a bug with no symptom: a
	// reduction that dropped the sample deciding which band a run belongs to
	// would change the *colour* of a stretch of chart rather than its outline.
	// A band's ink is its extremes, so it reduces the way a staircase does.
	mode, budget := g.cfg.reduction(shapeStair, g.s, f)

	for _, arm := range [...]stat.FoldArm{stat.FoldAbove, stat.FoldBelow} {
		// Outwards, so that a deeper band is drawn over the full bands below
		// it. The two arms never overlap, so their order is not a question.
		for band := range g.bands {
			g.buildBand(b, f, sc, cd, floor, mode, budget, band, arm)
		}
	}
	g.reportRows(f, sc, cd)
	return nil
}

// buildBand draws one band of one arm: the part of each value that lies inside
// it, filled down to the band's floor.
func (g *horizonGeom) buildBand(b ir.Backend, f Frame, sc *scratch, cd coord.Coord,
	floor float32, mode Decimation, budget int, band int, arm stat.FoldArm,
) {
	fill := g.bandColor(band, arm)
	if fill.A == 0 {
		return
	}
	g.frac = stat.AppendFold(g.frac, g.s.y, g.origin, g.height, band, arm)
	if !reaches(g.frac) {
		// A band nothing reaches into. Drawing it anyway would emit a filled
		// run of no height per band per arm per frame — which is most of the
		// primitives of a chart whose series stays in one arm, and not one
		// pixel of picture.
		return
	}
	folded := series{x: g.s.x, y: g.frac, off: g.s.off, rows: g.s.rows, origin: g.s.origin}

	for _, seg := range sc.segments(folded, sc.plottable(folded, f.X, f.Y), g.cfg.missing) {
		x, y, _ := sc.project(seg, f)
		keep := sc.reduce(mode, budget, x, y, nil)
		top := sc.marks(cd, x, y, keep)
		if len(top) < 2 {
			continue
		}
		sc.fill.Reset()
		// Straight edges whatever [Curve] and [Tension] said: a spline through
		// a clamped fraction overshoots the band it is clamped to.
		sc.appendCurve(&sc.fill, cd, top, curveFit{}, true)
		appendFloor(&sc.fill, cd, x, keep, floor)
		sc.fill.Close()
		b.FillPath(&sc.fill, ir.Solid(fill), ir.NonZero)
	}
}

// reaches reports whether any value got into this band at all.
func reaches(frac []float64) bool {
	for _, v := range frac {
		if v > 0 {
			return true
		}
	}
	return false
}

// bandColor is the ink one band is painted in: the scale's reading of the
// middle of that band, in the data's own units.
//
// The middle rather than an edge, for [scale.Threshold]'s own reason — a class
// read at its lower boundary is the class below it.
func (g *horizonGeom) bandColor(band int, arm stat.FoldArm) ir.Color {
	return g.ramp.Color(g.origin + float64(arm)*(float64(band)+0.5)*g.height)
}

// reportRows attributes one position to each row: where it landed in the band
// it ends in.
//
// The fold cuts a row into up to k drawn spans, so the row-to-mark
// correspondence every other mark keeps is gone — a row is under the pointer in
// the band it reached, and full bands below that one are a stretch of solid
// colour rather than a reading. Reporting the terminal band is therefore the
// only answer that names one row per position, which is what [Rows] is for.
//
// It costs nothing unless somebody asked. See [Frame.Marks].
func (g *horizonGeom) reportRows(f Frame, sc *scratch, cd coord.Coord) {
	if !f.tracking() {
		return
	}
	n := len(g.s.x)
	sc.kx, sc.ky = grow(sc.kx, n)[:0], grow(sc.ky, n)[:0]
	sc.mrows = grow(sc.mrows, n)[:0]
	for i := range n {
		v := g.s.y[i]
		if !defined(f.X, g.s.x[i]) || !finite(v) {
			continue
		}
		arm := stat.FoldAbove
		if v < g.origin {
			arm = stat.FoldBelow
		}
		band := int(math.Floor(math.Abs(v-g.origin) / g.height))
		if band >= g.bands {
			// Past the last band the chart has: the value is drawn at the top
			// of the outermost one, and that is where it is reported.
			band = g.bands - 1
		}
		sc.kx = append(sc.kx, f.X.Map(g.s.x[i]))
		sc.ky = append(sc.ky, f.Y.Map(stat.Fold(v, g.origin, g.height, band, arm)))
		sc.mrows = append(sc.mrows, g.s.rowAt(i))
	}
	sc.pts = cd.Points(grow(sc.pts, len(sc.kx))[:0], sc.kx, sc.ky)
	f.Marks(MarkRows{At: sc.pts, Rows: sc.mrows})
}

// ColorGuide implements [Guided]: the classed colourbar that is the ladder this
// chart folded away.
//
// It can have one where [Hexbin] cannot, and the difference is when the numbers
// are known: a hexbin's counts depend on how large the plot rectangle turned
// out to be, and a fold's boundaries are settled in Train, before anything is
// measured.
func (g *horizonGeom) ColorGuide() (ColorGuide, bool) {
	if g.err != nil || g.ramp == nil || g.cfg.hideGuide {
		return ColorGuide{}, false
	}
	label := g.cfg.label
	if label == "" {
		label = g.cfg.ycol
	}
	return ColorGuide{Label: label, Scale: g.ramp}, true
}

// Legend is the layer's entry, and there is one only when the caller named the
// series.
//
// A horizon layer says what it is through its colourbar, where the band
// boundaries are printed in the data's units; a swatch beside it would pick one
// of the bands and label it with the column name, which names neither the
// series nor the band. A layer given a [Label] gets the entry anyway, because
// that is a caller naming the strip — which is exactly what a wall of these
// needs.
func (g *horizonGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil || g.cfg.label == "" {
		return LegendEntry{}, false
	}
	return LegendEntry{
		Label: g.cfg.label,
		Color: g.bandColor(g.bands-1, stat.FoldAbove),
		Kind:  SwatchBox,
	}, true
}

func (g *horizonGeom) Source() data.Source { return g.src }
func (g *horizonGeom) Subset(rows []int) Geom {
	return &horizonGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

// Describe reports the layer's configuration.
//
// The band count and the band height are the ones the caller asked for and not
// the ones the fold resolved, which is the opposite of [Contour]'s choice about
// its levels — and the difference is what each number means. A contour's chosen
// levels are the lines it drew, so a document that did not pin them would read
// back as different lines. A horizon's resolved count is a fact about the rows
// it happened to be given, so writing it down would turn a chart that asked for
// 50 kW bands into one pinned to however many of them today needed.
func (g *horizonGeom) Describe() Desc {
	d := g.cfg.describe(MarkHorizon)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*horizonGeom)(nil)
	_ Faceter   = (*horizonGeom)(nil)
	_ Guided    = (*horizonGeom)(nil)
)

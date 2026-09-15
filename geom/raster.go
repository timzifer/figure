package geom

import (
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// Raster draws a value sampled over a regular grid as one image.
//
// It is the mark a *measured field* needs, where [Rect] is the mark a table of
// categories needs. The two draw the same chart at different sizes: a heatmap
// of a few dozen categories by a few dozen is one labelled box per reading and
// [Rect] is right for it, and a spectrogram of one minute of audio is two
// thousand frames by five hundred bins — a million boxes through the IR, a
// million paths in the SVG, and a file no browser will open, to draw a picture
// whose every cell is smaller than a pixel. This layer emits one image.
//
//	p.Add(geom.Raster(src, geom.X("t"), geom.Y("hz"), geom.Z("db"),
//		geom.ColorBy("db", scale.Sequential(palette.Viridis))))
//
// # It reads the contour's channels, through the contour's lattice
//
// The three columns are the ones [Contour] takes, resolved by the same
// [stat.Lattice] and refused with the same messages. That is not tidiness: a
// raster with its own isolines over it is the most common form this mark
// appears in, and two resolvers that agree today disagree at the first
// duplicated position — so a field and the lines drawn on it come from one
// reading of the table. See docs/adr/0064-a-contour-and-its-lattice.md.
//
// # It has a colourbar, and that is the point
//
// [Hexbin] deliberately has none: its counts are not known until the plot
// rectangle is, and the guide column is measured before it. A raster's values
// are the data's, settled in Train, so the reader gets a labelled bar in the
// units they measured. Naming no ramp through [ColorBy] takes
// [palette.DefaultRamp] over the field's own extent; the column named in one is
// the bar's title rather than a second column this layer reads, because the
// quantity being coloured is [Z].
//
// # What it will not do
//
// **It will not smooth.** The image is built at the panel's own resolution with
// nearest neighbour, so a cell larger than a pixel is a block of that cell's
// colour and nothing in between — a smooth upscale would paint colours between
// two measurements, which is a chart drawing a value nobody recorded. Going the
// other way is a choice and is named rather than assumed: see [Resample].
//
// **It will not stretch an uneven lattice.** An image is a grid of equal cells,
// so samples at 1 s, 2 s and then 10 s are [ErrUnevenLattice] rather than a
// picture with the wrong cells stretched. [Rect] draws that table correctly,
// one box per row, and a table spaced that irregularly does not have a million
// rows in it.
//
// **It will not decimate.** [Decimate] on this layer is [ErrDecimatedRaster]:
// decimation reduces marks that would land on the same pixel column, and a
// raster's cell count *is* its resolution. The reduction that matters is
// [Resample], which is chosen in the data's terms rather than the device's.
//
// **It will not warp.** A blit is axis-aligned in device space, so a raster
// under a polar, ternary or Smith coord is [ErrWarpedRaster] rather than an
// image drawn square in a round panel.
//
// A cell whose value is not a number is left transparent, so the panel's own
// background and grid read through and a reader sees that nothing was measured
// there rather than seeing the bottom of the ramp.
//
// See docs/adr/0066-a-raster-mark.md.
func Raster(src data.Source, opts ...Option) Geom {
	return &rasterGeom{src: src, cfg: newConfig(opts)}
}

// Resampling names how a [Raster] combines the cells that land on one pixel.
//
// It is a named member of a closed family rather than a function the caller
// writes, which is the rule three records have now settled: a name serialises
// through the JSON spec and an arbitrary Go function does not. See
// docs/adr/0041-qq-plots.md.
type Resampling uint8

// The resamplings. Which one is right depends on the field rather than on the
// picture: a spectrum wants the peak to survive, a temperature map wants the
// mean.
const (
	// Nearest takes the cell the pixel's centre falls in. It is the default in
	// both directions, because it is the only one of the three that never
	// shows a number that was not measured.
	//
	// It is also the one that aliases: a lattice much finer than the panel
	// drops whole cells between one pixel centre and the next, so a tone one
	// bin wide survives or vanishes depending on where the panel's edges
	// happened to fall. That is what [Max] is for.
	Nearest Resampling = iota

	// Mean averages the cells that fall on the pixel, skipping the ones that
	// hold no reading. It is the resampling for a field that is a quantity
	// per area — a temperature, a concentration, an anomaly.
	Mean

	// Max takes the largest of them. It is the resampling for a field whose
	// reading is a peak: a spectrogram downsampled by a mean shows a smear
	// where the tone was.
	Max
)

// ErrUnevenLattice reports a raster whose lattice is not equally spaced.
//
// It is an error rather than an approximation because an image is a grid of
// equal cells: the alternatives are stretching the cells that are not at fault
// or inventing the readings that are missing, and both draw a field the
// instrument did not measure.
var ErrUnevenLattice = errors.New("figure/geom: a raster needs an equally spaced grid")

// ErrDecimatedRaster reports [Decimate] on a raster layer.
//
// A raster's resolution *is* its cell count, so there is nothing for a row
// reduction to reduce — see [Resample], which is the reduction this mark has.
var ErrDecimatedRaster = errors.New("figure/geom: a raster does not decimate")

// ErrWarpedRaster reports a raster under a coord that is not Cartesian.
//
// A blit is axis-aligned in device space. Drawing a field in a round panel is
// one quadrilateral per cell through the coordinate stage, which is a different
// mark rather than an option on this one.
var ErrWarpedRaster = errors.New("figure/geom: a raster is drawn under a Cartesian coord")

type rasterGeom struct {
	src data.Source
	cfg config

	// grid is rebuilt every Train into buffers the layer keeps, and step is
	// its spacing on each axis — the number that makes the grid an image.
	grid  stat.Lattice
	xstep float64
	ystep float64
	ramp  scale.ColorScale
	err   error

	// img is the panel-sized buffer the field is painted into, cols and rows
	// are the cells one pixel covers on each axis, and ink is the ramp read
	// into a table. All four are kept across frames: a chart redrawn per
	// pointer move repaints these pixels rather than allocating a new panel's
	// worth of them.
	img  *image.NRGBA
	cols []cellSpan
	rows []cellSpan
	ink  ink
}

// cellSpan is the half-open range of lattice cells one pixel of the panel
// covers on one axis, already resolved for the layer's [Resampling]: a single
// cell under [Nearest], and everything the pixel reaches under the other two.
// An empty span is a pixel the field does not cover.
type cellSpan struct{ lo, hi int }

func (g *rasterGeom) Train(t Training) error {
	g.err = g.resolve(t)
	return g.err
}

// resolve reads the three columns, lays them out as a grid and settles the
// ramp.
//
// It runs in Train for [contourGeom.resolve]'s reason: what the layer draws is
// what the axes describe, and the colour scale a guide is drawn from has to be
// trained before the guide column is measured — which happens before there is
// a plot rectangle to build an image in.
func (g *rasterGeom) resolve(t Training) error {
	if g.src == nil {
		return fmt.Errorf("figure/geom: nil data source")
	}
	if g.cfg.decimate != AutoDecimation {
		return fmt.Errorf("%w: its cells are its resolution; geom.Resample is the reduction it has",
			ErrDecimatedRaster)
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
		return fmt.Errorf("%w: a raster needs the column it draws (use geom.Z)", ErrNoColumn)
	}
	zs, ok := data.Float64Column(g.src, g.cfg.zcol)
	if !ok {
		return fmt.Errorf("%w: %q, which has to be a number", ErrNoColumn, g.cfg.zcol)
	}
	if err := latticeError("raster", g.grid.Reset(xs, ys, zs), &g.grid, xs, ys, zs); err != nil {
		return err
	}

	// The spacing, which is what an image needs and a contour does not. The
	// message names the column so that a caller can see which axis is the
	// uneven one, and names the mark that draws the table as it is.
	if g.xstep, ok = stat.Step(g.grid.Xs); !ok {
		return fmt.Errorf("%w: %q is sampled unevenly, which an image cannot carry; geom.Rect draws it",
			ErrUnevenLattice, g.cfg.xcol)
	}
	if g.ystep, ok = stat.Step(g.grid.Ys); !ok {
		return fmt.Errorf("%w: %q is sampled unevenly, which an image cannot carry; geom.Rect draws it",
			ErrUnevenLattice, g.cfg.ycol)
	}
	// The outer edges of the outer cells rather than the sample positions:
	// a cell is centred on its reading and half of the first one lies below
	// the first x, so an axis trained on the positions alone would clip a half
	// cell off each side of the picture. An axis that has no position for an
	// edge — a log scale, and an edge that crosses zero — keeps the sample.
	trainEdge(t.X, g.grid.Xs, g.xstep)
	trainEdge(t.Y, g.grid.Ys, g.ystep)

	g.ramp = g.colorScale()
	g.ramp.Train(g.grid.V...)
	// The colours the last training produced are not this one's, and a layer
	// trained again is a layer whose field may be a different field.
	g.ink.built = false
	return nil
}

// colorScale is the ramp the field is painted through: the caller's, or a
// sequential one over the field's own extent.
//
// The column named in [ColorBy] is not read, exactly as [Hexbin]'s is not: the
// quantity being coloured is [Z], which the layer already has, and the name is
// what titles the bar.
func (g *rasterGeom) colorScale() scale.ColorScale {
	if g.cfg.colorScale != nil {
		return g.cfg.colorScale
	}
	return scale.Sequential(palette.DefaultRamp)
}

// trainEdge extends s to the outer edges of an axis's outermost cells, or to
// the samples themselves where the scale has no position for an edge.
func trainEdge(s scale.Scale, vs []float64, step float64) {
	lo, hi := vs[0]-step/2, vs[len(vs)-1]+step/2
	if !defined(s, lo) {
		lo = vs[0]
	}
	if !defined(s, hi) {
		hi = vs[len(vs)-1]
	}
	s.Train(lo, hi)
}

func (g *rasterGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	area := f.Area
	if area.Empty() {
		return nil
	}
	// The field rather than [Frame.Coords], which is the one place that is the
	// accurate question: a nil coord *is* Cartesian, and asking for the
	// fallback would build one to be told so.
	if d, ok := f.Coord.(coord.Describer); ok && !d.Describe().Default() {
		return fmt.Errorf("%w: %s warps a square cell", ErrWarpedRaster, d.Describe().Type)
	}

	w := int(math.Round(float64(area.Dx())))
	h := int(math.Round(float64(area.Dy())))
	if w <= 0 || h <= 0 {
		return nil
	}
	g.cols = spansAlong(g.cols, f.X, area.Min.X, w, g.grid.Xs, g.xstep, g.cfg.resample)
	g.rows = spansAlong(g.rows, f.Y, area.Min.Y, h, g.grid.Ys, g.ystep, g.cfg.resample)
	g.paint(w, h)
	b.Image(g.img, area)

	g.reportRows(f)
	return nil
}

// paint fills the panel-sized buffer, one pass over its pixels.
//
// What the resampling chooses is how wide a pixel's block of cells is, and
// then what to do with it. Under [Nearest] the block is one cell and there is
// nothing to reduce, which is why [stat] grew two reductions and not three.
func (g *rasterGeom) paint(w, h int) {
	g.img = reuseNRGBA(g.img, w, h)
	g.ink.reset(g.ramp)
	for py := range h {
		ry := g.rows[py]
		if ry.hi <= ry.lo {
			continue
		}
		line := g.img.Pix[py*g.img.Stride:]
		for px := range w {
			rx := g.cols[px]
			if rx.hi <= rx.lo {
				continue
			}
			var v float64
			switch g.cfg.resample {
			case Nearest:
				// One cell, which is not a reduction and does not need one:
				// the spans are a single cell wide under this resampling.
				v = g.grid.Value(rx.lo, ry.lo)
			case Max:
				v = g.grid.Max(stat.Block{X0: rx.lo, X1: rx.hi, Y0: ry.lo, Y1: ry.hi})
			default:
				v = g.grid.Mean(stat.Block{X0: rx.lo, X1: rx.hi, Y0: ry.lo, Y1: ry.hi})
			}
			if !finite(v) {
				// A hole in the field, or a pixel whose cells are all holes.
				// Left transparent so the panel reads through, which is what
				// says nothing was measured rather than naming the bottom of
				// the ramp.
				continue
			}
			c := g.ink.at(v)
			if c.A == 0 {
				continue
			}
			i := px * 4
			line[i], line[i+1], line[i+2], line[i+3] = c.R, c.G, c.B, c.A
		}
	}
}

// spansAlong resolves, for every pixel of one axis of the panel, which cells of
// the lattice that pixel covers.
//
// It is one pass over the pixels rather than one over the cells, which is what
// makes the cost of this mark the panel's size rather than the field's — a
// recurrence plot of ten thousand samples is a hundred million cells and still
// a few hundred thousand pixels.
//
// The positions are inverted through the scale rather than interpolated, so an
// axis that is not linear places the cells where it places everything else: a
// log frequency axis gives the low bins more pixels than the high ones, which
// is the reading it exists for.
func spansAlong(dst []cellSpan, s scale.Scale, at float32, n int,
	axis []float64, step float64, how Resampling,
) []cellSpan {
	dst = grow(dst, n)
	count := len(axis)
	for p := range n {
		d := at + float32(p)
		near := cellAt(s.Invert(d+0.5), axis[0], step)
		span := cellSpan{lo: near, hi: near + 1}
		if how != Nearest {
			// Everything the pixel's own width reaches. The two ends are not
			// ordered: a vertical scale maps its domain to a descending range
			// so that larger values are higher on screen.
			u, v := s.Invert(d), s.Invert(d+1)
			lo, hi := math.Min(u, v), math.Max(u, v)
			i0 := int(math.Ceil((lo - axis[0]) / step))
			i1 := int(math.Floor((hi-axis[0])/step)) + 1
			if i1 > i0 {
				span = cellSpan{lo: i0, hi: i1}
			}
			// A pixel narrower than a cell reaches no cell centre at all, and
			// there the nearest cell is the answer: this is the direction the
			// record calls an upscale, where the reduction has nothing to do.
		}
		dst[p] = cellSpan{lo: max(span.lo, 0), hi: min(span.hi, count)}
	}
	return dst
}

// cellAt is the cell a value falls in: the index whose sample is nearest it,
// which is the cell whose half-step either side contains it.
func cellAt(v, origin, step float64) int {
	if !finite(v) {
		return -1
	}
	return int(math.Round((v - origin) / step))
}

// reuseNRGBA returns a w by h image, reusing dst's pixels when it has enough of
// them. It is [stat.Grid.Raster]'s discipline, applied to a buffer whose size
// is the panel's rather than the density grid's.
func reuseNRGBA(dst *image.NRGBA, w, h int) *image.NRGBA {
	r := image.Rect(0, 0, w, h)
	if dst == nil || cap(dst.Pix) < 4*w*h {
		return image.NewNRGBA(r)
	}
	dst.Pix = dst.Pix[:4*w*h]
	dst.Stride = 4 * w
	dst.Rect = r
	clear(dst.Pix)
	return dst
}

// reportRows attributes one position to each cell the panel shows, so that a
// pointer on the image can name the row behind the reading under it.
//
// It reports only while a cell is at least a pixel across. Below that the
// pixel is a reduction over several cells — under [Mean] its value is not any
// one of them — so there is no single row behind it, and reporting one would
// name whichever cell the rounding happened to reach. That is the answer a
// hexbin's cell and a density raster's already give: the hit still carries the
// position, read back through the axes, which is the reading a field has at a
// point.
//
// It costs nothing unless somebody asked. See [Frame.Marks].
func (g *rasterGeom) reportRows(f Frame) {
	if !f.tracking() || !g.cellsAreVisible(f) {
		return
	}
	x0, x1 := spanOf(g.cols)
	y0, y1 := spanOf(g.rows)
	n := (x1 - x0) * (y1 - y0)
	if n <= 0 {
		return
	}
	sc := acquire(f)
	defer sc.release()
	sc.pts, sc.mrows = grow(sc.pts, n)[:0], grow(sc.mrows, n)[:0]
	for j := y0; j < y1; j++ {
		for i := x0; i < x1; i++ {
			row := g.grid.Row[g.grid.Index(i, j)]
			if row < 0 {
				continue
			}
			sc.pts = append(sc.pts, ir.Point{
				X: f.X.Map(g.grid.Xs[i]),
				Y: f.Y.Map(g.grid.Ys[j]),
			})
			sc.mrows = append(sc.mrows, int(row))
		}
	}
	f.Marks(MarkRows{At: sc.pts, Rows: sc.mrows})
}

// cellsAreVisible reports whether one cell covers at least one pixel on both
// axes, which is when a reader can point at a cell rather than at a crowd.
//
// The average pixel per cell rather than the smallest, because a log axis has
// no single answer and the question is about the picture as a whole.
func (g *rasterGeom) cellsAreVisible(f Frame) bool {
	return perCell(f.X, g.grid.Xs) >= 1 && perCell(f.Y, g.grid.Ys) >= 1
}

func perCell(s scale.Scale, axis []float64) float64 {
	lo, hi := s.Map(axis[0]), s.Map(axis[len(axis)-1])
	return math.Abs(float64(hi-lo)) / float64(len(axis)-1)
}

// spanOf is the range of cells the panel's pixels reach on one axis.
func spanOf(spans []cellSpan) (lo, hi int) {
	lo, hi = math.MaxInt, 0
	for _, s := range spans {
		if s.hi <= s.lo {
			continue
		}
		lo, hi = min(lo, s.lo), max(hi, s.hi)
	}
	if hi <= lo {
		return 0, 0
	}
	return lo, hi
}

// ColorGuide implements [Guided]: the labelled bar that is the whole argument
// for this mark having a ramp at all.
func (g *rasterGeom) ColorGuide() (ColorGuide, bool) {
	if g.err != nil || g.ramp == nil {
		return ColorGuide{}, false
	}
	label := g.cfg.colorCol
	if label == "" {
		label = g.cfg.zcol
	}
	return ColorGuide{Label: label, Scale: g.ramp}, true
}

// Legend is the layer's entry, and there is one only when the caller named the
// layer — [Horizon]'s rule, for [Horizon]'s reason. A raster says what it is
// through its colourbar; a swatch beside it would pick one colour out of the
// ramp and label it with a column name.
func (g *rasterGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil || g.ramp == nil || g.cfg.label == "" {
		return LegendEntry{}, false
	}
	lo, hi := g.ramp.Domain()
	return LegendEntry{Label: g.cfg.label, Color: g.ramp.Color((lo + hi) / 2), Kind: SwatchBox}, true
}

func (g *rasterGeom) Source() data.Source { return g.src }

func (g *rasterGeom) Describe() Desc {
	d := g.cfg.describe(MarkRaster)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*rasterGeom)(nil)
	_ Guided    = (*rasterGeom)(nil)
)

// ink is a colour scale read into a table, so that painting a panel's worth of
// pixels does not mean a panel's worth of ramp arithmetic.
//
// It is here because a ramp lookup is not cheap and a raster does hundreds of
// thousands of them: [github.com/timzifer/figure/palette.Lerp] blends in linear
// light, which is three transfer functions in and three out per colour, and at
// that price the ink costs more than the field. Every other mark asks for a
// handful of colours and none of them noticed.
//
// The table is built once and stands while the scale's domain does, which is
// what makes a chart redrawn per pointer move pay for it once.
type ink struct {
	// cs is the scale the table came from, kept so that the cases a table
	// cannot answer can still be asked.
	cs scale.ColorScale
	// lut is the colours: one per class for a classed scale, and one per
	// sample along the ramp for a continuous one.
	lut []ir.Color
	// breaks are a classed scale's boundaries, and nil for a continuous scale.
	// Their presence is what says which kind of table lut is.
	breaks []float64
	// xf is the scale's own reading of where a value sits along the ramp,
	// hoisted out of the pixel loop — [scale.ColorPositionOf] would type
	// assert once per pixel.
	xf scale.ColorTransformer
	// logDomain marks a scale that has no colour for a value at or below zero,
	// which is the one case a position cannot express: the position clamps to
	// the bottom of the ramp and the scale answers with its undefined colour.
	logDomain bool

	lo, hi float64
	built  bool
}

// rampSamples is how finely a continuous ramp is read into the table.
//
// It is render's `colorbarStops` argument at a different scale: the bar samples
// a ramp thirty-two times across a couple of hundred pixels because that is
// already past the point where the difference is a rounding error, and this is
// a thousand samples of a ramp whose own anchors number in the hundreds. Two
// values that differ by less than a thousandth of the ramp differ by less than
// the byte the picture is drawn in.
//
// A classed scale is not sampled at all, for the reason render draws its bar
// in bands rather than as a gradient: a class boundary is an edge, and an edge
// is exactly what sampling steps over.
const rampSamples = 1024

// reset rebuilds the table if the scale has moved under it.
//
// The domain is what moves. A continuous table is the ramp itself — sampled
// along its own length, and read back through whatever position the scale
// reports today — so a retrained scale needs nothing from it; a classed table
// is not, because a scale that cuts its own domain puts its boundaries
// somewhere else the moment the domain changes, and the boundaries are what
// the table is indexed by.
//
// The domain rather than the scale itself is what is compared, and the layer
// drops the table outright when it trains again. Comparing the two scales is
// not available: a ColorScale is an interface, and an implementation is free
// to be a type that == panics on — the same reason [ColorGuide.Key] exists.
func (k *ink) reset(cs scale.ColorScale) {
	lo, hi := cs.Domain()
	if k.built && k.lo == lo && k.hi == hi {
		return
	}
	k.cs, k.lo, k.hi, k.built = cs, lo, hi, true
	k.xf, _ = cs.(scale.ColorTransformer)
	k.logDomain = scale.ColorTransformOf(cs) == scale.TransformLog
	k.lut = k.lut[:0]

	if c, classed := scale.Classed(cs); classed {
		// One colour per class, taken at the middle of the class exactly as
		// render's classed bar takes it — so the bar beside the chart and the
		// chart cannot disagree about which colour a class is.
		k.breaks = append(k.breaks[:0], c.Breaks()...)
		edges := append(append(make([]float64, 0, len(k.breaks)+2), lo), k.breaks...)
		edges = append(edges, hi)
		for i := 0; i+1 < len(edges); i++ {
			k.lut = append(k.lut, cs.Color(edges[i]+(edges[i+1]-edges[i])/2))
		}
		return
	}
	k.breaks = k.breaks[:0]
	for i := range rampSamples {
		t := float64(i) / (rampSamples - 1)
		k.lut = append(k.lut, cs.Color(scale.ColorValueOf(cs, t)))
	}
}

// at is the colour for one value.
func (k *ink) at(v float64) ir.Color {
	if len(k.lut) == 0 {
		return k.cs.Color(v)
	}
	if len(k.breaks) > 0 {
		// The class a value lands in is the number of boundaries at or below
		// it, which is scale.Threshold's own rule: a value equal to a boundary
		// belongs to the class above it.
		return k.lut[min(classAt(k.breaks, v), len(k.lut)-1)]
	}
	if k.logDomain && v <= 0 {
		// A position clamps and the scale does not: this value has no place on
		// the ramp at all, and the scale is the only thing that knows what it
		// paints instead.
		return k.cs.Color(v)
	}
	t := 0.5
	if k.xf != nil {
		t = k.xf.ColorPosition(v)
	} else if k.hi != k.lo {
		t = (v - k.lo) / (k.hi - k.lo)
	}
	switch {
	case !(t > 0):
		return k.lut[0]
	case t >= 1:
		return k.lut[len(k.lut)-1]
	}
	return k.lut[int(t*(rampSamples-1)+0.5)]
}

// classAt is how many of the boundaries are at or below v, which is the class
// it belongs to.
func classAt(breaks []float64, v float64) int {
	lo, hi := 0, len(breaks)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if breaks[mid] <= v {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

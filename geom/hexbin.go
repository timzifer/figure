package geom

import (
	"fmt"
	"slices"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// Hexbin counts the rows falling in each cell of a hexagonal lattice over the
// plot area and draws a hexagon per populated cell, shaded by its count.
//
// It is the third answer to overplotting, beside decimation and the density
// raster: a scatter of a million rows says more about row order than about the
// data, because the last mark drawn wins. A hexbin says how many rows are
// there. It differs from the raster in what a cell *is* — a hexagon has six
// neighbours all the same distance away, where a square has four near and four
// far, so a cloud binned into squares grows faint crosses and diagonal seams
// that belong to the bins rather than to the data. And it differs in
// resolution: the raster paints a pixel per cell and this draws a mark per
// cell, so the cells are counted in the hundreds.
//
// [DensityCells] sets the cell radius in device units; the default is
// [DefaultHexRadius].
//
// # Why the lattice is in device space
//
// A hexagon is only a hexagon if its six neighbours really are equidistant, and
// on screen is where that has to be true — a lattice laid out in data space and
// then mapped through the axes comes out stretched by whatever aspect ratio the
// panel happens to have. So the binning happens in Build, where the rectangle
// is known, exactly as the density raster's does.
//
// Device space is where the lattice is measured, not what it is pinned to. It
// is pinned to the data — to where the first row landed — and a cell at the
// edge of the panel counts every row in it, including the ones the clip hides.
// Together those are what let a panned plot move its cells rather than re-bin
// them: the same cells, the same counts, just somewhere else on screen.
//
// The cost of that is one thing this layer deliberately does not have: a
// colourbar. The counts are not known until the plot rectangle is, and the
// guide column is measured before it — the same ordering ADR 0011 describes for
// decimation. So a hexbin shades from a faded version of its own colour to the
// full one, and says how many rows are behind a cell through a hit rather than
// through a key. Give it a [ColorBy] scale to shade through a ramp instead; the
// column named there is not read, because the quantity being coloured is the
// layer's own count.
func Hexbin(src data.Source, opts ...Option) Geom {
	return &hexGeom{src: src, cfg: newConfig(opts)}
}

// DefaultHexRadius is the circumradius of one hexbin cell in device units when
// the layer names none. Ten pixels puts a few hundred cells in a normal panel,
// which is enough to show structure and few enough that each hexagon still
// reads as a mark.
const DefaultHexRadius = 10

type hexGeom struct {
	src data.Source
	cfg config
	s   series
	err error

	// class is each row's class for a multi-class hexbin — the series
	// [GroupBy] named, by first appearance — and keys the class names. Both
	// are nil for a hexbin of counts.
	class []int
	keys  []string
	bv    scale.BivariateColorScale
}

func (g *hexGeom) Train(t Training) error {
	x, y := t.X, t.Y
	// The colour column is deliberately not resolved: what a hexbin colours by
	// is its own count, and the name given to [ColorBy] is a label rather than
	// a column this layer reads.
	cfg := g.cfg
	cfg.colorCol = ""
	g.s, g.err = resolve(g.src, cfg, x, y)
	if g.err != nil {
		return g.err
	}
	if err := g.s.checkMissing(g.cfg, x, y); err != nil {
		return err
	}
	trainColumn(x, g.s.x)
	trainColumn(y, g.s.y)
	g.err = g.trainClasses()
	return g.err
}

// trainClasses reads the classes of a multi-class hexbin: a layer that names
// both [GroupBy] and a bivariate colour scale.
//
// A cell's colour is then two readings — which class dominates it and how
// purely — and the scale is trained on both ranges before any cell is counted:
// the classes are known in Train, because they are the groups, while the
// counts are not known until the plot rectangle is. That is the asymmetry
// docs/adr/0067-a-bivariate-colour-channel.md describes, and it is why this
// layer names its classes in the legend and has no key for its purity.
func (g *hexGeom) trainClasses() error {
	g.class, g.keys, g.bv = g.class[:0], g.keys[:0], nil
	bv, ok := scale.Bivariate(g.cfg.colorScale)
	if !ok || g.cfg.groupCol == "" {
		return nil
	}
	labels, ok := data.Labels(g.src, g.cfg.groupCol)
	if !ok {
		return fmt.Errorf("%w: %q", ErrNoColumn, g.cfg.groupCol)
	}
	if len(labels) != len(g.s.x) {
		return errLength(g.cfg.xcol, g.cfg.groupCol, len(g.s.x), len(labels))
	}
	at := make(map[string]int, 8)
	g.class = grow(g.class, len(labels))
	for i, l := range labels {
		k, seen := at[l]
		if !seen {
			k = len(g.keys)
			at[l] = k
			g.keys = append(g.keys, l)
		}
		g.class[i] = k
	}
	n := len(g.keys)
	// The first reading is the class index and the second is how impure a
	// cell may be: from a cell of one class to one split evenly over all of
	// them.
	bv.Train(0, float64(n-1))
	bv.TrainSecond(0, 1-1/float64(n))
	g.bv = bv
	return nil
}

func (g *hexGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	area := f.Area
	col := g.cfg.colorFor(f)
	if g.cfg.fill != nil {
		col = *g.cfg.fill
	}
	if area.Empty() || (col.A == 0 && g.cfg.colorScale == nil) {
		return nil
	}

	sc := acquire(f)
	defer sc.release()

	radius := g.cfg.cellSize
	if radius <= 0 {
		radius = DefaultHexRadius
	}
	cd := f.Coords()
	d, described := cd.(coord.Describer)
	exact := described && d.Describe().Default()
	ok := sc.plottable(g.s, f.X, f.Y)
	// The lattice is pinned to where the first row landed. See [hexGeom.at].
	first := slices.Index(ok, true)
	if first < 0 {
		return nil
	}
	ax, ay := g.at(f, cd, exact, first)
	sc.hex.ResetAt(radius, ax, ay,
		float64(area.Min.X), float64(area.Min.Y),
		float64(area.Max.X), float64(area.Max.Y))

	if g.bv != nil {
		sc.hex.CountClasses(len(g.keys))
	}
	for i := range g.s.x {
		if !ok[i] {
			continue
		}
		x, y := g.at(f, cd, exact, i)
		if g.bv != nil {
			sc.hex.AddClass(x, y, g.class[i])
			continue
		}
		sc.hex.Add(x, y)
	}
	if sc.hex.N == 0 {
		return nil
	}
	sc.cells = sc.hex.Cells(sc.cells)

	// One path per distinct shade rather than one per cell: a lattice of a
	// thousand cells over a handful of shades is a handful of drawing calls,
	// which is the same bargain groupByColor makes for per-mark colour.
	for _, run := range g.shades(sc, col) {
		if run.color.A == 0 {
			continue
		}
		sc.fill.Reset()
		for _, i := range run.idx {
			c := sc.cells[i]
			sc.verts = stat.Vertices(sc.verts, c.X, c.Y, radius)
			for k, v := range sc.verts {
				if k == 0 {
					sc.fill.MoveTo(float32(v.X), float32(v.Y))
					continue
				}
				sc.fill.LineTo(float32(v.X), float32(v.Y))
			}
			sc.fill.Close()
		}
		b.FillPath(&sc.fill, ir.Solid(run.color), ir.NonZero)
	}
	return nil
}

// at is where row i lands in device space, at float64 precision when the
// coord is Cartesian.
//
// The lattice is pinned to where the first row landed rather than to the
// rectangle, because the rectangle stays put when the plot is panned and the
// data does not. A lattice pinned to the rectangle re-bins a panned cloud
// against borders it has slid across, and cells flicker between counts; pinned
// to a row, the lattice slides with the cloud and every cell keeps its count.
//
// That only holds if a pan moves every position by exactly the same offset,
// and a position rounded to float32 does not: it is out by a different few
// hundred-thousandths of a pixel every frame, so the rows nearest a cell border
// hop across it and back while the plot is dragged — and a cell holding one
// row blinks between its faintest shade and nothing. Under a Cartesian coord
// the device position is the scales' own answer, so it is taken at float64
// through [scale.Map64]. Any other coord maps in float32 and keeps the flicker,
// which a pan there — a rotation, or a slide along a radius — makes the least
// of the problems a hexbin has.
func (g *hexGeom) at(f Frame, cd coord.Coord, exact bool, i int) (x, y float64) {
	if exact {
		return scale.Map64(f.X, g.s.x[i]), scale.Map64(f.Y, g.s.y[i])
	}
	p := cd.Point(f.X.Map(g.s.x[i]), f.Y.Map(g.s.y[i]))
	return float64(p.X), float64(p.Y)
}

// shades resolves each cell's colour and batches the cells by it.
//
// The scaling is logarithmic, for the reason the density raster's is: counts
// over a real point cloud span orders of magnitude, and under a linear mapping
// every cell but the densest few rounds to the background.
func (g *hexGeom) shades(sc *scratch, base ir.Color) []indexRun {
	sc.cols = grow(sc.cols, len(sc.cells))
	// The counts are compressed once, not twice. A colour scale carrying its
	// own log transform is already doing exactly this job, and taking the
	// logarithm of the cell fraction as well would spend most of the ramp on
	// the difference between one row and two.
	scaling := stat.Log
	if g.cfg.colorScale != nil && scale.ColorTransformOf(g.cfg.colorScale) != scale.TransformLinear {
		scaling = stat.Linear
	}
	for i, c := range sc.cells {
		if g.bv != nil {
			class, purity := c.Dominant()
			sc.cols[i] = g.bv.ColorAt(float64(class), 1-purity)
			continue
		}
		t := sc.hex.Fraction(c.Count, scaling)
		if g.cfg.colorScale != nil {
			// The ramp is read along itself rather than across the counts.
			// Nothing trained it on them — the count column does not exist in
			// the table — and training it here would write a scale two panels
			// may be drawing from at once. Reading it along itself is what
			// keeps an even step in density an even step along the ramp
			// whatever shape the ramp has.
			sc.cols[i] = g.cfg.colorScale.Color(scale.ColorValueOf(g.cfg.colorScale, t))
			continue
		}
		// Without a ramp the cell is the layer's own colour, faded by how empty
		// it is — never to nothing, so that a cell holding one row is still a
		// mark rather than a hole.
		sc.cols[i] = ir.Fade(base, hexFloor+(1-hexFloor)*t)
	}
	return sc.groupByColorAt(sc.cols[:len(sc.cells)])
}

// hexFloor is the opacity of the emptiest populated cell. A cell with one row
// in it is evidence, and evidence drawn at two percent opacity is evidence
// nobody sees.
const hexFloor = 0.25

// Legends names the classes of a multi-class hexbin, each in the colour a cell
// of that class alone is painted, and is the single entry [hexGeom.Legend]
// gives otherwise.
func (g *hexGeom) Legends(f Frame) []LegendEntry {
	if g.err != nil {
		return nil
	}
	if g.bv == nil {
		return LegendsOr(g, f, nil)
	}
	out := make([]LegendEntry, 0, len(g.keys))
	for k, name := range g.keys {
		out = append(out, LegendEntry{Label: name, Color: g.bv.ColorAt(float64(k), 0), Kind: SwatchBox})
	}
	return out
}

func (g *hexGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil {
		return LegendEntry{}, false
	}
	col := g.cfg.colorFor(f)
	if g.cfg.fill != nil {
		col = *g.cfg.fill
	}
	return LegendEntry{Label: g.cfg.labelFor(), Color: col, Kind: SwatchBox}, true
}

func (g *hexGeom) Source() data.Source { return g.src }
func (g *hexGeom) Subset(rows []int) Geom {
	return &hexGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *hexGeom) Describe() Desc {
	d := g.cfg.describe(MarkHexbin)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*hexGeom)(nil)
	_ Faceter   = (*hexGeom)(nil)
	_ Legender  = (*hexGeom)(nil)
)

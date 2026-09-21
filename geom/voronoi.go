package geom

import (
	"fmt"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/stat"
)

// ErrTooManySites reports more rows than a nearest-neighbour partition draws
// cells for.
var ErrTooManySites = fmt.Errorf("figure/geom: this mark cannot partition a panel between that many rows")

// Voronoi draws the panel divided into the part nearest each row: one cell per
// row, and the boundary between two cells halfway between the two rows.
//
//	geom.Voronoi(stations, geom.X("lon"), geom.Y("lat"),
//	    geom.ColorBy("rainfall", scale.Sequential(palette.Viridis)))
//
// It is the Thiessen polygon map — a nearest-station, nearest-depot or
// catchment diagram — and it answers one question: *which row is nearest
// here*. Given a colour column it answers a second one with the same ink, by
// filling each cell with that row's reading: the whole panel then carries the
// nearest measurement to every point of it, which is the coarsest honest
// interpolation of a scattered sample and the only one that invents no value.
//
// The sites themselves are not drawn. A layer of [Scatter] over the same table
// is what puts the dots on top of the cells, and keeping them apart is what
// lets a chart have the cells without the dots, or the dots in a size the
// partition knows nothing about.
//
// # It is measured on the panel, not in the data
//
// A cell is where it is because of a *distance*, and a distance needs two
// commensurable axes: rainfall against a longitude has no length, so nothing
// in the table says which station is nearer. What the reader sees does — the
// dots are so far apart *on the page* — so the partition is cut in device
// space, in Build, against the panel the chart was given, and the bisector
// between two cells is halfway between two dots as drawn. See
// docs/adr/0080-nearest-neighbour-cells.md.
//
// Two consequences worth knowing before the chart is used. The picture depends
// on the panel: the same data in a panel twice as wide is a different
// partition, because on the page the dots are differently placed. And the
// boundaries are a reading aid rather than a claim about a value in between —
// which is exactly what a nearest-neighbour cell is, and why this mark reads a
// pair of axes that need not be in the same unit at all.
//
// The cells fill the panel rectangle, and a coord that is not a rectangle cuts
// them to its own shape rather than being asked about them: under
// [github.com/timzifer/figure/coord.Polar] the partition is clipped to the
// disc, exactly as every other layer's ink is.
//
// # Colour, order and what is left out
//
// A colour column paints one cell at a time, through a ramp or a qualitative
// palette; without one the layer is one colour, which is a partition drawn for
// its boundaries rather than for its readings. [GroupBy] is accepted and
// ignored: there is one partition over every row of the layer, and a series
// within it is not a partition of anything.
//
// Row order decides nothing about the picture — an intersection of half-planes
// is the same whichever order it was taken in — with one exception: two rows at
// the same point have no boundary between them, so the *first* of them keeps
// the cell and the second draws nothing. A row whose position is missing is
// not a site at all. Each cell reports its own row, at its site, which is
// where a reader points when they mean that one.
//
// It refuses more than [stat.MaxVoronoiSites] rows with [ErrTooManySites],
// rather than drawing cells too small to tell apart: the chart for a sample
// that dense is [Raster] over the same values, which says the same thing in
// pixels.
func Voronoi(src data.Source, opts ...Option) Geom {
	return &voronoiGeom{src: src, cfg: newConfig(opts)}
}

type voronoiGeom struct {
	src data.Source
	cfg config
	s   series
	err error
}

func (g *voronoiGeom) Train(t Training) error {
	x, y := t.X, t.Y
	g.s, g.err = resolve(g.src, g.cfg, x, y)
	if g.err != nil {
		return g.err
	}
	if g.err = g.s.checkMissing(g.cfg, x, y); g.err != nil {
		return g.err
	}
	// The refusal is here rather than in Build so that it is reported before
	// anything has been drawn, and so that it is the same refusal on every
	// panel: what the partition costs is decided by the rows, and which of
	// them land on a panel is not.
	if n := finiteSites(g.s); n > stat.MaxVoronoiSites {
		g.err = fmt.Errorf("%w: %d rows, and this mark draws %d cells; geom.Raster is the same reading in pixels",
			ErrTooManySites, n, stat.MaxVoronoiSites)
		return g.err
	}
	trainColumn(x, g.s.x)
	trainColumn(y, g.s.y)
	g.cfg.trainColors(g.s)
	return nil
}

// finiteSites is how many rows have a position at all, which is how many cells
// the partition can have.
func finiteSites(s series) int {
	n := 0
	for i := range s.x {
		if finite(s.x[i]) && finite(s.y[i]) {
			n++
		}
	}
	return n
}

func (g *voronoiGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	if f.Area.Empty() {
		return nil
	}
	sc := acquire(f)
	defer sc.release()

	// The sites are placed like any other mark's positions — through the
	// scales and then through the coord — and it is only the partition between
	// them that is device geometry.
	cd := f.Coords()
	ok := sc.plottable(g.s, f.X, f.Y)
	sc.kx, sc.ky = grow(sc.kx, len(g.s.x))[:0], grow(sc.ky, len(g.s.x))[:0]
	rows := sc.rows[:0]
	for i := range g.s.x {
		if !ok[i] {
			continue
		}
		sc.kx = append(sc.kx, f.X.Map(g.s.x[i]))
		sc.ky = append(sc.ky, f.Y.Map(g.s.y[i]))
		rows = append(rows, i)
	}
	pts := cd.Points(grow(sc.pts, len(rows))[:0], sc.kx, sc.ky)
	sc.pts, sc.rows = pts, rows
	if len(pts) == 0 {
		return nil
	}

	sc.gx, sc.gy = grow(sc.gx, len(pts)), grow(sc.gy, len(pts))
	for i, p := range pts {
		sc.gx[i], sc.gy[i] = float64(p.X), float64(p.Y)
	}
	area := f.Area
	sc.vor.Reset(sc.gx, sc.gy,
		float64(area.Min.X), float64(area.Min.Y), float64(area.Max.X), float64(area.Max.Y))
	if sc.vor.TooMany {
		// Train refuses this already; a layer that reached here anyway draws
		// nothing rather than part of a partition.
		return fmt.Errorf("%w: %d sites on the panel, and this mark draws %d cells",
			ErrTooManySites, len(pts), stat.MaxVoronoiSites)
	}

	g.report(sc, f, pts)
	g.paint(b, sc, f)
	return nil
}

// report tells the frame which row is behind each cell, at the row's own site.
//
// A cell is bounded on every side, so — like a rect and unlike a bar — no part
// of it means more than another; the site is where the reader is pointing when
// they mean this cell, and it is inside it by construction. A site with no cell
// reports nothing, because nothing was drawn for it.
func (g *voronoiGeom) report(sc *scratch, f Frame, pts []ir.Point) {
	if !f.tracking() {
		return
	}
	at := grow(sc.sites, len(pts))[:0]
	keep := sc.keep[:0]
	for i := range pts {
		if sc.vor.Cells[i].N < 3 {
			continue
		}
		at = append(at, pts[i])
		keep = append(keep, sc.rows[i])
	}
	sc.sites, sc.keep = at, keep
	f.Marks(MarkRows{At: at, Rows: sc.sourceRows(g.s, keep)})
}

// paint fills the cells, batched by colour, with one subpath per cell.
//
// One subpath per cell rather than one path per cell is what keeps the drawing
// calls down to one per colour, and it is also what lets a pointer land on the
// cell it is inside: hit-testing indexes a mark per subpath
// (docs/adr/0015-hit-testing.md).
func (g *voronoiGeom) paint(b ir.Backend, sc *scratch, f Frame) {
	fill := g.cfg.fillOf(g.cfg.colorFor(f), 1)
	stroke := ir.Stroke{Color: g.cfg.colorFor(f), Width: pick(g.cfg.width, 1)}
	// A cell is outlined only where the caller named both a fill and a colour,
	// which is [Rect]'s rule and is here for [Rect]'s reason: interact ranks a
	// vertex above an area, so an outline nobody asked for would make every
	// hover over a partition report a corner.
	outline := g.cfg.fill != nil && g.cfg.color != nil

	cols := sc.colorsFor(g.cfg, g.s, sc.rows)
	if cols == nil {
		g.cells(b, sc, f, sc.marksIn(len(sc.vor.Cells)), fill, stroke, outline)
		return
	}
	for _, run := range sc.groupByColorAt(cols) {
		col := g.cfg.fillOf(run.color, 1)
		if col.A == 0 {
			continue
		}
		g.cells(b, sc, f, run.idx, col,
			ir.Stroke{Color: run.color, Width: stroke.Width}, outline)
	}
}

// cells emits one path of the cells named by idx.
func (g *voronoiGeom) cells(b ir.Backend, sc *scratch, f Frame, idx []int, fill ir.Color, stroke ir.Stroke, outline bool) {
	if fill.A == 0 && !outline {
		return
	}
	// Reserved once rather than grown per cell: a thousand cells of half a
	// dozen vertices appended into an empty path is a doubling ladder, and a
	// pool emptied by a collection walks it again on the next frame. A cell of
	// n vertices is n points and n+1 ops — the moves and lines, and the close.
	verts := 0
	for _, i := range idx {
		if n := sc.vor.Cells[i].N; n >= 3 {
			verts += n
		}
	}
	sc.fill.Reset()
	sc.fill.Grow(verts+len(idx), verts)
	for _, i := range idx {
		cell := sc.vor.Cell(i)
		if len(cell) < 3 {
			continue
		}
		sc.fill.MoveTo(float32(cell[0].X), float32(cell[0].Y))
		for _, p := range cell[1:] {
			sc.fill.LineTo(float32(p.X), float32(p.Y))
		}
		sc.fill.Close()
	}
	if sc.fill.Empty() {
		return
	}
	if fill.A != 0 {
		g.cfg.fillMark(b, &sc.fill, f, 0, fill)
	}
	if outline && stroke.Visible() {
		b.StrokePath(&sc.fill, stroke)
	}
}

func (g *voronoiGeom) ColorGuide() (ColorGuide, bool) {
	return g.cfg.colorGuide(g.s, g.err)
}

func (g *voronoiGeom) Legends(f Frame) []LegendEntry {
	if g.err != nil {
		return nil
	}
	return LegendsOr(g, f, g.cfg.legends(f, nil, g.s, SwatchBox))
}

func (g *voronoiGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil || g.cfg.varying(g.s) {
		return LegendEntry{}, false
	}
	col := g.cfg.colorFor(f)
	if g.cfg.fill != nil {
		col = *g.cfg.fill
	}
	return g.cfg.boxSwatch(f, g.cfg.labelFor(), col), true
}

func (g *voronoiGeom) Source() data.Source { return g.src }

func (g *voronoiGeom) Subset(rows []int) Geom {
	return &voronoiGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *voronoiGeom) Describe() Desc {
	d := g.cfg.describe(MarkVoronoi)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*voronoiGeom)(nil)
	_ Faceter   = (*voronoiGeom)(nil)
	_ Guided    = (*voronoiGeom)(nil)
	_ Legender  = (*voronoiGeom)(nil)
)

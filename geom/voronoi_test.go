package geom_test

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

// The nearest-neighbour partition. The geometry has its own tests in package
// stat; what is tested here is the seam — that a cell is a row, that the cells
// are cut on the panel rather than in the data, that a colour column paints
// them one at a time, and what a hit test is told.

// stations is four gauges spread over a region, each with a reading: the table
// this mark is for.
func stations() data.Source {
	return data.NewTable().
		Float64("lon", []float64{1, 9, 1, 9}).
		Float64("lat", []float64{1, 1, 9, 9}).
		Float64("rain", []float64{12, 30, 12, 48}).
		String("name", []string{"north", "east", "south", "west"})
}

// A cell per row, and each of them a subpath of its own so that a pointer
// lands on the cell it is inside.
func TestAVoronoiLayerDrawsACellPerRow(t *testing.T) {
	g := geom.Voronoi(stations(), geom.X("lon"), geom.Y("lat"))
	rec, f := relFrame(t, g, nil, 200, 200)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if got := len(shapesIn(t, rec)); got != 4 {
		t.Errorf("drew %d cells for four rows, want 4", got)
	}
}

// The cells cover the panel and nothing else: the partition is of the plot
// area, which is what makes every point of it belong to exactly one row.
func TestAVoronoiCoversThePanel(t *testing.T) {
	g := geom.Voronoi(stations(), geom.X("lon"), geom.Y("lat"))
	rec, f := relFrame(t, g, nil, 200, 120)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	covered := 0.0
	for _, s := range shapesIn(t, rec) {
		covered += ringArea(s)
	}
	if want := 200.0 * 120.0; math.Abs(covered-want) > want*1e-4 {
		t.Errorf("the cells cover %g of the panel, want all %g of it", covered, want)
	}
}

// The partition is cut where the reader measures it: on the panel. Four
// stations at the corners of a square divide a square panel into quarters, and
// the same four divide a wide panel into quarters of *it* — the boundary moves
// with the drawing, because halfway between two dots is a place on the page.
//
// It is the claim ADR 0080 is about, and the test that would fail if the cells
// were cut in the scaled pair and stretched afterwards.
func TestAVoronoiIsCutOnThePanel(t *testing.T) {
	for _, panel := range []struct {
		name string
		w, h float32
	}{{"square", 200, 200}, {"wide", 400, 100}} {
		g := geom.Voronoi(stations(), geom.X("lon"), geom.Y("lat"))
		rec, f := relFrame(t, g, nil, panel.w, panel.h)
		if err := g.Build(rec, f); err != nil {
			t.Fatal(err)
		}
		for _, s := range shapesIn(t, rec) {
			r := boundsOf(s)
			// Four sites symmetric about the middle: each cell is one quarter
			// of the panel, whatever shape the panel is.
			if got, want := r.Dx(), panel.w/2; !closef(got, want) {
				t.Errorf("%s panel: a cell is %g wide, want %g", panel.name, got, want)
			}
			if got, want := r.Dy(), panel.h/2; !closef(got, want) {
				t.Errorf("%s panel: a cell is %g high, want %g", panel.name, got, want)
			}
		}
	}
}

// A colour column paints one cell at a time: four readings in three distinct
// values are three fill calls, because the IR carries one colour per call and
// cells of one colour are batched into it.
func TestAVoronoiColourColumnPaintsEachCell(t *testing.T) {
	g := geom.Voronoi(stations(), geom.X("lon"), geom.Y("lat"),
		geom.ColorBy("rain", scale.Sequential(palette.Viridis)))
	rec, f := relFrame(t, g, nil, 200, 200)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	calls := rec.Filter("FillPath")
	if len(calls) != 3 {
		t.Fatalf("made %d fill calls for three distinct readings, want 3", len(calls))
	}
	// The two gauges reading 12 share a colour, so their cells are two
	// subpaths of one call and every cell is still its own shape.
	if got := len(shapesIn(t, rec)); got != 4 {
		t.Errorf("drew %d cells, want one per row (4)", got)
	}
}

// Each cell reports its own row, at its own site. A cell is bounded on every
// side, so no part of it means more than another, and the site is where a
// reader points when they mean that one.
func TestAVoronoiCellReportsItsRowAtItsSite(t *testing.T) {
	g := geom.Voronoi(stations(), geom.X("lon"), geom.Y("lat"))
	rec, f := relFrame(t, g, nil, 200, 200)
	var sink horizonRows
	f.Rows = &sink
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if len(sink.rows) != 4 {
		t.Fatalf("reported %d marks for four cells, want 4", len(sink.rows))
	}
	for i, row := range sink.rows {
		if row != i {
			t.Errorf("mark %d reports row %d, want %d", i, row, i)
		}
	}
	// The reported position is the site, and the site is inside its own cell.
	for i, at := range sink.at {
		if !insideRing(shapesIn(t, rec)[i], at) {
			t.Errorf("row %d is reported at (%g, %g), which is not in its cell", i, at.X, at.Y)
		}
	}
}

// Two rows at one position have no boundary between them, so the first keeps
// the cell and the second draws nothing — and reports nothing, because there
// is no shape for a pointer to land on.
func TestAVoronoiDrawsNothingForARowOnTopOfAnother(t *testing.T) {
	src := data.NewTable().
		Float64("lon", []float64{1, 1, 9}).
		Float64("lat", []float64{1, 1, 9})
	g := geom.Voronoi(src, geom.X("lon"), geom.Y("lat"))
	rec, f := relFrame(t, g, nil, 100, 100)
	var sink horizonRows
	f.Rows = &sink
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if got := len(shapesIn(t, rec)); got != 2 {
		t.Errorf("drew %d cells for three rows, two of them at one point, want 2", got)
	}
	if want := []int{0, 2}; len(sink.rows) != 2 || sink.rows[0] != want[0] || sink.rows[1] != want[1] {
		t.Errorf("reported rows %v, want %v — the first of the two keeps the cell", sink.rows, want)
	}
}

// A row with no position is not a site: it takes no cell and cuts none, so the
// rest of the partition is the partition of the rows that are there.
func TestAVoronoiSkipsARowWithNoPosition(t *testing.T) {
	src := data.NewTable().
		Float64("lon", []float64{1, math.NaN(), 9}).
		Float64("lat", []float64{5, 5, 5})
	g := geom.Voronoi(src, geom.X("lon"), geom.Y("lat"))
	rec, f := relFrame(t, g, nil, 100, 100)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	shapes := shapesIn(t, rec)
	if len(shapes) != 2 {
		t.Fatalf("drew %d cells for two rows with a position, want 2", len(shapes))
	}
	for _, s := range shapes {
		if got, want := ringArea(s), 100.0*100.0/2; math.Abs(got-want) > want*1e-4 {
			t.Errorf("a cell covers %g, want half the panel (%g)", got, want)
		}
	}
}

// A cell is outlined only where the caller named both a fill and a colour. It
// is [geom.Rect]'s rule and it is here for Rect's reason: interact ranks a
// vertex above an area, so an outline nobody asked for would make every hover
// over a partition report a corner.
func TestAVoronoiOutlinesItsCellsOnlyWhenAsked(t *testing.T) {
	plain := geom.Voronoi(stations(), geom.X("lon"), geom.Y("lat"))
	rec, f := relFrame(t, plain, nil, 200, 200)
	if err := plain.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if got := len(rec.Filter("StrokePath")); got != 0 {
		t.Errorf("a plain layer stroked %d paths, want none", got)
	}

	outlined := geom.Voronoi(stations(), geom.X("lon"), geom.Y("lat"),
		geom.Fill(ir.RGB(230, 230, 230)), geom.Color(ir.RGB(30, 30, 30)))
	rec, f = relFrame(t, outlined, nil, 200, 200)
	if err := outlined.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if got := len(rec.Filter("StrokePath")); got != 1 {
		t.Errorf("a layer given both a fill and a colour stroked %d paths, want 1", got)
	}
}

// A site outside the panel is a site like any other: it is not dropped, and it
// cuts the cells that are inside. That is what makes a zoomed or panned chart
// draw the partition it drew before, less the part that has scrolled away.
func TestAVoronoiIsCutByARowOffThePanel(t *testing.T) {
	src := data.NewTable().
		Float64("lon", []float64{5, 12}).
		Float64("lat", []float64{5, 5})
	g := geom.Voronoi(src, geom.X("lon"), geom.Y("lat"))
	// The axes are pinned to what is on screen, so the second row is off the
	// panel to the right — halfway between the two is x = 8.5, which is not.
	x, y := scale.Linear(scale.Domain(0, 10)), scale.Linear(scale.Domain(0, 10))
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	area := ir.R(0, 0, 100, 100)
	cd := coord.Cartesian().Frame(coord.Framing{Area: area, X: x, Y: y})
	rec := irtest.New()
	f := geom.Frame{Area: area, X: x, Y: y, Coord: cd, Theme: theme.Light}
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	shapes := shapesIn(t, rec)
	if len(shapes) != 2 {
		t.Fatalf("drew %d cells, want 2 — the row off the panel owns the strip nearest it", len(shapes))
	}
	// The visible row keeps the panel up to the bisector at x = 8.5, which is
	// 85 pixels across a panel showing a domain of ten; the rest belongs to the
	// row nobody can see, which is the honest answer to "which row is nearest".
	for i, want := range []float64{85 * 100, 15 * 100} {
		if got := ringArea(shapes[i]); math.Abs(got-want) > want*1e-3 {
			t.Errorf("cell %d covers %g, want %g", i, got, want)
		}
	}
}

// Past the cap the layer refuses in Train, before anything is drawn: cells too
// small to tell apart are not the chart the caller asked for, and the answer
// to a sample that dense is a raster of the same values.
func TestAVoronoiRefusesMoreRowsThanItDraws(t *testing.T) {
	n := stat.MaxVoronoiSites + 1
	xs, ys := make([]float64, n), make([]float64, n)
	for i := range n {
		xs[i], ys[i] = float64(i%40), float64(i/40)
	}
	g := geom.Voronoi(data.NewTable().Float64("x", xs).Float64("y", ys),
		geom.X("x"), geom.Y("y"))
	x, y := scale.Linear(), scale.Linear()
	err := g.Train(geom.Training{X: x, Y: y})
	if !errors.Is(err, geom.ErrTooManySites) {
		t.Fatalf("Train of %d rows returned %v, want ErrTooManySites", n, err)
	}
}

// ringArea is the unsigned area of one drawn shape.
func ringArea(pts []ir.Point) float64 {
	if len(pts) < 3 {
		return 0
	}
	sum := 0.0
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		sum += float64(p.X)*float64(q.Y) - float64(q.X)*float64(p.Y)
	}
	return math.Abs(sum) / 2
}

// insideRing reports whether p is inside the convex shape pts, boundary
// included.
func insideRing(pts []ir.Point, p ir.Point) bool {
	if len(pts) < 3 {
		return false
	}
	var positive, negative bool
	for i, a := range pts {
		b := pts[(i+1)%len(pts)]
		cross := float64(b.X-a.X)*float64(p.Y-a.Y) - float64(b.Y-a.Y)*float64(p.X-a.X)
		const eps = 1e-3
		if cross > eps {
			positive = true
		}
		if cross < -eps {
			negative = true
		}
	}
	return !(positive && negative)
}

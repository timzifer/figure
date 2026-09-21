package stat_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/timzifer/figure/stat"
)

// TestVoronoiOneSiteTakesTheWholeRectangle is the base case, and the one that
// says what a cell is: with nothing to be nearer to, every point of the
// rectangle is nearest this site.
func TestVoronoiOneSiteTakesTheWholeRectangle(t *testing.T) {
	var v stat.Voronoi
	v.Reset([]float64{3}, []float64{4}, 0, 0, 10, 20)

	cell := v.Cell(0)
	if len(cell) != 4 {
		t.Fatalf("the cell has %d vertices, want the rectangle's 4: %v", len(cell), cell)
	}
	if got, want := polygonArea(cell), 200.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("the cell covers %g, want the whole %g", got, want)
	}
}

// TestVoronoiSplitsOnTheBisector is the definition on the smallest case that
// has one: two sites divide the rectangle down the line halfway between them.
func TestVoronoiSplitsOnTheBisector(t *testing.T) {
	var v stat.Voronoi
	v.Reset([]float64{2, 8}, []float64{5, 5}, 0, 0, 10, 10)

	for i, want := range []float64{50, 50} {
		if got := polygonArea(v.Cell(i)); math.Abs(got-want) > 1e-9 {
			t.Errorf("cell %d covers %g, want %g", i, got, want)
		}
	}
	for i, bound := range []struct{ lo, hi float64 }{{0, 5}, {5, 10}} {
		for _, p := range v.Cell(i) {
			if p.X < bound.lo-1e-9 || p.X > bound.hi+1e-9 {
				t.Errorf("cell %d reaches x=%g, outside [%g, %g]", i, p.X, bound.lo, bound.hi)
			}
		}
	}
}

// TestVoronoiCellsPartitionTheRectangle is the property the whole construction
// is for: every point of the panel belongs to exactly one cell, so the areas
// add up to the rectangle's and no two cells overlap.
func TestVoronoiCellsPartitionTheRectangle(t *testing.T) {
	xs, ys := scatterSites(120, 1)
	var v stat.Voronoi
	v.Reset(xs, ys, 0, 0, 400, 300)

	total := 0.0
	for i := range xs {
		total += polygonArea(v.Cell(i))
	}
	if want := 400.0 * 300.0; math.Abs(total-want) > want*1e-9 {
		t.Errorf("the cells cover %g altogether, want the panel's %g", total, want)
	}

	// The partition, tested where it is defined: the nearest site to a point
	// is the site whose cell holds it.
	for range 500 {
		p := stat.Point{X: rand.Float64() * 400, Y: rand.Float64() * 300}
		nearest, best := -1, math.Inf(1)
		for i := range xs {
			if d := (xs[i]-p.X)*(xs[i]-p.X) + (ys[i]-p.Y)*(ys[i]-p.Y); d < best {
				nearest, best = i, d
			}
		}
		if !inPolygon(v.Cell(nearest), p) {
			t.Fatalf("(%g, %g) is nearest site %d and is not in its cell", p.X, p.Y, nearest)
		}
	}
}

// TestVoronoiHoldsItsOwnSite is the weaker property a reader depends on: the
// dot is inside the region drawn for it, which is what makes the region
// readable as that dot's.
func TestVoronoiHoldsItsOwnSite(t *testing.T) {
	xs, ys := scatterSites(80, 7)
	var v stat.Voronoi
	v.Reset(xs, ys, 0, 0, 400, 300)
	for i := range xs {
		if !inPolygon(v.Cell(i), stat.Point{X: xs[i], Y: ys[i]}) {
			t.Errorf("site %d at (%g, %g) is not inside its own cell", i, xs[i], ys[i])
		}
	}
}

// TestVoronoiCellsAreConvex is what lets a cell be one subpath and be
// hit-tested as the polygon it is: an intersection of half-planes cannot be
// anything else, and a clip written the wrong way round is how it would stop
// being one.
func TestVoronoiCellsAreConvex(t *testing.T) {
	xs, ys := scatterSites(60, 11)
	var v stat.Voronoi
	v.Reset(xs, ys, 0, 0, 150, 90)
	for i := range xs {
		cell := v.Cell(i)
		var sign float64
		for k := range cell {
			a, b, c := cell[k], cell[(k+1)%len(cell)], cell[(k+2)%len(cell)]
			cross := (b.X-a.X)*(c.Y-b.Y) - (b.Y-a.Y)*(c.X-b.X)
			if math.Abs(cross) < 1e-9 {
				continue
			}
			if sign == 0 {
				sign = cross
				continue
			}
			if (cross < 0) != (sign < 0) {
				t.Fatalf("cell %d turns both ways: %v", i, cell)
			}
		}
	}
}

// TestVoronoiSiteOutsideTheRectangleStillCuts is what keeps a panned chart
// drawing the partition it drew before: a site off the panel has no cell of
// its own and is still somewhere, so the cells inside stop where it says.
func TestVoronoiSiteOutsideTheRectangleStillCuts(t *testing.T) {
	var v stat.Voronoi
	v.Reset([]float64{5, 30}, []float64{5, 5}, 0, 0, 10, 10)

	if got := len(v.Cell(1)); got != 0 {
		t.Errorf("the site outside the panel has a cell of %d vertices, want none", got)
	}
	// Halfway between the two is x = 17.5, which is past the panel, so the
	// site inside keeps all of it.
	if got, want := polygonArea(v.Cell(0)), 100.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("the site inside covers %g, want the whole %g", got, want)
	}

	// Brought near enough for its bisector to fall inside, it cuts.
	v.Reset([]float64{5, 12}, []float64{5, 5}, 0, 0, 10, 10)
	if got, want := polygonArea(v.Cell(0)), 85.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("the site inside covers %g, want %g — the bisector at x=8.5", got, want)
	}
}

// TestVoronoiSecondSiteAtOnePointHasNoCell is the coincident case. The
// bisector between two sites at one point is undefined, so one of them draws
// the cell and the other draws nothing — and it is the first, because that is
// the row the caller's table put first.
func TestVoronoiSecondSiteAtOnePointHasNoCell(t *testing.T) {
	var v stat.Voronoi
	v.Reset([]float64{4, 4, 9}, []float64{4, 4, 9}, 0, 0, 10, 10)

	if len(v.Cell(0)) == 0 {
		t.Error("the first of the two coincident sites has no cell")
	}
	if got := len(v.Cell(1)); got != 0 {
		t.Errorf("the second of them has a cell of %d vertices, want none", got)
	}
	// The two that are drawn still partition the rectangle between them.
	if got, want := polygonArea(v.Cell(0))+polygonArea(v.Cell(2)), 100.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("the cells cover %g, want the whole %g", got, want)
	}
}

// TestVoronoiSkipsSitesThatAreNotNumbers checks the missing-value case: a site
// with no position is not a site, and it neither takes a cell nor cuts one.
func TestVoronoiSkipsSitesThatAreNotNumbers(t *testing.T) {
	var v stat.Voronoi
	v.Reset([]float64{2, math.NaN(), 8}, []float64{5, 5, 5}, 0, 0, 10, 10)

	if got := len(v.Cell(1)); got != 0 {
		t.Errorf("the missing site has a cell of %d vertices, want none", got)
	}
	for i, want := range []float64{50, 0, 50} {
		if got := polygonArea(v.Cell(i)); math.Abs(got-want) > 1e-9 {
			t.Errorf("cell %d covers %g, want %g", i, got, want)
		}
	}
}

// TestVoronoiDoesNotDependOnTheOrderOfItsSites is what makes this a layout the
// parallel-panel rule accepts: an intersection of half-planes does not care in
// which order it was taken, so the same sites shuffled draw the same cells.
func TestVoronoiDoesNotDependOnTheOrderOfItsSites(t *testing.T) {
	xs, ys := scatterSites(40, 3)
	var straight stat.Voronoi
	straight.Reset(xs, ys, 0, 0, 100, 100)

	order := rand.New(rand.NewPCG(5, 5)).Perm(len(xs))
	sx, sy := make([]float64, len(xs)), make([]float64, len(ys))
	for k, i := range order {
		sx[k], sy[k] = xs[i], ys[i]
	}
	var shuffled stat.Voronoi
	shuffled.Reset(sx, sy, 0, 0, 100, 100)

	for k, i := range order {
		a, b := straight.Cell(i), shuffled.Cell(k)
		if len(a) != len(b) {
			t.Fatalf("site %d has %d vertices in one order and %d in the other", i, len(a), len(b))
		}
		if got, want := polygonArea(b), polygonArea(a); math.Abs(got-want) > 1e-9 {
			t.Errorf("site %d covers %g in one order and %g in the other", i, want, got)
		}
	}
}

// TestVoronoiRefusesMoreSitesThanItDraws is the bound. Past it nothing is
// partitioned rather than some of it, because half a partition is not one.
func TestVoronoiRefusesMoreSitesThanItDraws(t *testing.T) {
	xs, ys := scatterSites(stat.MaxVoronoiSites+1, 2)
	var v stat.Voronoi
	v.Reset(xs, ys, 0, 0, 100, 100)

	if !v.TooMany {
		t.Error("TooMany is false for more sites than the cap")
	}
	if len(v.Cells) != 0 || len(v.Verts) != 0 {
		t.Errorf("it laid out %d cells and %d vertices, want neither", len(v.Cells), len(v.Verts))
	}
}

// TestVoronoiEmptyRectangleDrawsNothing covers the panel that has no room in
// it — a chart laid out at zero width, which every mark here survives.
func TestVoronoiEmptyRectangleDrawsNothing(t *testing.T) {
	var v stat.Voronoi
	v.Reset([]float64{1, 2}, []float64{1, 2}, 5, 5, 5, 40)
	if len(v.Cells) != 2 {
		t.Fatalf("it reported %d cells, want one per site", len(v.Cells))
	}
	for i := range v.Cells {
		if got := len(v.Cell(i)); got != 0 {
			t.Errorf("cell %d has %d vertices in a panel with no room", i, got)
		}
	}
}

// TestVoronoiReuseAllocatesNothing is the property a chart redrawn every frame
// rests on: the second partition of the same size runs out of the buffers the
// first one grew.
func TestVoronoiReuseAllocatesNothing(t *testing.T) {
	xs, ys := scatterSites(200, 9)
	var v stat.Voronoi
	v.Reset(xs, ys, 0, 0, 300, 300)

	if n := testing.AllocsPerRun(20, func() { v.Reset(xs, ys, 0, 0, 300, 300) }); n != 0 {
		t.Errorf("a second partition allocates %g times, want none", n)
	}
}

func BenchmarkVoronoiCells250(b *testing.B)  { benchmarkVoronoi(b, 250) }
func BenchmarkVoronoiCells1000(b *testing.B) { benchmarkVoronoi(b, stat.MaxVoronoiSites) }

func benchmarkVoronoi(b *testing.B, sites int) {
	// The sites fill the rectangle they are partitioning, which is the costly
	// case: every cell is small and has neighbours on every side of it.
	xs, ys := scatterSites(sites, 13)
	for i := range xs {
		xs[i], ys[i] = xs[i]*900/400, ys[i]*600/300
	}
	var v stat.Voronoi
	b.ReportAllocs()
	for b.Loop() {
		v.Reset(xs, ys, 0, 0, 900, 600)
	}
}

// scatterSites is n sites spread over a 400×300 rectangle, from a fixed seed
// so that a failure is the same failure twice.
func scatterSites(n int, seed uint64) (xs, ys []float64) {
	r := rand.New(rand.NewPCG(seed, seed*7+1))
	xs, ys = make([]float64, n), make([]float64, n)
	for i := range n {
		xs[i], ys[i] = r.Float64()*400, r.Float64()*300
	}
	return xs, ys
}

// polygonArea is the shoelace area of a ring, unsigned so that it does not
// matter which way round the ring runs.
func polygonArea(pts []stat.Point) float64 {
	if len(pts) < 3 {
		return 0
	}
	sum := 0.0
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		sum += p.X*q.Y - q.X*p.Y
	}
	return math.Abs(sum) / 2
}

// inPolygon reports whether p is inside the convex ring pts, allowing for a
// point on the boundary — which is where a cell's own site can legitimately
// sit when two sites share a position with a third between them.
func inPolygon(pts []stat.Point, p stat.Point) bool {
	if len(pts) < 3 {
		return false
	}
	var positive, negative bool
	for i, a := range pts {
		b := pts[(i+1)%len(pts)]
		cross := (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
		const eps = 1e-6
		if cross > eps {
			positive = true
		}
		if cross < -eps {
			negative = true
		}
	}
	return !(positive && negative)
}

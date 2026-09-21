package figure_test

import (
	"math"
	"runtime"
	"strconv"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/facet"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
	"github.com/timzifer/figure/three"
)

// signal is a plot of one line over n rows, ready to render repeatedly.
func signal(n int) *figure.Plot {
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range n {
		x[i] = float64(i)
		y[i] = math.Sin(float64(i)/50) + 0.2*math.Sin(float64(i)/3)
	}
	src := figure.Float64Columns(map[string][]float64{"x": x, "y": y})
	p := figure.New(figure.Size(800, 500), figure.Title("Signal"))
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))
	return p
}

// curvedSignal is [signal] drawn with the family that solves a system per run.
// Every other reduction in the library works out of the scratch pool, and the
// tridiagonal sweep is the newest thing that could quietly stop doing so.
func curvedSignal(n int, k geom.CurveKind) *figure.Plot {
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range n {
		x[i] = float64(i)
		y[i] = math.Sin(float64(i)/50) + 0.2*math.Sin(float64(i)/3)
	}
	src := figure.Float64Columns(map[string][]float64{"x": x, "y": y})
	p := figure.New(figure.Size(800, 500), figure.Title("Signal"))
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y"), geom.Curve(k)))
	return p
}

func colouredCloud(n int) *figure.Plot {
	x := make([]float64, n)
	y := make([]float64, n)
	v := make([]float64, n)
	for i := range n {
		x[i] = float64(i%997) / 997
		y[i] = math.Sin(float64(i) / 31)
		// A handful of distinct colours, which is what a real ramp over real
		// data quantises to anyway.
		v[i] = float64(i % 16)
	}
	src := figure.Float64Columns(map[string][]float64{"x": x, "y": y, "v": v})
	p := figure.New(figure.Size(800, 500))
	p.Add(geom.Scatter(src, geom.X("x"), geom.Y("y"),
		geom.ColorBy("v", scale.Sequential(palette.Viridis))))
	return p
}

// hatchedStack is [stackedSeries] under a theme that asked for redundant
// encoding, so that every series wears a pattern.
//
// A hatch is drawn rather than declared — the lines are generated per batch and
// stroked through a clip — which is exactly the shape that could start
// allocating per frame if the generator stopped reusing its buffer. The pattern
// is sized in device units and the plot is a fixed size, so what it emits does
// not grow with the rows: that is the claim the gate below checks.
func hatchedStack(n int) *figure.Plot {
	p := stackedSeries(n)
	figure.Theme(theme.Light.With(theme.Redundant(true)))(p)
	return p
}

// stackedSeries is a long table of n rows split into four series and stacked,
// which is the v0.7 data path: the groups are indexed and the adjustment is
// derived on every Train, and neither may cost anything per row.
//
// The series column is text, which is what a series column is. A numeric one
// is formatted into a label per row, and that is a cost of naming a category
// with a number rather than of stacking.
func stackedSeries(n int) *figure.Plot {
	names := [...]string{"north", "south", "east", "west"}
	var x, y []float64
	var series []string
	for i := range n {
		x = append(x, float64(i/len(names)))
		y = append(y, 1+math.Sin(float64(i)/97))
		series = append(series, names[i%len(names)])
	}
	src := figure.NewTable().Float64("x", x).Float64("y", y).String("s", series)
	p := figure.New(figure.Size(800, 500))
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Area(src, geom.X("x"), geom.Y("y"), geom.GroupBy("s")))
	return p
}

// polarSignal is [signal] drawn in a polar coord: the fourth data path, and
// the one that puts an interface call between a mapped pair and a device
// point.
//
// A polar coord does not decimate — see docs/adr/0018 — so this draws every
// row, which is exactly why it is worth measuring: if the per-point call ever
// stops going through the batch form, this is where it shows.
func polarSignal(n int) *figure.Plot {
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range n {
		x[i] = float64(i)
		y[i] = 2 + math.Sin(float64(i)/50)
	}
	src := figure.Float64Columns(map[string][]float64{"x": x, "y": y})
	p := figure.New(figure.Size(800, 500), figure.Coord(coord.Polar(coord.Chord())))
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))
	return p
}

// brokenRing is the v0.8 sugar's data path: a donut whose slices name their
// own inner and outer radius and are broken out of the ring per row.
//
// It is the fifth thing worth measuring, because it is the one that collects
// something per mark that is not a mark — a displacement — and carries it
// through the batching. The buffer for it comes from the scratch pool like
// every other, and this is what says so.
func brokenRing(n int) *figure.Plot {
	names := [...]string{"north", "south", "east", "west"}
	var share, floor, reach, pull []float64
	var series []string
	for i := range n {
		share = append(share, 1+math.Sin(float64(i)/97))
		floor = append(floor, 0.4)
		reach = append(reach, 0.6+0.4*math.Abs(math.Sin(float64(i)/31)))
		pull = append(pull, 0.05*float64(i%3))
		series = append(series, names[i%len(names)])
	}
	src := figure.NewTable().
		Float64("share", share).Float64("floor", floor).
		Float64("reach", reach).Float64("pull", pull).
		String("s", series)
	p := figure.New(figure.Size(800, 500), figure.Coord(coord.Donut(0.3)))
	p.X(scale.Linear(scale.Domain(0, 1)))
	p.Y(scale.Linear())
	p.Add(geom.Bar(src, geom.X("floor"), geom.X2("reach"), geom.Y("share"),
		geom.GroupBy("s"), geom.ExplodeBy("pull")))
	return p
}

func facetedSignal(panels, rows int) *figure.Plot { return facetedSignalWith(panels, rows, true) }

func facetedSignalWith(panels, rows int, parallel bool) *figure.Plot {
	var group []string
	var x, y []float64
	for g := range panels {
		for i := range rows {
			group = append(group, string(rune('a'+g)))
			x = append(x, float64(i))
			y = append(y, math.Sin(float64(i)/50+float64(g)))
		}
	}
	src := figure.NewTable().String("g", group).Float64("x", x).Float64("y", y)
	p := figure.New(figure.Size(900, 600), figure.Parallel(parallel))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))
	p.Facet(facet.Wrap("g", facet.Columns(3)))
	return p
}

// onOnePGate pins a benchmark's measurement to a single processor, and is what
// makes an allocation count reproducible rather than merely typical.
//
// The noise it removes is sync.Pool's, and it is worth writing down because it
// is invisible in a profile. A pool keeps a private slot per P. A render Gets
// its scratch and Puts it back on the same goroutine — but a frame here takes
// milliseconds, the scheduler preempts asynchronously every ten of them, and a
// goroutine that comes back on a different P finds its own scratch stranded in
// the old P's private slot, which nothing can steal from. The frame then
// refills every buffer it needs: about sixty allocations that have nothing to
// do with the data, landing in one iteration out of a few dozen. Measured over
// ten iterations that is the difference between 54 and 68 allocs/op for the
// same code, which is exactly the flake that makes a gate ignorable.
//
// One P has one private slot and nothing to migrate to, so the count is the
// same on every run. This is not a thumb on the scale: a pool miss is a real
// cost that a real chart pays occasionally, and it is simply not the cost this
// gate measures — the gate asks whether a frame allocates *per row*.
// testing.AllocsPerRun does the same thing for the test half of the gate, which
// is why the tests were steady all along while the benchmarks were not.
//
// The parallel benchmarks below are deliberately not pinned: what they measure
// is panels drawn on several goroutines, and nothing gates their counts.
func onOnePGate(b *testing.B) {
	prev := runtime.GOMAXPROCS(1)
	b.Cleanup(func() { runtime.GOMAXPROCS(prev) })
}

func benchmarkFrame(b *testing.B, rows int) {
	onOnePGate(b)
	p := signal(rows)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkStacked(b *testing.B, rows int) {
	onOnePGate(b)
	p := stackedSeries(rows)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

// The stacked path measured the same way as the plain one. The adjustment is
// derived on every Train — the axis has to describe the totals — so this is
// where a buffer that escaped the layer's own memory would show up.
func BenchmarkStacked1k(b *testing.B)   { benchmarkStacked(b, 1_000) }
func BenchmarkStacked100k(b *testing.B) { benchmarkStacked(b, 100_000) }

func benchmarkPolar(b *testing.B, rows int) {
	onOnePGate(b)
	p := polarSignal(rows)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

// The polar path measured the same way. Every row goes through
// coord.Coord.Points, and the batch form is the reason that costs nothing.
func BenchmarkPolar1k(b *testing.B)   { benchmarkPolar(b, 1_000) }
func BenchmarkPolar100k(b *testing.B) { benchmarkPolar(b, 100_000) }

func benchmarkBrokenRing(b *testing.B, rows int) {
	onOnePGate(b)
	p := brokenRing(rows)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

// The v0.8 sugar's path: a slice's radii come from two columns and its
// break-out from a third, and the displacement of every mark is collected and
// carried through the batching out of pooled memory.
func BenchmarkBrokenRing1k(b *testing.B)   { benchmarkBrokenRing(b, 1_000) }
func BenchmarkBrokenRing100k(b *testing.B) { benchmarkBrokenRing(b, 100_000) }

// bubbleCloud is the v0.9 data path: a scatter whose size comes from a column,
// so every mark is a circle collected into a path and the marks are ordered
// largest first before they are emitted.
//
// It is worth measuring because it is the one path that *sorts* per frame. The
// sort is over the scratch's own index list and in place, so a hundred times
// the rows must still cost what a thousand cost.
func bubbleCloud(n int) *figure.Plot {
	x := make([]float64, n)
	y := make([]float64, n)
	size := make([]float64, n)
	for i := range n {
		x[i] = float64(i%997) / 997
		y[i] = math.Sin(float64(i) / 31)
		size[i] = float64(i%64) + 1
	}
	src := figure.Float64Columns(map[string][]float64{"x": x, "y": y, "n": size})
	p := figure.New(figure.Size(800, 500))
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Scatter(src, geom.X("x"), geom.Y("y"),
		geom.SizeBy("n", scale.Size())))
	return p
}

func benchmarkBubbles(b *testing.B, rows int) {
	onOnePGate(b)
	p := bubbleCloud(rows)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBubbles1k(b *testing.B)   { benchmarkBubbles(b, 1_000) }
func BenchmarkBubbles100k(b *testing.B) { benchmarkBubbles(b, 100_000) }

// flowOf is an edge list of the given size: a wide bipartite graph, which is
// the shape whose layout costs the most — every node is reached in one step, so
// one column holds half of them and the relaxation has the most to do.
func flowOf(rows int) *figure.Plot {
	from := make([]string, rows)
	to := make([]string, rows)
	value := make([]float64, rows)
	for i := range rows {
		from[i] = "s" + strconv.Itoa(i%64)
		to[i] = "t" + strconv.Itoa(i%97)
		value[i] = float64(1 + i%13)
	}
	p := figure.New(figure.Size(900, 500), figure.Theme(bareLayoutTheme()), figure.Legend(false))
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Sankey(figure.NewTable().
		String("from", from).String("to", to).Float64("rps", value),
		geom.From("from"), geom.To("to"), geom.Value("rps")))
	return p
}

// crossingsOf is a table of categorical columns of the given size: four
// questions with a few dozen answers each, which is the shape a parallel-sets
// diagram is for — the rows grow and the picture does not, because what is
// drawn is the count.
func crossingsOf(rows int) *figure.Plot {
	a := make([]string, rows)
	b := make([]string, rows)
	c := make([]string, rows)
	d := make([]string, rows)
	for i := range rows {
		a[i] = "a" + strconv.Itoa(i%7)
		b[i] = "b" + strconv.Itoa(i%11)
		c[i] = "c" + strconv.Itoa(i%5)
		d[i] = "d" + strconv.Itoa(i%13)
	}
	p := figure.New(figure.Size(900, 500), figure.Theme(bareLayoutTheme()), figure.Legend(false))
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.ParallelSets(figure.NewTable().
		String("a", a).String("b", b).String("c", c).String("d", d),
		geom.Dims("a", "b", "c", "d")))
	return p
}

// treeOf is a hierarchy of the given size: one root, a fan of directories, and
// the rest leaves under them.
func treeOf(rows int) *figure.Plot {
	path := make([]string, rows)
	under := make([]string, rows)
	kb := make([]float64, rows)
	const dirs = 32
	for i := range rows {
		path[i] = "n" + strconv.Itoa(i)
		switch {
		case i == 0:
			under[i] = ""
		case i <= dirs:
			under[i] = "n0"
		default:
			under[i] = "n" + strconv.Itoa(1+i%dirs)
			kb[i] = float64(1 + i%29)
		}
	}
	p := figure.New(figure.Size(900, 500), figure.Theme(bareLayoutTheme()), figure.Legend(false))
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Treemap(figure.NewTable().
		String("path", path).String("under", under).Float64("kb", kb),
		geom.ID("path"), geom.Parent("under"), geom.Value("kb")))
	return p
}

func bareLayoutTheme() theme.Theme {
	return theme.Light.With(
		theme.Grid(false, false),
		theme.AxisLines(false, false),
		theme.Ticks(false, false),
	)
}

func benchmarkSankey(b *testing.B, rows int) {
	onOnePGate(b)
	benchmarkPlot(b, flowOf(rows))
}

func benchmarkParallelSets(b *testing.B, rows int) {
	onOnePGate(b)
	benchmarkPlot(b, crossingsOf(rows))
}

func benchmarkTreemap(b *testing.B, rows int) {
	onOnePGate(b)
	benchmarkPlot(b, treeOf(rows))
}

func benchmarkPlot(b *testing.B, p *figure.Plot) {
	b.Helper()
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

// The relational layouts. Both build a structure sized by the data on every
// Train — a node table, a depth per node, a total per node — so this is where a
// buffer that was allocated per frame rather than kept on the layer would show
// up, and it is the only thing keeping the interning map's buckets alive
// between frames.
func BenchmarkSankey1k(b *testing.B)   { benchmarkSankey(b, 1_000) }
func BenchmarkSankey100k(b *testing.B) { benchmarkSankey(b, 100_000) }

// The parallel-sets count, which is the one layer here whose *drawing* does
// not grow with the table at all: a hundred times the rows are the same few
// hundred crossings, and every buffer the count runs out of — the category
// index per row, the crossings, the order they are sorted into — is kept on
// the layer between frames. See docs/adr/0079-parallel-sets.md.
func BenchmarkParallelSets1k(b *testing.B)   { benchmarkParallelSets(b, 1_000) }
func BenchmarkParallelSets100k(b *testing.B) { benchmarkParallelSets(b, 100_000) }

func BenchmarkTreemap1k(b *testing.B)   { benchmarkTreemap(b, 1_000) }
func BenchmarkTreemap100k(b *testing.B) { benchmarkTreemap(b, 100_000) }

func BenchmarkFrame1k(b *testing.B)   { benchmarkFrame(b, 1_000) }
func BenchmarkFrame100k(b *testing.B) { benchmarkFrame(b, 100_000) }
func BenchmarkFrame1M(b *testing.B)   { benchmarkFrame(b, 1_000_000) }

func benchmarkFacet(b *testing.B, parallel bool) {
	p := facetedSignalWith(9, 50_000, parallel)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFacetParallel(b *testing.B) { benchmarkFacet(b, true) }
func BenchmarkFacetSerial(b *testing.B)   { benchmarkFacet(b, false) }

// labelled is a chart whose rows carry their own text: a strip of boxes with a
// label centred in each, which is the shape a locked terminal chart takes when
// a tooltip is not available to name a bar.
func labelled(n int) *figure.Plot {
	lo := make([]float64, n)
	hi := make([]float64, n)
	y := make([]float64, n)
	y2 := make([]float64, n)
	names := make([]string, n)
	for i := range n {
		lo[i], hi[i] = float64(i), float64(i)+0.9
		y[i], y2[i] = float64(i%8), float64(i%8)+0.9
		names[i] = labelNames[i%len(labelNames)]
	}
	src := data.NewTable().
		Float64("start", lo).Float64("end", hi).
		Float64("lo", y).Float64("hi", y2).
		String("label", names)

	opts := []geom.Option{geom.X("start"), geom.X2("end"), geom.Y("lo"), geom.Y2("hi")}
	p := figure.New(figure.Size(800, 500), figure.Title("Labelled"))
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Rect(src, opts...))
	p.Add(geom.Text(src, append(opts, geom.TextBy("label"), geom.Elide(true))...))
	return p
}

// Names of a length that straddles the boxes: some fit, some are cut, some are
// dropped, so a frame exercises all three answers rather than the cheap one.
var labelNames = []string{"a", "Wareneingang", "Halle 3", "Wareneingangsprüfung Süd", "ok"}

func benchmarkLabelled(b *testing.B, rows int) {
	onOnePGate(b)
	p := labelled(rows)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLabelled1k(b *testing.B)  { benchmarkLabelled(b, 1_000) }
func BenchmarkLabelled10k(b *testing.B) { benchmarkLabelled(b, 10_000) }

// --- the projected scene -------------------------------------------------

// benchmarkSurface prices a frame of the chart the third dimension exists for.
//
// A surface is not decimated — a reduction defined over pixel columns measures
// nothing in a projected scene — so this draws every quad of the grid, every
// frame, and is the honest measure of what turning one costs.
func benchmarkSurface(b *testing.B, n int) {
	onOnePGate(b)
	p := ripple(n)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSurface64(b *testing.B)  { benchmarkSurface(b, 64) }
func BenchmarkSurface256(b *testing.B) { benchmarkSurface(b, 256) }

// --- the measured field --------------------------------------------------

// benchmarkRaster prices a frame of a field drawn as one image.
//
// The pair behind it is the drawing half of the mark's claim: the picture is
// built at the panel's resolution, so a field of four thousand cells and one
// of a quarter of a million allocate the same frame. The ways to break that
// are to paint per cell rather than per pixel and to allocate the panel-sized
// buffer instead of repainting it.
//
// The two are not the same *time*, and that is not this gate's business: the
// larger field resolves a larger lattice in Train, which is what a chart
// re-trained every frame pays for having a quarter of a million rows in it.
func benchmarkRaster(b *testing.B, n int) {
	onOnePGate(b)
	p := measuredField(n)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRaster64(b *testing.B)  { benchmarkRaster(b, 64) }
func BenchmarkRaster512(b *testing.B) { benchmarkRaster(b, 512) }

// measuredField is an n by n field drawn as a raster over a fixed panel.
func measuredField(n int) *figure.Plot {
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	vs := make([]float64, 0, n*n)
	for j := range n {
		for i := range n {
			x := -3 + 6*float64(i)/float64(n-1)
			y := -3 + 6*float64(j)/float64(n-1)
			xs, ys = append(xs, x), append(ys, y)
			vs = append(vs, math.Exp(-(x*x+y*y)/4)*math.Cos(4*math.Hypot(x, y)))
		}
	}
	src := figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("v", vs)
	p := figure.New(figure.Size(800, 600), figure.Title("A measured field"))
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"),
		geom.ColorBy("v", scale.Sequential(palette.Viridis))))
	return p
}

// nicholsGrid is a chart whose furniture is a formula: two locus layers of
// sixteen curves between them, over a panel of the given width.
//
// The width is the point. A locus is sampled against the device rectangle, so a
// wider panel is a finer curve and more points — and none of them may be a
// fresh allocation, because the layer owns the buffers it samples into and the
// chart behind a resize handle is redrawn on every drag.
func nicholsGrid(width int) *figure.Plot {
	p := figure.New(figure.Size(width, width*7/8), figure.Title("Nichols"))
	p.X(scale.Linear(scale.Domain(-270, -90)))
	p.Y(scale.Linear(scale.Domain(-24, 36)))
	p.Add(geom.Locus(stat.NicholsM, []float64{-12, -6, -3, -1, 0, 1, 3, 6, 12}, geom.Dash()))
	p.Add(geom.Locus(stat.NicholsN, []float64{-1, -5, -10, -20, -45, -90, -150}, geom.Dash(2, 3)))
	return p
}

func benchmarkNichols(b *testing.B, width int) {
	onOnePGate(b)
	p := nicholsGrid(width)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNichols480(b *testing.B)  { benchmarkNichols(b, 480) }
func BenchmarkNichols1440(b *testing.B) { benchmarkNichols(b, 1440) }

// gaugeField is a nearest-neighbour partition of a panel between n gauges,
// each carrying a reading that paints its cell.
//
// The sites are the point. Every cell is clipped against every other site, so
// four times the rows are sixteen times the clipping — and none of it may be a
// fresh allocation, because the cells, their vertices and the two rings the
// clip swaps between are all buffers the layout owns.
func gaugeField(n int) *figure.Plot {
	xs, ys, vs := make([]float64, n), make([]float64, n), make([]float64, n)
	for i := range n {
		// A lattice turned by an irrational fraction of a turn: spread out, no
		// two sites at one point, and the same sites every run.
		t := float64(i) * 2.399963229728653
		r := math.Sqrt(float64(i+1) / float64(n))
		xs[i], ys[i] = r*math.Cos(t), r*math.Sin(t)
		vs[i] = math.Hypot(xs[i], ys[i])
	}
	src := figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("v", vs)
	p := figure.New(figure.Size(900, 600), figure.Title("Gauges"))
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Voronoi(src, geom.X("x"), geom.Y("y"),
		geom.ColorBy("v", scale.Sequential(palette.Viridis))))
	return p
}

// worldOfStations is n readings scattered over the sphere, drawn on the globe.
//
// The projection is the point. Every row goes through the orthographic map —
// three trigonometric functions and a visibility test — and rather more than
// half of them land on the far side and are not drawn at all, so this is where
// a coord that allocated per point, or a geom that grew a buffer for the
// points it decided to drop, would show up.
func worldOfStations(n int) *figure.Plot {
	lon, lat := make([]float64, n), make([]float64, n)
	for i := range n {
		// A spherical Fibonacci lattice: even over the sphere, no two rows at
		// one place, and the same rows every run.
		t := float64(i) * 2.399963229728653
		z := 1 - 2*(float64(i)+0.5)/float64(n)
		lon[i] = math.Mod(t*180/math.Pi+180, 360) - 180
		lat[i] = math.Asin(z) * 180 / math.Pi
	}
	src := figure.NewTable().Float64("lon", lon).Float64("lat", lat)
	p := figure.New(figure.Size(900, 600), figure.Title("Stations"),
		figure.Coord(coord.Geo(coord.Orthographic, coord.GeoCenter(10, 30))))
	p.X(scale.Linear(scale.Domain(-180, 180)))
	p.Y(scale.Linear(scale.Domain(-90, 90)))
	p.Add(geom.Scatter(src, geom.X("lon"), geom.Y("lat")))
	return p
}

// The globe, at a thousand rows and at a hundred thousand. What must not grow
// is the allocation count: the projection is arithmetic per point into buffers
// the layer already owns, and half the rows are dropped without one. See
// docs/adr/0081-a-map-projection.md.
func BenchmarkGlobe1k(b *testing.B)   { onOnePGate(b); benchmarkPlot(b, worldOfStations(1_000)) }
func BenchmarkGlobe100k(b *testing.B) { onOnePGate(b); benchmarkPlot(b, worldOfStations(100_000)) }

func benchmarkVoronoi(b *testing.B, sites int) {
	onOnePGate(b)
	benchmarkPlot(b, gaugeField(sites))
}

// The partition, at a quarter of its cap and at it. What must not grow is the
// allocation count: the work does, quadratically, and it is bounded by
// stat.MaxVoronoiSites rather than by this gate. See
// docs/adr/0080-nearest-neighbour-cells.md.
func BenchmarkVoronoi250(b *testing.B)  { benchmarkVoronoi(b, 250) }
func BenchmarkVoronoi1000(b *testing.B) { benchmarkVoronoi(b, stat.MaxVoronoiSites) }

// benchmarkTrajectory prices a path through the box, one primitive per
// segment.
func benchmarkTrajectory(b *testing.B, rows int) {
	onOnePGate(b)
	p := spiral(rows)
	target := irtest.NullTarget()
	if err := p.Render(target); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTrajectory1k(b *testing.B)   { benchmarkTrajectory(b, 1_000) }
func BenchmarkTrajectory100k(b *testing.B) { benchmarkTrajectory(b, 100_000) }

// BenchmarkOrbit is a frame of the interaction ADR 0057 is about: the same
// scene from a camera that moved, over and over, which is what a reader
// dragging across a chart produces.
//
// A camera is two floats and a frame is the same frame from another angle, so
// an orbit that allocated per frame would be a leak with a chart attached.
func benchmarkOrbit(b *testing.B, n int) {
	onOnePGate(b)
	// Into a backend that keeps nothing: irtest.Recorder copies every path it
	// is replayed, which is a cost of watching rather than of drawing, and a
	// gate that measured it would be a gate on the test harness.
	live, err := ripple(n).Live(irtest.NullTarget())
	if err != nil {
		b.Fatal(err)
	}
	defer live.Close()
	if err := live.Draw(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		live.Camera(three.Orbit(live.CameraValue(), 0.01, 0))
		if err := live.Draw(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOrbit32(b *testing.B) { benchmarkOrbit(b, 32) }
func BenchmarkOrbit96(b *testing.B) { benchmarkOrbit(b, 96) }

// ripple is an n by n surface: the shape a heatmap of the same grid reports
// without saying which way the ground falls.
func ripple(n int) *three.Plot {
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	zs := make([]float64, 0, n*n)
	for j := range n {
		for i := range n {
			x := -3 + 6*float64(i)/float64(n-1)
			y := -3 + 6*float64(j)/float64(n-1)
			r := math.Hypot(x, y)
			xs = append(xs, x)
			ys = append(ys, y)
			zs = append(zs, math.Exp(-r*r/4)*math.Cos(2*r))
		}
	}
	src := figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("z", zs)
	sc := three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("z")).
		Z(scale.Linear(scale.Nice())).
		Add(three.Surface(src, geom.X("x"), geom.Y("y"), geom.Z("z")))
	return three.New(three.Size(800, 600)).Scene(sc)
}

// spiral is a trajectory of the given length through the box.
// cloud is a projected scatter of rows points with a dropline each, which is
// two primitives per row.
func cloud(rows int) *three.Plot {
	xs := make([]float64, rows)
	ys := make([]float64, rows)
	zs := make([]float64, rows)
	for i := range xs {
		a := float64(i) * 2.399963
		r := math.Sqrt(float64(i) / float64(rows))
		xs[i], ys[i], zs[i] = r*math.Cos(a), r*math.Sin(a), math.Sin(3*a)
	}
	src := figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("z", zs)
	sc := three.NewScene().
		Add(three.Scatter3(src, geom.X("x"), geom.Y("y"), geom.Z("z")))
	return three.New(three.Size(800, 600)).Scene(sc)
}

func spiral(rows int) *three.Plot {
	xs := make([]float64, rows)
	ys := make([]float64, rows)
	zs := make([]float64, rows)
	for i := range xs {
		t := float64(i) / float64(rows-1) * 40 * math.Pi
		xs[i] = math.Cos(t)
		ys[i] = math.Sin(t)
		zs[i] = t
	}
	src := figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("t", zs)
	sc := three.NewScene().Z(scale.Linear(scale.Nice())).
		Add(three.Line3(src, geom.X("x"), geom.Y("y"), geom.Z("t")))
	return three.New(three.Size(800, 600)).Scene(sc)
}

package geom_test

import (
	"errors"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// lattice is a table of (x, y, v) rows over a regular grid: the shape a raster
// requires and will not guess at. The x positions are 0, 1, 2 … so that a
// cell's index and its value are easy to say in a test.
func lattice(nx, ny int, f func(i, j int) float64) data.Source {
	xs := make([]float64, 0, nx*ny)
	ys := make([]float64, 0, nx*ny)
	vs := make([]float64, 0, nx*ny)
	for j := range ny {
		for i := range nx {
			xs, ys = append(xs, float64(i)), append(ys, float64(j))
			vs = append(vs, f(i, j))
		}
	}
	return data.Float64Columns(map[string][]float64{"x": xs, "y": ys, "v": vs})
}

// rasterFrame trains a layer and draws it into a panel of the given size.
func rasterFrame(t *testing.T, g geom.Geom, w, h float32, opts ...func(*geom.Frame)) *irtest.Recorder {
	t.Helper()
	rec, err := drawRaster(g, w, h, opts...)
	if err != nil {
		t.Fatalf("drawing the raster: %v", err)
	}
	return rec
}

func drawRaster(g geom.Geom, w, h float32, opts ...func(*geom.Frame)) (*irtest.Recorder, error) {
	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		return nil, err
	}
	area := ir.R(0, 0, w, h)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)

	f := geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}
	for _, o := range opts {
		o(&f)
	}
	rec := irtest.New()
	return rec, g.Build(rec, f)
}

// ramp is a colour scale whose reading is easy to check: it paints the value v
// as the grey (v, v, v), so a pixel names the number behind it.
type greyRamp struct{ lo, hi float64 }

func (r *greyRamp) Train(vs ...float64) {
	for _, v := range vs {
		if !math.IsNaN(v) {
			r.lo, r.hi = math.Min(r.lo, v), math.Max(r.hi, v)
		}
	}
}
func (r *greyRamp) Domain() (float64, float64) { return r.lo, r.hi }
func (r *greyRamp) Color(v float64) ir.Color {
	n := uint8(math.Round(math.Max(0, math.Min(255, v))))
	return ir.RGB(n, n, n)
}

func grey() scale.ColorScale { return &greyRamp{} }

// The whole point of the mark: one field, one drawing call, whatever the cell
// count. A Rect of the same table would be one primitive per cell.
func TestARasterIsOneImage(t *testing.T) {
	src := lattice(64, 48, func(i, j int) float64 { return float64(i + j) })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"))

	rec := rasterFrame(t, g, 300, 200)
	if got := rec.Count("Image"); got != 1 {
		t.Fatalf("a raster of 3072 cells took %d images, want one", got)
	}
	if n := len(rec.Calls); n != 1 {
		t.Fatalf("a raster emitted %d calls, want the image alone", n)
	}
}

// The image is built at the panel's resolution and blitted one for one, so that
// no backend is asked what scaling means.
func TestARasterIsBuiltAtThePanelsResolution(t *testing.T) {
	src := lattice(8, 8, func(i, j int) float64 { return float64(i) })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"))

	rec := rasterFrame(t, g, 300, 200)
	c := rec.Filter("Image")[0]
	if got := c.Image.Bounds(); got.Dx() != 300 || got.Dy() != 200 {
		t.Errorf("the image is %v, want the panel's 300 by 200", got)
	}
	if c.Rect != ir.R(0, 0, 300, 200) {
		t.Errorf("the image was blitted into %v, want the panel", c.Rect)
	}
}

// Nearest neighbour, which is the whole of the upscaling rule: a cell larger
// than a pixel is a block of that cell's colour, and no pixel holds a colour
// between two measurements.
func TestARasterInventsNoColourBetweenTwoCells(t *testing.T) {
	// Two cells across, black and white, blown up over a wide panel.
	src := lattice(2, 2, func(i, j int) float64 { return float64(i) * 255 })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"),
		geom.ColorBy("v", grey()))

	rec := rasterFrame(t, g, 200, 40)
	img := rec.Filter("Image")[0].Image
	seen := map[color.NRGBA]int{}
	for px := range 200 {
		c := nrgba(img, px, 20)
		seen[c]++
	}
	if len(seen) != 2 {
		t.Fatalf("two cells were painted in %d colours across the panel: %v", len(seen), seen)
	}
	for c := range seen {
		if c.R != 0 && c.R != 255 {
			t.Errorf("a pixel is %v, which is a value between the two the field holds", c)
		}
	}
}

// A hole in the field is a hole in the picture, so the panel's own background
// and grid read through where nothing was measured.
func TestARasterLeavesAnUnmeasuredCellTransparent(t *testing.T) {
	src := lattice(4, 4, func(i, j int) float64 {
		if i == 1 && j == 1 {
			return math.NaN()
		}
		return 128
	})
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"), geom.ColorBy("v", grey()))

	rec := rasterFrame(t, g, 40, 40)
	img := rec.Filter("Image")[0].Image
	clear, opaque := 0, 0
	for py := range 40 {
		for px := range 40 {
			if nrgba(img, px, py).A == 0 {
				clear++
				continue
			}
			opaque++
		}
	}
	if clear == 0 {
		t.Error("the hole in the field was painted")
	}
	if opaque == 0 {
		t.Error("nothing but the hole was painted")
	}
	// One cell of sixteen, give or take where the panel's edges fall.
	if frac := float64(clear) / 1600; frac < 0.04 || frac > 0.10 {
		t.Errorf("the hole covers %.2f of the panel, want about one cell in sixteen", frac)
	}
}

// Downsampling is named rather than assumed, and this is why: a peak one cell
// wide survives Max and is as likely as not to be missed by the nearest cell to
// a pixel centre.
func TestMaxKeepsAPeakThatNearestCanDrop(t *testing.T) {
	// A field of zeros with one spike per column, on a panel eight times
	// coarser than the lattice.
	src := lattice(64, 8, func(i, j int) float64 {
		if i%8 == 3 && j == 4 {
			return 255
		}
		return 0
	})
	peaks := func(how geom.Resampling) int {
		g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"),
			geom.ColorBy("v", grey()), geom.Resample(how))
		rec := rasterFrame(t, g, 8, 8)
		img := rec.Filter("Image")[0].Image
		n := 0
		for py := range 8 {
			for px := range 8 {
				if nrgba(img, px, py).R == 255 {
					n++
				}
			}
		}
		return n
	}
	if got := peaks(geom.Max); got != 8 {
		t.Errorf("Max kept %d of the eight peaks, want all of them", got)
	}
	if got := peaks(geom.Nearest); got >= 8 {
		t.Errorf("Nearest kept %d peaks; this field was built so that it drops some", got)
	}
}

// Mean is the reduction for a field that is a quantity per area, and it reads
// the cells rather than the picture.
func TestMeanAveragesTheCellsOnOnePixel(t *testing.T) {
	// Alternating 0 and 200 along x: any pixel covering a pair means 100.
	src := lattice(64, 8, func(i, j int) float64 { return float64(i%2) * 200 })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"),
		geom.ColorBy("v", grey()), geom.Resample(geom.Mean))

	rec := rasterFrame(t, g, 8, 8)
	img := rec.Filter("Image")[0].Image
	for px := range 8 {
		if got := nrgba(img, px, 4).R; got != 100 {
			t.Fatalf("pixel %d reads %d, want the mean of 0 and 200", px, got)
		}
	}
}

// An image is a grid of equal cells, so an axis that is not evenly sampled is
// refused — and the message names the column and the mark that draws it today.
func TestAnUnevenLatticeIsRefusedAndNamesRect(t *testing.T) {
	xs := []float64{1, 2, 10, 1, 2, 10}
	ys := []float64{0, 0, 0, 1, 1, 1}
	vs := []float64{1, 2, 3, 4, 5, 6}
	src := data.Float64Columns(map[string][]float64{"t": xs, "y": ys, "v": vs})
	g := geom.Raster(src, geom.X("t"), geom.Y("y"), geom.Z("v"))

	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrUnevenLattice) {
		t.Fatalf("an uneven lattice gave %v, want ErrUnevenLattice", err)
	}
	for _, want := range []string{`"t"`, "geom.Rect"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message %q does not name %s", err, want)
		}
	}
}

// A raster's cell count is its resolution, so there is nothing for a row
// reduction to reduce. Saying so is better than quietly ignoring the option.
func TestDecimatingARasterIsAnError(t *testing.T) {
	src := lattice(8, 8, func(i, j int) float64 { return float64(i) })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"), geom.Decimate(geom.LTTB))

	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrDecimatedRaster) {
		t.Fatalf("a decimated raster gave %v, want ErrDecimatedRaster", err)
	}
}

// A blit is axis-aligned in device space. A field in a round panel is a
// different mark, not an option on this one.
func TestARasterUnderAPolarCoordIsAnError(t *testing.T) {
	src := lattice(8, 8, func(i, j int) float64 { return float64(i) })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"))

	_, err := drawRaster(g, 200, 200, func(f *geom.Frame) {
		f.Coord = coord.Polar().Frame(coord.Framing{Area: f.Area})
	})
	if !errors.Is(err, geom.ErrWarpedRaster) {
		t.Fatalf("a polar raster gave %v, want ErrWarpedRaster", err)
	}
}

// The mark that can have a colourbar, which is the argument for it existing
// beside the hexbin that cannot.
func TestARasterContributesAColourbar(t *testing.T) {
	src := lattice(8, 8, func(i, j int) float64 { return float64(i * j) })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"))
	if err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); err != nil {
		t.Fatalf("training the raster: %v", err)
	}
	gd, ok := g.(geom.Guided)
	if !ok {
		t.Fatal("a raster contributes no colour guide")
	}
	cg, ok := gd.ColorGuide()
	if !ok {
		t.Fatal("a raster with no ColorBy contributes no bar; it has values of its own")
	}
	if cg.Label != "v" {
		t.Errorf("the bar is titled %q, want the measured column", cg.Label)
	}
	if lo, hi := cg.Scale.Domain(); lo != 0 || hi != 49 {
		t.Errorf("the bar runs %v to %v, want the field's own extent", lo, hi)
	}
}

// A raster and a contour over one table refuse it with one sentence, because
// they resolve it through one lattice.
func TestARasterAndAContourRefuseATableAlike(t *testing.T) {
	xs := []float64{0, 1, 0, 1, 0}
	ys := []float64{0, 0, 1, 1, 2}
	vs := []float64{1, 2, 3, 4, 5}
	src := data.Float64Columns(map[string][]float64{"x": xs, "y": ys, "v": vs})

	msg := func(g geom.Geom) string {
		err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
		if err == nil {
			return ""
		}
		return err.Error()
	}
	r := msg(geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v")))
	c := msg(geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("v")))
	if r == "" || c == "" {
		t.Fatalf("a table of five rows over a 2 by 3 grid was accepted: raster %q, contour %q", r, c)
	}
	if strings.Replace(r, "raster", "contour", 1) != c {
		t.Errorf("the two marks disagree about one table:\n\traster:  %s\n\tcontour: %s", r, c)
	}
}

// A cell is centred on its reading, so the axis runs to the outer edge of the
// outer cells — otherwise half of the first cell and half of the last are drawn
// outside the panel.
func TestARasterTrainsTheAxesToItsCellEdges(t *testing.T) {
	src := lattice(5, 3, func(i, j int) float64 { return float64(i) })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"))
	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("training the raster: %v", err)
	}
	if lo, hi := x.Domain(); lo != -0.5 || hi != 4.5 {
		t.Errorf("x runs %v to %v, want the cell edges -0.5 to 4.5", lo, hi)
	}
	if lo, hi := y.Domain(); lo != -0.5 || hi != 2.5 {
		t.Errorf("y runs %v to %v, want the cell edges -0.5 to 2.5", lo, hi)
	}
}

// A pointer on the image names the row behind the cell under it, while a cell
// is something a reader can point at.
func TestARasterReportsTheRowUnderACell(t *testing.T) {
	src := lattice(4, 4, func(i, j int) float64 { return float64(4*j + i) })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"))

	var got markLog
	rasterFrame(t, g, 200, 200, func(f *geom.Frame) { f.Rows = &got })
	if len(got.rows) != 16 {
		t.Fatalf("a 4 by 4 field reported %d rows, want one per cell", len(got.rows))
	}
	// Row 0 is the cell at (0, 0), which is the bottom left of the panel.
	i := indexOfRow(got.rows, 0)
	if i < 0 {
		t.Fatal("the first cell was not reported")
	}
	if at := got.at[i]; at.X > 100 || at.Y < 100 {
		t.Errorf("the first cell was reported at %v, want the bottom left of the panel", at)
	}
}

// Below a pixel per cell there is no single row under a pixel: it is a
// reduction over several, and under Mean its value is not any one of them.
func TestARasterReportsNoRowWhenItsCellsAreSmallerThanPixels(t *testing.T) {
	src := lattice(400, 400, func(i, j int) float64 { return float64(i) })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"))

	var got markLog
	rasterFrame(t, g, 100, 100, func(f *geom.Frame) { f.Rows = &got })
	if len(got.rows) != 0 {
		t.Errorf("a field sixteen cells to the pixel reported %d rows, want none", len(got.rows))
	}
}

// A chart redrawn every frame repaints the same pixels. The buffer is the
// panel's size rather than the data's, so this is what keeps a pointer move
// from allocating a megabyte.
func TestARasterReusesItsBufferAcrossFrames(t *testing.T) {
	src := lattice(32, 32, func(i, j int) float64 { return float64(i + j) })
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"))

	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("training the raster: %v", err)
	}
	area := ir.R(0, 0, 200, 150)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)
	f := geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}

	var last *image.NRGBA
	seen := irtest.NullBackend()
	// The first frame allocates; every one after it must not.
	for range 3 {
		var held *image.NRGBA
		if err := g.Build(&imageSpy{Backend: seen, held: &held}, f); err != nil {
			t.Fatalf("drawing the raster: %v", err)
		}
		if held == nil {
			t.Fatal("the layer drew no image")
		}
		if last != nil && held != last {
			t.Fatal("a redraw allocated a new image rather than repainting the old one")
		}
		last = held
	}
}

// imageSpy keeps the pixels a layer handed over, which is how a test asks
// whether the next frame painted into the same ones.
type imageSpy struct {
	ir.Backend
	held **image.NRGBA
}

func (s *imageSpy) Image(img image.Image, dst ir.Rect) {
	if n, ok := img.(*image.NRGBA); ok {
		*s.held = n
	}
}

// markLog collects what a layer reported about its rows.
type markLog struct {
	at   []ir.Point
	rows []int
}

func (m *markLog) Marks(r geom.MarkRows) {
	m.at = append(m.at[:0], r.At...)
	m.rows = append(m.rows[:0], r.Rows...)
}

func indexOfRow(rows []int, want int) int {
	for i, r := range rows {
		if r == want {
			return i
		}
	}
	return -1
}

func nrgba(img image.Image, x, y int) color.NRGBA {
	c, ok := img.(*image.NRGBA)
	if !ok {
		r, g, b, a := img.At(x, y).RGBA()
		return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}
	return c.NRGBAAt(x, y)
}

// A classed scale is not sampled, for the reason render draws its bar in bands
// rather than as a gradient: a class boundary is an edge, and an edge is what
// sampling steps over.
func TestAClassedRasterPaintsInItsClasses(t *testing.T) {
	// Values 0, 10, 20, 30 across, cut at 10 and 20: three classes.
	src := lattice(4, 2, func(i, j int) float64 { return float64(i) * 10 })
	cs := scale.Threshold(palette.Viridis, []float64{10, 20})
	cs.Train(0, 30)
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"), geom.ColorBy("v", cs))

	rec := rasterFrame(t, g, 40, 20)
	img := rec.Filter("Image")[0].Image
	seen := map[color.NRGBA]bool{}
	for px := range 40 {
		seen[nrgba(img, px, 10)] = true
	}
	if len(seen) != 3 {
		t.Fatalf("three classes were painted in %d colours, want three", len(seen))
	}
	// The colours are the ones the colourbar beside it would draw: the scale's
	// own reading of the middle of each class.
	for _, mid := range []float64{5, 15, 25} {
		if !seen[cs.Color(mid)] {
			t.Errorf("the class holding %v was painted in a colour the bar would not use", mid)
		}
	}
}

// A value equal to a boundary belongs to the class above it, which is the rule
// scale.Threshold states and a table indexed the other way would break.
func TestAValueOnABreakTakesTheClassAbove(t *testing.T) {
	src := lattice(2, 2, func(i, j int) float64 { return float64(i) * 10 })
	cs := scale.Threshold(palette.Viridis, []float64{10})
	cs.Train(0, 10)
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"), geom.ColorBy("v", cs))

	rec := rasterFrame(t, g, 40, 20)
	img := rec.Filter("Image")[0].Image
	if got, want := nrgba(img, 30, 10), cs.Color(10); got != want {
		t.Errorf("the cell holding the boundary is %v, want the class above it, %v", got, want)
	}
}

// A ramp position clamps where a scale refuses: a log colour scale has no
// colour at all for zero, and the table must not answer with the bottom of the
// ramp on its behalf.
func TestALogRampLeavesAValueItCannotPlaceUndefined(t *testing.T) {
	undef := ir.RGBA(1, 2, 3, 200)
	cs := scale.Sequential(palette.Viridis, scale.ColorLog(10), scale.ColorUndefined(undef))
	cs.Train(1, 1000)
	// One cell at zero, the rest on the ramp.
	src := lattice(2, 2, func(i, j int) float64 {
		if i == 0 && j == 0 {
			return 0
		}
		return 1000
	})
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"), geom.ColorBy("v", cs))

	rec := rasterFrame(t, g, 40, 40)
	img := rec.Filter("Image")[0].Image
	// The bottom left cell, which is the one holding the zero.
	if got := nrgba(img, 10, 30); got != undef {
		t.Errorf("the cell at zero is %v, want the scale's undefined colour %v", got, undef)
	}
	if got, want := nrgba(img, 30, 10), cs.Color(1000); got != want {
		t.Errorf("a cell on the ramp is %v, want %v", got, want)
	}
}

// The domain can move under a layer that has already trained: two layers may
// share one ramp, and the second one's training is what the first one has to
// paint from. The position is therefore read from the scale every time rather
// than baked into the table.
func TestThePictureFollowsARetrainedRamp(t *testing.T) {
	src := lattice(2, 2, func(i, j int) float64 { return float64(i) * 100 })
	ramp := scale.Sequential(palette.Viridis)
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"), geom.ColorBy("v", ramp))

	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("training the raster: %v", err)
	}
	area := ir.R(0, 0, 40, 20)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)
	f := geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}

	// The cell holding 100, which is the top of the ramp until the ramp grows.
	draw := func() color.NRGBA {
		rec := irtest.New()
		if err := g.Build(rec, f); err != nil {
			t.Fatalf("drawing the raster: %v", err)
		}
		return nrgba(rec.Filter("Image")[0].Image, 30, 10)
	}
	before := draw()
	if want := ramp.Color(100); !alike(before, want) {
		t.Fatalf("the high cell is %v, want the ramp's reading of its value, %v", before, want)
	}

	// Another layer trains the shared ramp wider. The cell still holds 100, so
	// it is now a quarter of the way along the ramp rather than at the end of
	// it — and a table left over from the narrower domain would be read at the
	// new position and paint the value that used to be there.
	ramp.Train(400)
	want := ramp.Color(100)
	if alike(before, want) {
		t.Fatal("widening the ramp did not change the colour of the cell; the test proves nothing")
	}
	if got := draw(); !alike(got, want) {
		t.Errorf("after the ramp was retrained the cell is %v, want %v", got, want)
	}
}

// alike compares two colours at the resolution the picture is drawn in. The
// ramp is read into a table of a thousand samples, so a value between two of
// them can round to the neighbouring byte — which is the sampling the mark
// documents and not a colour a reader can see.
func alike(a, b color.NRGBA) bool {
	near := func(x, y uint8) bool { return int(x)-int(y) <= 2 && int(y)-int(x) <= 2 }
	return near(a.R, b.R) && near(a.G, b.G) && near(a.B, b.B) && a.A == b.A
}

// A classed table is the one the domain is baked into: its boundaries are
// where the table's index comes from, and a scale that cuts its own domain
// moves them every time the domain moves. A continuous table is the ramp
// itself and does not move, which is why only this one needs the check.
func TestAClassedTableIsRebuiltWhenTheDomainMoves(t *testing.T) {
	src := lattice(2, 2, func(i, j int) float64 { return float64(i) * 10 })
	cs := scale.Quantize(palette.Viridis, 2)
	cs.Train(0, 20)
	g := geom.Raster(src, geom.X("x"), geom.Y("y"), geom.Z("v"), geom.ColorBy("v", cs))

	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("training the raster: %v", err)
	}
	area := ir.R(0, 0, 40, 20)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)
	f := geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}

	// The cell holding 10, which is in the upper class.
	draw := func() color.NRGBA {
		rec := irtest.New()
		if err := g.Build(rec, f); err != nil {
			t.Fatalf("drawing the raster: %v", err)
		}
		return nrgba(rec.Filter("Image")[0].Image, 30, 10)
	}
	before := draw()

	// The scale cuts its own domain, so the boundary that was at 10 is now at
	// 50 — and the cell holding 10 has moved from the upper class to the
	// lower one.
	cs.Train(100)
	want := cs.Color(10)
	if before == want {
		t.Fatal("widening the scale did not move the class's colour; the test proves nothing")
	}
	if got := draw(); got != want {
		t.Errorf("after the scale was retrained the cell is %v, want the new middle of its class, %v", got, want)
	}
}

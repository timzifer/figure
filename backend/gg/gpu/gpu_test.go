package gpu_test

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"testing"

	"github.com/timzifer/figure"
	ggbackend "github.com/timzifer/figure/backend/gg"
	"github.com/timzifer/figure/backend/gg/gpu"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/three"
)

// What this module does is register an accelerator, and whether the
// registration takes depends on the machine: a CI runner has no GPU and a
// developer's laptop does. So the test is the property that has to hold either
// way — a chart renders, and it renders the same chart.
//
// Nothing here closes the tier until the test that has to: these run in one
// process in the order they are written, Close is final — the accelerator
// cannot be registered again once given back that way — and every test before
// the close is one that wants the tier as the import left it. Disable is not
// final, and the test of it puts the tier back the way it found it.

func TestAChartRendersWithTheTierEitherWay(t *testing.T) {
	t.Logf("GPU tier enabled: %v", gpu.Enabled())

	src := figure.Float64Columns(map[string][]float64{
		"x": {0, 1, 2, 3},
		"y": {0, 2, 1, 3},
	})
	p := figure.New(figure.Size(200, 150), figure.Title("GPU"))
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))

	var buf bytes.Buffer
	if err := p.Render(ggbackend.Writer(&buf, ggbackend.FormatPNG)); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("nothing was encoded")
	}

	// Decoding it rather than measuring it: a GPU tier that never flushed
	// encodes a perfectly valid PNG of an empty buffer, and a length check
	// passes on that. The chart has a title and a line on it, so more than one
	// colour has to come back.
	img, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decoding what was rendered: %v", err)
	}
	if !painted(img) {
		t.Error("the encoded chart is a single flat colour")
	}

	// The vector emitter does not go near a GPU, so it is the reference for
	// what the chart is: whatever the raster tier does, the geometry is the
	// same geometry.
	var svg bytes.Buffer
	if err := p.Render(figure.SVGWriter(&svg)); err != nil {
		t.Fatalf("Render SVG: %v", err)
	}
	if svg.Len() == 0 {
		t.Fatal("the reference render is empty")
	}
}

// painted reports whether an image has more than one colour in it.
func painted(img image.Image) bool {
	b := img.Bounds()
	if b.Empty() {
		return false
	}
	first := img.At(b.Min.X, b.Min.Y)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.At(x, y) != first {
				return true
			}
		}
	}
	return false
}

// maxWrongFraction is how much of a surface may differ from the CPU in colour
// before the difference is a lost face rather than anti-aliasing.
const maxWrongFraction = 0.02

func renderSurface(t *testing.T) image.Image {
	t.Helper()
	const n = 28
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	zs := make([]float64, 0, n*n)
	for j := range n {
		for i := range n {
			x := -3 + 6*float64(i)/(n-1)
			y := -3 + 6*float64(j)/(n-1)
			xs, ys = append(xs, x), append(ys, y)
			zs = append(zs, math.Sin(x)*math.Cos(y)*1.4+0.25*x)
		}
	}
	src := figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("z", zs)
	sc := three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("z")).
		Z(scale.Linear(scale.Nice())).
		Add(three.Surface(src, geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Fill(palette.SkyBlue)))
	p := three.New(three.Size(900, 700), three.Columns(2)).Scene(sc).Add(
		three.View{Camera: three.Home()},
		three.View{Camera: three.LookAt(three.Azimuth(-1.1), three.Elevation(0.5))},
		three.View{Camera: three.LookAt(three.Azimuth(0.4), three.Elevation(0.25))},
		three.View{Camera: three.LookAt(three.Azimuth(2.2), three.Elevation(0.6))},
	)
	var buf bytes.Buffer
	if err := p.Render(ggbackend.Writer(&buf, ggbackend.FormatPNG)); err != nil {
		t.Fatalf("Render: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return img
}

// wrongPixels counts the pixels where two pictures disagree by more than
// anti-aliasing can explain: some channel more than 48 apart.
func wrongPixels(a, b image.Image) (bad, total int) {
	r := a.Bounds().Intersect(b.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			total++
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			for _, d := range []int{int(ar>>8) - int(br>>8), int(ag>>8) - int(bg>>8), int(ab>>8) - int(bb>>8)} {
				if d > 48 || d < -48 {
					bad++
					break
				}
			}
		}
	}
	return bad, total
}

// A program offering "GPU on/off" switches the tier off and on again in one
// process, so the tier has to come back from Disable drawing — not registered
// over a device that is gone, which would draw the text and none of the
// geometry. The CPU render in between is the reference for how much ink there
// should be, as in the test below.
func TestDisableThenEnableBringsTheTierBack(t *testing.T) {
	was := gpu.Enabled()
	t.Logf("GPU tier enabled: %v", was)

	gpu.Disable()
	if gpu.Enabled() {
		t.Fatal("the tier is still registered after Disable")
	}
	if gpu.Available() != was {
		t.Errorf("Available = %v after Disable, want %v: a tier set aside can still be had", !was, was)
	}
	onCPU := ink(t, render(t))
	if onCPU == 0 {
		t.Fatal("the reference render is blank")
	}
	gpu.Disable() // a second Disable must not forget what the first set aside

	if got := gpu.Enable(); got != was {
		t.Fatalf("Enable = %v, but the tier was %v before Disable", got, was)
	}
	if gpu.Enabled() != was {
		t.Fatalf("Enabled = %v after Enable, want %v", gpu.Enabled(), was)
	}
	if back := ink(t, render(t)); back*2 < onCPU {
		t.Errorf("the tier drew %d ink pixels after coming back, against the CPU's %d: paths are being dropped",
			back, onCPU)
	}
	if !gpu.Enable() && was {
		t.Error("Enable with the tier already on reported false")
	}
}

// A tier that registers but cannot draw is the failure this package guards
// against: gg's path operations queued work for a device that was never
// obtained, dropped it at flush, and left a chart with its labels and none of
// its geometry. The CPU render is the reference for how much ink a chart has;
// the tier has to put down a comparable amount rather than a handful of
// glyphs.
//
// A projected surface is compared as well, colour by colour, and it is the
// figure that found this tier's worst bug: gg's GPU pipeline, after one fill of
// several large adjacent subpaths, lost most of the fills that followed over
// the same tiles — which was a cube's three walls and then most of a surface's
// faces. The flat chart never makes such a fill, so it passed while
// three-dimensional charts came out in shreds, and an ink count would have
// passed too: the failure leaves plenty of ink, the wall and the faces behind a
// lost one, in the wrong place.
//
// Both comparisons live in this one test because the tier cannot be registered
// again once given back. Rendering on the tier and on the CPU either side of
// one Close, in one test, is what stops a comparison quietly measuring the GPU
// against itself — which is what two tests relying on their order did.
func TestTheTierPutsDownAsMuchInkAsTheCPU(t *testing.T) {
	enabled := gpu.Enabled()
	t.Logf("GPU tier enabled: %v", enabled)

	// The tier as the import left it, then the same pictures with it given
	// back. This is where it is given back, which is why it runs before the
	// test that asserts Close is a no-op.
	withTier := ink(t, render(t))
	surfaceOnTier := renderSurface(t)
	signalOnTier := renderSignal(t)
	gpu.Close()
	if gpu.Enabled() {
		t.Fatal("the tier is still registered after Close, so the reference would be drawn on it")
	}
	onCPU := ink(t, render(t))
	surfaceOnCPU := renderSurface(t)
	signalOnCPU := renderSignal(t)

	if onCPU == 0 {
		t.Fatal("the reference render is blank")
	}
	if withTier*2 < onCPU {
		t.Errorf("the tier drew %d ink pixels against the CPU's %d: paths are being dropped", withTier, onCPU)
	}

	if !enabled {
		return // both pictures came from the CPU, so there is nothing to compare
	}
	bad, total := wrongPixels(surfaceOnTier, surfaceOnCPU)
	t.Logf("surface: %d of %d pixels differ by more than anti-aliasing (%.2f%%)",
		bad, total, 100*float64(bad)/float64(total))
	if float64(bad) > maxWrongFraction*float64(total) {
		t.Errorf("%d of %d pixels of a surface differ from the CPU by more than anti-aliasing can explain: "+
			"faces are being lost", bad, total)
	}

	// A thick line over a dense signal turns back on itself at almost every
	// sample. gg up to v0.52.5 filled a GPU stroke's outline even-odd, which
	// cancels wherever the outline covers itself, so such a line came out as
	// its own outline with the inside missing. The fork figure builds against
	// fills it non-zero, as gg's CPU stroker does.
	bad, total = wrongPixels(signalOnTier, signalOnCPU)
	t.Logf("signal: %d of %d pixels differ by more than anti-aliasing (%.2f%%)",
		bad, total, 100*float64(bad)/float64(total))
	if float64(bad) > maxWrongFraction*float64(total) {
		t.Errorf("%d of %d pixels of a thick line differ from the CPU by more than anti-aliasing can explain: "+
			"the stroke is cancelling where it overlaps itself", bad, total)
	}
}

// renderSignal is a thick line over a dense signal: the case a stroke outline
// filled even-odd gets wrong, because nearly every turn overlaps itself.
func renderSignal(t *testing.T) image.Image {
	t.Helper()
	const n = 400
	ts := make([]float64, n)
	vs := make([]float64, n)
	for i := range n {
		x := float64(i) / 40
		ts[i] = x
		vs[i] = math.Sin(x) + 0.55*math.Sin(37*x) + 0.35*math.Sin(91*x+0.3)
	}
	src := figure.Float64Columns(map[string][]float64{"t": ts, "v": vs})
	p := figure.New(figure.Size(900, 360))
	p.X(scale.Linear())
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Line(src, geom.X("t"), geom.Y("v"), geom.Color(palette.Blue), geom.Width(5)))

	var buf bytes.Buffer
	if err := p.Render(ggbackend.Writer(&buf, ggbackend.FormatPNG)); err != nil {
		t.Fatalf("Render: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return img
}

func render(t *testing.T) image.Image {
	t.Helper()
	src := figure.Float64Columns(map[string][]float64{
		"x": {0, 1, 2, 3},
		"y": {0, 2, 1, 3},
	})
	p := figure.New(figure.Size(200, 150), figure.Title("Ink"))
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))

	var buf bytes.Buffer
	if err := p.Render(ggbackend.Writer(&buf, ggbackend.FormatPNG)); err != nil {
		t.Fatalf("Render: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return img
}

// ink counts the pixels that are not the colour of the top-left corner.
func ink(t *testing.T, img image.Image) int {
	t.Helper()
	b := img.Bounds()
	bg := img.At(b.Min.X, b.Min.Y)
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.At(x, y) != bg {
				n++
			}
		}
	}
	return n
}

// Closing twice, and closing with no GPU, are both no-ops: a program that
// defers Close should not have to ask whether the tier took.
func TestCloseIsSafeWhateverHappened(t *testing.T) {
	gpu.Close()
	gpu.Close()
	if gpu.Enabled() {
		t.Error("the accelerator is still registered after Close")
	}
}

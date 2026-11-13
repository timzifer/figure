package stilus_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"testing"

	"github.com/go-opentype/fonts/dejavusans"
	"github.com/go-opentype/fonts/goregular"
	"github.com/timzifer/figure"
	stilusbackend "github.com/timzifer/figure/backend/stilus"
	"github.com/timzifer/figure/backend/svg"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
)

// open is a backend on a fresh surface, and the surface to read it back from.
func open(t *testing.T, w, h int, dpr float64, opts ...stilusbackend.Option) (ir.Backend, *stilusbackend.Surface) {
	t.Helper()
	s := stilusbackend.NewSurface(opts...)
	b, err := s.Open(ir.Surface{WidthPx: w, HeightPx: h, DPR: dpr})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return b, s
}

func flush(t *testing.T, b ir.Backend) {
	t.Helper()
	if err := b.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func alphaAt(img image.Image, x, y int) uint8 {
	_, _, _, a := img.At(x, y).RGBA()
	return uint8(a >> 8)
}

// ink is the bounding box of every pixel with any coverage.
func ink(img image.Image) image.Rectangle {
	var r image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if alphaAt(img, x, y) != 0 {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

func rect(x0, y0, x1, y1 float32) *ir.Path {
	var p ir.Path
	p.Rect(ir.R(x0, y0, x1, y1))
	return &p
}

var black = ir.RGB(0, 0, 0)

// The layout is computed from Measure and drawn by Text, so the two have to
// agree on how wide a run is: a run aligned to end at x is the run aligned to
// start at x minus its advance, pixel for pixel up to the glyph cache's
// subpixel steps.
func TestARunEndsWhereItsAdvanceSaysItDoes(t *testing.T) {
	run := ir.TextRun{Text: "amplitude 09:15", Font: ir.FontRef{Size: 13}, Color: black}

	b1, s1 := open(t, 200, 40, 1)
	adv := b1.Measure(run).Advance
	run.At = ir.Point{X: 20, Y: 25}
	b1.Text(run)
	flush(t, b1)

	b2, s2 := open(t, 200, 40, 1)
	run.At = ir.Point{X: 20 + adv, Y: 25}
	run.H = ir.AlignEnd
	b2.Text(run)
	flush(t, b2)

	a, z := ink(s1.Image()), ink(s2.Image())
	if a.Empty() {
		t.Fatal("the run drew nothing")
	}
	if d := math.Abs(float64(a.Min.X-z.Min.X)) + math.Abs(float64(a.Max.X-z.Max.X)); d > 1 {
		t.Errorf("start-aligned ink %v, end-aligned at the advance %v", a, z)
	}
}

// Advances are the font's own widths, unrounded, so they agree with the SVG
// backend's measurement of the same font — which is figure's own hmtx reader —
// to float32 rounding. The font box does not: the SVG backend reads hhea and
// this one reads OS/2, as gg does (ADR 0094), and on the Go fonts that is the
// 15 % backend/gg's parity test allows.
func TestMeasurementAgreesWithTheVectorEmitter(t *testing.T) {
	var buf bytes.Buffer
	sb, err := svg.Writer(&buf, svg.WithFont(goregular.TTF)).Open(ir.Surface{WidthPx: 100, HeightPx: 100, DPR: 1})
	if err != nil {
		t.Fatal(err)
	}
	rb, _ := open(t, 100, 100, 1)
	for _, size := range []float64{10, 11, 12, 16, 24} {
		for _, text := range []string{"0", "10.0", "-2.5", "09:15", "amplitude", "Mar 14 2026"} {
			run := ir.TextRun{Text: text, Font: ir.FontRef{Size: size}}
			a, b := sb.Measure(run), rb.Measure(run)
			if d := math.Abs(float64(a.Advance - b.Advance)); d > 0.01 {
				t.Errorf("%q at %v px: svg %v, stilus %v", text, size, a.Advance, b.Advance)
			}
			if rel := math.Abs(float64(a.Height()-b.Height())) / float64(a.Height()); rel > 0.20 {
				t.Errorf("font box at %v px: svg %v, stilus %v", size, a.Height(), b.Height())
			}
		}
	}
}

// The glyph cache reads its matrix's scale as an em size quantised to 1/64.
// An outline in design units gave a 10 px label a scale that rounded to zero:
// it was measured, laid out and never drawn.
func TestSmallTextIsDrawnAtItsSize(t *testing.T) {
	for _, size := range []float64{6, 8, 10, 12, 16, 32} {
		b, s := open(t, 120, 80, 1)
		b.Text(ir.TextRun{Text: "H", Font: ir.FontRef{Size: size}, At: ir.Point{X: 10, Y: 60}, Color: black})
		flush(t, b)
		got := float64(ink(s.Image()).Dy())
		// Go Regular's H is 1466 units of 2048 tall.
		want := size * 1466 / 2048
		if math.Abs(got-want) > 2 {
			t.Errorf("an H at %v px is %v px of ink tall, want about %.1f", size, got, want)
		}
	}
}

// A rune the embedded fonts cannot draw is a `?`, and a fallback font that has
// it draws it instead.
func TestAMissingRuneIsAQuestionMarkUnlessAFallbackHasIt(t *testing.T) {
	const bracket = "⟨"
	draw := func(text string, opts ...stilusbackend.Option) *image.RGBA {
		b, s := open(t, 60, 40, 1, opts...)
		b.Text(ir.TextRun{Text: text, Font: ir.FontRef{Size: 20}, At: ir.Point{X: 10, Y: 30}, Color: black})
		flush(t, b)
		return s.Image().(*image.RGBA)
	}
	if !bytes.Equal(draw(bracket).Pix, draw("?").Pix) {
		t.Error("a rune the font lacks was not drawn as ?")
	}
	withFallback := draw(bracket, stilusbackend.WithFallbackFont(dejavusans.TTF))
	if bytes.Equal(withFallback.Pix, draw("?").Pix) {
		t.Error("a fallback font that has the rune still drew ?")
	}
	if ink(withFallback).Empty() {
		t.Error("the fallback drew nothing")
	}
}

func TestASuppliedFontIsUsedAndABrokenOneIsAnError(t *testing.T) {
	b, _ := open(t, 100, 40, 1, stilusbackend.WithFont(dejavusans.TTF, nil, nil))
	d, _ := open(t, 100, 40, 1)
	run := ir.TextRun{Text: "amplitude", Font: ir.FontRef{Size: 16}}
	if b.Measure(run).Advance == d.Measure(run).Advance {
		t.Error("a supplied font measures the same as the embedded one")
	}
	s := stilusbackend.NewSurface(stilusbackend.WithFont([]byte("not a font"), nil, nil))
	if _, err := s.Open(ir.Surface{WidthPx: 10, HeightPx: 10, DPR: 1}); err == nil {
		t.Error("a broken font opened")
	}
}

func TestARectangleClipBoundsWhatIsDrawn(t *testing.T) {
	b, s := open(t, 100, 100, 1)
	b.Push(rect(20, 30, 60, 70), ir.Identity)
	b.FillPath(rect(0, 0, 100, 100), ir.Solid(black), ir.NonZero)
	b.Pop()
	b.FillPath(rect(90, 90, 100, 100), ir.Solid(black), ir.NonZero) // after the pop: unclipped
	flush(t, b)
	img := s.Image()
	if alphaAt(img, 40, 50) != 255 || alphaAt(img, 10, 10) != 0 || alphaAt(img, 65, 50) != 0 {
		t.Error("the fill escaped its clip or missed it")
	}
	if alphaAt(img, 95, 95) != 255 {
		t.Error("Pop did not lift the clip")
	}
}

func TestAPathClipBoundsWhatIsDrawn(t *testing.T) {
	b, s := open(t, 100, 100, 1)
	var tri ir.Path
	tri.MoveTo(0, 0)
	tri.LineTo(100, 0)
	tri.LineTo(0, 100)
	tri.Close()
	b.Push(&tri, ir.Identity)
	b.FillPath(rect(0, 0, 100, 100), ir.Solid(black), ir.NonZero)
	b.Pop()
	flush(t, b)
	if img := s.Image(); alphaAt(img, 20, 20) != 255 || alphaAt(img, 80, 80) != 0 {
		t.Error("a triangular clip did not cut the square diagonally")
	}
}

// A transform composes with the one already on the stack, a mirror included.
func TestTransformsComposeAndMayMirror(t *testing.T) {
	b, s := open(t, 100, 100, 1)
	b.Push(nil, ir.Translate(50, 0))
	b.Push(nil, ir.Affine{A: -1, D: 1}) // mirror about the translated origin
	b.FillPath(rect(10, 10, 20, 20), ir.Solid(black), ir.NonZero)
	b.Pop()
	b.Pop()
	flush(t, b)
	if r := ink(s.Image()); r != image.Rect(30, 10, 40, 20) {
		t.Errorf("the square landed at %v, want (30,10)-(40,20)", r)
	}
}

// A line thinner than a pixel is still drawn, and does not vanish.
func TestAHairlineIsDrawn(t *testing.T) {
	b, s := open(t, 100, 20, 1)
	b.Polyline([]ir.Point{{X: 5, Y: 10.5}, {X: 95, Y: 10.5}}, ir.Stroke{Color: black, Width: 0.25})
	flush(t, b)
	if r := ink(s.Image()); r.Dx() < 85 || r.Dy() > 2 {
		t.Errorf("a quarter-pixel line has ink %v", r)
	}
}

func TestADeviceScaleDrawsTheSameChartInMorePixels(t *testing.T) {
	b, s := open(t, 50, 50, 2)
	b.FillPath(rect(10, 10, 20, 20), ir.Solid(black), ir.NonZero)
	flush(t, b)
	if got := s.Image().Bounds(); got != image.Rect(0, 0, 100, 100) {
		t.Errorf("the buffer is %v", got)
	}
	if r := ink(s.Image()); r != image.Rect(20, 20, 40, 40) {
		t.Errorf("a 10-unit square at 2x is %v", r)
	}
}

// Colours between two stops blend in linear light: black to white is 188 at
// the middle, not 128 — palette.Lerp's number, and gg's.
func TestAGradientBlendsInLinearLight(t *testing.T) {
	b, s := open(t, 257, 4, 1)
	fill := ir.Fill{
		Start: ir.Point{X: 0}, End: ir.Point{X: 257},
		Stops: []ir.GradientStop{{Offset: 0, Color: black}, {Offset: 1, Color: ir.RGB(255, 255, 255)}},
	}
	b.FillPath(rect(0, 0, 257, 4), fill, ir.NonZero)
	flush(t, b)
	c := s.Image().At(128, 2).(color.RGBA)
	if c.R < 184 || c.R > 192 {
		t.Errorf("the midpoint of black to white is %v, want about 188", c.R)
	}
}

func TestAnImageIsStretchedOverItsRectangle(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for i := range src.Pix {
		src.Pix[i] = 255
	}
	src.Pix[3] = 0 // top-left pixel transparent
	b, s := open(t, 100, 100, 1)
	b.Image(src, ir.R(20, 20, 60, 60))
	flush(t, b)
	img := s.Image()
	if r := ink(img); r.Min.X < 19 || r.Max.X > 61 || r.Min.Y < 19 || r.Max.Y > 61 {
		t.Errorf("the image drew outside its rectangle: %v", r)
	}
	if alphaAt(img, 50, 50) != 255 || alphaAt(img, 22, 22) > 64 {
		t.Error("the picture was not stretched over the rectangle as it is")
	}
}

func TestTheFileTargetsEncode(t *testing.T) {
	p := figure.New(figure.Size(160, 120), figure.Title("t"))
	p.Add(geom.Line(figure.Float64Columns(map[string][]float64{"x": {0, 1, 2}, "y": {0, 2, 1}}), geom.X("x"), geom.Y("y")))

	var pb bytes.Buffer
	if err := p.Render(stilusbackend.Writer(&pb, stilusbackend.FormatPNG)); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&pb)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 160 || ink(img).Empty() {
		t.Errorf("the PNG is %v with ink %v", img.Bounds(), ink(img))
	}

	var jb bytes.Buffer
	if err := p.Render(stilusbackend.Writer(&jb, stilusbackend.FormatJPEG, stilusbackend.JPEGQuality(50))); err != nil {
		t.Fatal(err)
	}
	if _, err := jpeg.Decode(&jb); err != nil {
		t.Errorf("the JPEG does not decode: %v", err)
	}
}

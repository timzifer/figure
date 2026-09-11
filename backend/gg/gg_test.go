package gg_test

import (
	"bytes"
	"flag"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/timzifer/figure"
	ggbackend "github.com/timzifer/figure/backend/gg"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

var update = flag.Bool("update", false, "rewrite the golden PNG files")

// Rendering is deterministic on gg's CPU path, so in practice the diff against
// a golden image is exactly zero. The tolerance exists so that a rasterizer
// improvement upstream, or a platform that rounds anti-aliasing differently,
// surfaces as a reviewable difference rather than as a red build everywhere.
const (
	pngTolerance     = 0.001 // fraction of channels allowed to differ
	channelTolerance = 2     // out of 255
)

func goldenPNG(t *testing.T, name string, p *figure.Plot) {
	t.Helper()

	var buf bytes.Buffer
	if err := p.Render(ggbackend.Writer(&buf, ggbackend.FormatPNG)); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.Bytes()

	path := filepath.Join("testdata", "golden", name+".png")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run `go test ./... -update` to create it)", err)
	}
	diff, err := imageDiff(want, got)
	if err != nil {
		t.Fatalf("comparing %s: %v", path, err)
	}
	if diff > pngTolerance {
		t.Errorf("%s differs: %.4f%% of channels beyond tolerance", path, diff*100)
	}
}

func TestGoldenLine(t *testing.T) {
	xs := ramp(0, 10, 60)
	src := figure.Float64Columns(map[string][]float64{
		"x": xs,
		"y": apply(xs, math.Sin),
	})
	p := figure.New(figure.Size(480, 300), figure.Title("Sine"), figure.YTitle("sin x"))
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y"), geom.Color(palette.Blue)))
	goldenPNG(t, "line", p)
}

func TestGoldenMarkersAndBars(t *testing.T) {
	t.Run("markers", func(t *testing.T) {
		xs := ramp(0, 6, 15)
		src := figure.Float64Columns(map[string][]float64{"x": xs, "y": apply(xs, math.Cos)})
		p := figure.New(figure.Size(400, 260), figure.Title("Markers"))
		p.Add(geom.Scatter(src, geom.X("x"), geom.Y("y"),
			geom.Shape(ir.MarkerTriangle), geom.Size(10)))
		goldenPNG(t, "markers", p)
	})

	t.Run("bars", func(t *testing.T) {
		src := figure.Float64Columns(map[string][]float64{
			"x": {1, 2, 3, 4, 5},
			"y": {3, 7, 4, 9, 6},
		})
		p := figure.New(figure.Size(400, 260), figure.Title("Bars"))
		p.Y(scale.Linear(scale.Nice(), scale.Zero()))
		p.Add(geom.Bar(src, geom.X("x"), geom.Y("y"), geom.Color(palette.Green)))
		goldenPNG(t, "bars", p)
	})
}

func TestGoldenDarkThemeWithTimeAxis(t *testing.T) {
	const n = 90
	start := time.Date(2026, time.March, 14, 9, 0, 0, 0, time.UTC)
	times := make([]time.Time, n)
	values := make([]float64, n)
	for i := range n {
		times[i] = start.Add(time.Duration(i) * time.Minute)
		values[i] = math.Sin(float64(i) / 6)
	}
	src := figure.NewTable().Time("t", times).Float64("y", values)

	p := figure.New(figure.Theme(theme.Dark), figure.Size(560, 300), figure.Title("Signal"))
	p.X(scale.Time())
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Line(src, geom.X("t"), geom.Y("y"), geom.Color(palette.SkyBlue), geom.Tension(0.5)))
	goldenPNG(t, "time-dark", p)
}

// --- behaviour -----------------------------------------------------------

func TestPNGHasTheRequestedSize(t *testing.T) {
	img := renderImage(t, plot(400, 250), 1)
	if got := img.Bounds().Size(); got.X != 400 || got.Y != 250 {
		t.Fatalf("image is %v, want 400x250", got)
	}
}

func TestDeviceScaleEnlargesThePixelBufferOnly(t *testing.T) {
	// Coordinates stay in device-independent units; only the buffer grows. A
	// chart at DPR 2 must be the same picture, twice the pixels.
	one := renderImage(t, plot(200, 120), 1)
	two := renderImage(t, plot(200, 120), 2)

	if got := two.Bounds().Size(); got.X != 2*one.Bounds().Dx() || got.Y != 2*one.Bounds().Dy() {
		t.Fatalf("DPR 2 produced %v, want twice %v", got, one.Bounds().Size())
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	a := renderBytes(t, plot(300, 200))
	b := renderBytes(t, plot(300, 200))
	if !bytes.Equal(a, b) {
		t.Fatal("two identical renders produced different PNGs")
	}
}

func TestBackgroundIsActuallyPainted(t *testing.T) {
	img := renderImage(t, plot(120, 80), 1)
	r, g, b, a := img.At(2, 2).RGBA()
	if a>>8 != 255 || r>>8 != 255 || g>>8 != 255 || b>>8 != 255 {
		t.Fatalf("corner pixel is (%d,%d,%d,%d), want the light theme's opaque white",
			r>>8, g>>8, b>>8, a>>8)
	}
}

func TestDarkThemeCornerIsDark(t *testing.T) {
	p := plot(120, 80)
	dark := figure.New(figure.Theme(theme.Dark), figure.Size(120, 80))
	dark.Add(geom.Line(
		figure.Float64Columns(map[string][]float64{"x": {0, 1}, "y": {0, 1}}),
		geom.X("x"), geom.Y("y")))
	_ = p

	img := renderImage(t, dark, 1)
	r, _, _, a := img.At(2, 2).RGBA()
	if a>>8 != 255 || r>>8 > 40 {
		t.Fatalf("corner pixel red channel is %d with alpha %d, want a dark opaque background", r>>8, a>>8)
	}
}

func TestJPEGTarget(t *testing.T) {
	var buf bytes.Buffer
	if err := plot(200, 150).Render(ggbackend.Writer(&buf, ggbackend.FormatJPEG)); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("no JPEG was written")
	}
	// JPEG files start with the SOI marker.
	if b := buf.Bytes(); b[0] != 0xFF || b[1] != 0xD8 {
		t.Fatalf("output does not start with a JPEG SOI marker: %x", b[:2])
	}
}

func TestFileTargets(t *testing.T) {
	dir := t.TempDir()
	for name, target := range map[string]ir.Target{
		"chart.png":  ggbackend.PNG(filepath.Join(dir, "chart.png")),
		"chart.jpeg": ggbackend.JPEG(filepath.Join(dir, "chart.jpeg")),
	} {
		if err := plot(200, 150).Render(target); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if st.Size() == 0 {
			t.Errorf("%s is empty", name)
		}
	}
}

func TestBadFontIsReportedNotIgnored(t *testing.T) {
	var buf bytes.Buffer
	err := plot(100, 100).Render(ggbackend.Writer(&buf, ggbackend.FormatPNG,
		ggbackend.WithFont([]byte("not a font"), nil, nil)))
	if err == nil {
		t.Fatal("a malformed font must fail the render, not silently fall back")
	}
}

func TestZeroSizeIsRejected(t *testing.T) {
	target := ggbackend.PNG(filepath.Join(t.TempDir(), "x.png"))
	if _, err := target.Open(ir.Surface{WidthPx: 0, HeightPx: 100, DPR: 1}); err == nil {
		t.Fatal("a zero-width surface must be rejected")
	}
}

// --- helpers -------------------------------------------------------------

func plot(w, h int) *figure.Plot {
	src := figure.Float64Columns(map[string][]float64{"x": {0, 1, 2}, "y": {0, 2, 1}})
	p := figure.New(figure.Size(w, h), figure.Title("t"))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))
	return p
}

func renderBytes(t *testing.T, p *figure.Plot) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := p.Render(ggbackend.Writer(&buf, ggbackend.FormatPNG)); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return buf.Bytes()
}

func renderImage(t *testing.T, p *figure.Plot, dpr float64) image.Image {
	t.Helper()
	if dpr != 1 {
		// Rebuild the plot with the requested device pixel ratio.
		t.Helper()
	}
	var buf bytes.Buffer
	var err error
	if dpr == 1 {
		err = p.Render(ggbackend.Writer(&buf, ggbackend.FormatPNG))
	} else {
		err = withDPR(p, dpr).Render(ggbackend.Writer(&buf, ggbackend.FormatPNG))
	}
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	img, err := png.Decode(&buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return img
}

// withDPR rebuilds a fixed test plot at a device pixel ratio.
func withDPR(_ *figure.Plot, dpr float64) *figure.Plot {
	src := figure.Float64Columns(map[string][]float64{"x": {0, 1, 2}, "y": {0, 2, 1}})
	p := figure.New(figure.Size(200, 120), figure.Title("t"), figure.DPR(dpr))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))
	return p
}

func imageDiff(a, b []byte) (float64, error) {
	if bytes.Equal(a, b) {
		return 0, nil
	}
	ia, err := png.Decode(bytes.NewReader(a))
	if err != nil {
		return 0, err
	}
	ib, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return 0, err
	}
	ra, rb := ia.Bounds(), ib.Bounds()
	if ra.Size() != rb.Size() {
		return 1, nil
	}
	var bad, total int
	for y := range ra.Dy() {
		for x := range ra.Dx() {
			r1, g1, b1, a1 := ia.At(ra.Min.X+x, ra.Min.Y+y).RGBA()
			r2, g2, b2, a2 := ib.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			for _, pair := range [4][2]uint32{{r1, r2}, {g1, g2}, {b1, b2}, {a1, a2}} {
				total++
				if diff(pair[0]>>8, pair[1]>>8) > channelTolerance {
					bad++
				}
			}
		}
	}
	return float64(bad) / float64(total), nil
}

func diff(a, b uint32) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

func ramp(lo, hi float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = lo + (hi-lo)*float64(i)/float64(n-1)
	}
	return out
}

func apply(xs []float64, fn func(float64) float64) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		out[i] = fn(x)
	}
	return out
}

// A clip has to clip, whichever branch the backend takes to install it.
//
// A rectangular clip is handed to gg as a scissor rectangle rather than as a
// rasterised mask, because a mask is consulted again on every drawing call
// inside it and that made a figure cost its calls times its area. The two
// paths are different code, so both are checked here on the property that
// matters: ink outside the clip does not reach the pixels.
func TestAClipKeepsInkInsideItWhateverItsShape(t *testing.T) {
	for _, tc := range []struct {
		name string
		clip func(*ir.Path)
	}{
		{"a rectangle", func(p *ir.Path) { p.Rect(ir.R(20, 20, 60, 60)) }},
		{"a triangle", func(p *ir.Path) {
			p.MoveTo(20, 20).LineTo(60, 20).LineTo(60, 60).Close()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ggbackend.NewSurface()
			b, err := s.Open(ir.Surface{WidthPx: 100, HeightPx: 100, DPR: 1})
			if err != nil {
				t.Fatalf("Open: %v", err)
			}

			var bg ir.Path
			bg.Rect(ir.R(0, 0, 100, 100))
			b.FillPath(&bg, ir.Fill{Color: ir.RGB(255, 255, 255)}, ir.NonZero)

			var clip ir.Path
			tc.clip(&clip)
			b.Push(&clip, ir.Identity)
			var wide ir.Path
			wide.Rect(ir.R(-50, -50, 150, 150))
			b.FillPath(&wide, ir.Fill{Color: ir.RGB(0, 0, 0)}, ir.NonZero)
			b.Pop()

			if err := b.Flush(); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			img := s.Image()
			if img == nil {
				t.Fatal("nothing was drawn")
			}
			dark := func(x, y int) bool {
				r, _, _, _ := img.At(x, y).RGBA()
				return r>>8 < 128
			}
			if !dark(40, 40) {
				t.Error("the middle of the clip was not painted")
			}
			for _, p := range [][2]int{{5, 5}, {95, 5}, {5, 95}, {95, 95}, {40, 5}, {5, 40}} {
				if dark(p[0], p[1]) {
					t.Errorf("ink reached (%d,%d), which is outside the clip", p[0], p[1])
				}
			}
		})
	}
}

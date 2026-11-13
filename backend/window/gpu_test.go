package window

import (
	"image"
	"math"
	"os"
	"sync"
	"testing"

	_ "github.com/gogpu/wgpu/hal/allbackends"
	"github.com/timzifer/figure"
	stilusbackend "github.com/timzifer/figure/backend/stilus"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/stilus/gpu"
)

// The window composites on the GPU, and a runner has none — so this runs where
// there is one and skips where there is not, as stilus/gpu's own tests do.
// FIGURE_GPU_REQUIRE=1 turns the skip into a failure, for a machine that is
// supposed to have a device.
var (
	devOnce sync.Once
	dev     *gpu.Device
	devErr  error
)

func device(t *testing.T) *gpu.Device {
	t.Helper()
	devOnce.Do(func() { dev, devErr = gpu.Open(nil) })
	if devErr != nil {
		if os.Getenv("FIGURE_GPU_REQUIRE") == "1" {
			t.Fatalf("no GPU device: %v", devErr)
		}
		t.Skipf("no GPU device: %v", devErr)
	}
	return dev
}

func chart() *figure.Plot {
	xs, ys := make([]float64, 400), make([]float64, 400)
	for i := range xs {
		xs[i], ys[i] = float64(i), math.Sin(float64(i)/25)
	}
	src := figure.Float64Columns(map[string][]float64{"x": xs, "y": ys})
	p := figure.New(figure.Size(360, 240), figure.Title("On the GPU"))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))
	p.Add(geom.Scatter(src, geom.X("x"), geom.Y("y")))
	return p
}

// What the window presents is what a file shows. The chart a window's
// deferred surface composites onto a GPU texture is read back and compared
// with the same chart rasterized into memory: the GPU blends in floating
// point and rounds once, so a channel may differ by one or two, and no more
// than that over more than a sliver of the picture.
func TestTheWindowCompositesTheChartAFileShows(t *testing.T) {
	d := device(t)

	cpu := stilusbackend.NewSurface()
	live, err := chart().Live(cpu)
	if err != nil {
		t.Fatal(err)
	}
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	want := cpu.Image().(*image.RGBA)

	w := New(Size(360, 240))
	live2, err := chart().Live(w.Target())
	if err != nil {
		t.Fatal(err)
	}
	if err := live2.Draw(); err != nil {
		t.Fatal(err)
	}
	pw, ph := w.surf.PixelSize()
	tgt, err := d.NewTarget(pw, ph)
	if err != nil {
		t.Fatal(err)
	}
	defer tgt.Close()
	if err := w.surf.Composite(tgt); err != nil {
		t.Fatal(err)
	}
	got := image.NewRGBA(tgt.Bounds())
	if err := tgt.ReadRGBA(got); err != nil {
		t.Fatal(err)
	}

	if got.Rect != want.Rect {
		t.Fatalf("the texture is %v, the file %v", got.Rect, want.Rect)
	}
	var inked, off int
	for i := range want.Pix {
		if want.Pix[i] != 0 {
			inked++
		}
		if d := int(want.Pix[i]) - int(got.Pix[i]); d > 2 || d < -2 {
			off++
		}
	}
	if inked == 0 {
		t.Fatal("the file drew nothing to compare against")
	}
	frac := float64(off) / float64(len(want.Pix))
	t.Logf("%d of %d channels differ by more than 2 (%.4f%%)", off, len(want.Pix), frac*100)
	if frac > 0.002 {
		t.Errorf("%.3f%% of channels differ by more than 2 between the texture and the file", frac*100)
	}
}

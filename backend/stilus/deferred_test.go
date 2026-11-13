package stilus_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/timzifer/figure"
	stilusbackend "github.com/timzifer/figure/backend/stilus"
	"github.com/timzifer/figure/geom"
	st "github.com/timzifer/stilus"
)

// imageCompositor is a stilus compositor onto an image: the CPU stand-in for
// a GPU target, so a deferred surface can be checked without a GPU. It
// composites source-over on premultiplied colours, as the interface requires.
type imageCompositor struct {
	img    *image.RGBA
	clears int
}

func (c *imageCompositor) Clear(col color.RGBA) {
	c.clears++
	for i := 0; i < len(c.img.Pix); i += 4 {
		c.img.Pix[i], c.img.Pix[i+1], c.img.Pix[i+2], c.img.Pix[i+3] = col.R, col.G, col.B, col.A
	}
}

func (c *imageCompositor) Blitter(p *st.Paint) st.Blitter { return &imageBlitter{img: c.img, p: *p} }

type imageBlitter struct {
	img *image.RGBA
	p   st.Paint
	buf []uint32
}

func (b *imageBlitter) source(y, x, n int) []uint32 {
	if cap(b.buf) < n {
		b.buf = make([]uint32, n)
	}
	b.buf = b.buf[:n]
	if b.p.Shader != nil {
		b.p.Shader.ShadeSpan(y, x, b.buf)
		return b.buf
	}
	c := st.PackRGBA(b.p.Color)
	for i := range b.buf {
		b.buf[i] = c
	}
	return b.buf
}

func (b *imageBlitter) over(x, y int, src uint32, cov uint8) {
	s := st.UnpackRGBA(src)
	o := b.img.PixOffset(x, y)
	d := b.img.Pix[o : o+4]
	k := float64(cov) / 255
	sa := float64(s.A) * k / 255
	for i, v := range [4]uint8{s.R, s.G, s.B, s.A} {
		d[i] = uint8(math.Round(float64(v)*k + float64(d[i])*(1-sa)))
	}
}

func (b *imageBlitter) BlitRun(y, x0, x1 int, alpha uint8) {
	src := b.source(y, x0, x1-x0)
	for i := range src {
		b.over(x0+i, y, src[i], alpha)
	}
}

func (b *imageBlitter) BlitCoverage(y, x int, cov []uint8) {
	src := b.source(y, x, len(cov))
	for i, c := range cov {
		b.over(x+i, y, src[i], c)
	}
}

func deferredPlot() *figure.Plot {
	xs, ys := make([]float64, 300), make([]float64, 300)
	for i := range xs {
		xs[i], ys[i] = float64(i), math.Sin(float64(i)/20)
	}
	src := figure.Float64Columns(map[string][]float64{"x": xs, "y": ys})
	p := figure.New(figure.Size(320, 200), figure.Title("Deferred"))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))
	p.Add(geom.Scatter(src, geom.X("x"), geom.Y("y")))
	return p
}

// A deferred surface composited onto an image is the chart a surface draws
// directly — up to the rounding of a compositor that is not the canvas's own.
func TestADeferredFrameCompositesToTheChartASurfaceDraws(t *testing.T) {
	direct := stilusbackend.NewSurface()
	live, err := deferredPlot().Live(direct)
	if err != nil {
		t.Fatal(err)
	}
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	want := direct.Image().(*image.RGBA)

	s := stilusbackend.NewSurface(stilusbackend.Deferred())
	live2, err := deferredPlot().Live(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := live2.Draw(); err != nil {
		t.Fatal(err)
	}
	if s.Image() != nil {
		t.Error("a deferred surface has pixels of its own")
	}
	w, h := s.PixelSize()
	dst := &imageCompositor{img: image.NewRGBA(image.Rect(0, 0, w, h))}
	if err := s.Composite(dst); err != nil {
		t.Fatal(err)
	}
	if ink(want).Empty() {
		t.Fatal("the direct render drew nothing to compare against")
	}
	if dst.img.Rect != want.Rect {
		t.Fatalf("composited %v, drawn %v", dst.img.Rect, want.Rect)
	}
	var off int
	for i := range want.Pix {
		d := int(want.Pix[i]) - int(dst.img.Pix[i])
		if d > 2 || d < -2 {
			off++
		}
	}
	if frac := float64(off) / float64(len(want.Pix)); frac > 0.001 {
		t.Errorf("%.3f%% of channels differ by more than 2", frac*100)
	}
}

// The recording is the latest frame, not every frame: a second frame replaces
// the first, and an unchanged one keeps it and its generation.
func TestADeferredSurfaceKeepsTheLatestFrameOnly(t *testing.T) {
	s := stilusbackend.NewSurface(stilusbackend.Deferred())
	live, err := deferredPlot().Live(s)
	if err != nil {
		t.Fatal(err)
	}
	if s.Generation() != 0 {
		t.Error("an undrawn surface has a generation")
	}
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	first := s.Generation()
	if first == 0 {
		t.Fatal("a drawn frame did not advance the generation")
	}
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	if s.Generation() != first {
		t.Error("an unchanged frame advanced the generation")
	}
	if err := live.PanBy(40, 0); err != nil {
		t.Fatal(err)
	}
	if s.Generation() == first {
		t.Error("a pan did not advance the generation")
	}

	// Compositing twice draws the same picture: the second frame replaced the
	// first rather than being drawn on top of it, and Composite clears.
	w, h := s.PixelSize()
	a := &imageCompositor{img: image.NewRGBA(image.Rect(0, 0, w, h))}
	b := &imageCompositor{img: image.NewRGBA(image.Rect(0, 0, w, h))}
	b.Clear(color.RGBA{255, 0, 0, 255})
	if err := s.Composite(a); err != nil {
		t.Fatal(err)
	}
	if err := s.Composite(b); err != nil {
		t.Fatal(err)
	}
	if string(a.img.Pix) != string(b.img.Pix) {
		t.Error("compositing depends on what the destination held")
	}
}

func TestADeferredSurfaceFollowsAResizeAndAScale(t *testing.T) {
	s := stilusbackend.NewSurface(stilusbackend.Deferred())
	live, err := deferredPlot().Live(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	if err := live.Resize(400, 300); err != nil {
		t.Fatal(err)
	}
	if err := live.Rescale(2); err != nil {
		t.Fatal(err)
	}
	if w, h := s.PixelSize(); w != 800 || h != 600 {
		t.Errorf("the surface is %dx%d device pixels, want 800x600", w, h)
	}
	dst := &imageCompositor{img: image.NewRGBA(image.Rect(0, 0, 800, 600))}
	if err := s.Composite(dst); err != nil {
		t.Fatal(err)
	}
	if r := ink(dst.img); r.Max.X < 700 || r.Max.Y < 500 {
		t.Errorf("a resized, rescaled frame inked only %v", r)
	}
}

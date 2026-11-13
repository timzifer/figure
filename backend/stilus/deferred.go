package stilus

import (
	"errors"
	"fmt"
	"image"
	"image/color"

	"github.com/timzifer/figure/ir"
	st "github.com/timzifer/stilus"
)

// Compositor is what a deferred [Surface] draws onto: a stilus compositor —
// every span of a frame goes through its blitters — that can also be cleared.
// A stilus/gpu Target is one, which is what a window composites onto.
type Compositor interface {
	st.Compositor
	// Clear fills the destination with c, premultiplied, before a frame.
	Clear(c color.RGBA)
}

// Deferred makes a [Surface] record its frames rather than rasterise them.
//
// A window draws in two places: in its event handlers — a resize, a pan, a
// zoom — and in its draw callback, which is the only place it has a GPU frame
// to put pixels in. A deferred surface lets the chart draw whenever it likes
// and keeps the latest frame as IR; [Surface.Composite] rasterises that frame
// onto a [Compositor] when the window is ready for it. Rasterising happens
// once per frame that is shown, not once per frame that is drawn, and never
// into a buffer that is then uploaded.
//
// A deferred surface has no pixels of its own: [Surface.Image] is nil. Its
// [Surface.Generation] still says whether the latest frame differs from the
// one before it, which is how a caller knows to composite again.
func Deferred() Option { return func(o *options) { o.deferred = true } }

// deferred is a deferred surface's backend: it records drawing calls, measures
// through a stilus backend with no destination, and rasterises the recording
// through that backend when asked.
type deferred struct {
	b   *backend
	rec *ir.Recorder

	w, h  int
	dpr   float64
	gen   uint64
	drew  bool // a drawing call arrived since the last Flush
	ended bool // Flush ended a frame: the next drawing call starts a new one
}

func (o options) openDeferred(s ir.Surface, glyphs *st.GlyphCache) (*deferred, error) {
	fonts, err := o.resolve()
	if err != nil {
		return nil, err
	}
	if s.WidthPx <= 0 || s.HeightPx <= 0 {
		return nil, errors.New("figure/backend/stilus: chart size must be positive")
	}
	dpr := s.DPR
	if dpr <= 0 {
		dpr = 1
	}
	d := &deferred{
		b: newBackend(nil, s.WidthPx, s.HeightPx, dpr, fonts, glyphs),
		w: s.WidthPx, h: s.HeightPx, dpr: dpr,
	}
	d.rec = ir.NewRecorder(d.b)
	return d, nil
}

// begin is called by every drawing call. A drawing call after a Flush is the
// first of a new frame, so the recording of the last one goes.
func (d *deferred) begin() {
	if d.ended {
		d.rec.Reset()
		d.ended = false
	}
	d.drew = true
}

func (d *deferred) Polyline(pts []ir.Point, style ir.Stroke) {
	d.begin()
	d.rec.Polyline(pts, style)
}

func (d *deferred) StrokePath(p *ir.Path, style ir.Stroke) {
	d.begin()
	d.rec.StrokePath(p, style)
}

func (d *deferred) FillPath(p *ir.Path, fill ir.Fill, rule ir.FillRule) {
	d.begin()
	d.rec.FillPath(p, fill, rule)
}

func (d *deferred) Text(run ir.TextRun) {
	d.begin()
	d.rec.Text(run)
}

func (d *deferred) Markers(shape ir.Marker, at []ir.Point, style ir.MarkerStyle) {
	d.begin()
	d.rec.Markers(shape, at, style)
}

func (d *deferred) Image(img image.Image, dst ir.Rect) {
	d.begin()
	d.rec.Image(img, dst)
}

func (d *deferred) Push(clip *ir.Path, xform ir.Affine) {
	d.begin()
	d.rec.Push(clip, xform)
}

func (d *deferred) Pop() {
	d.begin()
	d.rec.Pop()
}

// Measure answers from the fonts the recording will be drawn with.
func (d *deferred) Measure(run ir.TextRun) ir.TextMetrics { return d.b.Measure(run) }

// Flush ends a frame, and stamps a new generation if it drew anything.
func (d *deferred) Flush() error {
	if d.drew {
		d.gen = generations.Add(1)
		d.drew = false
	}
	d.ended = true
	return nil
}

// Resize implements [ir.Resizer]. There is no buffer to replace: the size is
// what the next [Surface.Composite] rasterises at, and what
// [Surface.PixelSize] reports so the caller can make a destination to match.
func (d *deferred) Resize(s ir.Surface) error {
	if s.WidthPx <= 0 || s.HeightPx <= 0 {
		return fmt.Errorf("figure/backend/stilus: size %dx%d is not positive", s.WidthPx, s.HeightPx)
	}
	if s.DPR > 0 {
		d.dpr = s.DPR
	}
	d.w, d.h = s.WidthPx, s.HeightPx
	return nil
}

func (d *deferred) pixelSize() (int, int) {
	return int(float64(d.w)*d.dpr + 0.5), int(float64(d.h)*d.dpr + 0.5)
}

// composite clears dst and rasterises the latest frame onto it.
func (d *deferred) composite(dst Compositor) error {
	b := d.b
	w, h := d.pixelSize()
	dst.Clear(color.RGBA{})
	b.dpr, b.w, b.h = d.dpr, d.w, d.h
	b.dirty = false
	b.c.ResetCompositor(dst, image.Rect(0, 0, w, h))
	b.stack = append(b.stack[:0], level{m: st.Scale(d.dpr, d.dpr)})
	d.rec.Replay(b)
	return b.c.Err()
}

package stilus

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sync/atomic"

	"github.com/timzifer/figure/ir"
	st "github.com/timzifer/stilus"
)

// level is one entry of the Push stack: the transform in force and whether
// the push put a clip on the canvas.
type level struct {
	m    st.Matrix
	clip bool
}

// generations stamps a buffer each time a frame changes it. It is one counter
// for the process rather than one per surface, so a buffer replaced by Open or
// Resize can never be mistaken for the one it replaced.
var generations atomic.Uint64

// backend adapts figure's IR onto a stilus canvas.
type backend struct {
	img    *image.RGBA
	c      *st.Canvas
	fonts  *fontSet
	glyphs *st.GlyphCache
	stack  []level

	w, h  int     // logical size
	dpr   float64 // device pixels per logical unit
	gen   uint64  // the buffer's generation, for Surface.Generation
	drew  bool    // pixels were touched since the last Flush
	dirty bool    // a damage clip is on the canvas until Flush

	// Buffers reused across calls, so that a steady frame allocates nothing
	// here that grows with the chart.
	path   st.Path
	marker ir.Path
	dash   []float64
	grad   st.LinearGradient
	ramp   rampKey
	pix    []uint32
	shader st.ImageShader
}

func newBackend(img *image.RGBA, w, h int, dpr float64, fonts *fontSet, glyphs *st.GlyphCache) *backend {
	if glyphs == nil {
		glyphs = new(st.GlyphCache)
	}
	b := &backend{img: img, c: st.NewCanvas(img), fonts: fonts, glyphs: glyphs, w: w, h: h, dpr: dpr}
	b.stack = append(b.stack, level{m: st.Scale(dpr, dpr)})
	return b
}

func (b *backend) top() st.Matrix { return b.stack[len(b.stack)-1].m }

// matrixOf is the identity conversion: figure's affine and stilus's matrix
// are both x' = a·x + c·y + e, y' = b·x + d·y + f.
func matrixOf(a ir.Affine) st.Matrix {
	return st.Matrix{float64(a.A), float64(a.B), float64(a.C), float64(a.D), float64(a.E), float64(a.F)}
}

// pathOf replays an IR path into the backend's reusable stilus path.
func (b *backend) pathOf(p *ir.Path) *st.Path {
	sp := &b.path
	sp.Reset()
	p.Walk(func(op ir.PathOp, pts []ir.Point) {
		switch op {
		case ir.OpMoveTo:
			sp.MoveTo(pts[0].X, pts[0].Y)
		case ir.OpLineTo:
			sp.LineTo(pts[0].X, pts[0].Y)
		case ir.OpCubicTo:
			sp.CubicTo(pts[0].X, pts[0].Y, pts[1].X, pts[1].Y, pts[2].X, pts[2].Y)
		case ir.OpClose:
			sp.Close()
		}
	})
	return sp
}

// premul converts figure's straight-alpha colour into stilus's premultiplied
// one.
func premul(c ir.Color) color.RGBA {
	a := uint32(c.A)
	m := func(v uint8) uint8 { return uint8((uint32(v)*a + 127) / 255) }
	return color.RGBA{R: m(c.R), G: m(c.G), B: m(c.B), A: c.A}
}

func (b *backend) strokeStyle(s ir.Stroke) *st.StrokeStyle {
	ss := &st.StrokeStyle{
		Width:      float64(s.Width),
		Cap:        st.Cap(s.Cap),   // butt, round, square in both
		Join:       st.Join(s.Join), // miter, round, bevel in both
		MiterLimit: 4,
	}
	if s.MiterLimit > 0 {
		ss.MiterLimit = float64(s.MiterLimit)
	}
	if len(s.Dash) > 0 {
		b.dash = b.dash[:0]
		for _, d := range s.Dash {
			b.dash = append(b.dash, float64(d))
		}
		ss.Dash = b.dash
		ss.DashPhase = float64(s.DashOffset)
	}
	return ss
}

// --- ir.Backend ----------------------------------------------------------

func (b *backend) Polyline(pts []ir.Point, style ir.Stroke) {
	if !style.Visible() || len(pts) < 2 {
		return
	}
	sp := &b.path
	sp.Reset()
	sp.MoveTo(pts[0].X, pts[0].Y)
	for _, p := range pts[1:] {
		sp.LineTo(p.X, p.Y)
	}
	b.c.Stroke(sp, b.top(), b.strokeStyle(style), &st.Paint{Color: premul(style.Color)})
	b.drew = true
}

func (b *backend) StrokePath(p *ir.Path, style ir.Stroke) {
	if !style.Visible() || p == nil || p.Empty() {
		return
	}
	b.c.Stroke(b.pathOf(p), b.top(), b.strokeStyle(style), &st.Paint{Color: premul(style.Color)})
	b.drew = true
}

func ruleOf(r ir.FillRule) st.FillRule {
	if r == ir.EvenOdd {
		return st.EvenOdd
	}
	return st.NonZero
}

func (b *backend) FillPath(p *ir.Path, fill ir.Fill, rule ir.FillRule) {
	if p == nil || p.Empty() || !fill.Visible() {
		return
	}
	paint := st.Paint{Color: premul(fill.Color)}
	if fill.IsGradient() {
		if !b.gradient(fill) {
			return
		}
		paint = st.Paint{Shader: &b.grad}
	}
	b.c.Fill(b.pathOf(p), b.top(), ruleOf(rule), &paint)
	b.drew = true
}

func (b *backend) Markers(shape ir.Marker, at []ir.Point, style ir.MarkerStyle) {
	if len(at) == 0 || style.Size <= 0 {
		return
	}
	hasFill := style.Fill.A != 0
	hasStroke := style.Stroke.Visible()
	if !hasFill && !hasStroke {
		return
	}
	// One path, placed per instance by its matrix: stilus takes the
	// transform with every call, so nothing is copied or moved.
	b.marker.Reset()
	ir.MarkerPath(&b.marker, shape, style.Size)
	sp := b.pathOf(&b.marker)
	fill := st.Paint{Color: premul(style.Fill)}
	stroke := st.Paint{Color: premul(style.Stroke.Color)}
	var ss *st.StrokeStyle
	if hasStroke {
		ss = b.strokeStyle(style.Stroke)
	}
	top := b.top()
	for _, p := range at {
		m := st.Translate(float64(p.X), float64(p.Y)).Mul(top)
		if hasFill {
			b.c.Fill(sp, m, st.NonZero, &fill)
		}
		if hasStroke {
			b.c.Stroke(sp, m, ss, &stroke)
		}
	}
	b.drew = true
}

// Image draws img stretched over dst, filtered linearly.
//
// The picture is converted into stilus's premultiplied layout on every call,
// into a buffer the backend keeps. It is not cached by identity: a density
// raster redraws into the same image each frame, and a cache keyed by the
// pointer would show the first frame for ever.
func (b *backend) Image(img image.Image, dst ir.Rect) {
	if img == nil || dst.Empty() {
		return
	}
	r := img.Bounds()
	if r.Empty() {
		return
	}
	w, h := r.Dx(), r.Dy()
	if cap(b.pix) < w*h {
		b.pix = make([]uint32, w*h)
	}
	b.pix = b.pix[:w*h]
	convert(b.pix, img)
	// The texture is made per call because its base plane must not change
	// once it exists; its mip levels are made on demand and only when the
	// picture is drawn much smaller than it is.
	tex := st.NewTexture(st.Plane{Kind: st.PlaneRGBA, W: w, H: h, Stride: w, Pix32: b.pix})
	toDevice := st.Scale(float64(dst.Dx())/float64(w), float64(dst.Dy())/float64(h)).
		Mul(st.Translate(float64(dst.Min.X), float64(dst.Min.Y))).
		Mul(b.top())
	b.shader.Reset()
	if !b.shader.SetImage(tex, toDevice, true, 255) {
		return
	}
	sp := &b.path
	sp.Reset()
	sp.Rect(dst.Min.X, dst.Min.Y, dst.Dx(), dst.Dy())
	b.c.Fill(sp, b.top(), st.NonZero, &st.Paint{Shader: &b.shader})
	b.drew = true
}

// convert writes img into dst in stilus's layout, premultiplied, row by row
// from the picture's top-left corner. The two layouts figure's own rasters
// come in take a direct path; anything else goes through At.
func convert(dst []uint32, img image.Image) {
	r := img.Bounds()
	w := r.Dx()
	switch src := img.(type) {
	case *image.RGBA:
		for y := range r.Dy() {
			row := src.Pix[src.PixOffset(r.Min.X, r.Min.Y+y):]
			for x := range w {
				o := 4 * x
				dst[y*w+x] = st.PackRGBA(color.RGBA{row[o], row[o+1], row[o+2], row[o+3]})
			}
		}
	case *image.NRGBA:
		for y := range r.Dy() {
			row := src.Pix[src.PixOffset(r.Min.X, r.Min.Y+y):]
			for x := range w {
				o := 4 * x
				dst[y*w+x] = st.PackRGBA(premul(ir.Color{R: row[o], G: row[o+1], B: row[o+2], A: row[o+3]}))
			}
		}
	default:
		for y := range r.Dy() {
			for x := range w {
				cr, cg, cb, ca := img.At(r.Min.X+x, r.Min.Y+y).RGBA()
				dst[y*w+x] = st.PackRGBA(color.RGBA{uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), uint8(ca >> 8)})
			}
		}
	}
}

func (b *backend) Push(clip *ir.Path, xform ir.Affine) {
	m := b.top()
	if !xform.IsIdentity() {
		m = matrixOf(xform).Mul(m)
	}
	lv := level{m: m}
	if clip != nil && !clip.Empty() {
		lv.clip = true
		// A rectangle costs nothing per pixel; any other clip is
		// rasterised into a mask over its bounds. Every clip figure pushes
		// is a rectangle, so this is the path that matters.
		if r, ok := clip.AsRect(); ok {
			b.c.ClipRect(st.Rect{X0: float64(r.Min.X), Y0: float64(r.Min.Y), X1: float64(r.Max.X), Y1: float64(r.Max.Y)}, m)
		} else {
			b.c.ClipPath(b.pathOf(clip), m, st.NonZero)
		}
	}
	b.stack = append(b.stack, lv)
}

func (b *backend) Pop() {
	if len(b.stack) <= 1 {
		return
	}
	if b.stack[len(b.stack)-1].clip {
		b.c.PopClip()
	}
	b.stack = b.stack[:len(b.stack)-1]
}

// Damage implements [ir.Partial]: it limits the next frame to the rectangles
// that changed.
//
// The frame is clipped to them and they are cleared first, because a repaint
// composites: drawing an antialiased stroke a second time over the first one
// darkens its edges, so the pixels have to go before the frame is replayed.
// Both take the bounding box of the rectangles rather than each of them, as
// backend/gg does: a pixel inside the box and outside every rectangle is
// repainted for nothing, and a pixel outside the box is not touched.
//
// A nil list is the whole frame. An empty one — which figure never sends,
// because it skips an unchanged frame instead — clips the frame to nothing.
func (b *backend) Damage(rects []ir.Rect) {
	b.undamage()
	b.drew = true
	if rects == nil {
		clear(b.img.Pix)
		return
	}
	var box ir.Rect
	if len(rects) > 0 {
		box = rects[0]
		for _, r := range rects[1:] {
			box.Min.X = min(box.Min.X, r.Min.X)
			box.Min.Y = min(box.Min.Y, r.Min.Y)
			box.Max.X = max(box.Max.X, r.Max.X)
			box.Max.Y = max(box.Max.Y, r.Max.Y)
		}
	}
	px := b.pixels(box)
	b.clearRect(px)
	// The clip is the cleared pixels exactly rather than the logical box,
	// so an edge pixel the box only partly covers is repainted whole
	// instead of composited over what is left of it.
	b.c.ClipRect(st.Rect{X0: float64(px.Min.X), Y0: float64(px.Min.Y), X1: float64(px.Max.X), Y1: float64(px.Max.Y)}, st.Identity)
	b.dirty = true
}

// pixels is the device-pixel rectangle covering r, a rectangle in logical
// units, within the buffer.
func (b *backend) pixels(r ir.Rect) image.Rectangle {
	s := b.dpr
	return image.Rect(
		int(math.Floor(float64(r.Min.X)*s)),
		int(math.Floor(float64(r.Min.Y)*s)),
		int(math.Ceil(float64(r.Max.X)*s)),
		int(math.Ceil(float64(r.Max.Y)*s)),
	).Intersect(b.img.Bounds())
}

// clearRect makes a rectangle of the buffer transparent.
func (b *backend) clearRect(r image.Rectangle) {
	if r.Empty() {
		return
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		o := b.img.PixOffset(r.Min.X, y)
		clear(b.img.Pix[o : o+4*r.Dx()])
	}
}

// undamage releases a damage clip left over from the last frame.
func (b *backend) undamage() {
	if b.dirty {
		b.c.PopClip()
		b.dirty = false
	}
}

// Resize implements [ir.Resizer]: it replaces the pixel buffer the chart is
// drawn into, which is what a window being dragged wider needs. The glyph
// cache survives it; the pixels do not, and the next frame paints all of them.
func (b *backend) Resize(s ir.Surface) error {
	if s.WidthPx <= 0 || s.HeightPx <= 0 {
		return fmt.Errorf("figure/backend/stilus: size %dx%d is not positive", s.WidthPx, s.HeightPx)
	}
	if s.DPR <= 0 {
		s.DPR = b.dpr
	}
	img, dpr, err := buffer(s, nil)
	if err != nil {
		return err
	}
	b.dirty = false
	b.img, b.w, b.h, b.dpr = img, s.WidthPx, s.HeightPx, dpr
	b.c.Reset(img, img.Bounds())
	b.stack = append(b.stack[:0], level{m: st.Scale(dpr, dpr)})
	return nil
}

// Flush completes the frame. A frame that put something into the buffer
// stamps it with a new generation, so a window watching it re-uploads exactly
// the frames that changed.
func (b *backend) Flush() error {
	b.undamage()
	if b.drew {
		b.gen = generations.Add(1)
		b.drew = false
	}
	return b.c.Err()
}

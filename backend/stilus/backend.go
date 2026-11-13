package stilus

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/timzifer/figure/ir"
	st "github.com/timzifer/stilus"
)

// level is one entry of the Push stack: the transform in force and whether
// the push put a clip on the canvas.
type level struct {
	m    st.Matrix
	clip bool
}

// backend adapts figure's IR onto a stilus canvas.
type backend struct {
	img    *image.RGBA
	c      *st.Canvas
	fonts  *fontSet
	glyphs *st.GlyphCache
	stack  []level

	// Buffers reused across calls, so that a steady frame allocates nothing
	// here that grows with the chart.
	path   st.Path
	marker ir.Path
	dash   []float64
	grad   st.LinearGradient
	img2   st.ImageShader
}

func newBackend(img *image.RGBA, dpr float64, fonts *fontSet) *backend {
	b := &backend{img: img, c: st.NewCanvas(img), fonts: fonts, glyphs: new(st.GlyphCache)}
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
}

func (b *backend) StrokePath(p *ir.Path, style ir.Stroke) {
	if !style.Visible() || p == nil || p.Empty() {
		return
	}
	b.c.Stroke(b.pathOf(p), b.top(), b.strokeStyle(style), &st.Paint{Color: premul(style.Color)})
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
}

// gradient sets the backend's linear gradient to fill, and reports whether it
// can be drawn.
func (b *backend) gradient(fill ir.Fill) bool {
	g := &b.grad
	g.Ramp = g.Ramp[:0]
	g.Knots = g.Knots[:0]
	for _, s := range fill.Stops {
		g.Ramp = append(g.Ramp, st.PackRGBA(premul(s.Color)))
		g.Knots = append(g.Knots, min(max(s.Offset, 0), 1))
	}
	g.Extend = [2]bool{true, true}
	g.Alpha = 255
	return g.Set(float64(fill.Start.X), float64(fill.Start.Y), float64(fill.End.X), float64(fill.End.Y), b.top())
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
}

func (b *backend) Image(img image.Image, dst ir.Rect) {
	if img == nil || dst.Empty() {
		return
	}
	r := img.Bounds()
	if r.Empty() {
		return
	}
	// SPIKE: converted on every call. A texture cache keyed by the picture
	// is the obvious next step; the density raster redraws one per frame.
	rgba := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, r.Min, draw.Src)
	pl := st.Plane{Kind: st.PlaneRGBA, W: r.Dx(), H: r.Dy(), Stride: r.Dx(), Pix32: make([]uint32, r.Dx()*r.Dy())}
	for i := range pl.Pix32 {
		o := 4 * i
		pl.Pix32[i] = st.PackRGBA(color.RGBA{rgba.Pix[o], rgba.Pix[o+1], rgba.Pix[o+2], rgba.Pix[o+3]})
	}
	tex := st.NewTexture(pl)
	toDevice := st.Scale(float64(dst.Dx())/float64(r.Dx()), float64(dst.Dy())/float64(r.Dy())).
		Mul(st.Translate(float64(dst.Min.X), float64(dst.Min.Y))).
		Mul(b.top())
	b.img2.Reset()
	if !b.img2.SetImage(tex, toDevice, true, 255) {
		return
	}
	sp := &b.path
	sp.Reset()
	sp.Rect(dst.Min.X, dst.Min.Y, dst.Dx(), dst.Dy())
	b.c.Fill(sp, b.top(), st.NonZero, &st.Paint{Shader: &b.img2})
}

func (b *backend) Push(clip *ir.Path, xform ir.Affine) {
	m := b.top()
	if !xform.IsIdentity() {
		m = matrixOf(xform).Mul(m)
	}
	lv := level{m: m}
	if clip != nil && !clip.Empty() {
		lv.clip = true
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

func (b *backend) Flush() error { return b.c.Err() }

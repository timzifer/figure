package stilus

import (
	"github.com/timzifer/figure/ir"
	st "github.com/timzifer/stilus"
)

const defaultSize = 12 // gg's default when a run names no size

func sizeOf(run ir.TextRun) float64 {
	if run.Font.Size <= 0 {
		return defaultSize
	}
	return run.Font.Size
}

// advance is the width of run in device units: the sum of its glyphs'
// advances, each in the face it is drawn with, unrounded.
func (b *backend) advance(run ir.TextRun, primary *face) float64 {
	size := sizeOf(run)
	var w float64
	for _, r := range run.Text {
		f, g := b.fonts.resolve(primary, r)
		w += f.advance(g) * size / f.upem
	}
	return w
}

func (b *backend) Measure(run ir.TextRun) ir.TextMetrics {
	primary := b.fonts.pick(run.Font.Weight, run.Font.Italic)
	s := sizeOf(run) / primary.upem
	adv := b.advance(run, primary)
	asc, desc := primary.ascent()*s, primary.descent()*s
	return ir.TextMetrics{
		Advance: float32(adv),
		Ascent:  float32(asc),
		Descent: float32(desc),
		Ink:     ir.R(0, float32(-asc), float32(adv), float32(desc)),
	}
}

func (b *backend) Text(run ir.TextRun) {
	if run.Text == "" || run.Color.A == 0 {
		return
	}
	primary := b.fonts.pick(run.Font.Weight, run.Font.Italic)
	size := sizeOf(run)
	s := size / primary.upem

	// Anchored on the font box rather than the ink, as in backend/gg, so a
	// column of numbers shares a baseline.
	x, y := float64(run.At.X), float64(run.At.Y)
	switch run.H {
	case ir.AlignCenter:
		x -= b.advance(run, primary) / 2
	case ir.AlignEnd:
		x -= b.advance(run, primary)
	}
	asc, desc := primary.ascent()*s, primary.descent()*s
	switch run.V {
	case ir.AlignTop:
		y += asc
	case ir.AlignMiddle:
		y += (asc - desc) / 2
	case ir.AlignBottom:
		y -= desc
	}

	base := b.top()
	if run.Rotation != 0 {
		ax, ay := float64(run.At.X), float64(run.At.Y)
		base = st.Translate(-ax, -ay).Mul(st.Rotate(run.Rotation)).Mul(st.Translate(ax, ay)).Mul(base)
	}
	b.drew = true
	paint := st.Paint{Color: premul(run.Color)}
	for _, r := range run.Text {
		f, g := b.fonts.resolve(primary, r)
		// The outline is in ems, Y up; the device is Y down.
		m := st.Scale(size, -size).Mul(st.Translate(x, y)).Mul(base)
		b.glyphs.FillGlyph(b.c, f.id, int32(g), f.outline(g), m, &paint)
		x += f.advance(g) * size / f.upem
	}
}

package three

import (
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/theme"
)

// base is what every layer in this package holds: its data and the options it
// was configured with.
//
// The options are geom's, read back through [geom.Configure] — which is how a
// mark defined outside geom reads them, so these layers are written the way a
// third-party mark is written. That is deliberate twice over: the option set
// stays one namespace rather than gaining a second spelling of X, Color and
// GroupBy, and the extension model gets exercised by the library itself.
type base struct {
	src  data.Source
	cfg  geom.Desc
	mark geom.Mark
}

func newBase(src data.Source, mark geom.Mark, opts []geom.Option) base {
	return base{src: src, cfg: geom.Configure(opts...), mark: mark}
}

// colorFor is the layer's colour: what it was told, or its place in the
// palette.
func (b base) colorFor(f Frame) ir.Color {
	if b.cfg.Color != nil {
		return *b.cfg.Color
	}
	if b.cfg.Fill != nil {
		return *b.cfg.Fill
	}
	pal := f.Theme.Palette
	if len(pal) == 0 {
		pal = theme.Light.Palette
	}
	return pal.At(f.Index)
}

// fillFor is the interior colour, which is the fill where one was named and
// the layer's colour otherwise.
func (b base) fillFor(f Frame) ir.Color {
	if b.cfg.Fill != nil {
		return *b.cfg.Fill
	}
	return b.colorFor(f)
}

// outline is the colour a face is outlined in, and it is transparent unless
// the caller named both a fill and a colour.
//
// The rule is geom.Rect's, and so is the reason. A hit index ranks a stroked
// vertex above the area it outlines, so a mesh that always drew its own
// outline would report a corner on every hover over a surface — the reader
// points at a cell and is told about its nearest corner. Outlining stays
// something a chart asks for.
func (b base) outline() (ir.Color, float32) {
	if b.cfg.Fill == nil || b.cfg.Color == nil {
		return ir.Transparent, 0
	}
	w := b.cfg.Width
	if w <= 0 {
		w = 1
	}
	return *b.cfg.Color, w
}

// widthFor is the stroke width a line is drawn at.
func (b base) widthFor(f Frame) float32 {
	if b.cfg.Width > 0 {
		return b.cfg.Width
	}
	if f.Theme.LineWidth > 0 {
		return f.Theme.LineWidth
	}
	return 1
}

// legend is the entry a layer contributes, which is its label and its colour.
func (b base) legend(f Frame, kind geom.SwatchKind) (geom.LegendEntry, bool) {
	if b.cfg.Label == "" {
		return geom.LegendEntry{}, false
	}
	return geom.LegendEntry{
		Label: b.cfg.Label,
		Color: b.colorFor(f),
		Kind:  kind,
		Width: b.widthFor(f),
	}, true
}

// rowsWanted reports whether anyone is listening for row identity, so that a
// layer does the bookkeeping only when it is asked for.
func rowsWanted(f Frame) bool { return f.Rows != nil }

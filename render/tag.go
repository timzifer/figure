package render

import (
	"math"
	"slices"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// TagScales is the four axes a panel's tags can be written on. A nil scale is
// an axis the panel does not have, and a tag on it is not drawn.
type TagScales struct{ X, Y, X2, Y2 scale.Scale }

func (s TagScales) of(a geom.TagAxis) scale.Scale {
	switch a {
	case geom.TagX:
		return s.X
	case geom.TagX2:
		return s.X2
	case geom.TagY2:
		return s.Y2
	}
	return s.Y
}

func vertical(a geom.TagAxis) bool { return a == geom.TagY || a == geom.TagY2 }

// placedTag is a tag resolved to the panel: where on its axis it stands, the
// text it holds and the box it fills.
type placedTag struct {
	tag  geom.AxisTag
	at   float32 // device position along the axis
	text string
	box  ir.Rect
}

// placeTags resolves tags against a panel's area and scales: each is given its
// text and its position, those the axis does not show are dropped, and those
// on one axis that would overlap are pushed apart, in value order, the way a
// stack of labels is. The boxes reach from the axis line into the gutter.
func placeTags(m interface {
	Measure(ir.TextRun) ir.TextMetrics
}, th theme.Theme, area ir.Rect, sc TagScales, tags []geom.AxisTag) []placedTag {
	if len(tags) == 0 {
		return nil
	}
	font := th.Font(th.TickSize)
	pad := th.TickLabelPad
	out := make([]placedTag, 0, len(tags))
	for _, t := range tags {
		s := sc.of(t.Axis)
		if s == nil || math.IsNaN(t.Value) || math.IsInf(t.Value, 0) {
			continue
		}
		lo, hi := s.Domain()
		if t.Value < math.Min(lo, hi) || t.Value > math.Max(lo, hi) {
			continue
		}
		text := t.Text
		if text == "" {
			text = scale.ValueLabelOf(s, t.Value)
		}
		if text == "" {
			continue
		}
		out = append(out, placedTag{tag: t, at: s.Map(t.Value), text: text})
	}

	// One box height apart at least, per axis, keeping each tag as near its
	// value as the ones beside it allow.
	h := float32(font.Size)*1.3 + pad
	for _, a := range [4]geom.TagAxis{geom.TagY, geom.TagX, geom.TagY2, geom.TagX2} {
		var on []int
		for i := range out {
			if out[i].tag.Axis == a {
				on = append(on, i)
			}
		}
		slices.SortFunc(on, func(i, j int) int {
			switch {
			case out[i].at < out[j].at:
				return -1
			case out[i].at > out[j].at:
				return 1
			}
			return 0
		})
		gap := h
		if !vertical(a) {
			gap = 0 // measured per tag below: a horizontal tag is as wide as its text
		}
		for k := 1; k < len(on); k++ {
			prev, cur := &out[on[k-1]], &out[on[k]]
			need := gap
			if !vertical(a) {
				need = (textWidth(m, prev.text, font)+textWidth(m, cur.text, font))/2 + 2*pad
			}
			if cur.at-prev.at < need {
				cur.at = prev.at + need
			}
		}
	}

	for i := range out {
		p := &out[i]
		w := textWidth(m, p.text, font) + 2*pad
		switch p.tag.Axis {
		case geom.TagY:
			p.box = ir.R(area.Min.X-w, p.at-h/2, area.Min.X, p.at+h/2)
		case geom.TagY2:
			p.box = ir.R(area.Max.X, p.at-h/2, area.Max.X+w, p.at+h/2)
		case geom.TagX:
			p.box = ir.R(p.at-w/2, area.Max.Y, p.at+w/2, area.Max.Y+h)
		case geom.TagX2:
			p.box = ir.R(p.at-w/2, area.Min.Y-h, p.at+w/2, area.Min.Y)
		}
	}
	return out
}

func textWidth(m interface {
	Measure(ir.TextRun) ir.TextMetrics
}, text string, font ir.FontRef) float32 {
	return m.Measure(ir.TextRun{Text: text, Font: font}).Advance
}

// drawTags paints placed tags: the box in the tag's colour, the text centred
// in it in whichever of the theme's ink and the background reads against the
// box.
func drawTags(b ir.Backend, th theme.Theme, tags []placedTag) {
	font := th.Font(th.TickSize)
	for _, t := range tags {
		col := t.tag.Color
		if col.A == 0 {
			col = th.AnnotationColor
		}
		var p ir.Path
		p.Rect(t.box)
		b.FillPath(&p, ir.Solid(col), ir.NonZero)
		b.Text(ir.TextRun{
			Text:  t.text,
			Font:  font,
			At:    ir.Point{X: (t.box.Min.X + t.box.Max.X) / 2, Y: (t.box.Min.Y + t.box.Max.Y) / 2},
			H:     ir.AlignCenter,
			V:     ir.AlignMiddle,
			Color: inkOn(col),
		})
	}
}

// inkOn is black or white, whichever reads against col: the WCAG relative
// luminance against the midpoint of the two.
func inkOn(col ir.Color) ir.Color {
	lin := func(c uint8) float64 {
		v := float64(c) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	l := 0.2126*lin(col.R) + 0.7152*lin(col.G) + 0.0722*lin(col.B)
	if l > 0.179 {
		return ir.RGB(0x11, 0x11, 0x11)
	}
	return ir.RGB(0xFF, 0xFF, 0xFF)
}

// hideCovered blanks the labels of the ticks a tag covers on its axis, so that
// the tag — the more specific statement — is not overprinted by a tick label.
// It copies the tick list only when something is hidden.
func hideCovered(ticks []scale.Tick, tags []placedTag, axis geom.TagAxis, half float32) []scale.Tick {
	var out []scale.Tick
	for i, tk := range ticks {
		if tk.Label == "" {
			continue
		}
		for _, t := range tags {
			if t.tag.Axis != axis {
				continue
			}
			lo, hi := t.box.Min.Y, t.box.Max.Y
			if !vertical(axis) {
				lo, hi = t.box.Min.X, t.box.Max.X
			}
			if tk.Pos >= lo-half && tk.Pos <= hi+half {
				if out == nil {
					out = slices.Clone(ticks)
				}
				out[i].Label = ""
				break
			}
		}
	}
	if out == nil {
		return ticks
	}
	return out
}

// layerTags asks a panel's visible layers for their tags, against the frame
// each is drawn in.
func layerTags(p Panel, area ir.Rect, cd coord.Coord, th theme.Theme, hidden []bool) []geom.AxisTag {
	var out []geom.AxisTag
	for i, g := range p.Layers {
		t, ok := g.(geom.Tagger)
		if !ok || isHidden(hidden, i) {
			continue
		}
		x, y := p.axesOf(g)
		out = append(out, t.Tags(geom.Frame{Area: area, X: x, Y: y, Coord: cd, Theme: th, Index: i})...)
	}
	return out
}

// DrawAxisTags draws tags on a panel's axes, for an overlay: the same boxes a
// layer's tags get, placed and pushed apart the same way. It covers what is
// under it rather than hiding tick labels, because an overlay draws after the
// axes have been painted.
func DrawAxisTags(b ir.Backend, th theme.Theme, area ir.Rect, sc TagScales, tags []geom.AxisTag) {
	drawTags(b, th, placeTags(b, th, area, sc, tags))
}

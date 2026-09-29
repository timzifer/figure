package geom

import "github.com/timzifer/figure/ir"

// TagAxis is which axis an [AxisTag] is written on.
type TagAxis uint8

// The axes a tag can be written on: the panel's primary vertical and
// horizontal, and the secondary ones a layer on [OnY2] or [OnX2] reads.
const (
	TagY TagAxis = iota
	TagX
	TagY2
	TagX2
)

// AxisTag is a value written on an axis: a filled box on the axis line at
// Value, holding Text, drawn in the axis's gutter over the tick labels it
// covers. It is what a trading screen's last price is, and a crosshair's
// readout.
//
// Text empty is the value as the axis writes a value read off it —
// [github.com/timzifer/figure/scale.ValueLabelOf]: its own format, with the precision a value between
// two ticks needs. Color is the box; the text is chosen to read against it.
// A tag at a value the axis does not show, or on an axis the panel's coord
// does not draw straight, is not drawn. See
// docs/adr/0092-what-a-trading-screen-reads-off-its-edges.md.
type AxisTag struct {
	Axis  TagAxis
	Value float64
	Text  string
	Color ir.Color
}

// Tagger is implemented by a layer that writes values on its panel's axes.
// It is an optional interface, like [Legender]: [Geom] does not grow, and a
// layer that has nothing to tag need not say so.
//
// Render asks for the tags after the layer has trained and the panel's scales
// are ranged, so f's scales place a value exactly where the layer drew it. A
// hidden layer is not asked.
type Tagger interface {
	Tags(f Frame) []AxisTag
}

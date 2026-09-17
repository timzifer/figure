package ir

import "image/color"

// Color is figure's colour type: 8-bit non-premultiplied sRGBA.
//
// It is an alias for color.NRGBA rather than a new type so that colours pass
// straight into any stdlib or backend API taking a color.Color, with no
// conversion layer and no import of figure in code that only wants to name a
// colour.
type Color = color.NRGBA

// RGB returns an opaque colour.
func RGB(r, g, b uint8) Color { return Color{R: r, G: g, B: b, A: 255} }

// RGBA returns a colour with explicit alpha.
func RGBA(r, g, b, a uint8) Color { return Color{R: r, G: g, B: b, A: a} }

// Transparent is the fully transparent colour. A fill or stroke with A == 0 is
// skipped by every backend.
var Transparent = Color{}

// Fade returns c with its alpha scaled by f, clamped to [0, 1].
func Fade(c Color, f float64) Color {
	switch {
	case f <= 0:
		return Color{R: c.R, G: c.G, B: c.B}
	case f >= 1:
		return c
	}
	return Color{R: c.R, G: c.G, B: c.B, A: uint8(float64(c.A)*f + 0.5)}
}

// LineCap is how a stroke terminates.
type LineCap uint8

// The line caps.
const (
	CapButt LineCap = iota
	CapRound
	CapSquare
)

// LineJoin is how two stroke segments meet.
type LineJoin uint8

// The line joins.
const (
	JoinMiter LineJoin = iota
	JoinRound
	JoinBevel
)

// Stroke is the full state needed to stroke a polyline or path.
type Stroke struct {
	Color      Color
	Width      float32
	Cap        LineCap
	Join       LineJoin
	MiterLimit float32   // 0 means the backend default (4)
	Dash       []float32 // nil or empty means solid
	DashOffset float32
}

// Visible reports whether stroking with s would put any ink on the canvas.
func (s Stroke) Visible() bool { return s.Color.A != 0 && s.Width > 0 }

// FillRule selects how a path's interior is determined.
type FillRule uint8

// The fill rules.
const (
	NonZero FillRule = iota
	EvenOdd
)

// GradientStop is one colour stop of a linear gradient, at offset t in [0, 1]
// along the gradient axis.
type GradientStop struct {
	Offset float32
	Color  Color
}

// Fill describes how a path's interior is painted.
//
// A Fill with no Stops is a solid Color. With Stops, it is a linear gradient
// running from Start to End in the same coordinate space as the path; Color is
// then ignored. Radial and sweep gradients are not in the v0.1 IR — no v0.1
// geom needs them, and adding them later is additive.
type Fill struct {
	Color Color
	Start Point
	End   Point
	Stops []GradientStop
}

// Solid returns an opaque-rules solid fill of colour c.
func Solid(c Color) Fill { return Fill{Color: c} }

// IsGradient reports whether f paints a gradient rather than a solid colour.
func (f Fill) IsGradient() bool { return len(f.Stops) > 0 }

// Visible reports whether filling with f would put any ink on the canvas.
func (f Fill) Visible() bool {
	if f.IsGradient() {
		for _, s := range f.Stops {
			if s.Color.A != 0 {
				return true
			}
		}
		return false
	}
	return f.Color.A != 0
}

// Marker is a scatter-point shape, drawn centred on its position.
//
// The set grows at the end, so a backend must not assume it is complete:
// [MarkerPath] appends the outline of any Marker and appends a circle for one
// it does not know, which is what keeps a shape added in a later release from
// being drawn differently — or not at all — by a backend written before it.
type Marker uint8

// The marker shapes.
const (
	MarkerCircle Marker = iota
	MarkerSquare
	MarkerDiamond
	MarkerTriangle
	MarkerCross
	MarkerPlus
)

// MarkerStyle is the paint applied to every instance of a Markers call.
type MarkerStyle struct {
	Size   float32 // nominal diameter in device units
	Fill   Color
	Stroke Stroke
}

// Hatch is a pattern laid over a fill, drawn inside the shape it decorates.
//
// It is the third redundant channel. A dash tells two lines apart and a
// [Marker] tells two point clouds apart; neither does anything for a stacked
// bar, a pie or a stacked area, which have no stroke to dash and no shape to
// swap. A hatch is what those marks are told apart by when colour is not
// available — in greyscale, on a photocopy, or to a reader who cannot separate
// the first two palette entries. See docs/adr/0069.
//
// The set grows at the end, and a backend never sees a Hatch at all: figure
// lowers one into ordinary stroke and fill calls through [FillHatched], so a
// value added in a later release draws correctly on a backend written before
// it. [HatchPath] appends nothing for a Hatch it does not know, which leaves
// the mark with its plain fill rather than with the wrong pattern.
type Hatch uint8

// The hatch patterns.
//
// They fall into four families, and the family is what decides where one is
// worth using: lines are cheap and read at any size, dots carry no direction,
// wavering lines extend the ladder past the four slopes a straight line has,
// and the tilings are rich but want an area to be read in.
const (
	HatchNone Hatch = iota

	// Lines. Two points per stroke.
	HatchDiagonal     // ///
	HatchBackDiagonal // \\
	HatchCross        // both diagonals
	HatchHorizontal   // ===
	HatchVertical     // |||
	HatchGrid         // horizontal and vertical

	// Dots. Filled rather than stroked, and readable on a smaller mark than a
	// line hatch because they point nowhere.
	HatchDots          // on a square grid
	HatchDotsStaggered // every other row offset by half a step

	// Wavering lines. Told apart from the straight ones by shape rather than
	// by slope, which is what lets the ladder grow without reusing an angle.
	HatchZigzag
	HatchWave

	// Tilings. Expensive in path points — a brick course is four segments per
	// cell — and meant for a large area: a treemap box, a sankey band, a
	// region spanning the panel. On a four-pixel bar they read as noise.
	HatchBrick
	HatchTriangles
	HatchScales
)

// Hatching is the paint of one hatch pass: what pattern, in what ink, how
// dense.
//
// Spacing is the period of the pattern in device units, and it is the ordinal
// half of the channel. Changing Hatch says *different*; halving Spacing says
// *more*, because it doubles the ink — which is the reading a ladder of shapes
// cannot give and a stack of severities needs. A theme owns the base value so
// that [Hatching] scales with everything else on a chart drawn at half size.
type Hatching struct {
	// Hatch is the pattern. HatchNone draws nothing.
	Hatch Hatch

	// Line is the ink: its Color and Width are used, and for a filled pattern
	// — dots, triangles, scales — Width is the size of one element rather than
	// a stroke width.
	Line Stroke

	// Spacing is the period in device units. Zero or less draws nothing.
	Spacing float32
}

// Visible reports whether hatching with h would put any ink on the canvas.
func (h Hatching) Visible() bool {
	return h.Hatch != HatchNone && h.Spacing > 0 && h.Line.Color.A != 0 && h.Line.Width > 0
}

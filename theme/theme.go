// Package theme holds the visual tokens a chart is drawn with: colours,
// fonts, sizes and spacings.
//
// A theme is plain data. It is resolved once, before lowering, and every
// decision that is "how should this look" reads from it rather than hardcoding
// a value — which is what makes a second theme a data change instead of a code
// change.
//
// # Tokens
//
// The full [Theme] is wide, and most of it follows from a handful of choices.
// [Tokens] holds those choices and [Build] derives the rest, so a new theme is
// a dozen lines rather than fifty. [Theme.With] edits a built theme, and
// [Register] and [ByName] make a theme reachable by name — which is what a
// configuration file or a command-line flag needs.
package theme

import (
	"sort"
	"sync"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
)

// Theme is the full set of visual tokens for a chart.
type Theme struct {
	Name string

	// Surfaces.
	Background ir.Color // outside the plot area
	PlotFill   ir.Color // inside the plot area

	// Text.
	FontFamily string
	TitleSize  float64
	LabelSize  float64
	TickSize   float64
	TitleColor ir.Color
	LabelColor ir.Color
	TickColor  ir.Color

	// Axes and grid.
	AxisColor     ir.Color
	AxisWidth     float32
	GridColor     ir.Color
	GridWidth     float32
	GridDash      []float32
	TickLength    float32
	TickLabelPad  float32
	AxisTitlePad  float32
	ShowGridX     bool
	ShowGridY     bool
	ShowAxisLineX bool
	ShowAxisLineY bool
	// ShowTicksX and ShowTicksY turn each axis's tick marks and tick labels on
	// or off. They are the third of the three furniture switches, and the one
	// a chart drawn in a coordinate system that does not want an axis reaches
	// for: a pie has no radial quantity to label, and the numbers a linear
	// scale puts round the outside of one are noise about a slot rather than
	// information about the data.
	ShowTicksX     bool
	ShowTicksY     bool
	TickCountHintX int
	TickCountHintY int

	// Axis breaks and folds: how wide the gap is that an interval left out of
	// an axis leaves behind, how large its mark is drawn, and which mark a
	// break gets. A fold is always marked with a small slash. See
	// [github.com/timzifer/figure/scale.Break] and
	// docs/adr/0083-an-axis-break-is-marked-or-not-drawn.md.
	AxisBreakGap  float32
	AxisBreakSize float32
	AxisBreakMark BreakMark
	AxisFoldGap   float32
	AxisFoldSize  float32

	// Legend.
	LegendSwatch  float32
	LegendPad     float32
	LegendGap     float32
	LegendColor   ir.Color
	LegendBG      ir.Color
	LegendBorder  ir.Color
	LegendPadding float32

	// Guides. A chart can carry more than one — a legend for the layers that
	// have a colour each, a colourbar for a layer whose colour comes from a
	// continuous scale — and they stack in one column beside the plot.
	GuideGap float32 // vertical gap between stacked guides

	// Colourbar.
	ColorbarThickness float32 // across the bar
	ColorbarFraction  float64 // of the plot height, before clamping
	ColorbarBorder    ir.Color
	ColorbarTickCount int

	// Size key: the ladder of sample marks a size channel is read off.
	// BubbleSize is the diameter of the largest mark a size scale draws, and
	// therefore the widest sample the key shows; SizeKeyCount is how many
	// samples it shows.
	BubbleSize   float32
	SizeKeyCount int

	// Annotations: reference lines, bands and labels that are not data.
	AnnotationColor   ir.Color
	AnnotationWidth   float32
	AnnotationDash    []float32
	AnnotationOpacity float64 // fill alpha for a shaded band

	// Panels: subplots and facets.
	PanelGap    float32 // between adjacent panels
	StripBG     ir.Color
	StripColor  ir.Color
	StripSize   float64
	StripPad    float32
	StripBorder ir.Color

	// Spacing.
	Margin float32 // outer margin on all four sides

	// Series colours, and the ramps a colour scale falls back to.
	Palette    palette.Qualitative
	Sequential palette.Ramp
	Diverging  palette.Ramp

	// SeriesDashes and SeriesMarkers are the redundant encoding: a second and
	// a third channel a layer is told apart by, so that colour is not the only
	// one. A layer that set neither [github.com/timzifer/figure/geom.Dash]
	// nor [github.com/timzifer/figure/geom.Shape] takes the entry at its own
	// index, the same way it takes its colour from Palette.
	//
	// Both are empty by default, which draws exactly what figure has always
	// drawn. [Redundant] fills them in. See docs/adr/0024.
	SeriesDashes  [][]float32
	SeriesMarkers []ir.Marker

	// SeriesHatches is the third one, and the one a filled mark can use.
	//
	// A dash needs a stroke and a marker needs a point, so neither says
	// anything about a stacked bar, a pie or a stacked area — the marks whose
	// only channel is the colour of a filled shape, and the ones that fail
	// hardest in greyscale. A hatch is what those are told apart by. Empty by
	// default, filled in by [Redundant], replaced by [Hatches].
	// See docs/adr/0069.
	SeriesHatches []HatchStep

	// HatchSpacing and HatchWidth are what a hatch is drawn at: the period of
	// the pattern at density 1, and the width of its lines. Both are device
	// units and both are scaled by [Scaled], for the reason a dash is.
	HatchSpacing float32
	HatchWidth   float32

	// HatchColor is what a hatch is drawn in, mixed with the colour of the
	// mark it lies over.
	//
	// Mixed rather than used outright: a hatch in the mark's own colour would
	// be invisible on a solid bar, and a hatch in one fixed colour would lose
	// the series the mark belongs to. Mixing keeps the hue and guarantees the
	// contrast — and because this is the ink of the theme, a dark theme mixes
	// its hatches lighter without anything here saying so.
	HatchColor ir.Color

	// A projected scene. A flat chart needs none of this; a three-dimensional
	// one needs to know which way the light falls and what a face turned
	// toward it and away from it are mixed toward. There is one directional
	// light and no model beyond it: no specular, no shadows, no ambient
	// occlusion, no textures — see docs/adr/0056-three-dimensional-charts.md.
	//
	// DepthTop and DepthSide are ADR 0055's two shades under ADR 0055's
	// names: a mark extruded by an oblique coord and a face of a surface are
	// both a colour seen in depth, and one ladder is right for both.
	LightDir   [3]float32 // unit direction the light comes from, in scene space
	LightFloor float64    // how lit the least-lit face still is, in [0, 1]
	DepthTop   ir.Color   // a face turned toward the light mixes toward this
	DepthSide  ir.Color   // one turned away mixes toward this
	CubeFill   ir.Color   // the three faces a projected cube shows
	CubeEdge   ir.Color   // its edges
	CubeGrid   ir.Color   // the grid drawn on those faces

	// Default geometry weights.
	LineWidth  float32
	MarkerSize float32
}

// LightTokens are the choices the light theme makes: dark ink on white.
var LightTokens = Tokens{
	Name:       "light",
	Background: ir.RGB(0xFF, 0xFF, 0xFF),
	Panel:      ir.Transparent,
	Strip:      ir.RGB(0xEE, 0xEF, 0xF1),

	Ink:       ir.RGB(0x1A, 0x1A, 0x1A),
	InkMuted:  ir.RGB(0x33, 0x33, 0x33),
	InkSubtle: ir.RGB(0x55, 0x55, 0x55),

	Line:       ir.RGB(0x33, 0x33, 0x33),
	LineSubtle: ir.RGB(0xDD, 0xDD, 0xDD),

	FontSize: 12,
	Space:    4,

	Palette:    palette.OkabeIto,
	Sequential: palette.Viridis,
	Diverging:  palette.BlueOrange,
}

// DarkTokens are the choices the dark theme makes: light ink on near-black.
//
// The axis is deliberately not the same grey as the labels. On a dark surface
// a line at the label's lightness reads heavier than the same line does on
// white, because it is the brighter of the two against its background rather
// than the darker.
var DarkTokens = Tokens{
	Name:       "dark",
	Background: ir.RGB(0x14, 0x16, 0x1A),
	Panel:      ir.Transparent,
	Strip:      ir.RGB(0x22, 0x26, 0x2D),

	Ink:       ir.RGB(0xF2, 0xF3, 0xF5),
	InkMuted:  ir.RGB(0xC8, 0xCB, 0xD0),
	InkSubtle: ir.RGB(0xA0, 0xA4, 0xAB),

	Line:       ir.RGB(0x9A, 0x9E, 0xA6),
	LineSubtle: ir.RGB(0x2A, 0x2E, 0x35),

	FontSize: 12,
	Space:    4,

	Palette:    palette.OkabeIto,
	Sequential: palette.Viridis,
	Diverging:  palette.BlueOrange,
}

// Light is the default theme: dark ink on a white surface.
var Light = Build(LightTokens)

// Dark is the inverted theme: light ink on a near-black surface.
var Dark = Build(DarkTokens)

// Font builds a font reference from the theme's family at the given size.
func (t Theme) Font(size float64) ir.FontRef {
	return ir.FontRef{Family: t.FontFamily, Size: size}
}

// SequentialRamp returns the theme's sequential ramp, falling back to the
// package default so that a zero Theme still produces colours.
func (t Theme) SequentialRamp() palette.Ramp {
	if len(t.Sequential) == 0 {
		return palette.DefaultRamp
	}
	return t.Sequential
}

// DivergingRamp returns the theme's diverging ramp, falling back to the
// package default.
func (t Theme) DivergingRamp() palette.Ramp {
	if len(t.Diverging) == 0 {
		return palette.BlueOrange
	}
	return t.Diverging
}

// Option edits a theme. Options exist so that "the dark theme, in my
// typeface, without a grid" is a call rather than a copy of the struct with
// three fields changed and forty copied.
type Option func(*Theme)

// With returns a copy of t with the options applied. The receiver is not
// modified, so package-level themes stay what they are.
func (t Theme) With(opts ...Option) Theme {
	for _, o := range opts {
		o(&t)
	}
	return t
}

// Named sets the theme's name, which is what [Register] keys it under.
func Named(name string) Option { return func(t *Theme) { t.Name = name } }

// FontFamily sets the logical font family. It is a name, not a file: the
// backend resolves it, and the SVG backend passes it through to the viewer.
func FontFamily(name string) Option { return func(t *Theme) { t.FontFamily = name } }

// FontSize rescales all three text sizes so that the label size becomes size,
// keeping the ratios between title, label and tick.
func FontSize(size float64) Option {
	return func(t *Theme) {
		if size <= 0 || t.LabelSize <= 0 {
			return
		}
		f := size / t.LabelSize
		t.TitleSize *= f
		t.LabelSize = size
		t.TickSize *= f
		t.StripSize *= f
	}
}

// Redundant turns redundant encoding on: every layer gets a dash pattern, a
// marker shape and a hatch of its own alongside its colour, so that a chart
// survives being printed in greyscale, photocopied, or read by the eight
// percent of men who cannot separate its first two palette entries.
//
// It changes nothing about a layer that named its own [Dash], [Shape] or
// [Hatch]: an explicit choice is a choice, and this is a default.
//
// Three ladders, because a chart has three kinds of mark and each one has a
// different thing to spare. The dash ladder runs solid, dashed, dotted,
// dash-dot, long-dash and reaches a line; the marker ladder circle, square,
// triangle, diamond, plus, cross and reaches a point; the hatch ladder reaches
// the filled shapes — a bar, a slice, an area — which have neither a stroke to
// dash nor a shape to swap and were, until it existed, the marks this option
// did nothing for. All three start with what a chart already draws, so the
// first layer is unchanged and only the second one onwards picks up a
// difference — which is the right shape for a default, and the reason the
// ladders are not alphabetical.
//
// The hatch ladder it installs is [DefaultSeriesHatches], which is nominal.
// [Hatches] replaces it with [DensitySeriesHatches] where the layers are
// ordered rather than merely different.
//
// Pass false to turn it back off, which is what a theme built from tokens
// already is.
func Redundant(on bool) Option {
	return func(t *Theme) {
		if !on {
			t.SeriesDashes, t.SeriesMarkers, t.SeriesHatches = nil, nil, nil
			return
		}
		t.SeriesDashes = DefaultSeriesDashes
		t.SeriesMarkers = DefaultSeriesMarkers
		t.SeriesHatches = DefaultSeriesHatches
	}
}

// DefaultSeriesDashes is the ladder [Redundant] uses. The first entry is nil —
// a solid line — so that a single-layer chart looks the way it always did.
//
// The patterns are in device units at the theme's own scale; [Scaled] scales
// them with everything else, because a dash that stayed 6pt long on a chart
// drawn at half size would read as a different pattern.
var DefaultSeriesDashes = [][]float32{
	nil,
	{6, 3},
	{1.5, 2.5},
	{7, 3, 1.5, 3},
	{12, 4},
	{4, 2, 1.5, 2, 1.5, 2},
}

// DefaultSeriesMarkers is the shape ladder [Redundant] uses, starting with the
// circle a scatter draws anyway.
var DefaultSeriesMarkers = []ir.Marker{
	ir.MarkerCircle,
	ir.MarkerSquare,
	ir.MarkerTriangle,
	ir.MarkerDiamond,
	ir.MarkerPlus,
	ir.MarkerCross,
}

// SeriesDash returns the dash pattern for the layer at index i, or nil when
// the theme carries no redundant encoding.
func (t Theme) SeriesDash(i int) []float32 {
	if len(t.SeriesDashes) == 0 || i < 0 {
		return nil
	}
	return t.SeriesDashes[i%len(t.SeriesDashes)]
}

// SeriesMarker returns the marker shape for the layer at index i, and ok false
// when the theme carries no redundant encoding — which leaves the geom's own
// default in place rather than replacing it with a circle.
func (t Theme) SeriesMarker(i int) (ir.Marker, bool) {
	if len(t.SeriesMarkers) == 0 || i < 0 {
		return 0, false
	}
	return t.SeriesMarkers[i%len(t.SeriesMarkers)], true
}

// HatchStep is one rung of the hatch ladder: a pattern, and how dense it is.
//
// Two fields rather than one because the channel has two readings and they are
// not the same reading. Changing the Hatch says *different* — a category is
// not more than another category. Halving the Density says *more*, because it
// halves the period and so doubles the ink, which is what a stack of
// severities or of size classes actually means and what a ladder of shapes
// cannot say. A theme picks the ladder that matches what its layers are.
type HatchStep struct {
	// Hatch is the pattern. The zero value, [ir.HatchNone], draws nothing,
	// which is what the first rung of both shipped ladders is.
	Hatch ir.Hatch

	// Density scales [Theme.HatchSpacing]: 2 is half the ink, 0.5 twice it.
	// Zero means 1, so a ladder written as bare patterns still works.
	Density float32
}

// DefaultSeriesHatches is the ladder [Redundant] installs, and it is nominal:
// every rung a different pattern, no rung heavier than another.
//
// Nominal because the layers of a chart are categories, and a redundant
// encoding must not invent an order the data does not have — a reader who sees
// one series hatched twice as densely as another will read it as twice
// something. [DensitySeriesHatches] is for the case where that reading is the
// right one.
//
// The first rung is [ir.HatchNone], the same rule the dash ladder follows, so
// a single-layer chart is unchanged and the difference appears from the second
// layer onwards. The order after it runs coarse to fine: the patterns that
// survive a small mark come first.
var DefaultSeriesHatches = []HatchStep{
	{Hatch: ir.HatchNone},
	{Hatch: ir.HatchDiagonal},
	{Hatch: ir.HatchBackDiagonal},
	{Hatch: ir.HatchCross},
	{Hatch: ir.HatchDots},
	{Hatch: ir.HatchHorizontal},
	{Hatch: ir.HatchVertical},
	{Hatch: ir.HatchGrid},
	{Hatch: ir.HatchZigzag},
	{Hatch: ir.HatchDotsStaggered},
}

// DensitySeriesHatches is the ordinal ladder: one pattern, getting denser.
//
// A stack painted from it reads as an order in greyscale — which is what a
// stack of severities, of age bands, of size classes is, and what
// [DefaultSeriesHatches] deliberately refuses to claim. Install it with
// [Hatches].
var DensitySeriesHatches = []HatchStep{
	{Hatch: ir.HatchNone},
	{Hatch: ir.HatchDiagonal, Density: 2},
	{Hatch: ir.HatchDiagonal, Density: 1.4},
	{Hatch: ir.HatchDiagonal, Density: 1},
	{Hatch: ir.HatchDiagonal, Density: 0.7},
	{Hatch: ir.HatchDiagonal, Density: 0.5},
}

// SeriesHatch returns the hatch rung for the layer at index i, and ok false
// when the theme carries no hatch ladder — which leaves the layer's own
// pattern in place rather than clearing it, the rule [Theme.SeriesMarker]
// follows.
func (t Theme) SeriesHatch(i int) (HatchStep, bool) {
	if len(t.SeriesHatches) == 0 || i < 0 {
		return HatchStep{}, false
	}
	h := t.SeriesHatches[i%len(t.SeriesHatches)]
	if h.Density <= 0 {
		h.Density = 1
	}
	return h, true
}

// Hatches sets the ladder a layer takes its hatch from, replacing whatever
// [Redundant] installed.
//
// It is how a theme asks for the ordinal reading —
// theme.Hatches(theme.DensitySeriesHatches...) — or names its own house
// patterns. Passing nothing clears the ladder, which turns hatching off
// without turning off the dashes and markers beside it.
func Hatches(steps ...HatchStep) Option {
	return func(t *Theme) { t.SeriesHatches = steps }
}

// HatchSize sets how a hatch is drawn: the period of the pattern at density 1,
// and the width of its lines, both in device units.
//
// It is the knob [Hatches] is not. The ladder decides which pattern a layer
// gets; this decides how coarse every pattern on the chart is, which is a
// property of the chart rather than of a series — a poster read across a room
// wants a coarser hatch than a figure in a paper, and both want the same
// ladder. A value of zero or less leaves that half alone, so either can be set
// without naming the other.
//
// Both are scaled by [Scaled], and the spacing alone by [Density], for the
// reasons a dash is.
func HatchSize(spacing, width float32) Option {
	return func(t *Theme) {
		if spacing > 0 {
			t.HatchSpacing = spacing
		}
		if width > 0 {
			t.HatchWidth = width
		}
	}
}

// Palette sets the qualitative sequence layers take their colours from.
func Palette(p palette.Qualitative) Option { return func(t *Theme) { t.Palette = p } }

// Ramps sets the sequential and diverging ramps a colour scale falls back to.
// A nil argument leaves that ramp alone.
func Ramps(sequential, diverging palette.Ramp) Option {
	return func(t *Theme) {
		if sequential != nil {
			t.Sequential = sequential
		}
		if diverging != nil {
			t.Diverging = diverging
		}
	}
}

// Grid turns the horizontal and vertical grid lines on or off.
func Grid(x, y bool) Option {
	return func(t *Theme) { t.ShowGridX, t.ShowGridY = x, y }
}

// AxisLines turns the axis rules on or off.
func AxisLines(x, y bool) Option {
	return func(t *Theme) { t.ShowAxisLineX, t.ShowAxisLineY = x, y }
}

// Ticks turns each axis's tick marks and tick labels on or off. It is what a
// pie or a donut asks for: neither has an axis worth labelling.
func Ticks(x, y bool) Option {
	return func(t *Theme) { t.ShowTicksX, t.ShowTicksY = x, y }
}

// TickCounts sets how many ticks each axis aims for. They are hints: a scale
// rounds to whatever produces readable labels.
func TickCounts(x, y int) Option {
	return func(t *Theme) {
		if x > 0 {
			t.TickCountHintX = x
		}
		if y > 0 {
			t.TickCountHintY = y
		}
	}
}

// BreakMark is how an axis break is marked.
type BreakMark uint8

const (
	// BreakSlash draws // across the axis line at the two edges of the gap.
	BreakSlash BreakMark = iota
	// BreakZigzag runs a zigzag across the panel along both edges of the gap.
	BreakZigzag
)

// AxisBreaks sets how an axis break is marked, how large its mark is and how
// wide a gap it leaves. A size or gap that is not positive keeps the theme's.
func AxisBreaks(mark BreakMark, size, gap float32) Option {
	return func(t *Theme) {
		t.AxisBreakMark = mark
		if size > 0 {
			t.AxisBreakSize = size
		}
		if gap > 0 {
			t.AxisBreakGap = gap
		}
	}
}

// AxisFolds sets how large a fold's mark is and how wide a gap it leaves. A
// gap of zero folds the axis without leaving any room, which is what an axis
// with hundreds of folds may want; a negative value keeps the theme's.
func AxisFolds(size, gap float32) Option {
	return func(t *Theme) {
		if size > 0 {
			t.AxisFoldSize = size
		}
		if gap >= 0 {
			t.AxisFoldGap = gap
		}
	}
}

// Density scales every spacing by f, tightening or loosening the whole chart
// at once. Text sizes are left alone: a dense chart with unreadable labels is
// not denser, it is worse.
func Density(f float64) Option {
	return func(t *Theme) {
		if f <= 0 {
			return
		}
		s := float32(f)
		t.TickLength *= s
		t.TickLabelPad *= s
		t.AxisTitlePad *= s
		t.AxisBreakGap *= s
		t.AxisFoldGap *= s
		t.LegendSwatch *= s
		t.LegendPad *= s
		t.LegendGap *= s
		t.LegendPadding *= s
		t.GuideGap *= s
		t.ColorbarThickness *= s
		t.PanelGap *= s
		t.StripPad *= s
		t.Margin *= s
		// A hatch period is a spacing like any other: a chart packed tighter
		// wants its patterns packed tighter, or the same hatch reads as denser
		// on the smaller mark it now sits on.
		t.HatchSpacing *= s
	}
}

// Scaled multiplies every length in the theme by f: type sizes, stroke
// widths, spacings, the marker diameter, the margin. It is what makes a chart
// responsive — a plot drawn at half the size it was designed for wants half
// the type, not the same 12pt labels crowding a smaller canvas.
//
// It differs from [Density], which moves the spacings and deliberately leaves
// the text alone: Density is a decision about how tightly a chart is packed,
// and this one is a decision about how big the whole drawing is. Applying it
// to the text is the entire point.
//
// Colours are not lengths and are left alone. See [github.com/timzifer/figure.Responsive],
// which applies this to a plot from the size it is actually being drawn at.
func Scaled(f float64) Option {
	return func(t *Theme) {
		if f <= 0 || f == 1 {
			return
		}
		s := float32(f)

		t.TitleSize *= f
		t.LabelSize *= f
		t.TickSize *= f
		t.StripSize *= f

		t.AxisWidth *= s
		t.GridWidth *= s
		// A dash pattern is a slice, and the theme it came from is a package
		// variable every chart shares — so it is scaled into a new one rather
		// than in place. Scaling theme.Light's grid dash would scale it for
		// every chart in the process, once per render.
		t.GridDash = scaleDash(t.GridDash, s)
		t.TickLength *= s
		t.TickLabelPad *= s
		t.AxisTitlePad *= s
		t.AxisBreakGap *= s
		t.AxisBreakSize *= s
		t.AxisFoldGap *= s
		t.AxisFoldSize *= s

		t.LegendSwatch *= s
		t.LegendPad *= s
		t.LegendGap *= s
		t.LegendPadding *= s

		t.GuideGap *= s
		t.ColorbarThickness *= s

		t.AnnotationWidth *= s
		t.AnnotationDash = scaleDash(t.AnnotationDash, s)

		t.PanelGap *= s
		t.StripPad *= s
		t.Margin *= s

		t.LineWidth *= s
		t.MarkerSize *= s
		t.BubbleSize *= s

		t.SeriesDashes = scaleDashes(t.SeriesDashes, s)
		t.HatchSpacing *= s
		t.HatchWidth *= s
	}
}

func scaleDash(d []float32, f float32) []float32 {
	if len(d) == 0 {
		return d
	}
	out := make([]float32, len(d))
	for i, v := range d {
		out[i] = v * f
	}
	return out
}

func scaleDashes(ds [][]float32, f float32) [][]float32 {
	if len(ds) == 0 {
		return ds
	}
	out := make([][]float32, len(ds))
	for i, d := range ds {
		out[i] = scaleDash(d, f)
	}
	return out
}

// Background sets the canvas colour behind everything.
func Background(c ir.Color) Option { return func(t *Theme) { t.Background = c } }

// PlotFill sets the fill of the plot area itself, which is transparent by
// default so the background shows through.
func PlotFill(c ir.Color) Option { return func(t *Theme) { t.PlotFill = c } }

// The registry. A theme reachable by name is what a configuration file, a
// command-line flag or an HTTP query parameter needs; resolving it in a map
// here keeps that lookup out of every caller.
var (
	registryMu sync.RWMutex
	registry   = map[string]Theme{}
)

func init() {
	Register(Light)
	Register(Dark)
}

// Register adds t to the registry under its own name, replacing any theme
// already registered under it. A theme with an empty name is ignored.
func Register(t Theme) {
	if t.Name == "" {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[t.Name] = t
}

// ByName returns the registered theme with the given name.
func ByName(name string) (Theme, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	t, ok := registry[name]
	return t, ok
}

// Names lists the registered theme names in order.
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

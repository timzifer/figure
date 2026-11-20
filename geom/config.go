package geom

import (
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

type config struct {
	xcol, ycol string
	zcol       string
	x2col      string
	y2col      string
	label      string
	textCol    string

	groupCol   string
	keyCol     string
	widthCol   string
	fromCol    string
	toCol      string
	linkCol    string
	linkage    Linkage
	idCol      string
	parentCol  string
	valCol     string
	padding    float64
	thickness  float64
	midCol     string
	errCol     string
	progCol    string
	errXCol    string
	explode    float64
	explodeCol string
	stack      Stacking
	stackSet   bool
	dodge      bool
	dodgePad   float64
	order      Ordering
	orderSet   bool

	dims        []string
	levels      []float64
	levelCount  int
	top         int
	labelLevels bool
	levelFormat func(float64) string
	resample    Resampling
	branch      Branch
	orient      Orientation

	eventCol    string
	confidence  float64
	censorMarks bool

	hideGuide bool
	extrude   bool

	hideDroplines bool

	secondCol string

	// A candle's four value columns and how it reads and draws them. See
	// [Candle].
	ohlc        [4]string
	direction   CandleDirection
	rising      candleSide
	falling     candleSide
	hollow      bool
	hollowSet   bool
	candleStyle CandleStyleKind

	// The two columns a [DirectionBy] layer compares, from empty for the
	// previous row.
	dirFrom, dirTo string

	// rule is whether a [LastValue] draws its line across the panel.
	rule bool

	sizeCol   string
	sizeScale scale.SizeScale

	color    *ir.Color
	width    float32
	dash     []float32
	tension  float64
	curve    CurveKind
	curveSet bool
	missing  Missing
	marker   ir.Marker
	size     float32
	barWidth float64
	baseline float64
	fill     *ir.Color
	opacity  float64

	// The finishes a filled mark can take beyond its colour. See [Hatch],
	// [HatchDensity], [HatchColor], [HatchWidth], [Gradient], [Corner] and
	// [Inset].
	hatch      ir.Hatch
	hatchSet   bool
	density    float64
	ink        ir.Color
	inkSet     bool
	hatchWidth float64
	gradient   *ir.Color
	corner     float32
	inset      float32
	steps      StepPos
	colorCol   string
	colorScale scale.ColorScale
	whisker    float64
	outliers   bool
	caps       bool
	decimate   Decimation
	budget     int
	cellSize   float64

	bins       int
	binLo      float64
	binHi      float64
	bands      int
	bandHeight float64
	bandwidth  float64
	span       float64
	smooth     Smoothing
	overlap    float64

	closed      bool
	onY2        bool
	onX2        bool
	elide       bool
	avoidLabels bool
	callout     bool
	wrap        bool
	pinned      bool
	minFont     float64
	dashSet     bool
	markerSet   bool
	extend      bool
	extendSet   bool
	fontSize    float64
	halign      ir.HAlign
	valign      ir.VAlign
	alignSet    bool
	rotation    float64

	// extra holds what a third-party option set — see [Extra]. It is nil for
	// every layer built from this package's own options.
	extra map[string]any
}

func newConfig(opts []Option) config {
	c := config{
		barWidth: 0.8, whisker: 1.5, outliers: true, caps: true, opacity: -1, extend: true,
		span: stat.DefaultSpan, overlap: defaultOverlap,
	}
	for _, o := range opts {
		o(&c)
	}
	// A direction is a colour channel with a scale of its own, built once the
	// options that name its colours have all been applied. A layer that also
	// named a colour column keeps that one, and resolve refuses the pair.
	if c.dirTo != "" && c.colorCol == "" {
		c.colorScale = newDirectionScale(c)
	}
	return c
}

// colorFor resolves the mark colour: explicit if set, otherwise the palette
// entry for this layer.
func (c config) colorFor(f Frame) ir.Color {
	if c.color != nil {
		return *c.color
	}
	pal := f.Theme.Palette
	if len(pal) == 0 {
		pal = theme.Light.Palette
	}
	return pal.At(f.Index)
}

// dashFor resolves the dash pattern: the layer's own if it named one,
// otherwise the theme's redundant-encoding ladder — which is empty unless the
// theme asked for it, so this is nil for every ordinary chart.
func (c config) dashFor(f Frame) []float32 {
	if c.dashSet {
		return c.dash
	}
	return f.Theme.SeriesDash(f.Index)
}

// markerFor resolves the marker shape the same way. A layer that named a shape
// keeps it: redundant encoding is a default, and an explicit choice outranks
// a default even when the default is the more accessible one.
func (c config) markerFor(f Frame) ir.Marker {
	if c.markerSet {
		return c.marker
	}
	if m, ok := f.Theme.SeriesMarker(f.Index); ok {
		return m
	}
	return c.marker
}

// hatchFor resolves the hatch and its density for a layer: the layer's own if
// it named one, otherwise the theme's ladder at the layer's index — which is
// empty unless the theme asked for redundant encoding, so this is HatchNone
// for every ordinary chart. It is [config.dashFor] for a filled mark.
func (c config) hatchFor(f Frame) (ir.Hatch, float64) {
	return c.hatchAt(f, f.Index)
}

// hatchAt is hatchFor at an explicit rung of the ladder, which is what a
// grouped layer needs: its series walk the ladder from the layer's own index.
func (c config) hatchAt(f Frame, i int) (ir.Hatch, float64) {
	h, density := c.hatch, c.density
	if !c.hatchSet {
		step, ok := f.Theme.SeriesHatch(i)
		if !ok {
			return ir.HatchNone, 1
		}
		h = step.Hatch
		if density <= 0 {
			density = float64(step.Density)
		}
	}
	if density <= 0 {
		density = 1
	}
	return h, density
}

// hatchingOf assembles the hatching a mark is drawn with, in the colour the
// caller resolved for that mark.
//
// The ink is the mark's own colour rather than a darker relative of it,
// because the hatch is the mark saying which series it is — the same thing its
// outline says, and drawn in the same colour so that the two do not read as
// two different statements.
func (c config) hatchingOf(f Frame, i int, base ir.Color) ir.Hatching {
	h, density := c.hatchAt(f, i)
	if h == ir.HatchNone || base.A == 0 {
		return ir.Hatching{}
	}
	spacing, width := f.Theme.HatchSpacing, f.Theme.HatchWidth
	if spacing <= 0 {
		spacing = theme.Light.HatchSpacing
	}
	if width <= 0 {
		width = theme.Light.HatchWidth
	}
	if c.hatchWidth > 0 {
		width *= float32(c.hatchWidth)
	}
	// An explicit colour is used as given: a layer that names one has said the
	// hatch is not a second reading of the mark's colour, so mixing it back
	// toward the mark would undo exactly what it asked for.
	line := c.ink
	if !c.inkSet {
		ink := f.Theme.HatchColor
		if ink.A == 0 {
			ink = theme.Light.HatchColor
		}
		line = palette.Lerp(opaque(base), ink, hatchMix)
	}
	return ir.Hatching{
		Hatch: h,
		Line: ir.Stroke{
			Color: line,
			Width: width,
			Cap:   ir.CapButt,
		},
		Spacing: spacing * float32(density),
	}
}

// hatchMix is how far a hatch's ink is mixed from the mark's own colour toward
// the theme's.
//
// Far enough to contrast with a solid fill of that colour, which is what the
// hatch has to be legible against — and that is the harder of the two
// constraints, because the pattern is thin and a thin line needs more contrast
// than a filled shape does to read at all. Two thirds of the way keeps enough
// of the hue that the pattern still belongs to its series.
const hatchMix = 0.7

// opaque is c with its alpha discarded.
//
// A hatch lies over a fill that may be faded — an area is drawn at a fifth of
// its colour — and a pattern drawn at the same fifth would be a pattern nobody
// can see. It takes the hue and leaves the transparency to the fill.
func opaque(c ir.Color) ir.Color { return ir.Color{R: c.R, G: c.G, B: c.B, A: 255} }

// paint turns a resolved colour into the fill a mark is painted with: the
// colour itself, or the ramp [Gradient] asked for.
//
// The ramp runs along the value axis of the path being filled, from the colour
// at the baseline end to the named colour at the far end. The path is the
// layer's whole batch of marks, which is deliberate — see [Gradient].
func (c config) paint(col ir.Color, p *ir.Path) ir.Fill {
	if c.gradient == nil || p == nil || p.Empty() {
		return ir.Solid(col)
	}
	b := p.Bounds()
	if b.Empty() {
		return ir.Solid(col)
	}
	f := ir.Fill{Stops: []ir.GradientStop{{Offset: 0, Color: col}, {Offset: 1, Color: *c.gradient}}}
	if c.orient == Horizontal {
		// Measured across X: the baseline is on the left, the far end right.
		f.Start = ir.Point{X: b.Min.X, Y: b.Min.Y}
		f.End = ir.Point{X: b.Max.X, Y: b.Min.Y}
		return f
	}
	// Measured up Y, and Y grows downward on a canvas: the baseline is at the
	// bottom of the box and the tips are at the top.
	f.Start = ir.Point{X: b.Min.X, Y: b.Max.Y}
	f.End = ir.Point{X: b.Min.X, Y: b.Min.Y}
	return f
}

// insetOf is the inner border a filled mark is drawn with, or the zero stroke
// where the layer asked for none.
func (c config) insetOf(base ir.Color) ir.Stroke {
	if c.inset <= 0 || base.A == 0 {
		return ir.Stroke{}
	}
	return ir.Stroke{Color: opaque(base), Width: c.inset, Join: ir.JoinMiter}
}

// fillMark is the one call a filled geom makes: it paints p in col, lays the
// layer's hatch over it, and draws its inner border.
//
// It exists so that a layer gains every finish at one call site rather than
// three, and so that the order they are drawn in — fill, then hatch, then
// border — is decided once. The border goes last because it is the mark's edge
// and a hatch line running into it should stop at it.
func (c config) fillMark(b ir.Backend, p *ir.Path, f Frame, rung int, col ir.Color) {
	ir.FillHatched(b, p, c.paint(col, p), ir.NonZero, c.hatchingOf(f, f.Index+rung, col))
	if edge := c.insetOf(col); edge.Visible() {
		ir.FillInset(b, p, ir.Fill{}, ir.NonZero, edge)
	}
}

// boxSwatch is the single legend entry of a filled layer: the box, in the
// layer's colour, wearing the same finish its marks wear.
func (c config) boxSwatch(f Frame, label string, col ir.Color) LegendEntry {
	return LegendEntry{
		Label:    label,
		Color:    col,
		Kind:     SwatchBox,
		Hatching: c.hatchingOf(f, f.Index, col),
		Corner:   c.corner,
	}
}

func (c config) labelFor() string {
	if c.label != "" {
		return c.label
	}
	return c.ycol
}

// labelForX is labelFor for a layer whose subject is its X column: a
// histogram, an ECDF and a ridgeline all summarise one column, and it is on X.
// Falling back to the Y column would name them after an axis they compute.
func (c config) labelForX() string {
	if c.label != "" {
		return c.label
	}
	return c.xcol
}

// fillFor resolves the interior colour of a filled geom.
//
// An explicit [Fill] is taken at face value — a caller who names a colour has
// already decided how solid it should be. A fill inherited from the palette is
// faded to defaultOpacity instead, because a geom that fills the plot area has
// to sit behind the lines and points it belongs to rather than bury them.
func (c config) fillFor(f Frame, defaultOpacity float64) ir.Color {
	return c.fillOf(c.colorFor(f), defaultOpacity)
}

// fillOf is fillFor over a colour the caller already resolved, which is what a
// grouped layer needs: the base colour is the series' own rather than the
// layer's.
func (c config) fillOf(base ir.Color, defaultOpacity float64) ir.Color {
	op := defaultOpacity
	if c.fill != nil {
		base, op = *c.fill, 1
	}
	if c.opacity >= 0 {
		op = clamp01(c.opacity)
	}
	return ir.Fade(base, op)
}

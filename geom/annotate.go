package geom

import (
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

// Annotations are the marks a chart carries that are not data: a threshold
// line, a shaded window, a note pointing at what happened. They take values
// rather than a [data.Source], because there is no column behind "the SLO is
// 200ms".
//
// They are ordinary layers otherwise. They are added with [Plot.Add] in the
// order they should be drawn, they are clipped to the plot area like anything
// else, and by default they extend the axis domain so that the thing being
// annotated is actually in view — [Extend] turns that off for an annotation
// that should only appear if the data happens to reach it. [Locus] is the one
// whose default runs the other way, and its documentation says why.
//
// An annotation appears in the legend only if it was given a [Label]. Most are
// self-explanatory in place and an extra legend row would be noise.

// HLine draws a horizontal reference line across the plot at y.
func HLine(y float64, opts ...Option) Geom {
	return &ruleGeom{at: y, vertical: false, cfg: newConfig(opts)}
}

// VLine draws a vertical reference line across the plot at x.
func VLine(x float64, opts ...Option) Geom {
	return &ruleGeom{at: x, vertical: true, cfg: newConfig(opts)}
}

type ruleGeom struct {
	at       float64
	vertical bool
	cfg      config
}

func (g *ruleGeom) Train(t Training) error {
	x, y := t.X, t.Y
	if g.cfg.extend {
		g.axis(x, y).Train(g.at)
	}
	return nil
}

func (g *ruleGeom) axis(x, y scale.Scale) scale.Scale {
	if g.vertical {
		return x
	}
	return y
}

func (g *ruleGeom) Build(b ir.Backend, f Frame) error {
	s := g.axis(f.X, f.Y)
	if !defined(s, g.at) {
		return nil
	}
	stroke := g.cfg.annotationStroke(f)
	if !stroke.Visible() {
		return nil
	}
	// A rule spans the *other* axis end to end, and where that axis ends is
	// what the coord decided when it framed the panel — the edge of the
	// rectangle under Cartesian, and a whole turn of the circle under Polar,
	// where a rule at a constant Y is a ring.
	cd := f.Coords()
	x0, x1, y0, y1 := cd.Extent()
	x0, x1 = ordered(x0, x1)
	y0, y1 = ordered(y0, y1)
	p := s.Map(g.at)
	var path ir.Path
	if g.vertical {
		strokeRun(b, cd, &path, []ir.Point{cd.Point(p, y0), cd.Point(p, y1)}, stroke, false)
	} else {
		strokeRun(b, cd, &path, []ir.Point{cd.Point(x0, p), cd.Point(x1, p)}, stroke, false)
	}
	return nil
}

func (g *ruleGeom) Legend(f Frame) (LegendEntry, bool) {
	return g.cfg.annotationLegend(f, SwatchLine)
}

// HBand shades the horizontal strip between two Y values, across the whole
// plot. It is how a tolerance range or a target window is drawn.
func HBand(y0, y1 float64, opts ...Option) Geom {
	return &bandGeom{lo: y0, hi: y1, vertical: false, cfg: newConfig(opts)}
}

// VBand shades the vertical strip between two X values, across the whole plot.
// It is how a maintenance window or a highlighted interval is drawn.
func VBand(x0, x1 float64, opts ...Option) Geom {
	return &bandGeom{lo: x0, hi: x1, vertical: true, cfg: newConfig(opts)}
}

type bandGeom struct {
	lo, hi   float64
	vertical bool
	cfg      config
}

func (g *bandGeom) Train(t Training) error {
	x, y := t.X, t.Y
	if !g.cfg.extend {
		return nil
	}
	s := x
	if !g.vertical {
		s = y
	}
	s.Train(g.lo, g.hi)
	return nil
}

func (g *bandGeom) Build(b ir.Backend, f Frame) error {
	s := f.Y
	if g.vertical {
		s = f.X
	}
	if !defined(s, g.lo) || !defined(s, g.hi) {
		return nil
	}
	a, c := s.Map(g.lo), s.Map(g.hi)
	if c < a {
		a, c = c, a
	}
	// The band spans the other axis end to end, wherever the coord put its
	// ends — see the same argument in [ruleGeom.Build].
	cd := f.Coords()
	ax0, ax1, ay0, ay1 := cd.Extent()
	ax0, ax1 = ordered(ax0, ax1)
	ay0, ay1 = ordered(ay0, ay1)
	r := ir.Rect{Min: ir.Point{X: ax0, Y: a}, Max: ir.Point{X: ax1, Y: c}}
	if g.vertical {
		r = ir.Rect{Min: ir.Point{X: a, Y: ay0}, Max: ir.Point{X: c, Y: ay1}}
	}
	if a == c {
		// A band whose two bounds map to the same position has no area. It is
		// a legitimate degenerate case — a window of zero width — and drawing
		// a hairline instead would misreport it as having extent.
		return nil
	}
	fill := g.cfg.annotationFill(f)
	if fill.A == 0 {
		return nil
	}
	var p ir.Path
	area(&p, cd, r)
	b.FillPath(&p, ir.Solid(fill), ir.NonZero)
	return nil
}

func (g *bandGeom) Legend(f Frame) (LegendEntry, bool) {
	e, ok := g.cfg.annotationLegend(f, SwatchBox)
	if ok {
		e.Color = g.cfg.annotationFill(f)
	}
	return e, ok
}

// Segment draws a straight line between two points in data space. Unlike
// [HLine] it does not span the plot, so it is what an arrow-less callout or a
// hand-placed trend line is made of.
func Segment(x0, y0, x1, y1 float64, opts ...Option) Geom {
	return &segmentGeom{x0: x0, y0: y0, x1: x1, y1: y1, cfg: newConfig(opts)}
}

type segmentGeom struct {
	x0, y0, x1, y1 float64
	cfg            config
}

func (g *segmentGeom) Train(t Training) error {
	x, y := t.X, t.Y
	if g.cfg.extend {
		x.Train(g.x0, g.x1)
		y.Train(g.y0, g.y1)
	}
	return nil
}

func (g *segmentGeom) Build(b ir.Backend, f Frame) error {
	if !defined(f.X, g.x0) || !defined(f.X, g.x1) ||
		!defined(f.Y, g.y0) || !defined(f.Y, g.y1) {
		return nil
	}
	stroke := g.cfg.annotationStroke(f)
	if !stroke.Visible() {
		return nil
	}
	cd := f.Coords()
	var path ir.Path
	strokeRun(b, cd, &path, []ir.Point{
		cd.Point(f.X.Map(g.x0), f.Y.Map(g.y0)),
		cd.Point(f.X.Map(g.x1), f.Y.Map(g.y1)),
	}, stroke, false)
	return nil
}

func (g *segmentGeom) Legend(f Frame) (LegendEntry, bool) {
	return g.cfg.annotationLegend(f, SwatchLine)
}

// Region shades an axis-aligned rectangle in data space, bounded on both axes.
// [HBand] and [VBand] are the cases where one axis is unbounded.
func Region(x0, y0, x1, y1 float64, opts ...Option) Geom {
	return &regionGeom{x0: x0, y0: y0, x1: x1, y1: y1, cfg: newConfig(opts)}
}

type regionGeom struct {
	x0, y0, x1, y1 float64
	cfg            config
}

func (g *regionGeom) Train(t Training) error {
	x, y := t.X, t.Y
	if g.cfg.extend {
		x.Train(g.x0, g.x1)
		y.Train(g.y0, g.y1)
	}
	return nil
}

func (g *regionGeom) Build(b ir.Backend, f Frame) error {
	if !defined(f.X, g.x0) || !defined(f.X, g.x1) ||
		!defined(f.Y, g.y0) || !defined(f.Y, g.y1) {
		return nil
	}
	x0, x1 := f.X.Map(g.x0), f.X.Map(g.x1)
	y0, y1 := f.Y.Map(g.y0), f.Y.Map(g.y1)
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	r := ir.R(x0, y0, x1, y1)
	if r.Empty() {
		return nil
	}
	var p ir.Path
	area(&p, f.Coords(), r)
	if fill := g.cfg.annotationFill(f); fill.A != 0 {
		b.FillPath(&p, ir.Solid(fill), ir.NonZero)
	}
	// A region gets an outline as well as a fill when the caller asked for a
	// colour explicitly: the fill alone is faint by design, and a named colour
	// means the region is the point rather than the background.
	if g.cfg.color != nil {
		if stroke := g.cfg.annotationStroke(f); stroke.Visible() {
			b.StrokePath(&p, stroke)
		}
	}
	return nil
}

func (g *regionGeom) Legend(f Frame) (LegendEntry, bool) {
	e, ok := g.cfg.annotationLegend(f, SwatchBox)
	if ok {
		e.Color = g.cfg.annotationFill(f)
	}
	return e, ok
}

// Note places a text label at a position in data space.
//
// Alignment is about that position: the default puts the text's start on it.
// Use [Align] to hang the label off the other side of a line it is naming.
func Note(x, y float64, text string, opts ...Option) Geom {
	return &noteGeom{x: x, y: y, text: text, cfg: newConfig(opts)}
}

type noteGeom struct {
	x, y float64
	text string
	cfg  config
}

func (g *noteGeom) Train(t Training) error {
	x, y := t.X, t.Y
	if g.cfg.extend {
		x.Train(g.x)
		y.Train(g.y)
	}
	return nil
}

func (g *noteGeom) Build(b ir.Backend, f Frame) error {
	if g.text == "" || !defined(f.X, g.x) || !defined(f.Y, g.y) {
		return nil
	}
	col := g.cfg.annotationColor(f)
	if col.A == 0 {
		return nil
	}
	size := g.cfg.fontSize
	if size <= 0 {
		size = f.Theme.LabelSize
	}
	b.Text(ir.TextRun{
		Text:     g.text,
		Font:     f.Theme.Font(size),
		At:       f.Coords().Point(f.X.Map(g.x), f.Y.Map(g.y)),
		H:        g.cfg.halign,
		V:        g.cfg.valign,
		Rotation: g.cfg.rotation,
		Color:    col,
	})
	return nil
}

func (g *noteGeom) Legend(f Frame) (LegendEntry, bool) {
	return LegendEntry{}, false
}

// --- shared annotation styling -------------------------------------------

// annotationColor resolves the ink of a mark that is not data.
//
// It does not fall back to the palette the way [config.colorFor] does: an
// annotation is not a series, and giving it the next series colour would make
// a threshold line look like one more thing that was measured.
func (c config) annotationColor(f Frame) ir.Color {
	if c.color != nil {
		return *c.color
	}
	if f.Theme.AnnotationColor.A != 0 {
		return f.Theme.AnnotationColor
	}
	return theme.Light.AnnotationColor
}

func (c config) annotationStroke(f Frame) ir.Stroke {
	width := f.Theme.AnnotationWidth
	if width <= 0 {
		width = theme.Light.AnnotationWidth
	}
	dash := f.Theme.AnnotationDash
	if c.dashSet {
		dash = c.dash
	}
	return ir.Stroke{
		Color: c.annotationColor(f),
		Width: pick(c.width, width),
		Dash:  dash,
		Cap:   ir.CapButt,
	}
}

// annotationFill resolves the interior of a shaded annotation.
//
// The default is the annotation ink at the theme's annotation opacity, which
// is low: a band is background for the data, not a competitor to it. An
// explicit [Fill] is taken at full strength, because naming a colour is a
// decision about how solid it should be.
func (c config) annotationFill(f Frame) ir.Color {
	base := c.annotationColor(f)
	op := f.Theme.AnnotationOpacity
	if op <= 0 {
		op = theme.Light.AnnotationOpacity
	}
	if c.fill != nil {
		base, op = *c.fill, 1
	}
	if c.opacity >= 0 {
		op = clamp01(c.opacity)
	}
	return ir.Fade(base, op)
}

func (c config) annotationLegend(f Frame, kind SwatchKind) (LegendEntry, bool) {
	if c.label == "" {
		return LegendEntry{}, false
	}
	return LegendEntry{
		Label: c.label,
		Color: c.annotationColor(f),
		Kind:  kind,
		Dash:  c.annotationStroke(f).Dash,
		Width: c.annotationStroke(f).Width,
	}, true
}

// Locus draws a family of curves given by a formula rather than by a tick: the
// M and N contours printed under a Nichols diagram, the constant-VSWR circles
// and constant-Q arcs printed under a Smith chart.
//
//	p.X(scale.Linear(scale.Domain(-360, 0), scale.TickValues(-360, -270, -180, -90, 0)))
//	p.Y(scale.Linear(scale.Domain(-40, 40)))
//	p.Add(geom.Locus(stat.NicholsM, []float64{-12, -6, -3, -1, 0, 1, 3, 6, 12}))
//	p.Add(geom.Locus(stat.NicholsN, []float64{-1, -5, -10, -20, -45, -90, -150}))
//	p.Add(geom.Line(open, geom.X("phase"), geom.Y("gain")))
//
// It is an annotation and not furniture, which is the whole of
// docs/adr/0050-locus-annotations.md: a grid line is where an axis says a value
// is, and none of these is at a value of either axis. They are where the
// *chart* says something is, which is what an annotation has been since v0.1.
//
// # The coord draws the family
//
// A family emits points in data space, so a locus goes through the coordinate
// stage like every other mark and neither half knows what chart it landed on. A
// constant-reflection locus is the set of impedances whose reflection has a
// given magnitude; [github.com/timzifer/figure/coord.Smith] maps it into the
// circle it looks like, and there is no Smith-specific drawing code anywhere in
// it. [HLine] is the degenerate member of the same idea — the locus of constant
// y — which is why this sits beside it.
//
// # It does not train the axes
//
// Alone among the annotations. A threshold that is out of view is not an
// annotation, so [HLine] extends the domain; a locus says what the region of
// the plane means, and the region is whatever the axes already show. It is also
// the only defensible answer arithmetically: the 0 dB M contour runs to −∞ dB,
// and an axis trained on it has no ticks left anywhere a reader is looking.
// [Extend] turns training on for a caller drawing a bounded family.
//
// # What it does not do
//
// It writes nothing on the curves. A locus gets a legend entry from [Label]
// like any annotation, but "3 dB" written along the contour itself, rotated to
// the tangent, needs an anchor rule and a seat at the table
// docs/adr/0040-label-collision-avoidance.md sets for participating text
// layers. The chart is readable without it because the contours nest
// monotonically. It also announces nothing: there is no [github.com/timzifer/figure/data.Source]
// behind it, so a pointer passes over it exactly as it passes over a reference
// line.
func Locus(f Family, levels []float64, opts ...Option) Geom {
	g := &locusGeom{family: f, cfg: newConfig(opts)}
	g.cfg.levels = append(g.cfg.levels[:0], levels...)
	return g
}

// Family is one indexed set of curves in data space, and the four built-in ones
// are [github.com/timzifer/figure/stat.NicholsM], [github.com/timzifer/figure/stat.NicholsN],
// [github.com/timzifer/figure/stat.SmithVSWR] and [github.com/timzifer/figure/stat.SmithQ].
//
// It is stat's type rather than a second one of this package's because a family
// is a pure function of numbers, which is what that package is for — and
// because a family written there cannot accidentally reach a scale, a theme or
// a geom.
type Family = stat.Family

// Extent is the region of data space a [Family] is asked for a curve over.
type Extent = stat.Extent

type locusGeom struct {
	family Family
	cfg    config

	// The buffers one curve is built in, kept on the layer so that a chart
	// redrawn every frame does not allocate one per frame.
	xs, ys []float64
	pts    []ir.Point
}

// Train is where a locus does nothing, unless it was asked to.
//
// There is no chicken-and-egg here to be clever about: with [Extend] set the
// family is evaluated once against an empty extent, which is the curve it draws
// when nothing bounds it, and the finite points of that train the axes. That is
// right for a bounded family — a VSWR circle is the same circle whatever the
// axes say — and it is exactly why an unbounded one does not do it by default.
func (g *locusGeom) Train(t Training) error {
	if g.family == nil || !g.cfg.extendSet || !g.cfg.extend {
		return nil
	}
	for _, level := range g.cfg.levels {
		g.xs, g.ys = g.family.Locus(g.xs[:0], g.ys[:0], level, Extent{})
		for i := range g.xs {
			if finite(g.xs[i]) && finite(g.ys[i]) {
				t.X.Train(g.xs[i])
				t.Y.Train(g.ys[i])
			}
		}
	}
	return nil
}

func (g *locusGeom) Build(b ir.Backend, f Frame) error {
	if g.family == nil || len(g.cfg.levels) == 0 {
		return nil
	}
	stroke := g.cfg.annotationStroke(f)
	if !stroke.Visible() {
		return nil
	}
	cd, ext := f.Coords(), g.extent(f)
	var path ir.Path
	for _, level := range g.cfg.levels {
		g.xs, g.ys = g.family.Locus(g.xs[:0], g.ys[:0], level, ext)
		g.pts = g.pts[:0]
		for i := range g.xs {
			// A gap in the curve and a point neither scale can place are the
			// same thing to a mark: the run ends and the next one starts after
			// it. That is the missing-data policy every layer has followed
			// since v0.1, and it is what a family's own NaN is written in.
			if !defined(f.X, g.xs[i]) || !defined(f.Y, g.ys[i]) {
				strokeRun(b, cd, &path, g.pts, stroke, false)
				g.pts = g.pts[:0]
				continue
			}
			g.pts = append(g.pts, cd.Point(f.X.Map(g.xs[i]), f.Y.Map(g.ys[i])))
		}
		strokeRun(b, cd, &path, g.pts, stroke, false)
	}
	return nil
}

// extent is what the family is asked for its curves over: the panel's own
// domains, and how finely the panel is worth sampling.
//
// The sample count is measured against the device rectangle, which is ADR
// 0028's rule rather than a violation of ADR 0011's: how many points a curve is
// drawn with is a question about how wide the panel is. The half of that rule
// which keeps decimation out of Train — what an axis reports must not depend on
// how wide the chart is — does not bind a mark that reports nothing to an axis.
func (g *locusGeom) extent(f Frame) Extent {
	ext := Extent{Steps: locusSteps(f.Area)}
	ext.X0, ext.X1 = ordered64(f.X.Domain())
	ext.Y0, ext.Y1 = ordered64(f.Y.Domain())
	return ext
}

// locusSteps is how many samples a panel of this size is worth. The bounds are
// a curve that stays smooth on a chart small enough to be a sparkline, and one
// that stops getting finer long before a wall display.
func locusSteps(area ir.Rect) int {
	n := int((area.Dx() + area.Dy()) / 2)
	switch {
	case n < 90:
		return 90
	case n > 720:
		return 720
	}
	return n
}

func ordered64(a, b float64) (float64, float64) {
	if b < a {
		return b, a
	}
	return a, b
}

func (g *locusGeom) Legend(f Frame) (LegendEntry, bool) {
	return g.cfg.annotationLegend(f, SwatchLine)
}

func (g *locusGeom) Describe() Desc {
	d := g.cfg.describe(MarkLocus)
	d.Family = g.family
	// The training the layer is actually doing, which is none unless the caller
	// asked: a Desc reports what a layer is rather than what the option set's
	// default would have made it.
	d.Extend = g.cfg.extendSet && g.cfg.extend
	return d
}

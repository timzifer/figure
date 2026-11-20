package geom

import (
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// Option configures a geom. Options are shared across geom constructors: an
// option a given geom has no use for is accepted and ignored, which keeps the
// API one namespace instead of six — including for a geom defined outside this
// package, which reads what an option set through [Configure] and adds a knob
// of its own through [Extra].
type Option func(*config)

// X selects the column mapped to the horizontal axis.
func X(col string) Option { return func(c *config) { c.xcol = col } }

// Y selects the column mapped to the vertical axis.
func Y(col string) Option { return func(c *config) { c.ycol = col } }

// Z selects the column mapped to the depth axis.
//
// Nothing in this package reads it: a flat chart has no depth, and a geom
// accepts and ignores an option it has no use for. It lives here rather than
// in a second option set of its own because the options are one namespace —
// a parallel set would mean two spellings of [X], [Color] and [GroupBy] — and
// because a depth column is a channel of the data rather than a knob of the
// renderer. The layers that read it are in
// [github.com/timzifer/figure/three]. ADR 0056 is the record.
func Z(col string) Option { return func(c *config) { c.zcol = col } }

// Droplines decides whether a projected scatter —
// [github.com/timzifer/figure/three.Scatter3] — draws a rule from each point
// down to the floor of its scene. The default is true.
//
// They are on by default because they are what makes a 3D scatter readable at
// all: a point floating in a projected box has no height a reader can judge,
// and the rule to the floor is what turns its position into an x, a y and a z.
// Turn them off for a cloud dense enough that the rules become a curtain, and
// say so, because the heights have then stopped being readable. Nothing in
// this package reads it; see docs/adr/0058-what-3d-is-for.md.
func Droplines(show bool) Option { return func(c *config) { c.hideDroplines = !show } }

// Levels pins the contour levels a [Contour] traces.
//
// It is what makes two charts of two tables comparable, which is the argument
// [BinRange] makes for a histogram: levels chosen from each table's own extent
// put the same value on different lines, and a reader comparing the two is then
// comparing the level lists rather than the data. It is also how a flat contour
// and a projected scene's floor are made the same lines — both are handed this.
//
// Naming none asks [LevelCount] for a round set chosen from the data.
func Levels(vs ...float64) Option {
	return func(c *config) { c.levels = append(c.levels[:0], vs...) }
}

// LevelCount asks a [Contour] for about n levels, chosen from the data at a
// round step by [github.com/timzifer/figure/stat.Levels]. It is ignored when
// [Levels] named them, and defaults to [DefaultLevels].
//
// About n, because a round step a reader can do arithmetic with is worth more
// than an exact count of awkward ones.
func LevelCount(n int) Option { return func(c *config) { c.levelCount = n } }

// LabelLevels writes each level's own value along the curve it is drawn at, for
// a [Contour] and for a [Locus]. The curve is gapped to make room and the text
// is turned to it, so a contour map reads without a colourbar beside it.
//
// The placement is the curve's: the label sits at the flattest stretch of the
// run that has room for it, and a run with no such stretch carries no label.
// Labels ask the panel for their space in drawing order and are never moved off
// their own curve — a label nudged onto a neighbour would name a level it is
// not on. See docs/adr/0073-labels-on-a-curve.md.
//
// One label per run: a level that crosses the panel three times is written
// three times, and a run that snakes the whole way across is written once.
func LabelLevels(on bool) Option { return func(c *config) { c.labelLevels = on } }

// LevelFormat is how [LabelLevels] writes a level. The default is the shortest
// decimal that reads back as the same number.
//
// A Go function does not survive a document, which is the rule
// docs/adr/0041-qq-plots.md set for a quantile function and
// [github.com/timzifer/figure/scale.Format]'s for a tick label: a chart written
// down carries that it labels its levels, and reads back labelling them the
// standard way.
func LevelFormat(fn func(v float64) string) Option {
	return func(c *config) { c.levelFormat = fn }
}

// Top caps how many groups a mark that ranks its own groups draws: the biggest
// n, and the rest left out.
//
// [Intersections] and [SetMatrix] read it, because an UpSet plot of a real
// membership table has more combinations than a screen has columns and the
// long tail of them is a row of single elements. It is not a decimation — the
// marks that are drawn are drawn in full, and what is left out is left out
// because it was ranked rather than because the panel is narrow ([Budget] is
// the other thing).
//
// Both halves of an UpSet must be given the same value, or the dots would name
// columns the bars do not have.
func Top(n int) Option { return func(c *config) { c.top = n } }

// Dims names the columns a mark draws against a panel's own axes, in the order
// those axes are drawn.
//
// It is the one option in this package that carries a list of columns, and it
// is [Parallel]'s: a parallel-coordinates panel has an axis per dimension, and
// naming them one channel at a time would put a limit on how many there may
// be where the chart has none.
//
// The names are matched to the coord's dimensions **in order**, and a layer
// that names a different number of them is an error out of Train rather than a
// chart with an axis nothing is drawn against. The coord's dimensions carry a
// label and a scale and never a column name, which is what keeps a coord from
// knowing what a table is.
func Dims(cols ...string) Option {
	return func(c *config) { c.dims = append(c.dims[:0], cols...) }
}

// Resample sets how a [Raster] combines the cells that land on one pixel, when
// its lattice is finer than the panel.
//
// The default is [Nearest], because it is the only member of the family that
// never shows a number that was not measured. It is also the one that aliases,
// which is why a spectrogram passes [Max]: a tone one bin wide is a peak, and
// the nearest cell to a pixel centre is as likely to be the silence beside it.
//
// It has no effect where the lattice is coarser than the panel. Growing a cell
// to cover several pixels is not a reduction, and there is no member of this
// family that smooths — see [Raster] for why.
func Resample(r Resampling) Option { return func(c *config) { c.resample = r } }

// Color sets the mark colour, overriding the palette.
func Color(col ir.Color) Option { return func(c *config) { c.color = &col } }

// Fill sets a fill colour separately from the stroke colour, for geoms that
// have both.
func Fill(col ir.Color) Option { return func(c *config) { c.fill = &col } }

// Width sets the stroke width in device units.
func Width(w float32) Option { return func(c *config) { c.width = w } }

// Dash sets a dash pattern in device units. Dashing a series is redundant
// encoding: it keeps the chart readable in greyscale and for colourblind
// readers.
func Dash(pattern ...float32) Option {
	return func(c *config) { c.dash, c.dashSet = pattern, true }
}

// Tension is the parameter of the curve families that have one. For the
// cardinal family it scales the tangents: 1 is the classic uniform
// Catmull-Rom curve and smaller values pull it back towards the polyline. For
// [CurveBundle] it is beta, the blend towards the straight chord. Every other
// family ignores it.
//
// Naming a tension without naming a [Curve] selects [CurveCardinal], which is
// the whole of what this option meant before there was a family to choose —
// so `Tension(0.4)` draws today exactly what it drew before.
func Tension(t float64) Option { return func(c *config) { c.tension = t } }

// CurveKind is the path drawn between the vertices of a connected layer.
//
// The families divide in two, and the division is the thing to know before
// choosing one.
//
// An *interpolating* family — [CurveLinear], [CurveCardinal], [CurveMonotone],
// [CurveNatural] — passes through every vertex, so the curve is a claim about
// what the quantity did between two rows. An *approximating* family —
// [CurveBasis], [CurveBundle] — does not: it is a drawing of the sequence, and
// the ink between two rows is nowhere the data was. Reach for one when the
// shape of a bundle of traces is the point and the individual values are not,
// and say in the caption that you did.
//
// Either way [Rows] reports the vertices, because a row is where its value is
// regardless of where the ink went.
type CurveKind uint8

// The curve families. [CurveLinear] is the default: a polyline is the only
// path between two rows that invents nothing.
//
// The names are Vega-Lite's, and so are the open and closed variants: an
// *open* family drops the segments at the two ends, where the spline has to
// invent a neighbour, and a *closed* one wraps the neighbour round instead —
// which also closes the path, so it implies [Closed].
const (
	CurveLinear CurveKind = iota
	CurveCardinal
	CurveCardinalOpen
	CurveCardinalClosed
	CurveMonotone
	CurveNatural
	CurveBasis
	CurveBasisOpen
	CurveBasisClosed
	CurveBundle
)

// Smoothed reports whether the family draws anything but straight segments.
func (c CurveKind) Smoothed() bool { return c != CurveLinear }

// Interpolating reports whether the family passes through every vertex. It is
// false for [CurveBasis] and its variants and for [CurveBundle]; see [Curve].
func (c CurveKind) Interpolating() bool {
	switch c {
	case CurveBasis, CurveBasisOpen, CurveBasisClosed, CurveBundle:
		return false
	}
	return true
}

// closes reports whether the family wraps its neighbours, which also closes
// the path it draws.
func (c CurveKind) closes() bool {
	return c == CurveCardinalClosed || c == CurveBasisClosed
}

// Curve picks the path drawn between the vertices of a [Line], an [Area] or a
// [Trend]. The default is [CurveLinear].
func Curve(k CurveKind) Option {
	return func(c *config) { c.curve, c.curveSet = k, true }
}

// OnMissing sets the NaN/Inf policy.
func OnMissing(m Missing) Option { return func(c *config) { c.missing = m } }

// Label names the series in the legend. It defaults to the Y column's name.
func Label(s string) Option { return func(c *config) { c.label = s } }

// KeyBy names the column that identifies a row across renders, and across
// charts.
//
// One mark in this package reads it — [Depends] joins its two tables by it —
// and for every other one it changes nothing drawn: the layer draws exactly
// what it drew before, and the column need not be one the mark plots, because
// it is an answer to a question asked from outside: *which row is this,
// still*.
//
// It exists because a row number is not an identity. [Rows] reports the row
// behind a mark, and that row is an index into the table as it stands for that
// frame; a table appended to, filtered or windowed renumbers its rows, and a
// row number carried across two frames of a stream names two different
// measurements. A key is a value the data already carries, so it survives
// whatever happens to the ordering.
//
// Three things want one. A tooltip in one chart that highlights a flow in
// another needs a name for the thing under the pointer that the other chart
// also knows — see [github.com/timzifer/figure.Event.Key]. And a transition
// between two states of a table matches their rows by it, because "the same
// bar, moved" and "one bar gone and another arrived" are different pictures
// and only the data can say which this is. And a schedule's constraints name
// the tasks they join by it, which is the one case where leaving it out is an
// error rather than a missed opportunity — see [ErrNoKey].
//
// The values are read with [github.com/timzifer/figure/data.Label], so a
// numeric key is spelled the way a facet panel key and a categorical tick are
// spelled. Keys are not checked for uniqueness: a caller who names a column
// with duplicates gets duplicates, and [Rows] and the panel and layer indices
// are still there beside it.
func KeyBy(col string) Option { return func(c *config) { c.keyCol = col } }

// Shape sets the marker shape for scatter geoms. Setting it explicitly opts
// the layer out of a theme's redundant-encoding ladder — see
// [github.com/timzifer/figure/theme.Redundant].
func Shape(m ir.Marker) Option {
	return func(c *config) { c.marker, c.markerSet = m, true }
}

// Hatch sets the pattern laid over this layer's filled marks.
//
// It is the filled mark's [Dash]: the channel a bar, a slice, an area or a
// band is told apart by when colour is not available, which is in greyscale,
// on a photocopy, and to a reader who cannot separate two palette entries.
// Setting it explicitly opts the layer out of a theme's redundant-encoding
// ladder — see [github.com/timzifer/figure/theme.Redundant].
//
// The pattern is drawn in the layer's own colour, clipped to the mark, and
// phased on the origin so that neighbouring marks continue one rhythm. It has
// no effect on a mark with no interior: a line takes a [Dash] instead.
//
//	geom.Bar(src, geom.GroupBy("region"), geom.Hatch(ir.HatchDiagonal))
func Hatch(h ir.Hatch) Option {
	return func(c *config) { c.hatch, c.hatchSet = h, true }
}

// HatchDensity scales the period of this layer's hatch: 2 is half the ink,
// 0.5 is twice it, and the default 1 is the theme's own spacing.
//
// It is the ordinal half of the hatch channel, and it is a separate option
// from [Hatch] because the two readings are separate. Changing the pattern
// says *different*; changing the density says *more*. A layer that wants the
// second reading over a whole chart asks its theme for
// [github.com/timzifer/figure/theme.DensitySeriesHatches] instead.
func HatchDensity(f float64) Option {
	return func(c *config) { c.density = f }
}

// HatchColor draws this layer's hatch in col rather than in the ink the theme
// mixes from the mark's own colour.
//
// The default is the mix, and it is the right default: a hatch is normally the
// same series saying which series it is, so it belongs to the mark's colour
// and only leans toward the theme's ink far enough to stay legible against a
// solid fill of it. Setting a colour here breaks that tie on purpose, for the
// case where the pattern is not a second reading of the same thing but a
// category of its own — a status band where the fill says *what* and the hatch
// says *why*, and the why has a colour the reader already knows from
// somewhere else on the screen.
//
// It is the caller's job to keep the result legible: figure will draw a black
// hatch on a black fill if asked.
//
//	geom.Rect(src, geom.Hatch(ir.HatchDiagonal), geom.HatchColor(green))
func HatchColor(col ir.Color) Option {
	return func(c *config) { c.ink, c.inkSet = col, true }
}

// HatchWidth scales the stroke width of this layer's hatch: 2 is twice the
// theme's, 0.5 half it, and the default 1 is the theme's own.
//
// It is [HatchDensity]'s other half. Density moves the lines apart, width
// makes each one heavier, and the two are separate because a thick line spaced
// wide and a thin line spaced close carry the same amount of ink but do not
// read the same: the first is one bold pattern, the second is texture. For a
// filled pattern — dots, triangles, scales — it scales the size of one element
// instead, which is the same reading.
func HatchWidth(f float64) Option {
	return func(c *config) { c.hatchWidth = f }
}

// Gradient makes this layer's fill a linear ramp from its own colour to col.
//
// The ramp runs along the marks' own extent — up a bar, up an area — so a bar
// chart fades toward its tips and an area chart fades toward its baseline,
// which is the second-commonest thing a fill is asked to do after being one
// colour. A transparent col is how an area is faded out rather than tinted.
//
// It is one fill rather than one per mark: a layer draws its marks into a
// single path, and the ramp spans that path. Two bars of different heights
// therefore share one ramp and are shaded consistently, rather than each
// running the whole ramp over its own height and so all ending the same
// colour — which would be a gradient that says nothing.
func Gradient(col ir.Color) Option {
	return func(c *config) { c.gradient = &col }
}

// Corner rounds the corners of this layer's rectangular marks to radius r in
// device units, clamped to half the shorter side.
//
// It reaches the marks that are rectangles — a bar, a histogram bin, a tile, a
// gantt task, a box — and nothing else. Under a polar coordinate system a
// "rectangle" is an annular sector whose corners are arcs, so there is nothing
// to round and the option is ignored: a pie slice with rounded corners would
// have to be a different mark, not the same one drawn differently.
func Corner(r float32) Option { return func(c *config) { c.corner = r } }

// Inset draws an inner border of width w just inside each filled mark, in the
// layer's own colour.
//
// It is what separates marks that touch — the segments of a stack, the boxes
// of a treemap, the bands of a sankey — without thickening the silhouette of
// the layer as a whole, which is what an ordinary outline does. The border is
// drawn inside the shape, so a mark keeps exactly the area the data gave it.
func Inset(w float32) Option { return func(c *config) { c.inset = w } }

// Size sets the marker diameter in device units.
func Size(s float32) Option { return func(c *config) { c.size = s } }

// BarWidth sets bar width as a fraction of the spacing between adjacent bars,
// in (0, 1]. The default is 0.8.
func BarWidth(f float64) Option { return func(c *config) { c.barWidth = f } }

// Baseline sets the value bars and areas grow from. The default is 0.
func Baseline(v float64) Option { return func(c *config) { c.baseline = v } }

// Y2 selects a second Y column, turning an area into a band between the two
// series rather than between one series and a baseline. It is how a confidence
// interval or a min/max envelope is drawn, and how a [Rect] is given a
// per-row baseline.
func Y2(col string) Option { return func(c *config) { c.y2col = col } }

// X2 selects a second X column, giving a [Rect] or a [Bar] its far edge on
// that axis: a gantt bar runs from a start column to an end column, a candle
// from open to close.
//
// A mark with no X2 spans its slot instead — a band scale's own bandwidth, or
// the closest spacing in the data narrowed by [BarWidth] — which is what a
// heatmap wants and what makes the cell the size of the category.
//
// Under [github.com/timzifer/figure/coord.Donut] the X axis is the radius, so
// the pair is a slice's inner and outer radius and both are dimensions of the
// data rather than a constant the coord chose:
//
//	geom.Bar(src, geom.X("floor"), geom.X2("reach"), geom.Y("share"),
//	    geom.GroupBy("browser"))
//
// The angular extent is still the stacked Y value, so such a layer reads three
// columns as three dimensions: how far round the slice goes, where it starts
// and where it stops.
//
// [Dodge] still divides the span between the layer's series, whether the span
// came from a column or from the slot: dodging is about sharing a mark's width,
// and a row that named its own edges named the width to share.
func X2(col string) Option { return func(c *config) { c.x2col = col } }

// From and To name the two ends of an edge, and [Value] what flows along it.
// The three are the whole encoding of a [Sankey] and an [Arc]: one row per
// edge, two names and a number.
//
//	geom.Sankey(src, geom.From("stage"), geom.To("next"), geom.Value("units"))
//
// The names are read as they are written — [github.com/timzifer/figure/data.Labels]
// spells a numeric or temporal column the same way a categorical axis does —
// and a node is created the first time a row mentions it. That first mention is
// what fixes the order of everything downstream: the columns a sankey stacks,
// the arcs a chord diagram goes round in, and which colour of the palette each
// node takes. It is the order of the table and never the order of a map, so a
// chart whose panels are built on separate goroutines draws what a serial one
// drew — see docs/adr/0012-parallel-panels.md.
//
// An edge naming a node only on one side still creates it: a stage nothing
// leaves is a sink, and dropping it would lose the end of the flow.
func From(col string) Option { return func(c *config) { c.fromCol = col } }

// To is [From]'s other end.
func To(col string) Option { return func(c *config) { c.toCol = col } }

// ProgressBy names a column holding how much of each cell is finished, as a
// fraction in [0, 1].
//
// It is what turns a [Rect] into a gantt bar that reads as a status rather
// than as a plan: the cell is drawn twice, the whole span faded and the
// finished part of it solid, so the bar still says when the task runs and now
// also says how far it has got.
//
//	geom.Rect(tasks, geom.X("start"), geom.X2("end"), geom.Y("task"),
//	    geom.ProgressBy("done"))
//
// The fraction grows from the edge the row named *first* — from [X] towards
// [X2], or from [Y] towards [Y2] — so a reversed axis fills from the same end
// of the task rather than from the same side of the screen. A row naming
// neither pair fills its slot along the horizontal axis; a row naming both is
// [ErrProgressAxis], because which of the two the fraction runs along is not
// something to guess at. A value outside [0, 1] is clamped, and a missing one
// is a cell whose progress is unknown, drawn solid.
func ProgressBy(col string) Option { return func(c *config) { c.progCol = col } }

// Link sets which edges of two spans a [Depends] layer joins, for every row of
// its link table. The default is [FinishToStart], which is what a dependency
// means when nobody said otherwise.
//
// [LinkBy] names a column holding it per row, and wins where both are given.
func Link(k Linkage) Option { return func(c *config) { c.linkage = k } }

// LinkBy names a column of a [Depends] layer's link table holding each row's
// [Linkage] by name — "fs", "ss", "ff" or "sf", spelled as [Linkage.String]
// spells it. A row naming nothing this package knows takes the layer's own
// [Link], which is how a document from a later version draws a chart that is
// wrong where the reader can see it rather than not at all.
func LinkBy(col string) Option { return func(c *config) { c.linkCol = col } }

// ID and Parent name a hierarchy: one row per node, the name it is known by and
// the name of the node above it. With [Value] they are the whole encoding of a
// [Treemap] and an [Icicle].
//
//	geom.Treemap(src, geom.ID("path"), geom.Parent("under"), geom.Value("bytes"))
//
// A row whose parent is empty, or names a node no row declares, is a root.
// There may be several, and they divide the whole between them.
//
// They are spelled apart from [From] and [To] on purpose, although a hierarchy
// is an edge table too: a hierarchy's edge runs from the child to its parent
// and a flow's runs from source to target, so a document that called both
// "from" and "to" would read a treemap as a flow. The columns are the same
// shape; what they mean is not.
func ID(col string) Option { return func(c *config) { c.idCol = col } }

// Parent is [ID]'s other end: the node this row hangs under.
func Parent(col string) Option { return func(c *config) { c.parentCol = col } }

// Value names the column carrying the magnitude of an edge or of a node: the
// units flowing along a link, the bytes in a directory, the weight of a chord.
//
// For a hierarchy it is usually the leaves that carry a number and the internal
// nodes that carry none, because an internal node's size is what is under it.
// A row that carries one anyway keeps it, and its share shows as the part of
// its box its children do not cover — which is how an unaccounted-for remainder
// becomes visible rather than invisible.
func Value(col string) Option { return func(c *config) { c.valCol = col } }

// Padding is the gap left between adjacent shapes of a layout that places its
// own: the space between a treemap's cells, between a sunburst's rings, between
// a sankey's nodes and between a chord diagram's arcs. It is a fraction of the
// plot, in [0, 1), and zero — the default — means the shapes touch.
//
// It is a gap rather than a stroke on purpose. A border drawn round a cell is
// ink a reader has to discount from the area they are being asked to compare,
// and under hit-testing a stroked outline sits *above* the shape it outlines,
// so a pointer on the border reports the border — see
// docs/adr/0015-hit-testing.md.
func Padding(f float64) Option { return func(c *config) { c.padding = f } }

// Thickness is how much of its slot a node fills, in (0, 1]: the width of a
// sankey's columns, the depth of a chord diagram's ring of arcs, and the width
// of a [github.com/timzifer/figure/three.Ribbon] in scene units. Zero — the
// default — means the mark's own, which differs between them because a sankey's
// node is a landmark, a chord's is a rim, and a ribbon's width is only there so
// the path can be seen.
//
// It is not [BarWidth], although it means something close, because BarWidth's
// default is 0.8 rather than zero: a mark could not tell a caller who asked for
// 0.8 from one who asked for nothing, and a default that cannot be told from a
// choice is the trap [Dash] and [Stack] each carry a companion flag to avoid.
func Thickness(f float64) Option { return func(c *config) { c.thickness = f } }

// Orientation is which way round a mark reads its two axes: which one carries
// the thing being measured, and which one carries what it is measured against.
//
// It is the library-wide question [Tree] asked first — ADR 0053's amendment —
// and the answer lands here rather than in one mark's own option, so that the
// second mark to want it says it the same way. [Tree], [Bar], [Histogram] and
// [Boxplot] read it; [Orient] on any other is accepted and ignored, which is
// what [Decimate] on a projected surface already is. [Rect] and [ErrorBar]
// need no option: which edges a row names already says which way they run.
type Orientation uint8

const (
	// Vertical is the default: the measurement runs up the Y axis and what it
	// is measured against runs across the X one. A dendrogram drawn this way
	// lines up with a heatmap's columns in a top track.
	Vertical Orientation = iota
	// Horizontal turns the pair over: the measurement runs across the X axis
	// and what it is measured against runs up the Y one. A dendrogram drawn
	// this way lines up with a heatmap's rows in a left track.
	Horizontal
)

// Orient sets which way round a mark reads its two axes. The default is
// [Vertical]. See [Orientation] for which marks read it.
func Orient(o Orientation) Option { return func(c *config) { c.orient = o } }

// axes splits a panel's two scales into the one a mark's slots run along and
// the one its measurement does: X and Y as the mark has always drawn them, and
// the other way round under [Horizontal].
func (c config) axes(x, y scale.Scale) (slot, value scale.Scale) {
	if c.orient == Horizontal {
		return y, x
	}
	return x, y
}

// roles is the config with its positional columns renamed for the roles they
// play, so that a mark written for the vertical case reads a horizontal layer
// unchanged: under [Horizontal] the slot is the [Y] column and its far edge
// [Y2], and the measurement is the [X] column and its far end [X2]. It is used
// only to read columns; the layer's own config is what it describes itself
// with, because the document holds the columns the caller named.
func (c config) roles() config {
	if c.orient == Horizontal {
		c.xcol, c.ycol = c.ycol, c.xcol
		c.x2col, c.y2col = c.y2col, c.x2col
	}
	return c
}

// box is the rectangle a mark computed in slot and value space, in the space
// the scales map into: the slot across X and the value up Y, or turned a
// quarter under [Horizontal]. It is the one place a mark written for the
// vertical case turns, which is why everything after it — the coord, explode,
// extrude — needs to know nothing about orientation.
func (c config) box(s0, v0, s1, v1 float32) ir.Rect {
	if c.orient == Horizontal {
		return ir.R(v0, s0, v1, s1)
	}
	return ir.R(s0, v0, s1, v1)
}

// point is [config.box] for a single position.
func (c config) point(s, v float32) (x, y float32) {
	if c.orient == Horizontal {
		return v, s
	}
	return s, v
}

// Opacity scales the fill alpha, in [0, 1]. The default is 1 for an explicit
// [Fill] colour and 0.25 for an area that takes its colour from the palette —
// a filled band has to sit behind the lines it belongs to.
func Opacity(f float64) Option { return func(c *config) { c.opacity = f } }

// StepPos is where a step geom changes value between two rows.
type StepPos uint8

// The step positions. StepPost is the default: the value holds from each row
// until the next one, which is what a sampled signal or a state over time
// actually did.
const (
	StepPost StepPos = iota
	StepPre
	StepMid
)

// Steps sets where a [Step] geom changes value.
func Steps(where StepPos) Option { return func(c *config) { c.steps = where } }

// Closed joins the last mark of a connected layer back to the first.
//
// It is what turns a [Line] over an angular axis into a radar contour and an
// [Area] over one into a filled one: five axes drawn as an open line leave a
// gap between the last and the first, which is a hole in a shape that has none.
// It applies to [Line], [Area] and [Step]; a layer whose marks are not
// connected ignores it.
//
// It is an option rather than something [coord.Polar] decides, because whether
// a series wraps is a fact about the series and not about the transform: a
// radar covers every axis and closes, and a polar time series spiralling
// through three revolutions does not.
func Closed(on bool) Option { return func(c *config) { c.closed = on } }

// ColorBy maps a column through a colour scale, giving every mark its own
// colour. It applies to [Scatter], [Bar] and [Rect]; geoms whose mark is one
// connected shape ignore it.
//
// Which guide the layer then contributes follows from the kind of scale it was
// handed. A continuous scale — [scale.Sequential], [scale.Diverging] — reads a
// numeric column and contributes a colourbar, because a single swatch cannot
// represent a continuum; see [ColorGuide]. A discrete one —
// [scale.Qualitative] — reads a column of categories and contributes one legend
// entry per category; see [Legender].
//
// The same scale also colours the series of a grouped layer, where the group
// label is the category. Sharing one across the layers of a chart, or across
// the panels of a facet, is what makes one colour mean one thing everywhere.
func ColorBy(col string, s scale.ColorScale) Option {
	return func(c *config) { c.colorCol, c.colorScale = col, s }
}

// TextBy names the column a [Text] layer reads its label from, one label per
// row. Any column will do: a text column is used as it is, and a numeric or
// temporal one is formatted the way a category name is, so an axis tick and a
// label for the same value are the same string.
//
// It is the channel that makes text data rather than annotation. [Note] places
// one literal string at one literal position; a layer that labels its rows
// reads them from the same [data.Source] every other mark reads, and needs no
// rebuild when the rows change.
func TextBy(col string) Option { return func(c *config) { c.textCol = col } }

// Elide lets a [Text] layer truncate a label that does not fit the box its row
// spans, ending it with an ellipsis.
//
// Without it the label is dropped instead, which is the default because a
// truncated label is a claim about a row that the row does not quite make: a
// column of "Wareneingang", "Warenausgang" and "Wartung" elides to three
// labels that are hard to tell apart, and no label at all is honest about it.
// Turn it on where the first few characters are enough to identify the row.
//
// It does nothing in point mode, where there is no box to overrun.
func Elide(on bool) Option { return func(c *config) { c.elide = on } }

// AvoidOverlap places Text labels in source order, trying the original anchor
// and eight nearby positions before dropping a label that still collides.
// Labels stay inside the panel rectangle. Box labels are never moved, only
// omitted. Only participating text layers avoid each other; marks and axis
// furniture are not obstacles. The default is false.
func AvoidOverlap(on bool) Option { return func(c *config) { c.avoidLabels = on } }

// MinFontSize is the smallest size a [Text] layer shrinks a label to before it
// gives up on the label's box, in device units.
//
// A box label that does not fit its box at the layer's size is drawn smaller,
// as large as still fits, but never below this. Below it a label stops being
// read and starts being squinted at, so what happens next is [Callout]'s, then
// [Elide]'s, and otherwise the label is dropped. The default is three quarters
// of the layer's size; a size at or above the layer's own turns shrinking off.
//
// It does nothing in point mode, where there is no box to fit.
func MinFontSize(pt float64) Option { return func(c *config) { c.minFont = pt } }

// Slide lets a [Text] layer move a label along its box to where it fits, when
// it does not fit the box's middle. It is on by default.
//
// Under a coord with a middle a box is not the same shape all the way along —
// a slice is wider at its rim, a long cell of a sunburst runs level in one
// place and diagonally in another — so a label that has no room across the
// middle may have room further along. The price is that the label no longer
// sits where the eye looks for it, and on a chart of many thin cells a label
// moved to the end of its cell can read as its neighbour's. Slide(false) keeps
// every label in the middle of its box: one that does not fit there, broken or
// shrunk, is called out with a leader where [Callout] allows it and dropped
// where it does not.
//
// Under Cartesian a box is the same shape all the way along and a label is
// never moved, so it changes nothing there.
func Slide(on bool) Option { return func(c *config) { c.pinned = !on } }

// Wrap lets a [Text] layer break a label that does not fit its box on one
// line over two or three, at spaces.
//
// The break chosen is the one whose widest line is narrowest, which is the
// most compact block the words make, and it is tried before the label is
// shrunk: "Wareneingang Halle 3" on two lines at the layer's size reads better
// than on one line at three quarters of it. Where neither fits, the label is
// shrunk as a block or as a line, whichever holds the larger type, and only
// then is it called out, cut or dropped. A label without a space is one line
// whatever this says.
//
// It does nothing in point mode, where there is no box to break a label for.
// A label called out of its box ([Callout]) is broken the same way when the
// room beside the chart does not hold it on one line.
func Wrap(on bool) Option { return func(c *config) { c.wrap = on } }

// Callout lets a [Text] layer draw a label that does not fit its box outside
// the box instead, joined to it by a leader line.
//
// The label goes out of the middle of the chart along the box's own bisector,
// past everything the coord draws, and is written level at the end of a short
// horizontal arm; labels on one side are stacked apart so that no two of them
// share a line. It is what a thin slice of a pie or of a sunburst needs, where
// the number is the reading and there is no room for it in the slice.
//
// A label too wide for the room beside the chart is broken over lines where
// [Wrap] allows it, then shrunk down to [MinFontSize], and dropped only when
// neither makes it fit.
//
// It needs a coord with a middle to go out of — [coord.Exploder] is that
// answer — so under [coord.Cartesian] it does nothing: a bar has no outside
// that is not the neighbouring bar's inside. See docs/adr/0082.
func Callout(on bool) Option { return func(c *config) { c.callout = on } }

// SizeBy maps a column through a size scale, giving every mark its own size.
// It applies to [Scatter]; geoms whose mark has a width the axes decide ignore
// it.
//
// It is the bubble chart, and it is a channel rather than a mark for the same
// reason a pie is not a geom: what changes is which column decides a mark's
// size, not what the mark is. The layer contributes a third guide kind beside
// the legend and the colourbar — a ladder of sample marks with the values they
// stand for — because a size is a continuum a single swatch cannot represent.
// See [scale.Size] for why the mapping is by area.
//
// A sized layer draws circles rather than markers, and that is a consequence of
// the IR rather than a preference: [ir.Backend.Markers] carries one style per
// call, so a layer whose size varied per row would be one drawing call per row.
// A circle per subpath of one path is one call per colour, and it gives a
// pointer the mark it is actually inside rather than the nearest centre.
func SizeBy(col string, s scale.SizeScale) Option {
	return func(c *config) { c.sizeCol, c.sizeScale = col, s }
}

// Bins sets how many bins a [Histogram] divides its column into. The default,
// 0, lets the layer choose: the Freedman-Diaconis rule where the data has an
// interquartile range to measure, and Sturges's rule where it does not.
func Bins(n int) Option { return func(c *config) { c.bins = n } }

// BinRange pins the interval a [Histogram] covers. The default is the extent of
// the data.
//
// Pinning it is what makes two histograms comparable: bins chosen from each
// column's own extent put the same value in different places, and a reader
// comparing two panels is then comparing the axes rather than the data.
func BinRange(lo, hi float64) Option {
	return func(c *config) { c.binLo, c.binHi = lo, hi }
}

// Bands sets how many bands a [Horizon] folds its trained extent into. The
// default is [DefaultBands].
//
// It is the convenient spelling and not the comparable one: the bands are then
// as tall as the data happened to reach, so two charts of the same quantity
// over different days have different bands and cannot be read against each
// other. [BandHeight] is the spelling for anything measured, and it wins where
// both are set.
func Bands(n int) Option { return func(c *config) { c.bands = n } }

// BandHeight sets the height of one band of a [Horizon] in the data's own
// units, and how many bands there are then follows from the data.
//
// It is the spelling to prefer. A band that means 50 kW is a band a plant
// engineer already has, and it means the same thing in every chart drawn this
// way — which is the whole point of the form, and what a count of bands chosen
// from each day's own maximum gives up. It overrides [Bands].
func BandHeight(h float64) Option { return func(c *config) { c.bandHeight = h } }

// Bandwidth sets the kernel width a [Violin] or a [Ridgeline] estimates its
// density with, in the data's own units. The default, 0, lets
// [github.com/timzifer/figure/stat.Silverman] choose it from each group's own
// spread.
//
// It is the one number that decides what a density looks like, so pinning it is
// how several groups are made comparable: a bandwidth chosen per group means
// each group is smoothed by a different amount, and a difference in shape can
// then be a difference in sample size rather than in distribution.
func Bandwidth(bw float64) Option { return func(c *config) { c.bandwidth = bw } }

// Span sets the fraction of the rows one local fit of a [Trend] sees, in
// (0, 1]. The default is stat.DefaultSpan for [Loess] and the narrower
// stat.DefaultWindow for the two window fits, whose window is this fraction of
// the rows rounded to a count. It has no effect on a straight fit, which sees
// all of them.
func Span(f float64) Option {
	return func(c *config) {
		if f > 0 {
			c.span = f
		}
	}
}

// Smoothing is how a [Trend] fits its line.
type Smoothing uint8

// The smoothings. Loess is the default: a trend line's job is to show what the
// data is doing, and a straight line shows what a straight line would do.
const (
	// Loess fits a locally weighted line through the neighbours of each
	// abscissa. See stat.Loess.
	Loess Smoothing = iota

	// LinearFit is ordinary least squares over the whole column: one straight
	// line, which is the right answer when the claim being made is that the
	// relationship *is* linear.
	LinearFit

	// MovingAverage is a centred running mean, [Span] wide. It is the fit a
	// reader can check by hand, and the one that flattens a peak. See
	// stat.MovingAverage.
	MovingAverage

	// SavitzkyGolay fits a quadratic to the rows around each one. It keeps the
	// height and width of a peak, which is what a running mean over the same
	// window takes away. See stat.SavitzkyGolay.
	SavitzkyGolay
)

// Smooth sets how a [Trend] fits.
func Smooth(m Smoothing) Option { return func(c *config) { c.smooth = m } }

// Overlap sets how far a [Ridgeline]'s tallest ridge rises, in slots of its
// categorical axis. The default is 1.6.
//
// Overlapping is the point of the chart rather than a defect of it: the ridges
// are read against each other, and separating them into a grid of little
// densities is the small multiple this chart exists to compress. Values below 1
// keep each ridge inside its own slot.
func Overlap(f float64) Option {
	return func(c *config) {
		if f > 0 {
			c.overlap = f
		}
	}
}

// defaultOverlap is how far a ridge rises by default: a little over one and a
// half slots, which is enough that the ridges interleave and read as one
// distribution changing, and little enough that a tall one does not reach the
// label of the row two above it.
const defaultOverlap = 1.6

// Whisker sets how far a boxplot whisker reaches, as a multiple of the
// interquartile range. The default is 1.5, Tukey's original choice.
func Whisker(k float64) Option { return func(c *config) { c.whisker = k } }

// Outliers turns the individual points beyond the whiskers on or off. They are
// on by default: a boxplot that hides them is a boxplot that hides exactly the
// rows a reader opened the chart to find.
func Outliers(show bool) Option { return func(c *config) { c.outliers = show } }

// OnY2 binds this layer to the chart's secondary vertical axis, the one
// [github.com/timzifer/figure.Plot.Y2] sets, instead of to its primary one.
//
// It is the option a chart of two quantities in different units needs —
// revenue as bars against a left axis, margin as a percentage line against a
// right one — and it is on the *layer* because that is where the binding is: a
// scale does not know which marks read it, and a chart with two Y axes is one
// chart with two of them rather than two charts overlaid.
//
// A layer that asks for it in a chart with no secondary axis draws against the
// primary one, silently, for the reason [Explode] is silent under a Cartesian
// coord: an option every mark accepts must not make a chart's validity depend
// on something set somewhere else.
func OnY2() Option { return func(c *config) { c.onY2 = true } }

// OnX2 is [OnY2] turned a quarter turn: it binds this layer to the chart's
// secondary *horizontal* axis, the one [github.com/timzifer/figure.Plot.X2]
// sets, drawn along the top of the panel.
//
// It is the option two series measured over different extents of the same
// thing need — a run indexed by cycle against one indexed by elapsed time, or
// a spectrum read in wavelength against the same spectrum in wavenumber. The
// two are independent: a layer may name both, and then it reads the top axis
// and the right one.
func OnX2() Option { return func(c *config) { c.onX2 = true } }

// Mid selects the column an [ErrorBar] marks the measurement at, inside the
// interval it draws.
//
// It is the third number an interval needs and the reason this mark has a
// channel of its own: [Y] and [Y2] are the two ends, exactly as they are for
// the band an [Area] draws and the box a [Rect] draws, which leaves the
// measurement itself nowhere to go. A layer that names no Mid draws the
// interval alone, which is the honest picture when the interval is all that
// was measured — a min and a max are not evidence of a mean.
//
// The symmetric spelling needs no Mid: [ErrorBy] reads the measurement from
// [Y] and derives both ends from it, so the value is already named.
func Mid(col string) Option { return func(c *config) { c.midCol = col } }

// ErrorBy selects a column of half-widths, and makes an [ErrorBar] symmetric
// about its [Y] value: the interval runs from y−e to y+e.
//
// It is the spelling a table usually has. A mean and a standard deviation, or
// a mean and a margin of error, are two columns; turning them into a low and a
// high column first is arithmetic the caller should not have to do to draw a
// chart of what they measured.
//
// The measurement is marked, because with this spelling there always is one:
// the centre is the [Y] column, so a layer given a spread is a point with an
// interval around it rather than an interval alone.
func ErrorBy(col string) Option { return func(c *config) { c.errCol = col } }

// ErrorXBy is [ErrorBy] along the horizontal axis: the interval runs from x−e
// to x+e about the [X] value, and the mark lies on its side.
//
// Which axis an error bar runs along follows from the encoding and nothing
// else, exactly as a [Rect]'s edges do: naming [Y2] or [ErrorBy] puts the
// interval on the vertical axis, naming [X2] or this puts it on the
// horizontal one. That is what a chart of measurements against categories on
// the Y axis needs, and it is why there is no orientation option.
func ErrorXBy(col string) Option { return func(c *config) { c.errXCol = col } }

// Caps turns the crossbars at the ends of an [ErrorBar] on or off. They are on
// by default.
//
// Turning them off is the point-range look: a rule with a marker on it and
// nothing at the ends, which is what a chart with many intervals close
// together wants — caps that touch read as a grid. The cap is half as wide as
// a [Bar] of the same [BarWidth] would be, so an error bar drawn over a bar
// chart is narrower than the bar it annotates.
func Caps(show bool) Option { return func(c *config) { c.caps = show } }

// Align sets how a text annotation sits about its position. The default is
// the run's start on the point, on the baseline — except in a [Text] layer
// that labels a box, where a label with nothing said about it is centred in
// the box rather than hung off its left edge.
//
// That is why the layer records having been told: the start of the run and the
// baseline are both the zero value and an alignment somebody may have asked
// for, exactly as a circle is for [Shape].
func Align(h ir.HAlign, v ir.VAlign) Option {
	return func(c *config) { c.halign, c.valign, c.alignSet = h, v, true }
}

// FontSize sets the type size of a text annotation in device units. The
// default is the theme's label size.
func FontSize(pt float64) Option { return func(c *config) { c.fontSize = pt } }

// Rotate turns a text annotation about its anchor, in radians clockwise.
func Rotate(radians float64) Option { return func(c *config) { c.rotation = radians } }

// Extend controls whether an annotation widens the axis domain to include
// itself. It is on by default: a threshold line the chart does not reach is
// still worth seeing, because "we are nowhere near the limit" is the answer
// the reader came for. Turn it off for an annotation that should appear only
// when the data reaches it.
//
// [Locus] is the one mark whose default is the other way round, and it reads
// this option rather than the default — a curve given by a formula says what
// the region of the plane means and does not ask the axes to go and find it.
func Extend(on bool) Option {
	return func(c *config) { c.extend, c.extendSet = on, true }
}

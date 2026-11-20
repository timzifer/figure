// Package geom holds the visual marks a chart is made of.
//
// A geom knows what its shape is and never how it is rendered: it reads
// columns, asks the scales where values go, and emits IR. It has no reference
// to a backend, no knowledge of SVG or raster, and no opinion about layout
// beyond the rectangle it is given.
//
// Every mark is built the same way — a constructor taking the table and a list
// of [Option] values — and every constructor takes every option. An option a
// mark has no use for is accepted and changes nothing, which keeps the options
// one namespace for every mark, including one defined outside this package
// through [Register] and [Configure]. Which options a mark does read is the
// table at the end of this page.
//
// # Marks
//
// The basic marks:
//   - [Line], [Scatter], [Bar], [Area], [Step], [Rect], [Text]
//
// Distributions and fitted statistics:
//   - [Histogram], [Violin], [Ridgeline], [Boxplot], [Beeswarm], [ECDF],
//     [QQ], [Trend], [Hexbin], [ErrorBar], [Survival]
//
// Fields, surfaces and curves given by a formula:
//   - [Contour], [Raster], [Horizon], [Voronoi], [Locus]
//
// Relational layouts — a hierarchy, a flow, a graph, or rows across several
// axes:
//   - [Treemap], [Icicle], [Sankey], [Arc], [Tree], [Graph], [NodeLink],
//     [Parallel], [ParallelSets]
//
// Sets, counted:
//   - [Intersections], [SetMatrix], [SetSizes], [Venn]
//
// Plans and markets:
//   - [Depends], [Candle], [LastValue]
//
// Annotations, placed at values rather than read from a table:
//   - [HLine], [VLine], [HBand], [VBand], [Segment], [Region], [Note]
//
// # Options
//
// Columns, naming what a mark reads from the table:
//   - [X], [Y], [X2], [Y2], [Mid], [From], [To], [ID], [Parent], [Value],
//     [Dims], [OHLC], [Event], [LinkBy], [ProgressBy], [ErrorBy],
//     [ErrorXBy], [UncertaintyBy], [KeyBy], [WidthBy], [ExplodeBy],
//     [DirectionBy], [TextBy], [OnX2], [OnY2]
//
// Colour, size and series:
//   - [Color], [ColorBy], [SizeBy], [GroupBy], [Label], [Guide], [Stack],
//     [Order], [Dodge]
//
// The stroke and the marker:
//   - [Width], [Dash], [Shape], [Size]
//
// The fill:
//   - [Fill], [Opacity], [Hatch], [HatchColor], [HatchDensity],
//     [HatchWidth], [Gradient], [Corner], [Inset]
//
// Rows, missing values and decimation:
//   - [OnMissing], [Decimate], [Budget], [DensityCells]
//
// Lines and curves:
//   - [Curve], [Tension], [Steps], [Closed]
//
// Text:
//   - [Align], [FontSize], [Rotate], [Elide], [AvoidOverlap], [MinFontSize],
//     [Slide], [Wrap], [Callout]
//
// Bars, boxes and intervals:
//   - [BarWidth], [Baseline], [Orient], [Whisker], [Outliers], [Caps],
//     [Explode]
//
// Depth, for a chart drawn in three dimensions:
//   - [Z], [Extrude], [Droplines]
//
// Distributions:
//   - [Bins], [BinRange], [Bandwidth], [Overlap], [Smooth], [Span],
//     [Confidence], [CensorMarks]
//
// Fields and levels:
//   - [Levels], [LevelCount], [LevelFormat], [LabelLevels], [Bands],
//     [BandHeight], [Resample]
//
// Layouts:
//   - [Padding], [Thickness], [Branches], [Link], [Top]
//
// Markets:
//   - [Direction], [Rising], [Falling], [Hollow], [CandleStyle], [Rule]
//
// Annotations:
//   - [Extend]
//
// A mark defined outside this package:
//   - [Extra]
//
// # Which options a mark reads
//
// The columns, and [Color], [ColorBy], [SizeBy], [GroupBy], [Label] and
// [Guide], are described with each mark's constructor. The table lists the
// rest: the options each mark reads as a property of itself, as the JSON spec
// writes them on its mark object. An option a mark is not listed against is
// accepted and ignored.
//
// An option counts as read when it changes the picture, the legend included:
// a violin rounds no corner of its own, but its swatch does.
//
// The table is generated; TestTheOptionMatrixIsCurrent fails when it is stale,
// and rewrites it when run with -update.
//
//	Arc            Baseline Corner Fill Gradient Hatch HatchColor
//	               HatchDensity HatchWidth Inset Opacity Padding Thickness
//	Area           Baseline Budget Closed Corner Curve Dash Decimate Dodge
//	               Fill Gradient Hatch HatchColor HatchDensity HatchWidth
//	               Inset OnMissing Opacity Order Tension Width
//	Bar            BarWidth Baseline Budget Corner Dash Decimate Dodge
//	               Explode Extrude Fill Gradient Hatch HatchColor
//	               HatchDensity HatchWidth Inset OnMissing Order Orient
//	               Width
//	Beeswarm       BarWidth Dash Dodge Order Shape Size Width
//	Boxplot        BarWidth Budget Corner Dash Decimate Fill Gradient
//	               Hatch HatchColor HatchDensity HatchWidth Inset
//	               OnMissing Opacity Orient Outliers Whisker Width
//	Candle         BarWidth Budget CandleStyle Corner Dash Decimate
//	               Direction Falling Gradient Hatch HatchColor
//	               HatchDensity HatchWidth Hollow Inset OnMissing Rising
//	               Width
//	Contour        Dash LabelLevels LevelCount Levels Width
//	Depends        BarWidth Dash Size Width
//	ECDF           Dash Dodge Order Width
//	ErrorBar       BarWidth Budget Caps Dash Decimate Dodge OnMissing
//	               Order Shape Size Width
//	Graph          Baseline Branches Corner Dash Fill Gradient Hatch
//	               HatchColor HatchDensity HatchWidth Inset Opacity Size
//	               Width
//	HBand          Corner Dash Extend Fill Hatch HatchColor HatchDensity
//	               HatchWidth Opacity Width
//	Hexbin         DensityCells Fill
//	Histogram      Baseline BinRange Bins Corner Dash Fill Gradient Hatch
//	               HatchColor HatchDensity HatchWidth Inset Opacity Orient
//	               Width
//	HLine          Dash Extend Width
//	Horizon        BandHeight Bands Baseline Budget Decimate OnMissing
//	Icicle         Corner Fill Gradient Hatch HatchColor HatchDensity
//	               HatchWidth Inset Opacity Padding
//	Intersections  Corner Fill Gradient Hatch HatchColor HatchDensity
//	               HatchWidth Inset Opacity Order Top
//	LastValue      Color Dash Rule Width
//	Line           Budget Closed Curve Dash Decimate Dodge OnMissing Order
//	               Tension Width
//	Locus          Dash Extend LabelLevels Levels Width
//	NodeLink       Corner Dash Fill Gradient Hatch HatchColor HatchDensity
//	               HatchWidth Inset Opacity Size Width
//	Note           Align Extend FontSize Rotate
//	Parallel       Dash Opacity Width
//	ParallelSets   Corner Fill Gradient Hatch HatchColor HatchDensity
//	               HatchWidth Inset Opacity Padding Thickness
//	QQ             Dash Dodge Fill Order Shape Size Width
//	Raster         Resample
//	Rect           BarWidth Corner Dash Explode Extrude Fill Gradient
//	               Hatch HatchColor HatchDensity HatchWidth Inset Width
//	Region         Corner Dash Extend Fill Hatch HatchColor HatchDensity
//	               HatchWidth Opacity Width
//	Ridgeline      Bandwidth Corner Dash Fill Gradient Hatch HatchColor
//	               HatchDensity HatchWidth Inset Opacity Overlap Width
//	Sankey         Corner Fill Gradient Hatch HatchColor HatchDensity
//	               HatchWidth Inset Opacity Padding Thickness
//	Scatter        Budget Dash Decimate DensityCells Dodge Fill OnMissing
//	               Opacity Order Shape Size Width
//	Segment        Dash Extend Width
//	SetMatrix      Corner Dash Hatch HatchColor HatchDensity HatchWidth
//	               Order Shape Size Top Width
//	SetSizes       Corner Fill Gradient Hatch HatchColor HatchDensity
//	               HatchWidth Inset Opacity
//	Step           Budget Closed Dash Decimate Dodge OnMissing Order Steps
//	               Width
//	Survival       CensorMarks Confidence Dash Dodge Fill Gradient Hatch
//	               HatchColor HatchDensity HatchWidth Inset Opacity Order
//	               Width
//	Text           Align AvoidOverlap Callout Elide Fill FontSize
//	               MinFontSize Rotate Slide Wrap
//	Tree           Baseline Branches Dash Opacity Orient Width
//	Treemap        Corner Fill Gradient Hatch HatchColor HatchDensity
//	               HatchWidth Inset Opacity Padding
//	Trend          Curve Dash Dodge Order Smooth Span Tension Width
//	VBand          Corner Dash Extend Fill Hatch HatchColor HatchDensity
//	               HatchWidth Opacity Width
//	Venn           Corner Dash Fill Hatch HatchColor HatchDensity
//	               HatchWidth Opacity Width
//	Violin         Bandwidth BarWidth Corner Dash Dodge Fill Gradient
//	               Hatch HatchColor HatchDensity HatchWidth Inset Opacity
//	               Width
//	VLine          Dash Extend Width
//	Voronoi        Corner Dash Fill Gradient Hatch HatchColor HatchDensity
//	               HatchWidth Inset Opacity Width
package geom

import (
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// Missing is the policy for NaN and infinite values in a column.
type Missing uint8

// The missing-data policies. Gap is the default: a hole in the data should
// look like a hole, not like a straight line someone might read as real.
const (
	Gap Missing = iota
	Interpolate
	Error
)

// Frame is everything a geom needs to turn data into IR.
type Frame struct {
	// Area is the plot rectangle in device space. Scales already map into it.
	Area ir.Rect
	// X and Y are the trained, ranged scales.
	X, Y scale.Scale
	// Coord decides what the interval a scale maps into means, and therefore
	// where a mapped pair lands. A nil Coord is [coord.Cartesian], so a Frame
	// built by code that never heard of coordinate systems draws the chart it
	// always drew — reach for [Frame.Coords] rather than the field.
	Coord coord.Coord
	// Theme supplies defaults the geom did not override.
	Theme theme.Theme
	// Index is the geom's position among the chart's layers, used to pick a
	// default colour from the palette.
	Index int

	// Rows, when non-nil, collects which source row is behind each mark the
	// geom draws. It is nil for an ordinary render and a geom must check it
	// before doing the bookkeeping — see [Rows] and [Frame.Marks].
	Rows Rows

	// Labels places opt-in text labels against earlier participating labels in
	// this panel. A nil placer preserves their original positions.
	Labels LabelPlacer
}

// LabelPlacer is supplied by the renderer. A geom requests a position but
// neither solves layout nor changes drawing order. move is false for a label
// inside a box: moving that label could attribute it to a neighbouring row.
type LabelPlacer interface {
	PlaceLabel(run ir.TextRun, move bool) (at ir.Point, ok bool)
}

// LabelAvoider optionally requests panel-local label placement. Geom's stable
// interface is unchanged; render allocates layout state only when requested.
type LabelAvoider interface {
	AvoidsLabels() bool
}

// Overhanger is implemented by a layer that may draw past the coord's clip and
// asks to be clipped to the panel rectangle instead: a label written outside a
// pie with a leader back to its slice is outside the disc the coord clips to,
// and inside the panel. Like [LabelAvoider] it is optional, so Geom's stable
// interface is unchanged, and a layer that does not implement it is clipped as
// it always was.
type Overhanger interface {
	Overhangs() bool
}

// Coords is the frame's coordinate system, which is [coord.Cartesian] framed
// in the plot rectangle when it has none.
//
// A geom asks for it once at the top of Build and uses what comes back: the
// answer never changes within one frame, and the nil check is not worth
// repeating per row. Framing the fallback rather than handing back a bare
// Cartesian is what makes a Frame built by hand — in a test, or by a caller
// driving a geom directly — behave as it always did: the coord it gets can
// answer [coord.Coord.Extent], which is where a rule that spans the plot finds
// the far edge.
func (f Frame) Coords() coord.Coord {
	if f.Coord == nil {
		return coord.Cartesian().Frame(coord.Framing{Area: f.Area})
	}
	return f.Coord
}

// SwatchKind is how a legend entry draws its sample.
type SwatchKind uint8

// The swatch kinds.
const (
	SwatchLine SwatchKind = iota
	SwatchMarker
	SwatchBox
)

// LegendEntry is a geom's contribution to the legend.
type LegendEntry struct {
	Label  string
	Color  ir.Color
	Kind   SwatchKind
	Marker ir.Marker
	Dash   []float32
	Width  float32

	// Hatching and Corner are what a box swatch needs to be the mark it
	// stands for. A legend whose swatch is a plain square beside a hatched bar
	// has to be translated before it can be used, which is the one thing a
	// legend exists not to require.
	Hatching ir.Hatching
	Corner   float32
}

// Training is what [Geom.Train] is handed: the scales a layer feeds its data
// into so they can establish their domains.
//
// It is a struct rather than a parameter list because a chart can gain a
// dimension — a depth scale, a second radial scale, an axis of time — and a
// struct with exported fields gains a field where a method signature cannot.
// Train is implemented outside this module, so widening it again would be a
// breaking change every time; widening this is additive. ADR 0056 is the
// record.
type Training struct {
	// X and Y are the scales for the horizontal and vertical channels. Both
	// are non-nil.
	X, Y scale.Scale

	// Z is the depth scale, and it is the field this struct was widened for.
	//
	// It is nil for every chart [github.com/timzifer/figure/render] draws,
	// because a panel has two dimensions; it is non-nil for the layers of a
	// [github.com/timzifer/figure/three.Scene]. A geom that never heard of a
	// third dimension reads X and Y and is right to — and a layer that needs
	// one is a three.Layer rather than a Geom, which is what keeps the two
	// from being confusable. ADR 0056 is the record.
	Z scale.Scale

	// Dims are the scales of a panel that has more axes than two, in the order
	// the coord draws them, and it is the second field this struct has been
	// widened by.
	//
	// It is nil for every chart with a pair of axes, which is all of them but
	// one: [github.com/timzifer/figure/coord.Parallel] gives a panel an axis
	// per dimension, and a mark drawn in it trains the dimension it reads
	// rather than the panel's Y. render fills this from the panel's coord
	// through coord.Dimensions, so a coord with two axes costs nothing and a
	// mark that never heard of a third axis reads X and Y and is right to.
	// See docs/adr/0078-a-coord-with-more-than-two-axes.md.
	Dims []scale.Scale

	// Within is the interval of X a layer's Y should be trained on, and the
	// third field this struct has been widened by. It is nil unless Y fits
	// its panel's view ([scale.FitView]) and X's domain is pinned, which is
	// the one case in which some rows are out of view before training
	// starts.
	//
	// It is a request, not a filter: render does not know which of a layer's
	// columns are positions. A layer that reads it trains Y on the rows in
	// view — for a mark with width, those whose span overlaps it; for a
	// connected one, also the value interpolated at each edge — and still
	// trains X on every row. A layer that does not read it trains Y on every
	// row, and the fitted axis is wider than the view, never narrower. See
	// docs/adr/0089-a-value-axis-fits-what-its-time-axis-shows.md.
	Within *scale.Interval
}

// Geom is a layer of marks.
//
// # Stability
//
// Geom is implemented outside this module, so it never gains a method: three
// is the whole contract. Everything else a layer can do is an optional
// interface beside it — [Faceter], [Guided], [Sized], [Legender], [Describer]
// — and a layer that implements none of them still draws. A layer defined
// outside this package reads the shared options through [Configure], takes
// its own through [Extra], and is written down and read back through
// [Describer] and [Register].
type Geom interface {
	// Train feeds the geom's data into the scales so they can establish their
	// domains. It runs before layout, because layout needs tick labels and
	// tick labels need a domain.
	Train(t Training) error

	// Build emits the geom's marks into b.
	Build(b ir.Backend, f Frame) error

	// Legend returns the entry this geom contributes, or ok == false if it
	// should not appear in the legend.
	Legend(f Frame) (LegendEntry, bool)
}

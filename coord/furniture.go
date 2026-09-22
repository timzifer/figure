package coord

import "github.com/timzifer/figure/ir"

// Shape is one piece of furniture: a straight run of points, or a path when
// the coord bends it.
//
// The two forms are not interchangeable and the difference is deliberate. A
// Cartesian grid line is two points and reaches the backend as a Polyline,
// exactly as it did before there was a coord to ask — which is why every
// golden file in the repository still matches. A polar ring is cubics and
// reaches it as a stroked path.
type Shape struct {
	// Pts is a straight run, empty when the shape is curved.
	Pts []ir.Point
	// Path is a curve, empty when the shape is straight. It is a value rather
	// than a pointer so that its buffers survive [Furniture.Reset].
	Path ir.Path
}

// Empty reports whether s holds nothing to draw.
func (s *Shape) Empty() bool { return len(s.Pts) < 2 && s.Path.Empty() }

// line makes s the straight segment from a to b, reusing its buffer.
func (s *Shape) line(a, b ir.Point) { s.Pts = append(s.Pts[:0], a, b) }

// reset empties s without giving up its memory.
func (s *Shape) reset() {
	s.Pts = s.Pts[:0]
	s.Path.Reset()
}

// Label is where one tick label sits and how it is aligned about that point.
type Label struct {
	At ir.Point
	H  ir.HAlign
	V  ir.VAlign
	// Rotation turns the label about its anchor, in radians clockwise. It is
	// zero for every label a Cartesian axis writes.
	Rotation float64
}

// Family is a grid family that belongs to the coord rather than to either
// axis: its own levels, its own label positions, and — this is the whole of
// why it exists — its own label text.
//
// A panel has two tick lists, and until this type there was no way for a coord
// to draw a third labelled ladder: geometry it could always smuggle into a
// tick's own shape as a second subpath, but label text reached render only
// from [github.com/timzifer/figure/scale.Tick.Label], so a family with no tick
// behind it had nothing to be labelled by. A ternary chart's third component
// is the first customer; a projection's graticule and a Smith chart drawn on Γ
// are the next two. See docs/adr/0070-a-third-labelled-family.md.
//
// The three slices are parallel: level i is Lines[i], labelled Text[i] at
// Labels[i]. Labels and Text may both be empty, which draws the family
// unlabelled.
type Family struct {
	// Name says what the ladder reads, for a test and for a coord that
	// describes itself. Nothing is drawn from it.
	Name string
	// Lines is one shape per level.
	Lines []Shape
	// Labels is where each level's label sits, and Text is what it says.
	Labels []Label
	Text   []string
}

// next lengthens the family by one level and hands back that level's shape,
// with whatever buffer the last frame left it, and empty.
func (f *Family) next() *Shape {
	f.Lines = growShapes(f.Lines)
	return &f.Lines[len(f.Lines)-1]
}

// label records where the level just added is labelled and what it says.
func (f *Family) label(l Label, text string) {
	f.Labels = append(f.Labels, l)
	f.Text = append(f.Text, text)
}

// reset empties the family without giving up its memory.
func (f *Family) reset() {
	f.Name = ""
	f.Lines = resetShapes(f.Lines)
	f.Labels, f.Text = f.Labels[:0], f.Text[:0]
}

// Furniture is the geometry of one panel's grid lines, axis lines, tick marks
// and tick labels. A coord fills it; render strokes it.
//
// Every per-tick slice is parallel to the tick list it came from, so index i
// is tick i whether or not that tick is drawn. That is what lets render keep
// the decisions that are its own — which grid lines the theme asked for, which
// labels would collide, whether this panel writes labels at all — while the
// coord answers only where things go.
//
// It is filled rather than returned so that a chart redrawn every frame does
// not allocate its furniture again: [Furniture.Reset] keeps every buffer.
type Furniture struct {
	// AxisX and AxisY are the two axis lines.
	AxisX, AxisY Shape

	// GridX is one shape per X tick and GridY one per Y tick, in tick order.
	// A tick with no grid line — a minor one, or one outside the panel — has
	// an empty shape.
	GridX, GridY []Shape

	// TickX and TickY are the tick marks, in tick order, empty where there is
	// none.
	TickX, TickY []Shape

	// LabelX and LabelY are where the tick labels go, in tick order.
	LabelX, LabelY []Label

	// Families are the grid families that belong to the coord rather than to
	// either axis, each with its own labels. It is empty for every coord that
	// draws its ladders from the panel's own ticks, which is all but the
	// barycentric one. See [Family].
	Families []Family

	// InX and InY report, per tick, whether the tick falls inside the panel at
	// all. A Cartesian coord culls a tick that a float32 mapping put a hair
	// outside the plot rectangle; a polar one has nothing to fall off.
	InX, InY []bool

	// XLabelsShareARow reports whether the X tick labels sit along one
	// horizontal line and can therefore run into each other. render drops the
	// ones that would overlap when they do — and must not when they do not:
	// two labels on opposite sides of a ring can share an x and still be a
	// finger apart.
	XLabelsShareARow bool

	// AxesOverData reports whether the axis lines, tick marks and tick labels
	// lie inside the region the data is drawn in, so that render has to stroke
	// them after the marks rather than before. A Cartesian axis runs along the
	// panel's edge, outside every mark, and keeps the order it always had. A
	// polar radial axis runs along a spoke through the ring — the first slice
	// of a pie and the empty end of a gauge both start exactly on it — and
	// drawn first it is painted over. The grid stays under the data either
	// way: it is a reference behind the marks, not a label on them.
	AxesOverData bool

	// FamiliesAreTheAxes reports that the families *are* this panel's axes,
	// so their labels are written wherever the panel writes labels at all —
	// even where the theme has turned the panel's own tick labels off.
	//
	// It is false for a coord whose families are a third reading beside two
	// axes that carry their own numbers: a ternary chart with its ticks turned
	// off should not come back with one component labelled and two not. It is
	// true for a coord whose two axes carry nothing a reader wants numbered,
	// where the alternative is a chart whose every axis loses its numbers to a
	// theme option that looks like it is about something else. See
	// docs/adr/0078-a-coord-with-more-than-two-axes.md.
	//
	// It does not reach past the panel: an inner panel of a facet leaves its
	// labels to the outer ones either way, which is ADR 0070's own rule and
	// the reason the gate exists at all.
	FamiliesAreTheAxes bool

	// Breaks are the marks of the gaps left in either axis, one shape with a
	// subpath per stroke. render strokes them after the data, because a bar
	// that crosses a break must not paint over the mark saying so. It is
	// empty on every axis without a break. See [Breakable].
	Breaks Shape

	// LabelsYFirst decides which axis keeps its labels where the two collide.
	// Labels that do not share a row are thinned by render against every
	// other label by their boxes, greedily in axis order — X first, unless
	// this is set. A polar coord sets it when Y is the angle: the labels round
	// the rim are the reading, and the radial ones are a scale beside it.
	LabelsYFirst bool
}

// Reset empties f while keeping every buffer it has grown, including those of
// the shapes past its current length.
func (f *Furniture) Reset() {
	f.AxisX.reset()
	f.AxisY.reset()
	f.Breaks.reset()
	f.GridX, f.GridY = resetShapes(f.GridX), resetShapes(f.GridY)
	f.TickX, f.TickY = resetShapes(f.TickX), resetShapes(f.TickY)
	f.LabelX, f.LabelY = f.LabelX[:0], f.LabelY[:0]
	f.InX, f.InY = f.InX[:0], f.InY[:0]
	f.Families = resetFamilies(f.Families)
	f.XLabelsShareARow = false
	f.AxesOverData = false
	f.FamiliesAreTheAxes = false
	f.LabelsYFirst = false
}

// resetShapes empties every shape the slice has ever held — not merely the
// ones inside its current length — and truncates it. Reaching past the length
// is the point: [Furniture.nextX] hands those shapes back out on the next
// frame, and a shape still holding last frame's points would draw them again.
func resetShapes(s []Shape) []Shape {
	full := s[:cap(s)]
	for i := range full {
		full[i].reset()
	}
	return s[:0]
}

// resetFamilies empties every family the slice has ever held — past its
// current length as well, for [resetShapes]'s reason — and truncates it.
func resetFamilies(f []Family) []Family {
	full := f[:cap(f)]
	for i := range full {
		full[i].reset()
	}
	return f[:0]
}

// family lengthens the furniture by one family, named, and hands it back to
// fill in. It comes back with whatever buffers the last frame left it.
func (f *Furniture) family(name string) *Family {
	if len(f.Families) < cap(f.Families) {
		f.Families = f.Families[:len(f.Families)+1]
	} else {
		f.Families = append(f.Families, Family{})
	}
	fam := &f.Families[len(f.Families)-1]
	fam.Name = name
	return fam
}

// side is one axis's worth of furniture. A coord fills X and Y with the same
// loop written once, which is the only reason it exists.
type side struct {
	axis  *Shape
	grid  *[]Shape
	tick  *[]Shape
	label *[]Label
	in    *[]bool
}

func (f *Furniture) x() side {
	return side{&f.AxisX, &f.GridX, &f.TickX, &f.LabelX, &f.InX}
}

func (f *Furniture) y() side {
	return side{&f.AxisY, &f.GridY, &f.TickY, &f.LabelY, &f.InY}
}

// next lengthens the side by one tick and hands back that tick's grid line and
// tick mark to fill in. Both come back with whatever buffers the last frame
// left them, and empty.
func (s side) next() (grid, tick *Shape) {
	*s.grid, *s.tick = growShapes(*s.grid), growShapes(*s.tick)
	return &(*s.grid)[len(*s.grid)-1], &(*s.tick)[len(*s.tick)-1]
}

// mark records whether the tick just added falls inside the panel, and where
// its label goes.
func (s side) mark(in bool, l Label) {
	*s.in = append(*s.in, in)
	*s.label = append(*s.label, l)
}

// growShapes lengthens s by one, reusing the shape that was there before Reset
// truncated the slice rather than appending a zero one over it.
func growShapes(s []Shape) []Shape {
	if len(s) < cap(s) {
		return s[:len(s)+1]
	}
	return append(s, Shape{})
}

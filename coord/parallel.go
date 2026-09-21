package coord

import (
	"math"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// Parallel gives a panel one vertical axis per dimension, evenly spaced across
// it, and reads a pair as (which axis, how far up it).
//
// It is the coordinate system of a parallel-coordinates plot: every row of a
// table is a line crossing every axis at its own value, and a reader looks for
// the shape the lines make — where they converge, where they cross, which of
// them run against the rest.
//
//	p := figure.New(figure.Coord(coord.Parallel(
//		coord.Dim("mpg", scale.Linear()),
//		coord.Dim("power", scale.Linear()),
//		coord.Dim("weight", scale.Linear()),
//	)))
//	p.Add(geom.Parallel(cars,
//		geom.Dims("mpg", "power", "weight"),
//		geom.ColorBy("origin", scale.Qualitative(palette.OkabeIto))))
//
// # It is the first coord that holds scales
//
// Every other coord in this package borrows the panel's two and re-ranges
// them. This one carries its dimensions, because that is what a
// parallel-coordinates panel *is*: N axes, each with a domain of its own, and
// two of them would be a Cartesian panel. [Frame] gives each dimension the
// unit interval as its range, so a mark maps a value through its own
// dimension's scale and hands the result here as a fraction of the axis; the
// panel's own X and Y are pinned and carry nothing.
//
// A dimension is a **label and a scale** and never a column name: which
// columns a chart draws is the mark's business, and a coord that knew what a
// table was would be a coord in the wrong package. The mark's columns and
// these dimensions are matched in order, and a mark that names a different
// number of them is an error rather than a chart with an unlabelled axis.
//
// # The axes are furniture
//
// Each dimension raises a [Family] — the axis line, a mark at every level, the
// level labels in that dimension's own units and the dimension's name at the
// top — which render strokes in grid ink and writes in tick ink, exactly as it
// does a ternary chart's third ladder. Nothing in render knows how many axes a
// panel has. See docs/adr/0078-a-coord-with-more-than-two-axes.md.
//
// The panel's own two axes are silenced by the coord: left to themselves they
// would number an axis index and a fraction of an axis. Turning them off in
// the theme as well — `theme.Ticks(false, false)`, beside
// `theme.Grid(false, false)` and `theme.AxisLines(false, false)` — is worth
// doing, and gives back the gutter a panel reserves for tick labels it is not
// going to write. The dimensions keep their own numbers either way, which is
// what [Furniture.FamiliesAreTheAxes] is for.
//
// # Which way up an axis reads
//
// A dimension's scale is its own, so an axis that reads better upside down
// says so where it is built — `scale.Linear(scale.Descending())` — and this
// coord needs to know nothing about it.
func Parallel(dims ...ParallelDim) Coord {
	p := &parallel{dims: append([]ParallelDim(nil), dims...)}
	return p
}

// ParallelDim is one axis of a [Parallel] coord: what it is called, and the
// scale that decides where a value sits on it.
type ParallelDim struct {
	// Name is written at the top of the axis.
	Name string
	// Scale maps this dimension's values. It is ranged onto the unit interval
	// by [Coord.Frame], and a mark reads it to place a row.
	Scale scale.Scale
}

// Dim is one dimension of a [Parallel] coord.
func Dim(name string, s scale.Scale) ParallelDim {
	return ParallelDim{Name: name, Scale: s}
}

// Dimensional is implemented by a coord whose panel has more than two axes, so
// that the marks drawn in it can find the scales.
//
// It is an optional interface beside [Coord], like [Exploder] and [Fixed]: a
// coord with two axes does not implement it and nothing asks twice. Reach it
// through [Dimensions] rather than asserting, which is what
// [github.com/timzifer/figure/render] does when it fills
// [github.com/timzifer/figure/geom.Training].
type Dimensional interface {
	// Dimensions are the coord's axes, in the order they are drawn.
	Dimensions() []ParallelDim
}

// Dimensions are c's axes beyond the panel's two, or nil for a coord that has
// none — which is every coord but [Parallel].
func Dimensions(c Coord) []ParallelDim {
	d, ok := c.(Dimensional)
	if !ok {
		return nil
	}
	return d.Dimensions()
}

// Scales are the scales of dims, in order. It is what a caller training the
// axes of a parallel panel is handed.
func Scales(dims []ParallelDim) []scale.Scale {
	if len(dims) == 0 {
		return nil
	}
	out := make([]scale.Scale, 0, len(dims))
	for _, d := range dims {
		out = append(out, d.Scale)
	}
	return out
}

// parallel is the coord and — once [parallel.Frame] has been called — the
// rectangle it was given. Frame returns a copy rather than moving the
// receiver, so two panels drawn on two goroutines never share one rectangle.
type parallel struct {
	dims []ParallelDim

	area   ir.Rect
	framed bool
}

func (p *parallel) Frame(f Framing) Coord {
	q := *p
	q.area = f.Area
	q.framed = true

	// Every dimension maps onto the unit interval, so what reaches Point is a
	// fraction of an axis rather than a value in somebody's units. It is
	// ternary.pin's move made N times: a coord that ranges the scales it
	// places is how the pair arriving here is made meaningful.
	for _, d := range q.dims {
		if d.Scale != nil {
			d.Scale.SetRange(0, 1)
		}
	}
	// The panel's own axes carry nothing and are pinned so that they cannot
	// say otherwise: X is which axis, Y is how far up it.
	pinUnit(f.X, 0, float64(max(len(q.dims)-1, 0)))
	pinUnit(f.Y, 0, 1)
	return &q
}

// pinUnit fixes one of the panel's own axes to the interval this coord reads
// it as, and gives it that interval as its range so that Map is the identity.
func pinUnit(sc scale.Scale, lo, hi float64) {
	if sc == nil {
		return
	}
	if z, ok := sc.(scale.Zoomer); ok {
		z.SetDomain(lo, hi)
	}
	a, b := sc.Domain()
	sc.SetRange(float32(a), float32(b))
}

func (p *parallel) Extent() (x0, x1, y0, y1 float32) {
	return 0, float32(max(len(p.dims)-1, 0)), 0, 1
}

// Point places (which axis, how far up it). x is an axis index and need not be
// a whole number: the segment of a line between two axes is drawn by handing
// the two ends, and everything between them is the straight line the coord
// promises.
func (p *parallel) Point(x, y float32) ir.Point {
	return ir.Point{X: p.at(x), Y: p.area.Max.Y - y*p.area.Dy()}
}

// at is where axis x stands, in device units. One axis stands in the middle,
// because an axis at the left edge of a panel that has room for one is a chart
// pushed into a corner.
func (p *parallel) at(x float32) float32 {
	n := len(p.dims)
	if n <= 1 {
		return (p.area.Min.X + p.area.Max.X) / 2
	}
	return p.area.Min.X + x*p.area.Dx()/float32(n-1)
}

func (p *parallel) Points(dst []ir.Point, xs, ys []float32) []ir.Point {
	for i := range xs {
		dst = append(dst, p.Point(xs[i], ys[i]))
	}
	return dst
}

// Straight reports true: the map is affine in both directions, so a line
// between two axes is the straight line a reader reads it as.
func (p *parallel) Straight() bool { return true }

func (p *parallel) Edge(path *ir.Path, _, to ir.Point) { path.LineTo(to.X, to.Y) }

func (p *parallel) Area(path *ir.Path, x0, y0, x1, y1 float32) {
	q := [4]ir.Point{p.Point(x0, y0), p.Point(x1, y0), p.Point(x1, y1), p.Point(x0, y1)}
	path.MoveTo(q[0].X, q[0].Y).LineTo(q[1].X, q[1].Y).
		LineTo(q[2].X, q[2].Y).LineTo(q[3].X, q[3].Y).Close()
}

// Clip is the panel rectangle: the axes divide it and nothing about it is
// out of bounds.
func (p *parallel) Clip(path *ir.Path, area ir.Rect) { path.Rect(area) }

// Invert reads a device point back as (which axis, how far up it), which is
// what a tooltip needs before it asks a dimension's own scale what the value
// was.
func (p *parallel) Invert(pt ir.Point) (x, y float32) {
	n := len(p.dims)
	x = 0
	if n > 1 && p.area.Dx() != 0 {
		x = (pt.X - p.area.Min.X) * float32(n-1) / p.area.Dx()
	}
	if p.area.Dy() == 0 {
		return x, 0
	}
	return x, (p.area.Max.Y - pt.Y) / p.area.Dy()
}

// Decimates reports false. A reduction defined over pixel columns buckets by
// screen x, and here screen x is which axis a point is on rather than a
// quantity — so every row would reduce to the handful of columns the axes
// stand in.
func (p *parallel) Decimates() bool { return false }

// Fixed reports true: the panel's own X and Y say which axis and how far up,
// and a pan or a zoom of either moves nothing a reader is looking at. Zooming
// a *dimension* is a zoom of that dimension's own scale.
func (p *parallel) Fixed() bool { return true }

func (p *parallel) Dimensions() []ParallelDim { return p.dims }

func (p *parallel) Describe() Desc {
	d := Desc{Type: TypeParallel}
	for _, dim := range p.dims {
		e := DimDesc{Name: dim.Name}
		if sd, ok := scale.Describe(dim.Scale); ok {
			e.Scale = sd
		}
		d.Dims = append(d.Dims, e)
	}
	return d
}

// ParallelTicks is how many levels a dimension's ladder asks its scale for.
//
// It is a constant rather than a number derived from the panel because a
// parallel panel's axes are as tall as the panel however many of them there
// are: what changes with the count is how much room each has *across*, and a
// ladder's levels do not use it. Five is what a vertical axis a few hundred
// pixels tall carries without crowding.
const ParallelTicks = 5

// Furniture raises one family per dimension: the axis line, a mark at every
// level, the level labels in that dimension's own units, and the dimension's
// name above it.
//
// The panel's own two tick lists are left alone — they describe an axis index
// and a fraction, and a chart drawn in this coord turns them off — so nothing
// here fills GridX, TickY or their neighbours. That is the whole of what
// [Family] was added for: a ladder with no tick list behind it now has
// somewhere to be drawn and something to be labelled by.
func (p *parallel) Furniture(dst *Furniture, req FurnitureRequest) {
	// The axes stand apart across the panel, so no two of their labels share a
	// row and render thins them against each other by their boxes.
	dst.XLabelsShareARow = false
	// The axes stand inside the region the lines are drawn in, so their labels
	// are written after the marks rather than under them — the polar coord's
	// answer for a radial axis that runs through its own ring.
	dst.AxesOverData = true
	// The families are this panel's axes, so their labels are written wherever
	// the panel writes labels at all: a chart that turned the panel's own two
	// meaningless ticks off in the theme would otherwise lose the numbers on
	// every dimension with them.
	dst.FamiliesAreTheAxes = true
	// The panel's own ticks say nothing worth drawing either way: left to
	// themselves they would number an axis index and a fraction of an axis.
	silence(dst.x(), req.XTicks)
	silence(dst.y(), req.YTicks)
	if !p.framed || len(p.dims) == 0 {
		return
	}
	m := req.Metrics
	for i, dim := range p.dims {
		fam := dst.family(dim.Name)
		at := p.at(float32(i))
		top, bottom := p.area.Min.Y, p.area.Max.Y

		// The axis line itself is the family's first level. It takes grid ink
		// like the rest of a family, which is what a parallel-coordinates axis
		// is printed in: the lines are the reading and the axes are behind
		// them.
		run(fam.next(), ir.Point{X: at, Y: top}, ir.Point{X: at, Y: bottom})
		// The name goes inside the panel rather than above it: a panel with
		// four axes has four of them, and a row of names above the plot would
		// be a second title competing with the first.
		fam.label(Label{
			At: ir.Point{X: at, Y: top + m.labelGap()},
			H:  p.nameAlign(i), V: ir.AlignTop,
		}, dim.Name)

		if dim.Scale == nil {
			continue
		}
		side := p.labelSide(i)
		for _, tk := range dim.Scale.Ticks(scale.TickRequest{Want: ParallelTicks}) {
			v := tk.Pos
			if v < 0 || v > 1 || math.IsNaN(float64(v)) {
				continue
			}
			y := p.Point(0, v).Y
			l := m.tickLen(tk)
			run(fam.next(), ir.Point{X: at - side*l, Y: y}, ir.Point{X: at, Y: y})
			if tk.Minor {
				continue
			}
			fam.label(Label{
				At: ir.Point{X: at - side*m.labelGap(), Y: y},
				H:  outwardAlign(side), V: ir.AlignMiddle,
			}, tk.Label)
		}
	}
}

// nameAlign is how a dimension's name sits about its axis: centred, except on
// the outermost axes, which stand on the panel's edges and would hang half a
// name over them.
func (p *parallel) nameAlign(i int) ir.HAlign {
	switch {
	case len(p.dims) < 2:
		return ir.AlignCenter
	case i == 0:
		return ir.AlignStart
	case i == len(p.dims)-1:
		return ir.AlignEnd
	}
	return ir.AlignCenter
}

// labelSide is which way a dimension's levels are labelled: to the right of
// its axis, except for the last, whose labels would fall out of the panel.
//
// Inside the panel on purpose. A parallel panel has no gutters to put N
// ladders in — a gutter is a side of the plot and there are two of those — so
// the numbers sit against their own axis, where a reader reads them anyway.
func (p *parallel) labelSide(i int) float32 {
	if i == len(p.dims)-1 && len(p.dims) > 1 {
		return 1
	}
	return -1
}

// silence marks every tick of one of the panel's own axes as outside the
// panel, which is how a coord says "draw none of these" without the theme
// having to: the panel's X is an axis index and its Y a fraction of an axis,
// and neither is a quantity a reader wants numbered.
//
// A chart that also turns the ticks off in its theme gets the gutters back as
// well — [Furniture.FamiliesAreTheAxes] is what keeps the dimensions labelled
// when it does — and one that leaves the theme alone still draws no numbers it
// cannot explain. Both are right, which is the point of doing it here.
func silence(s side, ticks []scale.Tick) {
	for range ticks {
		s.next()
		s.mark(false, Label{})
	}
}

// outwardAlign is how a level's label sits about its anchor, given which side
// of the axis it was put on.
func outwardAlign(side float32) ir.HAlign {
	if side < 0 {
		return ir.AlignStart
	}
	return ir.AlignEnd
}

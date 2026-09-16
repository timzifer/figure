package coord

import (
	"math"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// Ternary reads a panel's two axes as two components of a composition and
// derives the third, which is what puts a point inside the triangle every
// physical science reads a three-part mixture in.
//
// X is the first component and Y the second; the third is Sum − x − y. A
// rock's three oxides, a soil's sand, silt and clay, an alloy's three metals,
// a classifier's three-class probability: two of the three are free, and the
// third is what is left.
//
//	p := figure.New(figure.Coord(coord.Ternary()))
//	p.X(scale.Linear())
//	p.Y(scale.Linear())
//	p.Add(geom.Scatter(rocks, geom.X("quartz"), geom.Y("feldspar"),
//		geom.ColorBy("unit", scale.Qualitative())))
//
// [TernarySum] is the percentage spelling, which is what most tables in these
// fields come in.
//
// # The third component is derived, not read
//
// The alternative was three named columns, and it was refused twice over. A
// coord transforms a *mapped pair* — that is the stage ADR 0018 built — so a
// third column has nowhere to enter. And three columns bring a consistency
// question the derivation does not have: a row whose three columns sum to 0.98
// has to be normalised, refused or ignored, and every answer is wrong for
// somebody. Deriving the third makes the constraint hold by construction. A
// table that genuinely carries three columns divides by their sum, which is
// arithmetic at the call site.
//
// # It is the cheapest coord in the package
//
// The barycentric map is affine — a 2×2 matrix and a translation — and every
// method follows from that. [Coord.Straight] is true, so an edge is one LineTo
// and every geom draws exactly what it drew under Cartesian; an Area is four
// transformed corners, so a [github.com/timzifer/figure/geom.Rect] over binned
// compositions is a ternary density chart and needed no new mark; Invert is
// the inverse matrix, so a tooltip reports a composition. Only [Coord.Clip]
// differs: it is the triangle rather than the panel.
//
// [Coord.Decimates] is false, and the reason is a derivation rather than an
// observation. Working the map out gives px = ½ + ½b − ½a, so a column of
// screen is a band of constant b − a rather than of constant a: a reduction
// defined over pixel columns does not measure what it was defined to measure,
// even though nothing about the map is curved.
//
// # Three ladders, two tick lists
//
// The constant-a and constant-b families are the images of the two axes' own
// ticks, exactly as a Smith chart's two families are, and they are labelled
// from the ticks' own labels. The constant-c family needs no new field to be
// drawn: it is the diagonal a + b = Sum − v, it pairs with the X tick at v,
// and the coord emits it as a second subpath inside that tick's grid shape. So
// it takes the theme's grid ink like its neighbours and [Furniture] gained
// nothing.
//
// What is genuinely missing is that family's labels, because a panel carries
// one label per tick per side and there is no third side. That is the seam
// [ADR 0033] named for the VSWR circles, and it is not spent on two thirds of
// the problem — see docs/adr/0051-barycentric-coord.md. Until it is, the third
// edge is labelled the way ternary charts are usually labelled anyway:
// [github.com/timzifer/figure/geom.Note] at the corner, naming the component.
// The numeric ladder there is redundant with the other two, which is why the
// omission is survivable and why an unlabelled third family is not a broken
// chart.
//
// # The domains are pinned
//
// Both axes run from zero to Sum whatever the data does, for the reason a
// Smith chart's are pinned: the chart's extent is the whole simplex, and an
// axis that autoscaled to a tight cluster of compositions would draw a
// triangle that is not one. A zoom therefore relabels and moves nothing, and
// the coord reports itself [Fixed] so an interactive chart leaves the panel's
// axes alone.
//
// [ADR 0033]: https://github.com/timzifer/figure/blob/main/docs/adr/0033-smith-charts.md
func Ternary(opts ...TernaryOption) Coord {
	t := &ternary{sum: 1}
	for _, o := range opts {
		o(t)
	}
	return t
}

// TernaryOption configures a barycentric coord. It is named for the coord it
// configures, as [SmithOption] and [PolarOption] are.
type TernaryOption func(*ternary)

// TernarySum sets what the three components add up to. The default is 1, and
// 100 is the percentage spelling most tables in these fields come in.
func TernarySum(k float64) TernaryOption {
	return func(t *ternary) {
		if k > 0 {
			t.sum = k
		}
	}
}

// ternary is the coord and — once [ternary.Frame] has been called — the
// triangle it was inscribed in. Frame returns a copy rather than moving the
// receiver, so two panels drawn on two goroutines never share one triangle.
type ternary struct {
	sum float64

	// a, b and c are the corners of the triangle in device space: the corner
	// where the first component is everything, where the second is, and where
	// the derived third is.
	a, b, c ir.Point
	framed  bool
}

// triangleHeight is the height of an equilateral triangle of unit side.
var triangleHeight = float32(math.Sqrt(3) / 2)

func (t *ternary) Frame(f Framing) Coord {
	area := f.Area
	q := *t
	side := area.Dx()
	if h := area.Dy() / triangleHeight; h < side {
		side = h
	}
	if side < 0 {
		side = 0
	}
	cx, cy := (area.Min.X+area.Max.X)/2, (area.Min.Y+area.Max.Y)/2
	base, apex := cy+side*triangleHeight/2, cy-side*triangleHeight/2
	q.a = ir.Point{X: cx - side/2, Y: base}
	q.b = ir.Point{X: cx + side/2, Y: base}
	q.c = ir.Point{X: cx, Y: apex}
	q.pin(f.X)
	q.pin(f.Y)
	q.framed = true
	return &q
}

// pin fixes one axis's domain to the whole simplex and gives it that interval
// as its range, so that a linear scale's Map is the identity and the pair
// reaching [ternary.Point] is the composition itself.
//
// It is [smith.Frame]'s identityRange with the domain pinned as well, which is
// the difference between a chart whose extent the caller states and one whose
// extent is a fact about the coordinate system. A scale that cannot be pinned —
// an ordinal one — keeps its own domain and lands wherever its Map puts it.
func (t *ternary) pin(sc scale.Scale) {
	if sc == nil {
		return
	}
	if z, ok := sc.(scale.Zoomer); ok {
		z.SetDomain(0, t.sum)
	}
	lo, hi := sc.Domain()
	sc.SetRange(float32(lo), float32(hi))
}

func (t *ternary) Extent() (x0, x1, y0, y1 float32) {
	k := float32(t.sum)
	return 0, k, 0, k
}

// Point places a composition. The third component is what is left, and the
// point is the weighted mean of the three corners — the barycentric map, which
// is the whole of this coord.
func (t *ternary) Point(x, y float32) ir.Point {
	k := float32(t.sum)
	if k == 0 {
		return t.c
	}
	a, b := x/k, y/k
	c := 1 - a - b
	return ir.Point{
		X: a*t.a.X + b*t.b.X + c*t.c.X,
		Y: a*t.a.Y + b*t.b.Y + c*t.c.Y,
	}
}

func (t *ternary) Points(dst []ir.Point, xs, ys []float32) []ir.Point {
	for i := range xs {
		dst = append(dst, t.Point(xs[i], ys[i]))
	}
	return dst
}

// Straight reports true: an affine map takes a straight segment to a straight
// segment, so every geom draws the polyline it drew under Cartesian.
func (t *ternary) Straight() bool { return true }

func (t *ternary) Edge(p *ir.Path, _, to ir.Point) { p.LineTo(to.X, to.Y) }

// Area appends the image of a data-space rectangle, which under an affine map
// is a parallelogram — correctly, since a cell of binned compositions is one.
func (t *ternary) Area(p *ir.Path, x0, y0, x1, y1 float32) {
	q := [4]ir.Point{t.Point(x0, y0), t.Point(x1, y0), t.Point(x1, y1), t.Point(x0, y1)}
	p.MoveTo(q[0].X, q[0].Y).LineTo(q[1].X, q[1].Y).
		LineTo(q[2].X, q[2].Y).LineTo(q[3].X, q[3].Y).Close()
}

// Clip is the triangle, which is the one thing this coord does not inherit
// from Cartesian: everything outside the simplex is a composition with a part
// that is less than nothing or more than everything.
func (t *ternary) Clip(p *ir.Path, area ir.Rect) {
	if !t.framed {
		p.Rect(area)
		return
	}
	p.MoveTo(t.a.X, t.a.Y).LineTo(t.b.X, t.b.Y).LineTo(t.c.X, t.c.Y).Close()
}

// Invert reads a device point back as a composition: the inverse of the 2×2
// matrix Point applies, measured from the third corner.
func (t *ternary) Invert(pt ir.Point) (x, y float32) {
	e1x, e1y := t.a.X-t.c.X, t.a.Y-t.c.Y
	e2x, e2y := t.b.X-t.c.X, t.b.Y-t.c.Y
	det := e1x*e2y - e1y*e2x
	if det == 0 {
		return 0, 0
	}
	dx, dy := pt.X-t.c.X, pt.Y-t.c.Y
	k := float32(t.sum)
	return k * (dx*e2y - dy*e2x) / det, k * (e1x*dy - e1y*dx) / det
}

// Decimates reports false. See [Ternary]: a column of screen is a band of
// constant b − a rather than of constant a, so a reduction defined over pixel
// columns is not measuring what it was defined to measure.
func (t *ternary) Decimates() bool { return false }

// Fixed reports true: the triangle is the whole picture whatever the domains
// are, which is why they are pinned.
func (t *ternary) Fixed() bool { return true }

func (t *ternary) Describe() Desc { return Desc{Type: TypeTernary, Sum: t.sum} }

// Furniture places the three ladders.
//
// The X axis line is the edge where the second component is nothing — the one
// the first component's ladder is read along — and the Y axis line is the
// other two edges in one run: a panel has two axis lines, a triangle has three
// sides, and a ternary chart with one side unstroked is not a triangle.
func (t *ternary) Furniture(dst *Furniture, req FurnitureRequest) {
	m := req.Metrics
	// The X labels run up a slanted edge rather than along one horizontal
	// line, so two of them that share an x are still a finger apart.
	dst.XLabelsShareARow = false

	dst.AxisX.line(t.a, t.c)
	dst.AxisY.Pts = append(dst.AxisY.Pts[:0], t.c, t.b, t.a)

	t.ladder(dst.x(), req.XTicks, m, true)
	t.ladder(dst.y(), req.YTicks, m, false)
}

// ladder fills one axis's worth of furniture: a grid line per tick, a mark on
// the edge that axis is read along, and the label beyond it.
//
// The first component's ladder carries the derived third family as a second
// subpath, which is the whole of how a chart with three grid families is drawn
// by a panel with two tick lists.
func (t *ternary) ladder(s side, ticks []scale.Tick, m Metrics, first bool) {
	k := float32(t.sum)
	out := t.outward(first)
	for _, tk := range ticks {
		grid, mark := s.next()
		v := tk.Pos
		if v < 0 || v > k || math.IsNaN(float64(v)) {
			s.mark(false, Label{})
			continue
		}
		var at ir.Point
		if first {
			// Constant a: from the b = 0 edge across to the c = 0 edge.
			at = t.Point(v, 0)
			if !tk.Minor {
				run(grid, at, t.Point(v, k-v))
				// And the derived family at the same level: constant c, which
				// is the diagonal a + b = k − v.
				run(grid, t.Point(k-v, 0), t.Point(0, k-v))
			}
		} else {
			// Constant b: from the a = 0 edge across to the c = 0 edge.
			at = t.Point(0, v)
			if !tk.Minor {
				run(grid, at, t.Point(k-v, v))
			}
		}
		if l := m.tickLen(tk); l > 0 {
			mark.line(at, ir.Point{X: at.X + out.X*l, Y: at.Y + out.Y*l})
		}
		gap := m.labelGap()
		h, va := radialAlign(math.Atan2(float64(out.X), float64(-out.Y)))
		s.mark(true, Label{
			At: ir.Point{X: at.X + out.X*gap, Y: at.Y + out.Y*gap},
			H:  h,
			V:  va,
		})
	}
}

// run appends one straight grid line as a subpath, which is what lets a single
// tick's shape hold two of them.
func run(s *Shape, from, to ir.Point) {
	s.Path.MoveTo(from.X, from.Y).LineTo(to.X, to.Y)
}

// outward is the unit direction out of the edge one ladder is read along, away
// from the corner opposite it.
func (t *ternary) outward(first bool) ir.Point {
	from, to, away := t.a, t.c, t.b
	if !first {
		from, to, away = t.b, t.c, t.a
	}
	// The normal of the edge, turned to point away from the third corner.
	nx, ny := -(to.Y - from.Y), to.X-from.X
	if nx*(away.X-from.X)+ny*(away.Y-from.Y) > 0 {
		nx, ny = -nx, -ny
	}
	if l := float32(math.Hypot(float64(nx), float64(ny))); l > 0 {
		nx, ny = nx/l, ny/l
	}
	return ir.Point{X: nx, Y: ny}
}

var (
	_ Coord     = (*ternary)(nil)
	_ Describer = (*ternary)(nil)
	_ Fixed     = (*ternary)(nil)
)

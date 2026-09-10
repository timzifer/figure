package three

import (
	"fmt"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

// Bar3 draws one box per cell of two categorical axes.
//
// # Read the heatmap first
//
// Bars occlude each other, the back row is unreadable, and the height of a bar
// behind another cannot be compared to it. The flat chart of the same table —
// [github.com/timzifer/figure/geom.Rect] with a colour scale, which is what a
// heatmap is — wins almost every time. This ships because refusing it invites
// a worse reimplementation by every caller, and its doc comment says so
// because a reader who chose wrong is a reader who was not told. See
// docs/adr/0058-what-3d-is-for.md.
//
//	three.Bar3(src, geom.X("service"), geom.Y("region"), geom.Z("latency"))
//
// Both floor axes want a [github.com/timzifer/figure/scale.Ordinal]: this is a
// chart of two categoricals, and a continuous axis under it draws boxes at
// arbitrary widths.
//
// The three faces a box shows are one primitive each and they carry the same
// source row, so a pointer anywhere on a bar reports that row. They are
// ordered by the cell the bar stands on rather than by the middle of the box,
// so a tall bar never draws itself in front of the short one standing between
// it and the reader.
func Bar3(src data.Source, opts ...geom.Option) Layer {
	return &bar3{base: newBase(src, "bar3", opts)}
}

type bar3 struct {
	base
	xs, ys, zs []float64
	ok         bool
}

func (g *bar3) Train(t geom.Training) error {
	g.ok = false
	var err error
	if g.xs, err = column(g.src, g.cfg.X, t.X); err != nil {
		return err
	}
	if g.ys, err = column(g.src, g.cfg.Y, t.Y); err != nil {
		return err
	}
	if g.zs, err = column(g.src, g.cfg.Z, t.Z); err != nil {
		return err
	}
	if len(g.xs) != len(g.ys) || len(g.ys) != len(g.zs) {
		return fmt.Errorf("figure/three: the bars' columns have %d, %d and %d rows",
			len(g.xs), len(g.ys), len(g.zs))
	}
	t.X.Train(g.xs...)
	t.Y.Train(g.ys...)
	t.Z.Train(g.zs...)
	// A bar stands on the floor, so the height axis includes it whether or not
	// the data reaches down that far: a field of bars measured from a floor
	// nobody can see measures nothing.
	t.Z.Train(g.cfg.Baseline)
	g.ok = true
	return nil
}

func (g *bar3) Emit(s *Sink, f Frame) error {
	if !g.ok {
		return nil
	}
	// The cell a bar stands on is what it is occluded by, and its own height
	// is not. That is a reading of the data rather than a rendering trick, and
	// it is what makes a field of bars orderable at all.
	s.Depth(DepthGround)
	track := rowsWanted(f)

	hx, hy := g.halfSlot(f.X), g.halfSlot(f.Y)
	base := g.fillFor(f)
	stroke, width := g.outline()
	floor := at(f.Z, g.cfg.Baseline)

	// Back to front over the cells, so that the sort confirms an order the
	// emission already had.
	for _, i := range g.order(f) {
		if !defined(f.X, g.xs[i]) || !defined(f.Y, g.ys[i]) || !defined(f.Z, g.zs[i]) {
			continue
		}
		cx, cy := at(f.X, g.xs[i]), at(f.Y, g.ys[i])
		top := at(f.Z, g.zs[i])
		x0, x1 := clamp01(cx-hx), clamp01(cx+hx)
		y0, y1 := clamp01(cy-hy), clamp01(cy+hy)
		lo, hi := floor, top
		if hi < lo {
			lo, hi = hi, lo
		}
		if track {
			s.Row(i)
		}
		for _, face := range visibleFaces(f.Forward, x0, y0, x1, y1, lo, hi) {
			c := face
			s.Face(c[:], Style{
				Fill:   shade(f.Theme, base, faceNormal(c[0], c[1], c[2])),
				Stroke: stroke,
				Width:  width,
			})
		}
	}
	return nil
}

// visibleFaces returns the top of a box and the two sides that face the
// camera. The other three are behind them by construction and drawing them
// would be ink nobody sees.
func visibleFaces(fwd Vec3, x0, y0, x1, y1, lo, hi float32) [3][4]Vec3 {
	var out [3][4]Vec3
	out[0] = [4]Vec3{{x0, y0, hi}, {x1, y0, hi}, {x1, y1, hi}, {x0, y1, hi}}

	// The visible face along an axis is the one on the near side: the camera
	// looks along fwd, so a face at the low end is visible when fwd points
	// toward the high one.
	xs := x1
	if fwd.X > 0 {
		xs = x0
	}
	out[1] = [4]Vec3{{xs, y0, lo}, {xs, y1, lo}, {xs, y1, hi}, {xs, y0, hi}}

	ys := y1
	if fwd.Y > 0 {
		ys = y0
	}
	out[2] = [4]Vec3{{x0, ys, lo}, {x1, ys, lo}, {x1, ys, hi}, {x0, ys, hi}}
	return out
}

// halfSlot is half a bar's footprint along one axis: the scale's own band
// where it has one, narrowed by geom.BarWidth.
func (g *bar3) halfSlot(s scale.Scale) float32 {
	w := float32(0.04)
	if b, ok := s.(scale.Band); ok {
		w = b.Bandwidth() / 2
	}
	if f := g.cfg.BarWidth; f > 0 && f <= 1 {
		w *= float32(f)
	}
	return w
}

// order walks the cells from the far corner toward the reader, which is the
// order they will sort into anyway. Doing it here costs one comparison and
// leaves the sort with a sequence it can confirm without a swap.
func (g *bar3) order(f Frame) []int {
	idx := make([]int, len(g.xs))
	for i := range idx {
		idx[i] = i
	}
	fwd := f.Forward
	depth := func(i int) float64 {
		v := Vec3{at(f.X, g.xs[i]), at(f.Y, g.ys[i]), 0}
		return v.Sub(centre).Dot(fwd)
	}
	// An insertion sort: the field is small by construction — it is a chart of
	// two categoricals — and this way the order of equal depths is the order
	// of the rows, which is ADR 0012's tie-break.
	for i := 1; i < len(idx); i++ {
		for j := i; j > 0 && depth(idx[j]) > depth(idx[j-1]); j-- {
			idx[j], idx[j-1] = idx[j-1], idx[j]
		}
	}
	return idx
}

func (g *bar3) Legend(f Frame) (geom.LegendEntry, bool) {
	return g.base.legend(f, geom.SwatchBox)
}

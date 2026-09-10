package three

import (
	"fmt"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
)

// Line3 draws an ordered path through x, y and z.
//
// It is the trajectory: an orbit, a tool path, an IMU track, an attractor — a
// curve that crosses itself in every flat projection of it and not in the
// data. See docs/adr/0058-what-3d-is-for.md.
//
//	three.Line3(src, geom.X("t"), geom.Y("offset"), geom.Z("power"))
//
// [github.com/timzifer/figure/geom.GroupBy] draws one path per series, and
// that is all a cascade is: a family of traces offset along one floor axis,
// which is the display a spectrum analyser has had since the seventies. It is
// a recipe rather than a mark of its own, which is why there is no Cascade
// here — see examples/cascade.
//
// A path is emitted one segment at a time rather than whole. A whole path is
// one primitive with one depth, and a curve that spans the scene has no single
// depth, so a trajectory crossing a surface would be drawn wholly in front of
// it or wholly behind. Per segment it interleaves at every scale a reader can
// see. What it still cannot do is pass *through* a surface within one segment:
// splitting a primitive along an intersection is a BSP tree, which is a
// renderer, and this package draws the shapes whose order is decidable.
func Line3(src data.Source, opts ...geom.Option) Layer {
	return &line3{base: newBase(src, "line3", opts)}
}

type line3 struct {
	base
	xs, ys, zs []float64
	groups     []string
	ok         bool
}

func (g *line3) Train(t geom.Training) error {
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
		return fmt.Errorf("figure/three: the line's columns have %d, %d and %d rows",
			len(g.xs), len(g.ys), len(g.zs))
	}
	g.groups = nil
	if g.cfg.Group != "" {
		labels, ok := data.Labels(g.src, g.cfg.Group)
		if !ok {
			return fmt.Errorf("%w: %q", ErrNoColumn, g.cfg.Group)
		}
		if len(labels) != len(g.xs) {
			return fmt.Errorf("figure/three: the group column has %d rows and the positions have %d",
				len(labels), len(g.xs))
		}
		g.groups = labels
	}
	t.X.Train(g.xs...)
	t.Y.Train(g.ys...)
	t.Z.Train(g.zs...)
	g.ok = true
	return nil
}

func (g *line3) Emit(s *Sink, f Frame) error {
	if !g.ok || len(g.xs) < 2 {
		return nil
	}
	// A segment is ordered by its own middle, which is what a path wants: a
	// segment is in front of another exactly when its middle is nearer.
	s.Depth(DepthCentroid)
	track := rowsWanted(f)

	st := Style{Stroke: g.colorFor(f), Width: g.widthFor(f)}
	var seg [2]Vec3
	for i := 1; i < len(g.xs); i++ {
		if g.groups != nil && g.groups[i] != g.groups[i-1] {
			// A new series starts here rather than a segment joining the end
			// of one trace to the start of the next.
			continue
		}
		if !g.plottable(f, i-1) || !g.plottable(f, i) {
			continue
		}
		seg[0] = g.point(f, i-1)
		seg[1] = g.point(f, i)
		if track {
			// A segment stands for the row it ends at, which is the row the
			// reader is pointing at when they point at the far end of it.
			s.Row(i)
		}
		s.Line(seg[:], st)
	}
	return nil
}

func (g *line3) point(f Frame, i int) Vec3 {
	return Vec3{at(f.X, g.xs[i]), at(f.Y, g.ys[i]), at(f.Z, g.zs[i])}
}

func (g *line3) plottable(f Frame, i int) bool {
	return defined(f.X, g.xs[i]) && defined(f.Y, g.ys[i]) && defined(f.Z, g.zs[i])
}

func (g *line3) Legend(f Frame) (geom.LegendEntry, bool) {
	return g.base.legend(f, geom.SwatchLine)
}

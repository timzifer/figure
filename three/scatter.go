package three

import (
	"fmt"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// Scatter3 draws one marker per row at x, y and z, each with a rule down to
// the floor of the scene.
//
//	three.Scatter3(src, geom.X("pc1"), geom.Y("pc2"), geom.Z("pc3"))
//
// It is the rank-1 form docs/adr/0058-what-3d-is-for.md names for three
// measured columns: whether a cluster is a cluster, or two clouds that overlap
// along the axis a flat chart happened not to plot. That is a question the
// third axis answers and no projection of it can.
//
// # The droplines are the chart
//
// A point floating in a projected box has no height a reader can judge — it
// is somewhere along a ray, and the picture does not say where. The rule from
// each point to the floor is what turns its position back into three numbers:
// where it meets the floor is its x and y, and how long it is is its z. So the
// rules are on by default, and [github.com/timzifer/figure/geom.Droplines]
// turns them off for a cloud dense enough that they become a curtain.
//
// # Markers are symbols, not solids
//
// A marker is drawn the size it was given wherever it stands. One that shrank
// with distance would be a size channel nobody asked for and one that
// foreshortened would be a disc lying in a plane — see [Sink.Marker]. Each
// marker and each rule is its own primitive in the scene's one depth order, so
// a point behind a surface is drawn behind it.
//
// [github.com/timzifer/figure/geom.ColorBy] colours each point from a column —
// a number through a continuous scale, or a label through a discrete one —
// and [github.com/timzifer/figure/geom.Shape] and
// [github.com/timzifer/figure/geom.Size] choose the symbol. A pointer on a
// marker reports its row.
func Scatter3(src data.Source, opts ...geom.Option) Layer {
	return &scatter3{base: newBase(src, "scatter3", opts)}
}

type scatter3 struct {
	base
	xs, ys, zs []float64
	// cv and cl are the colour column, as numbers or as labels, and nil for a
	// layer painted in one colour.
	cv []float64
	cl []string
	ok bool
}

// droplineOpacity is how strongly a rule is drawn against its point's colour.
// A rule is a reading aid rather than a mark: at full strength a few hundred of
// them are a fence standing in front of the points they belong to.
const droplineOpacity = 0.45

func (g *scatter3) Train(t geom.Training) error {
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
		return fmt.Errorf("figure/three: the scatter's columns have %d, %d and %d rows",
			len(g.xs), len(g.ys), len(g.zs))
	}
	if err := g.colors(); err != nil {
		return err
	}
	t.X.Train(g.xs...)
	t.Y.Train(g.ys...)
	t.Z.Train(g.zs...)
	g.ok = true
	return nil
}

// colors reads the colour column, if the layer named one and a scale to read
// it through.
func (g *scatter3) colors() error {
	g.cv, g.cl = nil, nil
	cs := g.cfg.ColorScale
	if g.cfg.ColorCol == "" || cs == nil {
		return nil
	}
	if _, discrete := scale.Discrete(cs); discrete {
		labels, ok := data.Labels(g.src, g.cfg.ColorCol)
		if !ok {
			return fmt.Errorf("%w: %q", ErrNoColumn, g.cfg.ColorCol)
		}
		if len(labels) != len(g.xs) {
			return fmt.Errorf("figure/three: the colour column has %d rows and the positions have %d",
				len(labels), len(g.xs))
		}
		g.cl = labels
		return nil
	}
	vs, ok := data.Float64Column(g.src, g.cfg.ColorCol)
	if !ok {
		return fmt.Errorf("%w: %q, which has to be a number for a continuous colour scale", ErrNoColumn, g.cfg.ColorCol)
	}
	if len(vs) != len(g.xs) {
		return fmt.Errorf("figure/three: the colour column has %d rows and the positions have %d",
			len(vs), len(g.xs))
	}
	cs.Train(vs...)
	g.cv = vs
	return nil
}

// colorAt is row i's colour.
func (g *scatter3) colorAt(f Frame, i int, fallback ir.Color) ir.Color {
	switch {
	case g.cl != nil:
		d, _ := scale.Discrete(g.cfg.ColorScale)
		return d.ColorOf(g.cl[i])
	case g.cv != nil:
		return g.cfg.ColorScale.Color(g.cv[i])
	}
	return fallback
}

func (g *scatter3) Emit(s *Sink, f Frame) error {
	if !g.ok {
		return nil
	}
	track := rowsWanted(f)

	base := g.fillFor(f)
	size := g.cfg.Size
	if size <= 0 {
		size = f.Theme.MarkerSize
	}
	if size <= 0 {
		size = 6
	}
	shape := ir.MarkerCircle
	if g.cfg.MarkerSet {
		shape = g.cfg.Marker
	}
	stroke, width := g.outline()
	rule := g.widthFor(f) / 2
	if rule < 0.75 {
		rule = 0.75
	}

	var seg [2]Vec3
	for i := range g.xs {
		if !defined(f.X, g.xs[i]) || !defined(f.Y, g.ys[i]) || !defined(f.Z, g.zs[i]) {
			continue
		}
		p := f.Point(g.xs[i], g.ys[i], g.zs[i])
		col := g.colorAt(f, i, base)
		if col.A == 0 {
			continue
		}
		if track {
			s.Row(i)
		}
		if !g.cfg.HideDroplines {
			// A rule to the floor in a box; in a sphere, which has no floor,
			// the radius from the centre — the arrow a Bloch vector is drawn
			// as, and what makes a radius as readable as a height.
			foot := Vec3{p.X, p.Y, 0}
			if f.Spherical() {
				foot = centre
			}
			if foot != p {
				seg[0], seg[1] = foot, p
				s.Line(seg[:], Style{Stroke: ir.Fade(col, droplineOpacity), Width: rule})
			}
		}
		s.Marker(p, Style{Fill: col, Stroke: stroke, Width: width, Shape: shape, Size: size})
	}
	return nil
}

func (g *scatter3) Legend(f Frame) (geom.LegendEntry, bool) {
	return g.base.legend(f, geom.SwatchMarker)
}

func (g *scatter3) Describe() geom.Desc { return g.base.describe() }

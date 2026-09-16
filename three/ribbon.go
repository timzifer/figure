package three

import (
	"fmt"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
)

// Ribbon draws an ordered path through x, y and z as a band with width rather
// than as a stroke.
//
//	three.Ribbon(src, geom.X("t"), geom.Y("offset"), geom.Z("power"))
//
// It is [Line3] with one quad per segment instead of one line, and it is here
// for the reason docs/adr/0058-what-3d-is-for.md ranks it second: it carries
// the same reading a projected line does, differently. A band is a surface, so
// it is shaded and it occludes — a trajectory drawn as a ribbon says which way
// it is facing and which of two crossings is in front, where a stroke of
// constant width says neither.
//
// **A line with a band usually beats it.** In two dimensions width means an
// interval — a confidence band, a tolerance, a spread — and a reader who sees
// a band reads one. A ribbon's width is not a value here; it is the same
// number everywhere, chosen so the path can be seen. Where the width would
// carry a reading, that chart is a flat line with a band and this is the wrong
// mark for it. That note is the catalogue's own, and it is in the doc comment
// because a reader who chose wrong is a reader who was not told.
//
// [github.com/timzifer/figure/geom.Thickness] sets the width in scene units —
// a fraction of the cube's edge, or of the ball's diameter — and the default
// is a fortieth of it. The band lies flat: its width runs square to the path
// and square to the scene's up, which on a sphere is the radius through the
// point, so a ribbon on the ball lies on the ball. A corner is mitred by
// averaging the two segments' widths at the vertex they share, which is what
// makes a turn one continuous band rather than two quads with a wedge between
// them.
//
// [github.com/timzifer/figure/geom.GroupBy] draws one ribbon per series, and a
// pointer on a quad reports the row its far end is. The band follows the same
// path [Line3] does, so on a ball it runs along the great circle rather than
// cutting through the inside of it — see [Arc].
func Ribbon(src data.Source, opts ...geom.Option) Layer {
	return &ribbon3{base: newBase(src, "ribbon3", opts)}
}

// ribbonWidth is how wide a band is when nobody chose: a fortieth of the
// scene, which is wide enough to read as a surface at every size a figure is
// drawn at and narrow enough that a path crossing itself still shows both
// strands.
const ribbonWidth = 0.025

type ribbon3 struct {
	base
	xs, ys, zs []float64
	groups     []string
	ok         bool
}

func (g *ribbon3) Train(t geom.Training) error {
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
		return fmt.Errorf("figure/three: the ribbon's columns have %d, %d and %d rows",
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

func (g *ribbon3) Emit(s *Sink, f Frame) error {
	if !g.ok || len(g.xs) < 2 {
		return nil
	}
	track := rowsWanted(f)

	half := float32(g.cfg.Thickness) / 2
	if g.cfg.Thickness <= 0 {
		half = ribbonWidth / 2
	}
	fill := g.fillFor(f)
	stroke, width := g.outline()

	var quad [4]Vec3
	for i := 1; i < len(g.xs); i++ {
		if !g.linked(f, i) {
			continue
		}
		wa, wb := g.widthAt(f, i-1), g.widthAt(f, i)
		if wa == (Vec3{}) || wb == (Vec3{}) {
			continue
		}
		if track {
			// A quad stands for the row it ends at, which is the row the
			// reader is pointing at when they point at its far end — Line3's
			// rule, for the same reason.
			s.Row(i)
		}
		// The band follows the same path [Line3] does: the straight line in a
		// box, the great circle on a ball. Its width at a step between two
		// rows is square to the path there rather than interpolated between
		// the two mitres, so a band round the ball keeps its width all the way
		// along.
		arc := f.Arc(g.xs[i-1], g.ys[i-1], g.zs[i-1], g.xs[i], g.ys[i], g.zs[i])
		n := arc.Steps()
		p, w := arc.At(0), wa.Mul(half)
		for k := 1; k <= n; k++ {
			q := arc.At(float32(k) / float32(n))
			nw := wb
			if k < n {
				nw = acrossOf(f, arc.At(float32(k-1)/float32(n)), arc.At(float32(k+1)/float32(n)), q)
				if nw == (Vec3{}) {
					nw = wb
				}
			}
			nw = nw.Mul(half)
			quad[0], quad[1] = p.Sub(w), q.Sub(nw)
			quad[2], quad[3] = q.Add(nw), p.Add(w)
			s.Face(quad[:], Style{
				Fill:   shade(f.Theme, fill, faceNormal(quad[0], quad[1], quad[2])),
				Stroke: stroke,
				Width:  width,
			})
			p, w = q, nw
		}
	}
	return nil
}

// linked reports whether rows i-1 and i are two ends of one segment: the same
// series, and both with a position.
func (g *ribbon3) linked(f Frame, i int) bool {
	if i <= 0 || i >= len(g.xs) {
		return false
	}
	if g.groups != nil && g.groups[i] != g.groups[i-1] {
		return false
	}
	return g.plottable(f, i-1) && g.plottable(f, i)
}

// widthAt is the unit direction the band runs across at row i: square to the
// path and square to the scene's up.
//
// It is the mean of the two segments meeting at the row rather than either of
// them, which is what joins them into one band: two quads that each took their
// own segment's direction would leave a wedge open on the outside of every
// turn.
func (g *ribbon3) widthAt(f Frame, i int) Vec3 {
	var sum Vec3
	if g.linked(f, i) {
		sum = sum.Add(g.across(f, i-1, i))
	}
	if g.linked(f, i+1) {
		sum = sum.Add(g.across(f, i, i+1))
	}
	return sum.Unit()
}

// across is the unit direction square to the segment from row a to row b and
// to the scene's up at its middle.
func (g *ribbon3) across(f Frame, a, b int) Vec3 {
	p, q := g.point(f, a), g.point(f, b)
	return acrossOf(f, p, q, p.Add(q).Mul(0.5))
}

// acrossOf is the unit direction square to the run from p to q and to the
// scene's up at the point the band is being measured at.
func acrossOf(f Frame, p, q, at Vec3) Vec3 {
	d := q.Sub(p)
	w := d.Cross(upAt(f, at)).Unit()
	if w == (Vec3{}) {
		// The segment runs along the up direction, which leaves the band's
		// facing undecided; any direction square to it will do, and the
		// scene's x is the one every view of a box has a name for.
		w = d.Cross(Vec3{1, 0, 0}).Unit()
	}
	return w
}

// upAt is what a band lies flat against at a point: the scene's up in a box,
// and the radius through the point on a sphere, so that a ribbon on the ball
// lies on the ball rather than in a horizontal plane cutting through it.
func upAt(f Frame, p Vec3) Vec3 {
	if !f.Spherical() {
		return Vec3{0, 0, 1}
	}
	if r := p.Sub(centre).Unit(); r != (Vec3{}) {
		return r
	}
	return Vec3{0, 0, 1}
}

func (g *ribbon3) point(f Frame, i int) Vec3 {
	return f.Point(g.xs[i], g.ys[i], g.zs[i])
}

func (g *ribbon3) plottable(f Frame, i int) bool {
	return defined(f.X, g.xs[i]) && defined(f.Y, g.ys[i]) && defined(f.Z, g.zs[i])
}

func (g *ribbon3) Legend(f Frame) (geom.LegendEntry, bool) {
	return g.base.legend(f, geom.SwatchBox)
}

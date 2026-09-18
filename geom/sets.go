package geom

import (
	"errors"
	"fmt"
	"strings"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// The two marks an UpSet plot is made of, and the arithmetic they share.
//
// An UpSet is a matrix chart rather than a relational layout: nothing is laid
// out, the elements are counted by which sets they are in and the rest is a bar
// chart over a dot matrix. Both halves read the same membership table and run
// the same [stat.Intersections], so the column a bar stands over and the column
// of dots under it cannot disagree — which is the only thing that makes the
// chart readable. See docs/adr/0074-sets-are-counted.md.

// ErrTooManySets reports a membership table with more sets than the mark can
// draw: more than [stat.MaxSets] for an UpSet, more than three for a [Venn].
var ErrTooManySets = errors.New("figure/geom: this mark cannot draw that many sets")

// SetSeparator joins the names of the sets in one combination into the category
// an UpSet's axis carries.
//
// It is the mathematical symbol rather than a comma because the column is an
// intersection and not a list: the elements under "A ∩ B" are in both, and the
// ones in A alone are under "A".
const SetSeparator = " ∩ "

// membership is a resolved membership table: which element each row names, and
// which set it puts that element in.
//
// It is two interners rather than the one [edges] uses, and that is the
// difference between a membership and a graph edge. Both ends of a graph edge
// are nodes of one kind, so one namespace is right; an element and a set are
// different kinds of thing, and a customer called "mail" and a product called
// "mail" are two names rather than one node named twice.
type membership struct {
	elems, sets interner
	elem, set   []int

	x     stat.Intersections
	order []int    // the combinations to draw, in drawing order
	keys  []string // one category per drawn combination
	sb    strings.Builder
}

func (m *membership) reset(src data.Source, c config) error {
	if src == nil {
		return errors.New("figure/geom: nil data source")
	}
	if c.fromCol == "" || c.toCol == "" {
		return fmt.Errorf("%w: this mark reads a membership table; name the element with geom.From and the set it is in with geom.To", ErrNoColumn)
	}
	elems, ok := data.Labels(src, c.fromCol)
	if !ok {
		return fmt.Errorf("%w: %q", ErrNoColumn, c.fromCol)
	}
	sets, ok := data.Labels(src, c.toCol)
	if !ok {
		return fmt.Errorf("%w: %q", ErrNoColumn, c.toCol)
	}
	if len(elems) != len(sets) {
		return errLength(c.fromCol, c.toCol, len(elems), len(sets))
	}
	n := len(elems)

	m.elems.reset()
	m.sets.reset()
	m.elem, m.set = grow(m.elem, n), grow(m.set, n)
	for i := range n {
		// Element and set are interned row by row, so both orders are the
		// order a reader meets the names going down the table — the sets take
		// their lanes and their colours in that order.
		m.elem[i] = m.elems.of(elems[i], i)
		m.set[i] = m.sets.of(sets[i], i)
	}
	if m.sets.count() > stat.MaxSets {
		return fmt.Errorf("%w: %d sets, and a combination of them is one bit each of %d",
			ErrTooManySets, m.sets.count(), stat.MaxSets)
	}

	m.x.Reset(m.elem, m.set, m.elems.count(), m.sets.count())
	m.arrange(c)
	return nil
}

// arrange decides which combinations are drawn and in what order.
//
// [OrderValue] is the default here and nowhere else in this package, and the
// reason is that it is the form's own reading: an UpSet plot is a ranking of
// how the sets overlap, and one in table order is a bar chart whose bars are in
// no order at all. [Order] is how a caller asks for the table's order back, and
// the sort is stable, so the ranking never depends on how a tie was broken.
func (m *membership) arrange(c config) {
	m.order = grow(m.order, len(m.x.Combinations))[:0]
	for i := range m.x.Combinations {
		m.order = append(m.order, i)
	}
	if !c.orderSet || c.order != OrderAppearance {
		insertionSortBy(m.order, func(a, b int) bool {
			return m.x.Combinations[a].Count > m.x.Combinations[b].Count
		})
	}
	if c.top > 0 && len(m.order) > c.top {
		m.order = m.order[:c.top]
	}
	m.keys = grow(m.keys, len(m.order))[:0]
	for _, i := range m.order {
		m.keys = append(m.keys, m.key(m.x.Combinations[i]))
	}
}

// key names one combination: the sets in it, in the order the table named them,
// joined by [SetSeparator].
func (m *membership) key(c stat.Combination) string {
	m.sb.Reset()
	for i, name := range m.sets.keys {
		if !c.Has(i) {
			continue
		}
		if m.sb.Len() > 0 {
			m.sb.WriteString(SetSeparator)
		}
		m.sb.WriteString(name)
	}
	if m.sb.Len() == 0 {
		return "∅"
	}
	return m.sb.String()
}

// insertionSortBy sorts in place and keeps equal elements in the order they
// came in.
//
// A stable sort by a total key is what makes a ranking a pure function of its
// input — docs/adr/0072-layered-graph-layout.md's first claim, applied to a
// list whose ties are the common case, because two combinations of the same
// size are exactly what an UpSet plot is full of. It is an insertion sort
// rather than sort.SliceStable because that one allocates per call and this one
// runs in Train, on a list as long as the chart has columns.
func insertionSortBy(idx []int, less func(a, b int) bool) {
	for i := 1; i < len(idx); i++ {
		v := idx[i]
		j := i - 1
		for j >= 0 && less(v, idx[j]) {
			idx[j+1] = idx[j]
			j--
		}
		idx[j+1] = v
	}
}

// categorical is the axis a column of combinations or a lane of sets needs.
func categorical(s scale.Scale, what string) (scale.Categorical, error) {
	cat, ok := s.(scale.Categorical)
	if !ok {
		return nil, fmt.Errorf("%w: %s are names rather than quantities; give that axis a scale.Ordinal",
			ErrCategorical, what)
	}
	return cat, nil
}

// Intersections draws one bar per combination of sets: how many elements are in
// exactly those sets and no others.
//
// It is the top half of an UpSet plot, and the bottom half is [SetMatrix] in a
// [github.com/timzifer/figure.Plot.Track] under it, sharing the X scale:
//
//	sets := scale.Ordinal()
//	p := figure.New()
//	p.X(sets).Y(scale.Linear())
//	p.Add(geom.Intersections(src, geom.From("customer"), geom.To("product")))
//	p.Track(figure.Bottom, figure.TrackSize(90)).
//		Add(geom.SetMatrix(src, geom.From("customer"), geom.To("product")))
//
// The table is a membership list: one row per (element, set) pair, which is the
// shape a join already has. [From] names the element and [To] the set — a
// membership is a bipartite edge, so these are ADR 0039's channels unchanged.
//
// # Exactly those sets
//
// A bar counts the elements in *exactly* its combination. An element in A and B
// is one element of the "A ∩ B" bar and is not counted again under "A", so the
// bars partition the elements and add up to how many there are. That is the
// reading a Venn diagram is usually drawn wrong for, and it is the whole reason
// the form exists.
//
// # Both halves must be told the same thing
//
// [Order] and [Top] decide which columns there are and in what order, and the
// two marks are two layers rather than one: hand them the same options, exactly
// as a flat [Contour] and a projected scene are handed the same [Levels]. The
// default is the biggest combination first, which is the form's convention.
func Intersections(src data.Source, opts ...Option) Geom {
	return &intersectionsGeom{src: src, cfg: newConfig(opts)}
}

type intersectionsGeom struct {
	src data.Source
	cfg config
	m   membership
	err error

	// xs is the encoded position of each drawn column, filled in Train because
	// that is where the axis learns its categories.
	xs []float64
}

func (g *intersectionsGeom) Train(t Training) error {
	g.err = g.resolve(t)
	return g.err
}

func (g *intersectionsGeom) resolve(t Training) error {
	if err := g.m.reset(g.src, g.cfg); err != nil {
		return err
	}
	cat, err := categorical(t.X, "combinations of sets")
	if err != nil {
		return err
	}
	g.xs = grow(g.xs, len(g.m.order))[:0]
	for k, i := range g.m.order {
		g.xs = append(g.xs, cat.Encode(g.m.keys[k]))
		t.Y.Train(float64(g.m.x.Combinations[i].Count))
	}
	// A bar is read as the distance from the baseline, so the baseline has to
	// be inside the domain — [Histogram]'s rule, for the same reason.
	t.Y.Train(g.cfg.baseline)
	return nil
}

func (g *intersectionsGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	fill := g.cfg.colorFor(f)
	if g.cfg.fill != nil {
		fill = *g.cfg.fill
	}
	if g.cfg.opacity >= 0 {
		fill = ir.Fade(fill, clamp01(g.cfg.opacity))
	}
	if fill.A == 0 || len(g.m.order) == 0 {
		return nil
	}

	sc := acquire(f)
	defer sc.release()

	cd := f.Coords()
	base := baselinePos(f, g.cfg.baseline)
	half := slotHalfWidth(f, len(g.m.order), g.cfg.barWidth)
	sc.fill.Reset()
	for k, i := range g.m.order {
		x := f.X.Map(g.xs[k])
		y := f.Y.Map(float64(g.m.x.Combinations[i].Count))
		y0, y1 := y, base
		if y1 < y0 {
			y0, y1 = y1, y0
		}
		areaRound(&sc.fill, cd, ir.R(x-half, y0, x+half, y1), ir.Point{}, g.cfg.corner)
	}
	if sc.fill.Empty() {
		return nil
	}
	g.cfg.fillMark(b, &sc.fill, f, 0, fill)
	return nil
}

// slotHalfWidth is half the width of one column, in device units.
//
// A band scale knows it — that is what [scale.Band] is for, and it is what an
// ordinal axis is. A caller who gave the axis some other categorical scale gets
// an even share of the panel instead, which is a bar rather than a mark of no
// width at all.
func slotHalfWidth(f Frame, columns int, frac float64) float32 {
	if frac <= 0 || frac > 1 {
		frac = 0.8
	}
	if band, ok := f.X.(scale.Band); ok {
		return band.Bandwidth() / 2 * float32(frac) / 0.8
	}
	if columns < 1 {
		columns = 1
	}
	return f.Area.Dx() / float32(columns) / 2 * float32(frac)
}

func (g *intersectionsGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil || g.cfg.label == "" {
		return LegendEntry{}, false
	}
	col := g.cfg.colorFor(f)
	if g.cfg.fill != nil {
		col = *g.cfg.fill
	}
	return g.cfg.boxSwatch(f, g.cfg.label, col), true
}

func (g *intersectionsGeom) Source() data.Source { return g.src }

func (g *intersectionsGeom) Subset(rows []int) Geom {
	return &intersectionsGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *intersectionsGeom) Describe() Desc {
	d := g.cfg.describe(MarkIntersections)
	d.Source = g.src
	return d
}

// SetMatrix draws which sets each combination is: a dot per set in it, a muted
// dot per set it leaves out, and a line joining the dots of one column.
//
// It is the bottom half of an UpSet plot — see [Intersections], which is the
// top half and which this must be given the same [Order] and [Top] as. The X
// axis is the combinations and the Y axis is the sets, both ordinal, and the
// usual place for it is a track under the bars sharing their X scale object.
//
// Drawn on its own it is a membership matrix, which is a chart in its own right
// when what is being asked is which things go together rather than how many.
func SetMatrix(src data.Source, opts ...Option) Geom {
	return &setMatrixGeom{src: src, cfg: newConfig(opts)}
}

type setMatrixGeom struct {
	src data.Source
	cfg config
	m   membership
	err error

	xs, ys []float64
	on     []ir.Point
	off    []ir.Point
}

func (g *setMatrixGeom) Train(t Training) error {
	g.err = g.resolve(t)
	return g.err
}

func (g *setMatrixGeom) resolve(t Training) error {
	if err := g.m.reset(g.src, g.cfg); err != nil {
		return err
	}
	x, err := categorical(t.X, "combinations of sets")
	if err != nil {
		return err
	}
	y, err := categorical(t.Y, "sets")
	if err != nil {
		return err
	}
	g.xs = grow(g.xs, len(g.m.order))[:0]
	for k := range g.m.order {
		g.xs = append(g.xs, x.Encode(g.m.keys[k]))
	}
	// Every set takes a lane, including one that is in no drawn column: a lane
	// missing from the matrix would silently change what the columns mean.
	g.ys = grow(g.ys, g.m.sets.count())[:0]
	for _, name := range g.m.sets.keys {
		g.ys = append(g.ys, y.Encode(name))
	}
	return nil
}

func (g *setMatrixGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	if len(g.m.order) == 0 || len(g.ys) == 0 {
		return nil
	}
	col := g.cfg.colorFor(f)
	if col.A == 0 {
		return nil
	}
	// The muted dot is the one that says a set is *not* in this combination,
	// so it has to read as an absence rather than as a second series. The grid
	// colour is what the theme already uses for "structure, not data".
	absent := f.Theme.GridColor

	sc := acquire(f)
	defer sc.release()
	cd := f.Coords()

	g.on, g.off = g.on[:0], g.off[:0]
	sc.fill.Reset()
	for k, i := range g.m.order {
		c := g.m.x.Combinations[i]
		lo, hi := -1, -1
		for s := range g.ys {
			p := cd.Point(f.X.Map(g.xs[k]), f.Y.Map(g.ys[s]))
			if c.Has(s) {
				g.on = append(g.on, p)
				if lo < 0 {
					lo = s
				}
				hi = s
			} else {
				g.off = append(g.off, p)
			}
		}
		if lo >= 0 && hi > lo {
			// One connector per column, drawn from the first member lane to
			// the last: it is what makes a column read as one combination
			// rather than as a scatter of dots.
			a := cd.Point(f.X.Map(g.xs[k]), f.Y.Map(g.ys[lo]))
			z := cd.Point(f.X.Map(g.xs[k]), f.Y.Map(g.ys[hi]))
			sc.fill.MoveTo(a.X, a.Y)
			cd.Edge(&sc.fill, a, z)
		}
	}

	size := pick(g.cfg.size, f.Theme.MarkerSize)
	marker := g.cfg.markerFor(f)
	if len(g.off) > 0 && absent.A != 0 {
		b.Markers(marker, g.off, ir.MarkerStyle{Size: size, Fill: absent})
	}
	if !sc.fill.Empty() {
		b.StrokePath(&sc.fill, ir.Stroke{Color: col, Width: pick(g.cfg.width, f.Theme.LineWidth)})
	}
	if len(g.on) > 0 {
		b.Markers(marker, g.on, ir.MarkerStyle{Size: size, Fill: col})
	}
	return nil
}

func (g *setMatrixGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil || g.cfg.label == "" {
		return LegendEntry{}, false
	}
	return g.cfg.boxSwatch(f, g.cfg.label, g.cfg.colorFor(f)), true
}

func (g *setMatrixGeom) Source() data.Source { return g.src }

func (g *setMatrixGeom) Subset(rows []int) Geom {
	return &setMatrixGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *setMatrixGeom) Describe() Desc {
	d := g.cfg.describe(MarkSetMatrix)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*intersectionsGeom)(nil)
	_ Faceter   = (*intersectionsGeom)(nil)
	_ Describer = (*setMatrixGeom)(nil)
	_ Faceter   = (*setMatrixGeom)(nil)
)

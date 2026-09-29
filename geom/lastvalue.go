package geom

import (
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
)

// LastValue writes the newest value of a column on its axis: a tag on the
// value axis at the value of the row with the greatest position, and a rule
// across the panel to it — a trading screen's last price.
//
//	geom.LastValue(src, geom.X("start"), geom.Y("close"), geom.DirectionBy("", "close"))
//
// The newest row is the one furthest along X, whatever order the rows are in,
// so a layer over a stream follows its open row as it is revised. The colour
// is [Color], or the row's direction through [DirectionBy] — the tag is the
// rising colour when the last candle closed up — or the layer's palette
// colour. [Dash] styles the rule and [Rule] with false leaves the tag alone. A
// layer on [OnY2] tags the secondary axis.
//
// It trains the value axis on the newest value only: it annotates a series
// another layer draws, and should not widen the axis to rows it does not show.
// See docs/adr/0092-what-a-trading-screen-reads-off-its-edges.md.
func LastValue(src data.Source, opts ...Option) Geom {
	return &lastGeom{src: src, cfg: newConfig(append([]Option{Rule(true)}, opts...))}
}

// Rule turns the line a [LastValue] draws across the panel on or off. The
// default is on.
func Rule(on bool) Option { return func(c *config) { c.rule = on } }

type lastGeom struct {
	src  data.Source
	cfg  config
	s    series
	last int // the newest row, or -1
	err  error
}

func (g *lastGeom) Train(t Training) error {
	x, y := t.X, t.Y
	g.s, g.err = resolve(g.src, g.cfg, x, y)
	if g.err != nil {
		return g.err
	}
	g.last = -1
	for i := range g.s.x {
		if !defined(x, g.s.x[i]) || !defined(y, g.s.y[i]) {
			continue
		}
		if g.last < 0 || g.s.x[i] >= g.s.x[g.last] {
			g.last = i
		}
	}
	if g.last >= 0 {
		y.Train(g.s.y[g.last])
	}
	return nil
}

// color is the tag's and the rule's colour.
func (g *lastGeom) color(f Frame) ir.Color {
	if g.cfg.color != nil {
		return *g.cfg.color
	}
	if g.cfg.varying(g.s) && g.last >= 0 {
		if c := g.cfg.colorScale.Color(g.s.c[g.last]); c.A != 0 {
			return c
		}
	}
	return g.cfg.colorFor(f)
}

func (g *lastGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil || g.last < 0 || !g.cfg.rule {
		return g.err
	}
	v := g.s.y[g.last]
	if !defined(f.Y, v) {
		return nil
	}
	lo, hi := f.X.Domain()
	if lo > hi {
		lo, hi = hi, lo
	}
	cd := f.Coords()
	y := f.Y.Map(v)
	var run ir.Path
	strokeRun(b, cd, &run, []ir.Point{cd.Point(f.X.Map(lo), y), cd.Point(f.X.Map(hi), y)}, ir.Stroke{
		Color: g.color(f),
		Width: pick(g.cfg.width, 1),
		Dash:  g.cfg.dash,
	}, false)
	return nil
}

func (g *lastGeom) Tags(f Frame) []AxisTag {
	if g.err != nil || g.last < 0 {
		return nil
	}
	axis := TagY
	if g.cfg.onY2 {
		axis = TagY2
	}
	return []AxisTag{{Axis: axis, Value: g.s.y[g.last], Color: g.color(f)}}
}

// Legend is only for a layer that was given a label: a last value annotates a
// series another layer names.
func (g *lastGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil || g.cfg.label == "" {
		return LegendEntry{}, false
	}
	return LegendEntry{Label: g.cfg.label, Color: g.color(f), Kind: SwatchLine, Width: pick(g.cfg.width, 1)}, true
}

func (g *lastGeom) Source() data.Source { return g.src }
func (g *lastGeom) Subset(rows []int) Geom {
	return &lastGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *lastGeom) Describe() Desc {
	d := g.cfg.describe(MarkLastValue)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*lastGeom)(nil)
	_ Faceter   = (*lastGeom)(nil)
	_ Tagger    = (*lastGeom)(nil)
)

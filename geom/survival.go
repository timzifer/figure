package geom

import (
	"fmt"
	"slices"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// Survival draws a Kaplan–Meier survival curve of the X column: the estimated
// probability of surviving past each time, as the step function it is.
//
//	geom.Survival(src, geom.X("weeks"), geom.Event("relapsed"), geom.GroupBy("arm"),
//	    geom.Confidence(0.95), geom.CensorMarks(true))
//
// X is each subject's time — to the event, or to the last moment it was seen —
// and [Event] names the column that says which: non-zero for an event, zero for
// a subject censored at that time. A layer that names no event column treats
// every row as an event, which is an ECDF read from the top. The estimator runs
// in Train, for the reason [ECDF]'s does: the Y axis is the curve's own, 0 to
// 1, and the Y column is not read.
//
// The curve starts at time zero with every subject alive, as a survival curve
// is printed, unless the X axis is a time scale or has no position for zero —
// then it starts at the first observation.
//
// # What the layer adds, and what it does not
//
// [Confidence] shades the log-log pointwise band at the given level, and
// [CensorMarks] ticks the curve wherever a subject left observation. Both are
// off by default and both are opt-ins on the same layer, the way [Outliers] is
// on a boxplot.
//
// The numbers-at-risk table printed under a published curve is not the mark's.
// It is a table on the shared time axis under the panel, which is a
// [github.com/timzifer/figure.Track], and a mark cannot make a panel — see
// docs/adr/0054-statistical-instruments.md. The counts are in
// [stat.SurvivalPoint], and the caller puts them in a track with [Text].
//
// Given [GroupBy] it draws one curve per series, which is how two arms of a
// trial are compared.
func Survival(src data.Source, opts ...Option) Geom {
	return &survivalGeom{src: src, cfg: newConfig(opts)}
}

// Event names the column a [Survival] layer reads its event indicator from:
// non-zero for a subject whose time ends in the event, zero for one censored
// at that time. A row whose indicator is NaN is missing, under the layer's
// missing-data policy.
func Event(col string) Option { return func(c *config) { c.eventCol = col } }

// Confidence shades a pointwise confidence band around a [Survival] curve, at
// the given level — 0.95 for 95 %. Zero, the default, draws none.
func Confidence(level float64) Option { return func(c *config) { c.confidence = level } }

// CensorMarks ticks a [Survival] curve at every time a subject left
// observation without the event. The default is false.
func CensorMarks(show bool) Option { return func(c *config) { c.censorMarks = show } }

// survivalBandOpacity is what a confidence band is painted at when the layer
// named no opacity: faint enough that two arms' bands overlapping still show
// both curves.
const survivalBandOpacity = 0.2

type survivalGeom struct {
	src    data.Source
	cfg    config
	s      series
	gs     groups
	ev     []float64
	curves [][]stat.SurvivalPoint
	groups []int
	start  float64 // where every curve begins
	pairs  []timed // one series' observations, sorted
	ts     []float64
	es     []bool
	all    []int
	err    error
}

// timed is one observation: its time and whether it ended in the event.
type timed struct {
	t     float64
	event bool
}

func (g *survivalGeom) Train(t Training) error {
	x, y := t.X, t.Y
	g.s, g.err = resolveOne(g.src, g.cfg, x)
	if g.err != nil {
		return g.err
	}
	if g.err = g.readEvents(); g.err != nil {
		return g.err
	}
	if err := g.s.checkMissing(g.cfg, x, x); err != nil {
		return err
	}
	if g.err = g.gs.train(g.src, g.s, g.cfg, x, x, NoStack); g.err != nil {
		return g.err
	}
	g.accumulate(x)

	trainColumn(x, g.s.x)
	g.start = 0
	_, temporal := x.(scale.Temporal)
	if temporal || !defined(x, 0) {
		g.start = firstTime(g.curves)
	} else {
		x.Train(0)
	}
	y.Train(0, 1)
	return nil
}

// readEvents reads the event column, or a column of ones when the layer named
// none.
func (g *survivalGeom) readEvents() error {
	n := len(g.s.x)
	if g.cfg.eventCol == "" {
		g.ev = grow(g.ev, n)
		for i := range g.ev {
			g.ev[i] = 1
		}
		return nil
	}
	ev, ok := data.Float64Column(g.src, g.cfg.eventCol)
	if !ok {
		return fmt.Errorf("%w: %q, which has to be a number: non-zero for an event, zero for a censored subject", ErrNoColumn, g.cfg.eventCol)
	}
	if len(ev) != n {
		return errLength(g.cfg.xcol, g.cfg.eventCol, n, len(ev))
	}
	g.ev = ev
	return nil
}

func (g *survivalGeom) accumulate(x scale.Scale) {
	n := max(len(g.gs.keys), 1)
	g.curves = g.curves[:0]
	g.groups = grow(g.groups, n)[:0]
	for _, grp := range g.seriesOf() {
		g.pairs = g.pairs[:0]
		for _, i := range g.rowsOfSeries(grp) {
			if defined(x, g.s.x[i]) && finite(g.ev[i]) {
				g.pairs = append(g.pairs, timed{g.s.x[i], g.ev[i] != 0})
			}
		}
		if len(g.pairs) == 0 {
			continue
		}
		slices.SortStableFunc(g.pairs, func(a, b timed) int {
			switch {
			case a.t < b.t:
				return -1
			case a.t > b.t:
				return 1
			}
			return 0
		})
		g.ts, g.es = grow(g.ts, len(g.pairs)), growBools(g.es, len(g.pairs))
		for k, p := range g.pairs {
			g.ts[k], g.es[k] = p.t, p.event
		}
		var curve []stat.SurvivalPoint
		if i := len(g.curves); i < cap(g.curves) {
			curve = g.curves[:i+1][i]
		}
		g.curves = append(g.curves, stat.AppendKaplanMeier(curve, g.ts, g.es))
		g.groups = append(g.groups, grp)
	}
}

func firstTime(curves [][]stat.SurvivalPoint) float64 {
	first := 0.0
	for k, c := range curves {
		if len(c) > 0 && (k == 0 || c[0].T < first) {
			first = c[0].T
		}
	}
	return first
}

func (g *survivalGeom) seriesOf() []int {
	if !g.gs.grouped() {
		return oneSeries
	}
	return g.gs.order
}

func (g *survivalGeom) rowsOfSeries(grp int) []int {
	if g.gs.grouped() {
		return g.gs.rows[grp]
	}
	g.all = grow(g.all, len(g.s.x))
	for i := range g.s.x {
		g.all[i] = i
	}
	return g.all
}

func (g *survivalGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	sc := acquire(f)
	defer sc.release()
	cd := f.Coords()

	z := 0.0
	if lv := g.cfg.confidence; lv > 0 && lv < 1 {
		z = stat.NormalQuantile((1 + lv) / 2)
	}
	for k, curve := range g.curves {
		col, dash := g.cfg.colorFor(f), g.cfg.dashFor(f)
		if g.gs.grouped() {
			col, dash = g.cfg.groupColor(f, &g.gs, g.groups[k]), g.cfg.groupDash(f, g.groups[k])
		}
		if z > 0 {
			g.band(b, f, sc, cd, curve, z, g.cfg.fillOf(col, survivalBandOpacity))
		}
		stroke := ir.Stroke{
			Color: col,
			Width: pick(g.cfg.width, f.Theme.LineWidth),
			Cap:   ir.CapButt,
			Join:  ir.JoinMiter,
			Dash:  dash,
		}
		g.staircase(b, f, sc, cd, curve, stroke)
		if g.cfg.censorMarks {
			g.ticks(b, f, sc, cd, curve, stroke)
		}
	}
	return nil
}

// staircase draws the curve: level at each estimate until the next event
// time, then straight down to the new one.
func (g *survivalGeom) staircase(b ir.Backend, f Frame, sc *scratch, cd coord.Coord, curve []stat.SurvivalPoint, stroke ir.Stroke) {
	if len(curve) == 0 || !stroke.Visible() {
		return
	}
	sc.sx, sc.sy = sc.sx[:0], sc.sy[:0]
	at, prev := g.start, 1.0
	sc.sx, sc.sy = append(sc.sx, f.X.Map(at)), append(sc.sy, f.Y.Map(prev))
	for _, p := range curve {
		sc.sx, sc.sy = append(sc.sx, f.X.Map(p.T)), append(sc.sy, f.Y.Map(prev))
		if p.S != prev {
			sc.sx, sc.sy = append(sc.sx, f.X.Map(p.T)), append(sc.sy, f.Y.Map(p.S))
			prev = p.S
		}
	}
	pts := cd.Points(grow(sc.pts, len(sc.sx))[:0], sc.sx, sc.sy)
	sc.pts = pts
	strokeRun(b, cd, &sc.line, pts, stroke, false)
}

// band fills the confidence band as one stepped polygon: along the upper
// bound, and back along the lower.
func (g *survivalGeom) band(b ir.Backend, f Frame, sc *scratch, cd coord.Coord, curve []stat.SurvivalPoint, z float64, col ir.Color) {
	if len(curve) == 0 || col.A == 0 {
		return
	}
	// The upper edge is walked forwards into sx/sy and the lower edge into
	// the second half of the same buffers, which are then read end to end.
	n := 2*len(curve) + 1
	sc.sx, sc.sy = grow(sc.sx, 2*n)[:0], grow(sc.sy, 2*n)[:0]
	edge := func(upper bool) {
		prev := 1.0
		sc.sx, sc.sy = append(sc.sx, f.X.Map(g.start)), append(sc.sy, f.Y.Map(prev))
		for _, p := range curve {
			lo, hi := p.Band(z)
			v := lo
			if upper {
				v = hi
			}
			sc.sx, sc.sy = append(sc.sx, f.X.Map(p.T), f.X.Map(p.T)), append(sc.sy, f.Y.Map(prev), f.Y.Map(v))
			prev = v
		}
	}
	edge(true)
	half := len(sc.sx)
	edge(false)
	slices.Reverse(sc.sx[half:])
	slices.Reverse(sc.sy[half:])

	pts := cd.Points(grow(sc.pts, len(sc.sx))[:0], sc.sx, sc.sy)
	sc.pts = pts
	if len(pts) < 3 {
		return
	}
	sc.fill.Reset()
	appendEdges(&sc.fill, cd, pts, true)
	closeLoop(&sc.fill, cd, pts)
	g.cfg.fillMark(b, &sc.fill, f, 0, col)
}

// ticks marks every time at which a subject was censored, on the curve.
func (g *survivalGeom) ticks(b ir.Backend, f Frame, sc *scratch, cd coord.Coord, curve []stat.SurvivalPoint, stroke ir.Stroke) {
	sc.sx, sc.sy = sc.sx[:0], sc.sy[:0]
	for _, p := range curve {
		if p.Censored > 0 {
			sc.sx, sc.sy = append(sc.sx, f.X.Map(p.T)), append(sc.sy, f.Y.Map(p.S))
		}
	}
	if len(sc.sx) == 0 {
		return
	}
	pts := cd.Points(grow(sc.pts, len(sc.sx))[:0], sc.sx, sc.sy)
	sc.pts = pts
	b.Markers(ir.MarkerPlus, pts, ir.MarkerStyle{
		Size:   pick(g.cfg.size, 2*stroke.Width+4),
		Stroke: ir.Stroke{Color: stroke.Color, Width: stroke.Width},
	})
}

func (g *survivalGeom) Legends(f Frame) []LegendEntry {
	if g.err != nil {
		return nil
	}
	return LegendsOr(g, f, g.cfg.legends(f, &g.gs, g.s, SwatchLine))
}

func (g *survivalGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil {
		return LegendEntry{}, false
	}
	return LegendEntry{
		Label: g.cfg.labelForX(),
		Color: g.cfg.colorFor(f),
		Kind:  SwatchLine,
		Dash:  g.cfg.dashFor(f),
		Width: pick(g.cfg.width, f.Theme.LineWidth),
	}, true
}

func (g *survivalGeom) Source() data.Source { return g.src }

func (g *survivalGeom) Subset(rows []int) Geom {
	return &survivalGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *survivalGeom) Describe() Desc {
	d := g.cfg.describe(MarkSurvival)
	d.Source = g.src
	return d
}

func growBools(buf []bool, n int) []bool {
	if cap(buf) < n {
		return make([]bool, n)
	}
	return buf[:n]
}

var (
	_ Describer = (*survivalGeom)(nil)
	_ Faceter   = (*survivalGeom)(nil)
	_ Legender  = (*survivalGeom)(nil)
)

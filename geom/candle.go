package geom

import (
	"errors"
	"fmt"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// Candle draws a candlestick per row: a wick from the row's low to its high,
// and a body from its open to its close, coloured by which way the row went.
//
//	geom.Candle(src, geom.X("start"), geom.X2("end"),
//		geom.OHLC("open", "high", "low", "close"))
//
// The four values are one option, [OHLC], because a row is a candle only with
// all four of them. The position is [X], with [X2] when the row names both
// edges of its period — which is what a candle out of
// [github.com/timzifer/figure/stat.OHLC] knows, and what
// [github.com/timzifer/figure.CandleTable] writes. With X alone the slot is the
// closest spacing in the data or a band scale's width, as a [Bar]'s is, and
// the body takes [BarWidth] of it — 0.7 by default rather than a bar's 0.8,
// because the gap between two candles is where their wicks are told apart.
//
// # Direction
//
// A row is rising when its close is at or above its open, and falling
// otherwise; [Direction] with [SincePrevious] compares with the previous row's
// close instead. A close equal to its open is rising, and its body is drawn at
// least a device pixel tall. The direction is drawn twice: in colour —
// [Rising] and [Falling], defaulting to the theme's — and, with [Hollow], as
// an outlined body for a rising row and a filled one for a falling row. Hollow
// is on by default under a theme with redundant encoding, so the direction
// survives a greyscale print. [ColorBy] replaces the direction's colours with
// a column's, and leaves hollow and filled to say which way the row went.
//
// # What it draws, and what it does not repair
//
// The wicks of one colour are one stroked path and the bodies one filled path,
// so a thousand candles are the same handful of drawing calls as ten. A row
// missing any of its four values is skipped, and is an error under
// OnMissing(Error). A row whose data contradicts itself — a high below its
// close — is drawn as given: repairing it would hide an error in the data,
// and a candle is where a reader looks for one.
//
// [CandleStyle] with [Ticks] draws the OHLC bar instead: a rule from low to
// high with a tick left at the open and right at the close. See
// docs/adr/0088-a-candle-is-one-mark-that-reads-four-values.md.
func Candle(src data.Source, opts ...Option) Geom {
	return &candleGeom{src: src, cfg: newConfig(append([]Option{BarWidth(defaultCandleWidth)}, opts...))}
}

// defaultCandleWidth is the share of its slot a candle's body takes.
const defaultCandleWidth = 0.7

// OHLC names a candle's four value columns, in the order the name spells
// them.
func OHLC(open, high, low, close string) Option {
	return func(c *config) { c.ohlc = [4]string{open, high, low, close} }
}

// CandleDirection is what a candle's direction is measured against.
type CandleDirection uint8

const (
	// SinceOpen is the candle's own statement: it rose if it closed at or
	// above where it opened.
	SinceOpen CandleDirection = iota
	// SincePrevious compares a row's close with the previous row's, so a day
	// that opened low and rallied still reads as down if it closed below
	// yesterday. The first row has no previous close and uses its own open.
	SincePrevious
)

// Direction sets what a [Candle]'s direction is measured against. The default
// is [SinceOpen].
func Direction(d CandleDirection) Option { return func(c *config) { c.direction = d } }

// candleSide is one direction's colour and legend label, as [Rising] and
// [Falling] set them.
type candleSide struct {
	color *ir.Color
	label string
}

// Rising sets the colour and the legend label of a [Candle]'s rising rows.
// The defaults are the theme's Rising colour and "rising"; an empty label
// keeps the default.
func Rising(col ir.Color, label string) Option {
	return func(c *config) { c.rising = candleSide{&col, label} }
}

// Falling is [Rising] for the falling rows. The defaults are the theme's
// Falling colour and "falling".
func Falling(col ir.Color, label string) Option {
	return func(c *config) { c.falling = candleSide{&col, label} }
}

// Hollow draws a [Candle]'s rising bodies as outlines and its falling bodies
// filled, the convention of a chart printed in one colour. The default is on
// under a theme with redundant encoding and off otherwise.
func Hollow(on bool) Option { return func(c *config) { c.hollow, c.hollowSet = on, true } }

// CandleStyleKind is how a [Candle] draws a row.
type CandleStyleKind uint8

const (
	// Bodies is the candlestick: a wick and a body.
	Bodies CandleStyleKind = iota
	// Ticks is the OHLC bar: a rule from low to high, a tick to the left at
	// the open and one to the right at the close.
	Ticks
)

// CandleStyle sets how a [Candle] draws a row. The default is [Bodies].
func CandleStyle(s CandleStyleKind) Option { return func(c *config) { c.candleStyle = s } }

// ErrNoOHLC reports a [Candle] layer that was not told its four columns.
var ErrNoOHLC = errors.New("figure/geom: a candle needs geom.OHLC(open, high, low, close)")

type candleGeom struct {
	src data.Source
	cfg config
	// s is the position and the close, with the colour column and the row
	// bookkeeping a series carries; o, h and l are the other three values.
	s       series
	x2      []float64
	o, h, l []float64
	up      []bool
	gap     float64
	gaps    []float64
	err     error
}

func (g *candleGeom) Train(t Training) error {
	x, y := t.X, t.Y
	c := g.cfg
	for _, col := range c.ohlc {
		if col == "" {
			g.err = ErrNoOHLC
			return g.err
		}
	}
	rc := c
	rc.ycol, rc.y2col = c.ohlc[3], ""
	if g.s, g.err = resolve(g.src, rc, x, y); g.err != nil {
		return g.err
	}
	n := len(g.s.x)
	read := func(col string, s scale.Scale) ([]float64, error) {
		vs, err := column(g.src, col, s)
		if err == nil && len(vs) != n {
			err = errLength(c.xcol, col, n, len(vs))
		}
		return vs, err
	}
	if g.o, g.err = read(c.ohlc[0], y); g.err != nil {
		return g.err
	}
	if g.h, g.err = read(c.ohlc[1], y); g.err != nil {
		return g.err
	}
	if g.l, g.err = read(c.ohlc[2], y); g.err != nil {
		return g.err
	}
	g.x2 = nil
	if c.x2col != "" {
		if g.x2, g.err = read(c.x2col, x); g.err != nil {
			return g.err
		}
	}

	ok := g.plottable(x, y)
	if c.missing == Error {
		for i, fine := range ok {
			if !fine {
				g.err = fmt.Errorf("figure/geom: candle row %d has no position on the scales and OnMissing(Error) is set", i)
				return g.err
			}
		}
	}
	g.up = g.directions(ok)

	g.gap, g.gaps = smallestGap(g.gaps, g.s.x)
	half := g.gap * g.widthFraction() / 2
	w := t.Within
	for i, fine := range ok {
		if !fine {
			continue
		}
		x.Train(g.s.x[i])
		a, b := g.s.x[i]-half, g.s.x[i]+half
		if g.x2 != nil {
			x.Train(g.x2[i])
			a, b = min(g.s.x[i], g.x2[i]), max(g.s.x[i], g.x2[i])
		}
		// A candle is in view when any of its period is, so one half in view
		// still fits.
		if w != nil && (b < w.Lo || a > w.Hi) {
			continue
		}
		// All four, so that a row whose data contradicts itself still has its
		// whole drawing inside the axis.
		y.Train(g.o[i], g.h[i], g.l[i], g.s.y[i])
	}
	g.cfg.trainColors(g.s)
	if _, band := x.(scale.Band); band || g.x2 != nil {
		return nil
	}
	widen(x, g.s.x, g.gap/2*g.widthFraction())
	return nil
}

// plottable is which rows have all four values and a position.
func (g *candleGeom) plottable(x, y scale.Scale) []bool {
	ok := make([]bool, len(g.s.x))
	for i := range ok {
		ok[i] = defined(x, g.s.x[i]) && defined(y, g.o[i]) && defined(y, g.h[i]) &&
			defined(y, g.l[i]) && defined(y, g.s.y[i]) && (g.x2 == nil || defined(x, g.x2[i]))
	}
	return ok
}

// directions is which rows rose, by the layer's [CandleDirection].
func (g *candleGeom) directions(ok []bool) []bool {
	up := make([]bool, len(ok))
	prev, have := 0.0, false
	for i, fine := range ok {
		if !fine {
			continue
		}
		ref := g.o[i]
		if g.cfg.direction == SincePrevious && have {
			ref = prev
		}
		up[i] = g.s.y[i] >= ref
		prev, have = g.s.y[i], true
	}
	return up
}

func (g *candleGeom) widthFraction() float64 {
	if g.cfg.barWidth <= 0 || g.cfg.barWidth > 1 {
		return defaultCandleWidth
	}
	return g.cfg.barWidth
}

// sideColor is one direction's colour: the layer's own, or the theme's.
func (g *candleGeom) sideColor(f Frame, up bool) ir.Color {
	side, def := g.cfg.falling, f.Theme.Falling
	if up {
		side, def = g.cfg.rising, f.Theme.Rising
	}
	if side.color != nil {
		return *side.color
	}
	if def.A == 0 {
		// A theme built before candles had colours of its own.
		if up {
			return candleRising
		}
		return candleFalling
	}
	return def
}

// The direction colours a theme without its own falls back to: Okabe–Ito's
// blue and vermilion, the pair the most common colour deficiency still tells
// apart.
var (
	candleRising  = ir.RGB(0x00, 0x72, 0xB2)
	candleFalling = ir.RGB(0xD5, 0x5E, 0x00)
)

func (g *candleGeom) sideLabel(up bool) string {
	if up {
		if g.cfg.rising.label != "" {
			return g.cfg.rising.label
		}
		return "rising"
	}
	if g.cfg.falling.label != "" {
		return g.cfg.falling.label
	}
	return "falling"
}

func (g *candleGeom) hollowIn(f Frame) bool {
	if g.cfg.hollowSet {
		return g.cfg.hollow
	}
	return len(f.Theme.SeriesDashes) > 0
}

// candleMark is one row, resolved to device space.
type candleMark struct {
	row               int
	x0, mid, x1       float32
	open, hi, lo, cls float32
	top, bot          float32
	col               ir.Color
	up                bool
}

func (g *candleGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	sc := acquire(f)
	defer sc.release()
	cd := f.Coords()
	ok := g.plottable(f.X, f.Y)

	rows := sc.rows[:0]
	for i, fine := range ok {
		if fine {
			rows = append(rows, i)
		}
	}
	sc.rows = rows
	if len(rows) == 0 {
		return nil
	}
	cols := sc.colorsFor(g.cfg, g.s, rows)
	frac := g.widthFraction()
	half := g.gap * frac / 2

	marks := make([]candleMark, 0, len(rows))
	for k, i := range rows {
		x0, x1 := spanOn(f.X, g.s.x, g.x2, i, half, true)
		if g.x2 != nil {
			// The row named its period; the body is its share of it.
			w := (x1 - x0) * float32(frac)
			c := (x0 + x1) / 2
			x0, x1 = c-w/2, c+w/2
		}
		m := candleMark{
			row: i, x0: x0, x1: x1, mid: (x0 + x1) / 2,
			open: f.Y.Map(g.o[i]), hi: f.Y.Map(g.h[i]), lo: f.Y.Map(g.l[i]), cls: f.Y.Map(g.s.y[i]),
			up: g.up[i],
		}
		m.top, m.bot = min(m.open, m.cls), max(m.open, m.cls)
		if m.bot-m.top < 1 {
			// A doji is a mark, not nothing.
			c := (m.top + m.bot) / 2
			m.top, m.bot = c-0.5, c+0.5
		}
		if cols != nil {
			m.col = cols[k]
		} else {
			m.col = g.sideColor(f, m.up)
		}
		marks = append(marks, m)
	}

	if f.tracking() {
		sc.pts = grow(sc.pts, len(marks))
		for k, m := range marks {
			sc.pts[k] = cd.Point(m.mid, m.cls)
		}
		f.Marks(MarkRows{At: sc.pts, Rows: sc.sourceRows(g.s, rows)})
	}

	hollow := g.hollowIn(f)
	stroke := func(col ir.Color) ir.Stroke {
		return ir.Stroke{Color: col, Width: pick(g.cfg.width, 1), Cap: ir.CapButt}
	}
	// One batch per colour and direction, in the order they first appear: the
	// wicks of a batch are one stroked path and its bodies one filled path.
	type batch struct {
		col ir.Color
		up  bool
	}
	var batches []batch
	for _, m := range marks {
		key := batch{m.col, m.up && hollow}
		seen := false
		for _, bt := range batches {
			if bt == key {
				seen = true
				break
			}
		}
		if !seen {
			batches = append(batches, key)
		}
	}
	var lines, bodies ir.Path
	for _, bt := range batches {
		lines.Reset()
		bodies.Reset()
		for _, m := range marks {
			if (batch{m.col, m.up && hollow}) != bt {
				continue
			}
			if g.cfg.candleStyle == Ticks {
				segment(&lines, cd, m.mid, m.hi, m.mid, m.lo)
				segment(&lines, cd, m.x0, m.open, m.mid, m.open)
				segment(&lines, cd, m.mid, m.cls, m.x1, m.cls)
				continue
			}
			// The wick in two halves, so a hollow body is empty inside.
			segment(&lines, cd, m.mid, min(m.hi, m.lo), m.mid, m.top)
			segment(&lines, cd, m.mid, m.bot, m.mid, max(m.hi, m.lo))
			areaRound(&bodies, cd, ir.R(m.x0, m.top, m.x1, m.bot), ir.Point{}, g.cfg.corner)
		}
		if !lines.Empty() {
			b.StrokePath(&lines, stroke(bt.col))
		}
		if bodies.Empty() {
			continue
		}
		if bt.up {
			b.StrokePath(&bodies, stroke(bt.col))
		} else {
			g.cfg.fillMark(b, &bodies, f, 0, bt.col)
		}
	}
	return nil
}

// segment appends a straight run from (x0, y0) to (x1, y1), placed by the
// coord at both ends. A wick is short, so it is drawn straight under any coord
// rather than bent along it.
func segment(p *ir.Path, cd coord.Coord, x0, y0, x1, y1 float32) {
	if y0 == y1 && x0 == x1 {
		return
	}
	a, z := cd.Point(x0, y0), cd.Point(x1, y1)
	p.MoveTo(a.X, a.Y).LineTo(z.X, z.Y)
}

func (g *candleGeom) Legends(f Frame) []LegendEntry {
	if g.err != nil {
		return nil
	}
	if g.cfg.varying(g.s) {
		var none groups
		return g.cfg.legends(f, &none, g.s, SwatchBox)
	}
	return []LegendEntry{
		g.cfg.boxSwatch(f, g.sideLabel(true), g.sideColor(f, true)),
		g.cfg.boxSwatch(f, g.sideLabel(false), g.sideColor(f, false)),
	}
}

func (g *candleGeom) Legend(f Frame) (LegendEntry, bool) {
	es := g.Legends(f)
	if len(es) == 0 {
		return LegendEntry{}, false
	}
	return es[0], true
}

func (g *candleGeom) ColorGuide() (ColorGuide, bool) { return g.cfg.colorGuide(g.s, g.err) }

func (g *candleGeom) Source() data.Source { return g.src }
func (g *candleGeom) Subset(rows []int) Geom {
	return &candleGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

func (g *candleGeom) Describe() Desc {
	d := g.cfg.describe(MarkCandle)
	d.Source = g.src
	return d
}

var (
	_ Describer = (*candleGeom)(nil)
	_ Faceter   = (*candleGeom)(nil)
	_ Guided    = (*candleGeom)(nil)
	_ Legender  = (*candleGeom)(nil)
)

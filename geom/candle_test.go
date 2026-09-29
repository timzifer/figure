package geom_test

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// candles is a small market: two rising rows, one falling, and a doji.
func candles() *data.Table {
	return data.NewTable().
		Float64("t", []float64{0, 1, 2, 3}).
		Float64("o", []float64{10, 12, 15, 13}).
		Float64("h", []float64{13, 16, 16, 14}).
		Float64("l", []float64{9, 11, 12, 12}).
		Float64("c", []float64{12, 15, 13, 13})
}

var ohlc = geom.OHLC("o", "h", "l", "c")

func count(rec *irtest.Recorder, op string) int { return rec.Count(op) }

// A thousand candles are the same handful of calls as four: two per
// direction, a stroke for the wicks and a fill for the bodies.
func TestACandleLayerDrawsInFourCallsWhateverItsRows(t *testing.T) {
	const n = 1000
	ts, o, h, l, c := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	for i := range n {
		v := 100 + 10*math.Sin(float64(i)/7)
		ts[i], o[i], c[i] = float64(i), v, v+math.Cos(float64(i))
		h[i], l[i] = max(o[i], c[i])+1, min(o[i], c[i])-1
	}
	src := data.NewTable().Float64("t", ts).Float64("o", o).Float64("h", h).Float64("l", l).Float64("c", c)
	g := geom.Candle(src, geom.X("t"), ohlc)
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 800, 400)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if s, fl := count(rec, "StrokePath"), count(rec, "FillPath"); s != 2 || fl != 2 {
		t.Errorf("%d strokes and %d fills for %d candles, want two of each", s, fl, n)
	}
}

// The direction is the mark's: rising rows in the rising colour, falling in
// the falling one, with no column computed by the caller.
func TestACandleDecidesItsOwnDirection(t *testing.T) {
	up, down := ir.RGB(1, 2, 3), ir.RGB(4, 5, 6)
	g := geom.Candle(candles(), geom.X("t"), ohlc, geom.Rising(up, "up"), geom.Falling(down, "down"))
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	fills := map[ir.Color]int{}
	for _, c := range rec.Calls {
		if c.Op == "FillPath" {
			fills[c.Fill.Color] += len(rectsOf(t, c))
		}
	}
	// Rows 0, 1 and the doji rose; row 2 fell.
	if fills[up] != 3 || fills[down] != 1 {
		t.Errorf("bodies by colour %v, want three rising and one falling", fills)
	}
	es := geom.Legends(g, f)
	if len(es) != 2 || es[0].Label != "up" || es[1].Label != "down" || es[0].Color != up {
		t.Errorf("legend %+v, want the two directions once", es)
	}
}

// Since the previous close, a row that closed under the one before falls even
// though it rose from its own open.
func TestSincePreviousComparesWithTheCloseBefore(t *testing.T) {
	src := data.NewTable().
		Float64("t", []float64{0, 1}).
		Float64("o", []float64{10, 5}).Float64("h", []float64{12, 9}).
		Float64("l", []float64{9, 4}).Float64("c", []float64{11, 8})
	up, down := ir.RGB(1, 2, 3), ir.RGB(4, 5, 6)
	g := geom.Candle(src, geom.X("t"), ohlc, geom.Direction(geom.SincePrevious),
		geom.Rising(up, ""), geom.Falling(down, ""))
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	var cols []ir.Color
	for _, c := range rec.Calls {
		if c.Op == "FillPath" {
			cols = append(cols, c.Fill.Color)
		}
	}
	// Row 0 has no previous close and rose from its open; row 1 rose from its
	// open but closed under row 0.
	if len(cols) != 2 || cols[0] != up || cols[1] != down {
		t.Errorf("body colours %v, want rising then falling", cols)
	}
}

// A doji's body is a mark rather than nothing.
func TestADojiIsVisible(t *testing.T) {
	g := geom.Candle(candles(), geom.X("t"), ohlc)
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	for _, r := range allRects(t, rec) {
		if r.Max.Y-r.Min.Y < 1-1e-4 {
			t.Errorf("a body %v less than a pixel tall", r)
		}
	}
}

// Hollow draws the rising bodies as outlines: no rising fill, and a stroke
// call more.
func TestHollowOutlinesTheRisingBodies(t *testing.T) {
	up := ir.RGB(1, 2, 3)
	g := geom.Candle(candles(), geom.X("t"), ohlc, geom.Rising(up, ""), geom.Hollow(true))
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Calls {
		if c.Op == "FillPath" && c.Fill.Color == up {
			t.Error("a rising body is filled")
		}
	}
	if n := count(rec, "StrokePath"); n != 3 {
		t.Errorf("%d strokes, want two for the wicks and one for the hollow bodies", n)
	}

	// And it is on by default under redundant encoding.
	g = geom.Candle(candles(), geom.X("t"), ohlc, geom.Rising(up, ""))
	rec, f = frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	f.Theme = theme.Light.With(theme.Redundant(true))
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Calls {
		if c.Op == "FillPath" && c.Fill.Color == up {
			t.Error("a rising body is filled under redundant encoding")
		}
	}
}

// The OHLC bar is a style of the same mark: rules and ticks, nothing filled.
func TestTheTickStyleDrawsNoBodies(t *testing.T) {
	g := geom.Candle(candles(), geom.X("t"), ohlc, geom.CandleStyle(geom.Ticks))
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if n := count(rec, "FillPath"); n != 0 {
		t.Errorf("%d fills, want none", n)
	}
	if n := count(rec, "StrokePath"); n != 2 {
		t.Errorf("%d strokes, want one per direction", n)
	}
}

// The row is reported at its close, in the middle of its slot.
func TestACandlesRowIsAtItsClose(t *testing.T) {
	g := geom.Candle(candles(), geom.X("t"), ohlc)
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	at := map[int]ir.Point{}
	f.Rows = rowsFunc(func(pts []ir.Point, rows []int) {
		for i, r := range rows {
			at[r] = pts[i]
		}
	})
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	closes := []float64{12, 15, 13, 13}
	for i, c := range closes {
		p, ok := at[i]
		if !ok {
			t.Fatalf("row %d not reported", i)
		}
		if want := f.Y.Map(c); math.Abs(float64(p.Y-want)) > 1e-3 {
			t.Errorf("row %d at y %v, want its close at %v", i, p.Y, want)
		}
		if want := f.X.Map(float64(i)); math.Abs(float64(p.X-want)) > 1e-3 {
			t.Errorf("row %d at x %v, want the middle of its slot at %v", i, p.X, want)
		}
	}
}

// A row whose high is under its close is drawn as given, and the axis still
// holds all of it.
func TestInconsistentDataIsDrawnAsGiven(t *testing.T) {
	src := data.NewTable().Float64("t", []float64{0, 1}).
		Float64("o", []float64{10, 10}).Float64("h", []float64{11, 11}).
		Float64("l", []float64{9, 9}).Float64("c", []float64{11, 20})
	g := geom.Candle(src, geom.X("t"), ohlc)
	y := scale.Linear()
	if err := g.Train(geom.Training{X: scale.Linear(), Y: y}); err != nil {
		t.Fatal(err)
	}
	if _, hi := y.Domain(); hi < 20 {
		t.Errorf("the axis stops at %v, under the close of 20", hi)
	}
}

func TestACandleWithoutItsFourColumnsIsRefused(t *testing.T) {
	g := geom.Candle(candles(), geom.X("t"))
	if err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); !errors.Is(err, geom.ErrNoOHLC) {
		t.Errorf("err = %v, want ErrNoOHLC", err)
	}
}

// A row missing a value is skipped, and refused under OnMissing(Error).
func TestAMissingValueSkipsTheRow(t *testing.T) {
	src := data.NewTable().Float64("t", []float64{0, 1}).
		Float64("o", []float64{10, 10}).Float64("h", []float64{12, math.NaN()}).
		Float64("l", []float64{9, 9}).Float64("c", []float64{11, 11})
	g := geom.Candle(src, geom.X("t"), ohlc)
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if n := len(allRects(t, rec)); n != 1 {
		t.Errorf("%d bodies, want the one complete row", n)
	}
	strict := geom.Candle(src, geom.X("t"), ohlc, geom.OnMissing(geom.Error))
	if err := strict.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); err == nil {
		t.Error("a missing high was not refused under OnMissing(Error)")
	}
}

// X2 names the period's far edge, and the body is its share of it.
func TestACandleWithAPeriodTakesItsShareOfIt(t *testing.T) {
	src := data.NewTable().Float64("s", []float64{0, 10}).Float64("e", []float64{10, 20}).
		Float64("o", []float64{1, 2}).Float64("h", []float64{3, 4}).
		Float64("l", []float64{0, 1}).Float64("c", []float64{2, 3})
	g := geom.Candle(src, geom.X("s"), geom.X2("e"), ohlc, geom.BarWidth(0.5))
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	for _, r := range allRects(t, rec) {
		if w := r.Max.X - r.Min.X; math.Abs(float64(w-100)) > 0.5 {
			t.Errorf("a body %v wide, want half of a 200-pixel period", w)
		}
	}
}

func TestACandleIsWrittenDown(t *testing.T) {
	up := ir.RGB(1, 2, 3)
	g := geom.Candle(candles(), geom.X("t"), ohlc, geom.Direction(geom.SincePrevious),
		geom.Rising(up, "bull"), geom.Hollow(true), geom.CandleStyle(geom.Ticks))
	d := g.(geom.Describer).Describe()
	back, err := geom.FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	again := back.(geom.Describer).Describe()
	if again.Mark != geom.MarkCandle || again.OHLC != [4]string{"o", "h", "l", "c"} ||
		again.Direction != geom.SincePrevious || again.Rising == nil || *again.Rising != up ||
		again.RisingLabel != "bull" || !again.Hollow || !again.HollowSet ||
		again.CandleStyle != geom.Ticks || again.BarWidth != 0.7 {
		t.Errorf("read back as %+v", again)
	}
}

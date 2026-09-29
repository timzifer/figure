package geom_test

import (
	"errors"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

var (
	upColor   = ir.RGB(1, 2, 3)
	downColor = ir.RGB(4, 5, 6)
)

// days is four rows: rising, falling, a tie, and falling again, with a close
// column that rises and falls against the previous row differently.
func days() *data.Table {
	return data.NewTable().
		Float64("t", []float64{0, 1, 2, 3}).
		Float64("o", []float64{10, 12, 13, 15}).
		Float64("c", []float64{12, 11, 13, 14}).
		Float64("v", []float64{5, 6, 7, 8})
}

// fillColours are the colours a layer's fill calls paint, one per rectangle,
// in drawing order.
func fillColours(t *testing.T, g geom.Geom) []ir.Color {
	t.Helper()
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	var out []ir.Color
	for _, c := range rec.Calls {
		if c.Op != "FillPath" {
			continue
		}
		for range rectsOf(t, c) {
			out = append(out, c.Fill.Color)
		}
	}
	return out
}

func colourCount(cols []ir.Color, c ir.Color) int {
	n := 0
	for _, x := range cols {
		if x == c {
			n++
		}
	}
	return n
}

// A bar takes the direction through its colour channel: rows 0 and 2 (a tie)
// rise, rows 1 and 3 fall.
func TestABarIsColouredByDirection(t *testing.T) {
	g := geom.Bar(days(), geom.X("t"), geom.Y("v"), geom.DirectionBy("o", "c"),
		geom.Rising(upColor, ""), geom.Falling(downColor, ""))
	cols := fillColours(t, g)
	if colourCount(cols, upColor) != 2 || colourCount(cols, downColor) != 2 {
		t.Errorf("bar colours %v, want two rising and two falling", cols)
	}
}

// With from empty, a row is compared with the previous close: 12 is the first
// and rises, 11 falls, 13 rises, 14 rises.
func TestTheDirectionSinceThePreviousRow(t *testing.T) {
	g := geom.Bar(days(), geom.X("t"), geom.Y("v"), geom.DirectionBy("", "c"),
		geom.Rising(upColor, ""), geom.Falling(downColor, ""))
	cols := fillColours(t, g)
	if colourCount(cols, upColor) != 3 || colourCount(cols, downColor) != 1 {
		t.Errorf("bar colours %v, want three rising and one falling", cols)
	}
}

// It agrees with a candle told the same: the candle's bodies and the bars
// under them are painted alike, row by row.
func TestADirectionAgreesWithACandle(t *testing.T) {
	src := data.NewTable().
		Float64("t", []float64{0, 1, 2, 3}).
		Float64("o", []float64{10, 12, 13, 15}).
		Float64("h", []float64{13, 13, 14, 16}).
		Float64("l", []float64{9, 10, 12, 13}).
		Float64("c", []float64{12, 11, 13, 14}).
		Float64("v", []float64{5, 6, 7, 8})
	for _, since := range []geom.CandleDirection{geom.SinceOpen, geom.SincePrevious} {
		from := "o"
		if since == geom.SincePrevious {
			from = ""
		}
		candle := fillColours(t, geom.Candle(src, geom.X("t"), geom.OHLC("o", "h", "l", "c"),
			geom.Direction(since), geom.Rising(upColor, ""), geom.Falling(downColor, "")))
		bars := fillColours(t, geom.Bar(src, geom.X("t"), geom.Y("v"), geom.DirectionBy(from, "c"),
			geom.Rising(upColor, ""), geom.Falling(downColor, "")))
		if colourCount(candle, upColor) != colourCount(bars, upColor) || colourCount(candle, downColor) != colourCount(bars, downColor) {
			t.Errorf("direction %v: candles %v, bars %v", since, candle, bars)
		}
	}
}

// A line takes it as it takes a discrete colour: its stretches are drawn in
// the two colours.
func TestALineIsDrawnInStretchesOfItsDirection(t *testing.T) {
	g := geom.Line(days(), geom.X("t"), geom.Y("c"), geom.DirectionBy("", "c"),
		geom.Rising(upColor, ""), geom.Falling(downColor, ""))
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	seen := map[ir.Color]bool{}
	for _, c := range rec.Calls {
		if c.Op == "Polyline" || c.Op == "StrokePath" {
			seen[c.Stroke.Color] = true
		}
	}
	if !seen[upColor] || !seen[downColor] {
		t.Errorf("stroke colours %v, want both directions", seen)
	}
}

func TestAScatterTakesTheDirection(t *testing.T) {
	g := geom.Scatter(days(), geom.X("t"), geom.Y("v"), geom.DirectionBy("o", "c"),
		geom.Rising(upColor, ""), geom.Falling(downColor, ""))
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	seen := map[ir.Color]int{}
	for _, c := range rec.Calls {
		if c.Op == "Markers" {
			seen[c.Style.Fill] += len(c.Points)
		}
	}
	if seen[upColor] != 2 || seen[downColor] != 2 {
		t.Errorf("markers by colour %v, want two of each", seen)
	}
}

// A row has one colour, so a layer that names two ways of choosing it is
// refused.
func TestColorByAndDirectionByAreExclusive(t *testing.T) {
	g := geom.Bar(days(), geom.X("t"), geom.Y("v"), geom.DirectionBy("o", "c"),
		geom.ColorBy("v", scale.Sequential(palette.Viridis)))
	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrColorAndDirection) {
		t.Errorf("err = %v, want ErrColorAndDirection", err)
	}
}

// The legend names the two directions, with the candle's labels.
func TestADirectionLayersLegendNamesTheTwoDirections(t *testing.T) {
	g := geom.Bar(days(), geom.X("t"), geom.Y("v"), geom.DirectionBy("o", "c"),
		geom.Rising(upColor, "up"), geom.Falling(downColor, "down"))
	_, f := frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	es := geom.Legends(g, f)
	if len(es) != 2 || es[0].Label != "up" || es[0].Color != upColor || es[1].Label != "down" {
		t.Errorf("legend %+v", es)
	}
	g = geom.Bar(days(), geom.X("t"), geom.Y("v"), geom.DirectionBy("o", "c"))
	_, f = frameOn(t, g, scale.Linear(), scale.Linear(), 400, 300)
	if es := geom.Legends(g, f); len(es) != 2 || es[0].Label != "rising" || es[1].Label != "falling" {
		t.Errorf("default legend %+v", es)
	}
}

func TestADirectionIsWrittenDown(t *testing.T) {
	g := geom.Bar(days(), geom.X("t"), geom.Y("v"), geom.DirectionBy("", "c"), geom.Rising(upColor, "up"))
	d := g.(geom.Describer).Describe()
	back, err := geom.FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	again := back.(geom.Describer).Describe()
	if again.DirectionTo != "c" || again.DirectionFrom != "" || again.RisingLabel != "up" || again.ColorCol != "" {
		t.Errorf("read back as %+v", again)
	}
}

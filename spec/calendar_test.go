package spec_test

import (
	"strings"
	"testing"
	"time"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
)

// A calendar is not written down — its folds are — but the week start it gave
// the axis is, and reads back.
func TestAWeekStartSurvivesTheDocument(t *testing.T) {
	c := chartOf(geom.Line(longTable(), geom.X("t"), geom.Y("v")))
	c.X = scale.Time(scale.WeekStart(time.Sunday))
	s, err := spec.Of(c)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := spec.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	out, err := back.Chart()
	if err != nil {
		t.Fatal(err)
	}
	if d := out.X.(scale.Describer).Describe(); d.WeekStart != "sunday" {
		t.Errorf("read back with week start %q\n%s", d.WeekStart, b)
	}
}

func candleTable() *data.Table {
	return data.NewTable().
		Float64("t", []float64{0, 1, 2, 3}).
		Float64("o", []float64{10, 12, 15, 13}).
		Float64("h", []float64{13, 16, 16, 14}).
		Float64("l", []float64{9, 11, 12, 12}).
		Float64("c", []float64{12, 15, 13, 13})
}

// A candlestick survives the document: its mark word, its four channels and
// every option that differs from the default.
func TestACandleSurvivesTheDocument(t *testing.T) {
	src := candleTable()
	ohlc := geom.OHLC("o", "h", "l", "c")
	for name, layer := range map[string]geom.Geom{
		"plain":  geom.Candle(src, geom.X("t"), ohlc),
		"styled": geom.Candle(src, geom.X("t"), ohlc, geom.Direction(geom.SincePrevious), geom.Hollow(true), geom.CandleStyle(geom.Ticks), geom.BarWidth(0.5)),
		"named":  geom.Candle(src, geom.X("t"), ohlc, geom.Rising(ir.RGB(1, 2, 3), "bull"), geom.Falling(ir.RGB(4, 5, 6), "bear")),
	} {
		t.Run(name, func(t *testing.T) {
			c := chartOf(layer)
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the candle did not survive the round trip\n%s", b)
			}
		})
	}

	s, err := spec.Of(chartOf(geom.Candle(src, geom.X("t"), ohlc)))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Layer) != 1 {
		t.Fatalf("%d layers", len(s.Layer))
	}
	l := s.Layer[0]
	enc := l.Encoding
	if l.Mark.Type != "candlestick" || enc == nil || enc.Open == nil || enc.Open.Field != "o" ||
		enc.High.Field != "h" || enc.Low.Field != "l" || enc.Close == nil || enc.Close.Field != "c" {
		t.Errorf("mark %q, encoding %+v", l.Mark.Type, enc)
	}
	if enc != nil && (enc.Y != nil || enc.Y2 != nil) {
		t.Errorf("a candle spent y on one of its values: %+v %+v", enc.Y, enc.Y2)
	}
}

// A direction survives the document on the marks that take it, with and
// without a from column and with the colours it was given.
func TestADirectionSurvivesTheDocument(t *testing.T) {
	src := candleTable()
	for name, layer := range map[string]geom.Geom{
		"bar":      geom.Bar(src, geom.X("t"), geom.Y("h"), geom.DirectionBy("o", "c")),
		"previous": geom.Bar(src, geom.X("t"), geom.Y("h"), geom.DirectionBy("", "c")),
		"named":    geom.Scatter(src, geom.X("t"), geom.Y("c"), geom.DirectionBy("o", "c"), geom.Rising(ir.RGB(1, 2, 3), "up"), geom.Falling(ir.RGB(4, 5, 6), "down")),
		"line":     geom.Line(src, geom.X("t"), geom.Y("c"), geom.DirectionBy("", "c")),
	} {
		t.Run(name, func(t *testing.T) {
			c := chartOf(layer)
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the direction did not survive the round trip\n%s", b)
			}
		})
	}
}

// A last value survives the document with its rule, its direction and its
// colour.
func TestALastValueSurvivesTheDocument(t *testing.T) {
	src := candleTable()
	for name, layer := range map[string]geom.Geom{
		"plain":     geom.LastValue(src, geom.X("t"), geom.Y("c")),
		"no rule":   geom.LastValue(src, geom.X("t"), geom.Y("c"), geom.Rule(false)),
		"direction": geom.LastValue(src, geom.X("t"), geom.Y("c"), geom.DirectionBy("o", "c"), geom.Dash(4, 2)),
		"coloured":  geom.LastValue(src, geom.X("t"), geom.Y("c"), geom.Color(ir.RGB(9, 8, 7))),
	} {
		t.Run(name, func(t *testing.T) {
			c := chartOf(geom.Line(src, geom.X("t"), geom.Y("c")), layer)
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the last value did not survive the round trip\n%s", b)
			}
		})
	}
}

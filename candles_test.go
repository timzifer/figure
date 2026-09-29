package figure_test

import (
	"math"
	"testing"
	"time"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// fortnight is ten trading days of hourly prices, summarised into daily
// candles, with the weekend between them folded.
func fortnight(t *testing.T, opts ...geom.Option) *figure.Plot {
	t.Helper()
	first := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cal := scale.Workweek(time.UTC, scale.Weekdays(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday))
	var ts, ps []float64
	for d := range 12 {
		day := first.AddDate(0, 0, d)
		if len(cal.Open(nil, day)) == 0 {
			continue
		}
		for h := range 8 {
			x := float64(d*8 + h)
			ts = append(ts, scale.Nanos(day.Add(time.Duration(9+h)*time.Hour)))
			ps = append(ps, 100+8*math.Sin(x/9)+3*math.Sin(x*1.7))
		}
	}
	days := stat.OHLC(ts, ps, nil, scale.Nanos(first), float64(24*time.Hour))
	folds, err := scale.Folds(cal, first, first.AddDate(0, 0, 12))
	if err != nil {
		t.Fatal(err)
	}
	p := figure.New(figure.Size(520, 300))
	p.X(scale.Time(scale.TimeCalendar(cal), scale.TimeFold(folds...)))
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Candle(figure.CandleTable(days), append([]geom.Option{
		geom.X("start"), geom.X2("end"), geom.OHLC("open", "high", "low", "close"),
	}, opts...)...))
	return p
}

func TestCandleTableCarriesEveryField(t *testing.T) {
	tb := figure.CandleTable([]stat.Candle{{Start: 1, End: 2, Open: 3, High: 4, Low: 5, Close: 6, Volume: 7, Count: 8}})
	for col, want := range map[string]float64{"start": 1, "end": 2, "open": 3, "high": 4, "low": 5, "close": 6, "volume": 7, "count": 8} {
		vs, ok := data.Float64Column(tb, col)
		if !ok || len(vs) != 1 || vs[0] != want {
			t.Errorf("column %s = %v, want [%v]", col, vs, want)
		}
	}
}

func TestGoldenCandles(t *testing.T) { golden(t, "candles", fortnight(t)) }
func TestGoldenCandleTicks(t *testing.T) {
	golden(t, "candles-ticks", fortnight(t, geom.CandleStyle(geom.Ticks)))
}

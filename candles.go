package figure

import (
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/stat"
)

// CandleTable is candles as the table [github.com/timzifer/figure/geom.Candle]
// reads: one row per candle, with the columns start, end, open, high, low,
// close, volume and count.
//
//	days := stat.OHLC(ts, ps, vs, scale.Nanos(first), float64(24*time.Hour))
//	p.Add(geom.Candle(figure.CandleTable(days), geom.X("start"), geom.X2("end"),
//		geom.OHLC("open", "high", "low", "close")))
//
// The columns are the float64 values the candles were computed in. On a time
// axis those are [github.com/timzifer/figure/scale.Nanos], which is what a
// time scale without an [github.com/timzifer/figure/scale.Origin] reads.
//
// It lives here for the reason [SpansWhere] does: stat does not know a table,
// and data does not know a candle.
func CandleTable(cs []stat.Candle) *data.Table {
	n := len(cs)
	start, end := make([]float64, n), make([]float64, n)
	o, h, l, c := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	vol, count := make([]float64, n), make([]float64, n)
	for i, k := range cs {
		start[i], end[i] = k.Start, k.End
		o[i], h[i], l[i], c[i] = k.Open, k.High, k.Low, k.Close
		vol[i], count[i] = k.Volume, float64(k.Count)
	}
	return NewTable().
		Float64("start", start).Float64("end", end).
		Float64("open", o).Float64("high", h).Float64("low", l).Float64("close", c).
		Float64("volume", vol).Float64("count", count)
}

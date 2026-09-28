// Command market renders a price chart the way a trading screen draws one:
// candles on a calendar with the weekends taken out, two moving averages and a
// Bollinger band over them, the crossings marked as buys and sells, the volume
// traded each day underneath, and the volume traded at each price beside.
//
// docs/adr/0085-what-a-market-chart-needs.md catalogues what such a chart is
// made of, and each part is one layer here: geom.Candle reads the four values
// and decides each day's direction itself (docs/adr/0088); the band is an area
// between two columns; the volume is a bar in a bottom track that shares the
// time axis; the profile is a horizontal bar in a right track that shares the
// price axis. Which days the market trades is a scale.Calendar, and the axis
// folds what it says is closed (docs/adr/0086). The volume bars are coloured
// by the same direction through geom.DirectionBy, in the candles' own colours
// (docs/adr/0091), so no column in the table is computed for the chart's sake.
//
// It is executed by a test so that it cannot silently stop compiling or stop
// producing a chart.
package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"time"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

func main() {
	out := flag.String("o", "market.svg", "output SVG path")
	live := flag.Bool("live", false, "draw the live chart: a tick feed into minute candles, zoomed halfway")
	flag.Parse()
	var err error
	if *live {
		_, _, err = runLive(*out, 7200)
	} else {
		err = run(*out)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "market:", err)
		os.Exit(1)
	}
}

// up and down are the two candle colours. Blue and vermilion rather than green
// and red, because the pair a trading screen uses is the pair the most common
// colour deficiency cannot tell apart; the body's position against the wick
// says the same thing a second time.
var (
	up   = palette.Blue
	down = palette.Vermilion
)

// The indicator windows, in trading days.
const (
	fast = 9  // the EMA
	slow = 20 // the simple mean the band is drawn about
)

func run(out string) error {
	m := simulate()
	days := stat.OHLC(m.ts, m.ps, m.vs, scale.Nanos(start), float64(24*time.Hour))
	candles := candleTable(days)

	p := figure.New(
		figure.Size(1000, 560),
		figure.Title("FIG — daily, weekends and holidays folded"),
	)
	// The days the market is closed are left out of the axis rather than
	// drawn empty: the weekends and the holiday, from the calendar that also
	// decided which days were simulated.
	closed, err := scale.Folds(trading, start, end)
	if err != nil {
		return err
	}
	p.X(scale.Time(scale.TimeCalendar(trading), scale.TimeFold(closed...)))
	p.Y(scale.Linear(scale.Nice()))

	// The band first, so everything else is drawn over it.
	p.Add(geom.Area(candles, geom.X("t"), geom.Y("lower"), geom.Y2("upper"),
		geom.Color(palette.Gray), geom.Opacity(0.18), geom.Label("Bollinger 20, 2σ")))

	// One layer per candle chart: the mark reads the four values and draws the
	// wick and the body in the direction it decides.
	p.Add(geom.Candle(candles, geom.X("t"), geom.OHLC("open", "high", "low", "close"),
		geom.Rising(up, "up"), geom.Falling(down, "down")))

	p.Add(geom.Line(candles, geom.X("t"), geom.Y("ema"),
		geom.Color(palette.Orange), geom.Label(fmt.Sprintf("EMA %d", fast))))
	p.Add(geom.Line(candles, geom.X("t"), geom.Y("mid"),
		geom.Color(palette.Purple), geom.Label(fmt.Sprintf("SMA %d", slow))))

	// Where the fast average crosses the slow one: a buy under the candle, a
	// sell over it, pointing the way the price is expected to go.
	buys, sells := signals(days)
	p.Add(geom.Scatter(buys, geom.X("t"), geom.Y("at"),
		geom.Shape(ir.MarkerTriangle), geom.Size(10), geom.Color(up), geom.Label("buy")))
	p.Add(geom.Scatter(sells, geom.X("t"), geom.Y("at"),
		geom.Shape(ir.MarkerTriangleDown), geom.Size(10), geom.Color(down), geom.Label("sell")))

	// The volume each day, under the price and on the same time axis: one
	// scale object, so a zoom on a live chart moves both. Coloured by the day's
	// direction with the candles' options, so the two agree and the legend
	// names up and down once.
	p.Track(figure.Bottom, figure.TrackFraction(0.2), figure.TrackScale(scale.Linear(scale.Zero())), figure.TrackAxis(true)).
		Add(geom.Bar(candles, geom.X("t"), geom.Y("volume"), geom.BarWidth(0.7),
			geom.DirectionBy("open", "close"), geom.Rising(up, "up"), geom.Falling(down, "down"),
			geom.Opacity(0.6)))

	// The volume traded at each price, beside the price and on the same price
	// axis: a bar lying on its side per price bucket, buys stacked first and
	// sells after them, so the bar's length is the total and its split is the
	// balance. The buckets are contiguous, so the bars fill their slots.
	p.Track(figure.Right, figure.TrackSize(140), figure.TrackScale(scale.Linear(scale.Zero())), figure.TrackAxis(true)).
		Add(geom.Bar(profile(m), geom.Y("price"), geom.X("volume"),
			geom.Orient(geom.Horizontal), geom.BarWidth(1), geom.GroupBy("side"),
			geom.ColorBy("side", scale.Named(map[string]ir.Color{"buy": up, "sell": down})),
			geom.Opacity(0.7)))

	return p.Render(figure.SVG(out))
}

// start is the first calendar day of the chart: a Monday, so that the weeks
// line up with the folds and the picture is the same every run.
var start = time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

// calendarDays is how long the chart runs: twelve weeks.
const calendarDays = 7 * 12

// end is the day after the chart's last.
var end = start.AddDate(0, 0, calendarDays)

// holiday is the one weekday the market is closed on in the chart: the Friday
// Independence Day is observed on in 2026.
var holiday = time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC)

// trading is the market's calendar: Monday to Friday, all day — a daily chart
// needs the days, not the hours — without the holiday.
var trading = scale.Closed(
	scale.Workweek(time.UTC, scale.Weekdays(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)),
	scale.Dates(holiday))

// market is the ticks of the simulated trading: a time, a price, and the
// volume bought and sold at it.
type market struct {
	ts, ps, vs  []float64
	buys, sells []float64
}

// simulate is a seeded random walk, traded between 09:30 and 16:00 on
// weekdays. It is a walk rather than a recorded series so that the example
// needs no data file and draws the same picture every time.
func simulate() market {
	r := rand.New(rand.NewPCG(7, 85))
	var m market
	price := 100.0
	for d := range calendarDays {
		day := start.AddDate(0, 0, d)
		if len(trading.Open(nil, day)) == 0 {
			continue
		}
		// A drift that changes sign every few weeks, so the averages cross.
		drift := 0.04 * math.Sin(float64(d)/9)
		for tick := range 40 {
			at := day.Add(9*time.Hour + 30*time.Minute + time.Duration(tick)*585*time.Second)
			price *= 1 + drift/40 + r.NormFloat64()*0.004
			v := 100 + r.Float64()*900
			bought := v * (0.5 + drift*4 + r.NormFloat64()*0.1)
			bought = min(max(bought, 0), v)
			m.ts = append(m.ts, scale.Nanos(at))
			m.ps = append(m.ps, price)
			m.vs = append(m.vs, v)
			m.buys = append(m.buys, bought)
			m.sells = append(m.sells, v-bought)
		}
	}
	return m
}

// candleTable is the day candles as the columns the layers read, with the
// indicators beside them. A candle is drawn at the middle of its day, so that
// its slot is the day rather than straddling midnight into a fold.
func candleTable(days []stat.Candle) *data.Table {
	n := len(days)
	t := make([]time.Time, n)
	xs := make([]float64, n)
	closes := make([]float64, n)
	o, h, l, c, v := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	for i, d := range days {
		t[i] = scale.FromNanos(d.Start + float64(12*time.Hour))
		xs[i] = float64(i)
		closes[i] = d.Close
		o[i], h[i], l[i], c[i], v[i] = d.Open, d.High, d.Low, d.Close, d.Volume
	}

	// The indicators run over the row index rather than the time: a trading
	// day is a row, and the weekend between two of them is not two days of
	// average.
	ema := ys(stat.EMA(xs, closes, fast))
	mid := ys(stat.TrailingMean(xs, closes, slow))
	sd := ys(stat.RollingStdDev(xs, closes, slow))
	lower, upper := make([]float64, n), make([]float64, n)
	for i := range n {
		lower[i], upper[i] = mid[i]-2*sd[i], mid[i]+2*sd[i]
	}

	return figure.NewTable().
		Time("t", t).
		Float64("open", o).Float64("high", h).Float64("low", l).Float64("close", c).
		Float64("volume", v).
		Float64("ema", ema).Float64("mid", mid).
		Float64("lower", lower).Float64("upper", upper)
}

// signals is where the EMA crosses the simple mean: upwards is a buy, drawn
// under the day's low, and downwards a sell, drawn over its high. The first
// slow window is skipped — the mean is over fewer rows there than its name
// says, and a signal from it is a signal from a different indicator.
func signals(days []stat.Candle) (buys, sells *data.Table) {
	xs := make([]float64, len(days))
	closes := make([]float64, len(days))
	for i, d := range days {
		xs[i], closes[i] = float64(i), d.Close
	}
	ema := ys(stat.EMA(xs, closes, fast))
	mid := ys(stat.TrailingMean(xs, closes, slow))

	var bt, st []time.Time
	var ba, sa []float64
	for i := slow; i < len(days); i++ {
		at := scale.FromNanos(days[i].Start + float64(12*time.Hour))
		pad := (days[i].High - days[i].Low) * 0.4
		switch {
		case ema[i-1] <= mid[i-1] && ema[i] > mid[i]:
			bt, ba = append(bt, at), append(ba, days[i].Low-pad)
		case ema[i-1] >= mid[i-1] && ema[i] < mid[i]:
			st, sa = append(st, at), append(sa, days[i].High+pad)
		}
	}
	return figure.NewTable().Time("t", bt).Float64("at", ba),
		figure.NewTable().Time("t", st).Float64("at", sa)
}

// profile is the volume at each price, bought and sold, one row per bucket
// and side, for a stacked horizontal bar. The two sides are binned over one
// interval — the whole price column's — so that their buckets line up.
func profile(m market) *data.Table {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range m.ps {
		lo, hi = min(lo, p), max(hi, p)
	}
	const bins = 28
	b := stat.BinWeighted(m.ps, m.buys, lo, hi, bins)
	s := stat.BinWeighted(m.ps, m.sells, lo, hi, bins)

	var price, volume []float64
	var side []string
	for i := range b {
		price = append(price, b[i].Mid(), s[i].Mid())
		volume = append(volume, b[i].Sum, s[i].Sum)
		side = append(side, "buy", "sell")
	}
	return figure.NewTable().
		Float64("price", price).Float64("volume", volume).
		String("side", side)
}

func ys(ps []stat.Point) []float64 {
	out := make([]float64, len(ps))
	for i, p := range ps {
		out[i] = p.Y
	}
	return out
}

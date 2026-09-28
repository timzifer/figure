package main

import (
	"io"
	"math"
	"math/rand/v2"
	"time"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// liveCandles is how many minutes of candles the live chart keeps.
const liveCandles = 120

// runLive is the chart as a trading screen redraws it: a feed of ticks folded
// into minute candles by a stat.Resampler, each tick either revising the open
// candle with Stream.ReplaceLast or opening the next with Stream.Append, drawn
// by a Live whose price axis fits whatever minutes are in view. Halfway
// through, the reader wheels into the last half hour; the time axis zooms, the
// host keeps that half hour's right-hand edge on the newest candle, and the
// price axis follows frame by frame without being zoomed itself. See
// docs/adr/0089-a-value-axis-fits-what-its-time-axis-shows.md.
//
// The frames are drawn to nowhere — a real host draws them to a window or a
// canvas — and the last one is written to out, so the artefact is the zoomed,
// fitted view the reader ended on. It returns the plot's axes for the test.
func runLive(out string, ticks int) (x, y scale.Scale, err error) {
	st := data.NewStream("start", "end", "open", "high", "low", "close", "volume").Window(liveCandles)

	p := figure.New(figure.Size(900, 420), figure.Title("FIG — live, one-minute candles"))
	x = scale.Time()
	y = scale.Linear(scale.Nice(), scale.FitView())
	p.X(x).Y(y)
	p.Add(geom.Candle(st.Source(), geom.X("start"), geom.X2("end"),
		geom.OHLC("open", "high", "low", "close"),
		geom.Rising(up, "up"), geom.Falling(down, "down")))
	// The volume fits its view too: the tallest bar of the session is not the
	// scale of the last half hour.
	p.Track(figure.Bottom, figure.TrackFraction(0.2),
		figure.TrackScale(scale.Linear(scale.Zero(), scale.FitView())), figure.TrackAxis(true)).
		Add(geom.Bar(st.Source(), geom.X("start"), geom.Y("volume"), geom.BarWidth(0.7), geom.Opacity(0.6),
			// The stream carries numbers only, and the direction needs none
			// other: it is read from the open and the close the candle has.
			geom.DirectionBy("open", "close"), geom.Rising(up, "up"), geom.Falling(down, "down")))

	live, err := p.Live(figure.SVGWriter(io.Discard))
	if err != nil {
		return nil, nil, err
	}
	defer live.Close()

	r := stat.Resampler{Origin: scale.Nanos(liveOpen), Width: float64(time.Minute)}
	rng := rand.New(rand.NewPCG(3, 89))
	price := 100.0
	row := make([]float64, 7)
	zoomed, width := false, 0.0
	for i := range ticks {
		at := liveOpen.Add(time.Duration(i) * 2 * time.Second)
		price *= 1 + 0.0001*math.Sin(float64(i)/400) + rng.NormFloat64()*0.0008
		c, fresh, err := r.Add(scale.Nanos(at), price, 1+rng.Float64()*9)
		if err != nil {
			return nil, nil, err
		}
		row[0], row[1], row[2], row[3], row[4], row[5], row[6] = c.Start, c.End, c.Open, c.High, c.Low, c.Close, c.Volume
		if fresh {
			err = st.Append(row...)
		} else {
			err = st.ReplaceLast(row...)
		}
		if err != nil {
			return nil, nil, err
		}

		// A frame every ten ticks, as a screen refreshes on its own clock
		// rather than on the feed's.
		if i%10 != 9 {
			continue
		}
		st.Snapshot()
		if zoomed {
			// Following the newest data is the host's, not the library's: it
			// keeps the width the reader zoomed to and moves the right-hand
			// edge to the open candle, and the price axis fits whatever that
			// leaves in view.
			x.(scale.Zoomer).SetDomain(c.End-width, c.End)
		}
		if err := live.Draw(); err != nil {
			return nil, nil, err
		}
		if !zoomed && i >= ticks/2 {
			// The reader wheels in over the right-hand edge, where the newest
			// candles are. Only time zooms; the price axis follows.
			area := live.Index().Panels()[0].Area
			if err := live.Wheel(float64(area.Max.X)-2, float64(area.Min.Y+area.Max.Y)/2, 0.25); err != nil {
				return nil, nil, err
			}
			lo, hi := x.Domain()
			width, zoomed = hi-lo, true
		}
	}
	st.Snapshot()
	return x, y, p.Render(figure.SVG(out))
}

// liveOpen is when the simulated session opens.
var liveOpen = time.Date(2026, time.September, 28, 9, 30, 0, 0, time.UTC)

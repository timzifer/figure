// Command horizon renders the chart the fold exists for: a wall of sensors,
// each one legible in a strip a line chart would have nothing to say in.
//
// Sixteen meters, six hours apiece. Drawn as sixteen line charts the panel is
// forty pixels tall and every trace is a smudge; folded into three bands it is
// the same forty pixels at four times the resolution, and which band a stretch
// is in is a colour. The colourbar down the side is the ladder the chart gave
// up, printed in the data's own units.
//
// The band height is pinned rather than the band count, which is the whole
// discipline of the form: 30 kW means 30 kW in this chart and in tomorrow's, so
// two meters can be compared with each other and two days with themselves.
//
// It is executed by a test so that it cannot silently stop compiling or stop
// producing a chart.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/facet"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

func main() {
	out := flag.String("o", "horizon.svg", "output SVG path")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "horizon:", err)
		os.Exit(1)
	}
}

// meters is how many strips the wall has, and samples how many readings each
// one carries. Sixteen is where a screen of line charts stops being readable,
// which is the crossover this chart is for.
const (
	meters  = 16
	samples = 360
)

// bandHeight is one band, in kilowatts. It is pinned rather than derived so
// that a colour means the same load here as in the next chart drawn this way —
// a band that meant "a third of today's peak" would make two days
// incomparable, which is what the form exists to prevent.
const bandHeight = 30

func run(out string) error {
	src := load()

	p := figure.New(
		figure.Size(900, 700),
		figure.Title("Metered load — 16 meters, six hours"),
		figure.XTitle("time"),
	)
	p.X(scale.Time())
	p.Y(scale.Linear())

	// One layer, sixteen panels. A horizon layer draws one series on purpose —
	// every band of it is the whole panel, so two series in one would be a
	// solid rectangle rather than a crowded chart — and the wall is therefore
	// the facet the library already builds walls with.
	p.Add(geom.Horizon(src,
		geom.X("t"), geom.Y("kw"),
		geom.BandHeight(bandHeight),
		geom.Baseline(baseLoad),
	))
	p.Facet(facet.Wrap("meter", facet.Columns(2)))

	return p.Render(figure.SVG(out))
}

// baseLoad is the load the fold is measured from: the site's idle draw. The
// chart is therefore about the deviation from it, with the two arms of the ramp
// separating "drawing more than idle" from "drawing less".
const baseLoad = 120

// load builds the wall: one long table of (meter, t, kw).
//
// Deterministic, like every other example here — a fixed formula rather than
// math/rand, so the committed figure and a fresh run are the same chart.
func load() figure.Source {
	start := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)

	ts := make([]time.Time, 0, meters*samples)
	kw := make([]float64, 0, meters*samples)
	names := make([]string, 0, meters*samples)

	for m := range meters {
		// Each meter runs its own shift pattern: a slow swell over the six
		// hours, a faster machine cycle on top of it, and one excursion that a
		// line chart at this height would hide.
		phase := float64(m) * 0.37
		swell := 40 + 25*math.Sin(phase)
		cycle := 18 + 9*math.Cos(1.7*phase)
		peak := float64(samples) * (0.25 + 0.5*frac(0.61*float64(m)))

		for i := range samples {
			x := float64(i) / float64(samples-1)
			v := baseLoad +
				swell*math.Sin(2*math.Pi*x+phase) +
				cycle*math.Sin(23*math.Pi*x+2*phase)

			// The excursion: a short, sharp overload a couple of bands deep.
			if d := float64(i) - peak; math.Abs(d) < 9 {
				v += 95 * (1 - math.Abs(d)/9)
			}

			ts = append(ts, start.Add(time.Duration(i)*time.Minute))
			kw = append(kw, v)
			names = append(names, fmt.Sprintf("M%02d", m+1))
		}
	}

	return figure.NewTable().
		String("meter", names).
		Time("t", ts).
		Float64("kw", kw)
}

// frac is the fractional part, which is how each meter gets a different
// excursion time out of its own index without a random source.
func frac(v float64) float64 { return v - math.Floor(v) }

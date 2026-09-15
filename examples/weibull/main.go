// Command weibull renders a Weibull probability plot.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. And like examples/smith, its point is
// what is *not* here: there is no Weibull mark and no reliability package. The
// chart is a geom.Scatter over two scales — a log X for the hours and
// scale.Probability(scale.CLogLog) for the fraction failed — and it is the
// Y scale alone that makes a Weibull sample fall on a straight line. See
// docs/adr/0052-probability-scales.md.
//
// The points are stat.MedianRank rather than an ECDF, because an ECDF reaches
// 1 at the last failure and 1 has no place on the axis. The line is the
// distribution the sample was drawn from, added by hand: a scale that fitted
// one would be asserting the answer the plot exists to let a reader find.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

func main() {
	out := flag.String("o", "weibull.svg", "output path")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "weibull:", err)
		os.Exit(1)
	}
}

// The population the bearings were drawn from: wear-out, since β > 1, with
// 63.2 % of units failed by η hours.
const (
	beta  = 2.2
	eta   = 1000.0
	units = 20
)

func run(out string) error {
	hours := failures()
	ranks := stat.MedianRank(hours)
	rx, ry := make([]float64, len(ranks)), make([]float64, len(ranks))
	for i, r := range ranks {
		rx[i], ry[i] = r.X, r.Y
	}

	// The population's own line, over the span the axis shows.
	var lx, ly []float64
	for t := 80.0; t <= 3000; t *= 1.05 {
		lx, ly = append(lx, t), append(ly, cdf(t))
	}

	p := figure.New(
		figure.Size(640, 520),
		figure.Title("Bearing life on Weibull paper"),
		figure.Theme(theme.Light),
		figure.XTitle("hours to failure"),
		figure.YTitle("fraction failed"),
	)
	p.X(scale.Log(scale.LogDomain(80, 3000)))
	p.Y(scale.Probability(scale.CLogLog, scale.ProbabilityDomain(0.01, 0.99)))
	p.Add(geom.Line(figure.NewTable().Float64("t", lx).Float64("F", ly),
		geom.X("t"), geom.Y("F"), geom.Color(palette.OkabeIto[0]), geom.Width(1.5),
		geom.Label(fmt.Sprintf("β = %.1f, η = %.0f h", beta, eta))))
	p.Add(geom.Scatter(figure.NewTable().Float64("t", rx).Float64("F", ry),
		geom.X("t"), geom.Y("F"), geom.Color(palette.OkabeIto[1]), geom.Size(7),
		geom.Label(fmt.Sprintf("%d bearings, median ranks", units))))
	p.Add(geom.Segment(80, 1-1/math.E, eta, 1-1/math.E, geom.Dash(2, 3)))
	p.Add(geom.Note(90, 0.68, "η: 63.2 % failed"))
	return p.Render(figure.SVG(out))
}

// cdf is the population's distribution function.
func cdf(t float64) float64 { return -math.Expm1(-math.Pow(t/eta, beta)) }

// failures is a sample of the population, ascending. It is drawn at evenly
// spaced probabilities and then nudged by a fixed pattern, so the chart is the
// same every time and still looks like a test rather than like a formula.
func failures() []float64 {
	out := make([]float64, units)
	for i := range out {
		p := (float64(i) + 0.5) / units
		p = math.Min(math.Max(p+0.02*math.Sin(float64(i)*2.4), 0.005), 0.995)
		out[i] = eta * math.Pow(-math.Log1p(-p), 1/beta)
	}
	// The nudge can swap neighbours; the ranks need the column ascending.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

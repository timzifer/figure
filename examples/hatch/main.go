// Command hatch renders the chart redundant encoding could not reach until
// patterns existed: a stacked bar chart, readable in greyscale.
//
// theme.Redundant has always given a line its dash and a point its shape. A
// bar has neither — no stroke to dash, no shape to swap — so a stacked bar
// chart printed in black and white was three identical grey blocks, and the
// one option figure offers for exactly this problem did nothing about it. A
// hatch is the filled mark's dash.
//
// Five figures say it: the same stack plain, under the nominal ladder that
// gives every series a different pattern, under the ordinal one where the
// pattern gets denser instead, wearing the finishes that are not patterns at
// all, and — the oldest use of a hatch on a chart — with a band saying that
// the last stretch of a series is a projection rather than a measurement.
//
// It is executed by a test so that it cannot silently stop compiling or stop
// producing a chart.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func main() {
	out := flag.String("o", "hatch.svg", "output path; the five figures are written beside it")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "hatch:", err)
		os.Exit(1)
	}
}

// quarters is the table the stacks draw: three regions over four quarters.
//
// Deterministic, like every other example here — a fixed table rather than
// math/rand, so the committed figure and a fresh run are the same chart.
func quarters() figure.Source {
	return figure.NewTable().
		String("q", []string{
			"Q1", "Q2", "Q3", "Q4",
			"Q1", "Q2", "Q3", "Q4",
			"Q1", "Q2", "Q3", "Q4",
		}).
		String("region", []string{
			"north", "north", "north", "north",
			"south", "south", "south", "south",
			"east", "east", "east", "east",
		}).
		Float64("units", []float64{18, 24, 21, 30, 12, 9, 15, 11, 7, 13, 9, 16})
}

func run(out string) error {
	base := strings.TrimSuffix(out, ".svg")

	stacks := []struct {
		suffix string
		title  string
		theme  theme.Theme
		opts   []geom.Option
	}{
		{
			suffix: "-plain",
			title:  "Units shipped — colour and nothing else",
			theme:  theme.Light,
		},
		{
			// Nominal: every series a different pattern, none heavier than
			// another, because three regions are three categories and none of
			// them is more than another.
			suffix: "-nominal",
			title:  "Units shipped — a pattern per region",
			theme:  theme.Light.With(theme.Redundant(true)),
		},
		{
			// Ordinal: one pattern getting denser. Wrong for regions and right
			// for a stack that is an order — severities, age bands, size
			// classes — which is why the ladder is a choice and not a default.
			suffix: "-ordinal",
			title:  "Units shipped — denser is more",
			theme:  theme.Light.With(theme.Redundant(true), theme.Hatches(theme.DensitySeriesHatches...)),
		},
		{
			// The finishes that are not patterns: rounded corners, an inner
			// border separating segments that touch, and a fill that lifts
			// toward the top of each segment.
			suffix: "-finished",
			title:  "Units shipped — corners, edges and a lift",
			theme:  theme.Light,
			opts: []geom.Option{
				geom.Corner(4),
				geom.Inset(1),
				geom.Gradient(ir.RGBA(0xFF, 0xFF, 0xFF, 0x55)),
			},
		},
	}

	for _, v := range stacks {
		p := figure.New(
			figure.Size(560, 360),
			figure.Title(v.title),
			figure.YTitle("units"),
			figure.Legend(true),
			figure.Theme(v.theme),
		)
		p.X(scale.Ordinal())
		p.Y(scale.Linear(scale.Nice(), scale.Zero()))
		p.Add(geom.Bar(quarters(), append([]geom.Option{
			geom.X("q"), geom.Y("units"), geom.GroupBy("region"),
		}, v.opts...)...))
		if err := p.Render(figure.SVG(base + v.suffix + ".svg")); err != nil {
			return err
		}
	}
	return projection(base + "-projection.svg")
}

// forecastFrom is the week the series stops being a measurement. Everything
// from here on is modelled, and the hatched band is how the chart says so —
// which is the use a hatch was put to on paper long before it was put to
// telling two bars apart.
const forecastFrom = 34

// weeks is how many points the series carries.
const weeks = 52

// projection draws the second thing a hatch is for: a region of the chart that
// is not measured.
//
// The band takes an explicit pattern rather than one from the theme's ladder,
// and that is the rule rather than an oversight — an annotation is not a
// series. It walks no palette, it has no rung, and a theme turning redundant
// encoding on must not start hatching the one layer on a chart that is
// deliberately quiet.
func projection(out string) error {
	x := make([]float64, weeks)
	y := make([]float64, weeks)
	for i := range weeks {
		x[i] = float64(i + 1)
		// A slow seasonal swell with a trend under it, and no noise: the
		// example is about the band, not about the series.
		y[i] = 40 + 0.35*float64(i) + 8*math.Sin(2*math.Pi*float64(i)/26)
	}
	src := figure.NewTable().Float64("week", x).Float64("units", y)

	p := figure.New(
		figure.Size(560, 360),
		figure.Title("Units shipped — the last eighteen weeks are modelled"),
		figure.XTitle("week"),
		figure.YTitle("units"),
		figure.Legend(true),
		figure.Theme(theme.Light),
	)
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice(), scale.Zero()))

	// The band is added first so the series is drawn over it.
	p.Add(geom.VBand(forecastFrom, weeks,
		geom.Label("projection"),
		geom.Hatch(ir.HatchBackDiagonal),
	))
	p.Add(geom.Area(src, geom.X("week"), geom.Y("units"),
		geom.Label("shipped"),
		geom.Color(palette.Blue),
	))
	return p.Render(figure.SVG(out))
}

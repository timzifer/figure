// Command parallel renders the chart a panel with more than two axes draws.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. The table is a fleet of cars — four
// measured quantities in four different units — and the point of the form is
// that a reader can see all four at once and look for the shape the lines
// make: where they converge, where they cross, which few of them run against
// the rest. See docs/adr/0078-a-coord-with-more-than-two-axes.md.
//
// The second chart is the same table with one axis turned upside down, which
// is how a parallel-coordinates plot is made to read "better is up" on every
// axis: fewer kilograms is better, so the weight axis descends.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func main() {
	fleet := flag.String("fleet", "fleet.svg", "output path for the fleet chart")
	better := flag.String("better", "fleet-better-up.svg", "output path for the chart with weight reversed")
	flag.Parse()
	if err := run(*fleet, *better); err != nil {
		fmt.Fprintln(os.Stderr, "parallel:", err)
		os.Exit(1)
	}
}

func run(fleet, better string) error {
	if err := fleetChart(fleet); err != nil {
		return err
	}
	return betterIsUp(better)
}

// cars is sixty cars, each with four measurements and an origin. The numbers
// are made up and the correlations are not: power rises as economy falls, and
// weight follows power.
func cars() data.Source {
	var mpg, power, weight, year []float64
	var origin []string
	for i := range 60 {
		t := float64(i) / 59
		o := []string{"europe", "japan", "usa"}[i%3]
		base := map[string]float64{"europe": 26, "japan": 31, "usa": 17}[o]
		economy := base + 6*math.Sin(t*7)
		hp := 200 - 3.5*economy + 20*math.Cos(t*5)
		mpg = append(mpg, economy)
		power = append(power, hp)
		weight = append(weight, 700+4*hp+120*math.Sin(t*3))
		year = append(year, 1970+float64(i%13))
		origin = append(origin, o)
	}
	return figure.NewTable().
		Float64("mpg", mpg).
		Float64("power", power).
		Float64("weight", weight).
		Float64("year", year).
		String("origin", origin)
}

// bare is the theme a parallel panel wants: nothing of the panel's own two
// axes, because the axes this chart has are the coord's and are drawn as
// furniture.
//
// Turning the ticks off is worth doing rather than merely harmless: a panel
// reserves a gutter for the tick labels it is going to write, and this one
// writes none. The dimensions keep their own numbers, because the coord
// reports that its families are the axes — see ADR 0078.
func bare() theme.Theme {
	return theme.Light.With(
		theme.Grid(false, false), theme.AxisLines(false, false), theme.Ticks(false, false))
}

// fleetChart is the form itself: four axes, sixty lines, one colour per
// origin.
func fleetChart(out string) error {
	p := figure.New(
		figure.Size(760, 460),
		figure.Title("Sixty cars, four measurements"),
		figure.Theme(bare()),
		figure.Coord(coord.Parallel(
			coord.Dim("mpg", scale.Linear()),
			coord.Dim("power (hp)", scale.Linear()),
			coord.Dim("weight (kg)", scale.Linear()),
			coord.Dim("year", scale.Linear()),
		)),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Parallel(cars(),
		// One column per axis, in the order the axes are drawn. The coord's
		// dimensions carry a label and a scale and never a column name, which
		// is what keeps a coord from knowing what a table is.
		geom.Dims("mpg", "power", "weight", "year"),
		geom.ColorBy("origin", scale.Qualitative(palette.OkabeIto)),
		geom.Opacity(0.7),
	))
	return p.Render(figure.SVG(out))
}

// betterIsUp turns the weight axis over, so that every axis reads with the
// better end at the top and a car that is good on all four is a line across
// the top of the chart.
//
// Which way up an axis reads is the dimension's own scale, so this needs
// nothing from the coord and nothing from the mark — see
// docs/adr/0075-an-axis-has-a-direction.md.
func betterIsUp(out string) error {
	p := figure.New(
		figure.Size(760, 460),
		figure.Title("The same fleet, with every axis reading better-is-up"),
		figure.Theme(bare()),
		figure.Coord(coord.Parallel(
			coord.Dim("mpg", scale.Linear()),
			coord.Dim("power (hp)", scale.Linear()),
			coord.Dim("weight (kg)", scale.Linear(scale.Reverse())),
		)),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Parallel(cars(),
		geom.Dims("mpg", "power", "weight"),
		geom.ColorBy("origin", scale.Qualitative(palette.OkabeIto)),
		geom.Opacity(0.7),
	))
	return p.Render(figure.SVG(out))
}

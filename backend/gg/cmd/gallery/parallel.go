package main

import (
	"math"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// parallelFigure is the chart a panel with more than two axes draws, and the
// first one in this gallery whose axes are neither of the panel's own.
//
// A parallel-coordinates plot puts one vertical axis per measured quantity and
// draws every row as a line crossing all of them. Each axis has its own
// domain — miles per gallon and kilograms on one panel, neither squashed into
// the other's range — which is what a radar chart, whose spokes share one
// scale, cannot do.
//
// The axes are the coord's: coord.Parallel holds a scale per dimension, ranges
// them onto the unit interval and raises each as a labelled family, which
// render strokes with the grid and writes with the ticks. Nothing in render
// knows how many axes a panel has. See
// docs/adr/0078-a-coord-with-more-than-two-axes.md.
func parallelFigure() plate {
	return plate{
		name: "parallel", width: 720, high: 460,
		// Nothing of the panel's own two axes: the axes this chart has are the
		// coord's, drawn as furniture. Turning the ticks off gives back the
		// gutter a panel reserves for tick labels, and the dimensions keep
		// their own numbers because the coord reports that its families are
		// the axes.
		theme: theme.Light.With(
			theme.Grid(false, false), theme.AxisLines(false, false), theme.Ticks(false, false)),
		title: "Sixty cars, four measurements at once",
		opts: []figure.Option{figure.Coord(coord.Parallel(
			coord.Dim("mpg", scale.Linear()),
			coord.Dim("power (hp)", scale.Linear()),
			coord.Dim("weight (kg)", scale.Linear()),
			coord.Dim("year", scale.Linear()),
		))},
		build: func(p *figure.Plot) {
			p.X(scale.Linear())
			p.Y(scale.Linear())
			p.Add(geom.Parallel(fleetTable(),
				geom.Dims("mpg", "power", "weight", "year"),
				geom.ColorBy("origin", scale.Qualitative(palette.OkabeIto)),
				geom.Opacity(0.7),
			))
		},
	}
}

// fleetTable is the figure's data: four measurements per car, in four units.
// The numbers are made up and the correlations are not — power rises as
// economy falls, and weight follows power.
func fleetTable() data.Source {
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
		Float64("mpg", mpg).Float64("power", power).
		Float64("weight", weight).Float64("year", year).
		String("origin", origin)
}

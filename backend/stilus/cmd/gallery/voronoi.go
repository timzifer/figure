package main

import (
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// voronoiFigure is the chart for a sample that was measured where the
// instruments happen to be: the panel divided into the part nearest each of
// them, filled with what that one read.
//
// It is the coarsest honest interpolation of a scattered sample — every point
// of the map carries the nearest measurement rather than a blend of several —
// and the boundaries are where the nearest gauge changes. The dots on top are
// an ordinary scatter layer over the same table: the partition mark draws the
// regions and nothing else.
//
// The cells are cut on the panel rather than in the data, because a distance
// needs two commensurable axes and a reading against a grid reference has no
// length. See docs/adr/0080-nearest-neighbour-cells.md.
func voronoiFigure() plate {
	return plate{
		name: "voronoi", width: 720, high: 500, theme: theme.Light,
		title: "Last month's rainfall, as the nearest gauge measured it",
		opts: []figure.Option{
			figure.XTitle("Easting (km)"),
			figure.YTitle("Northing (km)"),
			// Two layers, so a legend would appear by default — and it would
			// name the gauges' scatter rather than say anything: what the
			// colours mean is the colourbar, which is a guide and not a
			// legend entry.
			figure.Legend(false),
		},
		build: func(p *figure.Plot) {
			p.X(scale.Linear())
			p.Y(scale.Linear())
			p.Add(geom.Voronoi(gaugeTable(),
				geom.X("east"), geom.Y("north"),
				geom.ColorBy("rain", scale.Sequential(palette.Viridis)),
				geom.Label("Rainfall (mm)"),
			))
			p.Add(geom.Scatter(gaugeTable(),
				geom.X("east"), geom.Y("north"),
				geom.Color(ir.RGB(255, 255, 255)), geom.Size(4)))
		},
	}
}

// gaugeTable is the figure's data: twenty rain gauges, where each stands and
// what it measured. The numbers are made up and the pattern in them is the
// ordinary one — it rains more over the high ground in the north-west.
func gaugeTable() data.Source {
	var east, north, rain []float64
	for _, g := range []struct {
		east, north, rain float64
	}{
		{4, 46, 188}, {12, 38, 174}, {19, 44, 166}, {27, 36, 142},
		{34, 45, 131}, {41, 37, 118}, {8, 29, 165}, {16, 22, 151},
		{24, 28, 139}, {32, 21, 124}, {39, 27, 109}, {6, 14, 147},
		{14, 8, 132}, {22, 13, 121}, {29, 6, 104}, {37, 12, 96},
		{44, 18, 88}, {2, 22, 158}, {45, 30, 97}, {10, 47, 181},
	} {
		east, north = append(east, g.east), append(north, g.north)
		rain = append(rain, g.rain)
	}
	return data.NewTable().
		Float64("east", east).
		Float64("north", north).
		Float64("rain", rain)
}

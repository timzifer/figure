// Command voronoi renders the two charts a nearest-neighbour partition draws.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. The table is twenty rain gauges over a
// catchment — where each one stands, and how much it measured last month. See
// docs/adr/0080-nearest-neighbour-cells.md.
//
// The first chart is the partition as a map of coverage: each cell is the part
// of the catchment nearer that gauge than any other, which is the question
// asked of a network of instruments before any reading is looked at — who
// speaks for this ground. The gauges are a scatter layer over the cells, which
// is how this library draws the dots: the partition mark draws the regions,
// and the mark for a dot is the one that already existed.
//
// The second chart is the same cells filled with the reading, which turns them
// from a map of coverage into a map of rainfall. It is the coarsest honest
// interpolation of a scattered sample and the only one that invents no value:
// every point of the panel carries the nearest measurement rather than a
// blend of several, and the boundaries are where the nearest one changes.
//
// The two pictures are not quite the same partition, and the reason is worth
// seeing: the rainfall map carries a colourbar, the colourbar takes room off
// the plot area, and the cells are cut against the plot area. Twenty gauges in
// a narrower panel are twenty gauges differently placed on the page, so the
// boundaries between them move. That is what it means for this mark to be
// measured where the reader measures it.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

func main() {
	coverage := flag.String("coverage", "coverage.svg", "output path for the map of coverage")
	rainfall := flag.String("rainfall", "rainfall.svg", "output path for the map of rainfall")
	flag.Parse()
	if err := run(*coverage, *rainfall); err != nil {
		fmt.Fprintln(os.Stderr, "voronoi:", err)
		os.Exit(1)
	}
}

func run(coverage, rainfall string) error {
	if err := gaugeCoverage(coverage); err != nil {
		return err
	}
	return gaugeRainfall(rainfall)
}

// gauges is twenty rain gauges: a grid reference each, and last month's total
// in millimetres. The numbers are made up; the pattern in them is the ordinary
// one — it rains more over the high ground in the north-west.
func gauges() data.Source {
	var name []string
	var east, north, rain []float64
	for _, g := range []struct {
		name        string
		east, north float64
		rain        float64
	}{
		{"Ardnoe", 4, 46, 188},
		{"Balvaig", 12, 38, 174},
		{"Carron", 19, 44, 166},
		{"Dalveen", 27, 36, 142},
		{"Eskdale", 34, 45, 131},
		{"Fettercairn", 41, 37, 118},
		{"Glenkens", 8, 29, 165},
		{"Hartfell", 16, 22, 151},
		{"Inverkip", 24, 28, 139},
		{"Kinloch", 32, 21, 124},
		{"Lochar", 39, 27, 109},
		{"Moffat", 6, 14, 147},
		{"Nithsdale", 14, 8, 132},
		{"Orchy", 22, 13, 121},
		{"Pentland", 29, 6, 104},
		{"Queensferry", 37, 12, 96},
		{"Rannoch", 44, 18, 88},
		{"Stinchar", 2, 22, 158},
		{"Tinto", 45, 30, 97},
		{"Urr", 10, 47, 181},
	} {
		name = append(name, g.name)
		east, north = append(east, g.east), append(north, g.north)
		rain = append(rain, g.rain)
	}
	return data.NewTable().
		String("gauge", name).
		Float64("east", east).
		Float64("north", north).
		Float64("rain", rain)
}

// gaugeCoverage is the partition drawn for its boundaries: which gauge is
// nearest here. The cells are outlined rather than told apart by colour, which
// is what naming both a fill and a colour asks for — and the gauges go on top
// as their own scatter layer.
func gaugeCoverage(out string) error {
	p := figure.New(
		figure.Size(720, 520),
		figure.Title("Which gauge speaks for this ground"),
		figure.XTitle("Easting (km)"),
		figure.YTitle("Northing (km)"),
		// Two layers, so a legend would appear by default. It would name the
		// gauges' scatter and say nothing a reader needs: the cells are told
		// apart by where they are, not by what colour they are.
		figure.Legend(false),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Voronoi(gauges(), geom.X("east"), geom.Y("north"),
		geom.Fill(ir.RGB(244, 246, 248)),
		geom.Color(ir.RGB(150, 160, 170)),
	))
	p.Add(geom.Scatter(gauges(), geom.X("east"), geom.Y("north"),
		geom.Color(ir.RGB(20, 30, 40)), geom.Size(5)))
	return p.Render(figure.SVG(out))
}

// gaugeRainfall fills the same cells with each gauge's reading. The partition
// has not changed — only what the cells are painted from — and the chart is
// now a rainfall map rather than a coverage one.
func gaugeRainfall(out string) error {
	p := figure.New(
		figure.Size(720, 520),
		figure.Title("Last month's rainfall, as the nearest gauge measured it"),
		figure.XTitle("Easting (km)"),
		figure.YTitle("Northing (km)"),
		// What the colours mean is the colourbar, which is a guide rather than
		// a legend entry, so the legend the second layer would raise is off.
		figure.Legend(false),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	// A ramp, so the guide column carries a colourbar: the reading is a
	// quantity, and a legend of twenty gauges would name the sites rather than
	// say what the colours mean.
	p.Add(geom.Voronoi(gauges(), geom.X("east"), geom.Y("north"),
		geom.ColorBy("rain", scale.Sequential(palette.Viridis)),
		geom.Label("Rainfall (mm)"),
	))
	p.Add(geom.Scatter(gauges(), geom.X("east"), geom.Y("north"),
		geom.Color(ir.RGB(255, 255, 255)), geom.Size(4)))
	return p.Render(figure.SVG(out))
}

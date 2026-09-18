// Command ternary renders the charts a barycentric coord draws, with all
// three of their components readable.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. And like examples/smith, its point is
// what is *not* here: there is no ternary geom and no soil package. Both
// charts are a geom.Scatter over two linear scales, drawn in coord.Ternary —
// the third component is derived, because two of three are free and the third
// is what is left. See docs/adr/0051-barycentric-coord.md.
//
// What is new here is the third ladder. A panel has two tick lists and a
// triangle has three edges, so until coord.Furniture carried families of its
// own the derived component was drawn and not labelled: the one edge of the
// three with no numbers on it, carrying the fraction a reader is usually
// after. Each component is now read along its own edge. See
// docs/adr/0070-a-third-labelled-family.md.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func main() {
	soil := flag.String("soil", "soil.svg", "output path for the soil texture triangle")
	qfl := flag.String("qfl", "qfl.svg", "output path for the sandstone provenance diagram")
	flag.Parse()
	if err := run(*soil, *qfl); err != nil {
		fmt.Fprintln(os.Stderr, "ternary:", err)
		os.Exit(1)
	}
}

func run(soil, qfl string) error {
	for _, step := range []func() error{
		func() error { return soilTexture(soil) },
		func() error { return provenance(qfl) },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// simplex gives a plot the two axes a ternary panel reads.
//
// Neither domain is trained and neither needs to be: the coord pins both to
// the whole simplex, because a triangle drawn around a tight cluster of
// compositions is not the simplex. The tick values are the ladder all three
// edges are printed with — every twenty per cent, which is what a soil
// triangle and a QFL diagram both use.
func simplex(p *figure.Plot) *figure.Plot {
	ticks := scale.TickValues(0, 20, 40, 60, 80, 100)
	p.X(scale.Linear(ticks))
	p.Y(scale.Linear(ticks))
	return p
}

// corners names the three components where each of them is everything, which
// is how a ternary chart says which edge is which. The numbers come from the
// coord now; the names never did.
func corners(p *figure.Plot, first, second, third string) {
	p.Add(geom.Note(100, 0, first, geom.Align(ir.AlignStart, ir.AlignTop)))
	p.Add(geom.Note(0, 100, second, geom.Align(ir.AlignEnd, ir.AlignTop)))
	p.Add(geom.Note(0, 0, third, geom.Align(ir.AlignCenter, ir.AlignBottom)))
}

// soilTexture is the chart the USDA has printed since 1911: a soil's sand,
// silt and clay fractions, which sum to a hundred by definition and so are one
// point in a triangle.
//
// Sand is on X and silt on Y, so clay — the fraction that decides how a soil
// drains and how it holds a nutrient, and the one a reader reaches for — is
// the derived component on the third edge.
func soilTexture(out string) error {
	sand, silt, name := soilSamples()
	src := figure.NewTable().
		Float64("sand", sand).
		Float64("silt", silt).
		String("horizon", name)

	p := simplex(figure.New(
		figure.Size(620, 580),
		figure.Title("Soil texture, five horizons of one profile"),
		figure.Theme(theme.Light),
		figure.Coord(coord.Ternary(coord.TernarySum(100))),
	))
	p.Add(geom.Scatter(src,
		geom.X("sand"), geom.Y("silt"), geom.Size(9),
		geom.ColorBy("horizon", scale.Qualitative(palette.OkabeIto))))
	corners(p, "sand", "silt", "clay")
	return p.Render(figure.SVG(out))
}

// provenance is the QFL diagram: quartz, feldspar and lithic fragments counted
// in a thin section, which is how a sandstone is read back to the terrain it
// was eroded from. A quartz-rich sample came a long way; a feldspar-rich one
// came off a granite nearby.
func provenance(out string) error {
	quartz, feldspar, unit := sandstones()
	src := figure.NewTable().
		Float64("quartz", quartz).
		Float64("feldspar", feldspar).
		String("unit", unit)

	p := simplex(figure.New(
		figure.Size(620, 580),
		figure.Title("Sandstone provenance, three units"),
		figure.Theme(theme.Light),
		figure.Coord(coord.Ternary(coord.TernarySum(100))),
	))
	p.Add(geom.Scatter(src,
		geom.X("quartz"), geom.Y("feldspar"), geom.Size(8),
		geom.ColorBy("unit", scale.Qualitative(palette.OkabeIto))))
	corners(p, "quartz", "feldspar", "lithics")
	return p.Render(figure.SVG(out))
}

// soilSamples is one profile's five horizons, in per cent of sand and silt.
// The clay fraction is what is left, which is the whole argument for deriving
// it: these three numbers cannot disagree about their own total.
func soilSamples() (sand, silt []float64, horizon []string) {
	sand = []float64{62, 55, 41, 33, 28}
	silt = []float64{28, 30, 36, 38, 34}
	horizon = []string{"A", "AB", "Bt1", "Bt2", "C"}
	return sand, silt, horizon
}

// sandstones is a point count of three units, in per cent of quartz and
// feldspar.
func sandstones() (quartz, feldspar []float64, unit []string) {
	quartz = []float64{
		88, 84, 91, 86, 80,
		54, 49, 58, 52, 45,
		37, 44, 31, 40, 35,
	}
	feldspar = []float64{
		7, 9, 5, 8, 12,
		33, 38, 29, 35, 41,
		22, 18, 25, 20, 27,
	}
	unit = make([]string, 0, len(quartz))
	for _, name := range []string{"upper", "middle", "lower"} {
		for i := 0; i < 5; i++ {
			unit = append(unit, name)
		}
	}
	return quartz, feldspar, unit
}

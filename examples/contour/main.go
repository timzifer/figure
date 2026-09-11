// Command contour draws one field three ways, off one colour scale.
//
// It is the example for the thing the contour exists to make possible: a plan
// and a shape that agree. The flat chart is where a reader takes a number off —
// "the 3 dB line runs here" — and the surface is where they see what the
// numbers are doing; the contours on its floor are the same lines as the flat
// chart's, because both come out of one call to stat.Contour over one lattice.
//
// # One ramp, not two like it
//
// The three pictures share a single scale.ColorScale, pinned with
// scale.ColorDomain. That is the point of the example and it is easy to get
// wrong: two scale.Sequential values over one column look identical and are two
// domains that agree by luck, so the same colour means two different numbers
// the moment either chart is drawn over a subset, a filter or a live window.
//
// Pinning it also makes the outermost isoline the end of the bar rather than
// something short of it, because the levels are derived from the same pair.
//
//	go run ./examples/contour
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
	"github.com/timzifer/figure/three"
)

// The field's range, chosen once and used everywhere: by the colour scale, by
// the levels, and therefore by all three pictures.
const (
	lo = -12.0
	hi = 6.0
)

func main() {
	flat := flag.String("flat", "contour.svg", "output path for the flat contour plot")
	over := flag.String("over", "contour-heatmap.svg", "output path for the contours over a heatmap")
	scene := flag.String("scene", "contour-floor.svg", "output path for the surface with its floor")
	flag.Parse()
	if err := run(*flat, *over, *scene); err != nil {
		fmt.Fprintln(os.Stderr, "contour:", err)
		os.Exit(1)
	}
}

func run(flat, over, scene string) error {
	// One ramp and one level list, built once and handed to every chart below.
	// Nothing here is per picture, which is the whole argument.
	ramp := scale.Sequential(palette.Viridis, scale.ColorDomain(lo, hi))
	levels := stat.Levels(lo, hi, 9)

	for _, step := range []func() error{
		func() error { return flatContour(flat, ramp, levels) },
		func() error { return contoursOverAHeatmap(over, ramp, levels) },
		func() error { return surfaceWithItsFloor(scene, ramp, levels) },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// flatContour is the plan: where the response crosses each level, and how
// steeply. Close lines are a cliff and far ones are a plain, which is the one
// thing a heatmap of the same numbers cannot say.
func flatContour(out string, ramp scale.ColorScale, levels []float64) error {
	p := figure.New(
		figure.Size(640, 460),
		figure.Title("Where the gain crosses each level"),
		figure.XTitle("bias (V)"),
		figure.YTitle("drive (dBm)"),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Contour(response(),
		geom.X("bias"), geom.Y("drive"), geom.Z("gain"),
		geom.Levels(levels...),
		geom.ColorBy("gain", ramp),
		geom.Width(1.5),
	))
	return p.Render(figure.SVG(out))
}

// contoursOverAHeatmap is the pair a map uses: the cells give every value and
// the lines give the shape between them. They are two layers over one table and
// one ramp, so a colour in the fill and a colour in a line mean the same number.
func contoursOverAHeatmap(out string, ramp scale.ColorScale, levels []float64) error {
	src := response()

	p := figure.New(
		figure.Size(640, 460),
		figure.Title("The same field, read both ways"),
		figure.XTitle("bias (V)"),
		figure.YTitle("drive (dBm)"),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(
		// BarWidth(1) closes the gutters a rect leaves between its cells.
		// A heatmap of a sampled field is a picture of a continuous thing, and
		// a grid of gaps says the measurements stop between the samples.
		geom.Rect(src, geom.X("bias"), geom.Y("drive"),
			geom.ColorBy("gain", ramp), geom.BarWidth(1)),
		// The lines are drawn in one flat colour rather than the ramp's:
		// over a filled field a coloured line competes with the fill it is
		// meant to be read against, and white reads over every part of a
		// sequential ramp that a dark line does not.
		geom.Contour(src,
			geom.X("bias"), geom.Y("drive"), geom.Z("gain"),
			geom.Levels(levels...),
			geom.Color(palette.White), geom.Width(1),
		),
	)
	return p.Render(figure.SVG(out))
}

// surfaceWithItsFloor is the shape and its plan in one picture, at one angle.
//
// The floor contours are the same runs the flat chart drew — one tracing, so a
// reading taken off the plan and one taken off the floor cannot disagree, which
// they would if each chart traced the field for itself and met a saddle.
func surfaceWithItsFloor(out string, ramp scale.ColorScale, levels []float64) error {
	src := response()

	sc := three.NewScene(
		three.XTitle("bias (V)"),
		three.YTitle("drive (dBm)"),
		three.ZTitle("gain (dB)"),
	).
		Z(scale.Linear(scale.Nice())).
		Add(
			three.Contour(src,
				geom.X("bias"), geom.Y("drive"), geom.Z("gain"),
				geom.Levels(levels...), geom.ColorBy("gain", ramp)),
			three.Surface(src,
				geom.X("bias"), geom.Y("drive"), geom.Z("gain"),
				geom.ColorBy("gain", ramp)),
		)

	return three.New(
		three.Size(660, 560),
		three.Title("The shape, and its plan beneath it"),
		three.Theme(theme.Light),
	).Scene(sc).Render(figure.SVG(out))
}

// response is a device's small-signal gain over a bias and a drive level: a
// ridge that runs diagonally and rolls off at both ends.
//
// It is generated deterministically — no math/rand, because an example whose
// picture changes between runs is an example nobody can check.
func response() figure.Source {
	const n = 40
	bias := make([]float64, 0, n*n)
	drive := make([]float64, 0, n*n)
	gain := make([]float64, 0, n*n)
	for j := range n {
		for i := range n {
			b := -1 + 2*float64(i)/(n-1)
			d := -20 + 30*float64(j)/(n-1)
			bias, drive = append(bias, b), append(drive, d)
			gain = append(gain, 4-0.02*(d+6)*(d+6)-6*(b-0.1)*(b-0.1)+1.5*b*math.Cos(d/6))
		}
	}
	return figure.NewTable().
		Float64("bias", bias).Float64("drive", drive).Float64("gain", gain)
}

// Command surface renders the chart a third scale unlocks.
//
// It is the 3D example, and like the others it is executed by a test so that
// it cannot silently stop compiling or stop producing a chart. Its point is
// the one ADR 0058 makes about the surface: a heatmap of the same grid gives
// the values and hides which way the ground falls. Both charts are drawn here,
// from one table, so that the difference is the picture rather than a claim.
//
//	go run ./examples/surface
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
	"github.com/timzifer/figure/theme"
	"github.com/timzifer/figure/three"
)

func main() {
	shape := flag.String("shape", "response.svg", "output path for the surface")
	flat := flag.String("flat", "response-flat.svg", "output path for the heatmap of the same grid")
	terrain := flag.String("terrain", "terrain.svg", "output path for the surface under a colour ramp")
	flag.Parse()
	if err := run(*shape, *flat, *terrain); err != nil {
		fmt.Fprintln(os.Stderr, "surface:", err)
		os.Exit(1)
	}
}

func run(shape, flat, terrain string) error {
	for _, step := range []func() error{
		func() error { return responseSurface(shape) },
		func() error { return responseHeatmap(flat) },
		func() error { return responseTerrain(terrain) },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// responseSurface is the chart this package exists for: the shape of a
// response between its samples.
//
// Three things are worth noticing. The scene holds the data and the three
// scales and no camera at all, which is what lets examples/views point four of
// them at one of these. The depth scale is niced exactly as a flat chart's
// vertical axis is — a third scale is a third scale and nothing about it is
// new. And nothing here says how the quads are ordered: a surface over a
// regular grid is ordered by where each cell stands on the floor, which is
// exact under an orthographic camera, so the painter's algorithm is a reading
// of the data rather than an approximation of a depth buffer.
func responseSurface(out string) error {
	src := response()

	sc := three.NewScene(
		three.XTitle("bias (V)"),
		three.YTitle("drive (dBm)"),
		three.ZTitle("gain (dB)"),
	).
		X(scale.Linear()).
		Y(scale.Linear()).
		Z(scale.Linear(scale.Nice())).
		Add(three.Surface(src,
			geom.X("bias"), geom.Y("drive"), geom.Z("gain"),
			geom.Fill(palette.Blue),
		))

	return three.New(
		three.Size(660, 520),
		three.Title("Small-signal gain over bias and drive"),
		three.Theme(theme.Light),
	).Scene(sc).Render(figure.SVG(out))
}

// responseHeatmap is the same table as the flat chart it is usually better to
// draw, and it is here because the catalogue is not neutral about which to
// choose. It gives every value exactly and says nothing about the slope
// between two of them.
func responseHeatmap(out string) error {
	src := response()

	p := figure.New(
		figure.Size(660, 460),
		figure.Title("The same table as a heatmap"),
		figure.XTitle("bias (V)"),
		figure.YTitle("drive (dBm)"),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Rect(src,
		geom.X("bias"), geom.Y("drive"),
		geom.ColorBy("gain", scale.Sequential(palette.Viridis)),
	))
	return p.Render(figure.SVG(out))
}

// responseTerrain is the same surface with a sequential ramp over its height.
//
// It is ADR 0058's rank-two "terrain" row, and it costs one option. There is
// no colourbar beside it and none is missing: the depth axis is the key, with
// its own ticks and its own title, and a second one saying the same thing in
// another channel would be a guide that repeats an axis.
func responseTerrain(out string) error {
	sc := three.NewScene(
		three.XTitle("bias (V)"),
		three.YTitle("drive (dBm)"),
		three.ZTitle("gain (dB)"),
	).
		Z(scale.Linear(scale.Nice())).
		Add(three.Surface(response(),
			geom.X("bias"), geom.Y("drive"), geom.Z("gain"),
			geom.ColorBy("gain", scale.Sequential(palette.Viridis)),
		))

	return three.New(
		three.Size(660, 520),
		three.Title("The same surface, coloured by height"),
		three.Theme(theme.Dark),
	).Scene(sc).Render(figure.SVG(out))
}

// response is a device's small-signal gain over a bias and a drive level: a
// ridge that runs diagonally and rolls off at both ends, which is the shape a
// heatmap of the same numbers cannot show.
//
// It is generated deterministically — no math/rand, because an example whose
// picture changes between runs is an example nobody can check.
func response() figure.Source {
	const n = 24
	bias := make([]float64, 0, n*n)
	drive := make([]float64, 0, n*n)
	gain := make([]float64, 0, n*n)
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			b := 1 + 2*float64(i)/float64(n-1)
			d := -20 + 25*float64(j)/float64(n-1)
			// A ridge along the bias/drive diagonal, compressing as the drive
			// rises: the two readings an amplifier's designer wants at once.
			ridge := math.Exp(-math.Pow((b-1.9)-(d+12)/22, 2) * 3)
			comp := 1 / (1 + math.Exp((d+2)/3))
			bias = append(bias, b)
			drive = append(drive, d)
			gain = append(gain, 6+14*ridge*comp)
		}
	}
	return figure.NewTable().
		Float64("bias", bias).
		Float64("drive", drive).
		Float64("gain", gain)
}

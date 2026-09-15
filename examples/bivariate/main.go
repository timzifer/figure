// Command bivariate renders the three charts a colour channel with two
// readings unlocks.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. None of the three has a mark of its own,
// which is the argument of docs/adr/0067-a-bivariate-colour-channel.md: one
// optional interface beside scale.ColorScale, one option on the layer, and a
// square key, and three unrelated-looking charts fall out.
//
//   - A value-suppressing uncertainty palette over a field of estimates: the
//     less certain an estimate, the fewer colours it may be one of.
//   - A bivariate square: two rates per region, crossed into nine colours.
//   - A multi-class hexbin: which class dominates each cell, and how purely.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func main() {
	vsup := flag.String("vsup", "vsup.svg", "output path for the value-suppressing palette")
	square := flag.String("square", "square.svg", "output path for the bivariate square")
	hex := flag.String("hexbin", "hexbin.svg", "output path for the multi-class hexbin")
	flag.Parse()
	if err := run(*vsup, *square, *hex); err != nil {
		fmt.Fprintln(os.Stderr, "bivariate:", err)
		os.Exit(1)
	}
}

func run(vsup, square, hex string) error {
	for _, step := range []func() error{
		func() error { return estimates(vsup) },
		func() error { return rates(square) },
		func() error { return classes(hex) },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// field is a grid of estimates and their standard errors: a smooth quantity
// measured densely on the left of the grid and sparsely on the right, so the
// uncertainty grows across it while the estimate does whatever the field does.
func field() (x, y, mean, sd []float64) {
	for j := range 12 {
		for i := range 16 {
			fx, fy := float64(i), float64(j)
			x, y = append(x, fx), append(y, fy)
			mean = append(mean, 20+8*math.Sin(fx/3)*math.Cos(fy/4)+0.4*fx)
			sd = append(sd, 0.5+0.35*fx)
		}
	}
	return
}

// estimates draws the field under a value-suppressing palette. On the left,
// where the estimates are good, eight classes of the ramp tell them apart; on
// the right, where they are not, a single muted colour says only that nothing
// can be said — which is what the numbers support, and what a conventional
// heatmap of the same means would have hidden behind a confident gradient.
func estimates(out string) error {
	x, y, mean, sd := field()
	src := figure.NewTable().Float64("x", x).Float64("y", y).Float64("mean", mean).Float64("sd", sd)
	p := figure.New(
		figure.Size(720, 420),
		figure.Title("Estimates, and how well they are known"),
		figure.Theme(theme.Light),
		figure.XTitle("station"),
		figure.YTitle("depth band"),
	)
	p.X(scale.Linear(scale.Domain(-0.5, 15.5)))
	p.Y(scale.Linear(scale.Domain(-0.5, 11.5)))
	p.Add(geom.Rect(src, geom.X("x"), geom.Y("y"),
		geom.ColorBy("mean", scale.VSUP(palette.Viridis, 8, 4)),
		geom.UncertaintyBy("sd"),
		geom.Label("mean")))
	return p.Render(figure.SVG(out))
}

// rates draws two rates per region crossed into the 3×3 square: the oldest of
// the three charts, and the only one most readers have seen.
func rates(out string) error {
	var x, y, a, b []float64
	for j := range 8 {
		for i := range 10 {
			fx, fy := float64(i), float64(j)
			x, y = append(x, fx), append(y, fy)
			a = append(a, math.Mod(fx*0.37+fy*0.11, 1))
			b = append(b, math.Mod(fy*0.29+fx*0.07+0.3, 1))
		}
	}
	src := figure.NewTable().Float64("x", x).Float64("y", y).Float64("obesity", a).Float64("inactivity", b)
	p := figure.New(
		figure.Size(640, 420),
		figure.Title("Two rates, one colour"),
		figure.Theme(theme.Light),
		figure.Legend(false),
	)
	p.X(scale.Linear(scale.Domain(-0.5, 9.5)))
	p.Y(scale.Linear(scale.Domain(-0.5, 7.5)))
	p.Add(geom.Rect(src, geom.X("x"), geom.Y("y"),
		geom.ColorBy("obesity", scale.BivariateMatrix(palette.BivariateBlueRed)),
		geom.UncertaintyBy("inactivity"),
		geom.Label("obesity")))
	return p.Render(figure.SVG(out))
}

// classes draws three overlapping clouds as a multi-class hexbin. A density
// raster of the same rows would say how many; this says whose, and where the
// clouds overlap it says so by fading to the matrix's mixed column.
func classes(out string) error {
	var x, y []float64
	var class []string
	centres := [][2]float64{{-1, 0}, {1, 0.4}, {0, 1.4}}
	names := []string{"setosa", "versicolor", "virginica"}
	state := uint64(0x9e3779b97f4a7c15)
	gauss := func() float64 {
		// A Box–Muller pair from a fixed xorshift, so the chart is the same
		// every time.
		next := func() float64 {
			state ^= state << 13
			state ^= state >> 7
			state ^= state << 17
			return (float64(state>>11) + 0.5) / (1 << 53)
		}
		return math.Sqrt(-2*math.Log(next())) * math.Cos(2*math.Pi*next())
	}
	for k, c := range centres {
		for range 700 {
			x, y = append(x, c[0]+0.7*gauss()), append(y, c[1]+0.7*gauss())
			class = append(class, names[k])
		}
	}
	src := figure.NewTable().Float64("x", x).Float64("y", y).String("class", class)
	p := figure.New(
		figure.Size(640, 480),
		figure.Title("Who is where, and how mixed"),
		figure.Theme(theme.Light),
	)
	p.X(scale.Linear(scale.Domain(-3, 3)))
	p.Y(scale.Linear(scale.Domain(-2, 3.5)))
	// Three classes down the rows and purity across: each class's own colour
	// where a cell is all that class, faded toward grey as the cell mixes.
	// It is the matrix a multi-class bin wants rather than a square of two
	// quantities — the first reading is a name, and a name wants its own hue.
	p.Add(geom.Hexbin(src, geom.X("x"), geom.Y("y"), geom.GroupBy("class"),
		geom.ColorBy("class", scale.BivariateMatrix(classPurity(3, 3))),
		geom.DensityCells(14)))
	return p.Render(figure.SVG(out))
}

// classPurity is a matrix of n class colours, each in `levels` steps from its
// own hue at full purity toward a neutral grey at an even split.
func classPurity(n, levels int) [][]ir.Color {
	grey := ir.RGB(0xbd, 0xbd, 0xbd)
	out := make([][]ir.Color, n)
	for k := range out {
		base := palette.OkabeIto[(k+1)%len(palette.OkabeIto)]
		for j := range levels {
			out[k] = append(out[k], palette.Lerp(base, grey, 0.8*float64(j)/float64(levels-1)))
		}
	}
	return out
}

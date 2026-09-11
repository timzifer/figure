// Command views renders one scene from several cameras at once.
//
// It is the example for the arrangement the 3D package is built on: a scene is
// the data, a camera is a way of looking at it, and the two are separate
// values. **One data repository, several views on it.** The scene below is
// built once and trained once; four cameras are pointed at it and land as four
// cells of one figure, so the same surface is read as a shape, as two profiles
// and as a plan.
//
// It also shows what an orbit is, without opening a window: a camera is an
// immutable value and three.Orbit is a pure function from one to another, so
// turning the scene is arithmetic the host does and this library never sees a
// pointer. See turn() at the bottom, which is the whole of the host's side.
//
//	go run ./examples/views
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
	four := flag.String("o", "views.svg", "output path for the four-view figure")
	turned := flag.String("turn", "turned.svg", "output path for the scene after an orbit")
	compare := flag.String("compare", "compare.svg", "output path for two scenes from one angle")
	flag.Parse()
	if err := run(*four, *turned, *compare); err != nil {
		fmt.Fprintln(os.Stderr, "views:", err)
		os.Exit(1)
	}
}

func run(four, turned, compare string) error {
	for _, step := range []func() error{
		func() error { return fourViews(four) },
		func() error { return afterAnOrbit(turned) },
		func() error { return twoScenes(compare) },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// fourViews is the figure this example is about.
//
// The scene is one value and it appears once. Every view names a camera and a
// label and nothing else — no data, no scales, no theme — because those are
// facts about what is plotted rather than about where it is looked at from.
// The scales are trained once for all four cells, which is not an optimisation
// but a correctness property: a scale accumulates, so training it per view
// would give the layer four times its weight and move the domain.
func fourViews(out string) error {
	sc := saddle()

	return three.New(
		three.Size(960, 720),
		three.Title("One surface, four cameras"),
		three.Theme(theme.Light),
		three.Columns(2),
	).Scene(sc).Add(
		three.View{Camera: three.Home(), Label: "three-quarter"},
		three.View{Camera: three.LookAt(three.Elevation(1.45)), Label: "plan"},
		three.View{Camera: three.LookAt(three.Azimuth(0), three.Elevation(0.02)), Label: "front"},
		three.View{Camera: three.LookAt(three.Azimuth(-math.Pi/2), three.Elevation(0.02)), Label: "side"},
	).Render(figure.SVG(out))
}

// afterAnOrbit draws the same scene at a camera the host computed.
//
// This is what a drag amounts to. The host owns the event, converts pixels
// into radians with its own constant, and hands the result back as a value;
// figure installs no handler, opens no window and runs no loop. In a live
// surface the two lines in turn() are followed by live.Draw(), and nothing
// else about the chart changes.
func afterAnOrbit(out string) error {
	cam := turn(three.Home(), 140, -60)

	return three.New(
		three.Size(660, 520),
		three.Title("The same scene, after a drag of 140 by -60 pixels"),
		three.Theme(theme.Light),
	).Scene(saddle()).
		Add(three.View{Camera: cam}).
		Render(figure.SVG(out))
}

// twoScenes is the other half of the arrangement: a view may bring its own
// scene, so one figure compares two datasets from one angle.
//
// It is the same mechanism read the other way round. A view is a camera *and*
// what it is pointed at, and leaving the second half out is the ordinary case
// rather than the only one.
func twoScenes(out string) error {
	cam := three.LookAt(three.Azimuth(-0.7), three.Elevation(0.4))

	return three.New(
		three.Size(960, 420),
		three.Title("Two measurements from one angle"),
		three.Theme(theme.Light),
		three.Columns(2),
	).Add(
		three.View{Camera: cam, Label: "as designed", Scene: saddle()},
		three.View{Camera: cam, Label: "as built", Scene: dented()},
	).Render(figure.SVG(out))
}

// turn is the host's entire side of an orbit.
//
// The constant is the host's, not figure's: how many radians a pixel of drag
// is worth is a statement about how the interaction feels, and feel belongs to
// whoever owns the input layer. Inertia, momentum and springs are the same
// argument and are not here either. The signs make a drag take hold of the
// scene: the side facing the reader follows the pointer. See [three.Orbit].
func turn(cam three.Camera, dx, dy float64) three.Camera {
	const perPixel = 0.008 // radians
	return three.Orbit(cam, -dx*perPixel, dy*perPixel)
}

// saddle is a response with a ridge in one direction and a trough in the
// other: the shape whose whole point is that a heatmap of it looks symmetric
// and it is not.
func saddle() *three.Scene { return sceneOf(saddleSource()) }

// dented is the same response with a notch cut into it, which is what a
// measured part looks like beside a designed one.
func dented() *three.Scene { return sceneOf(dentedSource()) }

func saddleSource() figure.Source {
	return grid(func(x, y float64) float64 {
		return math.Sin(x)*math.Cos(y)*1.4 + 0.25*x
	})
}

func dentedSource() figure.Source {
	return grid(func(x, y float64) float64 {
		z := math.Sin(x)*math.Cos(y)*1.4 + 0.25*x
		return z - 1.6*math.Exp(-((x-1.2)*(x-1.2)+(y+0.9)*(y+0.9))*2)
	})
}

func sceneOf(src figure.Source) *three.Scene {
	return three.NewScene(
		three.XTitle("x"),
		three.YTitle("y"),
		three.ZTitle("z"),
	).
		Z(scale.Linear(scale.Nice())).
		Add(three.Surface(src,
			geom.X("x"), geom.Y("y"), geom.Z("z"),
			geom.Fill(palette.Blue),
		))
}

// grid samples a function over a regular lattice, which is what a surface
// requires: the rows must be the full product of the distinct x and y values,
// each cell present exactly once. Anything else is an error rather than a
// picture with holes in it.
func grid(f func(x, y float64) float64) figure.Source {
	const n = 22
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	zs := make([]float64, 0, n*n)
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			x := -3 + 6*float64(i)/float64(n-1)
			y := -3 + 6*float64(j)/float64(n-1)
			xs = append(xs, x)
			ys = append(ys, y)
			zs = append(zs, f(x, y))
		}
	}
	return figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("z", zs)
}

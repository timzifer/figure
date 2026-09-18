//go:build !race

package geom_test

// Apart from contour_test.go because the race detector allocates on figure's
// behalf and would fail this for nothing figure did. See AGENTS.md.

import (
	"image"
	"math"
	"testing"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// A redrawn contour traces into the same memory, which is why stat.Contour is a
// struct with a Reset.
func TestARedrawnContourDoesNotAllocatePerRow(t *testing.T) {
	src := field(24, 24, func(x, y float64) float64 { return math.Sin(x) * math.Cos(y) })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"), geom.LevelCount(6))

	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	small := geom.Contour(field(6, 6, func(x, y float64) float64 { return x * y }),
		geom.X("x"), geom.Y("y"), geom.Z("z"), geom.LevelCount(6))
	if err := small.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); err != nil {
		t.Fatal(err)
	}

	big := testing.AllocsPerRun(10, func() {
		if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
			t.Fatal(err)
		}
	})
	tiny := testing.AllocsPerRun(10, func() {
		if err := small.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); err != nil {
			t.Fatal(err)
		}
	})
	if big > tiny+2 {
		t.Errorf("a 24x24 field allocates %.0f times against a 6x6 field's %.0f", big, tiny)
	}
}

// A labelled contour redrawn allocates no more than an unlabelled one: the two
// halves a gap cuts the run into are built into buffers the layer keeps, beside
// the device-point scratch it already had. See
// docs/adr/0073-labels-on-a-curve.md.
func TestARedrawnLabelledContourDoesNotAllocatePerLabel(t *testing.T) {
	src := field(24, 24, func(x, y float64) float64 { return math.Sin(x) * math.Cos(y) })
	build := func(opts ...geom.Option) func() float64 {
		g := geom.Contour(src, append([]geom.Option{
			geom.X("x"), geom.Y("y"), geom.Z("z"), geom.LevelCount(6)}, opts...)...)
		x, y := scale.Linear(), scale.Linear()
		if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
			t.Fatal(err)
		}
		area := ir.R(0, 0, 400, 300)
		x.SetRange(area.Min.X, area.Max.X)
		y.SetRange(area.Max.Y, area.Min.Y)
		// A backend that keeps nothing: what is measured here is what the
		// layer allocates, not what a recorder does with its calls.
		b := discarding{irtest.New()}
		f := geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}
		return func() float64 {
			return testing.AllocsPerRun(10, func() {
				if err := g.Build(b, f); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	plain, labelled := build()(), build(geom.LabelLevels(true))()
	if labelled > plain+2 {
		t.Errorf("a labelled redraw allocates %.0f times against an unlabelled one's %.0f", labelled, plain)
	}
}

// discarding is an ir.Backend that draws nothing and measures like the
// recorder, so that an allocation count is the layer's own.
type discarding struct{ *irtest.Recorder }

func (discarding) Polyline([]ir.Point, ir.Stroke)                {}
func (discarding) StrokePath(*ir.Path, ir.Stroke)                {}
func (discarding) FillPath(*ir.Path, ir.Fill, ir.FillRule)       {}
func (discarding) Text(ir.TextRun)                               {}
func (discarding) Markers(ir.Marker, []ir.Point, ir.MarkerStyle) {}
func (discarding) Image(image.Image, ir.Rect)                    {}
func (discarding) Push(*ir.Path, ir.Affine)                      {}
func (discarding) Pop()                                          {}
func (discarding) Flush() error                                  { return nil }

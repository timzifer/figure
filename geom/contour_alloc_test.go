//go:build !race

package geom_test

// Apart from contour_test.go because the race detector allocates on figure's
// behalf and would fail this for nothing figure did. See AGENTS.md.

import (
	"math"
	"testing"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
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

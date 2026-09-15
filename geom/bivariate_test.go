package geom_test

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

func estimates() data.Source {
	return data.NewTable().
		Float64("x", []float64{0, 1, 0, 1}).
		Float64("y", []float64{0, 0, 1, 1}).
		Float64("mean", []float64{10, 40, 10, 40}).
		Float64("sd", []float64{0, 0, 10, 10})
}

// The point of a value-suppressing palette: two estimates that differ are two
// colours while they are certain, and one colour once the uncertainty is
// larger than the difference can support.
func TestAnUncertainPairOfEstimatesIsPaintedOneColour(t *testing.T) {
	cs := scale.VSUP(palette.Viridis, 2, 2)
	g := geom.Rect(estimates(), geom.X("x"), geom.Y("y"),
		geom.ColorBy("mean", cs), geom.UncertaintyBy("sd"))
	rec := build(t, g, scale.Linear(), scale.Linear())
	got := map[[3]uint8]bool{}
	for _, f := range rec.Filter("FillPath") {
		got[[3]uint8{f.Fill.Color.R, f.Fill.Color.G, f.Fill.Color.B}] = true
	}
	// Certain row: two classes, two colours. Uncertain row: one class, one
	// colour. Three distinct colours in all.
	if len(got) != 3 {
		t.Errorf("%d distinct colours, want 3: two certain estimates apart and two uncertain ones together", len(got))
	}
	if c, _ := geom.Describe(g); c.UncertaintyCol != "sd" {
		t.Errorf("Desc.UncertaintyCol = %q", c.UncertaintyCol)
	}
	if lo, hi := cs.SecondDomain(); lo != 0 || hi != 10 {
		t.Errorf("the second reading trained to [%v, %v], want [0, 10]", lo, hi)
	}
}

func TestASecondColumnNeedsAScaleThatReadsIt(t *testing.T) {
	g := geom.Rect(estimates(), geom.X("x"), geom.Y("y"),
		geom.ColorBy("mean", scale.Sequential(palette.Viridis)), geom.UncertaintyBy("sd"))
	if err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); !errors.Is(err, geom.ErrNotBivariate) {
		t.Errorf("err = %v, want ErrNotBivariate", err)
	}
	line := geom.Line(estimates(), geom.X("x"), geom.Y("y"),
		geom.ColorBy("mean", scale.VSUP(palette.Viridis, 4, 2)), geom.UncertaintyBy("sd"))
	if err := line.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); !errors.Is(err, geom.ErrRampOnPath) {
		t.Errorf("a line with two readings: err = %v, want ErrRampOnPath", err)
	}
}

// A multi-class hexbin colours each cell by which class dominates it and how
// purely, and names the classes in its legend.
func TestAMultiClassHexbinSaysWhoAndHowPurely(t *testing.T) {
	var xs, ys []float64
	var class []string
	for i := range 60 {
		a := float64(i)
		// Two tight clouds, one of each class, and a mixed one between.
		xs = append(xs, 10+math.Mod(a, 3), 90+math.Mod(a, 3), 50+math.Mod(a, 3))
		ys = append(ys, 50, 50, 50)
		mixed := "a"
		if i%2 == 1 {
			mixed = "b"
		}
		class = append(class, "a", "b", mixed)
	}
	src := data.NewTable().Float64("x", xs).Float64("y", ys).String("k", class)
	cs := scale.BivariateMatrix(palette.BivariateBlueRed)
	g := geom.Hexbin(src, geom.X("x"), geom.Y("y"), geom.GroupBy("k"), geom.ColorBy("k", cs), geom.DensityCells(8))
	rec, f := frameWith(t, g, scale.Linear(scale.Domain(0, 100)), scale.Linear(scale.Domain(0, 100)))
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	colours := map[[3]uint8]bool{}
	for _, c := range rec.Filter("FillPath") {
		colours[[3]uint8{c.Fill.Color.R, c.Fill.Color.G, c.Fill.Color.B}] = true
	}
	if len(colours) < 3 {
		t.Errorf("%d cell colours, want at least a pure a, a pure b and a mixed cell", len(colours))
	}
	es := g.(geom.Legender).Legends(f)
	if len(es) != 2 || es[0].Label != "a" || es[1].Label != "b" {
		t.Errorf("legend %v, want the two classes in order of appearance", es)
	}
}

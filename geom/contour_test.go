package geom_test

import (
	"math"
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// field is a table of (x, y, z) rows over a regular grid, which is what a
// contour requires and will not guess at.
func field(nx, ny int, f func(x, y float64) float64) data.Source {
	xs := make([]float64, 0, nx*ny)
	ys := make([]float64, 0, nx*ny)
	zs := make([]float64, 0, nx*ny)
	for j := range ny {
		for i := range nx {
			x := -3 + 6*float64(i)/float64(nx-1)
			y := -3 + 6*float64(j)/float64(ny-1)
			xs, ys = append(xs, x), append(ys, y)
			zs = append(zs, f(x, y))
		}
	}
	return data.Float64Columns(map[string][]float64{"x": xs, "y": ys, "z": zs})
}

func drawContour(t *testing.T, g geom.Geom) *irtest.Recorder {
	t.Helper()
	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("training the contour: %v", err)
	}
	area := ir.R(0, 0, 300, 200)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)

	rec := irtest.New()
	if err := g.Build(rec, geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}); err != nil {
		t.Fatalf("drawing the contour: %v", err)
	}
	return rec
}

// A contour draws one stroke per level, because the runs come out of stat
// grouped by level and a level is one colour.
func TestAContourStrokesOncePerLevel(t *testing.T) {
	src := field(16, 16, func(x, y float64) float64 { return math.Sin(x) * math.Cos(y) })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.Levels(-0.5, 0, 0.5))

	rec := drawContour(t, g)
	if got := rec.Count("StrokePath"); got != 3 {
		t.Errorf("three levels took %d strokes, want three", got)
	}
	for _, c := range rec.Filter("StrokePath") {
		if len(c.Path.Pts) < 2 {
			t.Error("a level was stroked with nothing in it")
		}
	}
}

// The levels are the caller's when they named them, and a round set chosen from
// the data when they did not.
func TestAContourChoosesRoundLevelsWhenToldNone(t *testing.T) {
	src := field(16, 16, func(x, y float64) float64 { return x + y })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"), geom.LevelCount(6))

	rec := drawContour(t, g)
	lv, ok := g.(interface{ Levels() []float64 })
	if !ok {
		t.Fatal("a contour does not report its levels")
	}
	levels := lv.Levels()
	if len(levels) == 0 {
		t.Fatal("no levels were chosen")
	}
	for _, v := range levels {
		if math.Abs(v-math.Round(v)) > 1e-9 {
			t.Errorf("the levels are %v, which are not round numbers", levels)
			break
		}
	}
	if rec.Count("StrokePath") != len(levels) {
		t.Errorf("%d levels took %d strokes", len(levels), rec.Count("StrokePath"))
	}
}

// The axes describe the grid, because what a contour computes is what they
// describe — which is why it runs in Train.
func TestAContourTrainsTheAxesOnItsGrid(t *testing.T) {
	src := field(8, 8, func(x, y float64) float64 { return x * y })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"))

	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []scale.Scale{x, y} {
		lo, hi := s.Domain()
		if lo != -3 || hi != 3 {
			t.Errorf("an axis reports [%v, %v], want the grid's own [-3, 3]", lo, hi)
		}
	}
}

// A table that is not a grid is an error rather than a picture with holes in
// it, and the message says which mark and which rows.
func TestAContourRefusesATableThatIsNotAGrid(t *testing.T) {
	src := data.Float64Columns(map[string][]float64{
		"x": {0, 1, 0, 0},
		"y": {0, 0, 1, 1},
		"z": {1, 2, 3, 4},
	})
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"))

	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if err == nil {
		t.Fatal("a table with two rows at one node was accepted")
	}
	if !strings.Contains(err.Error(), "contour") || !strings.Contains(err.Error(), "one value per cell") {
		t.Errorf("the error is %q, which does not say what is wrong with the table", err)
	}
}

// A contour with a ramp gets a colourbar without being asked for one. It can
// where a hexbin cannot, because its levels are settled before anything is
// measured.
func TestAContourWithARampOffersAColourbar(t *testing.T) {
	src := field(12, 12, func(x, y float64) float64 { return x * y })
	ramp := scale.Sequential(palette.Viridis)
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.LevelCount(5), geom.ColorBy("z", ramp))

	if err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); err != nil {
		t.Fatal(err)
	}
	guided, ok := g.(geom.Guided)
	if !ok {
		t.Fatal("a contour offers no colour guide at all")
	}
	guide, ok := guided.ColorGuide()
	if !ok {
		t.Fatal("a contour with a ramp offered no colourbar")
	}
	if guide.Label != "z" {
		t.Errorf("the colourbar is titled %q, want the column it reads", guide.Label)
	}
	if guide.Scale != scale.ColorScale(ramp) {
		t.Error("the colourbar carries a scale the layer does not draw from")
	}
}

// Each level is drawn in its own colour, so that close lines and far ones are
// told apart by more than their spacing.
func TestEachLevelIsDrawnInItsOwnColour(t *testing.T) {
	src := field(16, 16, func(x, y float64) float64 { return math.Sin(x) * math.Cos(y) })
	g := geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
		geom.Levels(-0.5, 0, 0.5), geom.ColorBy("z", scale.Sequential(palette.Viridis)))

	rec := drawContour(t, g)
	seen := map[ir.Color]bool{}
	for _, c := range rec.Filter("StrokePath") {
		seen[c.Stroke.Color] = true
	}
	if len(seen) != 3 {
		t.Errorf("three levels were drawn in %d colours", len(seen))
	}
}

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

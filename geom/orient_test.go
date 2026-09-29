package geom_test

import (
	"math"
	"sort"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// The panel the orientation tests draw on. It is square so that a rectangle
// turned a quarter is compared in the same units it was drawn in.
const side = 300

// unit is a rectangle read back in slot and value space as fractions of the
// panel: where along its slot axis it sits, and how far up its value axis it
// reaches. A vertical and a horizontal layer of the same data give the same
// units, which is the whole claim of the option.
type unit struct{ s0, s1, v0, v1 float64 }

func vertical(r ir.Rect) unit {
	return unit{
		s0: float64(r.Min.X) / side, s1: float64(r.Max.X) / side,
		v0: 1 - float64(r.Max.Y)/side, v1: 1 - float64(r.Min.Y)/side,
	}
}

func horizontal(r ir.Rect) unit {
	return unit{
		s0: 1 - float64(r.Max.Y)/side, s1: 1 - float64(r.Min.Y)/side,
		v0: float64(r.Min.X) / side, v1: float64(r.Max.X) / side,
	}
}

func sortedUnits(us []unit) []unit {
	sort.Slice(us, func(i, j int) bool {
		if us[i].s0 != us[j].s0 {
			return us[i].s0 < us[j].s0
		}
		return us[i].v0 < us[j].v0
	})
	return us
}

// turned builds the same layer both ways round — the horizontal one with its
// columns exchanged, as a caller writes it — on scales made fresh for each, and
// compares the rectangles in slot and value space.
func turned(t *testing.T, up, across geom.Geom, slot, value func() scale.Scale) {
	t.Helper()
	recUp, fUp := frameOn(t, up, slot(), value(), side, side)
	if err := up.Build(recUp, fUp); err != nil {
		t.Fatalf("Build vertical: %v", err)
	}
	recAcross, fAcross := frameOn(t, across, value(), slot(), side, side)
	if err := across.Build(recAcross, fAcross); err != nil {
		t.Fatalf("Build horizontal: %v", err)
	}
	var a, b []unit
	for _, r := range allRects(t, recUp) {
		a = append(a, vertical(r))
	}
	for _, r := range allRects(t, recAcross) {
		b = append(b, horizontal(r))
	}
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("%d rectangles vertical and %d horizontal", len(a), len(b))
	}
	a, b = sortedUnits(a), sortedUnits(b)
	for i := range a {
		d := math.Max(math.Max(math.Abs(a[i].s0-b[i].s0), math.Abs(a[i].s1-b[i].s1)),
			math.Max(math.Abs(a[i].v0-b[i].v0), math.Abs(a[i].v1-b[i].v1)))
		if d > 1e-4 {
			t.Errorf("rectangle %d: %+v vertical, %+v horizontal", i, a[i], b[i])
		}
	}
}

func linear() scale.Scale  { return scale.Linear() }
func ordinal() scale.Scale { return scale.Ordinal() }

func TestAHorizontalBarIsTheVerticalOneTurned(t *testing.T) {
	src := data.NewTable().
		String("fruit", []string{"apple", "pear", "plum"}).
		Float64("n", []float64{3, 7, 5})
	turned(t,
		geom.Bar(src, geom.X("fruit"), geom.Y("n")),
		geom.Bar(src, geom.Y("fruit"), geom.X("n"), geom.Orient(geom.Horizontal)),
		ordinal, linear)
}

func TestAHorizontalBarOnAContinuousSlotAxisIsTurnedToo(t *testing.T) {
	src := data.NewTable().
		Float64("t", []float64{0, 1, 2, 4}).
		Float64("v", []float64{2, -1, 3, 1})
	turned(t,
		geom.Bar(src, geom.X("t"), geom.Y("v"), geom.BarWidth(0.6)),
		geom.Bar(src, geom.Y("t"), geom.X("v"), geom.BarWidth(0.6), geom.Orient(geom.Horizontal)),
		linear, linear)
}

func TestAHorizontalStackAndDodgeAreTurned(t *testing.T) {
	turned(t,
		geom.Bar(longTable(), geom.X("t"), geom.Y("v"), geom.GroupBy("series")),
		geom.Bar(longTable(), geom.Y("t"), geom.X("v"), geom.GroupBy("series"), geom.Orient(geom.Horizontal)),
		linear, linear)
}

// A dodged horizontal group reads down the page in the order its legend does:
// the first series is the top bar of its slot, not the bottom one a literal
// quarter turn would give.
func TestAHorizontalDodgeListsItsFirstSeriesOnTop(t *testing.T) {
	g := geom.Bar(longTable(), geom.Y("t"), geom.X("v"), geom.GroupBy("series"),
		geom.Dodge(0.1), geom.Orient(geom.Horizontal))
	rec, f := frameOn(t, g, scale.Linear(), scale.Linear(), side, side)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	// One fill call per series, in series order; the first slot (t = 0) is
	// the bottom one, so each series' bar there is its lowest.
	var tops []float32
	for _, c := range rec.Calls {
		if c.Op != "FillPath" {
			continue
		}
		low := float32(-1)
		for _, r := range rectsOf(t, c) {
			if r.Max.Y > low {
				low = r.Max.Y
			}
		}
		tops = append(tops, low)
	}
	if len(tops) != 3 || !(tops[0] < tops[1] && tops[1] < tops[2]) {
		t.Errorf("the first slot's bars end at y = %v, want series a above b above c", tops)
	}
}

// A floating bar names both ends of its value. Under Horizontal the far end
// is X2, because the value runs across X; Y2 would be the slot's far edge.
func TestAHorizontalFloatingBarReadsItsFarEndFromX2(t *testing.T) {
	src := data.NewTable().
		Float64("t", []float64{0, 1, 2}).
		Float64("lo", []float64{1, 2, 0}).
		Float64("hi", []float64{4, 3, 5})
	turned(t,
		geom.Bar(src, geom.X("t"), geom.Y("lo"), geom.Y2("hi")),
		geom.Bar(src, geom.Y("t"), geom.X("lo"), geom.X2("hi"), geom.Orient(geom.Horizontal)),
		linear, linear)
}

// A horizontal bar's baseline is on the value axis, so it is the value axis
// that is trained to include it.
func TestAHorizontalBarTrainsItsBaselineAcross(t *testing.T) {
	src := data.NewTable().Float64("t", []float64{0, 1}).Float64("v", []float64{5, 8})
	g := geom.Bar(src, geom.Y("t"), geom.X("v"), geom.Orient(geom.Horizontal))
	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	if lo, _ := x.Domain(); lo > 0 {
		t.Errorf("the value axis starts at %v, want the baseline 0 in it", lo)
	}
	if lo, _ := y.Domain(); lo >= 0 {
		t.Errorf("the slot axis starts at %v, want it widened by half a slot", lo)
	}
}

// The orientation is written down, so a document of a horizontal bar reads
// back as one — with the columns the caller named, not the swapped roles.
func TestABarsOrientationIsWrittenDown(t *testing.T) {
	src := data.NewTable().Float64("t", []float64{0, 1}).Float64("v", []float64{5, 8})
	g := geom.Bar(src, geom.Y("t"), geom.X("v"), geom.Orient(geom.Horizontal))
	d := g.(geom.Describer).Describe()
	if d.Orient != geom.Horizontal || d.X != "v" || d.Y != "t" {
		t.Fatalf("Describe: orient %v, X %q, Y %q", d.Orient, d.X, d.Y)
	}
	back, err := geom.FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	if again := back.(geom.Describer).Describe(); again.Orient != geom.Horizontal {
		t.Errorf("the rebuilt bar reports orientation %v", again.Orient)
	}
}

func TestAHorizontalHistogramBinsTheYColumn(t *testing.T) {
	src := data.NewTable().Float64("v", []float64{1, 2, 2, 3, 3, 3, 4, 4, 5, 9})
	turned(t,
		geom.Histogram(src, geom.X("v"), geom.Bins(5)),
		geom.Histogram(src, geom.Y("v"), geom.Bins(5), geom.Orient(geom.Horizontal)),
		linear, linear)
}

func TestAHorizontalBoxplotIsTheVerticalOneTurned(t *testing.T) {
	src := data.NewTable().
		String("g", []string{"a", "a", "a", "a", "a", "b", "b", "b", "b", "b"}).
		Float64("v", []float64{1, 2, 3, 4, 20, 5, 6, 6, 7, 8})
	turned(t,
		geom.Boxplot(src, geom.X("g"), geom.Y("v")),
		geom.Boxplot(src, geom.Y("g"), geom.X("v"), geom.Orient(geom.Horizontal)),
		ordinal, linear)

	// The whiskers and the outlier are turned with the box: the outlier at 20
	// is the rightmost thing drawn, not the topmost.
	g := geom.Boxplot(src, geom.Y("g"), geom.X("v"), geom.Orient(geom.Horizontal))
	rec, f := frameOn(t, g, scale.Linear(), scale.Ordinal(), side, side)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Calls {
		if c.Op == "Markers" {
			if len(c.Points) != 1 || c.Points[0].X < side*0.95 {
				t.Errorf("outliers at %v, want one at the right-hand end", c.Points)
			}
		}
	}
}

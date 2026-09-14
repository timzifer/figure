package geom_test

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// series builds a table of (t, v) rows, which is all a horizon reads.
func horizonSource(vs ...float64) data.Source {
	ts := make([]float64, len(vs))
	for i := range vs {
		ts[i] = float64(i)
	}
	return data.Float64Columns(map[string][]float64{"t": ts, "v": vs})
}

// drawHorizon trains and builds a layer against a panel, and hands back what it
// emitted.
func drawHorizon(t *testing.T, g geom.Geom, f func(*geom.Frame)) *irtest.Recorder {
	t.Helper()
	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("training the horizon: %v", err)
	}
	area := ir.R(0, 0, 400, 60)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)

	frame := geom.Frame{Area: area, X: x, Y: y, Theme: theme.Light}
	if f != nil {
		f(&frame)
	}
	rec := irtest.New()
	if err := g.Build(rec, frame); err != nil {
		t.Fatalf("drawing the horizon: %v", err)
	}
	return rec
}

// The fold replaces the vertical axis rather than describing it: after Train
// the domain is one band, and that is what makes the scale's own ticks true of
// every band on screen.
func TestAHorizonTrainsItsAxisToOneBand(t *testing.T) {
	g := geom.Horizon(horizonSource(0, 30, 90, -45), geom.X("t"), geom.Y("v"), geom.Bands(3))
	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	if lo, hi := y.Domain(); lo != 0 || hi != 1 {
		t.Errorf("the folded axis runs [%v, %v], want [0, 1]", lo, hi)
	}
	// The horizontal axis is the data's, untouched by the fold.
	if lo, hi := x.Domain(); lo != 0 || hi != 3 {
		t.Errorf("the time axis runs [%v, %v], want the data's own [0, 3]", lo, hi)
	}
}

// One filled run per band a value actually reaches, and none for a band nothing
// got into. A chart whose series never goes below its origin must not pay for
// three empty bands below it on every frame.
func TestAHorizonFillsOncePerBandReached(t *testing.T) {
	// Three bands of 30 over a reach of 90, every one of them entered.
	up := geom.Horizon(horizonSource(0, 25, 55, 90), geom.X("t"), geom.Y("v"), geom.Bands(3))
	if got := drawHorizon(t, up, nil).Count("FillPath"); got != 3 {
		t.Errorf("a series in one arm took %d fills, want three", got)
	}

	// The same reach either side of the origin: three bands up, three down.
	both := geom.Horizon(horizonSource(-90, -40, 0, 40, 90), geom.X("t"), geom.Y("v"), geom.Bands(3))
	if got := drawHorizon(t, both, nil).Count("FillPath"); got != 6 {
		t.Errorf("a series in both arms took %d fills, want six", got)
	}

	// A series that stays inside the first band draws that band and nothing
	// else, however many were asked for.
	flat := geom.Horizon(horizonSource(0, 1, 2, 3), geom.X("t"), geom.Y("v"), geom.BandHeight(1000))
	if got := drawHorizon(t, flat, nil).Count("FillPath"); got != 1 {
		t.Errorf("a series inside one band took %d fills, want one", got)
	}
}

// The two arms are different colours, and a deeper band is a stronger one.
// That is the whole reading: the fold gives up position, and colour is what it
// gets back.
func TestTheArmsAndTheBandsArePaintedApart(t *testing.T) {
	g := geom.Horizon(horizonSource(-90, -40, 0, 40, 90), geom.X("t"), geom.Y("v"), geom.Bands(3))
	rec := drawHorizon(t, g, nil)

	seen := map[ir.Color]bool{}
	for _, c := range rec.Filter("FillPath") {
		if c.Fill.IsGradient() {
			t.Fatal("a band was filled with a gradient, want a solid colour")
		}
		if seen[c.Fill.Color] {
			t.Errorf("two bands are painted in the same colour %v", c.Fill.Color)
		}
		seen[c.Fill.Color] = true
	}
	if len(seen) != 6 {
		t.Errorf("six bands used %d colours", len(seen))
	}
}

// The guide is the ladder the chart folded away: a classed colourbar whose
// boundaries are the fold's own, in the data's units. Getting these wrong is
// the failure a reader cannot see, because the picture is identical.
func TestTheColourbarBreaksAreTheFoldsOwn(t *testing.T) {
	g := geom.Horizon(horizonSource(0, 30, 60, 90), geom.X("t"), geom.Y("v"), geom.Bands(3))
	if err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); err != nil {
		t.Fatal(err)
	}
	guided, ok := g.(geom.Guided)
	if !ok {
		t.Fatal("a horizon contributes no colour guide")
	}
	guide, ok := guided.ColorGuide()
	if !ok {
		t.Fatal("a horizon's colour guide reported itself absent")
	}
	if guide.Label != "v" {
		t.Errorf("the bar is titled %q, want the column's name", guide.Label)
	}
	classed, ok := scale.Classed(guide.Scale)
	if !ok {
		t.Fatal("a horizon's bar is not classed, so it has no boundaries to print")
	}
	// Three bands either side of an origin of zero, at a height of 30: the
	// interior boundaries run -60, -30, 0, 30, 60.
	want := []float64{-60, -30, 0, 30, 60}
	got := classed.Breaks()
	if len(got) != len(want) {
		t.Fatalf("the bar has breaks %v, want %v", got, want)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("the bar has breaks %v, want %v", got, want)
		}
	}
}

// A pinned band height is the comparable spelling, and it is the one that wins:
// how many bands today needs is then a reading rather than a setting.
func TestAPinnedHeightDecidesTheBandsFromTheData(t *testing.T) {
	// A reach of 100 in bands of 25 is four of them, whatever Bands said.
	g := geom.Horizon(horizonSource(0, 50, 100), geom.X("t"), geom.Y("v"),
		geom.Bands(2), geom.BandHeight(25))
	if got := drawHorizon(t, g, nil).Count("FillPath"); got != 4 {
		t.Errorf("a reach of 100 in bands of 25 took %d fills, want four", got)
	}
}

// The origin is the caller's. A chart of a deviation from a set point folds
// about the set point, and the two arms are then above and below it.
func TestTheFoldIsMeasuredFromTheBaseline(t *testing.T) {
	g := geom.Horizon(horizonSource(100, 110, 90), geom.X("t"), geom.Y("v"),
		geom.Baseline(100), geom.BandHeight(10))
	// Ten above and ten below: one band in each arm.
	if got := drawHorizon(t, g, nil).Count("FillPath"); got != 2 {
		t.Errorf("a deviation of ±10 in bands of 10 took %d fills, want two", got)
	}
}

// A grouped horizon is an error rather than a picture. Every band of every
// series is the whole panel, so the drawn answer is a solid rectangle carrying
// no reading at all.
func TestAGroupedHorizonIsAnError(t *testing.T) {
	src := data.Float64Columns(map[string][]float64{
		"t": {0, 1, 2, 3},
		"v": {1, 2, 3, 4},
		"g": {0, 0, 1, 1},
	})
	g := geom.Horizon(src, geom.X("t"), geom.Y("v"), geom.GroupBy("g"))
	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrGroupedHorizon) {
		t.Fatalf("a grouped horizon trained with %v, want ErrGroupedHorizon", err)
	}
}

// horizonRows collects what a layer reports about its marks.
type horizonRows struct {
	at   []ir.Point
	rows []int
}

func (s *horizonRows) Marks(m geom.MarkRows) {
	s.at = append(s.at, m.At...)
	s.rows = append(s.rows, m.Rows...)
}

// A row is reported once, at the band it ends in — not once per band it passes
// through. The fold cuts a value into several drawn spans, and a hit test that
// got three answers for one row could not name the row under the pointer.
func TestAHorizonReportsEachRowOnce(t *testing.T) {
	vs := []float64{0, 25, 55, 90}
	g := geom.Horizon(horizonSource(vs...), geom.X("t"), geom.Y("v"), geom.Bands(3))

	var sink horizonRows
	drawHorizon(t, g, func(f *geom.Frame) { f.Rows = &sink })

	if len(sink.rows) != len(vs) {
		t.Fatalf("%d rows were reported for %d rows of data", len(sink.rows), len(vs))
	}
	for i, r := range sink.rows {
		if r != i {
			t.Errorf("mark %d reports row %d", i, r)
		}
	}
	// The value at 90 ends exactly at the top of the third band, so it is
	// reported at the top of the panel; the one at 0 sits on the floor.
	if sink.at[3].Y >= sink.at[0].Y {
		t.Errorf("the largest value was reported at y=%v and the smallest at y=%v; "+
			"the fold puts the deepest reading highest in its own band",
			sink.at[3].Y, sink.at[0].Y)
	}
}

// A hole in the data is a hole in the chart. The fold must not turn a row
// nobody measured into a row that touched the origin.
func TestAHoleSurvivesTheFold(t *testing.T) {
	whole := geom.Horizon(horizonSource(10, 20, 30, 20, 10), geom.X("t"), geom.Y("v"), geom.Bands(1))
	holed := geom.Horizon(horizonSource(10, 20, math.NaN(), 20, 10), geom.X("t"), geom.Y("v"), geom.Bands(1))

	if a, b := drawHorizon(t, whole, nil).Count("FillPath"), drawHorizon(t, holed, nil).Count("FillPath"); b <= a {
		t.Errorf("a series with a hole in it drew %d filled runs and an unbroken one drew %d; "+
			"the hole should have split the band in two", b, a)
	}
}

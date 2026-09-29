package figure_test

import (
	"io"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

// windowed is a stream of ten rows, drawn by a Live, and the two frames
// ADR 0090's probe drew: 0..9 rising to 90, then 20..29 flat at 5.
func windowed(t *testing.T, x, y scale.Scale) (*data.Stream, *figure.Live) {
	t.Helper()
	st := data.NewStream("x", "y").Window(10)
	p := figure.New(figure.Size(400, 300))
	p.X(x).Y(y)
	p.Add(geom.Line(st.Source(), geom.X("x"), geom.Y("y")))
	for i := range 10 {
		st.Append(float64(i), float64(i*10))
	}
	st.Snapshot()
	live, err := p.Live(figure.SVGWriter(io.Discard))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { live.Close() })
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	return st, live
}

func advance(t *testing.T, st *data.Stream, live *figure.Live) {
	t.Helper()
	for i := 10; i < 30; i++ {
		st.Append(float64(i), 5)
	}
	st.Snapshot()
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
}

// The window moves on and the axes follow it: time slides, and the value
// axis describes the rows in the window rather than a peak it has dropped.
func TestAWindowedStreamsAxesDescribeTheWindow(t *testing.T) {
	x, y := scale.Linear(), scale.Linear()
	st, live := windowed(t, x, y)
	advance(t, st, live)
	if lo, hi := x.Domain(); lo != 20 || hi != 29 {
		t.Errorf("X is [%v, %v], want the window's [20, 29]", lo, hi)
	}
	// A flat window is padded to a domain a reader can see a line in; what
	// matters is that the peak of 90 it dropped is gone.
	if lo, hi := y.Domain(); lo > 5 || hi < 5 || hi-lo > 1 {
		t.Errorf("Y is [%v, %v], want the window's flat 5", lo, hi)
	}
}

// A zoom is a pinned domain, and a pinned domain is kept.
func TestAZoomSurvivesTheNextFrame(t *testing.T) {
	x, y := scale.Linear(), scale.Linear()
	st, live := windowed(t, x, y)
	x.(scale.Zoomer).SetDomain(22, 25)
	advance(t, st, live)
	if lo, hi := x.Domain(); lo != 22 || hi != 25 {
		t.Errorf("X is [%v, %v], want the zoom's [22, 25]", lo, hi)
	}
}

// HighWater keeps the old behaviour on purpose.
func TestAHighWaterAxisKeepsItsExtent(t *testing.T) {
	x, y := scale.Linear(), scale.Linear(scale.HighWater())
	st, live := windowed(t, x, y)
	advance(t, st, live)
	if lo, hi := y.Domain(); lo != 0 || hi != 90 {
		t.Errorf("a high-water Y is [%v, %v], want the most it has been, [0, 90]", lo, hi)
	}
	if lo, _ := x.Domain(); lo != 20 {
		t.Errorf("X starts at %v; only the high-water axis remembers", lo)
	}
}

// A discovered category keeps its slot, and a series keeps its colour, when
// the rows stop naming it.
func TestNamesAreRememberedAcrossFrames(t *testing.T) {
	src := func(names []string, vs []float64) *data.Table {
		return figure.NewTable().String("k", names).Float64("v", vs)
	}
	cat := scale.Ordinal()
	colors := scale.Qualitative(palette.OkabeIto)
	var layer *data.Table = src([]string{"a", "b", "c"}, []float64{1, 2, 3})
	build := func(tb *data.Table) *figure.Plot {
		p := figure.New(figure.Size(400, 300))
		p.X(cat).Y(scale.Linear())
		p.Add(geom.Bar(tb, geom.X("k"), geom.Y("v"), geom.ColorBy("k", colors)))
		return p
	}
	if err := build(layer).Render(figure.SVGWriter(io.Discard)); err != nil {
		t.Fatal(err)
	}
	before := colors.Color(1)
	slot := cat.Map(1)

	// The next frame names only "b" and "c", in the other order.
	if err := build(src([]string{"c", "b"}, []float64{3, 2})).Render(figure.SVGWriter(io.Discard)); err != nil {
		t.Fatal(err)
	}
	if got := colors.Color(1); got != before {
		t.Errorf("series b changed colour from %v to %v", before, got)
	}
	if got := cat.Map(1); got != slot {
		t.Errorf("category b moved from %v to %v", slot, got)
	}
}

// A continuous colour ramp follows the range rule: a heatmap of a stream does
// not keep its colour bar stretched to a peak the window dropped.
func TestAColourRampDescribesTheFrame(t *testing.T) {
	ramp := scale.Sequential(palette.Viridis)
	st := data.NewStream("x", "y", "v").Window(3)
	p := figure.New(figure.Size(400, 300))
	p.X(scale.Linear()).Y(scale.Linear())
	p.Add(geom.Scatter(st.Source(), geom.X("x"), geom.Y("y"), geom.ColorBy("v", ramp)))
	for i := range 3 {
		st.Append(float64(i), 0, 100)
	}
	st.Snapshot()
	live, err := p.Live(figure.SVGWriter(io.Discard))
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	live.Draw()
	for i := 3; i < 6; i++ {
		st.Append(float64(i), 0, float64(i))
	}
	st.Snapshot()
	live.Draw()
	if lo, hi := ramp.Domain(); lo != 3 || hi != 5 {
		t.Errorf("the ramp is [%v, %v], want the window's [3, 5]", lo, hi)
	}
}

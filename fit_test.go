package figure_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/scale"
)

// rampSource is y = x over 0..100, so the fitted extent of any window is the
// window.
func rampSource() *data.Table {
	xs, ys := make([]float64, 101), make([]float64, 101)
	for i := range xs {
		xs[i], ys[i] = float64(i), float64(i)
	}
	return figure.NewTable().Float64("x", xs).Float64("y", ys)
}

func drawn(t *testing.T, p *figure.Plot) *figure.Live {
	t.Helper()
	live, err := p.Live(irtest.New().Target())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { live.Close() })
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	return live
}

// A value axis that fits its view is trained on the rows its pinned X shows,
// and a line crossing the edge fits the value where it crosses.
func TestAValueAxisFitsThePinnedWindow(t *testing.T) {
	x, y := scale.Linear(), scale.Linear(scale.FitView())
	x.(scale.Zoomer).SetDomain(20.5, 40.5)
	p := figure.New(figure.Size(400, 300))
	p.X(x).Y(y)
	p.Add(geom.Line(rampSource(), geom.X("x"), geom.Y("y")))
	drawn(t, p)
	lo, hi := y.Domain()
	if math.Abs(lo-20.5) > 1e-9 || math.Abs(hi-40.5) > 1e-9 {
		t.Errorf("the fitted axis is [%v, %v], want the window [20.5, 40.5] — the line's value at each edge", lo, hi)
	}
}

// Without the option, or with X unpinned, nothing changes.
func TestAnAxisThatDoesNotFitSeesEveryRow(t *testing.T) {
	x, y := scale.Linear(), scale.Linear()
	x.(scale.Zoomer).SetDomain(20, 40)
	p := figure.New(figure.Size(400, 300))
	p.X(x).Y(y)
	p.Add(geom.Line(rampSource(), geom.X("x"), geom.Y("y")))
	drawn(t, p)
	if lo, hi := y.Domain(); lo != 0 || hi != 100 {
		t.Errorf("an ordinary axis is [%v, %v], want every row's [0, 100]", lo, hi)
	}

	y = scale.Linear(scale.FitView())
	p = figure.New(figure.Size(400, 300))
	p.X(scale.Linear()).Y(y)
	p.Add(geom.Line(rampSource(), geom.X("x"), geom.Y("y")))
	drawn(t, p)
	if lo, hi := y.Domain(); lo != 0 || hi != 100 {
		t.Errorf("a fitting axis over an unpinned X is [%v, %v], want every row", lo, hi)
	}
}

// A wheel over a fitting chart zooms X alone, and the next frame fits Y.
func TestAWheelZoomsTimeAndThePriceAxisFollows(t *testing.T) {
	x, y := scale.Linear(), scale.Linear(scale.FitView())
	p := figure.New(figure.Size(400, 300))
	p.X(x).Y(y)
	p.Add(geom.Line(rampSource(), geom.X("x"), geom.Y("y")))
	live := drawn(t, p)
	area := live.Index().Panels()[0].Area
	cx, cy := float64(area.Min.X+area.Max.X)/2, float64(area.Min.Y+area.Max.Y)/2
	if err := live.Wheel(cx, cy, 0.5); err != nil {
		t.Fatal(err)
	}
	xlo, xhi := x.Domain()
	if xhi-xlo > 60 {
		t.Fatalf("X is [%v, %v] after a zoom by half", xlo, xhi)
	}
	if scale.Pinned(y) {
		t.Error("the wheel pinned the fitting axis")
	}
	ylo, yhi := y.Domain()
	if math.Abs(ylo-xlo) > 1e-6 || math.Abs(yhi-xhi) > 1e-6 {
		t.Errorf("Y is [%v, %v], want it fitted to X's [%v, %v]", ylo, yhi, xlo, xhi)
	}

	// A pan slides X, and Y follows again; Autoscale returns to every row.
	live.Move(cx, cy)
	if err := live.PanBy(-40, 0); err != nil {
		t.Fatal(err)
	}
	xlo2, _ := x.Domain()
	if ylo2, _ := y.Domain(); math.Abs(ylo2-xlo2) > 1e-6 || xlo2 == xlo {
		t.Errorf("after a pan X starts at %v and Y at %v", xlo2, ylo2)
	}
	if err := live.Autoscale(); err != nil {
		t.Fatal(err)
	}
	if lo, hi := y.Domain(); lo != 0 || hi != 100 {
		t.Errorf("after Autoscale Y is [%v, %v], want every row", lo, hi)
	}
}

// A candle half in view still fits, and the candles wholly out of it do not.
func TestAFittedPriceAxisFitsTheVisibleCandles(t *testing.T) {
	tb := figure.NewTable().
		Float64("t", []float64{0, 1, 2, 3, 4}).
		Float64("o", []float64{10, 50, 51, 52, 90}).
		Float64("h", []float64{11, 55, 56, 57, 95}).
		Float64("l", []float64{9, 45, 46, 47, 85}).
		Float64("c", []float64{10, 51, 52, 53, 91})
	x, y := scale.Linear(), scale.Linear(scale.FitView())
	x.(scale.Zoomer).SetDomain(1.2, 3)
	p := figure.New(figure.Size(400, 300))
	p.X(x).Y(y)
	p.Add(geom.Candle(tb, geom.X("t"), geom.OHLC("o", "h", "l", "c")))
	drawn(t, p)
	// The candle at 1 reaches to 1.35 with a body of 0.7, so it is in view;
	// those at 0 and 4 are not.
	if lo, hi := y.Domain(); lo != 45 || hi != 57 {
		t.Errorf("the price axis is [%v, %v], want the three candles in view's [45, 57]", lo, hi)
	}
}

// A layer that cannot restrict its training fits loosely, never tightly: the
// axis still holds everything it drew.
func TestALayerThatCannotFitIsLooseNotClipped(t *testing.T) {
	x, y := scale.Linear(), scale.Linear(scale.FitView())
	x.(scale.Zoomer).SetDomain(20, 40)
	p := figure.New(figure.Size(400, 300))
	p.X(x).Y(y)
	p.Add(geom.Boxplot(rampSource(), geom.X("x"), geom.Y("y")))
	drawn(t, p)
	if lo, hi := y.Domain(); lo > 0 || hi < 100 {
		t.Errorf("a boxplot under a fitting axis trained to [%v, %v], want at least every row", lo, hi)
	}
}

package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// StockCharts' published RSI example: fourteen periods over these closes,
// whose first RSI, at the fifteenth close, is published as 70.53. The page
// computes from closes it prints rounded to the cent, so its numbers sit a few
// hundredths from what these rounded closes give; the first value is checked
// exactly against the arithmetic — nine gains summing to 3.34 and four losses
// to 1.40 over fourteen changes — and the published ones to that rounding.
func TestRSIMatchesWildersPublishedExample(t *testing.T) {
	closes := []float64{
		44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08,
		45.89, 46.03, 45.61, 46.28, 46.28, 46.00, 46.03, 46.41, 46.22, 45.64,
	}
	xs := make([]float64, len(closes))
	for i := range xs {
		xs[i] = float64(i)
	}
	got := stat.RSI(xs, closes, 14)
	want := map[int]float64{14: 70.53, 15: 66.32, 16: 66.55, 17: 69.41, 18: 66.36, 19: 57.97}
	for i, w := range want {
		if math.Abs(got[i].Y-w) > 0.1 {
			t.Errorf("RSI at row %d = %.2f, want %.2f", i, got[i].Y, w)
		}
	}
	if exact := 100 - 100/(1+3.34/1.40); math.Abs(got[14].Y-exact) > 1e-9 {
		t.Errorf("RSI at row 14 = %v, want the seed's %v exactly", got[14].Y, exact)
	}
	if !math.IsNaN(got[0].Y) {
		t.Errorf("row 0 has no change and is %v, want NaN", got[0].Y)
	}
}

// Composing an RSI from EMA gives a different curve, which is why RSI is a
// function: the test that says the function is not that composition.
func TestRSIIsNotAnEMAComposition(t *testing.T) {
	xs, ys := wobble(60)
	gains, losses := make([]float64, len(ys)), make([]float64, len(ys))
	for i := 1; i < len(ys); i++ {
		d := ys[i] - ys[i-1]
		gains[i], losses[i] = math.Max(d, 0), math.Max(-d, 0)
	}
	g, l := stat.EMA(xs, gains, 14), stat.EMA(xs, losses, 14)
	rsi := stat.RSI(xs, ys, 14)
	differs := false
	for i := 20; i < len(ys); i++ {
		naive := 100 - 100/(1+g[i].Y/l[i].Y)
		if math.Abs(naive-rsi[i].Y) > 0.5 {
			differs = true
		}
	}
	if !differs {
		t.Error("an EMA-built RSI matches Wilder's, so the function would be a composition")
	}
}

func TestRSIOfAFlatOrOneWaySeries(t *testing.T) {
	xs := []float64{0, 1, 2, 3}
	if got := stat.RSI(xs, []float64{5, 5, 5, 5}, 2); got[3].Y != 50 {
		t.Errorf("a flat series is %v, want 50", got[3].Y)
	}
	if got := stat.RSI(xs, []float64{1, 2, 3, 4}, 2); got[3].Y != 100 {
		t.Errorf("a rising series is %v, want 100", got[3].Y)
	}
	if got := stat.RSI(xs, []float64{4, 3, 2, 1}, 2); got[3].Y != 0 {
		t.Errorf("a falling series is %v, want 0", got[3].Y)
	}
}

// The sums start again at every session.
func TestVWAPStartsAgainEachSession(t *testing.T) {
	ts := []float64{0, 1, 2, 10, 11}
	ps := []float64{10, 20, 30, 100, 200}
	vs := []float64{1, 1, 2, 1, 3}
	got := stat.VWAP(ts, ps, vs, []float64{0, 10})
	want := []float64{10, 15, (10 + 20 + 60) / 4.0, 100, (100 + 600) / 4.0}
	for i, w := range want {
		if math.Abs(got[i].Y-w) > 1e-12 {
			t.Errorf("row %d: %v, want %v", i, got[i].Y, w)
		}
	}
	// Without edges it is one session.
	if all := stat.VWAP(ts, ps, vs, nil); math.Abs(all[4].Y-(10+20+60+100+600)/8.0) > 1e-12 {
		t.Errorf("one session: %v", all[4].Y)
	}
	// Before the first edge there is no session.
	if early := stat.VWAP(ts, ps, vs, []float64{1}); !math.IsNaN(early[0].Y) {
		t.Errorf("a row before the first session is %v, want NaN", early[0].Y)
	}
}

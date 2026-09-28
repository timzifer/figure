package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

func TestOHLCSummarisesEachInterval(t *testing.T) {
	// Two intervals of width 10 from an origin of 5, and a gap between them.
	ts := []float64{5, 7, 9, 14.9, 35, 36}
	ps := []float64{10, 12, 8, 11, 20, 19}
	vs := []float64{1, 2, 3, 4, 5, 6}
	got := stat.OHLC(ts, ps, vs, 5, 10)
	want := []stat.Candle{
		{Start: 5, Open: 10, High: 12, Low: 8, Close: 11, Volume: 10, Count: 4},
		{Start: 35, Open: 20, High: 20, Low: 19, Close: 19, Volume: 11, Count: 2},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d candles %v, want %d — the empty interval between gets none", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candle %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if !got[0].Up() || got[1].Up() {
		t.Errorf("Up: %v, %v — want the first up and the second down", got[0].Up(), got[1].Up())
	}
}

// Without volumes the candle's Volume is its row count, and a row before the
// origin still lands in the interval that contains it.
func TestOHLCCountsWithoutVolumesAndFloorsBeforeTheOrigin(t *testing.T) {
	got := stat.OHLC([]float64{-3, -1, 2}, []float64{1, 2, 3}, nil, 0, 5)
	if len(got) != 2 || got[0].Start != -5 || got[0].Volume != 2 || got[1].Start != 0 || got[1].Volume != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestOHLCSkipsWhatIsNotANumber(t *testing.T) {
	nan := math.NaN()
	got := stat.OHLC([]float64{0, 1, nan, 2}, []float64{1, nan, 5, 3}, nil, 0, 10)
	if len(got) != 1 || got[0].Count != 2 || got[0].High != 3 {
		t.Errorf("got %+v, want one candle of the two finite rows", got)
	}
	if got := stat.OHLC([]float64{0}, []float64{1}, nil, 0, 0); len(got) != 0 {
		t.Errorf("a zero width gave %v", got)
	}
}

// Two calls over the same interval give buckets whose edges line up, which is
// what lets the buys and the sells be drawn as one profile.
func TestTwoWeightedBinsOverOneIntervalLineUp(t *testing.T) {
	prices := []float64{10, 11, 12, 13, 14, 15}
	buys := []float64{1, 0, 2, 0, 3, 0}
	sells := []float64{0, 4, 0, 5, 0, 6}
	b := stat.BinWeighted(prices, buys, 10, 16, 3)
	s := stat.BinWeighted(prices, sells, 10, 16, 3)
	if len(b) != 3 || len(s) != 3 {
		t.Fatalf("got %d and %d buckets", len(b), len(s))
	}
	wantB, wantS := []float64{1, 2, 3}, []float64{4, 5, 6}
	for i := range b {
		if b[i].Lo != s[i].Lo || b[i].Hi != s[i].Hi {
			t.Errorf("bucket %d: %v and %v do not line up", i, b[i], s[i])
		}
		if b[i].Sum != wantB[i] || s[i].Sum != wantS[i] {
			t.Errorf("bucket %d: sums %v and %v, want %v and %v", i, b[i].Sum, s[i].Sum, wantB[i], wantS[i])
		}
	}

	// And the edges are Bin's own.
	for i, c := range stat.Bin(prices, 10, 16, 3) {
		if c.Lo != b[i].Lo || c.Hi != b[i].Hi {
			t.Errorf("bucket %d: weighted %v, counted %v", i, b[i], c)
		}
	}
}

// Periods of different lengths, as a calendar's opens give them: a row falls
// in the period its edge starts, and one before the first edge in none.
func TestOHLCAtBucketsBetweenEdges(t *testing.T) {
	edges := []float64{10, 13, 30}
	ts := []float64{5, 10, 12, 13, 29, 31, 100}
	ps := []float64{9, 1, 3, 4, 2, 7, 8}
	got := stat.OHLCAt(ts, ps, nil, edges)
	want := []stat.Candle{
		{Start: 10, Open: 1, High: 3, Low: 1, Close: 3, Volume: 2, Count: 2},
		{Start: 13, Open: 4, High: 4, Low: 2, Close: 2, Volume: 2, Count: 2},
		{Start: 30, Open: 7, High: 8, Low: 7, Close: 8, Volume: 2, Count: 2},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candle %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := stat.OHLCAt(ts, ps, nil, nil); len(got) != 0 {
		t.Errorf("no edges gave %v", got)
	}
}

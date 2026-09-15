package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

func TestMedianRankIsBenardsApproximationPerRow(t *testing.T) {
	got := stat.MedianRank([]float64{math.NaN(), 10, 20, 20, math.Inf(1)})
	if len(got) != 3 {
		t.Fatalf("%d points, want one per usable row, ties included", len(got))
	}
	for i, p := range got {
		want := (float64(i+1) - 0.3) / 3.4
		if math.Abs(p.Y-want) > 1e-12 {
			t.Errorf("rank %d = %v, want %v", i+1, p.Y, want)
		}
		if !(p.Y > 0 && p.Y < 1) {
			t.Errorf("rank %d = %v is not strictly inside (0, 1)", i+1, p.Y)
		}
	}
	if got[1].X != 20 || got[2].X != 20 {
		t.Errorf("ties lost their rows: %v", got)
	}
	if n := len(stat.MedianRank(nil)); n != 0 {
		t.Errorf("an empty column gave %d points", n)
	}
}

package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

func rocNear(a, b float64) bool { return math.Abs(a-b) < 1e-12 }

func TestROCOfAPerfectAnInvertedAndATiedClassifier(t *testing.T) {
	scores := []float64{0.1, 0.2, 0.3, 0.4}
	cases := []struct {
		name  string
		score []float64
		pos   []bool
		want  float64
	}{
		{"perfect", scores, []bool{false, false, true, true}, 1},
		{"inverted", scores, []bool{true, true, false, false}, 0},
		{"tied", []float64{0.5, 0.5, 0.5, 0.5}, []bool{true, false, true, false}, 0.5},
	}
	for _, c := range cases {
		curve, auc := stat.ROC(c.score, c.pos)
		if !rocNear(auc, c.want) {
			t.Errorf("%s: AUC = %v, want %v", c.name, auc, c.want)
		}
		if first, last := curve[0], curve[len(curve)-1]; first != (stat.Point{}) || last != (stat.Point{X: 1, Y: 1}) {
			t.Errorf("%s: curve runs %v to %v, want (0,0) to (1,1)", c.name, first, last)
		}
	}
	if curve, _ := stat.ROC([]float64{0.5, 0.5, 0.5, 0.5}, []bool{true, false, true, false}); len(curve) != 2 {
		t.Errorf("an all-tied sample drew %d points, want one diagonal step", len(curve))
	}
}

// The area is the Mann–Whitney probability that a positive outscores a
// negative, a tie counting half. Positives score 0.3, 0.6 and 0.8; negatives
// 0.1, 0.6 and 0.7. Of the nine pairs, positive wins are 0.3>0.1, 0.6>0.1,
// 0.8>0.1, 0.8>0.6, 0.8>0.7 — five — and 0.6 against 0.6 is a tie: 5.5/9.
func TestROCAreaIsTheMannWhitneyCountWithTiesAsHalf(t *testing.T) {
	scores := []float64{0.1, 0.3, 0.6, 0.6, 0.7, 0.8}
	pos := []bool{false, true, true, false, false, true}
	curve, auc := stat.ROC(scores, pos)
	if !rocNear(auc, 5.5/9) {
		t.Errorf("AUC = %v, want %v", auc, 5.5/9)
	}
	want := []stat.Point{
		{X: 0, Y: 0},
		{X: 0, Y: 1.0 / 3},       // 0.8
		{X: 1.0 / 3, Y: 1.0 / 3}, // 0.7
		{X: 2.0 / 3, Y: 2.0 / 3}, // the tie at 0.6, diagonally
		{X: 2.0 / 3, Y: 1},       // 0.3
		{X: 1, Y: 1},             // 0.1
	}
	if len(curve) != len(want) {
		t.Fatalf("curve = %v, want %v", curve, want)
	}
	for i := range want {
		if !rocNear(curve[i].X, want[i].X) || !rocNear(curve[i].Y, want[i].Y) {
			t.Errorf("point %d = %v, want %v", i, curve[i], want[i])
		}
	}
}

func TestPrecisionRecallStepsAndAveragePrecision(t *testing.T) {
	// From the top: 0.9 positive, 0.8 negative, 0.7 positive, 0.1 negative.
	scores := []float64{0.1, 0.7, 0.8, 0.9}
	pos := []bool{false, true, false, true}
	curve, ap := stat.PrecisionRecall(scores, pos)
	want := []stat.Point{
		{X: 0, Y: 1},
		{X: 0.5, Y: 1},
		{X: 0.5, Y: 0.5},
		{X: 1, Y: 2.0 / 3},
		{X: 1, Y: 0.5},
	}
	if len(curve) != len(want) {
		t.Fatalf("curve = %v, want %v", curve, want)
	}
	for i := range want {
		if !rocNear(curve[i].X, want[i].X) || !rocNear(curve[i].Y, want[i].Y) {
			t.Errorf("point %d = %v, want %v", i, curve[i], want[i])
		}
	}
	// 0.5·1 + 0.5·(2/3): the recall gained at each positive, at the precision
	// reached there.
	if w := 0.5 + 0.5*2.0/3; !rocNear(ap, w) {
		t.Errorf("AP = %v, want %v", ap, w)
	}
}

func TestLorenzOfEqualityOfOneHolderAndAHandCase(t *testing.T) {
	if _, g := stat.Lorenz([]float64{3, 3, 3, 3}); !rocNear(g, 0) {
		t.Errorf("equal holdings: Gini = %v, want 0", g)
	}
	const n = 5
	if _, g := stat.Lorenz([]float64{0, 0, 0, 0, 10}); !rocNear(g, float64(n-1)/n) {
		t.Errorf("one holder of five: Gini = %v, want %v", g, float64(n-1)/n)
	}
	// 1, 2, 3, 4: shares 0.1, 0.3, 0.6, 1 at quarters. The area is
	// (0+0.1 + 0.1+0.3 + 0.3+0.6 + 0.6+1)/2/4 = 0.375, so the Gini is 0.25.
	curve, g := stat.Lorenz([]float64{1, 2, 3, 4})
	if !rocNear(g, 0.25) {
		t.Errorf("1,2,3,4: Gini = %v, want 0.25", g)
	}
	if len(curve) != 5 || curve[0] != (stat.Point{}) || !rocNear(curve[2].Y, 0.3) || curve[4] != (stat.Point{X: 1, Y: 1}) {
		t.Errorf("curve = %v", curve)
	}
}

func TestTheScoredReductionsIgnoreWhatHasNoValue(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	_, clean := stat.ROC([]float64{0.1, 0.3, 0.6, 0.6, 0.7, 0.8}, []bool{false, true, true, false, false, true})
	// The same sample with a NaN in the middle of the tie and an infinity at
	// the top, each carrying a label that must not count.
	_, dirty := stat.ROC([]float64{0.1, 0.3, 0.6, nan, 0.6, 0.7, 0.8, inf},
		[]bool{false, true, true, true, false, false, true, false})
	if !rocNear(clean, dirty) {
		t.Errorf("AUC with non-finite rows = %v, want %v", dirty, clean)
	}
	if _, g := stat.Lorenz([]float64{nan, 1, 2, 3, 4, inf}); !rocNear(g, 0.25) {
		t.Errorf("Lorenz with non-finite rows: Gini = %v, want 0.25", g)
	}

	if c, auc := stat.ROC([]float64{1, 2}, []bool{true, true}); len(c) != 0 || !math.IsNaN(auc) {
		t.Errorf("no negatives: %v, %v; want an empty curve and NaN", c, auc)
	}
	if c, ap := stat.PrecisionRecall([]float64{1, 2}, []bool{false, false}); len(c) != 0 || !math.IsNaN(ap) {
		t.Errorf("no positives: %v, %v; want an empty curve and NaN", c, ap)
	}
	if c, g := stat.Lorenz([]float64{-1, 0, 1}); len(c) != 0 || !math.IsNaN(g) {
		t.Errorf("a zero total: %v, %v; want an empty curve and NaN", c, g)
	}
}

func TestTheScoredReductionsReuseTheirBuffers(t *testing.T) {
	scores := []float64{0.1, 0.3, 0.6, 0.6, 0.7, 0.8}
	pos := []bool{false, true, true, false, false, true}
	fresh, auc := stat.ROC(scores, pos)

	buf, _ := stat.AppendROC(nil, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}, []bool{true, false, true, false, true, false, true, false, true})
	again, auc2 := stat.AppendROC(buf, scores, pos)
	if auc != auc2 || len(again) != len(fresh) {
		t.Fatalf("reused ROC = %v (%v), fresh %v (%v)", again, auc2, fresh, auc)
	}
	for i := range fresh {
		if again[i] != fresh[i] {
			t.Errorf("point %d: %v reused, %v fresh", i, again[i], fresh[i])
		}
	}

	pr, ap := stat.PrecisionRecall(scores, pos)
	pr2, ap2 := stat.AppendPrecisionRecall(again, scores, pos)
	if ap != ap2 || len(pr) != len(pr2) {
		t.Errorf("reused PR differs")
	}

	lz, g := stat.Lorenz([]float64{1, 2, 3, 4})
	lz2, g2 := stat.AppendLorenz(pr2, []float64{1, 2, 3, 4})
	if g != g2 || len(lz) != len(lz2) {
		t.Errorf("reused Lorenz differs")
	}
}

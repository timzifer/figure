package stat_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/timzifer/figure/stat"
)

func spcNear(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-3 {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func TestLimitsIMRFromTheMovingRange(t *testing.T) {
	// Mean 11.5; moving ranges 2, 1, 2 average 5/3. The NaN is a gap, not a
	// jump: it adds no reading and no range.
	ind, mr := stat.LimitsIMR([]float64{10, 12, 11, math.NaN(), 13})
	// Ranges are 2 and 1 only (13 has no finite neighbour before it).
	spcNear(t, "centre", ind.Centre, 11.5)
	spcNear(t, "upper", ind.Upper, 11.5+2.66*1.5)
	spcNear(t, "lower", ind.Lower, 11.5-2.66*1.5)
	spcNear(t, "MR centre", mr.Centre, 1.5)
	spcNear(t, "MR upper", mr.Upper, 3.267*1.5)
	if mr.Lower != 0 {
		t.Errorf("MR lower = %v, want 0", mr.Lower)
	}
	if a, b := stat.LimitsIMR(nil); a != (stat.Limits{}) || b != (stat.Limits{}) {
		t.Error("empty input should give zero Limits")
	}
	spcNear(t, "sigma", ind.Sigma(), 2.66*1.5/3)
}

func TestLimitsXbarRIgnoresAPartialSubgroup(t *testing.T) {
	// Subgroups (1,3) (2,6) (4,4), and a trailing 9 that is ignored: means 2, 4,
	// 4 give 10/3; ranges 2, 4, 0 give 2.
	mean, rng := stat.LimitsXbarR([]float64{1, 3, 2, 6, 4, 4, 9}, 2)
	spcNear(t, "grand mean", mean.Centre, 10.0/3)
	spcNear(t, "X̄ upper", mean.Upper, 10.0/3+1.880*2)
	spcNear(t, "X̄ lower", mean.Lower, 10.0/3-1.880*2)
	spcNear(t, "R centre", rng.Centre, 2)
	spcNear(t, "R upper", rng.Upper, 3.267*2)
	spcNear(t, "R lower", rng.Lower, 0)

	// Size 7 has a non-zero D3.
	_, r7 := stat.LimitsXbarR([]float64{0, 1, 2, 3, 4, 5, 6}, 7)
	spcNear(t, "R lower at n=7", r7.Lower, 0.076*6)

	for _, size := range []int{1, 26} {
		if m, r := stat.LimitsXbarR([]float64{1, 2, 3}, size); m != (stat.Limits{}) || r != (stat.Limits{}) {
			t.Errorf("size %d outside the table should give zero Limits", size)
		}
	}
}

func TestLimitsNPAndCClampAtZero(t *testing.T) {
	np := stat.LimitsNP([]float64{2, 4, 3, 3}, 50)
	spcNear(t, "np centre", np.Centre, 3)
	spcNear(t, "np upper", np.Upper, 3+3*math.Sqrt(50*0.06*0.94))
	if np.Lower != 0 {
		t.Errorf("np lower = %v, want clamped to 0", np.Lower)
	}

	c := stat.LimitsC([]float64{4, 9, 5, math.Inf(1)})
	spcNear(t, "c centre", c.Centre, 6)
	spcNear(t, "c upper", c.Upper, 6+3*math.Sqrt(6))
	if c.Lower != 0 {
		t.Errorf("c lower = %v, want clamped to 0", c.Lower)
	}
}

func TestLimitsPAndUAreAStaircase(t *testing.T) {
	centre, lo, hi := stat.AppendLimitsP(nil, nil, []float64{5, 10, 1}, []float64{50, 200, 0})
	spcNear(t, "p̄", centre, 0.06)
	spcNear(t, "p hi[0]", hi[0], 0.06+3*math.Sqrt(0.06*0.94/50))
	spcNear(t, "p lo[0]", lo[0], 0)
	spcNear(t, "p hi[1]", hi[1], 0.06+3*math.Sqrt(0.06*0.94/200))
	spcNear(t, "p lo[1]", lo[1], 0.06-3*math.Sqrt(0.06*0.94/200))
	if !math.IsNaN(lo[2]) || !math.IsNaN(hi[2]) {
		t.Errorf("a subgroup of size 0 should have NaN limits, got %v %v", lo[2], hi[2])
	}

	centre, lo, hi = stat.AppendLimitsU(lo, hi, []float64{6, 12}, []float64{2, 4})
	if len(lo) != 2 {
		t.Fatalf("lo was not truncated: %v", lo)
	}
	spcNear(t, "ū", centre, 3)
	spcNear(t, "u hi[0]", hi[0], 3+3*math.Sqrt(1.5))
	spcNear(t, "u lo[0]", lo[0], 0)
	spcNear(t, "u lo[1]", lo[1], 3-3*math.Sqrt(0.75))
}

// spcUnit is the limits every run-rule test judges against: centre 0, σ 1.
var spcUnit = stat.Limits{Centre: 0, Lower: -3, Upper: 3}

func spcRepeat(v float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func spcAlternating(v float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
		if i%2 == 1 {
			out[i] = -v
		}
	}
	return out
}

func spcRows(fs []stat.Flag) []int {
	out := []int{}
	for _, f := range fs {
		out = append(out, f.Row)
	}
	return out
}

func TestEachRunRuleFiresAndANearMissDoesNot(t *testing.T) {
	cases := []struct {
		name     string
		rule     stat.RunRule
		hit      []float64
		want     []int
		nearMiss []float64
	}{
		{"beyond", stat.RuleBeyondLimits, []float64{0, 3.5, -3.1}, []int{1, 2}, []float64{3, -3, 0}},
		{"nine", stat.RuleNineOneSide, spcRepeat(0.5, 10), []int{8, 9},
			append(spcRepeat(0.5, 4), append([]float64{0}, spcRepeat(0.5, 4)...)...)},
		{"six trending", stat.RuleSixTrending, []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5}, []int{5},
			[]float64{0, 0.1, 0.2, 0.2, 0.4, 0.5}},
		{"fourteen alternating", stat.RuleFourteenAlternating, spcAlternating(0.2, 14), []int{13}, spcAlternating(0.2, 13)},
		{"two of three", stat.RuleTwoOfThreeBeyond2Sigma, []float64{0, 2.5, 0, 2.5}, []int{3},
			[]float64{2.5, 0, -2.5}},
		{"four of five", stat.RuleFourOfFiveBeyond1Sigma, []float64{1.5, 1.5, 0, 1.5, 1.5}, []int{4},
			[]float64{1.5, 1.5, 0, 0, 1.5}},
		{"fifteen within", stat.RuleFifteenWithin1Sigma, spcRepeat(0.5, 15), []int{14}, spcRepeat(0.5, 14)},
		{"eight outside", stat.RuleEightOutside1Sigma, spcAlternating(1.5, 8), []int{7}, spcAlternating(1.5, 7)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stat.AppendRunRules(nil, tc.hit, spcUnit, tc.rule)
			if !reflect.DeepEqual(spcRows(got), tc.want) {
				t.Errorf("flagged rows %v, want %v", spcRows(got), tc.want)
			}
			for _, f := range got {
				if f.Rule != tc.rule {
					t.Errorf("flag carries rule %d, want %d", f.Rule, tc.rule)
				}
			}
			if miss := stat.AppendRunRules(nil, tc.nearMiss, spcUnit, tc.rule); len(miss) != 0 {
				t.Errorf("near miss flagged %v", miss)
			}
		})
	}
}

func TestTheCompletingPointMustBeOneOfThoseCounted(t *testing.T) {
	// Two of three beyond 2σ, but the next point is back near the centre: the
	// pattern was completed at row 2, and row 3 — still two of its three — is
	// not re-flagged, because it is not one of the two.
	got := stat.AppendRunRules(nil, []float64{0, 2.5, 2.5, 0}, spcUnit, stat.RuleTwoOfThreeBeyond2Sigma)
	if !reflect.DeepEqual(spcRows(got), []int{2}) {
		t.Errorf("flagged rows %v, want [2]", spcRows(got))
	}
}

func TestANonFiniteValueBreaksARun(t *testing.T) {
	x := append(spcRepeat(0.5, 5), append([]float64{math.NaN()}, spcRepeat(0.5, 5)...)...)
	if got := stat.AppendRunRules(nil, x, spcUnit, stat.RuleNineOneSide); len(got) != 0 {
		t.Errorf("a run across a NaN was flagged: %v", got)
	}
}

func TestNoRulesMeansAllAndFlagsAreOrderedByRowThenRule(t *testing.T) {
	// Eight points on one side, then a ninth beyond the limit: row 8 completes
	// both rule 1 and rule 2.
	x := append(spcRepeat(0.5, 8), 3.5)
	all := stat.AppendRunRules(nil, x, spcUnit)
	explicit := stat.AppendRunRules(nil, x, spcUnit,
		stat.RuleEightOutside1Sigma, stat.RuleFifteenWithin1Sigma, stat.RuleFourOfFiveBeyond1Sigma,
		stat.RuleTwoOfThreeBeyond2Sigma, stat.RuleFourteenAlternating, stat.RuleSixTrending,
		stat.RuleNineOneSide, stat.RuleBeyondLimits, stat.RuleBeyondLimits)
	if !reflect.DeepEqual(all, explicit) {
		t.Errorf("no rules = %v, all eight = %v", all, explicit)
	}
	want := []stat.Flag{{Row: 8, Rule: stat.RuleBeyondLimits}, {Row: 8, Rule: stat.RuleNineOneSide}}
	if !reflect.DeepEqual(all, want) {
		t.Errorf("flags = %v, want %v", all, want)
	}
}

func TestRunRulesAreDeterministicAndReuseTheirBuffer(t *testing.T) {
	x := []float64{0.2, 1.5, 1.6, -0.3, 2.4, 2.7, 3.4, 0.1, -1.2, -1.4, -1.9, -2.2, 0.4}
	first := stat.AppendRunRules(nil, x, spcUnit)
	dst := make([]stat.Flag, 0, 64)
	dst = append(dst, stat.Flag{Row: 99})
	second := stat.AppendRunRules(dst, x, spcUnit)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two runs differ: %v vs %v", first, second)
	}
}

func TestRulesInSigmaNeedASigma(t *testing.T) {
	flat := stat.Limits{}
	got := stat.AppendRunRules(nil, spcRepeat(2.5, 20), flat,
		stat.RuleTwoOfThreeBeyond2Sigma, stat.RuleFourOfFiveBeyond1Sigma,
		stat.RuleFifteenWithin1Sigma, stat.RuleEightOutside1Sigma)
	if len(got) != 0 {
		t.Errorf("rules in σ flagged %v with no σ", got)
	}
}

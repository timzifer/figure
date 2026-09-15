package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// leukemia is the 6-MP arm of Freireich et al. (1963), the data every survival
// textbook works its first Kaplan–Meier table from: weeks in remission, and
// whether the remission ended (true) or the patient was still in it when last
// seen (false).
func leukemia() ([]float64, []bool) {
	t := []float64{6, 6, 6, 6, 7, 9, 10, 10, 11, 13, 16, 17, 19, 20, 22, 23, 25, 32, 32, 34, 35}
	e := []bool{true, true, true, false, true, false, true, false, false, true, true, false, false, false, true, true, false, false, false, false, false}
	return t, e
}

func TestKaplanMeierMatchesTheTextbookTable(t *testing.T) {
	times, events := leukemia()
	km := stat.KaplanMeier(times, events)

	// The event times of the published table, with their risk sets and
	// estimates to three places.
	want := map[float64]struct {
		n, d int
		s    float64
	}{
		6: {21, 3, 0.857}, 7: {17, 1, 0.807}, 10: {15, 1, 0.753}, 13: {12, 1, 0.690},
		16: {11, 1, 0.627}, 22: {7, 1, 0.538}, 23: {6, 1, 0.448},
	}
	seen := 0
	for _, p := range km {
		w, ok := want[p.T]
		if !ok {
			if p.Events != 0 {
				t.Errorf("t=%v has %d events and is not an event time of the table", p.T, p.Events)
			}
			continue
		}
		seen++
		if p.AtRisk != w.n || p.Events != w.d || math.Abs(p.S-w.s) > 5e-4 {
			t.Errorf("t=%v: n=%d d=%d S=%.4f, want n=%d d=%d S=%.3f", p.T, p.AtRisk, p.Events, p.S, w.n, w.d, w.s)
		}
	}
	if seen != len(want) {
		t.Errorf("found %d of the %d event times", seen, len(want))
	}
	// Distinct times: 6 7 9 10 11 13 16 17 19 20 22 23 25 32 34 35.
	if len(km) != 16 {
		t.Errorf("%d points, want one per distinct time, censor-only times included", len(km))
	}

	// Greenwood's standard error at week 6 is S·√(3/(21·18)) = 0.0764.
	if se := km[0].S * math.Sqrt(km[0].Greenwood); math.Abs(se-0.0764) > 5e-4 {
		t.Errorf("Greenwood SE at week 6 = %.4f, want 0.0764", se)
	}
}

func TestACensoredSubjectIsAtRiskAtItsOwnTime(t *testing.T) {
	// Two subjects at t=5, one event and one censored: both were observed up
	// to 5, so both are in the risk set there, and the one censored at t=1 is
	// not.
	km := stat.KaplanMeier([]float64{1, 5, 5}, []bool{false, true, false})
	if km[1].AtRisk != 2 || km[1].Events != 1 || km[1].Censored != 1 {
		t.Errorf("t=5: %+v", km[1])
	}
	if km[0].S != 1 || km[1].S != 0.5 {
		t.Errorf("S = %v then %v, want 1 then 0.5", km[0].S, km[1].S)
	}
}

func TestTheLogLogBandStaysInsideTheUnitInterval(t *testing.T) {
	times, events := leukemia()
	for _, p := range stat.KaplanMeier(times, events) {
		lo, hi := p.Band(1.96)
		if !(0 <= lo && lo <= p.S && p.S <= hi && hi <= 1) {
			t.Errorf("t=%v: band [%v, %v] around S=%v", p.T, lo, hi, p.S)
		}
	}
	// Everybody dies: S reaches 0 and the band closes onto it.
	km := stat.KaplanMeier([]float64{1, 2}, []bool{true, true})
	if lo, hi := km[1].Band(1.96); km[1].S != 0 || lo != 0 || hi != 0 {
		t.Errorf("S=%v band [%v, %v], want 0 and [0, 0]", km[1].S, lo, hi)
	}
}

func TestKaplanMeierIgnoresUnusableTimesAndReusesItsBuffer(t *testing.T) {
	a := stat.KaplanMeier([]float64{math.NaN(), 1, 2, math.Inf(1)}, []bool{true, true, false, true})
	if len(a) != 2 || a[0].AtRisk != 2 {
		t.Errorf("with unusable times: %+v", a)
	}
	times, events := leukemia()
	buf := stat.AppendKaplanMeier(nil, []float64{1, 2, 3}, []bool{true, true, true})
	buf = stat.AppendKaplanMeier(buf, times, events)
	fresh := stat.KaplanMeier(times, events)
	for i := range fresh {
		if buf[i] != fresh[i] {
			t.Fatalf("point %d differs after reuse: %+v vs %+v", i, buf[i], fresh[i])
		}
	}
}

package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

func TestACFOfAShortRampByHand(t *testing.T) {
	// Mean 3, deviations −2 −1 0 1 2: c0 = 10, c1 = 4, c2 = −1, all over n.
	x := []float64{1, 2, 3, 4, 5}
	r := stat.ACF(x, 2)
	for k, want := range []float64{1, 0.4, -0.1} {
		if math.Abs(r[k]-want) > 1e-12 {
			t.Errorf("r_%d = %v, want %v", k, r[k], want)
		}
	}
	// φ22 = (r2 − r1²) / (1 − r1²).
	p := stat.PACF(x, 2)
	for k, want := range []float64{1, 0.4, (-0.1 - 0.16) / 0.84} {
		if math.Abs(p[k]-want) > 1e-12 {
			t.Errorf("pacf_%d = %v, want %v", k, p[k], want)
		}
	}
}

// ar1 is x_t = φ·x_{t−1} + e_t with uniform noise from a fixed xorshift, so the
// test sees the same series every run.
func ar1(n int, phi float64) []float64 {
	state := uint64(0x853c49e6748fea9b)
	x := make([]float64, n)
	prev := 0.0
	for i := range x {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		e := float64(state>>11)/float64(1<<53) - 0.5
		prev = phi*prev + e
		x[i] = prev
	}
	return x
}

func TestAnAR1DecaysInItsACFAndCutsOffInItsPACF(t *testing.T) {
	const phi = 0.7
	x := ar1(5000, phi)
	r := stat.ACF(x, 6)
	for k := 1; k <= 6; k++ {
		if want := math.Pow(phi, float64(k)); math.Abs(r[k]-want) > 0.06 {
			t.Errorf("r_%d = %.3f, want about φ^%d = %.3f", k, r[k], k, want)
		}
	}
	p := stat.PACF(x, 6)
	if math.Abs(p[1]-phi) > 0.06 {
		t.Errorf("pacf_1 = %.3f, want about φ = %.1f", p[1], phi)
	}
	bound := stat.CorrelationBound(len(x), 1.96)
	for k := 2; k <= 6; k++ {
		if math.Abs(p[k]) > 2*bound {
			t.Errorf("pacf_%d = %.3f, want inside the noise band ±%.3f — an AR(1) cuts off after lag 1", k, p[k], 2*bound)
		}
	}
}

func TestANonFiniteReadingPoisonsEveryLag(t *testing.T) {
	x := []float64{1, 2, math.NaN(), 4, 5}
	for name, got := range map[string][]float64{"ACF": stat.ACF(x, 3), "PACF": stat.PACF(x, 3)} {
		for k, v := range got {
			if !math.IsNaN(v) {
				t.Errorf("%s lag %d = %v, want NaN", name, k, v)
			}
		}
	}
}

func TestAConstantSeriesHasNoCorrelationBeyondLagZero(t *testing.T) {
	x := []float64{3, 3, 3, 3}
	for name, got := range map[string][]float64{"ACF": stat.ACF(x, 2), "PACF": stat.PACF(x, 2)} {
		if got[0] != 1 {
			t.Errorf("%s lag 0 = %v, want 1", name, got[0])
		}
		for k := 1; k < len(got); k++ {
			if !math.IsNaN(got[k]) {
				t.Errorf("%s lag %d = %v, want NaN", name, k, got[k])
			}
		}
	}
}

func TestTheLagIsClampedToTheSeries(t *testing.T) {
	x := []float64{1, 3, 2}
	if n := len(stat.ACF(x, 10)); n != 3 {
		t.Errorf("ACF to lag 10 of 3 readings has %d lags, want 3", n)
	}
	if n := len(stat.PACF(x, -4)); n != 1 {
		t.Errorf("PACF to lag −4 has %d lags, want lag 0 alone", n)
	}
	if n := len(stat.ACF(nil, 5)); n != 0 {
		t.Errorf("an empty series has %d lags", n)
	}
}

func TestCorrelationsReuseTheirBufferAndAgree(t *testing.T) {
	x := ar1(300, -0.4)
	wantA, wantP := stat.ACF(x, 20), stat.PACF(x, 20)

	var a, p []float64
	a = stat.AppendACF(a, ar1(50, 0.9), 40)
	p = stat.AppendPACF(p, ar1(50, 0.9), 40)
	a = stat.AppendACF(a, x, 20)
	p = stat.AppendPACF(p, x, 20)
	for k := range wantA {
		if a[k] != wantA[k] || p[k] != wantP[k] {
			t.Fatalf("lag %d after reuse: acf %v pacf %v, fresh %v %v", k, a[k], p[k], wantA[k], wantP[k])
		}
	}
	if allocs := testing.AllocsPerRun(10, func() { p = stat.AppendPACF(p, x, 20) }); allocs != 0 {
		t.Errorf("AppendPACF into a reused buffer allocated %v times", allocs)
	}
	for k := range wantP {
		if math.Abs(p[k]) > 1 {
			t.Errorf("pacf_%d = %v is outside [−1, 1]", k, p[k])
		}
	}
}

func TestCorrelationBound(t *testing.T) {
	if got := stat.CorrelationBound(100, 1.96); math.Abs(got-0.196) > 1e-12 {
		t.Errorf("bound = %v, want 0.196", got)
	}
	if !math.IsNaN(stat.CorrelationBound(0, 1.96)) {
		t.Error("a bound over no readings is not NaN")
	}
}

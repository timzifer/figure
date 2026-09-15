package stat

import "math"

// ACF returns the sample autocorrelation of a series at lags 0 through maxLag:
// how strongly the series resembles itself shifted by k steps. Index k is the
// correlation at lag k, and index 0 is 1.
//
// It is the arithmetic under a correlogram, and it is a reduction in the sense
// ADR 0054 admits: the bars of a correlogram are the estimator, and there is
// no reading of these numbers that is not that chart.
//
// # The estimator
//
// It is the standard biased one: the lag-k covariance is summed over the n−k
// pairs that exist but divided by n, not by n−k, and every term is centred on
// the mean of the whole series. Dividing by n−k would be unbiased term by term
// and is the wrong choice anyway, for the reason every time-series text gives:
// the biased sequence is positive semi-definite, which is what guarantees the
// partial autocorrelations [PACF] derives from it stay inside [−1, 1] and the
// Durbin–Levinson recursion never divides by a negative variance. It also
// shrinks the long lags, where only a handful of pairs contribute, towards
// zero rather than letting them swing wildly.
//
// # Input
//
// This is the one reduction of ADR 0054's five that does not take sorted input.
// A time series is ordered by time, and that order is the whole content of an
// autocorrelation: sorting it would turn every series into a monotone ramp
// whose correlogram says nothing about the data.
//
// A non-finite value anywhere makes every correlation NaN. Skipping it the way
// [ECDF] skips one would silently join the observations either side of the
// gap, so every pair straddling it would be measured at the wrong lag — a
// correlogram that looks exactly like a real one and is not. The caller is the
// one who knows whether to interpolate, split the series, or drop the reading.
//
// maxLag is clamped to n−1, the longest lag that has a pair at all, and to 0
// from below. A series with no variance has no correlation to measure: its lag
// 0 is 1 and every other lag is NaN.
func ACF(x []float64, maxLag int) []float64 { return AppendACF(nil, x, maxLag) }

// AppendACF is [ACF] writing into a caller-owned slice. dst is truncated first.
func AppendACF(dst []float64, x []float64, maxLag int) []float64 {
	n := len(x)
	if n == 0 {
		return dst[:0]
	}
	m := clampLag(maxLag, n)
	dst = growFloats(dst[:0], m+1)
	acfInto(dst, x)
	return dst
}

// PACF returns the sample partial autocorrelation of a series at lags 0
// through maxLag: the correlation at lag k left over once lags 1 to k−1 have
// been accounted for.
//
// Where an ACF of an autoregressive series decays gradually, its PACF cuts off
// after the order of the process, which is what a correlogram pair is read for.
// Index 0 is 1 by convention — nothing is partialled out at lag 0, so it is the
// plain correlation of the series with itself — which keeps the two slices
// indexed the same way and lets one chart draw both.
//
// It is computed from [ACF] by the Durbin–Levinson recursion, so it inherits
// every rule there: the series is not sorted, a non-finite value makes the
// whole result NaN, maxLag is clamped to n−1, and a series with no variance is
// 1 at lag 0 and NaN beyond.
func PACF(x []float64, maxLag int) []float64 { return AppendPACF(nil, x, maxLag) }

// AppendPACF is [PACF] writing into a caller-owned slice. dst is truncated
// first.
//
// The recursion needs the autocorrelations and one row of autoregressive
// coefficients beside the result, and it keeps both in dst's own backing array
// past the length it returns: dst is grown to three times maxLag+1 and
// resliced to maxLag+1. A caller reusing dst therefore allocates nothing after
// the first call, and one that does not allocates once.
func AppendPACF(dst []float64, x []float64, maxLag int) []float64 {
	n := len(x)
	if n == 0 {
		return dst[:0]
	}
	m := clampLag(maxLag, n)
	buf := growFloats(dst[:0], 3*(m+1))
	out, r, phi := buf[:m+1], buf[m+1:2*(m+1)], buf[2*(m+1):]
	acfInto(r, x)

	out[0] = r[0]
	if m == 0 {
		return out
	}
	if math.IsNaN(r[1]) {
		for k := 1; k <= m; k++ {
			out[k] = math.NaN()
		}
		return out
	}

	// Durbin–Levinson. phi[1..k] are the coefficients of the best order-k
	// autoregression, and the last of them is the partial correlation at lag
	// k. The update of the earlier coefficients reads each one's mirror
	// image, so it is done a symmetric pair at a time, in place.
	phi[1], out[1] = r[1], r[1]
	for k := 2; k <= m; k++ {
		num, den := r[k], 1.0
		for j := 1; j < k; j++ {
			num -= phi[j] * r[k-j]
			den -= phi[j] * r[j]
		}
		pkk := num / den
		for j, i := 1, k-1; j <= i; j, i = j+1, i-1 {
			a, b := phi[j], phi[i]
			phi[j] = a - pkk*b
			if j != i {
				phi[i] = b - pkk*a
			}
		}
		phi[k], out[k] = pkk, pkk
	}
	return out
}

// CorrelationBound is the half-width of the band a correlogram draws around
// zero: z/√n, for a series of n observations and z the standard normal quantile
// of the confidence wanted — 1.96 for 95 %.
//
// It is the large-sample bound under the white-noise null: a correlation
// outside it is unlikely if the series has no autocorrelation at all. It is not
// Bartlett's formula, which widens the band at lag k by the correlations at
// lags below k and is the right bound for judging a moving-average order.
// That widening is not applied here, because it answers a modelling question
// rather than the one the plain band on a correlogram asks.
func CorrelationBound(n int, z float64) float64 {
	if n <= 0 {
		return math.NaN()
	}
	return z / math.Sqrt(float64(n))
}

// acfInto writes the autocorrelations at lags 0..len(r)−1 of x into r.
func acfInto(r []float64, x []float64) {
	n := float64(len(x))
	mean := 0.0
	for _, v := range x {
		if !finite(v) {
			for k := range r {
				r[k] = math.NaN()
			}
			return
		}
		mean += v
	}
	mean /= n

	c0 := 0.0
	for _, v := range x {
		d := v - mean
		c0 += d * d
	}
	r[0] = 1
	for k := 1; k < len(r); k++ {
		if c0 == 0 {
			r[k] = math.NaN()
			continue
		}
		ck := 0.0
		for i := k; i < len(x); i++ {
			ck += (x[i] - mean) * (x[i-k] - mean)
		}
		r[k] = ck / c0
	}
}

// clampLag bounds a requested lag to the ones a series of n has pairs for.
func clampLag(maxLag, n int) int {
	return min(max(maxLag, 0), n-1)
}

// growFloats returns dst resliced to n, reallocating only when it is too
// small.
func growFloats(dst []float64, n int) []float64 {
	if cap(dst) < n {
		return make([]float64, n)
	}
	return dst[:n]
}

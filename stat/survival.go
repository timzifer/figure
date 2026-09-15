package stat

import "math"

// SurvivalPoint is one step of a Kaplan–Meier curve: a distinct time at which
// something happened, and what the estimate is just after it.
type SurvivalPoint struct {
	// T is the time.
	T float64
	// S is the estimated probability of surviving past T.
	S float64
	// AtRisk is how many subjects were still under observation just before T:
	// every row whose time is T or later.
	AtRisk int
	// Events is how many of them had the event at T, and Censored how many
	// left observation at T without having had it.
	Events, Censored int
	// Greenwood is the running sum Σ d/(n(n−d)) over every event time up to
	// and including T — the part of Greenwood's variance that does not depend
	// on S. It is +Inf from the time S reaches zero, where the variance has no
	// finite value. See [SurvivalPoint.Band].
	Greenwood float64
}

// KaplanMeier returns the Kaplan–Meier estimate of a survival function from
// right-censored observations: one point per distinct time, in ascending
// order, carrying the estimate just after that time.
//
// sorted is each subject's time — to the event, or to the last moment it was
// observed — ascending, and event says which: true for an event, false for a
// subject censored at that time. The two are read in parallel, and the shorter
// decides how many rows there are.
//
// It is the reduction ADR 0054 admits first, and for its reason: the
// estimator *is* the curve. There is no other object to hand back — the step
// function drawn is the product of (1 − d/n) over the event times, and every
// reading of it is a reading of the chart.
//
// # Ties and censoring
//
// Rows at one time are one step. A subject censored at a time is counted at
// risk at that time — it was observed up to it — and leaves afterwards, which
// is the convention every survival package follows. A time at which only
// censoring happened is still a point, with S unchanged: it is where a chart
// puts its censoring tick, and dropping it would lose the one thing a reader
// uses to judge how much of the tail is estimated from how few.
//
// The column must be sorted ascending, for the reason [ECDF] gives. A time
// that is NaN or infinite is ignored along with its event flag.
func KaplanMeier(sorted []float64, event []bool) []SurvivalPoint {
	return AppendKaplanMeier(nil, sorted, event)
}

// AppendKaplanMeier is [KaplanMeier] writing into a caller-owned slice. dst is
// truncated first.
func AppendKaplanMeier(dst []SurvivalPoint, sorted []float64, event []bool) []SurvivalPoint {
	dst = dst[:0]
	n := min(len(sorted), len(event))
	atRisk := 0
	for _, t := range sorted[:n] {
		if finite(t) {
			atRisk++
		}
	}

	s, gw := 1.0, 0.0
	for i := 0; i < n; {
		t := sorted[i]
		if !finite(t) {
			i++
			continue
		}
		d, c := 0, 0
		for ; i < n && (sorted[i] == t || !finite(sorted[i])); i++ {
			if !finite(sorted[i]) {
				continue
			}
			if event[i] {
				d++
			} else {
				c++
			}
		}
		if d > 0 {
			s *= 1 - float64(d)/float64(atRisk)
			if d < atRisk {
				gw += float64(d) / (float64(atRisk) * float64(atRisk-d))
			} else {
				gw = math.Inf(1)
			}
		}
		dst = append(dst, SurvivalPoint{T: t, S: s, AtRisk: atRisk, Events: d, Censored: c, Greenwood: gw})
		atRisk -= d + c
	}
	return dst
}

// Band returns a pointwise confidence interval for S at this point, z
// standard errors wide — 1.96 for 95 %.
//
// It is the log-log interval: Greenwood's variance carried through
// log(−log S), where the interval is symmetric, and back. That is the default
// in R's survival package and in lifelines, and it is chosen over the plain
// S ± z·SE for the reason a chart cares about most: the plain interval runs
// past 0 and 1 in the tails, which is where a survival curve is read hardest,
// and a band drawn below zero is a band nobody can believe.
//
// Before the first event S is 1 and the band is [1, 1]; once S reaches 0 it is
// [0, 0]. Both are what the estimate says, not a failure to say anything.
func (p SurvivalPoint) Band(z float64) (lo, hi float64) {
	switch {
	case p.S >= 1:
		return 1, 1
	case p.S <= 0:
		return 0, 0
	case math.IsInf(p.Greenwood, 1):
		return 0, 1
	}
	ls := math.Log(p.S)
	se := math.Sqrt(p.Greenwood) / math.Abs(ls)
	return math.Pow(p.S, math.Exp(z*se)), math.Pow(p.S, math.Exp(-z*se))
}

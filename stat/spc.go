package stat

import "math"

// Control limits and the run rules that judge a series against them.
//
// A control chart is three horizontal lines and a series: a centre, an upper
// and a lower limit, and the measurements plotted over them. Every line of
// that is a mark figure has had since v0.1. What it did not have is the
// arithmetic — the constants that turn a baseline's ranges into limits, and
// the eight patterns a quality engineer reads the series for — and that is
// what this file is.
//
// # The limits are the caller's
//
// There is deliberately no mark that draws a control chart, and the reason is
// the method rather than economy. ADR 0054 puts it plainly: "control limits
// are computed from a baseline period and then frozen; new observations are
// judged against limits derived from data that is not on the chart." Phase I
// establishes the limits from data believed to be in control; phase II watches
// new data against them. A layer that recomputed its limits from the points it
// was handed would be right for exploration and silently wrong for the use the
// chart exists for.
//
// So the Limits functions below are run once, by the caller, over the
// baseline, and [AppendRunRules] judges any series against whatever Limits it
// is given. Nothing here ever recomputes a limit from the series it judges.
// The three lines are then three [github.com/timzifer/figure/geom.HLine]s, and
// the flags are a derived column for a colour scale.

// Limits is a control chart's three lines.
type Limits struct {
	Centre, Lower, Upper float64
}

// Sigma is the width of one standard deviation implied by the limits: a third
// of the distance from the centre to the upper limit.
//
// It is read from the upper side because the lower one is the side that gets
// clamped — at zero for a count, or for a range — and a clamped limit no
// longer says anything about the spread.
func (l Limits) Sigma() float64 { return (l.Upper - l.Centre) / 3 }

// Shewhart's constants for a moving range of two, which is what an
// individuals chart uses: d2 = 1.128, so three sigma is 3/1.128 = 2.66 moving
// ranges, and D4 = 3.267 is the upper limit of the moving range itself.
const (
	imrE2 = 2.66
	imrD4 = 3.267
)

// LimitsIMR returns the limits of an individuals chart and of the moving-range
// chart that goes under it.
//
// The individuals centre is the mean of x, and its limits are 2.66 average
// moving ranges either side — 3/d2 with d2 = 1.128 for a range of two. The
// moving-range chart is centred on that average, with an upper limit of
// 3.267 times it and a lower limit of zero.
//
// Non-finite values are ignored. The mean is taken over the finite ones, and a
// moving range is taken only between two neighbouring rows that are both
// finite: a gap in the record is not a jump in the process. With no finite
// value both Limits are zero; with one, the individuals centre is that value
// and every range is zero.
func LimitsIMR(x []float64) (individuals, movingRange Limits) {
	sum, n := 0.0, 0
	mrSum, mrN := 0.0, 0
	for i, v := range x {
		if !finite(v) {
			continue
		}
		sum += v
		n++
		if i > 0 && finite(x[i-1]) {
			mrSum += math.Abs(v - x[i-1])
			mrN++
		}
	}
	if n == 0 {
		return Limits{}, Limits{}
	}
	centre := sum / float64(n)
	mr := 0.0
	if mrN > 0 {
		mr = mrSum / float64(mrN)
	}
	individuals = Limits{Centre: centre, Lower: centre - imrE2*mr, Upper: centre + imrE2*mr}
	movingRange = Limits{Centre: mr, Lower: 0, Upper: imrD4 * mr}
	return individuals, movingRange
}

// xbarR holds A2, D3 and D4 for a subgroup of size 2 to 25, indexed by the
// size. These are the standard tabulated values, to the three places every
// textbook prints them.
var xbarR = [26][3]float64{
	2:  {1.880, 0, 3.267},
	3:  {1.023, 0, 2.574},
	4:  {0.729, 0, 2.282},
	5:  {0.577, 0, 2.114},
	6:  {0.483, 0, 2.004},
	7:  {0.419, 0.076, 1.924},
	8:  {0.373, 0.136, 1.864},
	9:  {0.337, 0.184, 1.816},
	10: {0.308, 0.223, 1.777},
	11: {0.285, 0.256, 1.744},
	12: {0.266, 0.283, 1.717},
	13: {0.249, 0.307, 1.693},
	14: {0.235, 0.328, 1.672},
	15: {0.223, 0.347, 1.653},
	16: {0.212, 0.363, 1.637},
	17: {0.203, 0.378, 1.622},
	18: {0.194, 0.391, 1.608},
	19: {0.187, 0.403, 1.597},
	20: {0.180, 0.415, 1.585},
	21: {0.173, 0.425, 1.575},
	22: {0.167, 0.434, 1.566},
	23: {0.162, 0.443, 1.557},
	24: {0.157, 0.451, 1.548},
	25: {0.153, 0.459, 1.541},
}

// LimitsXbarR returns the limits of an X̄ chart and of the range chart under
// it, from x read as consecutive subgroups of the given size.
//
// The X̄ centre is the grand mean and its limits are A2 times the average range
// either side; the range chart runs from D3 to D4 times the average range. The
// constants are tabulated for sizes 2 to 25, which is the range the method is
// used over — past 25 an S chart is the right instrument — and a size outside
// it returns zero Limits for both rather than an extrapolated constant nobody
// published.
//
// A trailing partial subgroup is ignored: a range over fewer readings than the
// others is systematically smaller, and would pull the average range down. A
// subgroup containing a non-finite value is skipped whole, for the same reason.
// With no complete subgroup both Limits are zero.
func LimitsXbarR(x []float64, size int) (mean, rng Limits) {
	if size < 2 || size >= len(xbarR) {
		return Limits{}, Limits{}
	}
	c := xbarR[size]
	sum, rSum, k := 0.0, 0.0, 0
next:
	for start := 0; start+size <= len(x); start += size {
		lo, hi, s := math.Inf(1), math.Inf(-1), 0.0
		for _, v := range x[start : start+size] {
			if !finite(v) {
				continue next
			}
			lo, hi, s = math.Min(lo, v), math.Max(hi, v), s+v
		}
		sum += s / float64(size)
		rSum += hi - lo
		k++
	}
	if k == 0 {
		return Limits{}, Limits{}
	}
	grand, r := sum/float64(k), rSum/float64(k)
	mean = Limits{Centre: grand, Lower: grand - c[0]*r, Upper: grand + c[0]*r}
	rng = Limits{Centre: r, Lower: c[1] * r, Upper: c[2] * r}
	return mean, rng
}

// LimitsNP returns the limits of an np chart: the number of defective units in
// subgroups that all have the given size.
//
// The centre is n·p̄, with p̄ the total defectives over the total inspected, and
// the limits are three binomial standard deviations, √(n·p̄·(1−p̄)), either
// side — clamped to [0, n], because a count of defectives cannot leave it.
//
// Non-finite counts are ignored. A size that is not positive, or no finite
// count, returns zero Limits.
func LimitsNP(defectives []float64, size float64) Limits {
	if !(size > 0) || math.IsInf(size, 0) {
		return Limits{}
	}
	sum, k := 0.0, 0
	for _, d := range defectives {
		if finite(d) {
			sum += d
			k++
		}
	}
	if k == 0 {
		return Limits{}
	}
	p := sum / (float64(k) * size)
	centre := size * p
	s := math.Sqrt(size * p * (1 - p))
	return Limits{
		Centre: centre,
		Lower:  math.Max(centre-3*s, 0),
		Upper:  math.Min(centre+3*s, size),
	}
}

// LimitsC returns the limits of a c chart: a count of defects per unit of a
// constant size.
//
// The centre is the mean count c̄ and the limits are three Poisson standard
// deviations, √c̄, either side, with the lower one clamped at zero.
//
// Non-finite counts are ignored; with none left the Limits are zero.
func LimitsC(counts []float64) Limits {
	sum, k := 0.0, 0
	for _, c := range counts {
		if finite(c) {
			sum += c
			k++
		}
	}
	if k == 0 {
		return Limits{}
	}
	centre := sum / float64(k)
	s := math.Sqrt(centre)
	return Limits{Centre: centre, Lower: math.Max(centre-3*s, 0), Upper: centre + 3*s}
}

// AppendLimitsP returns the centre of a p chart — the fraction defective when
// subgroups differ in size — and the lower and upper limit of each subgroup,
// written into lo and hi, which it truncates and grows as needed.
//
// The centre is p̄, the total defectives over the total inspected. Subgroup i's
// limits are p̄ ± 3·√(p̄(1−p̄)/n_i), clamped to [0, 1]: a smaller subgroup is
// allowed to wander further, which is why a p chart's limits are a staircase
// rather than two lines.
//
// lo and hi are indexed like the input, so a caller can draw them against the
// same rows. A subgroup whose count or size is non-finite, or whose size is not
// positive, contributes nothing to p̄ and gets NaN limits — a hole in the
// staircase, under the layer's own missing-data policy. With nothing usable the
// centre is zero.
func AppendLimitsP(lo, hi []float64, defectives, sizes []float64) (centre float64, _, _ []float64) {
	n := min(len(defectives), len(sizes))
	d, total := 0.0, 0.0
	for i := range n {
		if usableSubgroup(defectives[i], sizes[i]) {
			d += defectives[i]
			total += sizes[i]
		}
	}
	lo, hi = lo[:0], hi[:0]
	if total > 0 {
		centre = d / total
	}
	for i := range n {
		if !usableSubgroup(defectives[i], sizes[i]) || !(total > 0) {
			lo, hi = append(lo, math.NaN()), append(hi, math.NaN())
			continue
		}
		s := math.Sqrt(centre * (1 - centre) / sizes[i])
		lo = append(lo, math.Max(centre-3*s, 0))
		hi = append(hi, math.Min(centre+3*s, 1))
	}
	return centre, lo, hi
}

// AppendLimitsU returns the centre of a u chart — defects per unit when the
// amount inspected varies — and the lower and upper limit of each subgroup,
// written into lo and hi, which it truncates and grows as needed.
//
// The centre is ū, the total count over the total units, and subgroup i's
// limits are ū ± 3·√(ū/units_i), with the lower one clamped at zero. Unusable
// subgroups are treated as [AppendLimitsP] treats them.
func AppendLimitsU(lo, hi []float64, counts, units []float64) (centre float64, _, _ []float64) {
	n := min(len(counts), len(units))
	c, total := 0.0, 0.0
	for i := range n {
		if usableSubgroup(counts[i], units[i]) {
			c += counts[i]
			total += units[i]
		}
	}
	lo, hi = lo[:0], hi[:0]
	if total > 0 {
		centre = c / total
	}
	for i := range n {
		if !usableSubgroup(counts[i], units[i]) || !(total > 0) {
			lo, hi = append(lo, math.NaN()), append(hi, math.NaN())
			continue
		}
		s := math.Sqrt(centre / units[i])
		lo = append(lo, math.Max(centre-3*s, 0))
		hi = append(hi, centre+3*s)
	}
	return centre, lo, hi
}

func usableSubgroup(count, size float64) bool {
	return finite(count) && finite(size) && size > 0
}

// RunRule is one of the eight Nelson rules: a pattern in a series that a
// process in control is very unlikely to produce. Their values are the rules'
// conventional numbers, 1 to 8. The first four with rules 5 and 6 are also the
// Western Electric rules, under a different numbering.
type RunRule uint8

// The Nelson rules. σ is [Limits.Sigma].
const (
	// RuleBeyondLimits: one point above the upper limit or below the lower.
	RuleBeyondLimits RunRule = iota + 1
	// RuleNineOneSide: nine points in a row on the same side of the centre.
	RuleNineOneSide
	// RuleSixTrending: six points in a row steadily rising or steadily
	// falling.
	RuleSixTrending
	// RuleFourteenAlternating: fourteen points in a row alternating up and
	// down.
	RuleFourteenAlternating
	// RuleTwoOfThreeBeyond2Sigma: two of three points in a row more than 2σ
	// from the centre, on the same side.
	RuleTwoOfThreeBeyond2Sigma
	// RuleFourOfFiveBeyond1Sigma: four of five points in a row more than 1σ
	// from the centre, on the same side.
	RuleFourOfFiveBeyond1Sigma
	// RuleFifteenWithin1Sigma: fifteen points in a row within 1σ of the
	// centre, either side — a process quieter than its limits say, which
	// usually means the limits are stale.
	RuleFifteenWithin1Sigma
	// RuleEightOutside1Sigma: eight points in a row more than 1σ from the
	// centre, on either side — a mixture of two processes.
	RuleEightOutside1Sigma
)

// Flag is one point a run rule caught: the row, and the rule that caught it.
type Flag struct {
	Row  int
	Rule RunRule
}

// AppendRunRules judges x against l by the given rules, or by all eight when
// none are given, and appends a Flag for every point that completes a
// qualifying pattern. dst is truncated first.
//
// The limits are the caller's. They come from a baseline — see the top of this
// file — and are applied as given: this function never derives a limit, a
// centre or a σ from the series it is judging, which is what lets it watch new
// data against old limits.
//
// # Which point is flagged
//
// A pattern is flagged at the point that completes it, and at every later
// point that keeps it complete: ten points on one side flag the ninth and the
// tenth under [RuleNineOneSide]. For the rules that count some points of a
// window — [RuleTwoOfThreeBeyond2Sigma] and [RuleFourOfFiveBeyond1Sigma] — the
// completing point must itself be one of the points counted, so a flag always
// lands on a point a reader can see is out of place.
//
// Flags are ordered by row, then by rule. A non-finite value never counts
// towards a pattern and breaks every run that would cross it. The rules that
// measure in σ flag nothing when [Limits.Sigma] is not positive, since there is
// then no scale to measure in; a point exactly on the centre is on neither side
// of it.
func AppendRunRules(dst []Flag, x []float64, l Limits, rules ...RunRule) []Flag {
	dst = dst[:0]
	var on [9]bool
	if len(rules) == 0 {
		for r := RuleBeyondLimits; r <= RuleEightOutside1Sigma; r++ {
			on[r] = true
		}
	}
	for _, r := range rules {
		if r >= RuleBeyondLimits && r <= RuleEightOutside1Sigma {
			on[r] = true
		}
	}
	sigma := l.Sigma()
	scaled := sigma > 0 && finite(sigma)
	c := l.Centre

	// run is how many finite values end at i, so a window of k rows is usable
	// exactly when run >= k.
	run := 0
	for i, v := range x {
		if !finite(v) {
			run = 0
			continue
		}
		run++
		for r := RuleBeyondLimits; r <= RuleEightOutside1Sigma; r++ {
			if on[r] && runRule(r, x, i, run, c, sigma, scaled, l) {
				dst = append(dst, Flag{Row: i, Rule: r})
			}
		}
	}
	return dst
}

// runRule reports whether r is completed at row i, given that the run rows ending
// at i are all finite.
func runRule(r RunRule, x []float64, i, run int, c, sigma float64, scaled bool, l Limits) bool {
	v := x[i]
	switch r {
	case RuleBeyondLimits:
		return v > l.Upper || v < l.Lower
	case RuleNineOneSide:
		return run >= 9 && runAll(x[i-8:i+1], func(u float64) bool { return u > c }) ||
			run >= 9 && runAll(x[i-8:i+1], func(u float64) bool { return u < c })
	case RuleSixTrending:
		if run < 6 {
			return false
		}
		w := x[i-5 : i+1]
		up, down := true, true
		for k := 1; k < len(w); k++ {
			up = up && w[k] > w[k-1]
			down = down && w[k] < w[k-1]
		}
		return up || down
	case RuleFourteenAlternating:
		if run < 14 {
			return false
		}
		w := x[i-13 : i+1]
		for k := 2; k < len(w); k++ {
			if (w[k]-w[k-1])*(w[k-1]-w[k-2]) >= 0 {
				return false
			}
		}
		return true
	case RuleTwoOfThreeBeyond2Sigma:
		return scaled && run >= 3 && runSomeOf(x[i-2:i+1], 2, c, 2*sigma)
	case RuleFourOfFiveBeyond1Sigma:
		return scaled && run >= 5 && runSomeOf(x[i-4:i+1], 4, c, sigma)
	case RuleFifteenWithin1Sigma:
		return scaled && run >= 15 && runAll(x[i-14:i+1], func(u float64) bool { return math.Abs(u-c) < sigma })
	case RuleEightOutside1Sigma:
		return scaled && run >= 8 && runAll(x[i-7:i+1], func(u float64) bool { return math.Abs(u-c) > sigma })
	}
	return false
}

func runAll(w []float64, ok func(float64) bool) bool {
	for _, u := range w {
		if !ok(u) {
			return false
		}
	}
	return true
}

// runSomeOf reports whether at least k points of w are more than dist from c on
// one side, with the last point of w among them.
func runSomeOf(w []float64, k int, c, dist float64) bool {
	last := w[len(w)-1]
	var side func(float64) bool
	switch {
	case last > c+dist:
		side = func(u float64) bool { return u > c+dist }
	case last < c-dist:
		side = func(u float64) bool { return u < c-dist }
	default:
		return false
	}
	n := 0
	for _, u := range w {
		if side(u) {
			n++
		}
	}
	return n >= k
}

package stat

import "math"

// ECDF returns the empirical cumulative distribution of an ascending column:
// one point per distinct value, whose Y is the fraction of observations at or
// below it.
//
// It is the distribution plot that invents nothing. A histogram picks bin edges
// and a density picks a bandwidth, and both choices change the picture; an ECDF
// has no parameter at all, so two of them drawn together are two datasets
// compared rather than two smoothing choices compared. That is what makes it
// worth having beside [Bin] and [KDE] rather than instead of either.
//
// The column must be sorted ascending, for the reason [Quantile] is: sorting it
// here would mean either mutating the caller's data or allocating a copy of it
// on every frame. NaN and infinities are ignored, which is why the fractions
// are taken against the number of usable values rather than against len.
func ECDF(sorted []float64) []Point { return AppendECDF(nil, sorted) }

// AppendECDF is [ECDF] writing into a caller-owned slice. dst is truncated
// first.
//
// One point per *distinct* value, not per row: a column with a thousand copies
// of one number is one step of height 1000/n, and emitting a thousand points at
// one X would draw a thousand identical vertices and report a thousand marks to
// a hit test.
func AppendECDF(dst []Point, sorted []float64) []Point {
	dst = dst[:0]
	n := countFinite(sorted)
	if n == 0 {
		return dst
	}

	seen := 0
	for i := 0; i < len(sorted); i++ {
		v := sorted[i]
		if !finite(v) {
			continue
		}
		seen++
		// Look ahead over the ties so the step is emitted once, at its full
		// height. The scan is linear overall: each row is visited by exactly one
		// of the two loops.
		for i+1 < len(sorted) && sorted[i+1] == v {
			i++
			seen++
		}
		dst = append(dst, Point{X: v, Y: float64(seen) / float64(n)})
	}
	return dst
}

// MedianRank returns Benard's median-rank plotting positions of an ascending
// column: one point per observation, whose Y is (i − 0.3)/(n + 0.4) for the
// i-th of n usable values.
//
// It is the plotting position probability paper is read with. An [ECDF]
// reaches 1 at its last observation, and 1 is off the end of every probability
// axis, so its top step cannot be drawn there; a median rank never reaches 0 or
// 1 by construction, so every observation has a place. It is also one point per
// *row* rather than per distinct value, because in a life test each failure is
// its own rank even when two units fail at the same hour.
//
// The column must be sorted ascending, and NaN and infinities are ignored, for
// the reasons [ECDF] gives.
func MedianRank(sorted []float64) []Point { return AppendMedianRank(nil, sorted) }

// AppendMedianRank is [MedianRank] writing into a caller-owned slice. dst is
// truncated first.
func AppendMedianRank(dst []Point, sorted []float64) []Point {
	dst = dst[:0]
	n := countFinite(sorted)
	if n == 0 {
		return dst
	}
	i := 0
	for _, v := range sorted {
		if !finite(v) {
			continue
		}
		i++
		dst = append(dst, Point{X: v, Y: (float64(i) - 0.3) / (float64(n) + 0.4)})
	}
	return dst
}

// ROC returns the receiver operating characteristic of a scored sample: one
// point per distinct score, whose X is the false positive rate and whose Y is
// the true positive rate of the classifier that calls everything at or above
// that score positive — and the area under the curve.
//
// sorted is the scores ascending, and positive[i] says whether row i is a
// positive. The column must be sorted for the reason [ECDF]'s must: sorting it
// here would mean either reordering the caller's scores away from their labels
// or allocating a copy of both on every frame. A score that is NaN or infinite
// is ignored, and its label with it.
//
// It is two ECDFs read against each other — the positives' and the
// negatives', each counted from the top — and it is walked from the highest
// score down, so the curve starts at (0, 0), where nothing is called positive,
// and ends at (1, 1), where everything is.
//
// # Ties
//
// A group of tied scores is one step, not a run of steps in row order: a
// threshold cannot separate rows that score the same, so the curve moves
// diagonally across the group. That is also what makes the trapezoidal area
// exactly the Mann–Whitney probability that a random positive outscores a
// random negative, with a tie counted as half — the area is a statistic with a
// reading, rather than one that depends on how the rows happened to be
// ordered.
//
// A sample with no positives or no negatives has no rate to divide by on one
// of the axes, and comes back as an empty curve and a NaN area. 0.5 would be a
// claim — a classifier no better than chance — about a sample that cannot say
// anything of the kind.
func ROC(sorted []float64, positive []bool) (curve []Point, auc float64) {
	return AppendROC(nil, sorted, positive)
}

// AppendROC is [ROC] writing into a caller-owned slice. dst is truncated
// first.
func AppendROC(dst []Point, sorted []float64, positive []bool) ([]Point, float64) {
	dst = dst[:0]
	pos, neg := scoredCounts(sorted, positive)
	if pos == 0 || neg == 0 {
		return dst, math.NaN()
	}
	dst = append(dst, Point{})
	tp, fp := 0, 0
	auc := 0.0
	walkScores(sorted, positive, func(dp, dn int) {
		prevTPR := float64(tp) / float64(pos)
		tp, fp = tp+dp, fp+dn
		tpr := float64(tp) / float64(pos)
		// A trapezoid per step: the width is the step's share of the
		// negatives, the height the mean of the rates either side of it.
		auc += float64(dn) / float64(neg) * (prevTPR + tpr) / 2
		dst = append(dst, Point{X: float64(fp) / float64(neg), Y: tpr})
	})
	return dst, auc
}

// PrecisionRecall returns the precision–recall curve of a scored sample: one
// point per distinct score, whose X is the recall and whose Y is the precision
// of calling everything at or above that score positive — and the average
// precision.
//
// It is [ROC]'s walk with the other two ratios, and it takes its input the same
// way: scores ascending, labels parallel, non-finite scores ignored with their
// labels, and a group of tied scores as one step.
//
// The curve starts at (0, 1). Precision at a recall of zero is a division of
// nothing by nothing, and 1 is the convention reference implementations use for
// it; the point is there so that the first step has a left edge.
//
// The average precision is Σ (Rₖ − Rₖ₋₁)·Pₖ: each step in recall weighted by
// the precision reached at its end. It is deliberately not the trapezoidal
// area. Precision does not move linearly between two thresholds — it can fall
// and recover — so interpolating straight across a step credits the classifier
// with operating points it never has, and the trapezoid over-states the curve.
//
// A sample with no positives has no recall, and comes back as an empty curve
// and a NaN average precision.
func PrecisionRecall(sorted []float64, positive []bool) (curve []Point, ap float64) {
	return AppendPrecisionRecall(nil, sorted, positive)
}

// AppendPrecisionRecall is [PrecisionRecall] writing into a caller-owned
// slice. dst is truncated first.
func AppendPrecisionRecall(dst []Point, sorted []float64, positive []bool) ([]Point, float64) {
	dst = dst[:0]
	pos, _ := scoredCounts(sorted, positive)
	if pos == 0 {
		return dst, math.NaN()
	}
	dst = append(dst, Point{X: 0, Y: 1})
	tp, called := 0, 0
	ap := 0.0
	walkScores(sorted, positive, func(dp, dn int) {
		tp, called = tp+dp, called+dp+dn
		precision := float64(tp) / float64(called)
		ap += float64(dp) / float64(pos) * precision
		dst = append(dst, Point{X: float64(tp) / float64(pos), Y: precision})
	})
	return dst, ap
}

// scoredCounts counts the positives and negatives among the rows with a
// usable score. A row past the end of either slice is not a row.
func scoredCounts(sorted []float64, positive []bool) (pos, neg int) {
	for i := range min(len(sorted), len(positive)) {
		if !finite(sorted[i]) {
			continue
		}
		if positive[i] {
			pos++
		} else {
			neg++
		}
	}
	return pos, neg
}

// walkScores visits the distinct usable scores from the highest down, handing
// step the number of positives and negatives that score exactly that.
//
// Non-finite scores are skipped wherever they sit, including between two tied
// finite ones, so a NaN in the middle of a tie does not split it.
func walkScores(sorted []float64, positive []bool, step func(dp, dn int)) {
	i := min(len(sorted), len(positive)) - 1
	for i >= 0 {
		if !finite(sorted[i]) {
			i--
			continue
		}
		v := sorted[i]
		dp, dn := 0, 0
		for ; i >= 0; i-- {
			if !finite(sorted[i]) {
				continue
			}
			if sorted[i] != v {
				break
			}
			if positive[i] {
				dp++
			} else {
				dn++
			}
		}
		step(dp, dn)
	}
}

// Lorenz returns the Lorenz curve of an ascending column — incomes, holdings,
// file sizes — and its Gini coefficient.
//
// The curve is one point per row: X is the share of the rows counted so far,
// smallest first, and Y the share of the total they hold. It runs from (0, 0)
// to (1, 1), and it lies on the diagonal exactly when every row holds the same.
// The Gini coefficient is how far it sags below that diagonal: one minus twice
// the trapezoidal area under the curve, 0 for perfect equality and (n − 1)/n
// for one row holding everything.
//
// Tied values are separate points rather than one. A run of equal values is a
// straight segment either way — each row adds the same share to both
// coordinates — so the extra vertices lie on the line between their neighbours
// and change neither the picture nor the area.
//
// The column must be sorted ascending, for the reason [ECDF]'s must. NaN and
// infinities are ignored. A negative value is summed as it stands rather than
// clamped, for the reason [Rollup] gives: a curve that dips below zero says
// something true about the data, and clamping would hide it. A column whose
// total is not positive has no shares to take, and comes back as an empty
// curve and a NaN coefficient.
func Lorenz(sorted []float64) (curve []Point, gini float64) {
	return AppendLorenz(nil, sorted)
}

// AppendLorenz is [Lorenz] writing into a caller-owned slice. dst is truncated
// first.
func AppendLorenz(dst []Point, sorted []float64) ([]Point, float64) {
	dst = dst[:0]
	n, total := 0, 0.0
	for _, v := range sorted {
		if finite(v) {
			n++
			total += v
		}
	}
	if n == 0 || !(total > 0) {
		return dst, math.NaN()
	}
	dst = append(dst, Point{})
	seen, run, area := 0, 0.0, 0.0
	for _, v := range sorted {
		if !finite(v) {
			continue
		}
		prev := run / total
		seen++
		run += v
		share := run / total
		area += (prev + share) / 2 / float64(n)
		dst = append(dst, Point{X: float64(seen) / float64(n), Y: share})
	}
	return dst, 1 - 2*area
}

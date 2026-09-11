package stat

import "math"

// Levels returns about n contour levels spanning [lo, hi] at a round step.
//
// The step is the rung of the 1, 2, 5 × 10ⁿ ladder nearest in ratio to
// (hi-lo)/n, and the levels are its multiples strictly inside the range.
// Strictly, because an isoline at the minimum of the data is a point and one at
// the maximum is the boundary, and neither draws anything a reader can read.
//
// About n, and not n. A round step a reader can do arithmetic with is worth
// more than an exact count of awkward ones — a request for seven levels over
// [0, 1] is answered with nine at a tenth rather than seven at 0.125 — so the
// count is a hint at the density and the step is the promise.
//
// It is here beside [Sturges] and [FreedmanDiaconis] because it is the same
// kind of rule: how many, chosen from the data and from nothing else.
//
// # Why not the tick search
//
// The scale package has an extended-Wilkinson search that chooses far better
// numbers, and it is the wrong tool. That search optimises a *labelling* —
// simplicity, coverage, and density against an axis of a given length — so the
// count it lands on depends on how wide the panel is. A chart whose number of
// isolines changed when it was resized would be a chart whose reading changed
// with its size, which is the rule ADR 0011 states for an axis and ADR 0028
// restates for anything computed in Train. A level list has no length to be
// dense against.
func Levels(lo, hi float64, n int) []float64 {
	return AppendLevels(nil, lo, hi, n)
}

// AppendLevels is [Levels] writing into a caller-owned slice, which it
// truncates first. It is what a chart redrawn every frame calls.
func AppendLevels(dst []float64, lo, hi float64, n int) []float64 {
	dst = dst[:0]
	if n < 1 || !(lo < hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		return dst
	}
	step := niceStep((hi - lo) / float64(n))
	if step <= 0 || math.IsInf(step, 0) {
		return dst
	}

	// From an integer multiple rather than by accumulating, so that the levels
	// are the same numbers however many there are and on whichever machine —
	// an accumulated sum drifts, and a level that drifted past hi is an isoline
	// that appears on one architecture and not on another.
	first := math.Floor(lo/step) + 1
	for k := first; ; k++ {
		v := k * step
		if !(v < hi) {
			break
		}
		if v > lo {
			dst = append(dst, v)
		}
	}
	return dst
}

// niceStep is the rung of the 1, 2, 5 × 10ⁿ ladder nearest to want.
//
// Nearest in ratio rather than in difference, so the boundaries are the
// geometric means — 1.5, 3 and 7 — which is where a reader would put them: 2.5
// is as far above 2 as it is below 5 on a ruler read logarithmically, and a
// ruler of round numbers is read that way.
//
// Rounding up instead would be defensible and is much worse in practice: asked
// for four levels over [0, 1] it answers with the single level 0.5, because 0.2
// gives five intervals and five is more than four.
func niceStep(want float64) float64 {
	if !(want > 0) {
		return 0
	}
	mag := math.Pow(10, math.Floor(math.Log10(want)))
	switch f := want / mag; {
	case f < 1.5:
		return mag
	case f < 3:
		return 2 * mag
	case f < 7:
		return 5 * mag
	default:
		return 10 * mag
	}
}

package stat

import "math"

// The fold: cutting one value into bands of equal height so that a chart can
// spend its vertical space more than once.
//
// It is the arithmetic under a horizon chart. A series is measured from an
// origin, the distance is cut into bands of equal height, and every band is
// drawn at the panel's full height — so a chart one quarter as tall keeps the
// resolution a chart four times as tall would have, and which band a stretch
// of chart is in is said with colour rather than with position. Heer, Kong and
// Agrawala measured the trade in 2009: below about forty pixels of chart
// height the folded chart is read faster and more accurately than the filled
// line chart of the same series.
//
// Numbers in, numbers out, like every other reduction here: no scales, no
// theme, no geometry. See docs/adr/0065-horizon-charts.md.

// FoldArm is which side of a fold's origin a band lies on.
//
// The values below the origin are mirrored back above it and coloured from the
// other arm of the ramp, so a band is identified by a pair — how far from the
// origin it starts, and which way "away" runs. The direction is a named member
// of a closed family rather than a sign, for the reason a quantile function is
// ([ADR 0041](../docs/adr/0041-qq-plots.md)): a name is something a caller can
// read and a document can be written down as, where -1 is a number that could
// have been a magnitude.
type FoldArm int8

// The two arms of a fold.
const (
	// FoldAbove is the values greater than the origin.
	FoldAbove FoldArm = 1
	// FoldBelow is the values less than it, measured downwards.
	FoldBelow FoldArm = -1
)

// Fold reports how much of v lies inside one band, as a fraction of the band's
// height: 0 where the value does not reach the band, 1 where it passes right
// through, and what is left over in the band it ends in.
//
// A value in the other arm reads as 0, so the two arms of one band never draw
// over each other. A NaN stays a NaN — a row nobody measured has no position
// in a band either, and the caller's missing-data policy is what decides what
// a hole looks like.
func Fold(v, origin, height float64, band int, arm FoldArm) float64 {
	if math.IsNaN(v) {
		return math.NaN()
	}
	if !(height > 0) || band < 0 {
		return 0
	}
	f := float64(arm)*(v-origin)/height - float64(band)
	switch {
	case !(f > 0):
		return 0
	case f > 1:
		return 1
	}
	return f
}

// AppendFold is [Fold] over a whole column, writing into a caller-owned slice
// which it truncates first.
//
// It is the form a chart calls: one band of one frame is one pass over the
// values, into a buffer the layer keeps, so a chart redrawn every frame does
// not allocate a column per band per frame.
func AppendFold(dst []float64, vs []float64, origin, height float64, band int, arm FoldArm) []float64 {
	dst = dst[:0]
	for _, v := range vs {
		dst = append(dst, Fold(v, origin, height, band, arm))
	}
	return dst
}

// FoldHeight is the band height that fits a column's reach into n bands: the
// further of the two ends from the origin, divided by n.
//
// The further end rather than the whole span, because the two arms of a fold
// are drawn at one height — a band that meant one thing above the origin and
// another below it would make the chart's own two halves incomparable, which
// is most of what the form is for.
//
// It returns 0 for a column with no reach at all, which is the answer a caller
// has to have an opinion about: a series flat on its origin has no band.
func FoldHeight(lo, hi, origin float64, bands int) float64 {
	if bands < 1 {
		return 0
	}
	reach := foldReach(lo, hi, origin)
	if !(reach > 0) {
		return 0
	}
	return reach / float64(bands)
}

// FoldBands is how many bands of a given height a column's reach needs — the
// question [FoldHeight] answers the other way round, and the one a caller who
// pinned the height in the data's own units is asking.
//
// It is never less than one: a series that does not leave its origin is still
// drawn, as the flat line it is.
func FoldBands(lo, hi, origin, height float64) int {
	reach := foldReach(lo, hi, origin)
	if !(reach > 0) || !(height > 0) {
		return 1
	}
	n := int(math.Ceil(reach / height))
	if n < 1 {
		return 1
	}
	return n
}

// foldReach is how far the further end of [lo, hi] is from the origin, and 0
// when neither end is anywhere — an empty or non-finite extent.
func foldReach(lo, hi, origin float64) float64 {
	reach := math.Max(hi-origin, origin-lo)
	if math.IsNaN(reach) || math.IsInf(reach, 0) {
		return 0
	}
	return reach
}

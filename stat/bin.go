package stat

import "math"

// Bucket is one bin of a histogram: the interval it covers and how many
// observations fell in it.
//
// The interval is half open, [Lo, Hi), except for the last bucket of a run,
// which includes its upper bound — otherwise the largest observation in a
// column would fall in no bucket at all, which is the one value a reader is
// certain to look for.
type Bucket struct {
	Lo, Hi float64
	Count  int
}

// Mid is the bucket's centre, which is where a histogram bar is positioned.
func (b Bucket) Mid() float64 { return b.Lo + (b.Hi-b.Lo)/2 }

// Bin counts vs into n equal-width buckets over [lo, hi].
//
// Pass lo >= hi to take the interval from the data, and n <= 0 to let
// [Sturges] choose the count. Values outside the interval are not counted:
// a histogram over an explicit range is a statement about that range, and
// silently folding the tails into the end buckets would misreport both.
func Bin(vs []float64, lo, hi float64, n int) []Bucket {
	return AppendBin(nil, vs, lo, hi, n)
}

// AppendBin is [Bin] writing into a caller-owned slice. dst is truncated
// first, so a chart redrawn every frame bins into the same memory.
func AppendBin(dst []Bucket, vs []float64, lo, hi float64, n int) []Bucket {
	dst = dst[:0]
	if lo >= hi {
		var ok bool
		lo, hi, ok = finiteExtent(vs)
		if !ok {
			return dst
		}
	}
	if n <= 0 {
		n = Sturges(countFinite(vs))
	}
	if n <= 0 || !finite(lo) || !finite(hi) {
		return dst
	}
	// A column with one distinct value has no width to divide. Giving it one
	// bucket a unit wide is the only reading that draws anything, and it says
	// the true thing: every observation is here.
	if lo == hi {
		pad := math.Abs(lo) * 0.05
		if pad == 0 {
			pad = 0.5
		}
		lo, hi, n = lo-pad, hi+pad, 1
	}

	w := (hi - lo) / float64(n)
	for i := range n {
		dst = append(dst, Bucket{Lo: lo + float64(i)*w, Hi: lo + float64(i+1)*w})
	}
	// The last bucket's upper edge is the interval's own, not the accumulated
	// one: n additions of a rounded width do not land back on hi, and a
	// histogram whose last bar stops a rounding error short of the maximum
	// loses whichever observations sit in the gap.
	dst[n-1].Hi = hi

	for _, v := range vs {
		if !finite(v) || v < lo || v > hi {
			continue
		}
		i := int((v - lo) / w)
		dst[min(max(i, 0), n-1)].Count++
	}
	return dst
}

// WeightedBucket is one bin of a weighted histogram: the interval it covers
// and the sum of the weights that fell in it.
//
// It is a type of its own rather than a float Count on [Bucket] because a
// count is a number of observations and a sum of weights is not: a volume
// profile's bar is shares traded, and a caller who reads Count expecting rows
// should not get shares.
type WeightedBucket struct {
	Lo, Hi float64
	Sum    float64
}

// Mid is the bucket's centre, where a histogram bar is positioned.
func (b WeightedBucket) Mid() float64 { return b.Lo + (b.Hi-b.Lo)/2 }

// BinWeighted is [Bin] summing ws rather than counting vs: each value adds its
// weight to the bucket it falls in.
//
// It is the volume profile's summary — prices binned, volumes summed — and the
// buckets are exactly Bin's, over the same interval and with the same edges,
// so two calls over the same lo, hi and n (the buys, then the sells) give two
// profiles whose bars line up. Pass lo >= hi to take the interval from vs, as
// Bin does; a caller splitting one column into two should take it once, from
// the whole column, and pass it to both, or the two halves bin differently.
//
// A value is skipped when it or its weight is not finite. n <= 0 chooses the
// count by [Sturges] over the values, as Bin does. The shorter column wins.
func BinWeighted(vs, ws []float64, lo, hi float64, n int) []WeightedBucket {
	return AppendBinWeighted(nil, vs, ws, lo, hi, n)
}

// AppendBinWeighted is [BinWeighted] writing into a caller-owned slice. dst is
// truncated first.
func AppendBinWeighted(dst []WeightedBucket, vs, ws []float64, lo, hi float64, n int) []WeightedBucket {
	dst = dst[:0]
	m := min(len(vs), len(ws))
	vs, ws = vs[:m], ws[:m]
	if lo >= hi {
		var ok bool
		lo, hi, ok = finiteExtent(vs)
		if !ok {
			return dst
		}
	}
	if n <= 0 {
		n = Sturges(countFinite(vs))
	}
	if n <= 0 || !finite(lo) || !finite(hi) {
		return dst
	}
	if lo == hi {
		pad := math.Abs(lo) * 0.05
		if pad == 0 {
			pad = 0.5
		}
		lo, hi, n = lo-pad, hi+pad, 1
	}

	w := (hi - lo) / float64(n)
	for i := range n {
		dst = append(dst, WeightedBucket{Lo: lo + float64(i)*w, Hi: lo + float64(i+1)*w})
	}
	dst[n-1].Hi = hi

	for i, v := range vs {
		if !finite(v) || !finite(ws[i]) || v < lo || v > hi {
			continue
		}
		j := int((v - lo) / w)
		dst[min(max(j, 0), n-1)].Sum += ws[i]
	}
	return dst
}

func countFinite(vs []float64) int {
	n := 0
	for _, v := range vs {
		if finite(v) {
			n++
		}
	}
	return n
}

// Sturges returns the bin count Sturges's rule chooses for n observations:
// ceil(log2 n) + 1.
//
// It is the default because it needs nothing but the count — no sort, no
// spread — so a histogram redrawn every frame costs a pass over the column
// rather than a copy of it. It assumes roughly normal data and under-bins a
// large or a skewed sample; [FreedmanDiaconis] is the answer to that, and it
// is a separate function because it asks the caller for a sorted column.
func Sturges(n int) int {
	if n <= 1 {
		return 1
	}
	return int(math.Ceil(math.Log2(float64(n)))) + 1
}

// FreedmanDiaconis returns the bin count the Freedman–Diaconis rule chooses
// for an ascending column: the interval divided by 2·IQR·n^(-1/3).
//
// It reports 0 when the rule has no answer — fewer than two values, or an
// interquartile range of zero, which is what a column of mostly one value has.
// A caller falls back to [Sturges] there rather than dividing by nothing.
//
// The column must be sorted ascending, for the reason [Quantile] is: sorting
// here would mean either mutating the caller's data or allocating a copy of it
// on every frame.
func FreedmanDiaconis(sorted []float64) int {
	n := len(sorted)
	if n < 2 {
		return 0
	}
	iqr := Quantile(sorted, 0.75) - Quantile(sorted, 0.25)
	if !finite(iqr) || iqr <= 0 {
		return 0
	}
	w := 2 * iqr / math.Cbrt(float64(n))
	span := sorted[n-1] - sorted[0]
	if w <= 0 || !finite(span) || span <= 0 {
		return 0
	}
	return max(int(math.Ceil(span/w)), 1)
}

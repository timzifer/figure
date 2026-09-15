package stat

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

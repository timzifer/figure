package stat

// MovingAverage smooths ys with a centred running mean of the given width.
//
// It is the smoother everyone already knows how to read, which is most of its
// argument: a reader who is told a line is a seven-day average knows exactly
// what was done to it, and a reader told it is a loess fit usually does not.
// What it buys with that is bluntness — the mean of a window is the same
// whichever way the data leans inside it, so a peak comes back shorter than it
// was and a step comes back as a ramp. Where the shape of a peak is the point,
// [SavitzkyGolay] keeps it and this does not.
//
// # What the columns must be
//
// Same as [Loess]: xs ascending, both columns finite, the same length — the
// shorter one wins if they are not. See that function for why the contract is
// narrower here than in the rest of this package.
//
// The window is a count of rows, forced odd so that it has a middle, and
// clamped to the column. Pass window <= 0 for [DefaultWindow] of the rows.
//
// The ends are averaged over the part of the window that exists rather than
// padded or dropped. Padding would invent readings; dropping would end the
// line short of the data it describes, which is where a reader looks first.
func MovingAverage(xs, ys []float64, window int) []Point {
	return AppendMovingAverage(nil, xs, ys, window)
}

// DefaultWindow is the fraction of the rows one window spans when the caller
// names no width. It is [DefaultSpan]'s sibling and deliberately smaller: a
// running mean over three quarters of the data is a horizontal line.
const DefaultWindow = 0.1

// AppendMovingAverage is [MovingAverage] writing into a caller-owned slice.
// dst is truncated first, so a chart redrawn every frame fits into the same
// memory.
//
// The sum is carried from one window to the next rather than recomputed, so a
// smoothing of a long column is linear in it and independent of the width.
func AppendMovingAverage(dst []Point, xs, ys []float64, window int) []Point {
	dst = dst[:0]
	n := min(len(xs), len(ys))
	if n == 0 {
		return dst
	}
	half := halfWindow(window, n)

	// The running sum covers [lo, hi). Both ends only ever move forward, which
	// is what keeps this one pass over the column.
	lo, hi, sum := 0, 0, 0.0
	for i := range n {
		want := min(i+half+1, n)
		for ; hi < want; hi++ {
			sum += ys[hi]
		}
		for ; lo < i-half; lo++ {
			sum -= ys[lo]
		}
		dst = append(dst, Point{X: xs[i], Y: sum / float64(hi-lo)})
	}
	return dst
}

// halfWindow turns a caller's width into the number of rows on each side of
// the middle one. A window of one row is the data unchanged, which is a
// smoothing nobody asked for but is not wrong, so it is allowed.
func halfWindow(window, n int) int {
	if window <= 0 {
		window = int(float64(n) * DefaultWindow)
	}
	if window < 1 {
		window = 1
	}
	if window > n {
		window = n
	}
	return (window - 1) / 2
}

package stat

import "math"

// The trailing windows.
//
// [MovingAverage] is centred: the value at a row is the mean of rows on both
// sides of it, which is right for a trend drawn after the fact and wrong for a
// quantity that is read at the right-hand edge of a live chart. A centred mean
// at the last row knows nothing of the row that has not arrived, so it moves
// when that row does — the line the reader acted on is not the line they see
// a minute later. Everything here looks only backwards: the value at a row is
// a function of that row and the ones before it, and appending a row never
// changes a value already drawn. That is the property a market indicator is
// defined by, and it is why these are functions of their own rather than an
// option on the centred one.
//
// All of them take the contract [Loess] takes — xs ascending, both columns
// finite, the shorter one wins — and all of them come in pairs with an Append
// form. The window is a count of rows, clamped to the column; pass window <= 0
// for [DefaultWindow] of the rows. The first rows have fewer predecessors than
// the window and are computed over the part of it that exists, for the reason
// MovingAverage gives for its ends: dropping them would end the line short of
// the data, and a chart with an indicator that starts twenty rows in reads as a
// chart with twenty rows missing.

// TrailingMean is the simple moving average of a market chart: at each row,
// the mean of that row and the window-1 rows before it.
func TrailingMean(xs, ys []float64, window int) []Point {
	return AppendTrailingMean(nil, xs, ys, window)
}

// AppendTrailingMean is [TrailingMean] writing into a caller-owned slice. dst
// is truncated first.
//
// The sum is carried from one row to the next, as in [AppendMovingAverage].
func AppendTrailingMean(dst []Point, xs, ys []float64, window int) []Point {
	dst = dst[:0]
	n := min(len(xs), len(ys))
	if n == 0 {
		return dst
	}
	w := trailingWindow(window, n)
	sum := 0.0
	for i := range n {
		sum += ys[i]
		if i >= w {
			sum -= ys[i-w]
		}
		dst = append(dst, Point{X: xs[i], Y: sum / float64(min(i+1, w))})
	}
	return dst
}

// EMA is the exponential moving average over a window of rows: each row moves
// the average a fraction 2/(window+1) of the way towards itself.
//
// That fraction is the market's convention rather than a statistician's, and
// it is the one a reader told "the 20-day EMA" is expecting: it gives the
// average the same centre of mass as a 20-row simple mean. The average starts
// at the first row's value rather than at the mean of the first window, so it
// has a value from the first row on, and the few rows where that start still
// shows are the rows where every other definition of the start disagrees too.
//
// The weights never reach zero, so unlike the trailing mean the window does not
// bound how far back a row is felt — only how quickly it fades.
func EMA(xs, ys []float64, window int) []Point {
	return AppendEMA(nil, xs, ys, window)
}

// AppendEMA is [EMA] writing into a caller-owned slice. dst is truncated first.
func AppendEMA(dst []Point, xs, ys []float64, window int) []Point {
	dst = dst[:0]
	n := min(len(xs), len(ys))
	if n == 0 {
		return dst
	}
	alpha := 2 / (float64(trailingWindow(window, n)) + 1)
	avg := ys[0]
	for i := range n {
		avg += alpha * (ys[i] - avg)
		dst = append(dst, Point{X: xs[i], Y: avg})
	}
	return dst
}

// RollingStdDev is the population standard deviation of each trailing window:
// the spread a Bollinger band is drawn at a multiple of.
//
// Population rather than sample — divided by the window rather than by one less
// — because that is how the band is defined, and a band that is a few percent
// wider than the reader's own platform draws it is a band nobody trusts. The
// whole-column [StdDev] is the sample form, for the reason its own comment
// gives; the two answer different questions.
//
// A band is two more columns rather than a function here:
//
//	mid := stat.TrailingMean(xs, ys, 20)
//	sd := stat.RollingStdDev(xs, ys, 20)
//	// lower[i] = mid[i].Y - 2*sd[i].Y, upper[i] = mid[i].Y + 2*sd[i].Y
//
// drawn with [github.com/timzifer/figure/geom.Area] between Y and Y2.
func RollingStdDev(xs, ys []float64, window int) []Point {
	return AppendRollingStdDev(nil, xs, ys, window)
}

// AppendRollingStdDev is [RollingStdDev] writing into a caller-owned slice. dst
// is truncated first.
//
// Each window is summed twice — once for its mean, once for the squared
// distances from it — rather than carried as a running sum of squares. That
// makes it linear in the window as well as the column, and it is the price of
// the two-pass form [StdDev] explains: a price column is large values with a
// small spread, which is exactly where the running identity cancels. An
// indicator window is tens of rows, not thousands.
func AppendRollingStdDev(dst []Point, xs, ys []float64, window int) []Point {
	dst = dst[:0]
	n := min(len(xs), len(ys))
	if n == 0 {
		return dst
	}
	w := trailingWindow(window, n)
	for i := range n {
		lo := max(i-w+1, 0)
		k := float64(i - lo + 1)
		mean := 0.0
		for _, v := range ys[lo : i+1] {
			mean += v
		}
		mean /= k
		sum := 0.0
		for _, v := range ys[lo : i+1] {
			d := v - mean
			sum += d * d
		}
		dst = append(dst, Point{X: xs[i], Y: math.Sqrt(sum / k)})
	}
	return dst
}

// RollingMin is the least value of each trailing window: the lower edge of a
// Donchian channel, and the floor a trailing stop is set from.
func RollingMin(xs, ys []float64, window int) []Point {
	return AppendRollingMin(nil, xs, ys, window)
}

// AppendRollingMin is [RollingMin] writing into a caller-owned slice. dst is
// truncated first.
func AppendRollingMin(dst []Point, xs, ys []float64, window int) []Point {
	return appendRollingExtreme(dst, xs, ys, window, func(a, b float64) bool { return a < b })
}

// RollingMax is the greatest value of each trailing window: the upper edge of
// a Donchian channel.
func RollingMax(xs, ys []float64, window int) []Point {
	return AppendRollingMax(nil, xs, ys, window)
}

// AppendRollingMax is [RollingMax] writing into a caller-owned slice. dst is
// truncated first.
func AppendRollingMax(dst []Point, xs, ys []float64, window int) []Point {
	return appendRollingExtreme(dst, xs, ys, window, func(a, b float64) bool { return a > b })
}

// appendRollingExtreme carries the extreme and where it was, and rescans the
// window only when that row falls out of it.
//
// The textbook answer is a deque of candidate rows, which is linear in the
// worst case and needs a buffer the size of the window — an allocation per
// call, or a type with a Reset to hold it. Carrying one row is allocation-free
// and linear on the data a chart actually has: a rescan happens only when the
// extreme is the oldest row, which on a price column is rare. The worst case —
// a column that falls for its whole length, under RollingMax — is linear in the
// window times the column, which for an indicator window is a small constant.
func appendRollingExtreme(dst []Point, xs, ys []float64, window int, better func(a, b float64) bool) []Point {
	dst = dst[:0]
	n := min(len(xs), len(ys))
	if n == 0 {
		return dst
	}
	w := trailingWindow(window, n)
	at := 0
	for i := range n {
		lo := max(i-w+1, 0)
		switch {
		case at < lo:
			at = lo
			for j := lo + 1; j <= i; j++ {
				if !better(ys[at], ys[j]) {
					at = j
				}
			}
		case !better(ys[at], ys[i]):
			// Ties move to the newer row, so the carried row is the last to
			// leave the window of all the rows that share its value.
			at = i
		}
		dst = append(dst, Point{X: xs[i], Y: ys[at]})
	}
	return dst
}

// Cumsum is the running total of ys: at each row, the sum of that row and every
// one before it.
//
// It is what turns the orders at each price into the depth of a book — the
// volume a market order of a given size would walk through — and returns into
// a growth curve. It is here rather than left to a loop because the Append form
// is the one a chart redrawn every frame wants, and because that loop is the
// one written with a running float in the wrong order often enough: this sums
// from the first row, so the last value is the column's total.
func Cumsum(xs, ys []float64) []Point {
	return AppendCumsum(nil, xs, ys)
}

// AppendCumsum is [Cumsum] writing into a caller-owned slice. dst is truncated
// first.
func AppendCumsum(dst []Point, xs, ys []float64) []Point {
	dst = dst[:0]
	n := min(len(xs), len(ys))
	sum := 0.0
	for i := range n {
		sum += ys[i]
		dst = append(dst, Point{X: xs[i], Y: sum})
	}
	return dst
}

// trailingWindow turns a caller's width into a count of rows. Unlike the
// centred [halfWindow] it need not be odd: a trailing window has no middle.
func trailingWindow(window, n int) int {
	if window <= 0 {
		window = int(float64(n) * DefaultWindow)
	}
	return min(max(window, 1), n)
}

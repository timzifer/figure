package stat

// MaxSavGolOrder is the highest polynomial degree [SavitzkyGolay] fits. Three
// is where the useful range stops: a quartic through a window of a dozen rows
// is fitting the noise it was called to remove.
const MaxSavGolOrder = 3

// DefaultSavGolOrder is the degree fitted when the caller names none. A
// quadratic is the usual choice because it is the lowest degree with a
// curvature, and curvature is the whole difference between this smoother and
// [MovingAverage].
const DefaultSavGolOrder = 2

// SavitzkyGolay smooths ys by fitting a low-degree polynomial to the rows
// around each one by least squares and taking that polynomial's value there.
//
// It is the smoother that keeps the shape of a peak. A running mean flattens a
// maximum, because the mean of a window straddling it is pulled down by both
// flanks; a quadratic fitted to the same window has a maximum of its own and
// lands near the real one. That is what it is for — spectra, chromatograms,
// any trace read for the height and width of its features.
//
// It is *not* a filter that removes a trend: a polynomial of degree d
// reproduces any polynomial of degree d exactly, so a parabola smoothed at
// order two comes back unchanged, noise and all where the noise is itself
// smooth.
//
// # What the columns must be
//
// Same as [Loess]: xs ascending, both columns finite, the same length — the
// shorter one wins if they are not. Unlike the classical tabulated form this
// does not require the rows to be evenly spaced; it fits against the real
// abscissae, which is the same thing where they are even and the honest answer
// where they are not.
//
// The window is a count of rows, forced odd, clamped to the column, and at
// least order+1 wide — a fit with fewer points than coefficients is not a fit.
// Pass window <= 0 for [DefaultWindow] of the rows and order <= 0 for
// [DefaultSavGolOrder]; order is clamped to [MaxSavGolOrder].
//
// At the ends the window is the part of it that exists, and the polynomial is
// evaluated at the row's own abscissa rather than at the middle of the window.
// That is the standard treatment, and it is why the fit reaches the first and
// last row instead of stopping short of them.
func SavitzkyGolay(xs, ys []float64, window, order int) []Point {
	return AppendSavitzkyGolay(nil, xs, ys, window, order)
}

// AppendSavitzkyGolay is [SavitzkyGolay] writing into a caller-owned slice.
// dst is truncated first, so a chart redrawn every frame fits into the same
// memory.
//
// The normal equations are accumulated into a fixed-size array and solved in
// place, so a smoothing of any length allocates nothing beyond dst.
func AppendSavitzkyGolay(dst []Point, xs, ys []float64, window, order int) []Point {
	dst = dst[:0]
	n := min(len(xs), len(ys))
	if n == 0 {
		return dst
	}
	if order <= 0 {
		order = DefaultSavGolOrder
	}
	order = min(order, MaxSavGolOrder)
	half := halfWindow(window, n)
	if w := 2*half + 1; w < order+1 {
		// Widen rather than drop the degree: the caller asked for a curvature,
		// and the cheapest way to have one is to look at more rows.
		half = order / 2
	}

	for i := range n {
		lo, hi := max(i-half, 0), min(i+half+1, n)
		dst = append(dst, Point{X: xs[i], Y: savgolAt(xs, ys, lo, hi, i, order)})
	}
	return dst
}

// savgolAt fits the polynomial over rows [lo, hi) and returns its value at
// xs[i].
//
// The abscissae are centred on xs[i] before the moments are taken, which does
// two things: it conditions the normal equations, whose powers would otherwise
// be of the raw x and overflow a float64 on a time axis, and it makes the
// answer the constant coefficient — so the back substitution only has to
// reach one of them.
func savgolAt(xs, ys []float64, lo, hi, i, order int) float64 {
	k := order + 1
	if hi-lo < k {
		order, k = hi-lo-1, hi-lo
	}
	if k < 1 {
		return ys[i]
	}

	// aug is the normal equations [AᵀA | Aᵀy], which for a polynomial basis is
	// built entirely from the power sums of the centred abscissae.
	var aug [MaxSavGolOrder + 1][MaxSavGolOrder + 2]float64
	var pow [2*MaxSavGolOrder + 1]float64
	var rhs [MaxSavGolOrder + 1]float64
	for j := lo; j < hi; j++ {
		d := xs[j] - xs[i]
		p := 1.0
		for e := range 2*order + 1 {
			pow[e] += p
			if e <= order {
				rhs[e] += p * ys[j]
			}
			p *= d
		}
	}
	for r := range k {
		for c := range k {
			aug[r][c] = pow[r+c]
		}
		aug[r][k] = rhs[r]
	}
	c0, ok := solveForConstant(&aug, k)
	if !ok {
		// A singular system means the window has fewer distinct abscissae than
		// coefficients — duplicated x values. The row's own reading is the
		// only answer that invents nothing.
		return ys[i]
	}
	return c0
}

// solveForConstant solves the k-by-k augmented system by Gaussian elimination
// with partial pivoting and returns the first unknown, which is the fitted
// polynomial's constant term.
func solveForConstant(aug *[MaxSavGolOrder + 1][MaxSavGolOrder + 2]float64, k int) (float64, bool) {
	for c := range k {
		p := c
		for r := c + 1; r < k; r++ {
			if abs(aug[r][c]) > abs(aug[p][c]) {
				p = r
			}
		}
		if abs(aug[p][c]) < 1e-12 {
			return 0, false
		}
		aug[c], aug[p] = aug[p], aug[c]
		for r := c + 1; r < k; r++ {
			f := aug[r][c] / aug[c][c]
			for j := c; j <= k; j++ {
				aug[r][j] -= f * aug[c][j]
			}
		}
	}
	// Back substitution, all the way down to the constant term.
	var x [MaxSavGolOrder + 1]float64
	for r := k - 1; r >= 0; r-- {
		s := aug[r][k]
		for j := r + 1; j < k; j++ {
			s -= aug[r][j] * x[j]
		}
		x[r] = s / aug[r][r]
	}
	return x[0], true
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

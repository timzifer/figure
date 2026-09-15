package stat

import "math"

// Step reports the constant spacing of a sorted axis, and whether it has one.
//
// It is the question an image asks of a lattice that a contour and a surface do
// not: a pixel grid has equal cells by construction, so a field sampled at 1 s,
// 2 s and then 10 s cannot be blitted without either stretching the wrong cells
// or inventing the missing ones. A mark that draws one box per row — the rect —
// draws that table correctly, so the honest answer here is no rather than an
// approximation. See docs/adr/0066-a-raster-mark.md.
//
// vs is an axis of [Lattice]: sorted, distinct, and at least two values long.
func Step(vs []float64) (float64, bool) {
	n := len(vs)
	if n < 2 {
		return 0, false
	}
	step := (vs[n-1] - vs[0]) / float64(n-1)
	if !finite(step) || step <= 0 {
		return 0, false
	}
	for i := 1; i < n; i++ {
		if math.Abs(vs[i]-vs[i-1]-step) > stepTolerance(vs, step) {
			return 0, false
		}
	}
	return step, true
}

// stepTolerance is how far one gap may be from the mean gap and still count as
// the same step.
//
// It is the larger of a millionth of the step and the precision float64 has
// left at the axis's own magnitude, because a lattice of timestamps is even in
// the clock's units and not in the machine's: a minute either side of
// 1.7e9 seconds is a number whose last bits are already spent, and an axis
// rejected for that would be an axis rejected for being large. Either way it
// is orders of magnitude tighter than the unevenness this exists to catch —
// a sample at 10 s where the grid says 3 s is out by a factor, not by an ulp.
func stepTolerance(vs []float64, step float64) float64 {
	mag := math.Max(math.Abs(vs[0]), math.Abs(vs[len(vs)-1]))
	return math.Max(step*1e-6, mag*1e-12)
}

// Block is a rectangle of lattice cells: the columns [X0, X1) by the rows
// [Y0, Y1), in the index space of [Lattice.Xs] and [Lattice.Ys].
//
// It is what one pixel of a panel covers, which is anything from part of a
// single cell to a few hundred of them. The reduction from many cells to the
// one value a pixel can show is named by the caller rather than assumed —
// [Lattice.Mean] and [Lattice.Max] are the two that are arithmetic, and taking
// the nearest cell is a block of one.
type Block struct{ X0, X1, Y0, Y1 int }

// Mean is the mean of the finite cells of b, or NaN when it holds none.
//
// A cell that is not a number is skipped rather than counted as a zero, which
// is [Lattice]'s rule for a hole: nothing was measured there, and a mean that
// counted it would report a reading nobody took.
func (l *Lattice) Mean(b Block) float64 {
	sum, n := 0.0, 0
	l.each(b, func(v float64) {
		sum, n = sum+v, n+1
	})
	if n == 0 {
		return math.NaN()
	}
	return sum / float64(n)
}

// Max is the largest finite cell of b, or NaN when it holds none.
//
// It is the reduction a spectrum wants: a peak one bin wide survives a
// downscale that a mean would average away, which is the difference between a
// spectrogram that shows a tone and one that shows a smear.
func (l *Lattice) Max(b Block) float64 {
	out, ok := math.Inf(-1), false
	l.each(b, func(v float64) {
		out, ok = math.Max(out, v), true
	})
	if !ok {
		return math.NaN()
	}
	return out
}

// each calls f with every finite cell of b, clamped to the lattice.
func (l *Lattice) each(b Block, f func(v float64)) {
	nx, ny := len(l.Xs), len(l.Ys)
	x0, x1 := max(b.X0, 0), min(b.X1, nx)
	y0, y1 := max(b.Y0, 0), min(b.Y1, ny)
	if x1 <= x0 || y1 <= y0 {
		// A block off the lattice, or one the caller left empty. Clamping it
		// leaves the two ends crossed, and a crossed range is no cells rather
		// than a slice taken backwards.
		return
	}
	for j := y0; j < y1; j++ {
		row := l.V[j*nx : (j+1)*nx]
		for _, v := range row[x0:x1] {
			if finite(v) {
				f(v)
			}
		}
	}
}

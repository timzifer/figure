package ir

import (
	"math"
	"sync"
)

// maxHatchElements caps how much geometry one hatch pass may emit.
//
// A hatch is measured in device units and a mark is measured in data, so
// nothing stops a chart from asking for a five-unit pattern over a shape ten
// thousand units wide. Without a cap that is two thousand strokes for one bar,
// and a pattern that dense is a solid block anyway — so past the cap the pass
// draws nothing and the mark keeps its plain fill, which is the same answer
// [HatchPath] gives for a pattern it does not recognise.
const maxHatchElements = 4096

// HatchPath appends the geometry of h covering bounds and reports whether that
// geometry is to be filled rather than stroked.
//
// The pattern is phased on the origin rather than on bounds, so two marks side
// by side continue one rhythm instead of each starting its own — which is what
// makes a stacked bar read as one hatched column rather than as a ladder of
// misaligned ones.
//
// Nothing is appended for [HatchNone], for a spacing that is zero or less, or
// for a Hatch this release does not know. That last case is the point: a
// backend never sees a Hatch, and a chart written against a later release
// draws its marks plainly here rather than wrongly.
func HatchPath(dst *Path, h Hatching, bounds Rect) (fill bool) {
	s := h.Spacing
	if dst == nil || s <= 0 || bounds.Empty() {
		return false
	}
	switch h.Hatch {
	case HatchDiagonal:
		hatchDiagonal(dst, bounds, s, +1)
	case HatchBackDiagonal:
		hatchDiagonal(dst, bounds, s, -1)
	case HatchCross:
		hatchDiagonal(dst, bounds, s, +1)
		hatchDiagonal(dst, bounds, s, -1)
	case HatchHorizontal:
		hatchHorizontal(dst, bounds, s)
	case HatchVertical:
		hatchVertical(dst, bounds, s)
	case HatchGrid:
		hatchHorizontal(dst, bounds, s)
		hatchVertical(dst, bounds, s)
	case HatchDots:
		hatchDots(dst, bounds, s, h.Line.Width, false)
		return true
	case HatchDotsStaggered:
		hatchDots(dst, bounds, s, h.Line.Width, true)
		return true
	case HatchZigzag:
		hatchZigzag(dst, bounds, s)
	case HatchWave:
		hatchWave(dst, bounds, s)
	case HatchBrick:
		hatchBrick(dst, bounds, s)
	case HatchTriangles:
		hatchTriangles(dst, bounds, s)
		return true
	case HatchScales:
		hatchScales(dst, bounds, s)
	}
	return false
}

// firstStep is the largest multiple of step not greater than lo. It is what
// phases a pattern on the origin instead of on the shape being filled.
func firstStep(lo, step float32) float32 {
	return float32(math.Floor(float64(lo/step))) * step
}

// stepCount reports how many rungs of the given step fit between lo and hi, and
// whether that is few enough to draw. See [maxHatchElements].
func stepCount(lo, hi, step float32) (int, bool) {
	n := int((hi-lo)/step) + 2
	return n, n > 0 && n <= maxHatchElements
}

// odd reports whether v sits on an odd rung of step, which is how every
// staggered pattern decides that a row is offset.
func odd(v, step float32) bool {
	return int(math.Round(float64(v/step)))%2 != 0
}

// hatchDiagonal appends lines at 45 degrees, sloping down-right for dir +1 and
// up-right for dir -1.
//
// Each line is cut to the box rather than to its top and bottom edges alone.
// The lines are drawn clipped to the mark, so an overshoot would be invisible
// — but it would also be written down: a tall narrow bar hatched to its full
// height emits segments as long as the bar is tall and as far outside it, and
// an SVG of a hundred bars then carries a hundred times that. Two comparisons
// per line is cheaper than the ink they save.
func hatchDiagonal(dst *Path, b Rect, s, dir float32) {
	// Lines of constant x + dir*y. Their perpendicular separation is the step
	// divided by the root of two, so the step along the constant is the
	// spacing times it.
	step := s * math.Sqrt2
	var lo, hi float32
	if dir > 0 {
		lo, hi = b.Min.X+b.Min.Y, b.Max.X+b.Max.Y
	} else {
		lo, hi = b.Min.X-b.Max.Y, b.Max.X-b.Min.Y
	}
	n, ok := stepCount(lo, hi, step)
	if !ok {
		return
	}
	dst.Grow(2*n, 2*n)
	for c := firstStep(lo, step); c <= hi; c += step {
		// Where the line enters and leaves the box, in y. The x it has there
		// follows from the constant.
		y0, y1 := b.Min.Y, b.Max.Y
		if dir > 0 {
			y0, y1 = max(y0, c-b.Max.X), min(y1, c-b.Min.X)
		} else {
			y0, y1 = max(y0, b.Min.X-c), min(y1, b.Max.X-c)
		}
		if y1 <= y0 {
			continue
		}
		dst.MoveTo(c-dir*y0, y0).LineTo(c-dir*y1, y1)
	}
}

func hatchHorizontal(dst *Path, b Rect, s float32) {
	n, ok := stepCount(b.Min.Y, b.Max.Y, s)
	if !ok {
		return
	}
	dst.Grow(2*n, 2*n)
	for y := firstStep(b.Min.Y, s); y <= b.Max.Y; y += s {
		dst.MoveTo(b.Min.X, y).LineTo(b.Max.X, y)
	}
}

func hatchVertical(dst *Path, b Rect, s float32) {
	n, ok := stepCount(b.Min.X, b.Max.X, s)
	if !ok {
		return
	}
	dst.Grow(2*n, 2*n)
	for x := firstStep(b.Min.X, s); x <= b.Max.X; x += s {
		dst.MoveTo(x, b.Min.Y).LineTo(x, b.Max.Y)
	}
}

// hatchDots appends a dot per grid point. The dot's radius is the hatching's
// line width, so that thickening a hatch thickens every family of them.
func hatchDots(dst *Path, b Rect, s, w float32, stagger bool) {
	if w <= 0 {
		return
	}
	rows, okR := stepCount(b.Min.Y, b.Max.Y, s)
	cols, okC := stepCount(b.Min.X, b.Max.X, s)
	if !okR || !okC || rows*cols > maxHatchElements {
		return
	}
	dst.Grow(6*rows*cols, 13*rows*cols)
	for y := firstStep(b.Min.Y, s); y <= b.Max.Y; y += s {
		off := float32(0)
		if stagger && odd(y, s) {
			off = s / 2
		}
		for x := firstStep(b.Min.X-off, s) + off; x <= b.Max.X; x += s {
			dst.Circle(Point{X: x, Y: y}, w)
		}
	}
}

// hatchZigzag appends a folded line per row: the same rhythm as a horizontal
// hatch, told apart from it by shape rather than by slope.
func hatchZigzag(dst *Path, b Rect, s float32) {
	rows, okR := stepCount(b.Min.Y, b.Max.Y, s)
	cols, okC := stepCount(b.Min.X, b.Max.X, s/2)
	if !okR || !okC || rows*cols > maxHatchElements {
		return
	}
	amp, half := s/4, s/2
	dst.Grow(rows*(cols+1), rows*(cols+1))
	for y := firstStep(b.Min.Y, s); y <= b.Max.Y; y += s {
		x := firstStep(b.Min.X, half)
		up := !odd(x, half)
		dst.MoveTo(x, y+fold(up, amp))
		for x += half; x <= b.Max.X+half; x += half {
			up = !up
			dst.LineTo(x, y+fold(up, amp))
		}
	}
}

func fold(up bool, amp float32) float32 {
	if up {
		return -amp
	}
	return amp
}

// hatchWave is hatchZigzag with the folds rounded off. One cubic per half
// period, its controls a third of the way along — near enough to a sine that
// the difference is under a tenth of a pixel at any spacing a chart uses.
func hatchWave(dst *Path, b Rect, s float32) {
	rows, okR := stepCount(b.Min.Y, b.Max.Y, s)
	cols, okC := stepCount(b.Min.X, b.Max.X, s/2)
	if !okR || !okC || rows*cols > maxHatchElements {
		return
	}
	amp, half := s/4, s/2
	dst.Grow(rows*(cols+1), 3*rows*(cols+1))
	for y := firstStep(b.Min.Y, s); y <= b.Max.Y; y += s {
		x0 := firstStep(b.Min.X, half)
		up := !odd(x0, half)
		dst.MoveTo(x0, y+fold(up, amp))
		for ; x0 <= b.Max.X+half; x0 += half {
			y0 := y + fold(up, amp)
			up = !up
			x1, y1 := x0+half, y+fold(up, amp)
			dst.CubicTo(x0+half/3, y0, x1-half/3, y1, x1, y1)
		}
	}
}

// hatchBrick appends a running bond: a course every step, and a head joint
// every two steps, offset by one step on alternate courses.
func hatchBrick(dst *Path, b Rect, s float32) {
	rows, okR := stepCount(b.Min.Y, b.Max.Y, s)
	cols, okC := stepCount(b.Min.X, b.Max.X, 2*s)
	if !okR || !okC || rows*cols > maxHatchElements {
		return
	}
	hatchHorizontal(dst, b, s)
	dst.Grow(2*rows*cols, 2*rows*cols)
	for y := firstStep(b.Min.Y, s); y <= b.Max.Y; y += s {
		off := float32(0)
		if odd(y, s) {
			off = s
		}
		for x := firstStep(b.Min.X-off, 2*s) + off; x <= b.Max.X; x += 2 * s {
			dst.MoveTo(x, y).LineTo(x, y+s)
		}
	}
}

// hatchTriangles appends a filled triangle per staggered grid point. Filled
// rather than stroked because a hollow triangle at chart sizes is three lines
// that read as a smudge.
func hatchTriangles(dst *Path, b Rect, s float32) {
	rows, okR := stepCount(b.Min.Y, b.Max.Y, s)
	cols, okC := stepCount(b.Min.X, b.Max.X, s)
	if !okR || !okC || rows*cols > maxHatchElements {
		return
	}
	hw, hh := s*0.25, s*0.22
	dst.Grow(4*rows*cols, 3*rows*cols)
	for y := firstStep(b.Min.Y, s); y <= b.Max.Y; y += s {
		off := float32(0)
		if odd(y, s) {
			off = s / 2
		}
		for x := firstStep(b.Min.X-off, s) + off; x <= b.Max.X; x += s {
			dst.MoveTo(x, y-hh).LineTo(x+hw, y+hh).LineTo(x-hw, y+hh).Close()
		}
	}
}

// hatchScales appends rows of semicircular arcs, each odd row offset by half a
// step — the fish-scale tiling. Two cubics per arc, for the reason
// [Path.Circle] uses four.
func hatchScales(dst *Path, b Rect, s float32) {
	rows, okR := stepCount(b.Min.Y, b.Max.Y, s)
	cols, okC := stepCount(b.Min.X, b.Max.X, s)
	if !okR || !okC || rows*cols > maxHatchElements {
		return
	}
	r := s / 2
	k := r * kappa
	dst.Grow(3*rows*cols, 7*rows*cols)
	for y := firstStep(b.Min.Y, s); y <= b.Max.Y+s; y += s {
		off := float32(0)
		if odd(y, s) {
			off = r
		}
		for x := firstStep(b.Min.X-off, s) + off; x <= b.Max.X+s; x += s {
			dst.MoveTo(x-r, y).
				CubicTo(x-r, y-k, x-k, y-r, x, y-r).
				CubicTo(x+k, y-r, x+r, y-k, x+r, y)
		}
	}
}

// hatchScratch holds the geometry of one hatch pass between calls.
//
// A pool rather than a package variable because figure builds IR on several
// goroutines — a faceted chart records its panels in parallel — and two panels
// hatching at once would otherwise write over each other's lines.
var hatchScratch = sync.Pool{New: func() any { return new(Path) }}

// FillHatched fills p and then lays h over it, clipped to p.
//
// It is one function rather than the same six lines at every call site, and it
// is the only thing that knows a hatch is drawn rather than declared: the
// [Backend] interface has no pattern in it and never gains one, so a hatch
// reaches every backend — including one written before hatching existed, and
// including the GPU tier, which flattens a brush to a single colour — as the
// ordinary stroke and fill calls it is made of. See docs/adr/0069.
//
// The hatch is bracketed in [Decoration] where the backend implements one, so
// that a hit-test index counts the mark and not the lines inside it.
func FillHatched(b Backend, p *Path, f Fill, rule FillRule, h Hatching) {
	if b == nil || p == nil || p.Empty() {
		return
	}
	if f.Visible() {
		b.FillPath(p, f, rule)
	}
	if !h.Visible() {
		return
	}
	sp, _ := hatchScratch.Get().(*Path)
	sp.Reset()
	solid := HatchPath(sp, h, p.Bounds())
	if !sp.Empty() {
		decorate(b, func() {
			b.Push(p, Identity)
			if solid {
				b.FillPath(sp, Solid(h.Line.Color), NonZero)
			} else {
				b.StrokePath(sp, h.Line)
			}
			b.Pop()
		})
	}
	hatchScratch.Put(sp)
}

// FillInset fills p and then draws an inner border just inside its edge.
//
// The border is edge stroked at twice its width and clipped to p, which is
// exactly an inset outline and needs no path offsetting — offsetting an
// arbitrary path is a hard problem with no answer at a self-intersection, and
// this has none. It is what gives touching areas an edge each: a treemap, a
// stacked bar, a sankey band, where an ordinary outline would thicken the
// silhouette of the whole layer rather than divide it.
func FillInset(b Backend, p *Path, f Fill, rule FillRule, edge Stroke) {
	if b == nil || p == nil || p.Empty() {
		return
	}
	if f.Visible() {
		b.FillPath(p, f, rule)
	}
	if !edge.Visible() {
		return
	}
	inner := edge
	inner.Width = edge.Width * 2
	decorate(b, func() {
		b.Push(p, Identity)
		b.StrokePath(p, inner)
		b.Pop()
	})
}

// decorate runs fn inside a [Decoration] bracket where b keeps one, and plainly
// where it does not.
func decorate(b Backend, fn func()) {
	d, ok := b.(Decoration)
	if ok {
		d.BeginDecoration()
	}
	fn()
	if ok {
		d.EndDecoration()
	}
}

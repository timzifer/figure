package coord

import (
	"math"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// BreakMark is how a break on an axis is marked. A fold is always marked the
// same way, with a [BreakSlash] at its own size, whatever this says: a zigzag
// across the panel per fold would be a hatch.
type BreakMark uint8

const (
	// BreakSlash marks a break with two short parallel strokes across the
	// axis line, one at each edge of the gap — the // of a hand-drawn axis.
	BreakSlash BreakMark = iota
	// BreakZigzag runs a zigzag across the whole panel along each edge of
	// the gap, inside it, so that a bar crossing the break reads as cut.
	BreakZigzag
)

// Breakable is implemented by a coord that can draw an axis with intervals
// left out of it: one that cuts its clip, splits its axis and grid lines at
// the gaps, and marks them. It is an optional interface, like [Exploder].
//
// Only [Cartesian] implements it. A break is a claim that the distance along
// an axis stops meaning what it means everywhere else, and a coord that
// cannot mark where is a coord that would draw the claim without saying so;
// under one the renderer switches every break off, and the axis is drawn
// whole. See docs/adr/0083-an-axis-break-is-marked-or-not-drawn.md.
type Breakable interface {
	// MarksBreaks reports whether the coord marks the breaks on its axes.
	MarksBreaks() bool
}

// MarksBreaks reports whether c draws an axis break: true only for a coord
// that implements [Breakable] and says so.
func MarksBreaks(c Coord) bool {
	b, ok := c.(Breakable)
	return ok && b.MarksBreaks()
}

func (cartesian) MarksBreaks() bool { return true }

// minGap is the narrowest gap the clip is cut for. A fold squeezed to nothing
// still has its mark; cutting the clip for it would only put a seam in the
// raster.
const minGap = 0.5

// axisGaps is the gaps of one scale in device order, ascending by position
// rather than by value — on a Y axis and a reversed one the two run opposite
// ways. A nil breaker has none.
type axisGaps struct {
	b    scale.Breaker
	n    int
	desc bool
}

func gapsOf(s scale.Scale) axisGaps {
	b, ok := s.(scale.Breaker)
	if !ok {
		return axisGaps{}
	}
	g := axisGaps{b: b, n: b.Gaps()}
	if g.n > 0 {
		lo, hi, _ := b.Gap(0)
		g.desc = hi < lo
	}
	return g
}

// at is the k-th gap in device order, smaller end first, and which side the
// lower value lies on: +1 when the lower side of the axis is at a.
func (g axisGaps) at(k int) (a, z float32, fold bool) {
	i := k
	if g.desc {
		i = g.n - 1 - k
	}
	lo, hi, fold := g.b.Gap(i)
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi, fold
}

// pieces calls fn with each stretch of [lo, hi] between the gaps, in device
// order, skipping gaps too narrow to cut.
func (g axisGaps) pieces(lo, hi float32, fn func(a, z float32)) {
	a := lo
	for k := 0; k < g.n; k++ {
		ga, gz, _ := g.at(k)
		if gz-ga < minGap || gz <= lo || ga >= hi {
			continue
		}
		fn(a, ga)
		a = gz
	}
	fn(a, hi)
}

// Clip is the panel rectangle cut into one rectangle per pair of kept
// pieces, so that whatever falls into a gap is not drawn. With no gaps it is
// the rectangle itself, exactly as [cartesian.Clip] writes it — which is what
// lets a raster backend keep treating it as a scissor.
func (f framedCartesian) Clip(p *ir.Path, area ir.Rect) {
	xg, yg := gapsOf(f.x), gapsOf(f.y)
	if xg.n == 0 && yg.n == 0 {
		p.Rect(area)
		return
	}
	xg.pieces(area.Min.X, area.Max.X, func(x0, x1 float32) {
		yg.pieces(area.Min.Y, area.Max.Y, func(y0, y1 float32) {
			p.Rect(ir.Rect{Min: ir.Point{X: x0, Y: y0}, Max: ir.Point{X: x1, Y: y1}})
		})
	})
}

// Furniture is [cartesian.Furniture] with the axis and grid lines split at
// the gaps and the gaps marked. With no gaps it is exactly the unframed
// coord's, and nothing about a chart without breaks moves.
func (f framedCartesian) Furniture(dst *Furniture, req FurnitureRequest) {
	f.cartesian.Furniture(dst, req)
	xg, yg := gapsOf(f.x), gapsOf(f.y)
	if xg.n == 0 && yg.n == 0 {
		return
	}
	area, m := req.Area, req.Metrics
	if xg.n > 0 {
		splitAlong(&dst.AxisX, xg, area.Min.X, area.Max.X, true)
		for i := range dst.GridY {
			splitAlong(&dst.GridY[i], xg, area.Min.X, area.Max.X, true)
		}
	}
	if yg.n > 0 {
		splitAlong(&dst.AxisY, yg, area.Min.Y, area.Max.Y, false)
		for i := range dst.GridX {
			splitAlong(&dst.GridX[i], yg, area.Min.Y, area.Max.Y, false)
		}
	}
	markGaps(&dst.Breaks, xg, area, m, true)
	markGaps(&dst.Breaks, yg, area, m, false)
}

// splitAlong rewrites a straight two-point shape running along one direction
// as one subpath per kept piece. A shape that is empty, or not a straight
// run, is left alone.
func splitAlong(s *Shape, g axisGaps, lo, hi float32, horizontal bool) {
	if len(s.Pts) != 2 {
		return
	}
	a, b := s.Pts[0], s.Pts[1]
	s.Pts = s.Pts[:0]
	s.Path.Reset()
	g.pieces(lo, hi, func(p0, p1 float32) {
		if horizontal {
			s.Path.MoveTo(p0, a.Y).LineTo(p1, b.Y)
		} else {
			s.Path.MoveTo(a.X, p0).LineTo(b.X, p1)
		}
	})
}

// markGaps appends the marks of one axis's gaps to dst. Marks nearer each
// other than their own size would print as a smudge, so a run of folds that
// close together is marked once.
func markGaps(dst *Shape, g axisGaps, area ir.Rect, m Metrics, horizontal bool) {
	last := float32(math.Inf(-1))
	for k := 0; k < g.n; k++ {
		a, z, fold := g.at(k)
		size := m.BreakSize
		if fold {
			size = m.FoldSize
		}
		if size <= 0 {
			continue
		}
		if mid := (a + z) / 2; mid-last < size {
			continue
		} else {
			last = mid
		}
		if !fold && m.BreakMark == BreakZigzag && z-a >= 1 {
			zigzag(&dst.Path, a, z, size, area, horizontal)
			continue
		}
		slash(&dst.Path, a, z, size, area, horizontal)
	}
}

// slash draws // across the axis line: one stroke at each edge of the gap,
// spread apart when the gap is narrower than a stroke is thick to read.
func slash(p *ir.Path, a, z, size float32, area ir.Rect, horizontal bool) {
	if min := size * 0.4; z-a < min {
		mid := (a + z) / 2
		a, z = mid-min/2, mid+min/2
	}
	lean, reach := size*0.3, size*0.5
	for _, e := range [2]float32{a, z} {
		if horizontal {
			// Across the X axis along the bottom edge: leaning right.
			y := area.Max.Y
			p.MoveTo(e-lean, y+reach).LineTo(e+lean, y-reach)
		} else {
			// Across the Y axis up the left edge, leaning the same way.
			x := area.Min.X
			p.MoveTo(x-reach, e+lean).LineTo(x+reach, e-lean)
		}
	}
}

// zigzag runs two parallel zigzags from one side of the panel to the other,
// one along each edge of the gap and folded into it, so that together they
// cover nothing that was not already hidden and read as the torn edges of
// one cut.
func zigzag(p *ir.Path, a, z, size float32, area ir.Rect, horizontal bool) {
	amp := size / 2
	if third := (z - a) / 3; amp > third {
		amp = third
	}
	half := size / 2
	// The first line runs between a and a+amp, the second is the same line
	// moved up to finish at z: in phase, so the two never cross.
	for _, base := range [2]float32{a, z - amp} {
		lo, hi := area.Min.Y, area.Max.Y
		if !horizontal {
			lo, hi = area.Min.X, area.Max.X
		}
		up := false
		for t := lo; ; t += half {
			if t > hi {
				t = hi
			}
			v := base
			if up {
				v += amp
			}
			if horizontal {
				// A vertical zigzag across a gap in the X axis.
				if t == lo {
					p.MoveTo(v, t)
				} else {
					p.LineTo(v, t)
				}
			} else if t == lo {
				p.MoveTo(t, v)
			} else {
				p.LineTo(t, v)
			}
			up = !up
			if t >= hi {
				break
			}
		}
	}
}

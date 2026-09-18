package geom

import (
	"math"
	"strconv"

	"github.com/timzifer/figure/ir"
)

// How a label on a curve is chosen. All three are constants rather than
// options for [stat.LayeredSweeps]'s reason: a picture whose labels moved when
// a relaxation settled differently would not be a pure function of its input.
const (
	// curveLabelTries is how many positions along one run are considered.
	curveLabelTries = 9
	// curveLabelSag is how far the curve may wander from the chord under the
	// label, in font heights. Text is straight and a contour is not, so the
	// question is how much bend a straight run can sit on before it reads as
	// crossed out.
	curveLabelSag = 0.35
	// curveLabelPad is the clear space at each end of the gap, in font
	// heights. The gap is what tells the reader the label belongs to this line
	// rather than to the one beside it, so it has to be visible.
	curveLabelPad = 0.45
	// curveLabelRoom is how much longer than its label a run must be to carry
	// one. A ring mostly made of gap is a worse picture than a ring with no
	// number on it.
	curveLabelRoom = 2.2
)

// curveLabeller places one label along a device-space run and gaps the run for
// it. It is shared by [Contour] and [Locus], which have no data model in
// common: by the time a run reaches here it is device points and nothing else,
// which is docs/adr/0073-labels-on-a-curve.md's first claim.
//
// The two halves the run becomes are built into buffers the layer keeps, so a
// chart redrawn every frame allocates neither.
type curveLabeller struct {
	head, tail []ir.Point
	// sags scores the candidates. It is an array rather than a slice so that
	// scoring one run touches no heap at all.
	sags  [curveLabelTries]float64
	tried [curveLabelTries]bool
}

// place chooses where run.Text sits on pts and returns the run to draw.
//
// On success l.head and l.tail are the curve with the label's gap cut out of
// it, either of which may be too short to stroke. On failure the caller strokes
// pts unchanged: a run with no room for its label keeps its line.
//
// placer is [Frame.Labels] and may be nil, which is what a caller driving a
// geom directly gets — the label is then placed by the curve alone, exactly as
// a text layer with no placer keeps its own anchor.
func (l *curveLabeller) place(m ir.Measurer, placer LabelPlacer, pts []ir.Point, run ir.TextRun) (ir.TextRun, bool) {
	if len(pts) < 2 || run.Text == "" {
		return run, false
	}
	tm := m.Measure(run)
	height := float64(tm.Height())
	if height <= 0 {
		return run, false
	}
	width := float64(tm.Advance) + 2*curveLabelPad*height
	total := polylineLength(pts)
	if width <= 0 || total < width*curveLabelRoom {
		return run, false
	}

	// The candidates: evenly spread centres, each scored by how far the curve
	// under the label wanders from the chord across it. A bend the text cannot
	// sit on is not a candidate at all.
	span := total - width
	for i := range l.sags {
		centre := width/2 + span*float64(i)/float64(curveLabelTries-1)
		l.sags[i], l.tried[i] = l.sagAt(pts, centre, width), false
	}

	run.H, run.V = ir.AlignCenter, ir.AlignMiddle
	for range l.sags {
		best := -1
		for i, sag := range l.sags {
			// Ties go to the earlier candidate, which is the order the run's
			// own points have.
			if l.tried[i] || sag > curveLabelSag*height {
				continue
			}
			if best < 0 || sag < l.sags[best] {
				best = i
			}
		}
		if best < 0 {
			return run, false
		}
		l.tried[best] = true

		centre := width/2 + span*float64(best)/float64(curveLabelTries-1)
		i0, p0 := pointAlong(pts, centre-width/2)
		i1, p1 := pointAlong(pts, centre+width/2)
		run.At = ir.Point{X: (p0.X + p1.X) / 2, Y: (p0.Y + p1.Y) / 2}
		run.Rotation = uprightAngle(p0, p1)
		if placer != nil {
			// move is false: a curve label nudged off its curve names a level
			// it is not on. The placer may refuse it and may not move it.
			at, ok := placer.PlaceLabel(run, false)
			if !ok {
				continue
			}
			run.At = at
		}
		l.head = append(append(l.head[:0], pts[:i0+1]...), p0)
		l.tail = append(append(l.tail[:0], p1), pts[i1+1:]...)
		return run, true
	}
	return run, false
}

// sagAt is how far the run wanders from the chord under a label centred at
// centre, in device units.
func (l *curveLabeller) sagAt(pts []ir.Point, centre, width float64) float64 {
	i0, p0 := pointAlong(pts, centre-width/2)
	i1, p1 := pointAlong(pts, centre+width/2)
	dx, dy := float64(p1.X-p0.X), float64(p1.Y-p0.Y)
	chord := math.Hypot(dx, dy)
	if chord == 0 {
		return math.Inf(1)
	}
	worst := 0.0
	lo, hi := i0+1, min(i1+1, len(pts))
	if lo > hi {
		// Both ends fall in one segment: the curve under the label is that
		// segment, and a segment does not sag.
		return 0
	}
	for _, p := range pts[lo:hi] {
		// The perpendicular distance from the chord, which is the cross
		// product over its length.
		d := math.Abs(dx*float64(p.Y-p0.Y)-dy*float64(p.X-p0.X)) / chord
		worst = max(worst, d)
	}
	return worst
}

// uprightAngle is the chord's direction, turned a further half turn when it
// points leftwards so that the text reads left to right whichever way the curve
// was traced.
func uprightAngle(p0, p1 ir.Point) float64 {
	a := math.Atan2(float64(p1.Y-p0.Y), float64(p1.X-p0.X))
	if a > math.Pi/2 {
		return a - math.Pi
	}
	if a < -math.Pi/2 {
		return a + math.Pi
	}
	return a
}

// polylineLength is the run's length in device units.
func polylineLength(pts []ir.Point) float64 {
	total := 0.0
	for i := 1; i < len(pts); i++ {
		total += math.Hypot(float64(pts[i].X-pts[i-1].X), float64(pts[i].Y-pts[i-1].Y))
	}
	return total
}

// pointAlong is the point at arc length s along the run, and the index of the
// sample it follows.
//
// It interpolates rather than snapping to the nearest sample because a contour
// traced over a coarse lattice has segments longer than a label: snapping would
// gap half the ring to write three characters on it.
func pointAlong(pts []ir.Point, s float64) (int, ir.Point) {
	if s <= 0 {
		return 0, pts[0]
	}
	run := 0.0
	for i := 1; i < len(pts); i++ {
		seg := math.Hypot(float64(pts[i].X-pts[i-1].X), float64(pts[i].Y-pts[i-1].Y))
		if run+seg >= s && seg > 0 {
			t := float32((s - run) / seg)
			return i - 1, ir.Point{
				X: pts[i-1].X + t*(pts[i].X-pts[i-1].X),
				Y: pts[i-1].Y + t*(pts[i].Y-pts[i-1].Y),
			}
		}
		run += seg
	}
	return len(pts) - 2, pts[len(pts)-1]
}

// formatLevel is what a level is written as when the caller named no format.
//
// The shortest decimal that reads back as the same float64, which is what a
// level chosen by [github.com/timzifer/figure/stat.Levels] is: a round step, and
// a round step written out in full is a number a reader can do arithmetic with.
func formatLevel(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// levelText is the string a labelled mark writes for one level.
func (c config) levelText(v float64) string {
	if c.levelFormat != nil {
		return c.levelFormat(v)
	}
	return formatLevel(v)
}

// levelTexts writes the levels once, into a buffer the layer keeps.
//
// Once rather than per frame: formatting a number allocates the string it
// returns, and a chart redrawn per pointer move would allocate one per label
// per frame for text that cannot have changed — the levels are settled before
// anything is drawn.
func (c config) levelTexts(dst []string, levels []float64) []string {
	dst = dst[:0]
	for _, v := range levels {
		dst = append(dst, c.levelText(v))
	}
	return dst
}

// levelTextAt is the written form of one level, found by value because a
// traced run names the level it is at rather than its place in the list.
func levelTextAt(levels []float64, texts []string, v float64) string {
	for i, l := range levels {
		if l == v && i < len(texts) {
			return texts[i]
		}
	}
	return ""
}

// labelFont is the font a curve label is written in: the layer's size, or the
// theme's label size.
func (c config) labelFont(f Frame) ir.FontRef {
	size := c.fontSize
	if size <= 0 {
		size = f.Theme.LabelSize
	}
	return f.Theme.Font(size)
}

package scale

import (
	"math"
	"sort"
	"time"
)

// Interval is a closed stretch of a numeric domain, Lo to Hi.
type Interval struct{ Lo, Hi float64 }

// TimeSpan is a stretch of time, From to To.
type TimeSpan struct{ From, To time.Time }

// Break leaves the interval lo to hi out of a linear axis. The values either
// side of it are drawn a small gap apart, and the gap is marked, so that a
// chart whose one tall bar would flatten everything else can show both.
//
// A break may be given more than once; overlapping and touching intervals
// merge. A break is only drawn where it is strictly inside the domain, so a
// zoom that moves one off the axis simply stops showing it.
//
// Nothing is removed from the data. A value inside the break maps into the
// gap and the panel's clip hides it, which is why a bar from 0 to 95 across a
// break from 10 to 90 is still one bar — drawn with its middle missing — and
// why [Scale.Invert] is exact everywhere. A break under a coord that cannot
// mark one is not drawn at all: the axis is unbroken, because a break without
// its mark is a chart that lies about its distances. See
// docs/adr/0083-an-axis-break-is-marked-or-not-drawn.md.
func Break(lo, hi float64) LinearOption {
	return func(l *linear) { l.cuts = l.cuts.with(false, Interval{lo, hi}) }
}

// Fold leaves each interval out of a linear axis the way [Break] does, marked
// as a fold rather than a break: a narrow gap and a small slash on the axis
// line, never a zigzag across the panel. It is for many stretches taken out
// of one axis because the caller does not want to look at them, rather than
// for one interval taken out because the data would not fit.
func Fold(ivs ...Interval) LinearOption {
	return func(l *linear) { l.cuts = l.cuts.with(true, ivs...) }
}

// TimeBreak leaves the stretch from to to out of a time axis. It is [Break]
// for time.
func TimeBreak(from, to time.Time) TimeOption {
	return func(s *timeScale) { s.spans = append(s.spans, timeCut{TimeSpan{from, to}, false}) }
}

// TimeFold leaves each span out of a time axis the way [Fold] does: the
// periods a machine stood idle, the nights of a trading week. See
// [github.com/timzifer/figure.SpansWhere] for reading them out of a table.
func TimeFold(spans ...TimeSpan) TimeOption {
	return func(s *timeScale) {
		for _, sp := range spans {
			s.spans = append(s.spans, timeCut{sp, true})
		}
	}
}

// timeCut is a span waiting for the scale's origin: [Origin] may be given
// after the fold, and a domain value means nothing until the origin is known.
type timeCut struct {
	span TimeSpan
	fold bool
}

// Breaker is implemented by a scale whose axis can leave intervals out. It is
// an optional interface, for the reason [Zoomer] is.
//
// The scale knows which intervals; it does not know how wide a gap is, because
// that is a length on the page and a scale has no theme. The renderer tells it
// once per render, before anything is mapped, exactly as it tells a size scale
// its range — and tells it off when the coord cannot mark a break, so that no
// axis is ever broken without saying so.
type Breaker interface {
	// SetBreakGap sets the device length a break and a fold leave between the
	// two sides of the axis. A negative brk switches every cut off, and the
	// scale maps as though it had none.
	SetBreakGap(brk, fold float32)

	// Gaps reports how many cuts are drawn on the axis at its current domain
	// and range, and Gap the device interval of the i-th, ascending by value,
	// and whether it is a fold. lo is where the lower side of the axis ends;
	// on a Y axis or a reversed one it is the larger number.
	Gaps() int
	Gap(i int) (lo, hi float32, fold bool)
}

// cut is one interval left out, and whether it is a fold rather than a break.
type cut struct {
	lo, hi float64
	fold   bool
}

// cuts is an axis's intervals, ascending and disjoint, with running sums over
// them so that a lookup is a binary search rather than a walk. It is built
// once, at construction, and never written afterwards, which is why a Clone
// and a Snapshot share it.
type cuts struct {
	c []cut
	// width[i] is the total width of c[:i], and folds[i] how many of c[:i]
	// are folds. Both have len(c)+1 entries.
	width []float64
	folds []int
}

// with returns the set with ivs added, sorted and merged. A break that
// absorbs a fold stays a break: the fold was the smaller claim.
func (cs cuts) with(fold bool, ivs ...Interval) cuts {
	all := make([]cut, 0, len(cs.c)+len(ivs))
	all = append(all, cs.c...)
	for _, iv := range ivs {
		lo, hi := order(iv.Lo, iv.Hi)
		if math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) || !(lo < hi) {
			continue
		}
		all = append(all, cut{lo, hi, fold})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].lo < all[j].lo })
	out := all[:0]
	for _, c := range all {
		if n := len(out); n > 0 && c.lo <= out[n-1].hi {
			last := &out[n-1]
			last.hi = math.Max(last.hi, c.hi)
			last.fold = last.fold && c.fold
			continue
		}
		out = append(out, c)
	}
	r := cuts{c: out, width: make([]float64, len(out)+1), folds: make([]int, len(out)+1)}
	for i, c := range out {
		r.width[i+1] = r.width[i] + (c.hi - c.lo)
		r.folds[i+1] = r.folds[i]
		if c.fold {
			r.folds[i+1]++
		}
	}
	return r
}

func (cs cuts) intervals(fold bool) []Interval {
	var out []Interval
	for _, c := range cs.c {
		if c.fold == fold {
			out = append(out, Interval{c.lo, c.hi})
		}
	}
	return out
}

// gaps is what the renderer told the scale. on is false until it has: a scale
// used anywhere but under a coord that marks breaks maps as an unbroken one.
type gaps struct {
	brk, fold float32
	on        bool
}

func (g *gaps) set(brk, fold float32) {
	if brk < 0 || math.IsNaN(float64(brk)) {
		*g = gaps{}
		return
	}
	if !(fold >= 0) {
		fold = 0
	}
	*g = gaps{brk: brk, fold: fold, on: true}
}

// broken is an axis's cuts resolved against one domain and one device range.
// It lives on the stack of the call that needs it: nothing about it is kept,
// so a panel on another goroutine can never see another panel's.
type broken struct {
	cs       *cuts
	i0, i1   int // the active cuts, cs.c[i0:i1]
	lo, hi   float64
	rlo, rhi float32
	sign     float64 // +1 when the range ascends, -1 when it descends
	unit     float64 // device length per unit of kept domain, positive
	brk, fld float64 // the gap a break and a fold leave, positive
}

// resolve answers false when the axis draws as though it had no cuts: none
// are inside the domain, the renderer never switched them on, or the range is
// too short to spend on gaps.
func (cs *cuts) resolve(g gaps, lo, hi float64, rlo, rhi float32) (broken, bool) {
	if !g.on || len(cs.c) == 0 || !(lo < hi) || rlo == rhi {
		return broken{}, false
	}
	// A cut that reaches an end of the domain would be a gap with nothing on
	// its far side, so it trims the axis instead: the domain ends where the
	// cut begins. That is what folding the last idle hour of a shift has to
	// mean, and it distorts no distance, so it needs no mark.
	trimmed := false
	if j := sort.Search(len(cs.c), func(i int) bool { return cs.c[i].hi > lo }); j < len(cs.c) && cs.c[j].lo <= lo {
		lo, trimmed = cs.c[j].hi, true
	}
	if k := sort.Search(len(cs.c), func(i int) bool { return cs.c[i].lo >= hi }) - 1; k >= 0 && cs.c[k].hi >= hi {
		hi, trimmed = cs.c[k].lo, true
	}
	if !(lo < hi) {
		return broken{}, false
	}
	// What is left is strictly inside the domain.
	i0 := sort.Search(len(cs.c), func(i int) bool { return cs.c[i].lo > lo })
	i1 := sort.Search(len(cs.c), func(i int) bool { return cs.c[i].hi >= hi })
	if i1 < i0 {
		i1 = i0
	}
	if i1 == i0 && !trimmed {
		return broken{}, false
	}
	nf := cs.folds[i1] - cs.folds[i0]
	nb := (i1 - i0) - nf
	span := math.Abs(float64(rhi) - float64(rlo))
	brk, fld := float64(g.brk), float64(g.fold)
	// Gaps may take at most half the axis. Folds give their room up first —
	// a fold with no gap is still folded and still marked — and a break that
	// still does not fit is not drawn, because a squeezed break is a picture
	// of nothing.
	if float64(nb)*brk+float64(nf)*fld > span/2 {
		fld = 0
		if float64(nb)*brk > span/2 {
			return broken{}, false
		}
	}
	kept := (hi - lo) - (cs.width[i1] - cs.width[i0])
	if !(kept > 0) {
		return broken{}, false
	}
	b := broken{
		cs: cs, i0: i0, i1: i1, lo: lo, hi: hi, rlo: rlo, rhi: rhi,
		sign: 1, brk: brk, fld: fld,
	}
	if rhi < rlo {
		b.sign = -1
	}
	b.unit = (span - float64(float64(nb)*brk) - float64(float64(nf)*fld)) / kept
	return b, true
}

func (b *broken) gapOf(i int) float64 {
	if b.cs.c[i].fold {
		return b.fld
	}
	return b.brk
}

// offset is how far along the axis, in device units and from rlo, the lower
// edge of cut i lies. The products are rounded explicitly so that no machine
// fuses them into the sums: see [place].
func (b *broken) offset(i int) float64 {
	cs := b.cs
	kept := (cs.c[i].lo - b.lo) - (cs.width[i] - cs.width[b.i0])
	nf := cs.folds[i] - cs.folds[b.i0]
	nb := (i - b.i0) - nf
	return float64(kept*b.unit) + float64(float64(nb)*b.brk) + float64(float64(nf)*b.fld)
}

func (b *broken) at(off float64) float32 { return b.rlo + float32(b.sign*off) }

// edges is the device interval of cut i, lower side first.
func (b *broken) edges(i int) (float32, float32) {
	off := b.offset(i)
	return b.at(off), b.at(off + b.gapOf(i))
}

// segment is the stretch of domain v falls in — a kept piece or a cut — and
// the device interval it maps onto. The ends of every segment are computed one
// way whichever value asked, so the two sides of a gap meet the gap exactly.
func (b *broken) segment(v float64) (a, z float64, pa, pz float32) {
	cs := b.cs
	// j is the first active cut not wholly below v.
	j := b.i0 + sort.Search(b.i1-b.i0, func(k int) bool { return cs.c[b.i0+k].hi >= v })
	if j < b.i1 && v >= cs.c[j].lo {
		pa, pz = b.edges(j)
		return cs.c[j].lo, cs.c[j].hi, pa, pz
	}
	a, pa = b.lo, b.rlo
	if j > b.i0 {
		a = cs.c[j-1].hi
		_, pa = b.edges(j - 1)
	}
	z, pz = b.hi, b.rhi
	if j < b.i1 {
		z = cs.c[j].lo
		pz, _ = b.edges(j)
	}
	return a, z, pa, pz
}

func (b *broken) mapTo(v float64) float32 {
	a, z, pa, pz := b.segment(v)
	switch v {
	case a:
		return pa
	case z:
		return pz
	}
	return place(pa, pz, (v-a)/(z-a))
}

func (b *broken) map64(v float64) float64 {
	a, z, pa, pz := b.segment(v)
	return place64(pa, pz, (v-a)/(z-a))
}

func (b *broken) invert(pos float32) float64 {
	cs := b.cs
	o := b.sign * (float64(pos) - float64(b.rlo))
	// j is the first active cut whose upper edge is not below o.
	j := b.i0 + sort.Search(b.i1-b.i0, func(k int) bool {
		i := b.i0 + k
		return b.offset(i)+b.gapOf(i) >= o
	})
	if j < b.i1 {
		if s := b.offset(j); o >= s {
			g := b.gapOf(j)
			if g == 0 {
				return cs.c[j].lo
			}
			return cs.c[j].lo + (o-s)/g*(cs.c[j].hi-cs.c[j].lo)
		}
	}
	a, oa := b.lo, 0.0
	if j > b.i0 {
		a, oa = cs.c[j-1].hi, b.offset(j-1)+b.gapOf(j-1)
	}
	return a + (o-oa)/b.unit
}

// pieces calls fn with each kept stretch of the domain, ascending.
func (b *broken) pieces(fn func(a, z float64)) {
	a := b.lo
	for i := b.i0; i < b.i1; i++ {
		fn(a, b.cs.c[i].lo)
		a = b.cs.c[i].hi
	}
	fn(a, b.hi)
}

// kept is the length of domain the axis still shows.
func (b *broken) kept() float64 {
	return (b.hi - b.lo) - (b.cs.width[b.i1] - b.cs.width[b.i0])
}

// inside reports whether v falls strictly inside one of the active cuts,
// where no tick may stand.
func (b *broken) inside(v float64) bool {
	cs := b.cs
	j := b.i0 + sort.Search(b.i1-b.i0, func(k int) bool { return cs.c[b.i0+k].hi > v })
	return j < b.i1 && v > cs.c[j].lo
}

// brokenValues is the tick sequence of a broken axis: one step, chosen for the
// length of domain the axis still shows, walked through each kept piece. One
// step is what keeps the numbers either side of a break reading as one axis,
// and walking the pieces rather than the whole domain is what keeps an axis
// with hundreds of folds as cheap as one with a single break.
func brokenValues(b *broken, want int) ([]float64, float64) {
	step := extendedWilkinson(0, b.kept(), want, false).step
	if !(step > 0) {
		return nil, 0
	}
	var out []float64
	const guard = 4096
	b.pieces(func(a, z float64) {
		eps := 1e-9 * math.Max(math.Abs(a), math.Abs(z))
		for k := math.Ceil((a - eps) / step); len(out) < guard; k++ {
			v := k * step
			if v > z+eps {
				break
			}
			if v == 0 {
				v = 0 // no negative zero on an axis
			}
			out = append(out, v)
		}
	})
	return out, step
}

// The implementations of [Breaker] share a shape: the cuts are resolved
// against the scale's own domain and device range, every time, so that a pan
// and a resize need nothing invalidated.

func (l *linear) SetBreakGap(brk, fold float32) { l.gaps.set(brk, fold) }

func (l *linear) broken() (broken, bool) {
	lo, hi := l.effective()
	rlo, rhi := l.device()
	return l.cuts.resolve(l.gaps, lo, hi, rlo, rhi)
}

func (l *linear) Gaps() int {
	b, ok := l.broken()
	if !ok {
		return 0
	}
	return b.i1 - b.i0
}

func (l *linear) Gap(i int) (float32, float32, bool) {
	b, ok := l.broken()
	if !ok || i < 0 || i >= b.i1-b.i0 {
		return 0, 0, false
	}
	lo, hi := b.edges(b.i0 + i)
	return lo, hi, b.cs.c[b.i0+i].fold
}

func (s *timeScale) SetBreakGap(brk, fold float32) { s.gaps.set(brk, fold) }

func (s *timeScale) broken() (broken, bool) {
	lo, hi := s.span()
	rlo, rhi := s.rangeOf()
	return s.cuts.resolve(s.gaps, lo, hi, rlo, rhi)
}

func (s *timeScale) Gaps() int {
	b, ok := s.broken()
	if !ok {
		return 0
	}
	return b.i1 - b.i0
}

func (s *timeScale) Gap(i int) (float32, float32, bool) {
	b, ok := s.broken()
	if !ok || i < 0 || i >= b.i1-b.i0 {
		return 0, 0, false
	}
	lo, hi := b.edges(b.i0 + i)
	return lo, hi, b.cs.c[b.i0+i].fold
}

var (
	_ Breaker = (*linear)(nil)
	_ Breaker = (*timeScale)(nil)
)

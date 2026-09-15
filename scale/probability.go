package scale

import (
	"math"
	"sort"
)

// Link is a monotone increasing map from a probability in (0, 1) onto the real
// line. It is what a [Probability] scale positions a value by: equal distances
// on the axis are equal distances in the link's output.
//
// Four links are named here — [Probit], [Logit], [CLogLog] and [Gumbel] — and
// a caller may pass one of their own. The named ones round-trip through a
// [Desc] as their name; one written in Go has no name and does not, which is
// the rule ADR 0041 set for a quantile function and ADR 0052 repeats for this.
type Link interface {
	// Apply maps a probability onto the real line. It is only asked about
	// values strictly between 0 and 1.
	Apply(p float64) float64
	// Unapply is the inverse of Apply.
	Unapply(z float64) float64
}

// The named links.
var (
	// Probit is Φ⁻¹, the inverse standard normal distribution function. A
	// normal sample's cumulative fractions plot as a straight line on it:
	// normal probability paper.
	Probit Link = namedLink(linkProbit)
	// Logit is ln(p/(1−p)), the log-odds. A logistic dose–response curve is a
	// straight line on it.
	Logit Link = namedLink(linkLogit)
	// CLogLog is ln(−ln(1−p)), the complementary log-log. Against a log X it
	// is Weibull paper: a Weibull sample plots as a line whose slope is the
	// shape parameter β, and which crosses 63.2 % at the characteristic life.
	CLogLog Link = namedLink(linkCLogLog)
	// Gumbel is −ln(−ln p), the largest-extreme-value link. An annual maximum
	// — a flood, a load, a gust — plots as a line on it.
	Gumbel Link = namedLink(linkGumbel)
)

// namedLink is a link this package can write down. The set is closed: see
// [LinkNamed].
type namedLink string

const (
	linkProbit  = "probit"
	linkLogit   = "logit"
	linkCLogLog = "cloglog"
	linkGumbel  = "gumbel"
)

func (l namedLink) Apply(p float64) float64 {
	switch l {
	case linkLogit:
		return math.Log(p) - math.Log1p(-p)
	case linkCLogLog:
		return math.Log(-math.Log1p(-p))
	case linkGumbel:
		return -math.Log(-math.Log(p))
	}
	return math.Sqrt2 * math.Erfinv(2*p-1)
}

func (l namedLink) Unapply(z float64) float64 {
	switch l {
	case linkLogit:
		return 1 / (1 + math.Exp(-z))
	case linkCLogLog:
		return -math.Expm1(-math.Exp(z))
	case linkGumbel:
		return math.Exp(-math.Exp(-z))
	}
	return 0.5 * math.Erfc(-z/math.Sqrt2)
}

// LinkName reports the name l is written down under, or ok == false for a link
// this package did not define.
func LinkName(l Link) (string, bool) {
	n, ok := l.(namedLink)
	return string(n), ok
}

// LinkNamed is the link written down under name, or ok == false for a name
// this package has no link for.
//
// The set is closed on purpose. A name that resolved to something this package
// cannot rebuild would make a document that does not round-trip.
func LinkNamed(name string) (Link, bool) {
	switch name {
	case linkProbit, linkLogit, linkCLogLog, linkGumbel:
		return namedLink(name), true
	}
	return nil, false
}

// ProbabilityOption configures a probability scale.
type ProbabilityOption func(*probScale)

// ProbabilityDomain pins the data domain explicitly, disabling training. Both
// bounds must lie strictly between 0 and 1 and be in order; any other pair is
// ignored, since the scale has no position for 0 or 1.
func ProbabilityDomain(min, max float64) ProbabilityOption {
	return func(s *probScale) {
		if !(min > 0 && max < 1 && min < max) {
			return
		}
		s.fixed = true
		s.dmin, s.dmax, s.trained = min, max, true
	}
}

// ProbabilityFormat overrides tick label formatting.
func ProbabilityFormat(fn func(v float64) string) ProbabilityOption {
	return func(s *probScale) { s.format = fn }
}

// ProbabilityNumberFormat is [NumberFormat] for a probability scale. Without
// one the axis writes per cent, because that is how the ladder is read: a spec
// replaces that, so "#" writes the bare fraction.
func ProbabilityNumberFormat(spec string) ProbabilityOption {
	f := mustFormat(spec)
	return func(s *probScale) { s.numFormat = f }
}

// ProbabilityMinorTicks turns the unlabelled rungs of the ladder on or off.
// They are on by default: a rung the axis had no room to label still tells a
// reader how far apart the labelled ones are.
func ProbabilityMinorTicks(show bool) ProbabilityOption {
	return func(s *probScale) { s.minor = show }
}

// Probability returns a probability scale: an axis warped by link, so that
// the cumulative distribution the link belongs to plots as a straight line.
//
// It is probability paper, and every mark can be drawn on it. [ECDF] on a
// [Probit] Y is a normal probability plot; on a [CLogLog] Y against a [Log] X
// it is a Weibull plot; on a [Gumbel] Y it is extreme-value paper. A nil link
// is [Probit].
//
// The domain is the open interval (0, 1). Training ignores 0, 1 and everything
// outside them, [Scale.Map] returns NaN for such a value, and geoms treat it as
// missing under the layer's own policy — the relationship [Log] has with zero.
//
// There is no Nice: there is no round number to round a probability to. The
// ticks are the conventional ladder, 0.1 / 1 / 5 / 10 / 20 / 30 / 50 / 70 /
// 80 / 90 / 95 / 99 / 99.9 per cent, extended a decade at a time into whichever
// tail the domain reaches, clipped to the domain and thinned symmetrically
// about the median when the axis is short.
//
// [ECDF]: https://pkg.go.dev/github.com/timzifer/figure/geom#ECDF
func Probability(link Link, opts ...ProbabilityOption) Scale {
	if link == nil {
		link = Probit
	}
	s := &probScale{link: link, minor: true}
	for _, o := range opts {
		o(s)
	}
	return s
}

type probScale struct {
	domainRange
	link   Link
	fixed  bool
	minor  bool
	format func(float64) string

	// numFormat and loc are the declarative half of the same choice; see
	// [NumberFormat].
	numFormat numberFormat
	loc       *Locale
}

// probDefaultLo and probDefaultHi frame an axis that has seen no data: one
// per cent to ninety-nine, the middle of the ladder.
const (
	probDefaultLo = 0.01
	probDefaultHi = 0.99
)

func (s *probScale) Train(vs ...float64) {
	if s.fixed {
		return
	}
	for _, v := range vs {
		if s.Defined(v) {
			s.domainRange.Train(v)
		}
	}
}

// Defined implements [Definite]: only values strictly between 0 and 1 have a
// position.
func (s *probScale) Defined(v float64) bool { return v > 0 && v < 1 }

// effective returns the domain actually mapped, inside (0, 1) and
// non-degenerate.
func (s *probScale) effective() (float64, float64) {
	if !s.trained {
		return probDefaultLo, probDefaultHi
	}
	lo, hi := s.dmin, s.dmax
	if lo == hi {
		// One repeated value still has to render. One unit either side in link
		// space is the analogue of the log scale's decade either side.
		z := s.link.Apply(lo)
		lo, hi = s.link.Unapply(z-1), s.link.Unapply(z+1)
		if !(lo > 0 && hi < 1 && lo < hi) {
			return probDefaultLo, probDefaultHi
		}
	}
	return lo, hi
}

func (s *probScale) Domain() (float64, float64) { return s.effective() }

// t is v's normalised position along the domain, in link space.
func (s *probScale) t(v, lo, hi float64) float64 {
	zlo, zhi := s.link.Apply(lo), s.link.Apply(hi)
	return (s.link.Apply(v) - zlo) / (zhi - zlo)
}

func (s *probScale) Map(v float64) float32 {
	if !s.Defined(v) {
		return float32(math.NaN())
	}
	lo, hi := s.effective()
	rlo, rhi := s.rangeOf()
	// Snap the endpoints, for the reason the linear scale does.
	switch v {
	case lo:
		return rlo
	case hi:
		return rhi
	}
	return place(rlo, rhi, s.t(v, lo, hi))
}

// Map64 implements [Precise].
func (s *probScale) Map64(v float64) float64 {
	if !s.Defined(v) {
		return math.NaN()
	}
	lo, hi := s.effective()
	rlo, rhi := s.rangeOf()
	return place64(rlo, rhi, s.t(v, lo, hi))
}

func (s *probScale) Invert(pos float32) float64 {
	lo, hi := s.effective()
	rlo, rhi := s.rangeOf()
	if rhi == rlo {
		return lo
	}
	zlo, zhi := s.link.Apply(lo), s.link.Apply(hi)
	t := float64((pos - rlo) / (rhi - rlo))
	return s.link.Unapply(zlo + t*(zhi-zlo))
}

// rung is one candidate on the probability ladder. rank orders how readily it
// is labelled: the median first, then the decades into both tails, then the
// rungs between them.
type rung struct {
	v      float64
	rank   int
	decade int // k for 10^-k and 1−10^-k, zero for every other rung
}

// maxProbDecades is how far into a tail the ladder reaches: 10^-12 per unit,
// past which 1−10^-k is no longer distinguishable from 1 in a float64 with
// digits to spare.
const maxProbDecades = 12

// ladder lists the rungs inside [lo, hi], ascending.
func ladder(lo, hi float64) []rung {
	out := make([]rung, 0, 16)
	add := func(v float64, rank, decade int) {
		if v >= lo*(1-logEps) && v <= hi+(1-hi)*logEps {
			out = append(out, rung{v, rank, decade})
		}
	}
	add(0.5, 0, 0)
	for k := 1; k <= maxProbDecades; k++ {
		p := math.Pow(10, -float64(k))
		if p < lo*(1-logEps) && 1-p > hi {
			break
		}
		add(p, 1, k)
		add(1-p, 1, k)
	}
	for _, v := range [...]float64{0.05, 0.2, 0.8, 0.95} {
		add(v, 2, 0)
	}
	add(0.3, 3, 0)
	add(0.7, 3, 0)
	sort.Slice(out, func(i, j int) bool { return out[i].v < out[j].v })
	return out
}

func (s *probScale) Ticks(req TickRequest) []Tick {
	want := req.Want
	if want <= 0 {
		want = defaultTickCount
	}
	if want < 2 {
		want = 2
	}
	lo, hi := s.effective()
	rungs := ladder(lo, hi)

	// Label the finest ranks that fit. When even the median and the decades
	// do not, step whole decades, so the labels that remain are still a
	// geometric sequence into each tail.
	counts := [4]int{}
	for _, r := range rungs {
		counts[r.rank]++
	}
	limit, step := 3, 1
	for limit > 1 && counts[0]+counts[1]+counts[2]+counts[3] > want {
		counts[limit] = 0
		limit--
	}
	if n := counts[0] + counts[1]; limit == 1 && n > want {
		step = (n + want - 1) / want
	}

	fmtFn := s.labelFunc()
	out := make([]Tick, 0, len(rungs))
	for _, r := range rungs {
		labelled := r.rank <= limit && (r.decade == 0 || (r.decade-1)%step == 0)
		pos := s.Map(r.v)
		switch {
		case labelled:
			out = append(out, Tick{Value: r.v, Pos: pos, Label: fmtFn(r.v)})
		case s.minor:
			out = append(out, Tick{Value: r.v, Pos: pos, Minor: true})
		}
	}
	return out
}

// percent is what a probability axis writes with no spec: the fraction as a
// percentage, with as many decimals as the rung needs.
var percent = numberFormat{set: true, decimals: -1, style: '%'}

func (s *probScale) labelFunc() func(float64) string {
	if s.format != nil {
		return s.format
	}
	return s.LabelOf
}

// LabelOf implements [Labeller].
func (s *probScale) LabelOf(v float64) string {
	if s.format != nil {
		return s.format(v)
	}
	f := s.numFormat
	if !f.set {
		f = percent
	}
	return f.label(v, autoFormat{mode: 'p'}, s.loc)
}

// SetLocale implements [Localizer].
func (s *probScale) SetLocale(loc *Locale) { s.loc = loc }

func (s *probScale) Describe() Desc {
	name, _ := LinkName(s.link)
	d := Desc{
		Kind: KindProbability, Link: name, Fixed: s.fixed,
		MinorTicks: s.minor, Formatted: s.format != nil,
		Format: s.numFormat.spec, Locale: localeName(s.loc),
	}
	if s.fixed {
		d.Min, d.Max = s.dmin, s.dmax
	}
	return d
}

func (s *probScale) Clone() Scale {
	c := *s
	if !c.fixed {
		c.domainRange = domainRange{}
	} else {
		c.rlo, c.rhi, c.rset = 0, 0, false
	}
	return &c
}

func (s *probScale) Snapshot() Scale { c := *s; return &c }

// probClamp is how close to 0 or 1 a drag may pin the domain. A probability
// axis has no position for either, so a pan that walks off an end stops at the
// sixth decade of that tail rather than emptying the axis.
const probClamp = 1e-6

func (s *probScale) SetDomain(min, max float64) {
	min, max = order(min, max)
	min = math.Min(math.Max(min, probClamp), 1-2*probClamp)
	max = math.Max(math.Min(max, 1-probClamp), min+probClamp)
	s.dmin, s.dmax = min, max
	s.trained, s.fixed = true, true
}

func (s *probScale) Autoscale() {
	s.domainRange = domainRange{rlo: s.rlo, rhi: s.rhi, rset: s.rset}
	s.fixed = false
}

var (
	_ Definite    = (*probScale)(nil)
	_ Precise     = (*probScale)(nil)
	_ Labeller    = (*probScale)(nil)
	_ Localizer   = (*probScale)(nil)
	_ Describer   = (*probScale)(nil)
	_ Cloner      = (*probScale)(nil)
	_ Snapshotter = (*probScale)(nil)
	_ Zoomer      = (*probScale)(nil)
)

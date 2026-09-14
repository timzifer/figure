package stat

import "math"

// A locus is a curve given by a formula rather than by a measurement, and a
// [Family] is an indexed set of them: the M and N contours of a Nichols chart,
// the constant-VSWR circles and constant-Q arcs of a Smith chart. Each is
// printed underneath the data and says what the region of the plane means.
//
// They are here for the reason everything else here is: each is a pure function
// of numbers, with no scale, no theme and no geom in it. What the curves look
// like is the coordinate stage's business — a constant-reflection locus is a
// circle in impedance and a circle on the disc, and neither this package nor
// the mark that draws it knows which chart it landed on. See
// docs/adr/0050-locus-annotations.md.

// Family is one indexed set of curves in data space.
//
// Given a level and the panel's extent, Locus appends the data-space points of
// that level's curve to xs and ys and returns the extended slices. Both are
// appended to in lockstep and come back the same length; the caller owns them
// and truncates them, which is what keeps a chart redrawn every frame from
// allocating a curve every frame.
//
// A curve that leaves the extent and comes back, or that repeats — a Nichols
// family repeats every 360° of phase — appends NaN between its runs, which is
// the gap every mark in figure already understands.
//
// # Stability
//
// Family is implemented outside this module, so it never gains a method, and
// the extent it is handed is a struct for the same reason — ADR 0060. The four
// families below are values rather than functions so that each has a name that
// can be written down: [FamilyName] reports it, [FamilyNamed] reads it back,
// and a family a caller wrote in Go has neither and does not serialise.
type Family interface {
	Locus(xs, ys []float64, level float64, ext Extent) ([]float64, []float64)
}

// Extent is the region of data space a curve is drawn over, and how finely.
//
// The bounds are the panel's own domains, ascending. They are what a repeating
// family needs in order to know how many times to repeat, and what an unbounded
// one needs in order to know where to stop; a family whose curves are bounded —
// a VSWR circle is a circle whatever the axes say — ignores them. Steps is how
// many samples the panel is wide enough to be worth, which is a question about
// the screen and is therefore answered when the chart is drawn rather than when
// its axes are trained.
//
// An empty extent is legitimate: it is what a family is handed before the
// domains exist, and every family answers it with a curve rather than with
// nothing.
type Extent struct {
	X0, X1, Y0, Y1 float64
	Steps          int
}

// The sampling bounds, for a caller who hands an extent along by hand and for
// one whose panel is enormous. What Steps means to a family is up to the
// family: the two Smith ones walk that many samples, and the two Nichols ones
// read it as how fine a step the panel deserves and refine down to it.
const (
	defaultLocusSteps = 360
	maxLocusSteps     = 4096
)

func (e Extent) steps() int {
	switch {
	case e.Steps < 8:
		return defaultLocusSteps
	case e.Steps > maxLocusSteps:
		return maxLocusSteps
	}
	return e.Steps
}

// far is the largest coordinate a family will emit, for the two of them whose
// curves run to infinity.
//
// It is a few times the panel rather than a constant, because "off the edge of
// this chart" is what the cut has to mean: a Cartesian panel clips it, and a
// Smith panel — where infinity is the rim and everything past r = 4·X1 is
// within a hair of it — shows the arc reaching the edge of the disc.
func (e Extent) far() float64 {
	f := 1.0
	for _, v := range [...]float64{e.X0, e.X1, e.Y0, e.Y1} {
		if a := math.Abs(v); a > f && !math.IsInf(a, 0) {
			f = a
		}
	}
	return 4 * f
}

type family string

const (
	nameNicholsM  family = "nichols-m"
	nameNicholsN  family = "nichols-n"
	nameSmithVSWR family = "smith-vswr"
	nameSmithQ    family = "smith-q"
)

// The built-in families.
//
// The two Nichols families describe the closed loop T = L/(1+L) over a chart
// whose axes hold the open loop L: X is arg L in degrees and Y is 20·log₁₀|L|
// in decibels. Both are circles in the complex L-plane before the chart's
// log-polar step, which is why neither costs more than a few lines of
// arithmetic.
//
// The two Smith families describe a normalised impedance z = r + jx over a
// chart whose axes hold r and x, which is the pair [github.com/timzifer/figure/coord.Smith]
// reads. Neither of them is drawn as a circle on the disc: each is the set of
// impedances satisfying a condition, and the coord maps it into the shape it
// looks like.
var (
	// NicholsM is the locus of constant closed-loop magnitude |T|, and its
	// level is that magnitude in decibels. It is what the resonance peak M_r is
	// read off. The 0 dB member is the perpendicular bisector of the origin and
	// the critical point rather than a circle, which is the one degenerate case
	// in either Nichols family.
	NicholsM Family = nameNicholsM

	// NicholsN is the locus of constant closed-loop phase ∠T, and its level is
	// that phase in degrees. Every member passes through L = 0 and through
	// L = −1, which is why the contours all converge on the critical point and
	// why each one plunges to −∞ dB where it passes through the origin — a
	// place no scale can put, and the missing-value policy has covered that
	// since v0.1. The 0° and 180° members are the real axis itself and are not
	// drawn.
	NicholsN Family = nameNicholsN

	// SmithVSWR is the locus of constant reflection magnitude, and its level is
	// the standing-wave ratio the chart is read in: 1.5, 2, 3. A level of 1 is a
	// matched load, which is the middle of the chart and a point rather than a
	// curve, so it draws nothing.
	SmithVSWR Family = nameSmithVSWR

	// SmithQ is the locus of constant reactance-to-resistance ratio |x| = Q·r,
	// and its level is Q. It is two straight rays in impedance, so it is two
	// arcs on the disc, by the coord's own map and with no second
	// implementation. They run to an infinite impedance, which is the rim.
	SmithQ Family = nameSmithQ
)

// FamilyNamed is the family written down under name, or ok == false for a name
// this package has no family for.
//
// The set is closed on purpose. A family a caller wrote in Go is a perfectly
// good [Family] and has no name here, because a name that resolved to something
// this package cannot rebuild would make a document that does not round-trip —
// which is [ADR 0041]'s rule for a quantile function, applied to a curve.
//
// [ADR 0041]: https://github.com/timzifer/figure/blob/main/docs/adr/0041-qq-plots.md
func FamilyNamed(name string) (Family, bool) {
	switch family(name) {
	case nameNicholsM:
		return NicholsM, true
	case nameNicholsN:
		return NicholsN, true
	case nameSmithVSWR:
		return SmithVSWR, true
	case nameSmithQ:
		return SmithQ, true
	}
	return nil, false
}

// FamilyName is the name f is written down under, or ok == false for a family
// this package did not define.
func FamilyName(f Family) (string, bool) {
	n, ok := f.(family)
	if !ok {
		return "", false
	}
	if _, known := FamilyNamed(string(n)); !known {
		return "", false
	}
	return string(n), true
}

func (f family) String() string { return string(f) }

func (f family) Locus(xs, ys []float64, level float64, ext Extent) ([]float64, []float64) {
	if math.IsNaN(level) || math.IsInf(level, 0) {
		return xs, ys
	}
	switch f {
	case nameNicholsM:
		return nicholsM(xs, ys, level, ext)
	case nameNicholsN:
		return nicholsN(xs, ys, level, ext)
	case nameSmithVSWR:
		return smithVSWR(xs, ys, level, ext)
	case nameSmithQ:
		return smithQ(xs, ys, level, ext)
	}
	return xs, ys
}

// --- the Nichols families --------------------------------------------------

// nicholsHeadroom is how far outside the panel a Nichols curve is drawn before
// it is cut, as a fraction of the panel's own height.
//
// Both families run to infinity — an M contour upwards along its degenerate
// member, every N contour down to −∞ dB where it passes through the origin — so
// a curve is drawn past the edge of the panel, in order to leave the picture at
// the edge rather than short of it, and cut a little beyond. It is cut because
// a device coordinate is a float32 and a polyline running to 10²⁰ is a number
// in a document rather than a line in a chart; it is cut close rather than far
// because every sample past the edge is a point in a file nobody can see, and
// an arc approaching the origin has as many of them as it is asked for.
const nicholsHeadroom = 0.1

// nicholsSpan is the height a family assumes when the panel has none, which is
// what it is handed before the domains exist.
const nicholsSpan = 80

// nicholsBreak is the phase step, in degrees, above which a curve is taken to
// have been cut rather than to have bent.
//
// It is how the branch point is found without looking for it. An N contour
// passes through L = 0, where the phase turns by half a turn between one sample
// and the next; on either side of that the curve is smooth. Without this the
// two branches would be joined by a line drawn across the chart at whatever
// depth the sampling happened to stop at.
const nicholsBreak = 90

// maxBands bounds how many times a repeating family is redrawn. A Nichols panel
// shows one turn of phase, occasionally two; sixteen is past any chart and stops
// a domain of a million degrees from being a million lines.
const maxBands = 16

// The sampling of a Nichols curve: where it starts, how fine it is allowed to
// get, and how much of it there may be.
//
// It is refined rather than merely sampled because the chart is the log-polar
// view of a circle, and the two are not evenly spaced against each other: the
// arc of the circle that passes near the origin is a hair of it and is the
// whole plunge to −∞ dB, while the rest of the same circle is a smooth curve a
// few dozen samples describe. A uniform walk resolves one of those or the
// other, never both — the −1° N contour sampled uniformly stops dead at −6 dB,
// which is a contour floating in mid-air where the chart says it should leave
// the bottom of the panel.
const (
	nicholsSeed  = 64
	nicholsDepth = 20
	// nicholsChord is the step a curve is refined down to, as a multiple of the
	// panel's own sampling step. Three of them is a chord of a few pixels on a
	// panel of any size a chart is printed at, which is below a pixel of sag
	// where the curve is smooth and is a corner a reader cannot see where it is
	// not — the contours converge on the critical point, and a family is drawn
	// underneath the data rather than read off.
	nicholsChord = 3
)

// nichols walks one circle of the complex L-plane — or the vertical line its
// degenerate member is, when rad is zero — and appends it as the pair a Nichols
// chart holds.
type nichols struct {
	cu, cv, rad float64
	// span is the half-angle the degenerate member is walked over, which is
	// where it leaves the top of the window rather than an arbitrary distance
	// up an infinite line.
	span float64
	// head is how far outside the panel the curve is drawn, in decibels.
	head       float64
	ext        Extent
	tolX, tolY float64
	xs, ys     []float64
	start      int
	prev       float64
	budget     int
}

func newNichols(xs, ys []float64, cu, cv, rad float64, ext Extent) *nichols {
	spanX, spanY := ext.X1-ext.X0, ext.Y1-ext.Y0
	if !(spanX > 0) {
		spanX = 360
	}
	if !(spanY > 0) {
		spanY = nicholsSpan
	}
	// The line's ends are where its magnitude reaches the top of the window:
	// |L| = 1/2·sec θ, so the angle it is walked to is the one that reaches it.
	top := math.Pow(10, (ext.Y1+nicholsHeadroom*spanY)/20)
	if !(top > 1) || top > 1e9 {
		top = 1e9
	}
	n := ext.steps()
	return &nichols{
		cu: cu, cv: cv, rad: rad, ext: ext, head: nicholsHeadroom * spanY,
		span: 2 * math.Acos(0.5/top) / math.Pi,
		tolX: nicholsChord * spanX / float64(n),
		tolY: nicholsChord * spanY / float64(n),
		xs:   xs, ys: ys, start: len(xs), prev: math.NaN(),
		budget: 8 * n,
	}
}

// at is the curve at t, as the chart holds it: the phase in degrees as atan2
// reports it, and the magnitude in decibels.
func (c *nichols) at(t float64) (phase, db float64) {
	u, v := c.cu, c.cv
	if c.rad > 0 {
		th := 2 * math.Pi * t
		u, v = c.cu+c.rad*math.Cos(th), c.cv+c.rad*math.Sin(th)
	} else {
		// The degenerate member is a line, and it is walked by the angle it is
		// seen at from the origin rather than by v: that is what spaces the
		// samples along the curve the chart draws, where uniform v would put
		// almost all of them in the ends.
		u, v = c.cu, c.cv+0.5*math.Tan(math.Pi*(t-0.5)*c.span)
	}
	return math.Atan2(v, u) * 180 / math.Pi, 20 * math.Log10(math.Hypot(u, v))
}

func (c *nichols) walk() ([]float64, []float64) {
	t0 := 0.0
	x0, y0 := c.at(t0)
	c.emit(x0, y0)
	for i := 1; i <= nicholsSeed; i++ {
		t1 := float64(i) / nicholsSeed
		x1, y1 := c.at(t1)
		c.refine(t0, x0, y0, t1, x1, y1, 0)
		c.emit(x1, y1)
		t0, x0, y0 = t1, x1, y1
	}
	return repeatBands(c.xs, c.ys, c.start, c.ext)
}

// refine appends the samples strictly between two the caller has, halving the
// interval while the step between its ends is coarser than the panel deserves.
//
// It stops at a step that leaves the window the curve is drawn in, which is
// what bounds the plunge: past the bottom of the panel there is nothing left to
// resolve, and the arc approaching the origin is self-similar, so a rule that
// only looked at the step would halve for ever.
func (c *nichols) refine(t0, x0, y0, t1, x1, y1 float64, depth int) {
	if depth >= nicholsDepth || c.budget <= 0 || !c.coarse(x0, y0, x1, y1) {
		return
	}
	tm := (t0 + t1) / 2
	xm, ym := c.at(tm)
	c.refine(t0, x0, y0, tm, xm, ym, depth+1)
	c.emit(xm, ym)
	c.refine(tm, xm, ym, t1, x1, y1, depth+1)
}

// coarse reports whether the step between two samples is more than the panel
// deserves, and there is anything left in view to resolve.
//
// One end outside the window is still refined: that interval holds the point
// where the curve leaves the picture, and stopping at the last sample that
// happened to be inside is what leaves an N contour hanging in mid-air twenty
// decibels above the bottom of the panel. Both ends outside is a stretch of
// curve nobody can see, and the depth limit is what stops the first case from
// halving for ever — the arc approaching the origin is self-similar, so the
// step never falls below six decibels however far it is chased.
func (c *nichols) coarse(x0, y0, x1, y1 float64) bool {
	if !c.drawable(y0) && !c.drawable(y1) {
		return false
	}
	d := x1 - x0
	d -= 360 * math.Round(d/360)
	return math.Abs(d) > c.tolX || math.Abs(y1-y0) > c.tolY
}

func (c *nichols) drawable(db float64) bool {
	return !math.IsNaN(db) && db >= c.ext.Y0-c.head && db <= c.ext.Y1+c.head
}

// emit appends one sample, unwrapping its phase so that a curve crossing the
// half-turn does not come back across the panel, and breaking the run where the
// curve leaves the window or turns through the branch point.
func (c *nichols) emit(phase, db float64) {
	if !c.drawable(db) {
		c.break_()
		return
	}
	p := phase
	if !math.IsNaN(c.prev) {
		p -= 360 * math.Round((p-c.prev)/360)
		if math.Abs(p-c.prev) > nicholsBreak {
			// The curve was cut rather than bent, so the run ends here and the
			// next one starts at this sample, as atan2 reported it.
			c.break_()
			p = phase
		}
	}
	c.xs, c.ys = append(c.xs, p), append(c.ys, db)
	c.prev = p
	c.budget--
}

func (c *nichols) break_() {
	c.xs, c.ys = appendBreak(c.xs, c.ys, c.start)
	c.prev = math.NaN()
}

func nicholsM(xs, ys []float64, level float64, ext Extent) ([]float64, []float64) {
	m := math.Pow(10, level/20)
	if !(m > 0) || math.IsInf(m, 0) {
		return xs, ys
	}
	if math.Abs(m-1) < 1e-12 {
		// |T| = 1 is |L| = |1 + L|: the points equidistant from the origin and
		// the critical point, which is the vertical line u = −1/2 rather than a
		// circle.
		return newNichols(xs, ys, -0.5, 0, 0, ext).walk()
	}
	// |T| = M is a circle, and which side of the critical point it is on is
	// what M − 1 decides: above 0 dB it is an oval around L = −1, and below it
	// is a loop around the origin, so the second kind winds a whole turn of
	// phase and the first does not. Nothing here has to know that; the phase is
	// unwrapped from sample to sample either way.
	return newNichols(xs, ys, -m*m/(m*m-1), 0, math.Abs(m/(m*m-1)), ext).walk()
}

func nicholsN(xs, ys []float64, level float64, ext Extent) ([]float64, []float64) {
	// N = tan ∠T, and ∠T = 0 or 180° is the real axis itself: the circle's
	// centre runs off to infinity and its radius with it. A chart prints
	// neither, so neither is drawn.
	tan := math.Tan(level * math.Pi / 180)
	if math.Abs(tan) < 1e-6 || math.IsInf(tan, 0) || math.IsNaN(tan) {
		return xs, ys
	}
	return newNichols(xs, ys, -0.5, 1/(2*tan), 0.5*math.Sqrt(1+1/(tan*tan)), ext).walk()
}

func appendBreak(xs, ys []float64, start int) ([]float64, []float64) {
	if len(xs) == start || math.IsNaN(xs[len(xs)-1]) {
		return xs, ys
	}
	return append(xs, math.NaN()), append(ys, math.NaN())
}

// repeatBands redraws the run that starts at start once per 360° band of phase
// the extent covers, because both Nichols families repeat with the phase.
//
// The copies are the curve itself shifted, so a family costs one evaluation
// however wide the panel is — and a panel that shows no whole band, which is
// what a family is handed before the domains exist, keeps the one run it has.
func repeatBands(xs, ys []float64, start int, ext Extent) ([]float64, []float64) {
	if !(ext.X0 < ext.X1) {
		return xs, ys
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range xs[start:] {
		if !math.IsNaN(p) {
			lo, hi = math.Min(lo, p), math.Max(hi, p)
		}
	}
	first, last := math.Ceil((ext.X0-hi)/360), math.Floor((ext.X1-lo)/360)
	if math.IsNaN(first) || math.IsInf(first, 0) || math.Abs(first) > maxBands*maxBands {
		return xs, ys
	}
	if last-first >= maxBands {
		last = first + maxBands - 1
	}
	bx, by := xs[start:], ys[start:]
	for k := int(first); k <= int(last); k++ {
		if k == 0 {
			continue
		}
		off := 360 * float64(k)
		xs, ys = appendBreak(xs, ys, start)
		for i := range bx {
			xs, ys = append(xs, bx[i]+off), append(ys, by[i])
		}
	}
	return xs, ys
}

// --- the Smith families ----------------------------------------------------

func smithVSWR(xs, ys []float64, level float64, ext Extent) ([]float64, []float64) {
	// The level is the standing-wave ratio s, and |Γ| = (s−1)/(s+1). A ratio
	// below one is not a reflection and a ratio of one is the middle of the
	// chart: a point, which no stroke draws.
	if !(level > 1) || math.IsInf(level, 0) {
		return xs, ys
	}
	g := (level - 1) / (level + 1)
	// The image of |Γ| = g under z = (1+Γ)/(1−Γ) is a circle on the resistance
	// axis. It is written here rather than walked in Γ so that the points this
	// family emits are impedances, which is what makes the coord — and not this
	// function — the thing that knows what the curve looks like.
	c, rad := (1+g*g)/(1-g*g), 2*g/(1-g*g)
	n := ext.steps()
	for i := 0; i <= n; i++ {
		th := 2 * math.Pi * float64(i) / float64(n)
		if i == n {
			th = 0 // the curve closes on the point it started at, exactly.
		}
		xs, ys = append(xs, c+rad*math.Cos(th)), append(ys, rad*math.Sin(th))
	}
	return xs, ys
}

func smithQ(xs, ys []float64, level float64, ext Extent) ([]float64, []float64) {
	q := math.Abs(level)
	if !(q > 0) || math.IsInf(q, 0) {
		return xs, ys
	}
	n, far := ext.steps(), ext.far()
	// |x| = Q·r is two rays from the origin, one in each half of the chart, and
	// they are sampled in the reflection plane rather than in r. Both are
	// straight lines in impedance, so where the samples fall along them changes
	// nothing about the line — and it changes everything about the arc the disc
	// draws, where an even walk in r would crowd every sample into the middle
	// and leave the rim a chord.
	for _, sign := range [...]float64{1, -1} {
		if len(xs) > 0 {
			xs, ys = append(xs, math.NaN()), append(ys, math.NaN())
		}
		// The image of the ray is the arc through Γ = −1 (a short circuit) and
		// Γ = +1 (an open) of the circle centred at −j·sign/Q.
		cv, rad := -sign/q, math.Sqrt(1+1/(q*q))
		from := math.Atan2(-cv, -1)
		to := math.Atan2(-cv, 1)
		for i := 0; i <= n; i++ {
			th := from + (to-from)*float64(i)/float64(n)
			gr, gi := rad*math.Cos(th), cv+rad*math.Sin(th)
			// z = (1 + Γ)/(1 − Γ), and the open circuit at the far end of the
			// ray is an infinite impedance. The arc stops where the panel stops
			// caring, which is what keeps a coordinate finite.
			d := (1-gr)*(1-gr) + gi*gi
			if d <= 0 {
				continue
			}
			r, x := (1-gr*gr-gi*gi)/d, 2*gi/d
			if math.Hypot(r, x) > far {
				continue
			}
			xs, ys = append(xs, r), append(ys, x)
		}
	}
	return xs, ys
}

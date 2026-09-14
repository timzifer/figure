// Command nichols renders the chart a family of curves unlocks.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. And like examples/smith, its point is
// what is *not* here: there is no Nichols coord, no control-systems package and
// no new mark. A Nichols diagram is a geom.Line over two ordinary linear axes —
// the open-loop phase in degrees along X, the open-loop gain in decibels along
// Y — and what makes it a Nichols diagram is the grid printed underneath it,
// which is two geom.Locus layers.
//
// The two families describe the *closed* loop T = L/(1+L) over a chart whose
// axes hold the open loop L. The M contours are the loci of constant |T| and
// are what the resonance peak is read off; the N contours are the loci of
// constant ∠T. Neither is at a value of either axis, which is why neither is
// furniture and both are annotations — see docs/adr/0050-locus-annotations.md.
package main

import (
	"flag"
	"fmt"
	"math"
	"math/cmplx"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

func main() {
	loop := flag.String("loop", "loop.svg", "output path for the open-loop response")
	peak := flag.String("peak", "peak.svg", "output path for the resonance detail")
	flag.Parse()
	if err := run(*loop, *peak); err != nil {
		fmt.Fprintln(os.Stderr, "nichols:", err)
		os.Exit(1)
	}
}

func run(loop, peak string) error {
	for _, step := range []func() error{
		func() error { return openLoop(loop) },
		func() error { return resonance(peak) },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// The levels a chart is printed at. They are a convention of the field rather
// than the answer to a tick search — which is scale.TickValues's situation
// exactly, and the reason the caller names them.
var (
	mLevels = []float64{-12, -6, -3, -1, 0, 1, 3, 6, 12}
	nLevels = []float64{-1, -5, -10, -20, -45, -90, -150, -210, -270}
)

// grid adds the two families under whatever the panel already holds.
//
// They go on first because a chart draws its layers in the order they were
// added, and furniture that covered the measurement would be furniture in the
// way. Neither trains an axis: a locus says what the region of the plane means,
// and the region is whatever the axes already show.
func grid(p *figure.Plot) *figure.Plot {
	// Solid for the magnitudes and dashed for the phases, which is how a chart
	// is printed and the only thing telling two families of thin grey curves
	// apart. Both draw in the theme's annotation ink: they are furniture, and
	// furniture in a series colour would read as something that was measured.
	p.Add(geom.Locus(stat.NicholsM, mLevels, geom.Dash(), geom.Label("closed-loop gain")))
	p.Add(geom.Locus(stat.NicholsN, nLevels, geom.Dash(2, 3), geom.Label("closed-loop phase")))
	return p
}

// openLoop is the chart itself: one loop drawn at two gains, against the grid
// that says what each of them does to the closed loop.
//
// The reading is the one a Bode plot cannot give in a single picture. The lower
// gain is tangent to the 3 dB contour, which is a closed loop that peaks at
// 3 dB — the textbook design target. The higher one cuts inside the 12 dB
// contour and passes close to the critical point at (−180°, 0 dB), where the
// contours converge and the loop would oscillate.
func openLoop(out string) error {
	p := figure.New(
		figure.Size(640, 560),
		figure.Title("An open loop, and what it does to the closed one"),
		figure.Theme(theme.Light),
		figure.XTitle("open-loop phase (degrees)"),
		figure.YTitle("open-loop gain (dB)"),
	)
	p.X(scale.Linear(scale.Domain(-270, -90),
		scale.TickValues(-270, -240, -210, -180, -150, -120, -90)))
	p.Y(scale.Linear(scale.Domain(-24, 36)))
	grid(p)
	for i, k := range []float64{tangent, 6} {
		phase, gain := sweep(k)
		p.Add(geom.Line(
			figure.NewTable().Float64("phase", phase).Float64("gain", gain),
			geom.X("phase"), geom.Y("gain"),
			geom.Color(palette.OkabeIto[1+i]), geom.Width(2),
			geom.Label(fmt.Sprintf("K = %.2f", k))))
	}
	return p.Render(figure.SVG(out))
}

// resonance is the same chart over a tenth of the area, which is the other half
// of what handing a family the panel's extent buys: the curves are computed
// against the panel rather than clipped from a picture of the whole plane, so
// zooming in gives more of the grid rather than more of the same lines.
//
// The marker is where the response touches the 3 dB contour. It is computed
// from the loop rather than placed by hand, so the chart is a reading of a
// system and not an illustration of one.
func resonance(out string) error {
	phase, gain := sweep(tangent)
	at := peakOf(tangent)

	p := figure.New(
		figure.Size(520, 460),
		figure.Title("Where the loop touches 3 dB"),
		figure.Theme(theme.Light),
		figure.XTitle("open-loop phase (degrees)"),
		figure.YTitle("open-loop gain (dB)"),
		figure.Legend(false),
	)
	p.X(scale.Linear(scale.Domain(-190, -110), scale.TickValues(-180, -160, -140, -120)))
	p.Y(scale.Linear(scale.Domain(-8, 10)))
	grid(p)
	p.Add(geom.Line(
		figure.NewTable().Float64("phase", phase).Float64("gain", gain),
		geom.X("phase"), geom.Y("gain"), geom.Color(palette.OkabeIto[1]), geom.Width(2)))
	p.Add(geom.Scatter(
		figure.NewTable().Float64("phase", []float64{at.phase}).Float64("gain", []float64{at.gain}),
		geom.X("phase"), geom.Y("gain"), geom.Color(palette.OkabeIto[0]), geom.Size(9)))
	p.Add(geom.Note(at.phase-2.5, at.gain+1.5,
		fmt.Sprintf("peak %.1f dB at ω = %.2f rad/s", at.peak, at.w),
		geom.Align(ir.AlignEnd, ir.AlignBaseline)))
	return p.Render(figure.SVG(out))
}

// --- the loop --------------------------------------------------------------

// tangent is the loop gain whose closed loop peaks at exactly 3 dB, which is
// the gain at which the response is tangent to that contour. It is solved for
// rather than tabulated, so the chart cannot drift away from its own caption.
var tangent = gainForPeak(3)

// L is the plant: an integrator and two lags, which is the shape of most things
// with a motor in them.
func L(w, k float64) complex128 {
	s := complex(0, w)
	return complex(k, 0) / (s * (1 + s/2) * (1 + s/10))
}

// sweep is the open loop over four decades, as the pair the panel reads.
//
// The phase is unwrapped as it goes, because cmplx.Phase reports a half-turn
// either side of zero and this loop passes through −180°: left alone, the
// response would leap across the whole chart where it crosses. That is the
// caller's arithmetic and not the chart's — a phase of −190° and one of +170°
// are the same reflection and different readings, and only the sweep knows
// which it meant.
func sweep(k float64) (phase, gain []float64) {
	const n = 400
	phase, gain = make([]float64, n), make([]float64, n)
	for i := range n {
		l := L(omega(float64(i)/float64(n-1)), k)
		phase[i] = cmplx.Phase(l) * 180 / math.Pi
		if i > 0 {
			phase[i] -= 360 * math.Round((phase[i]-phase[i-1])/360)
		}
		gain[i] = 20 * math.Log10(cmplx.Abs(l))
	}
	return phase, gain
}

func omega(t float64) float64 { return math.Pow(10, -1+4*t) }

type reading struct{ phase, gain, peak, w float64 }

// peakOf is where the closed loop resonates: the sample at which |T| is
// largest, and what the open loop was doing there.
func peakOf(k float64) reading {
	best := reading{peak: math.Inf(-1)}
	for i := range 4001 {
		w := omega(float64(i) / 4000)
		l := L(w, k)
		if db := 20 * math.Log10(cmplx.Abs(l/(1+l))); db > best.peak {
			best = reading{
				phase: unwrapped(cmplx.Phase(l) * 180 / math.Pi),
				gain:  20 * math.Log10(cmplx.Abs(l)),
				peak:  db,
				w:     w,
			}
		}
	}
	return best
}

// gainForPeak is the loop gain whose closed loop peaks at db, by bisection on a
// quantity that rises with it.
// unwrapped puts a phase in the turn the chart draws, which is the one this
// loop's lag runs through.
func unwrapped(deg float64) float64 {
	if deg > 0 {
		return deg - 360
	}
	return deg
}

func gainForPeak(db float64) float64 {
	lo, hi := 0.01, 100.0
	for range 64 {
		k := (lo + hi) / 2
		if peakOf(k).peak < db {
			lo = k
		} else {
			hi = k
		}
	}
	return (lo + hi) / 2
}

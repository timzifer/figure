package main

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

// nicholsFigure is the chart a family of curves unlocks, and the first one in
// this gallery whose grid is not furniture.
//
// A Nichols diagram is a geom.Line over two ordinary linear axes — the
// open-loop phase in degrees along X, the open-loop gain in decibels along Y.
// What makes it a Nichols diagram is the two geom.Locus layers underneath it:
// the loci of constant closed-loop magnitude and of constant closed-loop phase,
// neither of which is at a value of either axis, which is why neither is a grid
// line and both are annotations. See docs/adr/0050-locus-annotations.md.
//
// The reading is the one a Bode plot cannot give in a single picture: the
// response is tangent to the 3 dB contour, so the closed loop peaks at 3 dB.
// The gain that makes that true is solved for rather than tabulated, so the
// figure and its caption cannot drift apart. The whole of examples/nichols is
// this chart and its detail.
func nicholsFigure() plate {
	return plate{
		name: "nichols", width: 640, high: 560, theme: theme.Light,
		title: "An open loop, and what it does to the closed one",
		opts: []figure.Option{
			figure.XTitle("open-loop phase (degrees)"),
			figure.YTitle("open-loop gain (dB)"),
		},
		build: func(p *figure.Plot) {
			p.X(scale.Linear(scale.Domain(-270, -90),
				scale.TickValues(-270, -240, -210, -180, -150, -120, -90)))
			p.Y(scale.Linear(scale.Domain(-24, 36)))
			// Solid for the magnitudes and dashed for the phases, which is how
			// a chart is printed and the only thing telling two families of
			// thin grey curves apart. Both draw in the theme's annotation ink:
			// furniture in a series colour would read as something measured.
			p.Add(geom.Locus(stat.NicholsM,
				[]float64{-12, -6, -3, -1, 0, 1, 3, 6, 12},
				geom.Dash(), geom.Label("closed-loop gain")))
			p.Add(geom.Locus(stat.NicholsN,
				[]float64{-1, -5, -10, -20, -45, -90, -150, -210, -270},
				geom.Dash(2, 3), geom.Label("closed-loop phase")))
			for i, k := range []float64{tangentGain(), 6} {
				phase, gain := openLoopSweep(k)
				p.Add(geom.Line(
					figure.NewTable().Float64("phase", phase).Float64("gain", gain),
					geom.X("phase"), geom.Y("gain"),
					geom.Color(palette.OkabeIto[1+i]), geom.Width(2),
					geom.Label(loopLabel(k))))
			}
		},
	}
}

// openLoop is the plant the figure draws: an integrator and two lags, which is
// the shape of most things with a motor in them.
func openLoop(w, k float64) complex128 {
	s := complex(0, w)
	return complex(k, 0) / (s * (1 + s/2) * (1 + s/10))
}

func loopOmega(t float64) float64 { return math.Pow(10, -1+4*t) }

// openLoopSweep is the loop over four decades, as the pair the panel reads. The
// phase is unwrapped as it goes: cmplx.Phase reports a half-turn either side of
// zero and this loop passes through −180°, so left alone the response would
// leap across the whole chart where it crosses.
func openLoopSweep(k float64) (phase, gain []float64) {
	const n = 400
	phase, gain = make([]float64, n), make([]float64, n)
	for i := range n {
		l := openLoop(loopOmega(float64(i)/float64(n-1)), k)
		phase[i] = cmplx.Phase(l) * 180 / math.Pi
		if i > 0 {
			phase[i] -= 360 * math.Round((phase[i]-phase[i-1])/360)
		}
		gain[i] = 20 * math.Log10(cmplx.Abs(l))
	}
	return phase, gain
}

// closedLoopPeak is how high the closed loop resonates at a given loop gain,
// which is the quantity the contours are read in.
func closedLoopPeak(k float64) float64 {
	peak := math.Inf(-1)
	for i := range 4001 {
		l := openLoop(loopOmega(float64(i)/4000), k)
		peak = math.Max(peak, 20*math.Log10(cmplx.Abs(l/(1+l))))
	}
	return peak
}

// tangentGain is the loop gain whose closed loop peaks at exactly 3 dB, by
// bisection on a quantity that rises with it.
func tangentGain() float64 {
	lo, hi := 0.01, 100.0
	for range 64 {
		k := (lo + hi) / 2
		if closedLoopPeak(k) < 3 {
			lo = k
		} else {
			hi = k
		}
	}
	return (lo + hi) / 2
}

func loopLabel(k float64) string { return fmt.Sprintf("K = %.2f", k) }

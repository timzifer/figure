// Command cascade renders the chart ADR 0058 says decides whether the
// machinery pays for itself.
//
// A cascade — a waterfall, a spectrogram with a horizon — is a family of
// traces offset along one floor axis, and it is the display a spectrum
// analyser has had since the seventies. It is the chart an RF or
// instrumentation audience draws on a whiteboard when explaining what they
// measured, and it is what a library that already ships Smith charts, tracks
// and thresholds will be asked for first.
//
// **There is no Cascade layer, and that is the point.** It is a recipe over N
// three-dimensional lines: geom.GroupBy draws one path per sweep, and the
// sweep's own number is the second floor axis. The machinery is 0056's and
// nothing here adds to it.
//
//	go run ./examples/cascade
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
	"github.com/timzifer/figure/three"
)

func main() {
	out := flag.String("o", "cascade.svg", "output path for the cascade")
	flat := flag.String("flat", "cascade-flat.svg", "output path for the same sweeps overlaid")
	flag.Parse()
	if err := run(*out, *flat); err != nil {
		fmt.Fprintln(os.Stderr, "cascade:", err)
		os.Exit(1)
	}
}

func run(out, flat string) error {
	if err := cascade(out); err != nil {
		return err
	}
	return overlaid(flat)
}

// cascade is thirty sweeps of a spectrum, offset by sweep number.
//
// What the third dimension carries here is how the spectrum *moves*: a peak
// drifting, a harmonic appearing, a resonance splitting. The flat chart below
// has all the same numbers in it and buries them under each other.
//
// Two details are load-bearing. geom.GroupBy is what makes this one layer
// rather than thirty — a long table with a sweep column, exactly as a grouped
// line chart is drawn in two dimensions. And a path is emitted one segment at
// a time, so a trace that passes behind another is behind it for the part that
// is behind it rather than as a whole.
func cascade(out string) error {
	freq, sweep, power := sweeps()

	sc := three.NewScene(
		three.XTitle("frequency (MHz)"),
		three.YTitle("sweep"),
		three.ZTitle("power (dBm)"),
	).
		X(scale.Linear()).
		Y(scale.Linear()).
		Z(scale.Linear(scale.Nice())).
		Add(three.Line3(
			figure.NewTable().
				Float64("f", freq).
				Float64("n", sweep).
				Float64("p", power).
				String("trace", traceNames(sweep)),
			geom.X("f"), geom.Y("n"), geom.Z("p"),
			geom.GroupBy("trace"),
			geom.Color(palette.Blue),
			geom.Width(1),
		))

	return three.New(
		three.Size(760, 560),
		three.Title("A drifting carrier, thirty sweeps"),
		three.Theme(theme.Dark),
	).Scene(sc).
		Add(three.View{Camera: three.LookAt(three.Azimuth(-0.9), three.Elevation(0.42))}).
		Render(figure.SVG(out))
}

// overlaid is every sweep on one pair of axes, which is what the cascade
// replaces. The carrier's drift is in here somewhere and nobody can see it.
func overlaid(out string) error {
	freq, sweep, power := sweeps()

	p := figure.New(
		figure.Size(760, 420),
		figure.Title("The same thirty sweeps, overlaid"),
		figure.XTitle("frequency (MHz)"),
		figure.YTitle("power (dBm)"),
		figure.Legend(false),
		figure.Theme(theme.Dark),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear(scale.Nice()))
	p.Add(geom.Line(
		figure.NewTable().
			Float64("f", freq).
			Float64("p", power).
			String("trace", traceNames(sweep)),
		geom.X("f"), geom.Y("p"),
		geom.GroupBy("trace"),
		geom.Color(palette.Blue),
	))
	return p.Render(figure.SVG(out))
}

// sweeps is thirty spectra of a carrier that drifts upward while a harmonic
// grows behind it, over a noise floor that does not move.
//
// It is generated deterministically: no math/rand, because an example whose
// picture changes between runs is an example nobody can check.
func sweeps() (freq, sweep, power []float64) {
	const (
		traces = 30
		bins   = 121
	)
	for n := 0; n < traces; n++ {
		t := float64(n) / (traces - 1)
		carrier := 900 + 24*t
		harmonic := 960 + 6*t
		for i := 0; i < bins; i++ {
			f := 860 + 140*float64(i)/(bins-1)
			// A noise floor with a fixed ripple, so that the traces do not
			// look like copies of one another.
			p := -92 + 3*math.Sin(float64(i)*0.7+float64(n)*0.3)
			p = math.Max(p, peak(f, carrier, -28, 1.6))
			p = math.Max(p, peak(f, harmonic, -58+22*t, 1.1))
			freq = append(freq, f)
			sweep = append(sweep, float64(n))
			power = append(power, p)
		}
	}
	return freq, sweep, power
}

// peak is one Lorentzian line, which is the shape a resonance actually has.
func peak(f, at, top, width float64) float64 {
	d := (f - at) / width
	return top - 10*math.Log10(1+d*d)
}

// traceNames turns the sweep number into the group column, because a group is
// a category rather than a number: two sweeps with the same number would be
// one trace, and that is exactly the identity a group is.
func traceNames(sweep []float64) []string {
	out := make([]string, len(sweep))
	for i, n := range sweep {
		out[i] = fmt.Sprintf("sweep %02d", int(n))
	}
	return out
}

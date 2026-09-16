// Command sphere renders three charts a spherical scene unlocks: an antenna
// radiation pattern, a qubit's state moving on the Bloch sphere, and an
// oscillator's impedance leaving the Smith chart on the Smith sphere.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. Neither chart has a mark of its own. The
// pattern is three.Surface over (azimuth, polar angle) with the gain as the
// radius; the Bloch sphere is three.Line3 and three.Scatter3 through unit
// radii. What makes them what they are is one scene option, three.Spherical —
// the coordinate system docs/adr/0058-what-3d-is-for.md ranks third, and the
// reason one piece of machinery is worth five audiences. The third is the same
// scene under three.Smith, which reads a resistance and a reactance where the
// others read two angles — the test that the sphere is a coordinate system.
//
//	go run ./examples/sphere
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
	pattern := flag.String("pattern", "pattern.svg", "output path for the radiation pattern")
	bloch := flag.String("bloch", "bloch.svg", "output path for the Bloch sphere")
	smith := flag.String("smith", "smith.svg", "output path for the Smith sphere")
	flag.Parse()
	if err := run(*pattern, *bloch, *smith); err != nil {
		fmt.Fprintln(os.Stderr, "sphere:", err)
		os.Exit(1)
	}
}

func run(pattern, bloch, smith string) error {
	if err := radiation(pattern); err != nil {
		return err
	}
	if err := rabi(bloch); err != nil {
		return err
	}
	return oscillator(smith)
}

// impedance is the normalised input impedance of a one-port with a negative
// resistance near its resonance, at angular frequency w relative to it: the
// device an oscillator is built from. Away from resonance it is an ordinary
// lossy resonator; near it the resistance dips below zero, which is the whole
// point of the device and exactly what a flat Smith chart cannot show.
func impedance(w float64) (r, x float64) {
	r = 1.2 - 1.8*math.Exp(-math.Pow((w-1)/0.25, 2))
	x = 2 * (w - 1/w)
	return r, x
}

// oscillator draws the sweep on the Smith sphere. It starts near the short,
// climbs the northern hemisphere as a passive load does, crosses the equator
// where the resistance goes negative, runs through the southern hemisphere —
// off the page of a flat chart — and comes back over the equator when the
// resistance recovers.
func oscillator(out string) error {
	var r, x, one, crossR, crossX []float64
	prev := 0.0
	for k := 0; k <= 400; k++ {
		w := 0.25 + 2.75*float64(k)/400
		ri, xi := impedance(w)
		r, x, one = append(r, ri), append(x, xi), append(one, 1)
		if k > 0 && (prev < 0) != (ri < 0) {
			crossR, crossX = append(crossR, ri), append(crossX, xi)
		}
		prev = ri
	}
	sweep := figure.NewTable().Float64("r", r).Float64("x", x).Float64("one", one)
	marks := figure.NewTable().Float64("r", crossR).Float64("x", crossX).
		Float64("one", fill(len(crossR), 1))

	sc := three.NewScene(three.Spherical(three.Smith())).
		Z(scale.Linear(scale.Domain(0, 1))).
		Add(
			three.Line3(sweep, geom.X("r"), geom.Y("x"), geom.Z("one"),
				geom.Color(palette.Blue), geom.Width(2), geom.Label("z(ω)")),
			three.Scatter3(marks, geom.X("r"), geom.Y("x"), geom.Z("one"),
				geom.Color(palette.Vermilion), geom.Size(8), geom.Droplines(false),
				geom.Label("r = 0")),
		)
	return three.New(
		three.Size(560, 560),
		three.Title("A negative resistance, on the Smith sphere"),
		three.Theme(theme.Light),
	).Scene(sc).Render(figure.SVG(out))
}

func fill(n int, v float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

// The array: four isotropic elements along x, half a wavelength apart, over a
// ground plane that removes the lower hemisphere's back radiation.
const (
	elements = 4
	spacing  = 0.5 // wavelengths
)

// gain is the array's normalised power pattern at polar angle theta from +z
// and azimuth phi, both in degrees.
//
// It is the array factor of a uniform linear array along x times a cosine
// element pattern — the textbook broadside array, whose main lobe stands up
// from the ground plane and fans out across the axis the elements lie along.
func gain(phi, theta float64) float64 {
	th, ph := theta*math.Pi/180, phi*math.Pi/180
	psi := 2 * math.Pi * spacing * math.Sin(th) * math.Cos(ph)
	af := 1.0
	if s := math.Sin(psi / 2); math.Abs(s) > 1e-9 {
		af = math.Abs(math.Sin(elements*psi/2) / (elements * s))
	}
	el := math.Max(math.Cos(th), 0)
	return (af * el) * (af * el)
}

// radiation draws the pattern as a surface on the sphere: the radius in every
// direction is the power radiated that way. The two cut planes a datasheet
// prints are slices of this, and they are slices because the page is flat.
func radiation(out string) error {
	var phi, theta, g []float64
	for t := 0; t <= 180; t += 5 {
		for p := 0; p < 360; p += 5 {
			phi, theta = append(phi, float64(p)), append(theta, float64(t))
			g = append(g, gain(float64(p), float64(t)))
		}
	}
	src := figure.NewTable().Float64("phi", phi).Float64("theta", theta).Float64("gain", g)

	sc := three.NewScene(three.Spherical(three.AxisEnds("x", "", "y", "", "z", ""))).
		Z(scale.Linear(scale.Domain(0, 1))).
		Add(three.Surface(src, geom.X("phi"), geom.Y("theta"), geom.Z("gain"),
			geom.ColorBy("gain", scale.Sequential(palette.Viridis))))

	return three.New(
		three.Size(620, 560),
		three.Title("A four-element broadside array"),
		three.Theme(theme.Light),
	).Scene(sc).Render(figure.SVG(out))
}

// rabi draws a qubit driven off resonance: its state starts at |0⟩ and spirals
// out towards the equator and back as the drive rotates it about a tilted axis
// while it precesses. On the sphere that is one closed curve; as three
// expectation values against time it is three sinusoids nobody can see the
// curve in.
func rabi(out string) error {
	const (
		detuning = 0.6 // of the Rabi frequency
		steps    = 240
	)
	omega := math.Hypot(1, detuning)
	// The rotation axis, tilted from x towards z by the detuning.
	ax, az := 1/omega, detuning/omega

	var phi, theta, r []float64
	for k := 0; k <= steps; k++ {
		t := 2 * math.Pi * float64(k) / steps
		// Rodrigues' rotation of |0⟩ = (0, 0, 1) about k = (ax, 0, az) by t:
		// v·cos t + (k × v)·sin t + k·(k·v)·(1 − cos t), with k × v = (0, −ax, 0)
		// and k·v = az.
		c, s := math.Cos(t), math.Sin(t)
		x := ax * az * (1 - c)
		y := -ax * s
		z := c + az*az*(1-c)
		phi = append(phi, math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360))
		theta = append(theta, math.Acos(math.Max(-1, math.Min(1, z)))*180/math.Pi)
		r = append(r, 1)
	}
	path := figure.NewTable().Float64("phi", phi).Float64("theta", theta).Float64("r", r)
	ends := figure.NewTable().
		Float64("phi", []float64{phi[0], phi[steps/2]}).
		Float64("theta", []float64{theta[0], theta[steps/2]}).
		Float64("r", []float64{1, 1})

	// The kets are written with the mathematical angle brackets they are
	// written with in print. This example renders SVG, where the viewer's own
	// fonts supply the glyph; a PNG of the same chart needs a font that has it,
	// which is what gg.WithFallbackFont is for — the embedded Go fonts do not
	// cover U+27E8 and U+27E9. See docs/adr/0038-embedded-fonts.md.
	sc := three.NewScene(three.Spherical(three.AxisEnds("|+⟩", "|−⟩", "|+i⟩", "|−i⟩", "|0⟩", "|1⟩"))).
		Z(scale.Linear(scale.Domain(0, 1))).
		Add(
			three.Line3(path, geom.X("phi"), geom.Y("theta"), geom.Z("r"),
				geom.Color(palette.Blue), geom.Width(2), geom.Label("state")),
			three.Scatter3(ends, geom.X("phi"), geom.Y("theta"), geom.Z("r"),
				geom.Color(palette.Vermilion), geom.Size(9), geom.Label("start, half period")),
		)

	return three.New(
		three.Size(560, 560),
		three.Title("A detuned Rabi oscillation"),
		three.Theme(theme.Light),
	).Scene(sc).Render(figure.SVG(out))
}

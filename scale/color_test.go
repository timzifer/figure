package scale_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

func TestSequentialRunsTheRampAcrossTheDomain(t *testing.T) {
	c := scale.Sequential(palette.Viridis)
	c.Train(0, 100)
	if got, want := c.Color(0), palette.Viridis[0]; got != want {
		t.Errorf("Color(domain min) = %v, want the first ramp colour %v", got, want)
	}
	if got, want := c.Color(100), palette.Viridis[len(palette.Viridis)-1]; got != want {
		t.Errorf("Color(domain max) = %v, want the last ramp colour %v", got, want)
	}
}

func TestSequentialClampsOutsideTheDomain(t *testing.T) {
	c := scale.Sequential(palette.Blues, scale.ColorDomain(0, 10))
	if c.Color(-5) != c.Color(0) || c.Color(99) != c.Color(10) {
		t.Error("values outside the domain must clamp to its ends, not wrap or extrapolate")
	}
	c.Train(-1000, 1000)
	if lo, hi := c.Domain(); lo != 0 || hi != 10 {
		t.Errorf("Domain() = (%v, %v); ColorDomain must disable training", lo, hi)
	}
}

func TestDivergingKeepsTheCentreInTheMiddleOfTheRamp(t *testing.T) {
	// An asymmetric domain: the centre must still land on the ramp's middle,
	// and the short arm must not reach the ramp's end.
	c := scale.Diverging(nil, scale.ColorCenter(0))
	c.Train(-20, 100)

	mid := palette.BlueOrange.At(0.5)
	if got := c.Color(0); got != mid {
		t.Errorf("Color(centre) = %v, want the middle of the ramp %v", got, mid)
	}
	if got, want := c.Color(100), palette.BlueOrange.At(1); got != want {
		t.Errorf("Color(far end) = %v, want %v", got, want)
	}
	if c.Color(-20) == palette.BlueOrange.At(0) {
		t.Error("the short arm reached the end of the ramp; both arms must share one scale")
	}
}

func TestColorReverseFlipsTheRamp(t *testing.T) {
	fwd := scale.Sequential(palette.Viridis, scale.ColorDomain(0, 1))
	rev := scale.Sequential(palette.Viridis, scale.ColorDomain(0, 1), scale.ColorReverse())
	if fwd.Color(0) != rev.Color(1) || fwd.Color(1) != rev.Color(0) {
		t.Error("ColorReverse did not flip the ramp")
	}
}

func TestColorUndefinedValuesGetTheUndefinedColour(t *testing.T) {
	c := scale.Sequential(nil, scale.ColorDomain(0, 1))
	if got := c.Color(math.NaN()); got != ir.Transparent {
		t.Errorf("Color(NaN) = %v, want transparent by default", got)
	}
	grey := ir.RGB(0x80, 0x80, 0x80)
	c = scale.Sequential(nil, scale.ColorDomain(0, 1), scale.ColorUndefined(grey))
	if got := c.Color(math.NaN()); got != grey {
		t.Errorf("Color(NaN) = %v, want %v", got, grey)
	}
}

func TestColorConstantDomainDoesNotDivideByZero(t *testing.T) {
	c := scale.Sequential(palette.Viridis)
	c.Train(5, 5)
	if got := c.Color(5); got.A == 0 {
		t.Error("a constant domain must still produce a colour")
	}
}

// Training a colour scale twice is training it once, which is the property two
// charts of one table rest on: a flat contour and a projected surface each
// train the ramp they share, and a colour has to mean one number in both.
//
// It is checked through Color as well as through the domain, because the domain
// is only interesting as the thing that decides the ink — and over every kind
// of scale this package builds, because the property belongs to the contract
// rather than to one implementation.
func TestTrainingAColourScaleTwiceIsTrainingItOnce(t *testing.T) {
	values := []float64{-4, -1, 0, 2.5, 7}
	for _, tc := range []struct {
		name string
		make func() scale.ColorScale
	}{
		{"sequential", func() scale.ColorScale { return scale.Sequential(palette.Viridis) }},
		{"diverging", func() scale.ColorScale { return scale.Diverging(palette.BlueOrange) }},
		{"reversed", func() scale.ColorScale {
			return scale.Sequential(palette.Viridis, scale.ColorReverse())
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			once, twice := tc.make(), tc.make()
			once.Train(values...)
			twice.Train(values...)
			twice.Train(values...)

			if a, b := once.Domain(); true {
				if c, d := twice.Domain(); a != c || b != d {
					t.Errorf("one training gives [%v, %v] and two give [%v, %v]", a, b, c, d)
				}
			}
			for _, v := range values {
				if once.Color(v) != twice.Color(v) {
					t.Errorf("%v is %v after one training and %v after two", v, once.Color(v), twice.Color(v))
				}
			}
		})
	}
}

// A pinned domain ignores training altogether, which is the spelling to reach
// for when two charts must agree whatever each of them was handed: two subsets
// of one table train to two domains, and the same colour then means two
// different numbers.
func TestAPinnedColourDomainIgnoresTraining(t *testing.T) {
	s := scale.Sequential(palette.Viridis, scale.ColorDomain(-10, 10))
	before := s.Color(0)
	s.Train(-1000, 1000, 3)
	if lo, hi := s.Domain(); lo != -10 || hi != 10 {
		t.Errorf("a pinned domain moved to [%v, %v]", lo, hi)
	}
	if got := s.Color(0); got != before {
		t.Errorf("the colour of 0 moved from %v to %v", before, got)
	}
}

package stilus

import (
	"math"
	"slices"

	"github.com/timzifer/figure/ir"
	st "github.com/timzifer/stilus"
)

// rampSize is the number of entries a gradient's colours are sampled into.
// stilus looks an evenly spaced ramp up by nearest entry, and 256 steps is
// below what eight bits a channel can show between two neighbours.
const rampSize = 256

// rampKey is the gradient the backend's ramp was last built for, so that a
// chart filling many shapes with one gradient — a colourbar, a legend
// swatch per frame — builds it once.
type rampKey struct {
	stops []ir.GradientStop
}

// gradient sets the backend's linear gradient to fill, and reports whether it
// can be drawn.
//
// The colours between two stops are blended in linear light rather than in
// the encoded bytes, which is gg's rule for a gradient and palette.Lerp's for
// a ramp: averaging sRGB bytes is about a fifth too dark at the midpoint and
// shows as a band. stilus interpolates its own knots in the encoded values, so
// the ramp is sampled here, evenly, and handed over without knots.
func (b *backend) gradient(fill ir.Fill) bool {
	g := &b.grad
	if !slices.Equal(b.ramp.stops, fill.Stops) || len(g.Ramp) != rampSize {
		b.ramp.stops = append(b.ramp.stops[:0], fill.Stops...)
		if cap(g.Ramp) < rampSize {
			g.Ramp = make(st.Ramp, rampSize)
		}
		g.Ramp = g.Ramp[:rampSize]
		for i := range g.Ramp {
			c := sample(fill.Stops, float32(i)/(rampSize-1))
			g.Ramp[i] = st.PackRGBA(premul(c))
		}
	}
	g.Knots = g.Knots[:0]
	g.Extend = [2]bool{true, true}
	g.Alpha = 255
	return g.Set(float64(fill.Start.X), float64(fill.Start.Y), float64(fill.End.X), float64(fill.End.Y), b.top())
}

// sample is the gradient's colour at t: the first stop's before it, the last
// stop's after it, and between two stops a blend in linear light. Stops are
// taken in the order given, which is the order SVG reads them in.
func sample(stops []ir.GradientStop, t float32) ir.Color {
	if t <= stops[0].Offset {
		return stops[0].Color
	}
	for i := 1; i < len(stops); i++ {
		a, z := stops[i-1], stops[i]
		if t > z.Offset {
			continue
		}
		span := z.Offset - a.Offset
		if span <= 0 {
			return z.Color // a hard stop
		}
		return mix(a.Color, z.Color, float64((t-a.Offset)/span))
	}
	return stops[len(stops)-1].Color
}

// mix blends two straight-alpha colours in linear light, alpha linearly.
func mix(a, z ir.Color, f float64) ir.Color {
	ch := func(x, y uint8) uint8 {
		v := decode(x) + (decode(y)-decode(x))*f
		return encode(v)
	}
	return ir.Color{
		R: ch(a.R, z.R),
		G: ch(a.G, z.G),
		B: ch(a.B, z.B),
		A: uint8(math.Round(float64(a.A) + (float64(z.A)-float64(a.A))*f)),
	}
}

// decode is an sRGB byte in linear light, 0 to 1.
func decode(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// encode is a linear-light value as an sRGB byte.
func encode(v float64) uint8 {
	v = min(max(v, 0), 1)
	if v <= 0.0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return uint8(math.Round(v * 255))
}

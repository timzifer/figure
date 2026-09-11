package three

import (
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/theme"
)

// highlight caps how far a face turned into the light is lifted toward
// [theme.Theme.DepthTop]. A surface is a measurement and a lit face that went
// all the way to the theme's brightest ink would read as a blown-out
// photograph of one.
const highlight = 0.3

// pivot is where a face's own colour is drawn undisturbed. Most of a height
// field faces mostly upward and mostly toward the light, so putting the pivot
// three quarters of the way up is what makes a surface look like its own
// colour rather than like a mix of two others.
const pivot = 0.75

// shade darkens a face as it turns away from the light.
//
// There is one directional source and no model beyond it: no specular, no
// shadows, no ambient occlusion, no textures. The face's normal against the
// theme's light gives a number, the theme's floor lifts that off zero so that
// the far side of a surface reads as a surface rather than as a hole in one,
// and the colour is mixed along it between the theme's two depth shades.
//
// The mixing is [palette.Lerp], which decodes sRGB, blends and re-encodes.
// Averaging the encoded bytes instead is about a fifth too dark at the
// midpoint, and on a shaded surface that shows as a band across the slope.
func shade(th theme.Theme, base ir.Color, n Vec3) ir.Color {
	lit := n.Unit().Dot(lightOf(th))
	if lit < 0 {
		lit = 0
	}
	floor := th.LightFloor
	if floor < 0 || floor > 1 {
		floor = 0
	}
	t := floor + (1-floor)*lit
	if t < pivot {
		return palette.Lerp(th.DepthSide, base, t/pivot)
	}
	return palette.Lerp(base, th.DepthTop, (t-pivot)/(1-pivot)*highlight)
}

// defaultLight is where the light comes from for a theme built by hand rather
// than by [theme.Build] — over the reader's left shoulder and from above,
// which is where every reader has been taught light comes from.
var defaultLight = Vec3{-0.4, -0.6, 0.7}.Unit()

func lightOf(th theme.Theme) Vec3 {
	v := Vec3{th.LightDir[0], th.LightDir[1], th.LightDir[2]}
	if v == (Vec3{}) {
		return defaultLight
	}
	return v.Unit()
}

// faceNormal is the outward normal of a face given three of its corners in
// order around it.
func faceNormal(a, b, c Vec3) Vec3 { return b.Sub(a).Cross(c.Sub(b)).Unit() }

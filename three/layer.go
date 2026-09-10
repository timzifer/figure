package three

import (
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// Layer is one set of marks in a scene.
//
// It is deliberately not a [github.com/timzifer/figure/geom.Geom]. A geom's
// Build streams ink straight into the backend, which makes a layer a paint
// unit — and a projected scene has no paint unit smaller than the view, since
// a point can be in front of one part of a surface and behind another. So a
// layer here emits primitives and this package projects, orders and paints
// what every layer emitted, once, over all of them at a time.
//
// The separation is a guard as well as a consequence. A geom handed a depth
// would ignore it and draw a flat line inside a projected box — correct by its
// own lights, wrong by the chart's, and silent either way. Here that cannot
// happen: geom.Line is not a Layer and does not compile into a [Scene].
//
// # Stability
//
// Layer is implemented outside this module, so it never gains a method, and
// each method takes one parameter beyond its destination. What a layer can
// additionally do arrives as an optional interface beside it. ADR 0060.
type Layer interface {
	// Train feeds the layer's data into the three scales so they can
	// establish their domains. It runs before layout, because layout needs
	// tick labels and tick labels need a domain.
	//
	// It runs once per frame however many views are drawn, because a domain is
	// a fact about the data and not about where the data is looked at from.
	// The struct is geom's because that is the seam ADR 0056 widened for
	// exactly this: it gained a Z, and every two-dimensional geom kept
	// compiling.
	Train(t geom.Training) error

	// Emit appends the layer's primitives, in scene space, to s.
	//
	// It runs once per view, because which order a layer emits in is a fact
	// about the camera: a surface walks its lattice from the corner the view
	// direction picks.
	Emit(s *Sink, f Frame) error

	// Legend returns the entry this layer contributes, or ok == false if it
	// should not appear in one.
	Legend(f Frame) (geom.LegendEntry, bool)
}

// Frame is everything a layer needs in order to turn its data into
// primitives. It is [github.com/timzifer/figure/geom.Frame]'s counterpart, and
// the difference between them is the whole of [Layer]'s doc comment: this one
// carries no backend, because a layer here does not draw.
type Frame struct {
	// X, Y and Z are the trained scales, ranged into the unit cube.
	X, Y, Z scale.Scale
	// Theme supplies the defaults the layer did not override, and the light a
	// shaded face is lit by.
	Theme theme.Theme
	// Index is the layer's position in the scene, which is what picks its
	// default colour out of the palette.
	Index int
	// View is which view is being emitted for, and Forward the unit direction
	// its camera looks along. A layer that walks its own geometry in depth
	// order reads Forward: a surface's traversal is the signs of its three
	// components and nothing else.
	View    int
	Forward Vec3
	// Rows, when non-nil, collects which source row is behind each mark. It is
	// nil for an ordinary render, and a layer reports rows by naming them on
	// the sink rather than by calling anything here — see [Sink.Row].
	Rows geom.Rows
}

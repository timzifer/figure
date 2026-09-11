package three

import (
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// Overlay paints over a finished figure: a ring round the row another view is
// pointing at, a leader line out to a caption, a mark a host placed itself.
//
// # Why it is not a layer
//
// The reason is [github.com/timzifer/figure/render.Overlay]'s — a layer's
// positions come from its data through the scales, and an overlay's come from
// somewhere a layer has no access to: a pointer, a selection, or where the
// previous frame put something. This package adds a second reason of its own.
// An overlay is not in the depth order, and that is deliberate rather than a
// shortfall: its positions have no depth to be sorted by, and a mark inside the
// order would be occluded by the very surface it exists to point at.
//
// It is announced to no [github.com/timzifer/figure/render.Observer] and is
// therefore not hit-testable, which is the property that makes it usable — a
// ring a pointer can hit is a ring that flickers, because hovering it moves the
// pointer off whatever the ring was about.
//
// # Why it is not render's type
//
// The method and its shape are render's, spelled again here rather than shared,
// which is the trade drawStrip already makes: the two are the same idea and not
// the same drawing. A flat overlay is told each panel's X and Y scales and the
// coord that framed them, because that is how a crosshair finds a threshold on
// screen. A view is told three scales, a [Camera] and a [Projection] — and a
// projected scene announces its panels with X and Y nil on purpose, because
// there are no screen axes to invert a device point through. Sharing the type
// would mean a struct half of whose fields are nil in one of the two places it
// is used, a caller having to know which half, and
// [github.com/timzifer/figure.Crosshair] compiling against a scene it can say
// nothing about.
//
// # What it costs
//
// A figure with no overlay pays nothing. One that only moves is a damage
// rectangle like anything else, because [Live.Draw] compares the frame it just
// recorded with the last whenever no camera moved. But a *turn* with one
// installed repaints the whole canvas rather than the cells that turned — an
// overlay draws where it likes, so no cell list describes what it damaged — so
// install feedback for the gesture it belongs to and take it away afterwards.
//
// An Overlay is implemented outside this package, so it never gains a method.
type Overlay interface {
	// DrawOverlay paints over the finished figure.
	DrawOverlay(b ir.Backend, f OverlayFrame)
}

// Projection turns a point of the scene into a point on the surface.
//
// It is a value copied into every [OverlayView] rather than a function field or
// an interface, and the reason is the allocation gate: a closure is an
// allocation per view per frame, and boxing one into an interface is another. A
// value is neither, and it is comparable, so a caller can ask cheaply whether a
// view's projection moved.
//
// There is no exported matrix, for the reason ADR 0057 gives: a matrix is a
// second way to build a camera — one that can be sheared, non-orthonormal or
// perspective, all of which this package refuses.
type Projection struct{ p projector }

// Point projects a scene-space point: a [Vec3] in the unit cube, which is what
// a [Layer] emits into a [Sink]. Reach for [OverlayView.At] to start from a
// value instead.
func (pj Projection) Point(v Vec3) ir.Point { return pj.p.point(v) }

// Depth reports how far a scene point is from the camera along the view
// direction. Larger is farther, which is the order a painter walks.
func (pj Projection) Depth(v Vec3) float64 { return pj.p.depth(v) }

// Valid reports whether this projection came from a drawn view. The zero
// Projection maps every point to one place, so an overlay handed a frame it
// built itself checks this rather than drawing a pile of marks on top of each
// other.
func (pj Projection) Valid() bool { return pj.p.scale != 0 }

// OverlayView is one view of the frame an overlay draws over: where it is, what
// it looks at the scene from, and what turns a value into a place in it.
type OverlayView struct {
	// Index is the view's position in the figure, which is the index
	// [Live.ViewAt] reports, the one a
	// [github.com/timzifer/figure/render.Observer] was told, and the one
	// [github.com/timzifer/figure/interact.Hit] carries as its Panel.
	Index int
	// Label is the view's strip label, or "" for a view that has none — so an
	// overlay can say which camera it is annotating without holding the [View].
	Label string

	// Area is the view's cell: the rectangle the scene was clipped to and the
	// one [Live.ViewAt] matches against. An overlay is not clipped, so an
	// overlay that wants a clip pushes this.
	Area ir.Rect
	// Inner is the rectangle the cube was actually fitted into — Area less the
	// room the tick labels and axis titles need round it. It is what an overlay
	// placing a caption beside the cube measures against.
	Inner ir.Rect

	// Camera is where this view looks from. [Camera.Forward] is how an overlay
	// decides whether what it is ringing faces the reader.
	Camera Camera

	// X, Y and Z are the scene's three scales, which are ranged into the unit
	// cube rather than into Area: what turns a scene point into a device point
	// is the camera. They are the scene's own objects rather than copies, read
	// in the frame that drew them, and must not be modified.
	X, Y, Z scale.Scale

	// Project is the projection this view was drawn with.
	Project Projection
}

// At maps a point in the data through the view's three scales and projects it,
// which is what an overlay marking a value calls.
//
// ok is false when any of the three scales has no position for its value — a
// log axis has none for zero — so nothing is drawn at a coordinate that means
// nothing. A value outside a domain is clamped into the cube, which is what
// this package's own layers do with one.
func (v OverlayView) At(x, y, z float64) (ir.Point, bool) {
	if v.X == nil || v.Y == nil || v.Z == nil || !v.Project.Valid() {
		return ir.Point{}, false
	}
	if !defined(v.X, x) || !defined(v.Y, y) || !defined(v.Z, z) {
		return ir.Point{}, false
	}
	return v.Project.Point(Vec3{at(v.X, x), at(v.Y, y), at(v.Z, z)}), true
}

// OverlayFrame is what an overlay is told about the figure it draws over.
type OverlayFrame struct {
	// Canvas is the whole drawing, in device space.
	Canvas ir.Rect
	// Views are the figure's views, in the order [Plot.Add] gave them.
	Views []OverlayView
	// Theme is the figure's theme, so that an overlay over a dark scene is
	// legible on it.
	Theme theme.Theme
}

// ViewAt reports which view contains a device point, for an overlay deciding
// which cell a pointer is in. It is [Live.ViewAt]'s answer, from inside a draw.
func (f OverlayFrame) ViewAt(pt ir.Point) (OverlayView, bool) {
	for _, v := range f.Views {
		if v.Area.Contains(pt) {
			return v, true
		}
	}
	return OverlayView{}, false
}

// View returns the view with the given index, for an overlay that was told one
// by a hit rather than by a position.
func (f OverlayFrame) View(i int) (OverlayView, bool) {
	for _, v := range f.Views {
		if v.Index == i {
			return v, true
		}
	}
	return OverlayView{}, false
}

// Point3 is a position in the data, which is what an overlay names when it
// wants a mark in every view.
//
// It is three float64 and not a [Vec3] because none of it is a scene
// coordinate: it is a value on each of three axes, and a view's scales are what
// turn it into geometry.
type Point3 struct{ X, Y, Z float64 }

// Mark is one place to draw a ring, and whether the reader can see what is
// there.
//
// Hidden is the half that is easy to leave out and wrong to. A projected scene
// occludes itself: the point a reader picked out may be on the far side of the
// surface it is on, and a ring drawn plainly over it says "this is here" when
// what is here is a piece of surface with the marked point somewhere behind it.
// The reader then reads a position off the near face that is not the position
// they picked.
type Mark struct {
	// At is where to draw, in device space — typically what
	// [github.com/timzifer/figure/interact.Index.Locate] answered.
	At ir.Point
	// Hidden says something in the scene is in front of this point, so the ring
	// is drawn dashed rather than solid.
	Hidden bool
}

// Highlight rings a set of marks, to say "these ones".
//
// It is the reason this seam exists. A hit on a projected scene reports which
// layer and which row and leaves its X and Y at zero — there is no pair of
// values to read back out of a turned cube — so a host that knows *which row* a
// reader picked has no way of its own to say where that row landed. It asks
// [github.com/timzifer/figure/interact.Index.Locate] once per view and rings
// every answer, and the same measurement is then marked in the three-quarter,
// the plan and both profiles at once.
//
// # A ring behind something is dashed
//
// A scene hides its own far side, so a mark may be behind the surface it
// belongs to. Such a ring is drawn dashed and the rest solid, which is the
// convention an engineering drawing has used for a hidden edge for as long as
// there have been engineering drawings — and this figure is already in that
// register, since ADR 0062's whole argument for several views is the plan and
// the two elevations a machinist reads.
//
// Dashed rather than inverted, and the reason is the same one that makes the
// rings an overlay at all: an inverted ring would have to know what colour the
// thing in front of it was drawn in, which means reading pixels back — and what
// it read would be the shade of one face rather than a statement about depth. A
// dash says the one thing that is true, which is that something is in front.
//
// Whether a mark is hidden is the host's to decide, because only the host has
// the hit index the answer comes out of. See [Mark].
//
// The zero value draws nothing. A Highlight holds the paths it drew last, so
// one belongs to one figure — install a second for a second.
type Highlight struct {
	// Marks are the places to ring, in device space, each saying whether it is
	// behind something. Each is drawn in the view whose Area contains it, or in
	// View when one is named; a mark in no view is not drawn.
	Marks []Mark
	// Data are points in the data, rung in every view. That is the default
	// rather than a setting: a figure with four cameras is one chart looked at
	// four ways, so a value marked in one of them is marked in all four.
	//
	// They are always drawn solid. A value is a place in the cube rather than
	// something the scene drew, so there is nothing for it to be behind — and a
	// caller who does mean a drawn mark has [Mark] and its answer.
	Data []Point3

	// View confines the rings to one view, or -1 for the rule above.
	//
	// Minus one is the useful setting and not the zero value, which is the
	// trade [github.com/timzifer/figure.Highlight] makes for the same reason:
	// nought is a real view, so a zero that meant "any" would leave no way to
	// say "the first".
	View int

	// Radius is the ring's radius in device units. Zero takes six, a little
	// larger than a default marker.
	Radius float32
	// Color and Width override the theme. A zero Color takes the theme's label
	// colour and a zero Width takes two device units — a ring wants to read as
	// an annotation rather than as data.
	Color ir.Color
	Width float32
	// Dash is the pattern a hidden ring is drawn with. A nil one takes a short
	// even dash, which is what a hidden edge is drawn with.
	Dash []float32

	// The rings, kept between frames rather than made per frame: a highlight is
	// a pointer the caller moves, so it is redrawn on every frame of whatever
	// gesture is moving it.
	shown, hidden ir.Path
}

// DrawOverlay implements [Overlay].
func (h *Highlight) DrawOverlay(b ir.Backend, f OverlayFrame) {
	if len(h.Marks) == 0 && len(h.Data) == 0 {
		return
	}
	r := orElseLen(h.Radius, 6)
	h.shown.Reset()
	h.hidden.Reset()
	// Reserved in one go, which is [ir.Path.Grow]'s own arithmetic: a selection
	// of any size costs two allocations on its first frame and none after.
	n := len(h.Marks) + len(h.Data)*len(f.Views)
	h.shown.Grow(n*ir.CircleOps, n*ir.CirclePts)

	for _, m := range h.Marks {
		v, ok := h.viewFor(f, m.At)
		if !ok || !v.Area.Contains(m.At) {
			continue
		}
		if m.Hidden {
			h.hidden.Circle(m.At, r)
		} else {
			h.shown.Circle(m.At, r)
		}
	}
	for _, p := range h.Data {
		for _, v := range f.Views {
			if h.View >= 0 && v.Index != h.View {
				continue
			}
			// A value outside the cube is clamped onto its face rather than
			// dropped, which is what every layer here does with one, so a ring
			// stays somewhere the reader can see it.
			if at, ok := v.At(p.X, p.Y, p.Z); ok {
				h.shown.Circle(at, r)
			}
		}
	}

	stroke := ir.Stroke{
		Color: orElseColor(h.Color, f.Theme.LabelColor),
		Width: orElseLen(h.Width, 2),
	}
	if !h.shown.Empty() {
		b.StrokePath(&h.shown, stroke)
	}
	if !h.hidden.Empty() {
		stroke.Dash = orElseDash(h.Dash, hiddenDash)
		b.StrokePath(&h.hidden, stroke)
	}
}

// hiddenDash is what a ring behind something is drawn with: short and even, the
// way a hidden edge is drawn.
var hiddenDash = []float32{3, 3}

// viewFor resolves which view a device point belongs to: the one the highlight
// named, or the one containing it.
func (h *Highlight) viewFor(f OverlayFrame, at ir.Point) (OverlayView, bool) {
	if h.View >= 0 {
		return f.View(h.View)
	}
	return f.ViewAt(at)
}

// Overlays draws several overlays in order, so that a figure can have more than
// one without either knowing about the other. Later ones are on top, and a nil
// member is skipped — so a caller may keep a fixed-length list and switch one
// off by clearing it.
type Overlays []Overlay

// DrawOverlay implements [Overlay].
func (os Overlays) DrawOverlay(b ir.Backend, f OverlayFrame) {
	for _, o := range os {
		if o != nil {
			o.DrawOverlay(b, f)
		}
	}
}

// orElseDash returns d, or fallback when the caller named none.
func orElseDash(d, fallback []float32) []float32 {
	if d == nil {
		return fallback
	}
	return d
}

// orElseColor returns c, or fallback when c is fully transparent — which is
// what a zero ir.Color is, and therefore what "the caller did not say" looks
// like.
func orElseColor(c, fallback ir.Color) ir.Color {
	if c.A == 0 {
		return fallback
	}
	return c
}

// orElseLen returns v, or fallback when v is not a positive length.
func orElseLen(v, fallback float32) float32 {
	if v <= 0 {
		return fallback
	}
	return v
}

// Overlay installs something to paint over the figure, and returns p so the
// call can be chained. Passing nil removes whatever was there.
//
// It is a method rather than an [Option] because an overlay is an attachment
// like [Plot.Observer] and [Plot.TrackRows] rather than a statement about how
// large the figure is or how it looks, which is the split [Plot] already draws.
//
// A [Live] opened from this plot inherits it, because [Plot.Live] copies the
// plot. The copy is also why the inheritance goes one way only: installing one
// on the Live afterwards does not reach the plot the caller still holds, and
// installing one on the plot afterwards does not reach the Live.
func (p *Plot) Overlay(o Overlay) *Plot { p.overlay = o; return p }

// CurrentOverlay reports what is painting over the figure, or nil.
func (p *Plot) CurrentOverlay() Overlay { return p.overlay }

// Overlay installs something to paint over the figure, and returns l so the
// call can be chained onto [Plot.Live]. Passing nil removes it.
//
// It takes effect on the next [Live.Draw]. An overlay is a pointer to a struct
// whose fields the caller then moves — a ring's positions, a selection — so
// installing it once and mutating it per event is the intended shape and there
// is nothing to re-install. But this package installs no handler and runs no
// loop, so mutating one does not redraw by itself: call [Live.Draw], as you do
// after [Live.Camera].
//
// # Cost
//
// A turn with an overlay installed repaints the whole canvas rather than the
// cells whose cameras moved, because an overlay draws where it likes and no
// cell list describes what it damaged. So install the feedback a gesture needs
// for as long as the gesture lasts and take it away afterwards: one kept
// installed through an orbit costs a full repaint on every frame of the drag.
// A figure with no overlay pays exactly what it paid before.
func (l *Live) Overlay(o Overlay) *Live {
	if l.p.overlay == o {
		return l
	}
	l.p.overlay = o
	// Installing or removing one changes how many calls a frame has, which
	// makes it not comparable with the last; say so rather than leaving the
	// damage diff to discover it.
	l.drawn = false
	return l
}

// CurrentOverlay reports what is painting over the figure, or nil.
func (l *Live) CurrentOverlay() Overlay { return l.p.overlay }

var (
	_ Overlay = (*Highlight)(nil)
	_ Overlay = Overlays(nil)
)

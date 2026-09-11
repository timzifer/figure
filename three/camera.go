package three

import (
	"math"

	"github.com/timzifer/figure/ir"
)

// Camera is where a scene is looked at from.
//
// It is an immutable value: [Orbit], [Dolly] and [Slerp] each take one and
// return another, so the same inputs always give the same frame and a camera
// is testable without a surface. That is what lets the host own the drag —
// this package installs no handler, opens no window and runs no loop — and it
// is also what lets several [View]s hold several cameras onto one [Scene].
//
// It is orthographic, and stays so. Under perspective the same value is taller
// at the front of the scene than at the back, and a chart is a measuring
// instrument first. See docs/adr/0057-orbiting-a-chart.md.
//
// The zero Camera is the front view at the default zoom. The angle a chart is
// designed at is [Home], and it is the one a static export and a reader who
// cannot drag both get, so it has to be readable on its own.
//
// A Camera is comparable, which is what lets a live chart ask cheaply whether
// the reader moved it.
type Camera struct {
	az, el, zoom float64
}

// CameraOption configures a [Camera].
type CameraOption func(*Camera)

// Azimuth turns the camera about the up axis, in radians.
func Azimuth(rad float64) CameraOption { return func(c *Camera) { c.az = wrapAngle(rad) } }

// Elevation raises the camera above the floor plane, in radians. It is clamped
// just inside the poles: a camera looking straight down its own up-vector has
// no up-vector, and the scene would spin on its axis at the moment the reader
// least expects it.
func Elevation(rad float64) CameraOption { return func(c *Camera) { c.el = clampElevation(rad) } }

// Zoom sets how much of the view's cell the scene fills. One is the default
// fit and larger fills more.
func Zoom(f float64) CameraOption { return func(c *Camera) { c.zoom = clampZoom(f) } }

// LookAt builds a camera from its angles.
//
//	cam := three.LookAt(three.Azimuth(-0.6), three.Elevation(0.35))
func LookAt(opts ...CameraOption) Camera {
	c := Camera{zoom: 1}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// Home is the three-quarter view a scene is drawn at when nobody chose.
//
// ADR 0057 makes this a requirement rather than a default: a scene whose
// initial angle hides its own data behind itself is broken for everyone who
// cannot drag it, and for every static export — which is most of them.
// [Live.Home] is one call back to it.
func Home() Camera { return LookAt(Azimuth(-0.6), Elevation(0.35)) }

// Azimuth reports the camera's rotation about the up axis, in radians.
func (c Camera) Azimuth() float64 { return c.az }

// Elevation reports the camera's height above the floor plane, in radians.
func (c Camera) Elevation() float64 { return c.el }

// Zoom reports how much of a cell the scene fills.
func (c Camera) Zoom() float64 { return c.resolve().zoom }

// resolve reads the zero value as the front view at the default zoom, so that
// a View nobody set a camera on still draws something.
func (c Camera) resolve() Camera {
	if c.zoom <= 0 {
		c.zoom = 1
	}
	return c
}

// Eye is the unit direction from the scene toward the camera.
func (c Camera) Eye() Vec3 {
	ce, se := math.Cos(c.el), math.Sin(c.el)
	return Vec3{
		float32(ce * math.Cos(c.az)),
		float32(ce * math.Sin(c.az)),
		float32(se),
	}
}

// Forward is the unit direction the camera looks along, in scene space.
//
// It is what a layer that orders its own primitives reads: a surface's
// back-to-front traversal is the sign of these three components and nothing
// else. Depth increases along it, so a point further from the camera has the
// larger key.
func (c Camera) Forward() Vec3 { e := c.Eye(); return Vec3{-e.X, -e.Y, -e.Z} }

// Orbit turns a camera by two angle deltas and returns the result.
//
// It is a pure function: same inputs, same camera, no state. The deltas are
// radians rather than pixels, because pixels per radian is a statement about
// how an interaction feels and that belongs to the host's input layer — as do
// inertia, momentum and springs, none of which are here. A host converts once,
// with its own constant:
//
//	const perPixel = 0.008 // radians; the host's choice, not figure's
//	live.Camera(three.Orbit(live.CameraValue(), -dx*perPixel, dy*perPixel))
//
// The signs are the ones a reader expects of taking hold of the scene: the side
// facing them follows the pointer. The azimuth grows anticlockwise seen from
// above, so a drag to the right, which carries the camera the other way, is a
// falling azimuth; device y grows downward, so a drag down lifts the camera.
//
// Elevation is clamped just inside the poles; azimuth wraps.
func Orbit(c Camera, dAz, dEl float64) Camera {
	c = c.resolve()
	c.az = wrapAngle(c.az + dAz)
	c.el = clampElevation(c.el + dEl)
	return c
}

// Dolly multiplies a camera's zoom and returns the result.
//
// Under an orthographic camera the eye's distance from the scene changes
// nothing at all — not weakly, exactly nothing — so what a reader means by
// dollying here is a scale factor. That is why there is no Distance: a knob
// that does nothing is worse than no knob.
func Dolly(c Camera, by float64) Camera {
	c = c.resolve()
	c.zoom = clampZoom(c.zoom * by)
	return c
}

// Slerp interpolates between two cameras and is how a host eases an orbit.
//
// A camera has no rows, no keys and no domain, so interpolating between two of
// them is interpolating a view — which is exactly the tweening ADR 0044
// refused for data, and is trivially correct for a view, because a view has no
// identity to lose. So easing a camera is a loop over this function and the
// easings in the root package, and it does not go anywhere near a transition:
// animating the camera and animating the data are different features that
// happen to both produce frames.
//
// Azimuth takes the short way round. t at or below zero returns a exactly and
// t at or above one returns b exactly, which is what a golden test of an
// eased orbit's last frame depends on.
func Slerp(a, b Camera, t float64) Camera {
	a, b = a.resolve(), b.resolve()
	switch {
	case t <= 0 || math.IsNaN(t):
		return a
	case t >= 1:
		return b
	}
	return Camera{
		az:   wrapAngle(a.az + wrapAngle(b.az-a.az)*t),
		el:   clampElevation(a.el + (b.el-a.el)*t),
		zoom: clampZoom(a.zoom * math.Pow(b.zoom/a.zoom, t)),
	}
}

// maxElevation is a hair short of the pole. Past it the camera's up-vector and
// its view direction are the same line and the scene has no orientation.
const maxElevation = math.Pi/2 - 1e-3

// The zoom is clamped at both ends so that a runaway wheel cannot reduce a
// scene to a dot or blow it past the cell into nothing.
const (
	minZoom = 0.1
	maxZoom = 10
)

func clampElevation(rad float64) float64 {
	switch {
	case math.IsNaN(rad):
		return 0
	case rad > maxElevation:
		return maxElevation
	case rad < -maxElevation:
		return -maxElevation
	}
	return rad
}

func clampZoom(f float64) float64 {
	switch {
	case math.IsNaN(f) || f < minZoom:
		return minZoom
	case f > maxZoom:
		return maxZoom
	}
	return f
}

// wrapAngle brings an angle into (-pi, pi], which is what makes the difference
// of two azimuths the short way round.
func wrapAngle(rad float64) float64 {
	if math.IsNaN(rad) || math.IsInf(rad, 0) {
		return 0
	}
	for rad > math.Pi {
		rad -= 2 * math.Pi
	}
	for rad <= -math.Pi {
		rad += 2 * math.Pi
	}
	return rad
}

// projector turns a scene point into a device point.
//
// It is the whole of the projection: three orthonormal directions and an
// affine fit into a rectangle. There is no exported matrix, because a matrix
// is a second way to build a camera — one that can be sheared, non-orthonormal
// or perspective, all of which this package refuses.
type projector struct {
	right, up, fwd Vec3
	scale          float32
	at             ir.Point
}

// project fits the unit cube into area at cam's angles.
//
// The scale comes from the cube's own diameter rather than from its projected
// bounding box, so a scene keeps its size while it turns. See [diameter].
func project(cam Camera, area ir.Rect) projector {
	cam = cam.resolve()
	eye := cam.Eye()
	// right is horizontal in scene space by construction, which is what keeps
	// the floor's horizon level however far the camera rises.
	right := Vec3{-float32(math.Sin(cam.az)), float32(math.Cos(cam.az)), 0}
	up := eye.Cross(right)

	side := area.Dx()
	if h := area.Dy(); h < side {
		side = h
	}
	return projector{
		right: right,
		up:    up,
		fwd:   Vec3{-eye.X, -eye.Y, -eye.Z},
		scale: side / diameter * float32(cam.zoom),
		at:    ir.Point{X: (area.Min.X + area.Max.X) / 2, Y: (area.Min.Y + area.Max.Y) / 2},
	}
}

// point projects one scene point into device space.
func (p projector) point(v Vec3) ir.Point {
	d := v.Sub(centre)
	return ir.Point{
		X: p.at.X + p.scale*float32(d.Dot(p.right)),
		// Device Y grows downward and the scene's up does not, which is the
		// one sign in the whole projection.
		Y: p.at.Y - p.scale*float32(d.Dot(p.up)),
	}
}

// depth reports how far a scene point is from the camera along the view
// direction. Larger is farther, so a painter walks the primitives in
// decreasing order of it.
func (p projector) depth(v Vec3) float64 { return v.Sub(centre).Dot(p.fwd) }

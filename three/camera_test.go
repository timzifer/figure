package three

import (
	"math"
	"testing"

	"github.com/timzifer/figure/ir"
)

// A camera is a value, and every verb over it is a pure function: that is the
// whole of ADR 0057's decision, and it is what lets the arithmetic be tested
// without a surface.
func TestOrbitIsPure(t *testing.T) {
	before := Home()
	after := Orbit(before, 0.3, -0.1)

	if before != Home() {
		t.Errorf("Orbit modified its argument: %+v, want %+v", before, Home())
	}
	if after == before {
		t.Error("Orbit returned the camera it was given")
	}
	// Same inputs, same camera. A frame that depended on how many times the
	// reader had dragged before would not be golden-testable.
	if again := Orbit(Home(), 0.3, -0.1); again != after {
		t.Errorf("Orbit is not a function: %+v then %+v", after, again)
	}
}

// The poles are where an orthographic camera loses its up-vector, so elevation
// is clamped just inside them — at both ends, because a reader can drag down
// as easily as up.
func TestOrbitClampsAtBothPoles(t *testing.T) {
	for _, d := range []float64{+10, -10} {
		got := Orbit(Home(), 0, d).Elevation()
		if math.Abs(got) > maxElevation {
			t.Errorf("orbiting by %v reached elevation %v, past the clamp at %v",
				d, got, maxElevation)
		}
		if math.Abs(math.Abs(got)-maxElevation) > 1e-12 {
			t.Errorf("orbiting by %v stopped at %v, want the clamp at %v", d, got, maxElevation)
		}
	}
}

// Azimuth wraps rather than accumulating, so a reader who spins the scene
// twice round is at the same camera as one who did not.
func TestAzimuthWrapsIntoOneTurn(t *testing.T) {
	a := Orbit(LookAt(Azimuth(0)), 2*math.Pi, 0)
	if math.Abs(a.Azimuth()) > 1e-9 {
		t.Errorf("a full turn left the azimuth at %v, want 0", a.Azimuth())
	}
	if got := LookAt(Azimuth(3 * math.Pi)).Azimuth(); math.Abs(got-math.Pi) > 1e-9 {
		t.Errorf("three half-turns gave %v, want pi", got)
	}
}

// The endpoints are exact. An eased orbit's last frame is compared with a
// golden file drawn at the destination camera, and "almost b" would move
// every coordinate in it.
func TestSlerpHitsItsEndpointsExactly(t *testing.T) {
	a := LookAt(Azimuth(-1), Elevation(0.2), Zoom(1))
	b := LookAt(Azimuth(1.5), Elevation(-0.4), Zoom(2))

	if got := Slerp(a, b, 0); got != a {
		t.Errorf("Slerp at 0 gave %+v, want %+v", got, a)
	}
	if got := Slerp(a, b, 1); got != b {
		t.Errorf("Slerp at 1 gave %+v, want %+v", got, b)
	}
	// Out of range is clamped rather than extrapolated: a host's easing may
	// overshoot, and a camera past the pole is a camera with no up-vector.
	if got := Slerp(a, b, -3); got != a {
		t.Errorf("Slerp before 0 gave %+v, want %+v", got, a)
	}
	if got := Slerp(a, b, 4); got != b {
		t.Errorf("Slerp after 1 gave %+v, want %+v", got, b)
	}
}

// Turning from just west of due south to just east of it is a short step, not
// a lap of the scene.
func TestSlerpTakesTheShortWayRound(t *testing.T) {
	a := LookAt(Azimuth(math.Pi - 0.1))
	b := LookAt(Azimuth(-math.Pi + 0.1))
	mid := Slerp(a, b, 0.5).Azimuth()
	if math.Abs(math.Abs(mid)-math.Pi) > 1e-9 {
		t.Errorf("the midpoint is at %v, want the far side at ±pi: the interpolation went the long way", mid)
	}
}

func TestDollyClampsBothWays(t *testing.T) {
	if got := Dolly(Home(), 1e6).Zoom(); got != maxZoom {
		t.Errorf("dollying in hard gave %v, want the ceiling %v", got, maxZoom)
	}
	if got := Dolly(Home(), 1e-6).Zoom(); got != minZoom {
		t.Errorf("dollying out hard gave %v, want the floor %v", got, minZoom)
	}
}

// The zero value has to draw something: a View nobody set a camera on is a
// front view rather than a scene collapsed to a point.
func TestTheZeroCameraIsTheFrontView(t *testing.T) {
	var zero Camera
	if got := zero.Zoom(); got != 1 {
		t.Errorf("the zero camera zooms %v, want 1", got)
	}
	if got := zero.Azimuth(); got != 0 {
		t.Errorf("the zero camera's azimuth is %v, want 0", got)
	}
	// And it is not Home: the angle a chart is designed at is a decision, and
	// a zero value is the absence of one.
	if zero == Home() {
		t.Error("the zero camera is Home; then a caller cannot tell a chosen angle from an unset one")
	}
}

// Forward and Eye are opposite unit vectors, and depth grows along Forward.
// The whole depth order rests on that sentence.
func TestDepthGrowsAwayFromTheCamera(t *testing.T) {
	cam := Home()
	e, f := cam.Eye(), cam.Forward()
	if n := math.Sqrt(e.Dot(e)); math.Abs(n-1) > 1e-6 {
		t.Errorf("Eye has length %v, want a unit vector", n)
	}
	if d := e.Dot(f); math.Abs(d+1) > 1e-6 {
		t.Errorf("Eye dot Forward is %v, want -1: they are not opposite", d)
	}

	p := project(cam, ir.R(0, 0, 100, 100))
	near := centre.Add(e.Mul(0.4))
	far := centre.Sub(e.Mul(0.4))
	if p.depth(near) >= p.depth(far) {
		t.Errorf("the near point has depth %v and the far one %v: larger must mean farther",
			p.depth(near), p.depth(far))
	}
}

// The projection is orthonormal, which is what makes it a measurement rather
// than a picture: two equal lengths in the scene are two equal lengths on
// screen, whichever way the scene is turned.
func TestTheProjectionIsOrthonormal(t *testing.T) {
	for _, cam := range []Camera{Home(), LookAt(), LookAt(Azimuth(2), Elevation(-1.2))} {
		p := project(cam, ir.R(0, 0, 200, 200))
		for _, pair := range []struct {
			name string
			a, b Vec3
		}{
			{"right,up", p.right, p.up},
			{"right,fwd", p.right, p.fwd},
			{"up,fwd", p.up, p.fwd},
		} {
			if d := pair.a.Dot(pair.b); math.Abs(d) > 1e-6 {
				t.Errorf("%v: %s are not perpendicular (dot %v)", cam, pair.name, d)
			}
		}
		for _, v := range []Vec3{p.right, p.up, p.fwd} {
			if n := math.Sqrt(v.Dot(v)); math.Abs(n-1) > 1e-6 {
				t.Errorf("%v: a basis vector has length %v", cam, n)
			}
		}
	}
}

// A scene keeps its size while it turns. The alternative — fitting the
// projected bounding box every frame — makes the picture breathe under the
// reader's hand, and an instrument whose scale moves while it is being read is
// not one.
func TestTurningTheSceneDoesNotResizeIt(t *testing.T) {
	area := ir.R(0, 0, 300, 200)
	want := project(Home(), area).scale
	for az := -3.0; az < 3.0; az += 0.37 {
		for el := -1.4; el < 1.4; el += 0.31 {
			got := project(LookAt(Azimuth(az), Elevation(el)), area).scale
			if got != want {
				t.Fatalf("at azimuth %v elevation %v the scene is scaled %v, want %v",
					az, el, got, want)
			}
		}
	}
	// Dolly is the one thing that changes it, which is what it is for.
	if got := project(Dolly(Home(), 2), area).scale; got != 2*want {
		t.Errorf("dollying by two scaled %v, want %v", got, 2*want)
	}
}

// The whole cube fits inside the cell at every angle, because the sphere it is
// fitted to contains it.
func TestTheCubeFitsItsCellAtEveryAngle(t *testing.T) {
	area := ir.R(10, 20, 210, 220)
	corners := make([]Vec3, 0, 8)
	for _, x := range []float32{0, 1} {
		for _, y := range []float32{0, 1} {
			for _, z := range []float32{0, 1} {
				corners = append(corners, Vec3{x, y, z})
			}
		}
	}
	for az := -3.0; az < 3.0; az += 0.29 {
		for el := -1.5; el < 1.5; el += 0.23 {
			p := project(LookAt(Azimuth(az), Elevation(el)), area)
			for _, c := range corners {
				pt := p.point(c)
				if pt.X < area.Min.X || pt.X > area.Max.X || pt.Y < area.Min.Y || pt.Y > area.Max.Y {
					t.Fatalf("at azimuth %v elevation %v the corner %v projects to %v, outside %v",
						az, el, c, pt, area)
				}
			}
		}
	}
}

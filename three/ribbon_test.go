package three

import (
	"math"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

// noRows is a listener that wants row identity and does nothing with it,
// which is all a layer looks at: it reports rows when anybody is listening.
type noRows struct{}

func (noRows) Marks(geom.MarkRows) {}

// elbow is a path that turns a right angle in the floor plane, which is the
// shape a mitre either closes or leaves open.
func elbow() figure.Source {
	return figure.NewTable().
		Float64("x", []float64{0, 0.5, 0.5}).
		Float64("y", []float64{0.5, 0.5, 1}).
		Float64("z", []float64{0.5, 0.5, 0.5})
}

func ribbonScene(src figure.Source, opts ...geom.Option) *Scene {
	o := append([]geom.Option{geom.X("x"), geom.Y("y"), geom.Z("z")}, opts...)
	return NewScene().
		X(scale.Linear(scale.Domain(0, 1))).
		Y(scale.Linear(scale.Domain(0, 1))).
		Z(scale.Linear(scale.Domain(0, 1))).
		Add(Ribbon(src, o...))
}

// A ribbon is one quad per segment, not one per row and not one for the path:
// a band that spanned the scene would be drawn wholly in front of or wholly
// behind everything it crosses.
func TestARibbonIsOneFacePerSegment(t *testing.T) {
	s, _ := emitOne(t, ribbonScene(elbow()))
	if len(s.prims) != 2 {
		t.Fatalf("%d primitives over three rows, want two segments", len(s.prims))
	}
	for i, p := range s.prims {
		if p.kind != kindFace {
			t.Errorf("primitive %d is kind %d, want a face", i, p.kind)
		}
		if n := p.hi - p.lo; n != 4 {
			t.Errorf("primitive %d has %d corners, want a quad", i, n)
		}
	}
}

// The two quads meeting at a turn share their corners exactly. Each taking its
// own segment's width leaves a wedge open on the outside of the corner, which
// is the thing the mitre exists to close.
func TestARibbonsCornerIsOneBandRatherThanTwoQuads(t *testing.T) {
	s, _ := emitOne(t, ribbonScene(elbow()))
	first, second := s.prims[0], s.prims[1]
	// The first quad's far edge is corners 1 and 2; the second's near edge is
	// corners 0 and 3.
	a, b := s.verts[first.lo+1], s.verts[first.lo+2]
	c, d := s.verts[second.lo], s.verts[second.lo+3]
	nearVec(t, a, c, "the near corner of the joint")
	nearVec(t, b, d, "the far corner of the joint")
}

// The band is as wide as it was told, measured across the path.
func TestARibbonIsAsWideAsItWasTold(t *testing.T) {
	for _, want := range []float64{ribbonWidth, 0.1} {
		opts := []geom.Option{}
		if want != ribbonWidth {
			opts = append(opts, geom.Thickness(want))
		}
		src := figure.NewTable().
			Float64("x", []float64{0, 1}).
			Float64("y", []float64{0.5, 0.5}).
			Float64("z", []float64{0.5, 0.5})
		s, _ := emitOne(t, ribbonScene(src, opts...))
		if len(s.prims) != 1 {
			t.Fatalf("%d primitives, want one", len(s.prims))
		}
		p := s.prims[0]
		w := s.verts[p.lo+3].Sub(s.verts[p.lo])
		if got := math.Sqrt(w.Dot(w)); math.Abs(got-want) > 1e-5 {
			t.Errorf("the band is %v wide, want %v", got, want)
		}
	}
}

// On a sphere the band lies flat against the ball: its width runs square to
// the radius through the point rather than in a horizontal plane cutting
// through it, so every corner is the same small step off the surface.
func TestARibbonOnASphereLiesOnTheBall(t *testing.T) {
	src := figure.NewTable().
		Float64("phi", []float64{0, 30, 60}).
		Float64("theta", []float64{90, 90, 90}).
		Float64("r", []float64{1, 1, 1})
	sc := NewScene(Spherical()).X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear()).
		Add(Ribbon(src, geom.X("phi"), geom.Y("theta"), geom.Z("r")))
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatal(err)
	}
	f := Frame{X: scales[axisX], Y: scales[axisY], Z: scales[axisZ], space: sc.sphere}
	s := new(Sink)
	s.openLayer(0, Home().Forward())
	if err := sc.layers[0].Emit(s, f); err != nil {
		t.Fatal(err)
	}
	// A corner is half a width along the tangent plane from a point on the
	// ball, so it stands off the surface by exactly that much and no more.
	half := float64(ribbonWidth) / 2
	want := math.Hypot(radius, half)
	for i, v := range s.verts {
		if r := v.Sub(centre); math.Abs(math.Sqrt(r.Dot(r))-want) > 1e-5 {
			t.Errorf("corner %d is at radius %v, want %v — square to the radius",
				i, math.Sqrt(r.Dot(r)), want)
		}
	}
}

// A ribbon reports the row each quad ends at, which is the row the reader is
// pointing at when they point at its far end.
func TestARibbonReportsTheRowASegmentEndsAt(t *testing.T) {
	sc := ribbonScene(elbow())
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatal(err)
	}
	f := Frame{X: scales[axisX], Y: scales[axisY], Z: scales[axisZ], Rows: noRows{}}
	s := new(Sink)
	s.openLayer(0, Home().Forward())
	if err := sc.layers[0].Emit(s, f); err != nil {
		t.Fatal(err)
	}
	for i, p := range s.prims {
		if int(p.row) != i+1 {
			t.Errorf("quad %d reports row %d, want %d", i, p.row, i+1)
		}
	}
}

// A group column starts a new band rather than joining the end of one trace to
// the start of the next, which is Line3's rule.
func TestARibbonDoesNotJoinTwoSeries(t *testing.T) {
	src := figure.NewTable().
		Float64("x", []float64{0, 0.5, 0.6, 1}).
		Float64("y", []float64{0.5, 0.5, 0.5, 0.5}).
		Float64("z", []float64{0.5, 0.5, 0.5, 0.5}).
		String("g", []string{"a", "a", "b", "b"})
	s, _ := emitOne(t, ribbonScene(src, geom.GroupBy("g")))
	if len(s.prims) != 2 {
		t.Errorf("%d quads over two series of two rows, want one each", len(s.prims))
	}
}

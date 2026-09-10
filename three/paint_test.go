package three

import (
	"math"
	"testing"

	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
)

// lattice emits an n by n field of quads at height h(i, j), the way a surface
// does, and remembers which cell each quad came from so that a test can read
// the paint order back as cells.
func lattice(s *Sink, n int, h func(i, j int) float32) {
	fwd := s.Forward()
	is, js := outward(n, fwd.X), outward(n, fwd.Y)
	for _, j := range js {
		for _, i := range is {
			x0, x1 := float32(i)/float32(n), float32(i+1)/float32(n)
			y0, y1 := float32(j)/float32(n), float32(j+1)/float32(n)
			s.Row(j*n + i)
			s.Face([]Vec3{
				{x0, y0, h(i, j)}, {x1, y0, h(i+1, j)},
				{x1, y1, h(i+1, j+1)}, {x0, y1, h(i, j+1)},
			}, Style{Fill: ir.Color{R: 1, G: 2, B: 3, A: 255}})
		}
	}
}

// outward is the traversal order along one axis: from the far end toward the
// camera, chosen by the sign of that component of the view direction.
func outward(n int, f float32) []int {
	out := make([]int, n)
	for k := range out {
		if f < 0 {
			out[k] = n - 1 - k
		} else {
			out[k] = k
		}
	}
	return out
}

// The picture is right where it can be seen: wherever two primitives cover the
// same point of the screen, the one the camera reaches first is painted last.
//
// This is deliberately not a test of the ordering rule. It casts a ray through
// each sample point, intersects it with the plane of every primitive covering
// that point, and compares the paint order against which intersection is
// nearer — so it would fail for any key that gets the visible occlusion wrong,
// including the one the implementation happens to use. A test whose expected
// answer is computed by restating the sort would only prove the sort sorts.
func TestNothingIsPaintedOverSomethingNearer(t *testing.T) {
	for _, cam := range octants() {
		area := ir.R(0, 0, 200, 200)
		pr := project(cam, area)

		s := acquire()
		s.openLayer(0, cam.Forward())
		// A ridge, so that the heights disagree loudly with the floor: a
		// surface whose quads' own middles order differently from their cells
		// is the case the key is chosen for.
		lattice(s, 7, func(i, j int) float32 {
			return 0.9 * float32(math.Exp(-float64((i-3)*(i-3))/2))
		})
		rec := irtest.New()
		s.paint(rec, pr, nil, 0, nil, nil)

		checkOcclusion(t, s, pr, area, cam)
		release(s)
	}
}

// Two surfaces over one grid at two heights is an ordinary chart — a designed
// part and a measured one on the same axes — and it is the case a key that
// drops the height cannot order at all: both layers stand on the same
// footprints, so every pair of quads ties, and without a second key the
// emission order decides which is on top.
//
// The lower surface is emitted last here, so an order that fell back on
// emission would draw it over the higher one and the test would catch it.
func TestASurfaceIsNotPaintedOverTheOneAboveIt(t *testing.T) {
	for _, cam := range append(octants(),
		LookAt(Azimuth(-0.6), Elevation(1.5)), // all but straight down
		LookAt(Azimuth(0.4), Elevation(math.Pi/2)),
	) {
		area := ir.R(0, 0, 200, 200)
		pr := project(cam, area)

		s := acquire()
		flat := func(z float32) func(i, j int) float32 {
			return func(int, int) float32 { return z }
		}
		s.openLayer(0, cam.Forward())
		lattice(s, 4, flat(0.8))
		s.openLayer(1, cam.Forward())
		lattice(s, 4, flat(0.2))

		rec := irtest.New()
		s.paint(rec, pr, nil, 0, nil, nil)
		checkOcclusion(t, s, pr, area, cam)

		// And the same fact stated the way a reader would see it: looking down
		// on two stacked sheets, the top one is the one you see.
		if cam.Elevation() > 0 {
			topLast := false
			for _, k := range s.order {
				topLast = s.prims[k.idx].layer == 0
			}
			if !topLast {
				t.Errorf("camera %+v: the lower sheet is painted last and hides the upper one", cam)
			}
		}
		release(s)
	}
}

// A scene of two kinds of layer is one scene. A surface and a path through it
// have to be ordered against each other, which they can only be if the number
// each of them is keyed by means the same thing.
func TestASurfaceAndAPathAreOrderedAgainstEachOther(t *testing.T) {
	for _, cam := range octants() {
		area := ir.R(0, 0, 200, 200)
		pr := project(cam, area)

		s := acquire()
		s.openLayer(0, cam.Forward())
		lattice(s, 6, func(i, j int) float32 { return 0.35 })
		// A path that runs across the sheet at a rising height, so that part
		// of it is under the surface and part of it above.
		s.openLayer(1, cam.Forward())
		st := Style{Stroke: ir.Color{R: 200, A: 255}, Width: 2}
		for i := 0; i < 20; i++ {
			t0 := float32(i) / 20
			t1 := float32(i+1) / 20
			s.Line([]Vec3{{0.1 + 0.8*t0, 0.5, t0}, {0.1 + 0.8*t1, 0.5, t1}}, st)
		}
		rec := irtest.New()
		s.paint(rec, pr, nil, 0, nil, nil)

		// The two layers have to interleave. If their keys were two different
		// measures of depth — one dropping the height and one not — the two
		// numbers would be on different scales, and one layer would sort
		// wholly before the other however the geometry ran. That is the bug
		// this guards, and it is invisible in any picture with one layer in
		// it.
		first := map[int32]int{}
		last := map[int32]int{}
		for i, k := range s.order {
			l := s.prims[k.idx].layer
			if _, seen := first[l]; !seen {
				first[l] = i
			}
			last[l] = i
		}
		if !(first[1] < last[0] && first[0] < last[1]) {
			t.Errorf("camera %+v: the surface occupies places %d..%d and the path %d..%d; "+
				"one layer sorted wholly before the other, so their keys are not one number",
				cam, first[0], last[0], first[1], last[1])
		}
		release(s)
	}
}

// checkOcclusion is the independent half of these tests: for a grid of sample
// points it works out, from the geometry alone, which primitive the camera
// reaches first, and insists that one is painted last.
func checkOcclusion(t *testing.T, s *Sink, pr projector, area ir.Rect, cam Camera) {
	t.Helper()

	place := make(map[int32]int, len(s.order))
	for i, k := range s.order {
		place[k.idx] = i
	}

	const samples = 23
	for a := 1; a < samples; a++ {
		for b := 1; b < samples; b++ {
			pt := ir.Point{
				X: area.Min.X + area.Dx()*float32(a)/samples,
				Y: area.Min.Y + area.Dy()*float32(b)/samples,
			}
			type hit struct {
				idx int32
				t   float64
			}
			var hits []hit
			for i := range s.prims {
				p := &s.prims[i]
				if p.kind != kindFace {
					continue
				}
				poly := s.pts[p.lo:p.hi]
				if !polygonContains(poly, pt) {
					continue
				}
				d, ok := rayDepth(pr, s.verts[p.lo:p.hi], pt)
				if !ok {
					continue
				}
				hits = append(hits, hit{int32(i), d})
			}
			for i := range hits {
				for j := range hits {
					if i == j {
						continue
					}
					// hits[i] is nearer, so it must be painted later.
					if hits[i].t < hits[j].t-1e-6 && place[hits[i].idx] < place[hits[j].idx] {
						t.Fatalf("camera %+v: at %v the primitive at depth %.4f is painted before "+
							"the one at %.4f, so the farther one covers it",
							cam, pt, hits[i].t, hits[j].t)
					}
				}
			}
		}
	}
}

// rayDepth intersects the view ray through a device point with the plane of a
// primitive, and reports how far along the ray that is. Larger is farther.
func rayDepth(pr projector, vs []Vec3, pt ir.Point) (float64, bool) {
	if len(vs) < 3 {
		return 0, false
	}
	n := vs[1].Sub(vs[0]).Cross(vs[2].Sub(vs[1]))
	denom := n.Dot(pr.fwd)
	if denom > -1e-9 && denom < 1e-9 {
		// The plane is edge-on to the camera; it covers no area to argue about.
		return 0, false
	}
	// The ray's origin: the scene point that projects to pt and lies in the
	// plane through the cube's middle perpendicular to the view direction.
	u := float64(pt.X-pr.at.X) / float64(pr.scale)
	v := -float64(pt.Y-pr.at.Y) / float64(pr.scale)
	o := centre.
		Add(pr.right.Mul(float32(u))).
		Add(pr.up.Mul(float32(v)))
	return vs[0].Sub(o).Dot(n) / denom, true
}

// polygonContains is the even-odd rule over a projected polygon.
func polygonContains(poly []ir.Point, pt ir.Point) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		pi, pj := poly[i], poly[j]
		if (pi.Y > pt.Y) != (pj.Y > pt.Y) {
			x := pi.X + (pt.Y-pi.Y)/(pj.Y-pi.Y)*(pj.X-pi.X)
			if pt.X < x {
				in = !in
			}
		}
	}
	return in
}

// Two renders of one scene produce the same picture, and nothing in the order
// depends on anything but the primitives themselves.
func TestThePaintOrderIsDeterministic(t *testing.T) {
	draw := func() []string {
		s := acquire()
		defer release(s)
		s.openLayer(0, Home().Forward())
		lattice(s, 5, func(i, j int) float32 { return float32(i*j%3) / 3 })
		rec := irtest.New()
		s.paint(rec, project(Home(), ir.R(0, 0, 100, 100)), nil, 0, nil, nil)
		return rec.Trace()
	}
	a, b := draw(), draw()
	if len(a) != len(b) {
		t.Fatalf("two renders made %d and %d calls", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("call %d differs between two renders:\n got %s\nwant %s", i, b[i], a[i])
		}
	}
}

// Primitives the geometry cannot separate are ordered by when they were
// emitted, which is lexicographically (layer, the layer's own order). That is
// total and free, and it is what stops a scene shimmering while the reader
// holds the mouse still.
func TestPrimitivesTheGeometryCannotSeparateBreakByEmissionOrder(t *testing.T) {
	s := acquire()
	defer release(s)
	s.openLayer(0, Home().Forward())
	// Three faces at one place, emitted in order, each its own style so that
	// they cannot batch into one call and the order is readable.
	for i := 0; i < 3; i++ {
		s.Row(i)
		s.Face([]Vec3{{0.5, 0.5, 0.5}, {0.6, 0.5, 0.5}, {0.6, 0.6, 0.5}},
			Style{Fill: ir.Color{R: uint8(i), A: 255}})
	}
	rec := irtest.New()
	s.paint(rec, project(Home(), ir.R(0, 0, 100, 100)), nil, 0, nil, nil)

	fills := rec.Filter("FillPath")
	if len(fills) != 3 {
		t.Fatalf("got %d fills, want three faces that could not batch", len(fills))
	}
	for i, c := range fills {
		if got := c.Fill.Color.R; got != uint8(i) {
			t.Errorf("fill %d is face %d; the emission order was not preserved", i, got)
		}
	}
}

// octants returns a camera looking from each of the eight corners of the
// scene, so that a test of the ordering covers every combination of signs of
// the view direction.
func octants() []Camera {
	var out []Camera
	for _, az := range []float64{-2.4, -0.6, 0.9, 2.6} {
		for _, el := range []float64{-0.7, 0.4} {
			out = append(out, LookAt(Azimuth(az), Elevation(el)))
		}
	}
	return out
}

// Faces are merged into one call only where merging is provably the same
// picture. Two cases where it is not, and both of them are quiet: the outline
// of a farther face would land on top of a nearer one's fill, and translucent
// fills composite in their overlap rather than covering it once.
func TestOnlyOpaqueUnoutlinedFacesAreDrawnTogether(t *testing.T) {
	draw := func(st Style) *irtest.Recorder {
		s := acquire()
		defer release(s)
		s.openLayer(0, Home().Forward())
		lattice3(s, 4, st)
		rec := irtest.New()
		s.paint(rec, project(Home(), ir.R(0, 0, 100, 100)), nil, 0, nil, nil)
		return rec
	}

	opaque := ir.Color{R: 10, G: 20, B: 30, A: 255}
	if got := draw(Style{Fill: opaque}).Count("FillPath"); got != 1 {
		t.Errorf("%d fill calls for sixteen opaque unoutlined faces, want them merged into one", got)
	}

	// Outlined: sixteen fills and sixteen strokes, interleaved in the order,
	// so that a far outline cannot land over a near fill.
	outlined := draw(Style{Fill: opaque, Stroke: ir.Color{A: 255}, Width: 1})
	if got := outlined.Count("FillPath"); got != 16 {
		t.Errorf("%d fill calls for sixteen outlined faces, want one each", got)
	}
	if got := outlined.Count("StrokePath"); got != 16 {
		t.Errorf("%d stroke calls for sixteen outlined faces, want one each", got)
	}
	for i, op := range outlined.Ops() {
		want := "FillPath"
		if i%2 == 1 {
			want = "StrokePath"
		}
		if op != want {
			t.Fatalf("call %d is a %s, want the fills and strokes interleaved so that "+
				"each face is finished before the next begins", i, op)
		}
	}

	// Translucent: one call each, because a union filled once is not what
	// several overlapping fills compose to.
	if got := draw(Style{Fill: ir.Color{R: 10, A: 128}}).Count("FillPath"); got != 16 {
		t.Errorf("%d fill calls for sixteen translucent faces, want one each", got)
	}
}

// lattice3 is [lattice] at one height and one style, for the tests that are
// about how faces are drawn rather than about where they are.
func lattice3(s *Sink, n int, st Style) {
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			x0, x1 := float32(i)/float32(n), float32(i+1)/float32(n)
			y0, y1 := float32(j)/float32(n), float32(j+1)/float32(n)
			s.Face([]Vec3{{x0, y0, 0.5}, {x1, y0, 0.5}, {x1, y1, 0.5}, {x0, y1, 0.5}}, st)
		}
	}
}

// A quad is one mark whether or not it was drawn with its neighbours: the
// merge puts each face in its own subpath, and a hit index indexes one mark
// per subpath. Without that, pointing at one cell of a surface would report
// the whole sheet.
func TestAMergedRunIsStillOneMarkPerFace(t *testing.T) {
	const n = 4
	s := acquire()
	defer release(s)
	s.openLayer(0, Home().Forward())
	lattice3(s, n, Style{Fill: ir.Color{R: 10, A: 255}})

	rec := irtest.New()
	s.paint(rec, project(Home(), ir.R(0, 0, 100, 100)), nil, 0, nil, nil)

	fills := rec.Filter("FillPath")
	if len(fills) != 1 {
		t.Fatalf("got %d fill calls, want the run merged into one", len(fills))
	}
	subpaths := 0
	for _, op := range fills[0].Path.Ops {
		if op == ir.OpMoveTo {
			subpaths++
		}
	}
	if subpaths != n*n {
		t.Errorf("the merged call has %d subpaths, want one per quad (%d)", subpaths, n*n)
	}
}

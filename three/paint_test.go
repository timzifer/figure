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

// The case that took a review to find, with its numbers in it.
//
// Two sheets whose footprints overlap without coinciding, at two heights, seen
// from a quarter turn up. The point (0.9, 0.5, 0.7) on the upper sheet and the
// point (0.5, 0.5, 0.3) on the lower one project to the same pixel, and the
// upper one is nearer — so the upper sheet has to be painted last.
//
// A key that drops the height gets this backwards. It puts the upper sheet's
// centroid at 0.354 and the lower one's at 0.212, so the lower is painted
// last and covers the sheet in front of it. The argument that key rested on —
// that depth with the height dropped is monotone along every view ray — is
// true and does not reach: two centroids lie on two different rays, and the
// step from one to the other was never made. This test is the step not
// existing.
func TestASheetIsNotPaintedOverTheOneInFrontOfIt(t *testing.T) {
	cam := LookAt(Azimuth(0), Elevation(math.Pi/4))
	area := ir.R(0, 0, 200, 200)
	pr := project(cam, area)

	s := acquire()
	defer release(s)
	s.openLayer(0, cam.Forward())
	s.Face([]Vec3{{0, 0, 0.7}, {1, 0, 0.7}, {1, 1, 0.7}, {0, 1, 0.7}},
		Style{Fill: ir.Color{R: 1, A: 255}})
	s.openLayer(1, cam.Forward())
	s.Face([]Vec3{{0.4, 0, 0.3}, {1, 0, 0.3}, {1, 1, 0.3}, {0.4, 1, 0.3}},
		Style{Fill: ir.Color{R: 2, A: 255}})

	rec := irtest.New()
	s.paint(rec, pr, nil, 0, nil, nil)

	// The two points really are one pixel, which is what makes this a
	// question about occlusion rather than about arithmetic.
	upper, lower := Vec3{0.9, 0.5, 0.7}, Vec3{0.5, 0.5, 0.3}
	if a, b := pr.point(upper), pr.point(lower); a != b {
		t.Fatalf("the two points project to %v and %v; the example needs them on one pixel", a, b)
	}
	if pr.depth(upper) >= pr.depth(lower) {
		t.Fatalf("the upper point is at depth %v and the lower at %v; the upper one is meant to be nearer",
			pr.depth(upper), pr.depth(lower))
	}

	if last := s.prims[s.order[len(s.order)-1].idx].layer; last != 0 {
		t.Errorf("the lower sheet is painted last and covers the one in front of it")
	}
	checkOcclusion(t, s, pr, area, cam)
}

// Two surfaces over one grid at two heights is an ordinary chart — a designed
// part and a measured one on the same axes — and it is the same question with
// the footprints exactly on top of one another rather than merely overlapping.
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

		// The reading the picture has to give: where the path runs under the
		// sheet it is hidden by it, and where it runs over it, it is drawn on
		// top. That is checked point by point against the geometry rather than
		// against the order.
		checkLineOcclusion(t, s, pr, cam)

		// And the weaker structural fact that goes with it. If the two layers
		// were keyed by two different measures of depth, the two numbers would
		// be on different scales and one layer would sort wholly before the
		// other however the geometry ran — so they have to interleave.
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

// checkLineOcclusion is [checkOcclusion] for the primitives that have no area.
//
// A polyline covers a curve of the screen rather than a region, so it is
// sampled along its own length: at each sample the point's depth is exact —
// no plane to intersect, the point is on the segment — and every face covering
// that pixel is compared against it.
func checkLineOcclusion(t *testing.T, s *Sink, pr projector, cam Camera) {
	t.Helper()

	place := make(map[int32]int, len(s.order))
	for i, k := range s.order {
		place[k.idx] = i
	}

	for i := range s.prims {
		line := &s.prims[i]
		if line.kind != kindLine {
			continue
		}
		a, b := s.verts[line.lo], s.verts[line.lo+1]
		for step := 0; step <= 8; step++ {
			u := float32(step) / 8
			at := a.Add(b.Sub(a).Mul(u))
			pt := pr.point(at)
			d := pr.depth(at)

			for j := range s.prims {
				face := &s.prims[j]
				if face.kind != kindFace {
					continue
				}
				if !polygonContains(s.pts[face.lo:face.hi], pt) {
					continue
				}
				fd, ok := rayDepth(pr, s.verts[face.lo:face.hi], pt)
				if !ok {
					continue
				}
				switch {
				case d < fd-1e-6 && place[int32(i)] < place[int32(j)]:
					t.Fatalf("camera %+v: at %v the path is in front at %.4f and the sheet behind at %.4f, "+
						"and the sheet is painted over it", cam, pt, d, fd)
				case fd < d-1e-6 && place[int32(j)] < place[int32(i)]:
					t.Fatalf("camera %+v: at %v the sheet is in front at %.4f and the path behind at %.4f, "+
						"and the path is drawn over it", cam, pt, fd, d)
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

// A sweep over many arrangements of the case the review found, because one
// worked example proves one example.
//
// Two lattices at two heights with offset footprints, from forty cameras: the
// "designed part and measured part on one pair of axes" chart, arranged every
// way the generator can think of. This is the shape the package actually
// emits — a surface reaches the painter as one small quad per cell, never as
// one big sheet — and small quads are what keep a centroid a usable sample of
// a primitive's depth. See [Sink.depthOf] for where that stops being true.
//
// The numbers come from a fixed generator rather than math/rand, so a failure
// is the same failure on every machine and every run.
func TestStackedLatticesAreOrderedCorrectlyAcrossManyArrangements(t *testing.T) {
	rng := newLCG(0x5eed)
	area := ir.R(0, 0, 160, 160)

	for round := 0; round < 40; round++ {
		cam := LookAt(
			Azimuth(rng.between(-math.Pi, math.Pi)),
			Elevation(rng.between(-1.5, 1.5)),
		)
		pr := project(cam, area)

		s := acquire()
		for k, z := range [2]float32{0.25, 0.75} {
			ox := float32(rng.between(0, 0.2))
			oy := float32(rng.between(0, 0.2))
			s.openLayer(k, cam.Forward())
			sheet(s, 8, ox, oy, z, ir.Color{R: uint8(k * 90), A: 255})
		}
		rec := irtest.New()
		s.paint(rec, pr, nil, 0, nil, nil)
		checkOcclusion(t, s, pr, area, cam)
		release(s)
	}
}

// sheet emits one flat lattice of n by n cells, offset on the floor and at a
// fixed height.
func sheet(s *Sink, n int, ox, oy, z float32, fill ir.Color) {
	const span = 0.8
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			x0 := ox + span*float32(i)/float32(n)
			x1 := ox + span*float32(i+1)/float32(n)
			y0 := oy + span*float32(j)/float32(n)
			y1 := oy + span*float32(j+1)/float32(n)
			s.Face([]Vec3{{x0, y0, z}, {x1, y0, z}, {x1, y1, z}, {x0, y1, z}},
				Style{Fill: fill})
		}
	}
}

// The condition under which a centroid is enough, stated as a test rather than
// only as a doc comment: two primitives whose depth *ranges* do not overlap
// are always ordered correctly, whatever their shape or size.
//
// It is what [Sink.depthOf] promises and the whole of what it promises. The
// sweep above is the same claim for the shapes the package emits; this one is
// the claim itself.
func TestPrimitivesSeparatedInDepthAreAlwaysOrderedCorrectly(t *testing.T) {
	rng := newLCG(0xc0ffee)
	area := ir.R(0, 0, 160, 160)

	for round := 0; round < 200; round++ {
		cam := LookAt(Azimuth(rng.between(-math.Pi, math.Pi)), Elevation(rng.between(-1.5, 1.5)))
		pr := project(cam, area)

		s := acquire()
		s.openLayer(0, cam.Forward())
		a := randomQuad(rng)
		b := randomQuad(rng)
		s.Face(a[:], Style{Fill: ir.Color{R: 10, A: 255}})
		s.Face(b[:], Style{Fill: ir.Color{R: 20, A: 255}})

		// Only the arrangements the promise covers: the two depth ranges have
		// to be disjoint, which is what "the pieces can be totally ordered"
		// means for two primitives that each span a range.
		aLo, aHi := depthRange(pr, a[:])
		bLo, bHi := depthRange(pr, b[:])
		if aHi >= bLo && bHi >= aLo {
			release(s)
			continue
		}
		rec := irtest.New()
		s.paint(rec, pr, nil, 0, nil, nil)
		checkOcclusion(t, s, pr, area, cam)
		release(s)
	}
}

func randomQuad(rng *lcg) [4]Vec3 {
	x := float32(rng.between(0.05, 0.6))
	y := float32(rng.between(0.05, 0.6))
	z := float32(rng.between(0.05, 0.95))
	w := float32(rng.between(0.05, 0.35))
	h := float32(rng.between(0.05, 0.35))
	tilt := float32(rng.between(-0.2, 0.2))
	return [4]Vec3{
		{x, y, z}, {x + w, y, z + tilt}, {x + w, y + h, z + tilt}, {x, y + h, z},
	}
}

func depthRange(pr projector, vs []Vec3) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, v := range vs {
		d := pr.depth(v)
		lo, hi = math.Min(lo, d), math.Max(hi, d)
	}
	return lo, hi
}

// lcg is a linear congruential generator: the numerical recipe, chosen because
// it is four lines and produces the same sequence everywhere. A test whose
// inputs differ between runs is a test that fails on somebody else's Tuesday.
type lcg struct{ state uint64 }

func newLCG(seed uint64) *lcg { return &lcg{state: seed} }

func (g *lcg) next() float64 {
	g.state = g.state*6364136223846793005 + 1442695040888963407
	return float64(g.state>>11) / float64(uint64(1)<<53)
}

func (g *lcg) between(lo, hi float64) float64 { return lo + g.next()*(hi-lo) }

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
	s.Depth(DepthGround)
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

// The order a surface is painted in is the order its cells stand on the floor,
// farthest first — exactly, for every direction the camera can look from.
//
// That is ADR 0056's promise and it is what makes the painter's algorithm
// correct here rather than merely usual: a quad of a height field is occluded
// by its neighbours in the lattice and never by how tall it is, so ordering by
// the cell's ground position rather than by the quad's own middle is a reading
// of the data rather than an approximation of a depth buffer.
func TestASurfaceIsPaintedExactlyBackToFront(t *testing.T) {
	const n = 6
	// A ridge, so that the heights disagree loudly with the ground order: an
	// ordering by the quads' own centroids would fail this test, which is the
	// point of writing it with a ridge in it.
	height := func(i, j int) float32 {
		return 0.9 * float32(math.Exp(-float64((i-3)*(i-3))/2))
	}

	for _, cam := range octants() {
		s := acquire()
		s.openLayer(0, cam.Forward())
		lattice(s, n, height)

		rec := irtest.New()
		s.paint(rec, project(cam, ir.R(0, 0, 200, 200)), nil, 0, nil, nil)

		got := paintedRows(t, s)
		want := groundOrder(s, len(got))
		for k := range want {
			if got[k] != want[k] {
				t.Fatalf("camera %+v: cell %d of the paint order is %d, want %d",
					cam, k, got[k], want[k])
			}
		}
		release(s)
	}
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

// Primitives at the same depth are ordered by when they were emitted, which is
// lexicographically (layer, the layer's own order). That is total and free,
// and it is what stops a scene shimmering while the reader holds the mouse
// still.
func TestEqualDepthsBreakByEmissionOrder(t *testing.T) {
	s := acquire()
	defer release(s)
	s.openLayer(0, Home().Forward())
	// Three faces at one depth, emitted last-first, each its own style so that
	// they cannot batch into one call and the order is readable.
	for i := 0; i < 3; i++ {
		s.Row(i)
		s.Face([]Vec3{{0.5, 0.5, 0}, {0.6, 0.5, 0}, {0.6, 0.6, 0}},
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

// A run of adjacent faces sharing a layer and a style is one call with one
// subpath per face — so a quad is still a mark a pointer can land on, and a
// surface is not one call per quad either.
func TestAdjacentFacesBatchIntoOneCallAndStayOneMarkEach(t *testing.T) {
	const n = 4
	s := acquire()
	defer release(s)
	s.openLayer(0, Home().Forward())
	lattice(s, n, func(i, j int) float32 { return 0.5 })

	rec := irtest.New()
	s.paint(rec, project(Home(), ir.R(0, 0, 100, 100)), nil, 0, nil, nil)

	fills := rec.Filter("FillPath")
	if len(fills) != 1 {
		t.Fatalf("got %d fill calls for one style, want them batched into one", len(fills))
	}
	if got := subpaths(fills[0].Path); got != n*n {
		t.Errorf("the batched call has %d subpaths, want one per quad (%d)", got, n*n)
	}
	// Nothing is stroked, because no outline was asked for. A hit index ranks
	// a stroked vertex above the area it outlines, so an always-outlined mesh
	// would report a corner on every hover.
	if got := rec.Count("StrokePath"); got != 0 {
		t.Errorf("got %d stroke calls, want none: the outline is opt-in", got)
	}
}

// octants returns a camera looking from each of the eight corners of the
// scene, so that a test of the traversal covers every combination of signs.
func octants() []Camera {
	var out []Camera
	for _, az := range []float64{-2.4, -0.6, 0.9, 2.6} {
		for _, el := range []float64{-0.7, 0.4} {
			out = append(out, LookAt(Azimuth(az), Elevation(el)))
		}
	}
	return out
}

// paintedRows reads the paint order back as the rows the primitives carried.
func paintedRows(t *testing.T, s *Sink) []int {
	t.Helper()
	out := make([]int, 0, len(s.keys))
	for _, k := range s.keys {
		out = append(out, int(s.prims[k.idx].row))
	}
	return out
}

// groundOrder is the order the cells stand on the floor, farthest first,
// computed from the primitives directly rather than from the traversal — so
// the test compares the picture against the definition rather than against
// the code that produced it.
func groundOrder(s *Sink, n int) []int {
	type cell struct {
		row   int
		depth float64
	}
	cells := make([]cell, 0, n)
	for i := range s.prims {
		p := &s.prims[i]
		var c Vec3
		for _, v := range s.verts[p.lo:p.hi] {
			c = c.Add(v)
		}
		c = c.Mul(1 / float32(p.hi-p.lo))
		c.Z = 0
		cells = append(cells, cell{int(p.row), float64(float32(c.Sub(centre).Dot(s.fwd)))})
	}
	// A stable insertion sort, so that equal depths keep emission order — the
	// same tie-break the painter makes, spelled out independently.
	for i := 1; i < len(cells); i++ {
		for j := i; j > 0 && cells[j].depth > cells[j-1].depth; j-- {
			cells[j], cells[j-1] = cells[j-1], cells[j]
		}
	}
	out := make([]int, 0, len(cells))
	for _, c := range cells {
		out = append(out, c.row)
	}
	return out
}

func subpaths(p *ir.Path) int {
	n := 0
	for _, op := range p.Ops {
		if op == ir.OpMoveTo {
			n++
		}
	}
	return n
}

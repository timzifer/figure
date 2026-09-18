package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// The node-link layout. What these check is the two properties the record
// rests on — the same table gives the same picture, and the picture says what
// the graph says — on graphs whose right answer is known by looking at them.
// See docs/adr/0077-a-node-link-layout.md.

func laidOut(t *testing.T, from, to []int, n int) []stat.StressPoint {
	t.Helper()
	var s stat.Stress
	s.Reset(from, to, n)
	if s.TooMany {
		t.Fatalf("a graph of %d nodes was refused", n)
	}
	if len(s.Nodes) != n {
		t.Fatalf("laid out %d of %d nodes", len(s.Nodes), n)
	}
	return append([]stat.StressPoint(nil), s.Nodes...)
}

func apart(p []stat.StressPoint, i, j int) float64 {
	return math.Hypot(p[i].X-p[j].X, p[i].Y-p[j].Y)
}

// lattice builds the r×c lattice, whose drawing a reader can check at a glance.
func lattice(r, c int) (from, to []int, n int) {
	id := func(i, j int) int { return i*c + j }
	for i := range r {
		for j := range c {
			if j+1 < c {
				from, to = append(from, id(i, j)), append(to, id(i, j+1))
			}
			if i+1 < r {
				from, to = append(from, id(i, j)), append(to, id(i+1, j))
			}
		}
	}
	return from, to, r * c
}

func TestStressIsAPureFunctionOfItsInput(t *testing.T) {
	from, to, n := lattice(4, 5)
	first := laidOut(t, from, to, n)

	// The same table again, and then the same table through a struct that has
	// already laid out something else: the buffers are reused, and a layout
	// that read one of them stale would come out different.
	second := laidOut(t, from, to, n)
	var s stat.Stress
	s.Reset([]int{0, 1, 2}, []int{1, 2, 0}, 3)
	s.Reset(from, to, n)

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("node %d came out at %v and then at %v", i, first[i], second[i])
		}
		if s.Nodes[i] != first[i] {
			t.Fatalf("after reuse, node %d came out at %v rather than %v", i, s.Nodes[i], first[i])
		}
	}
}

func TestStressDrawsTheUnitSquare(t *testing.T) {
	from, to, n := lattice(3, 4)
	p := laidOut(t, from, to, n)
	var lox, hix, loy, hiy = 1.0, 0.0, 1.0, 0.0
	for _, q := range p {
		if q.X < 0 || q.X > 1 || q.Y < 0 || q.Y > 1 {
			t.Fatalf("a node landed outside the unit square: %v", q)
		}
		lox, hix = math.Min(lox, q.X), math.Max(hix, q.X)
		loy, hiy = math.Min(loy, q.Y), math.Max(hiy, q.Y)
	}
	// One side fills the square and the other is centred in it, because the
	// scale is the same on both axes.
	if span := math.Max(hix-lox, hiy-loy); math.Abs(span-1) > 1e-9 {
		t.Errorf("the drawing spans %v of the square's longer side, want all of it", span)
	}
	if off := math.Abs((lox + hix) / 2); math.Abs(off-0.5) > 1e-9 {
		t.Errorf("the drawing is centred at %v across", off)
	}
}

// A drawn distance is not a graph distance and is not meant to be one. What the
// layout promises is the order: two nodes joined by an edge are drawn closer
// than two nodes three edges apart, everywhere in the picture.
func TestStressDrawsNearThingsNear(t *testing.T) {
	from, to, n := lattice(4, 4)
	p := laidOut(t, from, to, n)

	var near, far = 0.0, math.Inf(1)
	for i := range n {
		for j := range n {
			d := math.Abs(float64(i/4-j/4)) + math.Abs(float64(i%4-j%4)) // the lattice distance
			switch {
			case d == 1:
				near = math.Max(near, apart(p, i, j))
			case d >= 3:
				far = math.Min(far, apart(p, i, j))
			}
		}
	}
	if near >= far {
		t.Errorf("the widest pair one edge apart is %v and the closest pair three edges apart is %v", near, far)
	}
}

func TestStressSeparatesTwoClusters(t *testing.T) {
	// Two triangles joined by one edge: 0-1-2 and 3-4-5, with 2-3 between.
	p := laidOut(t, []int{0, 1, 2, 3, 4, 5, 2}, []int{1, 2, 0, 4, 5, 3, 3}, 6)
	within := math.Max(apart(p, 0, 1), apart(p, 3, 4))
	across := apart(p, 0, 4)
	if across <= 2*within {
		t.Errorf("the far pair is %v apart and a pair inside one triangle is %v", across, within)
	}
}

// Two nodes with the same neighbours have the same distance to everything, so
// the arrangement that puts them in one place is a stationary one. Drawn there
// they are one dot with two names on it, which is a picture that lies about how
// many things there are.
func TestStressSeparatesTwins(t *testing.T) {
	// kip is joined to lia and to ora, which are joined to nothing else.
	p := laidOut(t, []int{0, 0}, []int{1, 2}, 3)
	if d := apart(p, 1, 2); d < 0.1 {
		t.Errorf("the twins are %v apart, which is one dot with two names on it", d)
	}
}

func TestStressPushesThePiecesApart(t *testing.T) {
	// Two separate edges: 0-1 and 2-3, with no path between the pairs.
	p := laidOut(t, []int{0, 2}, []int{1, 3}, 4)
	joined := math.Max(apart(p, 0, 1), apart(p, 2, 3))
	split := apart(p, 0, 2)
	if split <= joined {
		t.Errorf("a pair with no path between them is %v apart and a joined pair is %v", split, joined)
	}
}

func TestStressClassifiesTheRowsItCannotUse(t *testing.T) {
	var s stat.Stress
	s.Reset([]int{0, 1, 9, 2}, []int{1, 1, 0, 0}, 3)
	if len(s.Edges) != 4 {
		t.Fatalf("%d edges for 4 rows", len(s.Edges))
	}
	for i, want := range []stat.StressEdge{
		{OK: true},
		{OK: true, Self: true},
		{},
		{OK: true},
	} {
		if s.Edges[i] != want {
			t.Errorf("row %d came back %+v, want %+v", i, s.Edges[i], want)
		}
	}
}

func TestStressRefusesAGraphItCannotDraw(t *testing.T) {
	var s stat.Stress
	n := stat.MaxStressNodes + 1
	from, to := make([]int, n-1), make([]int, n-1)
	for i := range from {
		from[i], to[i] = i, i+1
	}
	s.Reset(from, to, n)
	if !s.TooMany {
		t.Errorf("a graph of %d nodes was laid out rather than refused", n)
	}
	if len(s.Nodes) != 0 {
		t.Errorf("a refused graph still placed %d nodes", len(s.Nodes))
	}
}

func TestStressPlacesAGraphWithNoEdges(t *testing.T) {
	// Every pair is the same distance apart, which is a ring.
	p := laidOut(t, nil, nil, 6)
	for i := range p {
		for j := range p {
			if i != j && apart(p, i, j) < 0.1 {
				t.Errorf("nodes %d and %d are %v apart", i, j, apart(p, i, j))
			}
		}
	}
}

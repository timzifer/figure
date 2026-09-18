package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// edgesOf is the shape every case here is written in: two parallel columns, the
// way a caller's table holds them.
func edgesOf(pairs ...[2]int) (from, to []int) {
	for _, p := range pairs {
		from = append(from, p[0])
		to = append(to, p[1])
	}
	return from, to
}

// checkLayout asserts the properties that hold for every graph, whatever it is,
// so that each case below only has to state what is particular about it.
func checkLayout(t *testing.T, l *stat.Layered, from, to []int, nodes int) {
	t.Helper()

	if got := len(l.Nodes); got != nodes {
		t.Fatalf("nodes = %d, want %d", got, nodes)
	}
	if got := len(l.Edges); got != len(from) {
		t.Fatalf("edges = %d, want %d", got, len(from))
	}

	for i, n := range l.Nodes {
		if !(n.X > 0 && n.X < 1) {
			t.Errorf("node %d: X = %v, want inside (0, 1)", i, n.X)
		}
		if !(n.Y >= 0 && n.Y <= 1) {
			t.Errorf("node %d: Y = %v, want inside [0, 1]", i, n.Y)
		}
		if n.Rank < 0 || n.Rank >= l.Ranks {
			t.Errorf("node %d: rank %d, want inside [0, %d)", i, n.Rank, l.Ranks)
		}
	}

	for e, edge := range l.Edges {
		switch {
		case !edge.OK, edge.Self, edge.Back:
			if edge.Hi != edge.Lo {
				t.Errorf("edge %d: %d bends, want none", e, edge.Hi-edge.Lo)
			}
			continue
		}
		lo, hi := l.Nodes[from[e]].Rank, l.Nodes[to[e]].Rank
		if hi <= lo {
			t.Errorf("edge %d: %d → %d runs from rank %d to rank %d, want forwards",
				e, from[e], to[e], lo, hi)
		}
		if got, want := edge.Hi-edge.Lo, hi-lo-1; got != want {
			t.Errorf("edge %d: %d bends across %d ranks, want %d", e, got, hi-lo, want)
		}
		for i, b := range l.Bends[edge.Lo:edge.Hi] {
			wantY := rankHeight(lo+1+i, l.Ranks)
			if math.Abs(b.Y-wantY) > 1e-12 {
				t.Errorf("edge %d bend %d: Y = %v, want %v", e, i, b.Y, wantY)
			}
		}
	}

	// Two nodes on one rank never share a position: the separation pass is what
	// keeps a pull towards a neighbour from stacking them.
	seen := map[int][]float64{}
	for i, n := range l.Nodes {
		for _, x := range seen[n.Rank] {
			if math.Abs(x-n.X) < 1e-9 {
				t.Errorf("node %d: X = %v collides on rank %d", i, n.X, n.Rank)
			}
		}
		seen[n.Rank] = append(seen[n.Rank], n.X)
	}
}

func rankHeight(r, ranks int) float64 {
	if ranks <= 1 {
		return 0
	}
	return float64(r) / float64(ranks-1)
}

func TestLayeredRanksAreTheLongestPath(t *testing.T) {
	// A diamond with a short side: 0 → 1 → 2 → 3 and 0 → 3. The long path is
	// what decides node 3's rank, which is what "longest path" means and what
	// keeps every edge pointing forwards.
	from, to := edgesOf([2]int{0, 1}, [2]int{1, 2}, [2]int{2, 3}, [2]int{0, 3})

	var l stat.Layered
	l.Reset(from, to, 4)
	checkLayout(t, &l, from, to, 4)

	if l.Ranks != 4 {
		t.Fatalf("Ranks = %d, want 4", l.Ranks)
	}
	for i, want := range []int{0, 1, 2, 3} {
		if got := l.Nodes[i].Rank; got != want {
			t.Errorf("node %d: rank %d, want %d", i, got, want)
		}
	}
	// The short edge crosses two ranks and so bends twice.
	if got := l.Edges[3].Hi - l.Edges[3].Lo; got != 2 {
		t.Errorf("0 → 3: %d bends, want 2", got)
	}
}

func TestLayeredBreaksACycleRatherThanRefusingIt(t *testing.T) {
	// The shape every state machine has: a run forwards and one transition
	// back to the start.
	from, to := edgesOf([2]int{0, 1}, [2]int{1, 2}, [2]int{2, 0})

	var l stat.Layered
	l.Reset(from, to, 3)
	checkLayout(t, &l, from, to, 3)

	if l.Ranks != 3 {
		t.Fatalf("Ranks = %d, want 3 — the cycle should have been broken, not laid out", l.Ranks)
	}
	if !l.Edges[2].Back {
		t.Error("2 → 0 should be the back edge: it is the one that closes the cycle")
	}
	for _, e := range []int{0, 1} {
		if l.Edges[e].Back {
			t.Errorf("edge %d should not be a back edge", e)
		}
	}
}

func TestLayeredCycleEntryIsTheLowestNodeIndex(t *testing.T) {
	// The walk visits nodes in index order, so the member of a cycle that a
	// reader met first in the table is the one it is entered at — and the one
	// that ends up on rank zero. Written here as the same three states in a
	// different row order, to show that what decides it is the node order the
	// interner produced rather than the order the rows happen to sit in.
	for _, tc := range []struct {
		name  string
		pairs [][2]int
	}{
		{"rows in order", [][2]int{{0, 1}, {1, 2}, {2, 0}}},
		{"rows reversed", [][2]int{{2, 0}, {1, 2}, {0, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			from, to := edgesOf(tc.pairs...)

			var l stat.Layered
			l.Reset(from, to, 3)
			checkLayout(t, &l, from, to, 3)

			if l.Nodes[0].Rank != 0 {
				t.Errorf("node 0: rank %d, want 0 — the walk enters the cycle at the lowest index",
					l.Nodes[0].Rank)
			}
			back := 0
			for _, e := range l.Edges {
				if e.Back {
					back++
				}
			}
			if back != 1 {
				t.Errorf("%d back edges, want exactly 1 — one edge closes one cycle", back)
			}
		})
	}
}

func TestLayeredSelfEdgeNeverReachesTheLayering(t *testing.T) {
	from, to := edgesOf([2]int{0, 0}, [2]int{0, 1}, [2]int{1, 1})

	var l stat.Layered
	l.Reset(from, to, 2)
	checkLayout(t, &l, from, to, 2)

	for _, e := range []int{0, 2} {
		if !l.Edges[e].Self {
			t.Errorf("edge %d should be flagged Self", e)
		}
		if l.Edges[e].Back {
			t.Errorf("edge %d is a self-edge and should not also be a back edge", e)
		}
	}
	if l.Ranks != 2 {
		t.Errorf("Ranks = %d, want 2 — a self-edge constrains nothing", l.Ranks)
	}
}

func TestLayeredSkipsAnEdgeOutsideTheNodeCount(t *testing.T) {
	from, to := edgesOf([2]int{0, 1}, [2]int{1, 7}, [2]int{-1, 0})

	var l stat.Layered
	l.Reset(from, to, 2)
	checkLayout(t, &l, from, to, 2)

	for _, e := range []int{1, 2} {
		if l.Edges[e].OK {
			t.Errorf("edge %d names a node outside the count and should not be OK", e)
		}
	}
	if !l.Edges[0].OK {
		t.Error("edge 0 is inside the count and should be OK")
	}
}

func TestLayeredLaysOutAForest(t *testing.T) {
	// Two components that share nothing. Neither has a reason to be placed
	// relative to the other, and both still have to land inside the square.
	from, to := edgesOf([2]int{0, 1}, [2]int{2, 3})

	var l stat.Layered
	l.Reset(from, to, 4)
	checkLayout(t, &l, from, to, 4)

	if l.Ranks != 2 {
		t.Fatalf("Ranks = %d, want 2", l.Ranks)
	}
	for _, i := range []int{0, 2} {
		if l.Nodes[i].Rank != 0 {
			t.Errorf("node %d: rank %d, want 0", i, l.Nodes[i].Rank)
		}
	}
}

func TestLayeredWithNoEdgesStillPlacesItsNodes(t *testing.T) {
	var l stat.Layered
	l.Reset(nil, nil, 3)
	checkLayout(t, &l, nil, nil, 3)

	if l.Ranks != 1 {
		t.Fatalf("Ranks = %d, want 1", l.Ranks)
	}
	for i, n := range l.Nodes {
		if n.Y != 0 {
			t.Errorf("node %d: Y = %v, want 0 — one rank is the zeroth", i, n.Y)
		}
	}
}

func TestLayeredEmptyGraph(t *testing.T) {
	var l stat.Layered
	l.Reset([]int{0}, []int{1}, 0)
	if len(l.Nodes) != 0 || len(l.Edges) != 0 {
		t.Fatalf("Nodes = %d, Edges = %d, want both empty", len(l.Nodes), len(l.Edges))
	}
}

func TestLayeredAChainIsStraight(t *testing.T) {
	// Every rank holds one node, so nothing constrains the chain sideways and
	// the straightening pass should leave it on one line. A layout that put
	// each rank at its own left edge would draw a staircase.
	const n = 6
	var from, to []int
	for i := range n - 1 {
		from = append(from, i)
		to = append(to, i+1)
	}

	var l stat.Layered
	l.Reset(from, to, n)
	checkLayout(t, &l, from, to, n)

	for i, node := range l.Nodes {
		if math.Abs(node.X-l.Nodes[0].X) > 1e-12 {
			t.Errorf("node %d: X = %v, want %v — a chain should be straight", i, node.X, l.Nodes[0].X)
		}
	}
}

func TestLayeredIsDeterministicAcrossBufferReuse(t *testing.T) {
	// The property docs/adr/0012-parallel-panels.md needs: the same table gives
	// the same picture, whether the struct is fresh or has just drawn another
	// graph. The sort in the ordering pass is the reason this test exists.
	from, to := edgesOf(
		[2]int{0, 1}, [2]int{0, 2}, [2]int{1, 3}, [2]int{2, 3},
		[2]int{3, 4}, [2]int{4, 0}, [2]int{2, 5}, [2]int{5, 4},
	)

	var fresh stat.Layered
	fresh.Reset(from, to, 6)

	var reused stat.Layered
	reused.Reset([]int{0, 1, 2}, []int{1, 2, 0}, 3)
	reused.Reset([]int{0}, []int{0}, 1)
	reused.Reset(from, to, 6)

	if fresh.Ranks != reused.Ranks {
		t.Fatalf("Ranks = %d after reuse, want %d", reused.Ranks, fresh.Ranks)
	}
	for i := range fresh.Nodes {
		if fresh.Nodes[i] != reused.Nodes[i] {
			t.Errorf("node %d: %+v after reuse, want %+v", i, reused.Nodes[i], fresh.Nodes[i])
		}
	}
	for i := range fresh.Edges {
		if fresh.Edges[i] != reused.Edges[i] {
			t.Errorf("edge %d: %+v after reuse, want %+v", i, reused.Edges[i], fresh.Edges[i])
		}
	}
	if len(fresh.Bends) != len(reused.Bends) {
		t.Fatalf("bends = %d after reuse, want %d", len(reused.Bends), len(fresh.Bends))
	}
	for i := range fresh.Bends {
		if fresh.Bends[i] != reused.Bends[i] {
			t.Errorf("bend %d: %+v after reuse, want %+v", i, reused.Bends[i], fresh.Bends[i])
		}
	}
}

func TestLayeredEqualBarycentresKeepTheTableOrder(t *testing.T) {
	// Three leaves under one root: every one of them has the same single
	// neighbour, so every barycentre is equal. A stable sort leaves them in the
	// order the rows named them, and that is the whole argument for allowing a
	// sort at all.
	from, to := edgesOf([2]int{0, 1}, [2]int{0, 2}, [2]int{0, 3})

	var l stat.Layered
	l.Reset(from, to, 4)
	checkLayout(t, &l, from, to, 4)

	if !(l.Nodes[1].X < l.Nodes[2].X && l.Nodes[2].X < l.Nodes[3].X) {
		t.Errorf("leaves at X = %v, %v, %v, want them in the order the rows named them",
			l.Nodes[1].X, l.Nodes[2].X, l.Nodes[3].X)
	}
}

func TestLayeredDeepChainDoesNotRecurse(t *testing.T) {
	// The cycle walk and the post-order both run on an explicit stack, so a
	// graph deeper than a goroutine's stack lays out rather than crashing.
	const n = 200_000
	from, to := make([]int, n-1), make([]int, n-1)
	for i := range n - 1 {
		from[i], to[i] = i, i+1
	}

	var l stat.Layered
	l.Reset(from, to, n)

	if l.Ranks != n {
		t.Fatalf("Ranks = %d, want %d", l.Ranks, n)
	}
	if l.Nodes[n-1].Y != 1 {
		t.Errorf("last node: Y = %v, want 1", l.Nodes[n-1].Y)
	}
}

func TestLayeredWideStar(t *testing.T) {
	const n = 50_000
	from, to := make([]int, n), make([]int, n)
	for i := range n {
		from[i], to[i] = 0, i+1
	}

	var l stat.Layered
	l.Reset(from, to, n+1)

	if l.Ranks != 2 {
		t.Fatalf("Ranks = %d, want 2", l.Ranks)
	}
	for i, node := range l.Nodes {
		if !(node.X > 0 && node.X < 1) {
			t.Fatalf("node %d: X = %v, want inside (0, 1)", i, node.X)
		}
	}
}

func BenchmarkLayered(b *testing.B) {
	// A graph of the shape the mark is for: bounded rank span, a few edges per
	// node, one cycle back to the start. A random edge list would rank into as
	// many layers as it has nodes and route a dummy per layer per edge, which
	// measures the pathology rather than the layout.
	const layers, width = 40, 50
	const n = layers * width

	var from, to []int
	for r := range layers - 1 {
		for i := range width {
			v := r*width + i
			from = append(from, v, v)
			to = append(to, (r+1)*width+i, (r+1)*width+(i+7)%width)
		}
	}
	from = append(from, n-1)
	to = append(to, 0)

	var l stat.Layered
	b.ReportAllocs()
	for b.Loop() {
		l.Reset(from, to, n)
	}
}

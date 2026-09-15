package stat_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// lay runs a layout over a parent list and hands back the breadths.
func lay(parent []int, leaves bool) (stat.Tidy, []int) {
	depth := stat.Depth(parent)
	var t stat.Tidy
	if leaves {
		t.ResetLeaves(parent, depth)
	} else {
		t.Reset(parent, depth)
	}
	return t, depth
}

// A tree with some shape to it: a root, a shallow leaf, a deep branch, a wide
// family and two identical subtrees in different places.
//
//	0
//	├── 1
//	├── 2 ── 3 ── 4
//	│         └── 5 ── 6
//	├── 7 ── 8, 9, 10, 11
//	├── 12 ── 13, 14
//	└── 15 ── 16, 17
var sample = []int{-1, 0, 0, 2, 3, 3, 5, 0, 7, 7, 7, 7, 0, 12, 12, 0, 15, 15}

func children(parent []int, p int) []int {
	var out []int
	for i, q := range parent {
		if q == p && i != p {
			out = append(out, i)
		}
	}
	return out
}

func TestEveryParentIsCentredOverItsChildren(t *testing.T) {
	for _, leaves := range []bool{false, true} {
		lt, _ := lay(sample, leaves)
		for p := range sample {
			ks := children(sample, p)
			if len(ks) == 0 {
				continue
			}
			mid := (lt.X[ks[0]] + lt.X[ks[len(ks)-1]]) / 2
			if math.Abs(lt.X[p]-mid) > 1e-12 {
				t.Errorf("leaves=%v: node %d at %v, want centred at %v", leaves, p, lt.X[p], mid)
			}
		}
	}
}

func TestNoTwoNodesAtOneDepthAreCloserThanTheirSeparation(t *testing.T) {
	lt, depth := lay(sample, false)
	unit := 2 * minOf(lt.X) // the outermost node sits half a slot in
	byDepth := map[int][]int{}
	// The leaves' walk order is the left-to-right order; a node's place in a
	// level follows the same walk, so read the levels in index order of a
	// pre-order walk.
	var walk func(int)
	walk = func(v int) {
		byDepth[depth[v]] = append(byDepth[depth[v]], v)
		for _, k := range children(sample, v) {
			walk(k)
		}
	}
	walk(0)
	for d, level := range byDepth {
		for i := 1; i < len(level); i++ {
			a, b := level[i-1], level[i]
			want := 2 * unit
			if sample[a] == sample[b] {
				want = unit
			}
			if gap := lt.X[b] - lt.X[a]; gap < want*(1-1e-9) {
				t.Errorf("depth %d: nodes %d and %d are %v apart, want at least %v", d, a, b, gap, want)
			}
		}
	}
}

func TestTwoIdenticalSubtreesAreDrawnIdentically(t *testing.T) {
	lt, _ := lay(sample, false)
	a := []float64{lt.X[13] - lt.X[12], lt.X[14] - lt.X[12]}
	b := []float64{lt.X[16] - lt.X[15], lt.X[17] - lt.X[15]}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-12 {
			t.Errorf("subtree 12 and subtree 15 differ: %v vs %v", a, b)
		}
	}
}

func TestATidyTreeTucksAShallowBranchIn(t *testing.T) {
	// 0 has a leaf and a deep chain that fans out at the bottom. The fan is
	// below the leaf's level, so the tidy tree need not move the chain right
	// to clear it; a slot per leaf does.
	parent := []int{-1, 0, 0, 2, 3, 3, 3}
	tidy, _ := lay(parent, false)
	slots, _ := lay(parent, true)
	if span(tidy.X) >= span(slots.X) {
		t.Errorf("tidy spread %v is not narrower than one slot per leaf %v", tidy.X, slots.X)
	}
}

func TestOneSlotPerLeafInWalkOrder(t *testing.T) {
	lt, _ := lay(sample, true)
	want := []int{1, 4, 6, 8, 9, 10, 11, 13, 14, 16, 17}
	if len(lt.Leaves) != len(want) {
		t.Fatalf("Leaves = %v, want %v", lt.Leaves, want)
	}
	for k, leaf := range lt.Leaves {
		if leaf != want[k] {
			t.Errorf("Leaves = %v, want %v", lt.Leaves, want)
			break
		}
		if x := (float64(k) + 0.5) / float64(len(want)); math.Abs(lt.X[leaf]-x) > 1e-12 {
			t.Errorf("leaf %d at %v, want slot %d at %v", leaf, lt.X[leaf], k, x)
		}
	}
}

func TestALayoutIsAPureFunctionOfItsInput(t *testing.T) {
	var reused stat.Tidy
	other := []int{-1, 0, 1, 1, 0, 4, 4, 4, -1, 8}
	reused.Reset(other, stat.Depth(other))
	reused.Reset(sample, stat.Depth(sample))
	fresh, _ := lay(sample, false)
	for i := range sample {
		if reused.X[i] != fresh.X[i] {
			t.Fatalf("node %d: %v after reuse, %v fresh", i, reused.X[i], fresh.X[i])
		}
	}
}

func TestAForestAndACycleAndOneNode(t *testing.T) {
	one, _ := lay([]int{-1}, false)
	if one.X[0] != 0.5 {
		t.Errorf("a lone node sits at %v, want 0.5", one.X[0])
	}

	// Two roots, and 4 and 5 on a cycle of their own.
	parent := []int{-1, 0, -1, 2, 5, 4}
	lt, _ := lay(parent, false)
	if !math.IsNaN(lt.X[4]) || !math.IsNaN(lt.X[5]) {
		t.Errorf("nodes on a cycle placed at %v and %v", lt.X[4], lt.X[5])
	}
	if !(lt.X[0] < lt.X[2]) || lt.X[2]-lt.X[0] < 2*2*minOf(lt.X)*(1-1e-9) {
		t.Errorf("two roots at %v and %v are not a family apart", lt.X[0], lt.X[2])
	}
}

func TestADeepOrWideHierarchyDoesNotRecurse(t *testing.T) {
	const n = 200_000
	chain, star := make([]int, n), make([]int, n)
	for i := range chain {
		chain[i], star[i] = i-1, 0
	}
	star[0] = -1
	for _, parent := range [][]int{chain, star} {
		var lt stat.Tidy
		lt.Reset(parent, stat.Depth(parent))
		for _, x := range lt.X {
			if !(x > 0 && x < 1) {
				t.Fatalf("a node at %v", x)
			}
		}
	}
}

func minOf(xs []float64) float64 {
	m := math.Inf(1)
	for _, x := range xs {
		if !math.IsNaN(x) {
			m = math.Min(m, x)
		}
	}
	return m
}

func span(xs []float64) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, x := range xs {
		lo, hi = math.Min(lo, x), math.Max(hi, x)
	}
	// Measured in slots, which is what the normalisation divides out.
	return (hi - lo) / (2 * lo)
}

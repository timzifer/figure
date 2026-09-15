package stat_test

import (
	"testing"

	"github.com/timzifer/figure/stat"
)

// chainOf is the worst case for a level-at-a-time sweep: every node one level
// below the one before, so the sweep needs a pass per node.
func chainOf(n int) []int {
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i - 1
	}
	return parent
}

// reversedChainOf is the same chain written leaf first, so that no node's
// parent has been seen by the time the node is read.
func reversedChainOf(n int) []int {
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i + 1
	}
	parent[n-1] = stat.NoParent
	return parent
}

func BenchmarkDepthChain20k(b *testing.B) {
	parent := chainOf(20_000)
	var dst []int
	for b.Loop() {
		dst = stat.AppendDepth(dst, parent)
	}
}

func BenchmarkDepthReversedChain20k(b *testing.B) {
	parent := reversedChainOf(20_000)
	var dst []int
	for b.Loop() {
		dst = stat.AppendDepth(dst, parent)
	}
}

// sweepDepth is the level-at-a-time definition Depth used to be: quadratic on
// a chain, and obviously right. It is the oracle the linear walk is held to.
func sweepDepth(parent []int) []int {
	n := len(parent)
	root := func(i int) bool { p := parent[i]; return p < 0 || p >= n || p == i }
	dst := make([]int, n)
	for i := range n {
		dst[i] = -1
		if root(i) {
			dst[i] = 0
		}
	}
	for d := 0; ; d++ {
		grew := false
		for i := range n {
			if dst[i] < 0 && !root(i) && dst[parent[i]] == d {
				dst[i], grew = d+1, true
			}
		}
		if !grew {
			return dst
		}
	}
}

func TestDepthAgreesWithTheLevelSweepOnEveryShape(t *testing.T) {
	// A fixed pseudo-random stream, so a failure names a reproducible table:
	// forests, cycles, tails hanging off cycles, self-parents and parents out
	// of range, in every order.
	state := uint64(0x9e3779b97f4a7c15)
	next := func(k int) int {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return int(state % uint64(k))
	}
	var dst []int
	for trial := range 2000 {
		n := 1 + next(40)
		parent := make([]int, n)
		for i := range parent {
			switch next(10) {
			case 0:
				parent[i] = stat.NoParent
			case 1:
				parent[i] = n + next(3) // names a node nobody declared
			default:
				parent[i] = next(n)
			}
		}
		dst = stat.AppendDepth(dst, parent)
		want := sweepDepth(parent)
		for i := range want {
			if dst[i] != want[i] {
				t.Fatalf("trial %d, parent %v:\nDepth = %v\nwant    %v", trial, parent, dst, want)
			}
		}
	}
}

func TestDepthIsLinearOnAChain(t *testing.T) {
	// Two hundred thousand levels, both ways round. The sweep would take
	// minutes; the walk is a few passes over the slice.
	for _, parent := range [][]int{chainOf(200_000), reversedChainOf(200_000)} {
		d := stat.Depth(parent)
		deepest := 0
		for _, v := range d {
			deepest = max(deepest, v)
		}
		if deepest != len(parent)-1 {
			t.Fatalf("deepest node at %d, want %d", deepest, len(parent)-1)
		}
	}
}

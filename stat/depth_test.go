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

func BenchmarkRollupPartitionChain20k(b *testing.B) {
	parent := chainOf(20_000)
	depth := stat.Depth(parent)
	value := make([]float64, len(parent))
	value[len(value)-1] = 1
	var total, lo, hi []float64
	for b.Loop() {
		total = stat.AppendRollup(total, value, parent, depth)
		lo, hi = stat.AppendPartition(lo, hi, total, parent, depth)
	}
}

// sweepRollup and sweepPartition are Rollup and Partition as they were written
// before they grouped the nodes by level: a scan of the whole table per level.
// The results must agree bit for bit, because a treemap's golden files are
// drawn from them and the sums have to be taken in the same order.
func sweepRollup(value []float64, parent, depth []int) []float64 {
	n := min(len(value), len(parent), len(depth))
	dst := append([]float64(nil), value[:n]...)
	deepest := 0
	for _, d := range depth[:n] {
		deepest = max(deepest, d)
	}
	for d := deepest; d > 0; d-- {
		for i := range n {
			if depth[i] == d {
				if p := parent[i]; p >= 0 && p < n {
					dst[p] += dst[i]
				}
			}
		}
	}
	for i := range n {
		if depth[i] < 0 {
			dst[i] = 0
		}
	}
	return dst
}

func sweepPartition(total []float64, parent, depth []int) (lo, hi []float64) {
	n := min(len(total), len(parent), len(depth))
	lo, hi = make([]float64, n), make([]float64, n)
	grand := 0.0
	for i := range n {
		if depth[i] == 0 {
			grand += total[i]
		}
	}
	if !(grand > 0) {
		return lo, hi
	}
	cursor := 0.0
	for i := range n {
		if depth[i] == 0 {
			lo[i] = cursor
			cursor += total[i] / grand
		}
	}
	deepest := 0
	for _, d := range depth[:n] {
		deepest = max(deepest, d)
	}
	for d := 1; d <= deepest; d++ {
		for i := range n {
			if depth[i] == d-1 {
				hi[i] = lo[i]
			}
		}
		for i := range n {
			if depth[i] == d {
				p := parent[i]
				lo[i] = hi[p]
				hi[p] += total[i] / grand
			}
		}
	}
	for i := range n {
		if depth[i] < 0 {
			lo[i], hi[i] = 0, 0
			continue
		}
		hi[i] = lo[i] + total[i]/grand
	}
	return lo, hi
}

func TestRollupAndPartitionAgreeWithTheLevelSweepBitForBit(t *testing.T) {
	state := uint64(0x2545f4914f6cdd1d)
	next := func(k int) int {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return int(state % uint64(k))
	}
	var total, lo, hi []float64
	for trial := range 2000 {
		n := 1 + next(40)
		parent := make([]int, n)
		value := make([]float64, n)
		for i := range parent {
			switch next(10) {
			case 0:
				parent[i] = stat.NoParent
			case 1:
				parent[i] = n + next(3)
			default:
				parent[i] = next(n)
			}
			// Values that do not add up exactly, so a change in the order of
			// the sums would show.
			value[i] = float64(next(1000)) / 7
		}
		depth := stat.Depth(parent)

		total = stat.AppendRollup(total, value, parent, depth)
		wantTotal := sweepRollup(value, parent, depth)
		lo, hi = stat.AppendPartition(lo, hi, total, parent, depth)
		wantLo, wantHi := sweepPartition(wantTotal, parent, depth)
		for i := range n {
			if total[i] != wantTotal[i] || lo[i] != wantLo[i] || hi[i] != wantHi[i] {
				t.Fatalf("trial %d, parent %v, node %d:\ntotal %v lo %v hi %v\nwant  %v lo %v hi %v",
					trial, parent, i, total[i], lo[i], hi[i], wantTotal[i], wantLo[i], wantHi[i])
			}
		}
	}
}

func TestRollupAndPartitionAreLinearOnAChain(t *testing.T) {
	parent := chainOf(200_000)
	depth := stat.Depth(parent)
	value := make([]float64, len(parent))
	value[len(value)-1] = 1
	total := stat.Rollup(value, parent, depth)
	if total[0] != 1 {
		t.Errorf("the root's total is %v, want the one leaf's 1", total[0])
	}
	lo, hi := stat.Partition(total, parent, depth)
	if lo[len(lo)-1] != 0 || hi[len(hi)-1] != 1 {
		t.Errorf("the leaf spans [%v, %v), want the whole interval", lo[len(lo)-1], hi[len(hi)-1])
	}
}

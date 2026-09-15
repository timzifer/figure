package stat

import "sync"

// A hierarchy is a self-referential edge table: every row is a node, and the
// row names the node that is its parent. That is the shape
// docs/chart-types.md argues a treemap, an icicle and a sunburst all read, and
// the three functions here are what turn it into geometry.
//
// Every one of them takes the parent link as an index rather than as a name.
// Naming is the caller's business, and deliberately so: which node is "first"
// decides the order of everything downstream, and that order has to come from
// the order the rows appeared in the source table rather than from a map
// (docs/adr/0012-parallel-panels.md). A package that knows about numbers and
// nothing else cannot intern a string without inventing an order, so it does
// not try — the geom interns, and hands indices down.
//
// The three run in sequence, each reading the one before: [Depth] says how far
// every node is from a root, [Rollup] adds every subtree up, and [Partition]
// turns the totals into a span of the unit interval per node. An icicle draws
// those spans against depth directly; a treemap feeds them to [Squarify] one
// sibling group at a time.

// NoParent is the parent of a root. Any negative index means the same thing;
// this is the one to write.
const NoParent = -1

// Depth returns how far each node is from a root: zero for a root, one more
// than its parent for everyone else.
//
// parent[i] is the index of i's parent, or [NoParent] — or any index outside
// the list, which means the same. A node that cannot be reached from a root is
// on a cycle, and comes back as -1 rather than as a guess or a panic: a
// hierarchy with a cycle is not a hierarchy, and the caller is the one that can
// say so in terms of its own data.
//
// It walks up from each node rather than recursing, so a hierarchy deep enough
// to blow a stack does not, and it costs time linear in the number of nodes
// however deep the hierarchy is: every node is given its depth once and never
// walked through again. That is a bounded traversal rather than an iteration
// to convergence, which makes it a pure function of its input in the sense
// ADR 0012 requires.
func Depth(parent []int) []int { return AppendDepth(nil, parent) }

// Markers dst holds while [AppendDepth] runs. A settled node holds its depth,
// or -1 for one on a cycle, and neither is ever overwritten.
const (
	depthUnknown = -2
	depthOnWalk  = -3
	depthOnCycle = -1
)

// AppendDepth is [Depth] writing into dst, which it truncates and grows as
// needed.
//
// It needs no memory beyond dst. From each unsettled node it walks up the
// parent links marking what it passes, until it reaches a node that is
// settled, a root, or a node it marked on this same walk — which is a cycle.
// It then walks the same path a second time, now knowing how long it is, and
// settles every node on it: one more than the node it stopped at, counting
// back down, or -1 when the walk ended on a cycle. A node that merely hangs
// off a cycle is as unreachable from a root as one on it, and gets -1 too.
func AppendDepth(dst []int, parent []int) []int {
	n := len(parent)
	dst = dst[:0]
	for range n {
		dst = append(dst, depthUnknown)
	}
	for i := range n {
		if dst[i] != depthUnknown {
			continue
		}

		// First walk: mark the path and find where it ends.
		steps, v := 0, i
		for dst[v] == depthUnknown && !isRoot(parent, v, n) {
			dst[v] = depthOnWalk
			v = parent[v]
			steps++
		}
		base := depthOnCycle
		switch {
		case dst[v] == depthUnknown:
			// A root nobody had reached yet.
			dst[v], base = 0, 0
		case dst[v] >= 0:
			base = dst[v]
		}

		// Second walk: settle the path, deepest node first.
		for k, u := 0, i; k < steps; k, u = k+1, parent[u] {
			if base == depthOnCycle {
				dst[u] = depthOnCycle
				continue
			}
			dst[u] = base + steps - k
		}
	}
	return dst
}

// isRoot reports whether i has no parent inside the list. An index out of
// range is a root rather than an error: a table that names a parent nothing
// else declares has said the node is a top-level one, and refusing to draw it
// would lose a row the reader can see in the data.
func isRoot(parent []int, i, n int) bool {
	p := parent[i]
	return p < 0 || p >= n || p == i
}

// Rollup returns each node's total: its own value plus every descendant's.
//
// value[i] is what row i carries on its own, which for the usual hierarchy —
// where only the leaves are measured — is zero for every internal node. depth
// comes from [Depth]; a node it left at -1 is on a cycle and contributes to
// nothing, including itself.
//
// A negative value is summed as it stands rather than clamped. A tree that
// mixes signs has no sensible area to draw and the caller is where that is
// worth saying; silently taking the absolute value would draw a picture the
// numbers do not support.
func Rollup(value []float64, parent, depth []int) []float64 {
	return AppendRollup(nil, value, parent, depth)
}

// AppendRollup is [Rollup] writing into dst, which it truncates and grows as
// needed. It is the form a geom calls, because a chart redrawn every frame
// should not allocate a total per node per frame.
func AppendRollup(dst []float64, value []float64, parent, depth []int) []float64 {
	n := min(len(value), len(parent), len(depth))
	dst = dst[:0]
	for i := range n {
		dst = append(dst, value[i])
	}
	// Deepest first, so that a node's own subtree is complete before it is
	// added into its parent's.
	lv := acquireLevels(depth[:n])
	defer lv.release()
	for d := lv.deepest(); d > 0; d-- {
		for _, i := range lv.at(d) {
			if p := parent[i]; p >= 0 && p < n {
				dst[p] += dst[i]
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

// Partition returns each node's half-open span [lo, hi) of the unit interval:
// the fraction of the whole hierarchy its subtree occupies, and where that
// fraction sits.
//
// It is the layout an icicle and a sunburst draw directly — a node's span
// across, its depth out — and the ranges a treemap hands to [Squarify] one
// sibling group at a time. total comes from [Rollup].
//
// Siblings are laid out in index order, which is the order their rows appeared
// in the source table. Children fill their parent's span from its start; a
// parent that carries a value of its own beyond its children's keeps the
// remainder, which is what makes an unaccounted-for share visible rather than
// invisible.
//
// Every span is zero when the totals sum to nothing, rather than a division by
// zero: a hierarchy of zeroes has no shape, and the caller draws nothing.
func Partition(total []float64, parent, depth []int) (lo, hi []float64) {
	return AppendPartition(nil, nil, total, parent, depth)
}

// AppendPartition is [Partition] writing into lo and hi, which it truncates and
// grows as needed.
func AppendPartition(lo, hi []float64, total []float64, parent, depth []int) ([]float64, []float64) {
	n := min(len(total), len(parent), len(depth))
	lo, hi = lo[:0], hi[:0]
	for range n {
		lo, hi = append(lo, 0), append(hi, 0)
	}

	grand := 0.0
	for i := range n {
		if depth[i] == 0 {
			grand += total[i]
		}
	}
	if !(grand > 0) {
		return lo, hi
	}

	// hi doubles as the cursor into each parent's span while its children are
	// being placed, and is overwritten with the real far edge at the end. That
	// saves a third buffer for state that lives for one pass — the same reason
	// stat's Append forms exist at all.
	cursor := 0.0
	for i := range n {
		if depth[i] == 0 {
			lo[i] = cursor
			cursor += total[i] / grand
		}
	}
	lv := acquireLevels(depth[:n])
	defer lv.release()
	for d := 1; d <= lv.deepest(); d++ {
		for _, i := range lv.at(d - 1) {
			hi[i] = lo[i]
		}
		for _, i := range lv.at(d) {
			p := parent[i]
			lo[i] = hi[p]
			hi[p] += total[i] / grand
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

// levels is a hierarchy's nodes grouped by depth: every node at depth d, in
// index order, is order[start[d]:start[d+1]].
//
// It is what lets [AppendRollup] and [AppendPartition] visit one level at a
// time without scanning the whole table for each level, which on a hierarchy n
// levels deep is n scans of n nodes. Grouping is a counting sort — one pass to
// count, one to place — and it is stable, so a level is still visited in the
// order its rows appeared and every sum is taken in the order it always was.
//
// The two functions return caller-owned slices and have no room in their
// signatures for a third buffer, so the grouping comes from a pool: a chart
// redrawn every frame takes the same one back each time rather than
// allocating it.
type levels struct {
	order, start []int
}

var levelPool = sync.Pool{New: func() any { return new(levels) }}

// acquireLevels groups the nodes of depth by level. A node at depth -1 is on a
// cycle and belongs to no level.
func acquireLevels(depth []int) *levels {
	lv := levelPool.Get().(*levels)
	deepest := -1
	for _, d := range depth {
		deepest = max(deepest, d)
	}
	lv.start = growInts(lv.start, deepest+2)
	clear(lv.start)
	for _, d := range depth {
		if d >= 0 {
			lv.start[d+1]++
		}
	}
	for d := 1; d < len(lv.start); d++ {
		lv.start[d] += lv.start[d-1]
	}
	lv.order = growInts(lv.order, lv.start[len(lv.start)-1])
	// start[d] doubles as level d's fill cursor, which leaves it pointing at
	// the start of level d+1; shifting back afterwards restores it.
	for i, d := range depth {
		if d >= 0 {
			lv.order[lv.start[d]] = i
			lv.start[d]++
		}
	}
	for d := len(lv.start) - 1; d > 0; d-- {
		lv.start[d] = lv.start[d-1]
	}
	lv.start[0] = 0
	return lv
}

func (lv *levels) release() { levelPool.Put(lv) }

// deepest is the deepest level with a node on it, or -1 for no nodes.
func (lv *levels) deepest() int { return len(lv.start) - 2 }

// at lists the nodes at depth d, in index order.
func (lv *levels) at(d int) []int {
	if d < 0 || d > lv.deepest() {
		return nil
	}
	return lv.order[lv.start[d]:lv.start[d+1]]
}

func growInts(dst []int, n int) []int {
	if cap(dst) < n {
		return make([]int, n)
	}
	return dst[:n]
}

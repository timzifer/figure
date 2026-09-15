package stat

import "math"

// Tidy lays a hierarchy out across the unit interval: one breadth per node, so
// that a tree drawn with that breadth across and a height out has no two
// subtrees overlapping and every parent centred over its children.
//
// It reads the same parent links and depths [Rollup] and [Partition] do, and
// like them it takes indices rather than names — see the note at the top of
// this file's package for why.
//
// It is a struct with a Reset rather than a pair of functions for the reason
// [Chord] is: the layout keeps a dozen per-node buffers, and a chart redrawn
// every frame should reuse them rather than allocate them again.
//
// # Two layouts
//
// [Tidy.Reset] is the Reingold–Tilford tidy tree, in the linear-time form
// Buchheim, Jünger and Leipert published. Subtrees are pushed together level by
// level until they would touch, so a shallow branch beside a deep one tucks in
// above it rather than claiming a whole column. That is the right answer when
// a node's height *is* its depth — an org chart, a decision tree, a file tree —
// because two nodes at different depths are then at different heights and
// cannot collide.
//
// [Tidy.ResetLeaves] gives every leaf its own slot, in order, and centres each
// parent over its children. It is the right answer when heights come from a
// column — a dendrogram's merge distances, a phylogram's branch lengths —
// because there a leaf's depth says nothing about where it is drawn, and the
// compaction the tidy tree does by depth would stack leaves on top of one
// another. It is also what lines a dendrogram's leaves up with the rows of a
// heatmap beside it.
//
// # Order
//
// Siblings are laid out in index order, which is the order their rows appeared
// in the caller's table. Nothing here sorts and nothing here reads a map; the
// layout is a pure function of its input (docs/adr/0012-parallel-panels.md).
//
// The zero Tidy is ready to use.
type Tidy struct {
	// X is each node's breadth, in (0, 1), indexed like the parent links. A
	// node that is on a cycle — depth -1 — has no place and is NaN.
	X []float64
	// Leaves lists the leaves in the order the layout places them, left to
	// right: a depth-first walk with children in index order. A caller that
	// draws a heatmap beside a dendrogram orders its rows by it.
	Leaves []int

	// The per-node state of the walk. Node n is a virtual parent of every
	// root, which is what lets a forest lay out as one tree, and n+1 is a
	// virtual parent of that one, which is what the published algorithm's
	// second walk starts from.
	par, first, count, kids, idx []int
	thread, anc, defAnc          []int
	z, mod, change, shift        []float64
	order, stack                 []int
}

// Reset lays the hierarchy out as a tidy tree. parent[i] is i's parent, or
// [NoParent]; depth comes from [Depth].
func (t *Tidy) Reset(parent, depth []int) {
	n := t.build(parent, depth)
	for _, v := range t.order {
		t.firstWalk(v, n)
	}
	t.mod[n+1] = -t.z[n]
	t.secondWalk()
	t.normalise(n)
}

// ResetLeaves lays the hierarchy out with one slot per leaf and every parent
// centred over its children. parent[i] is i's parent, or [NoParent]; depth
// comes from [Depth].
func (t *Tidy) ResetLeaves(parent, depth []int) {
	n := t.build(parent, depth)
	for _, v := range t.order {
		if v == n || t.count[v] == 0 {
			continue
		}
		lo, hi := t.kids[t.first[v]], t.kids[t.first[v]+t.count[v]-1]
		t.z[v] = (t.z[lo] + t.z[hi]) / 2
	}
	t.normalise(n)
}

// build resolves the child lists, the post-order the first walk runs in and
// the left-to-right leaf order, and returns the index of the virtual root.
func (t *Tidy) build(parent, depth []int) int {
	n := min(len(parent), len(depth))
	size := n + 2
	v, w := n, n+1

	t.par = fillInts(t.par, size, -1)
	t.count = fillInts(t.count, size, 0)
	t.first = fillInts(t.first, size, 0)
	t.idx = fillInts(t.idx, size, 0)
	t.thread = fillInts(t.thread, size, -1)
	t.defAnc = fillInts(t.defAnc, size, -1)
	t.anc = fillInts(t.anc, size, 0)
	t.z = fillFloats(t.z, size, 0)
	t.mod = fillFloats(t.mod, size, 0)
	t.change = fillFloats(t.change, size, 0)
	t.shift = fillFloats(t.shift, size, 0)
	for i := range size {
		t.anc[i] = i
	}

	// Parents, with every root under the virtual node and that one under the
	// second. A node on a cycle is under nothing and is never visited.
	for i := range n {
		switch {
		case depth[i] < 0:
			continue
		case depth[i] == 0:
			t.par[i] = v
		default:
			t.par[i] = parent[i]
		}
		t.count[t.par[i]]++
	}
	t.par[v] = w
	t.count[w] = 1

	// Child lists in one flat slice, each parent's children in index order.
	at := 0
	for i := range size {
		t.first[i] = at
		at += t.count[i]
	}
	t.kids = fillInts(t.kids, at, 0)
	// idx is borrowed as each parent's fill cursor, and then given its real
	// meaning — a node's position among its siblings — by the loop after.
	for i := range size {
		p := t.par[i]
		if p < 0 {
			continue
		}
		t.kids[t.first[p]+t.idx[p]] = i
		t.idx[p]++
	}
	for p := range size {
		t.idx[p] = 0
	}
	for p := range size {
		for k := range t.count[p] {
			t.idx[t.kids[t.first[p]+k]] = k
		}
	}

	// Post-order from the virtual root, children left to right, without
	// recursion: a hierarchy deep enough to exhaust a goroutine's stack is
	// still a hierarchy. The stack holds a node and how many of its children
	// have been entered, packed as two entries.
	t.order = t.order[:0]
	t.Leaves = t.Leaves[:0]
	stack := t.stack[:0]
	stack = append(stack, v, 0)
	for len(stack) > 0 {
		top := len(stack) - 2
		node, next := stack[top], stack[top+1]
		if next < t.count[node] {
			stack[top+1]++
			stack = append(stack, t.kids[t.first[node]+next], 0)
			continue
		}
		stack = stack[:top]
		t.order = append(t.order, node)
		if node < n && t.count[node] == 0 {
			t.z[node] = float64(len(t.Leaves))
			t.Leaves = append(t.Leaves, node)
		}
	}
	t.stack = stack
	return n
}

// separation is how far apart two neighbouring nodes are kept: one slot
// between siblings and two between cousins or separate roots, so a reader can
// see where one family ends and the next begins. It is the convention the
// published algorithm is usually drawn with.
func (t *Tidy) separation(a, b, n int) float64 {
	if t.par[a] == t.par[b] && t.par[a] < n {
		return 1
	}
	return 2
}

func (t *Tidy) nextLeft(v int) int {
	if t.count[v] > 0 {
		return t.kids[t.first[v]]
	}
	return t.thread[v]
}

func (t *Tidy) nextRight(v int) int {
	if t.count[v] > 0 {
		return t.kids[t.first[v]+t.count[v]-1]
	}
	return t.thread[v]
}

func (t *Tidy) firstWalk(v, n int) {
	p := t.par[v]
	w := -1
	if i := t.idx[v]; i > 0 {
		w = t.kids[t.first[p]+i-1]
	}
	if t.count[v] > 0 {
		t.executeShifts(v)
		lo, hi := t.kids[t.first[v]], t.kids[t.first[v]+t.count[v]-1]
		mid := (t.z[lo] + t.z[hi]) / 2
		if w >= 0 {
			t.z[v] = t.z[w] + t.separation(v, w, n)
			t.mod[v] = t.z[v] - mid
		} else {
			t.z[v] = mid
		}
	} else if w >= 0 {
		t.z[v] = t.z[w] + t.separation(v, w, n)
	} else {
		t.z[v] = 0
	}
	a := t.defAnc[p]
	if a < 0 {
		a = t.kids[t.first[p]]
	}
	t.defAnc[p] = t.apportion(v, w, a, n)
}

// apportion pushes v's subtree right until it clears everything to its left,
// walking the two facing contours down together, and spreads the shift over
// the siblings in between so that they stay evenly spaced.
func (t *Tidy) apportion(v, w, ancestor, n int) int {
	if w < 0 {
		return ancestor
	}
	vip, vop, vim := v, v, w
	vom := t.kids[t.first[t.par[vip]]]
	sip, sop, sim, som := t.mod[vip], t.mod[vop], t.mod[vim], t.mod[vom]
	for {
		vim, vip = t.nextRight(vim), t.nextLeft(vip)
		if vim < 0 || vip < 0 {
			break
		}
		vom, vop = t.nextLeft(vom), t.nextRight(vop)
		t.anc[vop] = v
		if s := t.z[vim] + sim - t.z[vip] - sip + t.separation(vim, vip, n); s > 0 {
			t.moveSubtree(t.nextAncestor(vim, v, ancestor), v, s)
			sip += s
			sop += s
		}
		sim += t.mod[vim]
		sip += t.mod[vip]
		som += t.mod[vom]
		sop += t.mod[vop]
	}
	if vim >= 0 && t.nextRight(vop) < 0 {
		t.thread[vop] = vim
		t.mod[vop] += sim - sop
	}
	if vip >= 0 && t.nextLeft(vom) < 0 {
		t.thread[vom] = vip
		t.mod[vom] += sip - som
		ancestor = v
	}
	return ancestor
}

func (t *Tidy) nextAncestor(vim, v, ancestor int) int {
	if a := t.anc[vim]; t.par[a] == t.par[v] {
		return a
	}
	return ancestor
}

func (t *Tidy) moveSubtree(wm, wp int, s float64) {
	c := s / float64(t.idx[wp]-t.idx[wm])
	t.change[wp] -= c
	t.shift[wp] += s
	t.change[wm] += c
	t.z[wp] += s
	t.mod[wp] += s
}

func (t *Tidy) executeShifts(v int) {
	s, c := 0.0, 0.0
	for k := t.count[v] - 1; k >= 0; k-- {
		w := t.kids[t.first[v]+k]
		t.z[w] += s
		t.mod[w] += s
		c += t.change[w]
		s += t.shift[w] + c
	}
}

// secondWalk turns the relative positions into absolute ones, parents before
// children. The post-order reversed is such an order.
func (t *Tidy) secondWalk() {
	for k := len(t.order) - 1; k >= 0; k-- {
		v := t.order[k]
		p := t.par[v]
		t.z[v] += t.mod[p]
		t.mod[v] += t.mod[p]
	}
}

// normalise writes X: the laid-out positions scaled into the unit interval
// with half a slot of margin at each end, so the outermost nodes sit inside
// the axis rather than on its edge. A single node sits in the middle.
func (t *Tidy) normalise(n int) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range t.order {
		if v < n {
			lo, hi = math.Min(lo, t.z[v]), math.Max(hi, t.z[v])
		}
	}
	t.X = fillFloats(t.X, n, math.NaN())
	for _, v := range t.order {
		if v < n {
			t.X[v] = (t.z[v] - lo + 0.5) / (hi - lo + 1)
		}
	}
}

func fillInts(dst []int, n, v int) []int {
	if cap(dst) < n {
		dst = make([]int, n)
	}
	dst = dst[:n]
	for i := range dst {
		dst[i] = v
	}
	return dst
}

func fillFloats(dst []float64, n int, v float64) []float64 {
	if cap(dst) < n {
		dst = make([]float64, n)
	}
	dst = dst[:n]
	for i := range dst {
		dst[i] = v
	}
	return dst
}

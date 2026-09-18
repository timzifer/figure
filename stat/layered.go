package stat

import "sort"

// LayeredSweeps is how many times a [Layered] reduces crossings, and how many
// times it straightens what is left.
//
// It is a constant rather than a tolerance for the reason [SankeySweeps] is
// one: a pass that ran until it settled would make the picture depend on
// floating-point noise, and a chart whose panels are built on separate
// goroutines has to be byte-identical to one built serially
// (docs/adr/0012-parallel-panels.md). Six is where the arrangement stops
// visibly improving on the graphs this was tested against.
const LayeredSweeps = 6

// LayeredPoint is one bend an edge makes on its way between two ranks.
type LayeredPoint struct{ X, Y float64 }

// LayeredNode is one node's place in a layered graph: which rank it stands on,
// and where in the unit square it sits.
type LayeredNode struct {
	// Rank is the node's layer, counting from zero at the sources.
	Rank int
	// X is the node's position across its rank and Y its rank's height, both
	// in (0, 1). Rank zero is at Y = 0, so a coord that puts y = 1 outermost
	// draws the sources at the hub — docs/adr/0039-relational-layouts.md.
	X, Y float64
}

// LayeredEdge is what became of one row of the caller's edge list. The rows
// keep their order and their count, so a caller can read the edge that a row
// drew without tracking an offset of its own.
type LayeredEdge struct {
	// Lo and Hi bound this edge's bends in [Layered.Bends]: the points it
	// passes through between its two ends, in order, empty for an edge that
	// joins adjacent ranks.
	Lo, Hi int
	// Back reports an edge that runs against the rank order — one of the
	// edges the cycle-breaking pass chose. It is drawn in its true direction;
	// the flag is there so that a mark can say so.
	Back bool
	// Self reports an edge whose two ends are the same node. It never reaches
	// the layering, and how it is drawn is the mark's business.
	Self bool
	// OK reports an edge whose two ends were both inside the node count. One
	// that was not is left at the zero value and laid nothing out.
	OK bool
}

// Layered lays a directed graph out in ranks: every edge that is not on a cycle
// runs from a lower rank to a higher one, and an edge that spans more than one
// rank bends through a point on each rank it crosses.
//
// It is a struct with a [Layered.Reset] rather than a pair of functions for the
// reason [Sankey] and [Tidy] are: the layout keeps a dozen buffers the size of
// the data, and a chart redrawn every frame should reuse them rather than
// allocate them again.
//
// # Cycles are broken, not refused
//
// [Sankey] reports a cyclic edge list and lays nothing out, because a flow that
// returns to where it came from has no column to stand in. A state machine that
// cannot return to a previous state is not a state machine, so this layout
// breaks cycles instead: a depth-first walk in index order marks the edges that
// close one, the ranks are assigned without them, and they are reported through
// [LayeredEdge.Back] so that a mark can draw them for what they are. Self-edges
// never reach the layering at all.
//
// # What it decides, and what it does not
//
// It decides three things: which rank each node stands on, the order of the
// nodes within a rank, and where along the rank each one sits.
//
// The order is where a sort appears, which is the thing [Sankey] and [Tidy]
// both refuse to do. It is admissible here because the sort is **stable** and
// the order it stabilises is the order the caller's rows appeared in: two nodes
// whose barycentres are equal — the common case, not the corner one — keep the
// order they came in with rather than whichever one the sort happened to move.
// The result is a pure function of the input in the sense
// docs/adr/0012-parallel-panels.md needs, on one goroutine or on eight.
//
// It is not, and does not claim to be, the arrangement with the fewest
// crossings. Crossing minimisation is NP-hard; the barycentre sweep is a
// heuristic, and a different row order gives a different picture that is just
// as valid. See docs/adr/0072-layered-graph-layout.md.
//
// # It never sees a label
//
// Nodes are placed knowing the graph and nothing else, so the layout does not
// depend on how wide a name happens to be in the backend's font. A mark that
// draws a box round a node measures the text itself and centres the box on the
// position this returned.
//
// # What it costs
//
// An edge that spans r ranks routes through r-1 bends, so the work is the
// number of ranks an edge list crosses rather than the number of edges it has.
// A graph with a long critical path and edges that jump it — a random edge list
// is the worst case — ranks into as many layers as it has nodes and routes a
// bend per layer per edge. That is inherent to layering rather than to this
// implementation, and it is the reason this mark is for graphs that are nearly
// layered already: a state machine, a pipeline, a dependency tree.
//
// The zero Layered is unusable; call [Layered.Reset] first.
type Layered struct {
	// Nodes has one entry per node, indexed as the caller's edge list indexes
	// them.
	Nodes []LayeredNode
	// Edges has one entry per row of the caller's edge list, in that order.
	Edges []LayeredEdge
	// Bends holds every edge's intermediate points, packed; edge e's own run
	// is Bends[e.Lo:e.Hi].
	Bends []LayeredPoint

	// Ranks is how many layers the graph has.
	Ranks int

	// The DAG the ranks are assigned over: the caller's edges less the
	// self-edges, the out-of-range ones and the ones that close a cycle.
	keep    []int
	rank    []int
	indeg   []int
	state   []int8
	adjHead []int
	adjNext []int
	adjTo   []int
	adjEdge []int
	queue   []int
	visit   []int

	// The layout's own elements: the nodes, then one dummy per rank an edge
	// crosses. Dummies are what makes a long edge bend rather than cut across
	// the ranks between its ends.
	elemRank []int
	pos      []float64
	slot     []int
	row      []int
	rowStart []int
	rowAt    []int
	key      []float64
	merge    []int
	up       neighbours
	down     neighbours
	nbr      []float64
}

// neighbours is a forward-linked adjacency over the layout's elements, in the
// order the elements were created. Nothing here reads a map, so the order a
// sweep sees is the order the rows gave it.
type neighbours struct {
	head, next, to []int
}

func (a *neighbours) reset(elems int) {
	a.head = fillInts(a.head, elems, -1)
	a.next, a.to = a.next[:0], a.to[:0]
}

func (a *neighbours) add(from, to int) {
	a.next = append(a.next, a.head[from])
	a.to = append(a.to, to)
	a.head[from] = len(a.to) - 1
}

// Reset lays out the edge list from[i] → to[i] over the given number of nodes.
//
// There is no value and no padding: an edge's magnitude is a channel the mark
// applies to what it draws, and the space between nodes is a fraction of a
// panel whose width this does not know. Everything here is the unit square.
//
// An edge naming a node outside [0, nodes) is skipped and its [LayeredEdge.OK]
// is false. A graph with no edges at all still places its nodes, on one rank.
func (l *Layered) Reset(from, to []int, nodes int) {
	l.Nodes, l.Edges, l.Bends = l.Nodes[:0], l.Edges[:0], l.Bends[:0]
	l.Ranks = 0
	if nodes <= 0 {
		return
	}
	m := min(len(from), len(to))

	l.classify(from, to, m, nodes)
	l.breakCycles(from, to, nodes)
	l.assignRanks(from, to, nodes)
	elems := l.route(from, to, nodes)
	l.order(elems)
	l.place()
	l.emit(nodes)
}

// classify sorts the caller's rows into the ones that can carry a rank
// constraint and the ones that cannot, and seeds l.keep with the first kind.
func (l *Layered) classify(from, to []int, m, nodes int) {
	l.keep = l.keep[:0]
	for i := range m {
		e := LayeredEdge{OK: from[i] >= 0 && from[i] < nodes && to[i] >= 0 && to[i] < nodes}
		if e.OK && from[i] == to[i] {
			e.Self = true
		}
		l.Edges = append(l.Edges, e)
		if e.OK && !e.Self {
			l.keep = append(l.keep, i)
		}
	}
}

// breakCycles marks the edges that close a cycle and drops them from l.keep.
//
// The walk is depth-first from every node in index order, over the edges in the
// order the rows gave them, with an explicit stack rather than recursion so
// that a graph deeper than the goroutine stack still lays out. An edge that
// reaches a node still on the stack — state 1 — is the edge that closes the
// cycle, and which edge that is depends on the row order, deterministically.
func (l *Layered) breakCycles(from, to []int, nodes int) {
	l.adjacency(from, to, nodes)
	l.state = fillInts8(l.state, nodes, 0)
	l.visit = fillInts(l.visit, nodes, 0)
	l.queue = l.queue[:0]

	for root := range nodes {
		if l.state[root] != 0 {
			continue
		}
		l.state[root] = 1
		l.visit[root] = l.adjHead[root]
		l.queue = append(l.queue, root)
		for len(l.queue) > 0 {
			v := l.queue[len(l.queue)-1]
			a := l.visit[v]
			if a < 0 {
				l.state[v] = 2
				l.queue = l.queue[:len(l.queue)-1]
				continue
			}
			l.visit[v] = l.adjNext[a]
			w, e := l.adjTo[a], l.adjEdge[a]
			switch l.state[w] {
			case 1:
				l.Edges[e].Back = true
			case 0:
				l.state[w] = 1
				l.visit[w] = l.adjHead[w]
				l.queue = append(l.queue, w)
			}
		}
	}

	kept := l.keep[:0]
	for _, e := range l.keep {
		if !l.Edges[e].Back {
			kept = append(kept, e)
		}
	}
	l.keep = kept
}

// adjacency builds the forward adjacency the cycle walk reads, over every edge
// still in l.keep.
func (l *Layered) adjacency(from, to []int, nodes int) {
	l.adjHead = fillInts(l.adjHead, nodes, -1)
	l.adjNext, l.adjTo, l.adjEdge = l.adjNext[:0], l.adjTo[:0], l.adjEdge[:0]
	// Appended in reverse so that the list reads forwards: the walk takes the
	// rows in the order they were written, which is what makes the choice of
	// back edge a property of the table rather than of this loop.
	for i := len(l.keep) - 1; i >= 0; i-- {
		e := l.keep[i]
		l.adjNext = append(l.adjNext, l.adjHead[from[e]])
		l.adjTo = append(l.adjTo, to[e])
		l.adjEdge = append(l.adjEdge, e)
		l.adjHead[from[e]] = len(l.adjTo) - 1
	}
}

// assignRanks gives every node the longest path from a source, which is the
// layering that puts each node one rank below its deepest predecessor.
//
// Longest path and not network simplex: the simplex gives shorter edges and
// needs a pivot rule, and a pivot rule is one more tie to break.
func (l *Layered) assignRanks(from, to []int, nodes int) {
	l.adjacency(from, to, nodes)
	l.rank = fillInts(l.rank, nodes, 0)
	l.indeg = fillInts(l.indeg, nodes, 0)
	for _, e := range l.keep {
		l.indeg[to[e]]++
	}

	l.queue = l.queue[:0]
	for v := range nodes {
		if l.indeg[v] == 0 {
			l.queue = append(l.queue, v)
		}
	}
	for head := 0; head < len(l.queue); head++ {
		v := l.queue[head]
		for a := l.adjHead[v]; a >= 0; a = l.adjNext[a] {
			w := l.adjTo[a]
			l.rank[w] = max(l.rank[w], l.rank[v]+1)
			l.indeg[w]--
			if l.indeg[w] == 0 {
				l.queue = append(l.queue, w)
			}
		}
	}

	l.Ranks = 1
	for v := range nodes {
		l.Ranks = max(l.Ranks, l.rank[v]+1)
	}
}

// route creates the layout's elements — the nodes, then one dummy per rank a
// long edge crosses — and the two neighbour lists a sweep reads. It returns how
// many elements there are.
//
// Each edge's dummies are created together and in edge order, so an element's
// index is decided by the table and not by the traversal.
func (l *Layered) route(from, to []int, nodes int) int {
	l.elemRank = l.elemRank[:0]
	for v := range nodes {
		l.elemRank = append(l.elemRank, l.rank[v])
	}
	for _, e := range l.keep {
		lo, hi := l.rank[from[e]], l.rank[to[e]]
		l.Edges[e].Lo = len(l.elemRank) - nodes
		for r := lo + 1; r < hi; r++ {
			l.elemRank = append(l.elemRank, r)
		}
		l.Edges[e].Hi = len(l.elemRank) - nodes
	}

	elems := len(l.elemRank)
	l.up.reset(elems)
	l.down.reset(elems)
	for _, e := range l.keep {
		prev := from[e]
		for d := l.Edges[e].Lo; d < l.Edges[e].Hi; d++ {
			cur := nodes + d
			l.down.add(prev, cur)
			l.up.add(cur, prev)
			prev = cur
		}
		l.down.add(prev, to[e])
		l.up.add(to[e], prev)
	}
	return elems
}

// order arranges each rank, sweeping down and then up, LayeredSweeps times.
//
// A sweep sorts every rank by the mean position of the neighbours on the side
// it came from. The sort is stable and an element with no neighbour on that
// side keeps its current position as its key, so nothing collapses to the left
// edge and nothing moves without a reason to.
func (l *Layered) order(elems int) {
	l.rows(elems)
	l.pos = fillFloats(l.pos, elems, 0)
	for _, e := range l.row {
		l.pos[e] = float64(l.rowAt[e])
	}
	l.key = fillFloats(l.key, elems, 0)

	for range LayeredSweeps {
		for r := 1; r < l.Ranks; r++ {
			l.sortRank(r, &l.up)
		}
		for r := l.Ranks - 2; r >= 0; r-- {
			l.sortRank(r, &l.down)
		}
	}
}

// rows buckets the elements by rank, keeping each rank in element order, which
// is the order the caller's rows created them in.
func (l *Layered) rows(elems int) {
	l.rowStart = fillInts(l.rowStart, l.Ranks+1, 0)
	for _, r := range l.elemRank {
		l.rowStart[r+1]++
	}
	for r := range l.Ranks {
		l.rowStart[r+1] += l.rowStart[r]
	}
	l.row = grow(l.row, elems)
	l.rowAt = grow(l.rowAt, elems)
	cursor := growInts(l.slot, l.Ranks)
	copy(cursor, l.rowStart[:l.Ranks])
	for e, r := range l.elemRank {
		l.row[cursor[r]] = e
		l.rowAt[e] = cursor[r] - l.rowStart[r]
		cursor[r]++
	}
	l.slot = cursor
}

// sortRank reorders one rank by the mean of each element's neighbours on the
// given side.
func (l *Layered) sortRank(r int, side *neighbours) {
	band := l.row[l.rowStart[r]:l.rowStart[r+1]]
	if len(band) < 2 {
		return
	}
	for _, e := range band {
		sum, n := 0.0, 0
		for a := side.head[e]; a >= 0; a = side.next[a] {
			sum += l.pos[side.to[a]]
			n++
		}
		if n == 0 {
			l.key[e] = l.pos[e]
			continue
		}
		l.key[e] = sum / float64(n)
	}
	l.sortStable(band)
	for i, e := range band {
		l.pos[e] = float64(i)
		l.rowAt[e] = i
	}
}

// place turns the slot indices into positions in the unit interval and then
// straightens what it can.
//
// Every rank shares one pitch — the widest rank's — so a narrow rank sits
// centred under a wide one rather than stretched across it, which is what makes
// a chain of single nodes draw as a straight line.
func (l *Layered) place() {
	widest := 1
	for r := range l.Ranks {
		widest = max(widest, l.rowStart[r+1]-l.rowStart[r])
	}
	pitch := 1 / float64(widest)

	for r := range l.Ranks {
		band := l.row[l.rowStart[r]:l.rowStart[r+1]]
		left := 0.5 - float64(len(band)-1)*pitch/2
		for i, e := range band {
			l.pos[e] = left + float64(i)*pitch
		}
	}
	l.straighten(pitch)
}

// straighten pulls each element towards the median of its neighbours and then
// pushes the rank apart again, LayeredSweeps times.
//
// It is [Sankey.relax]'s shape and it is there for the same reason: the order
// is already decided, and what is left is to stop a long edge zigzagging
// between two ranks that could have lined up. The median rather than the mean,
// because one distant neighbour should not drag a node off a line it is
// otherwise on.
func (l *Layered) straighten(pitch float64) {
	for range LayeredSweeps {
		for r := 1; r < l.Ranks; r++ {
			l.pull(r, &l.up)
			l.separate(r, pitch)
		}
		for r := l.Ranks - 2; r >= 0; r-- {
			l.pull(r, &l.down)
			l.separate(r, pitch)
		}
	}
}

// sortStable orders band by l.key, leaving equal keys in the order they came
// in — which is the order the caller's rows produced, and the whole reason a
// sort is admissible here at all.
//
// It is a bottom-up merge sort over a buffer the struct owns rather than
// [sort.SliceStable], which takes a closure and a reflect-based swapper and so
// allocates on every call. This one is called 2 × Ranks × [LayeredSweeps] times
// per layout, and docs/adr/0012-parallel-panels.md's neighbour — the claim that
// a frame's cost does not grow with the data — is measured in allocations.
func (l *Layered) sortStable(band []int) {
	n := len(band)
	l.merge = grow(l.merge, n)
	src, dst := band, l.merge
	for width := 1; width < n; width *= 2 {
		for lo := 0; lo < n; lo += 2 * width {
			mid := min(lo+width, n)
			hi := min(lo+2*width, n)
			i, j := lo, mid
			for k := lo; k < hi; k++ {
				// The left run wins a tie, which is what makes the sort stable.
				if i < mid && (j >= hi || l.key[src[i]] <= l.key[src[j]]) {
					dst[k] = src[i]
					i++
					continue
				}
				dst[k] = src[j]
				j++
			}
		}
		src, dst = dst, src
	}
	if &src[0] != &band[0] {
		copy(band, src)
	}
}

func (l *Layered) pull(r int, side *neighbours) {
	for _, e := range l.row[l.rowStart[r]:l.rowStart[r+1]] {
		l.nbr = l.nbr[:0]
		for a := side.head[e]; a >= 0; a = side.next[a] {
			l.nbr = append(l.nbr, l.pos[side.to[a]])
		}
		switch len(l.nbr) {
		case 0:
			continue
		case 1:
			l.pos[e] = l.nbr[0]
			continue
		case 2:
			l.pos[e] = (l.nbr[0] + l.nbr[1]) / 2
			continue
		}
		sort.Float64s(l.nbr)
		l.pos[e] = median(l.nbr)
	}
}

// separate restores the rank's order and its minimum gap after a pull, sweeping
// left to right and then right to left so that the band stays inside the unit
// interval however hard the pull was.
func (l *Layered) separate(r int, pitch float64) {
	band := l.row[l.rowStart[r]:l.rowStart[r+1]]
	if len(band) == 0 {
		return
	}
	lo, hi := pitch/2, 1-pitch/2
	l.pos[band[0]] = max(l.pos[band[0]], lo)
	for i := 1; i < len(band); i++ {
		l.pos[band[i]] = max(l.pos[band[i]], l.pos[band[i-1]]+pitch)
	}
	last := len(band) - 1
	l.pos[band[last]] = min(l.pos[band[last]], hi)
	for i := last - 1; i >= 0; i-- {
		l.pos[band[i]] = min(l.pos[band[i]], l.pos[band[i+1]]-pitch)
	}
}

// emit writes the answer: a position per node and a run of bends per edge.
func (l *Layered) emit(nodes int) {
	span := 1.0
	if l.Ranks > 1 {
		span = float64(l.Ranks - 1)
	}
	for v := range nodes {
		l.Nodes = append(l.Nodes, LayeredNode{
			Rank: l.rank[v],
			X:    l.pos[v],
			Y:    float64(l.rank[v]) / span,
		})
	}

	// The dummy indices route() recorded are offsets into the elements it
	// appended; they become offsets into Bends, which is the same run in the
	// same order.
	for _, e := range l.keep {
		lo, hi := l.Edges[e].Lo, l.Edges[e].Hi
		l.Edges[e].Lo = len(l.Bends)
		for d := lo; d < hi; d++ {
			el := nodes + d
			l.Bends = append(l.Bends, LayeredPoint{X: l.pos[el], Y: float64(l.elemRank[el]) / span})
		}
		l.Edges[e].Hi = len(l.Bends)
	}
	for i := range l.Edges {
		if !l.Edges[i].OK || l.Edges[i].Self || l.Edges[i].Back {
			l.Edges[i].Lo, l.Edges[i].Hi = 0, 0
		}
	}
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func fillInts8(dst []int8, n int, v int8) []int8 {
	if cap(dst) < n {
		dst = make([]int8, n)
	}
	dst = dst[:n]
	for i := range dst {
		dst[i] = v
	}
	return dst
}

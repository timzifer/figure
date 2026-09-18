package stat

import "math"

// StressSweeps is how many majorization passes a [Stress] layout makes.
//
// It is a constant rather than a tolerance, for the reason [SankeySweeps] and
// [LayeredSweeps] are: a pass that ran until it settled would make the picture
// depend on floating-point noise, and a chart whose panels are built on
// separate goroutines has to be byte-identical to one built serially
// (docs/adr/0012-parallel-panels.md).
//
// **Fifty is a budget rather than a convergence point**, and the difference is
// worth the sentence. Measured against the same run carried on to 400 sweeps,
// the stress left over at fifty is under a hundredth of a percent on a 4×5
// lattice and on a 31-node binary tree, under a tenth of a percent on the
// collaboration graph in docs/images/network.png, and about **four percent** on
// a 127-node binary tree. A sweep never raises the stress, so that leftover is
// the whole of what the bound costs — and on a big tree it is not nothing.
//
// Two things it does not mean. Stress is a sum over every pair and not what a
// reader sees: nothing here measures overlapping labels or crossing edges, and
// a drawing can be harder to read at the lower number. And the budget is not
// the biggest lever on how good the arrangement is — see [Stress] on what the
// order of the caller's rows decides.
const StressSweeps = 50

// MaxStressNodes is the largest graph a [Stress] lays out.
//
// Two limits arrive at about the same number, which is why one constant serves
// for both. The picture gives out first: a straight-line drawing of a few
// hundred nodes is a hairball, and the answer past that is a different form
// rather than a bigger picture of this one — [Layered] for a graph that has a
// direction, an adjacency matrix for one that has neither a direction nor a
// shape. The arithmetic gives out just after: every pair of nodes has a
// distance and every sweep reads all of them, so the work is quadratic —
// measured on the machine this was written on, 250 nodes is about 30 ms and
// 500 is about 140, which is a frame rather than a layout.
const MaxStressNodes = 250

// StressPoint is one node's place in the unit square.
type StressPoint struct{ X, Y float64 }

// StressEdge is what became of one row of the caller's edge list. The rows keep
// their order and their count, so a caller can read the edge a row drew without
// tracking an offset of its own.
type StressEdge struct {
	// Self reports an edge whose two ends are the same node. It says nothing
	// about where anything goes, and how it is drawn is the mark's business.
	Self bool
	// OK reports an edge whose two ends were both inside the node count. One
	// that was not is left at the zero value and laid nothing out.
	OK bool
}

// Stress places the nodes of a graph so that the distance between two of them
// on the page is as close as it can be to the number of edges between them.
//
// It is the layout a node-link diagram is drawn with. What it minimises is that
// distance, summed over every pair and weighted, and the number has a name —
// the *stress* — which is the first thing to know about it: whether one
// arrangement is better than another is a measurement here rather than a
// matter of taste.
//
// The method is majorization. At each sweep the stress is replaced by a
// quadratic that sits above it and touches it at the current arrangement, and
// the arrangement is moved downhill on that quadratic instead. Minimising the
// quadratic over *all* the nodes at once is a linear system; this does not
// solve one. It descends the quadratic one node at a time, solving each node's
// own block exactly with the others held still, which is enough for the
// property the bound rests on: the majorizing function never rises, and it sits
// above the stress, so the stress never rises either.
// See docs/adr/0077-a-node-link-layout.md.
//
// Three properties make it a pure function of its input:
//
//   - The starting arrangement comes from classical scaling of the distance
//     table, whose own arithmetic is the caller's rows and nothing else; a
//     graph with no spread at all falls back to a circle in index order.
//   - A sweep visits the nodes in index order, which is the order the caller's
//     rows interned them in, and writes each position back before the next node
//     reads it ([ADR 0012](../../docs/adr/0012-parallel-panels.md)). The
//     simultaneous form of the same update is not a descent at all — see
//     [Stress.sweep].
//   - The sweep count is [StressSweeps] and the refinement count is
//     [StressPowerIterations]; the distances are whole numbers of edges. Nothing
//     here reads a clock, and nothing stops on a tolerance.
//
// # The row order decides which minimum, not just which tie-break
//
// Stress is not convex, and a descent reaches the minimum whose basin it starts
// in. Both the start and the order a sweep visits the nodes in come from the
// order the caller's rows named them, so relabelling the same edges is not a
// relabelling of the same picture: the collaboration graph in
// docs/images/network.png settles at a stress of 3.47 in the order its rows
// arrive in, and at 2.30 under a relabelling of the same edges — a third lower,
// from the same code at the same budget. A 127-node binary tree has two minima
// this reaches, about ten percent apart.
//
// That is a property to know rather than a defect to route around here. It is
// the same sentence every layout in this package carries — the row order is the
// input — with a larger number attached than the others have.
// [github.com/timzifer/figure/geom.Order] is how a caller asks for a different
// one. Running several orders and keeping the arrangement with the lowest
// stress is a thing this could do and does not; it would be its own record,
// because it spends the budget differently rather than more.
//
// It is a struct with a [Stress.Reset] rather than a pair of functions for the
// reason [Sankey], [Tidy] and [Layered] are: the layout keeps several buffers
// the size of the data, one of them quadratic in it, and a chart redrawn every
// frame should reuse them rather than allocate them again.
//
// The zero Stress is unusable; call [Stress.Reset] first.
type Stress struct {
	// Nodes has one entry per node, indexed as the caller's edge list indexes
	// them, in the unit square.
	Nodes []StressPoint
	// Edges has one entry per row of the caller's edge list, in that order.
	Edges []StressEdge
	// Separation is the distance used for two nodes with no path between them:
	// one more than the longest path there is. A graph in several pieces
	// therefore pushes its pieces about its own width apart.
	Separation float64
	// TooMany reports a graph of more than [MaxStressNodes] nodes, in which
	// case nothing is laid out.
	TooMany bool

	adjHead, adjNext, adjTo []int
	queue                   []int
	dist                    []float64
	x, y                    []float64
	rowMean, scratch, probe []float64
}

// Reset lays out the edge list from[i] → to[i] over the given number of nodes.
//
// The edges are read as undirected, because that is what the drawing says: two
// nodes joined by a line are near each other, and which way the line was
// written down is not a distance. A mark that wants to show the direction draws
// it on the line.
//
// An edge naming a node outside [0, nodes) is skipped and its [StressEdge.OK]
// is false. An edge from a node to itself is marked and skipped: it is no
// constraint on where anything goes.
func (s *Stress) Reset(from, to []int, nodes int) {
	s.Nodes, s.Edges = s.Nodes[:0], s.Edges[:0]
	s.Separation, s.TooMany = 0, false
	if nodes <= 0 {
		return
	}
	if nodes > MaxStressNodes {
		s.TooMany = true
		return
	}

	s.adjacency(from, to, nodes)
	s.distances(nodes)
	// The distance table is read twice and written once: classical scaling
	// centres it in place, so the majorization below reads it back out of the
	// same buffer. Keeping one quadratic buffer rather than two is the whole
	// reason for the order.
	s.x, s.y = growFloats(s.x, nodes), growFloats(s.y, nodes)
	s.classical(nodes)
	s.scatter(nodes)
	s.distances(nodes)
	for range StressSweeps {
		s.sweep(nodes)
	}
	s.emit(nodes)
}

// adjacency builds the undirected neighbour lists and classifies the rows.
//
// It is a forward-linked list in row order rather than a map, so every walk
// below meets the neighbours of a node in the order the caller's rows named
// them — this package's rule, and ADR 0012's.
func (s *Stress) adjacency(from, to []int, nodes int) {
	s.adjHead = fillInts(s.adjHead, nodes, -1)
	s.adjNext, s.adjTo = s.adjNext[:0], s.adjTo[:0]
	add := func(a, b int) {
		s.adjNext = append(s.adjNext, s.adjHead[a])
		s.adjTo = append(s.adjTo, b)
		s.adjHead[a] = len(s.adjTo) - 1
	}
	m := min(len(from), len(to))
	for i := range m {
		e := StressEdge{OK: from[i] >= 0 && from[i] < nodes && to[i] >= 0 && to[i] < nodes}
		if e.OK && from[i] == to[i] {
			e.Self = true
		}
		s.Edges = append(s.Edges, e)
		if e.OK && !e.Self {
			add(from[i], to[i])
			add(to[i], from[i])
		}
	}
}

// distances fills the all-pairs shortest path table, in edges.
//
// One breadth-first walk per node, which is what makes the table exact rather
// than sampled: a layout that guessed at the far distances would be a layout
// whose picture changed with the guess. [MaxStressNodes] is what keeps the
// quadratic honest.
func (s *Stress) distances(nodes int) {
	s.dist = growFloats(s.dist, nodes*nodes)
	for i := range s.dist {
		s.dist[i] = -1
	}
	far := 0.0
	for root := range nodes {
		row := s.dist[root*nodes : (root+1)*nodes]
		row[root] = 0
		s.queue = append(s.queue[:0], root)
		for head := 0; head < len(s.queue); head++ {
			at := s.queue[head]
			d := row[at] + 1
			for e := s.adjHead[at]; e >= 0; e = s.adjNext[e] {
				next := s.adjTo[e]
				if row[next] >= 0 {
					continue
				}
				row[next] = d
				s.queue = append(s.queue, next)
			}
		}
		for _, d := range row {
			far = math.Max(far, d)
		}
	}
	// Two nodes with no path between them are given one more than the longest
	// path there is. It is a finite number, so the pieces of a graph in several
	// parts are laid out rather than flung apart, and it is bigger than any
	// real distance, so they do not overlap either.
	s.Separation = far + 1
	for i, d := range s.dist {
		if d < 0 {
			s.dist[i] = s.Separation
		}
	}
}

// StressPowerIterations is how many times [Stress] refines each of the two
// directions it starts from. See [Stress.classical].
const StressPowerIterations = 64

// circle is the fallback starting arrangement: the nodes evenly spaced round a
// circle in the order the caller's rows named them.
//
// It is what [Stress.classical] falls back to when the distances have no two
// directions to be spread along — a graph with no edges at all, where every
// pair is the same distance apart and a ring is the honest picture of that.
func (s *Stress) circle(nodes int) {
	if nodes == 1 {
		s.x[0], s.y[0] = 0, 0
		return
	}
	for i := range nodes {
		a := 2 * math.Pi * float64(i) / float64(nodes)
		s.x[i], s.y[i] = 0.5*math.Cos(a), 0.5*math.Sin(a)
	}
}

// classical is the starting arrangement: classical multidimensional scaling of
// the distance table, which is the two directions the graph is most spread out
// along.
//
// Majorization only ever goes downhill, so where it starts decides which
// minimum it reaches, and this is the whole of that decision. Started from a
// circle it folds a grid in half — measured, not feared: the two far corners of
// a 4×4 grid came out a tenth of the drawing apart. Started from here the grid
// comes out a grid.
//
// The two directions are the leading eigenvectors of the double-centred squared
// distance table, found by power iteration:
//
//   - The start vector is a node's own row of distances — data rather than a
//     seed, and never the vector this matrix annihilates. The two directions
//     start from two different nodes.
//   - The count is [StressPowerIterations] rather than a tolerance, for the
//     reason the sweep count is.
//   - The second direction is kept orthogonal to the first at every step, which
//     is deflation without forming a second matrix.
//   - The table is shifted so that "biggest eigenvalue" means biggest by value
//     rather than by magnitude — see below.
//
// A matrix with a repeated leading eigenvalue has no single answer to "the
// most spread out direction", and this is not the place that decides it: the
// iteration lands wherever its own arithmetic lands, the same way every time,
// which is the property this package needs. What it is not is *canonical* — a
// different implementation of the same method may pick a different pair from
// the same eigenspace — and nothing downstream depends on which.
func (s *Stress) classical(nodes int) {
	if nodes < 3 {
		s.circle(nodes)
		return
	}
	s.gram(nodes)
	// The centred table's biggest eigenvalue by *magnitude* need not be its
	// biggest by value, and classical scaling needs the biggest by value: a
	// negative eigenvalue names a direction the arrangement is not spread along
	// at all, and taking its square root as a scale is arithmetic on a number
	// that is not there. Graph distances are frequently not Euclidean, so this
	// is a case that arrives rather than one that could — the complete
	// bipartite graph K(5,5) centres to a spectrum of −5.5, eight 2s and a 0,
	// and an iteration that takes the biggest magnitude takes the −5.5.
	//
	// So the iteration runs once to measure the spectral radius, and twice more
	// on the table shifted up by it. Every eigenvalue of the shifted table is
	// non-negative, so there the biggest by magnitude *is* the biggest by
	// value, and subtracting the shift back gives the true one. Checking the
	// sign afterwards would not do instead: by then the iteration has already
	// converged to the wrong direction.
	s.probe = growFloats(s.probe, nodes)
	shift := math.Abs(s.power(nodes, 0, s.probe, nil, 0))
	lx := s.power(nodes, 0, s.x, nil, shift)
	ly := s.power(nodes, 1, s.y, s.x, shift)
	if !(lx > 0) {
		// Not one direction the arrangement is spread along: every pair the
		// same distance from every other, which is a graph with no edges in it,
		// and a ring is the honest picture of that.
		s.circle(nodes)
		return
	}
	if !(ly > 0) {
		// One direction and no second one, which is not a failure: a path's
		// distances are realised exactly by points on a line, so the table has
		// rank one and the truthful drawing is that line. Keeping the first
		// direction and flattening the second is what draws it; [Stress.scatter]
		// gives the nodes the width they need to be told apart, and the sweeps
		// take it from there. Falling back to a ring here instead is what made
		// a five-node chain come out bowed by a seventh of its own length.
		ly = 0
	}
	lx, ly = math.Sqrt(lx), math.Sqrt(ly)
	for i := range nodes {
		s.x[i], s.y[i] = lx*s.x[i], ly*s.y[i]
	}
}

// stressScatter is how far [Stress.scatter] moves a node off an exact tie, as a
// fraction of how far apart the arrangement is spread.
const stressScatter = 0.01

// scatter nudges every node a hair along a direction of its own.
//
// Two nodes with the same neighbours have the same distance to everything else
// and to each other, so an arrangement that puts them in one place is a
// stationary one: every sweep computes the same position for both and leaves
// them there. A pair of twins — two leaves off one parent, two members of one
// clique — is then drawn as one dot with two names on it, which is a drawing
// that lies about how many things there are. It is a saddle rather than a
// minimum, so the smallest push is enough; what it needs is a push.
//
// A random start is what the usual implementation relies on to supply one, and
// it is also the only reason that implementation cannot be repeated. A fixed
// spiral does the same work: the golden angle puts consecutive nodes'
// directions as far apart as an angle can be, and the distance is a hundredth
// of the spread, which is below what a reader can see and above what the sweeps
// need to pull on.
func (s *Stress) scatter(nodes int) {
	lox, hix := math.Inf(1), math.Inf(-1)
	loy, hiy := math.Inf(1), math.Inf(-1)
	for i := range nodes {
		lox, hix = math.Min(lox, s.x[i]), math.Max(hix, s.x[i])
		loy, hiy = math.Min(loy, s.y[i]), math.Max(hiy, s.y[i])
	}
	span := math.Max(hix-lox, hiy-loy)
	if !(span > 0) {
		span = 1
	}
	// The golden angle, π(3−√5): the one angle whose multiples never come
	// close to repeating, so no two nodes are pushed the same way.
	const golden = 2.399963229728653
	for i := range nodes {
		a := golden * float64(i)
		s.x[i] += stressScatter * span * math.Cos(a)
		s.y[i] += stressScatter * span * math.Sin(a)
	}
}

// gram double-centres the squared distances in place, which turns a table of
// distances into one of inner products about the arrangement's own middle.
func (s *Stress) gram(nodes int) {
	s.rowMean = growFloats(s.rowMean, nodes)
	n := float64(nodes)
	grand := 0.0
	for i := range nodes {
		row := s.dist[i*nodes : (i+1)*nodes]
		sum := 0.0
		for _, d := range row {
			sum += d * d
		}
		s.rowMean[i] = sum / n
		grand += sum
	}
	grand /= n * n
	for i := range nodes {
		row := s.dist[i*nodes : (i+1)*nodes]
		for j := range nodes {
			// The table is symmetric, so the column mean is the row mean.
			row[j] = -0.5 * (row[j]*row[j] - s.rowMean[i] - s.rowMean[j] + grand)
		}
	}
}

// power refines one direction of the centred table shifted up by shift, kept
// orthogonal to prev, and returns the eigenvalue of the *unshifted* table that
// it settled on. The vector comes back normalised.
//
// A shift big enough to make every eigenvalue non-negative is what turns "the
// biggest by magnitude", which is what this iteration finds, into "the biggest
// by value", which is what [Stress.classical] needs. It costs convergence —
// the gap between the top two eigenvalues shrinks against their size — and
// [StressPowerIterations] is set with that cost in it.
func (s *Stress) power(nodes, from int, v, prev []float64, shift float64) float64 {
	// The start vector is one node's own row of distances: data rather than a
	// seed, and never the vector this matrix annihilates. The second direction
	// starts from a different node, because starting both from the same one and
	// projecting the first out can leave nothing of the second in what remains
	// — K(5,5) is that case, and it came back with an eigenvalue of zero.
	copy(v, s.dist[from*nodes:(from+1)*nodes])
	s.scratch = growFloats(s.scratch, nodes)
	orth := func(v []float64) {
		// The centred table annihilates the vector of ones, so every direction
		// it is worth finding is orthogonal to that one and the iterate is held
		// there. Without this the shift below turns a direction with no spread
		// at all into a competitor — its shifted eigenvalue is the shift — and
		// a path came back with a second direction of zero.
		mean := 0.0
		for i := range nodes {
			mean += v[i]
		}
		mean /= float64(nodes)
		for i := range nodes {
			v[i] -= mean
		}
		if prev == nil {
			return
		}
		dot := 0.0
		for i := range nodes {
			dot += v[i] * prev[i]
		}
		for i := range nodes {
			v[i] -= dot * prev[i]
		}
	}
	norm := func(v []float64) float64 {
		sum := 0.0
		for i := range nodes {
			sum += v[i] * v[i]
		}
		return math.Sqrt(sum)
	}
	orth(v)
	if l := norm(v); l > 0 {
		for i := range nodes {
			v[i] /= l
		}
	} else {
		return 0
	}
	lambda := 0.0
	for range StressPowerIterations {
		for i := range nodes {
			row := s.dist[i*nodes : (i+1)*nodes]
			sum := 0.0
			for j := range nodes {
				sum += row[j] * v[j]
			}
			s.scratch[i] = sum + shift*v[i]
		}
		copy(v, s.scratch)
		orth(v)
		lambda = norm(v)
		if !(lambda > 0) {
			return -shift
		}
		for i := range nodes {
			v[i] /= lambda
		}
	}
	return lambda - shift
}

// sweep is one majorization pass: for each node in turn, the position that
// minimises the majorizing quadratic in that node alone, with the others held
// where they are.
//
// This is block coordinate descent on that quadratic — the Guttman transform
// applied one node at a time and written back before the next node reads it.
// Each block is solved exactly, so the majorizing function never rises; it sits
// above the stress and touches it at the current arrangement, so the stress
// never rises either. That is what makes a fixed sweep count cost quality and
// not correctness.
//
// It is deliberately not the simultaneous form, where every new position is
// computed from the old ones. That form is not a descent at all and it was
// measured rather than argued: two nodes whose target distance is 1, placed
// 1.02 apart, come out 0.98 apart, then 1.02 again, for ever, at constant
// stress. See TestASweepNeverRaisesTheStress.
//
// The order is the order the caller's rows interned the nodes in, which is the
// order every other layout in this package works in ([ADR 0012](../docs/adr/0012-parallel-panels.md)).
func (s *Stress) sweep(nodes int) {
	for i := range nodes {
		row := s.dist[i*nodes : (i+1)*nodes]
		var sx, sy, sw float64
		for j := range nodes {
			if j == i {
				continue
			}
			d := row[j]
			if d <= 0 {
				continue
			}
			// The usual weight: an edge one step long matters as much as the
			// square of a long one does not, which is what stops a far pair
			// from flattening the neighbourhood of a near one.
			w := 1 / (d * d)
			dx, dy := s.x[i]-s.x[j], s.y[i]-s.y[j]
			if norm := math.Hypot(dx, dy); norm > 0 {
				dx, dy = d*dx/norm, d*dy/norm
			} else {
				// Two nodes exactly on top of each other have no direction to
				// be pushed apart in. Leaving the term at the other node's own
				// position is the choice that does not invent one; [Stress.scatter]
				// is what stops the case arising in the first place.
				dx, dy = 0, 0
			}
			sx += w * (s.x[j] + dx)
			sy += w * (s.y[j] + dy)
			sw += w
		}
		if sw == 0 {
			continue
		}
		s.x[i], s.y[i] = sx/sw, sy/sw
	}
}

// emit scales the arrangement into the unit square.
//
// The scale is the same on both axes, because what the layout minimised is a
// distance: stretching one axis to fill the square would undo what every sweep
// was for. The shorter side is centred, so the drawing sits in the middle of
// the unit square rather than in a corner of it.
func (s *Stress) emit(nodes int) {
	lox, hix := math.Inf(1), math.Inf(-1)
	loy, hiy := math.Inf(1), math.Inf(-1)
	for i := range nodes {
		lox, hix = math.Min(lox, s.x[i]), math.Max(hix, s.x[i])
		loy, hiy = math.Min(loy, s.y[i]), math.Max(hiy, s.y[i])
	}
	span := math.Max(hix-lox, hiy-loy)
	if !(span > 0) {
		// One node, or every node in one place: the middle is the whole of the
		// truthful answer.
		for range nodes {
			s.Nodes = append(s.Nodes, StressPoint{X: 0.5, Y: 0.5})
		}
		return
	}
	// The offsets centre the shorter side: half of what the square has spare.
	offx := (span - (hix - lox)) / 2
	offy := (span - (hiy - loy)) / 2
	for i := range nodes {
		s.Nodes = append(s.Nodes, StressPoint{
			X: (s.x[i] - lox + offx) / span,
			Y: (s.y[i] - loy + offy) / span,
		})
	}
}

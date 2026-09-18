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
// Fifty is where the arrangement stops visibly improving on the graphs this was
// tested against — a grid, a ring, a tree, two clusters joined by one edge, and
// a graph with no edges at all. Majorization is monotone, so a sweep never
// makes the drawing worse; what a bound costs is the last fraction of a percent
// of a quantity no reader can see.
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
// It is the layout a node-link diagram is drawn with, and it is not a force
// simulation. A simulation integrates a system of forces until it settles,
// which is where [ADR 0039](../../docs/adr/0039-relational-layouts.md) refused
// it: the stopping point is a tolerance, so the picture depends on where the
// arithmetic happened to land. What this runs is *majorization* — at each sweep
// the stress function is replaced by a quadratic that touches it from above and
// is minimised in closed form, which is one arithmetic expression per node and
// no search at all. See docs/adr/0077-a-node-link-layout.md.
//
// Three properties make it a pure function of its input:
//
//   - The starting arrangement is a circle in index order, which is the order
//     the caller's rows named the nodes in.
//   - Each sweep computes every new position from the previous sweep's
//     positions only — Jacobi rather than Gauss–Seidel — so the result does not
//     depend on the order the nodes are visited in, only on the order they were
//     interned in.
//   - The sweep count is [StressSweeps], and the distances are whole numbers of
//     edges, so nothing here reads a tolerance or a clock.
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
	nx, ny                  []float64
	rowMean, scratch        []float64
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
	s.nx, s.ny = growFloats(s.nx, nodes), growFloats(s.ny, nodes)
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
//   - The start vector is the first node's own distances — data rather than a
//     seed, and never the vector this matrix annihilates.
//   - The count is [StressPowerIterations] rather than a tolerance, for the
//     reason the sweep count is.
//   - The second direction is kept orthogonal to the first at every step, which
//     is deflation without forming a second matrix.
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
	lx := s.power(nodes, s.x, nil)
	ly := s.power(nodes, s.y, s.x)
	if !(lx > 0) || !(ly > 0) {
		// No two directions to be spread along: every pair is the same
		// distance from every other, which is a graph with no edges in it.
		s.circle(nodes)
		return
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

// power refines one direction of the centred table, kept orthogonal to prev,
// and returns the eigenvalue it settled on. The vector comes back normalised.
func (s *Stress) power(nodes int, v, prev []float64) float64 {
	copy(v, s.dist[:nodes]) // the first node's row: its distances to everything
	s.scratch = growFloats(s.scratch, nodes)
	orth := func(v []float64) {
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
			s.scratch[i] = sum
		}
		copy(v, s.scratch)
		orth(v)
		lambda = norm(v)
		if !(lambda > 0) {
			return 0
		}
		for i := range nodes {
			v[i] /= lambda
		}
	}
	return lambda
}

// sweep is one majorization pass: the Guttman transform, which is the minimum
// of the quadratic that touches the stress function at the current positions.
//
// Every new position is computed from the old ones and written to a second
// buffer, so the pass is Jacobi rather than Gauss–Seidel and the arrangement
// does not depend on which node is visited first.
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
				// position is the choice that does not invent one; the next
				// sweep separates them, because their neighbours differ.
				dx, dy = 0, 0
			}
			sx += w * (s.x[j] + dx)
			sy += w * (s.y[j] + dy)
			sw += w
		}
		if sw == 0 {
			s.nx[i], s.ny[i] = s.x[i], s.y[i]
			continue
		}
		s.nx[i], s.ny[i] = sx/sw, sy/sw
	}
	s.x, s.nx = s.nx, s.x
	s.y, s.ny = s.ny, s.y
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

package stat

import (
	"math"
	"testing"
)

// The two properties [Stress] rests on that cannot be seen from outside the
// package: the directions it starts from are directions the arrangement is
// really spread along, and a sweep never makes the drawing worse. Both were
// wrong once and both were found by measuring rather than by reading.
// See docs/adr/0077-a-node-link-layout.md.

// graphs the properties are checked over, each with a shape of its own.
func stressCases() []struct {
	name     string
	from, to []int
	nodes    int
} {
	lattice := func(r, c int) (from, to []int) {
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
		return from, to
	}
	bipartite := func(a, b int) (from, to []int) {
		for i := range a {
			for j := range b {
				from, to = append(from, i), append(to, a+j)
			}
		}
		return from, to
	}
	gridFrom, gridTo := lattice(4, 5)
	kFrom, kTo := bipartite(5, 5)
	return []struct {
		name     string
		from, to []int
		nodes    int
	}{
		{"a 4x5 lattice", gridFrom, gridTo, 20},
		// K(5,5)'s distances are not Euclidean: the centred table's spectrum is
		// −5.5, eight 2s and a 0. An iteration that takes the biggest
		// eigenvalue by magnitude takes the −5.5 and then takes its square root
		// as a scale, which is arithmetic on a direction that is not there.
		{"the complete bipartite graph K(5,5)", kFrom, kTo, 10},
		{"a path", []int{0, 1, 2, 3, 4, 5}, []int{1, 2, 3, 4, 5, 6}, 7},
		{"two triangles on one edge", []int{0, 1, 2, 3, 4, 5, 2}, []int{1, 2, 0, 4, 5, 3, 3}, 6},
		{"a star", []int{0, 0, 0, 0, 0}, []int{1, 2, 3, 4, 5}, 6},
	}
}

// TestTheStartDirectionsAreReportedHonestly checks the invariant the sign
// check downstream depends on: the eigenvalue the iteration reports for a
// direction is the eigenvalue that direction really has.
//
// It was not. The iteration finds the biggest eigenvalue by *magnitude* and
// reported its magnitude, so a negative one came back positive and its square
// root was then used as a scale — arithmetic on a direction the arrangement is
// not spread along at all. Checking the sign afterwards cannot recover it,
// because by then the iteration has converged to the wrong direction.
func TestTheStartDirectionsAreReportedHonestly(t *testing.T) {
	for _, tc := range stressCases() {
		t.Run(tc.name, func(t *testing.T) {
			var s Stress
			s.adjacency(tc.from, tc.to, tc.nodes)
			s.distances(tc.nodes)
			s.x, s.y = growFloats(s.x, tc.nodes), growFloats(s.y, tc.nodes)
			s.gram(tc.nodes)

			s.probe = growFloats(s.probe, tc.nodes)
			shift := math.Abs(s.power(tc.nodes, 0, s.probe, nil, 0))
			got := [2]float64{
				s.power(tc.nodes, 0, s.x, nil, shift),
				s.power(tc.nodes, 1, s.y, s.x, shift),
			}
			for i, v := range [2][]float64{s.x, s.y} {
				want := rayleigh(s.dist, v, tc.nodes)
				if tol := 1e-6 * math.Max(1, math.Abs(want)); math.Abs(got[i]-want) > tol {
					t.Errorf("direction %d was reported at %.9f and is at %.9f", i, got[i], want)
				}
				if got[i] < -1e-9 {
					t.Errorf("direction %d has eigenvalue %.9f, and a negative one is not a direction to be spread along", i, got[i])
				}
			}
			// The first direction is the one the drawing is laid along, so a
			// graph with any structure at all has to have one.
			if got[0] <= 0 {
				t.Errorf("no direction to lay the drawing along: %.9f", got[0])
			}
		})
	}
}

// rayleigh is the eigenvalue a direction really has, whatever an iteration says
// it found.
func rayleigh(m, v []float64, n int) float64 {
	num, den := 0.0, 0.0
	for i := range n {
		row := m[i*n : (i+1)*n]
		mv := 0.0
		for j := range n {
			mv += row[j] * v[j]
		}
		num += v[i] * mv
		den += v[i] * v[i]
	}
	return num / den
}

// TestASweepNeverRaisesTheStress is the property the whole bound rests on: a
// sweep solves each node's own block of the majorizing quadratic exactly, so
// stopping after a fixed count costs quality and cannot cost correctness.
//
// The simultaneous form of the same update — every new position computed from
// the old ones — does not have it, which is why this is a test rather than a
// remark: two nodes whose target distance is 1, placed 1.02 apart, come out
// 0.98 apart, then 1.02 again, for ever, at constant stress.
func TestASweepNeverRaisesTheStress(t *testing.T) {
	for _, tc := range stressCases() {
		t.Run(tc.name, func(t *testing.T) {
			var s Stress
			s.adjacency(tc.from, tc.to, tc.nodes)
			s.distances(tc.nodes)
			s.x, s.y = growFloats(s.x, tc.nodes), growFloats(s.y, tc.nodes)
			s.classical(tc.nodes)
			s.scatter(tc.nodes)
			s.distances(tc.nodes)

			was := stressAt(&s, tc.nodes)
			for k := range StressSweeps {
				s.sweep(tc.nodes)
				now := stressAt(&s, tc.nodes)
				if now > was+1e-12 {
					t.Fatalf("sweep %d raised the stress from %.12f to %.12f", k, was, now)
				}
				was = now
			}
		})
	}
}

// TestTheSimultaneousSweepIsWhyThisOneIsNot pins the measurement the sweep's
// shape was chosen from, so that a future "simplification" back to it fails
// here rather than in a picture.
func TestTheSimultaneousSweepIsWhyThisOneIsNot(t *testing.T) {
	var s Stress
	s.adjacency([]int{0}, []int{1}, 2)
	s.distances(2)
	s.x, s.y = growFloats(s.x, 2), growFloats(s.y, 2)
	s.x[0], s.y[0] = 0, 0
	s.x[1], s.y[1] = 1.02, 0

	for range 8 {
		s.sweep(2)
	}
	if got := math.Hypot(s.x[1]-s.x[0], s.y[1]-s.y[0]); math.Abs(got-1) > 1e-9 {
		t.Errorf("two nodes whose target distance is 1 came to rest %.6f apart", got)
	}
	if got := stressAt(&s, 2); got > 1e-18 {
		t.Errorf("the stress settled at %.12f rather than at zero", got)
	}
}

// stressAt is the number the layout minimises: how far every drawn distance is
// from the graph distance it stands for, weighted the way the sweep weights it.
func stressAt(s *Stress, n int) float64 {
	total := 0.0
	for i := range n {
		row := s.dist[i*n : (i+1)*n]
		for j := i + 1; j < n; j++ {
			d := row[j]
			if d <= 0 {
				continue
			}
			got := math.Hypot(s.x[i]-s.x[j], s.y[i]-s.y[j])
			total += (got - d) * (got - d) / (d * d)
		}
	}
	return total
}

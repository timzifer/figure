//go:build !race

package stat_test

// The allocation checks, apart from the rest of the package's tests, because
// the race detector allocates on figure's behalf and would fail them for
// nothing figure did. See AGENTS.md.

import (
	"math"
	"testing"

	"github.com/timzifer/figure/stat"
)

// Reset keeps its buffers, which is why it is a struct at all.
func TestTracingAgainDoesNotAllocate(t *testing.T) {
	const n = 33
	xs := make([]float64, n)
	ys := make([]float64, n)
	for i := range n {
		xs[i], ys[i] = float64(i), float64(i)
	}
	z := make([]float64, n*n)
	for j := range n {
		for i := range n {
			z[j*n+i] = math.Sin(float64(i)/3) * math.Cos(float64(j)/3)
		}
	}
	levels := stat.Levels(-1, 1, 6)

	var c stat.Contour
	c.Reset(xs, ys, z, levels)
	if got := testing.AllocsPerRun(10, func() { c.Reset(xs, ys, z, levels) }); got != 0 {
		t.Errorf("tracing again allocated %.0f times, want none", got)
	}
}

// The type is a struct with a Reset so that a chart redrawn every frame
// resolves into the same memory.
func TestResolvingALatticeAgainDoesNotAllocate(t *testing.T) {
	const n = 24
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	vs := make([]float64, 0, n*n)
	for j := range n {
		for i := range n {
			xs, ys = append(xs, float64(i)), append(ys, float64(j))
			vs = append(vs, float64(i*j))
		}
	}

	var l stat.Lattice
	if f := l.Reset(xs, ys, vs); f != stat.LatticeOK {
		t.Fatal(f)
	}
	if got := testing.AllocsPerRun(10, func() { l.Reset(xs, ys, vs) }); got != 0 {
		t.Errorf("resolving again allocated %.0f times, want none", got)
	}
}

// The Append form writes into a caller's slice, which is how a chart redrawn
// every frame keeps its allocations flat.
func TestAppendLevelsDoesNotAllocateIntoRoom(t *testing.T) {
	dst := make([]float64, 0, 16)
	dst = stat.AppendLevels(dst, 0, 10, 5)

	if got := testing.AllocsPerRun(10, func() { dst = stat.AppendLevels(dst[:0], 0, 10, 5) }); got != 0 {
		t.Errorf("AppendLevels allocated %.0f times into a slice with room", got)
	}
}

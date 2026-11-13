//go:build !race

// The race detector allocates on figure's behalf, so a test that counts
// allocations is behind this tag — the core's alloc_test.go has the same one.

package stilus_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure"
	stilusbackend "github.com/timzifer/figure/backend/stilus"
	"github.com/timzifer/figure/geom"
)

// A frame redrawn on a surface allocates what a frame costs, not what its rows
// cost: the surface keeps its backend between opens — the canvas with its
// stroke and coverage buffers, the glyph cache, the pixel buffer — so a
// hundred times the rows allocates exactly what a hundredth does.
//
// The slack is the gate's. It was 20 once, and a coverage run broke it at 25:
// every Render opened a new canvas then, so a frame refilled the rasterizer's
// buffers and a larger one refilled more of them. Keeping the canvas made the
// two counts equal, with or without a collection between frames.
func TestARedrawDoesNotAllocatePerPoint(t *testing.T) {
	allocs := func(n int) float64 {
		xs, ys := make([]float64, n), make([]float64, n)
		for i := range xs {
			xs[i], ys[i] = float64(i), math.Sin(float64(i)/50)
		}
		p := figure.New(figure.Size(400, 300))
		p.Add(geom.Line(figure.Float64Columns(map[string][]float64{"x": xs, "y": ys}), geom.X("x"), geom.Y("y")))
		p.Add(geom.Scatter(figure.Float64Columns(map[string][]float64{"x": xs, "y": ys}), geom.X("x"), geom.Y("y")))
		s := stilusbackend.NewSurface()
		_ = p.Render(s) // warm the pools and the glyph cache
		return testing.AllocsPerRun(5, func() { _ = p.Render(s) })
	}
	small, large := allocs(200), allocs(20000)
	if large > small+8 {
		t.Errorf("a frame over 20000 rows allocates %v times, over 200 rows %v", large, small)
	}
}

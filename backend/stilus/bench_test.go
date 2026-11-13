package stilus_test

import (
	"math"
	"runtime"
	"testing"

	"github.com/timzifer/figure"
	stilusbackend "github.com/timzifer/figure/backend/stilus"
	"github.com/timzifer/figure/geom"
)

// onOnePGate pins a gated benchmark to one processor, for the reason the
// core's alloc_test.go gives: a goroutine that migrates leaves its pooled
// scratch in the old processor's private slot, and the frame that refills it
// is charged allocations nobody wrote.
func onOnePGate(b *testing.B) {
	prev := runtime.GOMAXPROCS(1)
	b.Cleanup(func() { runtime.GOMAXPROCS(prev) })
}

// rasterChart is a line and a scatter over the same rows, with a title and
// axes: everything a frame draws through the backend — strokes, markers,
// text, clips — and enough rows that the scatter's markers are the bulk.
func rasterChart(rows int) *figure.Plot {
	xs, ys := make([]float64, rows), make([]float64, rows)
	for i := range xs {
		xs[i] = float64(i)
		ys[i] = math.Sin(float64(i)/50) * math.Exp(-float64(i)/float64(rows))
	}
	src := figure.Float64Columns(map[string][]float64{"x": xs, "y": ys})
	p := figure.New(figure.Size(800, 400), figure.Title("Signal"))
	p.Add(geom.Line(src, geom.X("x"), geom.Y("y")))
	p.Add(geom.Scatter(src, geom.X("x"), geom.Y("y")))
	return p
}

// benchmarkStilusFrame redraws a chart into one surface, which is what a window
// does: the glyph cache and the backend's buffers are warm after the first
// frame, and what is measured is a frame's cost in the steady state.
func benchmarkStilusFrame(b *testing.B, rows int) {
	onOnePGate(b)
	p := rasterChart(rows)
	s := stilusbackend.NewSurface()
	if err := p.Render(s); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.Render(s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStilusFrame1k(b *testing.B)   { benchmarkStilusFrame(b, 1_000) }
func BenchmarkStilusFrame100k(b *testing.B) { benchmarkStilusFrame(b, 100_000) }

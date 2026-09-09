//go:build !race

package figure_test

import (
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

func TestDiagnosticsDoNotAllocatePerRow(t *testing.T) {
	for _, labels := range []bool{false, true} {
		chart := func(n int) *figure.Plot {
			vs := make([]float64, n)
			names := make([]string, n)
			for i := range vs {
				vs[i] = float64(i%997) / 100
				names[i] = "label"
			}
			src := data.NewTable().Float64("v", vs).String("label", names)
			p := figure.New(figure.Size(800, 500))
			p.X(scale.Linear(scale.Domain(-4, 10)))
			p.Y(scale.Linear(scale.Domain(0, 10)))
			if labels {
				p.Add(geom.Text(src, geom.X("v"), geom.Y("v"), geom.TextBy("label"), geom.AvoidOverlap(true)))
			} else {
				p.Add(geom.QQ(src, geom.X("v")))
			}
			return p
		}
		small, large := allocsPerFrame(t, chart(1000)), allocsPerFrame(t, chart(100000))
		t.Logf("labels=%v: %.0f / %.0f allocations", labels, small, large)
		if large > small+8 {
			t.Fatalf("labels=%v: allocations grew from %.0f to %.0f", labels, small, large)
		}
	}
}

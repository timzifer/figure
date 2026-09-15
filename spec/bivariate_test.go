package spec_test

import (
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

func TestABivariateLayerSurvivesTheRoundTrip(t *testing.T) {
	src := data.NewTable().
		Float64("x", []float64{0, 1, 0, 1}).
		Float64("y", []float64{0, 0, 1, 1}).
		Float64("mean", []float64{10, 40, 10, 40}).
		Float64("sd", []float64{0, 0, 10, 10})
	literal := [][]ir.Color{{palette.Blue, palette.SkyBlue}, {palette.Orange, palette.Yellow}}
	for name, cs := range map[string]scale.ColorScale{
		"vsup":           scale.VSUP(palette.Viridis, 4, 2),
		"named matrix":   scale.BivariateMatrix(palette.BivariateBlueRed),
		"literal matrix": scale.BivariateMatrix(literal),
	} {
		t.Run(name, func(t *testing.T) {
			c := spec.Chart{
				Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
				X: scale.Linear(), Y: scale.Linear(),
				Layers: []geom.Geom{geom.Rect(src, geom.X("x"), geom.Y("y"),
					geom.ColorBy("mean", cs), geom.UncertaintyBy("sd"))},
			}
			back := roundTrip(t, c)
			d, _ := geom.Describe(back.Layers[0])
			if d.UncertaintyCol != "sd" {
				t.Errorf("the second column read back as %q", d.UncertaintyCol)
			}
			if _, ok := scale.Bivariate(d.ColorScale); !ok {
				t.Errorf("the colour scale read back as %T, which reads one number", d.ColorScale)
			}
			want, got := draw(t, c), draw(t, back)
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				t.Errorf("the %s layer did not survive the round trip", name)
			}
		})
	}
}

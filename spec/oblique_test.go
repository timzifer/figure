package spec_test

import (
	"strings"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

func TestAnObliqueChartSurvivesTheRoundTrip(t *testing.T) {
	src := table()
	for name, tc := range map[string]struct {
		cd    coord.Coord
		layer geom.Geom
	}{
		"bars":     {coord.Oblique(), geom.Bar(src, geom.X("x"), geom.Y("y"), geom.Extrude(true))},
		"deep":     {coord.Oblique(coord.Depth(0.12), coord.DepthAngle(-2.4)), geom.Bar(src, geom.X("x"), geom.Y("y"), geom.Extrude(true))},
		"straight": {coord.Oblique(coord.DepthAngle(0)), geom.Rect(src, geom.X("x"), geom.Y("y"), geom.Extrude(true))},
		"flat":     {coord.Oblique(), geom.Bar(src, geom.X("x"), geom.Y("y"))},
	} {
		t.Run(name, func(t *testing.T) {
			c := spec.Chart{
				Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
				X: scale.Linear(), Y: scale.Linear(),
				Coord:  tc.cd,
				Layers: []geom.Geom{tc.layer},
			}
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				t.Errorf("the %s oblique chart did not survive the round trip", name)
			}
		})
	}
}

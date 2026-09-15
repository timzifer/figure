package spec_test

import (
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

func TestASurvivalLayerSurvivesTheRoundTrip(t *testing.T) {
	src := data.NewTable().
		Float64("weeks", []float64{6, 6, 7, 10, 13, 1, 2, 3, 4, 8}).
		Float64("relapsed", []float64{1, 0, 1, 0, 1, 1, 1, 1, 0, 1}).
		String("arm", []string{"a", "a", "a", "a", "a", "b", "b", "b", "b", "b"})
	for name, layer := range map[string]geom.Geom{
		"plain":   geom.Survival(src, geom.X("weeks"), geom.Event("relapsed")),
		"grouped": geom.Survival(src, geom.X("weeks"), geom.Event("relapsed"), geom.GroupBy("arm"), geom.Confidence(0.9), geom.CensorMarks(true)),
		"noevent": geom.Survival(src, geom.X("weeks")),
	} {
		t.Run(name, func(t *testing.T) {
			c := spec.Chart{
				Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
				X: scale.Linear(), Y: scale.Linear(),
				Layers: []geom.Geom{layer},
			}
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				t.Errorf("the %s survival layer did not survive the round trip", name)
			}
		})
	}
}

func TestADeclinedGuideSurvivesTheRoundTrip(t *testing.T) {
	src := table()
	c := spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Layers: []geom.Geom{geom.Scatter(src, geom.X("x"), geom.Y("y"),
			geom.ColorBy("z", scale.Sequential(palette.Viridis)), geom.Guide(false))},
	}
	back := roundTrip(t, c)
	d, ok := geom.Describe(back.Layers[0])
	if !ok || !d.HideGuide {
		t.Fatalf("the layer read back with its guide: %+v", d)
	}
	want, got := draw(t, c), draw(t, back)
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Error("a chart with a declined guide did not survive the round trip")
	}
}

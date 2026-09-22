package spec_test

import (
	"strings"
	"testing"
	"time"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

// Axis breaks and folds through the document. The words are `"cuts"` and
// `"folds"`, because `"breaks"` is already a threshold colour scale's class
// boundaries. See docs/adr/0083-an-axis-break-is-marked-or-not-drawn.md.

func brokenChart() spec.Chart {
	src := data.NewTable().
		String("site", []string{"a", "b", "c"}).
		Float64("load", []float64{4, 96, 6})
	return spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X: scale.Ordinal(),
		Y: scale.Linear(scale.Domain(0, 100), scale.Break(10, 88),
			scale.Fold(scale.Interval{Lo: 92, Hi: 94})),
		Layers: []geom.Geom{geom.Bar(src, geom.X("site"), geom.Y("load"))},
	}
}

func foldedTimeChart() spec.Chart {
	t0 := time.Date(2026, 3, 2, 6, 0, 0, 0, time.UTC)
	h := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Hour) }
	src := data.NewTable().
		Time("t", []time.Time{h(0), h(1), h(4), h(5), h(8)}).
		Float64("v", []float64{1, 2, 3, 2, 1})
	return spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X:      scale.Time(scale.TimeFold(scale.TimeSpan{From: h(1), To: h(4)}), scale.Origin(t0)),
		Y:      scale.Linear(scale.Nice()),
		Layers: []geom.Geom{geom.Line(src, geom.X("t"), geom.Y("v"))},
	}
}

func TestABrokenAxisSurvivesTheRoundTrip(t *testing.T) {
	for name, c := range map[string]spec.Chart{"linear": brokenChart(), "time": foldedTimeChart()} {
		want, got := draw(t, c), draw(t, roundTrip(t, c))
		if strings.Join(want, "\n") != strings.Join(got, "\n") {
			s, _ := spec.Of(c)
			b, _ := s.Marshal()
			t.Errorf("%s: a broken axis did not survive the round trip\n%s", name, b)
		}
	}
}

func TestABrokenAxisIsWrittenAsCutsAndFolds(t *testing.T) {
	s, err := spec.Of(brokenChart())
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	b, err := s.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, w := range []string{`"cuts"`, `"folds"`} {
		if !strings.Contains(string(b), w) {
			t.Errorf("the document does not carry %s:\n%s", w, b)
		}
	}
	if strings.Contains(string(b), `"breaks"`) {
		t.Errorf("an axis break was written as colour classes:\n%s", b)
	}
}

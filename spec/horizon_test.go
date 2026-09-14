package spec_test

import (
	"math"
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

func sensorTrace(n int) *data.Table {
	ts := make([]float64, n)
	kw := make([]float64, n)
	for i := range n {
		x := float64(i) / float64(n-1)
		ts[i] = x * 60
		kw[i] = 40*math.Sin(6*math.Pi*x) + 18*math.Sin(2*math.Pi*x)
	}
	return data.NewTable().Float64("t", ts).Float64("kw", kw)
}

// A horizon survives the round trip, folded the same way. Both spellings of the
// fold have to travel: a count and a pinned height are different charts over
// the same rows.
func TestAHorizonSurvivesTheRoundTrip(t *testing.T) {
	src := sensorTrace(120)
	for _, tc := range []struct {
		name  string
		layer geom.Geom
	}{
		{"bands", geom.Horizon(src, geom.X("t"), geom.Y("kw"), geom.Bands(4))},
		{"height", geom.Horizon(src, geom.X("t"), geom.Y("kw"), geom.BandHeight(12.5))},
		{"origin", geom.Horizon(src, geom.X("t"), geom.Y("kw"), geom.Bands(3), geom.Baseline(10))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := spec.Chart{
				Width: 600, Height: 180, DPR: 1, Theme: theme.Light,
				X: scale.Linear(), Y: scale.Linear(), Layers: []geom.Geom{tc.layer},
			}
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the horizon did not survive the round trip\n%s", b)
			}
		})
	}
}

// A horizon writes bands and not bins. A histogram's bins divide the rows and a
// fold's bands divide one row's value, so a document that said "bins" here
// would claim the mark binned.
func TestAHorizonWritesBandsAndNotBins(t *testing.T) {
	s, err := spec.Of(spec.Chart{
		Width: 600, Height: 180, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Layers: []geom.Geom{
			geom.Horizon(sensorTrace(60), geom.X("t"), geom.Y("kw"), geom.BandHeight(25)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	m := s.Layer[0].Mark
	if m.Type != "horizon" {
		t.Errorf("the mark is %q, want horizon", m.Type)
	}
	if m.BandHeight != 25 {
		t.Errorf("the horizon wrote a band height of %v, want 25", m.BandHeight)
	}
	if m.Bins != 0 || m.Bandwidth != 0 || len(m.Levels) != 0 {
		t.Errorf("the horizon carries a histogram's bins, a density's bandwidth or a contour's levels: %+v", m)
	}
}

// The pinned height is the one that has to travel intact, because it is the
// one that means something outside the document: a band of 25 kW is 25 kW in
// every chart that carries it, and a band count is not.
func TestAPinnedBandHeightOutranksACount(t *testing.T) {
	s, err := spec.Of(spec.Chart{
		Width: 600, Height: 180, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Layers: []geom.Geom{
			geom.Horizon(sensorTrace(60), geom.X("t"), geom.Y("kw"),
				geom.Bands(9), geom.BandHeight(25)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if m := s.Layer[0].Mark; m.BandHeight != 25 || m.Bands != 9 {
		t.Errorf("the document lost half of the fold: bands %d, height %v", m.Bands, m.BandHeight)
	}

	c, err := s.Chart()
	if err != nil {
		t.Fatal(err)
	}
	d, ok := geom.Describe(c.Layers[0])
	if !ok {
		t.Fatal("the rebuilt horizon does not describe itself")
	}
	if d.BandHeight != 25 || d.Bands != 9 {
		t.Errorf("the rebuilt horizon folds differently: bands %d, height %v", d.Bands, d.BandHeight)
	}
}

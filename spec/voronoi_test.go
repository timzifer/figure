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

// A nearest-neighbour partition through the document. It reads the two
// positional channels every other mark over a pair of axes reads and places
// nothing of its own, so what has to survive here is the mark's name, the
// colour column and the two inks that decide whether a cell is outlined.

func gauges() data.Source {
	return data.NewTable().
		Float64("lon", []float64{1, 9, 1, 9}).
		Float64("lat", []float64{1, 1, 9, 9}).
		Float64("rain", []float64{12, 30, 12, 48})
}

func cellChartSpec(layer geom.Geom) spec.Chart {
	return spec.Chart{
		Width: 480, Height: 320, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Layers: []geom.Geom{layer},
	}
}

func TestAVoronoiChartSurvivesTheRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		layer geom.Geom
	}{
		{"plain", geom.Voronoi(gauges(), geom.X("lon"), geom.Y("lat"))},
		{"outlined", geom.Voronoi(gauges(), geom.X("lon"), geom.Y("lat"),
			geom.Fill(ir.RGB(230, 230, 230)), geom.Color(ir.RGB(30, 30, 30)), geom.Width(0.5))},
		{"coloured", geom.Voronoi(gauges(), geom.X("lon"), geom.Y("lat"),
			geom.ColorBy("rain", scale.Sequential(palette.Viridis)))},
		{"classified", geom.Voronoi(gauges(), geom.X("lon"), geom.Y("lat"),
			geom.ColorBy("rain", scale.Qualitative(palette.OkabeIto)))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := cellChartSpec(tc.layer)
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the voronoi chart did not survive the round trip\n%s", b)
			}
		})
	}
}

// A hand-written document reads. The mark's name is figure's own — Vega-Lite
// has nothing to decode this into — so the vocabulary is the only thing
// standing between the document and the chart.
func TestAHandWrittenVoronoiSpecReads(t *testing.T) {
	doc := `{
	  "width": 480, "height": 320,
	  "data": {"values": [
	    {"lon": 1, "lat": 1, "rain": 12},
	    {"lon": 9, "lat": 9, "rain": 48}
	  ]},
	  "layer": [
	    {"mark": {"type": "voronoi"},
	     "encoding": {
	       "x": {"field": "lon", "type": "quantitative"},
	       "y": {"field": "lat", "type": "quantitative"},
	       "color": {"field": "rain", "scale": {"scheme": "viridis"}}
	     }}
	  ]
	}`
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Chart()
	if err != nil {
		t.Fatalf("a hand-written voronoi chart was refused: %v", err)
	}
	d, ok := geom.Describe(c.Layers[0])
	if !ok || d.Mark != geom.MarkVoronoi {
		t.Fatalf("the layer read back as %q", d.Mark)
	}
	if d.X != "lon" || d.Y != "lat" {
		t.Errorf("the layer's positions read back as %q and %q", d.X, d.Y)
	}
	if d.ColorCol != "rain" {
		t.Errorf("the colour column read back as %q", d.ColorCol)
	}
	if d.Source == nil {
		t.Error("the layer read back with no data source")
	}
}

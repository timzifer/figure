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

// A parallel-sets diagram through the document. It reads the same list of
// columns a parallel-coordinates layer does and draws in the unit square
// instead of a coord of its own, so what has to survive here is the list and
// the two numbers the flow layout takes.

func voyage() data.Source {
	return data.NewTable().
		String("class", []string{"first", "first", "steerage", "steerage"}).
		String("sex", []string{"male", "female", "male", "female"}).
		String("outcome", []string{"died", "lived", "lived", "lived"})
}

func setsChartSpec(layer geom.Geom) spec.Chart {
	return spec.Chart{
		Width: 480, Height: 320, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Layers: []geom.Geom{layer},
	}
}

func TestAParallelSetsChartSurvivesTheRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		layer geom.Geom
	}{
		{"plain", geom.ParallelSets(voyage(), geom.Dims("class", "sex", "outcome"))},
		{"shaped", geom.ParallelSets(voyage(), geom.Dims("class", "sex", "outcome"),
			geom.Padding(0.02), geom.Thickness(0.05), geom.Opacity(0.6))},
		{"coloured", geom.ParallelSets(voyage(), geom.Dims("class", "sex"),
			geom.ColorBy("outcome", scale.Qualitative(palette.OkabeIto)))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := setsChartSpec(tc.layer)
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the parallel-sets chart did not survive the round trip\n%s", b)
			}
		})
	}
}

// A hand-written document reads. The mark's name is figure's own — Vega-Lite
// has nothing to decode this into — so the vocabulary is the only thing
// standing between the document and the chart.
func TestAHandWrittenParallelSetsSpecReads(t *testing.T) {
	doc := `{
	  "width": 480, "height": 320,
	  "data": {"values": [
	    {"class": "first", "sex": "male"},
	    {"class": "steerage", "sex": "female"}
	  ]},
	  "layer": [
	    {"mark": {"type": "parallel-sets", "padding": 0.02},
	     "encoding": {"dims": [{"field": "class"}, {"field": "sex"}]}}
	  ]
	}`
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Chart()
	if err != nil {
		t.Fatalf("a hand-written parallel-sets chart was refused: %v", err)
	}
	d, ok := geom.Describe(c.Layers[0])
	if !ok || d.Mark != geom.MarkParallelSets {
		t.Fatalf("the layer read back as %q", d.Mark)
	}
	if len(d.Dims) != 2 || d.Dims[1] != "sex" {
		t.Errorf("the layer's columns read back as %v", d.Dims)
	}
	if d.Padding != 0.02 {
		t.Errorf("the gap between the boxes read back as %v", d.Padding)
	}
	if d.Source == nil {
		t.Error("the layer read back with no data source")
	}
}

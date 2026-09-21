package spec_test

import (
	"strings"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

// A parallel-coordinates chart through the document. What is new here is that
// the *coord* carries a list — its axes are its configuration — and that the
// layer's columns are matched to those axes in order, so a document that lost
// either would read back as a different chart or as none.

func cars() data.Source {
	return data.NewTable().
		Float64("mpg", []float64{30, 20, 10}).
		Float64("power", []float64{60, 120, 180}).
		Float64("weight", []float64{900, 1200, 1500}).
		String("origin", []string{"eu", "us", "us"})
}

func parallelChartSpec(layer geom.Geom) spec.Chart {
	return spec.Chart{
		Width: 480, Height: 320, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Coord: coord.Parallel(
			coord.Dim("mpg", scale.Linear()),
			coord.Dim("power", scale.Linear()),
			coord.Dim("weight", scale.Linear()),
		),
		Layers: []geom.Geom{layer},
	}
}

func TestAParallelChartSurvivesTheRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		layer geom.Geom
	}{
		{"plain", geom.Parallel(cars(), geom.Dims("mpg", "power", "weight"))},
		{"coloured", geom.Parallel(cars(), geom.Dims("mpg", "power", "weight"),
			geom.ColorBy("origin", scale.Qualitative(palette.OkabeIto)))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := parallelChartSpec(tc.layer)
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the parallel chart did not survive the round trip\n%s", b)
			}
		})
	}
}

// The coord writes its axes down, because they are what configures it: a
// document that named the type and nothing else would read back as a panel
// with no axes at all.
func TestAParallelCoordWritesItsDimensions(t *testing.T) {
	s, err := spec.Of(parallelChartSpec(geom.Parallel(cars(), geom.Dims("mpg", "power", "weight"))))
	if err != nil {
		t.Fatal(err)
	}
	if s.Coord == nil || s.Coord.Type != "parallel" {
		t.Fatalf("the document's coord is %+v", s.Coord)
	}
	if len(s.Coord.Dims) != 3 || s.Coord.Dims[0].Name != "mpg" {
		t.Fatalf("the coord wrote %+v, want its three axes in order", s.Coord.Dims)
	}
	if len(s.Layer[0].Encoding.Dims) != 3 || s.Layer[0].Encoding.Dims[2].Field != "weight" {
		t.Fatalf("the layer wrote %+v, want one channel per axis", s.Layer[0].Encoding.Dims)
	}
	b, err := s.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"type": "parallel"`, `"dims"`, `"name": "mpg"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("the document does not contain %s:\n%s", want, b)
		}
	}
}

// A hand-written document reads, which is what proves the decoder's source
// gate covers a mark whose columns are a list: a layer that read back with no
// table would be refused by geom.FromDesc, and a round trip over a built chart
// would never show it.
func TestAHandWrittenParallelSpecReads(t *testing.T) {
	doc := `{
	  "width": 480, "height": 320,
	  "coord": {"type": "parallel", "dims": [
	    {"name": "mpg", "scale": {"type": "linear"}},
	    {"name": "power", "scale": {"type": "linear"}}
	  ]},
	  "data": {"values": [
	    {"mpg": 30, "power": 60},
	    {"mpg": 20, "power": 120}
	  ]},
	  "layer": [
	    {"mark": {"type": "parallel"},
	     "encoding": {"dims": [{"field": "mpg"}, {"field": "power"}]}}
	  ]
	}`
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Chart()
	if err != nil {
		t.Fatalf("a hand-written parallel chart was refused: %v", err)
	}
	if got := len(coord.Dimensions(c.Coord)); got != 2 {
		t.Fatalf("the coord read back with %d axes, want two", got)
	}
	d, ok := geom.Describe(c.Layers[0])
	if !ok || d.Mark != geom.MarkParallel {
		t.Fatalf("the layer read back as %q", d.Mark)
	}
	if len(d.Dims) != 2 || d.Dims[1] != "power" {
		t.Errorf("the layer's columns read back as %v", d.Dims)
	}
	if d.Source == nil {
		t.Error("the layer read back with no data source")
	}
}

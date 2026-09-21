package spec_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

// The map projection, written down and read back. A projection that survived
// the round trip as its type and lost which projection it was would draw a
// different map from the one the document came from, which is the whole reason
// the set of them is closed and named.

func stations() *data.Table {
	return data.NewTable().
		Float64("lon", []float64{-3.2, 13.4, 2.35, -74, 139.7}).
		Float64("lat", []float64{55.9, 52.5, 48.9, 40.7, 35.7}).
		Float64("reading", []float64{4, 9, 7, 3, 8})
}

func mapChart(c coord.Coord, layers ...geom.Geom) spec.Chart {
	return spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X:     scale.Linear(scale.Domain(-180, 180)),
		Y:     scale.Linear(scale.Domain(-90, 90)),
		Coord: c, Layers: layers,
	}
}

func TestEveryMapSurvivesTheRoundTrip(t *testing.T) {
	src := stations()
	for _, tc := range []struct {
		name  string
		coord coord.Coord
	}{
		{"equirectangular", coord.Geo(coord.PlateCarree)},
		{"mercator", coord.Geo(coord.Mercator)},
		{"mollweide", coord.Geo(coord.Mollweide)},
		{"a globe", coord.Geo(coord.Orthographic, coord.GeoCenter(10, 30))},
		{"a Pacific-centred world", coord.Geo(coord.Mollweide, coord.GeoCenter(150, 0))},
		{"edges along the graticule", coord.Geo(coord.Mollweide, coord.GeoArc())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mapChart(tc.coord, geom.Scatter(src, geom.X("lon"), geom.Y("lat")))
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the %s map did not survive the round trip\n%s", tc.name, b)
			}
		})
	}
}

// A document says which map it is even when the projection is the default
// one. "geo" alone would be a coord whose picture a reader has to guess at,
// and the guess would be right only for the projection a table of degrees is
// already in.
func TestAMapDocumentNamesItsProjection(t *testing.T) {
	c := mapChart(coord.Geo(coord.PlateCarree), geom.Scatter(stations(), geom.X("lon"), geom.Y("lat")))
	s, err := spec.Of(c)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	b, err := s.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, field := range []string{`"type": "geo"`, `"projection": "plate-carree"`} {
		if !strings.Contains(string(b), field) {
			t.Errorf("a map document is missing %s:\n%s", field, b)
		}
	}
	// The polar fields are not a map's, and "theta": "x" is not omitted by
	// omitempty because "x" is not empty.
	for _, field := range []string{`"theta"`, `"hole"`, `"sum"`, `"admittance"`} {
		if strings.Contains(string(b), field) {
			t.Errorf("a map document carries %s, which is another coord's:\n%s", field, b)
		}
	}
}

// A document that names a map and nothing else draws what the constructor
// draws: the equal-area projection, centred on the prime meridian, with an
// edge between two rows drawn as the chord it is. The default is the one that
// does not mislead about size, because a map that does is one a document says
// so on.
func TestAMapsDefaultsAreTheConstructorsInJSONToo(t *testing.T) {
	s, err := spec.Parse([]byte(`{
		"width": 400, "height": 300,
		"encoding": {"x": {"field": "lon"}, "y": {"field": "lat"}},
		"mark": "point",
		"coord": {"type": "geo"}
	}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	doc, err := s.Chart()
	if err != nil {
		t.Fatalf("Chart: %v", err)
	}
	d, ok := coord.Describe(doc.Coord)
	if !ok {
		t.Fatalf("the decoded coord %T cannot describe itself", doc.Coord)
	}
	if want, _ := coord.Describe(coord.Geo(coord.Mollweide)); !reflect.DeepEqual(d, want) {
		t.Errorf("decoded %+v, want the coord Geo(Mollweide) builds, %+v", d, want)
	}
}

// A projection nobody defined is an error rather than a map drawn in some
// other projection: silently falling back would put a chart on the page whose
// every position is wrong by a projection's worth.
func TestAnUnknownProjectionIsRefused(t *testing.T) {
	s, err := spec.Parse([]byte(`{
		"width": 400, "height": 300,
		"encoding": {"x": {"field": "lon"}, "y": {"field": "lat"}},
		"mark": "point",
		"coord": {"type": "geo", "projection": "peirce-quincuncial"}
	}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := s.Chart(); err == nil {
		t.Error("a document naming a projection this library does not have was accepted")
	}
}

package spec_test

import (
	"strings"
	"testing"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

func nicholsChart(layers ...geom.Geom) spec.Chart {
	return spec.Chart{
		Width: 520, Height: 420, DPR: 1, Theme: theme.Light,
		X:      scale.Linear(scale.Domain(-360, 0), scale.TickValues(-360, -270, -180, -90, 0)),
		Y:      scale.Linear(scale.Domain(-40, 40)),
		Layers: layers,
	}
}

// A locus survives the round trip, which for this mark means the family and the
// levels: everything else about it is a formula, so a document that lost either
// would read back as a different set of curves or as none.
func TestALocusSurvivesTheRoundTrip(t *testing.T) {
	c := nicholsChart(
		geom.Locus(stat.NicholsM, []float64{-12, -6, -3, -1, 0, 1, 3, 6, 12}),
		geom.Locus(stat.NicholsN, []float64{-1, -5, -10, -20, -45, -90, -150}, geom.Label("∠T")),
	)
	want, got := draw(t, c), draw(t, roundTrip(t, c))
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		s, _ := spec.Of(c)
		b, _ := s.Marshal()
		t.Errorf("the locus did not survive the round trip\n%s", b)
	}
}

// The document names the family and lists the levels, and carries nothing that
// would read as though a locus measured something.
func TestALocusWritesItsFamilyAndItsLevels(t *testing.T) {
	s, err := spec.Of(nicholsChart(geom.Locus(stat.NicholsN, []float64{-45, -90})))
	if err != nil {
		t.Fatal(err)
	}
	m := s.Layer[0].Mark
	if m.Type != "locus" {
		t.Errorf("the mark is %q, want locus", m.Type)
	}
	if m.Family != "nichols-n" {
		t.Errorf("the mark's family is %q, want nichols-n", m.Family)
	}
	if len(m.Levels) != 2 {
		t.Errorf("the locus wrote levels %v, want two", m.Levels)
	}
	if s.Layer[0].Data != nil || s.Layer[0].Encoding != nil && s.Layer[0].Encoding.X != nil {
		t.Errorf("the locus wrote data or a positional channel: %+v", s.Layer[0])
	}
}

// The rule ADR 0041 set for a quantile function, applied to a curve: a family
// this library names has a name to write down and a family a caller wrote in Go
// has not. It fails rather than writing a document that decodes into a layer
// drawing nothing.
func TestAFamilyWrittenInGoDeclinesToSerialise(t *testing.T) {
	_, err := spec.Of(nicholsChart(geom.Locus(spiral{}, []float64{1})))
	if err == nil {
		t.Fatal("a locus of a family with no name was written down anyway")
	}
	if !strings.Contains(err.Error(), "locus") {
		t.Errorf("the error does not say what could not be written: %v", err)
	}
}

// A document naming a family this library does not have is refused, rather than
// decoded into a layer that silently draws nothing.
func TestADocumentNamingAnUnknownFamilyIsRefused(t *testing.T) {
	doc := []byte(`{"width":400,"height":300,"layer":[{"mark":{"type":"locus","family":"nichols-p","levels":[1]}}]}`)
	s, err := spec.Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := s.Chart(); err == nil {
		t.Error("a locus of an unknown family was built anyway")
	}
}

type spiral struct{}

func (spiral) Locus(xs, ys []float64, level float64, ext stat.Extent) ([]float64, []float64) {
	return append(xs, -180, -90), append(ys, level, level)
}

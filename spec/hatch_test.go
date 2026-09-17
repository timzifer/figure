package spec_test

import (
	"bytes"
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

// hatchSource is a small stacked bar: two series over three categories, which
// is the chart the hatch channel exists for.
func hatchSource() data.Source {
	return data.NewTable().
		String("q", []string{"Q1", "Q2", "Q3", "Q1", "Q2", "Q3"}).
		String("region", []string{"north", "north", "north", "south", "south", "south"}).
		Float64("v", []float64{4, 6, 5, 3, 7, 2})
}

func hatchChart(opts ...geom.Option) spec.Chart {
	base := []geom.Option{geom.X("q"), geom.Y("v"), geom.GroupBy("region")}
	return spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X: scale.Ordinal(), Y: scale.Linear(),
		Layers: []geom.Geom{geom.Bar(hatchSource(), append(base, opts...)...)},
	}
}

func TestEveryFillFinishSurvivesTheRoundTrip(t *testing.T) {
	cases := map[string][]geom.Option{
		"hatch":          {geom.Hatch(ir.HatchCross)},
		"hatch none":     {geom.Hatch(ir.HatchNone)},
		"density":        {geom.Hatch(ir.HatchDiagonal), geom.HatchDensity(0.5)},
		"gradient":       {geom.Gradient(palette.SkyBlue)},
		"corner":         {geom.Corner(4)},
		"inset":          {geom.Inset(1.5)},
		"all together":   {geom.Hatch(ir.HatchScales), geom.HatchDensity(1.5), geom.Gradient(ir.Transparent), geom.Corner(3), geom.Inset(1)},
		"nothing at all": nil,
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			c := hatchChart(opts...)
			back := roundTrip(t, c)
			want, got := draw(t, c), draw(t, back)
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				t.Errorf("the %s layer did not survive the round trip", name)
			}
		})
	}
}

// TestAnUnsetHatchIsNotAHatchOfNone guards the one distinction the pair of
// fields exists for: a layer that never named a pattern still takes the
// theme's, and a layer that named HatchNone stays plain whatever the theme
// says. A document that dropped the difference would turn the first into the
// second.
func TestAnUnsetHatchIsNotAHatchOfNone(t *testing.T) {
	th := theme.Light.With(theme.Redundant(true))

	unset := hatchChart()
	unset.Theme = th
	pinned := hatchChart(geom.Hatch(ir.HatchNone))
	pinned.Theme = th

	if a, b := draw(t, unset), draw(t, pinned); strings.Join(a, "\n") == strings.Join(b, "\n") {
		t.Fatal("a pinned plain layer drew what a layer taking the theme's ladder drew")
	}
	// And the two stay two across a document. A theme travels as its name, so
	// the round trip is taken over the default one and the distinction is read
	// back off the layer rather than off what it drew.
	for name, want := range map[string]bool{"unset": false, "pinned": true} {
		c := hatchChart()
		if want {
			c = hatchChart(geom.Hatch(ir.HatchNone))
		}
		d, ok := geom.Describe(roundTrip(t, c).Layers[0])
		if !ok {
			t.Fatalf("%s: the layer did not describe itself", name)
		}
		if d.HatchSet != want {
			t.Errorf("%s: HatchSet read back as %v, want %v", name, d.HatchSet, want)
		}
	}
}

// TestAPatternNobodyKnowsDrawsPlainly is the forward-compatibility rule every
// other vocabulary in spec follows: a document written by a later version is
// read as far as it can be, and the part this reader does not understand
// leaves the mark plain rather than failing the read.
func TestAPatternNobodyKnowsDrawsPlainly(t *testing.T) {
	sp, err := spec.Of(hatchChart(geom.Hatch(ir.HatchCross)))
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	b, err := sp.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// A pattern from a release this reader has never heard of.
	later := bytes.Replace(b, []byte(`"hatch": "cross"`), []byte(`"hatch": "guilloche"`), 1)
	if bytes.Equal(later, b) {
		t.Fatal("the document did not carry a hatch name to replace")
	}

	back, err := spec.Parse(later)
	if err != nil {
		t.Fatalf("parsing a document with an unknown pattern failed: %v", err)
	}
	c, err := back.Chart()
	if err != nil {
		t.Fatalf("Chart: %v", err)
	}
	d, ok := geom.Describe(c.Layers[0])
	if !ok {
		t.Fatal("the layer did not describe itself")
	}
	if d.Hatch != ir.HatchNone {
		t.Errorf("an unknown pattern read back as %v, want none", d.Hatch)
	}
	if !d.HatchSet {
		t.Error("an unknown pattern read back as unset, so the layer would take the theme's ladder")
	}
}

package figure_test

import (
	"strings"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// quarters is a stacked bar chart: three series over four quarters, which is
// the chart the hatch channel exists for. Two of its series are the two
// palette entries eight percent of men cannot separate.
func quarters() figure.Source {
	return figure.NewTable().
		String("q", []string{"Q1", "Q2", "Q3", "Q4", "Q1", "Q2", "Q3", "Q4", "Q1", "Q2", "Q3", "Q4"}).
		String("region", []string{
			"north", "north", "north", "north",
			"south", "south", "south", "south",
			"east", "east", "east", "east",
		}).
		Float64("units", []float64{18, 24, 21, 30, 12, 9, 15, 11, 7, 13, 9, 16})
}

func stackedBars(th theme.Theme, opts ...geom.Option) *figure.Plot {
	p := figure.New(
		figure.Size(520, 340),
		figure.Title("Units shipped"),
		figure.Legend(true),
		figure.Theme(th),
	)
	p.X(scale.Ordinal())
	p.Y(scale.Linear(scale.Nice(), scale.Zero()))
	p.Add(geom.Bar(quarters(), append([]geom.Option{
		geom.X("q"), geom.Y("units"), geom.GroupBy("region"),
	}, opts...)...))
	return p
}

func TestGoldenHatchedBars(t *testing.T) {
	// The nominal ladder: every series a different pattern, none heavier than
	// another. This is what theme.Redundant installs.
	t.Run("redundant", func(t *testing.T) {
		golden(t, "hatch-redundant", stackedBars(theme.Light.With(theme.Redundant(true))))
	})

	// The ordinal one: one pattern getting denser, for a stack that is an
	// order rather than a set of categories.
	t.Run("density", func(t *testing.T) {
		th := theme.Light.With(theme.Redundant(true), theme.Hatches(theme.DensitySeriesHatches...))
		golden(t, "hatch-density", stackedBars(th))
	})

	// The finishes that are not patterns, together: rounded corners, an inner
	// border, and a fill that ramps toward the tips.
	t.Run("finishes", func(t *testing.T) {
		golden(t, "hatch-finishes", stackedBars(theme.Light,
			geom.Corner(4), geom.Inset(1), geom.Gradient(ir.RGBA(0xFF, 0xFF, 0xFF, 0x60))))
	})
}

// TestRedundantEncodingReachesAFilledMark is the claim ADR 0069 makes: before
// it, theme.Redundant did nothing at all for a bar chart, because a dash needs
// a stroke and a marker needs a point.
func TestRedundantEncodingReachesAFilledMark(t *testing.T) {
	plain := render(t, stackedBars(theme.Light))
	marked := render(t, stackedBars(theme.Light.With(theme.Redundant(true))))
	if plain == marked {
		t.Fatal("theme.Redundant changed nothing about a stacked bar chart")
	}
}

// TestAHatchIsOffUnlessItIsAskedFor guards every golden file in the corpus: a
// chart that named no pattern and a theme that installs no ladder must draw
// exactly what they drew before hatching existed.
func TestAHatchIsOffUnlessItIsAskedFor(t *testing.T) {
	out := render(t, stackedBars(theme.Light))
	// A hatch reaches a backend as a clipped stroke, and a bar chart draws no
	// other clipped stroke inside a mark. Counting clip paths is therefore the
	// cheapest way to ask whether anything was hatched: one for the panel, and
	// no more.
	if n := strings.Count(out, "<clipPath"); n > 1 {
		t.Errorf("an unhatched chart emitted %d clip paths, want at most the panel's", n)
	}
}

// render draws p to SVG and returns it.
func render(t *testing.T, p *figure.Plot) string {
	t.Helper()
	var b strings.Builder
	if err := p.Render(figure.SVGWriter(&b)); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return b.String()
}

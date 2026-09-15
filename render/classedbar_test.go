package render_test

import (
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

// solidFills returns the colours of every solid fill in a recording. A classed
// bar paints one per class, where a continuous bar paints one gradient.
func solidFills(rec *irtest.Recorder) map[ir.Color]bool {
	out := map[ir.Color]bool{}
	for _, c := range rec.Filter("FillPath") {
		if !c.Fill.IsGradient() {
			out[c.Fill.Color] = true
		}
	}
	return out
}

func TestAClassedScaleGetsASteppedBarRatherThanAGradient(t *testing.T) {
	cs := scale.Threshold(palette.Viridis, []float64{20, 30})
	rec := draw(t, chart(colored(cs)))
	if got := len(gradients(rec)); got != 0 {
		t.Errorf("a classed scale drew %d gradients, want none", got)
	}
	// Three classes, three colours, and each of them painted.
	cs.Train(10, 40)
	seen := solidFills(rec)
	for _, v := range []float64{10, 25, 35} {
		if !seen[cs.Color(v)] {
			t.Errorf("the bar has no band in the colour of %g", v)
		}
	}
}

// A classed bar is labelled at its boundaries and nowhere else: a round number
// between two of them would invite a reader to interpolate across a step that
// has no inside.
func TestAClassedBarIsLabelledAtItsBreaks(t *testing.T) {
	rec := draw(t, chart(colored(scale.Threshold(palette.Viridis, []float64{20, 30}))))
	for _, want := range []string{"20", "30"} {
		if !hasText(rec, want) {
			t.Errorf("the bar has no %q boundary label: %v", want, texts(rec))
		}
	}
	// 25 is a round number inside a class, and the bar must not claim it is a
	// boundary.
	if hasText(rec, "25") {
		t.Errorf("the bar labelled a value inside a class: %v", texts(rec))
	}
}

func TestAQuantizedBarIsLabelledAtItsDerivedBreaks(t *testing.T) {
	// The column runs 10..40, so three classes break at 20 and 30.
	rec := draw(t, chart(colored(scale.Quantize(palette.Viridis, 3))))
	for _, want := range []string{"20", "30"} {
		if !hasText(rec, want) {
			t.Errorf("the bar has no %q boundary label: %v", want, texts(rec))
		}
	}
}

// Two classed scales that differ only in where they cut are two different
// bars, and the guide key has to see the difference.
func TestClassedBarsWithDifferentBreaksAreNotMerged(t *testing.T) {
	a := colored(scale.Threshold(palette.Viridis, []float64{20}))
	b := colored(scale.Threshold(palette.Viridis, []float64{30}))
	rec := draw(t, chart(a, b))
	labels := 0
	for _, want := range []string{"20", "30"} {
		if hasText(rec, want) {
			labels++
		}
	}
	if labels != 2 {
		t.Errorf("two scales cutting at different values drew %d of the two boundaries: %v", labels, texts(rec))
	}
}

// A quantile bar's bands are as tall as their classes are wide, which over
// skewed data means very different heights. That is the distribution showing
// through, and it is the reason the bar is not drawn in equal blocks.
func TestAQuantileBarKeepsItsClassesInProportion(t *testing.T) {
	rec := draw(t, chart(colored(scale.Quantile(palette.Viridis, 2))))
	if got := len(gradients(rec)); got != 0 {
		t.Errorf("a quantile scale drew %d gradients, want none", got)
	}
	// The column is 10, 20, 30, 40, so the median is 25.
	if !hasText(rec, "25") {
		t.Errorf("the bar has no median label: %v", texts(rec))
	}
}

// A line coloured by a classed scale contributes the same stepped bar a
// classed scatter does: the guide follows from the scale, not from the mark.
func TestAThresholdLineGetsAClassedColourbar(t *testing.T) {
	src := data.Float64Columns(map[string][]float64{
		"x": {0, 1, 2, 3},
		"y": {10, 25, 35, 15},
	})
	cs := scale.Threshold(palette.Viridis, []float64{20, 30})
	rec := draw(t, chart(geom.Line(src, geom.X("x"), geom.Y("y"), geom.ColorBy("y", cs))))
	for _, want := range []string{"20", "30"} {
		if !hasText(rec, want) {
			t.Errorf("the bar has no %q boundary label: %v", want, texts(rec))
		}
	}
	if got := len(gradients(rec)); got != 0 {
		t.Errorf("a classed line drew %d gradients, want a stepped bar", got)
	}
}

// A boundary somebody computed is written as precisely as it is. A control
// chart's limits at 12.73 and 37.26 on a bar over 10..40 were labelled "13"
// and "37" — the axis's whole-number precision — which are boundaries the
// chart does not have.
func TestAComputedBoundaryIsNotRoundedToTheAxis(t *testing.T) {
	rec := draw(t, chart(colored(scale.Threshold(palette.Viridis, []float64{12.73, 37.26}))))
	for _, want := range []string{"12.73", "37.26"} {
		if !hasText(rec, want) {
			t.Errorf("the bar has no %q boundary label: %v", want, texts(rec))
		}
	}
	for _, wrong := range []string{"13", "37"} {
		if hasText(rec, wrong) {
			t.Errorf("the bar rounded a boundary to %q: %v", wrong, texts(rec))
		}
	}
}

// A boundary with no short decimal is written to a precision the narrowest
// class can carry, not to every digit it has.
func TestAnIrrationalBoundaryIsWrittenToAReadablePrecision(t *testing.T) {
	rec := draw(t, chart(colored(scale.Threshold(palette.Viridis, []float64{20 + 1.0/3, 30}))))
	if !hasText(rec, "20.33") {
		t.Errorf("want the boundary at 20⅓ written as 20.33 over classes about ten wide: %v", texts(rec))
	}
}

func TestALayerCanDeclineItsGuide(t *testing.T) {
	rec := draw(t, chart(colored(scale.Sequential(palette.Viridis), geom.Guide(false))))
	if n := len(gradients(rec)); n != 0 {
		t.Errorf("a layer that declined its guide still drew %d colourbars", n)
	}
	classed := draw(t, chart(colored(scale.Threshold(palette.Viridis, []float64{20, 30}), geom.Guide(false))))
	if hasText(classed, "20") || hasText(classed, "30") {
		t.Errorf("a classed layer that declined its guide still labelled its boundaries: %v", texts(classed))
	}

	// Declining is this layer's alone: another layer on the same scale still
	// brings the bar.
	shared := scale.Sequential(palette.Viridis)
	both := draw(t, chart(colored(shared, geom.Guide(false)), colored(shared)))
	if n := len(gradients(both)); n != 1 {
		t.Errorf("%d colourbars with one of two layers declining, want 1", n)
	}
}

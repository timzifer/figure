package spec_test

import (
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

// A reversed axis through the document. The word is `"reverse"`, which a colour
// scale already uses for turning a ramp around; the channel a scale hangs off
// is what says which of the two ideas is meant, so the test pins both the word
// and the drawing it comes back as.
// See docs/adr/0075-an-axis-has-a-direction.md.

func reverseChart() spec.Chart {
	src := data.NewTable().
		Float64("n", []float64{3, 7, 5}).
		String("what", []string{"mail", "drive", "chat"})
	return spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X:      scale.Linear(scale.Zero(), scale.Reverse()),
		Y:      scale.Ordinal(),
		Layers: []geom.Geom{geom.Rect(src, geom.X("n"), geom.Y("what"))},
	}
}

func TestReversedAxisSurvivesTheRoundTrip(t *testing.T) {
	c := reverseChart()
	want, got := draw(t, c), draw(t, roundTrip(t, c))
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		s, _ := spec.Of(c)
		b, _ := s.Marshal()
		t.Errorf("a reversed axis did not survive the round trip\n%s", b)
	}
}

func TestReversedAxisIsWrittenAsReverse(t *testing.T) {
	s, err := spec.Of(reverseChart())
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	b, err := s.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"reverse": true`) {
		t.Errorf("the document does not carry the reversal:\n%s", b)
	}
}

// An unreversed axis must not start writing the word, which is what would
// happen if the field were filled in for every kind rather than for the one
// that reads it back.
func TestAnOrdinaryAxisWritesNoReverse(t *testing.T) {
	c := reverseChart()
	c.X = scale.Linear(scale.Zero())
	s, err := spec.Of(c)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	b, err := s.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(b), `"reverse"`) {
		t.Errorf("an ordinary axis wrote a reversal:\n%s", b)
	}
}

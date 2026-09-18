package render_test

import (
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/render"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// A reversed axis ([scale.Reverse]) is the one case where a tick sequence,
// which is ascending by value, crosses the panel right to left. The label
// collision pass sweeps across the panel dropping anything that would run into
// the label before it, so it has to meet them in screen order: sweeping in tick
// order kept the rightmost label and found every other one behind it, and a
// chart came out with one number on its axis.
// See docs/adr/0075-an-axis-has-a-direction.md.

func reversedAxisChart(x scale.Scale) render.Chart {
	src := data.NewTable().
		Float64("n", []float64{3, 7, 5}).
		String("what", []string{"mail", "drive", "chat"})
	return render.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X: x, Y: scale.Ordinal(),
		Layers: []geom.Geom{geom.Rect(src, geom.X("n"), geom.Y("what"))},
	}
}

func drawnTicks(t *testing.T, x scale.Scale) ([]string, []float32) {
	t.Helper()
	rec := irtest.New()
	if err := render.Draw(rec, reversedAxisChart(x)); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	var labels []string
	var at []float32
	for _, c := range rec.Filter("Text") {
		switch c.Text.Text {
		case "mail", "drive", "chat":
			continue // the ordinal axis's own labels
		}
		labels = append(labels, c.Text.Text)
		at = append(at, c.Text.At.X)
	}
	return labels, at
}

func TestReversedAxisKeepsEveryTickLabel(t *testing.T) {
	fwd, fwdAt := drawnTicks(t, scale.Linear(scale.Zero()))
	rev, revAt := drawnTicks(t, scale.Linear(scale.Zero(), scale.Reverse()))

	if len(fwd) < 3 {
		t.Fatalf("the forward axis drew %v, which is too few to tell anything from", fwd)
	}
	if len(rev) != len(fwd) {
		t.Fatalf("reversed axis drew %v, forward one drew %v", rev, fwd)
	}
	for i := range fwd {
		if rev[i] != fwd[i] {
			t.Errorf("label %d is %q reversed and %q forward", i, rev[i], fwd[i])
		}
	}

	// Same labels, mirrored: the forward axis's run left to right and the
	// reversed axis's run right to left.
	for i := 1; i < len(fwdAt); i++ {
		if fwdAt[i] <= fwdAt[i-1] {
			t.Fatalf("the forward axis's labels are not left to right: %v", fwdAt)
		}
		if revAt[i] >= revAt[i-1] {
			t.Errorf("the reversed axis's labels are not right to left: %v", revAt)
		}
	}
}

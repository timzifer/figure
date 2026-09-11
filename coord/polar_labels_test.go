package coord_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// polarFurniture frames c over a square panel centred on (200, 200) with a
// radial domain of [0, 1] and an angular one of [0, 100], and fills its
// furniture.
func polarFurniture(t *testing.T, c coord.Coord) (*coord.Furniture, []scale.Tick, []scale.Tick) {
	t.Helper()
	x := scale.Linear(scale.Domain(0, 1))
	y := scale.Linear(scale.Domain(0, 100))
	area := ir.R(0, 0, 400, 400)
	framed := c.Frame(coord.Framing{Area: area, X: x, Y: y})
	xt := x.Ticks(scale.TickRequest{Want: 5})
	yt := y.Ticks(scale.TickRequest{Want: 5})
	var fur coord.Furniture
	framed.Furniture(&fur, coord.FurnitureRequest{
		Area:    area,
		Metrics: coord.Metrics{TickLen: 4, LabelPad: 3},
		XTicks:  xt,
		YTicks:  yt,
	})
	return &fur, xt, yt
}

// A full turn's last tick is its first. A pie over [0, 100] labels twelve
// o'clock once, as 0, rather than writing 100 over it.
func TestAFullTurnLabelsItsSeamOnce(t *testing.T) {
	fur, _, yt := polarFurniture(t, coord.Pie())
	if len(yt) < 2 || yt[0].Value != 0 || yt[len(yt)-1].Value != 100 {
		t.Fatalf("the angular ticks are %v, want 0 through 100", yt)
	}
	if !fur.InY[0] {
		t.Error("the tick at the start of the turn was dropped")
	}
	if fur.InY[len(yt)-1] {
		t.Error("the tick at the end of the turn was kept, on top of the one at its start")
	}
	if !fur.GridY[len(yt)-1].Empty() {
		t.Error("the tick at the end of the turn drew a second spoke over the first")
	}
}

// A partial sweep has no seam: both ends of a gauge are labelled.
func TestAPartialSweepLabelsBothEnds(t *testing.T) {
	fur, _, yt := polarFurniture(t, coord.Pie(coord.Sweep(math.Pi), coord.Start(-math.Pi/2)))
	if !fur.InY[0] || !fur.InY[len(yt)-1] {
		t.Errorf("a half-turn gauge kept its end ticks as %v and %v, want both", fur.InY[0], fur.InY[len(yt)-1])
	}
}

// A gauge's radial axis lies along the diameter, with the marks above it and
// nothing below. Its labels go below, centred under their ticks; written on
// the other side, the first slice is drawn over them.
func TestAGaugeLabelsItsRadiusOnTheEmptySide(t *testing.T) {
	fur, xt, _ := polarFurniture(t, coord.Pie(coord.Sweep(math.Pi), coord.Start(-math.Pi/2)))
	for i := range xt {
		if !fur.InX[i] {
			continue
		}
		l := fur.LabelX[i]
		if l.At.Y <= 200 || l.H != ir.AlignCenter || l.V != ir.AlignTop {
			t.Errorf("radial label %d sits at %v aligned %d/%d, want below the diameter, centred and hanging", i, l.At, l.H, l.V)
		}
	}
	// Running the other way round puts the marks below and the labels above.
	fur, xt, _ = polarFurniture(t, coord.Pie(coord.Sweep(math.Pi), coord.Start(-math.Pi/2), coord.Counterclockwise(true)))
	for i := range xt {
		if fur.InX[i] && (fur.LabelX[i].At.Y >= 200 || fur.LabelX[i].V != ir.AlignBottom) {
			t.Errorf("counterclockwise radial label %d sits at %v, want above the diameter", i, fur.LabelX[i].At)
		}
	}
}

// A full turn has marks on both sides of its spoke, and its radial labels stay
// where they always were: clockwise of it, starting a pad off it.
func TestAFullTurnLabelsItsRadiusClockwiseOfTheSpoke(t *testing.T) {
	fur, xt, _ := polarFurniture(t, coord.Pie())
	for i := range xt {
		if !fur.InX[i] {
			continue
		}
		l := fur.LabelX[i]
		if l.At.X != 203 || l.H != ir.AlignStart || l.V != ir.AlignMiddle {
			t.Errorf("radial label %d sits at %v aligned %d/%d, want x=203, starting, middle", i, l.At, l.H, l.V)
		}
	}
}

// The angle's labels are the reading, so where the radius's run into them the
// angle's are kept — whichever scale the angle comes from.
func TestTheAngularLabelsGoFirst(t *testing.T) {
	if fur, _, _ := polarFurniture(t, coord.Pie()); !fur.LabelsYFirst {
		t.Error("a pie takes its angle from Y and did not put Y's labels first")
	}
	if fur, _, _ := polarFurniture(t, coord.Polar()); fur.LabelsYFirst {
		t.Error("a rose takes its angle from X and put Y's labels first")
	}
}

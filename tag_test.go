package figure_test

import (
	"math"
	"strings"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

func lastPriced() *data.Table {
	return figure.NewTable().
		Float64("t", []float64{0, 1, 2, 3}).
		Float64("o", []float64{90, 92, 95, 93}).
		Float64("c", []float64{92, 95, 93, 94.37})
}

// The last value is tagged on the price axis at its own precision, the rule
// runs across the panel to it, and the tick label it covers is not drawn.
func TestTheLastValueIsTaggedOnItsAxis(t *testing.T) {
	p := figure.New(figure.Size(400, 300))
	p.X(scale.Linear()).Y(scale.Linear(scale.Domain(90, 100)))
	p.Add(geom.Line(lastPriced(), geom.X("t"), geom.Y("c")),
		geom.LastValue(lastPriced(), geom.X("t"), geom.Y("c"), geom.DirectionBy("o", "c")))
	svg := renderString(t, p)
	if !strings.Contains(svg, ">94.37<") {
		t.Error("the last value is not tagged at its own precision")
	}
	if strings.Contains(svg, ">94<") {
		t.Error("the tick label under the tag was drawn as well")
	}
	// Rising — 94.37 closed above its open of 93 — so the tag is blue.
	if !strings.Contains(strings.ToLower(svg), "#0072b2") {
		t.Error("the tag is not in the rising colour")
	}
}

// The newest row is the one furthest along X, not the last in the table.
func TestTheLastValueIsTheNewestRow(t *testing.T) {
	src := figure.NewTable().Float64("t", []float64{3, 1, 2}).Float64("y", []float64{7.25, 1, 2})
	p := figure.New(figure.Size(400, 300))
	p.X(scale.Linear()).Y(scale.Linear(scale.Domain(0, 10)))
	p.Add(geom.LastValue(src, geom.X("t"), geom.Y("y"), geom.Rule(false)))
	if svg := renderString(t, p); !strings.Contains(svg, ">7.25<") {
		t.Error("the tag is not at the newest row's value")
	}
}

// A tag at a value the axis does not show is not drawn.
func TestATagOffTheAxisIsNotDrawn(t *testing.T) {
	p := figure.New(figure.Size(400, 300))
	p.X(scale.Linear()).Y(scale.Linear(scale.Domain(0, 50)))
	p.Add(geom.LastValue(lastPriced(), geom.X("t"), geom.Y("c")))
	if svg := renderString(t, p); strings.Contains(svg, ">94.37<") {
		t.Error("a tag above a pinned axis was drawn")
	}
}

// A snapping crosshair crosses at the candle nearest the pointer's X, at its
// close, and tags its time and price on the axes.
func TestACrosshairSnapsToTheCandleAndTagsItsAxes(t *testing.T) {
	src := figure.NewTable().
		Float64("t", []float64{0, 1, 2, 3}).
		Float64("o", []float64{10, 12, 15, 13}).
		Float64("h", []float64{13, 16, 16, 14}).
		Float64("l", []float64{9, 11, 12, 12}).
		Float64("c", []float64{12, 15, 13, 13.5})
	x, y := scale.Linear(), scale.Linear()
	p := figure.New(figure.Size(400, 300))
	p.X(x).Y(y)
	p.Add(geom.Candle(src, geom.X("t"), geom.OHLC("o", "h", "l", "c")))
	rec := irtest.New()
	live, err := p.Live(rec.Target())
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	live.TrackRows(true)
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	cross := &figure.Crosshair{Snap: live.Index(), Tags: true, Show: true, Panel: -1}
	live.Overlay(cross)
	// Just right of the candle at 2, and high above its close.
	cross.At = ir.Point{X: x.Map(2.3), Y: y.Map(15.9)}
	rec.Reset()
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	var vertical, horizontal float32 = -1, -1
	var texts []string
	for _, c := range rec.Calls {
		switch {
		case c.Op == "Polyline" && len(c.Points) == 2 && c.Points[0].X == c.Points[1].X:
			vertical = c.Points[0].X
		case c.Op == "Polyline" && len(c.Points) == 2 && c.Points[0].Y == c.Points[1].Y:
			horizontal = c.Points[0].Y
		case c.Op == "Text":
			texts = append(texts, c.Text.Text)
		}
	}
	if math.Abs(float64(vertical-x.Map(2))) > 0.5 {
		t.Errorf("the vertical line is at %v, want the candle at %v", vertical, x.Map(2))
	}
	if math.Abs(float64(horizontal-y.Map(13))) > 0.5 {
		t.Errorf("the horizontal line is at %v, want the candle's close at %v", horizontal, y.Map(13))
	}
	// The tags are the last two texts drawn: the snapped time on X and the
	// close on Y, each in its axis's format.
	if n := len(texts); n < 2 || !strings.HasPrefix(texts[n-2], "2") || !strings.HasPrefix(texts[n-1], "13") {
		t.Errorf("the crosshair's tags read %v, want the candle's time 2 and its close 13", texts)
	}
}

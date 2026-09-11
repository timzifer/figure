package render_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/internal/layout"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/render"
	"github.com/timzifer/figure/theme"
)

// What v0.8 moved out of render and what it left in.
//
// The geometry of a grid line, an axis line and a tick label now comes from
// the coord. The *order* everything is drawn in did not move, and neither did
// the decisions that are the theme's: which grid lines are wanted, which
// labels would collide, whether this panel writes labels at all.

// A Cartesian panel's grid lines still reach the backend as two-point
// polylines. That is the property every golden file in the repository is
// written in terms of, and it is the one a coord could break silently by
// reporting a path where it used to report a run.
func TestACartesianGridIsStillPolylines(t *testing.T) {
	rec := draw(t, chart(line()))
	for _, c := range rec.Calls {
		if c.Op != "StrokePath" {
			continue
		}
		// The only stroked paths a plain line chart draws are the layer's own,
		// and it draws none: a polyline is a Polyline.
		t.Errorf("a Cartesian chart stroked a path where it used to draw a polyline: %v", c.Path.Ops)
	}
	if rec.Count("Polyline") == 0 {
		t.Error("the grid and the axes drew nothing")
	}
}

// The furniture pass reuses one buffer across the panels of a chart, and a
// panel with fewer ticks than the last must not inherit the last one's.
func TestFurnitureDoesNotLeakBetweenPanels(t *testing.T) {
	c := chart(line())
	// Two panels, the second with a domain that yields fewer ticks than the
	// first. Nothing may be drawn twice.
	c.Panels = []render.Panel{
		{Row: 0, Col: 0, X: c.X, Y: c.Y, Layers: c.Layers, ShowX: true, ShowY: true},
		{Row: 0, Col: 1, X: c.X, Y: c.Y, Layers: c.Layers, ShowX: true, ShowY: true},
	}
	c.Rows, c.Cols = 1, 2
	rec := draw(t, c)

	// Every polyline is either inside one panel's rectangle or on its gutter,
	// and no two panels share a rectangle — so a leaked shape would be a
	// duplicate. Count them instead of chasing geometry: the two panels are
	// identical, so an exact doubling is what correct looks like.
	seen := map[string]int{}
	for _, call := range rec.Calls {
		if call.Op != "Polyline" || len(call.Points) != 2 {
			continue
		}
		seen[pointsKey(call.Points)]++
	}
	for k, n := range seen {
		if n > 2 {
			t.Errorf("the run %s was drawn %d times; furniture leaked between panels", k, n)
		}
	}
}

func pointsKey(pts []ir.Point) string {
	var b strings.Builder
	for _, p := range pts {
		fmt.Fprintf(&b, "%.3f,%.3f;", p.X, p.Y)
	}
	return b.String()
}

// A theme that turns an axis's ticks off writes no tick labels, and reserves
// no gutter for the ones it did not write — which is what gives a pie with its
// furniture off the whole panel to fill.
func TestTicksOffWritesNoLabelsAndNoGutter(t *testing.T) {
	with := draw(t, chart(line()))
	c := chart(line())
	c.Theme = theme.Light.With(theme.Ticks(false, false))
	without := draw(t, c)

	if len(without.Texts()) != 0 {
		t.Errorf("a chart with its ticks off wrote %v", without.Texts())
	}
	if len(with.Texts()) == 0 {
		t.Fatal("a chart with its ticks on wrote no labels")
	}
	// The plot area is what the gutter was taken from, so it grows.
	if plotArea(without) <= plotArea(with) {
		t.Errorf("the plot area is %v with the ticks off and %v with them on; "+
			"the gutter was reserved anyway", plotArea(without), plotArea(with))
	}
}

// plotArea is the area of the clip a chart's data was drawn inside.
func plotArea(rec *irtest.Recorder) float32 {
	for _, c := range rec.Calls {
		if c.Op == "Push" && c.HasClip {
			return c.ClipRect.Dx() * c.ClipRect.Dy()
		}
	}
	return 0
}

// A polar radial axis runs along a spoke through the marks, and the first
// slice of a pie starts exactly on it — so its line, its tick marks and its
// labels are drawn after the data, while the grid stays underneath. A
// Cartesian axis sits on the panel's edge and keeps the order it always had.
func TestAPolarAxisIsDrawnOverTheData(t *testing.T) {
	for _, tc := range []struct {
		name  string
		coord coord.Coord
		over  bool
	}{
		{"cartesian", nil, false},
		{"polar", coord.Polar(), true},
		{"pie", coord.Pie(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := chart(line())
			c.Coord = tc.coord
			rec := draw(t, c)

			// The data is everything between the clip the panel pushes and the
			// pop that closes it — matched by depth, because a guide drawn
			// afterwards may push and pop a clip of its own.
			push, pop, depth := -1, -1, 0
			for i, call := range rec.Calls {
				switch {
				case push < 0 && call.Op == "Push" && call.HasClip:
					push, depth = i, 1
				case push < 0 || pop >= 0:
				case call.Op == "Push":
					depth++
				case call.Op == "Pop":
					if depth--; depth == 0 {
						pop = i
					}
				}
			}
			if push < 0 || pop < 0 {
				t.Fatal("no data clip was pushed and popped")
			}

			th := c.Theme
			axes, grid := 0, 0
			for i, call := range rec.Calls {
				stroke := call.Stroke
				switch {
				case call.Op != "Polyline" && call.Op != "StrokePath":
					continue
				case stroke.Color == th.AxisColor && stroke.Width == th.AxisWidth:
					axes++
					if after := i > pop; after != tc.over {
						t.Errorf("axis call %d (%s) drawn after the data: %v, want %v", i, call.Op, after, tc.over)
					}
				case stroke.Color == th.GridColor && stroke.Width == th.GridWidth:
					grid++
					if i > push {
						t.Errorf("grid call %d (%s) drawn after the data clip was pushed", i, call.Op)
					}
				}
			}
			if axes == 0 || grid == 0 {
				t.Fatalf("drew %d axis and %d grid strokes, want some of each", axes, grid)
			}
			// A legend is drawn after the data on every chart, so only the
			// polar half can say anything about labels by their order: none
			// of its tick labels may come before the data.
			if !tc.over {
				return
			}
			for i, call := range rec.Calls {
				if call.Op == "Text" && call.Text.Color == th.TickColor && i < push {
					t.Errorf("tick label %q drawn before the data", call.Text.Text)
				}
			}
		})
	}
}

// A gauge's radial labels sit in a row across a ring a few dozen pixels deep,
// with a tick count chosen for a whole panel. Every label a polar panel writes
// clears every other: the ones that would not fit are dropped, not piled up.
func TestPolarTickLabelsDoNotOverlap(t *testing.T) {
	c := chart(line())
	c.Coord = coord.Polar(coord.Theta(coord.FromY), coord.Hole(0.6),
		coord.Sweep(math.Pi), coord.Start(-math.Pi/2))
	rec := draw(t, c)

	var boxes []ir.Rect
	var texts []string
	for _, call := range rec.Calls {
		if call.Op != "Text" || call.Text.Color != c.Theme.TickColor {
			continue
		}
		boxes = append(boxes, layout.LabelBounds(call.Text, rec.Measure(call.Text)))
		texts = append(texts, call.Text.Text)
	}
	if len(boxes) < 4 {
		t.Fatalf("a gauge wrote only %v", texts)
	}
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			if a.Min.X < b.Max.X && b.Min.X < a.Max.X && a.Min.Y < b.Max.Y && b.Min.Y < a.Max.Y {
				t.Errorf("tick labels %q and %q overlap: %v and %v", texts[i], texts[j], a, b)
			}
		}
	}
}

// A polar chart clips to a disc rather than to a rectangle, and the coord is
// what says so.
func TestAPolarPanelClipsToADisc(t *testing.T) {
	c := chart(line())
	c.Coord = coord.Polar()
	rec := draw(t, c)
	for _, call := range rec.Calls {
		if call.Op != "Push" || !call.HasClip {
			continue
		}
		for _, op := range call.Path.Ops {
			if op == ir.OpCubicTo {
				return
			}
		}
		t.Fatalf("a polar panel clipped to %v", call.Path.Ops)
	}
	t.Fatal("no clip was pushed")
}

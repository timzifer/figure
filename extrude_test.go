package figure_test

import (
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// extrudedLive is a chart of four extruded bars under an oblique coord, drawn
// with row tracking on.
func extrudedLive(t *testing.T) *figure.Live {
	t.Helper()
	p := figure.New(figure.Size(600, 300), figure.Coord(coord.Oblique(coord.Depth(0.1))))
	p.X(scale.Linear())
	p.Y(scale.Linear(scale.Zero()))
	p.Add(geom.Bar(keyTable(), geom.X("x"), geom.Y("y"), geom.Extrude(true)))
	live, err := p.Live(irtest.New().Target())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { live.Close() })
	live.TrackRows(true)
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	return live
}

// A rectangle over a whole extruded bar covers its front, its top and its
// side, and has still selected one row.
func TestASelectionOverAnExtrudedBarNamesItsRowOnce(t *testing.T) {
	live := extrudedLive(t)
	panel := live.Index().Panels()[0]
	evs := live.Select(panel.Area)
	if len(evs) != 1 {
		t.Fatalf("%d events, want 1", len(evs))
	}
	seen := map[int]int{}
	for _, r := range evs[0].Rows {
		seen[r]++
	}
	if len(seen) != 4 {
		t.Errorf("rows %v, want all four bars", evs[0].Rows)
	}
	for r, n := range seen {
		if n != 1 {
			t.Errorf("row %d selected %d times; a selection is a set", r, n)
		}
	}
}

// A pointer on the top face of a bar lands on that bar's row, not on nothing.
func TestAPointerOnATopFaceFindsTheBar(t *testing.T) {
	live := extrudedLive(t)
	panel := live.Index().Panels()[0]
	dx, dy := panel.Coord.(coord.Extruder).Extrude()
	// A row is located at its front: the middle of the bar's top edge, where
	// the flat chart would have put it.
	front, ok := live.Index().Locate(0, 0, 2)
	if !ok {
		t.Fatal("row 2 was not located")
	}
	// Half way back along the depth vector from the middle of the bar's top
	// edge is the middle of its top face.
	at := ir.Point{X: front.X + dx/2, Y: front.Y + dy/2}
	ev := live.Move(float64(at.X), float64(at.Y))
	if ev.Hit.Row != 2 {
		t.Errorf("a pointer on the top face of bar 2 hit row %d", ev.Hit.Row)
	}
}

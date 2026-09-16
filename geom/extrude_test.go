package geom_test

import (
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

func quarters() data.Source {
	return data.NewTable().
		Float64("q", []float64{1, 2, 3}).
		Float64("rev", []float64{5, 5, 5})
}

func TestAnExtrudedBarIsThreeFacesInDepthOrder(t *testing.T) {
	g := geom.Bar(quarters(), geom.X("q"), geom.Y("rev"), geom.Extrude(true), geom.Color(palette.Blue))
	rec, f := relFrame(t, g, coord.Oblique(), 400, 300)
	var rows []int
	f.Rows = rowsFunc(func(_ []ir.Point, r []int) { rows = append(rows, r...) })
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}

	fills := rec.Filter("FillPath")
	if len(fills) != 9 {
		t.Fatalf("%d fills, want a side, a top and a front for each of three bars", len(fills))
	}
	// Each bar is side, top, front — and the bars go left to right, because
	// the depth vector points right and the bar to the right covers the side
	// its left neighbour turns towards it.
	var fronts []float32
	for k := 0; k < 9; k += 3 {
		side, top, front := fills[k].Fill.Color, fills[k+1].Fill.Color, fills[k+2].Fill.Color
		if front != palette.Blue {
			t.Errorf("bar %d front is %v, want the mark's colour", k/3, front)
		}
		if lum(top) <= lum(front) || lum(side) >= lum(front) {
			t.Errorf("bar %d: top %v and side %v are not lighter and darker than the front %v", k/3, top, side, front)
		}
		fronts = append(fronts, fills[k+2].Path.Pts[0].X)
	}
	if !(fronts[0] < fronts[1] && fronts[1] < fronts[2]) {
		t.Errorf("fronts drawn at x %v, want left to right", fronts)
	}

	// One row per face, so a pointer on any of them finds its bar.
	count := map[int]int{}
	for _, r := range rows {
		count[r]++
	}
	for row := 0; row < 3; row++ {
		if count[row] != 3 {
			t.Errorf("row %d reported %d times, want once per face", row, count[row])
		}
	}
}

func lum(c ir.Color) int { return int(c.R) + int(c.G) + int(c.B) }

func TestExtrudeUnderACoordWithNoDepthDrawsTheFlatChart(t *testing.T) {
	flat := geom.Bar(quarters(), geom.X("q"), geom.Y("rev"))
	asked := geom.Bar(quarters(), geom.X("q"), geom.Y("rev"), geom.Extrude(true))
	a := build(t, flat, scale.Linear(), scale.Linear())
	b := build(t, asked, scale.Linear(), scale.Linear())
	if len(a.Calls) != len(b.Calls) || a.Count("FillPath") != b.Count("FillPath") {
		t.Errorf("asking for volume under Cartesian changed the drawing: %d calls against %d", len(b.Calls), len(a.Calls))
	}
}

func TestAnExtrudedHeatmapCellIsThreeFacesToo(t *testing.T) {
	src := data.NewTable().
		Float64("x", []float64{1, 2}).
		Float64("y", []float64{1, 1}).
		Float64("v", []float64{1, 2})
	g := geom.Rect(src, geom.X("x"), geom.Y("y"), geom.Extrude(true),
		geom.ColorBy("v", scale.Sequential(palette.Viridis)))
	rec, f := relFrame(t, g, coord.Oblique(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if n := rec.Count("FillPath"); n != 6 {
		t.Errorf("%d fills, want three faces for each of two cells", n)
	}
}

func TestAnExtrudedLayerSurvivesItsDesc(t *testing.T) {
	g := geom.Bar(quarters(), geom.X("q"), geom.Y("rev"), geom.Extrude(true))
	d := g.(geom.Describer).Describe()
	if !d.Extrude {
		t.Fatalf("Desc = %+v", d)
	}
	back, err := geom.FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	if !back.(geom.Describer).Describe().Extrude {
		t.Error("the extrusion did not survive")
	}
}

// A layer that names both a fill and a colour outlines its marks, and an
// extruded one used to lose the outline altogether: the flat path that carried
// it was never built.
func TestAnExtrudedRectKeepsItsOutline(t *testing.T) {
	src := data.NewTable().
		Float64("x", []float64{1, 2}).
		Float64("y", []float64{1, 1})
	g := geom.Rect(src, geom.X("x"), geom.Y("y"), geom.Extrude(true),
		geom.Fill(palette.Blue), geom.Color(palette.Gray))
	rec, f := relFrame(t, g, coord.Oblique(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	strokes := rec.Filter("StrokePath")
	if len(strokes) != 2 {
		t.Fatalf("%d strokes over two extruded cells, want one per mark", len(strokes))
	}
	for i, s := range strokes {
		if s.Stroke.Color != palette.Gray {
			t.Errorf("stroke %d is %v, want the layer's colour", i, s.Stroke.Color)
		}
	}

	// And a layer that asked for no outline still draws none.
	bare := geom.Rect(src, geom.X("x"), geom.Y("y"), geom.Extrude(true),
		geom.Fill(palette.Blue))
	rec2, f2 := relFrame(t, bare, coord.Oblique(), 400, 300)
	if err := bare.Build(rec2, f2); err != nil {
		t.Fatal(err)
	}
	if n := len(rec2.Filter("StrokePath")); n != 0 {
		t.Errorf("%d strokes on a layer that named no colour, want none", n)
	}
}

// The outline is stroked with its own mark, in the same depth order the faces
// are painted in. Stroking every front afterwards would put a back mark's
// outline over a front mark's faces.
func TestAnExtrudedOutlineIsStrokedInTheSameOrderAsTheFaces(t *testing.T) {
	src := data.NewTable().
		Float64("x", []float64{1, 2, 3}).
		Float64("y", []float64{1, 1, 1})
	g := geom.Rect(src, geom.X("x"), geom.Y("y"), geom.Extrude(true),
		geom.Fill(palette.Blue), geom.Color(palette.Gray))
	rec, f := relFrame(t, g, coord.Oblique(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	// Three marks, each of them side, top, front, stroke — so a stroke never
	// follows another stroke and never comes after the last front of all.
	ops := rec.Ops()
	seen, lastStroke := 0, -1
	for i, op := range ops {
		switch op {
		case "FillPath":
			seen++
		case "StrokePath":
			if seen%3 != 0 || seen == 0 {
				t.Errorf("a stroke at call %d follows %d fills, want a whole mark's three", i, seen)
			}
			if lastStroke == i-1 {
				t.Errorf("two strokes in a row at call %d; each mark outlines itself", i)
			}
			lastStroke = i
		}
	}
	if seen != 9 {
		t.Errorf("%d fills over three extruded cells, want three faces each", seen)
	}
	if lastStroke != len(ops)-1 {
		t.Errorf("the last call is %q, want the nearest mark's own outline", ops[len(ops)-1])
	}
}

// An extruded bar keeps its outline too: the rule is the layer's, not the
// mark's, and the fix is in the one place both marks paint through.
func TestAnExtrudedBarKeepsItsOutline(t *testing.T) {
	g := geom.Bar(quarters(), geom.X("q"), geom.Y("rev"), geom.Extrude(true),
		geom.Fill(palette.Blue), geom.Color(palette.Gray))
	rec, f := relFrame(t, g, coord.Oblique(), 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if n := len(rec.Filter("StrokePath")); n != 3 {
		t.Errorf("%d strokes over three extruded bars, want one per mark", n)
	}
}

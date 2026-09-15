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

package geom_test

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// fleet is the table a parallel-coordinates plot reads: several measured
// quantities per row, in units that have nothing to do with each other.
func fleet() data.Source {
	return data.NewTable().
		Float64("mpg", []float64{30, 20, 10, 25}).
		Float64("power", []float64{60, 120, 180, 90}).
		Float64("weight", []float64{900, 1200, 1500, 1050}).
		String("origin", []string{"eu", "us", "us", "eu"})
}

// parallelChart trains a parallel layer in a coord of the named axes and
// returns the recorder and the frame to build it in.
func parallelChart(t *testing.T, g geom.Geom, names ...string) (*irtest.Recorder, geom.Frame) {
	t.Helper()
	dims := make([]coord.ParallelDim, 0, len(names))
	for _, n := range names {
		dims = append(dims, coord.Dim(n, scale.Linear()))
	}
	cd := coord.Parallel(dims...)
	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y, Dims: coord.Scales(coord.Dimensions(cd))}); err != nil {
		t.Fatalf("Train: %v", err)
	}
	area := ir.R(0, 0, 300, 200)
	framed := cd.Frame(coord.Framing{Area: area, X: x, Y: y})
	return irtest.New(), geom.Frame{Area: area, X: x, Y: y, Coord: framed, Theme: theme.Light}
}

// One line per row, each crossing every axis: a row is a line, which is the
// whole of what this mark says.
func TestAParallelLayerDrawsOneLinePerRow(t *testing.T) {
	g := geom.Parallel(fleet(), geom.Dims("mpg", "power", "weight"))
	rec, f := parallelChart(t, g, "mpg", "power", "weight")
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	calls := rec.Filter("StrokePath")
	if len(calls) != 1 {
		t.Fatalf("drew %d paths, want one for one colour", len(calls))
	}
	runs := 0
	for _, op := range calls[0].Path.Ops {
		if op == ir.OpMoveTo {
			runs++
		}
	}
	if runs != 4 {
		t.Errorf("the path holds %d subpaths, want one per row", runs)
	}
}

// Each axis places its own column: the row with the largest weight sits at the
// top of the weight axis and near the bottom of the mpg one, which two axes
// sharing a domain could not both say.
func TestEachAxisPlacesItsOwnColumn(t *testing.T) {
	g := geom.Parallel(fleet(), geom.Dims("mpg", "power", "weight"))
	rec, f := parallelChart(t, g, "mpg", "power", "weight")
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	pts := rowVertices(t, rec, 2) // the third row: 10 mpg, 180 hp, 1500 kg
	if len(pts) != 3 {
		t.Fatalf("the row has %d vertices, want one per axis", len(pts))
	}
	if pts[0].Y <= pts[2].Y {
		t.Errorf("the least economical car is at %v on mpg and %v on weight; the first should be the lower on screen", pts[0].Y, pts[2].Y)
	}
	if math.Abs(float64(pts[2].Y-f.Area.Min.Y)) > 0.01 {
		t.Errorf("the heaviest car is at %v on the weight axis, want the top of the panel", pts[2].Y)
	}
}

// A row missing a value on one axis breaks there: a line drawn straight
// through the gap would assert a value nobody measured.
func TestAMissingValueGapsTheLine(t *testing.T) {
	src := data.NewTable().
		Float64("a", []float64{1, 2}).
		Float64("b", []float64{math.NaN(), 3}).
		Float64("c", []float64{5, 6})
	g := geom.Parallel(src, geom.Dims("a", "b", "c"))
	rec, f := parallelChart(t, g, "a", "b", "c")
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	// The whole row is one subpath, the gapped row none: a run of one point
	// either side of the gap is not a line.
	runs := 0
	for _, op := range rec.Filter("StrokePath")[0].Path.Ops {
		if op == ir.OpMoveTo {
			runs++
		}
	}
	if runs != 1 {
		t.Errorf("drew %d subpaths, want the one row that has every value", runs)
	}
}

// The columns and the coord's axes are matched in order, so a layer that names
// a different number of them is an error rather than a chart with an axis
// nothing is drawn against.
func TestAColumnCountMismatchIsAnError(t *testing.T) {
	g := geom.Parallel(fleet(), geom.Dims("mpg", "power"))
	cd := coord.Parallel(coord.Dim("mpg", scale.Linear()),
		coord.Dim("power", scale.Linear()), coord.Dim("weight", scale.Linear()))
	err := g.Train(geom.Training{
		X: scale.Linear(), Y: scale.Linear(),
		Dims: coord.Scales(coord.Dimensions(cd)),
	})
	if !errors.Is(err, geom.ErrDimensions) {
		t.Errorf("two columns against three axes gave %v, want ErrDimensions", err)
	}
}

// A layer that names no columns says so, and one drawn in a coord with no axes
// of its own — a Cartesian panel — is the same mistake.
func TestAParallelLayerNeedsItsColumnsAndItsAxes(t *testing.T) {
	if err := geom.Parallel(fleet()).Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); !errors.Is(err, geom.ErrNoColumn) {
		t.Errorf("a layer naming no columns gave %v, want ErrNoColumn", err)
	}
	err := geom.Parallel(fleet(), geom.Dims("mpg")).
		Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrDimensions) {
		t.Errorf("a layer in a Cartesian panel gave %v, want ErrDimensions", err)
	}
}

// A row is a line, so the row is reported at every axis it crosses: a pointer
// anywhere along a line finds the same row.
func TestARowIsReportedAtEveryAxis(t *testing.T) {
	g := geom.Parallel(fleet(), geom.Dims("mpg", "power", "weight"))
	rec, f := parallelChart(t, g, "mpg", "power", "weight")
	seen := map[int]int{}
	f.Rows = rowsFunc(func(at []ir.Point, rows []int) {
		for _, r := range rows {
			seen[r]++
		}
		if len(at) != len(rows) {
			t.Errorf("reported %d points against %d rows", len(at), len(rows))
		}
	})
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 4 {
		t.Fatalf("reported %d rows, want four", len(seen))
	}
	for row, n := range seen {
		if n != 3 {
			t.Errorf("row %d was reported %d times, want one per axis", row, n)
		}
	}
}

// A layer coloured by a column paints whole lines, batched by colour: the IR
// carries one style per call, so two categories are two calls however many
// rows there are.
func TestAParallelLayerBatchesByColour(t *testing.T) {
	g := geom.Parallel(fleet(), geom.Dims("mpg", "power", "weight"),
		geom.ColorBy("origin", scale.Qualitative(palette.OkabeIto)))
	rec, f := parallelChart(t, g, "mpg", "power", "weight")
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if got := rec.Count("StrokePath"); got != 2 {
		t.Errorf("drew %d paths for two origins, want two", got)
	}
	entries := geom.Legends(g, f)
	if len(entries) != 2 || entries[0].Label != "eu" {
		t.Errorf("the legend reads %+v, want one entry per origin in first-appearance order", entries)
	}
}

// Redrawn, it draws the same thing: the buffers are reused and the colours
// come out in the order their first row appears rather than a map's.
func TestAParallelLayerRedrawnTwiceDrawsTheSameThing(t *testing.T) {
	g := geom.Parallel(fleet(), geom.Dims("mpg", "power", "weight"),
		geom.ColorBy("origin", scale.Qualitative(palette.OkabeIto)))
	rec, f := parallelChart(t, g, "mpg", "power", "weight")
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	first := rec.String()
	rec.Reset()
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if second := rec.String(); first != second {
		t.Error("a redraw drew something else")
	}
}

// The round trip through Desc rebuilds the same layer, columns and all.
func TestAParallelLayerRoundTripsThroughDescribe(t *testing.T) {
	g := geom.Parallel(fleet(), geom.Dims("mpg", "power", "weight"))
	d, ok := geom.Describe(g)
	if !ok {
		t.Fatal("a parallel layer does not describe itself")
	}
	if len(d.Dims) != 3 || d.Dims[0] != "mpg" {
		t.Fatalf("described as %+v", d.Dims)
	}
	back, err := geom.FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	rec, f := parallelChart(t, g, "mpg", "power", "weight")
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	want := rec.String()

	rec2, f2 := parallelChart(t, back, "mpg", "power", "weight")
	if err := back.Build(rec2, f2); err != nil {
		t.Fatal(err)
	}
	if got := rec2.String(); got != want {
		t.Error("the rebuilt layer drew something else")
	}
}

// rowVertices is the points one row was reported at.
func rowVertices(t *testing.T, rec *irtest.Recorder, row int) []ir.Point {
	t.Helper()
	var out []ir.Point
	for _, c := range rec.Filter("StrokePath") {
		var cur []ir.Point
		var runs [][]ir.Point
		c.Path.Walk(func(op ir.PathOp, pts []ir.Point) {
			if op == ir.OpMoveTo {
				if len(cur) > 0 {
					runs = append(runs, cur)
				}
				cur = nil
			}
			cur = append(cur, pts...)
		})
		if len(cur) > 0 {
			runs = append(runs, cur)
		}
		if row < len(runs) {
			out = runs[row]
		}
	}
	return out
}

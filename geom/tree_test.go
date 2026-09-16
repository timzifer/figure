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

// clustering is a dendrogram's table: four leaves at height zero, merged
// pairwise at 1 and 2, and the root at 5.
func clustering() data.Source {
	return data.NewTable().
		String("node", []string{"root", "ab", "cd", "a", "b", "c", "d"}).
		String("under", []string{"", "root", "root", "ab", "ab", "cd", "cd"}).
		Float64("height", []float64{5, 1, 2, 0, 0, 0, 0})
}

func treeFrame(t *testing.T, g geom.Geom, x scale.Scale, c coord.Coord) (*irtest.Recorder, geom.Frame) {
	t.Helper()
	y := scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("Train: %v", err)
	}
	area := ir.R(0, 0, 400, 300)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)
	if c == nil {
		c = coord.Cartesian()
	}
	c = c.Frame(coord.Framing{Area: area, X: x, Y: y})
	return irtest.New(), geom.Frame{Area: area, X: x, Y: y, Coord: c, Theme: theme.Light}
}

// reported builds the layer and collects where it says each node is.
func reported(t *testing.T, g geom.Geom, rec *irtest.Recorder, f geom.Frame) map[int]ir.Point {
	t.Helper()
	at := map[int]ir.Point{}
	f.Rows = rowsFunc(func(pts []ir.Point, rows []int) {
		for i, r := range rows {
			at[r] = pts[i]
		}
	})
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	return at
}

func TestADendrogramStandsOnItsHeightsWithALeafPerSlot(t *testing.T) {
	g := geom.Tree(clustering(), geom.ID("node"), geom.Parent("under"), geom.Value("height"))
	rec, f := treeFrame(t, g, scale.Linear(), nil)
	if lo, hi := f.Y.Domain(); lo > 0 || hi < 5 {
		t.Errorf("height axis %v..%v does not cover the merge heights 0..5", lo, hi)
	}
	at := reported(t, g, rec, f)
	if len(at) != 7 {
		t.Fatalf("%d nodes reported, want 7", len(at))
	}
	// Leaves a, b, c, d at the quarters' centres, all on the zero line.
	for k, row := range []int{3, 4, 5, 6} {
		want := f.X.Map((float64(k) + 0.5) / 4)
		if p := at[row]; math.Abs(float64(p.X-want)) > 0.01 || p.Y != f.Y.Map(0) {
			t.Errorf("leaf row %d at %v, want x %v on the zero line", row, p, want)
		}
	}
	// A merge sits over the middle of what it merged, at its own height.
	if p := at[1]; math.Abs(float64(p.X-(at[3].X+at[4].X)/2)) > 0.01 || p.Y != f.Y.Map(1) {
		t.Errorf("merge ab at %v", p)
	}

	strokes := rec.Filter("StrokePath")
	if len(strokes) != 1 {
		t.Fatalf("%d strokes, want one for an uncoloured tree", len(strokes))
	}
	branches := 0
	strokes[0].Path.Walk(func(op ir.PathOp, _ []ir.Point) {
		if op == ir.OpMoveTo {
			branches++
		}
	})
	if branches != 6 {
		t.Errorf("%d branches, want one per node that has a parent", branches)
	}
}

func TestAnElbowTurnsAtItsParentsHeightAndAStraightBranchDoesNot(t *testing.T) {
	pointsOf := func(b geom.Branch) int {
		g := geom.Tree(clustering(), geom.ID("node"), geom.Parent("under"), geom.Value("height"), geom.Branches(b))
		rec, f := treeFrame(t, g, scale.Linear(), nil)
		if err := g.Build(rec, f); err != nil {
			t.Fatal(err)
		}
		return len(rec.Filter("StrokePath")[0].Path.Pts)
	}
	elbow, straight := pointsOf(geom.Elbow), pointsOf(geom.Straight)
	if straight != 12 {
		t.Errorf("straight branches have %d points, want two per branch", straight)
	}
	if elbow <= straight {
		t.Errorf("elbows have %d points, want more than the %d of straight branches", elbow, straight)
	}
}

func TestAnOrgChartIsATidyTreeOnItsDepth(t *testing.T) {
	g := geom.Tree(disk(), geom.ID("path"), geom.Parent("under"))
	rec, f := treeFrame(t, g, scale.Linear(), nil)
	if lo, hi := f.Y.Domain(); lo > 0 || hi < 2 {
		t.Errorf("depth axis %v..%v, want it to reach depth 2", lo, hi)
	}
	at := reported(t, g, rec, f)
	if at[0].Y != f.Y.Map(0) || at[3].Y != f.Y.Map(2) {
		t.Errorf("root at %v and a file at %v, want them at depths 0 and 2", at[0], at[3])
	}

	// Turned over, the root is where the deepest node was.
	flipped := geom.Tree(disk(), geom.ID("path"), geom.Parent("under"), geom.Baseline(1))
	rec2, f2 := treeFrame(t, flipped, scale.Linear(), nil)
	if at := reported(t, flipped, rec2, f2); at[0].Y != f2.Y.Map(2) {
		t.Errorf("a turned-over root at %v, want it at the top", at[0])
	}
}

func TestATreeBesideAHeatmapPlacesItsLeavesAtTheirNames(t *testing.T) {
	g := geom.Tree(clustering(), geom.ID("node"), geom.Parent("under"), geom.Value("height"))
	x := scale.Ordinal()
	rec, f := treeFrame(t, g, x, nil)
	cat := x.(scale.Categorical)
	if got := cat.Labels(); len(got) != 4 || got[0] != "a" || got[3] != "d" {
		t.Errorf("the axis learned %v, want the leaves in the tree's order", got)
	}
	at := reported(t, g, rec, f)
	if want := f.X.Map(cat.Encode("c")); at[5].X != want {
		t.Errorf("leaf c at %v, want its category at %v", at[5].X, want)
	}
	if want := (f.X.Map(0) + f.X.Map(1)) / 2; math.Abs(float64(at[1].X-want)) > 0.01 {
		t.Errorf("merge ab at %v, want between its leaves at %v", at[1].X, want)
	}
}

func TestARadialDendrogramBendsItsCrossPieces(t *testing.T) {
	g := geom.Tree(clustering(), geom.ID("node"), geom.Parent("under"), geom.Value("height"), geom.Baseline(1))
	rec, f := treeFrame(t, g, scale.Linear(), coord.Polar())
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	strokes := rec.Filter("StrokePath")
	if len(strokes) != 1 {
		t.Fatalf("%d strokes, want 1", len(strokes))
	}
	for _, p := range strokes[0].Path.Pts {
		if math.IsNaN(float64(p.X)) || math.IsNaN(float64(p.Y)) {
			t.Fatal("a NaN reached the backend")
		}
	}
}

func TestAColouredTreeStrokesARunPerColour(t *testing.T) {
	src := data.NewTable().
		String("node", []string{"root", "ab", "cd", "a", "b", "c", "d"}).
		String("under", []string{"", "root", "root", "ab", "ab", "cd", "cd"}).
		String("clade", []string{"-", "left", "right", "left", "left", "right", "right"})
	g := geom.Tree(src, geom.ID("node"), geom.Parent("under"),
		geom.ColorBy("clade", scale.Qualitative(palette.OkabeIto)))
	rec, f := treeFrame(t, g, scale.Linear(), nil)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	// ab, cd, a, b, c, d: left, right, left, left, right, right — four runs.
	if n := rec.Count("StrokePath"); n != 4 {
		t.Errorf("%d strokes, want one per run of one colour", n)
	}
}

func TestATreeRefusesACycleAndAnOrdinalHeight(t *testing.T) {
	loop := data.NewTable().
		String("node", []string{"a", "b", "c"}).
		String("under", []string{"", "c", "b"})
	g := geom.Tree(loop, geom.ID("node"), geom.Parent("under"))
	if err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); !errors.Is(err, geom.ErrCyclic) {
		t.Errorf("a cycle: err = %v, want ErrCyclic", err)
	}
	h := geom.Tree(disk(), geom.ID("path"), geom.Parent("under"))
	if err := h.Train(geom.Training{X: scale.Linear(), Y: scale.Ordinal()}); !errors.Is(err, geom.ErrNotContinuous) {
		t.Errorf("an ordinal height: err = %v, want ErrNotContinuous", err)
	}
}

func TestATreeSurvivesItsDesc(t *testing.T) {
	g := geom.Tree(clustering(), geom.ID("node"), geom.Parent("under"), geom.Value("height"),
		geom.Branches(geom.Straight), geom.Baseline(1))
	d := g.(geom.Describer).Describe()
	if d.Mark != geom.MarkTree || d.Branch != geom.Straight || d.Baseline != 1 {
		t.Errorf("Desc = %+v", d)
	}
	back, err := geom.FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	if back.(geom.Describer).Describe().Branch != geom.Straight {
		t.Error("the branch shape did not survive")
	}
}

// A horizontal tree reads its breadth up the Y axis and its height across the
// X one, which is what a dendrogram in a left track needs: a band down the
// left side shares the panel's Y, so a tree whose leaves are on X cannot line
// up with a heatmap's rows.
func TestAHorizontalTreeHangsTheSameLayoutOnTheOtherPairOfAxes(t *testing.T) {
	opts := []geom.Option{geom.ID("node"), geom.Parent("under"), geom.Value("height")}
	up := geom.Tree(clustering(), opts...)
	across := geom.Tree(clustering(), append(append([]geom.Option{}, opts...), geom.Orient(geom.Horizontal))...)

	recUp, fUp := treeFrame(t, up, scale.Linear(), nil)
	atUp := reported(t, up, recUp, fUp)
	recAcross, fAcross := treeFrame(t, across, scale.Linear(), nil)
	atAcross := reported(t, across, recAcross, fAcross)

	if len(atUp) != len(atAcross) {
		t.Fatalf("%d nodes up and %d across", len(atUp), len(atAcross))
	}
	// The layout is computed once and hung on the other pair of axes, so the
	// two pictures are each other with the roles of the axes exchanged. The
	// panel is 400 by 300 and Y is flipped, so the comparison is made in the
	// unit square each scale maps into.
	for i, p := range atUp {
		q, ok := atAcross[i]
		if !ok {
			t.Fatalf("node %d is missing from the horizontal tree", i)
		}
		bu, hu := (p.X-fUp.Area.Min.X)/fUp.Area.Dx(), (fUp.Area.Max.Y-p.Y)/fUp.Area.Dy()
		ha, ba := (q.X-fAcross.Area.Min.X)/fAcross.Area.Dx(), (fAcross.Area.Max.Y-q.Y)/fAcross.Area.Dy()
		if math.Abs(float64(bu-ba)) > 1e-4 || math.Abs(float64(hu-ha)) > 1e-4 {
			t.Errorf("node %d: (breadth %v, height %v) up and (breadth %v, height %v) across",
				i, bu, hu, ba, ha)
		}
	}
}

// The height axis is the one the orientation says it is, so a horizontal tree
// refuses an ordinal X rather than an ordinal Y.
func TestAHorizontalTreeRefusesAnOrdinalHeightAxis(t *testing.T) {
	g := geom.Tree(clustering(), geom.ID("node"), geom.Parent("under"), geom.Value("height"),
		geom.Orient(geom.Horizontal))
	err := g.Train(geom.Training{X: scale.Ordinal(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrNotContinuous) {
		t.Errorf("an ordinal height axis: err = %v, want ErrNotContinuous", err)
	}
	// And it takes an ordinal breadth, which is what lines its leaves up with
	// a heatmap's rows.
	ok := geom.Tree(clustering(), geom.ID("node"), geom.Parent("under"), geom.Value("height"),
		geom.Orient(geom.Horizontal))
	if err := ok.Train(geom.Training{X: scale.Linear(), Y: scale.Ordinal()}); err != nil {
		t.Errorf("an ordinal breadth axis: %v", err)
	}
}

// The orientation survives the round trip through a Desc, which is what a
// written-down chart needs.
func TestATreesOrientationIsWrittenDown(t *testing.T) {
	g := geom.Tree(clustering(), geom.ID("node"), geom.Parent("under"),
		geom.Orient(geom.Horizontal))
	d, ok := g.(geom.Describer)
	if !ok {
		t.Fatal("a tree cannot describe itself")
	}
	desc := d.Describe()
	if desc.Orient != geom.Horizontal {
		t.Fatalf("Describe reports orientation %v, want horizontal", desc.Orient)
	}
	back, err := geom.FromDesc(desc)
	if err != nil {
		t.Fatal(err)
	}
	again := back.(geom.Describer).Describe()
	if again.Orient != geom.Horizontal {
		t.Errorf("the rebuilt tree reports orientation %v, want horizontal", again.Orient)
	}
}

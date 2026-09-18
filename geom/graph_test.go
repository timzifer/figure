package geom_test

import (
	"errors"
	"math"
	"sort"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// The layered graph mark. What these tests are about is the seam rather than
// the layout — stat.Layered is a pure function with its own tests. What the
// geom has to get right is that a box is measured and the layout is not, that
// a cycle is drawn rather than refused, that one node is one subpath, and which
// rows it claims to have drawn. See docs/adr/0072-layered-graph-layout.md.

// door is the state machine the tests read: shut → opening → open → closing →
// shut, with one transition that returns and one that stays put.
func door() data.Source {
	return data.NewTable().
		String("state", []string{"shut", "opening", "open", "closing", "open"}).
		String("next", []string{"opening", "open", "closing", "shut", "open"})
}

// boxesIn returns the bounds of every node box: the shapes of the last fill
// call, which is the one the nodes are drawn in.
func boxesIn(t *testing.T, rec *irtest.Recorder) []ir.Rect {
	t.Helper()
	fills := rec.Filter("FillPath")
	if len(fills) == 0 {
		t.Fatal("nothing was filled")
	}
	var out []ir.Rect
	for _, c := range fills {
		for _, s := range subpathsOf(t, c) {
			r := boundsOf(s)
			// An arrowhead is a triangle a few points across; a node's box is
			// never that small once it holds a name.
			if r.Max.X-r.Min.X < 12 {
				continue
			}
			out = append(out, r)
		}
	}
	return out
}

func TestAGraphDrawsABoxPerNode(t *testing.T) {
	g := geom.Graph(door(), geom.From("state"), geom.To("next"))
	rec, f := relFrame(t, g, nil, 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}

	boxes := boxesIn(t, rec)
	if len(boxes) != 4 {
		t.Fatalf("got %d boxes, want one each for shut, opening, open and closing", len(boxes))
	}
	// Four names, four labels, and each one inside the box it belongs to.
	texts := rec.Texts()
	sort.Strings(texts)
	if want := []string{"closing", "open", "opening", "shut"}; !equalStrings(texts, want) {
		t.Errorf("labels %v, want %v", texts, want)
	}
}

// The claim the record is built on: the layout is a pure function of the graph.
// What that means precisely is that the *arrangement* — which node is on which
// rank, and their order across it — does not depend on the backend's font. Where
// that arrangement lands on the panel is fitted to the boxes, exactly as the
// panel itself is fitted around measured axis labels and a measured legend.
func TestAGraphsArrangementDoesNotDependOnTheFont(t *testing.T) {
	small := geom.Graph(door(), geom.From("state"), geom.To("next"), geom.FontSize(8))
	large := geom.Graph(door(), geom.From("state"), geom.To("next"), geom.FontSize(20))

	// The labels in reading order: down the ranks, and left to right across
	// each one. It is the arrangement with the scale divided out.
	arrangement := func(g geom.Geom) []string {
		rec, f := relFrame(t, g, nil, 400, 300)
		if err := g.Build(rec, f); err != nil {
			t.Fatal(err)
		}
		type placed struct {
			text string
			at   ir.Point
		}
		var ps []placed
		for _, c := range rec.Filter("Text") {
			ps = append(ps, placed{c.Text.Text, c.Text.At})
		}
		sort.SliceStable(ps, func(i, j int) bool {
			if math.Abs(float64(ps[i].at.Y-ps[j].at.Y)) > 1 {
				return ps[i].at.Y < ps[j].at.Y
			}
			return ps[i].at.X < ps[j].at.X
		})
		out := make([]string, 0, len(ps))
		for _, p := range ps {
			out = append(out, p.text)
		}
		return out
	}

	a, b := arrangement(small), arrangement(large)
	if !equalStrings(a, b) {
		t.Errorf("the states read %v at 8pt and %v at 20pt; the arrangement should not have moved", a, b)
	}
}

func TestAGraphsBoxGrowsWithItsLabel(t *testing.T) {
	src := data.NewTable().
		String("from", []string{"a", "a"}).
		String("to", []string{"b", "a rather longer name"})

	g := geom.Graph(src, geom.From("from"), geom.To("to"))
	rec, f := relFrame(t, g, nil, 500, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}

	boxes := boxesIn(t, rec)
	if len(boxes) != 3 {
		t.Fatalf("got %d boxes, want three", len(boxes))
	}
	widest := boxes[0]
	for _, r := range boxes {
		if r.Max.X-r.Min.X > widest.Max.X-widest.Min.X {
			widest = r
		}
	}
	narrowest := boxes[0]
	for _, r := range boxes {
		if r.Max.X-r.Min.X < narrowest.Max.X-narrowest.Min.X {
			narrowest = r
		}
	}
	if !(widest.Max.X-widest.Min.X > 2*(narrowest.Max.X-narrowest.Min.X)) {
		t.Errorf("the widest box is %v across and the narrowest %v; a box is measured from its label",
			widest.Max.X-widest.Min.X, narrowest.Max.X-narrowest.Min.X)
	}
}

// Sankey refuses a cycle with ErrCyclic because a flow that returns has no
// column to stand in. A state machine that cannot return is not a state
// machine, so this mark draws one.
func TestAGraphDrawsACycleRatherThanRefusingIt(t *testing.T) {
	g := geom.Graph(door(), geom.From("state"), geom.To("next"))
	rec, f := relFrame(t, g, nil, 400, 300)
	if err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); err != nil {
		t.Fatalf("Train over a cycle: %v, want it drawn", err)
	}
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if rec.Count("StrokePath") == 0 {
		t.Error("nothing was stroked; the transitions should have been drawn")
	}
}

// Every rank at its own height, and rank zero at the bottom of a Cartesian
// panel — the rim = y1 convention ADR 0039 set and this mark keeps.
func TestAGraphStacksItsRanks(t *testing.T) {
	src := data.NewTable().
		String("from", []string{"a", "b", "c"}).
		String("to", []string{"b", "c", "d"})

	g := geom.Graph(src, geom.From("from"), geom.To("to"))
	rec, f := relFrame(t, g, nil, 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}

	boxes := boxesIn(t, rec)
	if len(boxes) != 4 {
		t.Fatalf("got %d boxes, want four", len(boxes))
	}
	ys := make([]float32, 0, len(boxes))
	for _, r := range boxes {
		ys = append(ys, (r.Min.Y+r.Max.Y)/2)
	}
	// Device Y grows downwards, so the first node — rank zero — is the lowest
	// on the screen and so the largest Y.
	for i := 1; i < len(ys); i++ {
		if !(ys[i] < ys[i-1]) {
			t.Errorf("rank %d sits at y = %v and rank %d at y = %v, want each rank above the last",
				i, ys[i], i-1, ys[i-1])
		}
	}
}

// Baseline is what a rankdir option would be in another library: the coord
// decides the shape and this decides which end rank zero is at.
func TestAGraphsBaselineTurnsItOver(t *testing.T) {
	src := data.NewTable().
		String("from", []string{"a", "b"}).
		String("to", []string{"b", "c"})

	up := geom.Graph(src, geom.From("from"), geom.To("to"))
	recUp, fUp := relFrame(t, up, nil, 400, 300)
	if err := up.Build(recUp, fUp); err != nil {
		t.Fatal(err)
	}
	down := geom.Graph(src, geom.From("from"), geom.To("to"), geom.Baseline(1))
	recDown, fDown := relFrame(t, down, nil, 400, 300)
	if err := down.Build(recDown, fDown); err != nil {
		t.Fatal(err)
	}

	first := func(rec *irtest.Recorder) ir.Rect { return boxesIn(t, rec)[0] }
	a, b := first(recUp), first(recDown)
	if !(a.Min.Y > b.Min.Y) {
		t.Errorf("rank zero is at y = %v by default and y = %v with Baseline(1); they should be opposite ends",
			a.Min.Y, b.Min.Y)
	}
}

// A node is what several rows have in common rather than a row of its own, so
// it reports none. The edges are the rows.
func TestAGraphReportsItsEdgesAndNotItsNodes(t *testing.T) {
	g := geom.Graph(door(), geom.From("state"), geom.To("next"))
	rec, f := relFrame(t, g, nil, 400, 300)
	var got []int
	f.Rows = rowsFunc(func(_ []ir.Point, rows []int) { got = append(got, rows...) })
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("reported %d marks for five transitions and four states, want 5", len(got))
	}
	for i, r := range got {
		if r != i {
			t.Errorf("transition %d reports row %d", i, r)
		}
	}
}

// Under a polar coord the ranks are concentric rings. The boxes stay boxes,
// which is the one place this mark parts company with its neighbours: a name
// cannot be set in an annular sector.
func TestAGraphUnderAPolarCoordIsRadial(t *testing.T) {
	src := data.NewTable().
		String("from", []string{"a", "a", "a"}).
		String("to", []string{"b", "c", "d"})

	g := geom.Graph(src, geom.From("from"), geom.To("to"))
	rec, f := relFrame(t, g, coord.Polar(), 400, 400)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}

	boxes := boxesIn(t, rec)
	if len(boxes) != 4 {
		t.Fatalf("got %d boxes, want four", len(boxes))
	}
	// Every box is still a rectangle: four corners, axis-aligned.
	for i, r := range boxes {
		if !(r.Max.X > r.Min.X && r.Max.Y > r.Min.Y) {
			t.Errorf("box %d is %v, want a rectangle", i, r)
		}
	}
	// The three leaves share a rank, so they sit at one radius from the hub.
	mid := ir.Point{X: 200, Y: 200}
	var radii []float64
	for _, r := range boxes[1:] {
		cx, cy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
		radii = append(radii, math.Hypot(float64(cx-mid.X), float64(cy-mid.Y)))
	}
	for i := 1; i < len(radii); i++ {
		if math.Abs(radii[i]-radii[0]) > 1 {
			t.Errorf("leaf %d is %v from the hub and leaf 0 is %v; one rank is one ring",
				i, radii[i], radii[0])
		}
	}
}

// One node is one subpath, which is what makes it separately pointable —
// docs/adr/0015-hit-testing.md.
func TestAGraphDrawsOneSubpathPerNode(t *testing.T) {
	g := geom.Graph(door(), geom.From("state"), geom.To("next"),
		geom.Fill(ir.RGB(0x40, 0x80, 0xc0)))
	rec, f := relFrame(t, g, nil, 400, 300)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	fills := rec.Filter("FillPath")
	// The last fill is the nodes: they are drawn on top so that a pointer on
	// one reports the node rather than an edge passing behind it.
	shapes := subpathsOf(t, fills[len(fills)-1])
	if len(shapes) != 4 {
		t.Errorf("the nodes were drawn as %d subpaths, want one per node", len(shapes))
	}
}

func TestAGraphRefusesAnOrdinalAxis(t *testing.T) {
	g := geom.Graph(door(), geom.From("state"), geom.To("next"))
	err := g.Train(geom.Training{X: scale.Ordinal(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrNotContinuous) {
		t.Errorf("Train onto an ordinal axis returned %v, want ErrNotContinuous", err)
	}
}

func TestAGraphNamesItsNodesInTheLegend(t *testing.T) {
	g := geom.Graph(door(), geom.From("state"), geom.To("next"))
	_, f := relFrame(t, g, nil, 400, 300)
	entries := geom.Legends(g, f)
	if len(entries) != 4 {
		t.Fatalf("got %d legend entries, want one per node", len(entries))
	}
}

func TestAGraphRoundTripsThroughDescribe(t *testing.T) {
	g := geom.Graph(door(), geom.From("state"), geom.To("next"),
		geom.Baseline(1), geom.Branches(geom.Straight))
	d, ok := geom.Describe(g)
	if !ok {
		t.Fatal("a graph does not describe itself")
	}
	if d.Mark != geom.MarkGraph {
		t.Fatalf("Mark = %q, want %q", d.Mark, geom.MarkGraph)
	}
	back, err := geom.FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := geom.Describe(back)
	if !ok {
		t.Fatal("the rebuilt graph does not describe itself")
	}
	if got.Mark != d.Mark || got.Baseline != d.Baseline || got.Branch != d.Branch {
		t.Errorf("rebuilt as %+v, want the baseline and branch back", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

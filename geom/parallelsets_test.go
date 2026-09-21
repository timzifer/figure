package geom_test

import (
	"errors"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

// The parallel-sets mark. What it has to get right is the seam rather than the
// arithmetic — the count has its own tests in package stat: that a ribbon is a
// crossing and a box is a category, that every column of boxes is the same
// table cut a different way, that a colour column subdivides the ribbons, and
// that a count reports no row to a hit test.

// voyage is the table the tests read: six passengers by class, sex and
// whether they survived, so every column holds two categories and the counts
// are small enough to write down.
func voyage() data.Source {
	return data.NewTable().
		String("class", []string{"first", "first", "first", "steerage", "steerage", "steerage"}).
		String("sex", []string{"male", "male", "female", "male", "female", "female"}).
		String("outcome", []string{"died", "lived", "died", "lived", "lived", "lived"})
}

// ribbonsAndBoxes splits what the mark drew into the ribbons — each its own
// fill call, because each is its own colour — and the boxes, which are batched
// by colour into one subpath apiece.
func ribbonsAndBoxes(t *testing.T, rec *irtest.Recorder) (ribbons, boxes int) {
	t.Helper()
	calls := rec.Filter("FillPath")
	if len(calls) == 0 {
		t.Fatal("the layer drew nothing")
	}
	// The boxes go down last, so every call but the batched ones at the end is
	// a ribbon. A box is a rectangle of four points and a ribbon is two cubics,
	// which is what tells the two apart.
	for _, c := range calls {
		for _, s := range subpathsOf(t, c) {
			if len(s) <= 5 {
				boxes++
				continue
			}
			ribbons++
		}
	}
	return ribbons, boxes
}

// One ribbon per crossing and one box per category: the chart is a count, and
// these are the two things it counts.
func TestAParallelSetsLayerDrawsARibbonPerCrossing(t *testing.T) {
	g := geom.ParallelSets(voyage(), geom.Dims("class", "sex", "outcome"))
	rec, f := relFrame(t, g, nil, 400, 250)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	ribbons, boxes := ribbonsAndBoxes(t, rec)
	// Four pairs of class and sex, and four of sex and outcome: every
	// combination of the two happens to be in this table.
	if ribbons != 8 {
		t.Errorf("drew %d ribbons, want one per crossing of neighbouring columns (8)", ribbons)
	}
	if boxes != 6 {
		t.Errorf("drew %d boxes, want one per category in three columns of two", boxes)
	}
}

// Every column is the whole table cut a different way, so the boxes in each
// one cover the same height. It is the property that makes a ribbon's
// thickness mean the same thing anywhere in the diagram.
func TestEveryColumnOfBoxesCoversTheWholePanel(t *testing.T) {
	g := geom.ParallelSets(voyage(), geom.Dims("class", "sex", "outcome"))
	rec, f := relFrame(t, g, nil, 400, 250)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	// Boxes are the small subpaths, and a column is the ones sharing an X.
	heights := map[float32]float32{}
	for _, c := range rec.Filter("FillPath") {
		for _, s := range subpathsOf(t, c) {
			if len(s) > 5 {
				continue
			}
			r := boundsOf(s)
			heights[r.Min.X] += r.Max.Y - r.Min.Y
		}
	}
	if len(heights) != 3 {
		t.Fatalf("the boxes stand in %d columns, want one per dimension", len(heights))
	}
	for x, h := range heights {
		if want := float32(250.0); h < want*0.98 || h > want {
			t.Errorf("the column at x=%v covers %v of %v", x, h, want)
		}
	}
}

// A colour column subdivides the ribbons rather than blending into them: the
// rows that disagree about it are drawn apart, which is what lets a reader
// follow one class the length of the diagram.
func TestAColourColumnSubdividesTheRibbons(t *testing.T) {
	plain := geom.ParallelSets(voyage(), geom.Dims("class", "sex"))
	rec, f := relFrame(t, plain, nil, 400, 250)
	if err := plain.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	before, _ := ribbonsAndBoxes(t, rec)

	split := geom.ParallelSets(voyage(), geom.Dims("class", "sex"),
		geom.ColorBy("outcome", scale.Qualitative(palette.OkabeIto)))
	rec, f = relFrame(t, split, nil, 400, 250)
	if err := split.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	after, _ := ribbonsAndBoxes(t, rec)

	// first→male is two rows that disagree about the outcome, and nothing else
	// in this table does.
	if after != before+1 {
		t.Errorf("colouring by the outcome drew %d ribbons against %d, want one more", after, before)
	}
}

// A ribbon is a count over many rows, so there is no single row behind it and
// none is reported. The same is already true of a sankey's node and an UpSet's
// bars — see docs/adr/0015-hit-testing.md.
func TestAParallelSetsLayerReportsNoRows(t *testing.T) {
	g := geom.ParallelSets(voyage(), geom.Dims("class", "sex", "outcome"))
	rec, f := relFrame(t, g, nil, 400, 250)
	rows := &rowSink{}
	f.Rows = rows
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if len(rows.rows) != 0 {
		t.Errorf("reported %d rows, want none: a count is not a row", len(rows.rows))
	}
}

// A row missing a value in one dimension is counted in none of them, and every
// column still adds up to the same total. It is the one place in this library
// where an absent value costs a row, and what it buys is the property the
// chart is read by.
func TestARowMissingACategoryLeavesEveryColumnAddingUp(t *testing.T) {
	// The middle column is missing the second row's value, so only the first
	// and third are counted — and "x" is one row thick rather than two.
	src := data.NewTable().
		String("a", []string{"x", "x", "y"}).
		String("b", []string{"p", "", "q"}).
		String("c", []string{"u", "u", "v"})
	g := geom.ParallelSets(src, geom.Dims("a", "b", "c"))
	rec, f := relFrame(t, g, nil, 400, 250)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	ribbons, boxes := ribbonsAndBoxes(t, rec)
	if ribbons != 4 {
		t.Errorf("drew %d ribbons, want the four the two complete rows make", ribbons)
	}
	if boxes != 6 {
		t.Errorf("drew %d boxes, want the six categories the table holds", boxes)
	}
	heights := map[float32]float32{}
	for _, c := range rec.Filter("FillPath") {
		for _, sp := range subpathsOf(t, c) {
			if len(sp) > 5 {
				continue
			}
			r := boundsOf(sp)
			heights[r.Min.X] += r.Max.Y - r.Min.Y
		}
	}
	for x, h := range heights {
		if h < 249 || h > 250 {
			t.Errorf("the column at x=%v covers %v of 250: the columns no longer agree", x, h)
		}
	}
}

// The mark says what it needs rather than drawing something nobody asked for:
// a crossing is between two columns, so one column is not a chart.
func TestAParallelSetsLayerNeedsTwoColumns(t *testing.T) {
	g := geom.ParallelSets(voyage(), geom.Dims("class"))
	if err := trainErr(g); !errors.Is(err, geom.ErrDimensions) {
		t.Errorf("one dimension gave %v, want ErrDimensions", err)
	}
	g = geom.ParallelSets(voyage())
	if err := trainErr(g); !errors.Is(err, geom.ErrNoColumn) {
		t.Errorf("no dimensions gave %v, want ErrNoColumn", err)
	}
	g = geom.ParallelSets(voyage(), geom.Dims("class", "cabin"))
	if err := trainErr(g); !errors.Is(err, geom.ErrNoColumn) {
		t.Errorf("a column that is not there gave %v, want ErrNoColumn", err)
	}
}

// trainErr trains a layer on a pair of linear scales and hands back what it
// said, which is where every one of this mark's refusals is made.
func trainErr(g geom.Geom) error {
	return g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
}

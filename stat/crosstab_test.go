package stat_test

import (
	"math"
	"slices"
	"testing"

	"github.com/timzifer/figure/stat"
)

// passengers is the table every test here reads, as three columns of category
// indices: class 0 or 1, sex 0 or 1, survived 0 or 1.
//
//	class  sex  survived
//	  0     0      0
//	  0     0      1
//	  0     1      0
//	  1     0      1
//	  1     1      1
//	  1     1      1
func passengers() [][]int {
	return [][]int{
		{0, 0, 0, 1, 1, 1},
		{2, 2, 3, 2, 3, 3},
		{4, 5, 4, 5, 5, 5},
	}
}

// crossings is the answer as a list of (from, to, class, value) rows, which is
// what most of these tests compare.
type crossing struct {
	from, to, class int
	value           float64
}

func crossingsOf(x *stat.Crosstab) []crossing {
	out := make([]crossing, 0, len(x.From))
	for i := range x.From {
		out = append(out, crossing{x.From[i], x.To[i], x.Class[i], x.Value[i]})
	}
	return out
}

// The count is the whole of the chart: one crossing per pair of categories
// neighbouring columns hold, carrying how many rows hold both.
func TestACrosstabCountsEveryNeighbouringPair(t *testing.T) {
	var x stat.Crosstab
	x.Reset(passengers(), nil, nil)

	want := []crossing{
		{0, 2, -1, 2}, // class 0 with sex 0: two rows
		{0, 3, -1, 1},
		{1, 2, -1, 1},
		{1, 3, -1, 2},
		{2, 4, -1, 1}, // sex 0 with died: one row
		{2, 5, -1, 2},
		{3, 4, -1, 1},
		{3, 5, -1, 2},
	}
	if got := crossingsOf(&x); !slices.Equal(got, want) {
		t.Errorf("counted\n%v\nwant\n%v", got, want)
	}
}

// Every column is the same table cut a different way, so the counts across
// each gap add up to the same total. It is the reading the chart rests on: a
// ribbon's thickness means the same thing anywhere in the diagram.
func TestEveryGapCountsTheWholeTable(t *testing.T) {
	cats := passengers()
	var x stat.Crosstab
	x.Reset(cats, nil, nil)

	gaps := make([]float64, len(cats)-1)
	for i := range x.From {
		// A crossing's gap is the column its far end stands in, and the
		// columns here are 0-1, 2-3 and 4-5.
		gaps[x.To[i]/2-1] += x.Value[i]
	}
	for d, sum := range gaps {
		if sum != 6 {
			t.Errorf("gap %d counts %v of six rows", d, sum)
		}
	}
}

// The order is the crossings' own — by the category they leave, then the one
// they enter — and not the table's, so shuffling the rows cannot move a ribbon.
func TestACrosstabsOrderDoesNotDependOnTheRowOrder(t *testing.T) {
	forwards := passengers()
	backwards := make([][]int, len(forwards))
	for d, col := range forwards {
		backwards[d] = slices.Clone(col)
		slices.Reverse(backwards[d])
	}

	var a, b stat.Crosstab
	a.Reset(forwards, nil, nil)
	b.Reset(backwards, nil, nil)
	if got, want := crossingsOf(&b), crossingsOf(&a); !slices.Equal(got, want) {
		t.Errorf("the reversed table counted\n%v\nwant\n%v", got, want)
	}
	if !slices.IsSortedFunc(a.From, func(x, y int) int { return x - y }) {
		t.Errorf("crossings came out in %v, which is not the order they leave in", a.From)
	}
}

// A class subdivides a crossing rather than blending into it: two rows that
// agree on both categories and disagree on the class are two ribbons, because
// the colour that tells them apart has to have something of its own to paint.
func TestAClassSubdividesACrossing(t *testing.T) {
	cats := [][]int{
		{0, 0, 0},
		{1, 1, 1},
	}
	var x stat.Crosstab
	x.Reset(cats, []int{0, 1, 0}, nil)

	want := []crossing{{0, 1, 0, 2}, {0, 1, 1, 1}}
	if got := crossingsOf(&x); !slices.Equal(got, want) {
		t.Errorf("counted\n%v\nwant\n%v", got, want)
	}
}

// A row whose class is missing keeps its ribbon under class −1. Dropping it
// would take rows out of a chart that counts rows, and the colour scale has an
// answer for a value it has no category for.
func TestARowWithNoClassIsStillCounted(t *testing.T) {
	cats := [][]int{{0, 0}, {1, 1}}
	var x stat.Crosstab
	x.Reset(cats, []int{-1, 0}, nil)

	if got := crossingsOf(&x); !slices.Equal(got, []crossing{{0, 1, -1, 1}, {0, 1, 0, 1}}) {
		t.Errorf("counted %v, want the unclassed row under class -1", got)
	}
}

// A row missing a category anywhere is counted nowhere, because what it would
// be missing from is a partition: counting it in the gaps it does have would
// leave one column adding up to less than the next, and then no two
// thicknesses in the diagram would mean the same thing.
func TestARowMissingACategoryIsCountedNowhere(t *testing.T) {
	cats := [][]int{
		{0, 0},
		{1, -1},
		{2, 2},
	}
	var x stat.Crosstab
	x.Reset(cats, nil, nil)

	if got := crossingsOf(&x); !slices.Equal(got, []crossing{{0, 1, -1, 1}, {1, 2, -1, 1}}) {
		t.Errorf("counted %v, want only the complete row", got)
	}
	// Both gaps count the one complete row, which is the property the chart is
	// read by.
	for _, v := range x.Value {
		if v != 1 {
			t.Errorf("a gap counts %v rows, want the one complete row", v)
		}
	}
}

// Weights are what a pre-counted table carries, and a row weighing nothing is
// a ribbon nobody can be shown.
func TestACrosstabSumsWeightsAndSkipsTheEmptyOnes(t *testing.T) {
	cats := [][]int{{0, 0, 0}, {1, 1, 1}}
	var x stat.Crosstab
	x.Reset(cats, nil, []float64{2.5, 1.5, 0})

	if got := crossingsOf(&x); !slices.Equal(got, []crossing{{0, 1, -1, 4}}) {
		t.Errorf("counted %v, want one crossing of four", got)
	}
}

// Fewer than two columns is not a crossing, and a column list read alongside a
// shorter one counts what both have.
func TestACrosstabReadsWhatEveryColumnHas(t *testing.T) {
	var x stat.Crosstab
	x.Reset([][]int{{0, 0}}, nil, nil)
	if len(x.From) != 0 {
		t.Errorf("one column counted %d crossings, want none", len(x.From))
	}
	x.Reset([][]int{{0, 0, 0}, {1, 1}}, nil, nil)
	if got := crossingsOf(&x); !slices.Equal(got, []crossing{{0, 1, -1, 2}}) {
		t.Errorf("counted %v, want the two rows both columns have", got)
	}
}

// The columns a parallel-sets diagram stands in are given rather than derived,
// and this is the case that makes the difference: "steerage" is reached by
// nothing, because every row that mentions it is missing the column before.
// The longest path to it is zero, so a derived layout stands it among the
// sources — in the wrong column of the chart. See
// docs/adr/0079-parallel-sets.md.
func TestPinnedColumnsHoldWhereDerivedOnesWouldNot(t *testing.T) {
	// Nodes: 0 and 1 in column zero, 2 and 3 in column one, and only 2 has an
	// edge coming into it.
	from := []int{0, 2}
	to := []int{2, 3}
	value := []float64{1, 1}

	var derived, pinned stat.Sankey
	derived.Reset(from, to, value, 4, 0)
	if got := derived.Nodes[3].Layer; got != 2 {
		t.Fatalf("the derived layout put node 3 in column %d, want the two-step path's 2", got)
	}

	pinned.ResetColumns(from, to, value, []int{0, 0, 1, 1}, 4, 0)
	if pinned.Layers != 2 {
		t.Errorf("the pinned layout has %d columns, want the two it was given", pinned.Layers)
	}
	for i, want := range []int{0, 0, 1, 1} {
		if got := pinned.Nodes[i].Layer; got != want {
			t.Errorf("node %d stands in column %d, want %d", i, got, want)
		}
	}
	if pinned.Cyclic {
		t.Error("a layout whose columns are given has no cycle to find")
	}
}

// Pinning the columns changes where the nodes stand and nothing else: the
// bands are the same bands, stacked against the same edges.
func TestPinnedColumnsLayOutLikeDerivedOnes(t *testing.T) {
	from := []int{0, 0, 1, 2}
	to := []int{1, 2, 3, 3}
	value := []float64{3, 1, 3, 1}

	var derived, pinned stat.Sankey
	derived.Reset(from, to, value, 4, 0.02)
	pinned.ResetColumns(from, to, value, []int{0, 1, 1, 2}, 4, 0.02)

	for i := range derived.Nodes {
		if a, b := derived.Nodes[i], pinned.Nodes[i]; a != b {
			t.Errorf("node %d: derived %v, pinned %v", i, a, b)
		}
	}
	for i := range derived.Flows {
		if a, b := derived.Flows[i], pinned.Flows[i]; a != b {
			t.Errorf("flow %d: derived %v, pinned %v", i, a, b)
		}
	}
}

// A column nothing names stands at the left, and a layout is still a layout:
// the point is that a caller's mistake draws a chart rather than a NaN.
func TestAnUnnamedColumnStandsAtTheLeft(t *testing.T) {
	var s stat.Sankey
	s.ResetColumns([]int{0}, []int{1}, []float64{1}, []int{-3}, 2, 0)
	if got := s.Nodes[0].Layer; got != 0 {
		t.Errorf("a negative column became %d, want 0", got)
	}
	for _, n := range s.Nodes {
		if math.IsNaN(n.Lo) || math.IsNaN(n.Hi) {
			t.Errorf("node %v holds a NaN", n)
		}
	}
}

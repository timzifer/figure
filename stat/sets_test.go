package stat_test

import (
	"testing"

	"github.com/timzifer/figure/stat"
)

// membership is the worked example the tests read: five elements over three
// sets, written as the (element, set) pairs a table holds.
//
//	0: {A}        1: {A, B}     2: {A, B}
//	3: {B, C}     4: {C}
func membership() (elem, set []int) {
	return []int{0, 1, 1, 2, 2, 3, 3, 4},
		[]int{0, 0, 1, 0, 1, 1, 2, 2}
}

// The counts are of elements in *exactly* a combination, so they partition the
// elements rather than double-counting the ones in two sets.
func TestIntersectionsCountEachElementOnce(t *testing.T) {
	var x stat.Intersections
	elem, set := membership()
	x.Reset(elem, set, 5, 3)

	if x.Elements != 5 {
		t.Errorf("counted %d elements, want five", x.Elements)
	}
	total := 0
	for _, c := range x.Combinations {
		total += c.Count
	}
	if total != x.Elements {
		t.Errorf("the combinations hold %d elements between them and there are %d", total, x.Elements)
	}

	want := []struct {
		sets   []int
		count  int
		degree int
	}{
		{[]int{0}, 1, 1},    // element 0
		{[]int{0, 1}, 2, 2}, // elements 1 and 2
		{[]int{1, 2}, 1, 2}, // element 3
		{[]int{2}, 1, 1},    // element 4
	}
	if len(x.Combinations) != len(want) {
		t.Fatalf("found %d combinations, want %d: %+v", len(x.Combinations), len(want), x.Combinations)
	}
	for i, w := range want {
		got := x.Combinations[i]
		if got.Count != w.count || got.Degree != w.degree {
			t.Errorf("combination %d is %+v, want count %d degree %d", i, got, w.count, w.degree)
		}
		for _, s := range w.sets {
			if !got.Has(s) {
				t.Errorf("combination %d does not hold set %d: %+v", i, s, got)
			}
		}
	}
}

// A set's own size counts every element in it, so the sizes are larger than the
// element count wherever the sets overlap. Both readings are on an UpSet plot
// and they are not the same number.
func TestSetSizesCountEveryMembership(t *testing.T) {
	var x stat.Intersections
	elem, set := membership()
	x.Reset(elem, set, 5, 3)

	if got, want := x.Sizes, []int{3, 3, 2}; len(got) != len(want) {
		t.Fatalf("sizes %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("set %d holds %d elements, want %d", i, got[i], want[i])
			}
		}
	}
}

// The combinations come out in the order the elements first named them, which
// is the caller's row order. Nothing here reads a map's iteration order.
func TestIntersectionsAreDeterministic(t *testing.T) {
	elem, set := membership()
	var first, second stat.Intersections
	for range 8 {
		first.Reset(elem, set, 5, 3)
		second.Reset(elem, set, 5, 3)
		if len(first.Combinations) != len(second.Combinations) {
			t.Fatalf("two runs found %d and %d combinations", len(first.Combinations), len(second.Combinations))
		}
		for i := range first.Combinations {
			if first.Combinations[i] != second.Combinations[i] {
				t.Fatalf("combination %d differs between runs: %+v and %+v",
					i, first.Combinations[i], second.Combinations[i])
			}
		}
	}
}

// A membership named twice is one membership: a set holds an element or it does
// not, and a duplicated row would otherwise make a set look bigger.
func TestARepeatedMembershipIsCountedOnce(t *testing.T) {
	var x stat.Intersections
	x.Reset([]int{0, 0, 0}, []int{0, 0, 1}, 1, 2)

	if len(x.Combinations) != 1 || x.Combinations[0].Degree != 2 || x.Combinations[0].Count != 1 {
		t.Errorf("a repeated membership gave %+v, want one element in both sets", x.Combinations)
	}
	if x.Sizes[0] != 1 {
		t.Errorf("set 0 holds %d elements, want one", x.Sizes[0])
	}
}

// Disjoint sets produce one combination each and no crossing column, which is
// the picture a reader should get: nothing overlaps, so there is nothing to
// read in the middle.
func TestDisjointSetsShareNoCombination(t *testing.T) {
	var x stat.Intersections
	x.Reset([]int{0, 1, 2}, []int{0, 1, 2}, 3, 3)

	if len(x.Combinations) != 3 {
		t.Fatalf("three disjoint sets gave %d combinations, want three", len(x.Combinations))
	}
	for i, c := range x.Combinations {
		if c.Degree != 1 || c.Count != 1 {
			t.Errorf("combination %d is %+v, want one element of degree one", i, c)
		}
	}
}

// A row naming an element or a set that does not exist is skipped rather than
// guessed at, and an element nobody mentioned is in no combination.
func TestOutOfRangeMembershipsAreSkipped(t *testing.T) {
	var x stat.Intersections
	x.Reset([]int{0, 5, -1, 1}, []int{0, 0, 0, 9}, 3, 2)

	if x.Elements != 1 {
		t.Errorf("counted %d elements, want one — the other rows name nothing", x.Elements)
	}
	if len(x.Combinations) != 1 || !x.Combinations[0].Has(0) {
		t.Errorf("combinations %+v, want the one element in set 0", x.Combinations)
	}
}

// More sets than a mask holds is refused rather than truncated: a chart drawn
// from the first sixty-four of them would be a chart about data nobody chose.
func TestTooManySetsCountsNothing(t *testing.T) {
	var x stat.Intersections
	x.Reset([]int{0}, []int{0}, 1, stat.MaxSets+1)

	if !x.TooMany {
		t.Error("counting more sets than a mask holds did not report it")
	}
	if len(x.Combinations) != 0 {
		t.Errorf("it counted %d combinations anyway", len(x.Combinations))
	}
}

// CountOf answers for a combination nothing is in, which is what a Venn's empty
// region needs: the region is drawn either way and says zero.
func TestCountOfAnEmptyCombinationIsZero(t *testing.T) {
	var x stat.Intersections
	elem, set := membership()
	x.Reset(elem, set, 5, 3)

	if got := x.CountOf(1 << 0); got != 1 {
		t.Errorf("elements in set A alone: %d, want 1", got)
	}
	if got := x.CountOf(1<<0 | 1<<2); got != 0 {
		t.Errorf("elements in exactly A and C: %d, want 0 — nothing is", got)
	}
}

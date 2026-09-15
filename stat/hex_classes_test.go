package stat_test

import (
	"testing"

	"github.com/timzifer/figure/stat"
)

func TestAHexCountsItsClassesAndNamesTheDominantOne(t *testing.T) {
	var h stat.Hex
	h.Reset(10, 0, 0, 100, 100)
	h.CountClasses(3)
	for _, c := range []int{0, 2, 2, 1, 2} {
		h.AddClass(50, 50, c)
	}
	h.AddClass(50, 50, 7) // a class out of range counts in the total only
	cells := h.Cells(nil)
	if len(cells) != 1 {
		t.Fatalf("%d cells, want 1", len(cells))
	}
	c := cells[0]
	if c.Count != 6 || len(c.ByClass) != 3 || c.ByClass[2] != 3 {
		t.Fatalf("cell %+v", c)
	}
	class, purity := c.Dominant()
	if class != 2 || purity != 0.5 {
		t.Errorf("Dominant = %d, %v; want class 2 at 3 of 6", class, purity)
	}

	// A tie goes to the lower class.
	h.Reset(10, 0, 0, 100, 100)
	h.CountClasses(2)
	h.AddClass(50, 50, 1)
	h.AddClass(50, 50, 0)
	if k, p := h.Cells(nil)[0].Dominant(); k != 0 || p != 0.5 {
		t.Errorf("a tie went to class %d at %v, want class 0", k, p)
	}

	// Reset forgets the classes.
	h.Reset(10, 0, 0, 100, 100)
	h.Add(50, 50)
	if cells := h.Cells(nil); cells[0].ByClass != nil || h.Classes != 0 {
		t.Errorf("a reset lattice still counts classes: %+v", cells[0])
	}
}

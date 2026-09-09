package data_test

import (
	"testing"

	"github.com/timzifer/figure/data"
)

// The reason the kind exists. A float64 counts exactly to 2^53 and no further,
// so two ids past that point become one number — and a chart that groups,
// facets or keys by that column then draws two rows as one.
func TestAnIdentifierPast2To53StaysExact(t *testing.T) {
	var a, b int64 = 9007199254740992, 9007199254740993 // 2^53 and 2^53+1
	if float64(a) == float64(b) {
		// Not an assertion about figure: it is the premise, stated so that a
		// reader of this test does not have to take it on trust.
		t.Log("as float64 these two ids are the same number")
	} else {
		t.Fatal("the premise of this test no longer holds")
	}

	tab := data.NewTable().Int64("id", []int64{a, b})
	labels, ok := data.Labels(tab, "id")
	if !ok {
		t.Fatal("no labels for an exact integer column")
	}
	if labels[0] == labels[1] {
		t.Errorf("two ids spell the same: %q", labels[0])
	}
	if labels[0] != "9007199254740992" || labels[1] != "9007199254740993" {
		t.Errorf("labels are %q, want the digits as written", labels)
	}
}

// A position is allowed to lose what a pixel cannot show. That is the trade
// the kind is built on, and it is only honest if it is the trade actually
// made: the numbers convert, the spelling does not.
func TestAnExactColumnStillPlots(t *testing.T) {
	tab := data.NewTable().Int64("n", []int64{1, 2, 3})
	v, ok := data.Float64Column(tab, "n")
	if !ok {
		t.Fatal("an exact integer column has no numbers")
	}
	if len(v) != 3 || v[0] != 1 || v[2] != 3 {
		t.Errorf("numbers are %v, want 1 2 3", v)
	}
	if _, ok := data.StringColumn(tab, "n"); ok {
		t.Error("an exact integer column answered StringColumn; it is not text")
	}
}

// A spelling the source supplies wins over the default one, which is what lets
// a column carry values figure has no type for — a physical quantity, a
// currency, an enum with names — and still be labelled, grouped and keyed.
func TestASuppliedSpellingIsUsed(t *testing.T) {
	tab := data.NewTable().
		Float64("p", []float64{2.5, 3}).
		WithText("p", func(i int) string { return []string{"2.5 bar", "3 bar"}[i] })

	if got, _ := data.Label(tab, "p", 0); got != "2.5 bar" {
		t.Errorf("row 0 spells %q, want the source's own spelling", got)
	}
	labels, _ := data.Labels(tab, "p")
	if labels[1] != "3 bar" {
		t.Errorf("labels are %q, want the source's own spellings", labels)
	}
	// The values are untouched: a spelling says how a row reads, not where it
	// is drawn.
	v, _ := data.Float64Column(tab, "p")
	if v[0] != 2.5 {
		t.Errorf("the numbers changed: %v", v)
	}
}

// A cut keeps the spelling and the mask, which is the property that made the
// column one value rather than three questions: a facet's panel used to lose
// whichever of them its Source forgot to forward.
func TestACutKeepsSpellingAndNulls(t *testing.T) {
	tab := data.NewTable().
		String("s", []string{"a", "b", "c"}).
		WithNulls("s", []bool{false, true, false}).
		WithText("s", func(i int) string { return "row" + string(rune('0'+i)) })

	cut := data.Rows(tab, []int{2, 1})
	got, _ := data.Label(cut, "s", 0)
	if got != "row2" {
		t.Errorf("the cut spells row 0 as %q, want the parent's spelling of the row it came from", got)
	}
	mask, ok := data.NullMask(cut, "s")
	if !ok {
		t.Fatal("the cut lost the null mask")
	}
	if mask[0] || !mask[1] {
		t.Errorf("the mask is %v, want the parent's rows 2 and 1 in that order", mask)
	}
}

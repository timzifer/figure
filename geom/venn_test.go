package geom_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

// Every region carries a number, and the numbers add up to how many elements
// there are: a Venn drawn this way partitions the elements instead of counting
// the overlapping ones twice, which is the usual way it is drawn wrong.
func TestAVennWritesEveryRegionAndTheyAddUp(t *testing.T) {
	g := geom.Venn(subscriptions(), geom.From("who"), geom.To("what"))
	rec, f := upsetFrame(t, g, scale.Linear(), scale.Linear(), 400, 400)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}

	total, regions := 0, 0
	for _, s := range rec.Texts() {
		n, err := strconv.Atoi(s)
		if err != nil {
			continue // a set's own name
		}
		total, regions = total+n, regions+1
	}
	if regions != 7 {
		t.Errorf("wrote %d numbers, want one per region of three sets", regions)
	}
	if total != 10 {
		t.Errorf("the regions hold %d elements between them, and the table has ten customers", total)
	}
}

// Each set is named beside its own circle, so the diagram reads without the
// legend beside it.
func TestAVennNamesItsSets(t *testing.T) {
	g := geom.Venn(subscriptions(), geom.From("who"), geom.To("what"))
	rec, f := upsetFrame(t, g, scale.Linear(), scale.Linear(), 400, 400)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"mail", "drive", "chat"} {
		found := false
		for _, s := range rec.Texts() {
			found = found || s == want
		}
		if !found {
			t.Errorf("the diagram never names %q", want)
		}
	}
}

// One filled disc and one outline per set, each a closed run through the coord.
func TestAVennDrawsOneDiscPerSet(t *testing.T) {
	g := geom.Venn(subscriptions(), geom.From("who"), geom.To("what"))
	rec, f := upsetFrame(t, g, scale.Linear(), scale.Linear(), 400, 400)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if got := rec.Count("FillPath"); got != 3 {
		t.Errorf("filled %d discs, want one per set", got)
	}
	if got := rec.Count("StrokePath"); got != 3 {
		t.Errorf("outlined %d discs, want one per set", got)
	}
}

// A fourth set is refused rather than drawn. Four circles have no arrangement
// whose regions are all there, so a picture of four would be a picture that
// leaves combinations out without saying which.
func TestAVennRefusesAFourthSet(t *testing.T) {
	src := data.NewTable().
		String("who", []string{"a", "a", "a", "a"}).
		String("what", []string{"p", "q", "r", "s"})
	g := geom.Venn(src, geom.From("who"), geom.To("what"))
	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrTooManySets) {
		t.Errorf("four sets gave %v, want ErrTooManySets", err)
	}
}

// It places its own geometry in the unit square, so an axis with slots is
// refused the way every other such mark refuses one.
func TestAVennRefusesAnOrdinalAxis(t *testing.T) {
	g := geom.Venn(subscriptions(), geom.From("who"), geom.To("what"))
	err := g.Train(geom.Training{X: scale.Ordinal(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrNotContinuous) {
		t.Errorf("an ordinal axis gave %v, want ErrNotContinuous", err)
	}
}

// Both axes describe the unit square, which is the honest answer: a Venn's
// coordinates are a picture's rather than a quantity's.
func TestAVennTrainsTheUnitSquare(t *testing.T) {
	x, y := scale.Linear(), scale.Linear()
	g := geom.Venn(subscriptions(), geom.From("who"), geom.To("what"))
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []scale.Scale{x, y} {
		if lo, hi := s.Domain(); lo != 0 || hi != 1 {
			t.Errorf("an axis runs %v..%v, want the unit interval", lo, hi)
		}
	}
}

// The sets are the reading, so each one is a legend entry, in the order the
// table named them — the relational marks' rule.
func TestAVennNamesItsSetsInTheLegend(t *testing.T) {
	g := geom.Venn(subscriptions(), geom.From("who"), geom.To("what"))
	_, f := upsetFrame(t, g, scale.Linear(), scale.Linear(), 400, 400)
	entries := geom.Legends(g, f)
	if len(entries) != 3 {
		t.Fatalf("got %d legend entries, want one per set", len(entries))
	}
	for i, want := range []string{"mail", "drive", "chat"} {
		if entries[i].Label != want {
			t.Errorf("entry %d is %q, want %q", i, entries[i].Label, want)
		}
	}
}

// Two sets are two circles side by side, which is the other diagram this mark
// draws and the one whose regions are easiest to get wrong.
func TestATwoSetVennWritesThreeRegions(t *testing.T) {
	src := data.NewTable().
		String("who", []string{"a", "a", "b", "c", "c", "d", "e"}).
		String("what", []string{"mail", "drive", "mail", "mail", "drive", "drive", "drive"})
	g := geom.Venn(src, geom.From("who"), geom.To("what"))
	rec, f := upsetFrame(t, g, scale.Linear(), scale.Linear(), 400, 400)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, s := range rec.Texts() {
		counts[s]++
	}
	// mail alone: b. both: a and c. drive alone: d and e.
	if counts["1"] != 1 || counts["2"] != 2 {
		t.Errorf("the regions read %v, want one 1 and two 2s", counts)
	}
}

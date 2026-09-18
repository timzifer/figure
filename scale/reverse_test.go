package scale_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/scale"
)

// A reversed axis draws the low end of its domain at the high end of its range.
// What these check is that nothing else in the scale notices: the domain stays
// ascending, so training, framing, the tick search and every copy of the scale
// keep the invariant they were written with, and only the pixels run backwards.
// See docs/adr/0075-an-axis-has-a-direction.md.

func TestReverseMirrorsTheAxis(t *testing.T) {
	s := scale.Linear(scale.Domain(0, 10), scale.Reverse())
	s.SetRange(0, 400)

	for v, want := range map[float64]float32{0: 400, 5: 200, 10: 0} {
		if got := s.Map(v); math.Abs(float64(got-want)) > 1e-3 {
			t.Errorf("Map(%v) = %v, want %v", v, got, want)
		}
	}
	for _, v := range []float64{0, 2.5, 7, 10} {
		if back := s.Invert(s.Map(v)); math.Abs(back-v) > 1e-3 {
			t.Errorf("Map/Invert round trip lost %v: got %v", v, back)
		}
	}
}

func TestReverseComposesWithAnInvertedRange(t *testing.T) {
	// A Cartesian panel hands the Y scale its range already flipped, so that
	// larger values sit higher. Reversing that axis puts them back at the
	// bottom rather than cancelling out into nothing.
	s := scale.Linear(scale.Domain(0, 10), scale.Reverse())
	s.SetRange(400, 0)
	if got := s.Map(0); got != 0 {
		t.Errorf("Map(0) = %v, want 0", got)
	}
	if got := s.Map(10); got != 400 {
		t.Errorf("Map(10) = %v, want 400", got)
	}
}

// TestReverseKeepsEveryTick is the regression this option was built for: the
// axis used to be reversed by pinning a descending domain, and then the
// containment test in Ticks — which asks whether a value is between lo and hi —
// dropped every tick there was, including a sequence pinned by hand.
func TestReverseKeepsEveryTick(t *testing.T) {
	fwd := scale.Linear(scale.Domain(0, 12))
	rev := scale.Linear(scale.Domain(0, 12), scale.Reverse())
	fwd.SetRange(0, 400)
	rev.SetRange(0, 400)

	want := fwd.Ticks(scale.TickRequest{Want: 5})
	got := rev.Ticks(scale.TickRequest{Want: 5})
	if len(want) == 0 {
		t.Fatal("the forward axis produced no ticks to compare against")
	}
	if len(got) != len(want) {
		t.Fatalf("reversed axis has %d ticks, forward one has %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Value != want[i].Value || got[i].Label != want[i].Label {
			t.Errorf("tick %d is %v %q, want %v %q", i, got[i].Value, got[i].Label, want[i].Value, want[i].Label)
		}
		// Same value, mirrored position: the two land symmetrically about the
		// middle of the range.
		if sum := got[i].Pos + want[i].Pos; math.Abs(float64(sum-400)) > 1e-3 {
			t.Errorf("tick %v sits at %v reversed and %v forward, which do not mirror", got[i].Value, got[i].Pos, want[i].Pos)
		}
	}
}

func TestReversePinnedTicksSurvive(t *testing.T) {
	s := scale.Linear(scale.Domain(0, 12), scale.TickValues(0, 3, 6, 9, 12), scale.Reverse())
	s.SetRange(0, 400)
	if got := s.Ticks(scale.TickRequest{Want: 5}); len(got) != 5 {
		t.Errorf("pinned ticks on a reversed axis: %d of 5 survived (%v)", len(got), got)
	}
}

func TestReverseFramesTheDomainLikeAnyOtherAxis(t *testing.T) {
	// Zero and Nice take a minimum and a maximum of the domain, which is
	// exactly what a domain written backwards could not survive.
	s := scale.Linear(scale.Zero(), scale.Nice(), scale.Reverse())
	s.SetRange(0, 400)
	s.Train(4, 7, 9)
	lo, hi := s.Domain()
	if lo > hi {
		t.Fatalf("domain came back descending: [%v, %v]", lo, hi)
	}
	if lo != 0 {
		t.Errorf("Zero() did not reach the baseline: domain is [%v, %v]", lo, hi)
	}
	if s.Map(lo) <= s.Map(hi) {
		t.Errorf("axis is not reversed: Map(%v) = %v, Map(%v) = %v", lo, s.Map(lo), hi, s.Map(hi))
	}
}

func TestReverseSurvivesZoomAndAutoscale(t *testing.T) {
	s := scale.Linear(scale.Domain(0, 10), scale.Reverse())
	s.SetRange(0, 400)
	z := s.(scale.Zoomer)

	// A drag hands its two ends over in whatever order the pointer met them,
	// and on a reversed axis that is backwards. The direction is the axis's,
	// not the drag's.
	z.SetDomain(8, 2)
	if got := s.Map(2); math.Abs(float64(got-400)) > 1e-3 {
		t.Errorf("after a zoom, Map(2) = %v, want the range end 400", got)
	}

	z.Autoscale()
	s.Train(0, 10)
	if got := s.Map(0); math.Abs(float64(got-400)) > 1e-3 {
		t.Errorf("after autoscaling, Map(0) = %v, want the range end 400", got)
	}
}

func TestReverseSurvivesCloneAndSnapshot(t *testing.T) {
	s := scale.Linear(scale.Domain(0, 10), scale.Reverse())
	for name, copied := range map[string]scale.Scale{
		"clone":    s.(scale.Cloner).Clone(),
		"snapshot": s.(scale.Snapshotter).Snapshot(),
	} {
		copied.SetRange(0, 400)
		if got := copied.Map(0); math.Abs(float64(got-400)) > 1e-3 {
			t.Errorf("%s: Map(0) = %v, want the range end 400", name, got)
		}
	}
}

func TestReverseIsDescribedAndRebuilt(t *testing.T) {
	s := scale.Linear(scale.Domain(0, 10), scale.Reverse())
	d, ok := scale.Describe(s)
	if !ok {
		t.Fatal("a linear scale did not describe itself")
	}
	if !d.Reverse {
		t.Fatal("the description does not say the axis is reversed")
	}
	back, err := scale.FromDesc(d)
	if err != nil {
		t.Fatalf("FromDesc: %v", err)
	}
	back.SetRange(0, 400)
	if got := back.Map(0); math.Abs(float64(got-400)) > 1e-3 {
		t.Errorf("the rebuilt scale is not reversed: Map(0) = %v", got)
	}
}

// TestPinnedDomainIsOrdered covers the trap the option replaces. A domain
// written backwards is a domain, and the axis it belongs to runs the ordinary
// way — rather than mirroring the marks and then losing every number off the
// axis, which is what it used to do.
func TestPinnedDomainIsOrdered(t *testing.T) {
	s := scale.Linear(scale.Domain(12, 0))
	s.SetRange(0, 400)
	if got := s.Map(0); got != 0 {
		t.Errorf("Map(0) = %v, want the range start 0", got)
	}
	if n := len(s.Ticks(scale.TickRequest{Want: 5})); n == 0 {
		t.Error("a domain written backwards produced an axis with no ticks")
	}
}

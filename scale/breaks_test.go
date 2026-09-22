package scale

import (
	"math"
	"testing"
	"time"
)

// broke returns a linear scale over [0, 100] on a 0..220 pixel range with a
// break from 10 to 90 and a 20 pixel gap, so that each kept piece is 100 px.
func broke(opts ...LinearOption) Scale {
	s := Linear(append([]LinearOption{Domain(0, 100), Break(10, 90)}, opts...)...)
	s.SetRange(0, 220)
	s.(Breaker).SetBreakGap(20, 4)
	return s
}

func TestABreakSplitsTheRangeBetweenTheKeptPieces(t *testing.T) {
	s := broke()
	for _, c := range []struct {
		v    float64
		want float32
	}{{0, 0}, {5, 50}, {10, 100}, {50, 110}, {90, 120}, {95, 170}, {100, 220}} {
		if got := s.Map(c.v); math.Abs(float64(got-c.want)) > 1e-3 {
			t.Errorf("Map(%v) = %v, want %v", c.v, got, c.want)
		}
	}
	// The two edges of the gap are exact: they are what the clip and the mark
	// are cut against.
	if got := s.Map(10); got != 100 {
		t.Errorf("Map(10) = %v, want exactly 100", got)
	}
	if got := s.Map(90); got != 120 {
		t.Errorf("Map(90) = %v, want exactly 120", got)
	}
	lo, hi, fold := s.(Breaker).Gap(0)
	if lo != 100 || hi != 120 || fold {
		t.Errorf("Gap(0) = %v, %v, %v; want 100, 120, false", lo, hi, fold)
	}
}

func TestABrokenAxisInvertsExactlyAndMonotonically(t *testing.T) {
	s := broke()
	prev := float32(-1)
	for v := 0.0; v <= 100; v += 0.5 {
		p := s.Map(v)
		if p < prev {
			t.Fatalf("Map is not monotone at %v: %v after %v", v, p, prev)
		}
		prev = p
		if back := s.Invert(p); math.Abs(back-v) > 1e-3 {
			t.Errorf("Invert(Map(%v)) = %v", v, back)
		}
	}
}

func TestABreakFollowsAReversedAxis(t *testing.T) {
	s := broke(Reverse())
	if got := s.Map(0); got != 220 {
		t.Errorf("Map(0) = %v, want 220", got)
	}
	if got := s.Map(10); got != 120 {
		t.Errorf("Map(10) = %v, want 120", got)
	}
	if got := s.Map(90); got != 100 {
		t.Errorf("Map(90) = %v, want 100", got)
	}
	lo, hi, _ := s.(Breaker).Gap(0)
	if lo != 120 || hi != 100 {
		t.Errorf("Gap(0) = %v, %v; want 120, 100", lo, hi)
	}
}

func TestABreakIsOffUntilTheRendererSwitchesItOn(t *testing.T) {
	s := Linear(Domain(0, 100), Break(10, 90))
	s.SetRange(0, 100)
	if got := s.Map(50); got != 50 {
		t.Errorf("an unswitched break mapped 50 to %v", got)
	}
	if n := s.(Breaker).Gaps(); n != 0 {
		t.Errorf("an unswitched break reports %d gaps", n)
	}
	s.(Breaker).SetBreakGap(10, 2)
	s.(Breaker).SetBreakGap(-1, 0)
	if got := s.Map(50); got != 50 {
		t.Errorf("a break switched off mapped 50 to %v", got)
	}
}

func TestABreakOutsideTheDomainIsNotDrawn(t *testing.T) {
	s := broke()
	s.(Zoomer).SetDomain(20, 80) // wholly inside the break
	if n := s.(Breaker).Gaps(); n != 0 {
		t.Errorf("a domain inside the break reports %d gaps", n)
	}
}

// TestABreakAtAnEndTrimsTheAxis: a cut reaching an end of the domain has
// nothing on its far side to leave a gap before, so the axis ends where it
// begins — the last idle hour of a shift folded off the end of the chart.
func TestABreakAtAnEndTrimsTheAxis(t *testing.T) {
	s := broke()
	s.(Zoomer).SetDomain(50, 100) // the break covers the low end
	if n := s.(Breaker).Gaps(); n != 0 {
		t.Errorf("a break across the domain's end reports %d gaps", n)
	}
	if got := s.Map(90); got != 0 {
		t.Errorf("Map(90) = %v, want 0: the axis starts where the break ends", got)
	}
	if got := s.Map(95); math.Abs(float64(got-110)) > 1e-3 {
		t.Errorf("Map(95) = %v on a 90..100 axis over 220 px, want 110", got)
	}
	for _, tk := range s.Ticks(TickRequest{Want: 5}) {
		if tk.Value < 90 {
			t.Errorf("tick %v is inside the trimmed end", tk.Value)
		}
	}
}

func TestABreakThatDoesNotFitIsNotDrawn(t *testing.T) {
	s := Linear(Domain(0, 100), Break(10, 90))
	s.SetRange(0, 30)
	s.(Breaker).SetBreakGap(20, 4)
	if n := s.(Breaker).Gaps(); n != 0 {
		t.Errorf("a 20 px gap on a 30 px axis was drawn")
	}
}

func TestFoldsGiveTheirRoomUpBeforeTheyAreDropped(t *testing.T) {
	var ivs []Interval
	for i := 0; i < 50; i++ {
		ivs = append(ivs, Interval{float64(2*i) + 0.5, float64(2*i) + 1.5})
	}
	s := Linear(Domain(0, 100), Fold(ivs...))
	s.SetRange(0, 100)
	s.(Breaker).SetBreakGap(20, 4) // 50 folds × 4 px is twice the axis
	b := s.(Breaker)
	if n := b.Gaps(); n != 50 {
		t.Fatalf("Gaps() = %d, want 50", n)
	}
	lo, hi, fold := b.Gap(3)
	if lo != hi || !fold {
		t.Errorf("a fold that did not fit kept a gap: %v..%v fold=%v", lo, hi, fold)
	}
}

func TestManyFoldsMapLikeAWalkOverThem(t *testing.T) {
	var ivs []Interval
	for i := 0; i < 500; i++ {
		ivs = append(ivs, Interval{float64(10*i) + 3, float64(10*i) + 7})
	}
	s := Linear(Domain(0, 5000), Fold(ivs...))
	s.SetRange(0, 10000)
	s.(Breaker).SetBreakGap(8, 2)
	// 2000 px of kept domain... every piece but the ends is 6 wide, the gaps
	// 2 px: the reference walks them one at a time.
	kept := 5000.0 - 500*4
	unit := (10000.0 - 500*2) / kept
	ref := func(v float64) float64 {
		off, at := 0.0, 0.0
		for _, iv := range ivs {
			if v <= iv.Lo {
				break
			}
			if v < iv.Hi {
				return off + (iv.Lo-at)*unit + (v-iv.Lo)/(iv.Hi-iv.Lo)*2
			}
			off += (iv.Lo-at)*unit + 2
			at = iv.Hi
		}
		return off + (v-at)*unit
	}
	for v := 0.0; v <= 5000; v += 1.25 {
		if got, want := float64(s.Map(v)), ref(v); math.Abs(got-want) > 0.01 {
			t.Fatalf("Map(%v) = %v, want %v", v, got, want)
		}
	}
}

func TestABreakAbsorbsAFoldItOverlaps(t *testing.T) {
	s := Linear(Domain(0, 100), Fold(Interval{20, 40}), Break(30, 60))
	d := s.(Describer).Describe()
	if len(d.Cuts) != 1 || d.Cuts[0] != (Interval{20, 60}) || len(d.Folds) != 0 {
		t.Errorf("cuts %v folds %v, want one break 20..60", d.Cuts, d.Folds)
	}
}

func TestTicksOnABrokenAxisShareOneStepAndAvoidTheGap(t *testing.T) {
	s := broke()
	ts := s.Ticks(TickRequest{Want: 5})
	var got []float64
	for _, tk := range ts {
		got = append(got, tk.Value)
		if tk.Value > 10 && tk.Value < 90 {
			t.Errorf("tick at %v is inside the break", tk.Value)
		}
	}
	want := []float64{0, 5, 10, 90, 95, 100}
	if len(got) != len(want) {
		t.Fatalf("ticks %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ticks %v, want %v", got, want)
		}
	}
}

func TestABreakSurvivesCloneSnapshotAndDescribe(t *testing.T) {
	s := broke()
	snap := s.(Snapshotter).Snapshot()
	if snap.Map(95) != s.Map(95) {
		t.Errorf("a snapshot maps 95 to %v, original to %v", snap.Map(95), s.Map(95))
	}
	d := s.(Describer).Describe()
	back, err := FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	back.SetRange(0, 220)
	back.(Breaker).SetBreakGap(20, 4)
	if back.Map(95) != s.Map(95) {
		t.Errorf("a round trip maps 95 to %v, original to %v", back.Map(95), s.Map(95))
	}
	c := s.(Cloner).Clone()
	c.SetRange(0, 220)
	c.(Breaker).SetBreakGap(20, 4)
	if c.Map(95) != s.Map(95) {
		t.Errorf("a clone maps 95 to %v, original to %v", c.Map(95), s.Map(95))
	}
}

func TestATimeFoldIsMeasuredFromTheOriginGivenAfterIt(t *testing.T) {
	t0 := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	s := Time(TimeFold(TimeSpan{t0.Add(day), t0.Add(3 * day)}), Origin(t0))
	s.(Zoomer).SetDomain(0, float64(5*day))
	s.SetRange(0, 304)
	s.(Breaker).SetBreakGap(8, 4)
	// Three days kept on 300 px: 100 px a day, and a 4 px fold after the first.
	if got := s.Map(ValueOf(s, t0.Add(day))); got != 100 {
		t.Errorf("fold starts at %v, want 100", got)
	}
	if got := s.Map(ValueOf(s, t0.Add(3*day))); got != 104 {
		t.Errorf("fold ends at %v, want 104", got)
	}
	for _, tk := range s.Ticks(TickRequest{Want: 4}) {
		if v := tk.Value; v > float64(day) && v < float64(3*day) {
			t.Errorf("tick %v inside the fold", InstantOf(s, v))
		}
	}
	d := s.(Describer).Describe()
	if len(d.Folds) != 1 || d.Folds[0] != (Interval{float64(day), float64(3 * day)}) {
		t.Errorf("Folds = %v", d.Folds)
	}
}

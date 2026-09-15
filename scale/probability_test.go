package scale_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/timzifer/figure/scale"
)

func TestEachLinkInvertsAndCrossesZeroWhereItsDistributionSaysItShould(t *testing.T) {
	cases := []struct {
		name string
		link scale.Link
		zero float64 // the probability the link sends to zero
	}{
		{"probit", scale.Probit, 0.5},
		{"logit", scale.Logit, 0.5},
		{"cloglog", scale.CLogLog, 1 - 1/math.E},
		{"gumbel", scale.Gumbel, 1 / math.E},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if z := tc.link.Apply(tc.zero); math.Abs(z) > 1e-12 {
				t.Errorf("Apply(%v) = %v, want 0", tc.zero, z)
			}
			prev := math.Inf(-1)
			for _, p := range []float64{1e-9, 0.001, 0.1, 0.5, 0.9, 0.999, 1 - 1e-9} {
				z := tc.link.Apply(p)
				if !(z > prev) {
					t.Errorf("Apply is not increasing at %v", p)
				}
				prev = z
				if back := tc.link.Unapply(z); math.Abs(back-p) > 1e-9*math.Max(p, 1e-3) {
					t.Errorf("Unapply(Apply(%v)) = %v", p, back)
				}
			}
			name, ok := scale.LinkName(tc.link)
			if !ok || name != tc.name {
				t.Errorf("LinkName = %q, %v; want %q", name, ok, tc.name)
			}
			if l, ok := scale.LinkNamed(tc.name); !ok || l != tc.link {
				t.Errorf("LinkNamed(%q) did not return the link", tc.name)
			}
		})
	}
	if math.Abs(scale.Probit.Apply(0.975)-1.959964) > 1e-6 {
		t.Errorf("probit(0.975) = %v, want 1.96", scale.Probit.Apply(0.975))
	}
}

func TestAProbitAxisSpacesNormalQuantilesEvenly(t *testing.T) {
	phi := scale.Probit.Unapply
	s := scale.Probability(scale.Probit, scale.ProbabilityDomain(phi(-2), phi(2)))
	s.SetRange(0, 100)
	for i, z := range []float64{-2, -1, 0, 1, 2} {
		want := float64(i) * 25
		if got := s.Map(phi(z)); math.Abs(float64(got)-want) > 1e-3 {
			t.Errorf("Map(Φ(%v)) = %v, want %v", z, got, want)
		}
		if got := s.Invert(float32(want)); math.Abs(got-phi(z)) > 1e-6 {
			t.Errorf("Invert(%v) = %v, want %v", want, got, phi(z))
		}
	}
}

func TestAProbabilityAxisHasNoPlaceForZeroOrOne(t *testing.T) {
	s := scale.Probability(nil)
	s.Train(0, 1, -0.5, 2, math.NaN(), 0.2, 0.7)
	s.SetRange(0, 100)
	if lo, hi := s.Domain(); lo != 0.2 || hi != 0.7 {
		t.Errorf("domain = [%v, %v], want [0.2, 0.7]: training takes only the open interval", lo, hi)
	}
	d := s.(scale.Definite)
	for _, v := range []float64{0, 1, -1, 1.5} {
		if d.Defined(v) {
			t.Errorf("Defined(%v) = true", v)
		}
		if !math.IsNaN(float64(s.Map(v))) {
			t.Errorf("Map(%v) is not NaN", v)
		}
	}
}

func labels(ts []scale.Tick) []string {
	var out []string
	for _, t := range ts {
		if !t.Minor {
			out = append(out, t.Label)
		}
	}
	return out
}

func TestTheLadderIsTheConventionalOne(t *testing.T) {
	s := scale.Probability(scale.Probit, scale.ProbabilityDomain(0.001, 0.999))
	s.SetRange(0, 400)
	got := strings.Join(labels(s.Ticks(scale.TickRequest{Want: 20})), " ")
	want := "0.1% 1% 5% 10% 20% 30% 50% 70% 80% 90% 95% 99% 99.9%"
	if got != want {
		t.Errorf("ladder = %q\nwant     %q", got, want)
	}
}

func TestAShortAxisThinsTheLadderAboutTheMedian(t *testing.T) {
	s := scale.Probability(scale.Probit, scale.ProbabilityDomain(0.001, 0.999))
	s.SetRange(0, 100)
	ts := s.Ticks(scale.TickRequest{Want: 7})
	got := strings.Join(labels(ts), " ")
	if want := "0.1% 1% 10% 50% 90% 99% 99.9%"; got != want {
		t.Errorf("thinned ladder = %q, want %q", got, want)
	}
	if len(ts) != 13 {
		t.Errorf("%d ticks, want the 13 rungs with the unlabelled ones kept as minor ticks", len(ts))
	}
	// Symmetric about the median: every labelled rung has its mirror image.
	for _, tk := range ts {
		if tk.Minor {
			continue
		}
		mirror := s.Map(1 - tk.Value)
		if math.Abs(float64(mirror-(100-tk.Pos))) > 1e-3 {
			t.Errorf("rung %v at %v has no mirror at %v", tk.Value, tk.Pos, 100-tk.Pos)
		}
	}

	few := labels(s.Ticks(scale.TickRequest{Want: 3}))
	if len(few) > 3 || !contains(few, "50%") {
		t.Errorf("Want 3 labelled %v", few)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func TestTheLadderReachesAsFarIntoATailAsTheDomainDoes(t *testing.T) {
	s := scale.Probability(scale.CLogLog, scale.ProbabilityDomain(1e-5, 0.5))
	s.SetRange(0, 400)
	got := labels(s.Ticks(scale.TickRequest{Want: 20}))
	if got[0] != "0.001%" || got[len(got)-1] != "50%" {
		t.Errorf("ladder %v, want it to run from 0.001%% to 50%%", got)
	}
}

func TestAProbabilityLabelFollowsTheLocaleAndTheSpec(t *testing.T) {
	s := scale.Probability(nil)
	s.(scale.Localizer).SetLocale(scale.LocaleDE)
	if got, want := scale.LabelOf(s, 0.999), "99,9 %"; got != want {
		t.Errorf("German label = %q, want %q", got, want)
	}
	f := scale.Probability(nil, scale.ProbabilityNumberFormat("#.2"))
	if got, want := scale.LabelOf(f, 0.25), "0.25"; got != want {
		t.Errorf("spec label = %q, want %q", got, want)
	}
}

func TestAProbabilityScaleSurvivesItsDesc(t *testing.T) {
	s := scale.Probability(scale.Gumbel, scale.ProbabilityDomain(0.01, 0.9),
		scale.ProbabilityMinorTicks(false), scale.ProbabilityNumberFormat("#.1%"))
	d, ok := scale.Describe(s)
	if !ok {
		t.Fatal("a probability scale cannot describe itself")
	}
	if d.Kind != scale.KindProbability || d.Link != "gumbel" || !d.Fixed || d.MinorTicks {
		t.Errorf("Desc = %+v", d)
	}
	back, err := scale.FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, sc := range []scale.Scale{s, back} {
		sc.SetRange(0, 100)
	}
	if a, b := s.Ticks(scale.TickRequest{}), back.Ticks(scale.TickRequest{}); len(a) != len(b) || a[0] != b[0] {
		t.Errorf("ticks differ after the round trip: %v vs %v", a, b)
	}
}

type ownLink struct{}

func (ownLink) Apply(p float64) float64   { return p }
func (ownLink) Unapply(z float64) float64 { return z }

func TestALinkWrittenInGoIsRefusedRatherThanReplaced(t *testing.T) {
	d, _ := scale.Describe(scale.Probability(ownLink{}))
	if d.Link != "" {
		t.Errorf("a custom link described itself as %q", d.Link)
	}
	if _, err := scale.FromDesc(d); !errors.Is(err, scale.ErrUnknownKind) {
		t.Errorf("FromDesc of a nameless link: err = %v, want ErrUnknownKind", err)
	}
}

func TestAPanOffTheEndOfAProbabilityAxisStopsShortOfIt(t *testing.T) {
	s := scale.Probability(nil)
	s.(scale.Zoomer).SetDomain(-0.2, 1.3)
	lo, hi := s.Domain()
	if !(lo > 0 && hi < 1 && lo < hi) {
		t.Errorf("domain [%v, %v] is not inside (0, 1)", lo, hi)
	}
	s.(scale.Zoomer).Autoscale()
	if lo, hi := s.Domain(); lo != 0.01 || hi != 0.99 {
		t.Errorf("autoscaled axis = [%v, %v], want the untrained default", lo, hi)
	}
}

package scale_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/scale"
)

// Map64 is Map without the rounding, so the two agree to well within a pixel.
func TestMap64AgreesWithMap(t *testing.T) {
	for name, s := range map[string]scale.Scale{
		"linear": scale.Linear(),
		"log":    scale.Log(),
		"symlog": scale.SymLog(),
		"time":   scale.Time(),
	} {
		if _, ok := s.(scale.Precise); !ok {
			t.Errorf("the %s scale does not map at float64", name)
			continue
		}
		z, ok := s.(scale.Zoomer)
		if !ok {
			t.Fatalf("the %s scale cannot be given a domain", name)
		}
		z.SetDomain(1, 1000)
		s.SetRange(12.5, 937.25)
		for _, v := range []float64{1, 2.5, 17, 333.3, 999, 1000} {
			if d := math.Abs(scale.Map64(s, v) - float64(s.Map(v))); d > 1e-3 {
				t.Errorf("the %s scale maps %v to %v at float64 and %v at float32", name, v, scale.Map64(s, v), s.Map(v))
			}
		}
	}
}

// A pan moves every position by one offset. At float32 each position is
// rounded afresh every frame, so the distance between two of them wobbles by a
// few hundred-thousandths of a pixel — enough for a hexbin to move the rows on
// a cell border across it and back while the plot is dragged.
func TestAPanMovesEveryMap64PositionByOneOffset(t *testing.T) {
	s := scale.Linear()
	s.SetRange(12.5, 937.25)
	z := s.(scale.Zoomer)
	const lo, hi = 0.123, 87.456
	vs := make([]float64, 200)
	for i := range vs {
		vs[i] = lo + (hi-lo)*math.Mod(float64(i)*0.618034, 1)
	}
	base := make([]float64, len(vs))
	for k, d := range []float64{0, 0.0137, 1.91, -3.3e-3, 40.2} {
		z.SetDomain(lo+d, hi+d)
		a := scale.Map64(s, vs[0])
		for i, v := range vs {
			rel := scale.Map64(s, v) - a
			if k == 0 {
				base[i] = rel
				continue
			}
			if math.Abs(rel-base[i]) > 1e-9 {
				t.Fatalf("panned by %v, %v sits %v from the first row; unpanned it sat %v", d, v, rel, base[i])
			}
		}
	}
}

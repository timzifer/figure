package three

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

// fibonacci is n directions spread evenly over the whole sphere: equal steps
// in cos θ up the ball and the golden angle round it, which is as close to a
// uniform sample as a deterministic one gets.
func fibonacci(n int) figure.Source {
	az := make([]float64, n)
	pol := make([]float64, n)
	for i := range az {
		mu := 1 - 2*(float64(i)+0.5)/float64(n)
		pol[i] = math.Acos(mu) * 180 / math.Pi
		az[i] = math.Mod(float64(i)*137.50776405003785, 360)
	}
	return figure.NewTable().Float64("az", az).Float64("pol", pol)
}

func histScene(src figure.Source, opts ...geom.Option) *Scene {
	o := append([]geom.Option{geom.X("az"), geom.Y("pol")}, opts...)
	return NewScene(Spherical()).
		X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear()).
		Add(Histogram3(src, o...))
}

func histOf(t *testing.T, sc *Scene) (*hist3, Frame) {
	t.Helper()
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatal(err)
	}
	g, ok := sc.layers[0].(*hist3)
	if !ok {
		t.Fatalf("the layer is a %T", sc.layers[0])
	}
	return g, Frame{X: scales[axisX], Y: scales[axisY], Z: scales[axisZ], space: sc.sphere}
}

// The bands are equal in cos θ, which is what makes them equal in area: a band
// of the sphere between two polar angles has an area proportional to the
// difference of their cosines. Bands of equal *angle* would be the lat/long
// grid this layer exists not to be.
func TestASphericalHistogramsBandsAreEqualInArea(t *testing.T) {
	const bands = 8
	var step float64
	for b := 0; b < bands; b++ {
		lo := math.Cos(math.Pi * float64(bandEdge(b, bands)))
		hi := math.Cos(math.Pi * float64(bandEdge(b+1, bands)))
		d := lo - hi
		if b == 0 {
			step = d
			continue
		}
		if math.Abs(d-step) > 1e-6 {
			t.Errorf("band %d spans %v of cos θ, want %v — the cells are not equal-area", b, d, step)
		}
	}
	// And the ladder covers the whole ball, pole to pole.
	if got := bandEdge(0, bands); got != 0 {
		t.Errorf("the first band starts at %v, want a pole", got)
	}
	if got := bandEdge(bands, bands); math.Abs(float64(got)-1) > 1e-6 {
		t.Errorf("the last band ends at %v, want the other pole", got)
	}
}

// A uniform set of directions fills every cell about equally. Under a lat/long
// grid the same sample piles up at the poles, which is the failure this
// binning is for.
func TestAUniformSampleFillsEveryCellAboutEqually(t *testing.T) {
	const bands, n = 8, 1280
	g, _ := histOf(t, histScene(fibonacci(n), geom.Bins(bands)))
	want := float64(n) / float64(len(g.counts))
	lo, hi := math.Inf(1), 0.0
	total := 0.0
	for _, c := range g.counts {
		total += c
		lo, hi = math.Min(lo, c), math.Max(hi, c)
	}
	if total != n {
		t.Errorf("%v readings binned, want all %d of them", total, n)
	}
	if lo < want/2 || hi > want*2 {
		t.Errorf("cell counts run from %v to %v around a mean of %v; the cells are not equal-area",
			lo, hi, want)
	}
}

// Every band gets the same share of a uniform sample, exactly, because the
// sample and the bands are both uniform in cos θ.
func TestEveryBandOfAUniformSampleHoldsTheSameCount(t *testing.T) {
	const bands, n = 8, 1280
	g, _ := histOf(t, histScene(fibonacci(n), geom.Bins(bands)))
	for b := 0; b < bands; b++ {
		sum := 0.0
		for k := 0; k < g.sectors; k++ {
			sum += g.counts[b*g.sectors+k]
		}
		if sum != float64(n/bands) {
			t.Errorf("band %d holds %v readings, want %d", b, sum, n/bands)
		}
	}
}

// One face per non-empty cell, and none for an empty one: a face at the centre
// is ink with no reading in it.
func TestASphericalHistogramDrawsOneFacePerOccupiedCell(t *testing.T) {
	src := figure.NewTable().
		Float64("az", []float64{10, 12, 200}).
		Float64("pol", []float64{80, 85, 20})
	sc := histScene(src, geom.Bins(4))
	g, f := histOf(t, sc)
	s := new(Sink)
	s.openLayer(0, Home().Forward())
	if err := g.Emit(s, f); err != nil {
		t.Fatal(err)
	}
	occupied := 0
	for _, c := range g.counts {
		if c > 0 {
			occupied++
		}
	}
	if len(s.prims) != occupied {
		t.Errorf("%d faces over %d occupied cells", len(s.prims), occupied)
	}
	for i, p := range s.prims {
		if p.kind != kindFace || p.hi-p.lo != 4 {
			t.Errorf("primitive %d is not a quad", i)
		}
	}
}

// The count is the radius, read through the scene's own depth scale — so the
// ball's ladder is the key, as it is for a radiation pattern.
func TestASphericalHistogramStandsEachCellAtItsCount(t *testing.T) {
	src := figure.NewTable().
		Float64("az", []float64{10, 10, 10, 200}).
		Float64("pol", []float64{80, 80, 80, 20})
	sc := histScene(src, geom.Bins(4))
	g, f := histOf(t, sc)
	s := new(Sink)
	s.openLayer(0, Home().Forward())
	if err := g.Emit(s, f); err != nil {
		t.Fatal(err)
	}
	var lo, hi float64
	for _, v := range s.verts {
		r := v.Sub(centre)
		d := math.Sqrt(r.Dot(r))
		if lo == 0 || d < lo {
			lo = d
		}
		if d > hi {
			hi = d
		}
	}
	// Three readings in one cell and one in another: the busy cell stands
	// three times as far out as the quiet one, since the depth scale runs
	// from zero.
	if math.Abs(hi/lo-3) > 0.05 {
		t.Errorf("the cells stand at %v and %v, want a ratio of three", hi, lo)
	}
}

// A direction has nowhere to live in a box, and a Smith sphere's two values
// are an impedance rather than an angle.
func TestASphericalHistogramRefusesEveryOtherScene(t *testing.T) {
	src := fibonacci(64)
	for name, sc := range map[string]*Scene{
		"a box": NewScene().X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear()).
			Add(Histogram3(src, geom.X("az"), geom.Y("pol"))),
		"a Smith sphere": NewScene(Spherical(Smith())).
			X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear()).
			Add(Histogram3(src, geom.X("az"), geom.Y("pol"))),
	} {
		scales := sc.scales()
		if err := sc.train(scales); err != nil {
			t.Fatal(err)
		}
		f := Frame{X: scales[axisX], Y: scales[axisY], Z: scales[axisZ], space: sc.sphere}
		s := new(Sink)
		s.openLayer(0, Home().Forward())
		if err := sc.layers[0].Emit(s, f); !errors.Is(err, ErrNotSpherical) {
			t.Errorf("a histogram in %s: err = %v, want ErrNotSpherical", name, err)
		}
	}
}

// The layer never learns whether the scene reads degrees or radians, a polar
// angle or a latitude: a reading's position in its own pinned domain is what
// it bins, so the same sample comes out in the same cells either way.
func TestASphericalHistogramBinsTheSameWhicheverConventionTheSceneReads(t *testing.T) {
	deg := figure.NewTable().
		Float64("az", []float64{0, 90, 180, 270}).
		Float64("pol", []float64{40, 70, 110, 140})
	lat := figure.NewTable().
		Float64("az", []float64{0, 90, 180, 270}).
		Float64("pol", []float64{50, 20, -20, -50})

	byPolar, _ := histOf(t, histScene(deg, geom.Bins(4)))
	sc := NewScene(Spherical(Latitude())).
		X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear()).
		Add(Histogram3(lat, geom.X("az"), geom.Y("pol"), geom.Bins(4)))
	byLatitude, _ := histOf(t, sc)

	// A latitude runs the other way, so band b of one is band n−1−b of the
	// other; the counts are the same cell for cell either way. The readings
	// are kept off the band edges, where a boundary belongs to the band above
	// it on both sides and the mirror is therefore not exact.
	n := byPolar.bands
	for b := 0; b < n; b++ {
		for k := 0; k < byPolar.sectors; k++ {
			a := byPolar.counts[b*byPolar.sectors+k]
			c := byLatitude.counts[(n-1-b)*byLatitude.sectors+k]
			if a != c {
				t.Errorf("cell (%d, %d): %v by polar angle and %v by latitude", b, k, a, c)
			}
		}
	}
}

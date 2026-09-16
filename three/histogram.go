package three

import (
	"fmt"
	"math"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

// Histogram3 bins a column of directions into equal-area cells of the sphere
// and draws each cell as a face standing at the height of its count.
//
//	sc := three.NewScene(three.Spherical(three.Latitude()))
//	sc.Add(three.Histogram3(joints, geom.X("strike"), geom.Y("plunge")))
//
// It is the distribution of a *direction*: which way a set of crystals point,
// where a wind came from, how a population of fractures is oriented. X is the
// azimuth and Y the polar angle or, under [Latitude], the angle up from the
// equator — the same two angles every other layer in a spherical scene reads.
// There is no Z column, because the count is the value.
//
// docs/adr/0058-what-3d-is-for.md ranks it fourth, beside the Smith sphere and
// reachable by the same machinery. A stereonet's contoured density is the flat
// form of this chart and always was a projection of it.
//
// # The binning is equal-area, and that is the whole of the arithmetic
//
// A latitude–longitude grid is not: its cells shrink towards the poles, so a
// uniform set of directions would come out with a ring of tall cells round the
// equator and nothing at the ends. The fix is one substitution. A band of the
// sphere between two polar angles has an area proportional to the difference
// of their cosines, so bands of equal **cos θ** have equal area — and cutting
// every band into the same number of azimuth sectors leaves every cell with
// the same area as every other. [github.com/timzifer/figure/geom.Bins] is how
// many bands, and there are twice as many sectors round each of them.
//
// The substitution is also why this layer never has to know which convention
// the scene reads its second angle in: the polar angle and the latitude differ
// by a reflection, equal steps in the cosine of one are equal steps in the
// cosine of the other, and the cells come out the same size either way.
//
// # What it draws
//
// One face per non-empty cell, at the radius its count maps to through the
// scene's depth scale — so the ball's own ladder is the key, as it is for a
// radiation pattern. An empty cell is not drawn rather than drawn at the
// centre: a face of no size is ink with no reading in it.
// [github.com/timzifer/figure/geom.ColorBy]'s scale paints the count through a
// ramp instead of, or as well as, the radius.
//
// A histogram in a box or in a [Smith] scene is [ErrNotSpherical]: a
// resistance is not a direction, and a direction has nowhere to live in a box.
func Histogram3(src data.Source, opts ...geom.Option) Layer {
	return &hist3{base: newBase(src, "histogram3", opts)}
}

// defaultBands is how many bands of equal cos θ a spherical histogram uses
// when nobody chose. Eight bands and sixteen sectors is a hundred and
// twenty-eight cells over the whole ball, which is about as fine as a few
// hundred readings support.
const defaultBands = 8

type hist3 struct {
	base
	// bands and sectors are the resolved grid: bands of equal cos θ, and the
	// same number of azimuth sectors round each of them.
	bands, sectors int
	// counts is one per cell, indexed band-major.
	counts []float64
	ramp   scale.ColorScale
	ok     bool
}

func (g *hist3) Train(t geom.Training) error {
	g.ok = false
	az, err := column(g.src, g.cfg.X, t.X)
	if err != nil {
		return err
	}
	pol, err := column(g.src, g.cfg.Y, t.Y)
	if err != nil {
		return err
	}
	if len(az) != len(pol) {
		return fmt.Errorf("figure/three: the histogram's two angles have %d and %d rows",
			len(az), len(pol))
	}
	g.bands = g.cfg.Bins
	if g.bands <= 0 {
		g.bands = defaultBands
	}
	g.sectors = 2 * g.bands
	g.counts = clear64(g.counts, g.bands*g.sectors)

	// The angular domains are already pinned to the whole sphere, because
	// [sphere.pin] runs before any layer trains. So a reading's position is
	// its place in its own domain, which is exactly the fraction [Frame.Place]
	// takes — and this layer never learns whether the scene reads degrees or
	// radians, a polar angle or a latitude.
	x0, x1 := t.X.Domain()
	y0, y1 := t.Y.Domain()
	if x1 == x0 || y1 == y0 {
		return fmt.Errorf("figure/three: a spherical histogram needs two angular scales with a domain")
	}
	for i := range az {
		if !defined(t.X, az[i]) || !defined(t.Y, pol[i]) {
			continue
		}
		u := (az[i] - x0) / (x1 - x0)
		v := (pol[i] - y0) / (y1 - y0)
		if v < 0 || v > 1 {
			// A polar angle outside its own turn is not a direction.
			continue
		}
		g.counts[g.cell(u, v)]++
	}
	t.Z.Train(g.counts...)
	g.ramp = g.cfg.ColorScale
	if g.ramp != nil {
		g.ramp.Train(g.counts...)
	}
	g.ok = true
	return nil
}

// cell is which bin a reading falls in: its azimuth sector and its band of
// equal cos θ, band-major.
func (g *hist3) cell(u, v float64) int {
	s := int(math.Floor(wrap01(u) * float64(g.sectors)))
	if s >= g.sectors {
		s = g.sectors - 1
	}
	b := bandOf(v, g.bands)
	return b*g.sectors + s
}

// bandOf is which band of equal cos θ the second angle's position falls in.
//
// The bands are equal in cos θ rather than in θ, which is what makes them
// equal in area — see [Histogram3]. cos runs downward, so band zero is the one
// at v = 0.
func bandOf(v float64, bands int) int {
	mu := math.Cos(math.Pi * v)
	b := int(math.Floor((1 - mu) / 2 * float64(bands)))
	switch {
	case b < 0:
		return 0
	case b >= bands:
		return bands - 1
	}
	return b
}

// bandEdge is the second angle's position at the lower edge of band b, which
// is the inverse of [bandOf].
func bandEdge(b, bands int) float32 {
	mu := 1 - 2*float64(b)/float64(bands)
	return float32(math.Acos(clampMu(mu)) / math.Pi)
}

func clampMu(mu float64) float64 {
	switch {
	case mu < -1:
		return -1
	case mu > 1:
		return 1
	}
	return mu
}

func (g *hist3) Emit(s *Sink, f Frame) error {
	if !g.ok {
		return nil
	}
	if !f.Spherical() || f.space.smith {
		// A direction is an azimuth and a polar angle; a box has no place for
		// one and a Smith sphere's two values are an impedance.
		return ErrNotSpherical
	}
	base := g.fillFor(f)
	stroke, width := g.outline()

	var quad [4]Vec3
	for b := 0; b < g.bands; b++ {
		v0, v1 := bandEdge(b, g.bands), bandEdge(b+1, g.bands)
		for k := 0; k < g.sectors; k++ {
			n := g.counts[b*g.sectors+k]
			if n == 0 {
				// An empty direction is not drawn. A face at the centre is
				// ink with no reading in it.
				continue
			}
			r := at(f.Z, n)
			u0 := float32(k) / float32(g.sectors)
			u1 := float32(k+1) / float32(g.sectors)
			quad[0] = f.Place(u0, v0, r)
			quad[1] = f.Place(u1, v0, r)
			quad[2] = f.Place(u1, v1, r)
			quad[3] = f.Place(u0, v1, r)
			fill := base
			if g.ramp != nil {
				fill = g.ramp.Color(n)
			}
			// A cell is seen from outside, as every face of a pattern is.
			normal := awayFromCentre(faceNormal(quad[0], quad[1], quad[2]), quad)
			s.Face(quad[:], Style{
				Fill:   shade(f.Theme, fill, normal),
				Stroke: stroke,
				Width:  width,
			})
		}
	}
	return nil
}

func (g *hist3) Legend(f Frame) (geom.LegendEntry, bool) {
	return g.base.legend(f, geom.SwatchBox)
}

// wrap01 folds a position into [0, 1). An azimuth is cyclic, so a reading a
// turn either side of the domain is the same direction rather than a missing
// one.
func wrap01(u float64) float64 {
	u = math.Mod(u, 1)
	if u < 0 {
		u++
	}
	return u
}

// clear64 returns a zeroed slice of length n, reusing s where it fits. Train
// runs on every frame, so the bins are refilled rather than rebuilt.
func clear64(s []float64, n int) []float64 {
	if cap(s) < n {
		return make([]float64, n)
	}
	s = s[:n]
	for i := range s {
		s[i] = 0
	}
	return s
}

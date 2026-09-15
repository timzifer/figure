package scale

import (
	"fmt"
	"math"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
)

// BivariateColorScale is a colour scale that reads two numbers per mark: a
// quantity, and a second reading about it — its uncertainty, a second
// variable, how purely a bin holds its class.
//
// It rides the [ColorScale] interface the way [ClassedColorScale] and
// [DiscreteColorScale] do. [ColorScale.Color] still answers, with the colour
// the quantity has at the low end of the second reading — at full certainty,
// in a matrix's first column — so a layer that names no second column gets the
// univariate reading rather than a failure, and nothing that paints through
// the existing interface has to learn anything. See
// docs/adr/0067-a-bivariate-colour-channel.md.
type BivariateColorScale interface {
	ColorScale

	// ColorAt returns the colour of a mark whose quantity is v and whose
	// second reading is u. A NaN or infinite value of either is the undefined
	// colour.
	ColorAt(v, u float64) ir.Color

	// TrainSecond extends the second reading's domain to include us, ignoring
	// NaN and infinities. Calling it repeatedly accumulates.
	TrainSecond(us ...float64)

	// SecondDomain reports the second reading's trained domain, or (0, 0)
	// for a scale that has not been trained on one.
	SecondDomain() (lo, hi float64)

	// KeyCells returns the scale's key as cells in data space: one rectangle
	// of (v, u) per colour it paints. The slice is freshly allocated.
	KeyCells() []BivariateCell
}

// BivariateCell is one cell of a bivariate key: the rectangle of values it
// covers and the colour marks in it are painted.
type BivariateCell struct {
	VLo, VHi, ULo, UHi float64
	Color              ir.Color
}

// Bivariate reports whether a colour scale reads two numbers.
func Bivariate(cs ColorScale) (BivariateColorScale, bool) {
	b, ok := cs.(BivariateColorScale)
	return b, ok
}

// The colour kinds of the two bivariate scales.
const (
	// KindVSUP is a value-suppressing uncertainty palette. See [VSUP].
	KindVSUP ColorKind = "vsup"
	// KindBivariate is a matrix of colours over two classed readings. See
	// [BivariateMatrix].
	KindBivariate ColorKind = "bivariate"
)

// vsupNeutral is what an uncertain colour is mixed toward: a light grey, the
// colour that says nothing about the quantity. It is light rather than mid
// grey so that the most uncertain layer reads as faint rather than as a class
// of its own.
var vsupNeutral = ir.RGB(0xd9, 0xd9, 0xd9)

// vsupSuppression is how far the most uncertain layer is mixed toward the
// neutral. Not all the way: the most uncertain layer is mostly neutral but
// still carries a hue, because the value has been suppressed to one class
// rather than erased, and a reader should see which end of the ramp it was.
const vsupSuppression = 0.7

// VSUP returns a value-suppressing uncertainty palette: a colour scale that
// gives a quantity fewer distinguishable colours the less certain it is.
//
// Correll, Moritz and Heer introduced it in 2018 and measured what it fixes.
// On a conventional map readers read precision off colours that carry none; a
// palette that refuses to draw distinctions the data cannot support stops
// them. The key is a tree: at the certain end the quantity is cut into classes
// classes, and each of layers layers of uncertainty above that has half as
// many, so the most uncertain values of a scale with classes = 2^(layers−1)
// share one colour.
//
// Each layer is also mixed toward a light neutral grey the further up it is,
// so that uncertainty reads as fading as well as merging. Opacity is not used:
// a faded mark confounds with the background and with overlapping marks, which
// is the misreading the palette exists to avoid.
//
// The quantity takes the colour options every ramp does — [ColorDomain],
// [ColorReverse], [ColorLog] — and is cut into equal classes in the space the
// ramp runs in, as [Quantize] cuts it. The second reading is cut into equal
// layers over its trained domain; a scale never trained on one treats every
// mark as certain.
//
// It is a [ClassedColorScale] over its certain layer, so a layer that names no
// second column draws a classed colourbar.
//
// A nil ramp uses [palette.DefaultRamp]. A class count or a layer count below
// one is one.
func VSUP(ramp palette.Ramp, classes, layers int, opts ...ColorOption) BivariateColorScale {
	classes, layers = max(classes, 1), max(layers, 1)
	return &vsup{
		classed: classed{kind: KindQuantize, base: newColorScale(ramp, false, opts), n: classes},
		layers:  layers,
	}
}

type vsup struct {
	classed
	layers int
	second domainRange
}

func (s *vsup) TrainSecond(us ...float64) { s.second.Train(us...) }

func (s *vsup) SecondDomain() (float64, float64) { return secondDomain(s.second) }

// layerOf is the uncertainty layer u falls in, zero the most certain.
func (s *vsup) layerOf(u float64) int {
	lo, hi := s.SecondDomain()
	if !(hi > lo) {
		return 0
	}
	return classOf((u-lo)/(hi-lo), s.layers)
}

// binsAt is how many value classes layer l has.
func (s *vsup) binsAt(l int) int { return max(1, s.n>>l) }

func (s *vsup) Color(v float64) ir.Color { return s.colorIn(v, 0) }

func (s *vsup) ColorAt(v, u float64) ir.Color {
	if !finiteF(u) {
		return s.base.undef
	}
	return s.colorIn(v, s.layerOf(u))
}

func (s *vsup) colorIn(v float64, layer int) ir.Color {
	if !finiteF(v) || !s.base.defined(v) {
		return s.base.undef
	}
	bins := s.binsAt(layer)
	lo, hi := s.classed.Domain()
	b := classOf(s.base.positionIn(lo, hi, v), bins)
	col := s.base.ramp.At((float64(b) + 0.5) / float64(bins))
	return palette.Lerp(col, vsupNeutral, float64(layer)/float64(s.layers)*vsupSuppression)
}

func (s *vsup) KeyCells() []BivariateCell {
	vlo, vhi := s.classed.Domain()
	ulo, uhi := s.SecondDomain()
	out := make([]BivariateCell, 0, 2*s.n)
	for l := 0; l < s.layers; l++ {
		u0 := ulo + (uhi-ulo)*float64(l)/float64(s.layers)
		u1 := ulo + (uhi-ulo)*float64(l+1)/float64(s.layers)
		bins := s.binsAt(l)
		for b := 0; b < bins; b++ {
			v0 := s.base.valueIn(vlo, vhi, float64(b)/float64(bins))
			v1 := s.base.valueIn(vlo, vhi, float64(b+1)/float64(bins))
			col := palette.Lerp(s.base.ramp.At((float64(b)+0.5)/float64(bins)), vsupNeutral,
				float64(l)/float64(s.layers)*vsupSuppression)
			out = append(out, BivariateCell{VLo: v0, VHi: v1, ULo: u0, UHi: u1, Color: col})
		}
	}
	return out
}

func (s *vsup) DescribeColor() ColorDesc {
	d := s.classed.DescribeColor()
	d.Kind = KindVSUP
	d.Classes, d.Layers = s.n, s.layers
	return d
}

// BivariateMatrix returns a colour scale that cuts two readings into equal
// classes and paints each pair of classes one colour of a matrix:
// colors[i][j] is the colour of the i-th class of the quantity and the j-th
// class of the second reading, both counted from the low end. The matrix may
// be rectangular, which is a different number of classes per reading.
//
// It is the bivariate choropleth's scale — the 3×3 square of two variables —
// and, with the class on one reading and how purely a bin holds it on the
// other, the scale a multi-class hexbin is painted with.
//
// The colours are given rather than blended from two ramps. A colour mixed
// from two ramps corresponds to an entry of neither and so names no value, and
// a published scheme — [palette.BivariateBlueRed] — is designed square by
// square so that both readings survive. docs/adr/0067 declines blending for
// that reason.
//
// The quantity takes [ColorDomain] and the transforms; [ColorReverse] has no
// ramp to reverse and is ignored. The second reading is cut over its trained
// domain; a scale never trained on one paints the first column.
//
// An empty or ragged matrix panics: the matrix is a literal in the caller's
// source, where a malformed one is a programming error — the line
// [NumberFormat] draws for a malformed spec written in Go. A matrix read out of
// a document goes through [ColorFromDesc], which returns an error instead.
func BivariateMatrix(colors [][]ir.Color, opts ...ColorOption) BivariateColorScale {
	if err := checkMatrix(colors); err != nil {
		panic(err.Error())
	}
	m := make([][]ir.Color, len(colors))
	for i := range colors {
		m[i] = append([]ir.Color(nil), colors[i]...)
	}
	base := newColorScale(palette.Ramp(columnOf(m, 0)), false, opts)
	return &matrix{
		classed: classed{kind: KindQuantize, base: base, n: len(m)},
		colors:  m,
	}
}

func checkMatrix(colors [][]ir.Color) error {
	if len(colors) == 0 || len(colors[0]) == 0 {
		return fmt.Errorf("figure/scale: a bivariate matrix needs at least one colour")
	}
	for i, row := range colors {
		if len(row) != len(colors[0]) {
			return fmt.Errorf("figure/scale: bivariate matrix row %d has %d colours and row 0 has %d", i, len(row), len(colors[0]))
		}
	}
	return nil
}

func columnOf(m [][]ir.Color, j int) []ir.Color {
	out := make([]ir.Color, len(m))
	for i := range m {
		out[i] = m[i][j]
	}
	return out
}

type matrix struct {
	classed
	colors [][]ir.Color
	second domainRange
}

func (s *matrix) TrainSecond(us ...float64) { s.second.Train(us...) }

func (s *matrix) SecondDomain() (float64, float64) { return secondDomain(s.second) }

func (s *matrix) Color(v float64) ir.Color { return s.at(v, 0) }

func (s *matrix) ColorAt(v, u float64) ir.Color {
	if !finiteF(u) {
		return s.base.undef
	}
	cols := len(s.colors[0])
	lo, hi := s.SecondDomain()
	j := 0
	if hi > lo {
		j = classOf((u-lo)/(hi-lo), cols)
	}
	return s.at(v, j)
}

func (s *matrix) at(v float64, j int) ir.Color {
	if !finiteF(v) || !s.base.defined(v) {
		return s.base.undef
	}
	lo, hi := s.classed.Domain()
	return s.colors[classOf(s.base.positionIn(lo, hi, v), s.n)][j]
}

func (s *matrix) KeyCells() []BivariateCell {
	vlo, vhi := s.classed.Domain()
	ulo, uhi := s.SecondDomain()
	rows, cols := len(s.colors), len(s.colors[0])
	out := make([]BivariateCell, 0, rows*cols)
	for i := 0; i < rows; i++ {
		v0 := s.base.valueIn(vlo, vhi, float64(i)/float64(rows))
		v1 := s.base.valueIn(vlo, vhi, float64(i+1)/float64(rows))
		for j := 0; j < cols; j++ {
			out = append(out, BivariateCell{
				VLo: v0, VHi: v1,
				ULo:   ulo + (uhi-ulo)*float64(j)/float64(cols),
				UHi:   ulo + (uhi-ulo)*float64(j+1)/float64(cols),
				Color: s.colors[i][j],
			})
		}
	}
	return out
}

// DescribeColor writes the matrix by name where it is registered, and as its
// colours row-major otherwise, with the row count in Classes.
func (s *matrix) DescribeColor() ColorDesc {
	d := s.classed.DescribeColor()
	d.Kind = KindBivariate
	d.Reverse = false
	d.Ramp, d.Colors = "", nil
	d.Classes = len(s.colors)
	if name, ok := palette.MatrixName(s.colors); ok {
		d.Ramp = name
		return d
	}
	for _, row := range s.colors {
		d.Colors = append(d.Colors, row...)
	}
	return d
}

// matrixFromDesc rebuilds a bivariate matrix scale from its description.
func matrixFromDesc(d ColorDesc, opts []ColorOption) (ColorScale, error) {
	var m [][]ir.Color
	if d.Ramp != "" {
		named, ok := palette.MatrixByName(d.Ramp)
		if !ok {
			return nil, fmt.Errorf("figure/scale: unknown bivariate matrix %q", d.Ramp)
		}
		m = named
	} else {
		if d.Classes < 1 || len(d.Colors) == 0 || len(d.Colors)%d.Classes != 0 {
			return nil, fmt.Errorf("figure/scale: a bivariate matrix of %d colours cannot have %d rows", len(d.Colors), d.Classes)
		}
		cols := len(d.Colors) / d.Classes
		for i := 0; i < d.Classes; i++ {
			m = append(m, d.Colors[i*cols:(i+1)*cols])
		}
	}
	if err := checkMatrix(m); err != nil {
		return nil, err
	}
	return BivariateMatrix(m, opts...), nil
}

// secondDomain is a second reading's domain as reported: its trained extent,
// or (0, 0) for one never trained.
func secondDomain(d domainRange) (float64, float64) {
	if !d.trained {
		return 0, 0
	}
	return d.dmin, d.dmax
}

// classOf is which of n equal classes a position in [0, 1] falls in, with the
// ends clamped into the outermost classes.
func classOf(t float64, n int) int {
	i := int(math.Floor(t * float64(n)))
	if i >= n {
		return n - 1
	}
	if i < 0 {
		return 0
	}
	return i
}

func finiteF(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

var (
	_ BivariateColorScale = (*vsup)(nil)
	_ ClassedColorScale   = (*vsup)(nil)
	_ ColorDescriber      = (*vsup)(nil)
	_ BivariateColorScale = (*matrix)(nil)
	_ ClassedColorScale   = (*matrix)(nil)
	_ ColorDescriber      = (*matrix)(nil)
)

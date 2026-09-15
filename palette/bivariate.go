package palette

import (
	"sort"

	"github.com/timzifer/figure/ir"
)

// A bivariate matrix is a grid of colours, one per pair of classes of two
// readings: m[i][j] is the colour of the i-th class of the first reading and
// the j-th class of the second, both counted from the low end.
//
// It is a grid of explicit colours rather than two ramps blended, because a
// colour mixed from two ramps corresponds to an entry of neither and so names
// no value — which is the reason docs/adr/0067-a-bivariate-colour-channel.md
// declines blending. A published scheme is designed square by square so that
// both readings can still be recovered from it, and that design is the point.

// BivariateBlueRed is Joshua Stevens' 3×3 bivariate choropleth scheme.
//
// The orientation is rows for the first reading and columns for the second,
// both ascending: the first row is the first reading low, running from the
// neutral grey at the second reading low to teal at the second reading high;
// the last row is the first reading high, running from pink to deep blue. So
// the first reading grows toward pink, the second toward teal, and both high
// together is the dark corner.
var BivariateBlueRed = [][]ir.Color{
	{ir.RGB(0xe8, 0xe8, 0xe8), ir.RGB(0xac, 0xe4, 0xe4), ir.RGB(0x5a, 0xc8, 0xc8)},
	{ir.RGB(0xdf, 0xb0, 0xd6), ir.RGB(0xa5, 0xad, 0xd3), ir.RGB(0x56, 0x98, 0xb9)},
	{ir.RGB(0xbe, 0x64, 0xac), ir.RGB(0x8c, 0x62, 0xaa), ir.RGB(0x3b, 0x49, 0x94)},
}

// Matrices are registered by name for the reason ramps are: a spec carries
// "bluered" rather than nine hex triples.
var matrices = map[string][][]ir.Color{
	"bluered": BivariateBlueRed,
}

// RegisterMatrix adds a bivariate matrix under a name, replacing any already
// there. A matrix that is empty or ragged is not registered, because no scale
// can be built from it.
func RegisterMatrix(name string, m [][]ir.Color) {
	if name == "" || len(m) == 0 || len(m[0]) == 0 {
		return
	}
	for _, row := range m {
		if len(row) != len(m[0]) {
			return
		}
	}
	matrices[name] = m
}

// MatrixByName looks up a registered bivariate matrix.
func MatrixByName(name string) ([][]ir.Color, bool) {
	m, ok := matrices[name]
	return m, ok
}

// MatrixName reports the name a matrix was registered under, comparing the
// colours rather than the slice, so that a copy of a registered matrix is still
// written down by name.
func MatrixName(m [][]ir.Color) (string, bool) {
	names := make([]string, 0, len(matrices))
	for name := range matrices {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if sameMatrix(matrices[name], m) {
			return name, true
		}
	}
	return "", false
}

func sameMatrix(a, b [][]ir.Color) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameColors(a[i], b[i]) {
			return false
		}
	}
	return true
}

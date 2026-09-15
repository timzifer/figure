package geom

import (
	"errors"
	"fmt"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/scale"
)

// ErrNotBivariate reports a second colour column named for a layer whose
// colour scale reads one number. The column would be read and never painted,
// which is a reading the chart silently leaves out — so it is refused, by
// name, rather than ignored.
var ErrNotBivariate = errors.New("figure/geom: a second colour column needs a bivariate colour scale")

// UncertaintyBy names the column a bivariate colour scale reads its second
// reading from, beside the one [ColorBy] names.
//
//	geom.Rect(src, geom.X("lon"), geom.Y("lat"),
//	    geom.ColorBy("mean", scale.VSUP(palette.Viridis, 8, 4)),
//	    geom.UncertaintyBy("sd"))
//
// Under [scale.VSUP] it is the uncertainty, and the scale takes resolution away
// from the first reading as it grows: two means that differ by less than the
// data can support are painted one colour. Under [scale.BivariateMatrix] it is
// simply the second variable of the square. The name is the first case's,
// because that is the one this channel exists for — a colour reads as a
// measurement whether or not anybody measured it, and this is how a chart says
// how well.
//
// It is read only by a scale that implements [scale.BivariateColorScale], and
// only by marks that paint one colour per mark — a scatter, a rect, a bar, a
// hexbin's cells. Named for a scale that reads one number it is
// [ErrNotBivariate]; named for a line or a step it is [ErrRampOnPath], because
// a path changes colour where a reading crosses a boundary and two readings
// cross theirs in different places. The guide is a key rather than a bar,
// with the second reading's name on its other axis. See
// docs/adr/0067-a-bivariate-colour-channel.md.
func UncertaintyBy(col string) Option { return func(c *config) { c.secondCol = col } }

// secondColumn reads the second colour column, checked against the scale that
// is to paint from it and against the layer's own length.
func (c config) secondColumn(src data.Source, n int) ([]float64, error) {
	if _, ok := scale.Bivariate(c.colorScale); !ok || c.colorCol == "" {
		return nil, fmt.Errorf("%w: %q; give geom.ColorBy a scale.VSUP or a scale.BivariateMatrix", ErrNotBivariate, c.secondCol)
	}
	v, err := column(src, c.secondCol, nil)
	if err != nil {
		return nil, err
	}
	if len(v) != n {
		return nil, errLength(c.colorCol, c.secondCol, n, len(v))
	}
	return v, nil
}

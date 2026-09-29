package geom

import (
	"errors"
	"fmt"
	"math"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/theme"
)

// DirectionBy colours each row by which way it went: rising when its to
// column is at or above its from column, falling otherwise — a candle's rule,
// with a tie rising. With from empty, a row is compared with the previous
// row's to, which is a candle's [SincePrevious]; the first row rises.
//
//	geom.Bar(src, geom.X("start"), geom.Y("volume"), geom.DirectionBy("open", "close"))
//
// It is a colour channel, so every mark that takes [ColorBy] takes it — a
// bar, a rect, a scatter, an error bar, a label, an area, and a line or step,
// whose stretches are drawn rising or falling — and a layer given both is an
// error: a row has one colour. The colours and labels are a candle's:
// [Rising] and [Falling], or the theme's and "rising" and "falling", so a
// volume track under a candle layer paints in the candle's colours without
// being told them, and the legend, which merges entries by label, lists the
// directions once. A row missing either value has no direction and is drawn in
// no colour. See docs/adr/0091-a-direction-is-a-colour-channel-any-mark-can-take.md.
func DirectionBy(from, to string) Option {
	return func(c *config) { c.dirFrom, c.dirTo = from, to }
}

// ErrColorAndDirection reports a layer given both [ColorBy] and
// [DirectionBy].
var ErrColorAndDirection = errors.New("figure/geom: a layer is coloured by ColorBy or by DirectionBy, not both")

// directionScale is the colour scale a [DirectionBy] layer paints through: a
// discrete scale of two labels, rising and falling, so that everything written
// for a discrete colour — batching by colour, a path coloured in stretches,
// one legend entry per label — works without knowing what a direction is. Its
// colours are the layer's own where it named them, and otherwise the theme's,
// bound once per render by [directionScale.BindTheme].
type directionScale struct {
	rising, falling candleSide
	theme           struct{ rising, falling ir.Color }
}

func newDirectionScale(c config) *directionScale {
	d := &directionScale{rising: c.rising, falling: c.falling}
	d.theme.rising, d.theme.falling = candleRising, candleFalling
	return d
}

// BindTheme takes the direction colours of the theme a render draws in. Render
// calls it before training, so a chart switched to another theme repaints its
// directions with it.
func (d *directionScale) BindTheme(t theme.Theme) {
	if t.Rising.A != 0 {
		d.theme.rising = t.Rising
	}
	if t.Falling.A != 0 {
		d.theme.falling = t.Falling
	}
}

// The codes a row's direction is carried in, as its colour value.
const (
	codeRising  = 0
	codeFalling = 1
)

func (d *directionScale) Train(...float64)           {}
func (d *directionScale) Domain() (float64, float64) { return codeRising, codeFalling }

func (d *directionScale) Color(v float64) ir.Color {
	switch v {
	case codeRising:
		if d.rising.color != nil {
			return *d.rising.color
		}
		return d.theme.rising
	case codeFalling:
		if d.falling.color != nil {
			return *d.falling.color
		}
		return d.theme.falling
	}
	return ir.Transparent
}

func (d *directionScale) labels() (rising, falling string) {
	rising, falling = d.rising.label, d.falling.label
	if rising == "" {
		rising = "rising"
	}
	if falling == "" {
		falling = "falling"
	}
	return rising, falling
}

func (d *directionScale) Encode(label string) float64 {
	r, f := d.labels()
	switch label {
	case r:
		return codeRising
	case f:
		return codeFalling
	}
	return math.NaN()
}

func (d *directionScale) ColorOf(label string) ir.Color { return d.Color(d.Encode(label)) }

func (d *directionScale) Labels() []string {
	r, f := d.labels()
	return []string{r, f}
}

// directionCodes is each row's direction as a colour value: codeRising,
// codeFalling, or NaN for a row missing either value.
func (c config) directionCodes(src data.Source, n int) ([]float64, error) {
	to, err := column(src, c.dirTo, nil)
	if err != nil {
		return nil, err
	}
	var from []float64
	if c.dirFrom != "" {
		if from, err = column(src, c.dirFrom, nil); err != nil {
			return nil, err
		}
	}
	if len(to) != n || (from != nil && len(from) != n) {
		return nil, fmt.Errorf("figure/geom: direction columns %q and %q have %d and %d rows, and the plotted columns %d",
			c.dirFrom, c.dirTo, len(from), len(to), n)
	}
	out := make([]float64, n)
	prev, have := 0.0, false
	for i, v := range to {
		ref := prev
		if from != nil {
			ref = from[i]
		}
		switch {
		case !finite(v) || (from != nil && !finite(ref)):
			out[i] = math.NaN()
		case from == nil && !have:
			out[i] = codeRising // the first row has nothing to fall from
		case v >= ref:
			out[i] = codeRising
		default:
			out[i] = codeFalling
		}
		if finite(v) {
			prev, have = v, true
		}
	}
	return out, nil
}

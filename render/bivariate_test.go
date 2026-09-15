package render_test

import (
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

func uncertain(cs scale.ColorScale) geom.Geom {
	src := data.Float64Columns(map[string][]float64{
		"x":  {0, 1, 2, 3, 0, 1, 2, 3},
		"y":  {0, 0, 0, 0, 1, 1, 1, 1},
		"v":  {10, 20, 30, 40, 10, 20, 30, 40},
		"sd": {0, 0, 0, 0, 9, 9, 9, 9},
	})
	return geom.Rect(src, geom.X("x"), geom.Y("y"), geom.ColorBy("v", cs), geom.UncertaintyBy("sd"))
}

// A layer painted from two readings gets a key, not a bar: one fill per cell
// of the scale's key, no gradient, and the second reading's name written
// beside it.
func TestABivariateLayerGetsAKeyRatherThanABar(t *testing.T) {
	cs := scale.VSUP(palette.Viridis, 4, 3)
	rec := draw(t, chart(uncertain(cs)))
	if n := len(gradients(rec)); n != 0 {
		t.Errorf("drew %d gradient colourbars for a bivariate layer, want none", n)
	}
	if !hasText(rec, "sd") {
		t.Errorf("the key does not name its second reading: %v", texts(rec))
	}
	seen := solidFills(rec)
	for _, c := range cs.KeyCells() {
		if !seen[c.Color] {
			t.Errorf("the key has no cell in %v", c.Color)
		}
	}
}

// Without its second column the same scale is an ordinary classed scale, and
// a layer that names none draws the classed bar it always would have.
func TestAVSUPWithoutASecondColumnIsAClassedBar(t *testing.T) {
	rec := draw(t, chart(colored(scale.VSUP(palette.Viridis, 4, 3))))
	if hasText(rec, "sd") {
		t.Errorf("a univariate layer drew a key: %v", texts(rec))
	}
	if len(solidFills(rec)) < 4 {
		t.Errorf("a univariate VSUP layer did not draw its classed bar")
	}
}

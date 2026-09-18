package geom

import (
	"testing"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/theme"
)

// hatchConfig is the config a layer built with opts would carry.
func hatchConfig(opts ...Option) config {
	c := config{}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// TestHatchColorIsUsedAsGiven is the reason the option exists: a layer that
// names an ink means that ink, not a mix of it and the mark's own colour. A
// status band whose fill says "paused" and whose hatch says "but somebody
// worked through it" needs the hatch to be the green the reader already knows
// from the production band — two thirds of the way there from black is not
// that green.
func TestHatchColorIsUsedAsGiven(t *testing.T) {
	f := Frame{Theme: theme.Light}
	green := ir.RGBA(0x2E, 0xA0, 0x43, 0xFF)
	c := hatchConfig(Hatch(ir.HatchDiagonal), HatchColor(green))

	got := c.hatchingOf(f, 0, ir.RGBA(0, 0, 0, 0xFF)).Line.Color
	if got != green {
		t.Errorf("hatch ink = %v, want the colour the layer named, %v", got, green)
	}
}

// TestHatchColorLeavesTheDefaultAlone: without the option the ink is still the
// theme mix, so every chart drawn before the option existed draws the same.
func TestHatchColorLeavesTheDefaultAlone(t *testing.T) {
	f := Frame{Theme: theme.Light}
	base := ir.RGBA(0, 0, 0, 0xFF)
	plain := hatchConfig(Hatch(ir.HatchDiagonal))
	named := hatchConfig(Hatch(ir.HatchDiagonal), HatchColor(ir.RGBA(0x2E, 0xA0, 0x43, 0xFF)))

	if plain.hatchingOf(f, 0, base).Line.Color == named.hatchingOf(f, 0, base).Line.Color {
		t.Fatal("HatchColor changed nothing about the ink")
	}
	if got := plain.hatchingOf(f, 0, base).Line.Color; got.A == 0 {
		t.Errorf("the default ink is invisible: %v", got)
	}
}

// TestHatchWidthScalesTheThemeStroke: the option is a factor and not an
// absolute, so a chart drawn at half size keeps its proportions.
func TestHatchWidthScalesTheThemeStroke(t *testing.T) {
	f := Frame{Theme: theme.Light}
	base := ir.RGBA(0xFF, 0, 0, 0xFF)
	plain := hatchConfig(Hatch(ir.HatchDiagonal)).hatchingOf(f, 0, base).Line.Width
	thick := hatchConfig(Hatch(ir.HatchDiagonal), HatchWidth(3)).hatchingOf(f, 0, base).Line.Width

	if want := plain * 3; thick != want {
		t.Errorf("hatch width = %v, want %v", thick, want)
	}
}

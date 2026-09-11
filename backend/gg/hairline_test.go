package gg_test

import (
	"image/color"
	"testing"

	ggbackend "github.com/timzifer/figure/backend/gg"
	"github.com/timzifer/figure/ir"
)

// A closed box stroked thinner than a pixel is four edges and nothing else.
//
// gg v0.52.5 draws a stroke under a pixel wide as a hairline, snapping each
// horizontal and vertical segment to a pixel centre — and to do it, starts a
// new subpath at the snapped position. A ClosePath afterwards then closes to
// that new start rather than the box's own, joining the last corner to the
// wrong point: a long diagonal across the inside of the box. It is what drew a
// line across the backdrop of every three-dimensional chart, whose cube is
// stroked at 0.75 px, and it needs no data at all to appear.
//
// The shape is a wall of the cube as the camera sees it: one vertical edge,
// which is snapped, between slanted ones, which are not. That is what makes the
// bug show. On an axis-aligned box every edge is snapped, so the close returns
// to a point on the same edge and draws nothing extra; it takes a snapped edge
// in the middle of the path for the close to go back to the middle instead of
// to the start. The corners are off the pixel grid as well, because on it the
// snap is a no-op.
func TestAHairlineBoxHasNoDiagonal(t *testing.T) {
	for _, width := range []float32{0.5, 0.75, 1, 1.5} {
		s := ggbackend.NewSurface()
		b, err := s.Open(ir.Surface{WidthPx: 100, HeightPx: 100, DPR: 1})
		if err != nil {
			t.Fatal(err)
		}
		var wall ir.Path
		wall.MoveTo(10.3, 10.3)
		wall.LineTo(80.3, 20.3) // slanted
		wall.LineTo(80.3, 80.3) // vertical: snapped, and a new subpath starts
		wall.LineTo(10.3, 70.3) // slanted
		wall.Close()            // must go back to (10.3, 10.3)
		b.StrokePath(&wall, ir.Stroke{Width: width, Color: ir.RGB(0, 0, 0)})
		if err := b.Flush(); err != nil {
			t.Fatal(err)
		}

		// The inside of the wall, kept well clear of its edges: nothing its four
		// edges draw lands here, and a close to (80.5, 20.3) instead of to the
		// start runs straight through it.
		img := s.Image()
		inked := 0
		for y := 32; y < 60; y++ {
			for x := 25; x < 65; x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a != 0 && img.At(x, y) != (color.RGBA{}) {
					inked++
				}
			}
		}
		if inked > 0 {
			t.Errorf("a box stroked at %.2f px put ink on %d pixels inside it", width, inked)
		}
		_ = s.Close()
	}
}

//go:build !race

package three

// Apart from overlay_test.go because the race detector allocates on figure's
// behalf and would fail this for nothing figure did. See AGENTS.md.

import (
	"testing"

	"github.com/timzifer/figure/internal/irtest"
)

// A scene redrawn with an overlay installed allocates no more than the same
// scene without one, which is what keeps the view list on the pooled sink
// rather than made per frame — one slice per frame would be invisible in a
// benchmark and would show up on every figure once the views multiplied.
func TestAnOverlayDoesNotAllocatePerFrame(t *testing.T) {
	frame := func(ov Overlay) float64 {
		p := New(Size(640, 240), Columns(2)).Scene(surfaceScene(8, 8)).Add(
			View{Camera: Home()},
			View{Camera: LookAt(Elevation(1.45))},
		)
		if ov != nil {
			p.Overlay(ov)
		}
		live, err := p.Live(irtest.NullTarget())
		if err != nil {
			t.Fatal(err)
		}
		defer live.Close()

		if err := live.Draw(); err != nil {
			t.Fatal(err)
		}
		return testing.AllocsPerRun(20, func() {
			live.SetCamera(0, Orbit(live.CameraOf(0), 0.01, 0))
			if err := live.Draw(); err != nil {
				t.Fatal(err)
			}
		})
	}

	plain := frame(nil)
	with := frame(&Highlight{Data: []Point3{{X: 0.5, Y: 0.5, Z: 0.5}}, View: -1})
	if with > plain {
		t.Errorf("a frame with an overlay allocated %.0f times against %.0f without one", with, plain)
	}
}

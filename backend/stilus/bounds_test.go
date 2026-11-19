package stilus_test

import (
	"io"
	"math"
	"strings"
	"testing"

	"github.com/timzifer/figure/backend/stilus"
	"github.com/timzifer/figure/ir"
)

// A chart's size can come from a document somebody else wrote. A size the
// backend cannot allocate must be an error from Open, not an OOM, an int
// overflow or a panic inside image.NewRGBA.
func TestASurfaceTooLargeToAllocateIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    ir.Surface
		want string
	}{
		{"million square", ir.Surface{WidthPx: 1_000_000, HeightPx: 1_000_000, DPR: 1}, "larger than"},
		{"one side too long", ir.Surface{WidthPx: stilus.MaxSide + 1, HeightPx: 1, DPR: 1}, "larger than"},
		{"too many pixels", ir.Surface{WidthPx: 20000, HeightPx: 20000, DPR: 1}, "larger than"},
		{"huge ratio", ir.Surface{WidthPx: 800, HeightPx: 500, DPR: 1e9}, "larger than"},
		{"ratio past int", ir.Surface{WidthPx: 800, HeightPx: 500, DPR: 1e300}, "larger than"},
		{"infinite ratio", ir.Surface{WidthPx: 800, HeightPx: 500, DPR: math.Inf(1)}, "finite"},
		{"NaN ratio", ir.Surface{WidthPx: 800, HeightPx: 500, DPR: math.NaN()}, "finite"},
		{"rounds to nothing", ir.Surface{WidthPx: 1, HeightPx: 1, DPR: 0.1}, "less than one pixel"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := stilus.Writer(io.Discard, stilus.FormatPNG).Open(tc.s)
			if err == nil {
				t.Fatalf("Open(%+v) succeeded", tc.s)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Open(%+v) = %v, want an error mentioning %q", tc.s, err, tc.want)
			}
		})
	}
}

// The bounds must not refuse a size somebody really prints at, and a
// non-positive ratio still means one, as it always has.
func TestALargeButReasonableSurfaceIsAccepted(t *testing.T) {
	for _, s := range []ir.Surface{
		{WidthPx: 2048, HeightPx: 2048, DPR: 2},
		{WidthPx: 800, HeightPx: 500, DPR: 0},
		{WidthPx: 800, HeightPx: 500, DPR: -1},
	} {
		if _, err := stilus.Writer(io.Discard, stilus.FormatPNG).Open(s); err != nil {
			t.Errorf("Open(%+v): %v", s, err)
		}
	}
}

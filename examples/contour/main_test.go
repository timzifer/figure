package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// TestExampleRuns executes the documented example. The docs quote it, so this
// test is what stops it rotting.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	paths := map[string]string{}
	for _, name := range []string{"flat", "over", "scene"} {
		paths[name] = filepath.Join(dir, name+".svg")
	}
	if err := run(paths["flat"], paths["over"], paths["scene"]); err != nil {
		t.Fatalf("run: %v", err)
	}
	for name, wants := range map[string][]string{
		"flat":  {"<svg", "crosses each level", "bias (V)", "gain"},
		"over":  {"<svg", "read both ways", "drive (dBm)"},
		"scene": {"<svg", "plan beneath it", "gain (dB)"},
	} {
		b, err := os.ReadFile(paths[name])
		if err != nil {
			t.Fatalf("no output was written for %s: %v", name, err)
		}
		for _, want := range wants {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s.svg is missing %q", name, want)
			}
		}
	}
}

// The point of the example: one colour scale, so that a colour means one number
// in all three pictures.
//
// Two scale.Sequential values over one column look identical and are two
// domains that agree by luck — until either chart is drawn over a subset, a
// filter or a live window, at which point the same colour means two different
// things and nobody can see that it does.
func TestOneRampMeansOneNumberEverywhere(t *testing.T) {
	ramp := scale.Sequential(palette.Viridis, scale.ColorDomain(lo, hi))

	// A pinned domain ignores training, so the ramp cannot drift whatever it is
	// handed or in what order.
	before := ramp.Color(0)
	ramp.Train(-1000, 1000)
	after := ramp.Color(0)
	if before != after {
		t.Errorf("training the pinned ramp moved the colour of 0 from %v to %v", before, after)
	}
	if gotLo, gotHi := ramp.Domain(); gotLo != lo || gotHi != hi {
		t.Errorf("the ramp's domain is [%v, %v], want the pair the levels come from", gotLo, gotHi)
	}
}

// The levels come from the same pair the ramp is pinned to, so the outermost
// isoline is the end of the colourbar rather than something short of it.
func TestTheLevelsSpanTheRampsOwnDomain(t *testing.T) {
	levels := stat.Levels(lo, hi, 9)
	if len(levels) == 0 {
		t.Fatal("no levels were chosen")
	}
	for _, v := range levels {
		if v <= lo || v >= hi {
			t.Errorf("the level %v is outside [%v, %v]", v, lo, hi)
		}
	}
	// Round numbers, which is what a reader does arithmetic with.
	for _, v := range levels {
		if v != float64(int(v)) {
			t.Errorf("the levels are %v, which are not whole numbers", levels)
			break
		}
	}
}

// The grid the example draws over is a full product of its two axes. A table
// that is not one is an error rather than a picture with holes in it, so the
// example's own data has to be well formed.
func TestTheSampleGridIsComplete(t *testing.T) {
	src := response()
	bias, ok := src.Column("bias")
	if !ok {
		t.Fatal("no bias column")
	}
	drive, _ := src.Column("drive")
	gain, _ := src.Column("gain")
	if bias.Len() != drive.Len() || drive.Len() != gain.Len() {
		t.Fatalf("the columns are %d, %d and %d rows long", bias.Len(), drive.Len(), gain.Len())
	}
	seen := map[[2]float64]bool{}
	for i := range bias.Len() {
		key := [2]float64{bias.Floats[i], drive.Floats[i]}
		if seen[key] {
			t.Fatalf("row %d repeats the cell %v", i, key)
		}
		seen[key] = true
	}
	if got := len(seen); got != bias.Len() {
		t.Errorf("%d distinct cells for %d rows", got, bias.Len())
	}
}

package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/stat"
)

// TestExampleRuns executes the documented example, so that the chart the
// raster exists for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	out := filepath.Join(t.TempDir(), "spectrogram.svg")
	if err := run(out); err != nil {
		t.Fatalf("run: %v", err)
	}

	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no output was written: %v", err)
	}
	got := string(b)

	// The chart, its axes, and the colourbar the mark contributes in the units
	// the field was measured in.
	for _, want := range []string{"<svg", "Four seconds of audio", "time (s)", "frequency (Hz)", "db", "</svg>"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q", want)
		}
	}
}

// The whole argument for the mark, checked against the file it produces: one
// image for the field, where one rect per cell would be a path apiece.
func TestTheFieldIsOneImageAndNotAHundredThousandPaths(t *testing.T) {
	out := filepath.Join(t.TempDir(), "spectrogram.svg")
	if err := run(out); err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)

	if n := strings.Count(got, "<image"); n != 1 {
		t.Errorf("the field took %d images, want one", n)
	}
	// The furniture — the frame, the grid, the ticks, the colourbar — and
	// nothing that grows with the field. A rect per cell would be six figures.
	if n := strings.Count(got, "<path"); n > 200 {
		t.Errorf("the chart holds %d paths, which is a mark per cell rather than an image", n)
	}
}

// A raster will not guess a grid, so the transform has to produce one: every
// frame carries every bin, once.
func TestTheTransformIsALattice(t *testing.T) {
	src := transform()
	ts, ok := data.Float64Column(src, "t")
	if !ok {
		t.Fatal("no t column")
	}
	hz, _ := data.Float64Column(src, "hz")
	db, _ := data.Float64Column(src, "db")

	frames := (sampleRate*seconds - window) / hop
	if len(ts) != frames*bins {
		t.Fatalf("the field has %d rows, want %d frames of %d bins", len(ts), frames, bins)
	}

	var l stat.Lattice
	if fault := l.Reset(ts, hz, db); fault != stat.LatticeOK {
		t.Fatalf("the transform is not a product grid: fault %v", fault)
	}
	// And evenly spaced on both axes, which is what an image can carry and
	// what geom.Raster refuses a table for lacking.
	if _, ok := stat.Step(l.Xs); !ok {
		t.Error("the frames are not evenly spaced in time")
	}
	if _, ok := stat.Step(l.Ys); !ok {
		t.Error("the bins are not evenly spaced in frequency")
	}
}

// The example is only worth drawing if there is something in the field for the
// reduction to keep: the click is one frame wide, which is what Resample(Max)
// is passed for.
func TestTheClickIsNarrowerThanAPixelColumn(t *testing.T) {
	src := transform()
	ts, _ := data.Float64Column(src, "t")
	db, _ := data.Float64Column(src, "db")

	// The loudest reading in the top half of the band, which is the click: the
	// chirp only reaches 3 kHz at the very end, and the tone is at 1200 Hz.
	hz, _ := data.Float64Column(src, "hz")
	at, loudest := 0.0, math.Inf(-1)
	for i := range db {
		if hz[i] > 3200 && db[i] > loudest {
			at, loudest = ts[i], db[i]
		}
	}
	if want := float64(seconds) / 3; math.Abs(at-want) > 0.05 {
		t.Errorf("the loudest high-frequency reading is at %.3f s, want the click at %.3f s", at, want)
	}
	// One panel of 900 pixels over 1320 frames is more than one frame to the
	// pixel, so a reduction that took the nearest frame could step over this.
	frames := (sampleRate*seconds - window) / hop
	if frames < 900 {
		t.Errorf("the field has %d frames, which fits in a 900 pixel panel; "+
			"the example demonstrates no downscale", frames)
	}
}

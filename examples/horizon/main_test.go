package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
)

// TestExampleRuns executes the documented example, so that the chart the fold
// exists for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	out := filepath.Join(t.TempDir(), "horizon.svg")
	if err := run(out); err != nil {
		t.Fatalf("run: %v", err)
	}

	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no output was written: %v", err)
	}
	got := string(b)

	// The wall, its panels, and the colourbar that is the ladder the chart
	// gave up: the title, the first and last meter's panel headings, and a
	// band boundary printed in the data's own units.
	for _, want := range []string{"<svg", "Metered load", "M01", "M16", "kw", "</svg>"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q", want)
		}
	}
}

// The chart is only worth drawing if the series exceed a band: a wall of
// horizons whose every reading sits inside band one is a wall of flat colour,
// and the example would be demonstrating nothing.
func TestTheLoadReachesPastTheFirstBand(t *testing.T) {
	src := load()
	kw, ok := data.Float64Column(src, "kw")
	if !ok {
		t.Fatal("no kw column")
	}
	if len(kw) != meters*samples {
		t.Fatalf("the wall has %d readings, want %d", len(kw), meters*samples)
	}

	deepest := 0.0
	for _, v := range kw {
		deepest = math.Max(deepest, math.Abs(v-baseLoad))
	}
	if deepest <= bandHeight {
		t.Errorf("the furthest reading is %v kW from the base load, which is inside one band of %v; "+
			"the example folds nothing", deepest, bandHeight)
	}
}

// Every meter is drawn, and each one carries the same number of readings — a
// facet panel short of rows would be a strip that quietly covered less time
// than the one beside it.
func TestEveryMeterIsWholeAndTheSameLength(t *testing.T) {
	names, ok := data.StringColumn(load(), "meter")
	if !ok {
		t.Fatal("no meter column")
	}
	counts := map[string]int{}
	for _, n := range names {
		counts[n]++
	}
	if len(counts) != meters {
		t.Fatalf("the wall has %d meters, want %d", len(counts), meters)
	}
	for name, n := range counts {
		if n != samples {
			t.Errorf("meter %s carries %d readings, want %d", name, n, samples)
		}
	}
}

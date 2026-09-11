package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "cascade.svg")
	flat := filepath.Join(dir, "flat.svg")
	if err := run(out, flat); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		out:  {"<svg", "A drifting carrier", "frequency (MHz)", "power (dBm)", "sweep"},
		flat: {"<svg", "overlaid", "frequency (MHz)"},
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("no output was written for %s: %v", filepath.Base(path), err)
		}
		for _, want := range wants {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s is missing %q", filepath.Base(path), want)
			}
		}
	}
}

// The cascade is one layer over a long table, which is what makes it a recipe
// rather than a mark: thirty traces, one geom.GroupBy, and no code in the
// library that knows what a cascade is.
func TestTheSweepsAreOneLongTable(t *testing.T) {
	freq, sweep, power := sweeps()
	if len(freq) != len(sweep) || len(sweep) != len(power) {
		t.Fatalf("the columns are %d, %d and %d rows long", len(freq), len(sweep), len(power))
	}
	names := traceNames(sweep)
	distinct := map[string]int{}
	for _, n := range names {
		distinct[n]++
	}
	if len(distinct) != 30 {
		t.Errorf("%d distinct traces, want thirty sweeps", len(distinct))
	}
	for name, n := range distinct {
		if n != 121 {
			t.Errorf("%s has %d bins, want 121", name, n)
		}
	}
}

// The carrier drifts, which is the whole reading the flat chart cannot give.
// If the sample data stopped drifting the example would still draw, and would
// stop being about anything.
func TestTheCarrierDrifts(t *testing.T) {
	freq, sweep, power := sweeps()
	peakOf := func(n float64) float64 {
		best, at := -1e9, 0.0
		for i := range freq {
			if sweep[i] == n && power[i] > best {
				best, at = power[i], freq[i]
			}
		}
		return at
	}
	first, last := peakOf(0), peakOf(29)
	if last-first < 10 {
		t.Errorf("the carrier moved from %v to %v MHz; the cascade is about the drift", first, last)
	}
}

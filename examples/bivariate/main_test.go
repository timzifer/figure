package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the charts a
// bivariate colour channel exists for cannot stop compiling without anyone
// noticing.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "vsup.svg"), filepath.Join(dir, "square.svg"), filepath.Join(dir, "hexbin.svg")}
	if err := run(paths[0], paths[1], paths[2]); err != nil {
		t.Fatalf("run: %v", err)
	}
	wants := [][]string{
		{"<svg", "how well they are known", "sd", "</svg>"},
		{"<svg", "Two rates", "inactivity", "</svg>"},
		{"<svg", "how mixed", "setosa", "virginica", "</svg>"},
	}
	for i, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("no output was written: %v", err)
		}
		for _, want := range wants[i] {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s is missing %q", filepath.Base(path), want)
			}
		}
	}
}

// The field is the chart only if its uncertainty grows across it, from well
// known to barely known.
func TestTheUncertaintyGrowsAcrossTheField(t *testing.T) {
	x, _, _, sd := field()
	lo, hi := 1e9, 0.0
	for i := range x {
		if x[i] == 0 {
			lo = min(lo, sd[i])
		}
		if x[i] == 15 {
			hi = max(hi, sd[i])
		}
	}
	if hi < 5*lo {
		t.Errorf("sd runs from %v to %v across the field; want the right edge far less certain than the left", lo, hi)
	}
}

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the chart a
// scattered sample draws cannot stop compiling or stop producing a chart.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	coverage, rainfall := render(t, dir)
	for path, wants := range map[string][]string{
		coverage: {"<svg", "Which gauge speaks for this ground", "Easting (km)", "</svg>"},
		rainfall: {"<svg", "rainfall, as the nearest gauge measured it", "Rainfall (mm)", "</svg>"},
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("no output was written: %v", err)
		}
		for _, want := range wants {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s is missing %q", filepath.Base(path), want)
			}
		}
		if strings.Contains(string(b), "NaN") {
			t.Errorf("a NaN reached %s", filepath.Base(path))
		}
	}
}

// The difference between the two charts is what the cells are painted from,
// and it is visible in the ink: the coverage map draws every cell in one fill
// and separates them with an outline, while the rainfall map paints each one
// from its own gauge's reading — which is one fill per distinct value, because
// the IR carries one colour per drawing call.
func TestTheRainfallMapPaintsEachCellFromItsOwnReading(t *testing.T) {
	dir := t.TempDir()
	coverage, rainfall := render(t, dir)
	if got := len(fillsIn(t, coverage)); got > 4 {
		t.Errorf("the coverage map fills paths in %d colours, want the layer's own few", got)
	}
	// Twenty gauges, three of whose readings happen to be shared with another.
	if got := len(fillsIn(t, rainfall)); got < 15 {
		t.Errorf("the rainfall map fills paths in %d colours, want one per reading", got)
	}
}

func render(t *testing.T, dir string) (coverage, rainfall string) {
	t.Helper()
	coverage = filepath.Join(dir, "coverage.svg")
	rainfall = filepath.Join(dir, "rainfall.svg")
	if err := run(coverage, rainfall); err != nil {
		t.Fatalf("run: %v", err)
	}
	return coverage, rainfall
}

var pathFill = regexp.MustCompile(`<path[^>]*fill="(#[0-9a-fA-F]{6})"`)

// fillsIn is the set of colours the filled paths of an SVG are painted in.
func fillsIn(t *testing.T, path string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, m := range pathFill.FindAllStringSubmatch(string(b), -1) {
		out[m[1]] = true
	}
	return out
}

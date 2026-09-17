package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// figures is what run writes, and what the doc comment promises.
var figures = []string{"-plain", "-nominal", "-ordinal", "-finished", "-projection"}

// TestExampleRuns executes the documented example, so that the chart the hatch
// channel exists for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	for name, got := range render(t) {
		for _, want := range []string{"<svg", "Units shipped", "</svg>"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s is missing %q", name, want)
			}
		}
	}
}

// TestOnlyTheAskedForFiguresAreHatched is the claim the example is making. A
// hatch reaches a backend as a stroke clipped to the mark it lies over, and
// none of these charts clips anything else inside a mark — so counting clip
// paths asks, cheaply, whether anything was patterned.
//
// The plain figure must have exactly the panel's clip and no more: a chart
// that named no pattern and a theme that installs no ladder draw what they
// drew before hatching existed. The two ladder figures must have more, or the
// example is demonstrating nothing.
func TestOnlyTheAskedForFiguresAreHatched(t *testing.T) {
	out := render(t)
	plain := strings.Count(out["-plain"], "<clipPath")
	if plain != 1 {
		t.Errorf("the plain figure emitted %d clip paths, want just the panel's", plain)
	}
	for _, name := range []string{"-nominal", "-ordinal", "-projection"} {
		if n := strings.Count(out[name], "<clipPath"); n <= plain {
			t.Errorf("%s emitted %d clip paths and the plain figure %d; nothing was hatched",
				name, n, plain)
		}
	}
}

// TestTheProjectionIsNamed guards the half of the projection figure that is
// not the pattern: a hatched band nobody can look up in the legend is a band
// that says only "something is different here".
func TestTheProjectionIsNamed(t *testing.T) {
	if got := render(t)["-projection"]; !strings.Contains(got, "projection") {
		t.Error("the band is drawn but never named, so the legend cannot explain it")
	}
}

// render runs the example into a temporary directory and returns each figure's
// SVG, keyed by the suffix in its name.
func render(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	if err := run(filepath.Join(dir, "hatch.svg")); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := make(map[string]string, len(figures))
	for _, suffix := range figures {
		b, err := os.ReadFile(filepath.Join(dir, "hatch"+suffix+".svg"))
		if err != nil {
			t.Fatalf("no output for %s: %v", suffix, err)
		}
		out[suffix] = string(b)
	}
	return out
}

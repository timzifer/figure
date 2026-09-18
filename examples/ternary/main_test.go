package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the two charts a
// barycentric coord exists for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	soil, qfl := filepath.Join(dir, "soil.svg"), filepath.Join(dir, "qfl.svg")
	if err := run(soil, qfl); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		soil: {"<svg", "Soil texture", "sand", "silt", "clay", "</svg>"},
		qfl:  {"<svg", "Sandstone provenance", "quartz", "feldspar", "lithics", "</svg>"},
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		got := string(b)
		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Errorf("%s is missing %q", filepath.Base(path), want)
			}
		}
		if strings.Contains(got, "NaN") {
			t.Errorf("%s: a NaN reached the SVG", filepath.Base(path))
		}
	}
}

// All three ladders are labelled, which is the whole of what ADR 0070 added.
// Every level appears three times — once per edge — where before the derived
// component's edge carried the lines and no numbers.
func TestAllThreeLaddersAreLabelled(t *testing.T) {
	out := filepath.Join(t.TempDir(), "soil.svg")
	if err := soilTexture(out); err != nil {
		t.Fatalf("soilTexture: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	// 40 and 60 are levels no corner label and no title can spell, and they
	// are far enough from a neighbour that the thinning pass keeps all three.
	for _, level := range []string{">40<", ">60<"} {
		if n := strings.Count(got, level); n != 3 {
			t.Errorf("the level %s is written %d times, want one per edge", level, n)
		}
	}
}

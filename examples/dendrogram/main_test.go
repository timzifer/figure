package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the charts a tidy
// tree exists for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	heat, radial := filepath.Join(dir, "heatmap.svg"), filepath.Join(dir, "radial.svg")
	if err := run(heat, radial); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		heat:   {"<svg", "clustered by sample", "ACT1", "s9", "</svg>"},
		radial: {"<svg", "from the root out", "</svg>"},
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

// The clustering has a structure to find — three conditions — and the claim
// the chart makes is that the tree found it: the leaves come out with each
// condition's samples next to each other, although the table interleaves them.
func TestTheLeavesGroupTheConditions(t *testing.T) {
	_, _, _, leaves := cluster(samples, func(a, b int) float64 {
		return distance(len(genes), func(g int) (float64, float64) {
			return expression(a, g), expression(b, g)
		})
	})
	if len(leaves) != len(samples) {
		t.Fatalf("%d leaves, want %d", len(leaves), len(samples))
	}
	index := map[string]int{}
	for i, s := range samples {
		index[s] = i
	}
	changes := 0
	for i := 1; i < len(leaves); i++ {
		if condition[index[leaves[i]]] != condition[index[leaves[i-1]]] {
			changes++
		}
	}
	if changes != 2 {
		t.Errorf("leaf order %v changes condition %d times, want 2 — three contiguous groups", leaves, changes)
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example. The docs quote it, so this
// test is what stops it rotting.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	paths := map[string]string{}
	for _, name := range []string{"response", "flat", "terrain"} {
		paths[name] = filepath.Join(dir, name+".svg")
	}
	if err := run(paths["response"], paths["flat"], paths["terrain"]); err != nil {
		t.Fatalf("run: %v", err)
	}

	for name, wants := range map[string][]string{
		"response": {"<svg", "Small-signal gain", "bias (V)", "gain (dB)"},
		"flat":     {"<svg", "heatmap", "bias (V)"},
		"terrain":  {"<svg", "coloured by height", "gain (dB)"},
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

// A surface is filled polygons and nothing else: the IR gained nothing for the
// third dimension, so a scene reaches an emitter as the same path fills every
// other chart in the library reaches it as. That is ADR 0056's central claim,
// checked against the one output format that is the reference path.
func TestASurfaceIsDrawnWithOrdinaryPaths(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "response.svg")
	if err := responseSurface(out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	svg := string(b)
	if !strings.Contains(svg, "<path") {
		t.Error("the surface drew no paths at all")
	}
	// Nothing in the document carries a depth, a third coordinate or a
	// transform stack of its own.
	for _, forbidden := range []string{"transform=\"matrix3d", "z-index", "depth="} {
		if strings.Contains(svg, forbidden) {
			t.Errorf("the document carries %q; the IR is meant to have gained nothing", forbidden)
		}
	}
}

// The grid the surface is drawn over is a full product of its two floor axes.
// A table that is not one is an error rather than a picture with holes in it,
// so the example's own data has to be well formed.
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
	for i := 0; i < bias.Len(); i++ {
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

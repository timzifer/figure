package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the chart an edge
// list pays for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	out := filepath.Join(t.TempDir(), "network.svg")
	if err := run(out); err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no output was written: %v", err)
	}
	doc := string(b)
	for _, want := range []string{"<svg", "Who worked with whom", "ana", "jun", "ora", "</svg>"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the chart is missing %q", want)
		}
	}
	if strings.Contains(doc, "NaN") {
		t.Error("a NaN reached the chart")
	}
}

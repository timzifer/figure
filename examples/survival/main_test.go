package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the chart a
// survival estimator exists for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	out := filepath.Join(t.TempDir(), "survival.svg")
	if err := run(out); err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no output was written: %v", err)
	}
	got := string(b)
	for _, want := range []string{"<svg", "6-MP against placebo", "placebo", "</svg>"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q", want)
		}
	}
	if strings.Contains(got, "NaN") {
		t.Error("a NaN reached the SVG")
	}
}

// The risk table is the published one: 21 and 21 at the start, and at ten
// weeks 15 on 6-MP against 8 on placebo.
func TestTheRiskTableIsTheTextbooks(t *testing.T) {
	want := map[string][]int{"6-MP": {21, 15, 8, 4}, "placebo": {21, 8, 2, 0}}
	for _, a := range arms {
		for k, at := range tableAt {
			if got := atRisk(a.weeks, a.events, at); got != want[a.name][k] {
				t.Errorf("%s at week %v: %d at risk, want %d", a.name, at, got, want[a.name][k])
			}
		}
	}
}

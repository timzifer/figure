package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the charts a layered
// graph exists for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	states, build := filepath.Join(dir, "states.svg"), filepath.Join(dir, "build.svg")
	if err := run(states, build); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		states: {"<svg", "from closed and back", "established", "last-ack", "</svg>"},
		build:  {"<svg", "step by step", "checkout", "publish", "</svg>"},
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

// Every state in the table reaches the picture. A layout that dropped the ones
// on a cycle would still render, and the chart would quietly be wrong.
func TestEveryStateIsDrawn(t *testing.T) {
	dir := t.TempDir()
	states := filepath.Join(dir, "states.svg")
	if err := stateChart(states); err != nil {
		t.Fatalf("stateChart: %v", err)
	}
	b, err := os.ReadFile(states)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, s := range append(append([]string{}, transFrom...), transTo...) {
		seen[s] = true
	}
	for name := range seen {
		if !strings.Contains(string(b), ">"+name+"<") {
			t.Errorf("the chart never names %q", name)
		}
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the chart a panel
// with more than two axes draws cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	fleet := filepath.Join(dir, "fleet.svg")
	better := filepath.Join(dir, "better.svg")
	if err := run(fleet, better); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		fleet:  {"<svg", "Sixty cars", "mpg", "power (hp)", "weight (kg)", "year", "</svg>"},
		better: {"<svg", "better-is-up", "mpg", "weight (kg)", "</svg>"},
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

// Every row is a line, so the chart holds one polyline per car — and each of
// them crosses every axis. A chart that dropped rows would still look like a
// parallel-coordinates plot.
func TestEveryCarIsALine(t *testing.T) {
	dir := t.TempDir()
	fleet := filepath.Join(dir, "fleet.svg")
	if err := fleetChart(fleet); err != nil {
		t.Fatalf("fleetChart: %v", err)
	}
	b, err := os.ReadFile(fleet)
	if err != nil {
		t.Fatal(err)
	}
	// Each row is a subpath: an M and three L commands for four axes.
	if got, want := strings.Count(string(b), "M"), cars().Len(); got < want {
		t.Errorf("the chart holds %d subpath starts for %d cars", got, want)
	}
}

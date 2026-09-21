package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the chart a table of
// categorical columns draws cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	flow := filepath.Join(dir, "tickets.svg")
	byOutcome := filepath.Join(dir, "outcome.svg")
	if err := run(flow, byOutcome); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		flow:      {"<svg", "A quarter of support tickets", "channel: phone", "outcome: solved", "</svg>"},
		byOutcome: {"<svg", "escalated share", "</svg>"},
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

// Colouring by the outcome subdivides the ribbons rather than recolouring
// them, so the coloured chart draws strictly more of them. It is the property
// that makes the escalated share readable in every column.
func TestColouringByOutcomeDrawsMoreRibbons(t *testing.T) {
	dir := t.TempDir()
	flow := filepath.Join(dir, "tickets.svg")
	byOutcome := filepath.Join(dir, "outcome.svg")
	if err := run(flow, byOutcome); err != nil {
		t.Fatalf("run: %v", err)
	}
	plain, err := os.ReadFile(flow)
	if err != nil {
		t.Fatal(err)
	}
	split, err := os.ReadFile(byOutcome)
	if err != nil {
		t.Fatal(err)
	}
	// Every ribbon is a cubic pair, so counting the C commands counts them.
	if a, b := strings.Count(string(plain), "C"), strings.Count(string(split), "C"); b <= a {
		t.Errorf("the coloured chart has %d curve commands against %d, want more", b, a)
	}
}

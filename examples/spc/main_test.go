package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timzifer/figure/stat"
)

// TestExampleRuns executes the documented example, so that the chart control
// limits exist for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	out := filepath.Join(t.TempDir(), "spc.svg")
	if err := run(out); err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no output was written: %v", err)
	}
	got := string(b)
	for _, want := range []string{"<svg", "individuals chart", "UCL", "LCL", "</svg>"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q", want)
		}
	}
	if strings.Contains(got, "NaN") {
		t.Error("a NaN reached the SVG")
	}
}

// The chart is only worth drawing if the baseline is in control and the drift
// is caught: no rule fires on the readings the limits came from, and some rule
// fires after the drift begins and not before it.
func TestTheBaselineIsQuietAndTheDriftIsCaught(t *testing.T) {
	w := weights()
	limits, _ := stat.LimitsIMR(w[:baseline])
	if f := stat.AppendRunRules(nil, w[:baseline], limits); len(f) != 0 {
		t.Errorf("the baseline breaks rules %v; it is meant to be the process in control", f)
	}
	flags := stat.AppendRunRules(nil, w[baseline:], limits)
	if len(flags) == 0 {
		t.Fatal("the drift was not caught")
	}
	for _, f := range flags {
		if baseline+f.Row < 42 {
			t.Errorf("sample %d flagged by rule %d before the drift began", baseline+f.Row+1, f.Rule)
		}
	}
}

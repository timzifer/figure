package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// TestExampleRuns executes the documented example, so that the chart a
// probability scale exists for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	out := filepath.Join(t.TempDir(), "weibull.svg")
	if err := run(out); err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no output was written: %v", err)
	}
	got := string(b)
	// The title, the ladder in per cent, and a decade of hours.
	for _, want := range []string{"<svg", "Weibull paper", "50%", "99%", "1000", "</svg>"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q", want)
		}
	}
	if strings.Contains(got, "NaN") {
		t.Error("a NaN reached the SVG")
	}
}

// The claim the chart makes: on CLogLog paper against log hours, the sample's
// median ranks lie on a line whose slope is β. A rank regression that did not
// recover β would mean the axis is warped by the wrong function.
func TestTheRanksLieOnALineOfSlopeBeta(t *testing.T) {
	ranks := stat.MedianRank(failures())
	var sx, sy, sxx, sxy float64
	for _, r := range ranks {
		x, y := math.Log(r.X), scale.CLogLog.Apply(r.Y)
		sx, sy, sxx, sxy = sx+x, sy+y, sxx+x*x, sxy+x*y
	}
	n := float64(len(ranks))
	slope := (n*sxy - sx*sy) / (n*sxx - sx*sx)
	if math.Abs(slope-beta) > 0.35 {
		t.Errorf("rank regression slope = %.2f, want about β = %.1f", slope, beta)
	}
}

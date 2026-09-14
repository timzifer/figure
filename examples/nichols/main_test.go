package main

import (
	"math"
	"math/cmplx"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example. The docs quote it, so this
// test is what stops it rotting.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	loop, peak := filepath.Join(dir, "loop.svg"), filepath.Join(dir, "peak.svg")
	if err := run(loop, peak); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		loop: {"<svg", "An open loop, and what it does to the closed one",
			"open-loop phase (degrees)", "closed-loop gain", "closed-loop phase"},
		peak: {"<svg", "Where the loop touches 3 dB", "peak 3.0 dB"},
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("no output was written for %s: %v", path, err)
		}
		for _, want := range wants {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s is missing %q", filepath.Base(path), want)
			}
		}
	}
}

// The chart's caption is a claim about the loop: at this gain the response
// touches the 3 dB contour and does not cross it. Solved for rather than
// tabulated, so what the example says and what it draws cannot drift apart.
func TestTheLoopIsTangentToTheThreeDecibelContour(t *testing.T) {
	if got := peakOf(tangent).peak; math.Abs(got-3) > 0.01 {
		t.Errorf("the closed loop peaks at %.3f dB, want 3", got)
	}
	// Tangency is the peak touching the contour, which is only a reading if a
	// little more gain crosses it and a little less misses it.
	if got := peakOf(tangent * 1.05).peak; got <= 3 {
		t.Errorf("five per cent more gain still peaks at %.3f dB", got)
	}
	if got := peakOf(tangent / 1.05).peak; got >= 3 {
		t.Errorf("five per cent less gain still peaks at %.3f dB", got)
	}
}

// The point the chart marks is on the curve it draws, because both come from
// the same sweep of the same loop.
func TestTheMarkedPointIsOnTheResponse(t *testing.T) {
	at := peakOf(tangent)
	l := L(at.w, tangent)
	if got := 20 * math.Log10(cmplx.Abs(l)); math.Abs(got-at.gain) > 1e-9 {
		t.Errorf("the marker sits at %v dB, and the loop is at %v dB there", at.gain, got)
	}
	if at.phase > -90 || at.phase < -270 {
		t.Errorf("the marker's phase is %v°, which is outside the turn the chart draws", at.phase)
	}
}

// The response crosses half a turn of phase, which is where a wrapped phase
// would leap across the whole chart. The sweep unwraps it, and the test that
// would have caught the leap is this one.
func TestTheSweepDoesNotJumpAcrossTheChart(t *testing.T) {
	phase, _ := sweep(tangent)
	for i := 1; i < len(phase); i++ {
		if math.Abs(phase[i]-phase[i-1]) > 20 {
			t.Fatalf("the response jumps from %v° to %v° between samples %d and %d",
				phase[i-1], phase[i], i-1, i)
		}
	}
	if phase[0] > -90 || phase[len(phase)-1] < -270 {
		t.Errorf("the sweep runs from %v° to %v°, want the whole lag from −90° to −270°",
			phase[0], phase[len(phase)-1])
	}
}

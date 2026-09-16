package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the charts a
// spherical scene exists for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	pattern, bloch := filepath.Join(dir, "pattern.svg"), filepath.Join(dir, "bloch.svg")
	if err := run(pattern, bloch, filepath.Join(dir, "smith.svg")); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		pattern:                         {"<svg", "broadside array", "</svg>"},
		bloch:                           {"<svg", "Rabi oscillation", "|0⟩", "</svg>"},
		filepath.Join(dir, "smith.svg"): {"<svg", "Smith sphere", "match", "open", "</svg>"},
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

// The pattern is the textbook one: broadside, so its maximum stands straight up
// from the ground plane, and nothing radiates below it.
func TestTheArrayRadiatesBroadsideAndNotBelowTheGround(t *testing.T) {
	if g := gain(0, 0); math.Abs(g-1) > 1e-9 {
		t.Errorf("gain at the zenith = %v, want the normalised maximum 1", g)
	}
	for _, theta := range []float64{95, 120, 180} {
		if g := gain(45, theta); g != 0 {
			t.Errorf("gain at θ = %v° is %v, want nothing below the ground plane", theta, g)
		}
	}
	// Along the array's axis the elements fall out of phase and the beam
	// narrows; across it they stay in phase and it does not.
	if along, across := gain(0, 40), gain(90, 40); along >= across {
		t.Errorf("gain 40° off zenith along the array %v, across it %v: want the fan across the axis", along, across)
	}
}

// The sweep does what the chart is drawn to show: it starts passive, goes
// active near resonance, and comes back — so it crosses the equator of the
// Smith sphere twice, and a flat Smith chart would lose the stretch between.
func TestTheOscillatorLeavesThePassiveHemisphereAndReturns(t *testing.T) {
	crossings, prev := 0, 0.0
	for k := 0; k <= 400; k++ {
		r, _ := impedance(0.25 + 2.75*float64(k)/400)
		if k > 0 && (prev < 0) != (r < 0) {
			crossings++
		}
		prev = r
	}
	if crossings != 2 {
		t.Errorf("the resistance changes sign %d times over the sweep, want 2", crossings)
	}
	if r, _ := impedance(1); r >= 0 {
		t.Errorf("r at resonance = %v, want a negative resistance", r)
	}
}

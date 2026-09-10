package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timzifer/figure/three"
)

// TestExampleRuns executes the documented example.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	four := filepath.Join(dir, "views.svg")
	turned := filepath.Join(dir, "turned.svg")
	compare := filepath.Join(dir, "compare.svg")
	if err := run(four, turned, compare); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		four:    {"<svg", "One surface, four cameras", "three-quarter", "plan", "front", "side"},
		turned:  {"<svg", "after a drag"},
		compare: {"<svg", "as designed", "as built"},
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("no output was written for %s: %v", filepath.Base(path), err)
		}
		for _, want := range wants {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s is missing %q", filepath.Base(path), want)
			}
		}
	}
}

// The scene appears once and is looked at four times. That is what the example
// is for, so it is what the test pins: four cells, one scene value.
func TestOneSceneIsLookedAtFourTimes(t *testing.T) {
	sc := saddle()
	p := three.New(three.Columns(2)).Scene(sc).Add(
		three.View{Camera: three.Home(), Label: "a"},
		three.View{Camera: three.LookAt(), Label: "b"},
		three.View{Camera: three.LookAt(three.Elevation(1)), Label: "c"},
		three.View{Camera: three.LookAt(three.Azimuth(1)), Label: "d"},
	)
	views := p.Views()
	if len(views) != 4 {
		t.Fatalf("%d views, want four", len(views))
	}
	for i, v := range views {
		if v.Scene != nil {
			t.Errorf("view %d brought its own scene; the four share one", i)
		}
	}
	// And the four cameras are genuinely four: a figure whose cells all show
	// the same angle is a figure with one cell and three copies.
	seen := map[three.Camera]bool{}
	for _, v := range views {
		seen[v.Camera] = true
	}
	if len(seen) != 4 {
		t.Errorf("%d distinct cameras among four views", len(seen))
	}
}

// An orbit is a pure function, which is why the host can own the drag: the
// same drag from the same camera always gives the same camera, and nothing
// about the scene is consulted.
func TestTheOrbitIsArithmeticTheHostOwns(t *testing.T) {
	from := three.Home()
	a := turn(from, 140, -60)
	b := turn(from, 140, -60)
	if a != b {
		t.Errorf("the same drag gave two cameras: %+v and %+v", a, b)
	}
	if a == from {
		t.Error("the drag did not move the camera")
	}
	// Dragging back returns to where it started, which is what makes a drag
	// feel like a drag rather than like an accumulating error.
	if got := turn(a, -140, 60); math.Abs(got.Azimuth()-from.Azimuth()) > 1e-9 ||
		math.Abs(got.Elevation()-from.Elevation()) > 1e-9 {
		t.Errorf("dragging back gave %+v, want %+v", got, from)
	}
}

// The two scenes really are two, so that the comparison figure compares
// something.
func TestTheTwoScenesDiffer(t *testing.T) {
	a, _ := grid(func(x, y float64) float64 { return x }).Column("z")
	if a.Len() == 0 {
		t.Fatal("the grid has no rows")
	}
	designed, _ := saddleSource().Column("z")
	built, _ := dentedSource().Column("z")
	same := true
	for i := range designed.Floats {
		if designed.Floats[i] != built.Floats[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("the two scenes hold the same numbers; the comparison compares nothing")
	}
}

package main

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
)

// TestExampleRuns drives the documented streaming example, which is also the
// only place the producer and the renderer run at once — so this is the test
// that says something under -race.
func TestExampleRuns(t *testing.T) {
	out := filepath.Join(t.TempDir(), "live.svg")
	got, err := run(out, 30)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got.Painted == 0 {
		t.Error("no frame was painted")
	}

	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no output was written: %v", err)
	}
	for _, want := range []string{"<svg", "Live throughput", "capacity", "</svg>"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("output is missing %q", want)
		}
	}
}

// TestSteadyFramesRepaintLess is the claim the example prints: a frame repaints
// less of the canvas than all of it.
//
// It does not assert that some frames are skipped, although they usually are.
// Whether a frame's chart changed depends on how many samples the producer
// managed to append while the renderer was busy, and that is a race the
// detector itself perturbs — a flaky gate is a gate people learn to ignore.
// The deterministic form of that claim is in the root package's
// TestARedrawRepaintsOnlyWhatMoved, which draws the same data twice.
func TestSteadyFramesRepaintLess(t *testing.T) {
	out := filepath.Join(t.TempDir(), "live.svg")
	got, err := run(out, 60)
	if err != nil {
		t.Fatal(err)
	}
	if got.Painted == 0 {
		t.Fatal("no frame was painted")
	}
	if f := got.Fraction(); f > 0.85 {
		t.Errorf("frames repainted %.0f%% of the canvas on average, want less than a full repaint", 100*f)
	}
}

// TestAFullWindowSlides is the claim run's comment makes about a full window:
// once rows fall out of it, the time axis starts where the window does rather
// than where the series did. It appends deterministically, without the
// producer goroutine, so the answer does not depend on a race.
func TestAFullWindowSlides(t *testing.T) {
	st := data.NewStream("t", "y", "load").Window(window)
	p, x := chart(st)
	live, err := p.Live(&surface{w: 900, h: 400})
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	seed(st, window)
	st.Snapshot()
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(1, 2))
	for i := window; i < window+100; i++ {
		st.Append(sample(r, i))
	}
	st.Snapshot()
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	lo, hi := x.Domain()
	want, _, _ := sample(r, 100)
	last, _, _ := sample(r, window+99)
	if lo != want || hi != last {
		t.Errorf("the time axis is [%v, %v], want the window's [%v, %v]", lo, hi, want, last)
	}
}

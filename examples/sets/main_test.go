package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the three charts a
// membership table pays for cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	upset := filepath.Join(dir, "upset.svg")
	sizes := filepath.Join(dir, "upset-sizes.svg")
	venn := filepath.Join(dir, "venn.svg")
	if err := run(upset, sizes, venn); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		upset: {"<svg", "subscribe to", "mail", "drive", "chat", "</svg>"},
		sizes: {"<svg", "subscribers", "mail", "</svg>"},
		venn:  {"<svg", "Venn", "mail", "drive", "chat", "</svg>"},
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

// The Venn's regions partition the customers, so its seven numbers add up to
// how many there are. A diagram whose numbers did not would be the usual
// mislabelling, drawn convincingly.
func TestTheVennRegionsAddUpToTheCustomers(t *testing.T) {
	dir := t.TempDir()
	venn := filepath.Join(dir, "venn.svg")
	if err := vennChart(venn); err != nil {
		t.Fatalf("vennChart: %v", err)
	}
	b, err := os.ReadFile(venn)
	if err != nil {
		t.Fatal(err)
	}

	total := 0
	for _, part := range strings.Split(string(b), "</text>") {
		i := strings.LastIndex(part, ">")
		if i < 0 {
			continue
		}
		if n, err := strconv.Atoi(part[i+1:]); err == nil {
			total += n
		}
	}
	if want := len(distinct(who)); total != want {
		t.Errorf("the regions hold %d customers between them, and the table has %d", total, want)
	}
}

func distinct(names []string) map[string]bool {
	out := map[string]bool{}
	for _, n := range names {
		out[n] = true
	}
	return out
}

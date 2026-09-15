package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/timzifer/figure/data"
)

// column and times read one column of a table, failing the test rather than
// returning a second value nobody would check.
func column(t *testing.T, src data.Source, name string) []string {
	t.Helper()
	vs, ok := data.StringColumn(src, name)
	if !ok {
		t.Fatalf("no %q column", name)
	}
	return vs
}

func times(t *testing.T, src data.Source, name string) []time.Time {
	t.Helper()
	vs, ok := data.TimeColumn(src, name)
	if !ok {
		t.Fatalf("no %q column", name)
	}
	return vs
}

// TestExampleRuns executes the documented example, so that the chart a
// schedule is cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	out := filepath.Join(t.TempDir(), "gantt.svg")
	if err := run(out); err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no output was written: %v", err)
	}
	got := string(b)

	// The title, the lanes, the phases in the legend, and the two names the
	// chart's extra readings carry.
	for _, want := range []string{
		"<svg", "Instrument rebuild", "align optics", "hand-over",
		"supply", "critical", "milestone", "today", "</svg>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q", want)
		}
	}
}

// TestEveryConstraintNamesTwoTasksInThePlan. A link to a task nothing is called
// is dropped silently — that is what makes the mark survive a facet — so a typo
// in the example would cost an arrow and nothing would say so.
func TestEveryConstraintNamesTwoTasksInThePlan(t *testing.T) {
	tasks, links, milestones := plan()
	ids := map[string]bool{}
	for _, id := range column(t, tasks, "id") {
		ids[id] = true
	}
	for _, col := range []string{"before", "after"} {
		for i, name := range column(t, links, col) {
			if !ids[name] {
				t.Errorf("link %d names %q as its %s, which is not a task in the plan", i, name, col)
			}
		}
	}
	// A milestone sits on a lane, and a lane the axis was not given has no
	// slot to sit in.
	rows := map[string]bool{}
	for _, l := range lanes() {
		rows[l] = true
	}
	for i, name := range column(t, milestones, "task") {
		if !rows[name] {
			t.Errorf("milestone %d is on lane %q, which the axis has no slot for", i, name)
		}
	}
}

// TestTheProgressAgreesWithToday. The chart is worth reading only because the
// pale part of a bar means something: a task that ended before today and is not
// finished is the reading, and one that has not started yet cannot be.
func TestTheProgressAgreesWithToday(t *testing.T) {
	tasks, _, _ := plan()
	starts := times(t, tasks, "start")
	done, ok := data.Float64Column(tasks, "done")
	if !ok {
		t.Fatal("the plan has no progress column")
	}
	for i, at := range starts {
		if at.After(today) && done[i] != 0 {
			t.Errorf("task %d starts on %s, after today, and is %v done", i, at.Format(time.DateOnly), done[i])
		}
	}
}

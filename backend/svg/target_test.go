package svg_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/timzifer/figure/backend/svg"
	"github.com/timzifer/figure/ir"
)

// `defer t.Close()` after a render that failed before Open is the ordinary way
// to write this, and it used to dereference the writer Open never created.
func TestAFileTargetClosesWithoutHavingBeenOpened(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never.svg")
	if err := svg.File(path).Close(); err != nil {
		t.Fatalf("Close without Open: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Close without Open created %s (stat: %v)", path, err)
	}
}

// An Open whose Create failed leaves no writer behind either.
func TestAFileTargetClosesAfterOpenFailed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-dir", "chart.svg")
	tgt := svg.File(path)
	if _, err := tgt.Open(ir.Surface{WidthPx: 10, HeightPx: 10, DPR: 1}); err == nil {
		t.Fatal("Open into a directory that does not exist succeeded")
	}
	if err := tgt.Close(); err != nil {
		t.Fatalf("Close after a failed Open: %v", err)
	}
}

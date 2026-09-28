package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// TestExampleRuns executes the documented example, so that the chart a market
// is cannot stop compiling without anyone noticing.
func TestExampleRuns(t *testing.T) {
	out := filepath.Join(t.TempDir(), "market.svg")
	if err := run(out); err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no output was written: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		"<svg", "holidays folded", "Bollinger", "EMA 9", "SMA 20", "buy", "sell", "</svg>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q", want)
		}
	}
}

// Every candle falls on a trading day, so none of them is inside a fold — a
// candle drawn into a weekend or the holiday would be drawn into a gap the
// axis took out.
func TestNoCandleFallsInAWeekend(t *testing.T) {
	m := simulate()
	days := stat.OHLC(m.ts, m.ps, m.vs, scale.Nanos(start), float64(24*time.Hour))
	if len(days) != calendarDays*5/7-1 {
		t.Fatalf("%d candles, want one per weekday of %d calendar days but the holiday", len(days), calendarDays)
	}
	folds, err := scale.Folds(trading, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(folds) != 12 {
		t.Errorf("%d folds, want twelve weekends with the holiday joined to one", len(folds))
	}
	for _, d := range days {
		at := scale.FromNanos(d.Start + float64(12*time.Hour))
		for _, f := range folds {
			if !at.Before(f.From) && at.Before(f.To) {
				t.Errorf("candle at %v is inside the fold %v–%v", at, f.From, f.To)
			}
		}
	}
}

// The crossings exist in the simulated walk, so the markers are exercised
// rather than drawn from empty tables.
func TestTheWalkCrossesBothWays(t *testing.T) {
	m := simulate()
	days := stat.OHLC(m.ts, m.ps, m.vs, scale.Nanos(start), float64(24*time.Hour))
	buys, sells := signals(days)
	if buys.Len() == 0 || sells.Len() == 0 {
		t.Errorf("%d buys and %d sells, want at least one of each", buys.Len(), sells.Len())
	}
}

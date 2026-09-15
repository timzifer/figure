// Command survival renders a Kaplan–Meier comparison of two trial arms, with
// the numbers-at-risk table printed under it.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. The data is the remission trial every
// survival textbook opens with — Freireich et al. (1963), 6-mercaptopurine
// against placebo in acute leukaemia — and the chart is the figure every
// paper that reports such a trial prints.
//
// The curve is geom.Survival, which runs the estimator in Train. The table
// under it is not: it is a table on the shared time axis beneath the panel,
// which is a track, and a mark cannot make a panel. So the program asks
// stat.KaplanMeier for the risk sets itself and writes them into a track with
// geom.Text — see docs/adr/0054-statistical-instruments.md.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

func main() {
	out := flag.String("o", "survival.svg", "output path")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "survival:", err)
		os.Exit(1)
	}
}

// The two arms, in weeks of remission. An event of 0 is a patient still in
// remission when last seen.
var arms = []struct {
	name   string
	weeks  []float64
	events []float64
}{
	{"6-MP",
		[]float64{6, 6, 6, 6, 7, 9, 10, 10, 11, 13, 16, 17, 19, 20, 22, 23, 25, 32, 32, 34, 35},
		[]float64{1, 1, 1, 0, 1, 0, 1, 0, 0, 1, 1, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0}},
	{"placebo",
		[]float64{1, 1, 2, 2, 3, 4, 4, 5, 5, 8, 8, 8, 8, 11, 11, 12, 12, 15, 17, 22, 23},
		[]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}},
}

// tableAt is where the risk table reads the risk sets: every ten weeks.
var tableAt = []float64{0, 10, 20, 30}

func run(out string) error {
	var weeks, events []float64
	var arm []string
	for _, a := range arms {
		weeks = append(weeks, a.weeks...)
		events = append(events, a.events...)
		for range a.weeks {
			arm = append(arm, a.name)
		}
	}

	p := figure.New(
		figure.Size(680, 520),
		figure.Title("Remission, 6-MP against placebo"),
		figure.Theme(theme.Light),
		figure.XTitle("weeks"),
		figure.YTitle("in remission"),
	)
	p.X(scale.Linear(scale.Domain(-1.5, 36), scale.TickValues(0, 10, 20, 30)))
	p.Y(scale.Linear(scale.Domain(0, 1), scale.NumberFormat("#%")))
	p.Add(geom.Survival(figure.NewTable().
		Float64("weeks", weeks).Float64("relapsed", events).String("arm", arm),
		geom.X("weeks"), geom.Event("relapsed"), geom.GroupBy("arm"),
		geom.Confidence(0.95), geom.CensorMarks(true)))

	var at []float64
	var lane, count []string
	for _, a := range arms {
		for _, t := range tableAt {
			at, lane = append(at, t), append(lane, a.name)
			count = append(count, strconv.Itoa(atRisk(a.weeks, a.events, t)))
		}
	}
	p.Track(figure.Bottom, figure.TrackSize(44), figure.TrackGrid(false)).
		Add(geom.Text(figure.NewTable().Float64("weeks", at).String("arm", lane).String("n", count),
			geom.X("weeks"), geom.Y("arm"), geom.TextBy("n")))
	return p.Render(figure.SVG(out))
}

// atRisk is how many of an arm were still under observation at t: the risk
// set of the first step at or after t, or nobody once the arm has run out.
func atRisk(weeks, events []float64, t float64) int {
	sorted, flags := sortArm(weeks, events)
	for _, p := range stat.KaplanMeier(sorted, flags) {
		if p.T >= t {
			return p.AtRisk
		}
	}
	return 0
}

// sortArm orders an arm by time, carrying its events along. The arms above are
// already in order; this is here so that the table stays right if they are not.
func sortArm(weeks, events []float64) ([]float64, []bool) {
	idx := make([]int, len(weeks))
	for i := range idx {
		idx[i] = i
	}
	for i := 1; i < len(idx); i++ {
		for j := i; j > 0 && weeks[idx[j]] < weeks[idx[j-1]]; j-- {
			idx[j], idx[j-1] = idx[j-1], idx[j]
		}
	}
	sorted, flags := make([]float64, len(idx)), make([]bool, len(idx))
	for k, i := range idx {
		sorted[k], flags[k] = weeks[i], events[i] != 0
	}
	return sorted, flags
}

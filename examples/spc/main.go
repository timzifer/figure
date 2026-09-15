// Command spc renders an individuals control chart: limits established on a
// baseline, and new readings judged against them.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. There is no control-chart mark, and
// that is the point of docs/adr/0054-statistical-instruments.md: control
// limits come from a baseline period and are then frozen, so a mark that
// recomputed them from the points it was handed would be wrong for the chart's
// principal use. The program computes them once with stat.LimitsIMR, draws
// them as three HLines, and asks stat.AppendRunRules which of the new readings
// break a Nelson rule.
//
// The line colours in classes at the limits — geom.ColorBy over a
// scale.Threshold whose breaks are the limits — and the change of colour is
// interpolated onto the limit itself, which is where an out-of-control run
// begins (docs/adr/0049-paths-colour-in-classes.md).
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

func main() {
	out := flag.String("o", "spc.svg", "output path")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "spc:", err)
		os.Exit(1)
	}
}

// Fill weights in grams. The first baseline readings are the process in
// control; after them the filler drifts upward.
const baseline = 30

func weights() []float64 {
	w := make([]float64, 60)
	for i := range w {
		noise := 0.9*math.Sin(float64(i)*2.1) + 0.6*math.Cos(float64(i)*5.3)
		w[i] = 500 + noise
		if i >= 42 {
			w[i] += 0.12 * float64(i-41)
		}
	}
	return w
}

func run(out string) error {
	w := weights()
	limits, _ := stat.LimitsIMR(w[:baseline])
	flags := stat.AppendRunRules(nil, w[baseline:], limits)

	n := make([]float64, len(w))
	for i := range n {
		n[i] = float64(i + 1)
	}
	var fx, fy []float64
	seen := map[int]bool{}
	for _, f := range flags {
		if row := baseline + f.Row; !seen[row] {
			seen[row] = true
			fx, fy = append(fx, n[row]), append(fy, w[row])
		}
	}

	p := figure.New(
		figure.Size(760, 420),
		figure.Title("Fill weight, individuals chart"),
		figure.Theme(theme.Light),
		figure.XTitle("sample"),
		figure.YTitle("grams"),
		figure.Legend(false),
	)
	p.X(scale.Linear(scale.Domain(0, 61)))
	p.Add(
		geom.VLine(baseline+0.5, geom.Dash(3, 3), geom.Label("limits frozen")),
		geom.HLine(limits.Centre, geom.Color(palette.Gray)),
		geom.HLine(limits.Upper, geom.Color(palette.Vermilion), geom.Dash(6, 3)),
		geom.HLine(limits.Lower, geom.Color(palette.Vermilion), geom.Dash(6, 3)),
		geom.Line(figure.NewTable().Float64("n", n).Float64("w", w),
			geom.X("n"), geom.Y("w"),
			geom.ColorBy("w", scale.Threshold(
				palette.Ramp{palette.Vermilion, palette.Blue, palette.Vermilion},
				[]float64{limits.Lower, limits.Upper})),
			// The limits are already on the chart as rules with their values
			// beside them; a colourbar would say them a second time.
			geom.Guide(false)),
		geom.Scatter(figure.NewTable().Float64("n", fx).Float64("w", fy),
			geom.X("n"), geom.Y("w"), geom.Color(palette.Vermilion), geom.Size(8),
			geom.Shape(ir.MarkerDiamond)),
		geom.Note(1, limits.Upper+0.1, fmt.Sprintf("UCL %.2f", limits.Upper),
			geom.Align(ir.AlignStart, ir.AlignBottom)),
		geom.Note(1, limits.Lower+0.1, fmt.Sprintf("LCL %.2f", limits.Lower),
			geom.Align(ir.AlignStart, ir.AlignBottom)),
	)
	return p.Render(figure.SVG(out))
}

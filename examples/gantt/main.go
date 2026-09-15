// Command gantt renders the chart a schedule is: what runs when, how far each
// task has got, what is waiting on what, and where today is.
//
// The bars are a rect on a time axis against an ordinal one, which is what a
// gantt chart has always been here. What the other three lines of the plot add
// is the rest of the reading: geom.ProgressBy fills each bar as far as it has
// got, geom.Depends draws the constraints between them, and the milestones are
// a scatter of diamonds on the same two axes.
//
// It is executed by a test so that it cannot silently stop compiling or stop
// producing a chart.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

func main() {
	out := flag.String("o", "gantt.svg", "output SVG path")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "gantt:", err)
		os.Exit(1)
	}
}

// today is where the chart is being read from. It is a constant rather than
// time.Now so that the example draws the same picture every time it is run,
// which is what lets a test compare it against a golden file.
var today = day(23)

func run(out string) error {
	tasks, links, milestones := plan()

	p := figure.New(
		figure.Size(900, 420),
		figure.Title("Instrument rebuild — March"),
	)
	p.X(scale.Time())

	// The lanes are named from the bottom up, so that the first task of the
	// plan is the top row of the chart: an ordinal axis counts upwards and a
	// schedule reads downwards.
	p.Y(scale.Ordinal(scale.Categories(lanes()...)))

	// The bar is the span and the fill is the progress. Both come out of one
	// layer, which is what keeps them the same shape: a second layer drawn
	// over the first would be a second statement about where the task is.
	p.Add(geom.Rect(tasks,
		geom.X("start"), geom.X2("end"), geom.Y("task"),
		geom.ProgressBy("done"),
		geom.ColorBy("phase", scale.Qualitative(palette.Default))))

	// The constraints. Colouring them by the link table's own column is how
	// the critical path is drawn — there is no separate machinery for it,
	// because a critical path is a property of the links and this mark reads
	// the links.
	p.Add(geom.Depends(tasks, links,
		geom.X("start"), geom.X2("end"), geom.Y("task"),
		geom.KeyBy("id"), geom.From("before"), geom.To("after"),
		geom.LinkBy("kind"),
		geom.ColorBy("path", scale.Named(map[string]ir.Color{
			"critical": palette.Red,
			"slack":    palette.Gray,
		}))))

	// A milestone has no duration, so it is not a span at all: it is a point
	// on the same two axes, which is a scatter.
	p.Add(geom.Scatter(milestones,
		geom.X("at"), geom.Y("task"),
		geom.Shape(ir.MarkerDiamond), geom.Size(11),
		geom.Color(palette.Black), geom.Label("milestone")))

	// Where the reader is standing. Everything to the left of it should be
	// finished, and a bar that is pale on that side is the one to look at.
	p.Add(geom.VLine(scale.Nanos(today), geom.Label("today")))

	return p.Render(figure.SVG(out))
}

func day(d int) time.Time { return time.Date(2026, time.March, d, 0, 0, 0, 0, time.UTC) }

// lanes is the plan's rows, bottom to top. The axis is given them explicitly so
// that a task with no bar — a milestone — still has a row, and so that the
// order is the plan's rather than the order the rows happen to be written in.
func lanes() []string {
	return []string{
		"hand-over", "commission", "align optics", "fit detector",
		"machine frame", "parts in", "order parts", "design", "survey",
	}
}

// plan is the schedule: the spans, the constraints between them, and the dates
// that are not spans.
func plan() (tasks, links, milestones *data.Table) {
	tasks = figure.NewTable().
		String("id", []string{"survey", "design", "order", "parts", "frame", "detector", "optics", "commission"}).
		String("task", []string{"survey", "design", "order parts", "parts in", "machine frame", "fit detector", "align optics", "commission"}).
		Time("start", []time.Time{day(2), day(4), day(9), day(11), day(11), day(18), day(20), day(24)}).
		Time("end", []time.Time{day(5), day(9), day(11), day(18), day(17), day(21), day(24), day(28)}).
		Float64("done", []float64{1, 1, 1, 0.8, 0.55, 0.1, 0, 0}).
		String("phase", []string{
			"plan", "plan", "supply", "supply", "build", "build", "build", "accept",
		})

	// before → after, and what kind of constraint it is. The two finish-to-
	// finish links are the ones a plan actually needs the vocabulary for:
	// the frame does not have to start after the parts arrive, it has to be
	// standing when they do.
	links = figure.NewTable().
		String("before", []string{"survey", "design", "order", "parts", "frame", "detector", "optics", "detector"}).
		String("after", []string{"design", "order", "parts", "detector", "detector", "optics", "commission", "commission"}).
		String("kind", []string{"fs", "fs", "fs", "fs", "ff", "fs", "fs", "ss"}).
		String("path", []string{
			"critical", "critical", "critical", "critical",
			"slack", "critical", "critical", "slack",
		})

	// A milestone is a date rather than a span, so it has no start and no end
	// and cannot be a bar. "hand-over" is a lane with nothing else in it,
	// which is why the axis is given its categories explicitly: a lane whose
	// only mark is a diamond has no rect to create it.
	milestones = figure.NewTable().
		Time("at", []time.Time{day(9), day(18), day(27)}).
		String("task", []string{"design", "parts in", "hand-over"})
	return tasks, links, milestones
}

// A schedule, end to end.
//
// A plan is bars on a time axis against an ordinal one — which is what a gantt
// chart has been here since there was a rect mark — plus the three things that
// make it a schedule rather than a picture of one: how far each task has got,
// what is waiting on what, and the dates that are not spans at all. See
// docs/adr/0068-gantt-charts.md.
package figure_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/a11y"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

func march(d int) time.Time { return time.Date(2026, time.March, d, 0, 0, 0, 0, time.UTC) }

// schedule is a four-task plan, the constraints over it and one milestone.
func schedule() (tasks, links, milestones *data.Table) {
	tasks = figure.NewTable().
		String("id", []string{"survey", "design", "build", "accept"}).
		String("task", []string{"survey", "design", "build", "accept"}).
		Time("start", []time.Time{march(2), march(6), march(12), march(20)}).
		Time("end", []time.Time{march(6), march(12), march(20), march(23)}).
		Float64("done", []float64{1, 0.6, 0.15, 0}).
		String("phase", []string{"plan", "plan", "make", "make"})
	links = figure.NewTable().
		String("before", []string{"survey", "design", "build"}).
		String("after", []string{"design", "build", "accept"}).
		String("kind", []string{"fs", "fs", "ff"}).
		String("path", []string{"critical", "critical", "slack"})
	milestones = figure.NewTable().
		Time("at", []time.Time{march(12)}).
		String("task", []string{"design"})
	return tasks, links, milestones
}

// ganttChart is the chart the tests below read, built once so that the golden
// file and the assertions are about the same picture.
func ganttChart() *figure.Plot {
	tasks, links, milestones := schedule()
	p := figure.New(figure.Size(760, 320), figure.Title("Rebuild"))
	p.X(scale.Time())
	p.Y(scale.Ordinal(scale.Categories("accept", "build", "design", "survey")))
	p.Add(
		geom.Rect(tasks, geom.X("start"), geom.X2("end"), geom.Y("task"),
			geom.ProgressBy("done"),
			geom.ColorBy("phase", scale.Qualitative(palette.Default))),
		geom.Depends(tasks, links,
			geom.X("start"), geom.X2("end"), geom.Y("task"),
			geom.KeyBy("id"), geom.From("before"), geom.To("after"),
			geom.LinkBy("kind"),
			geom.ColorBy("path", scale.Named(map[string]ir.Color{
				"critical": palette.Red,
				"slack":    palette.Gray,
			}))),
		geom.Scatter(milestones, geom.X("at"), geom.Y("task"),
			geom.Shape(ir.MarkerDiamond), geom.Size(10),
			geom.Color(palette.Black), geom.Label("milestone")),
		geom.VLine(scale.Nanos(march(16)), geom.Label("today")),
	)
	return p
}

func TestGoldenGanttChart(t *testing.T) { golden(t, "gantt", ganttChart()) }

// TestAScheduleIsHitTestedAsItsOwnBars. A gantt bar is two shapes now — the
// span and the part of it that is done — and both have to name the same task,
// or a tooltip would answer differently depending on which half the pointer
// landed in.
func TestAScheduleIsHitTestedAsItsOwnBars(t *testing.T) {
	panels, idx := panelsOf(t, ganttChart())
	if len(panels) == 0 {
		t.Fatal("the chart announced no panel")
	}
	// The "design" bar runs from the 6th to the 12th and is 60 % done, so a
	// point a fifth of the way along it is in the finished part and one four
	// fifths along is in the rest.
	y := laneOf(t, panels[0], "design")
	early := xOf(t, panels[0], march(7))
	late := xOf(t, panels[0], march(11))
	a, okA := idx.At(ir.Point{X: early, Y: y}, 0)
	b, okB := idx.At(ir.Point{X: late, Y: y}, 0)
	if !okA || !okB {
		t.Fatalf("a bar was not hit: %v, %v", okA, okB)
	}
	if a.Row != b.Row {
		t.Errorf("the two halves of one bar report rows %d and %d", a.Row, b.Row)
	}
}

// laneOf and xOf invert the chart's own scales, which is how a test names a
// device position without knowing the layout's arithmetic.
func laneOf(t *testing.T, pa interact.Panel, lane string) float32 {
	t.Helper()
	cat, ok := pa.Y.(scale.Categorical)
	if !ok {
		t.Fatal("the vertical axis is not categorical")
	}
	return pa.Y.Map(cat.Encode(lane))
}

func xOf(t *testing.T, pa interact.Panel, at time.Time) float32 {
	t.Helper()
	return pa.X.Map(scale.Nanos(at))
}

// TestADescribedScheduleCountsItsConstraints. A dependency layer draws one mark
// per link and none per task, so the number that describes it is the link
// table's — the extent it reports is still the plan's, because that is where
// its marks land.
func TestADescribedScheduleCountsItsConstraints(t *testing.T) {
	p := ganttChart()
	sum := p.Describe()
	var arrows a11y.Series
	for _, s := range sum.Series {
		if s.Mark == geom.MarkDepends {
			arrows = s
		}
	}
	if arrows.Mark == "" {
		t.Fatal("the description does not mention the constraints")
	}
	if arrows.Rows != 3 {
		t.Errorf("the description counts %d constraints, want 3", arrows.Rows)
	}
	// The extent it reports is the plan's, on the axis that has one: the lanes
	// are names rather than quantities, so only the time axis has bounds.
	if !arrows.XRange.Ok {
		t.Error("the constraints report no extent; they are drawn against the plan's own time axis")
	}
	if !strings.Contains(sum.Detail, "dependency") {
		t.Errorf("the long description does not name the mark:\n%s", sum.Detail)
	}
}

// TestTheBarsAndTheArrowsAgreeAboutWhereASpanEnds. The arrow is worth drawing
// only because it lands on the bar, so the two layers have to measure a lane
// and a slot the same way — which is why the dependency mark carries the same
// half-width arithmetic the rect does rather than one of its own.
func TestTheBarsAndTheArrowsAgreeAboutWhereASpanEnds(t *testing.T) {
	tasks, links, _ := schedule()
	p := figure.New(figure.Size(760, 320))
	p.X(scale.Time())
	p.Y(scale.Ordinal(scale.Categories("accept", "build", "design", "survey")))
	p.Add(
		geom.Rect(tasks, geom.X("start"), geom.X2("end"), geom.Y("task")),
		geom.Depends(tasks, links, geom.X("start"), geom.X2("end"), geom.Y("task"),
			geom.KeyBy("id"), geom.From("before"), geom.To("after")),
	)
	panels, idx := panelsOf(t, p)
	pa := panels[0]
	cat := pa.Y.(scale.Categorical)
	// The arrow into "build" arrives at its start, on its lane's middle. A hit
	// a hair inside that point must be the build bar.
	x := pa.X.Map(scale.Nanos(march(12)))
	y := pa.Y.Map(cat.Encode("build"))
	h, ok := idx.At(ir.Point{X: x + 2, Y: y}, 0)
	if !ok {
		t.Fatal("nothing was hit just inside the start of the build bar")
	}
	if got := math.Abs(h.Y - cat.Encode("build")); got > 0.5 {
		t.Errorf("the hit names lane %v, want build's", h.Y)
	}
}

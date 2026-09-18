// Command statechart renders the two charts a layered graph unlocks.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. The first is a state machine — a TCP
// connection, cycles and all — and the second is a build pipeline, which is the
// same mark reading a directed acyclic graph. See
// docs/adr/0072-layered-graph-layout.md.
//
// The point both make is that the table is the whole input: two string columns
// naming each edge's ends. Nodes are not declared anywhere, and the order they
// are first mentioned in is the order they take their colours and their places
// in.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func main() {
	states := flag.String("states", "states.svg", "output path for the state chart")
	build := flag.String("build", "build.svg", "output path for the build pipeline")
	flag.Parse()
	if err := run(*states, *build); err != nil {
		fmt.Fprintln(os.Stderr, "statechart:", err)
		os.Exit(1)
	}
}

func run(states, build string) error {
	if err := stateChart(states); err != nil {
		return err
	}
	return pipeline(build)
}

// A connection's states, in the order a reader meets them going down the
// table. Three of these transitions return to an earlier state and one stays
// where it is — the shapes stat.Sankey refuses and this mark is for.
var (
	transFrom = []string{
		"closed", "listen", "syn-rcvd", "established", "established",
		"fin-wait", "close-wait", "last-ack", "listen", "syn-rcvd",
	}
	transTo = []string{
		"listen", "syn-rcvd", "established", "fin-wait", "close-wait",
		"closed", "last-ack", "closed", "listen", "listen",
	}
)

// bare is the theme every chart of a relational layout wants: both axes
// describe the unit square, which is the truthful answer and nothing a reader
// needs to see.
func bare() theme.Theme {
	return theme.Light.With(
		theme.Grid(false, false),
		theme.AxisLines(false, false),
		theme.Ticks(false, false),
	)
}

func stateChart(out string) error {
	p := figure.New(
		figure.Size(560, 420),
		figure.Title("A connection, from closed and back"),
		figure.Theme(bare()),
		figure.Legend(false),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	// Baseline(1) puts the first state at the top, which is how a state chart
	// is read. There is no rankdir option: the coord and this are what that
	// option is in figure's grammar.
	p.Add(geom.Graph(
		figure.NewTable().String("state", transFrom).String("next", transTo),
		geom.From("state"), geom.To("next"),
		geom.Baseline(1),
		geom.Fill(palette.Blue),
		geom.Corner(4),
		geom.FontSize(10),
	))
	return p.Render(figure.SVG(out))
}

// A build pipeline: a DAG with no cycle in it at all, so every edge runs
// forwards and the one that skips two ranks bends through them rather than
// cutting across.
var (
	stepFrom = []string{
		"checkout", "checkout", "deps", "generate", "build", "build",
		"test", "lint", "package", "checkout",
	}
	stepTo = []string{
		"deps", "lint", "generate", "build", "test", "package",
		"package", "package", "publish", "publish",
	}
)

func pipeline(out string) error {
	p := figure.New(
		figure.Size(560, 420),
		figure.Title("A build, step by step"),
		figure.Theme(bare()),
		figure.Legend(false),
		figure.Coord(coord.Cartesian()),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	// Straight branches rather than the default elbow: a pipeline reads as a
	// flow, and a flow's edge is a line rather than a corner.
	p.Add(geom.Graph(
		figure.NewTable().String("step", stepFrom).String("then", stepTo),
		geom.From("step"), geom.To("then"),
		geom.Baseline(1),
		geom.Branches(geom.Straight),
		geom.Corner(4),
		geom.FontSize(10),
	))
	return p.Render(figure.SVG(out))
}

// Command network renders the chart a plain edge list pays for: a node-link
// diagram, where two dots near each other are two things with a short path
// between them.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. The table is one row per edge — who
// worked with whom — and nothing declares the people: a node exists because a
// row mentioned it. See docs/adr/0077-a-node-link-layout.md.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func main() {
	out := flag.String("out", "network.svg", "output path for the node-link diagram")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "network:", err)
		os.Exit(1)
	}
}

// Who has worked on something with whom, over a quarter. Three teams that
// mostly keep to themselves, two people who work across all of them, and one
// pair off on their own — which is a shape nobody wrote down anywhere and the
// layout is what shows it.
var (
	a = []string{
		"ana", "ana", "ana", "bo", "bo", "cy", // the platform team
		"dag", "dag", "dag", "eve", "eve", "fen", // the mobile team
		"gil", "gil", "gil", "hana", "hana", "ivo", // the data team
		"cy", "fen", // the bridge: one link each way through the middle
		"kip", "kip", // a pair working on something of their own
		"jun", "jun", "jun", // the person who turns up everywhere
	}
	b = []string{
		"bo", "cy", "lux", "cy", "lux", "lux",
		"eve", "fen", "moe", "fen", "moe", "moe",
		"hana", "ivo", "nes", "ivo", "nes", "nes",
		"dag", "gil",
		"lia", "ora",
		"ana", "eve", "hana",
	}
)

func collaborations() data.Source {
	return figure.NewTable().String("who", a).String("with", b)
}

func run(out string) error {
	p := figure.New(
		figure.Size(720, 560),
		figure.Title("Who worked with whom, this quarter"),
		// Both axes describe the unit square, which is nothing a reader wants
		// numbered — the theme every mark that places its own geometry wants.
		figure.Theme(theme.Light.With(
			theme.Grid(false, false), theme.AxisLines(false, false), theme.Ticks(false, false))),
		figure.Legend(false),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.NodeLink(collaborations(), geom.From("who"), geom.To("with")))
	return p.Render(figure.SVG(out))
}

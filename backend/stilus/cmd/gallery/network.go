package main

import (
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// networkFigure is the chart the last refusal in bucket E was hiding.
//
// A node-link diagram says one thing and says it without an axis: two dots near
// each other are two things with a short path between them. Nothing in the
// table below records that there are three teams, a pair working on their own
// or one person who turns up everywhere — the layout is what shows it, and it
// is the reason this form exists.
//
// The picture is a pure function of the table. What places the dots is
// stat.Stress, which minimises how far the drawn distances are from the graph's
// own by majorization: a closed-form step, a fixed number of them, and a start
// that is the table's own arithmetic rather than a seed. See
// docs/adr/0077-a-node-link-layout.md.
func networkFigure() plate {
	return plate{
		name: "network", width: 720, high: 560,
		theme: theme.Light.With(
			theme.Grid(false, false), theme.AxisLines(false, false), theme.Ticks(false, false)),
		title: "Who worked with whom, this quarter",
		opts:  []figure.Option{figure.Legend(false)},
		build: func(p *figure.Plot) {
			p.X(scale.Linear())
			p.Y(scale.Linear())
			p.Add(geom.NodeLink(collaborations(), geom.From("who"), geom.To("with")))
		},
	}
}

// collaborations is one row per pair of people who worked on something
// together. The people are not declared anywhere: a node exists because a row
// mentioned it.
func collaborations() data.Source {
	return figure.NewTable().
		String("who", []string{
			"ana", "ana", "ana", "bo", "bo", "cy",
			"dag", "dag", "dag", "eve", "eve", "fen",
			"gil", "gil", "gil", "hana", "hana", "ivo",
			"cy", "fen",
			"kip", "kip",
			"jun", "jun", "jun",
		}).
		String("with", []string{
			"bo", "cy", "lux", "cy", "lux", "lux",
			"eve", "fen", "moe", "fen", "moe", "moe",
			"hana", "ivo", "nes", "ivo", "nes", "nes",
			"dag", "gil",
			"lia", "ora",
			"ana", "eve", "hana",
		})
}

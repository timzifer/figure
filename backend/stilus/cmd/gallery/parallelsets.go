package main

import (
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// parallelSetsFigure is the parallel-coordinates plot's other half: the chart
// for a table whose columns are categories rather than quantities.
//
// A line per row is the wrong drawing there — nine hundred tickets have a few
// dozen distinct answers between them, so the lines land on top of each other
// and the picture says nothing about how many. So the rows are counted
// instead, and a ribbon between two categories is as thick as the number of
// rows holding both.
//
// Nothing under it is new: stat.Crosstab counts the pairs and stat.Sankey
// stacks them, because a count over neighbouring columns *is* a flow — the
// same total passes through every column. The colour column subdivides the
// ribbons rather than recolouring them, which is what makes the escalated
// share readable in the first column instead of only in the last. See
// docs/adr/0079-parallel-sets.md.
func parallelSetsFigure() plate {
	return plate{
		name: "parallelsets", width: 720, high: 460,
		// Both axes describe the unit square here, exactly as they do under a
		// treemap or a sankey, so there is nothing for a grid line, an axis
		// line or a tick to mean.
		theme: theme.Light.With(
			theme.Grid(false, false), theme.AxisLines(false, false), theme.Ticks(false, false)),
		title: "A quarter of support tickets, and where the escalations come from",
		// One layer, so the legend has to be asked for: it is what names the
		// classes the ribbons are painted in.
		opts: []figure.Option{figure.Legend(true)},
		build: func(p *figure.Plot) {
			p.X(scale.Linear())
			p.Y(scale.Linear())
			p.Add(geom.ParallelSets(ticketTable(),
				geom.Dims("channel", "urgency", "outcome"),
				geom.Value("tickets"),
				geom.ColorBy("outcome", scale.Qualitative(palette.OkabeIto)),
				geom.Padding(0.02),
				geom.Opacity(0.55),
			))
		},
	}
}

// ticketTable is the figure's data, already counted: one row per combination
// of channel, urgency and outcome, and how many tickets that was. The numbers
// are made up and the pattern in them is the ordinary one — what arrives by
// phone is more often urgent, and what is urgent is more often escalated.
func ticketTable() data.Source {
	var channel, urgency, outcome []string
	var tickets []float64
	for _, r := range []struct {
		channel, urgency, outcome string
		tickets                   float64
	}{
		{"chat", "routine", "solved", 210},
		{"chat", "routine", "escalated", 18},
		{"chat", "urgent", "solved", 44},
		{"chat", "urgent", "escalated", 26},
		{"email", "routine", "solved", 180},
		{"email", "routine", "escalated", 30},
		{"email", "urgent", "solved", 38},
		{"email", "urgent", "escalated", 34},
		{"phone", "routine", "solved", 96},
		{"phone", "routine", "escalated", 22},
		{"phone", "urgent", "solved", 104},
		{"phone", "urgent", "escalated", 98},
	} {
		channel = append(channel, r.channel)
		urgency = append(urgency, r.urgency)
		outcome = append(outcome, r.outcome)
		tickets = append(tickets, r.tickets)
	}
	return data.NewTable().
		String("channel", channel).
		String("urgency", urgency).
		String("outcome", outcome).
		Float64("tickets", tickets)
}

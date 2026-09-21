// Command parallelsets renders the chart a table of categorical columns draws.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. The table is a quarter of support
// tickets — how they arrived, how urgent they were, and how they ended —
// already counted, which is the shape this data usually comes in. See
// docs/adr/0079-parallel-sets.md.
//
// The first chart is the form itself: a column of boxes per question, and a
// ribbon between two boxes as thick as the number of tickets that answer both
// that way. The second colours the ribbons by the outcome, which splits each
// of them into the part that was solved and the part that was escalated — the
// reading the chart exists for, because it is visible the whole length of the
// diagram rather than only in the last column.
//
// Neither chart is a parallel-coordinates plot with categorical axes. That
// chart would be one line per ticket and nine hundred lines drawn on top of
// eighteen paths; this one counts them, which is what makes it readable at
// all.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func main() {
	flow := flag.String("flow", "tickets.svg", "output path for the plain diagram")
	byOutcome := flag.String("outcome", "tickets-by-outcome.svg", "output path for the diagram coloured by outcome")
	flag.Parse()
	if err := run(*flow, *byOutcome); err != nil {
		fmt.Fprintln(os.Stderr, "parallelsets:", err)
		os.Exit(1)
	}
}

func run(flow, byOutcome string) error {
	if err := ticketFlow(flow); err != nil {
		return err
	}
	return ticketOutcomes(byOutcome)
}

// tickets is a quarter of support tickets, counted rather than listed: one row
// per combination of channel, urgency and outcome, and how many tickets that
// was. The numbers are made up; the pattern in them is the ordinary one —
// what arrives by phone is more often urgent, and what is urgent is more often
// escalated.
func tickets() data.Source {
	var channel, urgency, outcome []string
	var count []float64
	add := func(c, u, o string, n float64) {
		channel, urgency, outcome = append(channel, c), append(urgency, u), append(outcome, o)
		count = append(count, n)
	}
	for _, r := range []struct {
		channel, urgency, outcome string
		count                     float64
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
		add(r.channel, r.urgency, r.outcome, r.count)
	}
	return data.NewTable().
		String("channel", channel).
		String("urgency", urgency).
		String("outcome", outcome).
		Float64("tickets", count)
}

// bare is the theme a mark that places its own layout wants: both axes
// describe the unit square, so there is nothing for a grid line, an axis line
// or a tick to mean.
func bare() theme.Theme {
	return theme.Light.With(
		theme.Grid(false, false), theme.AxisLines(false, false), theme.Ticks(false, false))
}

// ticketFlow is the form itself. Three columns of boxes and a ribbon per
// crossing, with geom.Value naming the column the table was already counted
// into — a row here is a combination and not a ticket.
func ticketFlow(out string) error {
	p := figure.New(
		figure.Size(760, 460),
		figure.Title("A quarter of support tickets, by channel, urgency and outcome"),
		figure.Theme(bare()),
		// The legend is how the boxes are named: a node here is a category in
		// a dimension, so its name carries both and the legend reads
		// "channel: phone". A chart of one layer has no legend by default,
		// because one series does not need telling apart from anything.
		figure.Legend(true),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.ParallelSets(tickets(),
		geom.Dims("channel", "urgency", "outcome"),
		geom.Value("tickets"),
		geom.Padding(0.02),
	))
	return p.Render(figure.SVG(out))
}

// ticketOutcomes colours the ribbons by the last column, which subdivides
// every one of them: the escalated share of each channel is a band a reader
// can follow from the first column to the last, rather than a number that only
// appears at the end.
func ticketOutcomes(out string) error {
	p := figure.New(
		figure.Size(760, 460),
		figure.Title("The same tickets, with the escalated share followed through"),
		figure.Theme(bare()),
		// With a colour column the legend names the classes instead, because
		// that is what the colours mean.
		figure.Legend(true),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.ParallelSets(tickets(),
		geom.Dims("channel", "urgency", "outcome"),
		geom.Value("tickets"),
		geom.ColorBy("outcome", scale.Qualitative(palette.OkabeIto)),
		geom.Padding(0.02),
		geom.Opacity(0.55),
	))
	return p.Render(figure.SVG(out))
}

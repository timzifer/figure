package main

import (
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// upsetFigure is the chart a count over a membership table unlocks, and the
// first one in this gallery whose two panels are one reading.
//
// An UpSet plot answers the question a Venn diagram is usually drawn for and
// usually answers wrongly: how many things are in exactly this combination of
// sets. The bars are those counts and the matrix under them says which
// combination each column is — two layers over one table, in a panel and a
// track sharing the X scale object, so a bar cannot drift off its own column.
// See docs/adr/0074-sets-are-counted.md.
//
// Nothing here is laid out. The only arithmetic is stat.Intersections, which
// counts; the rest is a bar chart and a dot matrix, which figure drew on day
// one.
func upsetFigure() plate {
	return plate{
		name: "upset", width: 720, high: 460,
		theme: theme.Light.With(theme.Ticks(false, true)),
		title: "What our customers subscribe to, together",
		opts:  []figure.Option{figure.YTitle("customers")},
		build: func(p *figure.Plot) {
			p.X(scale.Ordinal())
			p.Y(scale.Linear())
			p.Add(geom.Intersections(subscriptionTable(), membershipChannels()...))
			p.Track(figure.Bottom, figure.TrackSize(96)).
				Add(geom.SetMatrix(subscriptionTable(), membershipChannels()...))
		},
	}
}

// vennFigure is the same table as the diagram everybody asks for.
//
// It is here beside the UpSet plot on purpose: the seven numbers are the same
// seven counts, and the reason the form stops at three sets is visible in the
// picture rather than argued about in a record.
func vennFigure() plate {
	return plate{
		name: "venn", width: 520, high: 460,
		theme: theme.Light.With(
			theme.Grid(false, false), theme.AxisLines(false, false), theme.Ticks(false, false)),
		title: "The same three products, as a Venn diagram",
		build: func(p *figure.Plot) {
			p.X(scale.Linear())
			p.Y(scale.Linear())
			p.Add(geom.Venn(subscriptionTable(), membershipChannels()...))
		},
	}
}

// The membership table both figures read: one row per (customer, product)
// pair, which is the shape a join already has. The products are not declared
// anywhere — the order they are first mentioned in is the order they take
// their lanes and their colours in.
var (
	upsetWho = []string{
		"ann", "ann", "bob", "cas", "cas", "cas", "dev", "eli", "eli",
		"fay", "gus", "gus", "hal", "ivy", "ivy", "jo", "jo", "kai", "lee", "lee",
	}
	upsetWhat = []string{
		"mail", "drive", "mail", "mail", "drive", "chat", "drive", "mail", "chat",
		"chat", "mail", "drive", "mail", "drive", "chat", "mail", "drive", "chat", "mail", "drive",
	}
)

func subscriptionTable() data.Source {
	return figure.NewTable().String("who", upsetWho).String("what", upsetWhat)
}

func membershipChannels() []geom.Option {
	return []geom.Option{geom.From("who"), geom.To("what")}
}

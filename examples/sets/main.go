// Command sets renders the three charts a membership table pays for.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. The table is one row per (customer,
// product) pair — the shape a join already has — and every chart here is a
// reading of that one table. See docs/adr/0074-sets-are-counted.md.
//
// The first is an UpSet plot: a bar per combination of products, over a matrix
// saying which products that is. It is two layers rather than one, because the
// bars and the dots are two panels — the matrix is a track under the panel,
// sharing its X scale object, which is what keeps a bar over its own column.
//
// The second is the same chart with the set sizes beside it, which needs a
// third panel and therefore a figure.Grid rather than a track.
//
// The third is the Venn diagram of the same three products, which is the chart
// people ask for and the one that stops working at four.
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
	upset := flag.String("upset", "upset.svg", "output path for the UpSet plot")
	sizes := flag.String("sizes", "upset-sizes.svg", "output path for the UpSet plot with set sizes")
	venn := flag.String("venn", "venn.svg", "output path for the Venn diagram")
	flag.Parse()
	if err := run(*upset, *sizes, *venn); err != nil {
		fmt.Fprintln(os.Stderr, "sets:", err)
		os.Exit(1)
	}
}

func run(upset, sizes, venn string) error {
	for _, step := range []func() error{
		func() error { return upsetChart(upset) },
		func() error { return upsetWithSetSizes(sizes) },
		func() error { return vennChart(venn) },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// Who is subscribed to what. The membership list is the whole input: the
// products are not declared anywhere, and the order they are first mentioned in
// is the order they take their lanes and their colours in.
var (
	who = []string{
		"ann", "ann", "bob", "cas", "cas", "cas", "dev", "eli", "eli",
		"fay", "gus", "gus", "hal", "ivy", "ivy", "jo", "jo", "kai", "lee", "lee",
	}
	what = []string{
		"mail", "drive", "mail", "mail", "drive", "chat", "drive", "mail", "chat",
		"chat", "mail", "drive", "mail", "drive", "chat", "mail", "drive", "chat", "mail", "drive",
	}
)

func subscriptions() data.Source {
	return figure.NewTable().String("who", who).String("what", what)
}

// The membership channels, named once: an element and the set it is in are the
// two ends of a bipartite edge, so they are the channels a sankey reads.
func channels() []geom.Option {
	return []geom.Option{geom.From("who"), geom.To("what")}
}

// bars is how the top half is configured, and matrix how the bottom half is:
// the same ranking, so the same options. Handing the two halves different ones
// would put a bar over somebody else's dots.
func bars(opts ...geom.Option) geom.Geom {
	return geom.Intersections(subscriptions(), append(channels(), opts...)...)
}

func matrix(opts ...geom.Option) geom.Geom {
	return geom.SetMatrix(subscriptions(), append(channels(), opts...)...)
}

// upsetChart is the form itself: how many customers have exactly each
// combination of products, and which combination that is.
//
// The X ticks under the bars are turned off because the matrix underneath names
// the columns already — the one place where two panels sharing an axis want one
// set of labels between them.
func upsetChart(out string) error {
	p := figure.New(
		figure.Size(760, 460),
		figure.Title("What our customers subscribe to, together"),
		figure.YTitle("customers"),
		figure.Theme(theme.Light.With(theme.Ticks(false, true))),
	)
	p.X(scale.Ordinal())
	p.Y(scale.Linear())
	p.Add(bars(geom.Label("customers")))
	p.Track(figure.Bottom, figure.TrackSize(96)).Add(matrix())
	return p.Render(figure.SVG(out))
}

// upsetWithSetSizes puts the products' own totals beside the matrix.
//
// That is a third panel on the matrix's own Y axis, and a track is a band on
// the *plot's* scales rather than on another track's — so this is a
// [figure.Grid] of four cells, sharing two scale objects. The empty cell under
// the bars is where a printed UpSet leaves a gap too.
func upsetWithSetSizes(out string) error {
	cols, lanes := scale.Ordinal(), scale.Ordinal()
	counts, totals := scale.Linear(scale.Zero()), scale.Linear(scale.Zero())

	top := figure.New(figure.Title("customers per combination"))
	top.X(cols)
	top.Y(counts)
	top.Add(bars())

	dots := figure.New()
	dots.X(cols)
	dots.Y(lanes)
	dots.Add(matrix())

	// The set sizes: one bar per product, on the lanes the matrix uses, so the
	// bar and the row of dots line up. They are a different count from the bars
	// above — a customer with two products is in two of these and in one of
	// those.
	// A member plot's title is the label above its panel, and its own axis
	// titles are not used — the canvas belongs to the grid.
	side := figure.New(figure.Title("subscribers per product"))
	side.X(totals)
	side.Y(lanes)
	// A bar that runs across rather than up is a rect from zero to its value,
	// which is the recipe a gantt row already is: geom.Bar measures up Y.
	side.Add(geom.Rect(sizeTable(), geom.X("zero"), geom.X2("n"), geom.Y("what")))

	g := figure.NewGrid(2,
		figure.GridSize(900, 520),
		figure.GridTitle("What our customers subscribe to, together"),
		figure.GridRowHeights(0, 120),
		figure.GridColWidths(200, 0),
		figure.GridSharedX(true),
		figure.GridLegend(false),
	)
	g.At(0, 1, top)
	g.At(1, 0, side)
	g.At(1, 1, dots)
	return g.Render(figure.SVG(out))
}

// sizeTable is how many customers each product has, which is a count of the
// membership rows and not of the combinations.
func sizeTable() data.Source {
	order := []string{}
	n := map[string]int{}
	for _, s := range what {
		if _, seen := n[s]; !seen {
			order = append(order, s)
		}
		n[s]++
	}
	counts := make([]float64, 0, len(order))
	for _, s := range order {
		counts = append(counts, float64(n[s]))
	}
	return figure.NewTable().
		String("what", order).
		Float64("zero", make([]float64, len(order))).
		Float64("n", counts)
}

// vennChart is the picture everybody asks for, drawn so that it says what it
// looks like it says: each region holds the customers who have exactly those
// products, so the seven numbers add up to how many customers there are.
func vennChart(out string) error {
	p := figure.New(
		figure.Size(560, 520),
		figure.Title("The same three products, as a Venn diagram"),
		// Both axes describe the unit square here, which is nothing a reader
		// wants numbered — the theme every chart that places its own geometry
		// wants.
		figure.Theme(theme.Light.With(
			theme.Grid(false, false), theme.AxisLines(false, false), theme.Ticks(false, false))),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Venn(subscriptions(), channels()...))
	return p.Render(figure.SVG(out))
}

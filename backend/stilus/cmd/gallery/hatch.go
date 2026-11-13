package main

import (
	"math"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// The plates for the hatch channel: what it is for, what it does to a mark
// that is not a bar, and what the whole vocabulary looks like.
//
// See docs/adr/0069-hatching-as-the-third-redundant-channel.md.
func hatchFigures() []plate {
	return []plate{
		greyscaleFigure("greyscale-plain", "Colour only", theme.Light),
		greyscaleFigure("greyscale-hatched", "theme.Redundant(true)", theme.Light.With(theme.Redundant(true))),
		hatchedAreaFigure(),
		hatchKindsFigure(),
	}
}

// greyscaleFigure is the argument the channel exists for, and it is two plates
// because it is a comparison: the gallery lists figures two to a row, so these
// sit side by side there and the difference is the row.
//
// One stacked bar chart, twice, with the colour taken out of the palette rather
// than out of the picture afterwards — which is what a laser printer, a
// photocopier and a reader with deuteranopia each do to it in their own way.
// Plain, three products that were three distinguishable colours are three greys
// a reader has to measure against the legend and then against each other. With
// redundant encoding on, the greys are unchanged and the stack is readable
// anyway, because each product now carries a pattern as well.
//
// Every other hatched figure in this gallery is in colour, where a pattern
// looks like decoration. This pair is the case where it is not.
func greyscaleFigure(name, title string, th theme.Theme) plate {
	return plate{
		name: name, width: 620, high: 400, theme: th,
		title: title,
		opts:  []figure.Option{figure.Legend(true)},
		build: func(p *figure.Plot) {
			quarters, products, revenue := ledger()
			src := figure.NewTable().
				String("quarter", quarters).
				String("product", products).
				Float64("revenue", revenue)
			p.X(scale.Ordinal())
			p.Y(scale.Linear(scale.Nice(), scale.Zero()))
			// The palette is greyed rather than the render: a chart is printed
			// in greyscale by having no colour to print, and desaturating the
			// finished picture afterwards would demonstrate an image filter
			// rather than a chart.
			p.Add(geom.Bar(src,
				geom.X("quarter"), geom.Y("revenue"),
				geom.GroupBy("product"),
				geom.ColorBy("product", scale.Qualitative(greys(palette.OkabeIto))),
			))
		},
	}
}

// hatchedAreaFigure is the channel on a mark that is not a bar.
//
// Stacked areas are where colour runs out fastest: the bands touch, they are
// drawn at a fraction of their colour so that what is behind them stays
// visible, and four of them leave a reader comparing four pale tints. A pattern
// per band survives all three, and it costs nothing a bar does not cost — the
// same ladder, the same option, a different mark. The points on each crest are
// there for the same reason the patterns are: a band's top edge is the number
// it reports, and a reader following one across the chart is following that
// edge.
//
// The bands are drawn from cumulative edges rather than through geom.Stack so
// that the second layer can sit on the crest of each one. A scatter is not a
// stacking mark — see geom.Stack — and would put its marks at each series' own
// value rather than at the top of the band it belongs to. Both layers read the
// same two columns, so the points are on the crest by construction rather than
// by agreement.
func hatchedAreaFigure() plate {
	return plate{
		name: "hatched-area", width: 780, high: 430,
		theme: theme.Light.With(theme.Redundant(true)),
		title: "Traffic by channel, stacked",
		opts: []figure.Option{
			figure.XTitle("week"),
			figure.YTitle("visits"),
			figure.Legend(true),
		},
		build: func(p *figure.Plot) {
			weeks, names, lo, hi := stackedTraffic()
			src := figure.NewTable().
				Float64("week", weeks).
				String("channel", names).
				Float64("lo", lo).
				Float64("hi", hi)
			p.X(scale.Linear(scale.Nice()))
			p.Y(scale.Linear(scale.Nice(), scale.Zero()))
			p.Add(
				geom.Area(src,
					geom.X("week"), geom.Y("hi"), geom.Y2("lo"),
					geom.GroupBy("channel"),
					// The rows are already stacked, so the layer must not stack
					// them again: a grouped area stacks from zero unless it is
					// told otherwise, and doing it twice would draw each band
					// on top of the cumulative total it already carries.
					geom.Stack(geom.NoStack),
					geom.Tension(0.35),
				),
				geom.Scatter(src,
					geom.X("week"), geom.Y("hi"),
					geom.GroupBy("channel"),
					geom.Size(5),
					geom.Guide(false),
				),
			)
		},
	}
}

// hatchKinds is the whole vocabulary, in the order the enumeration declares it:
// the line family, the dots, the wavering lines, the tilings, and the empty
// pattern every ladder starts with.
var hatchKinds = []struct {
	hatch ir.Hatch
	name  string
}{
	{ir.HatchDiagonal, "diagonal"},
	{ir.HatchBackDiagonal, "back"},
	{ir.HatchCross, "cross"},
	{ir.HatchHorizontal, "horizontal"},
	{ir.HatchVertical, "vertical"},
	{ir.HatchGrid, "grid"},
	{ir.HatchDots, "dots"},
	{ir.HatchDotsStaggered, "staggered"},
	{ir.HatchZigzag, "zigzag"},
	{ir.HatchWave, "wave"},
	{ir.HatchBrick, "brick"},
	{ir.HatchTriangles, "triangles"},
	{ir.HatchScales, "scales"},
	{ir.HatchNone, "none"},
}

// hatchKindsFigure is that vocabulary drawn rather than listed.
//
// Four families, and the family is what decides where a pattern is worth using.
// Lines are cheap and read at any size. Dots point nowhere, so they survive a
// mark too small for a line hatch. The wavering lines are told apart from the
// straight ones by shape rather than by slope, which is what lets a ladder grow
// past the four slopes a straight line has. The tilings are rich, expensive in
// path points, and want an area to be read in — a treemap box, a sankey band,
// a region spanning the panel, not a four-pixel bar.
//
// It is one layer. The bars are grouped by their own name and the theme's
// ladder is the whole enumeration, so each bar takes the rung at its index —
// which is exactly how theme.Redundant hands patterns to the series of a real
// chart, demonstrated on a chart whose series are the patterns.
func hatchKindsFigure() plate {
	ladder := make([]theme.HatchStep, len(hatchKinds))
	names := make([]string, len(hatchKinds))
	ones := make([]float64, len(hatchKinds))
	for i, k := range hatchKinds {
		ladder[i] = theme.HatchStep{Hatch: k.hatch}
		names[i] = k.name
		ones[i] = 1
	}
	return plate{
		name: "hatch-kinds", width: 900, high: 260,
		theme: theme.Light.With(
			theme.Hatches(ladder...),
			// Coarser than a chart would use. These bars are samples rather
			// than data: the point is to see what each pattern *is*, and a
			// tiling at chart density reads as texture rather than as bricks.
			theme.HatchSize(9, 1),
			theme.Grid(false, false),
			theme.AxisLines(false, false),
			theme.Ticks(true, false),
		),
		title: "The fourteen patterns",
		// No legend: the bars are the samples and the axis names them, so a
		// column of fourteen swatches beside them would say it twice and take
		// the width that makes the patterns readable.
		opts: []figure.Option{figure.Legend(false)},
		build: func(p *figure.Plot) {
			src := figure.NewTable().
				String("pattern", names).
				Float64("one", ones)
			p.X(scale.Ordinal(scale.Categories(names...)))
			p.Y(scale.Linear())
			// One colour under all of them: this plate is about the patterns,
			// and a palette walking underneath would be a second reading
			// nobody asked for.
			p.Add(geom.Bar(src,
				geom.X("pattern"), geom.Y("one"),
				geom.GroupBy("pattern"),
				geom.Color(palette.Lerp(palette.Blue, ir.RGB(0xFF, 0xFF, 0xFF), 0.5)),
				geom.BarWidth(0.9),
			))
		},
	}
}

// greys is a qualitative palette seen without colour: every entry replaced by
// its own luminance.
//
// Rec. 709 weights, which is what a display, a print driver and the standard
// greyscale conversion all use — so these are the greys a reader actually gets
// rather than a flattering approximation of them. Two palette entries that
// differ mostly in hue collapse onto nearly the same grey, which is the failure
// the pair of plates above is about.
func greys(q palette.Qualitative) palette.Qualitative {
	out := make(palette.Qualitative, len(q))
	for i, c := range q {
		y := uint8(0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B) + 0.5)
		out[i] = ir.Color{R: y, G: y, B: y, A: c.A}
	}
	return out
}

// stackedTraffic is thirteen weeks of traffic for four channels, already
// stacked: each row carries the bottom and the top of its band rather than its
// own value.
//
// Stacking here rather than through geom.Stack is what lets a second layer draw
// on the crest — see [hatchedAreaFigure]. The numbers are a fixed formula, like
// every other figure in this gallery, so a fresh render and the committed one
// are the same picture.
func stackedTraffic() (weeks []float64, names []string, lo, hi []float64) {
	channels := []string{"search", "social", "direct", "email"}
	const n = 13
	for w := range n {
		floor := 0.0
		for i, name := range channels {
			// A slow swell per channel, out of phase with its neighbours, so
			// that the bands change width against each other without any of
			// them vanishing.
			v := 30 + 14*math.Sin(float64(w)/3+float64(i)*1.9) + 4*float64(i)
			weeks = append(weeks, float64(w+1))
			names = append(names, name)
			lo = append(lo, floor)
			hi = append(hi, floor+v)
			floor += v
		}
	}
	return weeks, names, lo, hi
}

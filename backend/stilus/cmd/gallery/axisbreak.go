package main

import (
	"time"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// The plates for axis breaks and folds: one interval left out of an axis
// because a single value would flatten the rest, and many left out because
// the reader does not want to look at them.
//
// See docs/adr/0083-an-axis-break-is-marked-or-not-drawn.md.
func axisBreakFigures() []plate {
	return []plate{
		axisBreakFigure("axis-break", "One site, thirty times the rest", theme.Light),
		axisBreakFigure("axis-break-zigzag", "theme.AxisBreaks(theme.BreakZigzag, 0, 10)",
			theme.Light.With(theme.AxisBreaks(theme.BreakZigzag, 0, 10))),
		machineFoldsFigure(),
	}
}

// axisBreakFigure is the chart a break exists for. Drawn on one unbroken axis,
// the outlier leaves every other bar a sliver at the bottom; broken between
// 12 and 85, each side reads at its own resolution and the bar that crosses
// the break is drawn with its middle cut out.
func axisBreakFigure(name, title string, th theme.Theme) plate {
	return plate{
		name: name, width: 620, high: 400, theme: th, title: title,
		opts: []figure.Option{figure.YTitle("requests per second (thousands)")},
		build: func(p *figure.Plot) {
			src := figure.NewTable().
				String("site", []string{"Berlin", "Hamburg", "Köln", "Frankfurt", "München", "Leipzig"}).
				Float64("load", []float64{6.2, 8.9, 4.1, 94.5, 10.3, 3.2})
			p.X(scale.Ordinal())
			p.Y(scale.Linear(scale.Domain(0, 100), scale.Break(12, 85)))
			p.Add(geom.Bar(src, geom.X("site"), geom.Y("load"), geom.Color(palette.SkyBlue)))
		},
	}
}

// machineFoldsFigure is a machine's state log over two shifts with every
// idle period folded off the time axis: the axis shows the time the machine
// was doing something, and a small slash marks each place the clock jumps.
func machineFoldsFigure() plate {
	t0 := time.Date(2026, 3, 2, 6, 0, 0, 0, time.UTC)
	at := func(h float64) time.Time { return t0.Add(time.Duration(h * float64(time.Hour))) }
	hours := []float64{0, 1.5, 3, 3.5, 4.5, 6, 7.5, 8, 9.5, 11, 12.5, 13, 14.5, 16}
	states := []string{"Aktiv", "Inaktiv", "Aktiv", "Störung", "Aktiv", "Inaktiv", "Rüsten",
		"Aktiv", "Inaktiv", "Aktiv", "Störung", "Aktiv", "Inaktiv"}
	start := make([]time.Time, len(states))
	end := make([]time.Time, len(states))
	for i := range states {
		start[i], end[i] = at(hours[i]), at(hours[i+1])
	}
	tbl := figure.NewTable().Time("start", start).Time("end", end).String("state", states)
	return plate{
		name: "machine-folds", width: 800, high: 260, theme: theme.Light,
		title: `Machine 7, "Inaktiv" folded out`,
		build: func(p *figure.Plot) {
			idle, err := figure.SpansWhere(tbl, "start", "end", "state", "Inaktiv")
			if err != nil {
				panic(err)
			}
			p.X(scale.Time(scale.TimeFold(idle...)))
			p.Y(scale.Ordinal(scale.Categories("Aktiv", "Rüsten", "Störung")))
			p.Add(geom.Rect(tbl,
				geom.X("start"), geom.X2("end"), geom.Y("state"),
				geom.ColorBy("state", scale.Qualitative(palette.Default)),
			))
		},
	}
}

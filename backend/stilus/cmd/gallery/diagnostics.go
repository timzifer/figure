package main

import (
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func qqFigure() plate {
	return plate{name: "qq", width: 640, high: 400, theme: theme.Light, title: "Normal QQ plot",
		opts: []figure.Option{figure.XTitle("Standard normal quantile"), figure.YTitle("Observed value")},
		build: func(p *figure.Plot) {
			src := data.NewTable().Float64("value", []float64{8, 9, 9.5, 10, 10.2, 11, 12, 14, 19})
			p.Y(scale.Linear(scale.Domain(6, 20)))
			p.Add(geom.QQ(src, geom.X("value"), geom.Size(7)))
		},
	}
}

func labelsFigure() plate {
	return plate{name: "label-placement", width: 640, high: 400, theme: theme.Light, title: "Nearby labels, placed in source order",
		opts: []figure.Option{figure.Legend(false)},
		build: func(p *figure.Plot) {
			src := data.NewTable().Float64("x", []float64{5, 5.05, 5.1, 5.15}).Float64("y", []float64{5, 5.02, 5.04, 5.06}).String("name", []string{"Alpha", "Beta", "Gamma", "Delta"})
			p.X(scale.Linear(scale.Domain(0, 10)))
			p.Y(scale.Linear(scale.Domain(0, 10)))
			p.Add(geom.Scatter(src, geom.X("x"), geom.Y("y")), geom.Text(src, geom.X("x"), geom.Y("y"), geom.TextBy("name"), geom.AvoidOverlap(true)))
		},
	}
}

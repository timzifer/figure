// Command diagnostics writes a normal QQ plot and a chart with placed labels.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

func main() {
	qq := flag.String("qq", "qq.svg", "QQ output path")
	labels := flag.String("labels", "labels.svg", "label output path")
	flag.Parse()
	if err := run(*qq, *labels); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(qq, labels string) error {
	src := data.NewTable().Float64("value", []float64{8, 9, 9.5, 10, 10.2, 11, 12, 14, 19})
	p := figure.New(figure.Size(640, 400), figure.Title("Normal QQ plot"), figure.XTitle("Standard normal quantile"), figure.YTitle("Observed value"))
	p.Y(scale.Linear(scale.Domain(6, 20)))
	p.Add(geom.QQ(src, geom.X("value")))
	if err := p.Render(figure.SVG(qq)); err != nil {
		return err
	}
	src = data.NewTable().Float64("x", []float64{5, 5.05, 5.1, 5.15}).Float64("y", []float64{5, 5.02, 5.04, 5.06}).String("name", []string{"Alpha", "Beta", "Gamma", "Delta"})
	p = figure.New(figure.Size(640, 400), figure.Title("Placed labels"), figure.Legend(false))
	p.X(scale.Linear(scale.Domain(0, 10)))
	p.Y(scale.Linear(scale.Domain(0, 10)))
	p.Add(geom.Scatter(src, geom.X("x"), geom.Y("y")), geom.Text(src, geom.X("x"), geom.Y("y"), geom.TextBy("name"), geom.AvoidOverlap(true)))
	return p.Render(figure.SVG(labels))
}

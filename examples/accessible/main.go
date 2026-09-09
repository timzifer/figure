// Command accessible renders a chart that can be read without being seen.
//
// It writes three files that are one chart:
//
//   - accessible.svg — the picture, carrying an accessible name and a
//     description a screen reader announces, and drawn with dashes and marker
//     shapes so that colour is not the only thing telling the layers apart;
//   - accessible.html — a page holding the picture and the same data as a
//     table, which is the fallback for a reader who cannot use the picture and
//     the honest answer to what is in it;
//   - accessible.txt — the description on its own, for a caption or an alt
//     attribute somewhere else.
//
// The labels are set as notation: the y axis is a fraction with a radical in
// it, which is what a standard error actually looks like.
//
//	go run ./examples/accessible
package main

import (
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/a11y"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/mathtext"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func main() {
	if err := run("."); err != nil {
		fmt.Fprintln(os.Stderr, "accessible:", err)
		os.Exit(1)
	}
}

// run writes the three files into dir and returns nothing but an error, so
// that the test can call it with a temporary directory.
func run(dir string) error {
	p := plot()
	summary := p.Describe()

	svgPath := filepath.Join(dir, "accessible.svg")
	if err := p.Render(figure.SVG(svgPath)); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "accessible.txt"),
		[]byte(summary.Title+"\n\n"+summary.Detail+"\n"), 0o644); err != nil {
		return err
	}

	var table strings.Builder
	if err := p.DataTable(&table); err != nil {
		return err
	}
	page := fmt.Sprintf(`<!doctype html>
<html lang="en">
<meta charset="utf-8">
<title>%s</title>
<h1>%s</h1>
<img src="accessible.svg" alt="%s">
<h2>The data</h2>
%s`,
		html.EscapeString(summary.Title),
		html.EscapeString(summary.Title),
		html.EscapeString(summary.Detail),
		table.String())

	return os.WriteFile(filepath.Join(dir, "accessible.html"), []byte(page), 0o644)
}

func plot() *figure.Plot {
	const n = 24
	x := make([]float64, n)
	measured := make([]float64, n)
	modelled := make([]float64, n)
	for i := range n {
		x[i] = float64(i)
		measured[i] = 12 + 6*math.Sin(float64(i)/3) + float64(i)/8
		modelled[i] = 12 + 5.5*math.Sin(float64(i)/3+0.1)
	}
	src := figure.Float64Columns(map[string][]float64{
		"sample": x, "measured": measured, "modelled": modelled,
	})

	p := figure.New(
		figure.Size(720, 420),
		// Redundant encoding: the second layer is dashed and its markers are a
		// different shape, so the chart survives greyscale and colour blindness.
		figure.Theme(theme.Light.With(theme.Redundant(true))),
		figure.Math(mathtext.TeX()),
		figure.Title("Signal against model"),
		figure.XTitle("sample"),
		figure.YTitle(`$\frac{\sigma}{\sqrt{n}}$ (mV)`),
		figure.Legend(true),
	)
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice(), scale.Zero()))
	p.Add(
		geom.Line(src, geom.X("sample"), geom.Y("measured"), geom.Label("measured")),
		geom.Line(src, geom.X("sample"), geom.Y("modelled"), geom.Label("modelled")),
		geom.HLine(18, geom.Label("limit")),
	)
	return p
}

// summarize is what the description looks like when a program wants the parts
// rather than the prose. It is here because it is the question anyone reading
// this example asks next.
func summarize(p *figure.Plot) []a11y.Series { return p.Describe().Series }

//go:build js && wasm

// Command web draws an interactive chart in a browser.
//
// Build it and serve the directory:
//
//	GOOS=js GOARCH=wasm go build -o examples/web/chart.wasm ./examples/web
//	cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" examples/web/
//	go run ./examples/web/serve   # or any static file server
//
// It is the browser story end to end: the same model that renders SVG on a
// server draws on a canvas, a pointer over it reports the row underneath, the
// wheel zooms, a drag pans, a double click resets the view, and clicking a
// legend row puts that series away.
package main

import (
	"math"
	"math/rand/v2"
	"strconv"
	"syscall/js"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/backend/canvas"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func main() {
	doc := js.Global().Get("document")
	el := doc.Call("getElementById", "chart")
	readout := doc.Call("getElementById", "readout")

	p := plot()

	// The tooltip. A handler is told what the pointer is over; drawing the
	// result is the page's business, not figure's.
	p.On(figure.Hover, func(ev figure.Event) {
		if !ev.Found {
			readout.Set("textContent", "—")
			return
		}
		text := ev.Series() + ": " +
			strconv.FormatFloat(ev.Hit.X, 'f', 2, 64) + ", " +
			strconv.FormatFloat(ev.Hit.Y, 'f', 2, 64)
		// Row identity is what a table beside the chart would highlight. It is
		// -1 for a mark no single row is behind, and off unless asked for —
		// see live.TrackRows below.
		if ev.Hit.Row >= 0 {
			text += "  (row " + strconv.Itoa(ev.Hit.Row) + ")"
		}
		readout.Set("textContent", text)
	})
	p.On(figure.Leave, func(figure.Event) { readout.Set("textContent", "—") })

	live, err := p.Live(canvas.Element(el, canvas.Clear(true)))
	if err != nil {
		js.Global().Get("console").Call("error", err.Error())
		return
	}
	defer live.Close()
	live.TrackRows(true)

	// A clickable legend. figure says a legend row was clicked and which
	// series it stands for; what that means is this program's — putting the
	// series away here, but it could as easily select it, or open something.
	// See docs/adr/0047-clickable-legend.md for why the four lines are here
	// rather than behind a flag on the chart.
	p.On(figure.Click, func(ev figure.Event) {
		if ev.Hit.Kind == figure.LegendRow {
			if err := live.Toggle(ev.Hit.Layer); err != nil {
				js.Global().Get("console").Call("error", err.Error())
			}
		}
	})

	if err := live.Draw(); err != nil {
		js.Global().Get("console").Call("error", err.Error())
		return
	}
	defer live.Bind(el)()

	// A wasm main that returns takes the page's Go runtime with it, so park.
	select {}
}

func plot() *figure.Plot {
	sample, value, load := series(4000)
	src := figure.Float64Columns(map[string][]float64{
		"sample": sample, "value": value, "load": load,
	})

	p := figure.New(
		figure.Theme(theme.Light),
		figure.Size(900, 460),
		figure.Title("Interactive"),
		figure.XTitle("sample"),
		figure.YTitle("value"),
		// A legend, because it is what the reader clicks to put a series away.
		figure.Legend(true),
	)
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice()))
	p.Add(
		geom.Line(src, geom.X("sample"), geom.Y("value"), geom.Color(palette.Blue), geom.Label("signal")),
		// No Label here: a layer coloured from a column titles its colourbar
		// with the label when it has one, and "load" is what that bar shows.
		// The tooltip falls back to the column the layer plots, which reads as
		// "value" — see render.layerLabel.
		geom.Scatter(src, geom.X("sample"), geom.Y("value"),
			geom.ColorBy("load", scale.Sequential(palette.Viridis)), geom.Size(4)),
		geom.HLine(2, geom.Label("threshold"), geom.Dash(6, 4)),
	)
	return p
}

// series is four thousand rows, which is enough that the chart is decimated
// when it is zoomed out and not when it is zoomed in — the thing an
// interactive chart is for.
func series(n int) (sample, value, load []float64) {
	r := rand.New(rand.NewPCG(17, 19))
	sample, value, load = make([]float64, n), make([]float64, n), make([]float64, n)
	for i := range n {
		sample[i] = float64(i)
		value[i] = math.Sin(float64(i)/90) + 0.35*r.NormFloat64()
		load[i] = math.Abs(value[i])
	}
	value[n/3] = 3.2 // a spike, so that decimation has something to keep
	return sample, value, load
}

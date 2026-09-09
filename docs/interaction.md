# Charts that answer to a pointer

The browser, the native window, the GPU tier, live data, and a chart that follows the surface it is drawn on.

## Interactive, in a browser

The same model that renders SVG on a server draws on a `<canvas>`, with the
pointer reporting what it is over:

```go
//go:build js && wasm

p.On(figure.Hover, func(ev figure.Event) {
	if ev.Found {
		readout.Set("textContent", fmt.Sprintf("%s: %.2f, %.2f", ev.Series(), ev.Hit.X, ev.Hit.Y))
	}
})

live, err := p.Live(canvas.Element(el))   // a surface, redrawn
defer live.Close()
live.Draw()
defer live.Bind(el)()                     // pointer, wheel zoom, drag pan, double-click reset
```

Hit-testing works over the marks the render actually emitted, so it is right for
every geom — including a decimated one, where the rows you can point at are
exactly the rows on screen ([ADR 0015](adr/0015-hit-testing.md)). Zoom and
pan are arithmetic on the scales, so the value under the pointer stays under the
pointer on a log or a time axis as much as on a linear one.

Turn on row identity when a hit has to name a row rather than describe a point —
highlighting the matching row of a table beside the chart is the case:

```go
live.TrackRows(true)

p.On(figure.Hover, func(ev figure.Event) {
	if ev.Found && ev.Hit.Row >= 0 {
		highlightTableRow(ev.Hit.Row)   // a row of the table you handed in
	}
})
```

It is off by default and costs a position and a row number per mark; it does not
cost per-frame allocations, and CI pins that. Decimation is not in the way —
LTTB and min/max keep *real* rows — and neither is faceting, whose per-panel
cuts are resolved back to the table you passed. A mark that no single row is
behind — a boxplot's box, a density raster, an interpolated point across a
gap — reports `-1` rather than a plausible neighbour.

A runnable version is in [`examples/web`](../examples/web):

```sh
GOOS=js GOARCH=wasm go build -o examples/web/chart.wasm ./examples/web
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" examples/web/
```

## Interactive, in a window

The same model again, on the desktop:

```go
import (
	"github.com/timzifer/figure/backend/window"
	"github.com/timzifer/figure/backend/window/show"
)

func main() {
	p := figure.New(figure.Responsive(true), figure.Title("Signal"))
	p.Add(geom.Line(src, geom.X("t"), geom.Y("y")))

	// Hover, drag to pan, wheel to zoom about the pointer, double click to reset.
	log.Fatal(show.Plot(p, window.Title("Signal"), window.Size(900, 560)))
}
```

The window comes from [`gogpu/gogpu`](https://github.com/gogpu/gogpu) —
Windows, macOS, X11 and Wayland, no cgo — and the chart is drawn by the same CPU
rasterizer that writes your PNGs, presented as one texture per changed frame. So
a window shows exactly what a file would; there is one implementation of every
mark rather than two that disagree ([ADR 0021](adr/0021-native-window.md)).

It is also cheap when nothing is happening: the loop blocks on the operating
system's event queue, figure paints nothing when a frame is identical to the
last, and the window uploads no texture when the pixels have not changed.

The steering — is this move a hover or a drag, was that release a click — is
`figure.Input`, in the core, and it is the same state machine `Live.Bind` uses
in a browser. Drive it yourself if you want different controls:

```go
in := live.Input()
in.Down(x, y); in.Move(x, y); in.Up(x, y)   // press, pan, release
in.Wheel(x, y, deltaY)                      // zoom about the pointer
in.Resize(w, h)                             // lay out again at a new size
```

A runnable version is [`backend/window/cmd/demo`](../backend/window/cmd/demo):

```sh
cd backend/window && go run ./cmd/demo
```

### The GPU tier, opt-in

```go
import _ "github.com/timzifer/figure/backend/gg/gpu"
```

That import registers gg's GPU accelerator, and every chart rasterized
afterwards — in a window or into a file — uses it. It is a module of its own so
that the import is the opt-in: `backend/gg` never links `wgpu`, and a program
that wants a PNG on a server links no GPU stack at all
([ADR 0022](adr/0022-gpu-tier.md)). On a machine with no usable device the
registration fails quietly and gg falls back to the CPU, so the chart still
renders; `gpu.Enabled()` says which way it went.

It stays **opt-in beta**. For server-side stills the CPU rasterizer
and the vector emitters are the supported path.

## Live data

A `data.Stream` is appended to from one goroutine and frozen for the renderer on
another. It is deliberately not a `Source`: a table being appended to between
two column reads is a table that disagrees with itself.

```go
st := data.NewStream("t", "y").Window(2000)
p.Add(geom.Line(st.Source(), geom.X("t"), geom.Y("y")))

go func() {
	for s := range samples {
		st.Append(scale.Nanos(s.At), s.Value)   // any goroutine, any time
	}
}()

for range ticker.C {
	st.Snapshot()   // freeze what has arrived
	live.Draw()     // draw the frozen view
}
```

Each `Draw` compares the frame with the last one and repaints only where they
differ; a frame identical to the last is not painted at all. A backend says it
can do that by implementing `ir.Partial`, and one that cannot gets the whole
frame as before ([ADR 0016](adr/0016-streaming-and-damage.md)). Appending a
row and freezing a view both allocate nothing in the steady state, and the
benchmark gate keeps it that way.

See [`examples/stream`](../examples/stream).

## Charts that follow their surface

```go
p := figure.New(figure.Size(800, 500), figure.Responsive(true))
// ...
live.Resize(400, 250)   // half the size: half the type, half the strokes
```

A plot is designed at one size and often drawn at another. `Responsive` scales
the theme — type, strokes, spacings, markers, margins — by how much smaller or
larger the drawing is, so a chart at a third of its design size is the chart
rather than a photograph of it. At the design size the factor is exactly 1, so
turning it on cannot change a still you already have
([ADR 0025](adr/0025-responsive-charts.md)).

`Live.Resize` is what a window's resize event and a reflowed canvas call. The
scales keep whatever they were zoomed to: a reader who dragged a view into place
has not asked to leave it.

For a still at another size — a thumbnail of a chart designed larger — name the
design explicitly:

```go
figure.New(figure.Size(200, 125), figure.ResponsiveFrom(800, 500))
```

## Nanoseconds at any zoom

A Unix nanosecond count in this century needs 61 bits, and a float64 has 53. Two
instants a nanosecond apart are therefore the *same number*, and an axis zoomed
to a microsecond window has nothing left to separate them with.

```go
p.X(scale.Time(scale.Origin(runStart)))   // the domain is nanoseconds since runStart
```

With an origin near the data, the subtraction happens in `int64` and the axis
keeps whole nanoseconds for the hundred days either side of it that a float64
counts exactly. A geom reading a time column goes through the axis's own space,
so nothing needs converting by hand, and the JSON spec carries the origin so a
document reads back as the same axis.

---

**[README](../README.md)** · **[CONCEPT](../CONCEPT.md)** · **[ADRs](adr)** · [The gallery](gallery.md) · [Chart forms](charts.md) · [A million rows](scale-out.md) · [Reading a chart](reading.md) · [JSON and Arrow](spec.md) · [Features](features.md) · [Chart-type catalogue](chart-types.md) · [Benchmarks](benchmarks.md) · [How it was built](milestones.md)

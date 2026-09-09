# figure

**A grammar-driven plotting library for Go: one model, many backends, runs everywhere — built on the GoGPU stack.**

> Status: **`v0.x`.** The model is settled and the API is not frozen: three
> seams that take positional scale arguments are being reshaped so that a chart
> can gain a dimension additively
> ([ADR 0056](docs/adr/0056-three-dimensional-charts.md),
> [ADR 0060](docs/adr/0060-parameter-structs-at-every-seam.md)), and `v1.0.0`
> returns when they are done. [§14](#14-what-is-built-and-what-is-next) says
> what exists and what is next.

> **Implementation note.** Where this document and the code disagree, the code
> wins and this document is wrong — please fix it. Every decision
> [§17](#17-the-decisions-this-design-rests-on) names has a record in
> [docs/adr](docs/adr/).

> **Revision note (this version):** the rendering strategy has been rewritten
> around the **GoGPU** ecosystem — `gogpu/gg` (2D graphics), `gogpu/wgpu` (pure-Go
> WebGPU), `gogpu/gogpu` (windowing). An earlier draft assumed figure would need
> its own rasterizer, its own GPU path (possibly cgo/Vello), and its own text
> stack. `gogpu/gg` already provides all of that in pure Go with zero CGO, so
> figure now sits *on top of* it. This shrinks figure to its actual value —
> the grammar, data, and interaction layers — and keeps the whole stack cgo-free.
> See [§4a](#4a-relationship-to-gogpu) and [§10](#10-rendering-via-gogpugg).

---

## 1. One-paragraph pitch

`figure` turns a single, declarative chart specification into any output you
need — SVG, PNG, PDF, an interactive browser canvas, or a GPU-accelerated native
window — from **the same model**, with **the same visual result**. It renders
through the **GoGPU** stack, which is **pure Go with zero CGO**, so figure
compiles to a single static binary (`CGO_ENABLED=0`), runs unchanged on a server,
cross-compiles to any target, and runs in the browser via WebAssembly. GPU
acceleration exists and is also cgo-free, but it is strictly **opt-in**: the
default path is a high-quality CPU rasterizer. figure's own job is the part
GoGPU does not do — the grammar of graphics, the data layer, and interaction.

The name is the thesis. A figure is what a chart is called once it is finished
and placed — in a paper, a report, a dashboard — and what makes it a figure is
the model behind it, not the format it was saved in. One figure is described
once; a spectrum of output formats comes out.

---

## 2. Motivation

Go's plotting ecosystem is real but fragmented, and no single library owns the
combination this project targets:

- **`gonum/plot`** — the de-facto standard. Pure Go, static-only, verbose API,
  no grammar-of-graphics model, no faceting, weak big-data handling.
- **`go-echarts`** — emits HTML + JavaScript wrapping Apache ECharts. Great in a
  browser, but there is no Go renderer underneath; server-side PNG needs a
  headless browser. Chart logic lives in JS.
- **`go-chart`** — simple raster/SVG charts, limited model.
- **Gio / Cogent Core** — GUI toolkits, not plotting libraries; their desktop GPU
  paths historically pull in cgo.
- **Plotly / Vega-Lite** — the reference for declarative, serializable,
  interactive charts, but not Go.

Two things changed the calculus versus older Go plotting attempts. First, a
grammar-driven, serializable model (à la Vega-Lite) is now the expected shape of a
modern charting library. Second — and decisively — **the rendering layer that used
to be the multi-year blocker now exists in pure Go**: the GoGPU stack provides a
Skia-quality 2D engine, a pure-Go WebGPU implementation, and a windowing layer,
all cgo-free. That removes the single biggest reason ambitious Go plotters stalled.

So the unoccupied position is: **a grammar-driven Go plotting library whose entire
stack is cgo-free and portable, that renders identically across static, web, and
(optionally) GPU backends, and that scales up to big data.** That is the space
`figure` claims. It does not try to be a renderer; it is the grammar on top of one.

---

## 3. Guiding principles

1. **Pure Go, zero CGO, end to end.** The core, the standard backends, and the
   optional GPU path are all cgo-free. `CGO_ENABLED=0` builds and cross-compiles
   everywhere, including `GOOS=js` for the browser.

2. **Decouple "process millions of points" from "interact with millions of points
   at 60 fps."** The former is CPU aggregation (decimation, binning → raster),
   pure Go, covers big-data *stills* completely. Only the latter needs a GPU.

3. **One model → many backends, identical output.** The user builds a spec once.
   Backends are interchangeable consumers of a small intermediate representation.

4. **Don't build a renderer — stand on one, but stay insulated.** figure renders
   through `gogpu/gg`. It does **not** reimplement rasterization, text shaping,
   GPU pipelines, or vector export. To avoid being welded to a young, fast-moving
   dependency, figure defines its own thin `Backend` interface and small IR; the
   GoGPU integration is *one* backend behind that interface (see [§9](#9-intermediate-representation-ir), [§10](#10-rendering-via-gogpugg)).

5. **Correctness is testable or it isn't real.** Golden-image tests from day one.
   `gogpu/gg` already ships golden-image infrastructure and a deterministic CPU
   rasterizer; figure's tests build on that.

---

## 4. Positioning & differentiation

| | Rendering | Web / interactive | Big data | cgo | Model |
|---|---|---|---|---|---|
| gonum/plot | raster, SVG, PDF | no | none | free | verbose, no GoG |
| go-echarts | HTML + JS (ECharts) | yes (browser) | via ECharts | free¹ | chart-type |
| go-chart | raster, SVG | no | none | free | simple |
| Gio / Cogent Core | GPU native | yes | partial | cgo (desktop) | GUI toolkit |
| Plotly / Vega-Lite | Canvas / SVG (JS) | yes (reference) | yes | — (not Go) | GoG / spec |
| **figure (goal)** | SVG / raster / PDF / GPU / browser via one model | yes (Wasm, cgo-free) | two tiers | **zero CGO throughout** | GoG-lite + Vega-Lite-compatible spec |

¹ go-echarts is cgo-free but needs a headless browser for server-side images.

### The sharp USPs

1. **Zero-CGO end to end, including GPU.** Static binary on a server, Wasm in the
   browser, optional GPU in a native window — all without a C toolchain. No other
   Go charting library offers this whole span.
2. **One spec → many backends, identical output.** gonum/plot can't do
   web/interactive; go-echarts can't do native server-side images without a
   browser. figure does both from one specification.
3. **Big data in two decoupled tiers** — CPU aggregation always, GPU interaction
   optional.
4. **Vega-Lite-compatible JSON spec** — ship a chart from a Go backend to a
   Go-Wasm *or* JS frontend that renders it identically.
5. **First-class concurrency & allocation discipline**, inherited from and
   extending `gogpu/gg`'s zero-alloc hot paths.
6. **Serious text for free** — real shaping (GSUB/GPOS, ligatures, kerning, CJK,
   bidi, color emoji, variable fonts) via `gogpu/gg`'s pure-Go font stack.

### 4a. Relationship to GoGPU

figure is to `gogpu/gg` what a plotting library is to a canvas: gg is the canvas
and the pen; figure is the grammar that decides *what* to draw. The split of
responsibilities:

| Concern | Owner |
|---|---|
| Rasterization (CPU + GPU), AA, dashes, gradients, blend modes, clipping/masks | `gogpu/gg` |
| Text: font parsing, shaping (GSUB/GPOS), layout, MSDF/glyph-mask rendering | `gogpu/gg` |
| Vector export (SVG, PDF) | **figure** — two built-in emitters, no dependency. `gg-pdf` cannot draw geometry ([ADR 0009](docs/adr/0009-pdf-backend.md)) |
| GPU device / backends (Vulkan, Metal, DX12, GLES, software, browser WebGPU) | `gogpu/wgpu` |
| Native window + input | `gogpu/gogpu` |
| Linear-space color, HiDPI/device scale, damage tracking | `gogpu/gg` |
| **Scales, geoms, stats, coords, facets, guides, layout, ticks** | **figure** |
| **Data layer (columnar/batch, Arrow, decimation)** | **figure** |
| **Serializable spec (Vega-Lite-compatible), interaction/event model** | **figure** |

**Honest caveats about depending on GoGPU.** The stack is young (`v0.x`, rapid
release cadence, small maintainer base — its own materials describe reaching
~100K LOC extremely fast). The native GPU backends are impressive but unproven
across the full real-world hardware matrix, and the zero-CGO FFI approach (goffi)
has shown sensitivity to Go's internal ABI across versions. figure's mitigations:
render through figure's own `Backend` interface so a gg API change is contained to
one adapter; pin exact GoGPU versions; treat the **CPU rasterizer as the stable,
supported path** and the **native GPU tier as opt-in beta**; and keep a
**zero-dependency SVG backend** (see [§10](#10-rendering-via-gogpugg)) so the
leanest static use case does not depend on GoGPU at all.

---

## 5. Non-goals

- **Not a dashboarding framework.** No server, no WebSocket layer.
- **Not a GUI toolkit.** It renders into a window (via `gogpu/gogpu`); it does not
  own widgets or the event loop.
- **Not a grammar-of-graphics purist clone.** A pragmatic subset ("GoG-lite").
- **Not a renderer.** Rasterization, text, and GPU pipelines belong to GoGPU.
- **Not a text-layout engine.** Chart text is short single-line runs (ticks, axis
  titles, legends, annotations). Paragraph wrapping, BiDi paragraph flow, and
  editable/cursored text are **out of scope**; shaping is delegated to the active
  backend (see [§9](#9-intermediate-representation-ir), [§10](#10-rendering-via-gogpugg)).
- **No 3D until much later**, and only then tightly scoped.

**Positioning.** figure is a **standalone, permissively-licensed** library. It
never depends on a specific application framework. The dependency runs the other
way: a UI framework such as `lux` may *embed* figure as a charting widget and
supply its own text/rendering backend. This keeps figure's license unencumbered
(it can be embedded by projects with any license, including dual-licensed ones)
and forbids pulling framework-specific code (e.g. a UI framework's rich-text
stack) into the core.

---

## 6. Architecture overview

One-directional lowering from a high-level model to a backend:

```
   User spec (GoG-lite API)  ──►  Model  ──►  IR  ──►  Backend interface  ──►  output
   ────────────────────────      ─────      ────      ─────────────────       ──────
   geoms, scales, coords,        resolved   small     svg (built-in) │         SVG
   facets, theme, guides         layout,    backend-  gg  (adapter) ─┼──►      PNG/PDF/
                                 ticks,     agnostic                 │         GPU frame/
                                 mappings   drawing                  │         window/
                                            ops                      │         browser
```

- **Data** enters through a batch/columnar interface ([§7](#7-data-layer)).
- **The model** ([§8](#8-model-layer-gog-lite)) resolves the abstract chart into
  concrete geometry and text runs.
- **The IR** ([§9](#9-intermediate-representation-ir)) is a small set of drawing
  primitives consumed by any `Backend`.
- **Backends** ([§10](#10-rendering-via-gogpugg)): a built-in zero-dependency SVG
  emitter, and a `gg` adapter that unlocks raster, PDF, GPU, browser, and
  interactive rendering — all cgo-free.

### Design tenets that resolve known tensions

- **Batch data access, not scalar** — the data interface returns typed columns,
  never one value at a time ([§7](#7-data-layer)).
- **Float64 origin rebasing for deep zoom** — subtract a per-view f64 anchor on the
  CPU, hand gg/GPU f32 deltas; never "fix it in the shader" after precision is lost.
- **Thin IR, adapter-based lowering** — figure's IR is small and stable; the gg
  adapter maps it to `gg.Context`/`scene`, insulating figure from gg churn.
- **Decimation is a family, not an algorithm** — LTTB (lines), min/max-per-column
  (envelopes), density binning (point clouds) ([§12](#12-big-data-strategy)).

---

## 7. Data layer

**Batch/columnar in, zero-copy where possible, Arrow as an optional adapter.**

```go
// Source exposes columnar, batch access. A Column carries typed slices the
// caller must not mutate; figure never copies when it can borrow.
type Source interface {
    Len() int
    Columns() []string
    Column(name string) (Column, bool)
}

// A Column is one value that grows: a kind, the values of that kind, which
// rows are absent, and how a row reads as text. A kind added later is a field
// here rather than a method above ([ADR 0061](docs/adr/0061-columns-are-one-value.md)).
type Column struct {
    Kind    Kind
    Floats  []float64
    Ints    []int64
    Strings []string
    Times   []time.Time
    Nulls   []bool
    Text    func(i int) string // optional; nil spells by kind
}
```

- **Zero-copy common case** — a `[]float64`-backed source returns its slice
  directly.
- **Arrow adapter as a separate module** (`figure/arrow/v18`) so the core never links
  Apache Arrow. A null-free `float64` column is borrowed
  outright, everything else converts once and caches, and an Arrow null becomes
  a `NaN` so that one missing-data policy covers both
  ([ADR 0013](docs/adr/0013-arrow-adapter.md)).
- **Missing-data policy is explicit** — `NaN`/`Inf`: interpolate, gap, or error.
  (gg is already NaN-safe at the path level, so a gap never corrupts a render.)
  The same policy covers a value the *scale* cannot place — zero on a
  log axis, a category outside a fixed set — because from the chart's point of
  view those are the same failure ([ADR 0008](docs/adr/0008-categorical-axes.md)).
- **Streaming** via a snapshot model ([§11](#11-concurrency--allocation)):
  produce on one goroutine, render a consistent snapshot on another:
  `data.Stream`, which is deliberately *not* a `Source` — a table being
  appended to between two column reads is a table that disagrees with itself,
  so the only way to draw one is to freeze it
  ([ADR 0016](docs/adr/0016-streaming-and-damage.md)).

---

## 8. Model layer (GoG-lite)

The part figure actually builds. Building blocks:

- **Scales** — `Linear`, `Log`, `SymLog`, `Time`, `Ordinal`/categorical, plus
  color scales (sequential, diverging, qualitative). Scales own data→visual
  mapping and tick generation.
- **Coordinate systems** — Cartesian and polar, behind a genuinely pluggable
  stage: a scale keeps mapping a value into an interval, and the coord decides
  what the interval means and turns a pair of them into a device point.
  `Cartesian` is the identity, so the stage costs an existing geom nothing and
  the golden files prove it ([ADR 0018](docs/adr/0018-coordinate-systems.md)).
  Geographic projections are a wider seam — they transform every point with no
  linear interval underneath — and stay out of scope for now.
- **Geoms** — `Line`, `Scatter`, `Bar`, `Area`, `Step`, `Boxplot`, `Rect`, and
  the distribution marks `Histogram`, `Violin`, `Ridgeline`, `Hexbin`,
  `Beeswarm`, `ECDF` and `Trend`. Geoms know *what* their shape is, never *how*
  it's rendered.
- **Groups and position adjustments** — a layer over a long table is N series,
  split by `geom.GroupBy` and painted from a discrete colour scale; `Stack`,
  `Dodge` and the streamgraph offsets are defined over those groups. The
  offsets are derived in `Train`, because the axis has to describe the totals
  ([ADR 0019](docs/adr/0019-position-adjustments.md)).
- **Stats / transforms** — `Bin` (histogram), `KDE` (density with Silverman's
  bandwidth rule), `Loess` (locally weighted regression), `ECDF`, `Hex`
  (hexagonal binning) and the decimation family. Numbers in, numbers out, each
  with an `Append` form and a determinism test. A distribution stat runs in
  `Train` and the axis is trained on its output, because the summary is what the
  axis has to describe ([ADR 0028](docs/adr/0028-distribution-stats.md)).
- **Facets** — `facet.Wrap` / `facet.Grid`, shared or free scales.
- **Aesthetic channels** — position, colour (`geom.ColorBy` through a
  `ColorScale`) and size (`geom.SizeBy` through a `scale.SizeScale`, mapped by
  area rather than by radius, which is the bubble chart).
- **Guides** — legends, colorbars, size keys, axes as positionable elements. The
  three keys are one ordered column with one stacking rule
  ([ADR 0027](docs/adr/0027-size-channel-and-the-guide-column.md)).
- **Annotations** — reference lines, spans, callouts, arrows.
- **Theme** — global tokens (fonts, palettes, grid styling, dark/light), applied
  as a resolved layer.

### Layout

A **constraint-based layout** (flex/grid-like) with margins, padding, relative
sizing, and — critically — **axis alignment** across subplots. Faceting builds on
it. (gg supplies device-scale/HiDPI awareness underneath.)

---

## 9. Intermediate representation (IR)

A small, backend-agnostic scene description, kept deliberately thin so it maps
cleanly onto `gg.Context` (immediate) or `gg/scene` (retained) — and onto the
built-in SVG emitter without gg.

Primitive set, frozen early and unchanged since:

- `Polyline` — points, stroke style (width, dashes, joins, caps).
- `FilledPath` — path + fill (solid / gradient), fill rule.
- `TextRun` — a single-line string with font reference, size, style, and anchor
  position. figure passes strings, never pre-shaped glyphs: **shaping belongs to
  the active backend** (gg's shaper for the gg backend; the SVG viewer for the SVG
  backend; a host framework's text stack if figure is embedded). figure owns only
  *placement* — formatting, rotation, and overlap/collision avoidance. Paragraph
  layout is not represented in the IR (see [§5](#5-non-goals)).
- `Marker` / `Instances` — one marker shape at N positions (scatter fast path;
  maps to gg SDF/instancing on GPU).
- `Image` — raster blit (the density-raster big-data path).
- `Group` / `Clip` — transform + clip region, nestable.

The `Backend` interface is what every renderer implements:

```go
type Backend interface {
    Polyline(pts []f32.Point, style Stroke)
    FillPath(p Path, fill Fill, rule FillRule)
    Text(run TextRun)
    Markers(shape Marker, at []f32.Point, style Style)
    Image(img image.Image, dst f32.Rect)
    Push(clip *Path, xform Affine); Pop()
    Flush() error

    // Measure reports advance width and bounding box for a run, so layout can
    // size margins, space ticks, and detect label overlap. It is the ONLY text
    // capability figure requires beyond drawing — figure has no shaper.
    Measure(run TextRun) TextMetrics
}
```

The `Measure` seam is the whole of figure's "text problem": layout needs exact
metrics from whatever shaper will actually draw the text. The gg backend answers
from gg's shaper (exact); a host-framework backend answers from its own stack
(exact); the built-in SVG backend answers from a lightweight pure-Go metrics
reader (`hmtx`/`cmap`, approximate for complex scripts — acceptable, since SVG
text is shaped by the viewer anyway).

**Corrected by implementation.** Geometry is identical across backends *given
identical metrics* — but the metrics are not identical, so the layout differs
slightly, and the imprecision is a little wider than this section originally
claimed. Even when both backends are handed the same font file, gg hints glyph
advances to whole pixels while figure's own `hmtx` reader sums them unrounded,
and the two read vertical metrics from different tables. Measured for Go
Regular: advances differ by at most about half a pixel per glyph, the font box
height by about 15%, and the resulting plot rectangle by a few pixels. Those
bounds are asserted in `backend/gg/parity_test.go`, so they cannot drift
unnoticed; see [ADR 0003](docs/adr/0003-text-and-fonts.md).

Two implementations ship: `backend/svg` (pure `encoding/xml`, zero deps) and
`backend/gg` (adapter to `gogpu/gg`, in a separate nested module).

---

## 10. Rendering via gogpu/gg

The rendering tiers, organized by what they cost the user's build:

### The built-in SVG backend — zero dependencies

`backend/svg` emits SVG with nothing but the standard library. This is the leanest
possible target: a static chart with no GoGPU dependency at all. It exists so the
minimal use case ("give me an SVG on a server") pulls in nothing native and
nothing young. Text is emitted as `<text>` elements — the SVG viewer or browser
shapes them — and measured via a lightweight `hmtx`/`kern` reader for layout; no
shaper is linked.

### The gg backend — everything else, still zero CGO

`backend/gg` (module `github.com/timzifer/figure/backend/gg`) adapts the IR onto `gogpu/gg` and unlocks the
full span, all `CGO_ENABLED=0`:

- **Raster (PNG/JPEG/WebP)** via gg's CPU rasterizer (Skia-AAA analytic AA, smart
  per-path algorithm selection). This is the **stable, supported** rendering path.
- **PDF and SVG** are built-in emitters in the core module rather than
  recordings replayed through gg. `gg-pdf` cannot draw geometry — its path
  operations reach a stub in `gxpdf` — so routing PDF through the recording API
  produces pages containing tick labels and nothing else
  ([ADR 0009](docs/adr/0009-pdf-backend.md)).
- **Text** — gg's pure-Go font stack: GSUB/GPOS shaping, ligatures, kerning,
  variable fonts, OpenType features, CJK, bidi, color emoji. figure adds no text
  dependency of its own.
- **GPU acceleration (opt-in beta)** — but not from this
  module. `backend/gg` still must not import `gg/gpu`
  ([ADR 0006](docs/adr/0006-gg-coupling-surface.md)), so the tier is a nested
  module of its own, `backend/gg/gpu`, and importing it is the opt-in
  ([ADR 0022](docs/adr/0022-gpu-tier.md)). Transparent CPU fallback when no GPU
  is present.
- **Browser (`GOOS=js`)** is `backend/canvas` in the core: gg has no
  `syscall/js` at the pinned version, and a canvas 2D context needs nothing
  outside the standard library
  ([ADR 0017](docs/adr/0017-browser-backend.md)).
- **Native window** via `gogpu/gogpu` for desktop interactive plots:
  the nested module `backend/window`. It draws with this module's CPU
  rasterizer and presents the result as a texture, so a window and a file are
  the same picture ([ADR 0021](docs/adr/0021-native-window.md)).

### What this buys figure (and what it must still not assume)

gg already provides: linear-space color, dashes, gradients, 29 blend modes,
clipping/masks, HiDPI/device scale, damage-aware partial repaint, zero-alloc hot
paths, and golden-image test infrastructure. figure inherits all of it.

What figure must **not** naively assume: that the young native GPU backends are
production-grade on every GPU (hence opt-in beta), and that gg's API is stable
(hence the `Backend` interface and pinned versions). For high-volume server-side
stills, prefer the CPU rasterizer or the built-in SVG emitter over the software
GPU path, which is a compatibility fallback, not a throughput path.

---

## 11. Concurrency & allocation

- **Parallel subplots** — independent subplots build IR on separate goroutines,
  composited at the end: each panel records into an
  `ir.Recorder` and the recordings replay into the backend *in panel order*, so
  a parallel render emits byte-identical output to a serial one
  ([ADR 0012](docs/adr/0012-parallel-panels.md)).
- **Snapshot streaming** — producer and renderer never share mutable state; the
  renderer reads an immutable snapshot (copy-on-swap), and only the dirty
  regions are repainted: `data.Stream` is the snapshot and the
  swap, `ir.Damage` compares two recordings to find what moved, and `ir.Partial`
  is the one-method interface a backend implements to act on it. The damage is
  computed above the backends rather than inside one, because gg is one backend
  of four and the browser backend is not it
  ([ADR 0016](docs/adr/0016-streaming-and-damage.md)).
- **Allocation discipline** — figure keeps its IR construction allocation-light
  and leans on gg's zero-alloc fill/stroke/text paths. A benchmark gate asserts no
  per-frame allocations on the hot path: everything sized by the
  data comes from a pool, so a steady-state frame costs the same handful of
  allocations over a thousand rows and over a million.
  `TestARenderDoesNotAllocatePerPoint` is the gate as a test, and CI's
  `Benchmarks and the allocation gate` job is the gate as a benchmark —
  `.github/scripts/allocgate.awk` reads `go test -bench` output and enforces the
  same property from the numbers, over all three modules.

---

## 12. Big-data strategy

Two decoupled tiers:

- **CPU tier (always, pure Go).** Aggregate before rendering, in `stat/`,
  applied by geoms at draw time ([ADR 0011](docs/adr/0011-decimation.md)):
    - **LTTB** for line/time-series (sorted x, one y per x; lossy but shape-preserving).
    - **Min/max-per-pixel-column** for signal envelopes and staircases.
    - **Density binning → raster** (datashader-style) for large scatter / point
      clouds; emit an `Image` primitive.
- **GPU tier (opt-in).** Interactive pan/zoom over the full dataset at framerate
  via the gg GPU backend, as a nested module whose import
  is the opt-in: `_ "github.com/timzifer/figure/backend/gg/gpu"` registers gg's
  accelerator, and importing nothing leaves `backend/gg` exactly as it was
  ([ADR 0022](docs/adr/0022-gpu-tier.md)). Registration fails quietly on a
  machine with no device and gg falls back to the CPU, so a chart still renders.
  Precision at deep zoom turned out to be a separate matter and to belong
  elsewhere: device coordinates are pixels within a canvas and never large, and
  what runs out of digits is the float64 a timestamp becomes — so the rebasing
  is `scale.Origin`, which measures a time domain from an instant near the data.

Default per geom and dataset size; user-overridable through `geom.Decimate`,
`geom.Budget` and `geom.NoDecimation`. The reduction happens in `Build`, never
in `Train`, so an axis always reports the data rather than the subset that
survived.

---

## 13. API sketch

**GoG-lite with ergonomic constructors and functional options.**

```go
package main

import (
    "github.com/timzifer/figure"
    "github.com/timzifer/figure/geom"
    "github.com/timzifer/figure/scale"
    "github.com/timzifer/figure/palette"
    "github.com/timzifer/figure/theme"

    ggbackend "github.com/timzifer/figure/backend/gg" // raster; PDF/GPU/browser later
)

func main() {
    // A []time.Time column and a []float64 column, both borrowed, not copied.
    src := figure.NewTable().Time("t", times).Float64("y", values)

    p := figure.New(
        figure.Theme(theme.Dark),
        figure.Size(800, 500),
        figure.Title("Signal"),
    )
    p.X(scale.Time())
    p.Y(scale.Linear(scale.Nice()))
    p.Add(
        geom.Line(src, geom.X("t"), geom.Y("y"),
            geom.Color(palette.Blue),
            geom.Tension(0.4),
            geom.OnMissing(geom.Gap),
        ),
    )

    // Lean, zero-dependency SVG:
    _ = p.Render(figure.SVG("signal.svg"))

    // Or raster via the gg adapter (PDF and GPU are later milestones):
    _ = p.Render(ggbackend.PNG("signal.png"))
}
```

Serialization and the web workflow. The document is
Vega-Lite-*shaped* rather than a Vega-Lite subset: it borrows the vocabulary
where the concept exists in both and names figure's own things plainly, and
what it guarantees is the round trip through figure
([ADR 0014](docs/adr/0014-json-spec.md)):

```go
doc, _ := json.Marshal(p)           // a Plot is a json.Marshaler
// ship `doc` to a Go-Wasm frontend, or to a server, which renders it identically
q, _ := figure.ParseJSON(doc)
```

Interactivity, for the browser, through the built-in
canvas backend rather than through gg, which has no js target at the pinned
version ([ADR 0017](docs/adr/0017-browser-backend.md)). A native window followed
through `backend/window`, and is one call:

```go
show.Plot(p, window.Title("Signal"))   // backend/window/show
```

```go
p.On(figure.Hover, func(ev figure.Event) { /* ev.Point, ev.Series(), ev.Hit */ })
p.On(figure.Zoom,  func(ev figure.Event) { /* ev.Rect, ev.Factor */ })

live, _ := p.Live(canvas.Element(el))   // a surface, redrawn
defer live.Close()
live.Draw()
defer live.Bind(el)()                   // pointer, wheel and drag
```

The kinds share one `Event` struct rather than the per-kind types this section
first sketched: Go has no sum types, and a handler signature per kind means
`On` takes an `any` and the caller writes a type assertion
([ADR 0015](docs/adr/0015-hit-testing.md)).

---

## 14. What is built, and what is next

Rendering is delegated to gg, so effort concentrates on the model, the data
layer and interaction.

**What exists.** Linear, time, log, symlog and ordinal scales; the marks,
coordinate systems, colour and size channels, facets and guides listed in
[docs/features.md](docs/features.md); SVG, PDF, PNG, JPEG, a browser canvas, a
native window and an opt-in GPU tier behind one `ir.Backend`; hit-testing,
overlays, clickable guides, linked views, keyed transitions and streaming;
typeset notation in labels; a described chart with a data table for a reader
that is not an eye; and a JSON dialect a chart writes itself down as and reads
itself back from.

The record of how each of those arrived, and the argument that shaped it, is in
[docs/milestones.md](docs/milestones.md).

### What is next

- Harden the GPU tier as GoGPU matures.
- **More coordinate systems.** A **barycentric coord** is the fourth, on the
  same seam as Cartesian, polar and Smith and cheaper than any of them: the map
  is affine, so an edge stays a straight line and only the clip changes
  ([ADR 0051](docs/adr/0051-barycentric-coord.md)). A geographic projection is
  the wider one — it transforms every point with no linear interval underneath
  it, and its graticule has no tick behind it — and is argued on its own
  evidence rather than smuggled in beside the affine case.
- **A tidy tree.** Reingold–Tilford in Buchheim's linear-time form is O(n),
  deterministic, bounded and a pure function of its input, which is
  `stat.Squarify`'s shape exactly — so a dendrogram, a phylogram, an org chart
  and, under a polar coord, a radial dendrogram are one mark reading the
  channels the relational family already defines
  ([ADR 0053](docs/adr/0053-tidy-tree-layout.md)). The clustered heatmap falls
  out of it.
- **Node-link diagrams, Venn and UpSet** are what is left of the relational
  family ([ADR 0039](docs/adr/0039-relational-layouts.md)). A force layout's
  whole method is to run until it settles, so it cannot be a pure function of
  its input at a bounded sweep count that also looks good, and
  [ADR 0012](docs/adr/0012-parallel-panels.md) has to be answered on its own
  terms before it lands. Venn is a circle-packing optimiser and UpSet is a
  matrix chart rather than a relational layout at all.
- **More stats: contour**, and a bucket of **domain reductions** — the
  Kaplan–Meier estimator, the SPC control-limit family, ACF and PACF, ROC and
  Lorenz — none of which needs a shape figure does not already draw. The record
  is mostly about where the line is, because "put the field's arithmetic in
  `stat`" has no natural end: a reduction belongs there when its output is the
  chart's geometry and there is no reading of it that is not the chart
  ([ADR 0054](docs/adr/0054-statistical-instruments.md)).
- **A locus** — a family of curves given by a formula rather than by data, of
  which `geom.HLine` is the degenerate member. It is what a Nichols diagram's
  closed-loop contours are, and what the VSWR circles, constant-Q arcs and ZY
  overlay [ADR 0033](docs/adr/0033-smith-charts.md) declined are: not
  furniture, but annotations defined in data space, so the coordinate stage
  draws them and `render` keeps its two tick lists
  ([ADR 0050](docs/adr/0050-locus-annotations.md)).
- **A probability scale**, beside `Log` and `SymLog`: an axis warped by Φ⁻¹, the
  logit, the complementary log-log or the Gumbel link, so that a distribution's
  cumulative function plots straight. It is `geom.QQ` turned round — that mark
  warps the sample and leaves the axis linear; this warps the axis and leaves
  the sample alone — and both are worth having because a mark does one job and a
  scale composes with every mark there is. Weibull, normal and extreme-value
  probability paper cost no new mark at all: each is `geom.ECDF` on a warped
  axis ([ADR 0052](docs/adr/0052-probability-scales.md)).
- **Automatic label avoidance** for every kind of mark. The opt-in, panel-local
  form exists ([ADR 0040](docs/adr/0040-label-collision-avoidance.md)).
- **A keyframe timeline**, over more than two states. A sequence is a list of
  two-state transitions and building one is a host-side loop; an API that owned
  the sequence would be a real addition rather than sugar, and would be argued
  on the evidence of people writing that loop.
- **3D** — deliberately late and tightly scoped, split into four records: depth
  as decoration, a third axis, orbiting a scene, and what the dimension is for
  ([ADR 0055](docs/adr/0055-depth-without-a-third-axis.md) to
  [ADR 0058](docs/adr/0058-what-3d-is-for.md)).
- A community plugin ecosystem.

---

## 15. Versioning & stability

- **Pre-1.0 (`v0.x`):** breaking changes permitted in any release. The model, IR,
  and `Backend` interface need room to be gotten right before they freeze. GoGPU
  dependencies are pinned to exact versions per release.
- **1.0 onward:** standard Go semver. Public API, IR, `Backend` interface, and the
  JSON spec become stability surfaces. New geoms/backends arrive through the
  extension interface, not by changing existing signatures.
- **A major version is a tool, not a failure.** Within a major version the
  growth rule below holds absolutely. Between them, a seam whose *shape* is
  wrong is corrected rather than papered over with a parallel path — and while
  figure has few enough users that a migration is a compiler pass, that
  correction is preferred to carrying the mistake. The current `v0.x` series is
  that correction being taken cheaply rather than expensively: it widens
  `Geom.Train`, `Observer.Panel` and `Coord.Frame` from positional scale
  arguments to growable parameter structs, so that adding a dimension is
  additive from then on
  ([ADR 0056](docs/adr/0056-three-dimensional-charts.md),
  [ADR 0060](docs/adr/0060-parameter-structs-at-every-seam.md)), and the freeze
  returns at `v1.0.0`.
- **The growth rule.** An interface a third party implements — `data.Source`,
  `scale.Scale`, `coord.Coord`, `geom.Geom`, `ir.Backend`, `ir.Target`,
  `render.Observer`, `mathtext.Typesetter` and the colour and size scales —
  never gains a method, **and a method on one never takes more than one
  parameter beyond its output destination**, so that what it is told can grow
  without the interface changing
  ([ADR 0060](docs/adr/0060-parameter-structs-at-every-seam.md)). A capability
  an implementation may legitimately lack is an optional interface beside it,
  asked for with a type assertion, the way `Resizer`, `Definite` and `Legender`
  already are; a fact every correct implementation needs is a field or a
  method, which is why `render.Observer.End` is a method and the deleted
  `EndData` was not the answer. `ir.Backend`'s drawing calls are the exception
  the rule names: their parameters are the ink, not a description that grows. A struct with exported fields gains fields and never loses one,
  and its zero value keeps its meaning. A string-typed name (`geom.Mark`,
  `scale.Kind`, `coord.Type`) is open to third-party values; an iota enum grows
  at the end.
- **The JSON dialect.** Within v1 a field is only ever added; a reader ignores a
  field it does not know; `$schema` moves only with the module's major version.
- **Nested modules.** `backend/gg` and `backend/window` tag with the
  core: they are the supported raster and native paths, and their own APIs are
  small. `backend/gg/gpu` stays `v0.x` for as long as the GPU tier is opt-in
  beta — the tag says what the README says. `arrow/v18` tracks its upstream:
  its major version follows `apache/arrow-go`'s, which is the reason it is a
  module of its own, and Go's rule that a major version above 1 is spelled in
  the import path is why the module is `…/arrow/v18` in a directory of that
  name and tags as `arrow/v18.x.y`
  ([ADR 0030](docs/adr/0030-arrow-major-version.md)).
- **License:** permissive (e.g. MIT), and the core links only permissive deps
  (gg is MIT). This is a requirement, not a preference: figure must be embeddable
  by downstream frameworks under any license — including dual-licensed ones such
  as `lux` — so no copyleft or noncommercial-licensed code enters the dependency
  graph.

---

## 16. Repository layout

Dependency boundaries enforce the "lean by default" promise: the core depends on
nothing at all, and GoGPU enters only through the nested `backend/gg` module.

Every package below exists; the milestone each arrived in is marked. `coord/`
is the pluggable stage the Cartesian mapping was once hard-coded into, settled
by [ADR 0018](docs/adr/0018-coordinate-systems.md).

```
figure/                     # core module — pure Go, STDLIB ONLY (no requires)
  figure.go                 # top-level API: New, X, Y, Add, Render
  input.go                   # the portable pointer state machine
  describe.go                # Plot.Describe, Plot.DataTable
  data/                      # Source, Column, Float64Columns, Table
  scale/                     # linear, time (+ log, symlog, ordinal, colour)
                             # + Origin, for a time domain that keeps its
                             # nanoseconds at any zoom
                             # + Qualitative, the discrete colour scale
                             # + Size, the area-mapped size channel
  geom/                      # line, scatter, bar (+ area, step, boxplot)
                             # + rect, groups and the position adjustments
                             # + histogram, violin, ridgeline, hexbin,
                             #   beeswarm, ecdf, trend, and SizeBy
  stat/                      # LTTB, min/max, density binning
                             # + StackOffsets, the streamgraph baselines
                             # + Bin, KDE, ECDF, Loess, Hex
  coord/                     # cartesian + polar (the pluggable stage)
  internal/layout/           # panel-grid constraint solver; one importer,
                             # so not public — ADR 0029
  render/                    # model -> IR lowering, + Observer
  facet/                     # faceting (wrap/grid)
  theme/  palette/           # tokens, colourblind-safe palettes
                             # + Scaled and Redundant
  ir/                        # IR primitives + Backend interface
                             # + Damage and Partial
                             # + Semantics and Resizer
  interact/                  # hit index and event vocabulary
  spec/                      # JSON (Vega-Lite-shaped) marshal/unmarshal
  mathtext/                  # pluggable notation, and a TeX subset
  a11y/                      # description and data-table fallback
  backend/svg/               # built-in, zero-dependency SVG emitter
  backend/pdf/               # built-in, zero-dependency PDF emitter
  backend/canvas/            # built-in canvas 2D, js/wasm only
  internal/fontmetrics/      # stdlib hmtx/cmap reader + Helvetica table

  backend/gg/                # NESTED MODULE: the raster backend, and the
                             # in-memory Surface a window draws into.
                             # Depends on gogpu/gg. Zero CGO. PDF did not
                             # land here — see ADR 0009 — and neither did
                             # the browser — see ADR 0017.
    cmd/gallery/             # renders every documented figure
    gpu/                     # NESTED MODULE: the opt-in GPU tier.
                             # Importing it is the opt-in — ADR 0022.

  backend/window/            # NESTED MODULE: a native window.
                             # Depends on gogpu/gogpu and on backend/gg.
                             # Zero CGO. See ADR 0021.
    show/                    # one call: show.Plot(p)
    cmd/demo/                # a signal to pan and zoom by hand

  arrow/v18/                 # NESTED MODULE: Apache Arrow adapter
                             # Depends on apache/arrow-go. Zero CGO. The
                             # directory is its major version — ADR 0030.
```

`backend/gg`, `backend/gg/gpu`, `backend/window` and `arrow/v18` depend on the core,
never the reverse. A nested module is excluded from its parent's module graph, so
importing only `figure` yields a graph with no external packages in it at all —
CI asserts this on every commit, and the same mechanism one level down is what
keeps `backend/gg` free of a GPU stack while `backend/gg/gpu` offers one. SVG
output works with no GoGPU present. See
[ADR 0001](docs/adr/0001-module-layout.md) for why the gg backend is a nested
module here rather than the separate `figure-gg` repository this section
originally proposed.

---

## 17. The decisions this design rests on

Nothing here is open. Each line is a rule the code depends on, with the record
that argued it.

1. **The gg coupling surface.** gg is pinned to an exact version, and the
   adapter imports only `gg` and `gg/text` — never `gg/gpu`, `gg/scene` or
   `gg/recording`. The whole adapter is about 300 lines
   ([ADR 0006](docs/adr/0006-gg-coupling-surface.md)).
2. **SVG and PDF are built-in emitters**, and the only vector paths. `gg-pdf`
   cannot draw geometry, so there is no gg vector path to unify with
   ([ADR 0004](docs/adr/0004-svg-source-of-truth.md),
   [ADR 0009](docs/adr/0009-pdf-backend.md)).
3. **The JSON dialect is Vega-Lite's vocabulary with figure's semantics** —
   neither a strict subset nor "inspired by". The names and the structure are
   Vega-Lite's wherever the concept exists in both; what figure has and
   Vega-Lite does not is named plainly rather than smuggled through a borrowed
   name; and `$schema` says which dialect the document is. What is guaranteed
   is the round trip through figure, tested per mark and per scale
   ([ADR 0014](docs/adr/0014-json-spec.md)).
4. **The backend shapes text, figure places it**, and `Measure` is the entire
   seam between them. The core carries a stdlib metrics reader and a fallback
   advance table, never a shaper
   ([ADR 0003](docs/adr/0003-text-and-fonts.md)).
5. **Module layout.** `github.com/timzifer/figure`, with the gg backend at
   `github.com/timzifer/figure/backend/gg`
   ([ADR 0001](docs/adr/0001-module-layout.md)).
6. **Go 1.25 is the minimum**, forced by gg's `iter.Seq`-returning `Face`. The
   ABI sensitivity lives in `goffi`, reachable only through `gg/gpu`, which is
   a module a caller opts into rather than something on the supported path
   ([ADR 0005](docs/adr/0005-go-version.md)).
7. **The extension API.** A third-party geom reads the shared options through
   `geom.Configure`, takes its own through `geom.Extra`, and is read back from
   a document through `geom.Register`; `scale` and `coord` register the same
   way; the JSON spec carries a registered mark's own properties on the mark
   object ([ADR 0029](docs/adr/0029-extension-model.md)). The test to apply to
   anything queued behind it: does the change alter an interface a third party
   implements, or ride beside it? A coordinate stage alters one — a `Frame`
   that is a rectangle with an X scale and a Y scale is a library that is
   Cartesian forever ([ADR 0018](docs/adr/0018-coordinate-systems.md)). A
   multi-entry legend does not, because an optional interface widens nothing
   ([ADR 0020](docs/adr/0020-discrete-colour-and-multi-entry-legends.md)).

---

## 18. References (ecosystem context)

- Rendering substrate: `gogpu/gg` (2D graphics, pure Go, zero CGO),
  `gogpu/wgpu` (pure-Go WebGPU: Vulkan/Metal/DX12/GLES/software/browser),
  `gogpu/gogpu` (windowing/input), `gg-svg`, `gg-pdf`, `gogpu/naga` (WGSL compiler),
  `go-webgpu/goffi` (pure-Go FFI). gg's GPU tiers port techniques from Vello and
  tiny-skia; its CPU AA ports Skia's analytic rasterizer.
- Existing Go plotting: `gonum/plot`, `go-echarts`, `go-chart`.
- Declarative reference: Vega-Lite, Plotly.
- Algorithms: LTTB (decimation), extended Wilkinson (ticks), datashader
  (density binning).
- Fallback text stacks (only if a gg gap appears): `go-text/typesetting`,
  `boxesandglue/textshape`.

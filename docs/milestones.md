# How figure was built

Every capability in the library, in the order it arrived, with the argument
that shaped it. It is kept because the arguments are the part that is still
load-bearing — the version numbers beside them are not: they belong to a module
path this library no longer publishes under
([ADR 0059](adr/0059-renaming-and-restarting-the-version.md)), and the headings
are left as written rather than renumbered into a fiction.

For what the library does *now*, read [features.md](features.md) and
[../README.md](../README.md). For why a given decision went the way it did,
read [adr](adr). The identifier-by-identifier audit taken before the API froze
is [v1-api-audit.md](v1-api-audit.md).

Because rendering is delegated to gg, effort concentrates on the model, data, and
interaction. Each milestone has a Definition of Done. Pre-1.0, breaking changes
between milestones are expected.

### v0.1 — "It plots" (static) — **shipped**

- IR + `Backend` interface defined and frozen for the cycle
  ([ADR 0002](adr/0002-ir-and-backend.md)).
- Two backends: built-in `svg` (zero deps) and `gg` raster (text/AA via gg).
- Scales: linear + time; extended-Wilkinson ticks; dedicated time-axis ticks.
- Geoms: line, scatter, bar; axes, title, basic legend.
- Golden-image tests: golden SVG in the core and golden PNG in the gg backend,
  both compared with a tolerance below anything visible — bit-identical float
  results are not available across architectures — plus a cross-backend parity
  test.
- **API model decided and documented** (GoG-lite, per [§13](../CONCEPT.md#13-api-sketch)).
- *DoD:* static SVG + PNG, `CGO_ENABLED=0`, gg pinned. ✔

Beyond the stated DoD, v0.1 also carries: tick-label collision avoidance, a
missing-data policy per geom (gap / interpolate / error), light and dark themes
with a colourblind-safe palette, and a generated figure gallery that CI
re-renders and checks against the committed images.

### v0.2 — Data layer & scales — **shipped**

- Columnar/batch `Source`; zero-copy for `[]float64`. Categories close the
  interface sketched in [§7](../CONCEPT.md#7-data-layer), so a table can carry them
  alongside numbers and times.
- Log, symlog, ordinal/categorical scales. Log and symlog emit minor ticks; the
  ordinal scale is a band scale, so bars and boxplots take their width from it
  ([ADR 0008](adr/0008-categorical-axes.md)).
- `NaN`/`Inf` policies (interpolate / gap / error), extended to cover values a
  scale cannot place — zero on a log axis is the same failure as a `NaN`, and
  one policy answers both.
- Geoms: area (with `Y2` for bands), step (pre/mid/post), boxplot (Tukey
  whiskers, type-7 quartiles, outliers).
- Color scales mapped onto gg's linear-space pipeline — ramps interpolate in
  linear light, not in sRGB bytes — with colorblind-safe sequential (Viridis,
  Cividis, Magma) and diverging (blue/orange, purple/green) palettes.
- *DoD:* a chart can be built over categories, over orders of magnitude, and
  over signed data that crosses zero, and a mark's colour can come from the
  data. ✔

Colourbars are **not** in v0.2: a layer coloured from a continuous scale
contributes no legend entry rather than a swatch that would misrepresent it.
Guides are v0.3.

### v0.3 — Layout, theming, PDF — **shipped**

- Constraint layout for subplots with axis alignment; faceting (wrap/grid).
  `layout.Panels` solves one grid: per-column left gutters and per-row bottom
  gutters, with every panel the same size, so a position means the same thing
  in every panel. `Compute` is that solver over a one-by-one grid, which is
  what keeps a lone chart and a facet of one identical.
- Theme engine: `theme.Tokens` holds the dozen choices a theme actually makes
  and `theme.Build` derives the other forty, `Theme.With` edits a built theme,
  and a registry resolves one by name for a config file or a flag.
- Annotations (`HLine`, `VLine`, `HBand`, `VBand`, `Segment`, `Region`,
  `Note`), colourbars, and a guide column that stacks them beside the plot.
- PDF output. **Not** through gg recording and `gg-pdf`: that library cannot
  draw geometry — its path operations reach a stub in `gxpdf` that has been a
  `TODO` since v0.4.0 and still is at v0.9.4 — so figure emits PDF itself,
  from the core module, with no dependency at all
  ([ADR 0009](adr/0009-pdf-backend.md)).
- *DoD:* a chart can be split into small multiples with aligned axes, annotated
  with things that are not data, given a guide for a continuous colour scale,
  restyled by editing tokens rather than fifty fields, and written as a vector
  PDF. ✔

Colourbars close the gap v0.2 left open: a layer using `geom.ColorBy` now
contributes a colour guide instead of nothing.

### v0.4 — Big data (CPU tier) — **shipped**

- Decimation family in `stat/`: LTTB, min/max-per-pixel-column, density binning
  → raster. Geoms apply it at draw time, on device coordinates, and choose one
  by mark and by size unless told otherwise
  ([ADR 0011](adr/0011-decimation.md)). `Train` still sees every row, so a
  reduced chart's axes are the data's, not the subset's.
- Allocation pass; a benchmark gate on per-frame allocations, running in CI over
  all three modules. Everything sized by the data comes from a pool, and both
  `TestARenderDoesNotAllocatePerPoint` and `.github/scripts/allocgate.awk`
  assert that a frame over a million rows allocates what a frame over a thousand
  does — 76 either way when the pass landed, and the same count either way
  since; the current numbers are in [docs/benchmarks.md](benchmarks.md).
  Along the way, feeding a column into a scale row by row through a variadic
  interface method turned out to cost one allocation per row; it is now one
  call per column.
- Arrow adapter as a separate module, `figure/arrow` — since the v1 audit
  `figure/arrow/v18`, because its major version is Arrow's
  ([ADR 0030](adr/0030-arrow-major-version.md)). A null-free `float64`
  column is Arrow's own buffer; everything else converts once and caches; an
  Arrow null becomes `NaN`, so the missing-data policy that was already there
  covers it ([ADR 0013](adr/0013-arrow-adapter.md)).
- Parallel subplot rendering. Each panel records into an `ir.Recorder` on its
  own goroutine and the recordings replay **in panel order**, so the output is
  byte-identical to a serial render and one set of golden files covers both
  ([ADR 0012](adr/0012-parallel-panels.md)). `scale.Snapshotter` is what
  removes the sharing that panels sharing an axis otherwise have.
- *DoD:* a chart over a million rows renders, with no option set and no spike
  lost, in around 60 ms into under 30 kB of SVG — where drawing every row takes
  six times as long and produces 15 MB — and the same chart redrawn every frame
  allocates nothing that grows with its data. ✔

Not in v0.4, in case they look like oversights. Streaming was v0.5: there was no
`StreamSource` and no snapshot/swap, because the interesting half of that is
damage-aware repaint, which needs the interactive backends. `stat` carries the
decimation family only — smoothing, regression and hexbin are stats rather than
big-data machinery, and they belong with the geoms that would draw them. And
the GPU tier is untouched, which is the point of the CPU tier: big-data
*stills* are complete without it.

### v0.5 — Web & interactivity — **shipped**

- Browser rendering under `GOOS=js`. **Not** via the gg backend: gg has no
  `syscall/js` anywhere in it at the pinned version, so there is no WebGPU path
  to take and no canvas fallback to fall back to. figure draws on the canvas
  2D context itself, from `backend/canvas` in the core module, because
  `syscall/js` is the standard library and costs the core nothing
  ([ADR 0017](adr/0017-browser-backend.md)). Paths go to `Path2D` as one
  string per drawing call: crossing the WebAssembly boundary is what costs in a
  browser.
- Event system: hover, click, zoom and pan, with hit-testing over the marks a
  render actually emitted. `interact.Index` watches a render — `render.Chart`
  gained an `Observer` that says which panel and which layer is drawing — so
  hit-testing is correct for every geom including ones that do not exist yet,
  and the IR gained no identity channel
  ([ADR 0015](adr/0015-hit-testing.md)). `Plot.On` registers handlers,
  `Plot.Live` draws into a surface, and `Live.Bind` wires a DOM element to it.
- Full-spec JSON serialization, in `spec/`. Vega-Lite-shaped rather than a
  Vega-Lite subset, and §17.3 is settled with the reasoning
  ([ADR 0014](adr/0014-json-spec.md)). A `Plot` is a `json.Marshaler`; the
  round trip through figure is guaranteed and tested per mark and per scale.
  It rests on three new description APIs — `geom.Desc`, `scale.Desc` and
  `facet.Desc` — so that the format lives outside the model packages.
- Streaming with snapshot/double-buffer, and damage-aware updates computed in
  the IR rather than in a backend. `data.Stream` is appended to from any
  goroutine and frozen between frames; `ir.Damage` diffs two recordings;
  `ir.Partial` is what a repaintable backend implements
  ([ADR 0016](adr/0016-streaming-and-damage.md)).
- *DoD:* a chart can be written down as JSON and read back as the same chart, a
  pointer over it reports the row underneath, the wheel zooms about that point
  and a drag pans, a producer can append to it on another goroutine while it is
  drawn, a redraw repaints only what changed and skips a frame that changed
  nothing — and all of that runs in a browser from the same model that renders
  SVG on a server. ✔

- Row identity, opt-in. A hit always reports the data values under the pointer;
  with `Live.TrackRows(true)` it also reports the source row behind the mark,
  which is what highlighting the matching row of a table beside the chart
  needs. Geoms report where each of their rows landed through `geom.Rows`,
  separately from what they drew — a smoothed line is a curve through its rows
  and a bar is four corners around one, so the drawn points are not the rows.
  Decimation is not in the way: LTTB and MinMax keep real rows. Faceting is not
  either: rows are resolved back through `data.Subset` to the table that was
  handed in.

Not in v0.5, in case they look like oversights. There is no native window and
no GPU tier: both landed in v0.6, and both needed `gogpu/gogpu` and `gg/gpu`
rather than anything in this milestone. Row identity is off by default and not
every mark has one — a boxplot's box aggregates many rows, a density raster is
not a mark,
an interpolated point was never measured — and those report no row rather than
a nearby one. And the damage unit is a drawing call, so moving one point of a
line repaints the line's box; what it does not repaint is the title, the axes
and the margins, which is most of the canvas.

### v0.6 — Native interactive & polish — **shipped**

- Native interactive window via `gogpu/gogpu`, in a new nested module,
  `backend/window`. The event system it needed was already here, and so was the
  rasterizer: the window draws with the same CPU backend that writes a PNG, into
  a new in-memory `gg.Surface`, and presents that as one texture per changed
  frame. There is one implementation of every mark, so a window shows exactly
  what a file would ([ADR 0021](adr/0021-native-window.md)). The steering
  moved into the core as `figure.Input` — a portable state machine that turns
  presses, moves and wheel notches into hovers, clicks, pans and zooms — and
  `Live.Bind` was rewritten onto it, so the browser and the window share one
  implementation rather than two that drift.
- GPU tier enabled, opt-in, as a nested module of its own:
  importing `backend/gg/gpu` registers gg's accelerator, and importing nothing
  keeps `backend/gg`'s dependency graph exactly as small as it was
  ([ADR 0022](adr/0022-gpu-tier.md)). Origin rebasing turned out not to
  belong here: figure's device coordinates are pixels within a canvas and are
  never large, and the precision a deep zoom loses is in the float64 a timestamp
  becomes. So it landed in `scale.Origin`, which measures a time domain from an
  instant near the data — and keeps a nanosecond a nanosecond, where absolute
  Unix nanoseconds in this century need 61 bits of a 53-bit mantissa.
- Math typesetting for labels, optional and pluggable. `mathtext.Typesetter` is
  the seam and `mathtext.TeX` is the subset that ships — scripts, `\frac`,
  `\sqrt`, `\mathrm`, the spacing commands and the symbols a chart label
  reaches for. A typesetter is installed by wrapping the backend, so notation
  works in every label a chart has, including the ones layout measures and the
  ones a geom that does not exist yet will draw
  ([ADR 0023](adr/0023-math-typesetting.md)).
- Responsiveness: `figure.Responsive` scales a theme by how much smaller or
  larger the drawing is than the design it was made at, `Live.Resize` is how a
  surface says its size changed, and `ir.Resizer` is what tells a backend
  ([ADR 0025](adr/0025-responsive-charts.md)). Device scale was already
  handled and is a different thing.
- Accessibility, in three channels
  ([ADR 0024](adr/0024-accessibility.md)): a chart's title becomes an SVG
  `<title>` with `role="img"`, a PDF document title and a canvas `aria-label`,
  through a new optional backend interface, `ir.Semantics`; `Plot.Describe`
  reads the data and writes the `<desc>` a screen reader announces after it;
  `Plot.DataTable` writes the rows as an HTML table, which is the fallback a
  picture cannot be; and `theme.Redundant` gives every layer a dash pattern and
  a marker shape of its own, so that colour is not the only channel.
- *DoD:* a chart opens in a native window on the desktop and pans, zooms and
  resizes there; the same chart renders with the GPU tier switched on by one
  import and without it; a label can be written as notation and is measured as
  it is drawn; a chart drawn at a third of its size is the chart rather than a
  photograph of it; and a reader who cannot see it gets a name, a description
  and the numbers. ✔

Not in v0.6, in case they look like oversights. The GPU tier is **opt-in beta**
and stays that way past v1.0: for server-side stills the CPU rasterizer and the
vector emitters are the supported path. The window cannot be opened by CI, so
what is tested is everything that is not the window and the window itself is
only compiled — the same hole `backend/canvas` has about a browser. The built-in
typesetter is a small subset on purpose: no matrices, no growing delimiters, no
document-class macros, and a label needing those wants an engine plugged into
the interface. Accessibility stops at the document: there is no per-mark
`<title>`, no tab order through the data, and no reduced-motion or contrast
handling — a chart with ten thousand points has no useful reading as ten
thousand elements, and the table is the better answer to the same question.

### v0.7 — Marks, groups and adjustments — **shipped**

The plumbing half the catalogue was waiting on. Nothing here is a coordinate
system and nothing here is a stat; it is the four pieces that most missing
chart types share. See [docs/chart-types.md](chart-types.md) for the full
list and what each form costs.

- A data-driven rectangle mark. `Bar` grows from a baseline and `Region` takes
  four literals, so no mark occupied an arbitrary box per row. `geom.Rect`, a
  `geom.X2` beside the existing `geom.Y2`, and a per-row baseline through `Y2`
  turn heatmap, gantt, candlestick, waterfall, bullet, waffle and calendar into
  recipes rather than into seven geoms. An edge the row does not name is the
  slot the axis implies — a band's own width, or the closest spacing in the
  data — so a heatmap over two categorical axes is two columns and a colour.
- Groups within one layer, and a discrete colour scale to paint them.
  `geom.GroupBy` splits a long table into series; `scale.Qualitative` is the
  categorical colour scale, shaped like `scale.Categorical` and riding the
  `ColorScale` interface the same way, so one `geom.ColorBy` binds either kind
  and the guide follows from the scale rather than from a second option
  ([ADR 0020](adr/0020-discrete-colour-and-multi-entry-legends.md)).
- A layer may contribute many legend entries, through `geom.Legender` — an
  optional interface rather than a wider `Geom`. §17.7 freezes that interface
  at v1.0, and a pie with N slices in one layer must not be the reason it
  freezes badly.
- Position adjustments: stack, dodge, fill, and the silhouette and wiggle
  offsets a streamgraph is made of. Derived in `Train`, because the axis has to
  describe the totals; drawn in `Build`, because that is where geometry lives
  ([ADR 0019](adr/0019-position-adjustments.md)). The Byron–Wattenberg
  offset itself is `stat.StackOffsets` — numbers in, numbers out.
- *DoD:* one layer over a long table draws N coloured series and the legend
  names all N; a stacked bar's axis reaches the stacked total and each segment
  is separately hittable and separately attributable to its row; a heatmap and
  a gantt chart render from the public API; every new mark and option survives
  the JSON round trip, including the `("rect", "")` collision between a
  data-driven rect and the region annotation; the allocation gates are
  unchanged. ✔

Not in v0.7, in case they look like oversights. A grouped **bar and area stack
by default** and everything else does not: two lines drawn over one another are
two readings, and adding them would invent a third nobody measured. Stacking
accumulates in group order, so a stack of mixed signs runs each segment from
where the last one ended rather than splitting into a positive and a negative
half — the pictures differ, and the sequential one is the one a running total
is. `geom.WidthBy` gives a bar its width from a column, in the axis's own
units; it does not *reposition* the slots, so a marimekko is one layer whose X
column already holds each column's centre. That is a deliberate line: unequal
slots that label themselves are an axis question rather than an adjustment
one. And a group index is per layer, so a facet whose panels hold different
groups wants an explicit `scale.Qualitative` — a shared scale is what makes one
colour mean one thing across panels, and the palette index cannot be.

### v0.8 — Coordinates — **shipped**

- `coord/`, at last: the stage [§8](../CONCEPT.md#8-model-layer-gog-lite) has promised since
  v0.1. A scale still maps into an interval; a coord decides what the interval
  means. `Cartesian` is the identity and the default, so every existing geom
  draws what it always drew and the golden files are the proof
  ([ADR 0018](adr/0018-coordinate-systems.md)).
- `Polar`, and with it pie, donut, radar/spider, rose/coxcomb, wind rose and
  gauge. A pie is a stacked bar with θ from the Y axis; a radar is a line over
  an ordinal angular axis. Neither is a new geom, which is the point of having
  built v0.7 first.
- Arcs are cubics. The IR gains nothing: ADR 0002 froze the primitive set on the
  claim that every curve a chart needs is expressible as cubics, and a
  coordinate system is the first serious test of it.
- Polar furniture — concentric rings instead of horizontal grid lines, tick
  labels around the ring instead of along an edge. The coord reports the
  geometry; `render` still strokes it, because `render` is the only package that
  knows drawing order.
- *DoD:* the same `geom.Bar` layer draws a bar chart in `coord.Cartesian` and a
  pie in `coord.Polar`; **every existing golden file is unchanged**; a pointer
  over a slice names its category and its row rather than a pixel; a donut's
  hole is an explicit annulus; the spec round-trips the coord; the allocation
  gates are unchanged and the per-point call has a batch form so that they stay
  that way. ✔

Two things the interface grew beyond what
[ADR 0018](adr/0018-coordinate-systems.md) sketched, both for reasons the
sketch could not have known. `Frame` **returns** the coord positioned in a panel
rather than moving the receiver into it, because panels are built concurrently
and a coord that remembered which panel it was in would be a data race — the
same problem `scale.Snapshotter` exists for, answered without a second
interface. And `Furniture` **fills** a struct the caller owns rather than
returning one: a Cartesian panel's furniture is two dozen little slices, and
allocating them per frame would have cost more than the whole rest of the frame
does. The record carries both as an amendment.

Not in v0.8, in case they look like oversights. There is no **geographic
projection**: a projection transforms every point with no linear interval
underneath it, which is a wider seam than this one, and ADR 0018 says it is to
be argued on its own evidence rather than smuggled in as a third `Coord`. A
polar coord **does not decimate** — a bucket of equal angle is not a bucket of
equal width, so a reduction defined over pixel columns would be measuring the
wrong thing, and nothing polar is a big-data chart. `layout` is **untouched**: a
polar coord inscribes itself in whatever rectangle the solver gives it, and
gutter rules that understand a radial axis are a later milestone. A radar's
contour closes because `geom.Closed` says so rather than because the coord
guessed: whether a series wraps is a fact about the series, and a polar time
series spiralling through three revolutions does not. And a **hit on a filled
mark is decided against the outline the drawing call carried**, so a shape whose
edge is a curve can be hit a little way outside its ink at a bulge — the convex
hull of a cubic, which is the same slack a vertex already gets and the opposite
error from missing a slice the pointer is plainly inside.

#### The v0.8 sugar

Three additions, none of them a new mark, all of them things the coordinate
stage made expressible and nothing spelled:

- A slice's **inner and outer radius are columns**. `geom.Bar` reads `X` and
  `X2` as its two edges on the cross axis, exactly as `geom.Rect` has since
  v0.7 — and under a polar coord that axis is the radius. So a donut carries
  three numbers per slice instead of one: how far round it goes, where it
  starts, and where it stops.
- A slice can be **broken out of the ring**. `geom.Explode(f)` moves every mark
  of a layer away from the middle by a fraction of the outer radius and
  `geom.ExplodeBy(col)` reads that per row, which is what pulls one slice out
  and leaves the others in place. It is a displacement along the mark's own
  bisector rather than a longer radius, so the slice still says what it said
  and the gap shows where it came from. The coord answers how far and the geom
  moves the path, because a coord does not draw; `coord.Cartesian` deliberately
  cannot answer, so a Cartesian chart is unchanged by an option every geom now
  accepts ([ADR 0026](adr/0026-breaking-a-mark-out.md)).
- `coord.Pie()` and `coord.Donut(f)` **name the recipe**. They are sugar for
  `Polar(Theta(FromY))` and that plus `Hole(f)`, describe themselves as the
  polar coord they are, and are where the two facts a pie needs beyond the
  coord are written down: that the angular scale must not be niced, and that
  the hole is where the radial scale starts.

### v0.9 — Distributions, density and size — **shipped**

- The stats [§8](../CONCEPT.md#8-model-layer-gog-lite) has promised since v0.1 and `stat/`
  had never carried: a 1-D `Bin`, `KDE` with a bandwidth rule, `Hex`, `ECDF` and
  `Loess`. Each a pure function with an `Append` form, each with a determinism
  test — a reduction that reached for `math/rand` would make a parallel render
  stop being byte-identical to a serial one.
- The charts they carry: `geom.Histogram`, `geom.Violin`, `geom.Hexbin`,
  `geom.Ridgeline`, `geom.Beeswarm`, `geom.ECDF` and `geom.Trend`.
- A size channel, and the bubble chart. `geom.SizeBy` reads a column through
  `scale.Size`, mapped by **area**, not radius, and guided by a third guide kind
  — which was the moment the guide column in `layout` was generalised once
  rather than extended twice ([ADR 0027](adr/0027-size-channel-and-the-guide-column.md)).
- *DoD:* every stat is a pure function of its input under test; violin and
  ridgeline compose with v0.7's groups; a bubble chart's size key sits beside a
  legend and a colourbar without overlap, and doubling a value multiplies the
  diameter by the square root of two, under test. ✔

The line that had to be drawn twice is where a stat runs
([ADR 0028](adr/0028-distribution-stats.md)). ADR 0011 puts decimation in
`Build`, in device space, on the rule that what a chart's axis reports must not
depend on how wide the chart is. A distribution stat is the same rule pointing
the other way: a histogram's Y axis holds counts that appear nowhere in the
table, an ECDF's holds a fraction it computed, and a trend's fit reaches values
no row has — so the summary *is* what the axis has to describe, and it is
computed in `Train`. Two marks are the exception because what they compute is a
length on screen rather than a value: a hexbin bins over the plot rectangle,
because a hexagon laid out in data space comes out stretched by the panel's
aspect ratio, and a beeswarm places its marks against a marker's width.

Not in v0.9, in case they look like oversights. A **hexbin has no colourbar**:
its counts are not known until the plot rectangle is, and the guide column is
measured before that — the same ordering ADR 0011 describes, reached from the
other side. A **histogram ignores `GroupBy`**, because two overlapping
histograms hide each other exactly where the comparison is; `Violin`,
`Ridgeline` and `ECDF` are the three marks that answer that question without
overplotting, and each of them takes the series column. A **sized layer draws
circles rather than markers**, because `ir.Backend.Markers` carries one style per
call and a per-row size would be a call per row — the same refusal
[ADR 0007](adr/0007-per-mark-colour.md) makes about colour, and the same
answer, with better hit-testing as a bonus. And **`stat` still does no sorting**:
`Quantile`, `ECDF` and `Loess` take ordered columns, because sorting means a
buffer and the geom already keeps one.

### v1.0 — Stable & complete enough — **shipped**

- API freeze; semver. The audit that precedes it is
  [docs/v1-api-audit.md](v1-api-audit.md): every exported identifier with
  a verdict, and the changes it asked for are in.
- Stable registration/extension model for third-party geoms and backends —
  **done** ([ADR 0029](adr/0029-extension-model.md)): `geom.Configure`,
  `geom.Extra`, and `Register` in `geom`, `scale` and `coord`, with the JSON
  spec carrying a registered mark's own properties.
- Docs, gallery, public benchmark suite — **done**. The docs are the
  [README](../README.md), the API reference on pkg.go.dev, the ADRs and
  [docs/chart-types.md](chart-types.md). The gallery is
  [docs/images](images), rendered by `backend/gg/cmd/gallery` from every
  milestone's marks and checked against the code on every commit. The
  benchmark suite is [docs/benchmarks.md](benchmarks.md): every benchmark
  in the repository, what it measures, which numbers CI gates, and a results
  table CI publishes on every run.
- CPU rendering is the supported baseline; **GPU tier remains opt-in beta** until
  the GoGPU native backends prove out across hardware.
- Tagged. Under the old name the core went from `v1.0.0` to `v1.7.0`; under
  this one it restarts at `v0.8.0`. `backend/gg` and
  `backend/window` share it, the opt-in GPU tier is `backend/gg/gpu/v0.3.0`
  — it stays at `v0` for as long as it is opt-in beta, whatever the core does
  — and the Arrow adapter is `arrow/v18.0.4`, whose major is Arrow's. The
  milestones before `v1.0.0` were tagged at the same time as it, so every one
  of them names a commit. The order — the core first, then the nested modules'
  `require` lines, then their own tags — is in
  [CONTRIBUTING](../CONTRIBUTING.md#releasing).

  The post-freeze milestones below shipped in `v1.1.0` — tracks and the text
  mark — in `v1.2.0`, the Smith chart, in `v1.3.0`, the six gaps that were not
  chart types, in `v1.4.0`, the relational and hierarchical layouts, in
  `v1.5.0`, label placement and QQ plots, and in `v1.6.0`, colour ramps that
  compress or class their domain. `v1.3.0` and `v1.4.0` tag the core
  alone: no nested module had a release of its own between `v1.2.0` and
  `v1.5.0`, and a nested tag whose `require` line named an older core than the
  one it was tested against is the failure the release order exists to prevent.
  All of them are additive: no interface gained a method, no struct lost a
  field, and every option they add is one an existing mark accepts and
  ignores.

### v0.10 — Tracks: a band at a panel's edge — **shipped**

The first milestone after the freeze, and additive throughout: `Plot.Track`
attaches a band to the bottom or top of the plot area that shares the plot's X
scale *object* and carries a vertical scale of its own, of a different kind if
that is what the data is — an ordinal strip of machine states under a linear
speed trace, on one time axis. Sharing the object rather than the domain is
what makes a zoom one zoom, and it is why hit-testing, the parallel path and
the Fyne widget needed no changes at all.

Its cost is one narrow widening of the layout solver — `layout.Grid.RowHeights`
gives a row its height instead of deriving one — which is the revisit
[ADR 0010](adr/0010-panel-layout.md) asked for by name, answered without
becoming the general size-per-panel solver it warned about. The rows not given
a height are still all the same size. See
[ADR 0031](adr/0031-tracks.md).

All four edges are there. Bottom and top are rows sharing X; left and right are
columns sharing Y, which is what a colour key or a marginal distribution wants.
They are one code path — `layout.extents` sizes fixed rows and fixed columns
with the same function — and bands on two edges leave the corner between them
empty, which the solver already understood because a wrapped facet leaves holes
too.

The same solver change gives stacked plots on one domain — the linked-axes
shape — as `Grid` options rather than a `Link` API, because a `Grid` already
routes its plots through the one solver and already takes their scale
objects.

### Text: a label per row — **shipped**

`geom.Note` placed one literal string at one literal position, so labelling
rows cost a layer per row — and since a plot only ever gains layers, a chart
whose rows change had to be rebuilt, which takes the reader's zoom with it.
`geom.Text` reads its labels from a column, like every other mark reads its
numbers, and needs no rebuild when the rows change.

The label goes where the encoding says. Naming neither `X2` nor `Y2` puts it at
the row's point; naming either puts it in the middle of the box the row spans —
and that box is the one `Rect` would draw for the same options, so one option
list describes the rectangles and labels them. That is what makes it a mark
rather than a recipe: the anchor is the middle of the box's *visible* part, so a
bar half scrolled off the edge keeps its label; the run is measured through
`ir.Backend.Measure` with the font it will be drawn in and dropped, or elided,
when the box is too narrow, because a label that overruns reads as belonging to
the neighbour; and a layer given `ColorBy` takes each label's ink from the fill
that scale gives the row, so a qualitative palette does not leave half its
categories unreadable.

It pairs with the tracks above: a track gives the state strip its lane, and this
gives its bars something to say — which is what a chart locked against panning
needs, because locked means no hover and no hover means no tooltip.

Neighbouring labels are not moved apart. A box too narrow drops its label
already, and a general de-overlap pass is a layout question rather than a
mark's. See [ADR 0032](adr/0032-text-as-a-mark.md).

### Smith charts: a third coordinate system — **shipped**

`coord.Smith` reads a panel's two axes as a complex impedance — r = R/Z₀ and
x = X/Z₀ — and maps the pair through the reflection coefficient
Γ = (z − 1)/(z + 1), which carries the whole right half-plane, every passive
impedance including the infinite ones, into the unit disc. It is the standard
instrument of RF, microwave and antenna work, and no general-purpose plotting
library draws one, because a library whose coordinate stage is hard-coded
Cartesian cannot.

It is the sharpest evidence that §8's pluggable stage was cut in the right
place: the chart's two grid families are the images of the two axes' own grid
lines, so the constant-resistance circles are what the X ticks look like once
the coord has had them and the constant-reactance arcs are the Y ticks — and
`render` was not touched at all. No new mark either: the locus is a `geom.Line`
from v0.1.

Two additions came with it. `scale.TickValues` pins a linear axis's tick
sequence, because the 0.2 / 0.5 / 1 / 2 / 5 of a paper chart is a convention
rather than the answer to a tick search. And `coord.SmithAdmittance` is the Y
chart — one sign, since y = 1/z gives Γ_y = −Γ_z — applied to the picture and
not to the data, so one load lands in one place whichever chart it is read on.

Not drawn: constant-|Γ| circles, constant-Q arcs and a combined ZY overlay.
Each is a third grid family, and a coord may draw one grid line per tick a
scale emits — the same constraint that made the columns an impedance rather
than the reflection coefficient an instrument reports.
See [ADR 0033](adr/0033-smith-charts.md).

### v1.3 — What is not a chart type — **shipped**

Six gaps that [docs/chart-types.md](chart-types.md) could not hold,
because a catalogue sorted by machinery has no line for a mark that needs
neither a coordinate system nor a stat, and no line at all for the three that
are not marks. None of them is a shape; all of them were the difference
between a chart being *drawable* and being *usable*. The catalogue grew a
bucket H for them.

- **A null is a missing value, in every column kind.** A missing number is NaN
  and every policy figure has is written against that; a missing *category*
  read back as `""` became a band of its own on an ordinal axis and a missing
  *instant* as the zero time stretched a three-hour domain across two
  millennia. Absence is `data.Column.Nulls`, carried by the column it belongs
  to, `geom.column` is the one place it is read, and everything downstream is
  the machinery that already handled a NaN
  ([ADR 0034](adr/0034-null-values.md),
  [ADR 0061](adr/0061-columns-are-one-value.md)).
- **A tick label is described rather than computed.** `scale.Desc` carried an
  honest field saying a document had lost the axis's formatter and gave it
  nowhere to put one, so a chart authored as JSON could not set a thousands
  separator, a currency or a decimal place at all. `scale.NumberFormat` and
  `scale.TimeLayout` are the declarative spelling, beside the Go function
  rather than instead of it ([ADR 0035](adr/0035-label-format-and-locale.md)).
- **A chart in a language.** The time ladder rendered through Go's own English
  tables and `strconv` writes a decimal point — which for a German reader is
  not foreign but wrong, because "1.234" reads as one and a bit.
  `figure.Locale` walks the scales the chart description holds, which is what
  reaches a track's own scale and a free facet axis's clone.
- **An interval around a measurement.** Every chart of a mean, a forecast or a
  tolerance carries two numbers per row and figure drew fifteen marks with
  nowhere to put the second. `geom.ErrorBar` takes either spelling a table
  comes in, and which axis it runs along follows from the encoding
  ([ADR 0036](adr/0036-error-bars.md)).
- **A second axis, in both directions.** A `Plot` had one scale per direction
  and a layer no way to name another. `Plot.Y2`/`geom.OnY2` and
  `Plot.X2`/`geom.OnX2` are a scale on the chart and a binding on the layer,
  and each axis reaches the layout, the coord's furniture, hit-testing,
  steering, the description and the document
  ([ADR 0037](adr/0037-secondary-axis.md)). The vertical one is two
  quantities in different units — revenue against margin; the horizontal one is
  most often one reading with two rulers, an oven curve counted in cycles and
  in minutes, which needs no second layer because an axis with nothing drawn on
  it is still an axis. The two are independent: a layer may name both.
- **A PDF in a script WinAnsi cannot hold.** The emitter named the base-14
  Helvetica, so every rune outside Latin-1 became `?` — in the format people
  send to customers. `pdf.WithFont` embeds a face, subset to the glyphs the
  document drew, with a `ToUnicode` map so the text is still selectable
  ([ADR 0038](adr/0038-embedded-fonts.md)). It needed an sfnt parser, and
  it is in `internal/sfnt` because the core module has no dependencies and
  keeps none.
- *DoD:* a null in a text or temporal column is gapped rather than drawn; a
  chart written down as JSON labels its ticks the way it was told to, in the
  language it was told to; a mean and its interval are one mark; revenue and
  margin are one chart with two axes that zoom together and describe
  themselves separately, and so are minutes and cycles along the bottom and the
  top; and a Japanese label reaches a PDF as a glyph rather
  than as a question mark. ✔

Every one of them is additive: no interface gained a method, no struct lost a
field, and a chart that mentions none of them draws exactly what it drew —
which is what every golden file in the repository asserts.

### v1.4 — Bucket E, the last one — **shipped**

The relational and hierarchical layouts: treemap, icicle, sunburst, sankey, arc
diagram, chord diagram. It is the bucket this document called the only family
that shares no machinery with the rest — "its own data shape, its own solver,
its own legend, its own hit-testing" — and half of that sentence turned out to
be wrong, which is the interesting part.

The data shape and the solvers were real, and they are where the work went:
five new channels (`geom.From`, `geom.To`, `geom.ID`, `geom.Parent`,
`geom.Value`) and six pure functions in `stat/` — a depth, a roll-up, a
partition, a squarified packing, a flow layout and a chord layout, each with a
determinism test. The legend and the hit test needed nothing at all. A
multi-entry legend has been what a layer whose series live inside it
contributes since v0.7, and a relational layer's nodes are exactly that; and
`interact` has indexed one mark per subpath of a fill since v0.5, so a layout
that draws one subpath per cell is pointable with no new machinery. The only
line outside `geom`, `stat` and `spec` is `Plot.showLegend` learning that an
edge table is a second kind of series.

**Four marks, six charts.** Every layout fills the unit square — a span across,
a height out — and the coordinate stage decides what that looks like, which is
the v0.8 move made twice. An `Icicle` under `coord.Polar` is a sunburst; an
`Arc` under one, with its rail moved to the rim by `geom.Baseline(1)`, is a
chord diagram. Neither is a mark of its own, for the same reason a pie is not a
second implementation of a bar.

One thing had to move across a line, and it is worth the sentence: a squarified
treemap packs against the *panel* rather than against the unit square, because
what it optimises is an aspect ratio on screen and squarifying a square to
stretch it into a wide panel defeats the algorithm. That makes it the third
instance of [ADR 0028](adr/0028-distribution-stats.md)'s stated exception,
after the hexagonal lattice and the beeswarm's offsets.

- *DoD:* a hierarchy of `(id, parent, value)` draws as a treemap and as a
  sunburst from one layer and two coords; an edge list of `(from, to, value)`
  draws as a sankey and as a chord diagram; every layout is a pure function
  with a determinism test and a fixed sweep count; a frame costs the same over
  a hundred thousand rows as over a thousand; and every existing golden file is
  byte-for-byte what it was. ✔

Still not drawn: node-link and Venn/UpSet. See
[ADR 0039](adr/0039-relational-layouts.md).

### v1.5 — Two diagnostics — **shipped**

Bucket H's de-overlap pass and the QQ plot the ECDF made obvious, both of which
this document had listed as beyond v1.0 and neither of which needed a new
stage. `geom.AvoidOverlap(true)` opts a text layer into panel-local placement:
`render` lends participating layers a placer, measures through the active
backend and solves in `internal/layout`, trying the original anchor and eight
nearby candidates against the panel rectangle and the labels already placed.
It is bounded and deterministic rather than a relaxation, so a parallel or
watched render draws the same chart as a serial one, and a label in a box is
dropped rather than moved into its neighbour's row.
See [ADR 0040](adr/0040-label-collision-avoidance.md).

`geom.QQ` ranks a sample against the standard normal at plotting positions
(i+0.5)/n, with observations in their own units and no fit or reference line
implied. It shares the ECDF's sorting and grouping buffers — both summarise one
sorted series per group — and `stat.QQ` takes any quantile function, so another
distribution is a `Scatter` over its pairs.
See [ADR 0041](adr/0041-qq-plots.md).

Both are additive, both round-trip through the spec, and the allocation gate
covers a full frame of each over a thousand rows and a hundred thousand. ✔

### v1.6 — Colour ramps that compress and class — **shipped**

The colour channel had one shape: a ramp interpolated linearly from one end of
the domain to the other. The positional channel has had `Log` and `SymLog`
since v0.3, and the quantity the colour channel is most often bound to — a bin
count, a hexbin density — is exactly the one a linear reading destroys.

`scale.ColorLog` and `scale.ColorSymLog` give each decade an equal share of the
ramp. The transform is a field beside the kind rather than a value of it,
because a diverging ramp over a log-fold change is both; on a diverging scale
it runs on the signed deviation from the centre, so a logarithm there is a
symmetric one. A log domain is strictly positive as `Log`'s is: an empty bin
gets the undefined colour rather than the colour of the rarest observation.

`scale.Threshold`, `scale.Quantize` and `scale.Quantile` cut the domain into
classes, so a colour names an interval instead of a shade to estimate.
`Quantile` is the one colour scale that keeps its sample: a digest or a
reservoir would make the boundaries depend on row order, which
[ADR 0012](adr/0012-parallel-panels.md) forbids.

The colourbar stopped assuming a linear reading. It asks the scale which values
are worth labelling, where a value sits on the bar, and which value the ramp
reaches at a point of it — so a log bar is ticked at decades and its gradient is
sampled evenly along itself rather than across the domain, and a classed bar is
drawn in bands and labelled at the boundaries.

All of it is additive: `ColorScale` gained no method, and the three capabilities
are optional interfaces beside it in the shape ADR 0020 established.
See [ADR 0042](adr/0042-colour-transforms-and-classes.md). ✔

### v1.7 — Identity, transitions, and guides that answer to a pointer — **shipped**

One milestone with four parts, and they are one milestone because each is the
answer to the question the last one ended on. Identity was the thing this
document had said animation was blocked on for six releases; having it made
transitions a join rather than an interpolator; having those made the overlay
the only way left to draw what a reader was pointing at; and an overlay that
could not be pointed at made the guides the last piece of furniture with
nothing to say. Six records, 0043 to 0048.

#### Identity

The one thing this document had said was blocked, and the two things that
turned out to be the same problem.

A row number is not an identity. `geom.Rows` reports the row behind a mark, and
that row is an index into a table as it stands for one frame — append to it,
filter it, or window a stream and the numbers renumber. So `geom.KeyBy` names a
column instead, and the key is read from the layer's own source at the row that
was already being reported. Nothing in `geom` reads it, `Geom` gained no
method, and the IR gained no identity channel: `geom.Faceter` already exposed
both halves of what this needed, so twenty geoms became identifiable without
one of them changing. See [ADR 0043](adr/0043-mark-identity.md).

With a key, tweening is a join. `data.Tween` blends two states of a table in
**data space, before the scales** — one `Source` whose contents change rather
than a source per frame, the way `data.Stream` already worked — so every geom,
coord and backend animates without knowing it can. The row set is the union and
is fixed for the whole transition, which is what keeps two frames comparable to
`ir.Damage` and an animation off the full-repaint path. `figure.Transition`
drives it, and figure owns no clock: `At(f)` is the whole primitive, which is
also what makes a golden file of a half-finished movement a real frame rather
than an approximation of one. See
[ADR 0044](adr/0044-transitions.md).

The same key is what makes one chart able to say something another understands.
`interact.Index.Locate` is the inverse of a hit test, `Select` and `Event.Rows`
are the brush the audit had committed to, and `Live.Rebuild` now keeps the
reader's zoom — because the caller rebuilding is usually answering something
the reader did, and discarding their view as a side effect is a chart that
fights back. What is deliberately *not* here is a linking engine: a link is a
statement about two charts and this model is about one, so the host is the
link and `examples/linked` is what that looks like. See
[ADR 0045](adr/0045-linked-views.md).

Not in v1.7, in case they look like oversights. A **string does not
interpolate** — half of "ingest" is not a node — so anything a geom decides
from one decides it abruptly: a categorical slot, a discrete colour class, a
group membership. That answers "can text animate" twice: a label over a
*number* counts, because a text layer re-spells its column every frame, and
`data.Round` is what stops it counting in raw floats; a label over a *string*
snaps, with no cross-fade and no character morph, though its position still
moves. **Enter and exit do not fade**, because opacity is a property
of a layer rather than of a row and a per-row opacity channel is the IR change
[ADR 0007](adr/0007-per-mark-colour.md) refuses; `EnterFrom` puts the
answer in data space instead, so a bar grows out of its baseline. A
**`data.Stream` has no identities to join on** and animates by being redrawn,
which already worked. There is **no keyframe timeline**: a sequence is a list of
two-state transitions and building one is a host-side loop, so the pair shipped
first. There was still **no overlay layer** at this point — bucket H's
remainder — so `Input.Dragged` handed the host the rectangle and the host drew
it; v1.8 closed that. And there is **no `Grid.Live`**: a grid composes plots
into a document, and several interactive charts are several `Live`s on several
surfaces.

#### The overlay, and the last of bucket H

The chart can now draw over itself. A crosshair, a ring round a mark, the
rectangle a reader is dragging out, a tooltip sized to its own text: all of
them are things whose positions come from a pointer rather than from a column,
which is why none of them could be a layer and why `interact` — which only
reads — could not draw them either. So the overlay is a stage in `render`,
after the guides, given the backend and where the panels are.
See [ADR 0046](adr/0046-overlay-layer.md).

It is announced to no observer, and that is the property that makes it usable
rather than merely present: what an overlay draws is not hit-testable, because
a tooltip a pointer can hit is a tooltip that flickers. Getting there needed a
bug fixed that had been latent since v0.5 — `render.Observer` had no way to
*close* a layer, so everything drawn after the last one was attributed to it,
and a legend's swatches were being indexed as marks. `render.Observer.End` is
the seam that closes it.

`Crosshair`, `Highlight`, `Brush`, `Tooltip` and `Overlays` ship in the root
package, each a struct whose zero value draws nothing and whose colours come
from the theme when it is not told. `Tooltip` is the only one that measures,
and it measures through the backend that is about to draw it — which is what
`ir.Backend.Measure` has always been for.

Not in v1.8, in case they look like oversights. An overlay that **appears or
disappears is a full repaint**, because it changes how many calls a frame has
and `ir.Damage` compares them call for call; one that only moves is a damage
rectangle, which is the case a crosshair following a pointer is in. An overlay
is **not in the JSON spec**: where a pointer is is not a fact about a chart.
An overlay is given scales and areas but **not the marks**:
a tooltip that snaps to the nearest point gets that from `interact.Index` on the
caller's side, where the hit test already lives.

#### A legend you can click

The one piece of furniture a reader expects to act on. v1.8 said a legend was
not clickable because that would mean announcing the guides as hittable, and
this is that change: `render.LegendEntry` reports each row — which layer, what
label, what rectangle, whether it is off — and `interact` indexes them under a
kind of their own, `Guide`. The separation is the point rather than an
implementation detail: a hit on a swatch must not be confusable with a hit on
the thing the swatch stands for, so a guide hit carries a layer and a series and
deliberately carries no value read off an axis.
See [ADR 0047](adr/0047-clickable-legend.md).

Hiding is `render.Chart.Hidden`, indexed by layer, because visibility is a
statement about the chart rather than about a geom — a geom that knew whether
it was being shown would be carrying a fact about a reader. A hidden layer is
not drawn and is not announced, so a pointer where it used to be finds what is
behind it; it still trains its scales, and it keeps its legend row, dimmed.

The axes deliberately do not move. A toggle is a reading aid — let me see this
one without that one on top — and an axis that rescaled under it would make the
two readings incomparable, which is what the toggle was for. A caller who means
"this series is not part of this chart" says that with `Plot.SetLayers`.

And figure does not wire the click. `Live.Toggle`, `Hide`, `IsHidden` and
`ShowAll` are the mechanism, and four lines in a Click handler are the policy —
because a legend that always toggled would be wrong for one that selects rather
than filters, one where a series opens something else, or one where only one
may be shown at a time.

Not in v1.9. A layer contributing several legend rows **toggles as one**: the
rows are one drawing and there is no way to draw a third of it. And a hidden
layer **still costs its Train**, which is what keeps the axes still: hiding a
series does not make a slow chart fast, removing it does.

#### The other two guides

A colourbar and a size key answer to a pointer too, and the reason they needed
a record of their own is that neither is a series. A legend row stands for a
layer, which already exists and can be toggled; a colourbar stands for a
continuum, and clicking one could mean a threshold, a range, a filter or
nothing. So a guide hit reports a **quantity** and what the quantity means is
the caller's — the same answer, and load-bearing rather than habitual: a band
that always filtered would be wrong for a chart where it should select.

A classed bar reports one band per class with the interval it covers, because
a band is a discrete thing a reader can mean. A continuous one reports itself,
and the value is read back by inverting the *ramp* — not the axis beside it,
which disagrees wherever the ramp is compressed
([ADR 0042](adr/0042-colour-transforms-and-classes.md) settled which of
the two the reader is looking at). A size key row reports the magnitude its
sample stands for, which meant `sizeSample` finally carrying its value: a
filter written against "1.2k" is a filter against a string.

`interact.Kind` gained `Colorbar` and `SizeKey`, and `Guide` was renamed
`LegendRow` — once there were three kinds of actionable furniture, the general
name on the specific one was misleading.
See [ADR 0048](adr/0048-clickable-colourbar-and-size-key.md).

Not in v1.10. A band reports its **midpoint** rather than a position inside
itself: one colour stands for one interval and there is no gradient in there to
read, so `Lo` and `Hi` are the truth. A **drag along a bar selects a range** —
press at one value, release at another — and needs no mode: a bar cannot be
panned and there is no view on it to zoom, so a drag over one has one sensible
reading. Across bands the range is their union. And an **axis is still not
clickable**: it is drawn per panel
rather than once per chart, so a hit would have to carry which panel and which
axis, which is a third vocabulary. ✔

### Three dimensions: a third scale, projected — **shipped**

3D had been deferred as one indivisible thing for the library's whole life —
`CONCEPT.md` §5 and §14 and the v1 audit each defer it in a single line, none
of them saying what "it" is — and that deferral stayed cheap only for as long
as nobody priced the parts. Split into four records
([ADR 0055](adr/0055-depth-without-a-third-axis.md) to
[ADR 0058](adr/0058-what-3d-is-for.md)) the parts turned out to have different
costs, different blast radii and different answers. This is the second and the
third of them, built together because the second is worth little without the
third.

**`figure/three` is a package of the core and not a second path through
`render`.** The reason is one sentence: `drawLayers` walks a panel's layers and
each layer's `Build` streams straight into the backend, so **a layer is a paint
unit** — and a projected scene has no paint unit smaller than the view, because
a point can be in front of one part of a surface and behind another. Producing
one order over every layer's primitives inside `render` would need either a
depth on every drawing call, which is the identity channel
[ADR 0007](adr/0007-per-mark-colour.md) exists to keep out of the IR, or two
drawing orders, which [ADR 0010](adr/0010-panel-layout.md) exists to prevent.
So a `three.Layer` emits geometry rather than ink, and one painter projects,
orders and draws all of it.

That is also a guard rather than only a consequence. A geom handed a depth
would ignore it and draw a flat line inside a projected box — correct by its
own lights, wrong by the chart's, and silent either way. Here it cannot happen:
`geom.Line` is not a `three.Layer` and does not compile into a scene. The
channels are still `geom`'s options, read back through `geom.Configure` the way
a mark defined outside `geom` reads them, so the option set stays one namespace
and the extension model gets exercised by the library itself.

**The IR gained nothing**, and that is the invariant the record actually
defends. No `ir.Point3`, no depth on a drawing call, no `Backend3`: the package
owns its own `Vec3`, its own matrix and its own camera, projects above the
seam, and calls `FillPath`, `Polyline` and `Text` with the plain
two-dimensional coordinates those have taken since v0.1. So every backend drew
a surface on the day the package compiled — SVG, PDF, canvas, raster, the
native window, the GPU tier — and a third-party backend written against v0.1
draws one without its author ever having heard of one. There is a test that
walks a whole scene's calls and fails on anything that is not one of the calls
the IR has always had.

**Hidden surfaces are ordered, not buffered**, because a z-buffer needs pixels
and SVG and PDF have none. That makes the scope: a painter's algorithm is exact
only over a set that can be totally ordered, so the package draws the shapes
whose order is decidable and declines the ones that are not.

What makes it exact rather than merely usual is the depth key, and getting that
key right took two goes. It is the same formula for every primitive of every
layer — **one order means one formula**, and the first draft let each layer
choose between two, which are two numbers on two scales, so merging a surface
with a path was arithmetic rather than geometry. What the sort compares is a
pair. The primary is *footprint* depth, the centroid's depth with its height
set to zero: monotone along every view ray exactly as true depth is, and a far
better sample, because a quad of a surface has a footprint one cell wide
however steep it is and the side of a bar has one of no width at all however
tall it is. For a single-valued height field that makes the order provably
right, which is a reading of the data rather than an approximation of a depth
buffer, and a second thing the refusal of perspective buys. The secondary is
true depth, and it is what makes the key work for a *scene* rather than for one
layer: two surfaces over one grid stand on the same footprints and tie, and a
camera looking straight down collapses every footprint at once. Last is the
emission index, which is lexicographically (layer, row), so it is total and
free and independent of scheduling
([ADR 0012](adr/0012-parallel-panels.md)'s rule).

No hysteresis: two marks whose depths differ by a millionth swap legitimately
as the camera turns, and a picture that depended on which frames preceded it
could not be golden-tested. And the tests cast rays rather than restating the
rule: a test whose expected answer recomputes the ordering proves only that the
sort sorts, so the occlusion check intersects the view ray through each sample
point with the plane of every primitive covering it and insists the nearest is
painted last.

Adjacent faces of one style are merged into one drawing call, and only where
that is provably the same picture — an opaque, unoutlined run. Two faces next
to each other in the order can still overlap on screen, and then a run that
draws all its fills and then all its outlines puts a farther outline over a
nearer fill, while a union filled once is not what several translucent fills
compose to. An optimisation that changes the picture is not one.

**The camera is a value the caller holds.** `three.Orbit`, `three.Dolly` and
`three.Slerp` are pure functions from one camera to another — same inputs, same
camera, testable without a surface — and the package installs no handler, opens
no window and runs no loop. How many radians a pixel of drag is worth is a
statement about how an interaction *feels*, which belongs to whoever owns the
input layer, along with inertia, momentum and springs. Under a turn the damage
diff is skipped, because every drawing call in the moved view differs and
walking two whole recordings to arrive at an answer known before it started is
work nobody asked for; at rest the diff runs and earns its keep exactly as it
does in a flat chart.

**And because a camera is a value, there can be more than one.** That is the
consequence none of the four records contained and
[ADR 0062](adr/0062-a-scene-and-its-views.md) is: a `three.Scene` is the data,
a `three.View` is one camera on it, and four views of one surface are the plan
and elevations an engineering drawing has always had. The scales are trained
**once** however many angles are drawn from them — a scale accumulates, so
training per view would move the domain by however many pictures the author
asked for, which is a bug in the axis rather than in the picture and there is a
test that counts the calls. It matters more than it looks: a static export
cannot be turned, a printed page cannot be turned, and a reader comparing a
designed part with a measured one wants both from the *same* angle.

The forms are [ADR 0058](adr/0058-what-3d-is-for.md)'s rank one, less the
scatter: `three.Surface` over a regular grid, `three.Line3` through a volume,
and — out of rank two, because it costs four lines over the same machinery —
`three.Bar3`, whose doc comment says to read the heatmap first because that is
usually the right answer. The cascade, which is the form that decides whether
the machinery pays for itself, needed no code at all: it is thirty
`three.Line3` traces and one `geom.GroupBy`, and `examples/cascade` is all of
it.

Not in this milestone. **No perspective** — under one the same value is taller
at the front of the scene than at the back, and a chart is a measuring
instrument first. **No lighting model** beyond one directional shade per face.
**No arbitrary meshes**, no volume rendering, no 3D pie. **No legend and no
colourbar** in a projected scene, and none is missing: every channel of all
three forms is an axis with ticks, and `render`'s guide column is unexported
because that record says `render` gains nothing here. A form that genuinely
needs one is what would move the guide column into a shared place, and that is
its own decision rather than a side effect. **A hit reports its mark and its
row and not a pair of values**, because a turned cube has no screen axes to
invert a device position through — figure says exactly which datum it is and
the program says what that datum contains, which is
[ADR 0045](adr/0045-linked-views.md)'s bargain one dimension up. ✔

---

**[README](../README.md)** · **[CONCEPT](../CONCEPT.md)** · **[ADRs](adr)** · [The gallery](gallery.md) · [Chart forms](charts.md) · [Interaction](interaction.md) · [A million rows](scale-out.md) · [Reading a chart](reading.md) · [JSON and Arrow](spec.md) · [Features](features.md) · [Chart-type catalogue](chart-types.md) · [Benchmarks](benchmarks.md)

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
  a marker shape of its own, so that colour is not the only channel. The
  patterns half of that promise landed later, as
  [ADR 0069](adr/0069-hatching-as-the-third-redundant-channel.md): a dash needs
  a stroke and a marker needs a point, so until a filled mark could be hatched
  this reached a line and a point cloud and left a stacked bar chart exactly as
  unreadable as it was.
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

Node-link was the one row left, and it is drawn since
[ADR 0077](adr/0077-a-node-link-layout.md): stress majorization descends a named
objective and never raises it, so a constant number of sweeps is a bound whose
cost can be stated. (The set charts arrived in between, and they were never a
relational layout — see [ADR 0074](adr/0074-sets-are-counted.md).) See
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

The depth key took three goes to get right, and the two wrong ones are worth
recording because both looked better than the answer.

It is **one formula for every primitive of every layer**, which is the part
that was wrong first: a surface keyed its quads one way and a path keyed its
segments another, and two measures of depth are two numbers on two scales — so
merging them is arithmetic rather than geometry, and one layer sorts wholly
before the other however the geometry runs.

The second attempt made that one formula the depth of the centroid *dropped to
the floor*, on the argument that it varies less over a steep primitive and that
it is monotone along every view ray. Both halves of that are true and the
conclusion does not follow, which is the sentence worth keeping: **monotone
along a ray says nothing about two centroids, which lie on two different
rays.** Two sheets at different heights whose footprints overlap without
coinciding come out backwards under it — a review found the case and it is now
a test, with its numbers and its shared pixel in it.

So the key is the depth of the primitive's own centroid, ties to the emission
index, which is lexicographically (layer, row) and so is total, free and
independent of scheduling ([ADR 0012](adr/0012-parallel-panels.md)'s rule).

And the scope the record states loosely is stated exactly, because it is
smaller than "correct" and pretending otherwise is how the next version of this
mistake gets made. **Two primitives are ordered correctly whenever their depth
ranges are disjoint — when a plane across the view direction separates them.**
Where the ranges interleave, no per-primitive number decides between them, and
this package does not cut them apart to find out: cutting is a BSP tree, which
is a renderer. What keeps real charts inside the promise is that the shapes
emitted here are already small — one quad per cell, one face per bar side, one
primitive per path segment — so a depth range is a cell wide rather than a
scene wide. The case that leaves it is several layers over a grid coarse enough
that one cell spans more depth than the layers are apart, and the answers to
that are a finer grid or a view each.

No hysteresis: two marks whose depths differ by a millionth swap legitimately
as the camera turns, and a picture that depended on which frames preceded it
could not be golden-tested.

The tests cast rays rather than restating the rule, which is the other thing
that review changed. A test whose expected answer recomputes the ordering
proves only that the sort sorts; the occlusion check intersects the view ray
through each sample point with the plane of every primitive covering it and
insists the nearest is painted last. Every one of the four fails on the key it
replaced, which is what a test is for.

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

**The furniture is drawn by `three` itself and it says less rather than more.**
The three walls facing away from the camera carry the grid; the ticks come out
of the same `Scale.Ticks` every flat axis uses; the labels stay upright at
projected anchors because a sheared label is harder to read than an upright
one and `ir.TextRun` has no shear anyway. The rule worth knowing is what
happens when an axis points at the reader and projects to a few pixels: it
labels itself only if **at least two** of its numbers survive the collision
pass, and otherwise shows none. One number there looks like a label and reads
like one while naming a position the reader cannot tell from any other on that
axis, and unlike a pile of overlapping numbers nothing about it looks wrong.

**A projected scene also found a cost model in the raster backend.** A shaded
surface reaches a backend as one drawing call per quad, because the IR carries
no per-mark colour; gg was rasterising the panel's clip into a coverage mask
and consulting it on every one of those calls, so a figure cost its calls
times its area. Every clip this library pushes is a rectangle, so
[`ir.Path.AsRect`](../ir/path.go) names that shape once and `backend/gg` hands
it to a scissor instead. The gallery renders five times faster than it did,
every chart included; nothing about it is specific to three dimensions except
that nothing before made enough drawing calls to notice.

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

### A locus: a family of curves given by a formula — **shipped**

[ADR 0033](adr/0033-smith-charts.md) had named what would reopen it — a second
chart wanting a grid family its axes have no tick for — and when the second
chart turned up it turned out not to want that seam at all.

A **Nichols diagram** is how control engineers read an open loop: phase in
degrees along X, gain in decibels along Y. figure drew that on day one; it is a
`geom.Line` over two linear axes. What makes it a Nichols diagram is the grid
printed underneath, two families of curves describing the *closed* loop
T = L/(1+L) — and neither family is at a value of either axis. A grid line is
where an axis says a value is; these are where the *chart* says something is,
which is the definition of an annotation, and figure has had annotations since
v0.1. So `geom.Locus` went in `geom/annotate.go` beside `HLine`, which is the
degenerate member of the same idea: the locus of constant y.

**The coord draws the family for free, and that is the whole return.** A family
emits points in data space, so a locus goes through the coordinate stage like
every other mark. A constant-VSWR circle is not implemented as a circle: it is
the set of impedances whose reflection has a given magnitude, and `coord.Smith`
makes it the circle it looks like. A constant-Q arc is the locus |x| = Q·r, two
straight rays in impedance and therefore two arcs on the disc, by the same map
and with no second implementation. Four families, two charts, and nothing below
`geom` touched: `render`, `ir`, `coord`, `scale` and `layout` are unchanged.

**What the implementation had to decide is how finely to draw a formula.** A
Nichols chart is the log-polar view of a circle, and the two are not evenly
spaced against each other: the arc passing near the origin is a hair of the
circle and is the whole plunge to −∞ dB, while the rest of it is a smooth curve
a few dozen samples describe. Walked uniformly the −1° contour stops dead at
−6 dB — a contour hanging in mid-air well above the bottom of the panel — so a
curve is bisected until its step is a few pixels and the refinement stops at a
step that leaves the window, which is also what bounds it. The branch point
where every N contour passes through L = 0 is found by the half-turn the phase
takes between two samples rather than by looking for it.

Not in this milestone. **No labels written along a curve** — "3 dB" rotated to
the tangent needs an anchor rule and a seat at the table
[ADR 0040](adr/0040-label-collision-avoidance.md) sets, and the chart reads
without it because the contours nest. **No automatic level selection**: which
contours a chart prints is a convention of its field, which is
`scale.TickValues`'s situation exactly. **No ZY overlay, Hall chart or
psychrometric families** — each is a `Family` and nothing else, a few dozen
lines of arithmetic in `stat` whenever someone wants one. And a family a caller
writes in Go **draws but does not serialise**, which is
[ADR 0041](adr/0041-qq-plots.md)'s rule for a quantile function applied to a
curve. ✔

### A ternary chart: three components on a triangle — **shipped**

A ternary plot reads three components that sum to a constant — a soil's sand,
silt and clay, a rock's three oxides, a classifier's three-class probability —
as one point inside an equilateral triangle. It has been standard in petrology,
metallurgy and soil science for over a century, the tooling for it is thin
everywhere, and Go had none. Everything it wants apart from the coord had
shipped between v0.1 and v0.9, so `coord.Ternary` is the whole milestone
([ADR 0051](adr/0051-barycentric-coord.md)).

**The third component is derived, not read.** X is the first component, Y the
second and the third is `Sum − x − y`; `coord.TernarySum(100)` is the
percentage spelling most tables arrive in. Three named columns were refused
because a coord transforms a *mapped pair* — that is what the coordinate stage
is built on — and because a row whose columns sum to 0.98 has to be normalised,
refused or ignored, and every answer is wrong for somebody. Deriving the third
makes the constraint hold by construction.

**It is the cheapest coord in the package, because the map is affine.** A
barycentric point is a 2×2 matrix and a translation, so `Straight()` is true and
every geom draws what it drew under Cartesian; `Area` is four transformed
corners, which makes a `geom.Rect` a parallelogram and a ternary heatmap over
binned compositions cost no new mark; `Invert` is the inverse matrix, so a
tooltip reports a composition. `Clip` is the triangle, the one thing it does not
inherit. `Decimates()` is false by derivation rather than observation: a column
of screen is a band of constant *b − a*, not of constant *a*, so a reduction
over pixel columns does not measure what it was defined to. Both domains are
pinned to `[0, Sum]`, as a Smith chart's are — a triangle autoscaled to a tight
cluster is not the simplex — so a zoom relabels and moves nothing.

**The third grid family is where the record earned its place.** A ternary has
three labelled ladders and a panel has two tick lists, the constraint
[ADR 0033](adr/0033-smith-charts.md) had named. The constant-c line at level v
pairs with the X tick at v, so it was first drawn as a second subpath inside
`GridX[i]`, taking grid ink, with `Furniture` unchanged — and its labels were
declined, to be spent once on the general problem rather than on two thirds of
it. [ADR 0070](adr/0070-a-third-labelled-family.md) spent them: the third
family is now a family of its own with its own labels, and each component is
read along its own edge, cyclically, which is how every printed ternary chart
is arranged. `examples/ternary` draws a soil texture triangle and a QFL diagram.
A `geom.Locus` was the alternative for that family and stays the escape hatch
for a fourth one; it was not the default because it takes annotation ink and
this is a grid line.

Not in this milestone. **No ternary zoom**: three ranges have to stay mutually
consistent or the region stops being a triangle, which is its own decision.
**No three named columns and no normalisation**, per the argument above — a
caller whose table carries three divides by their sum. **No quaternary
diagram**: three free components is three dimensions, the 3D question and not
this one. The corner labels naming the components are still `geom.Note`,
because a ladder carries numbers and not a name. And a Piper diagram is two
ternary panels and a Cartesian one in a `Grid` — a recipe, not a feature. ✔

### A horizon chart: forty series in the room of one — **shipped**

Every answer figure had to scale was an answer to *row count*: decimation, the
density raster, the hexbin. None helped with forty sensors at a thousand samples
each, which is not a big series but a lot of them, and the only answer in the
tree was faceting, which divides the space rather than reusing it. A horizon
chart cuts a series' range into bands of equal height, draws every band at the
panel's full height and tells them apart by colour, negative values mirrored
back from the other arm of the ramp; Heer, Kong and Agrawala measured it
reading better than a filled line below about forty pixels
([ADR 0065](adr/0065-horizon-charts.md)).

**`geom.Horizon` is a mark, and the fold runs in `Train`.** `geom.Bands(k)`
cuts the trained domain into k bands; `geom.BandHeight(h)` gives the band in
the data's own units and wins where both are set, because 50 kW is the spelling
an engineer has and "a third of the maximum" changes when tomorrow's data
arrives. It is not a position adjustment: an adjustment *moves* a mark, where
the fold cuts one row into up to k clipped spans, all occupying the same
rectangle. And it runs in `Train` by [ADR 0028](adr/0028-distribution-stats.md)'s
rule — the fold does not describe the Y axis, it replaces it — which also puts
decimation after it, in the order that matters: LTTB dropping the sample that
decided a band would change the *colour* of a stretch of chart.

**The colourbar is the ladder the chart gives up.** The folded Y axis describes
one band and is correct for every band on screen; what the reader is missing is
*which* band, and that is a colour. So the layer's guide is a classed colourbar
whose breaks are the fold's own boundaries, printed in the data's units — no new
furniture, no new guide kind, no change to `render`. The implementation made it
a `scale.Threshold` rather than the `Quantize` the record named, because a
quantize scale derives its boundaries from its domain and that would be two
computations of one fact; `geom.Horizon.breaks` is the single place the edges
exist. The ramp is `palette.BlueOrange`.

Three smaller things came out of building it. A band nothing reaches into is not
drawn, so the fill count is a fact about the data rather than k empty runs per
arm per frame. The mark reports one position per row, at the band the row ends
in, which is where the reading stopped. And `Tension` is ignored, because a
spline through a clamped fraction overshoots the only interval a band has; the
floor is shared with `geom.Area` through `appendFloor` so the two cannot
disagree. `examples/horizon` is sixteen meters faceted, with the band height
pinned.

Not in this milestone. **`GroupBy` is an error**, not an overlap: N series in
one panel are N full-height bands painted over each other, and a wall of them is
a facet or a `Plot.Track` per series. **No stacked horizon**: the fold destroys
the position a stack speaks through. **No band count chosen from the data**,
because a band that moves with the maximum makes two charts of one quantity
incomparable, which is the one thing the form is for. **No `Plot.Horizons`
sugar** until somebody has written the long form twice. ✔

### A raster: a measured field as one image — **shipped**

A heatmap is `geom.Rect` with `ColorBy` over two band scales, and that is enough
for a few dozen categories by a few dozen. It stops being enough at the size a
measured field comes in: a minute's spectrogram is two thousand frames by five
hundred bins, and one `Rect` per cell is a million paths in an SVG no browser
will open, to draw cells smaller than a pixel. `ir.Backend.Image` was already
in every backend, `stat.Grid.Raster` already painted a grid into an
`*image.NRGBA`, and `stat.Lattice` already resolved a long `(x, y, v)` table
into a product grid. `geom.Raster` wires the three together
([ADR 0066](adr/0066-a-raster-mark.md)).

**It reads the channels `geom.Contour` reads, for the contour's reason.** A
raster with isolines over it is the commonest form this mark takes, and two
resolvers that agree today disagree at the first duplicated position. So both
run `stat.Lattice`, and `geom.latticeError` is the one function both call to
turn a fault into a sentence — a test compares the two word for word.

**The image is built at the panel's resolution, with nearest neighbour.**
Letting `Backend.Image` scale one pixel per cell was refused: a smooth upscale
invents colours between measurements, and the backends do not agree on what
scaling means. Downsampling is named rather than assumed — `geom.Resample` over
`geom.Nearest`, `geom.Mean` and `geom.Max`, a small closed family that
serialises. Nearest is the default because it never shows a number nobody
measured; Max is what a spectrogram wants so a peak survives. The pixel-to-cell
mapping is inverted through the scale per pixel column and row, so a log
frequency axis gives the low bins more pixels, and the mark costs the panel's
size rather than the field's.

**A ramp read per pixel was the whole cost.** `palette.Lerp` blends in linear
light and measured 640 ns a colour — 150 ms for a 600 by 400 panel. `geom.ink`
reads the scale into a table once per domain, one colour per class or 1024
samples along a continuous ramp, and the same panel costs 3.3 ms with no
allocations. `examples/spectrogram` is 1322 frames by 128 bins and one `<image>`
where a rect per cell would be 169,216 paths.

**And it has a colourbar**, which the hexbin cannot: a raster's values are known
in `Train`. It is the contour's `ColorGuide` rather than the shared helper, and
a raster naming no `ColorBy` still takes `palette.DefaultRamp` and still draws
its bar. Axes are trained to the cell edges, so the outer half cells are not
clipped. A non-finite cell is transparent. A hit reports a row per cell only
while a cell is at least a pixel across, because below that a pixel under
`Mean` is no one cell.

Not in this milestone. **An unequally spaced lattice is refused** and the fault
names `Rect`, which draws it correctly one box per row. **A non-Cartesian coord
is `ErrWarpedRaster`**: a blit is axis-aligned, and a square image in a round
panel is a false chart. **`geom.Decimate` is an error**, because a raster's cell
count is its resolution. **No interpolation of scattered points onto a grid**,
which is an estimator rather than a way of drawing, **no bilinear upscaling**,
**no image-as-data** and **no per-cell borders**. ✔

### A colour that carries two readings — **shipped**

Three charts wanted one missing piece and none was big enough to build it for
alone: a value-suppressing uncertainty palette, which gives a quantity fewer
distinguishable colours the less certain it is; a multi-class hexbin, coloured
by which class dominates a bin and how purely; and the 3×3 bivariate
choropleth. `scale.ColorScale` is one number in and one colour out, and
[ADR 0036](adr/0036-error-bars.md)'s sentence — a measurement and a claim about
how well it is known, whose second half had nowhere to go — applied to the
colour channel with more force than anywhere, because a filled cell reads as a
measurement whether or not anybody made one
([ADR 0067](adr/0067-a-bivariate-colour-channel.md)).

**A bivariate scale rides `ColorScale` rather than widening it.**
`scale.BivariateColorScale` is the third rider, after `DiscreteColorScale` and
`ClassedColorScale`: `Color(v)` stays and answers at full certainty, so every
mark keeps working. The second column is `geom.UncertaintyBy(col)`, and naming
one against a scale that reads one number is `ErrNotBivariate` rather than a
column silently ignored. Building it added `TrainSecond`, which `geom` calls
beside `Train` when a layer read a second column, and `KeyCells`, the key as
rectangles in data space with a colour each — which is how one drawing function
draws a VSUP's tree and a matrix's square without asking which it has.

**VSUP is a classed scale whose class count is a function of the second
reading.** `scale.VSUP(ramp, classes, layers)` is `layers` rows of `Quantize`,
each halving the classes and mixing at most seven tenths of the way toward a
light grey — resolution taken away, not the colour removed.
`scale.BivariateMatrix` takes its colours rather than two ramps, because the
record also refused blending two ramps and both cannot hold: a mixed colour
names no value. `palette.BivariateBlueRed` is Joshua Stevens' published 3×3,
registered by name. Both constructors are also `ClassedColorScale`s for the
first reading, so a layer with no second column draws the classed bar it would
have drawn, and only a layer naming both gets the square key — the fourth guide
kind, a constant and two functions as [ADR 0027](adr/0027-size-channel-and-the-guide-column.md)
priced it.

**The multi-class hexbin is `geom.Hexbin` plus `geom.GroupBy`.** `stat.Hex`
counts per class, classes are named by first appearance, and the cell's colour
is the dominant class against impurity. Its classes are known in `Train` and its
counts are not, so it contributes ordinary legend entries and says how pure a
cell is through its colour. `examples/bivariate` draws all three charts.

Not in this milestone. **A path given two readings is `ErrRampOnPath`**: a line
changes colour where its reading crosses a class boundary, and two readings
cross in different places. **No opacity as uncertainty**, the cheap version
VSUP's authors measured as misleading. **No N-variate channel** — three readings
have no key a reader can decode. **The key reports nothing to an observer**,
because a position in a square is a pair and a drag across it is the
two-dimensional brush left to the host. **The uncertainty is the caller's
column**; `stat` does not compute it. ✔

### Probability paper: the axis warps, not the sample — **shipped**

Probability paper warps an axis so that one distribution's cumulative function
plots as a straight line, and the line's slope and intercept *are* the fitted
parameters. Weibull paper is the load-bearing kind — the standard method for
lifetime data in reliability engineering — beside normal, Gumbel and logit, and
no general-purpose plotting library draws it without an incantation or a
third-party package. `geom.QQ` looked like the same thing and is its opposite:
a QQ plot warps the *sample* and labels its axis in z-scores; probability paper
warps the *axis* and labels it 0.1 %, 1 %, 50 %, 99.9 %, which is what the
reader came for ([ADR 0052](adr/0052-probability-scales.md)).

**One scale with four named links.** `scale.Probability` takes `scale.Probit`,
`scale.Logit`, `scale.CLogLog` or `scale.Gumbel`, and the four share a domain of
(0, 1) exclusive, a ladder, a `Definite` bound and every line of mapping but
one call, so four types would have been the naming rule followed off a cliff.
`scale.Link` is `Apply` and `Unapply`, open to a caller, and the named four
round-trip through `scale.LinkName` and `scale.LinkNamed`. A link written in Go
has an empty name in its `Desc`, which `scale.FromDesc` refuses with
`ErrUnknownKind` rather than substituting probit. That rule is now three
records old — a named member of a small closed family is written down, an
arbitrary Go function is not.

**The charts cost no marks.** `Definite` covers the ends exactly as a log scale
covers zero, so `geom.ECDF` on a probit Y is a normal probability plot and on a
Gumbel Y is extreme-value paper. What the ECDF did need was to stop handing the
backend NaN: its staircase reaches 0 and 1, both unplaceable here — and the
bottom one already was on a log Y — so it now leaves out a vertex its axis
cannot place.

**The ladder is a convention, and it is the scale's own.** 0.1 / 1 / 5 / 10 /
20 / 30 / 50 / 70 / 80 / 90 / 95 / 99 / 99.9 is no search's answer. It extends a
decade at a time into whichever tail the domain reaches, and thins by rank —
the median, then the decades, then 5 / 20 / 80 / 95, then 30 / 70 — so a short
axis loses rungs in mirrored pairs; a rung that loses its label stays a minor
tick unless `ProbabilityMinorTicks(false)`. Labels are per cent at the shortest
precision each rung needs, and `ProbabilityNumberFormat` replaces that outright.
A pan stops at the sixth decade of a tail.

**The plotting position closed on `stat.MedianRank`.** Benard's rank is one
point per row and is read as points, where an ECDF is a staircase over distinct
values, so an option on the ECDF would have made one mark draw two objects. A
Weibull plot is `geom.Scatter` over median ranks on a `CLogLog` Y against a log
X, and `examples/weibull` recovers β from the ranks by regression in its test.

Not in this milestone. **No fitting**: a scale that fitted a distribution would
assert the answer the chart exists to let a reader find, and the line is a
`geom.Segment` a caller who knows the parameters draws. **No confidence bounds**
on the plot, **no censored plotting positions** — that is
[ADR 0054](adr/0054-statistical-instruments.md)'s territory — and **no
probability colour scale**. `Nice` is not offered, because there is no round
number to round to. ✔

### A tidy tree, and the clustered heatmap it completes — **shipped**

[ADR 0039](adr/0039-relational-layouts.md) had declined a node-link layout
because a force simulation runs until it settles. That argument binds *force*
layouts and not *tree* layouts, and a tree is where most of that category's
charts live. The Reingold–Tilford tidy tree in Buchheim, Jünger and Leipert's
linear form is O(n), deterministic, bounded and a pure function of its input —
`stat.Squarify`'s shape — with published invariants for oracles: no two
subtrees overlap, a parent is centred over its children, isomorphic subtrees
are drawn alike ([ADR 0053](adr/0053-tidy-tree-layout.md)).

**`geom.Tree` is a fifth relational mark, in the unit square, reading 0039's
channels and adding none.** `geom.ID`, `geom.Parent` and `geom.Value` are what
it reads. Under `coord.Cartesian` it is a dendrogram or an org chart; under
`coord.Polar()` a radial dendrogram — the third time the unit square has turned
one mark into two charts. An edge is an elbow by default and a segment on
request, both as data-space points, so under a polar coord the bracket's
cross-piece becomes the arc. Leaf order is first appearance in the table;
reordering to reduce crossings is refused for 0039's reason, and `geom.Order`
is how a caller asks for another.

**There are two layouts, and the height column chooses.** A tidy tree compacts
subtrees by depth, which is right only while depth is where a node is drawn. A
dendrogram's leaves all sit at height zero, so compaction would tuck a shallow
leaf above a deep neighbour's. `stat.Tidy` therefore has `Reset`, the tidy
tree, and `ResetLeaves`, one slot per leaf with each parent centred — used
exactly when `geom.Value` is present, which also lines a dendrogram up with a
heatmap's columns. It is a struct because the walk keeps a dozen per-node
buffers, runs without recursion, lays a forest out under a virtual root, and
exports `Leaves` so a caller can order a heatmap by it. Its tests include a
200 000-node chain and star.

**The headline chart needed two things nobody had predicted.** A track shares
the panel's axis and a heatmap's is ordinal, so the refusal of an ordinal
breadth narrowed to the height axis: leaves are placed at their own names
through `scale.Categorical.Encode`. And a dendrogram on the *left* edge needs its
breadth on Y, which is `geom.Orient` with `geom.Vertical` and
`geom.Horizontal` — named for the library rather than for the tree, so the next
mark says it the same way. `Baseline(1)` mirrors the heights within their
extent, one rule for a depth tree and a dendrogram, and a root at the hub wants
`coord.Hole`, because the centre has no angle. `examples/dendrogram` draws a
clustered heatmap with a dendrogram on two edges, and a radial tree.

Not in this milestone. **No force-directed layout and no general graph**: 0039's
refusal stands verbatim, and every property above depends on one parent per
node. **A cycle is `ErrCyclic`**, naming the node, where the icicle beside it
silently drops the nodes. **No optimal leaf ordering, no crossing minimisation,
no edge bundling**, and **no clustering**: a dendrogram's heights are the
caller's columns however they were produced. **A DAG** wants a layered layout
whose crossing reduction is the heuristic sort both records refuse. ✔

### The statistical instruments: survival, control, correlation, ranking — **shipped**

Some charts had every mark they needed since v0.1 and none of the arithmetic: a
Kaplan–Meier curve is `Step` and `Area`, an SPC chart `Line` and `HLine`, a
correlogram `Bar`, ROC and Lorenz curves `Line`. Outside domain packages —
`survminer`, `lifelines`, `qcc`, `statsmodels` — nobody draws them, and in Go
nobody at all. The question [ADR 0054](adr/0054-statistical-instruments.md)
answers is not whether they are pure but **where the line is**, because "put
the field's arithmetic in `stat`" has no natural end.

**The admission test: a reduction belongs in `stat` when its output is the
chart's geometry and has no reading that is not the chart.** An ECDF is its
staircase; a loess curve has no life off the chart. A hierarchical clustering,
a regression model and a meta-analysis all fail it, and the test admits five.

**`stat.KaplanMeier` gets a mark.** `geom.Survival` runs it in `Train`, reading
X as the time and `geom.Event` as the indicator — a numeric column, non-zero
for an event, because `data` has no boolean and a 0/1 column is how every
survival dataset arrives. `stat.SurvivalPoint` carries the risk set and the
running Greenwood sum, and `Band(z)` is the log-log interval, the default in R
and lifelines because the plain one runs past 0 and 1 where a curve is read
hardest. `geom.Confidence` and `geom.CensorMarks` are the opt-ins. The layer
starts at time zero unless the axis is a time scale, where zero is an epoch.
The numbers-at-risk table is a `Plot.Track`, which a mark cannot make —
`examples/survival` is Freireich's remission trial with its table in one.

**Control limits get no mark, and that is domain-correct.** Limits come from a
baseline and are then frozen; a mark recomputing them from the points it was
handed would be silently wrong for the principal use. So `stat.Limits` is the
result and `LimitsIMR`, `LimitsXbarR`, `LimitsNP` and `LimitsC` return it,
beside `AppendLimitsP` and `AppendLimitsU` for per-subgroup limits.
`stat.AppendRunRules` is Nelson's eight rules returning a selection, flagging
every point that completes a window so a derived column colours the whole run.
`examples/spc` is an individuals chart whose drift the rules catch. It found two
colourbar faults, fixed outside `stat`: a classed bar now labels a boundary with
the decimals it needs, and `geom.Guide(false)` lets a layer decline its bar.

**`stat.ACF`, `stat.PACF`, `stat.ROC` and `stat.Lorenz` are stat only.** The
ACF is the biased estimator, which Durbin–Levinson needs, and a gap makes every
lag NaN rather than silently shifting the ones after it. ROC walks tied scores
as one diagonal step, so its area is the Mann–Whitney probability; a curve with
nothing to rank is empty with a NaN area, not 0.5. `Lorenz` returns the Gini
coefficient beside its curve.

Not in this milestone. **No fitting, modelling or inference**, and **no
recomputed control limits**. **No censoring beyond right-censoring**, which
changes the estimator rather than the chart. **Forest, funnel, Pareto and
Bland–Altman plots are recipes** in `docs/charts.md` — a forest plot's pooled
estimate is a meta-analysis and fails the test on every clause. **No mark for
the ACF**, because bars, stems and points are all conventions and no reading
distinguishes them. ✔

### A schedule you can read: progress, constraints and milestones — **shipped**

"Gantt / timeline" had been a row in [chart-types.md](chart-types.md) since
`geom.Rect` shipped — *a rect on a time X against an ordinal Y* — and that row
was true and was about a third of the chart. The bars said when each task ran.
They did not say how far any of them had got, what was waiting on what, or
where the dates that are not spans were, and a schedule without those is a
picture of a plan rather than something to work from.

**Two additions, and the rest turned out to already be there.**
`geom.ProgressBy` is an option on the rect: the mark paints twice over, the
whole span at the unfinished alpha and the finished part of it at full
strength, which keeps the colour batching a heatmap depends on and keeps a
bar's two halves the same colour. `geom.Depends` is a mark, and it is the first
in this library to read **two** tables — the plan and a list of links over it,
joined by `geom.KeyBy` — because a dependency is a statement about two rows of
the task table and has no position of its own. Everything else a schedule needs
was already a recipe: a milestone is `geom.Scatter` with a diamond, a baseline
is a second narrower rect, today is `geom.VLine`, a label is `geom.Text`. The
two that shipped are the two that could not be composed.

**A critical path is a column rather than a feature.** `geom.ColorBy` over the
link table paints each arrow from its own row, so colouring the links whose
slack is zero *is* drawing a critical path. A `CriticalPath` option would have
put a scheduling algorithm in a plotting library and answered a question — what
counts as critical — that belongs to whoever built the plan.

**One rule was broken on purpose, and it is written down where it happens.**
Every other geom computes a midpoint, a corner or a staircase step *before* the
coord, because those are statements about the data. A dependency's elbow is
not: there is nothing in data space between the finish of one task and the
start of another, and the path a reader's eye takes between them is a reading
aid. So the two ends go through `coord.Point` like every other mark and the
corners between them are device points, with a stub measured in pixels because
it is there so that the corner can be seen. The direction out of each bar is
*measured* rather than assumed, which is what makes a reversed axis turn both
ends together and what makes one routing function serve a gantt drawn down the
page as well as across it — the same quarter turn ADR 0031 makes between a
bottom track and a left one.

Not in this milestone. **No summary bars**: a phase drawn as a bracket
spanning its children is a third shape rather than a third option, and it would
have to answer whether the rollup is computed or given. **No obstacle-avoiding
routing**: an arrow goes over a bar rather than round it, because routing round
one needs to know where every bar in *other* layers is, and no mark has that.
**A hit on an arrow names the layer and not the constraint**, because the row
it would report belongs to the link table and `Source` hands out the other one —
which is the first time in this library a layer has had two tables to be
ambiguous about, and is ADR 0015's revisit clause rather than this record's. ✔

### Hatching: the third redundant channel — **shipped**

[ADR 0024](adr/0024-accessibility.md) quoted the promise of *"redundant
encoding (patterns/dashes)"* and shipped the dashes. `theme.Redundant(true)`
filled in a dash ladder and a marker ladder, and that was the whole of it. A
dash needs a stroke and a marker needs a point, and **a bar has neither** — so
the marks that fail hardest in greyscale, a stacked bar, a pie, a stacked area,
a treemap, were exactly the marks the one option did nothing for. Turning
`Redundant` on for a stacked bar chart changed nothing at all.

**A hatch is an enumeration, like a marker.** `ir.Hatch` has fourteen values in
four families — lines, dots, wavering lines and tilings — and `geom.Hatch`
names one on a layer. An enumeration rather than a parameter struct because a
redundant encoding needs a *ladder* a layer index walks, and a ladder of
arbitrary angles is a ladder nobody can name a rung of. It is `ir.Marker`
again, including the rule that makes that safe: the set grows at the end, and a
value this release does not know leaves the mark with its plain fill.

**The pattern says *different* and the period says *more*, and the two are kept
apart.** `Redundant` installs `theme.DefaultSeriesHatches`, where every rung is
a different shape and none is heavier than another, because a redundant
encoding must not invent an order the data does not have.
`theme.DensitySeriesHatches` is the other ladder, for a stack that really is an
order — severities, age bands — and `theme.Hatches` installs it by asking, since
guessing wrong means a chart that claims a magnitude nobody measured. How
coarse every pattern is belongs to the chart rather than to a series, so it is
`theme.HatchSize`.

**It is drawn, not declared.** `ir.Backend` and `ir.Fill` are unchanged, and a
hatch never reaches a backend at all: `ir.FillHatched` fills the path, pushes it
as a clip and strokes the lines `ir.HatchPath` generates inside it. The frozen
interface is the first reason — a `Fill.Hatch` field would be ignored in
silence by every backend written before it, and a chart that quietly loses its
pattern still looks finished. The second is that the GPU tier's SDF accelerator
collapses any non-solid brush to a single colour, so a brush-based hatch would
vanish the moment `backend/gg/gpu` was imported. The third is that an SVG
`<pattern>`, a PDF tiling pattern, a canvas pattern and a gg brush are four
rasterisations of one idea, and lowering once means every backend draws the
same lines.

**A hatch is not a mark.** `interact` indexes every stroked subpath, and hatch
lines would put dozens of phantom targets inside every bar. `ir.Decoration` is
the fix, in the shape every other extension of the backend contract has — an
optional interface beside `Partial`, `Resizer` and `Semantics`. `FillHatched`
brackets its pass in it, the probe stops indexing inside the bracket, and
`ir.Recorder` records it so a parallel render hit-tests as a serial one does.

Three finishes came with it, none needing a backend to learn anything.
`geom.Corner` rounds rectangular marks, asking `Path.AsRect` whether a shape
*has* corners rather than asking the coord. `geom.Inset` strokes a mark's own
path at twice the border width clipped to itself, which is an inset outline
exactly with no path offsetting. And `geom.Gradient` gives a geom the linear
gradient `ir.Fill` had carried since v0.1, whose only caller had been the
colourbar. The first rung of the hatch ladder is `HatchNone`, so every existing
golden file is byte-identical, and `geom.Desc` carries `Hatch` beside
`HatchSet` for the reason `Dash` and `DashSet` do.

Not in this milestone. **No hatch scale**: no `geom.HatchBy`, no hatch guide —
a pattern driven by data would be a fourth guide kind and a fifth scale
interface, and this is about a chart surviving a photocopier rather than about
a fifth variable. **An annotation is never hatched by a theme**, because a band
that is deliberately quiet should not acquire a pattern by default. **A layer
painted per row from a discrete colour scale wears one hatch**, since its marks
are batched by colour. And **no drop shadows**: there is no blur in the IR, and
an offset copy looks cheap in vector output. ✔

### A third labelled family: the ternary's missing ladder — **shipped**

Three records had deferred the same seam.
[ADR 0033](adr/0033-smith-charts.md) declined the Smith chart's constant-|Γ|
circles because `render.drawAxes` takes label text from `t.Label`, so *"a
family with no tick behind it has nowhere to come from and nothing to be
labelled by"*, and named the trigger: a second chart wanting a grid family its
axes have no tick for. [ADR 0051](adr/0051-barycentric-coord.md) was that
chart. A ternary panel has three labelled ladders and two tick lists; it drew
the third family as a second subpath inside the X tick's shape and left it
**unlabelled** — which is the one edge carrying the clay fraction or the third
phase a reader is often after.

**A family carries its own text, and that is the whole of the widening.**
`coord.Furniture` gained `Families`, each a `coord.Family` with a name, one
shape per level, a label position per level and the text each says. The
constraint 0033 named was never that a family had nowhere to be *drawn*; it was
that label text reached `render` only from `scale.Tick.Label`. A family that
says what its own levels are called removes that for every customer at once.
The slice is reset and regrown with the struct's other buffers, and a coord
that raises none costs nothing.

**`render` applies three rules, and each is the one it is for a reason.** Lines
take grid ink and answer to `HideGrid` and the theme's grid stroke, but *not*
to `ShowGridX` or `ShowGridY` — a family is neither axis, and taking it down
with the horizontal grid would make the answer depend on which component the
caller put on X. Labels take tick ink and the tick font, because a ladder's
numbers are tick labels wherever they appear. And labels are **thinned last**:
a family is a third reading of the chart, and where its number collides with an
axis's the axis keeps it.

`coord.Ternary` raises the constant-c family, and placing it made the one
visible change to existing charts: **each component is now read along its own
edge**, cyclically, which is how a soil texture triangle, a QFL diagram and a
phase diagram are printed. Before, the first two ladders both ran out of one
corner, which left the third nowhere to go but on top of them. The grid lines
did not move.

A later amendment, from [ADR 0078](adr/0078-a-coord-with-more-than-two-axes.md),
split "when the panel writes tick labels at all" into its two switches. A
parallel-coordinates panel's families *are* its axes, and letting
`theme.ShowTicksX` govern them took the numbers off every dimension. So a coord
may set `Furniture.FamiliesAreTheAxes`; `coord.Parallel` does and
`coord.Ternary` does not.

Not in this milestone. **It is not a caller-facing seam** — a coord fills
`Families` and `render` reads it; a fourth ladder is still `geom.Locus`, which
is an annotation and a layer. **It is not a spec field**: a family is derived
from the coord and its ticks, so a ternary chart serialises exactly as it did.
**It is not a third side** — a side has an axis line, tick marks and three
placement flags a family does not. **No hit-testing**, because furniture is not
indexed, and **no minor levels**, because there is no tick generator behind a
family. The Smith chart's families and Γ as input are each now a coord's own
arithmetic; none of them is built here. ✔

### A curve is chosen for the drawing, a smoother for the data — **shipped**

A line had one smoothing knob. `geom.Tension` fitted a Catmull-Rom spline, and
zero meant a polyline. That was wrong for some data — **a cardinal spline
overshoots**, so through a cumulative total it dips below the previous reading
between two samples, ink at a value nobody measured — and the JSON dialect
already spoke Vega-Lite's `mark.interpolate`, writing `"cardinal"` beside the
tension and throwing the word away on the way back in.

**A curve is a drawing, fitted after the coord.** `geom.Curve` names a
`CurveKind`, spelled as Vega-Lite spells them: linear, the cardinal three,
monotone, natural, the basis three and bundle. The fit runs on device points in
`geom/curve.go` and changes the path between the vertices and nothing else: no
axis is trained on it and `stat` never hears of it, which is what lets it be
chosen last, by whoever is looking at the picture.

**Interpolating and approximating are two claims, and both are allowed.**
Linear, cardinal, monotone and natural pass through every vertex, so the ink
between two rows is a claim about what the quantity did between them. Basis and
bundle do not — the vertices are control points — and their doc comments say
they miss the data in those words. `CurveKind.Interpolating` makes the
difference a value a caller can branch on. What does not change is what a row
is: every family reports its vertices, so a basis curve that sails past a peak
still hands a tooltip the peak.

**Monotone falls back to the polyline, not to the spline.** The family takes
whichever device axis is strictly monotone, so a series running down the page
works as well as one running across it. When neither is, no monotone fit
exists, and falling back to cardinal would silently deliver the overshoot the
family exists to prevent. A bent coord still takes the edges over from every
family, because a tangent fitted in device space under polar is smooth on the
screen and wrong about the data, and `geom.Horizon` still draws straight edges
through its clamped bands.

**A smoother is a fit, and it lives in `stat`.** `stat.MovingAverage` and
`stat.SavitzkyGolay` run in data space in `Train`, and the axis is trained on
what they produce, beside `Loess` in `geom.Smoothing`. They are two because they
fail differently: a running mean can be checked by hand and flattens every
peak; a Savitzky-Golay fit keeps a peak's height and width, and the tests
assert that a quadratic survives an order-two fit untouched. Both take `Span`
as their width — a second knob spelling the same fraction in other units would
be the thing to regret. And the two halves are **not one option**, because
"what happened between two readings" and "what is happening under the noise"
are different questions and a reader needs to know which was answered.

`Tension` alone is still `CurveCardinal` at that tension, so every existing
chart draws what it drew; `Desc` carries `CurveSet` because `CurveLinear` is
both the zero value and a choice.

Not in this milestone. **No curve under a bent coord** — a rounded radar is a
reasonable thing to want, and it needs a spline fitted in scale space and
mapped per sample, a different mechanism to be decided as such. **No family
parameterised by arc length**, which `CurveCardinalClosed` approximates rather
than solves. **No Savitzky-Golay order on a layer**: `geom.Trend` fixes it at
two, the lowest degree with a curvature. And **no family may be spelled with a
`step` prefix**, because that is how the document tells a staircase from a
line. ✔

### A label on a curve: contours that read without a colourbar — **shipped**

`geom.Contour`'s own doc comment deferred it, [ADR 0050](adr/0050-locus-annotations.md)
deferred it for "3 dB" along an M contour, and a contour chart was the one
chart in the catalogue that could not be read without a second guide: it says
*where* a crossing is, and which crossing it is came from a colourbar or from
counting rings. `geom.LabelLevels(true)` writes the level on the line, for
`Contour` and `Locus` both, and `geom.LevelFormat` says how
([ADR 0073](adr/0073-labels-on-a-curve.md)).

**It is placed after the coord, and that is the second exception.** The label
is turned to the curve's tangent, and a tangent in data space is not the
tangent on screen: under `coord.Polar` a constant-radius arc is straight in the
scaled pair and bent on the panel, and under `coord.Smith` a straight sweep in
impedance is a circle. So the placement reads only the device points the mark
is about to stroke. It never sees a datum, which is why one helper serves a
traced lattice and a formula.

**The curve offers the candidates; the placer only says yes or no.** A label
joins [ADR 0040](adr/0040-label-collision-avoidance.md)'s table with one
difference: **`move` is false**. A text label may be nudged, because near its
point it still names its point; a curve label moved off its curve names a level
it is not on, and that is a chart that lies rather than one that is crowded.
`curveLabelTries` positions spread along each run are scored by how far the
curve sags from the chord under the label, one bending more than
`curveLabelSag` font heights is no candidate at all, the flattest wins with ties
to the start of the run, and a refusal tries the next. The list is bounded and
the order total, so the picture is the same on one goroutine or eight.

**A gap, not a halo.** A halo needs the background colour, which a layer does
not know over a transparent panel or a raster; a knockout needs a text
background, which would be the IR's first compositing decision. A gap is
geometry: the run is stroked up to the label and resumed after it, as two
subpaths of the path it was going to be anyway, with the ends interpolated so a
coarse lattice is not gapped for half a ring. The text is turned upright and
centred in its own gap, **once per run** — a level is many runs, each a separate
statement, and repeating along one would need a spacing knob in device units.

**The text is decoration and the geometry is data.** Levels are settled in
`Train`; where a label sits is chosen in `Build` from what the backend says the
string measures, so two backends may gap a curve a few pixels apart and neither
moves a line. `ir`, `render`, `coord` and `scale` are unchanged, the gapped
halves go into buffers the layer keeps, and a contour drawn earlier wins the
label space from a later text layer, which is 0040's rule as written.

Not in this milestone. **No text following the curve per glyph** — that is
paragraph layout by another name. **No repeated labels along one run**, **no
leader lines**, and **no thinning of which levels are written**: which levels a
chart shows is `Levels`' question. `LevelFormat` **does not serialise**,
[ADR 0041](adr/0041-qq-plots.md)'s rule for a Go function. **Filled bands
between levels** are still a polygon where this is a path. And a labelled
*furniture* family is not this: this labels a mark. ✔

### A layered graph: state charts and pipelines — **shipped**

[ADR 0039](adr/0039-relational-layouts.md) refused node-link layouts, and
[ADR 0053](adr/0053-tidy-tree-layout.md) narrowed the refusal to force layouts,
shipped `geom.Tree`, and left a clause for a DAG: layered layout is bounded
too, but its crossing reduction is a heuristic sort, and *"a sort is where a
layout stops being a pure function of its input."* The DAG that turned up was a
state machine, and [ADR 0072](adr/0072-layered-graph-layout.md) is the answer.

**A stable sort by a computed key, with the index as its tie-break, is a pure
function.** Barycentre sweeps sort each rank by where its neighbours are, and
equal keys are the common case — a node with one neighbour takes its position
exactly. The fix is to make the key total: the sort is stable, the incoming
order is the interning order and therefore the table's, the sweep count is the
constant `stat.LayeredSweeps`, and nothing reads a map's iteration order. What
does not survive is any claim to be optimal. Crossing minimisation is NP-hard;
a different row order draws a different, equally valid picture, and that is
written down as a documented input rather than smuggled in as quality.

**Cycles are broken, not refused.** `geom.ErrCyclic` is right for a sankey and
a hierarchy, but a state machine that cannot return is not one. A depth-first
walk in index order marks the back edges, layering runs on what remains, and
the marked edges are drawn in their true direction against the rank order,
flagged by `stat.LayeredEdge.Back`. Five phases — break, longest-path rank,
dummy-node routing, ordering sweeps, placement — each bounded by one pass or by
the sweep constant. Longest-path rather than network simplex, whose pivot rule
is a tie-break that would need justifying all over again.

**The layout never sees a label.** Graphviz sizes nodes from their text; doing
that here would give the SVG and the PNG of one document different geometry.
`stat.Layered` places nodes in the unit square knowing only the graph, and
`geom.Graph` sizes each box from `ir.Backend.Measure` at build time, as
decoration. Building it sharpened that: nodes on the outer ranks need an inset
of half the widest box, so a node's *position* does move with the font — as
every mark already does when the panel is fitted round measured labels. What is
a pure function of the graph is the **arrangement**, which node on which rank in
which order, and `TestAGraphsArrangementDoesNotDependOnTheFont` holds it.

It fills the unit square, so the orientations are free: ranks up the panel
under Cartesian, down it with `geom.Baseline(1)`, concentric under
`coord.Polar()` — which is why there is no `rankdir`. Edges are `geom.From` and
`geom.To`, the flow channels unchanged. A node's box is a device rectangle built
with `ir.Path.RoundRect`, not an area handed to the coord, because a labelled
box must not become an annular sector. `stat.Layered` carries its own merge sort
because `sort.SliceStable` allocates per call, and `TestLayeringAgainDoesNotAllocate`
holds the gate.

Not in this milestone. **No force-directed layout** — 0039's refusal stands.
**No clusters or subgraphs**, which would be a second solver, and **no ports**.
**No edge labels**: a transition's `event [guard] / action` needs the de-overlap
pass [ADR 0032](adr/0032-text-as-a-mark.md) deferred. **No `geom.NodeSep`**:
the record offered it, and in a layout that fills the unit square there is
nothing for it to widen, so a long name can still crowd its neighbour. **No
DOT frontend**, which is a separate record if it is ever written. And the cost
is the ranks an edge crosses rather than the edges, so this mark is for graphs
that are nearly layered already. ✔

### Sets are counted: an UpSet plot, a Venn diagram and the set sizes — **shipped**

[ADR 0039](adr/0039-relational-layouts.md) had refused Venn and UpSet in one
sentence with two reasons in it, and the second was not a refusal at all: an
UpSet plot is a matrix chart rather than a relational layout, which makes it
cheaper than the neighbour it was refused beside. It has no geometry to solve.
Its columns are the combinations of sets that occur, its bars are how many
elements are in exactly each, its rows are the sets — a bar chart over a dot
matrix, both of which figure drew on day one. What was missing was the
arithmetic.

**It is a count, and the count is a `stat`.** `stat.Intersections` counts a
membership list by which sets each element is in; a combination is a bit per
set in a `uint64`, so sixty-four sets (`stat.MaxSets`) is a mask rather than a
limit anybody meets. It never sees a string: the geom interns, the stat counts
indices. And it counts *exactly* — an element in A and B is counted once, under
{A, B}, and not again under {A} — so the counts partition the elements and add
up to how many there are, which is what a circle labelled with its own total is
not. A membership row is a bipartite edge, so `geom.From` names the element and
`geom.To` the set: 0039's channels, unchanged.

**Two marks, one count, so the panels cannot disagree.** `geom.Intersections`
draws the bars and `geom.SetMatrix` the dots; both read the same table and run
the same count in `Train`, because a bar standing over the wrong column is the
one way this form can lie. `geom.Order` and `geom.Top` decide which columns
exist and must be handed to both, and ranked biggest-first is the default — the
only place in the package where appearance order is not, because an UpSet plot
*is* a ranking. That default is why `geom.Order` now records having been told.
The panels themselves are the caller's, because a mark cannot make one: the
two-panel form is `Plot.Track(figure.Bottom)` sharing the X scale object, and
the three-panel form is a `figure.Grid`. Nothing was added to `figure`,
`render`, `layout`, `coord`, `scale` or `ir` for either.

**The third panel's count had been done by hand, and it was the wrong count.**
`examples/sets` built the set-size bars with a map and a loop that counted
*rows*, while the stat beside it counts *elements* — a join that hands back the
same (element, set) pair twice made the left panel disagree with the two beside
it, and the library held the right number with no way to ask for it.
`geom.SetSizes` reads `stat.Intersections`' `Sizes` from the same table
([ADR 0076](adr/0076-the-other-half-of-the-count.md)). It accepts `Order` and
`Top` and ignores them, because a set's total is over the whole table, and it
has no orientation option: which way its bars grow is the axis's business, which
is the next section.

**A Venn diagram of two or three sets is a table, not a solver.** `geom.Venn`
draws one disc, two side by side, or three on the corners of an equilateral
triangle — the arrangement every printed diagram uses — so 0039's objection to
an optimiser does not reach it. See [ADR 0074](adr/0074-sets-are-counted.md).

Not in this milestone. **No area-proportional diagram from three sets on**: two
circles have three free numbers against three region areas and a bisection
settles it, but three have six against seven, so a solver minimises an error it
cannot drive to zero and fails subtly. The two-set case is not refused, only
not built. **No fourth set** (`MaxVennSets` is 3): the reason first given was
geometric and was wrong — four ellipses do show every region — so the amendment
restates it as this mark's contract, a count written *into* its region at one
type size in a fixed arrangement, which a four-set diagram's slivers cannot
hold. **No row reported**, since a column and a region are aggregates. **No
lane order by set size**, which would decide what two marks draw and so needs a
record of its own. ✔

### An axis has a direction — **shipped**

The set-size bars of a printed UpSet plot grow *away* from the matrix they
label, and `examples/sets` grew them towards it because there was no way to ask
for the other direction. [ADR 0039](adr/0039-relational-layouts.md) had named
the answer — a positional `scale.Reverse` that survives the round trip — and
said in passing that a reversed domain would not have survived it anyway.

**The refusal was measured before it was answered, and the round trip was never
what stopped it.** Pinning `Domain(hi, lo)` mirrors `Map` and `Invert` exactly
and writes `"domain": [12, 0]`, which reads back unchanged. What stopped it was
three parts of one file, each right on its own terms: `Zero` and `Nice` handed
back an ascending pair, the containment test in `Ticks` found no value between
a descending `lo` and `hi` — so the axis drew its marks mirrored and carried *no
numbers at all*, even a sequence pinned with `TickValues` — and the first pan
turned it back round, because `Zoomer.SetDomain` orders what a drag hands it.
So `Domain`, `LogDomain` and `SymLogDomain` now order their bounds, which keeps
the invariant rather than patching it three times.

**What reverses is the pixels, not the numbers.** `scale.Reverse()` sets one
field on the linear scale, and `linear.device` hands `Map`, `Map64` and
`Invert` their range the other way round. Everything that trains, frames,
searches or filters a domain still sees a low number and then a high one, which
is why a reversed axis pans, zooms, autoscales, clones, snapshots and describes
itself without another line written for any of them; no branch was added to a
drawing path. `Domain()` still reports ascending bounds and `Ticks` is still
ascending **by value** — it is the positions that descend. The dialect spells
it `"reverse"`, the word a colour scale already used, and it is written only by
the kind that reads it back.

**Drawing the chart found the fourth thing.** `render.selectXLabels` drops a
label that would run into the one before it, and it swept in tick order — which
on a reversed axis is right to left. A 400-pixel chart came out with `0` on its
X axis and nothing else. The sweep now runs in screen order, and it is the only
change outside `scale` and `spec`: reading the code had said the work was a
comparison in `Ticks`. See [ADR 0075](adr/0075-an-axis-has-a-direction.md).

Not in this milestone. **Log, symlog, time and probability axes** do not
reverse yet — the flip is `device()` and three call sites in each, and what is
missing is a chart asking and the tests with it; their pinned domains are
ordered either way, so none half-works meanwhile. **No ordinal reverse**,
because turning a band scale around is a question about category order, which
is `geom.Order`'s. **A projected scene presumably inherits it**, and presumably
is not a claim: nothing was tested against one. And writing a domain backwards
is refused rather than supported. ✔

### A node-link layout by stress majorization — **shipped**

The last row of bucket E had been refused since
[ADR 0039](adr/0039-relational-layouts.md) with a sentence welding two claims
together: that a force layout cannot be a pure function at a bounded sweep
count, and that it cannot be one "that also looks good". The first was too
strong — a fixed start, step and count repeat exactly — and the second had never
been tested. What is true is narrower. **Stress is a named objective, and
majorization never goes uphill**, so the cost of stopping after
`stat.StressSweeps` (fifty) can be stated and measured; a simulation stopped
early is wherever it happened to be in its own swing.

`stat.Stress` places the nodes and `geom.NodeLink` draws them, and three of the
claims behind it are measurements.

**The sweep is in place, and the order is load-bearing.** Each node's block of
the majorizing quadratic is solved with the others held still and written back
before the next node reads it, which is block coordinate descent and does not
raise the stress. The first version computed every position from the previous
sweep's and called that order-independence a virtue; two nodes a target of 1
apart, placed 1.02 apart, cycled 0.98, 1.02 for ever at constant stress.
`TestASweepNeverRaisesTheStress` and `TestTheSimultaneousSweepIsWhyThisOneIsNot`
hold both halves.

**The start is classical scaling, chosen by eigenvalue value.** A circle start
folded a 4×4 grid in half. Power iteration finds the biggest eigenvalue by
magnitude, and graph distances are often not Euclidean — K(5,5)'s centred table
has a −5.5 that the iteration converged to and reported as 5.5 — so the
spectrum is shifted non-negative first. A table of rank one is an answer rather
than a failure: a path is a line, and keeping the one direction brought a
five-node chain from bowed by a seventh of its span to under a percent.

**A fixed spiral does a random start's real job.** Two nodes with the same
neighbours sit on a saddle every sweep leaves alone; the gallery drew two team
members as one dot with two names. `Stress.scatter` pushes every node a
hundredth of the spread along its own direction, the golden angle apart.

The layout runs in `Train` in the unit square, and `Build` fits it into the
largest square the panel holds, because what it matched is a distance and a
stretched axis would undo it. `stat.MaxStressNodes` is 250, where the hairball
and the quadratic cost arrive together; a bigger graph is `ErrTooManyNodes`. See
[ADR 0077](adr/0077-a-node-link-layout.md).

Not in this milestone. **No force simulation** — not impossible, but it has no
objective, so the cost of its bound cannot be said. **The row order picks the
minimum**: the gallery's collaboration graph settles at a stress of 3.47 in its
own order and 2.30 under one relabelling, and a cleverer start only moved which
numbering was unlucky. **No multi-start**, because a trial was mixed — the
gallery's order gained a third, the relabelled one lost — and a default changes
on better evidence; fifty sweeps is a budget, not convergence. **No edge
weights as distances, no multi-level coarsening, no arrowheads or bundling, no
canonical rotation.** A node reports no row and an edge its own. ✔

### Parallel coordinates: a coord with more than two axes — **shipped**

[chart-types.md](chart-types.md) had called parallel coordinates the widest
genuine gap on the page. A radar gets away with one scale because its spokes are
one quantity measured several times; a parallel plot exists because its axes are
*different* quantities, and squashing miles per gallon into the range of
kilograms says something false. What it needed was per-axis scales inside one
panel, and half of that had arrived unnoticed:
[ADR 0070](adr/0070-a-third-labelled-family.md)'s `Furniture.Families` are
ladders with their own lines and their own label text.

**`coord.Parallel` is the first coord to hold scales.** Every other coord
borrows the panel's two and re-ranges them; this one carries one per dimension,
because N axes with N domains is what the panel *is*. `Frame` ranges each onto
the unit interval and pins the panel's own X and Y to mean which axis and how
far up it — `ternary.pin`'s move made N times. A dimension is a label and a
scale and never a column name: `geom.Dims` names the columns, the two are
matched in order, and a count mismatch is `geom.ErrDimensions` out of `Train`.
A mark reaches the scales through `geom.Training.Dims`, filled via the optional
`coord.Dimensions`, and at `Build` through `f.Coords()` — `Coord` gained no
method and `geom.Frame` is unchanged.

**The axes are furniture, and nothing in `render` knows how many there are.**
Each dimension raises one `Family` — line, level marks, labels in its own units
and its name at the top — stroked and written by code that has been there since
0070. The panel's own ticks are silenced by the coord, since they would number
an axis index. One of 0070's rules had to be amended: a theme's
`theme.Ticks(false, false)` would have unlabelled the whole chart, so
`Furniture.FamiliesAreTheAxes` keeps the dimensions' numbers through it, and a
ternary chart, which does not set it, is unchanged. The labels sit inside the
panel, over the lines, because there is nowhere outside to put N ladders.

**A row is a line, reported at every axis it crosses**, batched by colour with a
subpath per row so a thousand rows in three colours are three drawing calls. A
row missing a value gaps there rather than being joined across. And which way
up an axis reads is free: `scale.Linear(scale.Reverse())` gives the "better is
up" arrangement with nothing from the coord or the mark. See
[ADR 0078](adr/0078-a-coord-with-more-than-two-axes.md).

Not in this milestone. **No categorical axes**, which are a second placement
rule and a half-step towards parallel sets. **No brushing or dragging axes**,
which are the host's. **No chosen axis order**, since an order out of an
optimiser is not a pure function of the input. **No curves between axes.** And
a gutter is still sized from the panel's own tick labels, so a chart that
leaves its theme alone reserves room it never writes into — `layout` knows
nothing about coords, [ADR 0018](adr/0018-coordinate-systems.md)'s deliberate
deferral, and this is the second chart to notice. ✔

### Parallel sets: the count between categorical columns — **shipped**

[ADR 0078](adr/0078-a-coord-with-more-than-two-axes.md) left the table with
*categorical* columns drawn nowhere. A line per row is no answer: nine hundred
tickets over three such columns have twelve distinct paths between them, so the
picture shows which combinations occur and nothing about how many. And a caller
crossing neighbouring columns by hand into a `geom.Sankey` edge list is the
shape [ADR 0076](adr/0076-the-other-half-of-the-count.md) had just refused for
the set sizes.

**It is a count, and not a coord.** `geom.ParallelSets` shares `geom.Dims` with
`geom.Parallel` and nothing else. A category has no value to place on a scale,
and the ribbons lie *between* the axes, so a coord that drew them would be
deciding a layout and drawing it. Both axes describe the unit square, as a
treemap's do. The arithmetic is `stat.Crosstab` — indices in, counts out, names
interned by the geom.

**The layout is `stat.Sankey`'s, because the count is a flow.** The same total
passes through every column, the strongest form of what a flow layout needs. In
the busiest column the nodes fill the interval exactly, so the relaxation is a
no-op there and the plain stacked partition comes out; a sparser column spends
its slack standing nearer what it joins. One scale serves the diagram, so a
ribbon's thickness means the same thing everywhere. The column each category
stands in is *given*, through a new `Sankey.ResetColumns`: the longest-path rule
would send a category nothing reaches to the far left among the sources, which
`TestPinnedColumnsHoldWhereDerivedOnesWouldNot` pins.

**The crossings are sorted once, and that is not the sort 0039 refuses.** A
crossing is a count over rows, not a row, so the table has no opinion about its
place; `Crosstab` orders by source category, target and class, a total order on
distinct keys and one sort of the answer rather than one per sweep. It stacks
ribbons in the order of the boxes at their far end, so they cross only where
the data does. A `geom.ColorBy` column subdivides the ribbons rather than
averaging a colour nobody named, and the boxes stay in the theme's label ink,
because asking the discrete scale for a box's colour would put every category
in a legend naming the classes.

**A row missing a category is counted nowhere** — the one place an absent value
costs a row. Each column partitions the same total, and skipping only the
crossings beside a gap would leave one column adding up to less than the next.
See [ADR 0079](adr/0079-parallel-sets.md).

Not in this milestone. **No labels on the boxes and no column titles**: a mark
that wrote text would need a policy for a box too small to hold its name. The
legend names the categories — `channel: phone`, joined by
`geom.DimensionSeparator`, so two columns' "yes" stay two — which makes this
the form that usually wants `figure.Legend(true)`. **No reordering to reduce
crossings**, the sankey's refusal unchanged. **No invented "missing"
category**, **no crosstab of every pair**, and **no row behind a ribbon**. An
alluvial diagram needs nothing that is not here. ✔

### Nearest-neighbour cells: a partition cut where the reader measures it — **shipped**

The sweep of unusual forms in [chart-types.md](chart-types.md) had one row left
that was a decision rather than a recipe, and it had been left out for the
weakest reason in the file: nobody had asked. What people draw with it is a
rainfall map, a catchment, a coverage map of depots or gauges — and the only
architectural objection had gone when [ADR 0077](adr/0077-a-node-link-layout.md)
admitted a layout under a fixed budget. A nearest-neighbour partition is not
even that: it is an intersection of half-planes, arithmetic with an answer
rather than a search with a stopping rule.

**The question no mark had answered is in what space a distance is measured.**
`geom.Voronoi` reads `X` and `Y` like a scatter, trains them like a scatter and
places its sites through `coord.Point` like a scatter. But a cell's whole
content is that its boundary is *halfway* between two dots, and there is no
distance between a millimetre of rain and a kilometre of easting. Cut in the
scaled pair and stretched into the panel, the partition goes through an
anisotropic map that takes perpendicular bisectors to lines that are not, and
every boundary on screen would be halfway between nothing. So `stat.Voronoi`
runs in `Build` against `Frame.Area` — the fourth stat to do so, after the
hexagonal lattice, the beeswarm and the treemap's squarify. The cost is stated
rather than hidden: the picture depends on the panel, and `examples/voronoi`
draws the same table with and without a colourbar to show the cells move. Under
`coord.Polar` the partition is clipped to the disc by the coord's own clip; the
mark asks the coord nothing.

**It intersects half-planes; it does not sweep.** Each cell is the panel
rectangle clipped by one bisector per other site — quadratic where Fortune's
sweep is O(n log n), and chosen anyway, because a convex clip has one branch and
no topology to get wrong where a sweep decides its picture on circle-event
predicates a rounding away from a different answer. A site farther than twice
the cell's current radius is skipped with one distance. `stat.MaxVoronoiSites`
is 1000 and `geom.ErrTooManySites` refuses past it in `Train`, in
`geom.NodeLink`'s manner: a thousand cells in a 900×600 panel are twenty pixels
a side, and past that the answer is a raster. The result is a pure function of
its input in which even the row order does not matter, which no other layout
here can say.

**A cell is a row, and it reports one** — the first layout-shaped mark for which
that is true. Two rows at the same device point have no bisector: the first
keeps the cell, the second draws and reports nothing but still cuts every other
cell. Colour is per cell, batched one call per colour with one subpath per
cell, which keeps [ADR 0007](adr/0007-per-mark-colour.md) intact and lets a
pointer land on the cell it is inside. Nothing below `geom` learned anything.
See [ADR 0080](adr/0080-nearest-neighbour-cells.md).

Not in this milestone. **No dots and no labels** — `geom.Scatter` and
`geom.Text` over the same table do both. **`GroupBy` is ignored**, because one
partition over every row has no series inside it. **An outline only when both
`Fill` and `Color` are named**, `geom.Rect`'s rule for `interact`'s reason, and
**no padding**, because an inset cell is no longer the set of points nearest its
row. **No Lloyd relaxation, weighted diagram or Voronoi treemap** — each an
optimiser [ADR 0039](adr/0039-relational-layouts.md) refuses — and **no
Delaunay triangulation**, because interpolating between samples is a claim about
values where a cell is a claim about nearness. ✔

### A map projection: a coord handed degrees — **shipped**

A geographic projection was deferred in one line in four places and argued in
none. [ADR 0018](adr/0018-coordinate-systems.md) had set the terms: a
projection has no linear interval underneath it and wants to be handed the data
domain rather than a mapped position, and that wider seam should be argued on
its own evidence. When the argument was finally written, both halves of the bill
turned out to have been paid already, by charts nobody thought of as maps.

**The axes are degrees, because the Smith chart already did this.** `coord.Geo`
calls the same `identityRange` [ADR 0033](adr/0033-smith-charts.md) wrote for
the reflection coefficient: each scale is given its own domain as its range, so
`Map` is the identity and what reaches `Point` is a longitude and a latitude.
The axes want a **linear** scale for the Smith chart's reason. And three things
come free from the axes being ordinary: the graticule *is* the ticks —
`scale.TickValues(-180, -120, …)` asks for an atlas's graticule — a regional
map is a narrower domain, and a map **zooms**, because unlike a Smith chart its
picture is derived from the domain every `Frame`.

**The graticule had a tick behind it all along.** Four records had circled the
claim that a graticule needs a labelled family with no tick behind it, and
[ADR 0070](adr/0070-a-third-labelled-family.md) spent the seam partly for it.
With degrees on the axes a meridian is a longitude tick's own shape, exactly as
a polar ring is a Y tick's, and goes into `Furniture.GridX`. What has no tick
behind it is the **edge of the map** — a globe's rim, a Mollweide's ellipse —
and that one unlabelled line is the family. Each ladder of numbers is written
along the longest line of the other, which reproduces the atlas on a
rectangular map and puts a Mollweide's longitudes on the equator.

**The projection is a name, and the default is the equal-area one.**
`Mollweide`, `Mercator`, `PlateCarree` and `Orthographic` are a closed set of
`coord.Projection` values, so a document can name one; an unknown name is
`coord.ErrUnknownProjection` rather than the nearest guess. `DefaultProjection`
is `Mollweide` because on a map how much ground a thing covers reads as how much
of it there is — the other three are fine things to ask for and never a fine
thing to get by not choosing. The map keeps its shape with one scale factor for
both directions, fitted by walking a 33 × 33 lattice over the domain (plus the
rim for a globe) rather than by four sets of extremal formulas. A place with no
image is NaN, and the walk starts a new subpath where the image resumes;
Mercator is cut at ±85.051129°, and `Extent` reports the cut. Everything the
coord draws for itself is walked in longitudes measured from the centre
meridian, or a Pacific-centred map would draw its parallels straight back across
the world. Nothing in `geom`, `render` or `layout` changed. See
[ADR 0081](adr/0081-a-map-projection.md).

Not in this milestone. **No geography**: a coastline is rows in the caller's
table, and a GeoJSON reader is a package allowed dependencies the core does not
have. **No spherical arithmetic** — a great circle and a rhumb line are two
claims about one pair of places, and `examples/map` draws both from its own
rows. **No antimeridian wrapping**, because 179° and −179° are two degrees apart
or three hundred and fifty-eight and the pair does not say which. **No filled
ring of rows** for a choropleth, which is a mark with four questions of its own.
**No conic projection**, whose two standard parallels `coord.Desc` has no field
for. And **no decimation**, because a pixel column is a band of longitude under
two projections and of nothing in particular under the other two. ✔

### A label fits its box, or is called out of it — **shipped**

`geom.Text` has always dropped a box label that does not fit, because an
overrunning label reads as its neighbour's. What "fits" meant was one number,
the width of the box — under a polar coord the chord across the middle of the
slice — and a label is a width and a height laid out level in a box that is
there a wedge of an annulus. Donut and sunburst labels were drawn over the
edges of their slices, which is exactly what dropping exists to prevent. And a
pie's thin slices are where the number is most wanted, which is the leader line
[ADR 0040](adr/0040-label-collision-avoidance.md) deferred in its last line.

**Fitting is decided on screen, by the coord.** The label's font box, laid out
as it will be drawn with a pixel and a half of padding, must lie inside the box,
and whether a device point does is what `coord.Coord.Invert` answers for every
coord. The test walks the font box's edges four points to a side, because a
donut's hole bows into a label whose corners are clear of it. Under Cartesian
the only change is that height now counts.

**A label is moved, broken and shrunk before it is dropped, in that order.**
Under a coord with a middle — `coord.Exploder` again — it is also tried at a
fifth, a third, two thirds and four fifths along each span, because a sunburst
cell turns as it goes round; under Cartesian only the middle is tried and a bar
chart pays nothing. `geom.Slide(false)` pins it to the middle. `geom.Wrap`
breaks it at spaces over two or three lines, choosing the break whose widest
line is narrowest, and comes before shrinking because two lines at the layer's
size read better than one at three quarters of it. Then it is shrunk to the
largest size that fits, down to `geom.MinFontSize`, three quarters of the
layer's size by default — found by scaling metrics already measured and rounded
down to a quarter of a unit, so a chart asks for a few fonts rather than one per
label.

**Below the floor, `geom.Callout` writes it outside.** The label leaves through
the edge furthest out along the box's bisector, runs out past everything the
coord draws, turns level on a short arm, and is stacked apart from its
neighbours a side at a time so that thin slices write a column. A label that
would overrun the panel is drawn in on a shorter arm first; the first version
dropped it outright and the gallery's donut lost its thinnest slice's label by
four pixels. The amendment went further: a called-out label is fitted to the
room beside the chart the way a box label is fitted to its box — one line,
then broken, then shrunk — because in a sunburst not much wider than its ring
the labels being dropped were the long names, the ones a reader could not guess
from the colour. A called-out label is outside the disc a polar panel clips to,
so `geom.Overhanger` asks `render` for the panel rectangle instead. A text layer
given `geom.ExplodeBy` moves with its slice, leader and all. See
[ADR 0082](adr/0082-a-label-fits-its-box-or-is-called-out.md).

Not in this milestone. **No callouts under Cartesian**, for `Exploder`'s
reason: a bar has no outside that is not the next bar's inside, and a label
above a short bar would be a named option rather than a default. **Called-out
labels do not take part in `AvoidOverlap`** — they are stacked against each
other, and a label from another layer is not an obstacle to them. **A leader out
of an inner ring crosses the rings outside it**, which is the price of leaving
by the shortest way. And **no labels turned along a ring**: the fit takes a
rotation already, and what is missing is the rule that picks one. ✔

### An axis break is marked or not drawn — **shipped**

Two charts asked for the same thing from opposite ends. Six sites, one thirty
times the others, where every other bar is a sliver unless the axis leaves out
the empty stretch between 12 and 85 — and says so with a `//`. And a machine's
state log, where the reader wants the time it was doing something and every
idle stretch folded away: not one interval chosen by eye but dozens read out of
the table the chart draws.

**A break belongs to the scale, like a direction.** `scale.Break` and
`scale.TimeBreak` change `Map`, `Invert` and `Ticks`, which is
[ADR 0075](adr/0075-an-axis-has-a-direction.md)'s argument: every geom,
`interact`'s inversion, a track sharing the panel's scale and every facet panel
then agree by construction. It is configuration, so `Clone` keeps it,
`Snapshot` shares it and `Describe` writes it down.

**`Map` stays continuous, and a value inside a break is not missing.** The kept
pieces share the range less the gaps, each through `place` with its own ends
snapped and its offsets rounded explicitly against a fused multiply-add. A value
in a cut maps linearly across the gap and the panel's clip hides it — so
`Invert` stays exact, `Defined` is unchanged, no geom learns about breaks, and a
bar from 0 to 95 across a break is one bar with its middle missing. Treating it
as missing would cut the tall bar the break exists for.

**An unmarked break is a lie, so a coord that cannot mark one does not get
one.** The gap is a length on the page, which neither a scale nor a coord can
know; `scale.Breaker.SetBreakGap` is called once per render from
`theme.AxisBreakGap` and `theme.AxisFoldGap`, the way a size scale learns its
range. The cuts are off until then, and switched on only under a coord that
implements `coord.Breakable` — today only `Cartesian`, whose framed form clips
to one rectangle per pair of kept pieces and splits its axis and grid lines
into subpaths, with the marks in `Furniture.Breaks` stroked after the data. A
second axis, which has no clip, stays whole; so does a break that would take
more than half the axis. A cut reaching an end of the domain trims it instead.
Ticks are chosen once for the length the axis still shows and walked per piece,
so both sides read at one step and one precision.

**A fold is a break of another kind.** `scale.Fold` and `scale.TimeFold` cut
with a narrow gap and a small slash, never a zigzag — a zigzag per fold would be
a hatch — and a binary search over running sums keeps a row at O(log n) with no
allocation, gated by `BenchmarkFolded1k` against `BenchmarkFolded100k`.
`figure.SpansWhere` reads the folds out of a table, living in the root package
because it is the one place holding both a table and a scale; the document
spells them `"cuts"` and `"folds"`, since `"breaks"` was taken, and writes the
spans rather than the query. With no break, every golden file still matches.
See [ADR 0083](adr/0083-an-axis-break-is-marked-or-not-drawn.md).

Not in this milestone. **No breaks on a log, symlog or probability axis** —
the same arithmetic per piece, and nobody asked. **No recurring breaks as a
rule**, because a business-day axis is a calendar rather than an interval list,
and a calendar's spans can already be passed as folds. **No folds a layer
derives in `Train`**, because an axis would then change when a layer was added.
**No folding a category**, which is filtering rows. **No different scale on each
side of a break**: a break leaves an interval out of one axis and does not join
two. ✔

---

**[README](../README.md)** · **[CONCEPT](../CONCEPT.md)** · **[ADRs](adr)** · [The gallery](gallery.md) · [Chart forms](charts.md) · [Interaction](interaction.md) · [A million rows](scale-out.md) · [Reading a chart](reading.md) · [JSON and Arrow](spec.md) · [Features](features.md) · [Chart-type catalogue](chart-types.md) · [Benchmarks](benchmarks.md)

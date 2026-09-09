# What figure does

The feature surface in one list: scales, marks, coordinate systems, layout, output, interaction.

- **Scales** — linear, time, **log**, **symlog** and **ordinal/categorical**.
  Linear tick placement uses
  [extended Wilkinson](https://rdrr.io/rforge/labeling/man/extended.html)
  (Talbot, Lin & Hanrahan 2010), so axis labels come out round rather than
  merely evenly spaced. Time ticks step in calendar units. Log and symlog
  subdivide each decade with unlabelled minor ticks; symlog is linear near zero,
  so signed data spanning orders of magnitude is plottable at all.
  `scale.TickValues` pins the sequence outright, for an axis whose ticks are a
  convention rather than a reading — the 0.2 / 0.5 / 1 / 2 / 5 of a Smith chart,
  the five points of a Likert item.
- **Geoms** — `Line` (optionally tension-smoothed), `Scatter` (six marker
  shapes), `Bar`, **`Area`** (to a baseline, or a band between two series),
  **`Step`** (pre/mid/post), **`Boxplot`** (Tukey whiskers, type-7 quartiles,
  outliers), and **`Rect`** — one box per row, bounded by the row rather than by
  a baseline, which is what a heatmap, a gantt bar, a candle and a waterfall
  step all are.
- **Distribution marks** — **`Histogram`**, **`Violin`**, **`Ridgeline`**,
  **`Hexbin`**, **`Beeswarm`**, **`ECDF`** and **`Trend`**. Each is a pure
  function in [`stat/`](../stat) — a 1-D binner, a Gaussian KDE with Silverman's
  bandwidth rule, a hexagonal lattice, an empirical CDF, locally weighted
  regression — with a determinism test, drawn by a mark that trains its axis on
  the summary rather than on the rows
  ([ADR 0028](adr/0028-distribution-stats.md)).
- **Relational and hierarchical marks** — **`Treemap`**, **`Icicle`**,
  **`Sankey`** and **`Arc`**, which read an edge table rather than a pair of
  axes: `geom.From`/`geom.To` for a flow, `geom.ID`/`geom.Parent` for a
  hierarchy, and `geom.Value` for the magnitude of either. Each places its own
  layout in the unit square and hands it to the coordinate stage, so an
  `Icicle` under `coord.Polar` is a **sunburst** and an `Arc` under one is a
  **chord diagram** — four marks, six charts, and no second implementation of
  anything ([ADR 0039](adr/0039-relational-layouts.md)).
- **Series in one layer** — `geom.GroupBy` splits a long table into N series
  drawn by one layer, each with its own colour and its own legend entry.
- **Position adjustments** — `geom.Stack` (from zero, to 100 %, about a
  silhouette, or with the streamgraph's wiggle) and `geom.Dodge` (side by side).
  The offsets are derived while the scales are trained, so a stacked axis
  reaches the total rather than the tallest single value
  ([ADR 0019](adr/0019-position-adjustments.md)).
- **Colour** — a qualitative palette per chart, plus **colour scales** bound to
  a column by `geom.ColorBy`: a sequential or diverging ramp for a quantity, or
  `scale.Qualitative` for categories. Which guide the layer contributes follows
  from which it was handed — a ramp gets a colourbar, a palette gets one legend
  entry per category
  ([ADR 0020](adr/0020-discrete-colour-and-multi-entry-legends.md)) — and a
  chart of one layer painted from categories shows that legend by default,
  because the colours are the only thing naming them.
  `scale.Named` colours categories the caller enumerates — `RUN` green, `FAULT`
  red — rather than in order of first appearance, so the colour of a state does
  not depend on which window of a stream is on screen.
  Ramps interpolate in linear light, so a gradient has no dark band through its
  middle. A ramp can run logarithmically across its domain (`scale.ColorLog`,
  `scale.ColorSymLog`) — without it a heatmap over counts spanning orders of
  magnitude rounds every cell but the densest few to one end — or be cut into
  classes so that a colour names an interval rather than a shade to estimate:
  `scale.Threshold` for boundaries that come from outside the data,
  `scale.Quantize` for equal ones, `scale.Quantile` for equally many
  observations in each. A classed scale's colourbar is drawn in bands and
  labelled at the boundaries.
- **Paths that change colour** — `Line` and `Step` take `ColorBy` too, and draw
  the path in stretches of one colour. Where a stretch ends follows from the
  scale rather than from an option: a classed scale puts the corner *on* the
  threshold, interpolated between the two rows, so the chart says when the limit
  was passed; a discrete scale puts it on the row where the new category was
  first seen, because nothing was measured in between and a machine's state has
  no halfway. A continuous ramp on a path is refused — a stroke carries one
  colour and no stops ([ADR 0049](adr/0049-paths-colour-in-classes.md)).
- **Coordinate systems** — `coord.Cartesian` is the identity and the default;
  `coord.Polar` wraps one axis around a circle and reads the other as a radius,
  which turns the marks that already exist into pie, donut, radar, rose, wind
  rose and gauge. A scale still maps a value into an interval — the coord
  decides what the interval means — so no geom and no scale changed shape for
  it, and the arcs are cubics because the IR has always had those
  ([ADR 0018](adr/0018-coordinate-systems.md)). A slice's inner and outer
  radius are columns like its share is (`geom.X` and `geom.X2`), and
  `geom.ExplodeBy` breaks one out of the ring without changing what it says
  ([ADR 0026](adr/0026-breaking-a-mark-out.md)). **`coord.Smith`** is the
  third one: it reads the pair as a complex impedance and maps it through
  Γ = (z−1)/(z+1) onto the unit disc, which is the chart every RF engineer works
  on and almost no plotting library draws. Its grid is the two axes' own ticks —
  constant-resistance circles from X, constant-reactance arcs from Y — so
  `render` was not touched for it
  ([ADR 0033](adr/0033-smith-charts.md)).
- **Size** — `geom.SizeBy` reads a column through `scale.Size`: the bubble
  chart. The mapping is by **area**, not radius, so doubling a value multiplies
  the diameter by √2 and two bubbles compare the way a reader already reads
  them. The layer contributes a third guide kind — a ladder of sample marks —
  beside the legend and the colourbar
  ([ADR 0027](adr/0027-size-channel-and-the-guide-column.md)).
- **Missing data** — one explicit policy per layer (gap, interpolate, error),
  covering both `NaN`/`Inf` and values a scale has no position for, such as zero
  on a log axis.
- **Annotations** — `HLine`, `VLine`, `HBand`, `VBand`, `Segment`, `Region` and
  `Note`. They take values rather than a data source, because there is no column
  behind "the SLO is 200ms", and they extend the axis so the threshold is in
  view even when the data is nowhere near it.
- **Small multiples and subplots** — `facet.Wrap` and `facet.Grid` split one
  plot by a column; `figure.NewGrid` puts different plots on one canvas. Both
  go through one constraint solver, so panels are the same size and their axes
  line up ([ADR 0010](adr/0010-panel-layout.md)).
- **Chart furniture** — axes, grid, tick labels with collision avoidance, chart
  and axis titles, and one guide column carrying a legend, **colourbars** and
  **size keys**, stacked in that order and measured by one solver.
- **Themes** — light and dark, built from a dozen
  [tokens](../theme/tokens.go) rather than fifty fields, with a colourblind-safe
  ([Okabe-Ito](https://jfly.uni-koeln.de/color/)) default palette and
  perceptually uniform sequential ramps (Viridis, Cividis, Magma). `Theme.With`
  edits one; `theme.Register` and `theme.ByName` resolve one from a config file.
- **Big data** — a layer with more rows than the plot has pixels reduces itself
  before it draws: `stat.LTTB` for a line, min/max per pixel column for a
  staircase or a band, density binning to an image for a point cloud, or a
  `geom.Hexbin` when the counts themselves are the answer. It happens
  when the chart is drawn, never when the scales are trained, so the axes still
  report the data rather than the subset that survived
  ([ADR 0011](adr/0011-decimation.md)).
- **Parallel panels** — a facet or a grid builds its panels on separate
  goroutines and replays them in panel order, so the output is byte-identical to
  a serial render ([ADR 0012](adr/0012-parallel-panels.md)).
- **Data** — columnar and batch-oriented, carrying numeric, time and categorical
  columns. A `[]float64`-backed source is borrowed, never copied, and so is a
  null-free `float64` column read straight out of an **Apache Arrow** record
  through the optional `figure/arrow/v18` module.
- **Interaction** — `Plot.On` registers handlers for hover, click, zoom and
  pan; `Plot.Live` draws into a surface that can be redrawn; `figure.Input` is
  the state machine that turns raw pointer input into those, and `Live.Bind`
  drives it from a DOM element. Hit-testing runs over the marks a render emitted
  rather than over a second copy of every geom's projection, and
  `Live.TrackRows` makes a hit name the source row behind the mark
  ([ADR 0015](adr/0015-hit-testing.md)).
- **Live data** — `data.Stream` is appended to from any goroutine and frozen
  between frames, and a redraw repaints only what changed
  ([ADR 0016](adr/0016-streaming-and-damage.md)).
- **A chart as JSON** — a `*Plot` marshals to a Vega-Lite-shaped document and
  reads back as the same chart ([ADR 0014](adr/0014-json-spec.md)).
- **Accessibility** — a chart's title becomes an SVG `<title>` with
  `role="img"`, a PDF document title and a canvas `aria-label`; `Plot.Describe`
  writes the `<desc>` a screen reader announces after it; `Plot.DataTable`
  writes the rows as an HTML table; and `theme.Redundant` tells layers apart by
  dash and shape as well as by colour
  ([ADR 0024](adr/0024-accessibility.md)).
- **Notation in labels** — optional and pluggable, with a TeX subset built in.
  A label is measured as it will be drawn, in every place a chart writes one
  ([ADR 0023](adr/0023-math-typesetting.md)).
- **Responsive charts** — `figure.Responsive` scales a theme with the size the
  chart is drawn at, and `Live.Resize` is how a surface says its size changed
  ([ADR 0025](adr/0025-responsive-charts.md)).
- **Backends** — three built-in emitters — SVG, PDF and a browser canvas — the
  gg raster adapter, a native window, and an opt-in GPU tier.

- **Identity and transitions** — `geom.KeyBy` names the column that says which
  row is which, so a hover in one chart can be acted on in another, and two
  states of a table can be blended into a movement between them. The blend is
  in data space, before the scales, and figure owns no clock: `At(f)` is the
  whole primitive ([ADR 0043](adr/0043-mark-identity.md),
  [ADR 0044](adr/0044-transitions.md)).

- **An overlay the chart owns** — `figure.Crosshair`, `Highlight`, `Brush` and
  `Tooltip` paint over the finished chart, after the guides and clipped by
  nothing. What an overlay draws is not hit-testable, because a tooltip a
  pointer can hit is a tooltip that flickers
  ([ADR 0046](adr/0046-overlay-layer.md)).
- **Guides you can click** — a hit on a legend row reports which series it
  stands for, and `Live.Toggle` puts that series away and brings it back; the
  row stays, dimmed, and the axes do not move
  ([ADR 0047](adr/0047-clickable-legend.md)). A colourbar and a size key
  report a *quantity* instead, because neither is a series: a classed band
  gives the interval it covers, a continuous ramp the value under the pointer
  ([ADR 0048](adr/0048-clickable-colourbar-and-size-key.md)).

Deliberately **not** here: geographic projections, node-link and Venn diagrams,
contour plots, 3D, and any engine that links two charts together — a link is a
statement about two charts and this model is about one, so the host is the link
([ADR 0045](adr/0045-linked-views.md)). The rest are further out in
[CONCEPT.md §14](../CONCEPT.md#14-what-is-built-and-what-is-next), and
[docs/chart-types.md](chart-types.md) says what each one would need.

---

**[README](../README.md)** · **[CONCEPT](../CONCEPT.md)** · **[ADRs](adr)** · [The gallery](gallery.md) · [Chart forms](charts.md) · [Interaction](interaction.md) · [A million rows](scale-out.md) · [Reading a chart](reading.md) · [JSON and Arrow](spec.md) · [Chart-type catalogue](chart-types.md) · [Benchmarks](benchmarks.md) · [How it was built](milestones.md)

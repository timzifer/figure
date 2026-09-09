# Chart forms, and the code that draws them

A walk through the shapes figure draws, each one with the program that produced the picture beside it.

## Categories, distributions and orders of magnitude

```go
src := figure.NewTable().
    String("region", []string{"north", "south", "east", "west"}).
    Float64("sales", []float64{18, 42, 31, 25})

p := figure.New(figure.Size(700, 400), figure.Title("Sales by region"))
p.X(scale.Ordinal())                        // equal slots, one per category
p.Y(scale.Linear(scale.Nice(), scale.Zero()))
p.Add(geom.Bar(src,
    geom.X("region"), geom.Y("sales"),
    geom.ColorBy("sales", scale.Sequential(palette.Viridis)),
))
```

An ordinal axis is a *band* scale: it tells the bar how wide to be, rather than
the bar guessing from the spacing of the data. The same applies to
`geom.Boxplot`. For data that spans decades, swap in `scale.Log(scale.LogNice())`
— or `scale.SymLog()` when it also crosses zero.

A runnable version, together with a boxplot over the same kind of data, is in
[`examples/categories`](../examples/categories).

## Series in one layer, stacked or side by side

A table with a series column is *one* layer, not N. `geom.GroupBy` splits it,
and the position adjustments are defined over the groups it makes:

```go
// quarter, product, revenue — twelve rows, one per (quarter, product) pair
p.X(scale.Ordinal())
p.Y(scale.Linear(scale.Nice(), scale.Zero()))
p.Add(geom.Bar(src,
    geom.X("quarter"), geom.Y("revenue"),
    geom.GroupBy("product"),
    geom.ColorBy("product", scale.Qualitative(palette.OkabeIto)),
))
```

A grouped bar stacks from the baseline up, because that is what a bar chart
with a series column means. `geom.Dodge(0.1)` puts the products side by side
instead, `geom.Stack(geom.StackFill)` makes it a 100 % chart, and
`geom.Stack(geom.StackWiggle)` over `geom.Area` is a streamgraph. The axis is
trained on what will be drawn rather than on the column, so a stacked axis
reaches the total; each segment is its own shape, so a pointer lands on the
segment and `Live.TrackRows` names the row behind it.

The legend names every series. One swatch per layer could not, which is why a
layer contributes as many entries as it has to
([ADR 0020](adr/0020-discrete-colour-and-multi-entry-legends.md)).

## A pie is a stacked bar in a different coordinate system

A scale maps a value into an interval; a **coord** decides what that interval
means. `coord.Cartesian` — the default — says it is a distance along an edge of
the plot. `coord.Polar` says one of the two intervals is an angle and the other
a radius, and the marks that were already there draw the family that was
missing:

```go
p := figure.New(
    figure.Coord(coord.Polar(coord.Theta(coord.FromY), coord.Hole(0.45))),
    figure.Theme(theme.Light.With(
        theme.Grid(false, false), theme.AxisLines(false, false), theme.Ticks(false, false))),
)
p.X(scale.Linear())          // one slot, filling the radius
p.Y(scale.Linear())          // the stacked total, filling the circle
p.Add(geom.Bar(src, geom.X("all"), geom.Y("share"),
    geom.GroupBy("browser"),
    geom.ColorBy("browser", scale.Qualitative(palette.OkabeIto))))
```

That is the whole of the donut above: the same `geom.Bar` layer that draws a
stacked bar chart in Cartesian, with θ taken from the Y axis instead. The ring
closes into a full circle because a stacked domain ends at the total — which is
why neither scale is niced — and the hole is where the radial scale starts, so
it is an annulus rather than a circle of background painted over the middle.

### A slice's radii are dimensions, and a slice can leave the ring

The donut above carries one number per slice: its share, which is the angle.
Two more are already there for the taking, because the radial axis is an axis
like any other. `geom.X` and `geom.X2` name a mark's two edges on it — the pair
a gantt bar uses — so a slice starts and stops where its row
says:

```go
p := figure.New(figure.Coord(coord.Pie(coord.Radius(0.95))), figure.Theme(bare))
p.X(scale.Linear(scale.Domain(0, 1)))   // the radius: 1 is the whole budget
p.Y(scale.Linear())                     // the angle: the stacked share
p.Add(geom.Bar(src,
    geom.X("floor"), geom.X2("used"),   // where the slice starts and stops
    geom.Y("share"),                    // how far round it goes
    geom.GroupBy("team"),
    geom.ExplodeBy("pull"),             // and which of them leaves the ring
    geom.ColorBy("team", scale.Qualitative(palette.OkabeIto))))
```

That is the figure above: three numbers per slice, one layer, no new mark.
`coord.Pie()` and `coord.Donut(f)` are sugar for the polar recipe and describe
themselves as the polar coord they are.

`geom.Explode(f)` breaks every mark of a layer out of the middle by a fraction
of the outer radius; `geom.ExplodeBy(col)` reads that fraction per row, which is
what pulls one slice out and leaves the rest where they were. It is a
displacement rather than a longer radius, and that is the whole point: the slice
still says what it said, the gap shows where it came from, and a pointer follows
it out — the path a geom hands the backend is the path that gets indexed, so a
hit in the gap finds nothing. A coord with no middle to move away from —
`coord.Cartesian` — ignores it rather than inventing a direction, which is why
every golden file in the repository is unchanged by an option every geom now
accepts ([ADR 0026](adr/0026-breaking-a-mark-out.md)).

A radar is `geom.Line` or `geom.Area` over an ordinal angular axis, with
`coord.Chord()` for sides that are straight and `geom.Closed(true)` for a
contour that comes back to the first axis. A rose, a wind rose and a gauge are
bars; a polar boxplot is a boxplot. None of them is a new geom, which is the
point of having a stage rather than a shape
([ADR 0018](adr/0018-coordinate-systems.md)).

The stage costs an existing chart nothing: `Cartesian` is the identity, and
every golden file and every figure in the gallery is unchanged by it. What it
does change is what a pointer can be told — a hit is inverted back through the
coord before the scales see it, so a pointer over a slice reports the value the
slice stands for rather than a pixel, and `Live.TrackRows` names the row.
Concentric rings replace horizontal grid lines and the tick labels go round the
outside; the coord reports that geometry and `render` still strokes it, because
`render` is the only package that knows the drawing order of a chart.

Decimation is deliberately off under a polar coord. `stat.LTTB` buckets by pixel
column and a bucket of equal angle is not a bucket of equal width, so the coord
reports that it does not decimate rather than have a reduction measure something
it was not designed for. Nothing polar is a big-data chart, so this costs
nothing real.

A runnable version of the donut, the radar, a wind rose, a gauge and the
broken-out donut above is in [`examples/polar`](../examples/polar).

## A Smith chart is a coordinate system too

Almost no general-purpose plotting library draws a Smith chart, because a Smith
chart is not a mark. It is a conformal map of the impedance half-plane onto the
unit disc — Γ = (z−1)/(z+1) — and a library whose coordinate stage is hard-coded
Cartesian cannot express it at any price. `figure`'s stage can, and `coord.Smith`
is the third one:

```go
p := figure.New(figure.Coord(coord.Smith()))

// The two columns are the normalised impedance: r = R/Z₀ and x = X/Z₀.
// TickValues asks for the grid a paper chart is printed at — six values every
// RF engineer expects in those places, which no tick-choosing algorithm
// produces because they are not evenly spaced and are not meant to be.
p.X(scale.Linear(scale.Domain(0, 50), scale.TickValues(0, 0.2, 0.5, 1, 2, 5)))
p.Y(scale.Linear(scale.Domain(-50, 50),
    scale.TickValues(-5, -2, -1, -0.5, -0.2, 0.2, 0.5, 1, 2, 5)))

p.Add(geom.Line(sweep, geom.X("r"), geom.Y("x")))
```

![A patch antenna's reflection swept across its band, on a Smith chart](images/smith.png)

There is no Smith geom and no Smith mark: that is a plain `geom.Line`. And
there is nothing drawing the grid, either — the constant-resistance circles are
what the X ticks look like once the coord has had them, and the
constant-reactance arcs are the Y ticks, so `render` draws this with the same two
loops it draws a Cartesian grid with. Not one line of `render/` changed for it;
see [ADR 0033](adr/0033-smith-charts.md), which argues why this is the same
seam polar is and not the wider one a map projection needs.

An instrument reports S₁₁ as a reflection coefficient rather than as an
impedance, so a measured sweep is one line at the call site:

```go
r[i], x[i] = coord.SmithZ(re[i], im[i])   // z = (1+Γ)/(1−Γ)
```

Three more things are worth knowing. **Both axes are linear**, and the domains
are pinned rather than trained: the chart's extent is the whole disc whatever
the data does, and a near-open reflection is a resistance in the thousands that
would otherwise drag every tick into the last pixel before the rim. **An edge is
a chord by default**, because a line between two measured samples asserting a
linear sweep in impedance is an assertion the instrument did not make;
`coord.SmithArc` draws the exact image for a locus that genuinely is straight in
impedance, which is what a matching network's steps are. And
**`coord.SmithAdmittance`** turns the disc through half a turn and reads the pair
as a conductance and a susceptance — the Y chart a shunt element is read on, and
the same physical reflection in the same place, against the other grid.

Not drawn: constant-|Γ| circles, constant-Q arcs and a combined ZY overlay. Each
is a third grid family, and a coord may draw one grid line per tick a scale
emits — the same constraint that makes the columns an impedance in the first
place.

A runnable version of the sweep above, a two-element matching network and the
admittance chart is in [`examples/smith`](../examples/smith).

## An edge table is a chart too

The last family of charts figure could not draw read neither a pair of axes nor
a summary of a column: they read a *relationship*. A treemap and a sunburst read
a hierarchy, `(id, parent, value)`; a sankey and a chord diagram read an edge
list, `(from, to, value)`. Both are ordinary columns, which is why `data.Source`
did not change to accommodate them.

```go
p := figure.New(figure.Size(700, 420), figure.Theme(bare))
p.X(scale.Linear())
p.Y(scale.Linear())
p.Add(geom.Treemap(src,
    geom.ID("path"), geom.Parent("under"), geom.Value("kb"),
    geom.Padding(0.006)))
```

![Disk usage by directory as a treemap](images/treemap.png)

Each mark lays its own geometry out in the unit square — a span across, a height
out — and hands it to the coordinate stage. Which means the polar half of this
family is not new drawing code at all. An icicle is a hierarchy's span across
and its depth out; wrapped round a circle, the root is at the middle and the
leaves are at the rim, and that is a **sunburst**:

```go
p := figure.New(figure.Size(520, 460), figure.Theme(bare),
    figure.Coord(coord.Polar(coord.Hole(0.12))))
p.X(scale.Linear())
p.Y(scale.Linear())
p.Add(geom.Icicle(src, geom.ID("path"), geom.Parent("under"), geom.Value("kb")))
```

![The same directory tree as a sunburst](images/sunburst.png)

It is `coord.Polar` and not `coord.Pie`, because a pie sweeps the *Y* axis round
the circle and this chart's Y is its depth.

A **sankey** reads the other shape. Nothing declares a node: a node exists
because a row mentioned it, it stands one column past the deepest source that
reaches it, and it is as thick as the greater of what enters and what leaves.

```go
p.Add(geom.Sankey(src, geom.From("from"), geom.To("to"), geom.Value("rps")))
```

![Requests per second through a service, as a sankey diagram](images/sankey.png)

And the same trick again: `geom.Arc` puts the nodes on a rail with the ribbons
rising off it, which is an arc diagram. Each band is as thick as what it
carries and arcs as high as it reaches, so the height reads as distance:

```go
p.Add(geom.Arc(src, geom.From("from"), geom.To("to"), geom.Value("rps")))
```

![The same traffic as an arc diagram](images/arc.png)

Move the rail to the rim and wrap it round a circle, and the ribbons cross the
middle — a **chord diagram**, from the same layer with one option and one coord
different.

```go
p := figure.New(figure.Size(520, 460), figure.Theme(bare),
    figure.Coord(coord.Polar()))
p.Add(geom.Arc(src, geom.From("from"), geom.To("to"), geom.Value("rps"),
    geom.Baseline(1)))
```

![The same traffic as a chord diagram](images/chord.png)

Four marks, six charts, and no second implementation of anything — the same
thing the coordinate stage bought for the pie, one bucket later. The layouts
themselves are pure functions in [`stat/`](../stat), each with a determinism test:
node order comes from the order the rows first named them and never from a map,
and the sankey's relaxation runs a fixed number of sweeps rather than to
convergence, so a chart whose panels are built on several goroutines draws
exactly what a serial one draws
([ADR 0039](adr/0039-relational-layouts.md)).

Both axes describe the unit square, which is nothing a reader needs to see — so
these charts want the same bare theme a pie does. A runnable version of all six
is in [`examples/relational`](../examples/relational).

## Boxes bounded by their own row

`geom.Rect` occupies an arbitrary `[x0,x1] × [y0,y1]` per row — the mark a bar
is not, because a bar always touches the baseline. An edge no column names is
the slot the axis implies, so a heatmap is a rect and a ramp:

```go
p.X(scale.Ordinal(scale.OrdinalPadding(0)))
p.Y(scale.Ordinal(scale.OrdinalPadding(0)))
p.Add(geom.Rect(src, geom.X("day"), geom.Y("hour"),
    geom.ColorBy("calls", scale.Sequential(palette.Viridis))))
```

and a gantt bar, which knows where it starts and stops, names both:

```go
p.X(scale.Time())
p.Y(scale.Ordinal())
p.Add(geom.Rect(src, geom.X("from"), geom.X2("to"), geom.Y("task")))
```

Candlestick, waterfall, waffle and calendar are the same mark with different
columns — see [docs/chart-types.md](chart-types.md).

A runnable version of all four charts is in [`examples/groups`](../examples/groups).

## Distributions

Seven marks that summarise a column rather than plotting it. Each is a pure
function in [`stat/`](../stat) with a determinism test, and each trains its axis on
the summary — a histogram's Y axis holds counts that appear nowhere in the
table, an ECDF's holds a fraction it computed
([ADR 0028](adr/0028-distribution-stats.md)).

```go
p.Add(geom.Histogram(src, geom.X("latency")))              // bins chosen by Freedman–Diaconis
p.Add(geom.Histogram(src, geom.X("latency"), geom.Bins(40), geom.BinRange(0, 500)))
```

A violin draws the shape a boxplot summarises away, one per slot and — given a
series column — one per series within it:

```go
p.X(scale.Ordinal())
p.Add(geom.Violin(src, geom.X("service"), geom.Y("latency"), geom.GroupBy("region")))
```

A ridgeline is the same estimate laid out down a categorical axis, overlapping
on purpose: twenty little density panels are twenty comparisons a reader has to
carry between them, and twenty ridges are one picture.

```go
p.Y(scale.Ordinal())
p.Add(geom.Ridgeline(src, geom.X("temperature"), geom.Y("month"), geom.Overlap(2)))
```

A swarm shows every observation and hides none of them, deterministically — no
jitter, so the same data draws the same picture on every machine and every
frame. An ECDF shows a distribution with no parameter in it at all, and takes a
series column so several can be compared without overplotting:

```go
p.Add(geom.Beeswarm(src, geom.X("cohort"), geom.Y("score")))
p.Add(geom.ECDF(src, geom.X("score"), geom.GroupBy("cohort")))
```

A hexbin is the third answer to overplotting, beside decimation and the density
raster: a hexagon has six neighbours all the same distance away, so a cloud
binned into one grows none of the crosses and seams a square grid does.

```go
p.Add(geom.Hexbin(src, geom.X("x"), geom.Y("y"), geom.DensityCells(8)))
```

And a trend line goes on top of a scatter — locally weighted by default, so it
follows the data rather than assuming a shape:

```go
p.Add(
    geom.Scatter(src, geom.X("x"), geom.Y("y")),
    geom.Trend(src, geom.X("x"), geom.Y("y"), geom.Span(0.4)),
    geom.Trend(src, geom.X("x"), geom.Y("y"), geom.Smooth(geom.LinearFit)),
)
```

All seven, over samples that make the point, are in
[`examples/distributions`](../examples/distributions).

## Bubbles: a third channel

`geom.SizeBy` gives every mark its size from a column. The scale maps by
**area** rather than by radius, because a reader compares two circles by how
much ink is in them — so a value twice another's is drawn with twice the ink and
√2 times the diameter, and the layer contributes a key of sample marks beside
the legend and the colourbar:

```go
p.Add(geom.Scatter(src,
    geom.X("gdp_per_capita"), geom.Y("life_expectancy"),
    geom.SizeBy("population", scale.Size()),
    geom.ColorBy("continent", scale.Qualitative(palette.OkabeIto)),
))
```

A sized layer draws circles rather than markers, and that is the IR's doing
rather than a preference: `ir.Backend.Markers` carries one style per drawing
call, so a per-row size would be a call per row. One path per colour with a
circle per subpath is one call per colour — and it gives a pointer the bubble it
is actually inside rather than the nearest centre
([ADR 0027](adr/0027-size-channel-and-the-guide-column.md)).

The chart is in [`examples/distributions`](../examples/distributions) too.

## Small multiples

```go
p := figure.New(figure.Size(900, 520), figure.Title("Throughput by region"))
p.Add(
    geom.Line(src, geom.X("hour"), geom.Y("rps"), geom.Label("throughput")),
    geom.HLine(60, geom.Label("target")),          // no data: drawn on every panel
)
p.Facet(facet.Wrap("region", facet.Columns(3)))
```

Panels share their scales by default, which is what makes small multiples
comparable at a glance. `facet.FreeX`, `facet.FreeY` and `facet.Free` give each
panel its own — a deliberate choice, because a reader who does not notice the
axes changed will read the panels as comparable when they are not.

For unrelated charts on one canvas, build a grid of plots instead:

```go
g := figure.NewGrid(2, figure.GridSize(900, 560), figure.GridTitle("Fleet"))
g.Add(latency, throughput, errors, saturation)
err := g.Render(figure.PDF("overview.pdf"))
```

A runnable version of both, with annotations and PDF output, is in
[`examples/dashboard`](../examples/dashboard).

## A band at the edge, on the same axis

Some of what a chart shows is not on its other axis at all: a strip of machine
states under a speed trace, a rug of event times, a ribbon of shifts, a key or
a marginal distribution beside the panel. A track is a band at an edge of the
plot area that shares the axis it runs along and carries a scale of its own
across it — an ordinal one under a linear panel, which is the case it exists
for.

```go
p := figure.New(figure.Size(900, 480), figure.Title("Line 3"))
p.X(scale.Time()).Y(scale.Linear(scale.Zero()))
p.Add(geom.Line(speed, geom.X("t"), geom.Y("speed")))

p.Track(figure.Bottom, figure.TrackSize(48)).
    Add(geom.Rect(states, geom.X("start"), geom.X2("end"), geom.Y("state"),
        geom.ColorBy("state", scale.Qualitative(palette.Default))))
```

`figure.Bottom` and `figure.Top` are grid rows and share the plot's X;
`figure.Left` and `figure.Right` are grid columns and share its Y. Bands on
two edges at once are fine — the corner between them is simply empty.

The thickness comes out of the panel, not out of the panel's domain: the axis
the track does not share is identical with the track and without it, so
`scale.Zero()` still means what it says. The axis it *does* share is trained by
both, because it is one axis — a rug of event times widens the time axis to
cover the events, which is the reason to draw them against it.

The track and the panel hold the *same* scale object for that axis, so a zoom
is one zoom rather than two that agree, and a pointer over a state bar reports
its layer and its row like any other mark. Its lanes do not zoom, because half
a category is not a view of anything.

For the same shape across separate plots, stack them in a one-column grid on
one scale object — `figure.GridRowHeights(0, 48)` makes the second row a strip
and `figure.GridSharedX(true)` writes the time axis once, under the bottom
row, with `GridColWidths` and `GridSharedY` the same turned a quarter turn.
That path renders; interaction is what a track is for.

## Label placement and QQ plots

```go
// The renderer places participating point labels, dropping those that still
// collide after trying nearby positions. Labels in boxes are never moved.
p.Add(geom.Text(src, geom.X("x"), geom.Y("y"), geom.TextBy("name"),
    geom.AvoidOverlap(true)))

// X names the sample column. Display axes are theoretical normal quantiles
// horizontally and ordered observations vertically, without standardisation.
q := figure.New(figure.XTitle("Standard normal quantile"),
    figure.YTitle("Observed value"))
q.Add(geom.QQ(src, geom.X("value")))
```

`GroupBy` compares several samples; facets split them as usual. Other
theoretical distributions use `stat.QQ(sorted, quantile)` and `geom.Scatter`.
See the runnable [diagnostics example](../examples/diagnostics),
[label placement decision](adr/0040-label-collision-avoidance.md) and
[QQ decision](adr/0041-qq-plots.md).

| Normal QQ plot | Label placement |
|---|---|
| ![Ordered observations against normal quantiles](images/qq.png) | ![Nearby labels placed without overlapping one another](images/label-placement.png) |

---

**[README](../README.md)** · **[CONCEPT](../CONCEPT.md)** · **[ADRs](adr)** · [The gallery](gallery.md) · [Interaction](interaction.md) · [A million rows](scale-out.md) · [Reading a chart](reading.md) · [JSON and Arrow](spec.md) · [Features](features.md) · [Chart-type catalogue](chart-types.md) · [Benchmarks](benchmarks.md) · [How it was built](milestones.md)

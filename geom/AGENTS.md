# Working in `geom`

Notes for changing `geom`, moved here from the root [AGENTS.md](../AGENTS.md),
which holds the rules that apply everywhere — read that one first. This file
covers device-space exceptions, batching, decimation, row reporting, stacks and
groups, the scratch pool, distribution marks, text layers, relational layouts —
and what the marks deliberately do not do.

## Things that will bite

**A dependency arrow is routed in device space, a curve label is placed there,
and a nearest-neighbour partition is cut there; those three are the whole
list.** Everywhere else a geom computes a
midpoint, a corner or a staircase step *before* the coord, because those are
statements about the data — `geom.stepColumns` is the canonical case.
`geom.Depends` puts its two *ends* through `coord.Point` like every other mark
and then joins them with an elbow whose corners are device points and whose stub is in pixels, because there is
nothing in data space between the finish of one task and the start of another:
the route is a reading aid rather than a claim about any value in between. The
direction out of each bar is **measured** from its own two edges rather than
assumed, which is what makes a reversed axis turn both ends together and what
makes one routing function serve a gantt drawn down the page as well as across
it. See [ADR 0068](../docs/adr/0068-gantt-charts.md).

The second exception is `geom.LabelLevels`, whose argument is sharper. A label
on a contour is turned to the curve's **tangent**, and a tangent in data space
is not the tangent on screen: under `coord.Polar` and `coord.Smith` a curve that
is straight in the scaled pair is bent on the panel, so an angle chosen before
the coord would write the label across its own line on exactly the two coords
the feature exists for. `geom.curveLabeller` therefore reads the device points
the mark is about to stroke and nothing else — which is also why one helper
serves a traced lattice and a formula. See
[ADR 0073](../docs/adr/0073-labels-on-a-curve.md).

The third is `geom.Voronoi`, and its argument is the treemap's rather than
either of those. A cell's whole content is that its boundary is **halfway
between two dots**, which needs a length — and the two axes of a chart are not
obliged to share one: there is no distance between a millimetre of rain and a
kilometre of easting. Cutting the partition in the scaled pair and stretching
it into the panel is an anisotropic map, which takes perpendicular bisectors to
lines that are not perpendicular bisectors, so every boundary on screen would
be halfway between nothing. The sites go through the scales and `coord.Point`
like every other mark's positions; only the partition between them is device
geometry, which is why `stat.Voronoi` runs in `Build` against `Frame.Area` —
the fourth stat to do so, beside the hexagonal lattice, the beeswarm's offsets
and the treemap's squarify. The honest cost is that the picture depends on the
panel: a colourbar narrows the plot area and the boundaries move. See
[ADR 0080](../docs/adr/0080-nearest-neighbour-cells.md).

**A progress layer paints twice over, and that is what keeps the batching.**
`geom.ProgressBy` draws every cell at the unfinished alpha and the finished
part of every cell at full strength on top, in two passes — not one composite
shape per cell. `geom.groupByRect` batches by colour, and a shape per cell
would be a drawing call per cell and a per-mark colour in the IR, which is what
[ADR 0007](../docs/adr/0007-per-mark-colour.md) refuses. The fraction grows from
the edge the *row* named first, which is why `geom.edgesOn` exists beside
`spanOn`: the two return one pair of edges in the order the data has them and
in the order the screen has them, and filling from the screen's side would say
a task on a reversed axis had started at the end.

**A layer may hold data its `Source` does not.** `geom.Depends` reads a second
table, and `geom.Desc.Links`, `spec.Layer.Links` and
`a11y`'s row count all had to learn it. Three things there are load-bearing:
the link table is **never hoisted** in a document, because two dependency
layers over one plan are two sets of constraints; `Subset` cuts the *task*
table and leaves the links alone, so a constraint whose other end is in the
next panel resolves to nothing and is dropped; and the layer reports **no
rows**, because the row it would report belongs to the table `Source` does not
hand out.

**A geom must not assume a scale accepts every finite value.** A log scale has
no position for zero and returns NaN from `Map`. Geoms ask `scale.Definite`
through `geom.defined` and treat such a row as missing, so a NaN coordinate
never reaches a backend — [ADR 0008](../docs/adr/0008-categorical-axes.md). Adding
a geom means computing `scratch.plottable` once and using it, not re-testing
`finite` per traversal: the traversals have to agree about where the holes are.

**Per-mark colour batches; it does not widen the IR.** `geom.groupByColor`
emits one drawing call per distinct colour. Adding a per-vertex colour channel
to `ir.Backend` is the thing that record exists to refuse —
[ADR 0007](../docs/adr/0007-per-mark-colour.md).

**Decimation happens in `Build`, never in `Train`.** A geom trains on every row
and reduces only when it draws, so an axis reports the data rather than the
subset that survived — [ADR 0011](../docs/adr/0011-decimation.md). Reducing in
`Train`, or in the data layer, would make a chart's axis depend on how wide the
chart is, and there is a test that catches it.

**The reduction runs on device coordinates.** A pixel column is the unit the
whole exercise is about, and on a log axis equal steps in data are not equal
steps on screen. `stat.LTTB` and `stat.MinMax` are generic over `float32` and
`float64` precisely so the device-space path costs no conversion.

**A geom that holds data implements `geom.Faceter`; one that does not, must
not.** Faceting splits the layers that have rows and replicates the ones that
do not, which is why an `HLine` appears on every panel. Adding `Source`/`Subset`
to an annotation would make a threshold vanish from every panel but the one its
value happens to fall in.

**A geom reports its rows separately from what it draws, and that is the
whole point.** `geom.Rows` takes the positions a row landed at, not the points
of a drawing call — a smoothed line is a Bézier path whose control points are
not measurements, a staircase draws two points per row, a bar is four corners
around one value. Attributing rows to a call's points would attribute them to
whichever encoding the geom happened to use, and would be wrong for three of
the six geoms that have rows. There is a test per geom.

**Row reporting is gated on `Frame.Rows != nil` at every step.** `acquire(f)`
records it on the scratch as `wantRows`, and `rowsOf`, `sourceRows` and the
interpolated series' row list all check it. An ordinary render must keep
costing exactly what it did — `BenchmarkFrame1k` did not move when row
identity landed (76 allocations then; the current count is in
[docs/benchmarks.md](../docs/benchmarks.md)), and `BenchmarkWatchedFrame`
against `BenchmarkWatchedFrameRows` is pinned flat.

**A row is reported in the caller's table, not in the cut figure made.**
Faceting cuts each layer with `data.Rows`, and a row number relative to that
cut is a row number in a table nobody holds. `series.origin` comes from
`data.Origins` and `series.rowAt` resolves through it. A geom that collects its
own element list — a scatter, a bar, both for per-mark colour — must report
through `scratch.sourceRows` rather than handing over the element numbers: the
two lists look identical and mean different things, and a faceted chart is
where that shows.

**An error bar's orientation is its encoding, and its bounds are derived in
`Train`.** `Y2`/`ErrorBy` runs it vertically and `X2`/`ErrorXBy` horizontally —
the rule `Rect` already follows about its edges, which is why there is no
orientation option to contradict — and naming both is `ErrBothAxes` rather than
a guess that depends on option order ([ADR 0036](../docs/adr/0036-error-bars.md)).
The bounds are computed in `Train` because the axis has to describe them: an
interval whose top runs off the plot is the reading the chart was opened for,
which is the same boundary ADR 0019 draws for a stack's totals. `errorGeom.half`
is measured once per `Train` out of a buffer the layer keeps — `smallestGap`
sorts, and asking it per row is what made a bar layer quadratic once already.
And every segment goes through the coord: a cap drawn as two device points
would be a straight line under a polar coord, where the mark is an arc.

**Redundant encoding is a default, not an override.** `theme.Redundant` fills in
a dash and a marker ladder, and `config.dashFor`/`markerFor` use them only when
the layer set neither `geom.Dash` nor `geom.Shape`. That is why `config` carries
`markerSet` beside `dashSet` and why `geom.Desc` carries `MarkerSet`: a circle
is both the zero value and a shape somebody may have asked for, and without the
flag a round trip through the spec turns an unset shape into a pinned one.

**A position adjustment is derived in `Train` and drawn in `Build`, and both
halves are forced.** `render.Draw` trains every layer before it measures
anything, because tick labels need a domain — so a stacked bar's Y scale has to
be trained on the *totals*, or the tallest stack runs off the top of an axis
that describes rows nobody can see. `geom.groups.train` therefore computes the
per-row `(lo, hi)` pair, and `Build` only maps it through the scales. This is
the same boundary ADR 0011 draws for decimation, drawn for the same reason: what
a chart's axis says must not depend on how wide the chart is. See
[ADR 0019](../docs/adr/0019-position-adjustments.md).

**A stacked layer's holes are decided once, and both traversals read that
answer.** `groups.ok` is computed in `Train` and an unplottable row gets `NaN`
bounds, so the cumulative sum and the drawing traversal agree about where the
holes are. A NaN the sum skipped but the draw did not would shift every segment
above it — there is a test, and it compares a table with a hole against the same
table with the row removed.

**A grouped layer's buffers live on the layer, not in the frame's pool.** The
group index, the row lists and the derived bounds are refilled by every `Train`
out of memory the geom keeps: `Train` runs as often as `Build` does, and a chart
redrawn over a live table would otherwise allocate its group index per frame.
`TestAStackedLayerDoesNotAllocatePerPoint` is what holds that. Note that the
rows of each group are listed once, in `groups.split`, rather than found by
scanning the table per group — the scan is what makes a chart with many series
quadratic.

**There is exactly one scratch per `Build`, and a helper takes the caller's
rather than acquiring its own.** Two scratches held at once put two objects with
*disjoint* buffers into one pool; whichever comes back in the other's role next
frame has to grow the other's buffers, which is a per-row allocation on the path
whose whole point is not having one. `geom.eachGroup` therefore takes a
`*scratch` parameter. This shipped broken for exactly one commit and cost a
grouped frame 127 kB and seven allocations it did not need; nothing failed
except the allocation gate, which is what the gate is for.

**Group order is order of first appearance, and `geom.Order` is the only thing
that changes it.** Map iteration order is not an order; ADR 0012 requires a
parallel render to be byte-identical to a serial one, and `scale.Qualitative`,
`geom.groupByColor` and `boxplot.summarise` all already establish the
convention.

**A grouped `Bar` and `Area` stack by default; nothing else does.** That is
`config.stackFor`, and it is why `geom.Desc` carries `Stack` *and* `StackSet` —
`NoStack` is both the zero value and an adjustment somebody may have asked for,
exactly as `Dash`/`DashSet` and `Marker`/`MarkerSet` already are. Without the
flag, a round trip through the spec turns a grouped bar's default into a pinned
"do not stack", and the chart silently changes.

**Which guide a layer contributes follows from the kind of colour scale it was
handed.** `scale.Qualitative` is a `ColorScale` — it rides that interface the way
`scale.Categorical` rides `Scale` — so `geom.ColorBy` takes either kind, and
`config.colorGuide` returns false for a discrete one while `config.legends`
returns an entry per category. A colourbar over eight categories would be a ramp
through colours nothing is painted with.

**A layer that implements `geom.Legender` is not asked for `Legend` as well.**
`geom.Legends` prefers the list; `geom.LegendsOr` is the fallback written once,
and a geom that answers `Legends` without it silently loses its single entry.
There is a test — it was the first thing that broke when the interface landed.

**A geom works in mapped space and lets the coord place the point.** What
`scale.Map` returns is a position in an interval, which under Cartesian happens
to be a device coordinate and under Polar is an angle or a radius. So a geom
that computes a midpoint, a corner or a staircase step computes it *before* the
coord, never after: `geom.stepColumns` builds the staircase out of the mapped
columns for exactly this reason, because the corner of a step is a statement
about the data and not about where two device points happened to land.

**A run's displacement buffer is tested by length, not against nil.** The
batching runs come back from the scratch pool emptied rather than cleared away,
so a layer that breaks nothing out is handed a non-nil `offs` with nothing in
it. `geom.offsetAt` therefore compares `i >= len(offs)`. Testing for nil there
compiles, passes every test with a fresh pool, and panics on the second frame of
a chart drawn after a broken-out one.

**A bar's slot is measured once per Train, out of a buffer the layer keeps.**
Two mistakes lived in one line here, and both are worth recognising again.
`smallestGap` sorts, and `barGeom.halfWidth` asked it for the answer *per row* —
which made a bar layer quadratic in its rows: a ring of sixteen thousand slices
spent 2.5 seconds a frame, of which 64 % was that sort. And `smallestGap`
allocated the copy it sorted, so even once per `Train` it was a copy of the
column **per frame** — 800 kB of the 810 kB a hundred-thousand-slice frame
allocated, which is also enough garbage to drive the collections that empty the
scratch pool. It is `barGeom.gap` now, measured in `Train` out of `barGeom.gaps`
— `smallestGap(buf, vs)` hands the buffer back — and `Rect` and `Boxplot` keep
one each for the same reason. The buffer lives on the layer rather than in the
frame's pool because `Train` runs outside a `Build`, where there is no scratch
to take; that is the same argument the group index already makes.

**A distribution stat runs in `Train`; a device-space one runs in `Build`.**
ADR 0011 puts decimation in `Build` on the rule that what a chart's axis reports
must not depend on how wide the chart is. A histogram, a violin, an ECDF and a
trend are the same rule pointing the other way: their output *is* what the axis
has to describe — counts, a fraction, a fit — so they are computed in `Train`
and the scales are trained on the answer. Two marks are the exception because
what they compute is a length on screen: `geom.Hexbin` bins over the plot
rectangle, and `geom.Beeswarm` places its marks against a marker diameter. See
[ADR 0028](../docs/adr/0028-distribution-stats.md).

**A hexbin has no colourbar, and that is the price of regular hexagons.** Its
counts are not known until the plot rectangle is, and the guide column is
measured before that. Adding one means either binning in data space — cells that
are hexagons in the data and something else on screen — or measuring the guides
twice. Both are worse than shading from a faded colour to the full one.

**A sized layer draws circles, not markers, and that is the IR's doing.**
`ir.Backend.Markers` carries one `MarkerStyle` per call, so a per-row size would
be a drawing call per row — the cost ADR 0007 refused for colour. `geom.SizeBy`
therefore emits one path per colour with a circle per subpath, which also gives
a pointer the bubble it is inside rather than the nearest centre. Do not
"restore" marker shapes there: it would be a call per row again.

**A histogram ignores `GroupBy` on purpose.** Two overlapping histograms hide
each other exactly where the comparison is. `Violin`, `Ridgeline` and `ECDF` are
the three marks that answer that question without overplotting, and each takes
the series column. Do not "fix" it by stacking or dodging bins.

**A text layer measures while it builds, and that is the one call it may
make.** `geom.Text` asks `ir.Backend.Measure` how wide each label is, because a
label that overruns its box reads as belonging to the neighbouring row and only
the layer knows both. `render.syncMeasurer` serialises those calls onto one font
stack: panels build on separate goroutines, and two of them disagreeing about
how wide a label is would draw different text in the same box. Measure is the
*only* backend call a geom may make during `Build`; anything else belongs in the
drawing calls it emits.

**A truncated label is cached per row, and that is what keeps the text path off
the allocation gate.** Cutting a label builds a string, which is the one place a
per-row allocation could hide behind work that has to happen anyway.
`textGeom.remember` hands back last frame's string when the cut has not moved, so
a chart redrawn at the same size builds none — `BenchmarkLabelled1k` against
`BenchmarkLabelled10k` and `TestALabelledRenderDoesNotAllocatePerPoint` are what
hold it. The cache lives on the layer rather than in the scratch pool because it
has to survive a `Build`, the same argument `barGeom.gaps` makes.

**A text layer's box is the box `Rect` would draw, and its anchor is clamped to
what is visible.** The same options handed to both marks label the rectangles
exactly, which is why the slot rule for an unnamed edge is `spanOn` rather than
anything of its own. The clamp is against `coord.Extent` rather than the plot
rectangle — that is the interval the scales map into, so it means something
under a polar coord too — and it is what puts the label in the middle of a bar
that is half scrolled off the edge rather than off-screen with the box's true
centre. The room a label has is measured as the chord between the box's mapped
edges, because under polar the box's width is an angle and what a label needs is
a length.

**`geom.Align` records having been told, for the reason `Dash` and `Shape` do.**
The start of a run on the baseline is the zero value *and* an alignment somebody
may have asked for, and a text layer centres a label in its box when nobody has.
Without `config.alignSet` and `Desc.AlignSet` a round trip through the spec turns
that default into a pinned left edge and the chart silently changes.

### A relational layout is in the unit square, and the two coords disagree about Y

`Treemap`, `Icicle`, `Sankey` and `Arc` all put their span on X in `[0, 1]` and
their height on Y in `[0, 1]`, and both scales are trained on exactly that. Four
things about that will bite.

**The rim is `y = 1` and the hub is `y = 0`, and the choice is not free.** A
Cartesian panel flips Y — `y.SetRange(area.Max.Y, area.Min.Y)` in
`coord/coord.go` — so `y = 1` is the top of the plot. A polar one does not —
`rad.SetRange(q.r0, q.r1)` in `coord/polar.go` — so `y = 1` is the outer rim.
`rim = y1` is the one convention that makes a sunburst and a chord diagram work
with one pair of scales. It also means the Cartesian readings are unusually
oriented: an icicle grows upward from a root along the bottom. That is the
price, and it is written down in
[ADR 0039](../docs/adr/0039-relational-layouts.md) rather than worked around.

**A recipe wants `coord.Polar()` and never `coord.Pie()`.** `Pie` is
`Polar(Theta(FromY))`, which sweeps the *Y* axis round the circle. These marks'
Y is their depth, so a sunburst under `Pie` draws concentric wedges of the depth
axis — which is nonsense that renders without erroring.

**The interning is in the geom, not in `stat`.** `stat` takes `[]int` node ids
and never sees a name. Which node is "first" decides everything downstream —
the column a sankey's node stands in, the way round a chord diagram goes, the
palette entry each node takes — and that order has to come from the table rather
than from a map ([ADR 0012](../docs/adr/0012-parallel-panels.md)). The map in
`geom/relational.go` is only ever asked *whether* it has seen a name, never what
it holds, and it is `clear`ed rather than replaced so the buckets survive a
frame. A map made per `Train` is the allocation the gate exists to catch.

**A treemap squarifies in `Build`, against the panel.** It optimises an aspect
ratio *on screen*, so packing the unit square and stretching the result into a
wide panel defeats the algorithm. It is the third instance of
[ADR 0028](../docs/adr/0028-distribution-stats.md)'s exception, after the hexagonal
lattice and the beeswarm. It walks the tree breadth-first out of a pooled queue
rather than recursing, and writes only to the scratch — `Build` has to be safe
to run concurrently with itself.

**`coord.Edge` takes device points.** Handing it the values the scales produced
asks a polar coord to read an angle as an abscissa, and it draws a shape nobody
can recognise — which is exactly the bug the chord diagram's first golden file
caught and no unit test did. `geom.edgeAlong` is the shared spelling, and it
splits a wide span because `Edge` joins two points the *short* way round.

**A stroked outline hit-tests above the shape it outlines.** `interact` ranks a
vertex above an area, so bordering a treemap cell would make every hover report
a corner. That is why the separation between cells is `geom.Padding` — room —
and not ink, and why these marks stroke only when the caller named both a
`Fill` and a `Color`. `geom.Rect` has the same rule for the same reason.

## Deliberately not done

Deliberately not done. There is **no force simulation** — not because one
cannot be made repeatable, which a fixed start and a fixed budget do, but
because it descends no objective, so what stopping it early costs cannot be
said. The chart it was once refused for is drawn: `geom.NodeLink` places nodes
by stress majorization, a descent that never goes uphill, stopped after a
constant number of sweeps — so
[ADR 0012](../docs/adr/0012-parallel-panels.md) is satisfied the way every other
layout here satisfies it ([ADR 0077](../docs/adr/0077-a-node-link-layout.md)).
Two things there are load-bearing and were wrong once: a sweep writes each
node's position back **before the next node reads it**, because the
simultaneous form is not a descent at all and cycles; and the starting
directions are chosen by eigenvalue **value** rather than magnitude, because
graph distances are often not Euclidean and the biggest by magnitude can be
negative. It refuses a graph of more
than `stat.MaxStressNodes` nodes rather than drawing a hairball. There is **no area-proportional Venn and no Venn of four sets** — the
first is a construction at two sets and an optimiser from three on, where seven
region areas have six free numbers to hit them with; the second is refused
because this mark writes each count *into* its region at one type size in a
fixed arrangement, and a four-set diagram has regions that hold nothing under
those conditions — a decision about this mark's promise rather than a fact about
four sets, and not because no fixed arrangement of four exists.
`geom.Venn` draws the two- and three-set diagram, whose positions are a table
rather than a solution, and `geom.Intersections` is the chart that keeps working
past three ([ADR 0074](../docs/adr/0074-sets-are-counted.md) and its
[amendment](../docs/adr/0074-sets-are-counted.md#amendment-two-refusals-restated)).
**An UpSet plot is a count and not a layout**, which is why it is three ordinary
marks and a track rather than anything new below `geom` — and why the set-size
bars beside the matrix are a mark reading that same count rather than arithmetic
repeated in the caller's code, though the panel they stand in is still the
caller's ([ADR 0076](../docs/adr/0076-the-other-half-of-the-count.md)). A
relational mark
**ignores `geom.SizeBy`**, because a size per node is
meaningless when the value already is the size. A **sankey does not reorder its
nodes to reduce crossings**: that means a sort per sweep, and a sort is where a
layout stops being a pure function of its input and starts depending on how a
tie was broken — `geom.Order` is how a caller asks for a different order, by
sorting its own rows. A **treemap draws its leaves only**, because an internal
node's rectangle is the union of its children's and the nesting shows as
padding. A **parallel-sets diagram counts complete rows only**: a row missing a
category anywhere is counted nowhere, which is the one place in this library
where an absent value costs a row rather than gapping what it is part of —
each of its columns is a partition of the same total, and a count that skipped
only the crossings beside the gap would leave one column adding up to less than
the next, after which no two thicknesses in the diagram mean the same thing.
Its ribbons also **report no row**, because a ribbon is a count over rows and a
count is not a row; and its boxes are **not painted from the colour column's
scale**, because asking a discrete scale for a box's name is what registers
that name, and the legend beside it is naming classes
([ADR 0079](../docs/adr/0079-parallel-sets.md)). And a **hit on one of these reports the layout's own coordinates**
rather than anything in the table: for these marks the row is the reading, which
is the first time that has been true and is
[ADR 0015](../docs/adr/0015-hit-testing.md)'s revisit clause rather than this
record's.

Deliberately not done. A **nearest-neighbour partition is none of the above and
sits at the other end of the same question**: its rows have coordinates, so
`geom.Voronoi` reads a pair of axes, and each cell *is* one row and reports it.
It **draws no dots** — a `geom.Scatter` layer over the same table does that —
and **no labels**; it **ignores `GroupBy`**, because there is one partition
over every row of the layer and a series inside it partitions nothing; and it
gives **one of two rows at the same point no cell at all**, because the
bisector between them is undefined and two cells drawn on top of each other are
two marks nobody can point at separately. It **outlines a cell only when both a
`Fill` and a `Color` are named**, which is `geom.Rect`'s rule for `interact`'s
reason, and the treemap's padding is not an alternative here: a cell that had
been inset would no longer be the set of points nearest its row, which is the
only thing the mark asserts. There is **no Lloyd relaxation, no weighted or
power diagram and no Voronoi treemap** — each is the optimiser
[ADR 0039](../docs/adr/0039-relational-layouts.md) refuses — and **no Delaunay
triangulation**, because a half-plane intersection does not produce one and an
interpolated surface over scattered samples is a claim about the values
*between* the rows where a cell is a claim about which row is nearest
([ADR 0080](../docs/adr/0080-nearest-neighbour-cells.md)).

Deliberately not done. A **hexbin has no colourbar**, for the
ordering reason above. A **histogram ignores `GroupBy`**. A **sized layer draws
circles** and ignores `geom.Shape`: a marker ladder is a redundant encoding for
telling series apart, and a bubble cloud is one series. A **violin's widths are
normalised against the widest density in the layer**, so the violins are compared
by shape rather than by sample size — `geom.Bandwidth` is what makes two groups
strictly comparable, and without it each group is smoothed by its own spread.
There is **no contour and no QQ plot**: the first needs a marching-squares stat
that nothing else shares, and the second is an ECDF against a theoretical
quantile function, which is a distribution library rather than a chart. And
`geom.Ridgeline` is the **first geom that refuses a scale outright** — it errors
on a continuous Y axis rather than drawing every ridge on top of the last.

Deliberately not done. There is no `geom.Slice` and
there are no `Inner`/`Outer` channels: `Geom.Train` is handed the two scales and
no coord, so a layer cannot know at training time which of them is the radius —
a channel that only exists under one coord would be the pie geom this project
does not have. The break-out is **per row rather than per group**, because that
is what a channel means everywhere else; a donut has one row per slice, so the
distinction only shows on a stack of several rows per series, whose segments
move independently. Only `Bar` and `Rect` honour it, because they are the marks
that draw an annular sector — a broken-out line has no ring to leave. And a
layer that asks for a break-out under a coord that cannot answer draws what it
drew, silently: an error would make every Cartesian chart's option list
conditional on a coord chosen somewhere else.

Deliberately not done. A grouped **line, step and scatter do not
stack**: two series drawn over one another are two readings, and adding them
would invent a third nobody measured. Stacking accumulates in group order, so a
stack of mixed signs runs each segment from where the last one ended rather than
splitting into a positive and a negative half. `geom.WidthBy` gives a bar its
width from a column and does **not** reposition the slots — a marimekko is one
layer whose X column already holds each column's centre, and unequal slots that
label themselves are an axis question rather than an adjustment one. A group
index is per layer, so a facet whose panels hold different groups needs an
explicit `scale.Qualitative` for a series to keep its colour across panels. And
a group column that is numeric or temporal is formatted into a label per row,
which is a cost of naming a category with a number rather than of grouping: the
allocation gate uses a text column, which is what a series column is.

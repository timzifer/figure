# 0078 — A coord may carry more than two axes, and it holds their scales

**Status:** Accepted · **Date:** 2026-09-21 · **Implemented:** 2026-09-21

## Context

[docs/chart-types.md](../chart-types.md) has called parallel coordinates **"the
widest genuine gap on the page"** since the sweep of the unusual forms was
written, and ranked it first among what is unbuilt: *"reaches the most readers
and needs the most argument."* Bucket D names the shape of the answer and the
thing that stopped it:

> Parallel coordinates needs no new channel but does need **per-axis scales
> inside one panel**, which makes it a near-relative of radar: both draw their
> own axes inside the plot area. Radar got its axes from `coord.Polar`'s
> furniture in the coordinate stage… and parallel coordinates would want the
> same shape of answer from a coord of its own.

A radar gets away with one scale because its spokes are one quantity measured
several times. A parallel-coordinates plot exists precisely because its axes
are *different* quantities — miles per gallon beside kilograms — and neither of
them can be squashed into the other's range without the chart saying something
false.

Half the machinery arrived while nobody was looking.
[ADR 0070](0070-a-third-labelled-family.md) gave a coord `Furniture.Families`:
ladders with their own lines, their own label positions and — the part that had
blocked every third ladder — **their own label text**. That is "N axes, each
labelled in its own units, with no third tick list in `render`". And 0070 wrote
down the rule this record follows: *"A coord that wants its families configured
configures the coord."*

## Decision

**`coord.Parallel` gives a panel one vertical axis per dimension and holds a
scale for each, and `geom.Parallel` draws one line per row across them.** Four
claims.

### 1. The axes belong to the coord, and it is the first coord to hold scales

Every other coord in the package borrows the panel's two scales and re-ranges
them — `smith.identityRange` and `ternary.pin` both do it. This one carries its
dimensions, because that is what a parallel panel *is*: N axes with N domains,
and two of them would be a Cartesian panel.

`Frame` ranges each dimension onto the unit interval, so what reaches `Point`
is a fraction of an axis rather than a value in somebody's units, and pins the
panel's own X and Y to say what they now mean: which axis, and how far up it.
It is `ternary.pin`'s move made N times.

A dimension is a **label and a scale and never a column name**. Which columns a
chart draws is the mark's business, and a coord that knew what a table was
would be a coord in the wrong package — so the mark's `geom.Dims` and the
coord's dimensions are matched **in order**, and a count mismatch is
`geom.ErrDimensions` out of `Train` rather than a chart with an axis nothing is
drawn against.

### 2. The axes are furniture, and `render` learned one thing

Each dimension raises one `Family`: the axis line, a mark at every level, the
level labels in that dimension's own units, and the dimension's name inside the
panel at the top. `render` strokes family lines with the grid and writes family
labels with the tick font — code that has been there since 0070 — so **nothing
in `render` knows how many axes a panel has**, and it never will.

Three things follow, and the second of them is a rule of 0070's that had to be
amended:

- **The panel's own ticks are silenced by the coord**, which marks every one of
  them as outside the panel: left to themselves they would number an axis index
  and a fraction of an axis. A chart that also turns them off in its theme gets
  back the gutter a panel reserves for tick labels it never writes — about a
  tenth of the canvas — and one that leaves the theme alone still draws no
  number it cannot explain.
- **A family that is the axis keeps its labels through `theme.Ticks(false,
  false)`**, which is [ADR 0070](0070-a-third-labelled-family.md)'s one rule
  this record had to amend. That record let the theme's per-axis tick switches
  govern family labels while refusing to let the per-axis *grid* switches
  govern family lines; for a coord whose own two axes carry nothing worth
  numbering, the first of those turns a chart into an unlabelled one. So
  `Furniture.FamiliesAreTheAxes` is a fourth flag beside `XLabelsShareARow`,
  `AxesOverData` and `LabelsYFirst`, the coord sets it, and a ternary chart —
  whose family is a third reading beside two labelled axes — does not and is
  unchanged.
- **The labels sit inside the panel.** A gutter is a side of the plot and there
  are two of those; there is nowhere outside to put N ladders. Each axis
  carries its numbers against itself, to its right except for the last, and the
  coord reports `AxesOverData` so they are written over the lines rather than
  under them — the polar coord's answer for a radial axis that runs through its
  own ring.

### 3. `geom.Training` gains `Dims`, and that is the whole widening of `geom`

`Training` exists to be widened — *"a chart can gain a dimension — a depth
scale, a second radial scale, an axis of time — and a struct with exported
fields gains a field where a method signature cannot"* — and `Z` is the
precedent for a field only some callers fill. `render` fills `Dims` from the
panel's coord through `coord.Dimensions`, a new optional interface beside
`Exploder` and `Fixed`, in the one line beside the `Train` call it already had.
At `Build` time the mark reads the same scales off `f.Coords()`, so `geom.Frame`
is unchanged.

### 4. A row is a line, and it is reported at every axis it crosses

So a pointer anywhere along a line finds that row. `MarkRows` carries the same
row index once per vertex, which is the extruded bar's precedent — a row
reported once per face, with `Index.Locate` returning the last position. A row
missing a value on one axis **gaps** there rather than being dropped or joined
across: a line drawn straight through the gap would assert a value nobody
measured, which is the missing-data policy every layer has followed since v0.1.

Lines are batched by colour with one subpath per row, so a thousand rows in
three colours are three drawing calls and a pointer still lands on the line it
is over ([ADR 0015](0015-hit-testing.md), [ADR 0007](0007-per-mark-colour.md)).

**Which way up an axis reads is free.** [ADR 0075](0075-an-axis-has-a-direction.md)
made direction a property of the scale, so the "better is up" arrangement every
parallel-coordinates plot wants — `scale.Linear(scale.Reverse())` on the axis
where less is better — needs nothing from the coord and nothing from the mark.
`examples/parallel` draws it both ways.

## Consequences

| | |
|---|---|
| `ir`, `internal/layout`, `figure`, `interact`, `a11y` | unchanged |
| `render` | the panel's dimensions reach `geom.Training`, and a family that is the axis is labelled where the panel is rather than where the theme's ticks are |
| `coord` | `Parallel`, `ParallelDim`, `Dim`, `Dimensional`, `Dimensions`, `Scales`, `Desc.Dims`, `Furniture.FamiliesAreTheAxes` |
| `geom` | `Parallel`, `Dims`, `ErrDimensions`, `Training.Dims`, `Desc.Dims` |
| `spec` | the `"parallel"` coord with its `dims`, the `"parallel"` mark, and a `dims` channel list |
| Charts unlocked | parallel coordinates, and the same panel read as a profile plot of any wide table |

**`coord.Desc` and `spec.Encoding` stop being comparable**, because each gains
its first list field. Three tests compared a `Desc` with `==` and now compare it
structurally, and `spec`'s "did this encoding say anything" check is a method
rather than a comparison with the zero value. That is the honest cost of the
first coord whose configuration is a list rather than a handful of numbers, and
it is worth paying here: a coordinate system whose axes *are* its configuration
cannot be written down as scalars without a limit on how many of them there may
be.

**A gutter is still sized from the panel's own tick labels**, so a parallel
chart that leaves its theme alone reserves room for labels it does not write.
`theme.Ticks(false, false)` is the answer and the examples use it — the
dimensions keep their numbers through it, per the amendment above — but
`render.measurePanels` asking the scales rather than the coord is why the
question comes up at all. [ADR 0018](0018-coordinate-systems.md) recorded that
`layout` knows nothing about coords as a deliberate deferral, and this is the
second chart to notice.

## Not in scope

- **Categorical axes.** An axis of names is a second placement rule — slots
  rather than a position — and the lines through it bundle into a handful of
  points. It is the half-step towards parallel sets and is refused with it.
- **Parallel sets.** Ribbons between adjacent axes whose width is a count are a
  flow layout: a sankey per pair of axes, which is `stat.Sankey`'s question
  rather than this coord's.
- **Brushing, and reordering axes by dragging them.** Interaction is the host's
  ([ADR 0045](0045-linked-views.md)): the library hands over the hit index and
  the scales, and reordering the axes is a caller reordering its dimensions.
- **Choosing the axis order.** By correlation, by crossing count, by anything —
  an order that came out of an optimiser is not a pure function of the input,
  which is [ADR 0039](0039-relational-layouts.md)'s refusal and still holds.
- **Curves between axes.** The coord is straight and a bundled parallel plot is
  a smoothing over a layout, which [ADR 0077](0077-a-node-link-layout.md)
  already declined for edges.
- **A second mark in a parallel panel.** Nothing stops one being written; what
  this record ships is the line per row.

## Revisit if

- A second coord wants more than two axes. `Dimensional` is the interface it
  would implement and `Training.Dims` the field it would fill, and neither is
  parallel-specific — which is why both are named for the general case.
- The gutter reserved for silenced ticks becomes a real complaint. Then ADR
  0018's deferral is due, and the answer is a coord telling `layout` what room
  it needs rather than anything here.
- Someone needs a dimension's ticks to be the caller's rather than the scale's
  own. `scale.TickValues` already pins them and the coord asks the scale, so
  this is a documentation question until it is not.

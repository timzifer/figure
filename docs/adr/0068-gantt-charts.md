# 0068 — A schedule is a rect with a fraction in it and a second table beside it

**Status:** Accepted · **Date:** 2026-09-15

## Context

[docs/chart-types.md](../chart-types.md) has listed "Gantt / timeline" as a
recipe since `geom.Rect` shipped: *a `Rect` on a time X against an ordinal Y*.
That is true, and it draws every span in a plan in the right place. It is also
about a third of a gantt chart.

The three things it does not draw are the three a reader opens a schedule to
find out:

- **How far each task has got.** A bar that says only when a task runs answers
  a question about the plan. A reader looking at a plan on the 16th is asking
  about the *work*, and the only mark that answers is a bar that is part
  filled.
- **What is waiting on what.** A schedule is a graph, not a list. Two bars that
  do not overlap may be unrelated or may be the reason the second one cannot
  move, and nothing in the picture tells them apart. Slipping a task is a
  question about its successors, and a chart that cannot show which they are
  cannot be used to answer it.
- **The dates that are not spans.** A milestone has no duration, a baseline is
  a second span for the same task, and "today" is a line. None of these is the
  bar.

What people did instead was draw the arrows in another tool and paste the two
pictures together, or leave them out and explain the plan in prose beside it.
Both are the same failure: the chart is a picture of a schedule rather than
something to read one from.

## Decision

**Progress is an option on the rect. A dependency is a mark of its own, and it
is the one mark in this library that reads two tables.**

```go
p.Add(geom.Rect(tasks, geom.X("start"), geom.X2("end"), geom.Y("task"),
    geom.ProgressBy("done")))
p.Add(geom.Depends(tasks, links,
    geom.X("start"), geom.X2("end"), geom.Y("task"),
    geom.KeyBy("id"), geom.From("before"), geom.To("after")))
```

### Progress is an option, because it is a property of a cell

`geom.ProgressBy(col)` reads a fraction per row and the mark paints twice over:
every cell at the unfinished alpha, and the finished part of every cell at full
strength on top. Three things follow from that sentence and each is a decision.

**It is a second pass rather than a second shape per cell.** `geom.Rect`
batches by colour — `groupByRect` is one drawing call per distinct colour, which
is what keeps a thousand-cell heatmap a handful of calls — and two passes keep
that batching exactly. A cell drawn as one composite shape would be a call per
cell and would put a per-mark colour into the IR, which is the thing
[ADR 0007](0007-per-mark-colour.md) exists to refuse.

**The fraction grows from the edge the row named first**, from `X` towards
`X2` or from `Y` towards `Y2`, rather than from the left of the panel. On an
ordinary time axis those are the same thing. On a reversed one they are not,
and filling from the side of the screen would say the task had started at the
end. That is why `edgesOn` exists beside `spanOn`: the two return the same pair
of edges, in the order the *data* has them and in the order the *screen* has
them, and which one a mark wants is exactly whether it is asking about the
reading or about the ink.

**A cell bounded on both axes is `ErrProgressAxis`.** A fraction runs along one
axis and a cell whose row named both pairs of edges gives no reason to prefer
either. It is the rule [ADR 0036](0036-error-bars.md) already draws for an
error bar's orientation, refused for the same reason: a guess that depends on
the order the options were written is a chart that changes when somebody tidies
a line.

**An unknown fraction is a whole bar.** A null in the column is a task whose
progress nobody reported. Drawing it empty would say it had not started, which
is a reading of the data rather than of its absence.

The alpha is a constant rather than an option. It is not a choice about a
particular chart: it is the one relationship the mark asserts — that the pale
part of a bar is the same task as the solid part and not a different colour —
and a caller who wants a different look draws two rect layers, which is what
this saves them from.

### A dependency is a mark, and it reads two tables

`geom.Depends(tasks, links, …)` takes the positional channels from `tasks` and
everything about the link — `From`, `To`, `LinkBy`, `ColorBy` — from `links`.

**Two sources are two arguments, and that is the honest shape.** A dependency
is a statement about two rows of the task table, so it has no position of its
own; a task has no other end. Three alternatives were tried on paper first:

- *One predecessor column on the task table.* One source, one new field, and it
  reads well — but a task waiting on two others is the ordinary case in a real
  plan, and a table with one column for it cannot say so. A second layer with a
  second column is not an answer, it is the same limitation twice.
- *A link table carrying the four coordinates.* Fully general, one source, and
  it makes the caller compute a join that the layer is about to compute
  anyway. The mark would exist to save nobody any work.
- *Reading the spans off the neighbouring rect layer.* Geoms do not see each
  other, and a mark whose output depended on which other layers were in the
  plot would be the first one in this library that did.

**A link naming a task the table does not hold is dropped rather than
refused.** That is what makes the mark survive a facet: faceting cuts the task
table through `Subset`, and a constraint whose other end is in the next panel
has nothing to point at there. It is also what makes a plan with a typo in it
draw, which is the cost — and the reason there is a test in `examples/gantt`
that checks every link names two tasks that exist.

**The four linkages are the vocabulary, and they are one branch each.**
`FinishToStart`, `StartToStart`, `FinishToFinish` and `StartToFinish` differ
only in which edge of each bar the arrow touches, so `Linkage.from` and
`Linkage.to` are two booleans and the rest of the mark does not know which
linkage it is drawing. `LinkBy` names a column holding it per row, and a name
this package does not have takes the layer's own `Link` — a document from a
later version then draws a chart that is wrong in a place the reader can see
rather than one that does not draw, which is the rule `edgeNamed` already
follows for a track's edge.

**A critical path is a column, not a feature.** `ColorBy` over the link table
paints each arrow from its own row, so colouring the links whose slack is zero
*is* drawing a critical path. Adding a `CriticalPath` option would put a
scheduling algorithm in a plotting library and would answer a question — what
counts as critical — that belongs to whoever built the plan.

**The route is computed in device space, and it is the one place a geom does
not work in mapped coordinates.** Everywhere else in `geom` a midpoint, a
corner or a staircase step is computed before the coord, because it is a
statement about the data. Here there is nothing in data space to make a
statement about: the finish of one task and the start of another are two
positions, and the path a reader's eye takes between them is a reading aid. So
the two *ends* go through `coord.Point` like every other mark — they land on
the bars wherever the coord put them — and the elbow between them is drawn on
the screen, with a stub whose length is in pixels because it is there so that
the corner can be seen.

**The direction out of each bar is measured, not assumed.** A reversed axis
puts a task's finish to the left of its start, and an arrow that left rightwards
anyway would cross its own bar. Both anchors report the direction that leads
*away* from their own span, which is also what makes one routing function serve
a gantt drawn down the page as well as across it — the same quarter turn
[ADR 0031](0031-tracks.md) makes between a bottom track and a left one.

### Everything else a schedule needs was already here

A **milestone** is a date rather than a span, so it is not a bar at all: it is
`geom.Scatter` with `geom.Shape(ir.MarkerDiamond)` on the same two axes. A
**baseline** is a second `geom.Rect` layer over the same lanes, narrowed with
`geom.BarWidth`. **Today** is `geom.VLine`. A **task label** is `geom.Text`,
which already measures the box `Rect` would draw. None of these needed
anything, and saying so is the point: the two additions here are the two that
could not be composed.

## Consequences

- `geom.Desc` gains `ProgressCol`, `Links`, `Linkage` and `LinkCol`, and
  `spec.Layer` gains a `links` data block beside `data`. It is the first layer
  in the dialect to carry a second table, and it is **never hoisted**: two
  dependency layers over one plan are two sets of constraints, and a document
  that shared them would say they were one.
- A progress layer emits **two drawing calls per colour instead of one**, and
  a cell whose fraction is zero emits no finished part at all — a subpath of no
  area is still a mark a pointer could be told it was inside.
- A dependency layer **reports no rows**. `Hit` names the panel, the layer and
  a position, and the row it would report is a row of the link table, which is
  not the table `Source` hands out. A hit on an arrow is therefore a hit on the
  layer and not on a constraint. That is a smaller promise than every other
  mark with data makes, and it is the honest one until a geom can name which of
  its two tables a row belongs to.
- A dependency layer **takes the annotation colour rather than a palette
  entry**, because a constraint is not a series — the argument
  `config.annotationColor` already makes. It is solid rather than dashed,
  because a dashed arrow already means something else in a schedule.
- **`a11y` counts the links rather than the tasks** for such a layer, because
  that is how many marks it drew. Its extent is still the plan's, because that
  is where those marks land.
- Nothing below `geom` changed: `render`, `ir`, `coord`, `scale` and `layout`
  are untouched, and the only new field anywhere outside `geom` and `spec` is
  one branch in `a11y`.

## Revisit if

A plan wants **summary bars** — a phase drawn as a bracket spanning its
children — which is a third shape rather than a third option, and which would
have to answer whether the rollup is computed or given. Or a chart wants the
arrows **routed around the bars** rather than over them, which is an
obstacle-avoiding router and a different kind of thing from a four-corner
elbow: it would need to know where every bar in the layer is, which is
knowledge the mark has, and where every bar in *other* layers is, which is
knowledge no mark has. Or a hit on an arrow needs to name the constraint, which
is the row-identity question above and is the first case in this library where
a layer would have to say which of two tables a row number belongs to.

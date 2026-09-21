# 0079 — A parallel-sets diagram is a count, and the flow layout already draws it

**Status:** Accepted · **Date:** 2026-09-21 · **Implemented:** 2026-09-21

## Context

[ADR 0078](0078-a-coord-with-more-than-two-axes.md) gave a panel more than two
axes and drew a parallel-coordinates plot on them. It closed by naming the one
thing it did not draw, in the mark's own doc comment:

> It draws no categorical axis, no ribbons between axes — that is a parallel
> *sets* diagram, which is a flow layout rather than this.

and again in [the sweep of the unusual forms](../chart-types.md#the-sweep-of-the-unusual-forms):

> **Parallel sets** is still not: ribbons between adjacent axes whose width is a
> count are a flow layout, which is `stat.Sankey`'s question rather than this
> coord's.

Both sentences are right and neither is a decision. What they leave open is the
table, not the picture: `geom.Parallel` is the chart for a table of measured
quantities, and the table with *categorical* columns — how a ticket arrived,
how urgent it was, how it ended; class, sex, survival — is drawn nowhere in this
library today.

Drawing that table a row at a time is not an option that was rejected on taste.
Nine hundred tickets over three categorical columns have twelve distinct
answers between them, so a line per row is nine hundred lines landing on twelve
paths: the picture shows which combinations occur and says nothing at all about
how many, which is the only question anybody asks of it.

The count is what is missing, and the count is not the caller's to do. A caller
can reach `geom.Sankey` today by crossing every neighbouring pair of columns by
hand and handing over the edge list — which is exactly the shape
[ADR 0076](0076-the-other-half-of-the-count.md) refused for the set-size bars:

> a bar standing over the wrong column is the one way this form can lie, and two
> layers computing one thing separately from one table cannot do it.

## Decision

**`geom.ParallelSets` draws a table of categorical columns as the flow between
them, over a count in `stat` and the flow layout that is already here.** Seven
claims.

### 1. It is a count, and not a coord

Both of this mark's axes describe the unit square, exactly as a treemap's and a
sankey's do ([ADR 0039](0039-relational-layouts.md)). There is no
`coord.ParallelSets`, and `coord.Parallel` is not widened to hold one.

The coord holds a scale per dimension and places a *value* on it. A category has
no value to place — that is what makes it a category — and the ribbons are
between the axes rather than on them, so a coord that drew them would be
deciding a layout and drawing it, which is the thing
[ADR 0018](0018-coordinate-systems.md) has refused since the coordinate stage
was cut: a coord reports where furniture goes and `render` strokes it.

So the two marks share their spelling and nothing else. `geom.Dims` names the
columns for both, and a reader who has one chart knows how to ask for the other.

### 2. The arithmetic is `stat.Crosstab`

One crossing is a pair of categories in neighbouring columns and the rows that
hold both. It is [ADR 0054](0054-statistical-instruments.md)'s admission rule
met exactly — *a reduction belongs in `stat` when its output is the chart's
geometry, and there is no reading of it that is not the chart* — and it is
[ADR 0074](0074-sets-are-counted.md)'s shape again: indices in, counts out, the
names interned by the geom because interning a string is where the order of
everything downstream is decided.

### 3. The layout is `stat.Sankey`'s, because the count is a flow

Not "is like a flow". The same total passes through every column of this chart,
which is the one property the flow layout needs and the strongest form of it:
the columns of a sankey are ragged and these are not.

That has a consequence worth writing down, because it is what makes the reuse
honest rather than convenient. `Sankey`'s relaxation moves a node towards the
middle of what it is joined to and then packs the column apart again. In the
busiest column here there is nothing to move into — the nodes fill the interval
between the gaps exactly, and `separate` puts each one back where the stack put
it — so for that column the relaxation is a no-op and the layout is the plain
stacked partition this form wants. A column with fewer categories has the slack
of the pads the busiest column spends, and the relaxation is what spends it, on
standing nearer what it is joined to. The other visible consequence is that one
scale serves the whole diagram, which is what makes a ribbon's thickness mean
the same thing anywhere in it: an axis of five categories therefore ends three
gaps short of an axis of two, and `geom.Padding` is what that costs.

### 4. The column each category stands in is given, not derived

`stat.Sankey.ResetColumns` is a second entry point beside `Reset`, the way
`Tidy.ResetLeaves` is one beside `Tidy.Reset`: the same layout, with the first
of its two questions answered differently.

Deriving it is right for an edge list, where nothing else says which stage is
which, and wrong here. A category's column is the column it was read from, and
the longest-path rule would get it wrong exactly where the data is thin — a
category that nothing in the column before it reaches has no incoming link, the
longest path to it is zero, and it would stand at the far left among the
sources. `TestPinnedColumnsHoldWhereDerivedOnesWouldNot` is that case.

### 5. The order of the crossings is this chart's own

Every other layout here takes its order from the caller's table, because a map's
iteration order would make a chart built on several goroutines differ from one
built on one ([ADR 0012](0012-parallel-panels.md)). A crossing is not a row,
though — it is a count over rows, and the table has no opinion about where it
sits — so `Crosstab` hands its answer back ordered by the category a ribbon
leaves, then the one it enters, then its class.

The keys are distinct, so this is a total order and nothing depends on how a tie
was broken; it is one sort of the answer rather than a sort per sweep, which is
what [ADR 0039](0039-relational-layouts.md) actually refuses. And it is the
order that stacks the ribbons against each box in the order of the boxes at
their far end, so the ribbons cross where the data crosses and nowhere else.

The categories themselves still stack in the order they first appear going down
the table, which is every relational mark's rule and every relational mark's
answer to a caller who wants another one: sort the rows.

### 6. A colour column subdivides the ribbons

`geom.ColorBy` here counts (category, category, class) rather than (category,
category), so rows that agree on both categories and disagree on the colour
column are drawn as separate ribbons. A coloured diagram has more ribbons than
an uncoloured one.

The alternative is to paint one ribbon in some average of its rows' colours,
which is a number nobody can read and a colour nobody named. The subdivision is
the reading the form exists for: the escalated share of what arrives by phone is
a band a reader can follow from the first column to the last, instead of a
number that only appears in the last.

The boxes stay out of it. They are landmarks rather than series, so a diagram
whose ribbons come from a colour column draws its boxes in the theme's label ink
— asking a discrete scale for a box's colour would also teach that scale the
box's name, and put every category in a legend that is naming the colour
column's classes.

### 7. A row missing a category is counted nowhere

This is the one place in this library where an absent value costs a row rather
than gapping what it is part of, and the reason is arithmetic rather than taste.
A `geom.Parallel` line can break and resume because a line *is* one row. Each
column here is a partition of the same total, and a count that skipped only the
crossings beside the gap would leave one column adding up to less than the next
— after which no two thicknesses in the diagram mean quite the same thing, which
is the only thing this chart asserts.

**And a ribbon reports no row.** It is a count over many rows, so there is no
single row behind it to report — the rule [ADR 0074](0074-sets-are-counted.md)
set for every mark whose marks are aggregates, and the one a sankey's *node*
already follows while its bands do not.

## Consequences

| | |
|---|---|
| `stat` | `Crosstab` and `Sankey.ResetColumns`; `Sankey.Reset` is refactored into `begin`, the column rule, and `place`, and draws the same layout it always did |
| `geom` | `ParallelSets`, `MarkParallelSets`, `DimensionSeparator`; `sankeyGeom.width` and `.columnAt` become `flowWidth` and `flowColumnAt`, which both marks read |
| `spec` | the mark `"parallel-sets"`, reading `dims` beside every other mark that has them, and the flow layout's `padding` and `thickness` |
| `ir`, `render`, `coord`, `scale`, `theme`, `facet`, `internal/layout`, `figure` | unchanged |
| docs | `examples/parallelsets`, the `parallelsets` gallery figure, and the sweep's verdict closed |

**The mark writes no text**, so the categories reach the reader through the
legend — which is why a node's name carries its dimension: `channel: phone`
rather than `phone`, because a table with a "yes" in two columns has two
categories and a legend that said "yes" twice would be naming neither. A chart
of one layer has no legend by default, so this is the one form in the library
that usually wants `figure.Legend(true)`.

**The gap between two axes is a crossing and not a path.** Three columns are
two counts, and a reader who wants to know how many rows went first→male→lived
is asking about a combination the diagram does not draw. That is true of every
parallel-sets diagram ever published and is the form's known limit; the chart
that answers it is an UpSet plot over the same table
([ADR 0076](0076-the-other-half-of-the-count.md)).

## Not in scope

- **Labels on the boxes and titles over the columns.** A mark that wrote text
  would need a policy for a box too small to hold its name, and the placement
  machinery [ADR 0040](0040-label-collision-avoidance.md) governs. The legend
  names the categories today and `geom.Note` places anything else.
- **Reordering the categories to reduce crossings.** The sankey's refusal
  unchanged: a sort per sweep is where a layout stops being a pure function of
  its input and starts depending on how a tie was broken.
- **Counting every pair of columns rather than the neighbouring ones.** It is a
  different chart — a matrix of crosstabs, which is `facet`'s shape — and it is
  not what the ribbons between two axes mean.
- **A "missing" category for the rows claim 7 drops.** Inventing a category for
  an absent value is what `colorColumn` already refuses to do for a colour
  scale. A caller who wants those rows counted spells the category themselves,
  which is a cast and not a chart decision.
- **An alluvial diagram.** It is this mark with the dimensions being one column
  per time step, and it needs nothing that is not here.

## Revisit if

- **Box labels are asked for.** They are the form's one real gap, and the
  question is whether a mark that places its own layout may also write into it
  — which is 0040's table rather than this record's.
- **A second customer for `ResetColumns` turns up.** Two would say the column
  rule belongs in the layout's own vocabulary rather than in a second entry
  point.
- **Somebody wants the incomplete rows shown rather than dropped.** The honest
  drawing of that is a column that does not fill its panel, and it would have to
  say what a ribbon's thickness means when two columns disagree about the total.
  That is a record, not an option.

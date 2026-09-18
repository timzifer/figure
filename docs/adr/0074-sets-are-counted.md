# 0074 — Sets are counted rather than laid out, and the composition is the caller's

**Status:** Accepted · **Date:** 2026-09-18 · **Implemented:** 2026-09-18

## Context

[Bucket E](../chart-types.md) has been shipped except for two rows since
[ADR 0039](0039-relational-layouts.md), and both of them were refused in one
sentence:

> **Venn and UpSet.** A Venn layout is a circle-packing optimiser with its own
> failure modes, and UpSet is a matrix chart rather than a relational layout at
> all.

Two different reasons, and the second one is not a refusal at all — it is an
observation that the form does not belong to that family.
[ADR 0053](0053-tidy-tree-layout.md) said so itself while declining to act on
it:

> UpSet is a matrix chart and is probably a `Grid` question rather than a
> relational one, which is worth saying because it makes it cheaper than its
> neighbour in the same sentence of 0039.

This is that answer. The other half of the sentence is true of the
*area-proportional* Venn diagram and not of the one people draw, which is three
circles in the arrangement they have always been drawn in.

The reading matters more than the catalogue row. A Venn diagram labelled the
usual way — each circle carrying its own total — counts the elements in an
overlap twice, and readers take the numbers as a partition anyway. An UpSet plot
is the form that answers what the diagram is being asked, and it keeps answering
it past three sets, where the circles stop.

## Decision

**The only thing either chart needs is a count, and the composition is the
caller's.** `stat.Intersections` counts a membership list by which sets each
element is in; `geom.Intersections` and `geom.SetMatrix` are the two halves of
an UpSet plot; `geom.Venn` draws the two- or three-set diagram of the same
counts. Four claims.

### 1. It is a count, not a layout

An UpSet plot has no geometry to solve. Its columns are the distinct
combinations of sets that occur, its bars are how many elements are in exactly
each, its rows are the sets: a bar chart over a dot matrix, both of which figure
drew on day one. What was missing was the arithmetic, and by
[ADR 0054](0054-statistical-instruments.md)'s admission test that arithmetic is
a `stat`: the counts *are* the bars, exactly as `stat.Bin`'s counts are the
histogram, and there is no reading of them that is not the chart.

`stat.Intersections` is a struct with a `Reset`, like `stat.Sankey`, because its
working state is the size of the data — a mask per element, a slot per
combination. It never sees a string: the geom interns, the stat counts indices,
which is this repository's rule for a layout and holds for a count as well.
Combinations come out in the order their first element appeared, because the map
that finds one again is only ever asked whether it has seen it
([ADR 0012](0012-parallel-panels.md)).

**Exactly, and that is the whole reading.** An element in A and B is counted
once, under {A, B}, and not again under {A}. The counts therefore partition the
elements and add up to how many there are — which is what makes the bars
comparable, and what a circle labelled with its own total is not.

### 2. Two marks, one count, so the panels cannot disagree

The bars and the dots are two panels, so they are two layers; both read the same
membership table and run the same count in `Train`, which is where a stat whose
output is what an axis describes belongs ([ADR 0028](0028-distribution-stats.md)).
That is [ADR 0064](0064-a-contour-and-its-lattice.md)'s argument for one tracing
serving a flat contour and a projected floor, one dimension down: a bar standing
over the wrong column is the one way this form can lie, and two layers computing
one thing separately from one table cannot do it.

Each encodes its own categories into the ordinal axis it was given, which
`geom.Tree` already does for a dendrogram's leaves.

**`Order` and `Top` are the two options that decide which columns exist**, and
the caller must hand both halves the same ones — the rule a flat `Contour` and a
`three.Contour` already share about `Levels`. Ranked biggest-first is the
default, which is the only place in this package where `OrderAppearance` is not:
an UpSet plot *is* a ranking of how the sets overlap. That default is why
`geom.Order` now records having been told — "unset" and "in the table's order"
have to stay apart or the document round trip turns the ranking off.

### 3. The composition is the caller's, because a mark cannot make a panel

ADR 0054 settled this shape already, for the numbers-at-risk table under a
survival curve:

> a track is a panel, and a mark cannot make one. The caller adds it, and that
> is the correct division rather than a shortfall.

So the two-panel UpSet is `Plot.Track(figure.Bottom)`, which shares the panel's X
**scale object** and carries an ordinal scale of its own
([ADR 0031](0031-tracks.md)) — one zoom, one set of columns, by construction. The
three-panel form, with each set's own total beside the matrix, is a
`figure.Grid` with `GridSharedX` and fixed row heights and column widths, because
the third panel hangs off the *matrix's* Y axis and a track is a band on the
plot's scales rather than on another track's. Both are in `examples/sets`.

Nothing was added to `figure`, `render`, `layout`, `coord`, `scale` or `ir` for
either.

### 4. A Venn of two or three sets is a fixed picture

`vennLayout` is a table of circles, not a solver: one disc in the middle, two
side by side, or three on the corners of an equilateral triangle — the
arrangement every printed Venn diagram uses, and the one whose seven regions all
exist. The region anchors are arithmetic on that arrangement. Nothing is
searched for, so 0039's objection does not apply to it.

What 0039 objected to is still refused by name:

- **Area-proportional Venn and Euler diagrams**, where the radii and distances
  are solved so that each region's area is its count. That is the optimiser, it
  has no exact solution for most three-set tables, and its failure modes are
  pictures that are subtly wrong rather than obviously missing.
- **Four sets or more.** Four circles have no arrangement whose sixteen regions
  all appear; the four-set diagram that gets drawn uses ellipses, which is a
  different picture, and the five-set one is a packing problem.
  `ErrTooManySets` says so, and `geom.Intersections` is the chart that keeps
  working.

The circles are sampled into points and taken through the coord like every other
mark, and both axes describe the unit square — the relational convention,
unchanged.

**No new channels.** A membership row is a bipartite edge, so `geom.From` names
the element and `geom.To` names the set: 0039's channels, unchanged, for the
third record running. The one difference from a graph is that these marks intern
into **two** namespaces rather than one, because an element and a set are
different kinds of thing and a customer called "mail" is not the product.

## Consequences

| | |
|---|---|
| `ir`, `render`, `coord`, `scale`, `internal/layout`, `figure` | unchanged |
| `stat` | `Intersections`, `Combination`, `MaxSets` — a count, with a determinism test and oracles for repeated, disjoint and out-of-range memberships |
| `geom` | `Intersections`, `SetMatrix`, `Venn`, the shared membership reader, and one new shared option, `Top` |
| `spec` | the marks `"intersections"`, `"set-matrix"` and `"venn"`, a `top` property, and an `order` that is now written whenever a layer was told one |
| `docs/chart-types.md` | bucket E's last row but one; force-directed node-link is the only thing left in it |
| Charts unlocked | UpSet plot, UpSet with set sizes, membership matrix, two- and three-set Venn |

**Sixty-four sets, and that is a mask rather than a limit anybody meets.** A
combination is a bit per set in a `uint64`; a membership table with more is
refused rather than truncated. A chart of even a dozen sets is a wall of dots
long before the arithmetic runs out.

**Neither chart reports a row.** A column is what several rows have in common
and a Venn region is a count, so `Hit.Row` stays at −1 — the rule 0039 set for a
sankey's nodes, which is now true of every mark whose marks are aggregates.

**A set chart's X axis carries a made-up category.** The column's name is the
names of the sets in it joined by `geom.SetSeparator` ("∩"), which is a string
this package invents rather than one out of the table. It is what the matrix's
lanes and the bars agree on, and it is why the two marks must be given the same
options: the categories are encoded into the axis in ranking order.

## Not in scope

- **Force-directed node-link.** 0039's refusal stands verbatim for the third
  time. It is now the only thing bucket E is missing.
- **Area-proportional Venn, Euler diagrams, four sets.** Per claim 4.
- **Set-size bars as an option on the matrix mark.** They are a third panel, and
  a mark cannot make one. `figure.Grid` is the answer and the example shows it.
- **A degree filter, or dropping sets.** `Top` ranks; anything else about which
  rows are in the chart is a cut of the caller's own table, which is
  `data.Rows`'s job and not a mark's.
- **A second dot matrix convention** — degree-sorted columns, grouped columns,
  "set size" ordering. `Order` has two members here and they are the two
  readings; a third would need its own argument.

## Revisit if

- A chart wants the columns ranked by something that is not the count — degree,
  or a value column. Then `Ordering` gains a member and both marks read it,
  which is a change to the vocabulary rather than to this record.
- Someone needs the elements themselves on the chart, not just counted. That is
  a different mark (a membership matrix of *elements* against sets), and the
  reader it serves is different too.
- An area-proportional diagram is asked for with a bounded, deterministic
  construction behind it. Two circles have one — the overlap area is a
  monotone function of the distance between the centres, so a bisection settles
  it — and three do not. A two-set-only proportional diagram is a separate
  record and would have to say why stopping there is honest.

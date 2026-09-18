# 0072 — A layered graph is bounded and deterministic once the sort is stable, and the node's box is decoration rather than layout

**Status:** Accepted, amended · **Date:** 2026-09-18 · **Implemented:** 2026-09-18

## Context

Two records refused this and one of them said what would bring it back.

[ADR 0039](0039-relational-layouts.md) shipped four relational marks and
declined a node-link layout because a force simulation "cannot be a pure
function of its input in any bounded sweep count that also looks good".
[ADR 0053](0053-tidy-tree-layout.md) observed that the refusal had been applied
to a category one size too large — it binds *force* layouts and not *tree*
layouts — shipped `geom.Tree`, and left this:

> A DAG turns up — a dependency graph that is nearly a tree. Layered
> (Sugiyama) layout is bounded too, but its crossing-reduction phase is a
> heuristic sort, which is the thing this record and 0039 both refuse. It
> would need its own answer to that, not an option here.

This is that answer, and the DAG that turned up is a state machine.

Both refusals rest on one sentence, written in 0039 and quoted in 0053:

> a sort is where a layout stops being a pure function of its input and starts
> depending on how a tie was broken.

That sentence is true of *a* sort. It is not true of *every* sort, and the
difference is the whole record.

## Decision

**`stat.Layered` is a layered graph layout, and `geom.Graph` is a sixth
relational mark that draws it in the unit square.** Three claims carry it.

### 1. A stable sort by a computed key with the index as its tie-break is a pure function

The objection is exactly right about the usual implementation. Crossing
reduction sorts each rank by the barycentre of its neighbours' positions, and
two nodes with equal barycentres — which is the common case, not the corner one,
because a node with one neighbour takes that neighbour's position exactly — are
then ordered by whatever the sort did with them.

The fix is not to avoid the sort. It is to make the key total:

- the sort is **stable**, so equal keys keep the order they came in with;
- that incoming order is the interning order, which is the order the rows
  appeared in the caller's table ([ADR 0012](0012-parallel-panels.md));
- the sweep count is a **constant**, `stat.LayeredSweeps`, for the reason
  `stat.SankeySweeps` is one — a relaxation that ran until it settled would make
  the picture depend on floating-point noise;
- nothing reads a map's iteration order, per the interner's existing rule.

With those four, the layout is a pure function of its input: the same table
produces the same coordinates, on one goroutine or on eight, on any platform.
That is the property 0039 and 0053 were protecting, and it survives a sort.

What does **not** survive is the claim that the result is *optimal*. Crossing
minimisation is NP-hard and the barycentre sweep is a heuristic; a different
row order produces a different, equally valid picture. That is the same
contract `stat.Sankey` already ships under — it relaxes positions and refuses to
reorder — except that here the reordering is the point, and so it is written
down as a consequence rather than smuggled in as a quality claim.

### 2. Cycles are broken, not refused

`geom.ErrCyclic` exists because a flow that returns to where it came from has no
column to stand in and a hierarchy that is its own ancestor has no root. Both
are true. Neither applies here: a state machine that cannot return to a previous
state is not a state machine, and `A → A` is the commonest edge there is.

So `stat.Layered` breaks cycles instead of reporting them. A depth-first walk in
index order marks the back edges, layering runs on the remaining DAG, and the
marked edges are drawn in their true direction against the rank order — which is
what makes a returning transition read as a returning transition. `LayeredEdge`
carries `Back` so the mark can draw it differently, and self-loops never reach
the layering at all.

Which edges come out as back edges depends on the walk order, and the walk order
is the table's order. That is the same dependency every other decision here has
and it is deterministic for the same reason.

### 3. The layout never sees a label, and the node's box is screen-space decoration

This is the claim that was genuinely open, and it went the opposite way from how
every graph drawing library answers it.

Graphviz sizes each node from its text and lays out against those sizes. Doing
that here would mean the layout depends on the active backend's font metrics —
so the same document rendered to SVG and to PNG would have different geometry,
not merely different glyphs. Principle 3 of `CONCEPT.md` is "one model → many
backends, **identical output**", and this would be the first thing in the
library to weaken it.

So the seam is drawn where the other marks already draw it. `stat.Layered`
places nodes in the unit square knowing only the graph. The box a node is drawn
in is sized from `ir.Backend.Measure` at build time and centred on that
position — the same kind of thing a point marker's radius is, or the seven
points `geom.DefaultArrowSize` gives a gantt arrowhead. Geometry from data,
decoration from the backend, and the two do not mix.

**The honest cost:** the layout spaces nodes without knowing how wide their
labels are, so a long label can overlap its neighbour. `geom.NodeSep` widens
every slot, in fractions of the unit square, because the stat is in the unit
square and does not know how wide the panel is. That is a knob rather than a
solver, and it is the price of the property above. A de-overlap pass is bucket
G's open item ([ADR 0032](0032-text-as-a-mark.md)) and stays there.

### The five phases, all bounded

| Phase | What it does | Bounded by |
|---|---|---|
| Break | depth-first walk in index order marks back edges and self-loops | one pass |
| Rank | longest-path layering over the remaining DAG | one pass per edge |
| Route | a chain of dummy nodes per edge that spans more than one rank | the span |
| Order | stable barycentre sweeps, forward and back | `LayeredSweeps` |
| Place | each rank's nodes spread across the unit interval, parents pulled towards their children's median | `LayeredSweeps` |

Longest-path ranking and not network simplex: network simplex gives shorter
edges and needs a pivot rule, and a pivot rule is a tie-break that would have to
be justified all over again. The compaction it buys is not worth reopening the
record's own argument.

### It fills the unit square, so the radial version is free

0039's convention, kept for the sixth time: the rank is the height out and the
position within a rank is the breadth across, both in `[0, 1]`, with `rim = y1`.

- under `coord.Cartesian`, a layered digraph, ranks running up the panel;
- with `geom.Baseline(1)`, ranks running down — the orientation a state chart
  and a `rankdir=TB` graph are usually drawn in;
- under `coord.Polar()`, concentric ranks with the start state at the hub.

There is no `rankdir` option, because the coord and `Baseline` are what that
option is in this grammar.

### The channels already exist

`geom.From` and `geom.To` name an edge's ends and `geom.Value` its magnitude —
0039's flow channels, unchanged, because a transition is a flow's shape even
though it is not a flow's meaning. A node's own name is its label, interned out
of those two columns exactly as a sankey's nodes are, so nothing names it
separately. No new channel.

### Arrowheads become shared

`arrowhead` is private in `geom/gantt.go`, where it draws a dependency link's
head. It moves beside the other things `geom/relational.go` already shares, with
`geom.DefaultArrowSize` unchanged as its size, and gantt keeps drawing exactly
what it drew.

## Consequences

| | |
|---|---|
| `render`, `ir`, `coord`, `scale`, `layout` | unchanged |
| `stat` | `Layered`, one bounded pure function, with a determinism test and cycle, self-loop and disconnected-forest oracles |
| `geom` | one mark, reusing 0039's `edges`, its interning order, its node colours, its legend and its refusal of an ordinal axis; `ErrCyclic` is not raised by it |
| `spec` | a `"graph"` mark and an edge-shape field |
| `docs/chart-types.md` | bucket E's "Node-link, force-directed — missing" is the only line left |
| Charts unlocked | state chart, dependency and call graph, DAG pipeline, layered flowchart, radial state diagram |

**The picture depends on the row order, and that is now a documented input
rather than an accident.** `geom.Order` is how a caller changes it, which is
what 0039 said and is now true of one more thing.

**A node reports no row where a node is not a row.** 0039's rule, unchanged: the
links are the rows, a hit on a node leaves `Hit.Row` at −1, and `Hit.X`/`Hit.Y`
are positions in the unit square.

**Both axes describe the unit square and mean nothing to a reader.** More
sharply than for a treemap, whose numbers at least are shares: a graph's
coordinates are layout output with no domain behind them. Such a chart wants a
theme with no grid, no axis line and no ticks, exactly as a pie does — and if
that turns out to be most of what people do with it, the answer is a theme, not
an axis that lies about being data.

## Not in scope

- **Force-directed layouts.** 0039's refusal stands verbatim and this record
  does not consume its revisit clause. This is the second time the missing
  category is narrowed rather than emptied.
- **Clusters and subgraphs.** A box around a set of nodes that also constrains
  the layout is a second solver, not an option on this one.
- **Ports and compass points.** An edge arrives at a node, not at a named point
  on it.
- **Edge labels.** A transition's `event [guard] / action` is the first thing a
  state chart wants and the second thing this mark will be asked for. It needs
  the de-overlap pass that ADR 0032 deferred; placing a label on a path without
  one produces a picture worse than no label. Bucket G first.
- **Crossing minimisation as a promise.** Per the argument above: sweeps
  reduce crossings and do not minimise them.
- **A DOT frontend.** A `figure/dot` module translating graphviz into this mark
  is a reasonable thing to want and is not this record's to grant. DOT's model
  is a literal style per element, and figure's is a channel through a scale;
  bridging them needs either an identity scale in `scale` or a lowering in the
  frontend that is always slightly wrong, and graphviz parity is a surface with
  no edge to it. `CONCEPT.md` §5's non-goals are the test such a module would
  have to pass, and it is a separate record if it is ever written.

## Revisit if

- **A layout wants the labels after all** — a graph of long names where
  `NodeSep` is not enough. The answer is not to feed `Measure` into the stat; it
  is a second pass in `Build` that widens slots using the measured widths, after
  the stat has fixed the order. The order stays backend-independent and only the
  spacing moves, which keeps most of claim 3.
- **Edge labels arrive** — then ADR 0032's deferred de-overlap is due, and this
  mark is one of its callers rather than its reason.
- **Network simplex is wanted** for shorter edges. It needs its own answer to
  the tie-break question, exactly as this record needed one for the sort.

## Amendment: what building it sharpened

`stat.Layered` is built, `geom.Graph` draws it, `spec` reads and writes a
`"graph"` mark, and `examples/statechart` draws the two charts the record argued
for: a TCP connection with three returning transitions, and a build pipeline
that is a plain DAG. Six things came out sharper than the record, and one of
them narrows a claim.

**Claim 3 was one word too strong, and the word is "geometry".** The layout
fills the unit square, so a node on the first or last rank sits exactly on the
panel's edge and half its box falls outside it. The fix is an inset — half the
widest box, as a fraction of the panel, taken off each end — and that inset is
measured, so where a node lands *does* move when the font does.

That is not the failure the record was guarding against, and the reason is that
figure already works this way everywhere else: the panel rectangle is fitted
around measured axis labels and a measured legend (`render/guide.go`), so two
backends with different metrics already place every mark slightly differently.
What claim 3 protects, and what is true as written, is the **arrangement** —
which node is on which rank, and their order across it. That is a pure function
of the graph, and `TestAGraphsArrangementDoesNotDependOnTheFont` is the test
that says so. Fitting it to a panel is not the layout.

**`geom.NodeSep` does not exist, because it cannot.** The record offered it as
the knob that buys room when a long label crowds its neighbour. In a layout that
fills the unit square there is nothing for it to widen: the pitch is
`1 / widest rank` by construction, and a rank cannot be given more of an
interval it already fills. The knobs that do work are `geom.FontSize`, which
narrows the boxes, and the panel, which is where the room actually is. The
record's consequence — that a long name can overlap its neighbour — stands; only
the remedy named for it was imaginary.

**The arrowhead did not move out of `geom/gantt.go`.** The record said it would
become shared. It could not: gantt's takes an *orientation*, because a
dependency link always arrives along an axis, and an edge in a graph arrives
from wherever its source happens to be. `arrowTip` takes a direction vector
instead, and the two live side by side — which is the smaller duplication, since
the alternative was giving gantt a vector it would only ever pass an axis in.

**A node's box is a device rectangle rather than an area handed to the coord.**
The record implied it and the code makes it load-bearing: `fillBoxes` sends a
rect through `coord.Coord.Area`, which turns it into an annular sector under a
polar coord. A treemap cell should become one and a labelled box must not, so
`geom.Graph` builds its own path with `ir.Path.RoundRect` and keeps one subpath
per node, which is what ADR 0015 indexes. Positions still go through the coord,
so a radial state diagram is free exactly as the record predicted.

**Back edges are dropped from the ranking rather than reversed.** The textbook
reverses them so the graph is a DAG. Reversing a back edge `v → u` adds the
constraint `rank(v) ≥ rank(u) + 1`, which the forward pass that made `u` an
ancestor of `v` has already satisfied — so it constrains nothing and costs a
pass. The record's "layering runs on the remaining DAG" is what the code does
and it is also the stronger statement.

**`stat.Layered` carries its own merge sort.** `sort.SliceStable` takes a
closure and a reflect-based swapper and so allocates on every call, and the
sweeps call it `2 × Ranks × LayeredSweeps` times. A bottom-up merge sort over a
buffer the struct owns is stable by construction and `TestLayeringAgainDoesNotAllocate`
holds the line AGENTS.md draws: a frame's cost does not grow with the data.

**The cost is the number of ranks an edge crosses, not the number of edges.** An
edge spanning *r* ranks routes through *r*−1 bends, so a graph with a long
critical path and edges that jump it ranks into as many layers as it has nodes
and routes a bend per layer per edge. A random edge list is the worst case. That
is inherent to layering rather than to this implementation, and it is written
into `stat.Layered`'s documentation as the reason this mark is for graphs that
are nearly layered already.

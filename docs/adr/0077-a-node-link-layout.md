# 0077 — A node-link layout is a minimisation with a closed-form step, and the refusal was of a method

**Status:** Accepted · **Date:** 2026-09-18 · **Implemented:** 2026-09-18

## Context

[ADR 0039](0039-relational-layouts.md) shipped four relational marks and refused
one thing:

> **A node-link / force layout.** It is in bucket E, and it is the one member
> that cannot be a pure function of its input in any bounded sweep count that
> also looks good — a force simulation's whole method is to run until it
> settles. ADR 0012 would have to be answered on its own terms first, and it is
> not answered by this record.

That sentence has been quoted four times. [ADR 0053](0053-tidy-tree-layout.md)
took tree layouts out of it — "the refusal had been applied to a category one
size too large". [ADR 0072](0072-layered-graph-layout.md) took layered ones out,
and found the same thing again: a heuristic sort is a pure function once its key
is total. [ADR 0074](0074-sets-are-counted.md) repeated the rest of it verbatim
and called it the only thing bucket E was missing.

Read closely, the sentence welds two claims together:

- **that it cannot be a pure function at a bounded sweep count**, which is a
  property of one *method*, and
- **that also looks good**, which is a claim about pictures and was never
  tested.

The first is true of a force simulation and of nothing else here. A simulation
integrates a system of forces and stops when the movement falls below a
tolerance; stop it early and it is caught mid-swing, so the bound and the
picture are the same question. That is not how this form has to be drawn.

**Stress majorization is not a simulation.** It minimises one number — how far
the drawn distances are from the graph's own — and it does so by replacing that
function, at each step, with a quadratic that touches it from above and whose
minimum is an arithmetic expression. There is no integration, no tolerance and
no clock. Every step goes downhill or nowhere, so stopping after a fixed count
costs quality and cannot cost correctness: the drawing at sweep fifty is the
drawing at sweep five hundred, further along.

So the refusal was of a method, and the form outlived it. What is left is the
second claim, and this record settles it the only way it can be settled: by
building the thing and looking at the pictures.

## Decision

**`stat.Stress` places the nodes and `geom.NodeLink` draws them.** Five claims,
three of which are measurements rather than arguments.

### 1. The sweep count is a constant, for the third time

`stat.StressSweeps` is 50, beside `stat.SankeySweeps` and
`stat.LayeredSweeps`, and for the reason those exist
([ADR 0012](0012-parallel-panels.md)). Each sweep computes every new position
from the previous sweep's positions only — Jacobi rather than Gauss–Seidel — so
the arrangement does not depend on the order the nodes are visited in, only on
the order the caller's rows interned them in. Nothing reads a map.

The two forms were measured against each other and came out within a tenth of a
percent of one another on the test graphs, so the one with the stronger property
was kept.

### 2. Where it starts is what a bounded run decides, and the start is arithmetic

Majorization only ever goes downhill, so the minimum it reaches is chosen by
where it begins. A random start is the usual answer and the reason the usual
implementation cannot be repeated. The first version here started from a circle
in interning order, which is repeatable and wrong:

> **A 4×4 grid came out folded in half.** Its two far corners were drawn a tenth
> of the picture apart.

So the start is `stat.Stress.classical`: classical multidimensional scaling of
the distance table — the two directions the graph is most spread out along —
found by power iteration over the double-centred squared distances. The
iteration count is `stat.StressPowerIterations` rather than a tolerance; the
start vector is the first node's own distances, which is data rather than a
seed and is never the vector that matrix annihilates. From there the same grid
comes out a grid.

A repeated leading eigenvalue has no single answer to "the most spread out
direction". The iteration then lands wherever its arithmetic lands — the same
way every time, which is the property this repository needs, but not a
*canonical* one: the 4×4 grid comes out standing on its diagonals, and another
implementation of the same method could pick another pair from the same
eigenspace. Nothing downstream depends on which.

### 3. A fixed spiral does the one job a random start really has

Two nodes with the same neighbours have the same distance to everything and to
each other, so the arrangement that puts them in one place is stationary: every
sweep computes one position for both and leaves them there. Measured, in the
gallery figure below:

> **Two members of one team were drawn as one dot with two names on it.**

That is what a random start is actually for. `stat.Stress.scatter` does it
without one: every node is moved a hundredth of the spread along a direction of
its own, the golden angle apart, so no two are pushed the same way. In the same
figure the two are now sixty-seven pixels apart, which is an ordinary distance
between neighbours there. The coincident arrangement is a saddle rather than a
minimum, so the smallest push is enough — what it needed was a push.

### 4. It refuses what it cannot draw

`stat.MaxStressNodes` is 250, and two limits arrive together at about that
number. The picture gives out first — a straight-line drawing of a few hundred
nodes is a hairball — and the arithmetic just after, because every pair has a
distance and every sweep reads all of them: 250 nodes is about 30 ms and 500
about 140, measured. `geom.NodeLink` refuses a bigger graph with
`ErrTooManyNodes` and names what draws it instead, which is the rule
`ErrTooManySets` already follows.

That the legibility bound and the frame budget agree is luck, and it is the kind
of luck worth writing down: had they not, the honest constant would have been
the smaller one.

### 5. The drawing is square, and that is ADR 0028's exception in its weak form

What the layout matched is a *distance*, so a panel that stretched one axis
would undo every sweep of it. [ADR 0039](0039-relational-layouts.md) met this in
the squarified treemap and answered it by running the layout in `Build` against
the real rectangle. Here only the *fitting* needs the rectangle: the layout runs
in `Train` where ADR 0028 puts it, in the unit square, and `Build` places that
square inside the largest square the panel holds, inset by one disc so that a
node is drawn whole. A treemap had to move its solver because what it optimised
was an aspect ratio; this optimises a ratio between distances, which survives
being scaled as long as both axes are scaled alike.

## Consequences

| | |
|---|---|
| `stat` | `Stress`, `StressPoint`, `StressEdge`, `StressSweeps`, `StressPowerIterations`, `MaxStressNodes` |
| `geom` | `NodeLink`, `MarkNodeLink`, `ErrTooManyNodes`, `NodeLinkRadius` |
| `spec` | the mark `"node-link"`, with a fill, a stroke and a size |
| `ir`, `render`, `coord`, `scale`, `internal/layout`, `figure` | unchanged |
| Charts unlocked | node-link diagram; `examples/network` and `docs/images/network.png` |

**Bucket E is empty.** `docs/chart-types.md` has had one row outstanding since
ADR 0039, and this is it. `CONCEPT §14`'s sentence about the family that shares
nothing with the rest is now spent in full.

**A long chain is drawn as a shallow arc.** Classical scaling's own artefact —
the horseshoe — partly straightened by the sweeps: the middle node of a
five-node path sits about a seventh of its span off the line between its ends.
It is the one place where the bound is visible in the picture rather than only
in the arithmetic. A straight line is the exact optimum of that graph, so this
is slow convergence and not a wrong answer; multi-level coarsening is the known
fix and is not here.

**The layout runs in `Train`,** which means once per render rather than once per
edge list. A chart that redraws every frame pays for it every frame, and at this
mark's node counts that is tens of milliseconds. It is a mark for a chart that
is drawn, not for one that is animated.

**A node reports no row and an edge reports its own,** which is ADR 0039's rule
for a sankey's nodes, unchanged.

## Not in scope

- **A force simulation.** Still refused, and now for a reason narrower than the
  one 0039 gave: not because the family cannot be a pure function, but because
  *that method* stops on a tolerance. Nothing needs it.
- **Edge weights as distances.** `geom.Value` is read by the flow marks and
  ignored here: a number on an edge is a magnitude, and turning it into a length
  is a different chart with a different reading. It would also break the whole
  number arithmetic the distance table rests on.
- **Multi-level coarsening.** The known fix for the chain artefact and the way
  past `MaxStressNodes`: coarsen by a maximal matching, lay the small graph out,
  interpolate, refine. Deterministic in principle — the matching is a walk in
  interning order — and its own record.
- **Arrowheads, curved edges, edge bundling.** A direction shown on a line is
  [`geom.Graph`](0072-layered-graph-layout.md)'s reading; bundling is a layout
  of the edges rather than of the nodes.
- **Label de-overlap.** Bucket G's open item since
  [ADR 0032](0032-text-as-a-mark.md), and this mark does not change where it
  stands.
- **A canonical rotation.** Per claim 2 — the picture is repeatable but not
  oriented, and no reading here depends on which way up it is.

## Revisit if

- **A chain artefact turns up in a chart somebody cares about.** Multi-level is
  the answer and it moves `MaxStressNodes` at the same time.
- **Someone wants the edge weights to be distances.** That is a different
  distance table — a weighted shortest path — and everything above it is
  unchanged. What it needs is an argument about what the number on an edge
  means, not a new solver.
- **A graph arrives that is too big for this and has a direction.** That is
  `geom.Graph` and it is already there. One with neither is an adjacency matrix,
  which `geom.Rect` over two ordinal axes already draws.

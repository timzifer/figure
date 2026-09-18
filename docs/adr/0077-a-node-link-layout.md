# 0077 — A node-link layout is a monotone descent on a named objective

**Status:** Accepted, amended · **Date:** 2026-09-18 · **Implemented:** 2026-09-18 · see [Amendment](#amendment-three-corrections-from-review)

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

- **that it cannot be a pure function at a bounded sweep count**, and
- **that also looks good**, which is a claim about pictures and was never
  tested.

The first is too strong, and saying so is the honest start. A force simulation
*can* be made a pure function of its input: fix the starting arrangement, fix
the step size, fix the iteration count, and it repeats exactly. A tolerance does
not make an algorithm non-deterministic either — a loop that stops when a
number falls below a threshold stops at the same place every time it is given
the same arithmetic. So there is no impossibility here, and this record does not
rest on one. What 0039 did was treat one method's *stopping rule* as a property
of the whole family, which is the same over-broad move
[ADR 0053](0053-tidy-tree-layout.md) and [ADR 0072](0072-layered-graph-layout.md)
found in it twice already.

What is true is narrower and is a reason rather than a proof:

- **The objective is named.** Stress is a number, so "is this arrangement better
  than that one" has an answer that can be measured and tested. A force
  simulation has no objective it is descending — the forces are the model — so
  the same question has no answer inside the method.
- **The descent never goes uphill.** Majorization moves the arrangement down a
  function that sits above the stress, so the cost of stopping after a fixed
  number of sweeps is a known trade: quality, bounded, and visible in a
  measurement. Stopping a simulation early leaves it wherever it happened to be
  in its own swing, and nothing in the method says how far that is from where it
  was going.
- **The pictures were measured**, which is the part 0039 never did and the part
  this record spends most of its length on.

A force simulation with a fixed budget would be a defensible choice too. It is
not the one made here, and the difference is an argument about objectives and
guarantees rather than about what is possible.

## Decision

**`stat.Stress` places the nodes and `geom.NodeLink` draws them.** Five claims,
three of which are measurements rather than arguments.

### 1. The sweep never goes uphill, and that is a test rather than a phrase

`stat.StressSweeps` is 50, beside `stat.SankeySweeps` and
`stat.LayeredSweeps`, and for the reason those exist
([ADR 0012](0012-parallel-panels.md)).

What a bound costs depends on what a sweep guarantees, so it is worth being
exact about the step. Majorization replaces the stress with a quadratic that
sits above it and touches it at the current arrangement. Minimising *that
quadratic*, over all the nodes at once, is a linear system — and this does not
solve one. It descends the quadratic one node at a time: each node's own block
is solved exactly with the others held still, and the result is written back
before the next node reads it. Block coordinate descent on a convex quadratic
never raises it, and the quadratic sits above the stress and meets it at the
start of the sweep, so the stress does not rise either. That is the whole of
"a bound costs quality and cannot cost correctness", and
`TestASweepNeverRaisesTheStress` is it as a property over five graphs.

**The simultaneous form does not have it, and that was measured rather than
assumed.** Computing every new position from the previous sweep's positions —
Jacobi rather than in place — is not a descent: two nodes whose target distance
is 1, placed 1.02 apart, come out 0.98 apart, then 1.02 again, for ever, at
constant stress. The first version of this record claimed that form as a virtue,
on the grounds that it does not depend on the order the nodes are visited in.
It does not descend either. `TestTheSimultaneousSweepIsWhyThisOneIsNot` pins the
case.

So the order a sweep visits the nodes in is load-bearing, and it is the order
the caller's rows interned them in — which is the order every other layout in
this package already works in, and the one ADR 0012 asks for.

### 2. Where it starts is what a bounded run decides, and the start is arithmetic

A descent that never goes uphill reaches the minimum whose basin it starts in,
so the starting arrangement is the whole of that decision. A random start is the
usual answer and the reason the usual implementation cannot be repeated. The
first version here started from a circle in interning order, which is repeatable
and wrong:

> **A 4×4 grid came out folded in half.** Its two far corners were drawn a tenth
> of the picture apart.

So the start is classical multidimensional scaling of the distance table — the
directions the graph is most spread out along — found by power iteration over
the double-centred squared distances. The iteration count is
`stat.StressPowerIterations` rather than a tolerance; the start vector is a
node's own row of distances, which is data rather than a seed. From there the
same grid comes out a grid.

**"Most spread out" means the biggest eigenvalue by value, and power iteration
finds the biggest by magnitude.** Those are the same thing only when the
distances are Euclidean, and graph distances frequently are not. The complete
bipartite graph K(5,5) is the case, not a contrived one: its centred table has
the spectrum −5.5, eight 2s and a 0, and the iteration converges to the −5.5 —
then reports 5.5, because what it has in hand is a norm. Taking the square root
of that as a coordinate scale is arithmetic on a direction the arrangement is
not spread along at all, and checking the sign afterwards does not help, because
by then the iteration has converged to the wrong direction.

The fix is to move the whole spectrum: measure the spectral radius with one
unshifted run, then iterate on the table shifted up by it, where every
eigenvalue is non-negative and the biggest by magnitude *is* the biggest by
value; subtract the shift back at the end. Two details come with it. The iterate
is held orthogonal to the vector of ones, because the centred table annihilates
that direction and the shift would otherwise make it a competitor. And the two
directions start from two different nodes' rows: started from the same one,
projecting the first direction out of the second can leave nothing of it behind
— K(5,5) again, which came back with a second eigenvalue of zero.

**A table of rank one is an answer, not a failure.** A path's distances are
realised exactly by points on a line, so its centred table has one positive
eigenvalue and nothing else, and the truthful drawing *is* that line. The first
version treated a missing second direction as a failure and fell back to the
circle, which is what bowed a five-node chain by a seventh of its own length —
a defect this record shipped with and described as a property of the method.
Keeping the first direction and flattening the second brings the same chain to
under one percent of its span.

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
distance and every sweep reads all of them: 250 nodes is about 40 ms on the
machine this was written on, and the growth is quadratic. `geom.NodeLink`
refuses a bigger graph with `ErrTooManyNodes` and names what draws it instead,
which is the rule `ErrTooManySets` already follows.

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
| `stat` | `Stress`, `StressPoint`, `StressEdge`, `StressSweeps`, `StressPowerIterations`, `MaxStressNodes`, and three internal tests for the properties above |
| `geom` | `NodeLink`, `MarkNodeLink`, `ErrTooManyNodes`, `NodeLinkRadius` |
| `spec` | the mark `"node-link"`, with a fill, a stroke and a size |
| `ir`, `render`, `coord`, `scale`, `internal/layout`, `figure` | unchanged |
| Charts unlocked | node-link diagram; `examples/network` and `docs/images/network.png` |

**Bucket E is empty.** `docs/chart-types.md` has had one row outstanding since
ADR 0039, and this is it. `CONCEPT §14`'s sentence about the family that shares
nothing with the rest is now spent in full.

**A long chain is drawn straight, to within about a hundredth of its span.**
That number is here because the first version of this record reported a seventh
instead, and called it classical scaling's horseshoe partly straightened. It was
not: it was the circle fallback of claim 2, and it went away when the fallback
did. The measurement is `TestStressDrawsNearThingsNear`'s neighbourhood, and the
chain case is in the shape probes behind claim 2.

**The layout runs in `Train`,** which means once per render rather than once per
edge list. A chart that redraws every frame pays for it every frame, and at this
mark's node counts that is tens of milliseconds. It is a mark for a chart that
is drawn, not for one that is animated.

**A node reports no row and an edge reports its own,** which is ADR 0039's rule
for a sankey's nodes, unchanged.

## Not in scope

- **A force simulation.** Not built, and not refused as impossible — per the
  Context, a fixed start, a fixed step and a fixed count make one repeatable
  too. What it does not have is an objective of its own, so the cost of its
  bound cannot be stated the way this one's can. Nothing needs it.
- **Edge weights as distances.** `geom.Value` is read by the flow marks and
  ignored here: a number on an edge is a magnitude, and turning it into a length
  is a different chart with a different reading. It would also break the whole
  number arithmetic the distance table rests on.
- **Multi-level coarsening.** The way past `MaxStressNodes`: coarsen by a
  maximal matching, lay the small graph out, interpolate, refine. Deterministic
  in principle — the matching is a walk in interning order — and its own record.
- **Arrowheads, curved edges, edge bundling.** A direction shown on a line is
  [`geom.Graph`](0072-layered-graph-layout.md)'s reading; bundling is a layout
  of the edges rather than of the nodes.
- **Label de-overlap.** Bucket G's open item since
  [ADR 0032](0032-text-as-a-mark.md), and this mark does not change where it
  stands.
- **A canonical rotation.** Per claim 2 — the picture is repeatable but not
  oriented, and no reading here depends on which way up it is.

## Revisit if

- **A graph turns up whose picture this gets wrong.** The shape probes behind
  claim 2 are where a new case goes; multi-level is the answer that also moves
  `MaxStressNodes`.
- **Someone wants the edge weights to be distances.** That is a different
  distance table — a weighted shortest path — and everything above it is
  unchanged. What it needs is an argument about what the number on an edge
  means, not a new solver.
- **A graph arrives that is too big for this and has a direction.** That is
  `geom.Graph` and it is already there. One with neither is an adjacency matrix,
  which `geom.Rect` over two ordinal axes already draws.

## Amendment: three corrections from review

**Date:** 2026-09-18

Three claims in the first version of this record were wrong. Two were wrong in
the *code* as well, and the pictures changed when they were fixed; the third was
only wrong in the prose, which in a record is not a lesser kind of wrong. All
three came from a reading that checked the arithmetic rather than the argument,
and each is now a test.

**The step was described as a closed-form minimisation of the majorizing
quadratic.** It is not: minimising that quadratic over all the nodes at once is
a linear solve, and what the sweep does is descend it one node at a time. Worse,
the version that shipped did it *simultaneously* — every new position from the
previous sweep's — which is not a descent at all, and the record had claimed
that form as a virtue for being order-independent. Two nodes whose target
distance is 1, placed 1.02 apart, cycled 0.98, 1.02, 0.98 at constant stress.
The sweep now writes each position back before the next node reads it, which is
block coordinate descent and does not raise the stress; claim 1 says so, and two
tests hold it.

**The starting directions were chosen by magnitude.** Power iteration finds the
biggest eigenvalue by magnitude, and classical scaling needs the biggest by
value; for non-Euclidean distances — which graph distances often are — those
differ, and the code then took the square root of a negative eigenvalue as a
coordinate scale. K(5,5) is the example, and it is an ordinary graph rather than
a corner case. Claim 2 carries the fix and the test.

**And the refusal was framed as an impossibility.** The first version said a
force simulation "cannot" be a pure function at a bounded sweep count and that a
tolerance makes a picture depend on where the arithmetic landed. Neither is so:
a fixed start, a fixed step and a fixed count repeat exactly, and a tolerance on
identical arithmetic is met in the same place every time. The Context now says
what is actually true — that this method has a named objective and a monotone
descent, so the cost of its bound can be stated — and does not claim the
alternative is unavailable.

**One defect had been written down as a property.** The first version reported
that a five-node chain came out bowed by a seventh of its span and explained it
as classical scaling's horseshoe, partly straightened. It was neither classical
scaling's nor partly straightened: a path's distance table has rank one, the
code treated a missing second direction as a failure, and the fallback put the
chain on a circle. Keeping the first direction brings the same chain under one
percent. A record that explains a defect is worse than one that omits it, which
is the reason this paragraph exists.

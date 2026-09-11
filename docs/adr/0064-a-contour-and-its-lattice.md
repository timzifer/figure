# 0064 — A contour is one tracing, and the lattice under it is one resolver

**Status:** Accepted · **Date:** 2026-09-11 · **Implementation:** `stat.Lattice`, `stat.Contour`, `stat.Levels`, `geom.Contour`, `three.Contour`

## Context

[docs/chart-types.md](../chart-types.md) has carried one row of bucket F unbuilt
since the bucket shipped — *Contour | `stat.Contour` | missing* — and
[ADR 0058](0058-what-3d-is-for.md) scheduled it between the 3D scatter and the
spherical coord, with the argument that one function pays for two charts: the
flat contour plot README disclaims, and the contours on a surface's floor.

That argument is the whole record. Neither chart needs a decision of its own —
[ADR 0028](0028-distribution-stats.md) already says where a stat like this runs,
and [ADR 0054](0054-statistical-instruments.md)'s admission test lets it in
without comment. What needs writing down is what *one* function means when two
packages draw from it, and the two things that turned out to be undecided.

## Decision

### `stat.Contour` is a struct with a `Reset`

CONTRIBUTING gives two shapes. A reduction takes plain slices and returns row
numbers; a layout with working state the size of the data is a struct with a
`Reset`, the way `stat.Hex` and `stat.Sankey` are.

Contour is the second, and the reduction contract does not merely fit less well
— it does not fit at all. A contour's vertices are interpolations *between*
rows, so no row number exists for any of them.

It is **not generic over `float32`**, and that is a decision rather than an
omission. A reduction is generic so that a geom can run it on projected device
coordinates, which is where [ADR 0011](0011-decimation.md) puts decimation. A
contour runs in `Train`, in data space, where there is no `float32` caller.

**Saddles are decided by the cell's centre**, which is the mean of its four
corners — and therefore the bilinear value there rather than an approximation of
one. If the centre is on the same side of the level as the two corners that are,
those two are joined; otherwise the other pair is. The rule is a function of the
four corner values and of nothing else, so it cannot depend on which way a walk
arrived. That is what makes the flat chart and the floor projection draw the
same lines through a saddle, which is the failure this record exists to prevent.

**A cell with a non-finite corner is a wall.** A contour through a number nobody
measured is a contour through a guess, so a run that reaches a hole ends at its
edge rather than being routed round it.

**A ring's closing vertex is assigned from its opening one** rather than
computed again. `Closed` claims the last point *is* the first, and an
interpolation the compiler was free to contract into a fused multiply-add gives
a value a bit away from it — which would make the flag silently false. It is
`stat.LTTB`'s lesson in a second place.

### `stat.Lattice` is the one resolver

`three.Surface` has turned a long table of (x, y, z) rows into a product grid
since [ADR 0056](0056-three-dimensional-charts.md), and both contours need
exactly that. `three` imports `geom` and not the other way round, so the shared
code cannot live in either; `stat` needs nothing from anyone and is where
CONTRIBUTING puts a layout with working state the size of the data.

The reason to share it is not the duplication. **Two resolvers that agree today
disagree at the first duplicated position**, and the symptom would be a surface
and its own floor contours that do not line up — the exact bug the sharing
exists to prevent, and one nobody would think to look for.

It reports a **fault code rather than an error**, which is `stat.Sankey.Cyclic`'s
trade: a useful message names a column and a mark, and this package knows about
numbers. The callers spell the sentences, and `three.Surface`'s are unchanged.
One of them was found to be unreachable — a row "not on the grid" cannot happen
when the axes are the distinct values of the very columns being placed — and now
reports the case that can, a position that is not a number.

### `stat.Levels`, and not the tick search

Levels are round multiples chosen from the data: the rung of the 1-2-5 ladder
nearest *in ratio* to the range over the count, strictly inside the range.

**Nearest, not rounded up.** Rounding up is the obvious reading of "no more than
n intervals" and is much worse in practice: asked for four levels over [0, 1] it
answers with the single level 0.5, because 0.2 gives five intervals and five is
more than four.

`scale`'s extended-Wilkinson search chooses better numbers and is the wrong
tool. It optimises a **labelling** — simplicity, coverage, and density against an
axis of a given length — so the count it lands on depends on how wide the panel
is. A chart whose number of isolines changed when it was resized would break
[ADR 0011](0011-decimation.md)'s rule in the place ADR 0028 restates it. A level
list has no length to be dense against.

### The two charts share two values, not a seam

`geom.Contour` traces in `Train`, which is ADR 0028's rule: what it computes is
what the axes describe. It is also where the ramp must be trained, because a
guide is measured before the plot rectangle exists — the constraint that costs
`geom.Hexbin` its colourbar. A contour's levels are settled in `Train`, so it
**can** have one, and does.

`three.Contour` draws the same runs on the cube's floor or ceiling, one
`Sink.Line` per segment so that a floor line interleaves with the surface above
it rather than being ordered wholly in front of it or wholly behind.

They are made the same picture by the caller passing **two values** — a level
list and a `scale.ColorScale` — and not by a new seam between the packages. That
is how `geom.ColorBy` already shares a scale across the layers of a chart and
the panels of a facet, and it is the same sentence one level up.

`colorScale.Train` widens a min/max and is therefore idempotent, so sharing one
is safe however many charts train it. `scale.ColorDomain` is the recommended
spelling regardless, and for a reason that is not safety: two charts are usually
two subsets — a facet, a filter, a live window — and trained domains then differ,
so the same colour means two numbers and nothing shows that it does.

### The floor and the ceiling, and not the walls

A contour is a statement about z over the (x, y) plane. A back wall is a plane
that *contains* z, so there is no isoline to draw on it, and projecting the floor
family sideways would draw lines that mean nothing. What a wall wants is a
**cross-section** of the surface at that wall's coordinate, which is a different
layer with a different stat and a later record.

## Consequences

- **The JSON dialect gains a `z` channel**, and it is a channel rather than a
  third axis: what z carries there is a value *at* a position, the way colour is.
  A projected scene's z stays out of the dialect entirely, which is ADR 0056's
  decision unchanged. Levels are spelled apart from `bins` because there are n
  levels and n+1 bands — a document saying `"bins": 8` for a contour would read
  as though the mark binned.
- **Filled bands and labels along a line are not here.** A band is a polygon
  where this is a path, and a label on a curve is a placement problem with a
  record of its own. The doc comment says so rather than leaving it to be
  discovered.
- **`three.Contour` costs a drawing call per segment**, which is what buys the
  interleaving. On a fine grid that is real, and the level count defaults low
  for that reason.
- **`testdata/golden/surface.svg` and `views.svg` did not move** when the
  surface was refactored onto the lattice, which is the whole proof that the
  move was a move.

## Revisit if

Filled bands arrive. They are a polygon problem rather than a path one — a band
is bounded by two levels and by the edge of the lattice between them — and they
would want the tracing to promise which side of a run is the high one, which it
does not today. Both are worth settling together rather than one at a time.

The other is a **cross-section layer** for the walls, which is what declining
them here leaves open.

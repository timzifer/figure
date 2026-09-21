# 0080 — A cell is the part of a panel nearest one row, and it is cut where the reader measures it

**Status:** Accepted · **Date:** 2026-09-21 · **Implemented:** 2026-09-21

## Context

The sweep of the unusual forms in [docs/chart-types.md](../chart-types.md) has
one row left that is a *decision* rather than a recipe or a helper, and it has
been carrying its own obituary:

> **Voronoi** — Fortune's algorithm is O(n log n), deterministic and bounded,
> which is `stat.Squarify`'s shape — it was named here as the one layout the
> node-link refusal did not cover, and since
> [ADR 0077](0077-a-node-link-layout.md) that refusal covers nothing. It would
> also sharpen hit-testing ([ADR 0015](0015-hit-testing.md)). Not written up:
> nobody has asked for the chart, and the hit-test is a performance question
> rather than a form.

Both halves of that verdict are worth reading again. The first half says the
only *architectural* objection is gone: [ADR 0039](0039-relational-layouts.md)
refused layouts that run until they settle, 0077 admitted the one that descends
a named objective under a fixed budget, and a nearest-neighbour partition
descends nothing at all — it is an intersection of half-planes, which is
arithmetic with an answer rather than a search with a stopping rule. The second
half is about demand and about a *different* feature. Nobody asked for a faster
hit test; what people draw is a rainfall map, a catchment, a coverage map of
depots or cells or gauges, and none of those is a performance question.

So the form was left out for the weakest reason in the file, and it is the last
shape in the catalogue with no record. Everything else in the sweep is either
drawn, a recipe over `Rect` (Hovmöller, calendar heatmap, isotype), a stat too
small to be a decision (recurrence plot), or refused with a reason that still
holds (word cloud, Demers cartogram, area-proportional Venn from three sets on).

What makes it worth a record rather than a recipe is that it cannot be written
without answering a question no mark here has had to answer: **in what space is
a distance measured?** Every other mark either places its shapes from values
(a bar, a rect, a line) or lays them out in the unit square because the data
has no coordinates at all (a treemap, a sankey, a tree). This one has data
coordinates *and* needs a distance between them — and there is no distance
between a millimetre of rain and a kilometre of easting.

## Decision

**`geom.Voronoi` divides the panel into the part nearest each row, cut on the
panel rather than in the data, by intersecting half-planes rather than by a
sweep.** Six claims.

### 1. It is a mark over the panel's own two axes

Not a coord, and not a relational layout. The sites are `X` and `Y` columns
like a scatter's, trained on the scales like a scatter's, and placed through
`coord.Point` like a scatter's — so a Voronoi layer composes with a scatter
layer over the same table, which is how the dots get drawn (this mark draws no
dots; `geom.Scatter` already exists and already sizes them).

That is the opposite end from the relational marks, whose layouts live in the
unit square because *"a treemap's nodes have no coordinates"*
([ADR 0039](0039-relational-layouts.md)). These rows have coordinates. What
they do not have is a metric, which is claim 2.

### 2. The partition is cut in device space, in `Build`, against the panel

A cell's whole content is **equidistance**: this boundary is halfway between
those two dots. Halfway needs a length, and the two axes of a chart are not
obliged to have a common one — rainfall against easting, price against seconds,
mass against temperature. A partition computed in the scaled pair and then
stretched into the panel is *not* the same picture: an anisotropic map takes
perpendicular bisectors to lines that are not perpendicular bisectors, so the
boundaries on screen would not be halfway between anything the reader can see.

So the distance is measured where the reader measures it — on the page — and
the cells are cut in `Build`, against `Frame.Area`, after the coord has placed
the sites.

This makes it **the fourth stat that runs in `Build` rather than in `Train`**,
beside [ADR 0028](0028-distribution-stats.md)'s exception for the hexagonal
lattice, the beeswarm's offsets and the treemap's squarify — and the argument
is the treemap's word for word: *"what it optimises is a shape on screen —
packing a square and then stretching it into a wide panel would defeat the
whole algorithm."* It is also the **third mark that computes geometry in device
space**, after the dependency arrow's elbow
([ADR 0068](0068-gantt-charts.md)) and the label on a curve
([ADR 0073](0073-labels-on-a-curve.md)), and it joins them for their reason:
each of those three is a *reading aid* rather than a claim about a value, and
two of the three are about what the eye measures rather than what the table
says.

Two costs, both stated rather than worked around:

- **The picture depends on the panel.** The same twenty gauges in a panel
  narrowed by a colourbar make slightly different cells, because on the page
  the dots are differently placed. `examples/voronoi` draws exactly that — the
  same table twice, once with a colourbar and once without — because it is
  better seen than argued about.
- **The cells are not a statement about the data between the rows.** They say
  which row is nearest, and nothing about what a value would be there. A mark
  that claims the value between samples is an interpolation, which is
  [ADR 0064](0064-a-contour-and-its-lattice.md)'s lattice, and it is refused
  here on purpose (see **Not in scope**).

The cells fill the panel **rectangle**, and a coord whose panel is not a
rectangle cuts them to its own shape without being asked: `render` pushes
`coord.Clip` round the data pass, so a Voronoi under `coord.Polar` is the
nearest-site partition of the panel clipped to the disc. The mark asks the
coord nothing, and the coord learns nothing about the mark.

### 3. It intersects half-planes; it does not sweep

`stat.Voronoi` clips the panel rectangle by one perpendicular bisector per
other site — [Sutherland and Hodgman](https://doi.org/10.1145/360767.360802)'s
clip of a convex polygon, run once per pair. That is quadratic where Fortune's
sweep is O(n log n), and it is what this does anyway, for three reasons in
increasing order of weight:

- **It is exact and has no cases.** A sweep decides its picture on circle-event
  predicates, where four nearly-cocircular sites are a rounding away from a
  different topology; clipping a convex polygon by a line has one branch and no
  topology to get wrong. This repository already knows what a decision taken on
  a float32 ulp costs — `stat.LTTB`'s forced rounding is the scar
  ([AGENTS.md](../../AGENTS.md)).
- **Most of the work is skipped rather than done.** A cell lies inside the
  circle of radius *R* about its site, so a site farther than *2R* cannot reach
  it; *R* shrinks as the cell is clipped, so the test strengthens as it goes.
  What is left per pair is one distance and no polygon walk.
- **The counts are bounded by the picture, not by the machine.** A cell has to
  be big enough to tell apart from its neighbours and point at, and a thousand
  cells in a 900×600 panel are twenty pixels a side already. Past that the
  drawing is a field rather than a set of regions, and the answer to a field is
  [ADR 0066](0066-a-raster-mark.md)'s raster of the value.

So `stat.MaxVoronoiSites` is 1000 and `geom.Voronoi` refuses past it in `Train`
with `geom.ErrTooManySites`, which is `geom.NodeLink`'s manner exactly
([ADR 0077](0077-a-node-link-layout.md)): refuse the drawing that would not be
one, name the mark that answers the same question at that size. The arithmetic
agrees with the picture about where the limit is — 250 sites is about 1 ms, a
thousand about 8, two thousand about 25 and four thousand about 83, which is a
layout rather than a frame.

**It is a pure function of its input**, which is what
[ADR 0012](0012-parallel-panels.md) requires of anything a panel builds on its
own goroutine. Nothing iterates to a tolerance, nothing reads a clock, and the
*order* of the sites does not matter at all: an intersection does not care in
which order it was taken. That is a stronger property than any other layout
here has — the sankey, the tidy tree, the layered graph and the stress layout
all take the row order as an input — and it is the one place where this mark is
easier than its neighbours rather than harder.

### 4. A cell is a row, and it reports one

This is the first layout-shaped mark in the library for which that is true.
A sankey's node is what many rows have in common and a parallel-sets ribbon is
a count over rows, so both report nothing ([ADR 0074](0074-sets-are-counted.md),
[ADR 0079](0079-parallel-sets.md)); a Voronoi cell is one row's own region, so
it reports that row, at its site — inside its own cell by construction, and
where a reader points when they mean that one. A cell is bounded on every side,
so no part of it means more than another, which is [ADR 0043](0043-mark-identity.md)
and the rect's rule rather than a new one.

Two rows at the same device point are the one degenerate case, and the
bisector between them is undefined rather than awkward. The **first** of them
keeps the cell and the second draws nothing and reports nothing: two cells
drawn on top of each other would be two marks a reader cannot point at
separately. It still cuts every other cell, because it is somewhere even if it
is nowhere of its own. A row with no position is not a site at all, which is
every layer's missing-value policy.

### 5. Colour is per cell, and an outline is asked for

A colour column paints one cell at a time, through a ramp or a qualitative
palette, batched into one drawing call per colour with **one subpath per cell**
— which is what keeps [ADR 0007](0007-per-mark-colour.md)'s refusal intact and
what lets a pointer land on the cell it is inside rather than on the sheet
([ADR 0015](0015-hit-testing.md)). Filled from a ramp, the partition stops
being a map of coverage and becomes a map of the reading, which is the chart
most people mean by this form.

`GroupBy` is accepted and ignored, for the reason a histogram ignores it: there
is one partition over every row of the layer, and a series inside it is not a
partition of anything.

A cell is **outlined only where the caller named both a `Fill` and a `Color`**,
which is `geom.Rect`'s rule and is here for Rect's reason — `interact` ranks a
vertex above an area, so an outline nobody asked for would make every hover
over a partition report a corner. The treemap's alternative, padding, is not
available: insetting a polygon is an offset construction and a cell that had
been shrunk would no longer be the set of points nearest its row, which is the
only thing this mark asserts.

### 6. Nothing below `geom` learned anything

No IR primitive, no scale, no coord, no theme entry, no layout change. The
whole of it is `stat.Voronoi` — numbers in, polygons out, in a package that
knows about numbers and nothing else — and a mark that calls it between placing
its sites and filling its paths.

## Consequences

| | |
|---|---|
| `stat` | `Voronoi`, `Voronoi.Reset`, `Voronoi.Cell`, `Region`, `MaxVoronoiSites` |
| `geom` | `Voronoi`, `MarkVoronoi`, `ErrTooManySites`, and one pooled field on the build scratch |
| `spec` | the mark `"voronoi"`, reading `x`, `y` and `color` like every other mark over a pair of axes |
| `ir`, `render`, `coord`, `scale`, `theme`, `facet`, `internal/layout`, `figure`, `interact`, `a11y` | unchanged |
| docs | `examples/voronoi`, the `voronoi` gallery figure, and the sweep's last open row closed |
| Charts unlocked | Thiessen polygons, a coverage or catchment map, nearest-facility regions, and a scattered sample read as a map of its own nearest measurement |

**A faceted Voronoi is a partition per panel, and that is correct rather than
convenient.** `Subset` cuts the rows, each panel cuts its own cells against its
own plot area, and a row that is in no panel cuts nothing — which is what
"nearest" means once the reader is looking at one panel.

**A zoom or a pan redraws the partition, and the cells move.** A row that has
scrolled off the panel is not dropped: it keeps cutting the cells that remain
and it still owns the strip of panel nearest it, which is the honest answer to
"which row is nearest here" and not the same thing as drawing a row that is
off screen. That falls out of the site list being the layer's rows rather than
the panel's, and it is tested.

## Not in scope

- **Lloyd relaxation, and any centroidal or weighted diagram.** Moving each
  site to its cell's centroid and repeating is an optimiser that runs until it
  settles, which is [ADR 0039](0039-relational-layouts.md)'s refusal; a power
  diagram's weights are a second geometry with its own degeneracies, and
  nothing in the catalogue asks for one. A **Voronoi treemap** is both at once
  and is refused with them.
- **The Delaunay triangulation, and natural-neighbour interpolation over it.**
  The dual is not computed here — a half-plane intersection does not produce it
  — and an interpolated surface over a scattered sample is a *claim about the
  value between the samples*, where a cell is a claim about which sample is
  nearest. That is a lattice resolver beside
  [ADR 0064](0064-a-contour-and-its-lattice.md)'s, and it is a record of its
  own when somebody wants a contour over irregular samples.
- **A nearest-site index for hit-testing.** The sweep named it and it is a
  performance question, exactly as the sweep said: `interact` already answers
  "which mark is under the pointer" correctly, and making it answer faster for
  a point cloud is [ADR 0015](0015-hit-testing.md)'s business, with its own
  measurements, and does not need this mark to exist.
- **Labels in the cells.** A mark that wrote text would need a policy for a
  cell too small to hold its name and the placement machinery
  [ADR 0040](0040-label-collision-avoidance.md) governs. `geom.Text` over the
  same table already writes a label at each site, with collision avoidance if
  it is asked for.
- **A spherical or geographic partition.** That needs a projection, and
  [ADR 0018](0018-coordinate-systems.md) says a projection is argued on its own
  evidence rather than smuggled in behind a mark. A Voronoi over projected
  coordinates handed in as columns is this mark, drawn on the panel like
  anything else.
- **A partition of a 3D scene.** `figure/three` orders shapes back to front;
  the cells of a 3D partition are polyhedra, and there is no reading in the
  catalogue that wants one.

## Revisit if

- **Someone wants more cells than the cap.** Then the sweep is due, and with it
  the predicates written down and tested as predicates rather than as pictures
  — plus the prior question of whether a thousand-cell partition is a chart at
  all, which the raster answers differently.
- **A second customer for the geometry turns up.** A contour over irregular
  samples, or a "which row is nearest this pixel" index, would want the
  Delaunay dual rather than the cells; two customers would say the
  triangulation belongs in `stat` as its own value, with the cells derived from
  it.
- **Cells want a gap between them.** It cannot be padding without breaking what
  a cell means (claim 5), so the honest version is a stroke in the panel's
  background colour — which is a theme question and an outline this mark
  already draws.
- **A hit on a partition should report the cell rather than the site.** Today
  the polygon is the mark and the site is the reported position, which is what
  every bounded mark here does. A reader who wants the *cell's* geometry back
  is asking `interact` for a shape rather than a point, which is 0015's table.

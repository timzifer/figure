# 0058 — What the third dimension is for, and where it stops paying

**Status:** Accepted · **Date:** 2026-09-08 · **Implementation:** the whole order of work, the Smith sphere last; the ribbon, the ternary prism and the spherical histogram were never scheduled — see the annotated order of work

## Context

[ADR 0055](0055-depth-without-a-third-axis.md), [0056](0056-three-dimensional-charts.md)
and [0057](0057-orbiting-a-chart.md) decide *how* 3D is built. None of them
says what it is for, and a machinery whose charts are never enumerated is a
machinery whose scope creeps: the surface ships, then a mesh loader looks like
one more geom, then the library is a renderer.

[docs/chart-types.md](../chart-types.md) is the precedent and the method —
"sorted **by the machinery each form needs** rather than by how popular it is.
Sorted that way the list stops being a wish list and becomes a schedule." It
worked because in 2D the machinery is the hard part and the chart is a recipe
over it.

**In 3D that method is not enough on its own,** because the machinery answers a
question nobody was asking about half the forms. Whether figure *can* draw a
3D pie is not interesting; whether a reader learns more from one is, and the
answer is no. So this catalogue is sorted by a second axis first.

## Decision

**A form is ranked by what the third dimension gives the reader that the flat
chart of the same data does not. The machinery decides the order of work within
a rank; it does not promote a form between them.**

Three answers to that question, and each rank is one of them:

- **it carries a reading the 2D chart cannot** — the forms that justify 0056;
- **it carries the same reading, differently** — worth having, usually not
  worth choosing;
- **it carries less** — refused, however easy it would be.

### Rank 1 — the forms that justify the machinery

Each needs exactly what 0056 builds and nothing more: a camera, a projection,
back-to-front order over an orderable set, and a cube of furniture.

| Form | What the third dimension carries | Machinery |
|---|---|---|
| **Surface** over a grid, z = f(x, y) | the shape of a response between its samples: a ridge, a saddle, a plateau's edge. A heatmap of the same grid gives the *values* and hides which way the ground falls | 0056 exactly: the grid's own back-to-front order |
| **3D scatter**, three measured columns | whether a cluster is a cluster or two clouds that overlap along the axis you happened not to plot | 0056, marker path, depth sort |
| **Trajectory / phase space**, an ordered path in x, y, z | a path that crosses itself in every 2D projection and not in the data — an orbit, a tool path, an IMU track, an attractor | 0056, polyline per segment |
| **Cascade / waterfall**, a family of traces offset in z | how a spectrum *moves*: a peak drifting, a harmonic appearing, a resonance splitting. It is the display a spectrum analyser has had since the seventies, and it is the tier-1 form this library's existing audience will ask for first | 0056 plus nothing: a recipe over N 3D lines |

The last row is the one that decides whether 0056 pays for itself. A library
that ships `coord.Smith` ([ADR 0033](0033-smith-charts.md)), tracks, thresholds
and error bars has an RF and instrumentation audience already, and the cascade
is the chart that audience draws on a whiteboard when explaining what they
measured.

### Rank 2 — the same reading, differently

These are recipes over rank 1's machinery. They ship because they cost nothing
once it exists, and each one carries a note saying which 2D chart usually beats
it — the catalogue is not neutral about this, because a reader who chose wrong
is a reader who was not told.

| Form | Recipe | The 2D chart that usually wins |
|---|---|---|
| **Terrain / DEM** | surface + a sequential ramp on z | a hillshaded heatmap, until the reader has to judge slope |
| **Ribbon** | a 3D line with width — `three.Ribbon` | a line with a band, which is what an interval means |
| **Stem / dropline 3D** | scatter + a rule to the floor | nothing: it is what makes a 3D scatter's heights readable, so it ships *with* the scatter |
| **Projected contours on the walls** | `stat.Contour` evaluated on the same grid, drawn on the floor and back walls | the contour plot itself — see below |
| **3D bars** over two categoricals | `geom.Bar` in a projected box | **almost always the heatmap.** Bars occlude each other, the back row is unreadable, and the height of a bar behind another cannot be compared to it. It ships because refusing it invites a worse reimplementation by every caller, and its doc comment says this |

**`stat.Contour` is a 2D feature that 3D wants.** `README.md` lists contour
plots among the things deliberately not here; they are a pure function in
`stat` over a grid and a `geom` that strokes the level sets
([ADR 0028](0028-distribution-stats.md)'s shape). It is not blocked on 3D and
3D is not blocked on it — but the same function serves the flat contour plot
and the surface's floor projection, so it is scheduled between them.

### Rank 3 — the sphere, and the family it unlocks

One more piece of machinery buys a whole family, exactly as `coord.Polar` did
in v0.8. A **spherical coord** — a direction (θ, φ) and a radius, mapped into
0056's scene — is one implementation, and five forms fall out of it.

| Form | Field | What the sphere carries |
|---|---|---|
| **Antenna radiation pattern**, r = f(θ, φ) | RF | the whole pattern: main lobe, nulls, back lobe, and their relation. The two cut planes an antenna datasheet prints are the 2D fallback, and they are cuts *because* the page is flat |
| **Directivity / beam scan** | RF, acoustics | the same, swept |
| **Poincaré sphere**, the Stokes vector | optics, RF | polarisation as one point instead of three coupled numbers |
| **Bloch sphere** | quantum | a two-level state as a point, which is the entire pedagogical reason it exists |
| **Stereonet / orientation density** | structural geology | a distribution of *directions*, whose flat form is a projection of this sphere and always has been |

They are one machinery and five audiences, which is the argument for building
it — and it is a separate step from 0056 for the reason polar was a separate
step from Cartesian: the first proves the projection, the second proves it was
general.

A **geographic globe** sits here too, and is the interesting case: the v1 audit
files geographic projections as "a third `Coord` behind the same interface".
Orthographic and perspective globes are *this* sphere rather than that coord,
and the flat projections stay 2D — so the two remain separate features that
happen to share a noun.

### Rank 4 — the absolute niche, and why it is in the catalogue at all

**The 3D Smith chart.** The 2D one maps impedance to the reflection coefficient
Γ inside the unit disc, and everything with |Γ| > 1 — an active device, an
oscillator's negative resistance, an unstable region — is *off the page*. Its
3D form projects the Γ-plane onto a sphere, where the outside of the unit
circle is simply the other hemisphere: the infinite plane becomes finite, and
the one class of circuit the flat chart cannot show becomes a place you can
point at.

It is the nichest chart in this document by a wide margin, and it is here for
three reasons that matter more than its audience size:

1. **It is rank 1 by the ranking rule.** The third dimension carries a reading
   the flat chart provably cannot — that is the definition, and popularity is
   not part of it. The catalogue would be dishonest if it quietly demoted a
   form for being rare.
2. **It is the proof the sphere is general.** A spherical coord that draws an
   antenna pattern and a Smith sphere is a coordinate system; one that draws
   only patterns is a chart type with delusions.
3. **This library already shipped its 2D counterpart**, and ADR 0033 argued
   that a Smith chart was not a special case but "the polar-shaped coord seam,
   over normalised impedance". The same sentence should survive one dimension
   up, and if it does not, the sphere is shaped wrong.

Beside it, at the same distance from the mainstream and reachable by the same
machinery: the **ternary prism** (a ternary diagram extruded by a fourth
variable — metallurgy, petrology) and the **spherical histogram** over a
direction column, which shipped as `three.Histogram3`: bands of equal cos θ
with the same number of azimuth sectors round each of them, which is what makes
every cell the same size — a lat/long grid piles a uniform set of directions up
at the poles, and that is the only part of the form that needed thought. The
prism shipped as `three.Prism` once
[ADR 0051](0051-barycentric-coord.md) had built the barycentric map, and it
needed exactly what this line said it would: a scene option that places a
layer's values through a different map, and a triangular solid of furniture
round them.

### What 3D does not enable

Named because they are what "we have 3D now" invites, and each is a different
library:

- **Arbitrary meshes, STL/glTF/OBJ, CAD.** 0056's painter order is exact only
  over orderable sets; a triangle soup needs a BSP tree or a depth buffer, and
  a depth buffer needs pixels the SVG and PDF backends do not have.
- **Volume rendering, isosurfaces, voxels.** These visualise fields;
  `data.Source` hands out columns.
- **Point clouds at LIDAR scale.** Decimation is off in a projected scene
  (0056), so the reduction has to happen in `stat` before projection, and a
  point cloud's reduction is a spatial one nobody has asked this library for.
- **The 3D pie.** [ADR 0055](0055-depth-without-a-third-axis.md) refused it
  where it was cheapest to draw; a real projection does not make it truer.
- **Animated 3D as a chart type.** Turning a scene is 0057; a scene that turns
  by itself is a video, and a reader cannot compare two moments of one.

## Order of work

1. **0056's machinery, with surface, 3D scatter + droplines, and 3D line.**
   Rank 1 minus the cascade, which needs no code. — **Shipped.**
   `three.Surface`, `three.Line3` and, out of rank 2 because it costs four
   lines over the same machinery, `three.Bar3` with the doc comment this
   record asks it to carry. The 3D scatter came last, as `three.Scatter3`,
   and needed what this line said it would and nothing else: a marker became
   the sink's fourth primitive beside the face, the line and the text, and a
   dropline is one `Line` from each point to the floor. The droplines are on
   by default, because they are the reading — a point floating in a projected
   box has no height anyone can judge — and `geom.Droplines(false)` turns
   them off for a cloud dense enough that they become a curtain. A marker is
   drawn the size it was given wherever it stands: one that shrank with
   distance would be a size channel nobody asked for. Markers adjacent in the
   depth order and alike in style are one drawing call, which keeps a
   hundred thousand points at the same handful of allocations per frame as a
   thousand, and the allocation gate says so.
2. **The cascade**, as an example and a doc page rather than a geom. —
   **Shipped**, as `examples/cascade`: thirty three-dimensional lines and one
   `geom.GroupBy`, with no code in the library that knows what a cascade is.
3. **`stat.Contour`**, which pays for the flat contour plot README currently
   disclaims and for the surface's floor in the same function.
4. **0057's orbit.** Independent of 2 and 3, and the point at which the surface
   stops being a picture and starts being an instrument. — **Shipped**, as
   `three.Camera` and `three.Live`.
5. **The spherical coord**, and with it rank 3 — patterns first, because they
   have the largest audience, and the Smith sphere last, because it is the
   test that the coord is a coord. — **Shipped, the Smith sphere included.**
   `three.Spherical` makes a scene's X an azimuth, its Y a polar angle (or,
   with `three.Latitude`, the angle up from the equator) and its Z a radius;
   `examples/sphere` draws a four-element array's radiation pattern as a
   `three.Surface` and a detuned Rabi oscillation on the Bloch sphere as a
   `three.Line3` and a `three.Scatter3`, with no mark of either's own.
   Building it decided six things the line above left open:

   - **It is a scene option, not a coord.** `coord.Coord` is the stage between
     two scales and the flat IR; a projected scene never passes through it,
     and `three` already owned the one mapping its layers use. So the sphere
     is `three.Spherical`, and every layer places its geometry through
     `Frame.Place` — the identity in a box, the ball in a sphere — which is
     all a layer has to know about which space it is in. `coord.Framing.Z`,
     widened for this, stays unread.
   - **The angles are pinned to the whole sphere.** An azimuth scale trained
     on readings from 0° to 355° would stretch them round the full turn and
     close the gap by lying about every other direction, so a spherical scene
     sets its angular domains before anything trains them — in degrees, or in
     radians with `three.Radians` — and trains its radius from zero, so a
     radius is a length the reader can compare. An ordinal angle is refused.
   - **The furniture is a globe, and its order is exact.** A graticule every
     thirty degrees, the silhouette, the three axes through the centre and the
     six ends `three.AxisEnds` labels: `|0>` and `|1>` on a Bloch sphere, S₁,
     S₂ and S₃ on a Poincaré one. The far half is drawn before the data and the
     near half, fainter, after it — exact for the reason the cube's back walls
     are, since every datum is inside the ball and a point inside a ball is in
     front of its far hemisphere and behind its near one along every ray. The
     panel is announced again before the near half, so a pointer on a
     graticule line has not landed on a row of whichever layer was painted
     last.
   - **A surface closes round the azimuth, and lights from outside.** Readings
     every ten degrees from 0 to 350 leave one column of cells to draw from
     350 back to 360, and a surface whose gap is about one step wide draws it;
     a sector stays open, because joining its edges would invent directions.
     And the order of a cell's corners faces in or out depending on which way
     the angles run, so on a sphere a face's normal is turned away from the
     centre before it is shaded — a pattern is seen from outside.
   - **A scatter's droplines become radii.** A sphere has no floor, so the rule
     a `three.Scatter3` draws from each point runs from the centre instead,
     which is the arrow a Bloch vector is drawn as. `three.Bar3` and
     `three.Contour` both stand on a floor and are `three.ErrNotSpherical`.
   - **A ball is fitted by its diameter, not the cube's.** A box is scaled by
     its main diagonal, √3, so it keeps its size as it turns; a ball's every
     diameter is 1, and fitting it the same way left it filling little more
     than half its cell.

   **The Smith sphere passed the test.** `three.Smith` is a `SphereOption`,
   not a scene of its own: it reads the first two values as a normalised
   resistance and reactance, places each impedance where Γ = (z − 1)/(z + 1)
   lies on the Riemann sphere — the match at the north pole, the unit circle
   on the equator, every active impedance in the south and z = −1 at the south
   pole — and draws the flat chart's grid carried onto the ball: whole
   circles of constant resistance and reactance, negative resistances
   included, all through the open circuit because a stereographic projection
   takes circles to circles, labelled where a flat chart labels them. The
   azimuth of an impedance is the phase of Γ and its polar angle is
   2·atan|Γ|, so the coordinate system underneath is the spherical one and
   the rest is a reading and a grid. Three things the building decided:

   - **A layer places values, not positions.** A reactance runs to infinity
     both ways, and no interval a scale maps into can hold it, so the one
     mapping a layer calls became `Frame.Point(x, y, z)` over the row's own
     values rather than over positions a scale had already clamped into the
     unit cube. The box and the angular sphere map them through their scales
     as before; the Smith sphere reads them raw, and its two scales are left
     unpinned.
   - **The grid is the globe's furniture, as it is the coord's on the flat
     chart.** `coord.Smith` draws its resistance and reactance circles as
     furniture, and the curves at a value of neither axis — VSWR circles,
     constant-Q arcs — are `geom.Locus` annotations over them
     ([ADR 0050](0050-locus-annotations.md)). The sphere keeps the division:
     its grid is drawn with the globe, far half first and near half after,
     for the same exactness the graticule has, since every circle is on the
     surface of the ball.
   - **A surface over impedances does not close.** A seam is a fact about an
     azimuth, and a resistance is not one.

   `examples/sphere` draws a one-port whose resistance dips below zero near
   resonance: the sweep climbs the northern hemisphere, crosses the equator,
   runs through the southern one a flat Smith chart has no page for, and
   comes back. That is the chart this record ranked fourth and kept anyway.

The forms this record names and never scheduled are tracked rather than
forgotten: the ribbon ([#18](https://github.com/timzifer/figure/issues/18)),
the ternary prism ([#19](https://github.com/timzifer/figure/issues/19)) and the
spherical histogram ([#20](https://github.com/timzifer/figure/issues/20)), with
great-circle interpolation for a trajectory on the sphere
([#21](https://github.com/timzifer/figure/issues/21)) and angle labels on its
graticule ([#22](https://github.com/timzifer/figure/issues/22)).

3D bars land wherever they land. They are four lines over the surface's
machinery and they are the one form in this document whose main purpose is to
be available so that nobody builds a worse one.

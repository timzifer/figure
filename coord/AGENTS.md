# Working in `coord`

Notes for changing `coord`, moved here from the root [AGENTS.md](../AGENTS.md),
which holds the rules that apply everywhere — read that one first. This file
covers Smith charts, map projections, parallel coordinates, furniture,
Cartesian as the identity, break-outs — and what the coords deliberately do not
do.

## Things that will bite

**A Smith coord makes a scale's range its own domain.** `coord.Smith.Frame`
calls `SetRange(lo, hi)` with the scale's *own domain*, so that `Map` is the
identity and the pair reaching `Point` is a normalised impedance rather than a
pixel. Choosing what the interval means is what a coord does — Cartesian chooses
a distance along an edge, Polar chooses radians and pixels — so this is the
mechanism and not an exception to it. Two consequences: the coord assumes an
**affine** scale on each axis (a log Smith axis is a different chart, and the
coord does not guess which), and a zoom moves nothing, because the next `Frame`
re-derives the range from the domain it was just handed. Anything new that
assumes "a scale's range is in pixels" is wrong under this coord and probably
under the next one. See [ADR 0033](../docs/adr/0033-smith-charts.md).

**A map's default projection is the equal-area one, and that is a decision
rather than a preference.** `coord.Geo("")` and a document naming no projection
are both `coord.DefaultProjection`, which is `Mollweide`: on a map, how much
ground a thing covers reads as how much of it there is, so the three
projections that draw a region larger or smaller than its share are asked for
by name. Do not "simplify" it to the affine one because that is the cheapest
map to compute.

**A map makes a scale's range its own domain too, and then cuts it.**
`coord.Geo.Frame` calls the same `identityRange` the Smith coord does — a
projection is not separable, so the pair reaching `Point` has to be the degrees
themselves — and then *narrows* its own copy to what the projection reaches:
±85.051129° for Mercator, a turn of longitude about the centre for everything.
`Extent` reports that cut rather than the scale's range, so a mark asked to
span the axis spans the map instead of running off to the infinity a Mercator's
pole is. Three things follow. An axis here wants a **linear** scale, for the
Smith chart's reason. A place the projection has no image for — the far side of
a globe, a latitude past the cut — is **NaN**, so anything new that assumes
`Point` is finite is wrong under this coord first: the graticule is drawn by a
walk that starts a new subpath wherever the image resumes, which is why a
meridian on a globe is one shape holding several subpaths. And the latitude
clamp is *a hair inside* the cut, because the domain travels as a float32 and
the nearest float32 to 85.051129 is outside it — clamp to the cut itself and
the map loses its southern edge. Everything the coord draws for **itself** — a meridian, a parallel, the side of
a cell, the outline of the map — is walked in longitudes measured from the
centre meridian rather than in the caller's, and `geo.relW`/`relE` are that
interval. A walk in the caller's longitudes jumps the width of the picture
wherever theirs crosses the map's cut: a Pacific-centred world map has a domain
of −180° to 180° and a map from −30° to 330°, and its parallels came out drawn
straight back across the world. Data points go through `Point`, which wraps;
anything the coord builds goes through `pointAt`, which does not. Unlike a
Smith chart a map **zooms**, because its picture is derived from the domain
rather than fixed: the fit is
recomputed every `Frame` by walking the image of the domain — a lattice, plus
the rim for a globe — rather than from a per-projection formula, and one scale
factor serves both directions because a map stretched to fill its panel is a
projection nobody chose. See
[ADR 0081](../docs/adr/0081-a-map-projection.md).

**A coord holds no scales, with one exception, and it is the one with more
than two axes.** `coord.Parallel` carries a scale per dimension because that is
what a parallel-coordinates panel is; every other coord borrows the panel's two
and re-ranges them (`smith.identityRange`, `ternary.pin`). Two things follow
and both are load-bearing. A mark reaches those scales through
`coord.Dimensions` — `geom.Training.Dims` at train time, `f.Coords()` at build
time — rather than through anything new on `Coord`, which never gains a method.
And the panel's own ticks are silenced **by the coord**: left to themselves
they would number an axis index and a fraction of an axis. A theme that turns
them off as well gives back the gutter a panel reserves for tick labels it
never writes, and the dimensions keep their numbers through it because
`Furniture.FamiliesAreTheAxes` says the families *are* the axes — ADR 0070's
label gate, amended. A coord whose family is a third reading beside two
labelled axes leaves that flag alone, which is why a ternary chart is
unchanged. See
[ADR 0078](../docs/adr/0078-a-coord-with-more-than-two-axes.md).

**A coord may draw one grid line per tick, and may label nothing the scale did
not.** `render.drawAxes` walks `for i, t := range xTicks`, takes the geometry
from `fur.GridX[i]` and the *text* from `t.Label`. That is why a Smith chart's
columns are an impedance rather than the reflection coefficient an instrument
reports: with Γ on the axes the impedance grid would have no tick behind it.
Before reaching for a second grid family, read ADR 0033's "Revisit if" —
widening this is one decision, not several.

**And check first whether the family is furniture at all.**
[ADR 0050](../docs/adr/0050-locus-annotations.md) is the answer for the four
families 0033 declined, and it widened nothing: a curve given by a formula
rather than by a tick is an *annotation* defined in data space, so the coord
draws it through `Point` like any other mark and `render` never sees it. That is
what `geom.Locus` is, and it is why there are VSWR circles now and still no
second tick list.
A `Shape` may also hold more than one subpath, which is how a ternary chart's
third grid family is drawn without a third tick list
([ADR 0051](../docs/adr/0051-barycentric-coord.md)). What genuinely needed the
wider seam was a labelled *furniture* family with no tick behind it — a
ternary's third ladder, and what everyone assumed was a projection's graticule
— and [ADR 0070](../docs/adr/0070-a-third-labelled-family.md) spent it once for
all of them. [ADR 0073](../docs/adr/0073-labels-on-a-curve.md) had narrowed it to
that much first: a family drawn as a **mark** writes its own levels along its
curves now, with the curve gapped for the text, and `render`'s one tick list
per axis is untouched. The graticule turned out not to be a customer: with
degrees on the axes a meridian *is* a longitude tick's own shape, exactly as a
polar ring is a Y tick's. What a map does raise as a family is the **edge of
the map** — a globe's rim is where the sphere turns away rather than a meridian
anybody can label, and it is drawn nowhere else
([ADR 0081](../docs/adr/0081-a-map-projection.md)).

**A locus is sampled against the panel, and the Nichols families are refined
rather than walked.** `stat.Extent.Steps` is a tolerance rather than a count for
them: a Nichols chart is the log-polar view of a circle, and the arc that passes
near the origin is a hair of the circle and is the whole plunge to −∞ dB, while
the rest of the same circle is a smooth curve a few dozen samples describe. A
uniform walk resolves one or the other — the −1° N contour sampled uniformly
stops dead at −6 dB, hanging in mid-air well above the bottom of the panel — so
`nichols.refine` bisects until the step is a few pixels and stops at a step that
leaves the window. Do not "simplify" the window rule away: it is also what
*bounds* the refinement, because the arc approaching the origin is self-similar
and a rule that only looked at the step would halve for ever. The other half is
`emit`'s ninety-degree break, which is how the branch point at L = 0 is found
without looking for it. See [ADR 0050](../docs/adr/0050-locus-annotations.md)'s
amendment.

**A coord is a value, and `Frame` hands one back rather than moving the
receiver.** `coord.Coord.Frame(coord.Framing{...})` returns the coord positioned in that
panel; the chart's own coord is never written to. That is not style — panels are
built on separate goroutines (ADR 0012), and a coord that remembered which panel
it was in would be a data race with no `Snapshotter` to fix it. `render.Panel`
may carry a coord of its own; a facet's panels do not, because the panels of a
facet are one plot over different rows, and a grid's cells do, because they are
separate plots sharing a canvas.

**`Cartesian` is the identity, and the golden files are the proof.**
`cartesian.Area` writes the same four corners in the same order as
`ir.Path.Rect`, `Edge` is one `LineTo`, `Clip` is the rectangle, and a straight
run still reaches the backend as a `Polyline` rather than as a stroked path.
Every one of those is load-bearing: the argument that the coordinate stage
changed nothing is that the committed golden files still match, and each of them
is a way to break it silently. `geom.strokeRun` is where the polyline/path
choice lives; do not "simplify" it into always building a path.

**The per-point call has a batch form and the geoms use it.**
`coord.Coord.Points(dst, xs, ys)` is called once per run, not once per row: a
per-row interface method is the shape that cost a million allocations on a
million-row column once already, and `scratch.marks` gathers the surviving rows
into a contiguous pair so there is one call. `BenchmarkPolar1k` against
`BenchmarkPolar100k` in `.github/scripts/allocgate.awk` and
`TestAPolarRenderDoesNotAllocatePerPoint` are the gates, and a polar coord does
not decimate — so those benchmarks really do draw every row.

**Furniture is filled, not returned, and `Reset` reaches past the length.**
`render` keeps one `coord.Furniture` in a pool for the whole furniture pass and
resets it between panels. `coord.resetShapes` empties every shape the slice has
*ever* held rather than only the ones inside its current length, because
`side.next` hands those back out on the next frame — a shape still holding last
frame's points would draw them again, on a chart with fewer ticks than the last
one. Returning the struct by value instead costs a steady-state frame about
twenty allocations; that was measured, and it is why the signature differs from
the one ADR 0018 sketched.

**A break-out is a displacement the coord computes and the geom applies.**
`coord.Exploder` answers how far a mark moves when it is broken out of the
middle; `geom` moves the points the `Area` call appended, in place. That split
is the coordinate stage's own: a coord reports geometry and does not draw. Two
consequences are load-bearing. `coord.Cartesian` deliberately does **not**
implement `Exploder`, so `geom.Explode` on a Cartesian chart draws exactly what
it drew — that silence is what keeps every golden file unchanged by an option
every geom now accepts, and making Cartesian answer would move every bar of a
layer the same way, which is a translation rather than a reading. And the
displacement is applied to the mark's path rather than to its extent: a
broken-out slice keeps the angle and the radii the data gave it, and growing the
radius instead would move the ink *and* the reading. See
[ADR 0026](../docs/adr/0026-breaking-a-mark-out.md).

## Deliberately not done

Deliberately not done. A Smith chart's **coord** draws no constant-|Γ|
(VSWR) circles and no constant-Q arcs — each would be a third grid family
against two tick lists, per the trap above. They are drawn as `geom.Locus`
layers instead, which is an annotation and not furniture
([ADR 0050](../docs/adr/0050-locus-annotations.md)); the **combined ZY overlay** is
one more family nobody has written. The chart still reads a **normalised
impedance** rather than the reflection coefficient a VNA reports, for the same
tick reason. `coord.SmithZ` is the bridge. It does not implement `coord.Exploder`:
the middle of a Smith chart is a matched load, not an origin of magnitude, so
there is no direction away from it that means anything. And it does not zoom.

Deliberately not done, under a **geographic projection**, which exists now:
`coord.Geo` ships **no geography** — a coastline, a border and a country's fill
are rows in the caller's table, because a shapefile or a GeoJSON reader is a
package that is allowed dependencies and the core has none — and **no
spherical arithmetic**: a great circle and a rhumb line are two different
claims about one pair of places, so the waypoints are the caller's rows and a
coord that picked one would be answering a question about the world. A path
crossing the **antimeridian** is drawn across the map rather than wrapped round
it, for the same reason: two consecutive rows at 179° and −179° are two degrees
apart or three hundred and fifty-eight, and the pair does not say which. There
is **no mark that fills a closed ring of rows**, which is what a choropleth of
country shapes wants: that is a mark rather than a coord, with four questions
of its own — which row a region reports, how a ring is named, what a hole is,
whether a vertex is a mark — and a record of its own when somebody has
geography to draw. A gridded field is `geom.Rect` and draws today. There is
**no conic projection**, because Albers and Lambert are configured by two
standard parallels and `coord.Desc` carries no such field yet; adding one is
additive. And the map **does not decimate**, because a pixel column is a band
of longitude under two of the four projections and of nothing in particular
under the other two ([ADR 0081](../docs/adr/0081-a-map-projection.md)).

Deliberately not done. A polar coord **does not
decimate**, per that record's fourth property. `layout` is **untouched** — a
polar coord inscribes itself in whatever rectangle the solver gives it, and
gutter rules that understand a radial axis are a later milestone. A radar's
contour closes because `geom.Closed` says so, not because the coord guessed:
whether a series wraps is a fact about the series, and a polar time series
spiralling through three revolutions does not wrap. And a curve is hit-tested as
its control polygon, so a filled shape can be pointed at a little way outside
its ink at a bulge.

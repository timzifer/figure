# 0081 — A map projection is a coord that is handed degrees, and its graticule is the two tick lists a panel already has

**Status:** Accepted · **Date:** 2026-09-21 · **Implemented:** 2026-09-21

## Context

A geographic projection is the last item in this repository that is deferred in
one line in four places and argued in none of them.
[CONCEPT.md §14](../../CONCEPT.md#14-what-is-built-and-what-is-next) files it
under "more coordinate systems" and says it is "the wider one";
[the v1 audit](../v1-api-audit.md) files it as "a third `Coord` behind the same
interface"; [ADR 0080](0080-nearest-neighbour-cells.md) repeats the deferral
from the other side, declining a spherical partition because *"that needs a
projection"*; and [ADR 0018](0018-coordinate-systems.md) sets the terms, in a
"Revisit if" that turns out to have predicted the whole of this record:

> **Geographic coordinates arrive.** A projection is a transform on every point
> with no linear interval underneath it, and it will want `Frame` to hand a
> coord the data domain rather than a mapped position. That is a wider seam
> than this one, and it should be argued on its own evidence rather than
> smuggled in as a third `Coord` implementation.

This is that argument.
[AGENTS.md](../../AGENTS.md) states the objection in full:

> There is no **geographic projection**: a projection transforms every point
> with no linear interval underneath it, which is a wider seam than this one,
> and ADR 0018 says it is argued on its own evidence rather than smuggled in as
> a third `Coord`.

That sentence names two things, and this record exists because **both of them
have since been paid for by work done for other charts**, three months and
seven records apart, with nobody noticing that the bill was settled.

**"No linear interval underneath it."** Every coord in this package until 0033
took two *independently mapped* fractions: `Cartesian` reads them as distances
along two edges, `Polar` as an angle and a radius, `Oblique` as two edges and a
depth. A projection cannot be written that way, because it is not separable —
the device *x* of a place depends on its latitude as much as on its longitude,
and two numbers that arrived already mapped cannot be un-mapped back into a
pair to project. [ADR 0033](0033-smith-charts.md) hit exactly this wall for the
reflection coefficient, which is a Möbius map of two variables at once, and the
answer it found is the mechanism rather than an exception to it: `Frame` gives
each scale **the range its own domain already is**, so `Map` is the identity
and what arrives at `Point` is the impedance itself — which is 0018's *"hand a
coord the data domain rather than a mapped position"*, arrived at for a chart
nobody thought of as a map. The same move, applied to a sphere, hands a coord a
longitude and a latitude. The seam was widened for the Smith chart; a map is
its second customer, and it needed no widening of its own.

**"Its graticule has no tick behind it."** This is the objection that was
actually load-bearing, and it is the one four records circled. 0033 refused Γ
as a Smith chart's input and refused the VSWR circles as furniture because a
grid family with no tick behind it has nothing to be labelled by;
[ADR 0050](0050-locus-annotations.md) declined to widen anything and named
*"the one thing that genuinely does need the wider seam — a projection's
graticule, which has no tick behind it and is not a data-space curve"*;
[ADR 0073](0073-labels-on-a-curve.md) narrowed it further; and
[ADR 0070](0070-a-third-labelled-family.md) spent it — `coord.Family` carries
its own levels, its own label positions and its own text, and 0070's list of what
the seam was spent for names *"a projection's graticule: meridians and
parallels are families with their own levels and their own labels, which was
the third customer and the reason the seam was worth spending once rather than
three times."*

The implementation found that 0070 was right that the seam was needed and
wrong about what for. **Once the axes are degrees, a graticule has ticks behind
it**: a meridian is what a longitude tick looks like after the projection has
had it, exactly as a polar ring is what a Y tick looks like after a polar coord
has had it. What has no tick behind it is the thing none of the four records
mentioned — **the edge of the map**. A globe's rim is where the sphere turns
away rather than a meridian anybody could label, and it is drawn nowhere else.
So the family seam is spent here after all, on one unlabelled line.

The third reason to write this now is that it is the last coordinate system in
the catalogue, and the only entry in
[docs/chart-types.md](../chart-types.md) whose absence is architectural rather
than a matter of somebody's afternoon.

## Decision

**A map projection is a coord: `coord.Geo(projection)` reads a panel's two axes
as a longitude and a latitude in degrees, places the pair by one of four named
projections, and fits the result into the panel without stretching it.** Eight
claims.

### 1. The axes are degrees, and the coord is handed them

`Geo.Frame` calls `SetRange(lo, hi)` with each scale's own domain, which is
`smith.identityRange` unchanged — so a linear scale's `Map` is the identity and
the pair reaching `Point` is a place on the sphere rather than a fraction of a
panel. It follows, as it does for a Smith chart, that an axis here wants a
**linear** scale: degrees on a log axis are a different chart and the coord does
not guess which.

Three things fall out of the axes being ordinary axes, and they are most of
what makes this cheap:

- **The graticule is the ticks.** `scale.TickValues(-180, -120, …)` is how a
  chart asks for the graticule an atlas prints, and an axis left to itself
  draws a correct graticule at unfamiliar levels. Nothing in `render` changed.
- **A map crops like any other chart.** A regional map is a narrower domain.
  `examples/map` draws its Mercator over 20°N–80°N because that is the ground
  the route crosses.
- **A map zooms.** This is where it parts company with the Smith chart, which
  reports itself [`coord.Fixed`](0033-smith-charts.md) because its picture is
  the unit disc whatever the domain says. A map's picture is *derived* from the
  domain — the fit is recomputed from it every `Frame` — so a pan or a zoom
  moves the map exactly as it moves a Cartesian one, and `coord.Steerable`
  answers true.

### 2. The projection is a name, and the set of names is closed

`coord.Projection` is a string-typed name with four values, not a function.
That is [ADR 0041](0041-qq-plots.md)'s rule — *a named member of a closed
family serialises and an arbitrary Go function does not* — applied to the one
stage of the pipeline that had not yet needed it, and it is what lets
`{"coord": {"type": "geo", "projection": "mollweide"}}` be a chart rather than
a chart with a hole in it.

Four, each earning its place by what it refuses to distort:

| | preserves | gives up | what it is for |
|---|---|---|---|
| `Mollweide` | area | shape at the corners | a quantity per unit area, where equal readings must cover equal ink — **the default** |
| `Mercator` | angle | area, without bound | a bearing: the rhumb line is straight, which is what the chart was built for |
| `PlateCarree` | nothing | shape and area | the projection a table of degrees is already in, and the only affine one |
| `Orthographic` | neither | half the world | the view of the globe as a planet, from far away |

**The default is the equal-area one**, `coord.DefaultProjection` = `Mollweide`,
and that is a claim about defaults rather than about projections. On a map, how
much ground a thing covers reads as *how much of it there is*: a reader
comparing two regions is comparing ink. The other three all get that wrong —
Mercator without bound, the equirectangular one by the cosine of the latitude,
the globe by the foreshortening at its rim — and each is a fine thing to ask
for by name, because the asking says the chart is about bearing, or about the
degrees themselves, or about the world as a planet. None of them is a fine
thing to get by not choosing. It is the same rule as everywhere else here: a
chart that misleads is asked for on purpose, never arrived at by default.

A fifth projection is a `Coord` of its own, registered with `coord.Register` —
the extension model every other model type here has
([ADR 0029](0029-extension-model.md)) — rather than a value this package would
have to grow. A name nobody defined **draws no map at all**, and `FromDesc`
refuses it with `coord.ErrUnknownProjection`: every position on a map is a
projection's worth of wrong under a different projection, so falling back to
the nearest one would be a chart nobody can see is wrong.

### 3. The map keeps its shape, and the panel gives up the difference

One scale factor serves both directions, chosen so that the image of the domain
fits inside the panel, and the map is centred in what it was given. A panel
wider than the map keeps the room over at the sides.

This is not a layout nicety. The whole content of choosing between projections
is the shape each one makes; a map stretched to fill its panel is a fifth
projection nobody asked for, under which a Mollweide is no longer equal-area
and a globe is no longer a circle.

The fit is found by **walking the domain rather than by a formula**: the image
of a 33 × 33 lattice over the domain box, plus — for a globe — the rim, which
no interior sample lands on and which is a circle of known radius rather than
something to search for. Four projections would otherwise be four sets of
extremal formulas to get subtly wrong, and a fifth would need a fifth.

### 4. The graticule is the two tick lists; the outline is the family

A meridian is the image of an X tick walked from the southern edge of the
domain to the northern one, and a parallel is the image of a Y tick walked
west to east. Both go into `Furniture.GridX` and `GridY`, one shape per tick in
tick order, like every other coord's grid — so the theme's grid switch, the
minor ticks and the label thinning all work on a map without knowing what one
is.

**Where the numbers go is the one question a rectangle does not ask.** A
Cartesian panel writes its X labels along the bottom edge because that edge is
a line; a Mollweide's southern edge is a *point*, where every meridian meets,
and a globe has no southern edge at all. So each ladder is written along the
longest line of the other ladder on the page — the widest parallel, the tallest
meridian — with a tie going to the edge. On a rectangular map that rule
reproduces the atlas exactly (the numbers sit outside the picture, along the
south and west edges); on a Mollweide it puts the meridians' numbers on the
equator, which is where an atlas puts them and the one parallel they do not
pile up on; on a globe it puts them on the equator and the central meridian.
Where the numbers land inside the picture, `Furniture.AxesOverData` says so and
`render` strokes them after the marks, which is the polar radial axis's rule
([ADR 0018](0018-coordinate-systems.md)) and not a new one.

The **outline** — the ellipse of a world map, the rectangle of a Mercator, the
rim of a globe — is a `coord.Family` with one line and no labels. It is the
only piece of a map's furniture with no tick behind it, and therefore the only
thing here that needed [ADR 0070](0070-a-third-labelled-family.md)'s seam. A
globe without it has no edge.

### 5. What has no image is NaN, and what the projection cannot reach is cut

Half the world is behind a globe, and a Mercator runs to infinity at the poles.
Both are answered rather than fudged:

- **A place with no image is `NaN`**, which is what every backend and every
  scale in this repository already treats as nothing to draw — `smith.Point`
  has answered that way for the pole at *z* = −1 since 0033. A graticule line
  that crosses the horizon is drawn in the pieces that are visible, because the
  walk starts a new subpath where the image resumes.
- **`Frame` cuts the domain to what the projection reaches**: ±85.051129° for
  Mercator, which is where the square tile of the web-map convention ends.
  `Extent` reports the cut rather than the domain, so a mark asked to span the
  axis spans the map instead of running off to an infinity.
- **`Invert` answers `NaN` off the map.** The four corners of a panel holding a
  Mollweide are not places, and a tooltip there must say nothing rather than
  the plausible pair an unchecked inverse would produce.

### 6. An edge between two rows is a chord; a side the coord draws itself is not

The default is `Straight() == true`: an edge between two rows is the straight
line between them on screen. That is the Smith chart's default and its
argument — two samples are what was measured and the line between them is a
convention — and here it is stronger, because the alternative reading is not
one curve but two: the great circle between two places and the path of constant
bearing between them are different routes, and a coord that quietly picked one
would be answering a question about the *world* that only the caller can
answer. `coord.GeoArc` asks for the image of the straight line in degrees,
which is what a boundary given by sparse vertices means.

What the coord draws **itself** follows the graticule whatever the edge policy
says. `Area` — the image of a box in degrees, which is what a
[`geom.Rect`](0018-coordinate-systems.md) cell is — walks its four sides along
the parallels and meridians that bound them, for `smith.Area`'s reason word for
word: a shape's outline is a claim about the region it encloses, and a chord
across a curved side encloses the wrong ground. On an equal-area projection
that is the difference between a field and a field painted over its
neighbour's ground.

### 7. It ships no geography, and it computes no routes

There is no coastline in this repository and there will not be one: a shapefile
reader or a GeoJSON parser is a package that is allowed dependencies, and the
core module has none ([ADR 0001](0001-module-layout.md)). A country outline is
rows in the caller's table like any other rows.

Nor does the coord compute a path between two places. `examples/map` draws a
great circle and a rhumb line between Edinburgh and Tokyo; both are the
example's own rows, sixteen lines of arithmetic each, and the chart's whole
point is that they are different claims about the same pair of places.

For the same reason a path that crosses the antimeridian is drawn **across** the
map rather than wrapped round it. A coord is handed two numbers; consecutive
rows at 179° and −179° are two degrees apart or three hundred and fifty-eight,
and nothing in the pair says which. Splitting the track is the caller's, where
the answer is known.

What the coord will not do is make that mistake in its own furniture, and
avoiding it is the one structural thing in the implementation. Every curve the
coord draws for itself — a meridian, a parallel, the side of a cell, the
outline — is walked in **longitudes measured from the centre meridian**, not in
the caller's. A Pacific-centred world map is the ordinary case: its domain runs
from −180° to 180° and its *map* runs from −30° to 330°, so a parallel walked
in the caller's longitudes would reach the right-hand edge of the picture,
wrap, and be drawn straight back across the world. The frame is set once, in
`Frame`, and a domain that straddles the map's own cut is drawn as the whole
turn — which is what it is, in two pieces with the map between them.

### 8. Nothing else learned anything

No IR primitive, no new mark, no `geom` change, no `render` change, no theme
entry, no layout change. The marks that draw maps are the marks that exist:
`geom.Scatter` is a point map, `geom.Line` a track or a route, `geom.Rect` a
gridded field, `geom.Text` the place names, `geom.Voronoi` the catchment of a
network of stations — cut on the panel, which under a projection is exactly
where 0080 argued a distance has to be measured.

## The globe this is not

[ADR 0058](0058-what-3d-is-for.md) says of the geographic globe that
*"orthographic and perspective globes are this sphere rather than that coord,
and the flat projections stay 2D — so the two remain separate features that
happen to share a noun."* That sentence stays true, and both halves now exist,
so it is worth saying which is which.

`three.Spherical` is a **scene**: data on a ball, with a depth order, a camera
the host turns, and the far hemisphere *drawn* — behind the data, because every
datum is inside the ball. It is an antenna pattern, a Bloch sphere, a
stereonet's directions.

`coord.Geo(coord.Orthographic)` is a **map**: one flat panel, no camera, and the
far hemisphere not drawn at all because the projection has no image for it.
Every flat mark, the guide column, the legend, hit-testing, facets and the JSON
document work on it because it is a coord and they already work on coords.

A reader who wants to turn the globe wants the scene. A reader who wants a map
that happens to be a disc wants the coord.

## Consequences

| | |
|---|---|
| `coord` | `Geo`, `Projection` with `PlateCarree`, `Mercator`, `Mollweide`, `Orthographic`, `Projection.Known`, `GeoOption`, `GeoCenter`, `GeoArc`, `GeoChord`, `TypeGeo`, `ErrUnknownProjection`, and three fields on `Desc` |
| `spec` | `"type": "geo"` with `projection`, `centerLon`, `centerLat` and the `edge` field two coords already share |
| `geom`, `stat`, `ir`, `render`, `scale`, `theme`, `facet`, `internal/layout`, `figure`, `interact`, `a11y`, `three` | unchanged |
| docs | `examples/map`, the `map` and `globe` gallery figures, `BenchmarkGlobe1k`/`100k` gated flat, and the catalogue's last architectural gap closed |
| Charts unlocked | a point map, a route map, a gridded field of any measured quantity, a catchment over a network of stations, a world map in three projections and the globe as a planet |

**`Decimates` is false.** A bucket of equal width on screen is a bucket of equal
longitude under the two cylindrical projections and of nothing in particular
under the other two, so a reduction defined over pixel columns would be
measuring something other than what it was defined to measure
([ADR 0011](0011-decimation.md)). A million-row map is a raster or a binned
count rather than a decimated scatter, so saying no costs nothing this coord is
for.

**Hit-testing works because `Invert` does.** A pointer over a map reads back the
degrees under it, which is `interact`'s existing path; what is new is that it
can read back *nothing*, and the ordinary NaN policy covers that.

**A faceted map is a map per panel**, each fitted to its own plot area, which
means two facets of different shapes draw the same map at two sizes rather than
two different maps. That is the fit doing its job and is the same statement as
0080's note about a panel narrowed by a colourbar.

## Not in scope

- **A filled ring of rows — the mark a choropleth of countries wants.** A
  `geom.Line` with `Closed` strokes a boundary today and nothing fills one.
  That is a *mark*, not a coord: it has to answer which row a region reports,
  how a ring is named, what a ring with a hole in it is, and whether the rows
  of a ring are marks of their own — four questions with nothing to do with
  projections, and a record of their own when somebody has geography to draw.
  A gridded field is `geom.Rect` today and is what `examples/map` draws.
- **Map data, in any format.** See claim 7. It belongs in a module that may
  have dependencies, next to `arrow/v18`.
- **A conic projection, and any projection with parameters.** Albers and
  Lambert conformal conic are configured by two standard parallels, which is a
  shape of configuration `Desc` does not have; adding it is additive and waits
  for somebody who needs the projection rather than the field.
- **A projected coordinate system — metres, eastings and northings, EPSG.** A
  chart of projected coordinates is a Cartesian chart of two columns and
  already draws; what it would want from here is the *inverse*, to label its
  axes in degrees, which is a different feature with a different seam.
- **Clipping and wrapping at the antimeridian.** See claim 7.
- **A geodesic, a rhumb line, a buffer or any other spherical construction.**
  Those are arithmetic over a table, and `stat` is where a reduction over rows
  lives — if one ever earns a place there it will be because several charts
  wanted the same one, which is [ADR 0054](0054-statistical-instruments.md)'s
  admission rule.
- **A spherical Voronoi, and the graticule as a `geom.Locus`.** The first is
  0080's own refusal restated: a partition on the sphere is cut with spherical
  bisectors, and this coord cuts nothing. The second would be a second way to
  draw the graticule, and this one is furniture.

## Revisit if

- **Somebody has geography to draw.** Then the ring mark above is the record to
  write, and it will want to say something about holes that nothing here does.
- **A projection with parameters is wanted.** `Desc` grows a field, which is
  additive within v1, and `Projection` grows a value at the end of its list.
- **A cylindrical map wants decimation.** `Decimates` is per coord and could be
  true for `PlateCarree` and `Mercator`, where a pixel column really is a band
  of longitude. It is false for all four today because the honest answer for
  the other two is no and a per-projection answer needs a reason beyond
  symmetry.
- **The graticule wants ink or a density of its own.** Today it is the axes'
  ticks, so it is drawn in grid ink at the levels the axes chose. A map that
  wants a graticule every 10° under labels every 30° is asking for a family
  with its own levels — which is 0070's seam, already spent here on the
  outline, and a small change rather than a new decision.
- **Labels want to sit outside the outline.** A globe's numbers ride the
  equator and the central meridian because those are the longest lines on the
  page; an atlas sometimes prints them round the rim instead, which is a third
  placement rule and would want a reason better than taste.

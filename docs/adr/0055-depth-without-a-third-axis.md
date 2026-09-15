# 0055 — A mark gains volume before a chart gains a dimension

**Status:** Accepted, amended · **Date:** 2026-09-08 · **Implementation:** `coord.Oblique`, `coord.Extruder`, `geom.Extrude`

## Context

Three documents in this repository say the same thing about 3D and none of them
says what it is. [CONCEPT §5](../../CONCEPT.md#5-non-goals) has "**No 3D until
well after v1.0**, and only then tightly scoped"; §14 files "3D
(surface/scatter3d) — deliberately late, tightly scoped" under the roadmap;
[the v1 audit](../v1-api-audit.md) parks it in one table cell — "3D | Its own
module | later". `README.md` lists it among the things deliberately not here.

Four years of "later" is a decision that was never taken, and it has stayed
untaken because **three unrelated features share the name**:

1. a chart of two variables **drawn with volume** — extruded bars, a slab of a
   panel seen from a corner. Depth carries nothing; it is ink;
2. a chart of **three variables**, x, y and z, projected onto the plane — a
   surface, a 3D scatter, a ribbon;
3. the same chart **turned with the mouse**, at framerate.

They have nothing in common but the word. The first touches one coord and two
geoms and cannot change what a chart *says*. The second needs a third scale,
which the frozen `geom.Frame` cannot carry ([ADR 0029](0029-extension-model.md)).
The third needs a camera, a frame loop and an answer about which backends can
even have one. Priced together they look like a rewrite, which is exactly why
they kept being deferred as a unit.

So they are three records, and this is the first:

| # | What | Blast radius |
|---|---|---|
| 0055 (this) | Depth as decoration: an oblique coord and extruded marks | one coord, two geoms, the IR untouched |
| [0056](0056-three-dimensional-charts.md) | A third scale, projected: surface, scatter3d, line3d, bar3d | three widened seams and a `v2` tag, the IR still untouched |
| [0057](0057-orbiting-a-chart.md) | Turning it with the mouse | that module's live loop; no core change |

Each is worth having on its own, and each is the honest prerequisite for the
next. This one is worth having because it is the whole of what most people
asking for "3D bars" are asking for, and it costs almost nothing.

## Decision

**Depth is a displacement with no meaning attached. It is chosen once per
chart by the coord, applied per mark by the geom, and no column may be bound
to it.**

That last clause is the record. A chart drawn with volume is a chart of two
variables, and the moment a third column could set the depth it would be a
chart of three variables drawn without a scale, without ticks and without a
legend — a quantity the reader can see and cannot measure. There is no
`geom.ExtrudeBy(col)`, and there will not be one: [0056](0056-three-dimensional-charts.md)
is where a third column gets an axis, and building a nameless version of it
here would poison that name before it was used.

### `coord.Oblique` is Cartesian with a depth vector

```go
figure.New(src).
    Coord(coord.Oblique(coord.Depth(14), coord.DepthAngle(-math.Pi/4))).
    Layer(geom.Bar(src, geom.X("quarter"), geom.Y("revenue"), geom.Extrude(true)))
```

`Oblique` embeds `cartesian`. Its `Point` is the identity its parent's is, its
`Straight` is true, its `Invert` is the inverse it always was, its `Edge` is
one `LineTo`, and `Decimates` stays true — a pixel column is still a pixel
column, so [ADR 0011](0011-decimation.md) keeps its guarantee unreduced. What
it adds is one method, behind the optional interface [ADR 0026](0026-breaking-a-mark-out.md)
established the shape of:

```go
// Extruder is implemented by a coord that sees its panel from an angle: a
// mark drawn under it has a back as well as a front.
type Extruder interface {
	// Extrude reports the device offset from a mark's front face to its
	// back one. It is the same vector for every mark in the panel.
	Extrude() (dx, dy float32)
}
```

`Cartesian` deliberately does not implement it, exactly as it declines
`Exploder`: a layer that asks to be extruded under a coord with no angle to
see it from draws precisely what it always drew, and nothing is invented. A
geom resolves the interface once per `Build`, never per mark.

**`Oblique.Frame` insets the panel rectangle by the depth vector before it
frames the scales.** The volume comes out of the plot area, not out of the
margins: the axes still bound the data, the silhouettes stay inside `Clip`, and
the layout solver of [ADR 0010](0010-panel-layout.md) is not asked a new
question.

**The front plane is unforeshortened, and that is the only reason any of this
is allowed.** An oblique projection moves the back face and leaves the front
one alone, so a bar's height in pixels is the height it would have had on a
flat chart, and two equal bars are equal wherever they stand. A perspective
projection — a vanishing point, a camera with a field of view — is refused
here and refused again in 0056: it makes the same value taller at the front of
the scene than at the back, which is the misreading that gave 3D charts their
reputation, and it would be one this library shipped on purpose.

### Three faces, one row, and one more area under the pointer

An extruded bar is a front face, a top face and a side face, filled in the
mark's colour and two fixed shades of it from the theme (`theme.DepthTop`,
`theme.DepthSide`, mixed in the linear-light space `palette` already mixes in —
there is no light model, no normals and no material).

[ADR 0015](0015-hit-testing.md) indexes **one mark per subpath**, so the
pointer sees three areas where it used to see one. That is correct rather than
unfortunate: each face is a place a reader can point, and all three answer with
the same row, because `Frame.Marks` still reports one position per row — the
front face's anchor, the position that row would have had on a flat chart. The
one visible consequence is that `Live.Select` gathers rows per mark, so a
rectangle over an extruded layer would name a row once per face it covered; the
row list a `Select` reports becomes a set.

### Paint order within the layer is decided, not left to chance

Extruded marks overlap, so a layer draws its marks back to front: sorted by the
projection of each mark's anchor onto the depth vector, ties broken by row
index. The tie-break is not decoration —
[ADR 0012](0012-parallel-panels.md) requires that nothing a chart draws depends
on scheduling, and a sort with an undefined order on equal keys is a golden
file that changes when the runtime feels like it.

The sort is over marks, not over pixels. There is no depth buffer here, and
[0056](0056-three-dimensional-charts.md) explains why there cannot be one under
a vector IR.

### The extruded pie is refused

`geom.Bar` and `geom.Rect` extrude. `geom.Arc` under `coord.Polar` does not,
and the refusal is the second half of this record's argument.

An extruded bar keeps its height, so the quantity survives the decoration. A
tilted pie does not: foreshortening the disc into an ellipse makes the near
slices cover more area than the far ones for the same angle, and area is what a
reader estimates a pie with. The chart would be lying in the one channel it
has. A library that draws a mark whose whole purpose is to distort a reading
cannot then say — as `docs/chart-types.md` does throughout — that it sorts
charts by what they cost the reader.

So `coord.Oblique` does not implement `Extruder` for a polar panel, because it
is not a polar coord, and `coord.Polar` does not gain the method. A layer
asking for it there draws a flat pie, which is a pie.

### What it costs elsewhere

- **The IR does not change.** Three faces are three fills, and a fill has been
  in `ir.Backend` since v0.1. Every backend — SVG, PDF, canvas, gg, window,
  GPU — draws this without knowing it exists.
- **The spec round-trips it.** `coord.TypeOblique` joins the built-in `Type`
  constants (and is therefore refused by `Register` afterwards, as the other
  three are), `coord.Desc` gains `Depth` and `DepthAngle`, and `geom.Desc`
  gains `Extrude bool`. Both are additive fields on structs that already carry
  per-coord and per-geom options.
- **The description does not change.** [ADR 0024](0024-accessibility.md) says a
  chart reports what it plots, over what range, and how much of it there is.
  Depth is none of those, and a `Detail` that mentioned it would be describing
  the ink.
- **Responsiveness works because the depth is a theme length.** A chart that
  follows its surface by scaling its theme ([ADR 0025](0025-responsive-charts.md))
  scales its depth with everything else, rather than keeping a 14-pixel slab on
  a chart half the size.

## What this does not do

- **No third column, no z axis, no z ticks.** That is 0056, and the split is
  the point of this record.
- **No perspective.** See above; 0056 keeps the refusal.
- **No shadows, no ambient occlusion, no bevels.** Two shades and a silhouette
  are what an extrusion is; anything more is a renderer, and CONCEPT §5 says
  figure is not one.
- **No extruded line or area.** A ribbon behind a line is a surface with a
  depth of one row, and it belongs with the surfaces in 0056 rather than
  alongside the two marks that are already rectangles.
- **No per-mark depth.** The depth vector is the panel's. A mark that stood
  further back than its neighbour would be encoding something, and this record
  is about the case where nothing is encoded.

## Amendment: what building it sharpened

`coord.Oblique` and `geom.Extrude` are built as the record describes — a
Cartesian coord with a depth vector behind `coord.Extruder`, a bar or a rect
drawn as a front, a top and a side face, the IR untouched, `coord.TypeOblique`
refused by `Register`, and `Depth`, `DepthAngle` and `Extrude` round-tripping
through `spec`. Five things came out sharper than the record, and none of them
moves its central clause: no column sets the depth.

**The depth is a fraction of the panel, not a theme length.** The record says
responsiveness works "because the depth is a theme length", and wants the
depth given in pixels. But the depth belongs to the coord, and a coord does
not know what a theme is — `coord.Metrics` exists precisely so that one never
has to. The one length a coord does know is the rectangle it is framed in, so
`coord.Depth` is a fraction of the panel's shorter side, 0.04 by default and
at most a quarter. That scales with the chart for the reason the record
wanted, without a theme crossing into `coord`.

**"Back to front" means along the depth vector, and ascending.** Every mark
shares one plane, so nothing is behind anything in depth; what overlaps is the
faces a mark turns towards its neighbour, and the neighbour lying the way the
depth vector points is the one that covers them. So a layer paints its marks in
ascending order of the projection of each mark's middle onto the depth vector,
ties broken by the row — with the default vector, left to right and bottom to
top, which is what puts a stacked segment's lid under the segment above it.
The order is per mark, so an extruded layer gives up batching by colour: three
fills per mark rather than one per colour, which for the few dozen bars a chart
of volume ever has costs nothing.

**A row is reported once per face, and its front last.** The record says
`Frame.Marks` still reports one position per row. It cannot: the hit index
finds an area's row by looking for a reported position *inside that area*
([ADR 0015](0015-hit-testing.md)), so a pointer on a top or a side face found
no row at all. An extruded mark therefore reports the middle of each face it
shows and then its front, and the front comes last because `interact.Index.Locate`
answers with a row's last position — so a highlight still points at the value,
where the flat bar would have been. The record anticipated the consequence it
named: `Live.Select` now gathers a layer's rows as a set, in order of first
appearance.

**An angle is reduced to (−π, π], and a zero in a `Desc` is the default.** A
`Desc` naming the type and nothing else has to draw what `coord.Oblique` draws,
so zero depth and zero angle are the defaults, and a depth vector pointing
straight to the right is written as a full turn. Reading that full turn back
exposed the one real bug the round trip found: `sin 2π` is a rounding error
rather than zero, and a vertical part of 10⁻¹⁶ gave every mark a top face of no
height. `DepthAngle` now reduces its argument, so a full turn is the angle zero
exactly.

**The spec tests now render the coord.** The helper every `spec` round-trip
test compares drawings with built its chart without the coord, so a sunburst
and an icicle — and an oblique chart and a flat one — drew the same calls and
compared equal whether or not the coord survived. It passes the coord now,
and every existing round trip still holds.

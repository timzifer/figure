# 0063 — A scene is painted over too, and the seam is `three`'s own

**Status:** Accepted · **Date:** 2026-09-11 · **Implementation:** `three.Overlay`, `three.OverlayFrame`, `three.OverlayView`, `three.Projection`, `three.Highlight`

## Context

[ADR 0046](0046-overlay-layer.md) gave a flat chart a stage after everything
else: a crosshair, a tooltip, a brush rectangle, a ring round the mark another
chart is pointing at. [ADR 0062](0062-a-scene-and-its-views.md) gave a scene
several views and closed by saying that highlighting the row a pointer landed on
in every one of them is the host's, exactly as [ADR 0045](0045-linked-views.md)
decided for two charts.

That is still the right division, and it turns out to be unbuildable as it
stands.

A host that wants to ring a row has to draw the ring somewhere. On a flat chart
`Live.Overlay` is where. On a scene there is nowhere: `three` has no such seam,
so the host's only remaining options are to draw in its own toolkit — which
diverges from figure's ink, is unavailable to a static export, and has to be
re-implemented per toolkit — or to add the ring as a layer, which puts it in the
depth order and lets the surface the ring exists to point at occlude it.

So the absent piece is not the selection. It is the paint.

**Reuse `render.Overlay`.** One interface, one `DrawOverlay` a host writes once
and installs on either. It is tempting, and `render.OverlayPanel` already
tolerates a nil field, so "what does not apply is nil" has precedent in exactly
this pair of packages.

It fails on the payload. A panel carries two scales and a `coord.Coord`; a view
carries three scales, a `Camera` and a projection, and there is nowhere in
`OverlayPanel` to put either of the last two without putting 3D into `render` —
which is the thing [ADR 0056](0056-three-dimensional-charts.md) spends a version
refusing. `OverlayPanel.Coords()` would have to answer with a Cartesian coord
over the cell, which is a lie the built-in `figure.Crosshair` would then call.
And the shared interface would let a caller install that crosshair on a scene at
all: it would compile, draw two rules through a device point, and name no value
on any axis — the same failure already recorded for `Hit.X` and `Hit.Y`, where
filling them in would name a value nothing was drawn at.

## Decision

**A `three.Plot` carries an `Overlay`, and it is this package's own type.** The
method and its shape are `render`'s — `DrawOverlay(b ir.Backend, f
OverlayFrame)` — spelled again here rather than shared, which is the trade
`drawStrip` already makes: the two are the same idea and not the same drawing.

```go
sel := &three.Highlight{Data: []three.Point3{{X: 2.5, Y: -3, Z: 11.4}}, View: -1}

live, _ := three.New(three.Columns(2)).Scene(sc).
	Add(front, plan, side, quarter).
	Overlay(sel).
	Live(target)
```

### A view is told where to put a value, not how to read one back

`OverlayView` carries the view's index, its label, its cell, the cube's own
rectangle inside that cell, the camera, the scene's three scales, and a
`Projection`. `OverlayView.At(x, y, z)` is what an overlay calls: a position in
the **data**, and the pixel the scene actually drew it at, in this view.

There is no inverse and there will not be one. That is the bargain 0056 struck
and 0062 restated — a device point in a turned cube does not resolve to three
values — so an overlay that wants to mark a *row* asks `interact.Index.Locate`
where it landed, and one that wants to mark a *value* projects it here.

`Projection` is a value rather than a function field or an interface, and the
reason is the allocation gate rather than taste: a closure is an allocation per
view per frame and boxing one is another, and this package's whole discipline is
that a frame allocates nothing that grows with what it draws. For the same
reason the view list lives on the pooled `Sink` beside the cube's label boxes,
and `Highlight` keeps the path it drew.

### Drawn last, over every view, clipped by nothing, announced to nobody

It is 0046's rule, unchanged, with one addition of this package's own: **an
overlay is not in the depth order**. Its positions come from a pointer or a
selection rather than from the scene, so they have no depth to be sorted by —
and a mark inside the order would be occluded by the very surface it exists to
point at.

It is drawn after the `Observer` is told the frame ended, so nothing it paints
is attributed to whichever layer happened to be painted last. A ring a pointer
can hit is a ring that flickers.

### A turn with an overlay repaints the canvas

[ADR 0057](0057-orbiting-a-chart.md)'s amendment damages only the cells whose
cameras moved, which is what makes a four-view figure affordable to drag. An
overlay draws where it likes — a ring on a cell's edge, a leader line in the
margin between two of them — so no list of cells describes what it damaged, and
`turnedRects` returns nil when one is installed.

This is deliberately not a dirty flag. An overlay is a pointer the caller
mutates and the library cannot observe a mutation, so a flag would be state that
nothing can verify — which is the argument `Live.Draw` already makes against a
"a drag is in flight" flag, in those words. The camera moved or it did not, and
this package knows which; the overlay is installed or it is not, and it knows
that too.

The consequence is worth stating as advice rather than hiding: install the
feedback a gesture needs for as long as the gesture lasts. One kept installed
through an orbit costs a full repaint on every frame of the drag. A figure with
no overlay pays exactly what it paid before, and there is a test that the frame
is identical call for call.

### What ships, and what does not

`three.Highlight` and `three.Overlays`. `Highlight.Data` rings a value in
**every** view by default rather than on request, for 0062's reason: a figure
with four cameras is one chart looked at four ways, so a value marked in one of
them is marked in all four.

**No `three.Crosshair`.** Two rules through a projected point name no value on
any axis. What a reader actually wants there is a dropline — the segments from a
point down to the floor and across to the back walls — and that is a *layer*,
not an overlay: it is in data space and must be occluded by the surface in front
of it. [ADR 0058](0058-what-3d-is-for.md) already ships it with the 3D scatter.

**No `three.Brush`.** A rectangle dragged over a projected scene selects a cone
through the data, and this library cannot invert one — the same bargain that
leaves `Hit.X` and `Hit.Y` at zero. Drawing feedback for a selection nothing can
resolve is worse than drawing none.

## Consequences

- **There is still no selection inside figure.** This record adds the paint and
  not the state. Which rows are marked remains the host's, which is 0045's whole
  decision and 0062's closing line.
- **A `Live` inherits its plot's overlay and does not write back**, because
  `Plot.Live` copies the plot. That gives the precedence rule by construction
  rather than by a resolution step — the root package needs `figure.Live` to
  hold one separately and resolve, and here the copy is the resolution.
- **Nothing about this appears in the JSON spec.** A document carrying an
  overlay would be a document about a pointer. The dialect has no vocabulary for
  one and gains none, which is 0046's consequence and 0062's.
- **`Highlight` holds the path it drew**, so one belongs to one figure. That is
  the cost of not allocating per frame, and it is stated on the type.

## Revisit if

A tooltip is wanted, which is the overlay a projected scene needs most — a hit
reports a row and nothing else, so the value under the pointer has nowhere else
to appear. It is `figure.Tooltip`'s measurement and edge-flipping over this
frame, and it is a straightforward addition to this seam rather than a change to
it.

The other one is `interact` growing a query that works over a scene's index, at
which point `Brush` is four lines and ships.

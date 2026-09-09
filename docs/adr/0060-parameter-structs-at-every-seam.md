# 0060 — Every seam an outsider implements takes a struct, and the count of optional interfaces stops growing

**Status:** Accepted · **Date:** 2026-09-09 · **Implementation:** this commit

## Context

[ADR 0056](0056-three-dimensional-charts.md) found three signatures that a
third dimension breaks, and [ADR 0059](0059-renaming-and-restarting-the-version.md)
made breaking them free. `Geom.Train`, `Observer.Panel` and `Coord.Frame` now
take `geom.Training`, `render.PanelInfo` and `coord.Framing`.

Fixing three seams named by one feature is not the same as fixing the shape.
The rule that produced them — *an interface a third party implements never
gains a method* ([CONCEPT §15](../../CONCEPT.md#15-versioning--stability)) — is
still right, and it is still true that under it a method with positional
parameters can never gain one either. Every such method in this library is a
future break or a future optional interface, and the library is currently in
the one window where the difference is a compiler pass rather than a migration.

So the question this record answers is not "what does 3D break" but **"where
else is the same mistake, and what does the repository already look like where
it has been made"**.

The answer is legible without guessing, because a parallel path leaves a trace.
Three of them were already in the tree:

- **`render.LayerAxes` exists because `Observer.Layer(i, label)` could not gain
  a parameter.** Its own doc comment says so. It announces the two scales a
  layer is drawn against, immediately before the call that announces the layer,
  and an observer that does not implement it silently reads a secondary axis
  through the wrong scale.
- **`coord.Opposite` exists because `Coord.Furniture(dst, area, m, xTicks,
  yTicks)` could not gain two more tick families.** It is two methods,
  `FurnitureY2` and `FurnitureX2`, which place the same furniture as
  `Furniture` against the opposite edges. A third family — a depth axis, a
  second radial one — would have been a third method.
- **`render.EndData` exists because `Observer` could not gain a third method.**
  It closes the data pass. Without it, everything drawn after the last layer is
  attributed to that layer, which is [ADR 0046](0046-overlay-layer.md)'s
  recorded bug: a legend swatch indexed as a mark of whichever series happened
  to be painted last.

Five optional interfaces stand beside `Observer` — those three plus
`LegendEntry`, `ColorbarEntry` and `SizeKeyEntry` — around an interface with
two methods. That ratio is the diagnosis.

Beside those, an audit of every exported interface implemented outside this
module found the same shape in four more places, and one different problem.

**The same shape.** `ir.Target.Open(widthPx, heightPx, dpr)` and its mirror
`ir.Resizer.Resize` describe a surface with three positional values, when a
surface also has a colour space, a background and an opacity it might one day
have to state. `mathtext.Typesetter.Typeset(src, font, measurer)` has no room
for a writing direction. `geom.Rows.Marks(at, rows)` is two parallel slices,
and a mark has more to say about itself than a position and a row.
`scale.Scale.Ticks(want int)` cannot be told the width the axis has, the
rotation its labels may take, or the locale their format follows.

**The different problem.** `ir.Marker` is an `iota` enum, and the growth rule
lets it gain members at the end. A backend switches on it. The outlines were
defined once — correctly, so that a diamond is the same diamond in PDF as in
SVG — in `internal/markers`, which a backend outside this repository cannot
import, and which `backend/gg` therefore *hand-copied* with a comment saying
so. The consequence is that adding a marker shape is "additive" by the letter
of the rule and silently degrades every third-party backend to a circle, with
no error and nothing to notice.

## Decision

**A seam implemented outside this module takes one growable struct, never a
list of positional values; and geometry a backend must reproduce is exported,
never internal.**

Concretely, in this commit:

| Seam | before | after |
|---|---|---|
| `render.Observer` | `Layer(i int, label string)` | `Layer(l render.LayerInfo)` |
| `render.Observer` | — | gains `End()` |
| `render.LayerAxes` | `LayerAxes(x, y scale.Scale)` | **deleted**; the scales are `LayerInfo.X` and `.Y` |
| `render.EndData` | `EndData()` | **deleted**; it is `Observer.End` |
| `render.LegendEntry` | `LegendEntry(layer, label, area, hidden)` | `LegendEntry(e render.LegendInfo)` |
| `render.ColorbarEntry` | `ColorbarEntry(cs, class, lo, hi, area)` | `ColorbarEntry(e render.ColorbarInfo)` |
| `render.SizeKeyEntry` | `SizeKeyEntry(value, label, area)` | `SizeKeyEntry(e render.SizeKeyInfo)` |
| `coord.Coord` | `Furniture(dst, area, m, xTicks, yTicks)` | `Furniture(dst, req coord.FurnitureRequest)` |
| `coord.Opposite` | `FurnitureY2(…)` and `FurnitureX2(…)` | `FurnitureOpposite(dst, req coord.FurnitureRequest)` |
| `ir.Target` | `Open(widthPx, heightPx int, dpr float64)` | `Open(s ir.Surface)` |
| `ir.Resizer` | `Resize(widthPx, heightPx int, dpr float64)` | `Resize(s ir.Surface)` |
| `mathtext.Typesetter` | `Typeset(src, font, m)` | `Typeset(req mathtext.Request)` |
| `geom.Rows` | `Marks(at []ir.Point, rows []int)` | `Marks(m geom.MarkRows)` |
| `scale.Scale` | `Ticks(want int)` | `Ticks(req scale.TickRequest)` |
| `internal/markers.Path` | internal | `ir.MarkerPath`, exported |

### Which optional interfaces survive, and why

Deleting `LayerAxes` and `EndData` while keeping `LegendEntry`,
`ColorbarEntry` and `SizeKeyEntry` is not inconsistency. The test is **whether
an implementation that omits it is narrower or wrong**:

- An observer that does not close its layers is **wrong**. It indexes furniture
  as data, and it has no way to know it is doing so. That is a method on
  `Observer`, implemented empty by anyone with nothing to close.
- An observer that read a layer's scales through a separate call was **wrong
  whenever it did not implement the call** — a secondary axis reported through
  the panel's own scale names a value from the other axis. That is a field.
- An observer that does not care where the legend was drawn is **narrower**. A
  hit-testing index wants it; a debug tracer does not. That stays optional.

The same test explains why `coord.Opposite` remains an interface rather than
becoming fields the primary `Furniture` reads: a polar coord has no far side to
put a second axis on, and a [Smith](0033-smith-charts.md) chart draws both its
axes inside the disc. Silently ignoring `X2Ticks` would turn "this coord
declines" into "this coord forgot". What changes is that declining is one
answer rather than two methods, and that a *third* family of axes is a field.

### What the structs deliberately do not carry

None of them gains a Z. `FurnitureRequest` has `XTicks` and `YTicks`;
`LayerInfo` has `X` and `Y`; `TickRequest` has `Want` and nothing else. This
record is about the shape of the seams, not about filling them — 0056's third
dimension arrives with the package that needs it, and the point of doing this
now is that it will then be a field rather than a release.

### `ir.Marker` and the exported outline

`ir.MarkerPath(p *ir.Path, m Marker, size float32)` appends the outline of any
marker and appends a circle for one it does not know. The three built-in
emitters call it, `backend/gg`'s hand-maintained copy is deleted, and a backend
written outside this repository has the same access to the geometry that the
ones inside it do.

This is the one item here that is not a breaking change, and it is the one with
the worst failure mode: the alternative was a library that adds a marker shape
in a minor release and quietly draws it wrong everywhere it does not control.
`Marker`'s doc comment now says that the set grows and that `MarkerPath` is how
a backend stays correct across that growth.

### The rule this leaves behind

CONCEPT §15's growth rule is unchanged and is now enforceable rather than
merely stated:

> An interface a third party implements never gains a method, **and a method on
> one never takes more than one parameter beyond its output destination** — so
> that what it is told can grow without the interface changing. A capability
> that an implementation may legitimately lack is an optional interface beside
> it; a fact that every correct implementation needs is a field or a method.

An optional interface added from here is evidence that either a capability is
genuinely absent from some implementations, or that this rule was broken
earlier and is being paid for. The second reason is no longer available.

## What this does not do

- **It does not touch `ir.Backend`.** A drawing call is a sink for ink, its
  parameters are what is being drawn rather than a description that grows, and
  optional interfaces beside it (`Resizer`, `Semantics`, `Partial`) are the
  design rather than a symptom — [ADR 0002](0002-ir-and-backend.md) argued
  that and it still holds.
- **It does not touch the tuple-returning coord calls.** `Coord.Extent`,
  `Point` and `Invert` return fixed pairs and quadruples that cannot grow to a
  third dimension. That is not an oversight: 0056 decided that `three`
  projects *above* this seam and calls the 2D coord, so the pairs are the
  contract rather than a limit. If that decision is ever reversed, it is
  reversed here first.
- **It does not settle `data.Source`.** Its three column-kind methods have the
  same growth problem in a different shape — a fourth kind is a fourth optional
  interface *and* a type assertion at every use — and its own doc comment
  already plans for that. Whether exact integers, booleans and durations belong
  in the interface is a question about the data model rather than about
  parameter shape, and it gets its own record.
- **It does not free the `v0.x` series for other breaks.** 0059's rule holds:
  a change not written down here or in 0056 needs a record before it needs
  code.

## Revisit if

An optional interface is proposed beside an interface this record touched. The
first question is whether the capability is really absent from some
implementations, or whether a field would have done — and if it is the second,
the seam is wrong and this record is where it gets fixed rather than papered
over.

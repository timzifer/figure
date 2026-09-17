# 0069 — A filled mark is told apart by a hatch, drawn rather than declared

**Status:** Accepted · **Date:** 2026-09-17

## Context

[ADR 0024](0024-accessibility.md) quotes CONCEPT §14's promise of *"redundant
encoding (patterns/dashes)"* and ships the dashes. `theme.Redundant(true)`
fills in `SeriesDashes` and `SeriesMarkers`, and a layer that named neither
`geom.Dash` nor `geom.Shape` takes the entry at its own index.

A dash needs a stroke. A marker needs a point. **A bar has neither.**

So the marks that fail hardest in greyscale — a stacked bar, a pie, a stacked
area, a treemap, a set of boxplots — are exactly the marks the one option
figure offers for this problem did nothing for. Turning `Redundant` on for a
stacked bar chart changed nothing at all, and a reader who could not separate
the first two palette entries was left with three identical grey blocks and a
legend they could not use.

The patterns half of the promise is the missing piece. The design questions are
what a pattern *is* in a model whose backend interface is frozen, and what
stops it from being a fourth thing every backend has to learn.

## Decision

**A hatch is a fixed enumeration in the IR, resolved by a theme ladder, and
lowered into ordinary drawing calls before any backend sees it.**

### The vocabulary is an enumeration, like a marker

`ir.Hatch` is a `uint8` with fourteen values in four families: lines
(diagonal, back-diagonal, cross, horizontal, vertical, grid), dots (square and
staggered), wavering lines (zigzag, wave) and tilings (brick, triangles,
scales). `ir.Hatching` pairs one with its ink and its period.

An enumeration rather than a parameter struct because a redundant encoding
needs a *ladder* — a fixed sequence a layer index walks — and a ladder of
arbitrary angles is a ladder nobody can name a rung of. It is `ir.Marker`
again, including the rule that makes `ir.Marker` safe: the set grows at the
end, and a value this release does not know appends no geometry and leaves the
mark with its plain fill.

### Two readings, not one

`Hatch` is nominal and `Hatching.Spacing` is ordinal. Changing the pattern says
*different*; halving the period doubles the ink and says *more*. They are
separate because the question they answer is separate, and because a redundant
encoding must not invent an order the data does not have: `Redundant` installs
`theme.DefaultSeriesHatches`, where every rung is a different shape and no rung
is heavier than another. `theme.DensitySeriesHatches` is the other ladder, for
a stack that really is an order — severities, age bands, size classes — and it
is installed by `theme.Hatches` rather than by default, because guessing wrong
here means a chart that claims a magnitude nobody measured.

How coarse every pattern on a chart is belongs to the theme rather than to a
series, so it is `theme.HatchSize` and not a third thing on the ladder: a poster
read across a room wants a coarser hatch than a figure in a paper, and both want
the same ladder.

### It is drawn, not declared

`ir.Backend` does not change. `ir.Fill` does not change. A hatch never reaches
a backend at all: `ir.FillHatched` fills the path, then pushes the path as a
clip and strokes the lines `ir.HatchPath` generates inside it.

Three reasons, in order of weight:

- **The interface is frozen** ([ADR 0002](0002-intermediate-representation.md),
  restated in `ir/backend.go`). A `Fill.Hatch` field would be ignored in
  silence by every backend written before it — including the ones implemented
  outside this module, which is the case the freeze exists for. A mark that
  quietly loses its pattern on one backend is worse than one that never had it,
  because the chart still looks finished.
- **The GPU tier would flatten it.** `gogpu/gg` has everything a native pattern
  needs — `SetFillBrush`, `CustomBrush`, even a ready-made `Stripes` — but its
  SDF accelerator collapses any non-solid brush to `ColorAt(0, 0)`, a single
  colour. A brush-based hatch would therefore vanish the moment
  `backend/gg/gpu` was imported, which is a difference between two builds of
  the same program.
- **Four native implementations would drift.** An SVG `<pattern>`, a PDF
  tiling pattern, a canvas `createPattern` and a gg brush are four different
  rasterisations of one idea. Lowering once in `ir` means every backend draws
  the same geometry, antialiases it the way it antialiases everything else, and
  prints it as real lines in SVG and PDF.

The cost is IR volume: a hatch is one clip and one multi-segment stroke per
batch. It is paid only by a chart that asked for one, the lines are cut to the
box rather than left long, and a pattern too dense to read draws nothing at all
rather than thousands of strokes.

### A hatch is not a mark

`interact.probe` indexes every `StrokePath` subpath as a hit target. Hatch
lines would put dozens of phantom targets inside every bar, and pointing at a
bar would report whichever line was nearest.

`ir.Decoration` is the fix, and it is the shape every other extension to the
backend contract has: an optional interface beside `Partial`, `Resizer` and
`Semantics`, asked for with a type assertion and done without when the answer
is no. `FillHatched` brackets its pass in it; `probe` implements it and stops
indexing; a backend that draws pixels never hears of it.

### The finishes that are not patterns

Three more things a filled mark can wear, none of which needs a backend to
learn anything:

- `geom.Corner` rounds rectangular marks, through `ir.Path.RoundRect`. Whether
  a mark *has* corners is asked of the shape rather than of the coordinate
  system — a rectangle under `coord.Donut` is an annular sector whose corners
  are arcs — so the test is `Path.AsRect` and it stays right for a coord this
  package has never met.
- `geom.Inset` draws an inner border: the mark's own path stroked at twice the
  border width and clipped to itself, which is an inset outline exactly and
  needs no path offsetting. It is what divides marks that touch without
  thickening the silhouette of the layer.
- `geom.Gradient` finally gives a geom the linear gradient `ir.Fill` has
  carried since v0.1 and every backend has implemented since, whose only
  caller until now was the colourbar.

## Consequences

- **`theme.Redundant` is three ladders.** The first rung of the hatch ladder is
  `HatchNone`, so a single-layer chart is unchanged and the difference appears
  from the second layer on — the rule the dash ladder already followed. Every
  existing golden file is byte-identical: hatching is off unless a theme
  installs a ladder or a layer names a pattern.
- **`geom.Desc` gained `Hatch` and `HatchSet`**, the pair `Dash`/`DashSet` and
  `Marker`/`MarkerSet` already have, for the reason ADR 0024's consequences
  give: `HatchNone` is both the zero value and a choice — *stay plain whatever
  the theme says* — and a round trip through the JSON spec must not turn the
  first into the second.
- **`ir.Recorder` records the decoration bracket.** A faceted chart records its
  panels in parallel and replays them into whatever the caller handed it, probe
  included; a bracket that did not survive the round trip would mean
  hit-testing worked for a serial render and not for a parallel one. The
  typesetting wrapper in `render` forwards it too, because an embedded
  interface promotes only its own methods.
- **An annotation is never hatched by a theme.** `geom.HBand`, `VBand` and
  `Region` take the theme's annotation colour rather than a palette entry and
  walk no ladder, so they hatch only when asked. A hatched band saying "out of
  spec" or "projection" is a statement the caller makes; it would be a strange
  default for the one layer on a chart that is deliberately quiet.
- **A layer painted per row from a discrete colour scale wears one hatch, not
  one per category.** Those marks are batched by colour and the pattern is laid
  over the batch, so the legend claims the layer's pattern for every row rather
  than promising a distinction the marks do not make.
- **What is not here:** a hatch *scale*. There is no `geom.HatchBy(col)`, no
  `scale.Hatches`, no hatch guide. A pattern channel driven by data would be a
  fourth guide kind and a fifth scale interface, and the case for it is not the
  case this record makes — which is about a chart surviving a photocopier, not
  about a fifth variable. `geom.Hatch` on a second layer covers the one use
  that keeps coming up, which is marking part of a series as modelled.
- **What is also not here:** drop shadows. There is no blur in the IR, and a
  faked offset copy costs a second fill per mark and looks cheap in vector
  output.

## Revisit if

a backend turns up where a native tiling pattern is dramatically cheaper than
the lowered geometry — a very large dashboard redrawn every frame, where the
stroke count per frame is the bottleneck. The lowering lives behind one
function, so that backend could be given a fast path without any geom or theme
knowing; what would have to be settled first is how the two rasterisations are
kept looking the same, which is the question this record declined to answer
four times over.

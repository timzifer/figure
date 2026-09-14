# 0065 — A horizon chart folds its own axis, and the colourbar is the ladder it gives up

**Status:** Proposed · **Date:** 2026-09-11 · **Implemented:** —

## Context

A **horizon chart** cuts a series' value range into bands of equal height,
draws every band at the panel's full height, and tells the bands apart by
colour rather than by position: the higher the band, the deeper the colour,
with negative values mirrored back above the baseline and coloured from the
other arm of the ramp. A chart one quarter as tall keeps the resolution a
chart four times as tall would have, because the vertical space is spent four
times over.

It is the one form in this catalogue that exists to answer a question about
*how many series*, rather than about how many rows or what shape they are.
Heer, Kong and Agrawala measured it in 2009: below about forty pixels of chart
height, a horizon chart is read faster and more accurately than the filled line
chart of the same series, and the crossover is exactly where a wall of series
puts a reader.

### Why this library in particular

figure's answers to scale are all answers to *row count*. Decimation reduces
marks in device space ([ADR 0011](0011-decimation.md)), the density raster
paints a million points as pixels, and a hexbin bins them. None of them helps
with forty sensors at a thousand samples each, which is not a big series but a
lot of them, and the only answer in the tree today is faceting — which divides
the space instead of reusing it.

The pieces a horizon chart wants are already here and were built for
neighbouring reasons. `Plot.Track` is a band at a panel's edge on a shared
axis, and [ADR 0031](0031-tracks.md) names the "sparkline gutter" as one of
the things it is for. `scale.Quantize` and `scale.Threshold` cut a ramp into
classes and report their boundaries ([ADR 0042](0042-colour-transforms-and-classes.md)).
`geom.Area` fills between a series and a baseline and already draws a band
between two series given `Y2`. The examples are called `machine`, `status`,
`stream` and `dashboard`.

Outside Go the form is ordinary and inside Go it does not exist: D3 has a
plugin, Vega-Lite draws it as a layered-area recipe, R has
`latticeExtra::horizonplot`, Python has gists.

## Decision

**`geom.Horizon` is a mark, and the fold runs in `Train`.**

```go
p.Y(scale.Linear())
p.Add(geom.Horizon(src, geom.X("t"), geom.Y("kw"), geom.Bands(3)))
```

`geom.Bands(k)` cuts the trained domain into k bands; `geom.BandHeight(h)`
gives the band in the data's own units and wins where both are set, because a
band that means 50 kW is the spelling a plant engineer has and a band that
means "a third of whatever the maximum turned out to be" is the spelling that
changes when a new day's data arrives. The fold's origin is `geom.Baseline`,
which exists.

### Why it is a mark and not a position adjustment

[ADR 0019](0019-position-adjustments.md) derives stacking in `Train` and might
look like the same machinery: both take a value and put it somewhere other than
where the scale says. They are not the same, and the difference is load-bearing
in two places.

A position adjustment **moves** a mark. The fold **cuts one row's value into k
pieces and clips each one**, so one row becomes up to k drawn spans and the
row-to-mark correspondence every adjustment preserves is gone. And a stack's
invariant is that its slots do not overlap, where every band of a horizon chart
occupies the same rectangle as every other. An adjustment that broke both would
not be an adjustment.

### Why the fold runs in `Train`

[ADR 0028](0028-distribution-stats.md)'s rule decides it: a stat runs in
`Train` when its output *is* what the axis describes. The fold does not merely
describe the Y axis — it replaces it. After folding, the Y domain is `[0, h]`
and nothing else is plotted against it.

It also settles the composition with decimation for free, and in the right
direction. Decimation runs in `Build`, in device space
([ADR 0011](0011-decimation.md)), so the order is fold-then-decimate. The
reverse would be a bug with no symptom: LTTB dropping the sample that decided
which band a run belongs to changes the *colour* of a stretch of chart, not
just its outline.

### The axis question, which is the reason this is a record

A horizon chart gives up its Y axis, and this library does not let a mark make
that up for itself. `render` walks two tick lists and labels nothing a scale
did not write — the constraint [ADR 0033](0033-smith-charts.md) recorded and
[ADR 0051](0051-barycentric-coord.md) deliberately did not reopen.

The folded ladder is honest, so nothing has to be reopened. **The Y scale after
the fold describes one band, and the ticks it writes are correct for every band
on screen** — a mark at two thirds of the panel's height really is two thirds of
a band above that band's floor, whichever band it is. What the reader is missing
is not the ladder but *which* band, and that is a colour.

So **the layer's guide is a classed colourbar, and its breaks are the fold's own
boundaries.** `scale.Quantize` and `scale.Threshold` already produce a
`ClassedColorScale`, `ColorGuide` already keys on `Breaks()`, `render` already
draws it and [ADR 0048](0048-clickable-colourbar-and-size-key.md) already makes
it answer a pointer with a quantity rather than with a series. A horizon chart
needs **no new furniture, no new guide kind and no change to `render`**: the
band boundaries are printed down the side of the key, in the data's units, which
is the reading the axis gave up.

The colour column follows `geom.Hexbin`'s precedent exactly: a `ColorBy` scale
may be given and the column named in it is not read, because the quantity being
coloured is the layer's own band index.

### One layer draws one series

`geom.GroupBy` on a horizon layer is an error rather than a silent overlap. The
whole premise of the form is that a series gets a strip of its own; N series in
one panel would be N chart-height bands painted over each other, which is not a
degraded horizon chart but a solid rectangle.

A wall of them is therefore what the library already builds walls with: a
`facet` over the sensor column, or a `Plot.Track` per series with `TrackSize`
naming the strip height. Neither needs anything.

## Consequences

| | |
|---|---|
| `geom` | one mark, two options (`Bands`, `BandHeight`), one error for `GroupBy`; `Baseline` and `ColorBy` reused |
| `scale` | unchanged — `Quantize` cuts the ramp and reports the breaks |
| `render`, `layout`, `ir`, `coord` | unchanged; the guide is a classed colourbar and there is already one |
| `stat` | the fold itself: numbers in, numbers out, with an `Append` form |
| `spec` | a `"horizon"` mark with a band count or height |
| `a11y` | a cell reads as a value, not as a band index — the fold is a way of drawing, not a way of measuring |
| Charts unlocked | horizon chart, and a panel of forty series that is legible at a glance |

## Not in scope

- **A horizon chart of a stacked series.** Stacking is a statement about parts
  of a whole and the fold destroys the position that says so.
- **Choosing the band count from the data.** A band height that moves with the
  maximum makes two charts of the same quantity incomparable, which is the one
  thing this form is for. The default is three bands over the trained domain
  and the doc comment says to pin it.
- **A second ramp per layer.** Positive and negative come from the two arms of
  one diverging ramp, which is what [ADR 0042](0042-colour-transforms-and-classes.md)
  already builds.
- **Interaction beyond what exists.** A hover reports the row's own value
  through the unfolded scale; that is `interact`'s ordinary path and needs
  nothing.

## Revisit if

- A second mark wants to fold an axis. One is a mark; two would be an argument
  for the fold being a property of the scale, and that is a much larger change
  than this one — it would put a scale on screen that no tick describes.
- Tracks turn out to be the normal spelling and the per-track ceremony is
  heavy. A `Plot.Horizons(src, groupCol)` that lays out one track per group is
  sugar over this and needs no new machinery, but it should not be invented
  before anyone has written the long form twice.

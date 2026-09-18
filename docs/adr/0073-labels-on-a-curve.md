# 0073 — A label on a curve is placed by the curve, gapped out of it, and seated at the table ADR 0040 set

**Status:** Accepted · **Date:** 2026-09-18 · **Implemented:** 2026-09-18

## Context

Three places in this repository defer the same feature to a record that did not
exist, and they are the three that need it.

[`geom.Contour`](../../geom/contour.go) says so in its own doc comment:

> Filled bands between levels, and labels along a line. Both are worth having
> and neither is this: a band is a polygon where this is a path, and a label on
> a curve is a placement problem with its own record.

[ADR 0050](0050-locus-annotations.md) shipped `geom.Locus` and said the same
about "3 dB" written along an M contour, and [AGENTS.md](../../AGENTS.md)
generalises both:

> What genuinely still needs the wider seam is a *labelled* family with no tick
> behind it — a projection's graticule, a ternary's third ladder — and that is
> one decision for all of them.

A contour chart without labels is the one chart in the catalogue that cannot be
read without a second guide. A heatmap says what every cell holds; a contour
says only *where* a crossing is, and which crossing it is comes from a
colourbar, from a legend, or from the reader counting rings inwards. That is the
whole gap: the number is in the picture and it is not written on it.

## Decision

**A label on a curve is placed from the curve's device-space geometry, the curve
is gapped to make room for it, and the label asks
[ADR 0040](0040-label-collision-avoidance.md)'s panel placer whether it may have
the space — without being allowed to move.** `geom.LabelLevels(true)` turns it
on for `Contour` and `Locus`, and `geom.LevelFormat` says how a level is
written.

Five claims carry it.

### 1. It is placed after the coord, and that is the second and last exception

Every geom computes a midpoint, a corner or a staircase step *before* the coord,
because those are statements about the data.
[ADR 0068](0068-gantt-charts.md) broke that once, for a dependency arrow's
elbow, on the grounds that there is nothing in data space between the finish of
one task and the start of another.

A curve label is the same kind of thing and the argument is sharper. The label
is rotated to the curve's **tangent**, and a tangent in data space is not the
tangent on screen: under `coord.Polar` a constant-radius arc is straight in the
scaled pair and bent on the panel, and under `coord.Smith` a straight sweep in
impedance is a circle. Choosing the angle before the coord would write the label
across its own line on exactly the two coords this feature exists for.

So the placement reads the device points the mark is about to stroke, and
nothing else. It never sees a datum, which is why it can be one function for two
marks that share no data model — a contour's runs come out of a traced lattice
and a locus's out of a formula.

### 2. Candidates come from the curve; the answer comes from ADR 0040's placer

The placer already arbitrates a panel's labels: earlier layers and earlier rows
win, a box that does not fit inside the panel is refused, and nothing converges
or reseeds. A curve label joins that table with one difference, and it is the
interesting one:

**`move` is false.** A text label that collides may be nudged to one of eight
neighbouring positions, because a label near its point still names its point. A
curve label may not: moved off its curve it names a level it is not on, which is
a chart that lies rather than a chart that is crowded. So the curve offers the
candidates and the placer only ever says yes or no.

The candidate list is bounded and its order is total:

- `curveLabelTries` positions spread evenly along the run, a constant for the
  reason `stat.LayeredSweeps` is one ([ADR 0072](0072-layered-graph-layout.md));
- each is scored by the **sag** of the curve away from the chord under the
  label, and one sagging more than `curveLabelSag` times the font height is not
  a candidate at all — that is the bend a straight run of text cannot sit on;
- the flattest wins, ties going to the one nearest the start of the run, which
  is the run's own order and therefore the table's;
- refused by the placer, the next-flattest is tried, and a run with no candidate
  left carries no label.

Same table, same panel size, same picture — on one goroutine or on eight.

### 3. The curve makes room: a gap, not a halo

The label needs the line out from under it. Two ways exist and only one of them
is in this IR.

A halo — the text drawn twice, once fat in the background colour — needs the
background colour, which a layer does not know: a panel may be transparent, and
a contour is drawn over a raster as often as not. A knockout needs a text
*background*, which `ir.TextRun` does not have and which would be the first
compositing decision in the IR.

A gap is geometry. The run is stroked up to where the label starts and resumed
where it ends, as two subpaths of the path it was already going to be. Every
backend already draws it, `ir` gains nothing, and the result is what a contour
map has looked like since before there were computers.

The gap's ends are interpolated along the device polyline. Under a curvilinear
coord, where `strokeRun` draws an arc between successive device points, that
puts a gap end a fraction of a pixel off the arc it cuts — the same order of
error as the polyline itself. Interpolating rather than snapping to the nearest
sample is for the coarse lattice, whose segments are long enough that snapping
would gap half the ring.

### 4. Upright, and one label per run

The run is turned to the chord between the gap's ends, and turned a further half
turn when that chord points leftwards, so the text reads left to right whichever
way the curve was traced. Nothing else about the anchor is configurable: the
label is centred in its own gap, because the gap was cut to fit it.

**One label per run.** A level is many runs and each of them is a separate
statement about where that value crosses, so each gets its own label; a run that
snakes across the whole panel gets one label and not one every few hundred
pixels. Repeating along a run is a spacing policy with a knob, and the knob would
have to be in device units — the first thing in `geom` whose value depended on
how large the chart is. That is not refused forever; it is not decided here.

### 5. The text is decoration, the geometry is data

0072's line, one dimension down. The levels a contour traces are settled in
`Train`, before anything has been measured; where the label sits is chosen in
`Build` from what the active backend says the string is wide. So two backends
may gap the same curve a few pixels apart, exactly as they place a tick label a
few pixels apart, and neither moves a line.

That also fixes what a level is written as. `geom.LevelFormat` takes a Go
function and does not serialise, which is [ADR 0041](0041-qq-plots.md)'s rule for
a quantile function and `scale.Format`'s for a tick; a document carries
`labelLevels` and reads back with the standard formatting.

## Consequences

| | |
|---|---|
| `ir`, `render`, `coord`, `scale`, `internal/layout` | unchanged — `render` already lends a placer to any layer that asks, and this is the first non-text layer to ask |
| `geom` | `LabelLevels`, `LevelFormat`, and one placement helper shared by `Contour` and `Locus` |
| `spec` | a `labelLevels` field on both marks |
| Charts unlocked | a contour map that reads without a colourbar, a Nichols chart with its M and N contours named, a Smith chart with its VSWR circles named |

**A labelled curve still reports no rows.** A contour's marks are its lines and a
locus announces nothing at all; a label is drawn where the mark chose and not at
a datum, so nothing is added to the hit index. `Hit` is unchanged.

**A contour label competes for the panel's label space.** A text layer with
`geom.AvoidOverlap(true)` in the same panel now competes with the contour's
labels, and the contour — drawn earlier — wins. That is 0040's rule working as
written rather than a new one, and it is why `AvoidsLabels` reports true for a
labelled contour without the caller opting in twice: a layer whose labels would
overlap each other has no use for a chart where they do.

**The allocation gate holds.** The gapped halves are built into two buffers the
layer keeps, beside the device-point scratch it already had, so a chart redrawn
every frame allocates neither.

## Not in scope

- **Text following the curve per glyph.** Shaping belongs to the backend and
  `ir.TextRun` is one string at one angle; per-glyph placement on a path is
  paragraph layout by another name, which `CONCEPT.md` §5 keeps out.
- **Repeated labels along one run.** Per claim 4.
- **Leader lines and callouts.** 0040 left them out and this does not bring them
  back.
- **Thinning which levels are labelled.** Every level a mark draws is a level it
  writes. Which levels a chart shows is `Levels` and `LevelCount`'s question,
  already answered.
- **A labelled grid family.** AGENTS.md's graticule and ternary ladder are
  *furniture* — drawn by `render` from a coord's `Furniture` — and this record
  labels a **mark**. It narrows that open question to furniture alone, the way
  0050 narrowed it to families with no tick, and it does not widen `render`'s one
  tick list per axis.
- **Filled bands between levels.** Still a polygon where this is a path.

## Revisit if

- a chart turns up whose runs are long enough that one label each is too few —
  then claim 4's spacing knob is due, in device units, with its own answer for
  what a resize does to it;
- the placer is asked for a candidate box that is not a rectangle. A rotated
  label's bounding box is the one `layout.labelBounds` already computes, and a
  label at 45° reserves noticeably more room than its ink needs. A tighter test
  is an oriented-box intersection in `internal/layout`, which is a change to the
  arbiter rather than to this record.

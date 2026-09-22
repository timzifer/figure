# 0083 — An axis break belongs to the scale, and it is marked or it is not drawn

**Status:** Accepted · **Date:** 2026-09-22 · **Implemented:** 2026-09-22

## Context

Two charts asked for the same thing from opposite ends.

The first is the chart every plotting library is eventually asked for: six
sites, one of them thirty times the others. On one continuous axis the outlier
is the whole picture and every other bar is a sliver along the bottom; a log
axis would fix the proportion and cost a reader who counts requests rather
than orders of magnitude. What is wanted is an axis that leaves the empty
stretch between 12 and 85 out, and **says so** — the `//` a hand-drawn axis
carries, or the torn `/\/\/\` of a printed one.

The second is a machine's state log. A shift timeline where "Inaktiv" is half
the day is a timeline of the machine standing still; what the reader wants is
the time it was doing something, with every idle stretch folded away. That is
not one interval chosen by eye, it is dozens read out of the same table the
chart draws, and they are a claim about which periods the reader does not want
to look at rather than about where the data runs out.

Nothing like either existed. `scale`, `coord`, `theme` and the dialect had no
concept of an interval left out of an axis, and three nearby names were already
taken for other things: `"breaks"` in the JSON dialect is a threshold colour
scale's class boundaries, `geom.Gap` is the missing-data policy, and
`ir.HatchZigzag` is a fill pattern.

## Decision

**An interval left out of an axis is a property of the scale; the coord cuts
the panel and marks the gap; and an axis that cannot be marked is not broken.**
Eight claims.

### 1. A break belongs to the scale, like a direction

`scale.Break(lo, hi)` on a linear scale and `scale.TimeBreak(from, to)` on a
time one. The reason is [ADR 0075](0075-an-axis-has-a-direction.md)'s: what a
break changes is `Map`, `Invert` and `Ticks`, and everything that reads a
position — every geom, `interact`'s inversion, a track sharing the panel's
scale ([ADR 0031](0031-tracks.md)), every panel of a facet — then agrees by
construction rather than by each learning the rule. A break is configuration,
so `Clone` keeps it, `Snapshot` shares it, and `Describe` writes it down.

The cuts are sorted and merged once, at construction, into a slice nothing
writes afterwards — `TickValues`' arrangement, and the reason a clone and a
snapshot can share it.

### 2. `Map` stays continuous, and a value inside a break lands in the gap

The kept pieces share the range less the gaps, in proportion to their length
in the domain, each through `place` with its own ends snapped — AGENTS.md's
two arithmetic rules applied per piece, with the piece offsets rounded
explicitly so no machine fuses them into a multiply-add. A value *inside* a
break is not refused: it maps linearly across the gap.

That is the choice that keeps everything else ordinary. `Map` is monotone and
continuous, so `Invert` is an exact inverse with no NaN for a pointer to fall
into; `Defined` is unchanged, so no geom learns about breaks; and a bar from 0
to 95 across a break from 10 to 88 is one bar, drawn with its middle missing,
because the panel's clip hides whatever falls into the gap. The alternative —
treating a value in the break as missing — would make a point there vanish
silently, and cut the tall bar that is the whole reason for the break.

### 3. The gap is a length on the page, and the renderer sets it

A scale does not know how wide a gap is, because it has no theme; neither does
a coord. `scale.Breaker.SetBreakGap(brk, fold)` is called once per render, on
the serial path and before anything is measured, from `theme.AxisBreakGap` and
`theme.AxisFoldGap` — which is how a size scale learns its range
([ADR 0027](0027-size-channel-and-the-guide-column.md)), for the same reason.
The resolution against the domain and the range is recomputed on the stack of
every call, so a pan, a zoom and a resize invalidate nothing, and a panel on
another goroutine never sees another panel's.

### 4. An unmarked break is a lie, so a coord that cannot mark one does not get one

A break says that equal distances along the axis stop meaning equal amounts.
Drawn without its mark, it is a chart whose ruler is wrong in a place the
reader is not told about. So a scale's cuts are **off until the renderer
switches them on**, and the renderer switches them on only under a coord that
implements `coord.Breakable` — today only `Cartesian`. Under polar, Smith,
ternary, a map or parallel coordinates the axis is drawn whole, silently, the
way `geom.Explode` under Cartesian draws what it drew
([ADR 0026](0026-breaking-a-mark-out.md)): an error would make an option's
validity depend on a coord chosen elsewhere.

A second axis is the same case. `drawOppositeAxis` places no clip and draws no
grid, so a break on `Y2` or `X2` would hide nothing and mark nothing; its
cuts are switched off too. And a break that would take more than half the axis
is not drawn either — a squeezed break is a picture of nothing.

A cut that reaches an end of the domain is the one case with no gap to mark: it
**trims** the axis, which then ends where the cut begins. That is what folding
the last idle hour of a shift has to mean, and it distorts no distance.

### 5. Ticks: one step for the length the axis still shows, walked per piece

A tick sequence is chosen for the *kept* length — `extendedWilkinson(0, kept)`
on a linear axis, `pick` over the kept duration on a time one — and walked
through each kept piece. A break from 10 to 90 over `[0, 100]` reads
0, 5, 10 | 90, 95, 100: one step, so the two sides read as one axis. Choosing a
sequence per piece was the other candidate and was rejected for two reasons: it
labels the two sides at different precisions, and it costs a tick search per
piece on an axis with hundreds of folds. No tick stands strictly inside a cut,
including one pinned with `TickValues`. The existing label collision pass thins
what remains.

### 6. A fold is a break of another kind

`scale.Fold(ivs...)` and `scale.TimeFold(spans...)` add cuts of kind *fold*: a
narrow gap (`theme.AxisFoldGap`, half a spacing unit) and a small slash on the
axis line at `theme.AxisFoldSize`, **never a zigzag** — a zigzag across the
panel per fold would be a hatch. Overlapping and touching cuts merge, and a
break that absorbs a fold stays a break, because the fold was the smaller
claim. When the folds do not fit in their share of the axis their gaps shrink
to nothing before anything is dropped: the axis is still folded and still
marked. Marks closer together than their own size are drawn once.

The lookup is a binary search over the cuts, with running sums of their widths
and counts computed at construction, so a row costs `O(log n)` in the number of
folds and allocates nothing — `BenchmarkFolded1k` against `BenchmarkFolded100k`
is the gate.

The folds are the caller's, and `figure.SpansWhere(src, from, to, col, value)`
reads them out of a table: every row whose `col` spells `value`, as a sorted,
merged list of `scale.TimeSpan`, with `SpansFunc` for any other test and
`IntervalsWhere`/`IntervalsFunc` for a numeric axis. It lives in the root
package because that is the one place holding both a table and a scale: `scale`
never reads a table and `data` never knows what an axis is.

### 7. The coord cuts, and render strokes over the data

The framed Cartesian coord keeps the two scales it was framed against and asks
them for their gaps in device space:

- **`Clip`** is one rectangle per pair of kept pieces, so whatever falls into a
  gap is not drawn. With no gaps it is the panel rectangle exactly as before —
  which is what keeps a raster backend treating it as a scissor.
- **`Furniture`** splits the axis lines and the grid lines at the gaps into
  subpaths, and fills a new `Furniture.Breaks` shape with the marks. With no
  gaps it is the unframed coord's furniture unchanged, two-point polylines and
  all, which is why no golden file moved.
- **render strokes `Breaks` after the data**, in the pass that already draws a
  polar coord's axes over it, with the axis stroke — because a bar crossing the
  break must not paint over the mark that says so. It is drawn whether or not
  the theme shows axis lines: an axis whose rule is off is still broken.

`theme.AxisBreakMark` chooses the mark: `BreakSlash`, two short parallel
strokes across the axis line at the edges of the gap, is the default;
`BreakZigzag` runs two parallel zigzags across the whole panel along the edges
of the gap, folded into it, so that together they cover only what the clip was
already hiding. `theme.AxisBreaks(mark, size, gap)` and
`theme.AxisFolds(size, gap)` set them; every length is scaled by
`theme.Scaled`, and the gaps by `theme.Density`.

### 8. The dialect spells them `"cuts"` and `"folds"`

```json
"y": {"field": "load", "scale": {"type": "linear", "domain": [0, 100], "cuts": [[12, 85]]}}
```

Each is a list of pairs written the way `domain` is — numbers, or timestamps on
a time axis, measured against the document's `origin`. `"breaks"` is the
threshold colour scale's word on the same struct and a different shape, and
ADR 0075's argument that the channel decides a word's meaning does not reach a
field whose type differs between the two. A document holds the intervals, never
the query they were computed with: a chart built from `SpansWhere` writes the
spans it found.

## Consequences

- A chart with no break — which is every chart there was — draws exactly what
  it drew. `TestAChartWithoutBreaksIsUnchanged` compares the bytes of a chart
  whose break lies outside its domain against one with no break at all, and
  every existing golden file still matches.
- A broken chart's clip is a path of several rectangles, so a raster backend
  masks rather than scissors it. That cost is paid only by charts with a break,
  and every figure stays under `TestNoFigureIsDrawnWithTooManyCalls`.
- The accessible description names the omitted intervals — "The vertical axis
  is broken, leaving out 10 to 88." A sighted reader is told by the mark; a
  listener has to be told in words, or the description says two distances are
  alike that are not.
- A pan that moves a break across the end of the view trims the axis to the far
  side of it rather than leaving a gap at the edge. That is the rule in claim
  4; it means dragging a view through a break jumps the axis by the break's
  width, which is what a break is.
- A layer drawn on a second axis is not cut by that axis's breaks, because
  they are off. It is cut by the primary axes' breaks, because the clip is the
  panel's.

## Not in scope

- **Breaks on a log, symlog or probability axis.** The arithmetic is the same
  per piece, in the transformed space; nobody has asked, and each of those
  scales writes its own mapping.
- **Recurring breaks as a rule** — every weekend, market hours, a business-day
  axis. That is a calendar rather than an interval list, and a caller can
  already pass the spans a calendar produces as folds.
- **Folds a layer derives by itself during `Train`.** An axis whose shape
  depended on a layer's filter would change when a layer was added; the helper
  keeps the question explicit and the answer in the document.
- **Folding a category off an ordinal axis.** That is filtering the rows.
- **A different scale on each side of a break** — ggbreak's `scales=`. A break
  here leaves an interval out of one axis; it does not join two.
- **Marks on the panel's far edges.** A Cartesian panel draws its axis lines on
  the left and the bottom, and the marks are where the lines are.

## Revisit if

- Someone needs a business-day time axis, which is claim 6's machinery plus a
  calendar.
- A broken figure's raster clip shows up against the call-count wall: a union
  of disjoint rectangles is something `backend/gg` could learn to scissor one
  at a time.
- A coord other than Cartesian finds a way to mark a break — a radial axis has
  a spoke to draw a slash across — at which point it implements
  `coord.Breakable` and claim 4 lets it in.

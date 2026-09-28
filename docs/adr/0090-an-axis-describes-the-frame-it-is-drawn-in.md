# 0090 — An axis describes the frame it is drawn in: a trained range is derived again every render, and a set of names is remembered

**Status:** Accepted, amended · **Date:** 2026-09-28 · **Implemented:** 2026-09-28 · **Revisits:** [ADR 0016](0016-streaming-and-damage.md) · **Follows from:** [ADR 0089](0089-a-value-axis-fits-what-its-time-axis-shows.md)'s amendment

## Context

Building [ADR 0089](0089-a-value-axis-fits-what-its-time-axis-shows.md) found
that a fitted price axis stayed where it was after a zoom, and the reason was
not the fit. A scale's trained domain only ever widens — `domainRange.Train`
takes the minimum and maximum of what it has seen — and render keeps a panel's
scales across renders. So a scale trained on one frame's rows is trained on
the next frame's *as well*. 0089 fixed it for fitting axes by releasing them
before each frame, and recorded the question it left for every other axis.

A probe answers it. A `data.Stream` with a window of ten rows, drawn by a
`Live`:

| | rows in the window | X domain | Y domain |
|---|---|---|---|
| frame 1 | x 0…9, y 0…90 | [0, 9] | [0, 90] |
| frame 2 | x 20…29, y all 5 | **[0, 29]** | **[0, 90]** |
| `p.Render` after it | the same | [0, 29] | [0, 90] |

The window has moved on and the axes have not. The time axis covers the whole
history the stream has already dropped, so the ten rows it holds are squeezed
into the right third of the panel; the value axis still reaches to a peak that
left the window nineteen rows ago, so a flat line at 5 is drawn along the
floor. `Plot.Render` called twice on a plot whose data changed does the same,
because it reuses the same scale objects.

Nothing decided this. [ADR 0016](0016-streaming-and-damage.md) designed the
stream and its window and says what a window is for — *"a live chart shows the
last few thousand samples"* — and `examples/stream` says what its full window
does: *"one whose window is full slides, and sliding is what damage tracking is
good at."* It does not slide. It was never tested, because the example pins its
value axis (`scale.Domain(0, 120)`) and its tests check that frames are drawn,
not where. The behaviour is an accident of two correct pieces — a scale that
accumulates within a render, which is what lets several layers share an axis,
and a plot that keeps its scales between renders, which is what lets a zoom
survive the next frame — meeting in a case neither was written for.

## Decision

**Every render derives a trained range again from the rows it draws. A pinned
domain is not trained and is kept; a set of discovered names is identity and
is kept; and a scale that should remember its extent says so.** Six claims.

### 1. A trained range is forgotten at the start of each render

Before any layer trains, render asks every scale it will train — each panel's
X, Y, X2 and Y2, the tracks' own scales, and the colour and size scales its
layers describe (claim 4) — to forget what it was trained on, once per scale
object. Then the layers train as they always have, accumulating across layers
within the render. The domain an axis ends a render with is therefore the
extent of the rows that render drew, which is what a reader takes an axis to
say.

"Once per scale object" is the same rule 0089's `refit` had, for the same
reason: panels that share an axis share one scale, and forgetting it between
two of them would forget the first panel's rows.

The forgetting is a new optional method, `Retrain()`, behind an interface of
its own — `scale.Retrainer` — on the scales with a trained range. It is not
`Zoomer.Autoscale`: autoscale releases a pin, and a pinned domain must survive
(claim 2). `Train` keeps its documented contract, *calling Train repeatedly
accumulates*, because within a render that is exactly what is wanted.

### 2. A pinned domain is kept

A domain fixed at construction (`scale.Domain`, `LogDomain`), set by a zoom or
a pan, or set by a caller's `Zoomer.SetDomain` is not trained and is not
forgotten: `Retrain` on a pinned scale does nothing. That is what keeps a zoom
across frames, the one thing the old behaviour was quietly also providing —
and it provided it only because training a pinned scale is already a no-op, so
nothing about zooming changes.

A fitting axis (0089) is unpinned by construction and so is simply one of the
scales this retrains. 0089's `refit` becomes this rule and is removed.

### 3. A set of names is remembered

An ordinal axis that discovers its categories, and a qualitative or named
colour scale that discovers its keys, are **not** retrained. A category's slot
on an axis and a series' colour are identity: a reader who learned that
"Störung" is the third row, or that the orange line is machine B, reads every
later frame by it. Rediscovering them per frame would reorder the slots and
repaint the series whenever the rows arrived in a different order, and a
colour that changes between frames is a colour that lies about which line is
which.

So a name, once seen, keeps its slot and its colour until the plot is rebuilt;
a category that stopped appearing leaves its slot empty, which is visible and
true. A caller who wants the set to follow the data fixes it
(`scale.Categories`) or rebuilds. This is the one asymmetry in the record and
the reason it is stated as a claim: a *range* is a measurement of the rows, a
*set of names* is a vocabulary for them.

### 4. Continuous colour and size scales follow the range rule

A sequential or diverging colour ramp and a size scale have a trained range
like an axis, and the same accident: a heatmap of a stream keeps its colour bar
stretched to a peak the window dropped, and every cell after it is drawn in
the bottom third of the ramp. They are retrained with the axes. Render reaches
them through each layer's `Describe` — `Desc.ColorScale` and `Desc.SizeScale`
— which every layer in the package answers; a third-party layer that does not
describe itself keeps its colour scale's old behaviour, which is the old
behaviour and not a wrong one.

A discrete colour scale is claim 3's, and a bivariate one retrains its range
and keeps its classes.

### 5. A scale that should remember says so

`scale.HighWater()` is a linear option for the chart that wants the old
behaviour on purpose: a monitor whose axis shows the worst load since it was
opened, a counter that must not appear to fall. It accumulates across renders
until `Autoscale`. It is an option rather than the default because it is the
rarer question — *what is the most this has ever been* — and because a chart
that answers it should say that it does, which a default cannot.

### 6. What it costs

- **An axis now moves when its data does.** A live chart with an unpinned value
  axis relabels whenever the extent of its window changes — which is what it
  claims to show, and what `examples/stream`'s comment argues for pinning
  against. That example keeps its pinned axis; its time axis, unpinned, now
  slides as its comment says, and the damage pass redraws what sliding moves.
- **A static render is unchanged.** One `Plot.Render` on fresh scales retrains
  nothing that was trained, and no golden file changes — the claim a test
  makes by running the golden suite untouched.
- **Retraining is O(scales), not O(rows)**: a scale forgets two numbers.

## Consequences

- A windowed stream's time axis slides, and its value axis describes the
  window.
- `Plot.Render` called again after the data changed draws the data it has now.
- A zoom, a pan and a fixed domain are kept exactly as before.
- Series keep their colours and categories their slots across frames.
- `scale` gains `Retrainer` and `HighWater`; render gains one pass before
  training; 0089's `refit` is folded into it.

## Not in scope

- **Easing an axis between frames.** A moving axis jumps; animating it is a
  transition ([ADR 0063](0063-an-overlay-over-a-scene.md)-adjacent host work),
  not a scale.
- **Forgetting names.** Claim 3. A vocabulary that should shrink is a caller's
  `scale.Categories`, or a rebuild.
- **Three-dimensional scenes.** A `three.Scene` trains its own scales on its own
  schedule — once per render, not once per view, by its own comment — and
  follows the same rule by the same mechanism when it next needs to; it is not
  changed here.

## Revisit if

- A category axis over a stream turns out to need forgetting often enough that
  fixing its categories is the recipe's cost everywhere.
- A third-party layer's colour scale is reported to grow; that is a layer that
  does not describe itself, and the answer is `Describer`, not a second route.

## Order of work

Every step is built.

1. `scale.Retrainer` and `scale.Retrain` in `scale/retrain.go`, on linear, log,
   symlog, time, probability, the continuous and classed colour scales, both
   bivariate scales and the size scale; a no-op when pinned. `scale.HighWater`,
   written `"highWater": true`.
2. Render's `retrain` pass before training, replacing 0089's `refit`. Tested
   with the probe's stream: X slides to the window, Y drops the peak, a zoom
   survives, `HighWater` remembers, a series keeps its colour and a category
   its slot, and a continuous ramp describes the frame.
3. `examples/stream` gains `TestAFullWindowSlides`, deterministic rather than
   raced against its producer, and its chart is built by a function the test
   shares.

## Amendment — what building it decided

- **No identity check.** Render forgets every reachable scale, shared ones
  perhaps several times, before any layer trains; forgetting is idempotent, so
  there is no set of scales to build and no interface comparison — which would
  panic on a third-party scale whose dynamic type is not comparable.
- **A classed scale forgets its sample, not its classes.** A quantile scale's
  boundaries are cut from the values it was trained on, so they are forgotten
  with the range and cut again; a threshold scale's boundaries are given and
  stay.
- **A flat window is still padded.** A window whose values are all 5 trains a
  domain of [4.75, 5.25], the padding every degenerate linear domain gets; what
  the record promised is that the peak it dropped is gone, and that is what the
  test checks.
- **Nothing else moved.** No golden file changed and every existing test
  passed unchanged: a single render on fresh scales forgets nothing that was
  trained, and a zoom was already a pinned domain.

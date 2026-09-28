# 0087 — An orientation is the encoding read a quarter turn round, and a mark turns only where it emits

**Status:** Accepted · **Date:** 2026-09-28 · **Implemented:** 2026-09-28 · **Revisits:** [ADR 0053](0053-tidy-tree-layout.md)'s amendment

## Context

[ADR 0053](0053-tidy-tree-layout.md)'s amendment gave the library an
orientation — `geom.Orient`, `geom.Vertical`, `geom.Horizontal` — and named it
for the library rather than for the tree, *"so that the second mark to want it
says it the same way — bars and boxplots still have none, and this is where
that answer will land."* Its doc comment said the rest: `Orient` on any mark
but the tree was accepted and ignored.

[ADR 0085](0085-what-a-market-chart-needs.md) found the second mark. A volume
profile is a bar per price bucket growing across a right track that shares the
price axis, and without a horizontal bar `examples/market` drew it as rects
with a hand-stacked `X` and `X2` — the one place the example was more code
than the chart. The same gap is the long list of named categories that reads
best down the page, the population pyramid, the ranked bar chart, and a
distribution drawn beside the axis it is a distribution of.

The option also already half-worked, which is worse than not working: the
gradient in `config.paint` read it for every filled mark, so a vertical bar
given `Orient(Horizontal)` and a `Gradient` was painted with a left-to-right
ramp.

## Decision

**Under `Horizontal` a mark's slots are its `Y` column and its measurement its
`X` one, the far ends swap with them, and the mark computes in slot and value
space and turns once, where it emits.** Five claims.

### 1. The roles swap with the axes

```go
geom.Bar(src, geom.Y("fruit"), geom.X("count"), geom.Orient(geom.Horizontal))
```

The caller names the columns on the axes they are drawn on, as Vega-Lite and
the tree do: the categories on `Y`, the value on `X`. `X2` is then the value's
far end — a floating bar's other bound — and `Y2` the slot's far edge. That is
the vertical encoding read a quarter turn round, and it is spelled
`config.roles()`: a copy of the layer's config with the positional columns
renamed for the roles they play, used only to read columns. The layer's own
config is what it describes itself with, so a document holds the columns the
caller named, not the swapped ones.

The orientation is stated rather than inferred. Vega-Lite infers it from which
channel is quantitative, and a figure layer does not know its scales' kinds
until it trains — and a bar of numbers against numbers, which is every bar on
a time axis, would have to guess.

### 2. A mark turns only where it emits

`config.axes(x, y)` splits the panel's scales into the slot scale and the value
scale; `config.box(s0, v0, s1, v1)` and `config.point(s, v)` turn a rectangle
or a point computed in slot and value space into the space the scales map
into. A mark reads its columns through `roles`, trains and maps through
`axes`, and calls `box` or `point` where it used to call `ir.R` or pass `x, y`
to the coord. Everything after that — the coord, `Explode`, `Extrude`,
hit-testing, the gradient — sees an ordinary rectangle and needs to know
nothing. The baseline is `baselineOn`, which is `baselinePos` or the
`baselineAcross` a set-size bar already had, whichever axis the value runs
along.

That is the tree's own strategy — lay it out once, hang it on the other pair
of axes — made into three functions the next mark calls rather than a fourth
copy of the swap.

### 3. A dodged group reads down the page

Dodging places a slot's series in ascending device order. Across X that is
left to right, which is the order of the legend; up a Y axis it would be top
to bottom as well, so the first series is the top bar of its slot rather than
the bottom one a literal quarter turn would give. It is the order a reader
reads a horizontal grouped bar and its legend in, and the order Vega-Lite
draws one in.

### 4. Which marks read it

- **Read it:** `Tree` (0053), `Bar`, `Histogram` (it bins the `Y` column) and
  `Boxplot` (it groups by `Y` and summarises `X`).
- **Need no option:** `Rect` and `ErrorBar`, because which edges a row names
  already says which way they run; ErrorBar's doc comment refuses an
  orientation option for exactly this reason, and keeps refusing it.
- **Next, when asked for:** `Violin`, `Beeswarm` and `ECDF`, each a slot or
  value swap at its emit points; and `Area`, whose decimation reduces by pixel
  column along X and so has to be told which axis is the independent one.
  `Ridgeline` is already the horizontal density.
- **Not ever:** `Candle` ([ADR 0088](0088-a-candle-is-one-mark-that-reads-four-values.md)),
  which nobody draws on its side. `Line` and `Scatter`, which have no slot — a line lying down is
  a line with its columns exchanged. `Trend`, where a horizontal fit is a
  regression of X on Y, a different statistic rather than the same one turned.
  `Horizon`, whose fold replaces its vertical axis. `Step`, until someone needs
  a staircase indexed by Y.

`Orient` on a mark outside the first list stays accepted and ignored, as the
doc comment says, so that a layer built from a shared option list does not
fail.

### 5. The dialect

The mark writes `"orientation": "horizontal"`, figure's own name from 0053, on
the marks that read it and on no others. A horizontal stack is written on the
**X** channel, which is where Vega-Lite puts the stack of a bar lying down.
Reading, a Vega-Lite `"orient": "horizontal"` on a `bar` or a `boxplot` is
taken as the same thing: there, orient means exactly this. On a rule and a
rect it chooses between two marks instead, and is read where it always was.

## Consequences

- A horizontal bar chart, a population pyramid, a ranked list and a volume
  profile are one option. `examples/market` draws its profile as a stacked
  horizontal `Bar` from `stat.BinWeighted`, and the picture is identical to the
  hand-stacked rects it replaces.
- The gradient on a horizontal bar ramps along the bar, which is the half of
  the option that already worked finally meeting the other half.
- A mark added later turns by calling `axes`, `roles`, `box` and `point`, not
  by writing its own swap.

## Not in scope

- **Inferring the orientation** from the scales. Claim 1.
- **A horizontal line or scatter.** Claim 4.
- **Orientation under polar coords as a named form.** A horizontal bar under
  `coord.Polar` puts the value on the angle — a radial bar chart — and it
  falls out of claim 2; it is not tested as a form of its own until someone
  draws one.

## Revisit if

- A mark in the *next* list is asked for; each is a small change of the shape
  this record describes, and Area's is the one that reaches into shared code.

# 0091 — A direction is a colour channel any mark can take, read from two columns and painted in the candle's colours

**Status:** Accepted, amended · **Date:** 2026-09-28 · **Implemented:** 2026-09-28 · **Revisits:** [ADR 0088](0088-a-candle-is-one-mark-that-reads-four-values.md)'s *Revisit if*

## Context

[ADR 0088](0088-a-candle-is-one-mark-that-reads-four-values.md) gave the
candle its own direction: the mark reads open and close and decides, per row,
whether the row rose. It left one thing on its *Revisit if*: *"Charts colour
their volume bars by direction often enough that computing the column is the
recipe's cost all over again."* They do. Every chart in this branch that has a
volume track colours it by direction, and every one of them pays for it the way
the candle used to:

- `examples/market` computes a `dir` column of `"up"` and `"down"` by hand and
  colours the volume bars through a `scale.Named` built from the same two
  colours the candle layer was given — two places that must agree, and nothing
  that says so when they do not.
- `examples/market -live` cannot do it at all. Its rows come from a
  `data.Stream`, which carries numbers only (ADR 0016), so the volume bars of
  the live chart are one colour.
- `SincePrevious` has no volume counterpart. A candle told to compare with the
  previous close colours a gap-down day falling; the volume bar under it, fed
  a column computed from open and close, colours the same day rising.

The question is not special to markets. A bar coloured by whether actual beat
plan, a dumbbell coloured by whether a value improved, a line whose segments
are drawn by whether the series is climbing — each is a row's colour decided
by comparing two of its values, and each today is a derived column the
`data.Table` does not have.

## Decision

**`geom.DirectionBy(from, to)` is a colour channel: each row rises when its
`to` is at or above its `from`, and is painted in the rising or falling colour
the candle uses. Any mark that takes `ColorBy` takes it, and the two are
exclusive.** Six claims.

```go
p.Add(geom.Candle(src, geom.X("start"), geom.X2("end"), geom.OHLC("open", "high", "low", "close")))
p.Track(figure.Bottom, …).
	Add(geom.Bar(src, geom.X("start"), geom.Y("volume"), geom.DirectionBy("open", "close")))
```

### 1. The direction is read from two columns, or from one and the row before

`DirectionBy(from, to)` compares two numeric columns of the same row: the row
rises when `to ≥ from` — the candle's rule, a tie rising. `DirectionBy("", to)`
compares `to` with the previous row's `to`, which is the candle's
`SincePrevious`; the first row has nothing to compare with and rises, as the
candle's first row falls back to its own open.

Two columns rather than a column of signs because the columns are what the
table has. A candle's source has an open and a close and no direction; a
stream has numbers and no strings; a plan-and-actual table has two figures.
Asking any of them for a third column is the recipe this record removes.

The previous row is the previous row *of the layer*, in table order, which is
the order a stream appends in and the order `stat.OHLC` writes candles. A
layer that draws its rows out of order is a layer whose "previous" means
something else, and it should say so with two columns.

### 2. It is a colour channel, so every mark that has one takes it

The direction becomes each row's colour through the channel `ColorBy` already
feeds: a mark that paints per row from a colour column paints per row from the
direction, with no code of its own. That is `Bar`, `Rect`, `Scatter`,
`ErrorBar`, `Text`, `Area` and the paths — a `Line` or `Step` coloured by
direction draws each stretch rising or falling, which is the "trend line
coloured by slope" nobody could draw before.

`ColorBy` and `DirectionBy` on one layer is an error at `Train`: a row has one
colour, and a layer that named two ways of choosing it has not said which. A
mark that does not take `ColorBy` — a histogram, a relational layout — does not
take this either, and says so the way it says so for `ColorBy`.

### 3. The colours are the candle's

`geom.Rising(color, label)` and `geom.Falling(color, label)` — the candle's two
options — set the colours and the labels; without them, the theme's `Rising`
and `Falling` and the labels "rising" and "falling". A volume track under a
candle chart that names neither therefore paints in exactly the candle's
colours, and one where both were given the same options paints in the same
ones — the agreement the hand-written `scale.Named` had to be kept in by hand.

The colours resolve against the theme a frame is drawn in, as the candle's do,
so a chart switched to the dark theme repaints its volume bars with it. A row
whose `from` or `to` is missing is drawn in the colour a colour scale gives an
undefined value — it has no direction to show — and is an error under
`OnMissing(Error)`.

### 4. The legend says rising and falling once

A direction layer contributes two legend entries, labelled as claim 3 says.
Render already merges legend entries by label, so a volume track under a
candle layer with the same labels adds nothing to a legend that already reads
rising and falling; a layer whose labels differ adds its own two, which is
what a chart with two different notions of direction should show.

### 5. The dialect

The direction is two encoding channels, figure's own: `"direction"` for `to`,
and `"directionFrom"` for `from`, left out when it is the previous row. The
rising and falling colours and labels are the mark properties 0088 already
defined, now written for any mark that carries a direction.

### 6. What it does not do

- **Three states.** Unchanged is rising, as the candle has it. A chart that
  wants "flat" as a third colour has a threshold, not a direction, and
  `ColorBy` over a difference column is what it is.
- **A magnitude.** How far a row rose is a quantity, and a quantity is
  `ColorBy` through a diverging ramp. Direction is the binary reading, and the
  reason it earns a channel is that it needs no column.

## Consequences

- `examples/market`'s `dir` column and its `scale.Named` disappear; its volume
  bars take `DirectionBy("open", "close")`.
- The live chart's volume track is coloured by direction for the first time,
  straight from the stream's numeric columns.
- A candle using `SincePrevious` and a volume track using `DirectionBy("",
  "close")` agree on every row.
- `geom` gains one option; the candle's `Rising` and `Falling` are read by
  every mark that carries a direction; the dialect gains two channels.

## Not in scope

- **Derived columns in general.** `data.Table` still has none. A direction is
  the one comparison common enough to be a channel; anything else — a spread, a
  return, a ratio — is a column the caller computes, or a function in `stat`.
- **Direction on a mark without a colour channel.** Claim 2.
- **Hollow and filled on other marks.** The candle's second channel for
  direction (0088 claim 3) is about bodies; a bar is already one filled
  rectangle, and a hollow volume bar reads as missing rather than as rising.

## Revisit if

- A third comparison — "above the moving average", "inside the band" — is
  wanted often enough to be a channel too, which would make this one of a
  family rather than a special case.

## Order of work

Every step is built.

1. `geom.DirectionBy` in `geom/direction.go`, with `ErrColorAndDirection`;
   tested on a bar, a line and a scatter, for a tie, for the previous-row form,
   against a candle in both directions, for the legend's two labels and for the
   Desc round trip. At chart level: a volume track under candles adds nothing
   to the legend, and a theme's `Rising` and `Falling` repaint the bars.
2. `"direction"` and `"directionFrom"` in the dialect, round-tripped on a bar,
   a scatter with named colours and a line.
3. `examples/market` drops its `dir` column and its `scale.Named`; `-live`
   colours its volume track from the stream's numbers.

## Amendment — what building it decided

- **A direction is a discrete colour scale of two labels.** `DirectionBy`
  installs a scale whose labels are rising and falling and whose values are
  each row's direction, so everything already written for a discrete colour —
  batching bars by colour, a path coloured in stretches, one legend entry per
  label, the undefined colour for a row with none — works unchanged. No mark
  has a line of direction code.
- **The theme is bound in render's retrain pass.** The scale's colours are the
  layer's options where given and the theme's otherwise, and a layer trains
  without a theme — so render hands each colour scale that asks for one the
  theme of the render it is in, in the same pass that retrains
  ([ADR 0090](0090-an-axis-describes-the-frame-it-is-drawn-in.md)). A scale
  never bound falls back to the shipped themes' blue and vermilion.
- **A row with no direction is not drawn.** The undefined colour of a colour
  scale is transparent, so a row missing either value leaves a gap, which is
  what claim 3 meant by "drawn in the colour a colour scale gives an undefined
  value".
- **The first row of the previous-row form rises.** A candle under
  `SincePrevious` falls back to its own open for its first row, so the two
  agree there only when that first candle rose. The test compares them on data
  where it did; a chart whose first candle fell sees its first volume bar in
  the other colour.

# 0088 — A candle is one mark that reads four values and decides its own direction

**Status:** Accepted, amended · **Date:** 2026-09-28 · **Implemented:** 2026-09-28 · **Ranked by:** [ADR 0085](0085-what-a-market-chart-needs.md), rank 2

## Context

[ADR 0085](0085-what-a-market-chart-needs.md) showed that a candlestick chart
composes from marks that exist, and `examples/market` draws one: an
`ErrorBar` with `Caps(false)` from low to high for the wick, a `Rect` from open
to close over it for the body, both coloured through `scale.Named` from a
direction column the caller computes. It ranked a mark of its own second,
after the calendar, because the recipe costs the same four things on every
chart:

- **A column the data does not have.** Up or down is a function of two
  columns, and a `data.Table` has no derived columns, so every caller writes
  the loop that `stat.Candle.Up` shortens but cannot remove.
- **Two layers that must agree.** The wick and the body are two layers over
  the same rows with the same X, the same slot, the same colour scale and the
  same `BarWidth` story. A caller who changes one and not the other draws
  bodies beside their wicks, and nothing says so.
- **Two legend entries for one thing, or none.** Each layer contributes its
  own entry; the chart's legend lists the direction twice or the caller
  suppresses both.
- **Four values in a vocabulary of two.** Open, high, low and close are all
  values on the same axis. X/Y/Y2 names two of them; the recipe spends the
  wick's Y/Y2 on low/high and the body's on open/close, and a document of the
  recipe says *two intervals* where the reader sees *one candle*.

Nothing in the recipe is wrong, which is why it stays documented. The mark is
for the chart that is drawn a thousand times.

## Decision

**`geom.Candle` reads a position and four values, decides each row's direction
itself, and draws the wick and the body as one mark with one legend.** Eight
claims.

```go
p.Add(geom.Candle(src, geom.X("start"), geom.X2("end"),
	geom.OHLC("open", "high", "low", "close")))
```

### 1. The four values are one option

`geom.OHLC(open, high, low, close)` names the four columns in the order the
name of the chart spells them. It is one option rather than four because the
four are one statement — a row is a candle only with all of them — and because
`geom.Open`, `geom.High`, `geom.Low` and `geom.Close` would be four names in a
package shared by every mark, three of which read as something other than a
column (`geom.Close` closes nothing). A layer without `OHLC` is an error at
`Train`, naming the option.

The position is the ordinary one: `X` for where the candle stands, and `X2`
when the row names both edges of its period — which is what a candle out of
`stat.OHLC` knows (claim 7). With `X` alone the slot is the closest spacing in
the data or a band scale's own width, exactly as a `Bar`'s.

### 2. The mark decides the direction

A row is **rising** when its close is at or above its open and **falling**
otherwise — `stat.Candle.Up`, now read by the mark rather than by the caller.
A close equal to its open is a *doji*, rising by that rule, and its body is
drawn at least one device pixel tall so that it is a mark rather than nothing.

`geom.Direction(geom.SincePrevious)` is the other convention some platforms
use: a row rises when its close is at or above the *previous row's* close, so a
gap-down day that rallied from its open still reads as down. The first row has
no previous close and falls back to its own open. The default is
`geom.SinceOpen`, the candle's own statement.

### 3. Direction is drawn twice: colour, and hollow or filled

`geom.Rising(color, label)` and `geom.Falling(color, label)` set each
direction's colour and legend label. Their defaults come from the theme —
two new theme fields, `Rising` and `Falling`, blue and vermilion in both
shipped themes, because the red and green of a trading screen is the pair the
most common colour-vision deficiency cannot tell apart — and the labels
default to "rising" and "falling".

`geom.Hollow(true)` draws rising bodies as outlines and falling bodies filled,
the convention of a chart printed in one colour. It is on by default under a
theme with redundant encoding (`theme.Redundant`), which is
[ADR 0069](0069-hatching-as-the-third-redundant-channel.md)'s argument about a
second channel applied to the one a candle already has: a direction that
survives a greyscale print is one that does not depend on hue.

`geom.ColorBy` replaces the direction's colours with a column's — a regime, a
volume decile — and leaves hollow and filled to carry the direction. A candle
coloured by something else still says which way it went.

### 4. A candle is a body and a wick, or an OHLC bar

`geom.CandleStyle(geom.Bodies)` is the default. `geom.CandleStyle(geom.Ticks)`
is the OHLC bar: a rule from low to high, a tick to the left at the open and
one to the right at the close, a tick as long as half the body would be wide.
It is a style of this mark rather than a mark of its own because everything
but the drawing is the same — the columns, the direction, the slot, the
legend, the rows a pointer lands on — and a second mark would be a second copy
of all of it that could drift. The document spells it as a property of the
mark (claim 8).

### 5. It draws in four calls, whatever the row count

The wicks of one direction are one stroked path and the bodies of one
direction one filled path — or, hollow, one stroked outline: two calls per
direction, four for the layer. A thousand candles are the same
four calls as ten. That is the batching `geom.Rect` does by colour, for the
reason [ADR 0007](0007-per-mark-colour.md) refuses a colour per mark in the IR.
A layer coloured by `ColorBy` batches by its colours instead, as `Rect` does.

The body's width is `BarWidth` of the slot, default **0.7** rather than a
bar's 0.8: between two candles is where the eye separates their wicks, and a
bar has no wick to separate. The wick is `geom.Width` device pixels wide,
default 1.

### 6. It trains, reports and refuses like the marks around it

- **The value axis** is trained on each row's low and high, and on its open and
  close as well, so a row whose data is inconsistent — a high below its close —
  still has its whole drawing inside the axis.
- **Inconsistent data is drawn as given.** A body outside its own wick is
  drawn outside it. Repairing it — stretching the wick to the body — would hide
  an error in the data, and a candle is exactly where a reader checks one.
- **A row missing any of the four values is skipped**, and is an error under
  `OnMissing(Error)`, as a bar's missing value is.
- **The row is reported at its close**, in the middle of its slot: the value a
  reader means when they point at a candle, and the one a crosshair reads out.
  A tooltip then has the whole row, four values and all.
- **`Orient` is not read.** A candle lying on its side is a chart nobody draws;
  [ADR 0087](0087-an-orientation-is-the-encoding-read-a-quarter-turn-round.md)'s
  list gains it under *not ever*.
- **Under a polar coord** it is drawn through the coord like a `Rect`, and not
  tested as a form of its own.

### 7. The output of `stat.OHLC` is a table in one call

`stat.Candle` gains an `End` field: the start of the next period for
`stat.OHLC` (start plus the width) and `stat.OHLCAt` (the next edge), and the
time of its last row for the last candle of `OHLCAt`, whose period the data
has not closed. It is a field added to a type that has not been released.

`figure.CandleTable(cs []stat.Candle)` returns a `*data.Table` with the
columns `start`, `end`, `open`, `high`, `low`, `close`, `volume` and `count`,
which is what `geom.Candle` with `X("start")`, `X2("end")` and
`OHLC("open", "high", "low", "close")` reads. It is in the root package for the
reason `SpansWhere` is: `stat` does not know a table and `data` does not know a
candle. The columns are float64 domain values; on a time axis they are
`scale.Nanos`, which is what a time scale without an origin reads.

### 8. The dialect

The mark is `"candlestick"`, figure's own word — Vega-Lite has no candle mark
and draws one as the same two-layer recipe. The four values are four
encoding channels, `"open"`, `"high"`, `"low"` and `"close"`, also figure's,
beside the ordinary `"x"` and `"x2"`. They are not spelled as `y` and `y2`,
because a consumer reading `y` as *the* value of a row would read a candle as
its open and nothing else. The mark's properties carry `"direction":
"previous"`, `"hollow"` and `"candleStyle": "ticks"` when they differ from the
defaults, and the rising and falling colours and labels.

## Consequences

- A candle chart is one layer, and the legend says rising and falling once.
- The direction column disappears from `examples/market`, and so does the
  second layer. The recipe stays in `docs/chart-types.md` as what a candle is
  made of.
- A chart printed in one colour keeps its direction, through hollow bodies.
- The theme gains two colours; every shipped theme sets them.
- Volume bars coloured by direction still need the direction as a column: the
  candle decides it for itself, not for the layer beside it. See *Revisit if*.

## Not in scope

- **Heikin-Ashi and other derived candles.** They are a transform of the four
  columns into four others — numbers in, numbers out — and belong in `stat` as
  a function whose output this mark draws unchanged.
- **Renko, Kagi, point-and-figure.** Charts of price bricks on an X that is not
  time; ADR 0085 left them out and this record does too.
- **Decimation.** Unlike a line's, a candle's aggregation is exact — the open of
  the first, the close of the last, the highest high and the lowest low — so a
  chart of more candles than pixels could merge them without inventing
  anything. It is `stat.OHLC` at a coarser width today, the caller's to call;
  whether the mark does it itself is a question for when a chart needs it.
- **Volume inside the candle** — width or opacity by volume. A second channel
  on a mark whose reading is already four values; the volume bar in a track is
  the reading people know.

## Revisit if

- Charts colour their volume bars by direction often enough that computing the
  column is the recipe's cost all over again — which would be a `geom.Bar`
  option reading the same four columns, or a `stat` function writing the
  direction.
- A chart of more candles than its width can hold asks for the exact merge
  above.

## Order of work

Every step is built.

1. `stat.Candle.End`, set by `OHLC` and `OHLCAt`; `figure.CandleTable`.
2. `geom.Candle` in `geom/candle.go`, with `OHLC`, `Direction`, `Rising`,
   `Falling`, `Hollow` and `CandleStyle`, and the theme's `Rising` and
   `Falling`; tested for four calls over a thousand rows, a visible doji,
   inconsistent data drawn as given, the row at its close, a period's share
   and the Desc round trip.
3. The dialect's `"candlestick"` with its four channels, round-tripped in
   three configurations.
4. `examples/market` draws its candles with the mark; `candles` and
   `candles-ticks` are goldens of both styles over a folded fortnight.

## Amendment — what building it decided

- **The mark is `geom.MarkCandle`, `"candle"`, and the document's word is
  `"candlestick"`.** The package's names are its own and the dialect
  translates them, as it does for every other mark.
- **A hollow batch is its own batch.** Batches are keyed by colour and by
  whether the body is drawn hollow, so a layer coloured by a column under
  `Hollow` still strokes its rising bodies and fills its falling ones. A
  hollow batch strokes its bodies instead of filling them, so every batch is
  still two calls — a direction-coloured layer is four, hollow or not — and a
  layer coloured by a column is two per colour, never one per row.
- **The legend is two box swatches**, one per direction, or the colour
  column's own entries under `ColorBy`. A hollow swatch would need a swatch
  kind the legend does not have; the direction's label says what the outline
  says.
- **A tick on the edge where a fold starts moves too.** Building the goldens
  found the one case ADR 0086's claim 5 missed: a weekend fold starts at
  Saturday midnight, a two-day tick landed exactly there, and it stood on the
  end of Friday labelled with Saturday. A tick at a fold's lower edge now
  moves to its upper one like a tick inside it; 0086's amendment records it.
- **A candle reaches the edge of its axis.** The value axis is trained on the
  lowest low and the highest high and nothing more, so the extreme wicks touch
  the panel's edge — the same fit a bar gets, and `scale.Nice` rounds the
  ticks rather than the domain.

# 0092 — What a trading screen reads off its edges: a value is tagged on its axis, a crosshair snaps to a row, and an indicator earns a function only where composing it goes wrong

**Status:** Proposed · **Date:** 2026-09-28 · **Ranked by:** [ADR 0085](0085-what-a-market-chart-needs.md), rank 5

## Context

[ADR 0085](0085-what-a-market-chart-needs.md) put three things last, together:
named indicators, a last-price label, and a crosshair that snaps to the
candle. Ranks 1 to 4 are built (ADRs 0086 to 0091), so a market chart now has
its calendar, its candles, a price axis that fits the zoom, a live last
candle and volume by direction. What it still lacks is what a trader reads off
the chart's *edges* rather than its middle, and the handful of indicators every
platform names.

The three look unrelated. Two of them are not:

- **The last price** is a number written on the price axis at the height of
  the last close, in the colour of the last candle, with a thin rule across the
  panel to it. Every trading screen draws it, because the right-hand edge is
  where the reader looks and the axis is where they read a value.
- **A crosshair's readout** is the same thing at the pointer: the price at its
  height tagged on the price axis, the time at its position tagged on the time
  axis. `figure.Crosshair` draws the two rules and nothing else, clipped to the
  panel so that a rule does not cross the axis it is read against.

Both are a value written *on an axis*, over its tick labels, and nothing in the
library draws there. Every mark is clipped to its panel by render; the gutter
belongs to the axes. So this is one mechanism, needed twice.

The third, snapping, is a question about rows rather than drawing. A
crosshair's `At` comes from the host's hover handler, and `interact.Index.At`
answers with the nearest mark in the plane — which, between two candles, is
the one the pointer is nearest to in both directions, and over a tall candle
may be a different one than the time the pointer is at. A trader means *the
candle at this time*: nearest along X, whatever the height.

And the indicators: ADR 0085 left RSI, MACD and VWAP as compositions of the
trailing functions and asked whether they earn names. The answer differs per
indicator, which is why it is a decision.

## Decision

**A value can be tagged on its axis: render draws it in the axis's gutter, over
the tick labels it covers. The last value of a column is a mark that draws a
rule and a tag. A crosshair can tag its own position, and the index answers
which row is nearest along one axis. `stat` gains RSI and VWAP, which composing
gets wrong, and not MACD, which composing gets right.** Four parts.

### 1. An axis tag is furniture that a layer or an overlay asks for

```go
type AxisTag struct {
	Axis  Axis     // X, Y, X2 or Y2
	Value float64  // in the axis's domain
	Text  string   // empty: the axis's own tick formatter writes the value
	Color ir.Color // the tag's fill; its text is chosen to read against it
}
```

A tag is a filled box on the axis line at `Value`, holding `Text`, drawn after
the axes and before the guides. Its text is the axis's own tick label format
for that value unless given, so a price tag reads `1,234.50` where the ticks
read `1,234` and a time tag reads the time the way the axis writes it. The box
reaches into the gutter by the width of its text; the tick labels it covers are
not drawn, because a tag is the more specific statement — the value *here* —
and two labels overprinting each other are two labels nobody can read.

A layer asks for tags through an optional interface, `geom.Tagger { Tags(f
Frame) []AxisTag }`, rather than a method on `Geom`, which does not grow
([ADR 0084](0084-what-v1-promises.md)). An overlay asks through the same
struct, drawn by the overlay after render has laid the axes out. A tag at a
value the axis does not show — off a zoomed axis, inside a fold — is not drawn,
and a tag on an axis the coord does not draw — a polar chart — is not either.

Tags on one axis are placed in value order and pushed apart until they do not
overlap, the way `Text` labels are placed ([ADR 0040](0040-label-collision-avoidance.md)),
so a last price and a crosshair readout a pixel apart are both legible.

### 2. The last value is a mark

```go
geom.LastValue(src, geom.X("start"), geom.Y("close"), geom.DirectionBy("", "close"))
```

`geom.LastValue` reads a position and a value column, takes the row with the
greatest position — the newest, whatever order the rows are in — and draws a
rule across the panel at its value and a tag on the value axis. Its colour is
`Color`, or its direction's through `DirectionBy`
([ADR 0091](0091-a-direction-is-a-colour-channel-any-mark-can-take.md)), so the
tag is blue when the last candle closed up. `geom.Dash` styles the rule and
`geom.Rule(false)` leaves the tag alone. On a stream it follows the open candle
by construction, because the open candle is the stream's last row
([ADR 0089](0089-a-value-axis-fits-what-its-time-axis-shows.md)).

It is a mark rather than an option on `Candle` because the question is not a
candle's: a line of a sensor reading, a gauge's history and a portfolio value
all end at a number the reader wants on the axis.

### 3. A crosshair snaps to a row and tags its axes

`interact.Index.NearestAlong(panel int, axis Axis, at float32) (RowRef, bool)`
answers the row whose reported position is nearest `at` along one axis, over
the layers of a panel that track rows. It is the query a trader's crosshair
needs — the candle at this time — and a scatter's does not, which is why it is
a query beside `At` rather than a change to it.

`Crosshair` gains `SnapX` and `Tags`:

- **`SnapX`** — the host passes the pointer as before; the crosshair's vertical
  rule is drawn at the X of the row `NearestAlong` finds, and its horizontal
  rule at that row's reported position — for a candle, its close
  ([ADR 0088](0088-a-candle-is-one-mark-that-reads-four-values.md) claim 6).
  Without row tracking there is nothing to snap to and the crosshair follows
  the pointer, as today.
- **`Tags`** — the crosshair tags its X and Y on their axes, through claim 1,
  in the theme's annotation colour.

Snapping is an option of the overlay rather than a host's arithmetic because
the overlay already has the frame and the index, and a host that has to invert
a device position, query the index and map the answer back is a host that gets
one of the three steps wrong.

### 4. An indicator earns a function where composing it goes wrong

- **RSI earns one.** It is not a composition of the functions `stat` has: its
  averages are Wilder's, an exponential average with α = 1/n rather than the
  2/(n+1) `EMA` uses, seeded with the simple mean of the first n changes. A
  reader told "RSI 14" expects that definition, and a composition from `EMA`
  gives a different curve that is close enough to be trusted and wrong enough
  to be noticed. `stat.RSI(xs, ys, window)` and its Append pair, in [0, 100].
- **VWAP earns one.** It is two cumulative sums divided — but reset at the
  start of every session, which is the part a composition from `Cumsum`
  forgets: a VWAP that runs across the overnight is not the number any
  platform shows. `stat.VWAP(ts, ps, vs, edges)` takes the session edges a
  calendar's `Opens` gives ([ADR 0086](0086-a-calendar-says-when-time-counts.md)),
  as `OHLCAt` does.
- **MACD does not.** It is `EMA(12) − EMA(26)` and a signal line `EMA(9)` of that
  difference, and that composition is the definition — nothing in it is
  seeded, reset or smoothed differently from what `stat.EMA` already does. It
  is documented as a recipe, with its three columns drawn as two lines and a
  histogram in a bottom track, and it earns a function the day a definition
  detail turns out to differ.

The rule is the one `stat`'s package comment already states — numbers in,
numbers out — sharpened into a test: a function exists when the obvious
composition gives the wrong numbers.

## Consequences

- A market chart can show its last price on the price axis, in the direction's
  colour, following the live candle.
- A crosshair can snap to the candle under the pointer's time and read its time
  and close off the axes.
- The first thing drawn outside a panel by something other than an axis is a
  tag, and it has a rule for what it covers.
- `stat` gains `RSI` and `VWAP`; the RSI panel is a bottom track with an
  `HBand` from 30 to 70, the MACD panel a bottom track of two lines and a bar.

## Not in scope

- **Other indicators** — stochastics, ATR, ADX, Ichimoku, Keltner. Each is a
  question for the rule in claim 4 when someone asks for it; most are
  compositions, and a few have a Wilder average inside, which `RSI`'s
  implementation will have made a helper by then.
- **Drawing tools** — trend lines, Fibonacci retracements, a reader's own
  annotations. They are state a reader creates, which the host owns, drawn
  with the marks that exist.
- **A tag that is dragged** — an alert level set by moving its tag. Input is
  the host's ([ADR 0045](0045-linked-views.md)); the tag is where it would be
  drawn.
- **Snapping along Y.** `NearestAlong` takes either axis, but a crosshair's
  `SnapX` is the case asked for; a `SnapY` waits for a chart that wants it.

## Revisit if

- Tags are wanted on an axis of a coord other than Cartesian.
- An indicator composition in the docs turns out to disagree with a platform's
  numbers, which is claim 4's rule telling us it earns a function.

## Order of work

1. `render` draws `AxisTag`s from `geom.Tagger` layers and from overlays, with
   the tick labels they cover hidden and tags on one axis pushed apart.
2. `geom.LastValue`.
3. `interact.Index.NearestAlong`; `Crosshair.SnapX` and `Crosshair.Tags`.
4. `stat.RSI` and `stat.VWAP` with their Append pairs, tested against published
   reference values; the MACD recipe in `docs/chart-types.md`.
5. `examples/market` gains an RSI track and a last-price tag, and `-live` a
   snapping crosshair driven by a scripted pointer.

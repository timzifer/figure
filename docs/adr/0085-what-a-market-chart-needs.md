# 0085 — A market chart is a recipe over marks that exist, and what it lacks is a calendar, a candle and statistics that only look back

**Status:** Proposed · **Date:** 2026-09-28 · **Implemented in part:** the easy wins in the order of work, with `examples/market`

## Context

[docs/chart-types.md](../chart-types.md) has listed "Candlestick / OHLC" as a
recipe since `geom.Rect` shipped: *`Rect` for open..close, a rule for
low..high, colour by sign*. [ADR 0083](0083-an-axis-break-is-marked-or-not-drawn.md)
named the other half of a price chart in its own comment — `scale.TimeFold`
is for "the nights of a trading week" — and then deferred it: *"Revisit if
someone needs a business-day time axis, which is claim 6's machinery plus a
calendar."* Someone does.

Nobody had assembled the parts. No example drew a candle, no test put a
volume under a price or a profile beside one, and the question a user of a
trading screen asks first — *can this library draw my chart* — had no answer
better than "probably, from pieces". A trading screen is also a harder reader
than most: it reads the right-hand edge of a live chart and acts on it, so a
line that moves when the next row arrives is not a smoothing choice but a
wrong answer.

This record is a catalogue, in the manner of
[ADR 0058](0058-what-3d-is-for.md): what a market chart is made of, which
parts exist, which are a recipe, which are missing, and the order the missing
ones are worth building in. Each mechanism it ranks gets a record of its own
before it gets code — the rule in [the index](README.md) for decisions opened
after v1.0 — except the handful at the bottom of the order of work that are
additive, follow a pattern already in the package, and open no seam. Those
landed with this record.

## Decision

**A market chart is composed from marks that exist. What it cannot compose is
a calendar, a candle as one mark, and indicators that look only backwards; and
there is still no Stat interface.** Six parts, each with what exists, what is
a recipe and what is missing.

### 1. The candle

**Exists.** `geom.Rect` for the body — `Y("open")`, `Y2("close")` — which is
the reason its doc comment names a candle; `geom.ErrorBar` with `Caps(false)`
for the wick, `Y("low")`, `Y2("high")`. Both take `BarWidth`, both take
`ColorBy`, and an ErrorBar is drawn under its Rect by being added first.
`examples/market` draws it.

**The recipe's cost.** The colour is a column the caller computes, because a
`data.Table` has no derived columns and a mark cannot read two columns to
decide a third. `stat.Candle.Up` makes the computation one call, not zero.

**Missing.**

- **A candle as one mark.** Four value columns — open, high, low, close — do
  not fit an X/Y/Y2 vocabulary, which is why the recipe needs two layers and a
  direction column. A `geom.Candle(src, X, Open, High, Low, Close)` mark would
  read the four, decide the direction itself, and be one legend entry rather
  than two. It is a new mark and a new dialect word, so it is a record of its
  own; the recipe is the proof that it composes.
- **The OHLC bar** — a rule with a tick left at the open and right at the
  close. Not a recipe today: no mark draws a tick off one side.
- **Hollow up, filled down** — the monochrome convention, which is
  [ADR 0069](0069-hatching-as-the-third-redundant-channel.md)'s question about
  redundant channels asked of a fill.
- **A data-driven rule.** `geom.Segment` takes four literals; the wick borrows
  ErrorBar because nothing else draws a rule per row. The lollipop in
  chart-types.md is the same gap.

### 2. The trading calendar

**Exists.** `scale.TimeFold(spans...)` leaves each span out of a time axis and
marks it as a fold, and `figure.SpansWhere` / `SpansFunc` read the spans from a
table. A weekend, a holiday or a night is a span, so a trading calendar is
already *drawable*: `examples/market` folds twelve weekends it computes itself.

**Missing.** The calendar. Every span is passed explicitly, so an intraday
chart over a year is five hundred folds the caller generates, a market's
holidays are a list the caller keeps, and a session that closes at 16:00 in
New York is arithmetic in a time zone the caller gets right. A calendar is a
value — sessions per weekday, holidays, a location — that produces the spans
for an interval; it is claim 6 of ADR 0083 plus exactly that, as the record
said. Folds on a log axis stay out of scope, for 0083's reason.

This is rank 1 below. A candle chart with the weekends drawn empty is a chart
of the calendar, not of the market.

### 3. The combined panels

**Exists, and it is the part that needed nothing.** `Plot.Track`
([ADR 0031](0031-tracks.md)) is a band at an edge that shares the scale object
of the axis it runs along:

- **Volume under the price** is `Track(figure.Bottom, …, TrackScale(scale.Linear()))`
  holding a `geom.Bar`. It shares the time axis, so a zoom on a live chart is
  one zoom. Further indicator panels — an RSI, a MACD — are further bottom
  tracks.
- **The volume profile, or an order book, beside the price** is
  `Track(figure.Right, …)`, which shares the price axis. The buys and the sells
  at each price are a horizontal `Bar` grouped by side and stacked across,
  from `stat.BinWeighted` over one interval.

**Missing.**

- **A bar on its side.** `geom.Orient` was read by the tree and by the
  gradient, not by `geom.Bar`, so the profile's bars were rects with a
  hand-stacked `X`/`X2`. — **Landed since**, as
  [ADR 0087](0087-an-orientation-is-the-encoding-read-a-quarter-turn-round.md):
  the profile is now a stacked horizontal `Bar`.
- **Tracks the reader resizes.** A track's size is the caller's; dragging the
  boundary between price and volume is a host concern today.
- **`Grid` panels that are live and linked.** `grid.go` says tracks are the
  answer, and for one symbol they are. For several symbols compared, a track
  per facet is refused (`ErrTrackWithFacet`); see *Revisit if*.

### 4. Indicators, and the interface for them

**There is no Stat interface, and this record does not add one.**
`stat/stat.go` says why — a pluggable stat would have to know which axis it
decides, how the layer treats a missing value and what the theme wants — and
[ADR 0028](0028-distribution-stats.md) and
[ADR 0029](0029-extension-model.md) record it. An indicator fits that rule
better than most summaries: it is numbers in and numbers out, a column of
prices becoming a column of values at the same rows. So the three ways in are
the three that already exist:

1. **A function in `stat`**, with its Append pair, that the caller calls and
   draws the result of with an ordinary mark — a `Line`, an `Area` between two
   columns, a `Bar` in a track. This is the way for everything below.
2. **A geom of the caller's own**, registered with `geom.Register`, that calls
   such a function in its `Train` — for an indicator that must retrain with the
   data, as `geom.Trend` retrains its fit.
3. **An overlay**, `render.Overlay`, for what is drawn over the chart rather
   than from its data: a crosshair, a measured range.

**Exists.** `MovingAverage` — centred, so wrong for this reader, for the
reason in the context — and `SavitzkyGolay`, `Loess`, `StdDev`, the SPC
limits, `LTTB` and `MinMax` decimation.

**Missing, and landed with this record.** The trailing forms:
`TrailingMean`, `EMA`, `RollingStdDev` (population, as a Bollinger band is
defined), `RollingMin` / `RollingMax` (a Donchian channel), and `Cumsum` (an
order book's depth). Each value is a function of its row and the rows before
it, so appending a row changes nothing already drawn; that property has a test.
A Bollinger band is `TrailingMean ± k · RollingStdDev`, drawn as an `Area`
between `Y` and `Y2` — three lines of the caller's, not a function.

**Missing, and landed with this record.** Resampling: `stat.OHLC` turns a
column of ticks into candles of a fixed width from an origin, and leaves an
interval nobody traded in without a candle rather than inventing one.

**Missing, and not landed.** RSI, MACD, VWAP, returns. Each is a composition of
the functions above — MACD is two EMAs and a third over their difference, VWAP
is two cumulative sums divided — and whether they earn names of their own is
a question for the record that ships the candle mark, when a user of the
dialect can say which ones a document needs to name.

### 5. Annotations

**Exists.** `HLine`, `HBand`, `Region`, `Note`, and `Text` with callouts
([ADR 0082](0082-a-label-fits-its-box-or-is-called-out.md)). A buy and a sell
are two `Scatter` layers.

**Landed with this record.** `ir.MarkerTriangleDown` — `"triangle-down"` in
the dialect, Vega-Lite's own name — appended to the marker set as its doc
comment requires, so a sell points the way a sell goes. It is not added to
`theme.DefaultSeriesMarkers`: the redundant-encoding ladder is a separate
decision and every golden file draws it.

**Missing.** A last-price label pinned to the axis at the right edge, which is
an `HLine` with a label in the gutter rather than in the panel — a small mark,
but it draws outside the panel, which no mark does today.

### 6. Interaction

**Exists.** `Live` with wheel zoom, pan, `View`, `Select`; the `Crosshair`,
`Tooltip`, `Highlight` and `Brush` overlays; `data.Stream` with a window.

**Missing.**

- **Zoom on time, autoscale on price.** A trading screen zooms the time axis
  and fits the price axis to the candles that remain visible; `Live` zooms
  what it is told and autoscales everything or nothing.
- **Updating the last row.** A live candle changes until its interval closes;
  `data.Stream` appends and never replaces.
- **A crosshair that snaps to the candle** rather than following the pointer
  between them.

## Ranking

By what a reader loses without it, as ADR 0058 ranks:

1. **The trading calendar.** Without it the chart is of the calendar.
2. **The candle mark.** Without it every chart is two layers, a direction
   column and two legend entries for one thing.
3. **Zoom on time, autoscale on price, and replacing the last row** — the two
   halves of a live chart.
4. **A bar on its side.** — **Landed**, as ADR 0087.
5. **Named indicators** (RSI, MACD, VWAP), the last-price label, the snapping
   crosshair, the OHLC bar.

## Order of work

1. **The easy wins.** — **Landed with this record.** `stat.TrailingMean`,
   `EMA`, `RollingStdDev`, `RollingMin`, `RollingMax`, `Cumsum`, `OHLC` and
   `BinWeighted`, each with its Append pair and allocation-free into a warm
   slice; `ir.MarkerTriangleDown`; and `examples/market`, which draws every
   part of section 3 from the recipes above, so that each gap in this record is
   a line of that example rather than a claim.
2. **A trading calendar**, as a record extending ADR 0083. — **Proposed** as
   [ADR 0086](0086-a-calendar-says-when-time-counts.md), and general rather
   than a market's: a market is one calendar among working weeks, rosters and
   terms.
3. **`geom.Candle`**, as a record: its columns, its dialect word, its legend,
   and whether the OHLC bar is an option on it or a mark beside it.
4. **Price autoscale and a replaceable last row**, as one record, because both
   are what a live market chart is and neither is useful alone.
5. The rest, as asked for.

## Consequences

- A market chart can be drawn today, and the example is the documentation of
  how.
- The indicator functions look only backwards and are therefore safe on the
  right-hand edge of a live chart; `MovingAverage` stays centred and says so.
- `stat` gains two types, `Candle` and `WeightedBucket`. Neither widens an
  existing type: `Bucket.Count` stays an integer count of observations.
- The marker set is one longer, at the end. A backend that draws markers
  through `ir.MarkerPath` draws the new one without change.

## Not in scope

- **A Stat interface, or a transform stage between data and layer.** For the
  reason `stat/stat.go` gives, which an indicator does not weaken.
- **Data feeds, brokers, order entry.** The library draws a chart of a
  market; it does not connect to one.
- **An indicator language** — Pine, or formulas in the dialect. A document
  holds the columns an indicator produced, not the program.
- **Point-and-figure, Renko, Kagi.** Each is a chart whose X is not time,
  built from bricks of price; each is a recipe over `Rect` on an ordinal X
  once the bricks are computed, and none has been asked for.

## Revisit if

- An indicator needs state across frames that a function of the column cannot
  carry — an adaptive average whose parameter is tuned live.
- Several symbols are compared on one screen, each with its volume, which is a
  track per facet and today is `ErrTrackWithFacet`.
- A float64 of nanoseconds loses the sub-microsecond resolution a tick chart
  of a fast market needs; `scale.Origin` is the answer for the axis, and
  `stat.OHLC`'s origin for the candles.

# 0089 — A value axis can fit what the other axis shows, and a stream can revise its last row

**Status:** Accepted, amended · **Date:** 2026-09-28 · **Implemented:** 2026-09-28 · **Ranked by:** [ADR 0085](0085-what-a-market-chart-needs.md), rank 3

## Context

[ADR 0085](0085-what-a-market-chart-needs.md) ranked two gaps third, together,
because each is half of what makes a market chart *live* and neither is useful
alone.

**Zoom on time, autoscale on price.** `Live.Wheel`, `ZoomTo` and `PanBy` move
every steerable axis of the panel, X and Y alike — that is the right default
for a scatter, where the reader zooms into a region of the plane. On a price
chart it is wrong twice over. Zooming into last week keeps the price axis of
the whole year, so a week that moved two percent is a flat line along the
middle of the panel; and wheeling over the chart zooms the price axis as well,
which no trader asked for. What a trading screen does is zoom and pan the time
axis alone, and fit the price axis to the candles still visible — every frame,
as the view moves.

The same fit is wanted without a pointer. A static chart of one month cut out
of a year's table — its time axis pinned to the month with `Zoomer.SetDomain`,
or a numeric axis fixed with `scale.Domain` — has a value axis trained on the
whole year today. A volume track under a zoomed price
chart is trained on the tallest bar of the year. Both are the same question:
the value axis should describe the rows the other axis shows.

**The candle that has not closed.** A live chart's last candle is not a row
that arrives once. It opens at the first tick of its period and changes on
every tick after — its close moves, its high and low widen — until the period
ends and the next candle opens. `data.Stream` appends and never replaces
(ADR 0016), so today a caller either appends a row per tick and draws a
hundred candles where there should be one, or keeps the open candle outside
the stream and draws it as a second layer that has to agree with the first.

## Decision

**A value scale can be told to fit the rows its panel's other axis shows;
render hands each layer that interval when it trains, and the layers that can
restrict their training do. A stream can replace its last row, and `stat` has
a resampler that says when a tick revises the open candle and when it opens the
next.** Seven claims.

### 1. Fitting is a property of the value scale

```go
p.X(scale.Time(...))
p.Y(scale.Linear(scale.FitView()))
p.Track(figure.Bottom, figure.TrackScale(scale.Linear(scale.Zero(), scale.FitView())))
```

`scale.FitView()` is a linear option, and `scale.LogFitView()` its log
counterpart, because a price axis is as often logarithmic as not. It declares
that the axis is trained on the rows the panel's *other* axis shows, and
nothing else about the scale changes: nicing, zero-forcing and breaks apply to
the fitted extent as they would to any other.

It is a scale option rather than a mode of `Live` for the reason the context
gives: a static chart with a fixed time domain wants the same fit, a track
wants it on its own scale while the panel above it does not, and a scale
option is also the one place the dialect already writes an axis's behaviour
down (claim 7). A `Live` has nothing to add but the zoom that makes the fit
change.

It is the vertical axis that fits the horizontal one. A value axis that is
horizontal — a bar chart lying down,
[ADR 0087](0087-an-orientation-is-the-encoding-read-a-quarter-turn-round.md) —
fitting a vertical one it is measured against is the same claim turned, and is
left until someone draws a zoomable horizontal chart.

### 2. The interval is the other axis's domain, when it has been decided

Render trains a panel's layers once. For a panel whose Y scale fits, the X
scale's domain is known *before* training exactly when something has pinned
it: a zoom or a pan, a caller's own `Zoomer.SetDomain`, a linear domain fixed
at construction with `scale.Domain`, or a view put back with `Live.SetView`. Then render passes
that interval to every layer as `Training.Within`, a field added to
`geom.Training` the way `Z` and `Dims` were — additively, and nil for every
panel that does not fit.

When the X axis is not pinned it is trained from the same rows, so every row is
in view and `Within` stays nil: the fitted axis is the ordinary one. That is
what keeps this a single training pass. There is no case in which the X domain
depends on the fitted Y domain, so nothing has to be trained twice.

Pinned needs asking, so the scales that implement `Zoomer` gain an optional
method, `Pinned() bool`, behind an interface of its own — `scale.Pinner` —
rather than a new method on `Zoomer`, which [ADR 0084](0084-what-v1-promises.md)
refuses on a published interface.

### 3. A layer restricts what it trains on, and a layer that cannot, does not

`Training.Within` is a request, not a filter render applies — render does not
know which of a layer's columns are positions. A layer that reads it trains its
value axis on:

- **the rows whose position is inside the interval**, for point marks —
  `Scatter`, `Text`;
- **the rows whose span overlaps it**, for marks with width — `Bar`, `Rect`,
  `ErrorBar`, `Candle` — so a candle half in view still fits;
- **the rows inside it and the value interpolated at each edge**, for connected
  marks — `Line`, `Area`, `Step` — because the stretch of a line between the
  last row inside and the first outside is drawn, and it crosses the edge at a
  value no row holds. A step holds its value to the edge instead of
  interpolating it.

It still trains its *position* axis on every row, so an axis that is not pinned
keeps its extent and one that is pinned ignores the training anyway.

A layer that does not read `Within` trains on all its rows, as it does today.
The fitted axis is then wider than the view, never narrower: the failure is a
chart that fits less tightly, not one that clips its data. The distribution
and relational marks are in that list and stay in it — a histogram's axis is
its counts, not a view of rows.

### 4. `Live` does not steer an axis that fits

`Wheel`, `ZoomTo` and `PanBy` leave a fitting axis alone and move the others.
Zooming a price chart then zooms time, about the pointer, and the next frame's
training fits the price axis to what remains; panning slides time and the price
axis follows. `Autoscale` releases the time axis and the fit returns to the
whole table.

A fitting axis is not pinned by any of them, so it never needs releasing: it is
derived every frame from the axis that is steered. A `View` records it as
unpinned and `SetView` leaves it so. A reader who wants to zoom the price axis
by hand is asking for an axis that does not fit — the host turns the option
off, which is a rebuild, not a gesture.

### 5. A stream can replace its last row

`Stream.ReplaceLast(vals...)` overwrites the most recent row with new values, in
the order `NewStream` named the columns, under the same lock as `Append`. It is
an error on an empty stream. It allocates nothing, and a `Window` is
unaffected — the row count does not change.

It is the one revision a stream admits, because it is the one a live source
makes: the newest row is the only one still being measured. Any row further
back is history, and a chart whose past changed between two frames is a chart
whose reader can no longer trust what they saw on the first. A producer that
needs to revise older rows owns a table, not a stream.

`Snapshot` is unchanged. A replaced row appears on the next snapshot, and
`Live.Draw`'s damage pass sees one candle that changed and repaints that, and
the axis if the fit moved it.

### 6. `stat.Resampler` says when a tick opens a candle

```go
r := stat.Resampler{Origin: scale.Nanos(open), Width: float64(time.Minute)}
for tick := range ticks {
	c, fresh := r.Add(scale.Nanos(tick.At), tick.Price, tick.Size)
	row := []float64{c.Start, c.End, c.Open, c.High, c.Low, c.Close, c.Volume}
	if fresh {
		stream.Append(row...)
	} else {
		stream.ReplaceLast(row...)
	}
}
```

`stat.OHLC` summarises a column it is given whole. A live source gives it one
tick at a time, and the question each tick asks is the one `OHLC` answers
internally — *is this the same period as the last one* — so `Resampler` is that
loop turned inside out: a small struct with a `Width` and an `Origin`, or
`Edges` from a calendar's `Opens` ([ADR 0086](0086-a-calendar-says-when-time-counts.md)),
whose `Add` returns the candle as it now stands and whether it is new. Its
candles are exactly `OHLC`'s over the same ticks; a test says so.

It is a struct with state rather than a function because the state is the
point — the open candle — and it has a `Reset`, as `stat.Contour` and
`stat.Lattice` do, so that a chart restarting its feed reuses it. `stat` stays
numbers in, numbers out; the stream is the caller's.

A tick older than the open candle's start is refused (`Add` reports it and
changes nothing): the candle it belongs to is history, by claim 5.

### 7. The dialect

A scale that fits is written `"fit": "view"`, figure's own, on the scale; a
document read back fits the same way. `Training.Within` is not written: it is
derived from the other axis's domain, which the document already holds when it
is fixed. The stream and the resampler are Go values the document never sees,
as a stream never was.

## Consequences

- A price chart zooms and pans on time and fits its price axis to the visible
  candles, with one option on the price scale; the volume track under it fits
  its own axis with the same option.
- A static chart of a month cut from a year fits its value axis to the month.
- A live chart's last candle is one row that changes, not a row per tick, and
  the frame that draws it repaints that candle.
- `geom.Training` gains `Within`; `scale` gains `FitView`, `LogFitView` and
  `Pinner`; `data.Stream` gains `ReplaceLast`; `stat` gains `Resampler`. All are
  additions.

## Not in scope

- **Following the newest data.** A trading screen's view slides right as
  candles arrive, when the reader has not scrolled away. The host knows when
  data arrives and where its end is, and `Live.SetView` or `PanBy` moves the
  view there; the library cannot tell "the reader is looking at the end" from
  "the reader is looking at a past that ends where the data did".
- **Easing the fit.** The price axis jumps to the new extent every frame. An
  animated axis is [ADR 0045](0045-linked-views.md)-adjacent host work and a
  transition, not a scale.
- **Fitting under a facet with free axes.** Each panel fits to its own X, which
  falls out of claim 2 per panel; a shared Y fitting several X views at once is
  a union of intervals, and waits for someone who wants it.
- **Revising any row but the last.** Claim 5.

## Revisit if

- A horizontal value axis should fit a zoomed vertical one (claim 1).
- A mark outside claim 3's list turns out to be drawn zoomed often enough that
  its loose fit is noticed.

## Order of work

Every step is built.

1. `scale.FitView`, `scale.LogFitView`, `scale.Fitter`, `scale.Pinner` and
   `"fit": "view"` — `scale/fit.go`.
2. `geom.Training.Within`, passed by render; `Line`, `Area`, `Step`, `Scatter`,
   `Text`, `Bar`, `Rect`, `ErrorBar` and `Candle` read it through three
   helpers in `geom/fit.go` — points, spans, and connected paths with their edge
   values — and a stack trains its totals in the view. A boxplot, which does
   not read it, is tested to fit loosely rather than clip.
3. `Live.Wheel`, `ZoomTo` and `PanBy` skip a fitting axis, and a `View` does not
   record one; tested end to end: a wheel zooms X only, the next frame's Y fits
   it, a pan slides both, `Autoscale` returns to every row.
4. `data.Stream.ReplaceLast`, tested unbounded, filling, full and wrapped, and
   under `-race` beside `Append`.
5. `stat.Resampler`, tested to produce `stat.OHLC`'s and `stat.OHLCAt`'s
   candles over the same ticks.
6. `examples/market -live`: a seeded tick feed through a resampler into a
   stream, one-minute candles and a volume track, both on fitted axes, wheeled
   into the last half hour at the midpoint.

## Amendment — what building it decided

- **A fitting axis is reset every frame.** A trained domain only ever widens,
  and render keeps a panel's scales across frames, so an axis fitted to last
  frame's view kept it after the view moved on — the first end-to-end test
  found a wheel that zoomed X and left Y where it was. Render now releases
  every fitting axis whose X is pinned once, before any layer trains, and the
  fit is derived from scratch. Once, for all panels, because panels that share
  an axis share its scale. An axis that does not fit keeps the old behaviour;
  whether an unpinned axis should forget last frame's rows too is a question
  about every live chart, not this record's — and
  [ADR 0090](0090-an-axis-describes-the-frame-it-is-drawn-in.md) asks it, and
  answers it for every axis; `refit` is folded into its `retrain` pass.
- **Following the newest data is shown, not built.** The example's host keeps
  the width the reader zoomed to and moves its right-hand edge to the open
  candle with `Zoomer.SetDomain` each frame — one line, as *Not in scope*
  predicted. Without it the view stays where the reader left it while the
  stream's window moves on, and the chart ends showing one candle.
- **A stack fits a padded view.** A stacked area's totals are trained on the
  rows within one spacing of the view, and a stacked bar's within half a
  bar — looser than a line's edge interpolation, and in the direction a fit
  may err.
- **Only a vertical value axis narrows.** A horizontal bar and a horizontal
  error bar measure along X, so a fit of Y leaves them trained on every row, as
  claim 1 said of horizontal value axes.
- **A floating bar still trains its value only.** `Bar` trained its Y and not
  its Y2 before this record, and the fit keeps that rather than change what a
  floating bar's axis covers as a side effect.

# 0086 — A trading calendar is a value that produces folds, and a tick that lands in one moves to the next open

**Status:** Proposed · **Date:** 2026-09-28 · **Revisits:** [ADR 0083](0083-an-axis-break-is-marked-or-not-drawn.md)'s business-day deferral · **Ranked by:** [ADR 0085](0085-what-a-market-chart-needs.md), rank 1

## Context

[ADR 0083](0083-an-axis-break-is-marked-or-not-drawn.md) gave a time axis
folds and left the calendar out of scope: *"Recurring breaks as a rule — every
weekend, market hours, a business-day axis. That is a calendar rather than an
interval list, and a caller can already pass the spans a calendar produces as
folds."* Its *Revisit if* named the case, and
[ADR 0085](0085-what-a-market-chart-needs.md) ranked it first among the gaps of
a market chart, because a candle chart with the weekends drawn empty is a
chart of the calendar rather than of the market.

`examples/market` shows what the caller pays today. Twelve weeks of daily
candles need twelve weekend spans, computed in a loop. That is cheap. What is
not cheap is everything the loop leaves out:

- **Nights.** An intraday chart over a month is twenty-two sessions and
  twenty-one nights and four weekends, and the night before a Monday is part
  of the weekend. The caller merges them or 0083's merge does, but the caller
  has to produce them first, one per day, in the market's own time zone.
- **Daylight saving.** A session that opens at 09:30 in New York opens at
  13:30 UTC for half the year and 14:30 for the other half. A loop that adds
  24 hours to the last open is an hour wrong from the second Sunday in March,
  and the chart does not say so: the fold is merely an hour off, and an hour
  of real trading disappears into it.
- **Holidays and short days.** A market is closed on days no rule predicts
  and closes early on others. Both are a list, and the list is the market's.
- **The labels.** This is the part a loop cannot fix. 0083 §5 walks one tick
  step through each kept piece and forbids a tick strictly inside a cut. On an
  intraday axis with the nights folded, every day tick is at midnight, and
  midnight is inside a fold. The axis is correct and has no dates on it.

So the calendar is two things: a value that knows when a market is open, and
a rule about where a tick goes when the step it was chosen at lands in the
market's closed time.

## Decision

**A calendar is a value in `scale` that says when a market is open and
produces the folds for an interval. The time axis takes those folds exactly as
it takes any others. A tick at a calendar step of a day or longer that lands
in a fold moves to the fold's far edge, which is the next open.** Six claims.

### 1. A calendar is a value, not a scale

```go
cal := scale.Sessions(nyc, 9*time.Hour+30*time.Minute, 16*time.Hour).
	Holidays(closed...).
	Early(date(2026, 11, 27), 13*time.Hour)

p.X(scale.Time(scale.In(nyc), scale.TimeFold(cal.Folds(from, to)...)))
```

`scale.Calendar` holds a location, the sessions of each weekday as offsets
from local midnight, a set of closed dates and a set of dates whose sessions
differ. Its constructors are the three cases that cover almost every chart:

- `scale.Weekdays(loc)` — open all day Monday to Friday. The daily chart.
- `scale.Sessions(loc, open, close)` — one session a weekday. The intraday
  chart of one exchange.
- `scale.Week(loc, [7][]Session)` — anything else: a session that spans
  midnight, a lunch break, a market open on Sunday evening.

It is not a new scale kind, and that is the decision the rest follows from. A
calendar changes nothing about how a time axis maps, inverts or ticks that a
fold does not already change, and 0083 has already said everything about how
a fold does that. A `scale.Business` kind would be a second time scale whose
only difference from the first is where its folds come from — a second
`Train`, a second `Ticks`, a second dialect word — to carry a list the first
already takes.

### 2. The calendar produces folds for an interval, and the caller names it

`cal.Folds(from, to)` is the complement of the sessions over `[from, to)`:
every closed stretch, sorted and merged, with a weekend and the nights on
either side of it one fold. It is computed day by day in the calendar's
location with `time.Date`, which is what makes it right across a daylight
saving change: an open is a local clock time, not a multiple of 24 hours from
the last one.

The interval is the caller's, rather than the scale producing folds for
whatever domain it trains to, for 0083's reason — *the document a chart is
written into holds the spans, not the query*. A folded axis in the JSON
dialect is a list of `"folds"`, readable by any consumer, and a document that
held a calendar instead would need every consumer to agree on what a calendar
is, which exchange's holidays it has, and in which year. So the dialect is
unchanged: a calendar is a Go value that writes folds, and the folds are what
is serialised.

The cost is a live chart whose data outgrows the interval it was folded for.
A caller streaming ticks folds far enough ahead — a calendar is cheap to ask
for a year — or refolds when the stream crosses the end; see *Revisit if*.

### 3. Holidays are the caller's list

`Holidays(dates...)` closes a date all day; `Early(date, close)` and
`Late(date, open)` shorten one session; `Special(date, sessions...)` replaces
one day entirely. The library ships no exchange's holidays. They change every
year, they are announced by the exchange rather than derived from a rule, and
a table in a charting library would be out of date by the release after the
one that shipped it — which, for a chart that silently draws a closed day as
a flat candle-less gap, is worse than no table. A caller already has the list,
because the same list decides when their data arrives.

### 4. A tick that lands in a fold moves to the next open

This amends 0083 §5 for one case. When the time step chosen for the kept
length is a calendar unit of a day or longer — a day, a week, a month — and a
tick of that step lands strictly inside a fold, it moves to the fold's upper
edge and keeps its label. A day tick at midnight becomes a tick at 09:30
labelled with the day; a month tick on a Saturday becomes a tick at Monday's
open labelled with the month.

It is only for calendar steps, because only they name a day rather than an
instant. A tick labelled 12:00 that moved to 09:30 would be a label that lies;
a tick labelled *3 Mar* at 09:30 on 3 March is true. It never moves a tick
past the next tick of the same step, and two ticks that land in the same fold
become one, the later label winning — a weekend that swallowed a month
boundary is labelled with the new month. 0083's rule that no tick stands
strictly inside a cut still holds; this chooses where the tick goes instead
of dropping it.

It is a property of folded time axes rather than of calendars, so it applies
to folds from `SpansWhere` too: a machine log with the nights folded gets day
labels for the same reason.

### 5. A calendar is also where a day's candles start

`stat.OHLC` buckets by a fixed width from an origin, which is right for a
minute and for an hour and wrong for a day across a daylight saving change,
where the day is 23 or 25 hours long. `cal.Opens(from, to)` returns the
session opens as a column of edges, and `stat.OHLCAt(ts, ps, vs, edges)`
buckets between consecutive edges — a candle per session, however long the
session was. `stat` stays numbers in and numbers out: it takes edges, not a
calendar.

### 6. Where it is refused

- **On an axis that is not a time axis.** A calendar is dates.
- **On a log time axis**, for 0083's reason that no coord yet cuts one.
- **With a session longer than a day or ending before it opens** outside
  `Week`, which is where an overnight session is spelled explicitly.

## Consequences

- A daily chart with the weekends and holidays out is one line, and an
  intraday chart with the nights out is the same line with a session.
- An intraday axis with folded nights has dates on it. Without claim 4 it
  cannot.
- The dialect is unchanged, and a document drawn from a calendar reads in any
  consumer that reads folds.
- A year of intraday folds is about three hundred cuts after merging, which is
  0083's `BenchmarkFolded1k` territory: a binary search per row and no
  allocation.
- The slash marks on the axis line are one per fold. On a year of intraday
  data they are drawn once where closer together than their own size, which
  0083 §6 already does.

## Not in scope

- **Exchange holiday tables.** Claim 3.
- **An index axis** — every session the same width regardless of its length,
  as some trading platforms draw one. That is an ordinal axis over session
  numbers with time labels, which a caller can build on `scale.Ordinal` now;
  it gives up the property that an hour is an hour, which is the property a
  shortened session is visible by.
- **Session colouring** — pre-market and after-hours drawn as tinted bands.
  That is `VBand` over the spans a second calendar produces.
- **A calendar in the dialect.** Claim 2.

## Revisit if

- A live chart needs folds that follow the stream without the caller
  refolding — which is a scale that derives cuts from its trained domain, and
  would reopen 0083's reason for keeping cuts fixed at construction.
- Someone needs the index axis above with time-aware ticks, which is a scale
  kind of its own.

## Order of work

1. `scale.Calendar`, its three constructors, `Holidays`, `Early`, `Late`,
   `Special`, `Folds` and `Opens`, with tests across both daylight saving
   changes of one location and a session spanning midnight.
2. Claim 4 in the time scale's tick walk, with a golden of an intraday week.
3. `stat.OHLCAt` and its Append pair.
4. `examples/market` folds from a calendar instead of its own loop, and gains
   an intraday panel.

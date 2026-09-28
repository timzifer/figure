# 0086 — A calendar says when time counts; the axis takes the rest as folds and starts its weeks where the calendar does

**Status:** Accepted, amended · **Date:** 2026-09-28 · **Implemented:** 2026-09-28 · **Revisits:** [ADR 0083](0083-an-axis-break-is-marked-or-not-drawn.md)'s business-day deferral · **Ranked by:** [ADR 0085](0085-what-a-market-chart-needs.md), rank 1

## Context

[ADR 0083](0083-an-axis-break-is-marked-or-not-drawn.md) gave a time axis
folds and left the calendar out of scope: *"Recurring breaks as a rule — every
weekend, market hours, a business-day axis. That is a calendar rather than an
interval list."* [ADR 0085](0085-what-a-market-chart-needs.md) ranked it first
among the gaps of a market chart. But a market is only the loudest customer.
The same question — *which stretches of time count, and which are left out* —
is asked by:

- a factory running Monday to Saturday in three shifts, with a maintenance
  window every second Sunday;
- an office in Tel Aviv or Riyadh, whose working week is Sunday to Thursday;
- a hospital ward that never closes, and a school that closes for its terms;
- a support desk on 09:00–17:00 in three time zones;
- a machine log whose idle nights are read out of a table
  (`figure.SpansWhere`, 0083 §6);
- and an exchange, with sessions, holidays and early closes.

A design that fits the exchange and asks the others to pretend to be one fails
most of them. "Weekdays" means Monday to Friday in one country and Monday to
Saturday in another; a week starts on Monday under ISO 8601 and in most of
Europe, on Sunday in the United States, Canada, Japan and Israel, and on
Saturday in much of the Middle East. And some calendars are not a rule at all:
a shift plan is a roster, a school year is a list of terms, a holiday falls on
a date only another calendar system can compute.

Two things in the code make it more than a list of spans.

- **The labels.** 0083 §5 walks one tick step through each kept piece and
  forbids a tick strictly inside a cut. With the nights folded, every day tick
  is at midnight, and midnight is inside a fold: the axis is correct and has
  no dates on it.
- **The week.** A week tick today is `Truncate(7 * 24h)` in the scale's
  zone (`scale/time.go`, `walk`). Go truncates from its zero time, 1 January
  of year 1, which was a Monday — so every weekly axis in the library starts
  its weeks on a Monday, in every locale, by an accident of the standard
  library rather than by a decision. It is the ISO answer, and it cannot be
  changed: an American or an Israeli week has no way to start on a Sunday.
  A calendar makes the week step the common case — twelve weeks of daily data
  is exactly where `pick` chooses it — so the accident becomes visible.

## Decision

**A calendar is an interface with one question — when is this day open — and
it is composed from rules about days that the library ships. Its closed
time becomes the axis's folds, its week start becomes the axis's weeks, and a
tick that lands in a fold moves to where time next counts.** Seven claims.

### 1. A calendar is an interface, and anyone can write one

```go
// A Calendar says when time counts.
type Calendar interface {
	// Location is the zone the calendar's days and hours are in.
	Location() *time.Location
	// WeekStart is the day a week begins on.
	WeekStart() time.Weekday
	// Open appends the stretches of the day that count, as offsets from its
	// local midnight, ascending and inside [0, 24h]. day is that midnight,
	// in Location. A closed day appends nothing.
	Open(dst []Span, day time.Time) []Span
}

type Span struct{ From, To time.Duration }
```

It lives in `scale`, beside `TimeSpan`, because what it produces is a time
axis's folds and a time axis's weeks, and `scale` already owns both.

The question is *per day* because that is the one granularity every calendar
answers without translation: a roster says who works on the 14th, a term list
says whether the 14th is in term, a lunar holiday is a date, a rule says what
Tuesdays look like. A calendar asked instead for "every closed span between two
instants" would make each implementation redo the walk over days, the time
zone arithmetic and the merging — which are the parts that are easy to get
wrong and identical everywhere. The day is given as its local midnight in the
calendar's own location, so an implementation never adds 24 hours to anything.

A stretch that runs past midnight — a night shift from 22:00 to 06:00 — is two
answers, 22:00–24:00 on one day and 00:00–06:00 on the next, and the walk
joins them. That keeps the contract to one day without making overnight
calendars second-class.

`Open` takes a destination slice for the reason every Append form in `stat`
does: a live chart refolds, and refolding must not allocate per day.

### 2. A calendar is composed from rules about days, and the library ships the rules

The calendars a caller writes are almost all one of two shapes: *which days
are open*, with the same hours on each, or *which hours each weekday has*. So
those are the two constructors, and the question of which days is a function
the caller composes rather than a builder the library anticipates:

```go
// A rule says whether a date is open. day is its local midnight.
type DayRule func(day time.Time) bool

// Open on the days the rule accepts, for the given stretches of each — all
// day when none are given.
func Workweek(loc *time.Location, open DayRule, hours ...Span) Calendar

// Each weekday named with its own stretches; a weekday not named is closed.
func Days(loc *time.Location, days ...Day) Calendar
type Day struct {
	Weekday time.Weekday
	Hours   []Span
}

// Anything else: the whole per-day question, as a function.
func CalendarFunc(loc *time.Location, open func(dst []Span, day time.Time) []Span) Calendar
```

The rules are ordinary functions, and the library ships the ones every
calendar is made of:

- `scale.Weekdays(days ...time.Weekday) DayRule` — open on the named weekdays.
  There is no rule called "weekdays" without arguments, because there is no
  global answer: Monday to Friday, Monday to Saturday and Sunday to Thursday
  are all spelled out, and the code says which one the chart means.
- `scale.Dates(dates ...time.Time) DayRule` — the named dates, by year, month
  and day in the calendar's location; the time of day of an argument is
  ignored.
- `scale.EveryNth(n int, from time.Time) DayRule` — every n-th day counted from
  a date: a fortnightly maintenance Sunday is
  `scale.And(scale.Weekdays(time.Sunday), scale.EveryNth(14, first))`, a
  four-on-four-off shift is `scale.EveryNth` over an eight-day cycle.
- `scale.Between(from, to time.Time) DayRule` — the dates of a term, a season,
  a campaign.
- `scale.And`, `scale.Or`, `scale.Not` — the rest.

Two wrappers change any calendar, the caller's included, rather than being
methods of one type:

- `scale.Closed(cal, rule)` closes the days the rule accepts — the holidays.
- `scale.Special(cal, rule, spans...)` gives the days the rule accepts those
  stretches instead — an early close, a late start, a Saturday worked.

```go
// A plant: Monday to Saturday, 06:00 to 22:00, closed on public holidays and
// every other Saturday, short on the day before a holiday.
cal := scale.Workweek(berlin,
	scale.And(scale.Weekdays(time.Monday, time.Tuesday, time.Wednesday,
		time.Thursday, time.Friday, time.Saturday),
		scale.Not(scale.And(scale.Weekdays(time.Saturday), scale.EveryNth(14, firstSaturday)))),
	scale.Span{From: 6 * time.Hour, To: 22 * time.Hour})
cal = scale.Closed(cal, scale.Dates(holidays...))
cal = scale.Special(cal, scale.Dates(eves...), scale.Span{From: 6 * time.Hour, To: 14 * time.Hour})

// An exchange: the same two constructors, nothing of its own.
nyse := scale.Closed(scale.Workweek(newYork,
	scale.Weekdays(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday),
	scale.Span{From: 9*time.Hour + 30*time.Minute, To: 16 * time.Hour}),
	scale.Dates(closed...))
```

**The rule is asked about a day, not about an instant.** A `func(time.Time)
bool` over instants is the more obvious signature and cannot work: the axis
needs the exact edges of every closed stretch, and a predicate over instants
only answers yes or no at the instants it is asked about, so the edges would
have to be found by sampling or bisection — approximate, and a search per
edge. A rule about a day is exact because a day is discrete, and the hours of
the day are then data, `[]Span`, rather than a second question.

A span with `From > To` is an overnight stretch written the way a person says
it, 22:00 to 06:00, and is split at midnight into the two answers claim 1
requires. `scale.Always(loc)` is `Workweek(loc, nil)` — nil accepts every day —
the ward that never closes, and the base a caller closes dates on.

**The week start** is `scale.StartWeek(cal, d)`, a third wrapper. Every
constructor defaults to Monday, ISO 8601's, and it is a default of the
calendar rather than of a locale: `scale.Locale` names days, it does not decide
which one a week begins on, and deriving one from the other would make a German
chart and a German chart about an American company's weeks disagree for no
reason the code shows.

Every calendar is immutable and every wrapper returns a new one, so one base
week is shared by several charts and specialised per chart.

No holiday table ships with the library, for any country or any exchange.
Holidays are announced rather than derived, differ by region inside one
country, and change; a table here would be wrong by the release after the one
that shipped it, and a closed day drawn open is a gap nobody notices. A
caller already has the list, because the same list decides when their data
arrives. A calendar whose holidays *are* a rule — a lunar or a solar-hijri
one — is an implementation of the interface, which is what it is for.

### 3. The calendar produces folds, and the caller names the interval

```go
p.X(scale.Time(scale.TimeCalendar(cal), scale.TimeFold(scale.Folds(cal, from, to)...)))
```

`scale.Folds(cal, from, to)` walks the days of `[from, to)` in the calendar's
location, asks `Open` for each, and returns the complement as `TimeSpan`s,
sorted and merged — a Friday night, a weekend and a Monday morning are one
fold. It is a function over the interface rather than a method, so every
calendar, written here or by a caller, gets the same walk.

The interval is the caller's, not derived from whatever domain the scale
trains to, for 0083's reason: *the document holds the spans, not the query*.
In the JSON dialect a folded axis is a list of `"folds"` any consumer reads,
and a document that held a calendar would need every consumer to agree on
what the calendar is — which a caller-written implementation makes impossible.
So the calendar is a Go value that writes folds, and the folds are what is
serialised. A live chart folds ahead or refolds when its stream passes the end;
see *Revisit if*.

### 4. A week starts where the calendar says, and nowhere else by accident

`scale.WeekStart(d)` is a new time-scale option: week ticks are aligned to
local midnight on that weekday, by walking from the weekday rather than by
truncating a duration. Without it the default is Monday — what every weekly
axis already draws, now as a decision rather than as Go's zero time — so no
golden file changes.

`scale.TimeCalendar(cal)` applies a calendar to a time axis's own conventions:
it sets the week start from `cal.WeekStart()` and the location from
`cal.Location()` unless `scale.In` names another. It does not fold: folding is
claim 3's, with its interval. The dialect writes `"weekStart"` on a time scale
when it is not Monday.

### 5. A tick that lands in a fold moves to where time next counts

This amends 0083 §5 for one case. When the step chosen for the kept length is
a calendar unit of a day or longer — a day, a week, a month, a year — and a
tick of that step lands strictly inside a fold, it moves to the fold's upper
edge and is labelled for where it now stands, in the step's own layout. A day
tick at midnight becomes a tick at the day's first open hour labelled with the
day; a Sunday-start week tick on a closed Sunday becomes a tick at Monday's
open labelled with Monday's date; a month tick on a closed first of the month
moves to the first open day and still reads as the month.

It is only for calendar units, because only they name a day rather than an
instant: a tick labelled 12:00 moved to 09:30 would have to say 09:30, and then
it is a tick nobody asked for. A moved tick never passes the next tick of its
own step; two that land in one fold become one, the later winning — a closed
week that swallowed a month boundary is labelled with the new month. 0083's
rule that no tick stands strictly inside a cut still holds; this chooses where
the tick goes instead of dropping it. It is a property of folded time axes,
not of calendars, so a machine log folded from `SpansWhere` gets day labels
too.

### 6. A calendar is also where a period starts

`stat.OHLC` buckets by a fixed width from an origin, which is right for a
minute and an hour and wrong for a day across a daylight saving change, where
the day is 23 or 25 hours long — and wrong for a shift, which is whatever the
roster says. `scale.Opens(cal, from, to)` returns the start of every open
stretch as a column of edges, and `stat.OHLCAt(ts, ps, vs, edges)` buckets
between consecutive edges: one candle per session, one bar per shift, however
long each was. `stat` stays numbers in, numbers out: it takes edges, not a
calendar.

### 7. Where it is refused

- **On an axis that is not a time axis.** A calendar is days.
- **On a log time axis**, for 0083's reason that no coord yet cuts one.
- **An `Open` answer out of order, overlapping, or outside `[0, 24h]`** is an
  error from `Folds`, naming the date: a calendar that cannot say when a day is
  open is a bug in the calendar, and silently merging its answer would draw
  someone's bug as their data.

## Consequences

- A chart of Monday-to-Saturday production, of a Sunday-to-Thursday office and
  of an exchange with its nights folded are the same two lines with different
  arguments, and none of them is a special case of another.
- A calendar the library could not anticipate — a roster, a term list, a
  calendar computed in another calendar system — is a `DayRule` if its days are
  all alike, a `CalendarFunc` if they are not, and a type with three methods if
  it wants to be a type.
- A weekly axis can start its weeks on any day. The default stays Monday, so
  nothing drawn today moves.
- An intraday axis with folded nights has dates on it.
- The dialect gains `"weekStart"` on a time scale and nothing else; a document
  drawn from a calendar reads in any consumer that reads folds.
- A year of intraday folds is about three hundred cuts after merging, which is
  0083's `BenchmarkFolded1k` territory, and the walk that produces them is one
  `Open` call per day into a reused slice.

## Not in scope

- **Holiday tables**, for any country or exchange. Claim 2.
- **A calendar in the dialect.** Claim 3.
- **Tick labels in another calendar system** — months that begin at the
  Hijri or Hebrew month rather than the Gregorian one. The folds of such a
  calendar are expressible now, through the interface; its *labels* are a
  second tick walk and a locale question, and a record of their own.
- **An index axis** — every open stretch the same width regardless of its
  length. That is an ordinal axis over period numbers, buildable on
  `scale.Ordinal` today; it gives up the property that an hour is an hour,
  which is what makes a short day visible.
- **Colouring the calendar** — pre-market, overtime, weekends drawn as tinted
  bands rather than folded. That is `VBand` over `Folds` of a second calendar.

## Revisit if

- A live chart needs folds that follow its stream without the caller refolding
  — a scale that derives cuts from its trained domain, which reopens 0083's
  reason for fixing cuts at construction.
- Someone draws a calendar system other than the Gregorian one and needs its
  months on the axis.

## Order of work

Every step is built.

1. `scale.Calendar`, `scale.Span`, `scale.Folds`, `scale.Opens`; the
   constructors `Workweek`, `Days`, `CalendarFunc` and `Always`; the rules
   `Weekdays`, `Dates`, `EveryNth`, `Between`, `And`, `Or`, `Not`; and the
   wrappers `Closed`, `Special` and `StartWeek` — `scale/calendar.go`, tested
   across both of New York's daylight saving changes, an overnight shift, a
   Sunday-to-Thursday week in Jerusalem and a caller-written roster.
2. `scale.WeekStart` and `scale.TimeCalendar`, with the week-tick walk aligned
   to a weekday by date instead of truncated from Go's zero time; the default
   is still Monday and no golden file moved.
3. Claim 5 in the time scale's tick walk, and `calendar-fortnight`, a golden of
   two weeks of sessions with nights and weekend folded.
4. `stat.OHLCAt` and its Append pair, allocation-free into a warm slice.
5. `examples/market` takes its trading days and its folds from one calendar,
   Monday to Friday with 3 July closed.

## Amendment — what building it decided

- **A span is wall clock.** `Span` offsets are read through `time.Date` as the
  hour, minute and second of the day, not added to midnight as elapsed time.
  That is what keeps an 09:30 open at 09:30 on the morning the clocks changed
  at 02:00, and why `Span.To` of 24h is the next midnight on a 23- or 25-hour
  day alike.
- **`Dates` reads a date as written.** Its year, month and day in whatever
  location the argument was built in, rather than converted to the calendar's:
  `time.Date(2026, 12, 25, 12, 0, 0, 0, elsewhere)` is Christmas in every
  calendar. Converting would make a holiday list built in UTC close the wrong
  day in any zone west of Greenwich.
- **A closed day is closed from midnight to midnight.** `Closed` cannot know
  which morning belongs to which evening's shift, so a night shift that began
  on the evening before a holiday stops at midnight; a calendar that must
  finish its shifts is a `CalendarFunc`. `Special` replaces a whole day the
  same way, and its stretches stop at midnight.
- **`Opens` returns the start of every uninterrupted open stretch**, merged
  across midnight. A night shift is one period, and so is a week open all day
  from Monday to Friday — which is why the market example, daily and open all
  day, keeps `stat.OHLC` with a width of a day and uses the calendar for its
  folds only. `OHLCAt` is for calendars with hours.
- **A tick moves out of a fold and is dropped from a break.** Claim 5 is about
  folds; a break is a stretch the reader was told is missing, and a tick on
  its far edge would be a date nobody chose standing against the break's
  mark. `axis-break-time`'s golden says so by not changing.
- **The day step is rarer than the record assumed.** The tick search chooses a
  step for the *kept* length, and on a fortnight of 6½-hour sessions at an
  ordinary width that is twelve hours, whose labels carry the date already.
  Claim 5 is what an axis narrow or long enough to choose a day step needs —
  without it such an axis has no ticks at all — and the test asks for exactly
  that case.

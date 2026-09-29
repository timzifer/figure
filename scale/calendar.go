package scale

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Calendar says when time counts: which stretches of each day are open, in
// which zone, and which day a week begins on. Its closed time is what a time
// axis folds out ([Folds]), and its week start is where the axis starts its
// weeks ([TimeCalendar]).
//
// It is an interface because the calendars people chart do not share a rule:
// a working week, a shift roster, a school's terms, an exchange's sessions, a
// calendar computed in another calendar system. The library ships the parts
// the rule-shaped ones are made of — [Workweek], [Days], the [DayRule]s and the
// wrappers [Closed], [Special] and [StartWeek] — and anything else is a type
// with these three methods, or a function handed to [CalendarFunc].
//
// The question is asked a day at a time because that is the one granularity
// every calendar answers without translation, and because it leaves the walk
// over days, the zone arithmetic and the merging — the parts that are easy to
// get wrong and identical everywhere — to [Folds]. See
// docs/adr/0086-a-calendar-says-when-time-counts.md.
type Calendar interface {
	// Location is the zone the calendar's days and hours are in.
	Location() *time.Location
	// WeekStart is the day a week begins on.
	WeekStart() time.Weekday
	// Open appends the stretches of the day that count to dst and returns
	// it. day is the day's local midnight in Location. The stretches are
	// wall-clock offsets from that midnight, ascending, not overlapping and
	// inside [0, 24h]; a closed day appends nothing.
	Open(dst []Span, day time.Time) []Span
}

// Span is a stretch of one day, as wall-clock offsets from its midnight:
// 9h30m to 16h is half past nine to four, on the day the clocks change as on
// any other.
//
// Given to [Workweek], [Days] or [Special], a span whose From is after its To
// runs overnight — 22h to 6h is a night shift — and is split at midnight into
// the evening of one day and the morning of the next. Out of a [Calendar]'s
// Open, every span is within its own day.
type Span struct{ From, To time.Duration }

// DayRule says whether a day is open. day is its local midnight in the
// calendar's location. A nil rule accepts every day.
type DayRule func(day time.Time) bool

// Day is one weekday's stretches, for [Days].
type Day struct {
	Weekday time.Weekday
	Hours   []Span
}

// ErrCalendar is returned by [Folds] and [Opens] when a calendar answers a
// day with stretches that are out of order, overlap, or leave the day.
var ErrCalendar = errors.New("figure/scale: calendar answered a day with invalid stretches")

// day is how long a wall-clock day is. A day the clocks change on is 23 or 25
// hours of elapsed time and still 24 of wall clock, which is the unit [Span]
// is written in.
const day = 24 * time.Hour

// Workweek is the calendar open on the days rule accepts, for the given
// stretches of each — all day when none are given.
//
// There is no default set of days, because there is no global one: a working
// week is Monday to Friday in one country, Monday to Saturday in another and
// Sunday to Thursday in a third, and the code says which one the chart means:
//
//	scale.Workweek(berlin, scale.Weekdays(time.Monday, time.Tuesday,
//		time.Wednesday, time.Thursday, time.Friday),
//		scale.Span{From: 9 * time.Hour, To: 17 * time.Hour})
//
// A span from 22h to 6h is a night shift: it belongs to the day the rule
// accepted, and runs into the next whether or not the next is open itself.
// The week starts on Monday; see [StartWeek].
func Workweek(loc *time.Location, rule DayRule, hours ...Span) Calendar {
	if len(hours) == 0 {
		hours = []Span{{0, day}}
	}
	h := slices.Clone(hours)
	return &ruled{loc: zone(loc), week: time.Monday, open: func(d time.Time) []Span {
		if rule == nil || rule(d) {
			return h
		}
		return nil
	}}
}

// Always is the calendar that never closes: a ward, a data centre, and the
// base a caller closes dates on with [Closed].
func Always(loc *time.Location) Calendar { return Workweek(loc, nil) }

// Days is the calendar whose weekdays each have their own stretches — a short
// Saturday, a lunch break on weekdays, an evening session on Thursdays. A
// weekday not named is closed; one named twice has the stretches of both.
func Days(loc *time.Location, days ...Day) Calendar {
	var by [7][]Span
	for _, d := range days {
		by[d.Weekday] = append(by[d.Weekday], d.Hours...)
	}
	return &ruled{loc: zone(loc), week: time.Monday, open: func(d time.Time) []Span {
		return by[d.Weekday()]
	}}
}

// CalendarFunc is the calendar whose days open answers, for a calendar no
// rule describes: a roster, a term list, a calendar computed in another
// calendar system. open appends the stretches of one day to dst exactly as
// [Calendar.Open] does, and is given the same local midnight. The week starts
// on Monday; see [StartWeek].
func CalendarFunc(loc *time.Location, open func(dst []Span, day time.Time) []Span) Calendar {
	return &funcCalendar{loc: zone(loc), week: time.Monday, open: open}
}

// Closed is cal with the days rule accepts closed: the holidays.
//
// A closed day is closed from midnight to midnight, so a night shift that began
// the evening before stops at midnight. A calendar whose shifts must finish is
// a [CalendarFunc], which knows which shift a morning belongs to.
func Closed(cal Calendar, rule DayRule) Calendar {
	return &wrapped{Calendar: cal, week: cal.WeekStart(), open: func(dst []Span, d time.Time) []Span {
		if rule == nil || rule(d) {
			return dst
		}
		return cal.Open(dst, d)
	}}
}

// Special is cal with the days rule accepts given these stretches instead:
// an early close, a late start, a Saturday worked. No stretches closes the
// days, as [Closed] does. The stretches replace the whole day and stay inside
// it: a span running past midnight stops there.
func Special(cal Calendar, rule DayRule, hours ...Span) Calendar {
	h := slices.Clone(hours)
	return &wrapped{Calendar: cal, week: cal.WeekStart(), open: func(dst []Span, d time.Time) []Span {
		if rule == nil || rule(d) {
			return appendDay(dst, h, nil)
		}
		return cal.Open(dst, d)
	}}
}

// StartWeek is cal with its weeks beginning on d.
//
// Every constructor here starts the week on Monday, ISO 8601's, and that is a
// default of the calendar rather than of a [Locale]: a locale names the days,
// it does not decide which one a week begins on, and deriving one from the
// other would make two charts in one language disagree about an American
// company's weeks for no reason the code shows.
func StartWeek(cal Calendar, d time.Weekday) Calendar {
	return &wrapped{Calendar: cal, week: d, open: cal.Open}
}

// Weekdays accepts the named weekdays. There is no rule for "the weekdays"
// without naming them: which ones they are is the question.
func Weekdays(days ...time.Weekday) DayRule {
	var in [7]bool
	for _, d := range days {
		in[d] = true
	}
	return func(d time.Time) bool { return in[d.Weekday()] }
}

// Dates accepts the named dates: their year, month and day as written, in
// whatever location each was built in. The time of day is ignored, so
// time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC) is Christmas in any calendar.
func Dates(dates ...time.Time) DayRule {
	in := make(map[int64]bool, len(dates))
	for _, t := range dates {
		in[civil(t)] = true
	}
	return func(d time.Time) bool { return in[civil(d)] }
}

// Between accepts every date from from to to, both included: a term, a
// season, a campaign. Dates are read as [Dates] reads them.
func Between(from, to time.Time) DayRule {
	a, z := civil(from), civil(to)
	if z < a {
		a, z = z, a
	}
	return func(d time.Time) bool { c := civil(d); return c >= a && c <= z }
}

// EveryNth accepts every n-th date counted from from, which it accepts: a
// fortnightly Sunday is And(Weekdays(time.Sunday), EveryNth(14, aSunday)),
// the first of a four-on-four-off shift's working days is EveryNth(8, first).
// Days are counted as dates, not as 24-hour intervals, so a daylight saving
// change does not shift the cycle. n < 1 accepts nothing.
func EveryNth(n int, from time.Time) DayRule {
	a := civil(from)
	return func(d time.Time) bool {
		if n < 1 {
			return false
		}
		k := (civil(d) - a) % int64(n)
		return k == 0
	}
}

// And accepts a day every rule accepts.
func And(rules ...DayRule) DayRule {
	return func(d time.Time) bool {
		for _, r := range rules {
			if r != nil && !r(d) {
				return false
			}
		}
		return true
	}
}

// Or accepts a day any rule accepts.
func Or(rules ...DayRule) DayRule {
	return func(d time.Time) bool {
		for _, r := range rules {
			if r == nil || r(d) {
				return true
			}
		}
		return false
	}
}

// Not accepts a day rule does not.
func Not(rule DayRule) DayRule {
	return func(d time.Time) bool { return rule != nil && !rule(d) }
}

// Folds returns the time cal is closed between from and to, as the folds of a
// time axis: every closed stretch, sorted, with a weekend and the nights on
// either side of it one span.
//
//	p.X(scale.Time(scale.TimeCalendar(cal), scale.TimeFold(folds...)))
//
// The days are walked in the calendar's location and each stretch is placed
// on its day's wall clock, which is what keeps an 09:30 open at 09:30 on the
// day the clocks change. The interval is the caller's rather than the axis's
// domain for ADR 0083's reason: a document holds the folds, not the calendar
// that produced them.
func Folds(cal Calendar, from, to time.Time) ([]TimeSpan, error) {
	var out []TimeSpan
	cursor := from
	err := walkOpen(cal, from, to, func(a, z time.Time) {
		if a.After(cursor) {
			out = append(out, TimeSpan{From: cursor, To: a})
		}
		if z.After(cursor) {
			cursor = z
		}
	})
	if err != nil {
		return nil, err
	}
	if to.After(cursor) {
		out = append(out, TimeSpan{From: cursor, To: to})
	}
	return out, nil
}

// Opens returns the instant each open stretch of cal between from and to
// begins: the edges [github.com/timzifer/figure/stat.OHLCAt] buckets between,
// so that a period is a session or a shift however long it was.
//
// A stretch is the whole of an uninterrupted open time, so a night shift that
// runs over midnight is one stretch, and so is a working week that is open all
// day from Monday to Friday. A caller who wants days on such a calendar wants
// a fixed width, which [github.com/timzifer/figure/stat.OHLC] already is.
func Opens(cal Calendar, from, to time.Time) ([]time.Time, error) {
	var out []time.Time
	var last time.Time
	started := false
	err := walkOpen(cal, from, to, func(a, z time.Time) {
		if !started || a.After(last) {
			out = append(out, a)
		}
		started = true
		if z.After(last) {
			last = z
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// walkOpen calls fn with every open stretch of cal that overlaps [from, to),
// clipped to it, in order. It asks for the day before from as well, so that a
// stretch a calendar reports as the morning after an overnight shift is found
// even when the interval starts inside it.
func walkOpen(cal Calendar, from, to time.Time, fn func(a, z time.Time)) error {
	if !to.After(from) {
		return nil
	}
	loc := zone(cal.Location())
	f, t := from.In(loc), to.In(loc)
	y, m, d := f.Date()
	var buf []Span
	for cur := time.Date(y, m, d-1, 0, 0, 0, 0, loc); cur.Before(t); {
		cy, cm, cd := cur.Date()
		buf = cal.Open(buf[:0], cur)
		if err := validSpans(buf, cur); err != nil {
			return err
		}
		for _, s := range buf {
			a, z := wall(cy, cm, cd, s.From, loc), wall(cy, cm, cd, s.To, loc)
			if a.Before(from) {
				a = from
			}
			if z.After(to) {
				z = to
			}
			if z.After(a) {
				fn(a, z)
			}
		}
		cur = time.Date(cy, cm, cd+1, 0, 0, 0, 0, loc)
	}
	return nil
}

func validSpans(ss []Span, d time.Time) error {
	prev := time.Duration(-1)
	for _, s := range ss {
		if s.From < 0 || s.To > day || s.From >= s.To || s.From < prev {
			return fmt.Errorf("%w: %s: %v", ErrCalendar, d.Format(time.DateOnly), ss)
		}
		prev = s.To
	}
	return nil
}

// wall is the instant off after midnight on the given date by the wall clock:
// time.Date normalises the hour, so 24h is the next midnight and 9h30m is half
// past nine whatever the clocks did at two.
func wall(y int, m time.Month, d int, off time.Duration, loc *time.Location) time.Time {
	return time.Date(y, m, d, 0, 0, 0, int(off), loc)
}

// ruled is a calendar whose days a function answers with overnight spans
// allowed: the evening half of a span belongs to its day and the morning half
// to the next, so a day's answer is its own evenings and the day before's
// mornings.
type ruled struct {
	loc  *time.Location
	week time.Weekday
	open func(day time.Time) []Span
}

func (r *ruled) Location() *time.Location { return r.loc }
func (r *ruled) WeekStart() time.Weekday  { return r.week }
func (r *ruled) Open(dst []Span, d time.Time) []Span {
	y, m, dd := d.Date()
	prev := r.open(time.Date(y, m, dd-1, 0, 0, 0, 0, r.loc))
	return appendDay(dst, r.open(d), prev)
}

// appendDay appends one day's stretches from the spans given for it and the
// overnight tails of the spans given for the day before, sorted and merged.
func appendDay(dst []Span, today, yesterday []Span) []Span {
	start := len(dst)
	for _, s := range yesterday {
		if s.From > s.To && s.To > 0 {
			dst = append(dst, Span{0, s.To})
		}
	}
	for _, s := range today {
		switch {
		case s.From > s.To:
			dst = append(dst, Span{s.From, day})
		case s.From < s.To:
			dst = append(dst, Span{max(s.From, 0), min(s.To, day)})
		}
	}
	ds := dst[start:]
	slices.SortFunc(ds, func(a, b Span) int { return int(a.From - b.From) })
	out := ds[:0]
	for _, s := range ds {
		if s.From >= s.To {
			continue
		}
		if n := len(out); n > 0 && s.From <= out[n-1].To {
			out[n-1].To = max(out[n-1].To, s.To)
			continue
		}
		out = append(out, s)
	}
	return dst[:start+len(out)]
}

type funcCalendar struct {
	loc  *time.Location
	week time.Weekday
	open func(dst []Span, day time.Time) []Span
}

func (c *funcCalendar) Location() *time.Location { return c.loc }
func (c *funcCalendar) WeekStart() time.Weekday  { return c.week }
func (c *funcCalendar) Open(dst []Span, d time.Time) []Span {
	if c.open == nil {
		return dst
	}
	return c.open(dst, d)
}

// wrapped is a calendar with its days or its week start changed.
type wrapped struct {
	Calendar
	week time.Weekday
	open func(dst []Span, day time.Time) []Span
}

func (w *wrapped) WeekStart() time.Weekday             { return w.week }
func (w *wrapped) Open(dst []Span, d time.Time) []Span { return w.open(dst, d) }

// civil is a date's day number: its year, month and day as written, counted
// from the epoch. Two times on the same date in different zones share it.
func civil(t time.Time) int64 {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400
}

func zone(loc *time.Location) *time.Location {
	if loc == nil {
		return time.UTC
	}
	return loc
}

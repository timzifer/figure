package scale_test

import (
	"errors"
	"testing"
	"time"

	"github.com/timzifer/figure/scale"
)

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("no time zone data for %s: %v", name, err)
	}
	return loc
}

func hm(h, m int) time.Duration { return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute }

var monFri = scale.Weekdays(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)

func folds(t *testing.T, cal scale.Calendar, from, to time.Time) []scale.TimeSpan {
	t.Helper()
	fs, err := scale.Folds(cal, from, to)
	if err != nil {
		t.Fatalf("Folds: %v", err)
	}
	return fs
}

// A working week open all day folds exactly its weekends, from Saturday's
// midnight to Monday's.
func TestAWorkweekFoldsItsWeekends(t *testing.T) {
	cal := scale.Workweek(time.UTC, monFri)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // a Monday
	got := folds(t, cal, from, from.AddDate(0, 0, 14))
	want := []scale.TimeSpan{
		{From: time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)},
		{From: time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if !got[i].From.Equal(want[i].From) || !got[i].To.Equal(want[i].To) {
			t.Errorf("fold %d: %v – %v, want %v – %v", i, got[i].From, got[i].To, want[i].From, want[i].To)
		}
	}
}

// The week a chart means is the one it names: Sunday to Thursday folds Friday
// and Saturday, and nothing else.
func TestASundayToThursdayWeekFoldsFridayAndSaturday(t *testing.T) {
	loc := mustZone(t, "Asia/Jerusalem")
	cal := scale.Workweek(loc, scale.Weekdays(time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday))
	from := time.Date(2026, 6, 7, 0, 0, 0, 0, loc) // a Sunday
	got := folds(t, cal, from, from.AddDate(0, 0, 7))
	if len(got) != 1 || got[0].From.Weekday() != time.Friday || got[0].To.Weekday() != time.Sunday ||
		got[0].To.Sub(got[0].From) != 48*time.Hour {
		t.Errorf("got %v, want Friday 00:00 to Sunday 00:00", got)
	}
}

// Hours are read off the wall clock, so an 09:30 open is at 09:30 on the
// Monday after each daylight saving change, and the nights between are 17½
// hours of wall clock however much elapsed time that is.
func TestSessionsKeepTheirWallClockAcrossBothChanges(t *testing.T) {
	loc := mustZone(t, "America/New_York")
	cal := scale.Workweek(loc, monFri, scale.Span{From: hm(9, 30), To: hm(16, 0)})
	for _, from := range []time.Time{
		time.Date(2026, 3, 5, 0, 0, 0, 0, loc),   // the week clocks go forward, 8 March
		time.Date(2026, 10, 29, 0, 0, 0, 0, loc), // and back, 1 November
	} {
		opens, err := scale.Opens(cal, from, from.AddDate(0, 0, 8))
		if err != nil {
			t.Fatal(err)
		}
		if len(opens) != 6 {
			t.Fatalf("%d opens from %v, want six sessions", len(opens), from)
		}
		for _, o := range opens {
			if h, m, _ := o.In(loc).Clock(); h != 9 || m != 30 {
				t.Errorf("a session opens at %v, want 09:30 local", o.In(loc))
			}
		}
		for _, f := range folds(t, cal, from, from.AddDate(0, 0, 8)) {
			if f.From.Equal(from) {
				continue // the interval opens on a closed morning
			}
			if h, m, _ := f.From.In(loc).Clock(); h != 16 || m != 0 {
				t.Errorf("a fold starts at %v, want 16:00 local", f.From.In(loc))
			}
		}
	}
}

// A night shift runs over midnight: it is one open stretch, not two, and the
// morning it runs into is open even on a day the rule closes.
func TestANightShiftRunsOverMidnight(t *testing.T) {
	cal := scale.Workweek(time.UTC, scale.Weekdays(time.Friday), scale.Span{From: 22 * time.Hour, To: 6 * time.Hour})
	from := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC) // a Friday
	to := from.AddDate(0, 0, 2)
	opens, err := scale.Opens(cal, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(opens) != 1 || !opens[0].Equal(from.Add(22*time.Hour)) {
		t.Fatalf("opens %v, want one at Friday 22:00", opens)
	}
	got := folds(t, cal, from, to)
	if len(got) != 2 || !got[0].To.Equal(from.Add(22*time.Hour)) || !got[1].From.Equal(from.Add(30*time.Hour)) {
		t.Errorf("folds %v, want everything but Friday 22:00 to Saturday 06:00", got)
	}

	// The interval may start inside the shift; the morning half is still found.
	late := from.Add(26 * time.Hour)
	got = folds(t, cal, late, to)
	if len(got) != 1 || !got[0].From.Equal(from.Add(30*time.Hour)) {
		t.Errorf("from 02:00 on Saturday: folds %v, want one from 06:00", got)
	}
}

// Each weekday its own hours: a short Saturday and a lunch break.
func TestDaysGiveEachWeekdayItsOwnHours(t *testing.T) {
	cal := scale.Days(time.UTC,
		scale.Day{Weekday: time.Monday, Hours: []scale.Span{{From: hm(8, 0), To: hm(12, 0)}, {From: hm(13, 0), To: hm(17, 0)}}},
		scale.Day{Weekday: time.Saturday, Hours: []scale.Span{{From: hm(8, 0), To: hm(12, 0)}}})
	mon := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	got := cal.Open(nil, mon)
	if len(got) != 2 || got[1].From != hm(13, 0) {
		t.Errorf("Monday %v, want two stretches with a lunch break", got)
	}
	if got := cal.Open(nil, mon.AddDate(0, 0, 1)); len(got) != 0 {
		t.Errorf("Tuesday %v, want closed", got)
	}
	if got := cal.Open(nil, mon.AddDate(0, 0, 5)); len(got) != 1 || got[0].To != hm(12, 0) {
		t.Errorf("Saturday %v, want the morning", got)
	}
}

// The rules compose, and the wrappers apply to any calendar.
func TestRulesAndWrappersCompose(t *testing.T) {
	first := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC) // a Saturday
	christmas := time.Date(2026, 12, 25, 12, 0, 0, 0, time.FixedZone("elsewhere", 5*3600))
	rule := scale.And(
		scale.Weekdays(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday),
		scale.Not(scale.And(scale.Weekdays(time.Saturday), scale.EveryNth(14, first))))
	cal := scale.Closed(scale.Workweek(time.UTC, rule), scale.Dates(christmas))
	cal = scale.Special(cal, scale.Dates(time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC)), scale.Span{From: 6 * time.Hour, To: 14 * time.Hour})
	cal = scale.StartWeek(cal, time.Sunday)

	open := func(y int, m time.Month, d int) []scale.Span {
		return cal.Open(nil, time.Date(y, m, d, 0, 0, 0, 0, time.UTC))
	}
	if len(open(2026, 6, 6)) != 0 {
		t.Error("the first fortnightly Saturday is open")
	}
	if len(open(2026, 6, 13)) != 1 {
		t.Error("the Saturday between is closed")
	}
	if len(open(2026, 6, 20)) != 0 {
		t.Error("the next fortnightly Saturday is open")
	}
	if len(open(2026, 12, 25)) != 0 {
		t.Error("Christmas, named in another zone, is open")
	}
	if s := open(2026, 12, 24); len(s) != 1 || s[0].To != 14*time.Hour {
		t.Errorf("Christmas Eve %v, want the short day", s)
	}
	if cal.WeekStart() != time.Sunday || cal.Location() != time.UTC {
		t.Errorf("week start %v, location %v", cal.WeekStart(), cal.Location())
	}
	if !scale.Between(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC))(time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)) {
		t.Error("Between leaves out its last date")
	}
	if !scale.Or(scale.Weekdays(time.Monday), scale.Weekdays(time.Sunday))(first.AddDate(0, 0, 1)) {
		t.Error("Or refuses a Sunday")
	}
}

// A calendar the library could not anticipate is a function: here, a roster
// that is open on the days it lists.
func TestACallersCalendarIsAFunction(t *testing.T) {
	roster := map[string]scale.Span{
		"2026-06-02": {From: 6 * time.Hour, To: 14 * time.Hour},
		"2026-06-04": {From: 14 * time.Hour, To: 22 * time.Hour},
	}
	cal := scale.CalendarFunc(time.UTC, func(dst []scale.Span, d time.Time) []scale.Span {
		if s, ok := roster[d.Format(time.DateOnly)]; ok {
			return append(dst, s)
		}
		return dst
	})
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	opens, err := scale.Opens(cal, from, from.AddDate(0, 0, 7))
	if err != nil {
		t.Fatal(err)
	}
	if len(opens) != 2 || opens[1].Hour() != 14 {
		t.Errorf("opens %v, want the two rostered shifts", opens)
	}
}

// A calendar that cannot say when a day is open is a bug in the calendar, and
// it is reported rather than drawn.
func TestAnInvalidAnswerIsAnError(t *testing.T) {
	bad := scale.CalendarFunc(time.UTC, func(dst []scale.Span, _ time.Time) []scale.Span {
		return append(dst, scale.Span{From: 10 * time.Hour, To: 9 * time.Hour})
	})
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if _, err := scale.Folds(bad, from, from.AddDate(0, 0, 1)); !errors.Is(err, scale.ErrCalendar) {
		t.Errorf("err = %v, want ErrCalendar", err)
	}
}

func TestAlwaysNeverFolds(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if got := folds(t, scale.Always(time.UTC), from, from.AddDate(0, 1, 0)); len(got) != 0 {
		t.Errorf("folds %v, want none", got)
	}
}

// weekTicks trains a time axis over ten weeks and asks for about ten ticks,
// which is where the tick search chooses a week.
func weekTicks(t *testing.T, s scale.Scale, loc *time.Location) []time.Time {
	t.Helper()
	from := time.Date(2026, 2, 18, 12, 0, 0, 0, loc)
	s.Train(scale.ValueOf(s, from), scale.ValueOf(s, from.AddDate(0, 0, 70)))
	s.SetRange(0, 1000)
	var out []time.Time
	for _, tk := range s.Ticks(scale.TickRequest{Want: 11}) {
		out = append(out, scale.InstantOf(s, tk.Value).In(loc))
	}
	if len(out) < 5 {
		t.Fatalf("%d ticks, want a weekly axis", len(out))
	}
	return out
}

func TestWeeksStartOnMondayUnlessToldOtherwise(t *testing.T) {
	for _, tk := range weekTicks(t, scale.Time(), time.UTC) {
		if tk.Weekday() != time.Monday || tk.Hour() != 0 {
			t.Errorf("a default week tick at %v, want Monday midnight", tk)
		}
	}
	for _, tk := range weekTicks(t, scale.Time(scale.WeekStart(time.Sunday)), time.UTC) {
		if tk.Weekday() != time.Sunday {
			t.Errorf("a Sunday-start week tick at %v", tk)
		}
	}
}

// A calendar's week start and zone reach the axis, and an explicit option
// wins over the calendar whichever order they are given in.
func TestTimeCalendarGivesTheAxisItsWeekAndZone(t *testing.T) {
	loc := mustZone(t, "Asia/Jerusalem")
	cal := scale.StartWeek(scale.Workweek(loc, nil), time.Sunday)
	for _, tk := range weekTicks(t, scale.Time(scale.TimeCalendar(cal)), loc) {
		if tk.Weekday() != time.Sunday || tk.Hour() != 0 {
			t.Errorf("a calendar week tick at %v, want Sunday midnight in Jerusalem", tk)
		}
	}
	for _, opts := range [][]scale.TimeOption{
		{scale.WeekStart(time.Saturday), scale.TimeCalendar(cal)},
		{scale.TimeCalendar(cal), scale.WeekStart(time.Saturday)},
	} {
		for _, tk := range weekTicks(t, scale.Time(opts...), loc) {
			if tk.Weekday() != time.Saturday {
				t.Errorf("an explicit Saturday start lost to the calendar: %v", tk)
			}
		}
	}
}

// Weeks are walked by date, so a week tick stays at local midnight across a
// daylight saving change instead of drifting to 23:00 or 01:00.
func TestAWeekTickStaysAtMidnightAcrossAClockChange(t *testing.T) {
	loc := mustZone(t, "Europe/Berlin")
	for _, tk := range weekTicks(t, scale.Time(scale.In(loc)), loc) {
		if tk.Hour() != 0 || tk.Weekday() != time.Monday {
			t.Errorf("a week tick at %v, want Monday midnight local", tk)
		}
	}
}

func TestAWeekStartIsWrittenDown(t *testing.T) {
	s := scale.Time(scale.WeekStart(time.Sunday))
	d := s.(scale.Describer).Describe()
	if d.WeekStart != "sunday" {
		t.Fatalf("Describe wrote week start %q", d.WeekStart)
	}
	back, err := scale.FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	if again := back.(scale.Describer).Describe(); again.WeekStart != "sunday" {
		t.Errorf("read back as %q", again.WeekStart)
	}
	if d := scale.Time().(scale.Describer).Describe(); d.WeekStart != "" {
		t.Errorf("a Monday week is written as %q, want it left out", d.WeekStart)
	}
}

// With the nights folded every midnight is inside a fold, and a day tick
// moves to the morning's open with that morning's date, instead of the axis
// having no dates at all.
func TestADayTickInAFoldMovesToTheNextOpen(t *testing.T) {
	cal := scale.Workweek(time.UTC, monFri, scale.Span{From: hm(9, 30), To: hm(16, 0)})
	from := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC) // Monday's open
	to := time.Date(2026, 6, 12, 16, 0, 0, 0, time.UTC)  // the next Friday's close
	fs, err := scale.Folds(cal, from, to)
	if err != nil {
		t.Fatal(err)
	}
	s := scale.Time(scale.TimeCalendar(cal), scale.TimeFold(fs...))
	s.Train(scale.ValueOf(s, from), scale.ValueOf(s, to))
	s.SetRange(0, 1000)
	s.(scale.Breaker).SetBreakGap(8, 4)

	// Four ticks over the 65 open hours is where the search chooses a day.
	ticks := s.Ticks(scale.TickRequest{Want: 4})
	if len(ticks) < 4 {
		t.Fatalf("%d ticks on a folded fortnight: %v", len(ticks), ticks)
	}
	seen := map[string]bool{}
	for _, tk := range ticks {
		at := scale.InstantOf(s, tk.Value).UTC()
		if h, m, _ := at.Clock(); h != 9 || m != 30 {
			t.Errorf("a day tick at %v, want a morning open", at)
		}
		if at.Weekday() == time.Saturday || at.Weekday() == time.Sunday {
			t.Errorf("a tick on a closed %v", at.Weekday())
		}
		if want := at.Format("Jan 2"); tk.Label != want {
			t.Errorf("a tick at %v is labelled %q, want %q — the date it stands on", at, tk.Label, want)
		}
		if seen[tk.Label] {
			t.Errorf("two ticks labelled %q", tk.Label)
		}
		seen[tk.Label] = true
	}
}

// A value read off an axis carries the precision a value between ticks needs:
// two decimals more than a linear axis's ticks, and the time of day on a time
// axis when the instant is not a midnight.
func TestAValueLabelIsFinerThanATickLabel(t *testing.T) {
	l := scale.Linear()
	l.Train(0, 100)
	l.SetRange(0, 400)
	if got := scale.ValueLabelOf(l, 94.37); got != "94.37" {
		t.Errorf("a value between whole-number ticks reads %q, want 94.37", got)
	}
	ts := scale.Time()
	day := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	ts.Train(scale.ValueOf(ts, day), scale.ValueOf(ts, day.AddDate(0, 0, 10)))
	if got := scale.ValueLabelOf(ts, scale.ValueOf(ts, day)); got != "Jun 3" {
		t.Errorf("a midnight reads %q, want the date", got)
	}
	if got := scale.ValueLabelOf(ts, scale.ValueOf(ts, day.Add(14*time.Hour+5*time.Minute))); got != "Jun 3 14:05" {
		t.Errorf("an afternoon reads %q, want the date and the time", got)
	}
}

// A value that needs more decimals than two beyond the ticks is rounded to
// two beyond them, not written out in full.
func TestAValueLabelStopsTwoDecimalsPastTheTicks(t *testing.T) {
	l := scale.Linear(scale.Nice())
	l.Train(89.98, 98.1)
	l.SetRange(300, 0)
	if got := scale.ValueLabelOf(l, 92.4028); got != "92.40" {
		t.Errorf("92.4028 on an axis ticked in twos reads %q, want 92.40", got)
	}
}

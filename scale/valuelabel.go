package scale

import (
	"math"
	"time"
)

// ValueLabelOf writes v the way s would write a value read off the axis at an
// arbitrary point — a price tag, a crosshair's readout — rather than at a tick.
//
// A tick label has the precision of the tick step, which is right for a tick
// and wrong for a value between two of them: a last price of 94.37 on an axis
// ticked every whole unit is not "94". So a linear axis writes the value with
// as many decimals as it needs, up to two more than its ticks, and a time axis
// writes the date and, when
// the instant is not a midnight, the time of day. An axis given a Go formatter
// or a layout is written with it, as its ticks are. Any other scale falls back
// to [LabelOf]. See docs/adr/0092-what-a-trading-screen-reads-off-its-edges.md.
func ValueLabelOf(s Scale, v float64) string {
	switch x := s.(type) {
	case *linear:
		return x.valueLabel(v)
	case *timeScale:
		return x.valueLabel(v)
	}
	return LabelOf(s, v)
}

func (l *linear) valueLabel(v float64) string {
	if l.format != nil {
		return l.format(v)
	}
	// As few decimals as the value needs, from the ticks' own to two more: 7.25
	// on an axis ticked in halves reads 7.25, not 7.250, and 94.37 on one ticked
	// in whole units reads 94.37, not 94.
	auto := autoFor(l.step())
	if auto.mode == 'f' {
		for k, most := auto.decimals, auto.decimals+2; k <= most; k++ {
			auto.decimals = k
			m := math.Pow(10, float64(k))
			if math.Abs(math.Round(v*m)-v*m) < 1e-6*math.Max(1, math.Abs(v*m)) {
				break
			}
		}
	}
	return l.numFormat.label(v, auto, l.loc)
}

// valueUnits are the layouts a value off a time axis is written in: a date
// alone for a midnight, and a date with a time otherwise.
var (
	valueDay     = timeUnit{every: 24 * time.Hour, layout: "Jan 2"}
	valueInstant = timeUnit{every: time.Minute, layout: "Jan 2 15:04"}
)

func (s *timeScale) valueLabel(v float64) string {
	t := s.Instant(v).In(s.loc)
	u := valueInstant
	if h, m, sec := t.Clock(); h == 0 && m == 0 && sec == 0 {
		u = valueDay
	}
	return s.label(t, u)
}

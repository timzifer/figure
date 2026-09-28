package scale

// Retrainer is implemented by a scale whose trained range can be forgotten,
// so that the next training describes only what it is shown.
//
// Render calls it once per scale at the start of every render, before any
// layer trains: a trained domain only widens, and a plot keeps its scales
// between renders, so without it an axis keeps every row it has ever been
// shown — a windowed stream's time axis kept the history the window had
// dropped. Within a render [Scale.Train] still accumulates, which is what lets
// several layers share an axis.
//
// A pinned domain is not trained and is not forgotten, so a zoom, a pan or a
// fixed domain survives. A scale that discovers a set of names — an ordinal
// axis, a qualitative or named colour scale — does not implement it: a
// category's slot and a series' colour are identity, and rediscovering them
// every frame would move them. See
// docs/adr/0090-an-axis-describes-the-frame-it-is-drawn-in.md.
type Retrainer interface {
	// Retrain forgets the trained range, keeping everything else.
	Retrain()
}

// Retrain forgets s's trained range if s can forget one, and does nothing
// otherwise.
func Retrain(s any) {
	if r, ok := s.(Retrainer); ok {
		r.Retrain()
	}
}

// HighWater keeps a linear axis's extent across renders, the way every axis
// kept it before [Retrainer]: an axis that shows the most this has ever been
// since the chart opened — the worst load on a monitor, a counter that must not
// appear to fall. [Zoomer.Autoscale] releases it. It is an option rather than
// the default because it answers the rarer question, and a chart that answers
// it should say so.
func HighWater() LinearOption { return func(l *linear) { l.highWater = true } }

// forget drops the trained range and keeps the range the scale maps into.
func (d *domainRange) forget() { d.dmin, d.dmax, d.trained = 0, 0, false }

func (l *linear) Retrain() {
	if l.fixed || l.highWater {
		return
	}
	l.forget()
	l.hasNiced = false
}

func (l *logScale) Retrain() {
	if !l.fixed {
		l.forget()
	}
}

func (s *symlogScale) Retrain() {
	if !s.fixed {
		s.forget()
	}
}

func (s *timeScale) Retrain() {
	if !s.fixed {
		s.forget()
	}
}

func (s *probScale) Retrain() {
	if !s.fixed {
		s.forget()
	}
}

func (c *colorScale) Retrain() {
	if !c.fixed {
		c.forget()
	}
}

// Retrain on a classed scale forgets its base range and, for a quantile scale,
// the sample its boundaries are cut from; a threshold scale's given boundaries
// are its classes and stay.
func (c *classed) Retrain() {
	c.base.Retrain()
	c.sample, c.cuts = c.sample[:0], c.cuts[:0]
}

// A bivariate scale forgets both readings' ranges and keeps its classes.
func (s *vsup) Retrain()   { s.classed.Retrain(); s.second.forget() }
func (s *matrix) Retrain() { s.classed.Retrain(); s.second.forget() }

func (s *sizeScale) Retrain() {
	if !s.fixed {
		s.forget()
	}
}

var (
	_ Retrainer = (*linear)(nil)
	_ Retrainer = (*logScale)(nil)
	_ Retrainer = (*symlogScale)(nil)
	_ Retrainer = (*timeScale)(nil)
	_ Retrainer = (*probScale)(nil)
	_ Retrainer = (*colorScale)(nil)
	_ Retrainer = (*classed)(nil)
	_ Retrainer = (*vsup)(nil)
	_ Retrainer = (*matrix)(nil)
	_ Retrainer = (*sizeScale)(nil)
)

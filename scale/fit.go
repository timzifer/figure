package scale

// FitView trains a linear axis on the rows its panel's other axis shows,
// rather than on every row: the price axis of a chart zoomed into one week
// fits that week, and a volume track under it fits the bars in view.
//
// Nothing else about the scale changes — [Nice], [Zero] and breaks apply to
// the fitted extent as to any other. It takes effect when the other axis's
// domain is pinned, by a zoom, a pan or a caller's [Zoomer.SetDomain]; while
// the other axis is trained from the data every row is in view anyway. A
// fitting axis is never steered by a zoom itself: it is derived, every frame,
// from the axis that is. See docs/adr/0089-a-value-axis-fits-what-its-time-axis-shows.md.
func FitView() LinearOption { return func(l *linear) { l.fit = true } }

// LogFitView is [FitView] for a log axis — a price axis is as often
// logarithmic as not.
func LogFitView() LogOption { return func(l *logScale) { l.fit = true } }

// Fitter is implemented by a scale that can be told to fit the rows its
// panel's other axis shows. It is an optional interface, like [Zoomer]: a
// scale that does not implement it trains on every row.
type Fitter interface {
	// FitsView reports whether the axis fits the other axis's view.
	FitsView() bool
}

// Pinner is implemented by a scale that can report whether its domain is
// pinned — fixed at construction, or set by [Zoomer.SetDomain] — rather than
// trained. A fitting axis is fitted to the other axis's domain exactly when
// that domain is pinned, which is the question this asks. It is an interface
// of its own rather than a method on Zoomer, because a published interface
// does not grow (ADR 0084).
type Pinner interface {
	// Pinned reports whether the domain is pinned.
	Pinned() bool
}

// FitsView reports whether s is a scale told to fit its panel's other axis.
func FitsView(s Scale) bool {
	f, ok := s.(Fitter)
	return ok && f.FitsView()
}

// Pinned reports whether s's domain is pinned. A scale that cannot say is not.
func Pinned(s Scale) bool {
	p, ok := s.(Pinner)
	return ok && p.Pinned()
}

func (l *linear) FitsView() bool   { return l.fit }
func (l *logScale) FitsView() bool { return l.fit }

func (l *linear) Pinned() bool      { return l.fixed }
func (l *logScale) Pinned() bool    { return l.fixed }
func (s *symlogScale) Pinned() bool { return s.fixed }
func (s *timeScale) Pinned() bool   { return s.fixed }
func (s *probScale) Pinned() bool   { return s.fixed }

var (
	_ Fitter = (*linear)(nil)
	_ Fitter = (*logScale)(nil)
	_ Pinner = (*linear)(nil)
	_ Pinner = (*logScale)(nil)
	_ Pinner = (*symlogScale)(nil)
	_ Pinner = (*timeScale)(nil)
	_ Pinner = (*probScale)(nil)
)

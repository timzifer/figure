package three

import (
	"errors"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/render"
	"github.com/timzifer/figure/theme"
)

// ErrNoScene is returned by a plot asked to draw before it was given
// something to draw.
var ErrNoScene = errors.New("three: the plot has no scene")

// Plot is a figure: one or more [View]s of a [Scene], at a size, in a theme.
//
// It is not a figure.Plot and the root package re-exports nothing of this one.
// figure's whole job is resolving a plot into a render.Chart, and a scene is
// not one — no coordinate system, no two-dimensional panels, no geoms — so a
// re-export would put names in the root package whose methods did not work on
// the type that lives there. That is the second path through one seam that
// ADR 0056 spends a version to avoid, moved to the root package.
//
// What is shared is the vocabulary: a scale is a scale, a theme is a theme, a
// channel is spelled with geom's options, and a target is an ir.Target — so a
// scene renders into every backend figure has without the root package being
// in the import graph at all.
type Plot struct {
	width, height int
	dpr           float64
	th            theme.Theme
	themeSet      bool
	title         string
	cols          int

	scene *Scene
	views []View

	desc    ir.Description
	descSet bool

	obs  render.Observer
	rows geom.Rows

	// sink is the scratch a repeatedly redrawn figure keeps rather than
	// borrowing — see draw. It is nil for a plot rendered once.
	sink *Sink
}

// Option configures a [Plot].
type Option func(*Plot)

// Size sets the figure's size in logical pixels.
func Size(w, h int) Option {
	return func(p *Plot) {
		if w > 0 && h > 0 {
			p.width, p.height = w, h
		}
	}
}

// DPR sets the device pixel ratio the figure is drawn at.
func DPR(r float64) Option {
	return func(p *Plot) {
		if r > 0 {
			p.dpr = r
		}
	}
}

// Theme sets the theme, which also supplies the light a face is shaded by.
func Theme(t theme.Theme) Option { return func(p *Plot) { p.th, p.themeSet = t, true } }

// Title sets the figure's title, above every view.
func Title(s string) Option { return func(p *Plot) { p.title = s } }

// Columns sets how many views sit side by side before the next row. It is one
// by default, which is what a figure with a single camera wants.
func Columns(n int) Option {
	return func(p *Plot) {
		if n > 0 {
			p.cols = n
		}
	}
}

// Description sets what the chart says about itself in words, for a backend
// that can carry it.
//
// It is camera-independent, and that is a requirement rather than a
// convenience: a reader using the description gets the same chart as a reader
// dragging the scene, and every static export says what the turned one says.
// See docs/adr/0057-orbiting-a-chart.md.
func Description(title, detail string) Option {
	return func(p *Plot) {
		p.desc, p.descSet = ir.Description{Title: title, Detail: detail}, true
	}
}

// New returns a plot.
func New(opts ...Option) *Plot {
	p := &Plot{width: 640, height: 480, dpr: 1, cols: 1}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Scene sets what the plot's views look at.
func (p *Plot) Scene(s *Scene) *Plot { p.scene = s; return p }

// Add appends views, in reading order across the grid.
func (p *Plot) Add(vs ...View) *Plot { p.views = append(p.views, vs...); return p }

// Views returns a copy of the plot's views.
func (p *Plot) Views() []View { return append([]View(nil), p.viewList()...) }

// Size reports the figure's size.
func (p *Plot) Size() (w, h int) { return p.width, p.height }

// Observer is told which view and which layer is drawing, so that a caller
// wrapping the backend can attribute a mark to the layer that made it. It is
// how hit-testing is built without widening the IR, and it is the same
// interface render announces through — so interact.Index needs no change to
// index a projected scene.
func (p *Plot) Observer(o render.Observer) *Plot { p.obs = o; return p }

// TrackRows collects which source row is behind each mark. It is nil for an
// ordinary render, and setting it is what turns the bookkeeping on.
func (p *Plot) TrackRows(r geom.Rows) *Plot { p.rows = r; return p }

// viewList is the plot's views, or the one view a plot that named none has.
//
// A figure with a single camera should not have to say so, and the camera it
// gets is [Home] rather than the zero value — because the angle a chart is
// designed at is a decision and a zero value is the absence of one.
func (p *Plot) viewList() []View {
	if len(p.views) == 0 {
		return []View{{Camera: Home()}}
	}
	return p.views
}

func (p *Plot) theme() theme.Theme {
	if p.themeSet {
		return p.th
	}
	return theme.Light
}

// Render draws the plot once into t.
//
// It wants a document or a surface indifferently: a single frame is the same
// call either way. A scene redrawn frame after frame into one surface is
// [Plot.Live].
func (p *Plot) Render(t ir.Target) (err error) {
	if t == nil {
		return errors.New("three: nil render target")
	}
	if p.scene == nil && !p.hasSceneInAView() {
		return ErrNoScene
	}
	b, err := t.Open(ir.Surface{WidthPx: p.width, HeightPx: p.height, DPR: p.dpr})
	if err != nil {
		return err
	}
	defer func() {
		if cerr := t.Close(); err == nil {
			err = cerr
		}
	}()
	if _, err = p.draw(b); err != nil {
		return err
	}
	return b.Flush()
}

func (p *Plot) hasSceneInAView() bool {
	for _, v := range p.views {
		if v.Scene != nil {
			return true
		}
	}
	return false
}

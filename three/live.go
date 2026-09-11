package three

import (
	"errors"
	"fmt"

	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/ir"
)

// Live is a scene drawn into a surface that can be redrawn and turned.
//
// It mirrors figure.Live's shape and keeps its split: a method that changes
// the view and does not draw returns nothing, and a method that draws returns
// an error. So turning a scene is two calls —
//
//	live.Camera(three.Orbit(live.CameraValue(), -dx*perPixel, dy*perPixel))
//	live.Draw()
//
// — and that is what leaves the host owning its own loop. This package
// installs no handler, opens no window and does not know where the two floats
// came from.
//
// It wants a surface rather than a document. The SVG and PDF emitters build a
// document and write it whole, so drawing many frames into one collects every
// frame in the same file; use [Plot.Render] for those. There is no interactive
// SVG and there will not be one: emitting a document with a script that
// re-projects the scene in the viewer means shipping a second renderer, in
// another language, inside a file.
//
// A Live is not safe for concurrent use.
type Live struct {
	p *Plot
	t ir.Target
	b ir.Backend

	width, height int
	dpr           float64

	idx  *interact.Index
	home []Camera

	prev, next *ir.Recorder
	rects      []ir.Rect
	drawn      bool

	areas []ir.Rect
	moved []bool
	turn  bool
}

// Live opens t and returns the scene drawn into it. The target stays open
// until [Live.Close].
func (p *Plot) Live(t ir.Target) (*Live, error) {
	if t == nil {
		return nil, errors.New("three: nil render target")
	}
	if p.scene == nil && !p.hasSceneInAView() {
		return nil, ErrNoScene
	}
	b, err := t.Open(ir.Surface{WidthPx: p.width, HeightPx: p.height, DPR: p.dpr})
	if err != nil {
		return nil, err
	}

	// The live plot is a copy with its own views, so that turning the scene
	// does not edit the specification the caller still holds.
	lp := *p
	lp.views = append([]View(nil), p.viewList()...)
	idx := interact.New()
	lp.obs = idx
	lp.sink = new(Sink)

	l := &Live{
		p: &lp, t: t, b: b,
		width: p.width, height: p.height, dpr: p.dpr,
		idx:   idx,
		home:  make([]Camera, len(lp.views)),
		moved: make([]bool, len(lp.views)),
	}
	for i, v := range lp.views {
		l.home[i] = v.Camera
	}
	l.prev = ir.NewRecorder(b)
	l.next = ir.NewRecorder(b)
	return l, nil
}

// TrackRows turns row identity on or off and returns l, so the call can be
// chained onto [Plot.Live].
//
// It is off by default because it is not free: with it on, every layer records
// where each of its rows landed and the hit index keeps a position and a row
// number per mark. Without it [interact.Hit.Row] is -1 — and in a projected
// scene that is the whole answer a pointer has, because there are no screen
// axes to invert a device point through.
func (l *Live) TrackRows(on bool) *Live {
	l.idx.TrackRows(on)
	l.p.rows = nil
	if on {
		l.p.rows = l.idx
	}
	l.drawn = false
	return l
}

// Camera points every view at cam.
//
// This is ADR 0057's spelling, and on a figure with several views it turns all
// of them together — a synchronised orbit of a front / top / side figure is one
// statement about the chart. [Live.SetCamera] turns one.
func (l *Live) Camera(cam Camera) *Live {
	for i := range l.p.views {
		l.setCamera(i, cam)
	}
	return l
}

// CameraValue is the first view's camera, which on a figure with one view is
// its camera.
func (l *Live) CameraValue() Camera { return l.CameraOf(0) }

// SetCamera points one view at cam.
func (l *Live) SetCamera(i int, cam Camera) *Live { l.setCamera(i, cam); return l }

// CameraOf reports one view's camera, or the zero camera for an index that is
// not a view.
func (l *Live) CameraOf(i int) Camera {
	if i < 0 || i >= len(l.p.views) {
		return Camera{}
	}
	return l.p.views[i].Camera
}

// ViewCount reports how many views the figure has.
func (l *Live) ViewCount() int { return len(l.p.views) }

// ViewAt reports which view a device position is in, or -1 for a position in
// none of them. It is what a host asks before deciding which view a drag
// turns.
func (l *Live) ViewAt(x, y float64) int {
	pt := ir.Point{X: float32(x), Y: float32(y)}
	for i, a := range l.areas {
		if a.Contains(pt) {
			return i
		}
	}
	return -1
}

// Home returns every view to the camera its author chose.
//
// A reader who has turned a scene into a mess is one call from the picture the
// chart was designed at, which is the other half of ADR 0057's requirement
// that the default camera be readable on its own.
func (l *Live) Home() *Live {
	for i, cam := range l.home {
		l.setCamera(i, cam)
	}
	return l
}

func (l *Live) setCamera(i int, cam Camera) {
	if i < 0 || i >= len(l.p.views) || l.p.views[i].Camera == cam {
		return
	}
	l.p.views[i].Camera = cam
	l.moved[i] = true
	l.turn = true
}

// Index is the hit index the last frame was drawn into.
func (l *Live) Index() *interact.Index { return l.idx }

// Size reports the surface's size.
func (l *Live) Size() (w, h int) { return l.width, l.height }

// Resize tells the figure its surface has changed size, and redraws it. The
// cameras keep whatever they were turned to.
func (l *Live) Resize(w, h int) error {
	if l.b == nil {
		return errors.New("three: Resize on a closed Live")
	}
	if w <= 0 || h <= 0 {
		return fmt.Errorf("three: chart size %dx%d is not positive", w, h)
	}
	if w == l.width && h == l.height {
		return nil
	}
	l.width, l.height = w, h
	l.p.width, l.p.height = w, h
	if r, ok := l.b.(ir.Resizer); ok {
		if err := r.Resize(ir.Surface{WidthPx: w, HeightPx: h, DPR: l.dpr}); err != nil {
			return err
		}
	}
	l.drawn = false
	return l.Draw()
}

// Rescale tells the figure its surface's device pixel ratio has changed, and
// redraws it.
func (l *Live) Rescale(dpr float64) error {
	if l.b == nil {
		return errors.New("three: Rescale on a closed Live")
	}
	if dpr <= 0 {
		return fmt.Errorf("three: device pixel ratio %v is not positive", dpr)
	}
	if dpr == l.dpr {
		return nil
	}
	l.dpr = dpr
	l.p.dpr = dpr
	if r, ok := l.b.(ir.Resizer); ok {
		if err := r.Resize(ir.Surface{WidthPx: l.width, HeightPx: l.height, DPR: dpr}); err != nil {
			return err
		}
	}
	l.drawn = false
	return l.Draw()
}

// Draw records a frame and paints what changed.
//
// # What a redraw costs
//
// A frame whose camera moved is not comparable with the last one: every
// drawing call in the turned view differs, because every point moved. Diffing
// two whole recordings to arrive at an answer known before it started is work
// nobody asked for, so a turn damages the views that turned and takes the
// diffing path when the cameras are at rest — where it earns its keep exactly
// as it does in a flat chart, because a changing dataset under a fixed camera
// is the case damage was designed for. See ADR 0016 and ADR 0057.
//
// Expressing it that way rather than as a "a drag is in flight" flag is
// deliberate. A flag is state the library cannot verify, it is wrong for a
// programmatic eased orbit that no pointer is involved in, and a host that
// forgets to turn it off pays for a full repaint forever with nothing failing.
// A camera moved or it did not, and this package knows which.
func (l *Live) Draw() error {
	if l.b == nil {
		return errors.New("three: Draw on a closed Live")
	}
	l.idx.Reset()
	l.next.Reset()
	areas, err := l.p.draw(l.idx.Watch(l.next))
	if err != nil {
		return err
	}
	turned := l.turn
	l.areas = areas

	if turned || !l.drawn {
		if partial, ok := l.b.(ir.Partial); ok {
			partial.Damage(l.turnedRects())
		}
	} else {
		rects, sameShape := ir.Damage(l.prev, l.next, l.rects)
		l.rects = rects
		if sameShape && len(rects) == 0 {
			// The frame is identical to the last one, so nothing is painted at
			// all — which is what makes a live chart under a still pointer
			// cost nothing.
			l.clearTurn()
			return nil
		}
		if partial, ok := l.b.(ir.Partial); ok {
			if sameShape {
				partial.Damage(rects)
			} else {
				partial.Damage(nil)
			}
		}
	}

	l.next.Replay(l.b)
	if err := l.b.Flush(); err != nil {
		return err
	}
	l.prev, l.next = l.next, l.prev
	l.drawn = true
	l.clearTurn()
	return nil
}

// turnedRects is what to repaint after a turn: the cells whose cameras moved,
// or the whole surface when there is nothing to compare against.
//
// Reporting the moved cells rather than the whole canvas is strictly more than
// ADR 0057 asks for, and it is what a figure of four views wants: three of
// them genuinely did not change.
func (l *Live) turnedRects() []ir.Rect {
	if !l.drawn {
		return nil
	}
	l.rects = l.rects[:0]
	for i, moved := range l.moved {
		if moved && i < len(l.areas) {
			l.rects = append(l.rects, l.areas[i])
		}
	}
	if len(l.rects) == 0 {
		return nil
	}
	return l.rects
}

func (l *Live) clearTurn() {
	l.turn = false
	for i := range l.moved {
		l.moved[i] = false
	}
}

// Close finalises the target. Drawing after it is an error.
func (l *Live) Close() error {
	if l.b == nil {
		return nil
	}
	l.b = nil
	return l.t.Close()
}

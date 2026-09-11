package three

import (
	"math"
	"testing"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/render"
)

// recordingOverlay notes what it was told and where in the frame it was told
// it, which is what the ordering and attribution tests read.
type recordingOverlay struct {
	frame  OverlayFrame
	before int  // how many calls the frame had made when this was called
	depth  int  // the clip depth it was called at
	drew   bool // whether it drew anything of its own
	rec    *irtest.Recorder
	mark   []ir.Point
}

func (o *recordingOverlay) DrawOverlay(b ir.Backend, f OverlayFrame) {
	o.frame = f
	o.drew = true
	if o.rec != nil {
		o.before, o.depth = len(o.rec.Calls), o.rec.Depth
	}
	if len(o.mark) == 0 {
		return
	}
	var p ir.Path
	for _, at := range o.mark {
		p.Circle(at, 4)
	}
	b.StrokePath(&p, ir.Stroke{Width: 1})
}

// atCorner is a layer whose whole output is one small triangle round a point of
// the data, so that a test can compare where the painter put it with where
// [OverlayView.At] says it is.
type atCorner struct{ x, y, z float64 }

func (atCorner) Train(geom.Training) error { return nil }

func (a atCorner) Emit(s *Sink, f Frame) error {
	c := Vec3{at(f.X, a.x), at(f.Y, a.y), at(f.Z, a.z)}
	// A square centred on the point, so that its centroid is the point exactly
	// — the projection is affine, so the centroid of what was drawn is the
	// projection of the centre and the test can compare the two directly.
	const e = 0.002
	s.Face([]Vec3{
		{c.X - e, c.Y - e, c.Z},
		{c.X + e, c.Y - e, c.Z},
		{c.X + e, c.Y + e, c.Z},
		{c.X - e, c.Y + e, c.Z},
	}, Style{Fill: ir.Color{R: 255, A: 255}})
	return nil
}

func (atCorner) Legend(Frame) (geom.LegendEntry, bool) { return geom.LegendEntry{}, false }

// A figure that was handed no overlay draws exactly what a figure that was
// never offered one draws. It is the property that lets one set of golden files
// cover both, and the one that says an unused seam costs nothing.
func TestAFigureWithNoOverlayDrawsWhatItAlwaysDid(t *testing.T) {
	draw := func(install bool) []string {
		rec := irtest.New()
		p := New(Size(420, 320)).Scene(surfaceScene(5, 5))
		if install {
			p.Overlay(nil)
		}
		if _, err := p.draw(rec); err != nil {
			t.Fatal(err)
		}
		return rec.Trace()
	}
	plain, offered := draw(false), draw(true)
	if len(plain) != len(offered) {
		t.Fatalf("an offered nil overlay changed the frame: %d calls against %d", len(offered), len(plain))
	}
	for i := range plain {
		if plain[i] != offered[i] {
			t.Fatalf("call %d differs:\n  plain:   %s\n  offered: %s", i, plain[i], offered[i])
		}
	}
}

// An overlay is drawn last of all and outside every clip. Last, because it
// paints over a finished figure; outside, because a ring may sit on the edge of
// the cell it belongs to and a caption beside it.
func TestAnOverlayIsDrawnLastAndUnclipped(t *testing.T) {
	rec := irtest.New()
	ov := &recordingOverlay{rec: rec, mark: []ir.Point{{X: 200, Y: 150}}}

	p := New(Size(420, 320)).Scene(surfaceScene(5, 5)).Overlay(ov)
	if _, err := p.draw(rec); err != nil {
		t.Fatal(err)
	}
	if !ov.drew {
		t.Fatal("the overlay was never called")
	}
	if ov.before != len(rec.Calls)-1 {
		t.Errorf("the overlay drew at call %d of %d; it must be last", ov.before, len(rec.Calls))
	}
	if ov.depth != 0 {
		t.Errorf("the overlay was called inside %d clips; it must be clipped by nothing", ov.depth)
	}
}

// An overlay is not a mark. It is drawn after the observer is told the frame is
// over, so that nothing it paints is attributed to the layer that happened to
// be painted last — which is what keeps a ring out of the hit index, and a ring
// a pointer can hit is a ring that flickers.
func TestAnOverlayIsNotAnnouncedToTheObserver(t *testing.T) {
	rec := irtest.New()
	obs := &endObserver{}
	ov := &recordingOverlay{rec: rec, mark: []ir.Point{{X: 200, Y: 150}}}

	p := New(Size(420, 320)).Scene(surfaceScene(5, 5)).Observer(obs).Overlay(ov)
	if _, err := p.draw(rec); err != nil {
		t.Fatal(err)
	}
	if !obs.ended {
		t.Fatal("the observer was never told the frame ended")
	}
	if obs.endedAt > ov.before {
		t.Errorf("the frame ended at call %d, after the overlay drew at %d", obs.endedAt, ov.before)
	}
	if obs.layersAfterEnd > 0 {
		t.Errorf("%d layers were announced after the frame ended", obs.layersAfterEnd)
	}
}

// endObserver notes when the frame was declared over and whether any layer was
// announced afterwards.
type endObserver struct {
	ended          bool
	endedAt        int
	layersAfterEnd int
	seen           int
}

func (o *endObserver) Panel(render.PanelInfo) {}
func (o *endObserver) Layer(render.LayerInfo) {
	if o.ended {
		o.layersAfterEnd++
	}
	o.seen++
}
func (o *endObserver) End() { o.ended = true; o.endedAt = o.seen }

// An overlay is told every view, in the order they were added, with the cell
// each was drawn in — which is the cell [Live.ViewAt] matches a pointer
// against.
func TestAnOverlaySeesEveryView(t *testing.T) {
	rec := irtest.New()
	ov := &recordingOverlay{}

	p := New(Size(640, 480), Columns(2)).Scene(surfaceScene(4, 4)).Overlay(ov).Add(
		View{Camera: Home(), Label: "three-quarter"},
		View{Camera: LookAt(Elevation(1.45)), Label: "plan"},
		View{Camera: LookAt(Azimuth(0)), Label: "front"},
		View{Camera: LookAt(Azimuth(-math.Pi / 2)), Label: "side"},
	)
	areas, err := p.draw(rec)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ov.frame.Views); got != 4 {
		t.Fatalf("the overlay saw %d views, want 4", got)
	}
	for i, want := range []string{"three-quarter", "plan", "front", "side"} {
		v := ov.frame.Views[i]
		if v.Index != i || v.Label != want {
			t.Errorf("view %d is %d %q, want %d %q", i, v.Index, v.Label, i, want)
		}
		if v.Area != areas[i] {
			t.Errorf("view %d covers %v, but the figure drew it in %v", i, v.Area, areas[i])
		}
		if !v.Project.Valid() {
			t.Errorf("view %d has no projection", i)
		}
		if v.X == nil || v.Y == nil || v.Z == nil {
			t.Errorf("view %d is missing a scale", i)
		}
	}
	if _, ok := ov.frame.ViewAt(ir.Point{X: areas[2].Min.X + 2, Y: areas[2].Min.Y + 2}); !ok {
		t.Error("ViewAt found no view at the third cell's own corner")
	}
}

// The load-bearing one: an overlay asking where a value is gets the place the
// painter actually put it. The two projections are one projection, and this is
// what stops them drifting apart.
func TestAnOverlayProjectsWhereTheSceneDrew(t *testing.T) {
	rec := irtest.New()
	ov := &recordingOverlay{}

	const x, y, z = 0.25, 0.75, 0.4
	sc := NewScene().Add(atCorner{x, y, z})
	if _, err := New(Size(400, 400)).Scene(sc).Overlay(ov).draw(rec); err != nil {
		t.Fatal(err)
	}
	v, ok := ov.frame.View(0)
	if !ok {
		t.Fatal("the overlay saw no view")
	}
	got, ok := v.At(x, y, z)
	if !ok {
		t.Fatal("the view has no place for a value inside its domain")
	}

	// The triangle is symmetric about its point and the projection is affine,
	// so the centroid of what was drawn is the projection of the centre.
	var drew []ir.Point
	for _, c := range rec.Filter("FillPath") {
		if pts := c.Path.Pts; len(pts) == 4 {
			drew = pts
		}
	}
	if drew == nil {
		t.Fatal("the layer's face was never painted")
	}
	var cx, cy float32
	for _, p := range drew {
		cx, cy = cx+p.X/4, cy+p.Y/4
	}
	const tol = 0.05
	if math.Abs(float64(cx-got.X)) > tol || math.Abs(float64(cy-got.Y)) > tol {
		t.Errorf("the overlay places the value at (%.3f, %.3f); the painter drew it at (%.3f, %.3f)",
			got.X, got.Y, cx, cy)
	}
}

// One value, four cameras, four rings. A figure with several views is one chart
// looked at several ways, so a point marked in the three-quarter view is marked
// in the plan and both profiles without the host saying so four times.
func TestAHighlightRingsOneValueInEveryView(t *testing.T) {
	rec := irtest.New()
	h := &Highlight{Data: []Point3{{X: 0.5, Y: 0.5, Z: 0.5}}, View: -1}

	p := New(Size(640, 480), Columns(2)).Scene(surfaceScene(4, 4)).Overlay(h).Add(
		View{Camera: Home()},
		View{Camera: LookAt(Elevation(1.45))},
		View{Camera: LookAt(Azimuth(0))},
		View{Camera: LookAt(Azimuth(-math.Pi / 2))},
	)
	areas, err := p.draw(rec)
	if err != nil {
		t.Fatal(err)
	}

	// One StrokePath, four circles: the rings are batched into one path, which
	// is what keeps a selection one drawing call however many views there are.
	var rings []ir.Point
	for _, c := range rec.Filter("StrokePath") {
		if pts := c.Path.Pts; len(pts) >= 4 && c.Stroke.Width == 2 {
			rings = pts
		}
	}
	if rings == nil {
		t.Fatal("the highlight drew nothing")
	}
	for i, a := range areas {
		found := false
		for _, pt := range rings {
			if a.Contains(pt) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("view %d has no ring in it", i)
		}
	}
}

// A scene hides its own far side, so a ring may be over a point the reader
// cannot actually see. Such a ring is dashed and a visible one is solid, which
// is the hidden-line convention an engineering drawing has always used — and it
// is what stops a reader taking a position off the near face that is not the
// position they picked.
func TestARingBehindSomethingIsDashed(t *testing.T) {
	rec := irtest.New()
	h := &Highlight{View: -1, Marks: []Mark{
		{At: ir.Point{X: 160, Y: 140}},
		{At: ir.Point{X: 240, Y: 200}, Hidden: true},
	}}
	if _, err := New(Size(400, 320)).Scene(surfaceScene(4, 4)).Overlay(h).draw(rec); err != nil {
		t.Fatal(err)
	}

	var solid, dashed int
	for _, c := range rec.Filter("StrokePath") {
		if c.Stroke.Width != 2 {
			continue
		}
		if len(c.Stroke.Dash) > 0 {
			dashed++
		} else {
			solid++
		}
	}
	if solid != 1 || dashed != 1 {
		t.Errorf("the highlight drew %d solid and %d dashed strokes, want one of each", solid, dashed)
	}
}

// The two groups are drawn as two paths and not two strokes per ring, so a
// selection of any size is two calls.
func TestRingsAreBatchedIntoTwoPaths(t *testing.T) {
	rec := irtest.New()
	h := &Highlight{View: -1}
	for i := range 8 {
		h.Marks = append(h.Marks, Mark{
			At:     ir.Point{X: float32(120 + 10*i), Y: 160},
			Hidden: i%2 == 0,
		})
	}
	if _, err := New(Size(400, 320)).Scene(surfaceScene(4, 4)).Overlay(h).draw(rec); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range rec.Filter("StrokePath") {
		if c.Stroke.Width == 2 {
			n++
		}
	}
	if n != 2 {
		t.Errorf("eight rings took %d strokes, want two", n)
	}
}

// A value named in the data is rung solid: it is a place in the cube rather
// than something the scene drew, so there is nothing for it to be behind.
func TestAValueIsRungSolid(t *testing.T) {
	rec := irtest.New()
	h := &Highlight{Data: []Point3{{X: 0.5, Y: 0.5, Z: 0.5}}, View: -1}
	if _, err := New(Size(400, 320)).Scene(surfaceScene(4, 4)).Overlay(h).draw(rec); err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Filter("StrokePath") {
		if c.Stroke.Width == 2 && len(c.Stroke.Dash) > 0 {
			t.Error("a value was rung dashed")
		}
	}
}

// A highlight confined to one view draws in that view and nowhere else.
func TestAHighlightMayBeConfinedToOneView(t *testing.T) {
	rec := irtest.New()
	h := &Highlight{Data: []Point3{{X: 0.5, Y: 0.5, Z: 0.5}}, View: 1}

	p := New(Size(640, 240), Columns(2)).Scene(surfaceScene(4, 4)).Overlay(h).Add(
		View{Camera: Home()},
		View{Camera: LookAt(Elevation(1.45))},
	)
	areas, err := p.draw(rec)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Filter("StrokePath") {
		if c.Stroke.Width != 2 {
			continue
		}
		for _, pt := range c.Path.Pts {
			if areas[0].Contains(pt) {
				t.Errorf("a ring confined to view 1 was drawn in view 0, at %v", pt)
			}
		}
	}
}

// The zero value draws nothing, which is the promise every overlay in this
// package and in the root one makes.
func TestAnEmptyHighlightDrawsNothing(t *testing.T) {
	rec := irtest.New()
	plain := irtest.New()

	sc := surfaceScene(4, 4)
	if _, err := New(Size(400, 320)).Scene(sc).draw(plain); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Size(400, 320)).Scene(sc).Overlay(&Highlight{}).draw(rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Calls) != len(plain.Calls) {
		t.Errorf("an empty highlight drew %d calls", len(rec.Calls)-len(plain.Calls))
	}
}

// Overlays draws its members in order and skips the nil ones, so a caller may
// keep a fixed-length list and switch one off by clearing it.
func TestOverlaysDrawInOrderAndSkipNil(t *testing.T) {
	rec := irtest.New()
	first := &recordingOverlay{rec: rec, mark: []ir.Point{{X: 100, Y: 100}}}
	second := &recordingOverlay{rec: rec, mark: []ir.Point{{X: 120, Y: 100}}}

	p := New(Size(400, 320)).Scene(surfaceScene(4, 4)).
		Overlay(Overlays{first, nil, second})
	if _, err := p.draw(rec); err != nil {
		t.Fatal(err)
	}
	if !first.drew || !second.drew {
		t.Fatal("a member of the list was not drawn")
	}
	if first.before >= second.before {
		t.Errorf("the second member drew at call %d, before the first at %d", second.before, first.before)
	}
}

// A Live opened from a plot inherits the plot's overlay, because it copies the
// plot — and installing one on the Live afterwards does not reach back to the
// specification the caller still holds.
func TestALiveInheritsThePlotsOverlayAndDoesNotWriteBack(t *testing.T) {
	ov := &recordingOverlay{}
	p := New(Size(400, 320)).Scene(surfaceScene(4, 4)).Overlay(ov)

	live, err := p.Live(irtest.New().Target())
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()

	if live.CurrentOverlay() != Overlay(ov) {
		t.Error("the live figure did not inherit the plot's overlay")
	}
	live.Overlay(nil)
	if p.CurrentOverlay() != Overlay(ov) {
		t.Error("installing an overlay on the live figure edited the plot")
	}
}

// Installing or removing an overlay changes how many calls a frame has, so the
// frame is not comparable with the last and the whole canvas is repainted.
func TestInstallingAnOverlayRepaintsTheWholeCanvas(t *testing.T) {
	rec := irtest.New()
	live, err := New(Size(400, 320)).Scene(surfaceScene(4, 4)).Live(rec.Target())
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()

	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	live.Overlay(&Highlight{Marks: []Mark{{At: ir.Point{X: 200, Y: 160}}}, View: -1})
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	if n := len(rec.Whole); n == 0 || !rec.Whole[n-1] {
		t.Errorf("installing an overlay damaged %v, want the whole canvas", rec.Damaged)
	}
}

// A turn with an overlay installed repaints the canvas rather than the cells
// whose cameras moved. An overlay draws where it likes, so no list of cells
// describes what it damaged — and a figure with no overlay still repaints only
// the cells that turned.
func TestATurnWithAnOverlayRepaintsTheWholeCanvas(t *testing.T) {
	turn := func(ov Overlay) (whole bool, rects []ir.Rect) {
		rec := irtest.New()
		p := New(Size(640, 240), Columns(2)).Scene(surfaceScene(4, 4)).Add(
			View{Camera: Home()},
			View{Camera: LookAt(Elevation(1.45))},
		)
		live, err := p.Live(rec.Target())
		if err != nil {
			t.Fatal(err)
		}
		defer live.Close()

		if ov != nil {
			live.Overlay(ov)
		}
		if err := live.Draw(); err != nil {
			t.Fatal(err)
		}
		live.SetCamera(0, Orbit(live.CameraOf(0), 0.4, 0.1))
		if err := live.Draw(); err != nil {
			t.Fatal(err)
		}
		n := len(rec.Whole)
		if n == 0 {
			t.Fatal("nothing was ever damaged")
		}
		return rec.Whole[n-1], rec.Damaged[n-1]
	}

	if whole, rects := turn(nil); whole || len(rects) != 1 {
		t.Errorf("a turn with no overlay damaged %v (whole=%v), want the one cell that moved", rects, whole)
	}
	if whole, rects := turn(&Highlight{Marks: []Mark{{At: ir.Point{X: 100, Y: 100}}}, View: -1}); !whole {
		t.Errorf("a turn with an overlay damaged %v, want the whole canvas", rects)
	}
}

// An overlay that moved under a camera that did not is repainted, because a
// frame no camera turned is compared with the last one call for call.
func TestAnOverlayThatMovedIsRepainted(t *testing.T) {
	rec := irtest.New()
	h := &Highlight{Marks: []Mark{{At: ir.Point{X: 120, Y: 120}}}, View: -1}

	live, err := New(Size(400, 320)).Scene(surfaceScene(4, 4)).Overlay(h).Live(rec.Target())
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()

	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	frames := rec.Frames
	h.Marks[0].At = ir.Point{X: 260, Y: 200}
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}
	if rec.Frames == frames {
		t.Fatal("moving an overlay painted no frame")
	}
	n := len(rec.Damaged)
	if !rec.Whole[n-1] && len(rec.Damaged[n-1]) == 0 {
		t.Error("moving an overlay damaged nothing")
	}
}

// A scene redrawn with an overlay installed allocates no more than the same
// scene without one, which is what keeps the view list on the pooled sink
// rather than made per frame — one slice per frame would be invisible in a
// benchmark and would show up on every figure once the views multiplied.
func TestAnOverlayDoesNotAllocatePerFrame(t *testing.T) {
	frame := func(ov Overlay) float64 {
		p := New(Size(640, 240), Columns(2)).Scene(surfaceScene(8, 8)).Add(
			View{Camera: Home()},
			View{Camera: LookAt(Elevation(1.45))},
		)
		if ov != nil {
			p.Overlay(ov)
		}
		live, err := p.Live(irtest.NullTarget())
		if err != nil {
			t.Fatal(err)
		}
		defer live.Close()

		if err := live.Draw(); err != nil {
			t.Fatal(err)
		}
		return testing.AllocsPerRun(20, func() {
			live.SetCamera(0, Orbit(live.CameraOf(0), 0.01, 0))
			if err := live.Draw(); err != nil {
				t.Fatal(err)
			}
		})
	}

	plain := frame(nil)
	with := frame(&Highlight{Data: []Point3{{X: 0.5, Y: 0.5, Z: 0.5}}, View: -1})
	if with > plain {
		t.Errorf("a frame with an overlay allocated %.0f times against %.0f without one", with, plain)
	}
}

// A scene tells a host how far each row it reports was from the camera, which
// is the number that says whether the reader can actually see it. A flat chart
// has no such number and says so.
func TestAReportedRowCarriesItsDepth(t *testing.T) {
	rec := irtest.New()
	idx := interact.New()
	idx.TrackRows(true)

	p := New(Size(400, 320)).Scene(surfaceScene(6, 6)).Observer(idx).TrackRows(idx)
	if _, err := p.draw(idx.Watch(rec)); err != nil {
		t.Fatal(err)
	}

	refs := idx.RowsOf(0, 0, nil)
	if len(refs) < 4 {
		t.Fatalf("the surface reported %d rows", len(refs))
	}
	near, far := math.Inf(1), math.Inf(-1)
	for _, r := range refs {
		if !r.Deep {
			t.Fatalf("row %d came back with no depth", r.Row)
		}
		near, far = math.Min(near, r.Depth), math.Max(far, r.Depth)
	}
	if !(far > near) {
		t.Errorf("every row of a turned surface is at depth %v, so nothing is in front of anything", near)
	}
}

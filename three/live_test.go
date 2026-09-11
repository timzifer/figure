package three

import (
	"testing"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
)

func liveOn(t *testing.T, rec *irtest.Recorder, p *Plot) *Live {
	t.Helper()
	l, err := p.Live(rec.Target())
	if err != nil {
		t.Fatalf("live: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

// A turn is not comparable with the frame before it, so the diff is skipped
// and the turned view is repainted outright. Walking two whole recordings to
// arrive at an answer known before it started is work nobody asked for.
func TestATurnRepaintsTheViewItTurned(t *testing.T) {
	rec := irtest.New()
	l := liveOn(t, rec, New(Size(600, 300), Columns(2)).Scene(surfaceScene(5, 5)).Add(
		View{Camera: Home()}, View{Camera: Home()},
	))
	if err := l.Draw(); err != nil {
		t.Fatalf("first draw: %v", err)
	}
	if !rec.Whole[0] {
		t.Error("the first frame did not repaint the whole surface")
	}

	l.SetCamera(1, Orbit(l.CameraOf(1), 0.4, 0.1))
	if err := l.Draw(); err != nil {
		t.Fatalf("second draw: %v", err)
	}
	if len(rec.Damaged) != 2 {
		t.Fatalf("got %d damage reports, want one per frame", len(rec.Damaged))
	}
	if rec.Whole[1] {
		t.Fatal("turning one view of two repainted the whole surface")
	}
	if got := len(rec.Damaged[1]); got != 1 {
		t.Fatalf("turning one view damaged %d rectangles, want the one cell that moved", got)
	}
	// And it is the cell that turned rather than the one that did not.
	if want := l.areas[1]; rec.Damaged[1][0] != want {
		t.Errorf("the damaged rectangle is %v, want the second view at %v", rec.Damaged[1][0], want)
	}
}

// At rest the diff earns its keep exactly as it does in a flat chart: a frame
// identical to the last one is not painted at all.
func TestAFrameThatDidNotMoveIsNotPainted(t *testing.T) {
	rec := irtest.New()
	l := liveOn(t, rec, New(Size(300, 300)).Scene(surfaceScene(5, 5)))
	if err := l.Draw(); err != nil {
		t.Fatalf("first draw: %v", err)
	}
	frames := rec.Frames
	if err := l.Draw(); err != nil {
		t.Fatalf("second draw: %v", err)
	}
	if rec.Frames != frames {
		t.Errorf("an unchanged frame was painted anyway: %d frames, was %d", rec.Frames, frames)
	}
}

// Setting a camera to the value it already has is not a turn, so a host that
// resends the same camera on every pointer move does not repaint forever.
func TestSettingTheSameCameraIsNotATurn(t *testing.T) {
	rec := irtest.New()
	l := liveOn(t, rec, New(Size(300, 300)).Scene(surfaceScene(4, 4)))
	if err := l.Draw(); err != nil {
		t.Fatal(err)
	}
	frames := rec.Frames
	l.Camera(l.CameraValue())
	if err := l.Draw(); err != nil {
		t.Fatal(err)
	}
	if rec.Frames != frames {
		t.Errorf("resending the same camera painted a frame")
	}
}

// Home is one call back to the picture the author chose, which is what a
// reader who has turned the scene into a mess needs.
func TestHomeReturnsToTheAuthorsCameras(t *testing.T) {
	rec := irtest.New()
	author := LookAt(Azimuth(1.2), Elevation(0.3))
	l := liveOn(t, rec, New(Size(300, 300)).Scene(surfaceScene(4, 4)).Add(
		View{Camera: author},
	))
	if err := l.Draw(); err != nil {
		t.Fatal(err)
	}
	l.Camera(Orbit(l.CameraValue(), 2, -0.9))
	if l.CameraValue() == author {
		t.Fatal("the orbit did not move the camera")
	}
	l.Home()
	if got := l.CameraValue(); got != author {
		t.Errorf("Home gave %+v, want the author's %+v", got, author)
	}
}

// The single-camera spelling turns every view together, which on a
// front/top/side figure is one statement about the chart.
func TestTheSingleCameraSpellingTurnsEveryView(t *testing.T) {
	rec := irtest.New()
	l := liveOn(t, rec, New(Size(900, 300), Columns(3)).Scene(surfaceScene(4, 4)).Add(
		View{Camera: Home()}, View{Camera: LookAt()}, View{Camera: LookAt(Elevation(1))},
	))
	want := LookAt(Azimuth(0.5), Elevation(0.2))
	l.Camera(want)
	for i := 0; i < l.ViewCount(); i++ {
		if got := l.CameraOf(i); got != want {
			t.Errorf("view %d is at %+v, want %+v", i, got, want)
		}
	}
}

// A host that wants to turn the view under the pointer has to be able to ask
// which one that is.
func TestViewAtNamesTheCellUnderThePointer(t *testing.T) {
	rec := irtest.New()
	l := liveOn(t, rec, New(Size(600, 300), Columns(2)).Scene(surfaceScene(4, 4)).Add(
		View{Camera: Home()}, View{Camera: Home()},
	))
	if err := l.Draw(); err != nil {
		t.Fatal(err)
	}
	for i, a := range l.areas {
		mid := ir.Point{X: (a.Min.X + a.Max.X) / 2, Y: (a.Min.Y + a.Max.Y) / 2}
		if got := l.ViewAt(float64(mid.X), float64(mid.Y)); got != i {
			t.Errorf("the middle of view %d reports view %d", i, got)
		}
	}
	if got := l.ViewAt(-10, -10); got != -1 {
		t.Errorf("a point outside every view reports %d, want -1", got)
	}
}

// A pointer over a projected scene reports which mark and which row it landed
// on. It does not report an x and a y, and that is ADR 0056's honest limit
// rather than a gap: a turned cube has no screen axes to invert a device
// position through, so the third value is read from the row the caller
// supplied.
func TestAHitReportsItsRowAndNotAPairOfValues(t *testing.T) {
	rec := irtest.New()
	sc := surfaceScene(6, 6)
	l := liveOn(t, rec, New(Size(400, 400)).Scene(sc)).TrackRows(true)
	if err := l.Draw(); err != nil {
		t.Fatal(err)
	}

	idx := l.Index()
	if idx.MarkCount() == 0 {
		t.Fatal("nothing was indexed")
	}
	if idx.RowCount() == 0 {
		t.Fatal("no marks carry a source row")
	}
	var hit interact.Hit
	var found bool
	for _, ref := range idx.RowsOf(0, 0, nil) {
		hit, found = idx.At(ref.At, interact.DefaultTolerance)
		if found {
			break
		}
	}
	if !found {
		t.Fatal("no mark was found where a row was reported")
	}
	if hit.Row < 0 {
		t.Error("the hit carries no source row; in a projected scene that is the whole answer")
	}
	if hit.X != 0 || hit.Y != 0 {
		t.Errorf("the hit reports x=%v y=%v; a projected scene has no screen axes to invert through",
			hit.X, hit.Y)
	}
}

// A watched render draws exactly what an unwatched one draws. Anything else
// would silently halve the coverage of every golden file.
func TestAWatchedSceneDrawsWhatAnUnwatchedOneDoes(t *testing.T) {
	sc := surfaceScene(5, 5).Add(Line3(ridge(5, 5), geom.X("x"), geom.Y("y"), geom.Z("z")))

	plain := irtest.New()
	if err := New(Size(400, 400)).Scene(sc).Render(plain.Target()); err != nil {
		t.Fatal(err)
	}
	watched := irtest.New()
	idx := interact.New()
	if err := New(Size(400, 400)).Scene(sc).Observer(idx).
		Render(watchTarget{rec: watched, idx: idx}); err != nil {
		t.Fatal(err)
	}

	a, b := plain.Trace(), watched.Trace()
	if len(a) != len(b) {
		t.Fatalf("watched made %d calls and unwatched %d", len(b), len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("call %d differs:\nunwatched %s\n  watched %s", i, a[i], b[i])
		}
	}
}

// Resizing keeps the cameras: a reader who has turned a scene into place has
// not asked to leave it because the window moved.
func TestResizeKeepsTheCameras(t *testing.T) {
	rec := irtest.New()
	l := liveOn(t, rec, New(Size(300, 300)).Scene(surfaceScene(4, 4)))
	if err := l.Draw(); err != nil {
		t.Fatal(err)
	}
	turned := Orbit(l.CameraValue(), 0.7, 0.2)
	l.Camera(turned)
	if err := l.Resize(500, 400); err != nil {
		t.Fatalf("resize: %v", err)
	}
	if got := l.CameraValue(); got != turned {
		t.Errorf("resizing moved the camera to %+v, want %+v", got, turned)
	}
	if w, h := l.Size(); w != 500 || h != 400 {
		t.Errorf("the surface is %dx%d, want 500x400", w, h)
	}
}

func TestDrawAfterCloseIsAnError(t *testing.T) {
	rec := irtest.New()
	l, err := New(Size(200, 200)).Scene(surfaceScene(3, 3)).Live(rec.Target())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := l.Draw(); err == nil {
		t.Error("drawing into a closed Live succeeded")
	}
}

// watchTarget opens a backend with a hit index wrapped round it, which is what
// a live chart draws through.
type watchTarget struct {
	rec *irtest.Recorder
	idx *interact.Index
}

func (w watchTarget) Open(ir.Surface) (ir.Backend, error) { return w.idx.Watch(w.rec), nil }
func (w watchTarget) Close() error                        { return nil }

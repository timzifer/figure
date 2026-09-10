package three

import (
	"strings"
	"testing"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/scale"
)

// The description does not change when the scene turns, and that is the
// requirement rather than a nicety: a reader using it gets the same chart as a
// reader dragging the picture, and every static export says what the turned
// one says.
func TestTheDescriptionIsCameraIndependent(t *testing.T) {
	sc := surfaceScene(5, 5)
	at := func(cams ...Camera) string {
		p := New(Size(400, 400)).Scene(sc)
		for _, c := range cams {
			p.Add(View{Camera: c})
		}
		return p.Describe().Detail
	}
	one := at(Home())
	turned := at(Orbit(Home(), 2.1, -0.6))
	four := at(Home(), LookAt(), LookAt(Elevation(1.4)), LookAt(Azimuth(3)))

	if one != turned {
		t.Errorf("turning the scene changed the description:\n %q\nvs %q", one, turned)
	}
	if one != four {
		t.Errorf("adding cameras changed the description:\n %q\nvs %q", one, four)
	}
}

// A reader who cannot see the picture is told there are three axes and what
// the third one's range is — which is the reading the flat chart of the same
// table does not have.
func TestTheDescriptionNamesTheDepthAxis(t *testing.T) {
	sc := NewScene(XTitle("x"), YTitle("y"), ZTitle("amplitude")).
		Z(scale.Linear(scale.Nice())).
		Add(Surface(ridge(4, 4), geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Label("ripple")))

	got := New(Size(300, 300)).Scene(sc).Describe().Detail
	for _, want := range []string{"amplitude upward", "ripple", "z from"} {
		if !strings.Contains(got, want) {
			t.Errorf("the description is missing %q:\n%s", want, got)
		}
	}
}

// The data table carries every column the scene reads, including the depth
// one — the third of the three channels, and the one a scene that must be
// turned to be read leans on hardest.
func TestTheDataTableCarriesTheDepthColumn(t *testing.T) {
	sc := surfaceScene(3, 3)
	var b strings.Builder
	if err := New().Scene(sc).DataTable(&b); err != nil {
		t.Fatalf("data table: %v", err)
	}
	for _, want := range []string{"<table>", ">x<", ">y<", ">z<"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("the data table is missing %q:\n%s", want, b.String())
		}
	}
}

// A backend that can carry a description is told one before any ink.
func TestTheSceneAnnouncesItselfBeforeDrawing(t *testing.T) {
	rec := irtest.New()
	err := New(Size(300, 300), Title("Ripple")).Scene(surfaceScene(3, 3)).Render(rec.Target())
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Described) != 1 {
		t.Fatalf("the backend was described %d times, want once", len(rec.Described))
	}
	if got := rec.Described[0].Title; got != "Ripple" {
		t.Errorf("the description is titled %q, want the chart's own title", got)
	}
}

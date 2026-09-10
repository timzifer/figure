package three

import (
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func latency() figure.Source {
	var svc, reg []string
	var ms []float64
	for j, r := range []string{"eu", "us", "ap"} {
		for i, s := range []string{"auth", "search", "cart", "pay"} {
			svc = append(svc, s)
			reg = append(reg, r)
			ms = append(ms, 40+float64(i*13)+float64(j*9))
		}
	}
	return figure.NewTable().String("s", svc).String("r", reg).Float64("v", ms)
}

func barScene() *Scene {
	return NewScene(XTitle("service"), YTitle("region"), ZTitle("p99")).
		X(scale.Ordinal()).Y(scale.Ordinal()).
		Z(scale.Linear(scale.Nice(), scale.Zero())).
		Add(Bar3(latency(), geom.X("s"), geom.Y("r"), geom.Z("v"),
			geom.Fill(theme.Light.Palette[0]), geom.BarWidth(0.7)))
}

// Every face of a box is shaded by the normal that points out of it, whichever
// side of the box the camera happens to be on.
//
// Deriving the normal from the corners is what makes this worth a test: a face
// written in one fixed order is wound outward from one side and inward from
// the other, so the shading would flip as the reader turned past an axis —
// invisible in a still and obvious in a drag.
func TestEveryBarFaceIsShadedByItsOutwardNormal(t *testing.T) {
	for _, cam := range octants() {
		faces, normals := visibleFaces(cam.Forward(), 0.2, 0.3, 0.4, 0.5, 0, 0.8)
		for k := range faces {
			c := faces[k]
			mid := c[0].Add(c[1]).Add(c[2]).Add(c[3]).Mul(0.25)
			// A normal points out when stepping along it leaves the box.
			out := mid.Add(normals[k].Mul(0.05))
			if inside(out, 0.2, 0.3, 0.4, 0.5, 0, 0.8) {
				t.Errorf("camera %+v: face %d's normal %v points into the box", cam, k, normals[k])
			}
			// And it is perpendicular to the face, so it says something about
			// how the face is lit rather than about how it was written down.
			if d := normals[k].Dot(c[1].Sub(c[0])); d > 1e-6 || d < -1e-6 {
				t.Errorf("camera %+v: face %d's normal is not perpendicular to it", cam, k)
			}
		}
	}
}

func inside(v Vec3, x0, y0, x1, y1, lo, hi float32) bool {
	return v.X > x0 && v.X < x1 && v.Y > y0 && v.Y < y1 && v.Z > lo && v.Z < hi
}

// A bar shows three faces and they all carry the same source row, so a pointer
// anywhere on it reports that bar.
func TestABarIsOneRowWhicheverFaceIsPointedAt(t *testing.T) {
	rec := irtest.New()
	l, err := New(Size(400, 400)).Scene(barScene()).Live(rec.Target())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	l.TrackRows(true)
	if err := l.Draw(); err != nil {
		t.Fatal(err)
	}
	refs := l.Index().RowsOf(0, 0, nil)
	if len(refs) != 3*12 {
		t.Errorf("%d marks carry a row, want three faces for each of twelve bars", len(refs))
	}
	seen := map[int]int{}
	for _, r := range refs {
		seen[r.Row]++
	}
	for row, n := range seen {
		if n != 3 {
			t.Errorf("row %d is behind %d marks, want its three visible faces", row, n)
		}
	}
}

// A tall bar never draws itself in front of the short one standing between it
// and the reader, because the order is the cell on the floor rather than the
// middle of the box.
func TestABarIsOrderedByItsCellAndNotItsHeight(t *testing.T) {
	s := acquire()
	defer release(s)
	cam := Home()
	fwd := cam.Forward()
	s.openLayer(0, fwd)

	f := Frame{
		X: unitScales()[0], Y: unitScales()[1], Z: unitScales()[2],
		Theme: theme.Light, Forward: fwd,
	}
	g := Bar3(nil).(*bar3)
	g.ok = true
	// Two cells on one line away from the camera, the far one much taller.
	g.xs = []float64{0.5, 0.5}
	g.ys = []float64{0.1, 0.9}
	g.zs = []float64{0.2, 0.9}
	if err := g.Emit(s, f); err != nil {
		t.Fatal(err)
	}
	if len(s.prims) != 6 {
		t.Fatalf("%d primitives, want three faces for each of two bars", len(s.prims))
	}
	// Whichever cell is farther along the view direction is emitted first, and
	// its own height changes nothing about that.
	first, second := s.prims[0].key, s.prims[3].key
	if first <= second {
		t.Errorf("the first bar's key is %v and the second's %v: the far cell must come first", first, second)
	}
}

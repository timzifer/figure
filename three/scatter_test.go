package three

import (
	"math"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// clouds is two clusters that overlap along x and y and are apart only in z:
// the chart a 3D scatter exists for.
func clouds() figure.Source {
	var xs, ys, zs []float64
	var group []string
	for i := range 20 {
		a := float64(i) * 2.399963 // the golden angle, so the points spread
		for k, lift := range []float64{0.2, 0.8} {
			xs = append(xs, 0.5+0.2*math.Cos(a))
			ys = append(ys, 0.5+0.2*math.Sin(a))
			zs = append(zs, lift+0.05*math.Sin(3*a))
			group = append(group, []string{"low", "high"}[k])
		}
	}
	return figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("z", zs).String("g", group)
}

func scatterScene(opts ...geom.Option) *Scene {
	o := append([]geom.Option{geom.X("x"), geom.Y("y"), geom.Z("z")}, opts...)
	return NewScene().
		X(scale.Linear(scale.Domain(0, 1))).Y(scale.Linear(scale.Domain(0, 1))).
		Z(scale.Linear(scale.Domain(0, 1))).
		Add(Scatter3(clouds(), o...))
}

func TestAScatterIsAMarkerAndARulePerRow(t *testing.T) {
	s := acquire()
	defer release(s)
	cam := Home()
	s.openLayer(0, cam.Forward())
	f := Frame{
		X: unitScales()[0], Y: unitScales()[1], Z: unitScales()[2],
		Theme: theme.Light, Forward: cam.Forward(),
	}
	g := Scatter3(nil).(*scatter3)
	g.ok = true
	g.xs, g.ys, g.zs = []float64{0.2, 0.7}, []float64{0.3, 0.6}, []float64{0.5, 0}
	if err := g.Emit(s, f); err != nil {
		t.Fatal(err)
	}
	var markers, lines int
	for _, p := range s.prims {
		switch p.kind {
		case kindMarker:
			markers++
		case kindLine:
			lines++
			// A rule runs from the floor straight up to its point.
			a, b := s.verts[p.lo], s.verts[p.lo+1]
			if a.Z != 0 || a.X != b.X || a.Y != b.Y {
				t.Errorf("a dropline from %v to %v is not a vertical rule from the floor", a, b)
			}
		}
	}
	// The second point lies on the floor already, so it has no rule to draw.
	if markers != 2 || lines != 1 {
		t.Errorf("%d markers and %d rules, want 2 and 1", markers, lines)
	}

	s.reset()
	s.openLayer(0, cam.Forward())
	g.cfg.HideDroplines = true
	if err := g.Emit(s, f); err != nil {
		t.Fatal(err)
	}
	for _, p := range s.prims {
		if p.kind == kindLine {
			t.Fatal("geom.Droplines(false) still drew a rule")
		}
	}
}

func TestAScatterPaintsMarkersAndReportsTheirRows(t *testing.T) {
	rec := irtest.New()
	l, err := New(Size(400, 400)).Scene(scatterScene(
		geom.ColorBy("g", scale.Qualitative(palette.OkabeIto)))).Live(rec.Target())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	l.TrackRows(true)
	if err := l.Draw(); err != nil {
		t.Fatal(err)
	}

	symbols := 0
	colours := map[ir.Color]bool{}
	for _, c := range rec.Filter("Markers") {
		symbols += len(c.Points)
		colours[c.Style.Fill] = true
	}
	if symbols != 40 {
		t.Errorf("%d symbols drawn, want one per row", symbols)
	}
	if len(colours) != 2 {
		t.Errorf("%d marker colours, want one per cluster", len(colours))
	}
	if n := rec.Count("Polyline"); n < 40 {
		t.Errorf("%d polylines, want at least a dropline per row besides the cube", n)
	}

	seen := map[int]bool{}
	for _, r := range l.Index().RowsOf(0, 0, nil) {
		seen[r.Row] = true
	}
	if len(seen) != 40 {
		t.Errorf("%d rows reported, want all 40", len(seen))
	}
}

func TestAMarkerBehindAFaceIsDrawnBeforeIt(t *testing.T) {
	s := acquire()
	defer release(s)
	cam := Home()
	fwd := cam.Forward()
	s.openLayer(0, fwd)
	// A face through the middle of the cube, square to the view, and one
	// marker on each side of it.
	c := centre
	u := Vec3{-fwd.Y, fwd.X, 0}.Unit()
	v := fwd.Cross(u).Unit()
	face := []Vec3{
		c.Add(u.Mul(0.3)).Add(v.Mul(0.3)), c.Sub(u.Mul(0.3)).Add(v.Mul(0.3)),
		c.Sub(u.Mul(0.3)).Sub(v.Mul(0.3)), c.Add(u.Mul(0.3)).Sub(v.Mul(0.3)),
	}
	s.Marker(c.Sub(fwd.Mul(0.2)), Style{Fill: palette.Blue, Size: 6}) // near
	s.Face(face, Style{Fill: palette.Orange})
	s.Marker(c.Add(fwd.Mul(0.2)), Style{Fill: palette.Vermilion, Size: 6}) // far

	rec := irtest.New()
	s.paint(rec, project(cam, ir.R(0, 0, 400, 400)), nil, 0, nil, nil)
	var order []string
	for _, call := range rec.Calls {
		switch call.Op {
		case "Markers":
			if call.Style.Fill == palette.Blue {
				order = append(order, "near")
			} else {
				order = append(order, "far")
			}
		case "FillPath":
			order = append(order, "face")
		}
	}
	if len(order) != 3 || order[0] != "far" || order[1] != "face" || order[2] != "near" {
		t.Errorf("painted %v, want far, face, near", order)
	}
}

func TestAScatterDescribesItself(t *testing.T) {
	d := Scatter3(clouds(), geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Droplines(false)).(Describer).Describe()
	if d.Mark != "scatter3" || !d.HideDroplines {
		t.Errorf("Desc = %+v", d)
	}
}

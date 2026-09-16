package three

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

func nearVec(t *testing.T, got, want Vec3, what string) {
	t.Helper()
	d := got.Sub(want)
	if math.Abs(float64(d.X))+math.Abs(float64(d.Y))+math.Abs(float64(d.Z)) > 1e-5 {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func TestASphericalSceneMapsAnglesAndARadiusOntoTheBall(t *testing.T) {
	sp := &sphere{}
	// Azimuth and polar angle as fractions of their turns, radius as a
	// fraction of the ball.
	nearVec(t, sp.place(0, 0, 1), Vec3{0.5, 0.5, 1}, "north pole")
	nearVec(t, sp.place(0, 1, 1), Vec3{0.5, 0.5, 0}, "south pole")
	nearVec(t, sp.place(0, 0.5, 1), Vec3{1, 0.5, 0.5}, "+x on the equator")
	nearVec(t, sp.place(0.25, 0.5, 1), Vec3{0.5, 1, 0.5}, "+y on the equator")
	nearVec(t, sp.place(0.3, 0.7, 0), centre, "radius zero")

	lat := &sphere{elevation: true}
	nearVec(t, lat.place(0, 1, 1), Vec3{0.5, 0.5, 1}, "latitude +90°")
	nearVec(t, lat.place(0, 0.5, 1), Vec3{1, 0.5, 0.5}, "latitude 0°")

	var f Frame
	f.X, f.Y, f.Z = unitScales()[0], unitScales()[1], unitScales()[2]
	if f.Spherical() || f.Point(1, 2, 3) != (Vec3{0.1, 0.2, 0.3}) {
		t.Error("a frame of an ordinary scene does not place a point where it is")
	}
}

func TestASphericalScenePinsItsAnglesAndTrainsItsRadiusFromZero(t *testing.T) {
	src := figure.NewTable().
		Float64("phi", []float64{10, 200}).Float64("theta", []float64{20, 40}).Float64("r", []float64{3, 5})
	for name, tc := range map[string]struct {
		opts    []SphereOption
		az, pol [2]float64
	}{
		"degrees":  {nil, [2]float64{0, 360}, [2]float64{0, 180}},
		"radians":  {[]SphereOption{Radians()}, [2]float64{0, 2 * math.Pi}, [2]float64{0, math.Pi}},
		"latitude": {[]SphereOption{Latitude()}, [2]float64{0, 360}, [2]float64{-90, 90}},
	} {
		t.Run(name, func(t *testing.T) {
			sc := NewScene(Spherical(tc.opts...)).
				X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear()).
				Add(Scatter3(src, geom.X("phi"), geom.Y("theta"), geom.Z("r")))
			scales := sc.scales()
			if err := sc.train(scales); err != nil {
				t.Fatal(err)
			}
			if lo, hi := scales[axisX].Domain(); lo != tc.az[0] || hi != tc.az[1] {
				t.Errorf("azimuth domain [%v, %v], want %v", lo, hi, tc.az)
			}
			if lo, hi := scales[axisY].Domain(); lo != tc.pol[0] || hi != tc.pol[1] {
				t.Errorf("polar domain [%v, %v], want %v", lo, hi, tc.pol)
			}
			if lo, _ := scales[axisZ].Domain(); lo > 0 {
				t.Errorf("radius domain starts at %v, want it to reach the centre", lo)
			}
		})
	}

	ordinal := NewScene(Spherical()).X(scale.Ordinal()).
		Add(Scatter3(src, geom.X("phi"), geom.Y("theta"), geom.Z("r")))
	if err := ordinal.train(ordinal.scales()); err == nil {
		t.Error("an ordinal azimuth was accepted")
	}
}

// pattern is a closed grid of directions every step degrees in azimuth from 0
// to (360 − step), over polar angles 0 to 180, with a radius of one.
func pattern(step, azTo int) figure.Source {
	var phi, theta, r []float64
	for th := 0; th <= 180; th += 30 {
		for p := 0; p <= azTo; p += step {
			phi, theta, r = append(phi, float64(p)), append(theta, float64(th)), append(r, 1)
		}
	}
	return figure.NewTable().Float64("phi", phi).Float64("theta", theta).Float64("r", r)
}

func emitFaces(t *testing.T, src figure.Source) (*Sink, int) {
	t.Helper()
	sc := NewScene(Spherical()).Add(Surface(src, geom.X("phi"), geom.Y("theta"), geom.Z("r")))
	scales := sc.scales()
	if err := sc.train(scales); err != nil {
		t.Fatal(err)
	}
	s := acquire()
	cam := Home()
	s.openLayer(0, cam.Forward())
	f := Frame{X: scales[0], Y: scales[1], Z: scales[2], Forward: cam.Forward(), space: sc.sphere}
	if err := sc.layers[0].Emit(s, f); err != nil {
		t.Fatal(err)
	}
	return s, len(s.prims)
}

func TestASurfaceClosesRoundTheAzimuthAndNotRoundASector(t *testing.T) {
	// Twelve columns every 30° from 0 to 330: the cell from 330 back to 360
	// is drawn, so every row of cells has twelve, not eleven.
	s, closed := emitFaces(t, pattern(30, 330))
	defer release(s)
	if closed != 6*12 {
		t.Errorf("%d faces over a closed pattern, want twelve cells in each of six rows", closed)
	}
	// A quarter sector stays open.
	s2, sector := emitFaces(t, pattern(30, 90))
	defer release(s2)
	if sector != 6*3 {
		t.Errorf("%d faces over a quarter sector, want three cells in each of six rows", sector)
	}
}

func TestEveryFaceOfASphericalSurfaceIsLitFromOutside(t *testing.T) {
	c := [4]Vec3{
		(&sphere{}).place(0, 0.5, 1), (&sphere{}).place(0.1, 0.5, 1),
		(&sphere{}).place(0.1, 0.6, 1), (&sphere{}).place(0, 0.6, 1),
	}
	n := awayFromCentre(faceNormal(c[0], c[1], c[2]), c)
	mid := c[0].Add(c[2]).Mul(0.5)
	if n.Dot(mid.Sub(centre)) <= 0 {
		t.Errorf("normal %v points into the ball", n)
	}
	flipped := awayFromCentre(n.Mul(-1), c)
	if flipped.Dot(mid.Sub(centre)) <= 0 {
		t.Errorf("an inward normal was not turned out: %v", flipped)
	}
}

func TestBarsAndContoursHaveNoPlaceOnASphere(t *testing.T) {
	src := figure.NewTable().
		Float64("x", []float64{0, 90, 0, 90}).Float64("y", []float64{0, 0, 90, 90}).Float64("z", []float64{1, 2, 3, 4})
	for name, l := range map[string]Layer{
		"bars":    Bar3(src, geom.X("x"), geom.Y("y"), geom.Z("z")),
		"contour": Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z")),
	} {
		sc := NewScene(Spherical()).Add(l)
		err := New(Size(300, 300)).Scene(sc).Render(irtest.New().Target())
		if !errors.Is(err, ErrNotSpherical) {
			t.Errorf("%s on a sphere: err = %v, want ErrNotSpherical", name, err)
		}
	}
}

func TestAGlobeDrawsItsFarHalfFirstAndItsLabelsLast(t *testing.T) {
	src := figure.NewTable().Float64("phi", []float64{45}).Float64("theta", []float64{60}).Float64("r", []float64{1})
	sc := NewScene(Spherical(AxisEnds("S1", "", "S2", "", "S3", ""))).
		Z(scale.Linear(scale.Domain(0, 1))).
		Add(Scatter3(src, geom.X("phi"), geom.Y("theta"), geom.Z("r")))
	rec := irtest.New()
	if err := New(Size(400, 400)).Scene(sc).Render(rec.Target()); err != nil {
		t.Fatal(err)
	}
	marker, lastStroke, firstStroke := -1, -1, -1
	texts := 0
	for i, c := range rec.Calls {
		switch c.Op {
		case "Markers":
			marker = i
		case "StrokePath":
			if firstStroke < 0 {
				firstStroke = i
			}
			lastStroke = i
		case "Text":
			for _, want := range []string{"S1", "S2", "S3"} {
				if c.Text.Text == want {
					texts++
					if i < marker {
						t.Errorf("label %s drawn before the data", want)
					}
				}
			}
		}
	}
	if marker < 0 {
		t.Fatal("the point was not drawn")
	}
	if !(firstStroke < marker && marker < lastStroke) {
		t.Errorf("globe strokes at %d..%d and the marker at %d: want the far half before it and the near half after", firstStroke, lastStroke, marker)
	}
	if texts != 3 {
		t.Errorf("%d axis labels, want the three that were named", texts)
	}
}

func TestAnOverlayFindsAValueWhereTheSphereDrewIt(t *testing.T) {
	sp := &sphere{}
	x, y, z := scale.Linear(scale.Domain(0, 360)), scale.Linear(scale.Domain(0, 180)), scale.Linear(scale.Domain(0, 1))
	for _, s := range []scale.Scale{x, y, z} {
		s.SetRange(0, 1)
	}
	pj := Projection{p: project(Home(), ir.R(0, 0, 400, 400))}
	v := OverlayView{X: x, Y: y, Z: z, Project: pj, space: sp}
	got, ok := v.At(90, 90, 1)
	if !ok {
		t.Fatal("At refused a direction on the sphere")
	}
	want := pj.Point(Vec3{0.5, 1, 0.5})
	if math.Abs(float64(got.X-want.X)) > 0.01 || math.Abs(float64(got.Y-want.Y)) > 0.01 {
		t.Errorf("At(90°, 90°, 1) = %v, want +y on the equator at %v", got, want)
	}
}

// The angular sphere writes values on its graticule: the azimuths round the
// equator and the second angle up one meridian. Without them a reader who
// wanted to name a direction had to count graticule lines out from an axis.
func TestASphereLabelsItsGraticule(t *testing.T) {
	texts := sphereTexts(t, Spherical())
	// Only the half facing the reader is written on, which is the rule the
	// Smith sphere's labels already follow — a number on the far side reads
	// backwards through the ball.
	azimuths := 0
	for _, v := range []string{"30", "60", "120", "150", "210", "240", "300", "330"} {
		if texts[v] {
			azimuths++
		}
	}
	if azimuths < 3 {
		t.Errorf("the graticule writes %d azimuths, want the near half of them", azimuths)
	}
	// And the polar angles, up whichever meridian faces the reader. At least
	// two of them, because one number is not a ladder.
	kept := 0
	for _, v := range []string{"30", "60", "120", "150"} {
		if texts[v] {
			kept++
		}
	}
	if kept < 2 {
		t.Errorf("the graticule writes %d polar angles, want a ladder", kept)
	}
}

// A direction that already has a name is read by it rather than by its angle,
// so an axis end [AxisEnds] labelled is not also given a number.
func TestASphereDoesNotNumberADirectionItHasNamed(t *testing.T) {
	texts := sphereTexts(t, Spherical(AxisEnds("east", "west", "north", "south", "up", "down")))
	if texts["0"] || texts["90"] || texts["180"] || texts["270"] {
		t.Error("the graticule numbers an azimuth the axis ends already name")
	}
	for _, want := range []string{"east", "west", "north", "south"} {
		if !texts[want] {
			t.Errorf("the axis end %q is missing", want)
		}
	}
}

// The values are in the units the scene's own scales are read in, so a
// latitude sphere writes latitudes and a radian one writes radians.
func TestASpheresGraticuleIsWrittenInTheScenesOwnUnits(t *testing.T) {
	lat := sphereTexts(t, Spherical(Latitude()))
	// A polar angle of 30° from the north pole is a latitude of 60°, and one
	// of 150° is −60°. At least one of the negative ones proves the
	// conversion, since only the near half of the ball is written on.
	signed := false
	for _, v := range []string{"-30", "-60"} {
		if lat[v] {
			signed = true
		}
	}
	if !signed {
		t.Error("a latitude sphere writes no southern latitude on its graticule")
	}

	rad := sphereTexts(t, Spherical(Radians()))
	if rad["30"] || rad["60"] {
		t.Error("a radian sphere writes its graticule in degrees")
	}
	if !rad["0.5236"] {
		t.Error("a radian sphere does not write 30° as 0.5236 radians")
	}
}

// sphereTexts draws a spherical scene and collects every string in it.
func sphereTexts(t *testing.T, opt SceneOption) map[string]bool {
	t.Helper()
	src := figure.NewTable().
		Float64("az", []float64{0, 90}).
		Float64("pol", []float64{45, 60}).
		Float64("r", []float64{1, 1})
	sc := NewScene(opt).
		X(scale.Linear()).Y(scale.Linear()).Z(scale.Linear(scale.Domain(0, 1))).
		Add(Scatter3(src, geom.X("az"), geom.Y("pol"), geom.Z("r")))
	rec := irtest.New()
	if err := New(Size(420, 380)).Scene(sc).Render(rec.Target()); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, s := range rec.Texts() {
		out[s] = true
	}
	return out
}

// The 3D milestone, end to end.
//
// A scene of three measured columns is drawn as a surface, as a trajectory and
// as a field of bars; one scene is looked at from four cameras in one figure;
// a pointer over a projected mark names the row it landed on; and the whole of
// it reaches the SVG emitter as the paths and text runs every other chart in
// this library reaches it as. That last sentence is ADR 0056's central claim
// and the reason every backend draws a surface on the day figure/three
// compiles.
package figure_test

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/backend/svg"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/internal/svgdiff"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
	"github.com/timzifer/figure/three"
)

// golden3 is [golden] for a projected scene. It shares the -update flag, the
// directory and the structural comparison, because two conventions for one
// kind of file is how two of them drift apart.
func golden3(t *testing.T, name string, p *three.Plot) {
	t.Helper()

	var buf bytes.Buffer
	if err := p.Render(figure.SVGWriter(&buf, svg.Pretty())); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.Bytes()

	path := filepath.Join("testdata", "golden", name+".svg")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run `go test . -update` to create it)", err)
	}
	if ok, why := svgdiff.Equal(got, want, svgdiff.DefaultTolerance); !ok {
		t.Errorf("%s differs from the golden file: %s", name, why)
	}
}

// saddleGrid is a response with a ridge one way and a trough the other: a
// shape a heatmap of the same numbers reports symmetrically and which is not.
func saddleGrid(n int) figure.Source {
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	zs := make([]float64, 0, n*n)
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			x := -3 + 6*float64(i)/float64(n-1)
			y := -3 + 6*float64(j)/float64(n-1)
			xs = append(xs, x)
			ys = append(ys, y)
			zs = append(zs, math.Sin(x)*math.Cos(y)*1.4+0.25*x)
		}
	}
	return figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("z", zs)
}

func saddleScene(n int) *three.Scene {
	return three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("z")).
		Z(scale.Linear(scale.Nice())).
		Add(three.Surface(saddleGrid(n),
			geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Fill(palette.Blue)))
}

func TestGoldenSurface(t *testing.T) {
	golden3(t, "surface", three.New(
		three.Size(560, 460),
		three.Title("A saddle"),
		three.Theme(theme.Light),
	).Scene(saddleScene(14)))
}

// Four cameras on one scene, which is the arrangement the package is built on:
// the data is one value and the way of looking at it is another.
func TestGoldenViews(t *testing.T) {
	golden3(t, "views", three.New(
		three.Size(720, 560),
		three.Title("One surface, four cameras"),
		three.Theme(theme.Light),
		three.Columns(2),
	).Scene(saddleScene(10)).Add(
		three.View{Camera: three.Home(), Label: "three-quarter"},
		three.View{Camera: three.LookAt(three.Elevation(1.45)), Label: "plan"},
		three.View{Camera: three.LookAt(three.Azimuth(0), three.Elevation(0.02)), Label: "front"},
		three.View{Camera: three.LookAt(three.Azimuth(-math.Pi/2), three.Elevation(0.02)), Label: "side"},
	))
}

// A trajectory through the same box, and a field of bars over two
// categoricals: the two other rank-one forms, on the same machinery.
func TestGoldenTrajectory(t *testing.T) {
	const n = 200
	xs := make([]float64, n)
	ys := make([]float64, n)
	zs := make([]float64, n)
	for i := range xs {
		t := float64(i) / float64(n-1) * 6 * math.Pi
		xs[i] = math.Cos(t) * (1 - float64(i)/float64(2*n))
		ys[i] = math.Sin(t) * (1 - float64(i)/float64(2*n))
		zs[i] = t
	}
	src := figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("t", zs)

	sc := three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("t")).
		Z(scale.Linear(scale.Nice())).
		Add(three.Line3(src, geom.X("x"), geom.Y("y"), geom.Z("t"),
			geom.Color(palette.Vermilion), geom.Width(1.5)))

	golden3(t, "trajectory", three.New(
		three.Size(520, 480),
		three.Title("A decaying spiral"),
		three.Theme(theme.Dark),
	).Scene(sc))
}

func TestGoldenBars3(t *testing.T) {
	services := []string{"auth", "search", "cart", "pay"}
	regions := []string{"eu", "us", "ap"}
	var svc, reg []string
	var ms []float64
	for j, r := range regions {
		for i, s := range services {
			svc = append(svc, s)
			reg = append(reg, r)
			ms = append(ms, 40+float64(i*13)+float64(j*9)+float64((i*j)%3)*11)
		}
	}
	src := figure.NewTable().String("service", svc).String("region", reg).Float64("ms", ms)

	sc := three.NewScene(three.XTitle("service"), three.YTitle("region"), three.ZTitle("p99 (ms)")).
		X(scale.Ordinal()).
		Y(scale.Ordinal()).
		Z(scale.Linear(scale.Nice(), scale.Zero())).
		Add(three.Bar3(src, geom.X("service"), geom.Y("region"), geom.Z("ms"),
			geom.Fill(palette.Blue), geom.BarWidth(0.75)))

	golden3(t, "bars3", three.New(
		three.Size(560, 460),
		three.Title("Latency by service and region"),
		three.Theme(theme.Light),
	).Scene(sc))
}

// A pointer over a projected mark names the row it landed on. It does not name
// an x and a y — a turned cube has no screen axes to invert a device position
// through — which is ADR 0056's bargain: figure says exactly which datum it
// is and the program says what that datum contains.
func TestAPointerOverASceneNamesItsRow(t *testing.T) {
	sc := saddleScene(8)
	rec := irtest.New()
	live, err := three.New(three.Size(420, 420)).Scene(sc).Live(rec.Target())
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	live.TrackRows(true)
	if err := live.Draw(); err != nil {
		t.Fatal(err)
	}

	idx := live.Index()
	refs := idx.RowsOf(0, 0, nil)
	if len(refs) == 0 {
		t.Fatal("no marks reported a row")
	}
	hit, ok := idx.At(refs[0].At, interact.DefaultTolerance)
	if !ok {
		t.Fatal("nothing was found where a row was reported")
	}
	if hit.Row < 0 {
		t.Error("the hit carries no row")
	}
	if hit.X != 0 || hit.Y != 0 {
		t.Errorf("the hit reports x=%v y=%v; a projected scene has no screen axes", hit.X, hit.Y)
	}
}

// The document a scene produces carries nothing the IR did not already have.
// If this ever fails, an implementation has started putting a depth into a
// drawing call and the design has to come back to ADR 0056 before it goes
// further.
func TestASceneReachesTheEmitterAsOrdinaryInk(t *testing.T) {
	var buf bytes.Buffer
	err := three.New(three.Size(400, 400), three.Theme(theme.Light)).
		Scene(saddleScene(8)).
		Render(figure.SVGWriter(&buf, svg.Pretty()))
	if err != nil {
		t.Fatal(err)
	}
	doc := buf.String()
	if !strings.Contains(doc, "<path") || !strings.Contains(doc, "<text") {
		t.Fatal("the scene drew neither paths nor text")
	}
	for _, forbidden := range []string{"matrix3d", "z-index", "<script", "perspective"} {
		if strings.Contains(doc, forbidden) {
			t.Errorf("the document carries %q", forbidden)
		}
	}
}

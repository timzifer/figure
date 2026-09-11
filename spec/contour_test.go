package spec_test

import (
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

func gridTable(nx, ny int) *data.Table {
	xs := make([]float64, 0, nx*ny)
	ys := make([]float64, 0, nx*ny)
	zs := make([]float64, 0, nx*ny)
	for j := range ny {
		for i := range nx {
			x := -3 + 6*float64(i)/float64(nx-1)
			y := -3 + 6*float64(j)/float64(ny-1)
			xs, ys = append(xs, x), append(ys, y)
			zs = append(zs, x*x-y*y)
		}
	}
	return data.NewTable().Float64("x", xs).Float64("y", ys).Float64("z", zs)
}

// A contour survives the round trip, levels and all — which is the point of
// writing them down: a document that reads back as different lines is a
// document about a different chart.
func TestAContourSurvivesTheRoundTrip(t *testing.T) {
	src := gridTable(12, 12)
	for _, tc := range []struct {
		name  string
		layer geom.Geom
	}{
		{"levels", geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
			geom.Levels(-4, -2, 0, 2, 4))},
		{"count", geom.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
			geom.LevelCount(9))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := spec.Chart{
				Width: 500, Height: 350, DPR: 1, Theme: theme.Light,
				X: scale.Linear(), Y: scale.Linear(), Layers: []geom.Geom{tc.layer},
			}
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the contour did not survive the round trip\n%s", b)
			}
		})
	}
}

// A contour writes levels and not bins. There are n levels and n+1 bands, so a
// document saying "bins" for a contour would claim the mark bins.
func TestAContourWritesLevelsAndNotBins(t *testing.T) {
	s, err := spec.Of(spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Layers: []geom.Geom{
			geom.Contour(gridTable(10, 10), geom.X("x"), geom.Y("y"), geom.Z("z"),
				geom.Levels(-2, 0, 2)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	m := s.Layer[0].Mark
	if m.Type != "contour" {
		t.Errorf("the mark is %q, want contour", m.Type)
	}
	if len(m.Levels) != 3 {
		t.Errorf("the contour wrote levels %v, want three", m.Levels)
	}
	if m.Bins != 0 || m.Bandwidth != 0 {
		t.Errorf("the contour carries a histogram's bins or a density's bandwidth: %+v", m)
	}
}

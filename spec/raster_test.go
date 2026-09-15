package spec_test

import (
	"math"
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

// measuredField is a table of (t, hz, db) rows over a regular grid, which is
// the shape a raster requires.
func measuredField(nx, ny int) *data.Table {
	ts := make([]float64, 0, nx*ny)
	hz := make([]float64, 0, nx*ny)
	db := make([]float64, 0, nx*ny)
	for j := range ny {
		for i := range nx {
			x, y := float64(i)/4, float64(j)*100
			ts, hz = append(ts, x), append(hz, y)
			db = append(db, 20*math.Sin(x)+float64(j))
		}
	}
	return data.NewTable().Float64("t", ts).Float64("hz", hz).Float64("db", db)
}

// A raster survives the round trip, resampled the same way. The reduction has
// to travel: a spectrogram reduced by the mean and one reduced by the peak are
// different pictures of the same rows.
func TestARasterSurvivesTheRoundTrip(t *testing.T) {
	src := measuredField(24, 16)
	ramp := scale.Sequential(palette.Viridis)
	for _, tc := range []struct {
		name  string
		layer geom.Geom
	}{
		{"default", geom.Raster(src, geom.X("t"), geom.Y("hz"), geom.Z("db"))},
		{"ramp", geom.Raster(src, geom.X("t"), geom.Y("hz"), geom.Z("db"),
			geom.ColorBy("db", ramp))},
		{"max", geom.Raster(src, geom.X("t"), geom.Y("hz"), geom.Z("db"),
			geom.ColorBy("db", ramp), geom.Resample(geom.Max))},
		{"mean", geom.Raster(src, geom.X("t"), geom.Y("hz"), geom.Z("db"),
			geom.Resample(geom.Mean))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := spec.Chart{
				Width: 480, Height: 320, DPR: 1, Theme: theme.Light,
				X: scale.Linear(), Y: scale.Linear(), Layers: []geom.Geom{tc.layer},
			}
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the raster did not survive the round trip\n%s", b)
			}
		})
	}
}

// The mark is figure's own word. Vega-Lite's "image" draws a picture the data
// points at, so borrowing it would make a document decode into a mark that
// draws something else.
func TestARasterWritesItsOwnMarkAndItsReduction(t *testing.T) {
	src := measuredField(12, 8)
	of := func(g geom.Geom) spec.Mark {
		s, err := spec.Of(spec.Chart{
			Width: 480, Height: 320, DPR: 1, Theme: theme.Light,
			X: scale.Linear(), Y: scale.Linear(), Layers: []geom.Geom{g},
		})
		if err != nil {
			t.Fatal(err)
		}
		return s.Layer[0].Mark
	}

	m := of(geom.Raster(src, geom.X("t"), geom.Y("hz"), geom.Z("db"), geom.Resample(geom.Max)))
	if m.Type != "raster" {
		t.Errorf("the mark is %q, want raster", m.Type)
	}
	if m.Resample != "max" {
		t.Errorf("the raster wrote %q, want max", m.Resample)
	}
	if m.Bins != 0 || len(m.Levels) != 0 || m.Bands != 0 {
		t.Errorf("the raster carries a histogram's bins, a contour's levels or a fold's bands: %+v", m)
	}

	// The default is not written down: an omitted reduction reads back as the
	// one that never shows a number nobody measured.
	if m := of(geom.Raster(src, geom.X("t"), geom.Y("hz"), geom.Z("db"))); m.Resample != "" {
		t.Errorf("a raster that was told nothing wrote %q, want nothing", m.Resample)
	}
}

// A document naming a reduction this library does not have reads back as the
// default rather than as an error, which is what every other closed family
// here does with a word it does not know.
func TestAnUnknownResamplingReadsBackAsNearest(t *testing.T) {
	src := measuredField(8, 8)
	s, err := spec.Of(spec.Chart{
		Width: 480, Height: 320, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Layers: []geom.Geom{geom.Raster(src, geom.X("t"), geom.Y("hz"), geom.Z("db"))},
	})
	if err != nil {
		t.Fatal(err)
	}
	s.Layer[0].Mark.Resample = "bicubic"
	c, err := s.Chart()
	if err != nil {
		t.Fatal(err)
	}
	d, ok := geom.Describe(c.Layers[0])
	if !ok {
		t.Fatal("the rebuilt raster does not describe itself")
	}
	if d.Resample != geom.Nearest {
		t.Errorf("an unknown reduction read back as %v, want Nearest", d.Resample)
	}
}

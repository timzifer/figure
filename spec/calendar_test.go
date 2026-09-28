package spec_test

import (
	"testing"
	"time"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
)

// A calendar is not written down — its folds are — but the week start it gave
// the axis is, and reads back.
func TestAWeekStartSurvivesTheDocument(t *testing.T) {
	c := chartOf(geom.Line(longTable(), geom.X("t"), geom.Y("v")))
	c.X = scale.Time(scale.WeekStart(time.Sunday))
	s, err := spec.Of(c)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := spec.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	out, err := back.Chart()
	if err != nil {
		t.Fatal(err)
	}
	if d := out.X.(scale.Describer).Describe(); d.WeekStart != "sunday" {
		t.Errorf("read back with week start %q\n%s", d.WeekStart, b)
	}
}

package geom_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// trial is two arms of weeks-to-relapse, with an event column where 0 means
// the patient was still in remission when last seen.
func trial() data.Source {
	return data.NewTable().
		Float64("weeks", []float64{6, 6, 7, 10, 13, 16, 22, 23, 1, 2, 3, 4, 5, 8, 8, 11}).
		Float64("relapsed", []float64{1, 0, 1, 0, 1, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1, 1}).
		String("arm", []string{"6-MP", "6-MP", "6-MP", "6-MP", "6-MP", "6-MP", "6-MP", "6-MP",
			"placebo", "placebo", "placebo", "placebo", "placebo", "placebo", "placebo", "placebo"})
}

func TestASurvivalCurveStepsDownAtEventsOnly(t *testing.T) {
	src := data.NewTable().
		Float64("t", []float64{1, 2, 3, 4}).
		Float64("e", []float64{1, 0, 1, 1})
	g := geom.Survival(src, geom.X("t"), geom.Event("e"))
	rec := build(t, g, scale.Linear(), scale.Linear())

	lines := rec.Filter("Polyline")
	if len(lines) != 1 {
		t.Fatalf("%d polylines, want 1", len(lines))
	}
	pts := lines[0].Points
	// Start at (0, 1); along to 1 and down to 3/4; along to 2 (censored, no
	// drop); along to 3 and down to 3/8; along to 4 and down to 0.
	if n := len(pts); n != 8 {
		t.Fatalf("%d vertices %v, want 8", n, pts)
	}
	_, f := frameWith(t, geom.Survival(src, geom.X("t"), geom.Event("e")), scale.Linear(), scale.Linear())
	want := []struct{ x, y float64 }{{0, 1}, {1, 1}, {1, 0.75}, {2, 0.75}, {3, 0.75}, {3, 0.375}, {4, 0.375}, {4, 0}}
	for i, w := range want {
		if math.Abs(float64(pts[i].X-f.X.Map(w.x))) > 0.01 || math.Abs(float64(pts[i].Y-f.Y.Map(w.y))) > 0.01 {
			t.Errorf("vertex %d at %v, want (%v, %v)", i, pts[i], w.x, w.y)
		}
	}
}

func TestTheBandAndTheTicksAreOptIns(t *testing.T) {
	plain := build(t, geom.Survival(trial(), geom.X("weeks"), geom.Event("relapsed")), scale.Linear(), scale.Linear())
	if plain.Count("FillPath") != 0 || plain.Count("Markers") != 0 {
		t.Errorf("a plain curve drew %d fills and %d marker sets", plain.Count("FillPath"), plain.Count("Markers"))
	}
	rec := build(t, geom.Survival(trial(), geom.X("weeks"), geom.Event("relapsed"),
		geom.Confidence(0.95), geom.CensorMarks(true)), scale.Linear(), scale.Linear())
	if rec.Count("FillPath") != 1 {
		t.Errorf("%d bands, want 1", rec.Count("FillPath"))
	}
	marks := rec.Filter("Markers")
	// Censored at weeks 4, 6, 10 and 22: one tick each.
	if len(marks) != 1 || len(marks[0].Points) != 4 {
		t.Fatalf("censor ticks %v, want one set of four", marks)
	}
	if marks[0].Marker != ir.MarkerPlus {
		t.Errorf("censor ticks drawn as %v, want a plus", marks[0].Marker)
	}
	for _, p := range rec.Filter("FillPath")[0].Path.Pts {
		if math.IsNaN(float64(p.X)) || math.IsNaN(float64(p.Y)) {
			t.Fatal("a NaN reached the backend in the band")
		}
	}
}

func TestTwoArmsAreTwoCurves(t *testing.T) {
	g := geom.Survival(trial(), geom.X("weeks"), geom.Event("relapsed"), geom.GroupBy("arm"))
	rec, f := frameWith(t, g, scale.Linear(), scale.Linear())
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if n := rec.Count("Polyline"); n != 2 {
		t.Errorf("%d curves, want one per arm", n)
	}
	if n := len(g.(geom.Legender).Legends(f)); n != 2 {
		t.Errorf("%d legend entries, want one per arm", n)
	}
	if lo, hi := f.Y.Domain(); lo > 0 || hi < 1 {
		t.Errorf("Y axis %v..%v, want the whole probability", lo, hi)
	}
}

func TestASurvivalLayerWithoutAnEventColumnCountsEveryRow(t *testing.T) {
	src := data.NewTable().Float64("t", []float64{1, 2})
	rec := build(t, geom.Survival(src, geom.X("t")), scale.Linear(), scale.Linear())
	pts := rec.Filter("Polyline")[0].Points
	last := pts[len(pts)-1]
	_, f := frameWith(t, geom.Survival(src, geom.X("t")), scale.Linear(), scale.Linear())
	if math.Abs(float64(last.Y-f.Y.Map(0))) > 0.01 {
		t.Errorf("the curve ends at %v, want at zero: with no event column every row is an event", last)
	}
}

func TestASurvivalLayerSurvivesItsDesc(t *testing.T) {
	g := geom.Survival(trial(), geom.X("weeks"), geom.Event("relapsed"), geom.Confidence(0.9), geom.CensorMarks(true))
	d := g.(geom.Describer).Describe()
	if d.Mark != geom.MarkSurvival || d.EventCol != "relapsed" || d.Confidence != 0.9 || !d.CensorMarks {
		t.Errorf("Desc = %+v", d)
	}
	if _, err := geom.FromDesc(d); err != nil {
		t.Error(err)
	}
}

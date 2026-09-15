package spec_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

// schedule is a plan and the constraints over it, as two tables.
func schedule() (tasks, links *data.Table) {
	tasks = data.NewTable().
		String("id", []string{"a", "b", "c"}).
		Float64("start", []float64{0, 10, 30}).
		Float64("end", []float64{10, 20, 40}).
		Float64("lane", []float64{0, 1, 2}).
		Float64("done", []float64{0.5, 0, 1})
	links = data.NewTable().
		String("before", []string{"a", "b"}).
		String("after", []string{"b", "c"}).
		String("kind", []string{"fs", "ff"}).
		String("path", []string{"critical", "slack"})
	return tasks, links
}

// TestAScheduleSurvivesTheRoundTrip. The bars carry a progress column and the
// arrows carry a whole second table, which is the one layer in this dialect
// that has one.
func TestAScheduleSurvivesTheRoundTrip(t *testing.T) {
	tasks, links := schedule()
	c := spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Layers: []geom.Geom{
			geom.Rect(tasks, geom.X("start"), geom.X2("end"), geom.Y("lane"),
				geom.ProgressBy("done"), geom.Color(palette.Blue)),
			geom.Depends(tasks, links,
				geom.X("start"), geom.X2("end"), geom.Y("lane"),
				geom.KeyBy("id"), geom.From("before"), geom.To("after"),
				geom.LinkBy("kind"), geom.Link(geom.StartToStart),
				geom.ColorBy("path", scale.Named(map[string]ir.Color{
					"critical": palette.Red,
					"slack":    palette.Gray,
				}))),
		},
	}
	back := roundTrip(t, c)
	want, got := draw(t, c), draw(t, back)
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Error("a schedule did not survive the round trip")
	}

	d, ok := geom.Describe(back.Layers[1])
	if !ok {
		t.Fatal("the dependency layer read back as something that cannot describe itself")
	}
	if d.Links == nil || d.Links.Len() != links.Len() {
		t.Fatalf("the link table did not survive: %v", d.Links)
	}
	if d.Linkage != geom.StartToStart {
		t.Errorf("the layer's linkage read back as %v, want start-to-start", d.Linkage)
	}
	if d.LinkCol != "kind" {
		t.Errorf("the per-row linkage column read back as %q", d.LinkCol)
	}
	if d.ProgressCol != "" {
		t.Errorf("the arrows read back with a progress column %q", d.ProgressCol)
	}
	if p, _ := geom.Describe(back.Layers[0]); p.ProgressCol != "done" {
		t.Errorf("the bars read back with progress column %q, want %q", p.ProgressCol, "done")
	}
}

// TestALinkTableIsWrittenBesideTheLayerThatReadsIt, and never hoisted: two
// dependency layers over one plan are two sets of constraints, and a document
// that shared them would say they were one.
func TestALinkTableIsWrittenBesideTheLayerThatReadsIt(t *testing.T) {
	tasks, links := schedule()
	c := spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Layers: []geom.Geom{
			geom.Rect(tasks, geom.X("start"), geom.X2("end"), geom.Y("lane")),
			geom.Depends(tasks, links, geom.X("start"), geom.X2("end"), geom.Y("lane"),
				geom.KeyBy("id"), geom.From("before"), geom.To("after")),
		},
	}
	s, err := spec.Of(c)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var doc struct {
		Data  *json.RawMessage `json:"data"`
		Layer []struct {
			Links *json.RawMessage `json:"links"`
		} `json:"layer"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// Both layers read the task table, so it is hoisted to the top level and
	// neither layer carries it. The link table belongs to the second.
	if doc.Data == nil {
		t.Error("the task table was not hoisted; both layers draw from it")
	}
	if doc.Layer[0].Links != nil {
		t.Error("the bars were written with a link table")
	}
	if doc.Layer[1].Links == nil {
		t.Fatal("the arrows were written without their link table")
	}
	if !strings.Contains(string(*doc.Layer[1].Links), "before") {
		t.Errorf("the link table does not name its ends: %s", *doc.Layer[1].Links)
	}
}

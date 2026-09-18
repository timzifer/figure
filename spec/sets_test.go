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

// The set charts through the document. A membership table is a bipartite edge
// list, so these read the channels the relational marks already spell — what is
// new is the ranking, which decides which columns the chart has and therefore
// has to survive the trip.

func members() *data.Table {
	return data.NewTable().
		String("who", []string{"a", "a", "b", "c", "c", "c", "d"}).
		String("what", []string{"mail", "drive", "mail", "mail", "drive", "chat", "chat"})
}

func TestEverySetMarkSurvivesTheRoundTrip(t *testing.T) {
	src := members()
	cases := []struct {
		name   string
		x, y   scale.Scale
		layer  geom.Geom
		theme_ theme.Theme
	}{
		{"intersections", scale.Ordinal(), scale.Linear(),
			geom.Intersections(src, geom.From("who"), geom.To("what")), theme.Light},
		{"top", scale.Ordinal(), scale.Linear(),
			geom.Intersections(src, geom.From("who"), geom.To("what"), geom.Top(2)), theme.Light},
		{"appearance", scale.Ordinal(), scale.Linear(),
			geom.Intersections(src, geom.From("who"), geom.To("what"), geom.Order(geom.OrderAppearance)), theme.Light},
		{"set-matrix", scale.Ordinal(), scale.Ordinal(),
			geom.SetMatrix(src, geom.From("who"), geom.To("what"), geom.Top(3)), theme.Light},
		{"set-sizes", scale.Linear(), scale.Ordinal(),
			geom.SetSizes(src, geom.From("who"), geom.To("what")), theme.Light},
		{"set-sizes reversed", scale.Linear(scale.Zero(), scale.Reverse()), scale.Ordinal(),
			geom.SetSizes(src, geom.From("who"), geom.To("what")), theme.Light},
		{"venn", scale.Linear(), scale.Linear(),
			geom.Venn(src, geom.From("who"), geom.To("what")), theme.Light},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := spec.Chart{
				Width: 400, Height: 300, DPR: 1, Theme: tc.theme_,
				X: tc.x, Y: tc.y, Layers: []geom.Geom{tc.layer},
			}
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				s, _ := spec.Of(c)
				b, _ := s.Marshal()
				t.Errorf("the %s layer did not survive the round trip\n%s", tc.name, b)
			}
		})
	}
}

// The names are figure's own, and a document is where a reader meets them.
func TestASetLayerWritesItsOwnMarkNames(t *testing.T) {
	src := members()
	for _, tc := range []struct {
		layer geom.Geom
		x, y  scale.Scale
		want  string
	}{
		{geom.Intersections(src, geom.From("who"), geom.To("what"), geom.Top(5)), scale.Ordinal(), scale.Linear(), "intersections"},
		{geom.SetMatrix(src, geom.From("who"), geom.To("what")), scale.Ordinal(), scale.Ordinal(), "set-matrix"},
		{geom.SetSizes(src, geom.From("who"), geom.To("what")), scale.Linear(), scale.Ordinal(), "set-sizes"},
		{geom.Venn(src, geom.From("who"), geom.To("what")), scale.Linear(), scale.Linear(), "venn"},
	} {
		s, err := spec.Of(spec.Chart{
			Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
			X: tc.x, Y: tc.y, Layers: []geom.Geom{tc.layer},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Layer[0].Mark.Type; got != tc.want {
			t.Errorf("mark type %q, want %q", got, tc.want)
		}
		b, err := s.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		doc := string(b)
		for _, want := range []string{`"from": {`, `"to": {`} {
			if !strings.Contains(doc, want) {
				t.Errorf("the %s document does not name the membership channels:\n%s", tc.want, doc)
			}
		}
		// A set chart has no positional channels of its own: the columns are
		// the mark's own counting, not something a row is at.
		if enc := s.Layer[0].Encoding; enc.X != nil || enc.Y != nil {
			t.Errorf("the %s layer wrote positional channels: x=%v y=%v", tc.want, enc.X, enc.Y)
		}
	}
}

// A hand-written document reads, which is what proves the decoder's source gate
// covers these marks — an UpSet that read back with no table would be refused
// by geom.FromDesc, and a round trip over a built chart would never show it.
func TestAHandWrittenUpSetSpecReads(t *testing.T) {
	doc := `{
	  "width": 500, "height": 320,
	  "data": {"values": [
	    {"who": "a", "what": "mail"},
	    {"who": "a", "what": "drive"},
	    {"who": "b", "what": "mail"}
	  ]},
	  "encoding": {"x": {"scale": {"type": "ordinal"}}, "y": {"scale": {"type": "linear"}}},
	  "layer": [
	    {"mark": {"type": "intersections", "top": 4},
	     "encoding": {"from": {"field": "who"}, "to": {"field": "what"}}}
	  ]
	}`
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Chart()
	if err != nil {
		t.Fatalf("a hand-written UpSet was refused: %v", err)
	}
	if len(c.Layers) != 1 {
		t.Fatalf("the document read back as %d layers", len(c.Layers))
	}
	d, ok := geom.Describe(c.Layers[0])
	if !ok || d.Mark != geom.MarkIntersections {
		t.Fatalf("the layer read back as %q", d.Mark)
	}
	if d.From != "who" || d.To != "what" || d.Top != 4 {
		t.Errorf("the layer read back as %+v, want who/what and Top 4", d)
	}
}

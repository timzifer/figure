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

func TestAProbabilityAxisSurvivesTheRoundTrip(t *testing.T) {
	src := data.NewTable().Float64("t", []float64{120, 340, 410, 900, 1500, 2300})
	for _, link := range []scale.Link{scale.Probit, scale.Logit, scale.CLogLog, scale.Gumbel} {
		name, _ := scale.LinkName(link)
		t.Run(name, func(t *testing.T) {
			c := spec.Chart{
				Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
				X: scale.Log(), Y: scale.Probability(link, scale.ProbabilityNumberFormat("#.1%")),
				Layers: []geom.Geom{geom.ECDF(src, geom.X("t"))},
			}
			want, got := draw(t, c), draw(t, roundTrip(t, c))
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				t.Errorf("the %s axis did not survive the round trip", name)
			}
		})
	}
}

func TestAProbabilityAxisWithNoLinkIsProbit(t *testing.T) {
	doc := `{"width": 300, "height": 200,
		"encoding": {"y": {"type": "quantitative", "scale": {"type": "probability"}}},
		"data": {"values": [{"t": 1}, {"t": 2}]},
		"layer": [{"mark": {"type": "ecdf"}, "encoding": {"x": {"field": "t"}}}]}`
	c, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	chart, err := c.Chart()
	if err != nil {
		t.Fatal(err)
	}
	d, _ := scale.Describe(chart.Y)
	if d.Kind != scale.KindProbability || d.Link != "probit" {
		t.Errorf("Y = %+v, want probit probability paper", d)
	}
}

type ownLink struct{}

func (ownLink) Apply(p float64) float64   { return p }
func (ownLink) Unapply(z float64) float64 { return z }

func TestAProbabilityAxisWithALinkOfItsOwnDeclinesToBeWrittenDown(t *testing.T) {
	c := spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		Y:      scale.Probability(ownLink{}),
		Layers: []geom.Geom{geom.ECDF(table(), geom.X("x"))},
	}
	if _, err := spec.Of(c); err == nil || !strings.Contains(err.Error(), "link") {
		t.Errorf("Of = %v, want an error naming the link", err)
	}
}

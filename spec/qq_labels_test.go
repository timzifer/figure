package spec_test

import (
	"reflect"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

func TestQQAndLabelPlacementRoundTrip(t *testing.T) {
	src := data.NewTable().Float64("x", []float64{1, 3, 7}).Float64("y", []float64{1, 3, 7}).String("label", []string{"a", "b", "c"})
	for _, g := range []geom.Geom{
		geom.QQ(src, geom.X("x"), geom.Size(7), geom.Shape(ir.MarkerDiamond)),
		geom.Text(src, geom.X("x"), geom.Y("y"), geom.TextBy("label"), geom.AvoidOverlap(true)),
	} {
		c := spec.Chart{Width: 500, Height: 350, DPR: 1, Theme: theme.Light, X: scale.Linear(), Y: scale.Linear(), Layers: []geom.Geom{g}}
		back := roundTrip(t, c)
		if !reflect.DeepEqual(draw(t, c), draw(t, back)) {
			t.Fatal("round trip changed drawing")
		}
		d, _ := geom.Describe(g)
		bd, _ := geom.Describe(back.Layers[0])
		if d.Mark != bd.Mark || d.AvoidOverlap != bd.AvoidOverlap {
			t.Fatal("round trip lost configuration")
		}
	}
}

func TestHandwrittenQQSpecNeedsOnlyTheSampleColumn(t *testing.T) {
	s, err := spec.Parse([]byte(`{"data":{"values":[{"v":1},{"v":3}]},"layer":[{"mark":{"type":"qq"},"encoding":{"x":{"field":"v"}}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Chart()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Layers) != 1 {
		t.Fatal("lost sample")
	}
	if err := c.Layers[0].Train(geom.Training{X: scale.Linear(), Y: scale.Linear()}); err != nil {
		t.Fatal(err)
	}
}

func TestCalloutAndFloorRoundTrip(t *testing.T) {
	src := data.NewTable().Float64("lo", []float64{0, 97}).Float64("hi", []float64{97, 100}).
		Float64("r0", []float64{0, 0}).Float64("r1", []float64{1, 1}).String("label", []string{"a", "b"})
	g := geom.Text(src, geom.X("r0"), geom.X2("r1"), geom.Y("lo"), geom.Y2("hi"), geom.TextBy("label"),
		geom.Callout(true), geom.MinFontSize(8), geom.Wrap(true), geom.Slide(false))
	c := spec.Chart{Width: 500, Height: 350, DPR: 1, Theme: theme.Light, X: scale.Linear(), Y: scale.Linear(), Layers: []geom.Geom{g}}
	back := roundTrip(t, c)
	bd, _ := geom.Describe(back.Layers[0])
	if !bd.Callout || bd.MinFontSize != 8 || !bd.Wrap || !bd.Pinned {
		t.Fatalf("read back as callout %v, floor %v, wrap %v, pinned %v", bd.Callout, bd.MinFontSize, bd.Wrap, bd.Pinned)
	}

	// A document that says nothing about sliding slides.
	s, err := spec.Parse([]byte(`{"data":{"values":[{"x":0,"x2":1,"y":0,"y2":1,"t":"a"}]},"layer":[{"mark":{"type":"text"},"encoding":{"x":{"field":"x"},"x2":{"field":"x2"},"y":{"field":"y"},"y2":{"field":"y2"},"text":{"field":"t"}}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	sc, err := s.Chart()
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := geom.Describe(sc.Layers[0]); d.Pinned {
		t.Error("a document that omits slide reads back pinned")
	}
}

package render_test

import (
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

// A layer that writes labels outside its slices is clipped to the panel
// rectangle rather than to the disc a polar coord clips the rest of the data
// to, and the layer after it is back inside the disc.
func TestALayerThatOverhangsIsClippedToThePanel(t *testing.T) {
	src := data.NewTable().
		Float64("r0", []float64{0, 0}).Float64("r1", []float64{1, 1}).
		Float64("lo", []float64{0, 99}).Float64("hi", []float64{99, 100}).
		String("label", []string{"most", "a sliver"})
	opts := []geom.Option{geom.X("r0"), geom.X2("r1"), geom.Y("lo"), geom.Y2("hi")}
	c := chart(
		geom.Rect(src, opts...),
		geom.Text(src, append(opts, geom.TextBy("label"), geom.Callout(true))...),
		geom.Rect(src, opts...),
	)
	c.X, c.Y = scale.Linear(), scale.Linear()
	c.Coord = coord.Polar(coord.Theta(coord.FromY))
	rec := draw(t, c)

	pushes := rec.Filter("Push")
	if len(pushes) != 3 || rec.Count("Pop") != 3 || rec.Depth != 0 {
		t.Fatalf("want the disc, the rectangle and the disc again, balanced; got %v", rec.Ops())
	}
	disc, rect := pushes[0].Path, pushes[1].Path
	if _, ok := disc.AsRect(); ok {
		t.Error("the data is clipped to a rectangle under a polar coord")
	}
	r, ok := rect.AsRect()
	d := pushes[0].ClipRect
	if !ok || r.Min.X > d.Min.X || r.Min.Y > d.Min.Y || r.Max.X < d.Max.X || r.Max.Y < d.Max.Y {
		t.Errorf("the overhanging layer is clipped to %v, want the panel round the disc %v", r, d)
	}
	if _, ok := pushes[2].Path.AsRect(); ok {
		t.Error("the layer after the overhanging one is not back inside the disc")
	}
}

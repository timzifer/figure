package figure_test

import (
	"strings"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/theme"
)

func candleDays(opts ...figure.Option) *figure.Plot {
	src := figure.NewTable().
		Float64("t", []float64{0, 1, 2}).
		Float64("o", []float64{10, 12, 13}).
		Float64("h", []float64{13, 13, 14}).
		Float64("l", []float64{9, 10, 12}).
		Float64("c", []float64{12, 11, 13}).
		Float64("v", []float64{5, 6, 7})
	p := figure.New(append([]figure.Option{figure.Size(500, 300)}, opts...)...)
	p.Add(geom.Candle(src, geom.X("t"), geom.OHLC("o", "h", "l", "c")))
	p.Track(figure.Bottom, figure.TrackSize(60)).
		Add(geom.Bar(src, geom.X("t"), geom.Y("v"), geom.DirectionBy("o", "c")))
	return p
}

// A volume track under candles lists the directions once: the legend merges
// entries by label, and both layers take the same default labels.
func TestAVolumeTrackAddsNothingToTheCandlesLegend(t *testing.T) {
	svg := renderString(t, candleDays(figure.Legend(true)))
	for _, label := range []string{">rising<", ">falling<"} {
		if n := strings.Count(svg, label); n != 1 {
			t.Errorf("%s appears %d times in the legend, want once", label, n)
		}
	}
}

// The direction's colours are the theme's, bound per render: a theme with
// other rising and falling colours paints the bars in them.
func TestADirectionTakesTheThemesColours(t *testing.T) {
	th := theme.Light
	th.Rising, th.Falling = ir.RGB(0x11, 0x22, 0x33), ir.RGB(0x44, 0x55, 0x66)
	src := figure.NewTable().
		Float64("t", []float64{0, 1}).Float64("o", []float64{1, 5}).
		Float64("c", []float64{2, 4}).Float64("v", []float64{3, 3})
	for _, tc := range []struct {
		th       theme.Theme
		up, down string
	}{
		{th, "#112233", "#445566"},
		{theme.Light, "#0072b2", "#d55e00"},
	} {
		p := figure.New(figure.Size(300, 200), figure.Theme(tc.th))
		p.Add(geom.Bar(src, geom.X("t"), geom.Y("v"), geom.DirectionBy("o", "c")))
		svg := strings.ToLower(renderString(t, p))
		if !strings.Contains(svg, tc.up) || !strings.Contains(svg, tc.down) {
			t.Errorf("the bars are not painted %s and %s", tc.up, tc.down)
		}
	}
}

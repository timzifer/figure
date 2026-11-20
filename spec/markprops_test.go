package spec

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/markprops"
	"github.com/timzifer/figure/ir"
)

// propOption names the geom option a property of the mark object comes from.
// A property two options write maps to the one a reader would look up.
var propOption = map[string]string{
	"align": "Align", "angle": "Rotate", "avoidOverlap": "AvoidOverlap",
	"bandHeight": "BandHeight", "bands": "Bands", "bandwidth": "Bandwidth",
	"barWidth": "BarWidth", "baseline": "Align", "binEnd": "BinRange",
	"binStart": "BinRange", "bins": "Bins", "branch": "Branches",
	"budget": "Budget", "callout": "Callout", "candleStyle": "CandleStyle",
	"caps": "Caps", "censorMarks": "CensorMarks", "closed": "Closed",
	"color": "Color", "confidence": "Confidence", "corner": "Corner",
	"decimate": "Decimate", "densityCells": "DensityCells",
	"direction": "Direction", "dodge": "Dodge", "elide": "Elide",
	"explode": "Explode", "extend": "Extend", "extent": "Whisker",
	"extrude": "Extrude", "falling": "Falling", "fallingLabel": "Falling",
	"fill": "Fill", "fontSize": "FontSize", "gradient": "Gradient",
	"hatch": "Hatch", "hatchColor": "HatchColor",
	"hatchDensity": "HatchDensity", "hatchWidth": "HatchWidth",
	"hollow": "Hollow", "inset": "Inset", "interpolate": "Curve",
	"labelLevels": "LabelLevels", "levelCount": "LevelCount",
	"levels": "Levels", "link": "Link", "method": "Smooth",
	"minFontSize": "MinFontSize", "missing": "OnMissing",
	"opacity": "Opacity", "order": "Order", "orientation": "Orient",
	"origin": "Baseline", "outliers": "Outliers", "overlap": "Overlap",
	"padding": "Padding", "resample": "Resample", "rising": "Rising",
	"risingLabel": "Rising", "rule": "Rule", "shape": "Shape",
	"size": "Size", "slide": "Slide", "span": "Span",
	"strokeDash": "Dash", "strokeWidth": "Width", "tension": "Tension",
	"thickness": "Thickness", "top": "Top", "wrap": "Wrap",
}

// probe sets every property option to a value that is not its default, so
// that an encoder which writes a property only when it was chosen writes it.
// GroupBy is a column rather than a property; it is set because a grouped
// layer is the only one that writes an ordering.
func probe() []geom.Option {
	c := ir.RGB(0x12, 0x34, 0x56)
	return []geom.Option{
		geom.GroupBy("g"),
		geom.Align(ir.AlignEnd, ir.AlignTop), geom.AvoidOverlap(true),
		geom.BandHeight(2), geom.Bands(3), geom.Bandwidth(0.5),
		geom.BarWidth(0.5), geom.Baseline(1), geom.BinRange(0, 1),
		geom.Bins(10), geom.Branches(geom.Straight), geom.Budget(100),
		geom.Callout(true), geom.CandleStyle(geom.Ticks), geom.Caps(false),
		geom.CensorMarks(true), geom.Closed(true), geom.Color(c),
		geom.Confidence(0.9), geom.Corner(2), geom.Curve(geom.CurveMonotone),
		geom.Dash(3, 1), geom.Decimate(geom.NoDecimation),
		geom.DensityCells(4), geom.Direction(geom.SincePrevious),
		geom.Dodge(2), geom.Elide(true), geom.Explode(0.2),
		geom.Extend(false), geom.Extrude(true), geom.Falling(c, "down"),
		geom.Fill(c), geom.FontSize(12), geom.Gradient(c),
		geom.Hatch(ir.HatchDiagonal), geom.HatchColor(c),
		geom.HatchDensity(2), geom.HatchWidth(1.5), geom.Hollow(true),
		geom.Inset(1), geom.LabelLevels(true), geom.LevelCount(5),
		geom.Levels(1, 2), geom.Link(geom.StartToStart),
		geom.MinFontSize(6), geom.OnMissing(geom.Interpolate),
		geom.Opacity(0.5), geom.Order(geom.OrderValue),
		geom.Orient(geom.Horizontal), geom.Outliers(false), geom.Overlap(2),
		geom.Padding(0.1), geom.Resample(geom.Mean), geom.Rising(c, "up"),
		geom.Rotate(0.5), geom.Rule(false), geom.Shape(ir.MarkerSquare),
		geom.Size(5), geom.Slide(false), geom.Smooth(geom.LinearFit),
		geom.Span(0.5), geom.Steps(geom.StepMid), geom.Tension(0.5),
		geom.Thickness(0.2), geom.Top(5), geom.Whisker(2), geom.Wrap(true),
		geom.Width(2),
	}
}

// readsOf is what the encoder writes on the mark object for m, as the
// options it came from.
func readsOf(t *testing.T, m geom.Mark) []string {
	t.Helper()
	d := geom.Configure(probe()...)
	d.Mark = m
	var out Mark
	writeMarkProps(&out, d)
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var props map[string]any
	if err := json.Unmarshal(b, &props); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for k := range props {
		if k == "type" {
			continue
		}
		opt, ok := propOption[k]
		if !ok {
			t.Fatalf("%s writes the property %q, which propOption does not map to an option", m, k)
		}
		if k == "interpolate" && m == geom.MarkStep {
			opt = "Steps"
		}
		seen[opt] = true
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// The table geom's documentation is rendered from must say what the encoder
// does: a mark the table says reads an option the encoder never writes, or
// the other way round, is documentation describing a different library.
func TestTheMarkTableIsWhatTheEncoderWrites(t *testing.T) {
	if len(markprops.Reads) == 0 {
		t.Fatal("markprops.Reads is empty")
	}
	var got []string
	for name := range markprops.Reads {
		got = append(got, name)
	}
	sort.Strings(got)
	for _, name := range got {
		want := markprops.Reads[name]
		have := readsOf(t, geom.Mark(name))
		if !slices.Equal(have, want) {
			t.Errorf("%s: the encoder writes %s\n\tthe table says %s", name, strings.Join(have, " "), strings.Join(want, " "))
		}
	}
}

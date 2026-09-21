package render_test

import (
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/render"
	"github.com/timzifer/figure/theme"
)

// A coord may raise a grid family of its own — one with no tick behind it —
// and that family carries its own labels. ADR 0070 is the record; a ternary
// chart's third component is the first customer.

// familyCoord is Cartesian with one family raised over it, at the levels it
// was given.
type familyCoord struct {
	coord.Coord
	at   []ir.Point
	text []string
}

func withFamily(at []ir.Point, text []string) *familyCoord {
	return &familyCoord{Coord: coord.Cartesian(), at: at, text: text}
}

func (f *familyCoord) Frame(fr coord.Framing) coord.Coord {
	return &familyCoord{Coord: f.Coord.Frame(fr), at: f.at, text: f.text}
}

func (f *familyCoord) Furniture(dst *coord.Furniture, req coord.FurnitureRequest) {
	f.Coord.Furniture(dst, req)
	dst.Families = append(dst.Families, coord.Family{Name: "third"})
	fam := &dst.Families[len(dst.Families)-1]
	for i, p := range f.at {
		var s coord.Shape
		s.Pts = []ir.Point{p, {X: p.X + 40, Y: p.Y}}
		fam.Lines = append(fam.Lines, s)
		fam.Labels = append(fam.Labels, coord.Label{At: p, H: ir.AlignCenter, V: ir.AlignMiddle})
		fam.Text = append(fam.Text, f.text[i])
	}
}

// familyChart is a line chart whose coord raises one family in the middle of
// the panel, well clear of either axis's labels.
func familyChart(text ...string) render.Chart {
	at := make([]ir.Point, len(text))
	for i := range text {
		at[i] = ir.Point{X: 200, Y: float32(120 + 40*i)}
	}
	c := chart(line())
	c.Coord = withFamily(at, text)
	return c
}

// A family is drawn in the grid's ink and labelled in the tick font's, which
// is what makes it a ladder rather than an annotation.
func TestAFamilyIsStrokedWithTheGridAndLabelledWithTheTicks(t *testing.T) {
	rec := draw(t, familyChart("A", "B"))

	var lines int
	for _, c := range rec.Calls {
		if c.Op == "Polyline" && len(c.Points) == 2 && c.Points[0].X == 200 &&
			c.Stroke.Color == theme.Light.GridColor {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("%d family lines drawn in grid ink, want 2", lines)
	}
	for _, want := range []string{"A", "B"} {
		found := false
		for _, c := range rec.Calls {
			if c.Op == "Text" && c.Text.Text == want {
				found = true
				if c.Text.Color != theme.Light.TickColor {
					t.Errorf("the family label %q is drawn in %v, want the tick ink", want, c.Text.Color)
				}
			}
		}
		if !found {
			t.Errorf("the family label %q was not drawn", want)
		}
	}
}

// The panel's grid is the two questions a family answers to. A track that hides
// its grid hides the family with it — and the per-axis theme flags do not,
// because a family is neither axis.
func TestHidingAPanelsGridHidesItsFamilies(t *testing.T) {
	c := familyChart("A")
	c.Panels = []render.Panel{{X: c.X, Y: c.Y, Layers: c.Layers, ShowX: true, ShowY: true, HideGrid: true}}
	c.Rows, c.Cols = 1, 1
	rec := draw(t, c)

	for _, call := range rec.Calls {
		if call.Op == "Polyline" && len(call.Points) == 2 && call.Points[0].X == 200 &&
			call.Stroke.Color == theme.Light.GridColor {
			t.Error("a panel that hides its grid drew a family line")
		}
	}
}

// A family is the third reading of a chart and the axes are the first two, so
// where the numbers would collide the axis keeps its label.
func TestAFamilyLabelYieldsToAnAxisLabel(t *testing.T) {
	// One family label per axis tick position, placed exactly where that
	// axis's own label goes: every one of them has to lose.
	c := chart(line())
	rec := draw(t, c)
	var axis []ir.Point
	for _, call := range rec.Calls {
		if call.Op == "Text" && call.Text.Text != "" {
			axis = append(axis, call.Text.At)
		}
	}
	if len(axis) == 0 {
		t.Fatal("the plain chart wrote no tick labels")
	}
	text := make([]string, len(axis))
	for i := range text {
		text[i] = "collides"
	}
	c = chart(line())
	c.Coord = withFamily(axis, text)
	rec = draw(t, c)

	for _, call := range rec.Calls {
		if call.Op == "Text" && call.Text.Text == "collides" {
			t.Errorf("a family label was drawn at %v, on top of an axis label", call.Text.At)
		}
	}
}

// A theme that turns the ticks off has said something about the panel's own
// two axes. A coord whose families *are* the axes keeps their labels through
// it — otherwise the one chart whose axes carry nothing a reader wants
// numbered would lose every number it has to a switch that looks like it is
// about something else. ADR 0078 amends ADR 0070's gate; family *lines* have
// never answered to the per-axis theme flags for the same reason.
func TestFamiliesThatAreTheAxesKeepTheirLabelsWithTheTicksOff(t *testing.T) {
	for _, tc := range []struct {
		name string
		axes bool
		want bool
	}{
		{"a third reading beside two labelled axes", false, false},
		{"the panel's own axes", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := familyChart("A")
			c.Coord = &axisFamilyCoord{familyCoord: c.Coord.(*familyCoord), axes: tc.axes}
			c.Theme = theme.Light.With(theme.Ticks(false, false))
			rec := draw(t, c)

			drawn := false
			for _, call := range rec.Calls {
				drawn = drawn || (call.Op == "Text" && call.Text.Text == "A")
			}
			if drawn != tc.want {
				t.Errorf("the family label was drawn = %v, want %v", drawn, tc.want)
			}
		})
	}
}

// axisFamilyCoord is the family coord with a say about whether its families
// are the panel's axes.
type axisFamilyCoord struct {
	*familyCoord
	axes bool
}

func (f *axisFamilyCoord) Frame(fr coord.Framing) coord.Coord {
	return &axisFamilyCoord{familyCoord: f.familyCoord.Frame(fr).(*familyCoord), axes: f.axes}
}

func (f *axisFamilyCoord) Furniture(dst *coord.Furniture, req coord.FurnitureRequest) {
	f.familyCoord.Furniture(dst, req)
	dst.FamiliesAreTheAxes = f.axes
}

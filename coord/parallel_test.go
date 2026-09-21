package coord

import (
	"math"
	"reflect"
	"testing"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// parallelIn frames a coord of n axes in a rectangle and returns it with the
// dimensions, whose scales are the thing a mark maps a value through.
func parallelIn(t *testing.T, area ir.Rect, domains ...[2]float64) (Coord, []ParallelDim) {
	t.Helper()
	dims := make([]ParallelDim, 0, len(domains))
	for i, d := range domains {
		dims = append(dims, Dim(string(rune('a'+i)), scale.Linear(scale.Domain(d[0], d[1]))))
	}
	c := Parallel(dims...).Frame(Framing{Area: area, X: scale.Linear(), Y: scale.Linear()})
	return c, Dimensions(c)
}

// The axes stand at even divisions of the panel, the first on its left edge
// and the last on its right: a parallel panel is read across, so its outermost
// axes are its edges.
func TestParallelAxesDivideThePanel(t *testing.T) {
	area := ir.R(0, 0, 300, 200)
	c, _ := parallelIn(t, area, [2]float64{0, 1}, [2]float64{0, 1}, [2]float64{0, 1})

	for i, want := range []float32{0, 150, 300} {
		if got := c.Point(float32(i), 0).X; got != want {
			t.Errorf("axis %d stands at %v, want %v", i, got, want)
		}
	}
}

// A value is placed by its own dimension's scale, so two axes with different
// domains put their own maxima at the same height — which is the whole reason
// the form exists.
func TestEachAxisHasItsOwnDomain(t *testing.T) {
	area := ir.R(0, 0, 300, 200)
	c, dims := parallelIn(t, area, [2]float64{0, 10}, [2]float64{1000, 2000})

	top := c.Point(0, float32(dims[0].Scale.Map(10))).Y
	if other := c.Point(1, float32(dims[1].Scale.Map(2000))).Y; other != top {
		t.Errorf("ten on the first axis is at %v and two thousand on the second at %v, want both at the top", top, other)
	}
	if mid := c.Point(0, float32(dims[0].Scale.Map(5))).Y; math.Abs(float64(mid-100)) > 0.01 {
		t.Errorf("the middle of the first axis is at %v, want the middle of the panel", mid)
	}
}

// One axis stands in the middle rather than on the left edge: a panel with
// room for one axis and the axis pushed into a corner is a chart with a
// mistake in it.
func TestOneAxisStandsInTheMiddle(t *testing.T) {
	c, _ := parallelIn(t, ir.R(0, 0, 300, 200), [2]float64{0, 1})
	if got := c.Point(0, 0).X; got != 150 {
		t.Errorf("the only axis stands at %v, want the middle", got)
	}
}

// Frame returns the coord positioned in a panel rather than moving the
// receiver, because panels are built on separate goroutines.
func TestParallelFrameDoesNotMoveTheReceiver(t *testing.T) {
	p := Parallel(Dim("a", scale.Linear()), Dim("b", scale.Linear()))
	one := p.Frame(Framing{Area: ir.R(0, 0, 100, 100)})
	two := p.Frame(Framing{Area: ir.R(200, 0, 400, 100)})

	if a, b := one.Point(0, 0).X, two.Point(0, 0).X; a == b {
		t.Errorf("two panels put their first axis at the same place, %v", a)
	}
	if got := one.Point(1, 0).X; got != 100 {
		t.Errorf("the first panel moved when the second was framed: %v", got)
	}
}

// Every dimension raises a family: the axis line, a mark per level, the level
// labels in that dimension's own units, and the dimension's name.
func TestEachDimensionRaisesALabelledFamily(t *testing.T) {
	area := ir.R(0, 0, 300, 200)
	c, _ := parallelIn(t, area, [2]float64{0, 10}, [2]float64{0, 100})

	var fur Furniture
	c.Furniture(&fur, FurnitureRequest{Area: area, Metrics: Metrics{TickLen: 4, LabelPad: 2}})

	if len(fur.Families) != 2 {
		t.Fatalf("raised %d families, want one per dimension", len(fur.Families))
	}
	for i, fam := range fur.Families {
		if fam.Name != string(rune('a'+i)) {
			t.Errorf("family %d is called %q", i, fam.Name)
		}
		if len(fam.Lines) < 2 {
			t.Errorf("family %d has %d lines, want an axis and some levels", i, len(fam.Lines))
		}
		if len(fam.Labels) != len(fam.Text) {
			t.Errorf("family %d has %d labels for %d texts", i, len(fam.Labels), len(fam.Text))
		}
		if fam.Text[0] != fam.Name {
			t.Errorf("family %d's first label is %q, want the dimension's name", i, fam.Text[0])
		}
		found := false
		for _, s := range fam.Text[1:] {
			found = found || s != ""
		}
		if !found {
			t.Errorf("family %d wrote no level labels: %v", i, fam.Text)
		}
	}
}

// The panel's own ticks are silenced rather than left to be drawn: X is an
// axis index and Y a fraction of an axis, and a reader numbering either would
// be reading the coord's plumbing.
func TestTheParallelPanelWritesNoTicksOfItsOwn(t *testing.T) {
	area := ir.R(0, 0, 300, 200)
	c, _ := parallelIn(t, area, [2]float64{0, 1}, [2]float64{0, 1})

	var fur Furniture
	ticks := []scale.Tick{{Pos: 0, Label: "0"}, {Pos: 1, Label: "1"}}
	c.Furniture(&fur, FurnitureRequest{
		Area: area, Metrics: Metrics{TickLen: 4, LabelPad: 2},
		XTicks: ticks, YTicks: ticks,
	})
	for _, in := range append(append([]bool{}, fur.InX...), fur.InY...) {
		if in {
			t.Error("a panel tick was reported inside the panel and would be drawn")
		}
	}
	if !fur.AxesOverData {
		t.Error("the axes stand among the lines and must be written over them")
	}
	// The families are this panel's axes, which is what keeps their numbers
	// when a theme turns the panel's own ticks off — and a ternary chart's
	// third ladder, which is a third reading beside two labelled axes, says
	// the opposite.
	if !fur.FamiliesAreTheAxes {
		t.Error("the dimensions did not report themselves as the panel's axes")
	}
	var tern Furniture
	ticksAt := []scale.Tick{{Pos: 0, Label: "0"}, {Pos: 0.5, Label: "0.5"}}
	Ternary().Frame(Framing{Area: area, X: scale.Linear(), Y: scale.Linear()}).
		Furniture(&tern, FurnitureRequest{Area: area, XTicks: ticksAt, YTicks: ticksAt})
	if tern.FamiliesAreTheAxes {
		t.Error("a ternary chart's third ladder claimed to be the panel's axes")
	}
}

// The dimensions come back in the order they were declared, which is the order
// they are drawn and the order a mark's columns are matched in.
func TestDimensionsComeBackInOrder(t *testing.T) {
	c := Parallel(Dim("one", scale.Linear()), Dim("two", scale.Linear()))
	got := Dimensions(c)
	if len(got) != 2 || got[0].Name != "one" || got[1].Name != "two" {
		t.Fatalf("dimensions came back as %+v", got)
	}
	if Dimensions(Cartesian()) != nil {
		t.Error("a Cartesian panel reported dimensions of its own")
	}
	if len(Scales(got)) != 2 {
		t.Error("the scales of two dimensions are not two scales")
	}
}

// A parallel chart survives the round trip through its description: the axes
// are the coord's configuration, so a document that lost them would read back
// as a different chart.
func TestAParallelCoordDescribesItsDimensions(t *testing.T) {
	c := Parallel(Dim("mpg", scale.Linear()), Dim("hp", scale.Log()))
	d, ok := Describe(c)
	if !ok {
		t.Fatal("a parallel coord does not describe itself")
	}
	if d.Type != TypeParallel || len(d.Dims) != 2 {
		t.Fatalf("described as %+v", d)
	}
	if d.Dims[0].Name != "mpg" || d.Dims[1].Scale.Kind != scale.KindLog {
		t.Errorf("the dimensions came out as %+v", d.Dims)
	}
	back, err := FromDesc(d)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Describe(back)
	if !reflect.DeepEqual(again, d) {
		t.Errorf("round trip = %+v, want %+v", again, d)
	}
}

// Invert reads a device point back as which axis and how far up it, which is
// what a tooltip asks before it asks a dimension what the value was.
func TestParallelInvertsToAnAxisAndAFraction(t *testing.T) {
	area := ir.R(0, 0, 300, 200)
	c, _ := parallelIn(t, area, [2]float64{0, 1}, [2]float64{0, 1}, [2]float64{0, 1})

	x, y := c.Invert(ir.Point{X: 150, Y: 100})
	if x != 1 || y != 0.5 {
		t.Errorf("the middle of the panel inverts to (%v, %v), want the second axis at half height", x, y)
	}
}

// A reduction defined over pixel columns measures nothing here: a column of
// screen is an axis rather than a quantity.
func TestAParallelPanelDoesNotDecimate(t *testing.T) {
	c := Parallel(Dim("a", scale.Linear()))
	if c.Decimates() {
		t.Error("a parallel panel accepted a reduction defined over pixel columns")
	}
	if f, ok := c.(Fixed); !ok || !f.Fixed() {
		t.Error("a parallel panel reported that panning its own axes moves something")
	}
}

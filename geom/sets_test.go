package geom_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// subscriptions is the membership table the set marks read: one row per
// (customer, product) pair, which is the shape a join already has.
//
//	mail ∩ drive  a, g, j        mail         b, h
//	mail ∩ chat   e              drive        d
//	drive ∩ chat  i              chat         f
//	mail ∩ drive ∩ chat  c
func subscriptions() data.Source {
	var who, what []string
	add := func(name string, sets ...string) {
		for _, s := range sets {
			who, what = append(who, name), append(what, s)
		}
	}
	add("a", "mail", "drive")
	add("b", "mail")
	add("c", "mail", "drive", "chat")
	add("d", "drive")
	add("e", "mail", "chat")
	add("f", "chat")
	add("g", "mail", "drive")
	add("h", "mail")
	add("i", "drive", "chat")
	add("j", "mail", "drive")
	return data.NewTable().String("who", who).String("what", what)
}

// upsetFrame trains a set mark on the axes it needs and returns a frame to
// build it in. bars takes an ordinal X and a linear Y; the matrix takes two
// ordinal axes.
func upsetFrame(t *testing.T, g geom.Geom, x, y scale.Scale, w, h float32) (*irtest.Recorder, geom.Frame) {
	t.Helper()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("Train: %v", err)
	}
	area := ir.R(0, 0, w, h)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)
	c := coord.Cartesian().Frame(coord.Framing{Area: area, X: x, Y: y})
	return irtest.New(), geom.Frame{Area: area, X: x, Y: y, Coord: c, Theme: theme.Light}
}

// The two halves of an UpSet plot are two layers over one ranking, so given the
// same table and the same options they name the same columns in the same order.
// A chart where they did not would put a bar over the wrong dots, which is the
// one way this form can lie.
func TestAnUpSetsTwoHalvesAgreeOnTheirColumns(t *testing.T) {
	src := subscriptions()
	bars := geom.Intersections(src, geom.From("who"), geom.To("what"))
	dots := geom.SetMatrix(src, geom.From("who"), geom.To("what"))

	barsX, dotsX := scale.Ordinal(), scale.Ordinal()
	upsetFrame(t, bars, barsX, scale.Linear(), 400, 200)
	upsetFrame(t, dots, dotsX, scale.Ordinal(), 400, 120)

	a := barsX.(scale.Categorical).Labels()
	b := dotsX.(scale.Categorical).Labels()
	if strings.Join(a, "|") != strings.Join(b, "|") {
		t.Errorf("the bars name %v and the matrix names %v", a, b)
	}
	if len(a) != 7 {
		t.Errorf("the table has seven combinations in it, and the chart has %d columns: %v", len(a), a)
	}
}

// The biggest combination first, which is the form's own reading — and the
// column is named after the sets in it.
func TestIntersectionsRankTheBiggestFirst(t *testing.T) {
	x := scale.Ordinal()
	g := geom.Intersections(subscriptions(), geom.From("who"), geom.To("what"))
	upsetFrame(t, g, x, scale.Linear(), 400, 200)

	got := x.(scale.Categorical).Labels()
	if got[0] != "mail"+geom.SetSeparator+"drive" {
		t.Errorf("the first column is %q, want the three customers who have both", got[0])
	}
	if got[1] != "mail" {
		t.Errorf("the second column is %q, want the two who have only mail", got[1])
	}
}

// The table's own order is what [geom.Order] asks for, and it is the order the
// combinations were first met in going down the rows.
func TestOrderAppearanceKeepsTheTablesOrder(t *testing.T) {
	x := scale.Ordinal()
	g := geom.Intersections(subscriptions(), geom.From("who"), geom.To("what"),
		geom.Order(geom.OrderAppearance))
	upsetFrame(t, g, x, scale.Linear(), 400, 200)

	got := x.(scale.Categorical).Labels()
	want := "mail" + geom.SetSeparator + "drive"
	if got[0] != want || got[1] != "mail" {
		t.Errorf("columns %v, want %q then %q — the order a reader meets them in", got, want, "mail")
	}
	if last := got[len(got)-1]; last != "drive"+geom.SetSeparator+"chat" {
		t.Errorf("the last column is %q, want the one row i named", last)
	}
}

// Top keeps the biggest n columns and drops the tail, which is what makes the
// form readable on a real table.
func TestTopKeepsTheBiggestColumns(t *testing.T) {
	x := scale.Ordinal()
	g := geom.Intersections(subscriptions(), geom.From("who"), geom.To("what"), geom.Top(3))
	rec, f := upsetFrame(t, g, x, scale.Linear(), 400, 200)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	if got := x.(scale.Categorical).Labels(); len(got) != 3 {
		t.Errorf("Top(3) drew %d columns: %v", len(got), got)
	}
	if n := len(subpathsOf(t, rec.Filter("FillPath")[0])); n != 3 {
		t.Errorf("Top(3) filled %d bars, want three", n)
	}
}

// One dot per set per column, whether the set is in the combination or not: the
// muted ones are what make a column readable as a row of choices rather than as
// a scatter.
func TestTheMatrixDrawsADotPerSetPerColumn(t *testing.T) {
	g := geom.SetMatrix(subscriptions(), geom.From("who"), geom.To("what"))
	rec, f := upsetFrame(t, g, scale.Ordinal(), scale.Ordinal(), 400, 120)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	dots := 0
	for _, c := range rec.Filter("Markers") {
		dots += len(c.Points)
	}
	if want := 7 * 3; dots != want {
		t.Errorf("drew %d dots for seven columns over three sets, want %d", dots, want)
	}
}

// A column of two or more sets is joined, and a column of one is not: the line
// says "these together", and there is nothing to say about a single dot.
func TestTheMatrixJoinsTheSetsOfOneColumn(t *testing.T) {
	g := geom.SetMatrix(subscriptions(), geom.From("who"), geom.To("what"))
	rec, f := upsetFrame(t, g, scale.Ordinal(), scale.Ordinal(), 400, 120)
	if err := g.Build(rec, f); err != nil {
		t.Fatal(err)
	}
	strokes := rec.Filter("StrokePath")
	if len(strokes) != 1 {
		t.Fatalf("the connectors took %d calls, want one path for all of them", len(strokes))
	}
	// Four of the seven combinations hold more than one set.
	if got := len(subpathsOf(t, strokes[0])); got != 4 {
		t.Errorf("joined %d columns, want the four with more than one set in them", got)
	}
}

// Both marks need both ends of a membership row, and say which is which.
func TestASetMarkNeedsTheElementAndTheSet(t *testing.T) {
	for _, g := range []geom.Geom{
		geom.Intersections(subscriptions(), geom.From("who")),
		geom.SetMatrix(subscriptions(), geom.To("what")),
		geom.Venn(subscriptions(), geom.From("who")),
	} {
		err := g.Train(geom.Training{X: scale.Ordinal(), Y: scale.Ordinal()})
		if !errors.Is(err, geom.ErrNoColumn) {
			t.Errorf("a half-named membership table gave %v, want ErrNoColumn", err)
		}
	}
}

// A combination of sets is a name rather than a quantity, so the axis has to be
// one with slots. Filling one in on a continuous axis would draw every column
// on top of the first.
func TestASetMarkRefusesAContinuousAxis(t *testing.T) {
	src := subscriptions()
	for _, g := range []geom.Geom{
		geom.Intersections(src, geom.From("who"), geom.To("what")),
		geom.SetMatrix(src, geom.From("who"), geom.To("what")),
	} {
		err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
		if !errors.Is(err, geom.ErrCategorical) {
			t.Errorf("a continuous axis gave %v, want ErrCategorical", err)
		}
	}
}

// A column is what several rows have in common rather than a row of its own, so
// neither half reports one — the sankey's rule, unchanged.
func TestAnUpSetReportsNoRows(t *testing.T) {
	src := subscriptions()
	for _, g := range []geom.Geom{
		geom.Intersections(src, geom.From("who"), geom.To("what")),
		geom.SetMatrix(src, geom.From("who"), geom.To("what")),
	} {
		y := scale.Scale(scale.Ordinal())
		if _, ok := g.(interface{ Describe() geom.Desc }); ok {
			if d, _ := geom.Describe(g); d.Mark == geom.MarkIntersections {
				y = scale.Linear()
			}
		}
		rec, f := upsetFrame(t, g, scale.Ordinal(), y, 400, 200)
		var got []int
		f.Rows = rowsFunc(func(_ []ir.Point, rows []int) { got = append(got, rows...) })
		if err := g.Build(rec, f); err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("a set mark reported %d rows, want none", len(got))
		}
	}
}

// Redrawn, it draws the same thing: the buffers are reused, the order is the
// table's, and the ranking's ties are broken by that order rather than by a
// sort's own idea.
func TestASetMarkRedrawnTwiceDrawsTheSameThing(t *testing.T) {
	src := subscriptions()
	for _, mk := range []func() (geom.Geom, scale.Scale, scale.Scale){
		func() (geom.Geom, scale.Scale, scale.Scale) {
			return geom.Intersections(src, geom.From("who"), geom.To("what")), scale.Ordinal(), scale.Linear()
		},
		func() (geom.Geom, scale.Scale, scale.Scale) {
			return geom.SetMatrix(src, geom.From("who"), geom.To("what")), scale.Ordinal(), scale.Ordinal()
		},
		func() (geom.Geom, scale.Scale, scale.Scale) {
			return geom.Venn(src, geom.From("who"), geom.To("what")), scale.Linear(), scale.Linear()
		},
	} {
		g, x, y := mk()
		rec, f := upsetFrame(t, g, x, y, 400, 240)
		if err := g.Build(rec, f); err != nil {
			t.Fatal(err)
		}
		first := rec.String()
		rec.Reset()
		if err := g.Build(rec, f); err != nil {
			t.Fatal(err)
		}
		if second := rec.String(); first != second {
			t.Error("a redraw drew something else")
		}
	}
}

// The round trip through Desc rebuilds the same layer, which is what makes the
// JSON document of one of these charts the same chart.
func TestASetMarkRoundTripsThroughDescribe(t *testing.T) {
	src := subscriptions()
	for _, g := range []geom.Geom{
		geom.Intersections(src, geom.From("who"), geom.To("what"), geom.Top(4)),
		geom.SetMatrix(src, geom.From("who"), geom.To("what"), geom.Top(4)),
	} {
		d, ok := geom.Describe(g)
		if !ok {
			t.Fatal("a set mark does not describe itself")
		}
		if d.Top != 4 {
			t.Errorf("Top round-tripped as %d, want 4", d.Top)
		}
		back, err := geom.FromDesc(d)
		if err != nil {
			t.Fatal(err)
		}
		y := scale.Scale(scale.Ordinal())
		if d.Mark == geom.MarkIntersections {
			y = scale.Linear()
		}
		rec, f := upsetFrame(t, g, scale.Ordinal(), y, 400, 200)
		if err := g.Build(rec, f); err != nil {
			t.Fatal(err)
		}
		want := rec.String()

		y2 := scale.Scale(scale.Ordinal())
		if d.Mark == geom.MarkIntersections {
			y2 = scale.Linear()
		}
		rec2, f2 := upsetFrame(t, back, scale.Ordinal(), y2, 400, 200)
		if err := back.Build(rec2, f2); err != nil {
			t.Fatal(err)
		}
		if got := rec2.String(); got != want {
			t.Errorf("the rebuilt %s layer drew something else", d.Mark)
		}
	}
}

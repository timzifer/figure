package geom_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// The third panel of an UpSet plot. Its bars are a different count from the
// ones above them — a customer with two products is in two of these and in one
// of those — and the whole point of it being a mark is that the count is the
// same arithmetic the other two panels run rather than one the caller writes
// again beside them. See docs/adr/0076-the-other-half-of-the-count.md.

// bars collects one rectangle per subpath of what a mark filled, which is one
// bar here: [ir.Path.Walk] starts a new subpath at every move.
func bars(t *testing.T, rec *irtest.Recorder) []ir.Rect {
	t.Helper()
	var out []ir.Rect
	for _, c := range rec.Filter("FillPath") {
		c.Path.Walk(func(op ir.PathOp, pts []ir.Point) {
			if op == ir.OpMoveTo {
				out = append(out, ir.Rect{Min: pts[0], Max: pts[0]})
			}
			if len(out) == 0 {
				return
			}
			r := &out[len(out)-1]
			for _, p := range pts {
				r.Min.X, r.Min.Y = min(r.Min.X, p.X), min(r.Min.Y, p.Y)
				r.Max.X, r.Max.Y = max(r.Max.X, p.X), max(r.Max.Y, p.Y)
			}
		})
	}
	return out
}

func TestSetSizesAreEachSetsOwnTotal(t *testing.T) {
	// mail has seven customers, drive six and chat four; the table has ten
	// customers and seventeen memberships in it.
	g := geom.SetSizes(subscriptions(), geom.From("who"), geom.To("what"))
	x, y := scale.Linear(scale.Zero()), scale.Ordinal()
	rec, f := upsetFrame(t, g, x, y, 400, 120)
	if _, hi := x.Domain(); hi != 7 {
		t.Errorf("the axis was trained to %v, and the biggest set has 7 in it", hi)
	}
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}

	got := bars(t, rec)
	if len(got) != 3 {
		t.Fatalf("drew %d bars, want one per set", len(got))
	}
	// The bars are in lane order, which is the order the table names the sets,
	// and their lengths are the counts.
	for i, want := range []float64{7, 6, 4} {
		length := float64(got[i].Max.X - got[i].Min.X)
		if ratio := length / float64(got[0].Max.X-got[0].Min.X); math.Abs(ratio-want/7) > 1e-3 {
			t.Errorf("bar %d is %v long, which is %v of the longest rather than %v", i, length, ratio, want/7)
		}
	}
}

// A join hands back a row per match, and the same (element, set) pair can come
// back twice. A set holds an element or it does not: the count says so, and
// this is the arithmetic a caller doing it by hand gets wrong.
func TestSetSizesCountAMembershipOnce(t *testing.T) {
	once := geom.SetSizes(subscriptions(), geom.From("who"), geom.To("what"))
	x, y := scale.Linear(scale.Zero()), scale.Ordinal()
	upsetFrame(t, once, x, y, 400, 120)
	_, want := x.Domain()

	// The same table with "a is on mail" said twice.
	var who, what []string
	src := subscriptions()
	col, _ := data.Labels(src, "who")
	set, _ := data.Labels(src, "what")
	for i := range col {
		who, what = append(who, col[i]), append(what, set[i])
		if col[i] == "a" && set[i] == "mail" {
			who, what = append(who, col[i]), append(what, set[i])
		}
	}
	twice := geom.SetSizes(data.NewTable().String("who", who).String("what", what),
		geom.From("who"), geom.To("what"))
	x2, y2 := scale.Linear(scale.Zero()), scale.Ordinal()
	upsetFrame(t, twice, x2, y2, 400, 120)
	if _, got := x2.Domain(); got != want {
		t.Errorf("a repeated membership made a set %v big, want %v", got, want)
	}
}

// The bars and the dots line up because they encode the same names into the
// same ordinal scale object, in the same order — not because the caller sorted
// two tables the same way.
func TestSetSizesShareTheMatrixLanes(t *testing.T) {
	src := subscriptions()
	lanes := scale.Ordinal()
	dots := geom.SetMatrix(src, geom.From("who"), geom.To("what"))
	side := geom.SetSizes(src, geom.From("who"), geom.To("what"))

	upsetFrame(t, dots, scale.Ordinal(), lanes, 400, 120)
	rec, f := upsetFrame(t, side, scale.Linear(scale.Zero()), lanes, 400, 120)
	if err := side.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}

	names := lanes.(scale.Categorical).Labels()
	if strings.Join(names, "|") != "mail|drive|chat" {
		t.Fatalf("the lanes are %v, want the order the table names them in", names)
	}
	for i, name := range names {
		got := bars(t, rec)[i]
		mid := (got.Min.Y + got.Max.Y) / 2
		if want := f.Y.Map(lanes.(scale.Categorical).Encode(name)); math.Abs(float64(mid-want)) > 0.5 {
			t.Errorf("the %q bar is centred at %v and its lane is at %v", name, mid, want)
		}
	}
}

// Order and Top say which combinations the chart has columns for. A set's total
// is over the whole table, so they say nothing here — and a mark that quietly
// dropped a lane would be a chart whose bars and dots disagree.
func TestSetSizesIgnoreTheRanking(t *testing.T) {
	g := geom.SetSizes(subscriptions(), geom.From("who"), geom.To("what"),
		geom.Top(1), geom.Order(geom.OrderAppearance))
	rec, f := upsetFrame(t, g, scale.Linear(scale.Zero()), scale.Ordinal(), 400, 120)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := bars(t, rec); len(got) != 3 {
		t.Errorf("Top(1) left %d bars, and the table has three sets", len(got))
	}
}

func TestSetSizesNeedAnOrdinalLane(t *testing.T) {
	g := geom.SetSizes(subscriptions(), geom.From("who"), geom.To("what"))
	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrCategorical) {
		t.Errorf("a linear lane axis gave %v, want ErrCategorical", err)
	}
}

func TestSetSizesNeedTheMembershipChannels(t *testing.T) {
	g := geom.SetSizes(subscriptions())
	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Ordinal()})
	if !errors.Is(err, geom.ErrNoColumn) {
		t.Errorf("a layer with no channels gave %v, want ErrNoColumn", err)
	}
}

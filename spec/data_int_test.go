package spec_test

import (
	"testing"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/spec"
	"github.com/timzifer/figure/theme"
)

// An exact integer column survives the JSON round trip with every digit.
//
// The document declares the column "integer" and the reader decodes the values
// with json.Number, because encoding/json reads a JSON number into an `any` as
// a float64 and an id past 2^53 would come back a different id — which is the
// one thing the kind exists to prevent.
func TestAnExactIntegerColumnRoundTrips(t *testing.T) {
	const id = 9007199254740993 // 2^53+1

	tbl := data.NewTable().
		Float64("x", []float64{0, 1}).
		Float64("y", []float64{1, 2}).
		Int64("id", []int64{id, id + 2})

	c := spec.Chart{
		Width: 400, Height: 300, DPR: 1, Theme: theme.Light,
		X: scale.Linear(), Y: scale.Linear(),
		Layers: []geom.Geom{geom.Scatter(tbl, geom.X("x"), geom.Y("y"), geom.KeyBy("id"))},
	}

	back := roundTrip(t, c)
	src, ok := geom.SourceOf(back.Layers[0])
	if !ok {
		t.Fatal("the layer came back without a source")
	}
	ids, ok := data.Int64Column(src, "id")
	if !ok {
		t.Fatal("the column came back as something other than exact integers")
	}
	if ids[0] != id || ids[1] != id+2 {
		t.Errorf("ids came back as %v, want %d and %d", ids, id, id+2)
	}
}

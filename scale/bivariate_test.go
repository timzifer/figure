package scale_test

import (
	"math"
	"testing"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

func trainedVSUP(classes, layers int) scale.BivariateColorScale {
	s := scale.VSUP(palette.Viridis, classes, layers)
	s.Train(0, 100)
	s.TrainSecond(0, 1)
	return s
}

// distinct counts the colours a layer paints across the value domain.
func distinct(s scale.BivariateColorScale, u float64) int {
	seen := map[ir.Color]bool{}
	for v := 0.5; v < 100; v++ {
		seen[s.ColorAt(v, u)] = true
	}
	return len(seen)
}

func TestVSUPHalvesItsClassesPerLayer(t *testing.T) {
	s := trainedVSUP(8, 4)
	for layer, want := range []int{8, 4, 2, 1} {
		u := (float64(layer) + 0.5) / 4
		if got := distinct(s, u); got != want {
			t.Errorf("layer %d paints %d colours, want %d", layer, got, want)
		}
	}
}

func TestVSUPSuppressesADistinctionTheDataCannotSupport(t *testing.T) {
	s := trainedVSUP(8, 4)
	// 10 and 40 are two classes apart at full certainty…
	if s.ColorAt(10, 0) == s.ColorAt(40, 0) {
		t.Error("two values in different certain classes share a colour")
	}
	// …and the same colour at the most uncertain layer.
	if s.ColorAt(10, 1) != s.ColorAt(40, 1) {
		t.Error("the most uncertain layer still tells 10 from 40")
	}
	// Color is the certain reading.
	for _, v := range []float64{3, 47, 99} {
		if s.Color(v) != s.ColorAt(v, 0) {
			t.Errorf("Color(%v) is not the certain colour", v)
		}
	}
}

func TestABivariateScaleGivesTheUndefinedColourToAnUnusableReading(t *testing.T) {
	undef := ir.RGB(1, 2, 3)
	for name, s := range map[string]scale.BivariateColorScale{
		"vsup":   scale.VSUP(palette.Viridis, 4, 2, scale.ColorUndefined(undef)),
		"matrix": scale.BivariateMatrix(palette.BivariateBlueRed, scale.ColorUndefined(undef)),
	} {
		s.Train(0, 1)
		s.TrainSecond(0, 1)
		for _, vu := range [][2]float64{{math.NaN(), 0.5}, {0.5, math.NaN()}, {math.Inf(1), 0.5}, {0.5, math.Inf(-1)}} {
			if got := s.ColorAt(vu[0], vu[1]); got != undef {
				t.Errorf("%s ColorAt(%v, %v) = %v, want the undefined colour", name, vu[0], vu[1], got)
			}
		}
	}
}

func TestAnUntrainedSecondReadingIsCertain(t *testing.T) {
	s := scale.VSUP(palette.Viridis, 8, 4)
	s.Train(0, 100)
	if lo, hi := s.SecondDomain(); lo != 0 || hi != 0 {
		t.Errorf("SecondDomain of an untrained scale = (%v, %v), want (0, 0)", lo, hi)
	}
	if s.ColorAt(10, 123) != s.Color(10) {
		t.Error("with no second domain every mark should be read as certain")
	}
}

func TestVSUPKeyCellsTileEachLayer(t *testing.T) {
	s := trainedVSUP(8, 4)
	cells := s.KeyCells()
	if len(cells) != 8+4+2+1 {
		t.Fatalf("%d key cells, want 15", len(cells))
	}
	width := map[float64]float64{}
	for _, c := range cells {
		width[c.ULo] += c.VHi - c.VLo
		if c.UHi <= c.ULo {
			t.Errorf("cell %+v has no height", c)
		}
		// Each cell's colour is what a mark in its middle is painted.
		if got := s.ColorAt((c.VLo+c.VHi)/2, (c.ULo+c.UHi)/2); got != c.Color {
			t.Errorf("cell %+v is keyed %v but paints %v", c, c.Color, got)
		}
	}
	for u, w := range width {
		if math.Abs(w-100) > 1e-9 {
			t.Errorf("the layer at u=%v covers %v of the domain, want all 100", u, w)
		}
	}
}

func TestABivariateMatrixLooksUpBothClasses(t *testing.T) {
	m := palette.BivariateBlueRed
	s := scale.BivariateMatrix(m)
	s.Train(0, 90)
	s.TrainSecond(0, 30)
	for _, tc := range []struct {
		v, u float64
		i, j int
	}{{0, 0, 0, 0}, {90, 30, 2, 2}, {89, 1, 2, 0}, {1, 29, 0, 2}, {45, 15, 1, 1}} {
		if got := s.ColorAt(tc.v, tc.u); got != m[tc.i][tc.j] {
			t.Errorf("ColorAt(%v, %v) = %v, want m[%d][%d] = %v", tc.v, tc.u, got, tc.i, tc.j, m[tc.i][tc.j])
		}
	}
	if s.Color(89) != m[2][0] {
		t.Error("Color should be the first column")
	}

	rect := scale.BivariateMatrix([][]ir.Color{
		{ir.RGB(1, 0, 0), ir.RGB(2, 0, 0), ir.RGB(3, 0, 0), ir.RGB(4, 0, 0)},
		{ir.RGB(5, 0, 0), ir.RGB(6, 0, 0), ir.RGB(7, 0, 0), ir.RGB(8, 0, 0)},
	})
	rect.Train(0, 1)
	rect.TrainSecond(0, 1)
	if got := rect.ColorAt(0.9, 0.6); got != ir.RGB(7, 0, 0) {
		t.Errorf("a 2×4 matrix at (0.9, 0.6) = %v, want row 1 column 2", got)
	}
	if n := len(rect.KeyCells()); n != 8 {
		t.Errorf("%d key cells, want one per entry", n)
	}
}

func TestARaggedOrEmptyMatrixPanics(t *testing.T) {
	for name, m := range map[string][][]ir.Color{
		"empty":  {},
		"ragged": {{ir.RGB(1, 1, 1), ir.RGB(2, 2, 2)}, {ir.RGB(3, 3, 3)}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("BivariateMatrix(%s) did not panic", name)
				}
			}()
			scale.BivariateMatrix(m)
		}()
	}
}

func TestBivariateScalesAreClassedOverTheQuantity(t *testing.T) {
	v := trainedVSUP(4, 3)
	c, ok := scale.Classed(v)
	if !ok || c.Classes() != 4 || len(c.Breaks()) != 3 {
		t.Errorf("VSUP is not classed over its certain layer: ok=%v", ok)
	}
	m := scale.BivariateMatrix(palette.BivariateBlueRed)
	m.Train(0, 3)
	c, ok = scale.Classed(m)
	if !ok || c.Classes() != 3 || len(c.Breaks()) != 2 {
		t.Errorf("a matrix is not classed over its rows: ok=%v", ok)
	}
	if _, ok := scale.Bivariate(scale.Sequential(palette.Viridis)); ok {
		t.Error("a sequential scale claims to be bivariate")
	}
}

func TestBivariateScalesSurviveTheirDesc(t *testing.T) {
	literal := [][]ir.Color{{ir.RGB(10, 0, 0), ir.RGB(20, 0, 0)}, {ir.RGB(30, 0, 0), ir.RGB(40, 0, 0)}, {ir.RGB(50, 0, 0), ir.RGB(60, 0, 0)}}
	for name, s := range map[string]scale.BivariateColorScale{
		"vsup":    scale.VSUP(palette.Magma, 8, 4, scale.ColorReverse(), scale.ColorDomain(0, 50)),
		"named":   scale.BivariateMatrix(palette.BivariateBlueRed),
		"literal": scale.BivariateMatrix(literal, scale.ColorDomain(-1, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			d, ok := scale.DescribeColor(s)
			if !ok {
				t.Fatal("cannot describe itself")
			}
			switch name {
			case "vsup":
				if d.Kind != scale.KindVSUP || d.Classes != 8 || d.Layers != 4 {
					t.Errorf("Desc = %+v", d)
				}
			case "named":
				if d.Kind != scale.KindBivariate || d.Ramp != "bluered" {
					t.Errorf("Desc = %+v", d)
				}
			case "literal":
				if d.Ramp != "" || len(d.Colors) != 6 || d.Classes != 3 {
					t.Errorf("Desc = %+v", d)
				}
			}
			back, err := scale.ColorFromDesc(d)
			if err != nil {
				t.Fatal(err)
			}
			b, ok := scale.Bivariate(back)
			if !ok {
				t.Fatal("did not read back as bivariate")
			}
			for _, sc := range []scale.BivariateColorScale{s, b} {
				sc.Train(-1, 50)
				sc.TrainSecond(0, 1)
			}
			for _, vu := range [][2]float64{{-0.5, 0.1}, {0.3, 0.9}, {12, 0.4}, {49, 0.99}} {
				if s.ColorAt(vu[0], vu[1]) != b.ColorAt(vu[0], vu[1]) {
					t.Errorf("ColorAt(%v, %v) differs after the round trip", vu[0], vu[1])
				}
			}
		})
	}

	if _, err := scale.ColorFromDesc(scale.ColorDesc{Kind: scale.KindBivariate, Colors: make(palette.Ramp, 5), Classes: 2}); err == nil {
		t.Error("five colours in two rows were accepted")
	}
	if _, err := scale.ColorFromDesc(scale.ColorDesc{Kind: scale.KindBivariate, Ramp: "nosuch"}); err == nil {
		t.Error("an unknown matrix name was accepted")
	}
}

func TestABivariateScaleIsDeterministic(t *testing.T) {
	a, b := trainedVSUP(8, 4), trainedVSUP(8, 4)
	for v := 0.0; v <= 100; v += 3.7 {
		for u := 0.0; u <= 1; u += 0.13 {
			if a.ColorAt(v, u) != b.ColorAt(v, u) {
				t.Fatalf("ColorAt(%v, %v) differs between two identical scales", v, u)
			}
		}
	}
}

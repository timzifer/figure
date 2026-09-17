package ir

import (
	"image"
	"math"
	"testing"
)

// hatchKinds is every pattern this release draws, which is every Hatch but
// HatchNone. A new one added to the enum and forgotten here is a pattern
// nothing below checks.
var hatchKinds = []Hatch{
	HatchDiagonal, HatchBackDiagonal, HatchCross,
	HatchHorizontal, HatchVertical, HatchGrid,
	HatchDots, HatchDotsStaggered,
	HatchZigzag, HatchWave,
	HatchBrick, HatchTriangles, HatchScales,
}

func hatching(h Hatch, spacing float32) Hatching {
	return Hatching{Hatch: h, Line: Stroke{Color: RGB(0, 0, 0), Width: 1}, Spacing: spacing}
}

func TestEveryHatchDrawsSomething(t *testing.T) {
	b := R(0, 0, 40, 40)
	for _, k := range hatchKinds {
		var p Path
		HatchPath(&p, hatching(k, 5), b)
		if p.Empty() {
			t.Errorf("hatch %d over %v appended nothing", k, b)
		}
	}
}

func TestAHatchIsPhasedOnTheOrigin(t *testing.T) {
	// Two marks side by side must continue one pattern rather than each
	// starting its own, or a stacked bar reads as a ladder of misaligned
	// hatches instead of as one hatched column. What makes them continue is
	// that every element sits on a lattice measured from the origin, whatever
	// box it was asked for — so the test is on the lattice, not on the box.
	const s = 5
	boxes := []Rect{R(0, 0, 40, 40), R(7, 3, 41, 44), R(-13, -6, 12, 19)}

	// The line families, and the multiple each one's offset must be. A
	// diagonal's lines are constants of x+y, and their step along that
	// constant is the spacing times the root of two.
	lines := []struct {
		h    Hatch
		step float64
		key  func(a Point) float64
	}{
		{HatchDiagonal, s * math.Sqrt2, func(a Point) float64 { return float64(a.X + a.Y) }},
		{HatchBackDiagonal, s * math.Sqrt2, func(a Point) float64 { return float64(a.X - a.Y) }},
		{HatchHorizontal, s, func(a Point) float64 { return float64(a.Y) }},
		{HatchVertical, s, func(a Point) float64 { return float64(a.X) }},
	}
	for _, fam := range lines {
		for _, box := range boxes {
			var p Path
			HatchPath(&p, hatching(fam.h, s), box)
			if p.Empty() {
				t.Fatalf("hatch %d over %v appended nothing", fam.h, box)
			}
			for i := 0; i+1 < len(p.Pts); i += 2 {
				v := fam.key(p.Pts[i])
				if off := math.Abs(math.Remainder(v, fam.step)); off > 1e-3 {
					t.Errorf("hatch %d over %v: a line at %v is %v off the lattice",
						fam.h, box, v, off)
				}
			}
		}
	}

	// The dotted patterns sit on a square lattice of the spacing, offset by
	// half a step on odd rows where they stagger. A dot is a circle started at
	// the top of itself, so its centre is a radius below its first point.
	const radius = 1 // hatching() draws at width 1, and a dot's radius is its width
	for _, k := range []Hatch{HatchDots, HatchDotsStaggered} {
		for _, box := range boxes {
			var p Path
			HatchPath(&p, hatching(k, s), box)
			if p.Empty() {
				t.Fatalf("hatch %d over %v appended nothing", k, box)
			}
			i := 0
			p.Walk(func(op PathOp, pts []Point) {
				if op != OpMoveTo {
					return
				}
				i++
				cx, cy := float64(pts[0].X), float64(pts[0].Y)+radius
				if off := math.Abs(math.Remainder(cy, s)); off > 1e-3 {
					t.Errorf("hatch %d over %v: dot row at %v is %v off the lattice", k, box, cy, off)
				}
				// A staggered row is half a step across; either phase is on
				// the lattice of half the spacing.
				step := float64(s)
				if k == HatchDotsStaggered {
					step = float64(s) / 2
				}
				if off := math.Abs(math.Remainder(cx, step)); off > 1e-3 {
					t.Errorf("hatch %d over %v: dot at %v is %v off the lattice", k, box, cx, off)
				}
			})
			if i == 0 {
				t.Errorf("hatch %d over %v placed no dots", k, box)
			}
		}
	}
}

func TestAHatchCoversWhatItIsGiven(t *testing.T) {
	b := R(10, 10, 50, 50)
	for _, k := range hatchKinds {
		var p Path
		HatchPath(&p, hatching(k, 5), b)
		got := p.Bounds()
		// The geometry may overshoot — it is drawn clipped — but it must not
		// leave a band of the mark unpatterned at any edge.
		if got.Min.X > b.Min.X || got.Min.Y > b.Min.Y || got.Max.X < b.Max.X || got.Max.Y < b.Max.Y {
			t.Errorf("hatch %d covers %v, short of %v", k, got, b)
		}
	}
}

func TestAHatchNobodyRecognisesDrawsNothing(t *testing.T) {
	b := R(0, 0, 40, 40)
	cases := []struct {
		name string
		h    Hatching
	}{
		{"none", hatching(HatchNone, 5)},
		{"unknown", hatching(Hatch(200), 5)},
		{"no spacing", hatching(HatchDiagonal, 0)},
		{"negative spacing", hatching(HatchDiagonal, -5)},
	}
	for _, c := range cases {
		var p Path
		if HatchPath(&p, c.h, b); !p.Empty() {
			t.Errorf("%s: appended %d ops, want none", c.name, len(p.Ops))
		}
	}
}

func TestAHatchTooDenseToReadDrawsNothing(t *testing.T) {
	// A five-unit pattern over a mark ten thousand units wide is a solid
	// block, and drawing it would cost thousands of strokes to say so.
	b := R(0, 0, 100000, 100000)
	for _, k := range hatchKinds {
		var p Path
		HatchPath(&p, hatching(k, 5), b)
		if len(p.Ops) > 4*maxHatchElements {
			t.Errorf("hatch %d emitted %d ops over a huge mark", k, len(p.Ops))
		}
	}
}

func TestOnlyTheFilledPatternsSayTheyAreFilled(t *testing.T) {
	filled := map[Hatch]bool{HatchDots: true, HatchDotsStaggered: true, HatchTriangles: true}
	b := R(0, 0, 40, 40)
	for _, k := range hatchKinds {
		var p Path
		if got := HatchPath(&p, hatching(k, 5), b); got != filled[k] {
			t.Errorf("hatch %d: fill = %v, want %v", k, got, filled[k])
		}
	}
}

func TestADiagonalHatchRunsAtFortyFiveDegrees(t *testing.T) {
	var p Path
	HatchPath(&p, hatching(HatchDiagonal, 5), R(0, 0, 20, 20))
	for i := 0; i+1 < len(p.Pts); i += 2 {
		a, b := p.Pts[i], p.Pts[i+1]
		dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
		if math.Abs(dx+dy) > 1e-4 {
			t.Fatalf("segment %v-%v is not at 45 degrees", a, b)
		}
	}
}

// hatchRecorder counts the calls FillHatched and FillInset make, and whether
// they were announced as ornament.
type hatchRecorder struct {
	nullBackend
	fills, strokes, pushes, pops int
	depth                        int
	maxDepth                     int
	decorated                    []string
}

func (r *hatchRecorder) FillPath(*Path, Fill, FillRule) {
	r.fills++
	r.note("fill")
}
func (r *hatchRecorder) StrokePath(*Path, Stroke) {
	r.strokes++
	r.note("stroke")
}
func (r *hatchRecorder) Push(*Path, Affine) { r.pushes++ }
func (r *hatchRecorder) Pop()               { r.pops++ }
func (r *hatchRecorder) BeginDecoration()   { r.depth++; r.maxDepth = max(r.maxDepth, r.depth) }
func (r *hatchRecorder) EndDecoration()     { r.depth-- }

func (r *hatchRecorder) note(op string) {
	if r.depth > 0 {
		r.decorated = append(r.decorated, op)
	}
}

// nullBackend is the rest of the interface, which these tests do not exercise.
type nullBackend struct{}

func (nullBackend) Polyline([]Point, Stroke)             {}
func (nullBackend) Text(TextRun)                         {}
func (nullBackend) Markers(Marker, []Point, MarkerStyle) {}
func (nullBackend) Image(image.Image, Rect)              {}
func (nullBackend) Measure(TextRun) TextMetrics          { return TextMetrics{} }
func (nullBackend) Flush() error                         { return nil }

func TestFillHatchedFillsThenDecorates(t *testing.T) {
	var p Path
	p.Rect(R(0, 0, 40, 40))

	r := &hatchRecorder{}
	FillHatched(r, &p, Solid(RGB(1, 2, 3)), NonZero, hatching(HatchDiagonal, 5))

	if r.fills != 1 || r.strokes != 1 {
		t.Fatalf("fills = %d, strokes = %d, want 1 and 1", r.fills, r.strokes)
	}
	if r.pushes != 1 || r.pops != 1 {
		t.Fatalf("pushes = %d, pops = %d, want 1 and 1", r.pushes, r.pops)
	}
	if r.depth != 0 || r.maxDepth != 1 {
		t.Fatalf("decoration depth ended at %d, peaked at %d", r.depth, r.maxDepth)
	}
	// The mark itself is a mark; only the hatch over it is ornament.
	if len(r.decorated) != 1 || r.decorated[0] != "stroke" {
		t.Fatalf("decorated calls = %v, want just the hatch stroke", r.decorated)
	}
}

func TestFillHatchedWithoutAHatchIsAPlainFill(t *testing.T) {
	var p Path
	p.Rect(R(0, 0, 40, 40))

	r := &hatchRecorder{}
	FillHatched(r, &p, Solid(RGB(1, 2, 3)), NonZero, Hatching{})

	if r.fills != 1 || r.strokes != 0 || r.pushes != 0 {
		t.Fatalf("fills = %d, strokes = %d, pushes = %d, want 1, 0, 0", r.fills, r.strokes, r.pushes)
	}
}

func TestFillHatchedFillsADottedPatternRatherThanStrokingIt(t *testing.T) {
	var p Path
	p.Rect(R(0, 0, 40, 40))

	r := &hatchRecorder{}
	FillHatched(r, &p, Solid(RGB(1, 2, 3)), NonZero, hatching(HatchDots, 8))

	if r.fills != 2 || r.strokes != 0 {
		t.Fatalf("fills = %d, strokes = %d, want 2 and 0", r.fills, r.strokes)
	}
}

func TestFillInsetStrokesInsideTheShape(t *testing.T) {
	var p Path
	p.Rect(R(0, 0, 40, 40))

	r := &hatchRecorder{}
	FillInset(r, &p, Solid(RGB(1, 2, 3)), NonZero, Stroke{Color: RGB(0, 0, 0), Width: 2})

	if r.fills != 1 || r.strokes != 1 || r.pushes != 1 || r.pops != 1 {
		t.Fatalf("fills = %d, strokes = %d, pushes = %d, pops = %d", r.fills, r.strokes, r.pushes, r.pops)
	}
	if r.maxDepth != 1 {
		t.Fatalf("the inner border was not announced as ornament")
	}
}

func TestAnUnroundedRoundRectIsARect(t *testing.T) {
	want := R(2, 3, 10, 20)
	for _, rad := range []float32{0, -1} {
		var p Path
		p.RoundRect(want, rad)
		got, ok := p.AsRect()
		if !ok || got != want {
			t.Errorf("radius %v: AsRect = %v, %v, want %v, true", rad, got, ok, want)
		}
	}
}

func TestARoundRectStaysInItsRectangle(t *testing.T) {
	want := R(2, 3, 10, 20)
	var p Path
	p.RoundRect(want, 3)
	if got := p.Bounds(); got != want {
		t.Errorf("bounds = %v, want %v", got, want)
	}
	if _, ok := p.AsRect(); ok {
		t.Error("a rounded rectangle reported itself as a plain one")
	}
}

func TestARoundRectClampsItsRadius(t *testing.T) {
	// A radius past half the shorter side would fold the corners through each
	// other; clamping makes it a stadium instead.
	r := R(0, 0, 10, 4)
	var wide, clamped Path
	wide.RoundRect(r, 100)
	clamped.RoundRect(r, 2)
	if len(wide.Pts) != len(clamped.Pts) {
		t.Fatalf("point counts differ: %d and %d", len(wide.Pts), len(clamped.Pts))
	}
	for i := range wide.Pts {
		if wide.Pts[i] != clamped.Pts[i] {
			t.Fatalf("point %d: %v, want %v", i, wide.Pts[i], clamped.Pts[i])
		}
	}
}

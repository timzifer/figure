package three

import (
	"fmt"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// Contour draws the level sets of a value on the floor of the cube.
//
// It is the reading a surface hides, put where a reader can take it: the shape
// stands above and its plan lies beneath, in one picture and at one angle. That
// is what a topographic map does with a mountain, and what a machinist's plan
// view does with a part.
//
//	sc.Add(
//		three.Surface(src, geom.X("x"), geom.Y("y"), geom.Z("z"), geom.ColorBy("z", ramp)),
//		three.Contour(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
//			geom.Levels(lv...), geom.ColorBy("z", ramp)),
//	)
//
// Handed the same [github.com/timzifer/figure/geom.Levels] and the same colour
// scale, it is the same lines a flat
// [github.com/timzifer/figure/geom.Contour] draws — one tracing in
// [stat.Contour] rather than two, which is what stops a reading taken off the
// plan and one taken off the floor disagreeing at a saddle.
//
// # The floor, or the ceiling, and not the walls
//
// [Plane] chooses which face the lines lie on, and the two faces it offers are
// the two the value is a function *over*. A back wall is a plane that contains
// z, so there is no isoline to draw on it — projecting the floor family
// sideways would draw lines that mean nothing. What a wall wants is a
// cross-section of the surface at that wall's coordinate, which is a different
// layer with a different stat.
//
// # What it costs
//
// One drawing call per segment, which is [Sink.Line]'s rule: a run that spans
// the floor has no single depth under any camera that is not looking straight
// down, so a whole run drawn as one primitive would be ordered wholly in front
// of the surface or wholly behind it. Per segment it interleaves properly. That
// is a real cost on a fine grid — keep the level count small, which is what its
// default is for.
func Contour(src data.Source, opts ...geom.Option) Layer {
	return &contour3{base: newBase(src, "contour3", opts)}
}

// ContourPlane names the face a family of isolines lies on.
type ContourPlane uint8

// The faces a contour may lie on. See [Contour] on why the walls are not among
// them.
const (
	// Floor is the bottom of the cube, which is where a plan belongs and is the
	// default.
	Floor ContourPlane = iota
	// Ceiling is the top, for a scene a reader is looking up into.
	Ceiling
)

// planeKey is the [github.com/timzifer/figure/geom.Extra] key [Plane] sets.
//
// It is spelled with the package in it because the extras are one namespace
// shared with every mark defined outside geom, which is what ADR 0029 asks of a
// mark that adds a knob of its own.
const planeKey = "three.plane"

// Plane chooses which face of the cube a [Contour] lies on.
//
// The value travels as a string rather than as the constant, because a
// description round-trips through JSON and a number comes back as a float64 —
// a typed byte would not survive the trip, and the failure would be a chart
// that read back drawing on the wrong face.
func Plane(p ContourPlane) geom.Option {
	name := "floor"
	if p == Ceiling {
		name = "ceiling"
	}
	return geom.Extra(planeKey, name)
}

type contour3 struct {
	base

	grid   stat.Lattice
	lines  stat.Contour
	levels []float64
	ramp   scale.ColorScale
	plane  ContourPlane
	ok     bool

	seg [2]Vec3
}

func (g *contour3) Train(t geom.Training) error {
	g.ok = false
	xs, err := column(g.src, g.cfg.X, t.X)
	if err != nil {
		return err
	}
	ys, err := column(g.src, g.cfg.Y, t.Y)
	if err != nil {
		return err
	}
	zs, err := column(g.src, g.cfg.Z, t.Z)
	if err != nil {
		return err
	}

	switch f := g.grid.Reset(xs, ys, zs); f {
	case stat.LatticeOK:
	case stat.LatticeRagged:
		return fmt.Errorf("figure/three: the contour's columns have %d, %d and %d rows",
			len(xs), len(ys), len(zs))
	case stat.LatticeTooSmall:
		return fmt.Errorf("figure/three: a contour needs at least two distinct values on each floor axis, got %d and %d",
			len(g.grid.Xs), len(g.grid.Ys))
	case stat.LatticeWrongCount:
		nx, ny := len(g.grid.Xs), len(g.grid.Ys)
		return fmt.Errorf("figure/three: a contour needs one row per cell of its grid: "+
			"%d by %d is %d cells and the table has %d rows", nx, ny, nx*ny, len(zs))
	case stat.LatticeBadPosition:
		r := g.grid.At
		return fmt.Errorf("figure/three: row %d of the contour is at (%v, %v), which is not a position",
			r, xs[r], ys[r])
	default:
		a, b := g.grid.At, g.grid.With
		return fmt.Errorf("figure/three: rows %d and %d are both at (%v, %v); a contour has one value per cell",
			a, b, xs[b], ys[b])
	}

	g.ramp = g.cfg.ColorScale
	g.plane = planeOf(g.cfg)
	g.levels = g.levelsFor()
	g.lines.Reset(g.grid.Xs, g.grid.Ys, g.grid.V, g.levels)

	// The floor axes are the scene's, and the depth axis is trained on the
	// values even though nothing is drawn at them: the lines are a reading of
	// z, so an axis that did not cover them would label a cube the contour
	// contradicts. It is the same reason a heatmap trains its colour scale on
	// what it paints rather than on what it draws.
	t.X.Train(g.grid.Xs...)
	t.Y.Train(g.grid.Ys...)
	t.Z.Train(g.grid.V...)
	if g.ramp != nil {
		g.ramp.Train(g.levelsOrValues()...)
	}
	g.ok = true
	return nil
}

func (g *contour3) levelsFor() []float64 {
	if len(g.cfg.Levels) > 0 {
		return g.cfg.Levels
	}
	lo, hi := fieldExtent(g.grid.V)
	return stat.AppendLevels(g.levels, lo, hi, orElseCount(g.cfg.LevelCount, geom.DefaultLevels))
}

func (g *contour3) levelsOrValues() []float64 {
	if len(g.levels) > 0 {
		return g.levels
	}
	return g.grid.V
}

func (g *contour3) Emit(s *Sink, f Frame) error {
	if !g.ok {
		return nil
	}
	base := g.colorFor(f)
	width := g.widthFor(f)
	z := float32(0)
	if g.plane == Ceiling {
		z = 1
	}

	for _, run := range g.lines.Lines {
		st := Style{Stroke: base, Width: width}
		if g.ramp != nil {
			st.Stroke = g.ramp.Color(run.Level)
		}
		pts := g.lines.Line(run)
		for i := 1; i < len(pts); i++ {
			// One segment at a time. A run that spans the floor has no single
			// depth, so drawn whole it would be ordered wholly in front of the
			// surface above it or wholly behind — see [Sink.Line].
			g.seg[0] = Vec3{at(f.X, pts[i-1].X), at(f.Y, pts[i-1].Y), z}
			g.seg[1] = Vec3{at(f.X, pts[i].X), at(f.Y, pts[i].Y), z}
			s.Line(g.seg[:], st)
		}
	}
	return nil
}

// Levels reports the levels the layer traced, so that a caller can hand the
// same list to a flat chart and get the same lines.
func (g *contour3) Levels() []float64 { return g.levels }

func (g *contour3) Legend(f Frame) (geom.LegendEntry, bool) {
	return g.base.legend(f, geom.SwatchLine)
}

// planeOf reads the face out of the layer's extras, defaulting to the floor —
// including for a value that arrived as something other than a string, which is
// what a hand-written document can carry.
func planeOf(c geom.Desc) ContourPlane {
	if s, ok := c.Extra[planeKey].(string); ok && s == "ceiling" {
		return Ceiling
	}
	return Floor
}

// fieldExtent is the smallest and largest finite value of a field.
func fieldExtent(vs []float64) (lo, hi float64) {
	first := true
	for _, v := range vs {
		if v != v || v-v != 0 { // NaN or infinite
			continue
		}
		if first {
			lo, hi, first = v, v, false
			continue
		}
		lo, hi = min(lo, v), max(hi, v)
	}
	return lo, hi
}

func orElseCount(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}

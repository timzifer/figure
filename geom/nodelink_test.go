package geom_test

import (
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

// The node-link mark: dots joined by lines, placed by distance rather than by
// rank. See docs/adr/0077-a-node-link-layout.md.

// friends is a small undirected graph: two triangles joined by one edge.
func friends() data.Source {
	return data.NewTable().
		String("who", []string{"ana", "bo", "cy", "dag", "eve", "fen", "cy"}).
		String("with", []string{"bo", "cy", "ana", "eve", "fen", "dag", "dag"})
}

func nodeLinkFrame(t *testing.T, g geom.Geom, w, h float32) (*irtest.Recorder, geom.Frame) {
	t.Helper()
	x, y := scale.Linear(), scale.Linear()
	if err := g.Train(geom.Training{X: x, Y: y}); err != nil {
		t.Fatalf("Train: %v", err)
	}
	area := ir.R(0, 0, w, h)
	x.SetRange(area.Min.X, area.Max.X)
	y.SetRange(area.Max.Y, area.Min.Y)
	c := coord.Cartesian().Frame(coord.Framing{Area: area, X: x, Y: y})
	return irtest.New(), geom.Frame{Area: area, X: x, Y: y, Coord: c, Theme: theme.Light}
}

func TestNodeLinkDrawsADotPerNodeAndALinePerEdge(t *testing.T) {
	g := geom.NodeLink(friends(), geom.From("who"), geom.To("with"))
	rec, f := nodeLinkFrame(t, g, 400, 400)
	if err := g.Build(rec, f); err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Six people, so six discs, whatever colours they were batched into.
	discs := 0
	for _, c := range rec.Filter("FillPath") {
		c.Path.Walk(func(op ir.PathOp, _ []ir.Point) {
			if op == ir.OpMoveTo {
				discs++
			}
		})
	}
	if discs != 6 {
		t.Errorf("drew %d discs, and the table names six people", discs)
	}
	// Seven rows, so seven lines.
	lines := 0
	for _, c := range rec.Filter("StrokePath") {
		c.Path.Walk(func(op ir.PathOp, _ []ir.Point) {
			if op == ir.OpMoveTo {
				lines++
			}
		})
	}
	if lines != 7 {
		t.Errorf("drew %d lines, and the table has seven rows", lines)
	}
	names := map[string]bool{}
	for _, s := range rec.Texts() {
		names[s] = true
	}
	for _, want := range []string{"ana", "bo", "cy", "dag", "eve", "fen"} {
		if !names[want] {
			t.Errorf("no label for %q", want)
		}
	}
}

// What the layout matched is a distance, so the panel's shape must not stretch
// the drawing: the same graph on a square panel and on one three times as wide
// is the same picture at a different size.
func TestNodeLinkIsNotStretchedByThePanel(t *testing.T) {
	drawn := func(w, h float32) (float64, float64) {
		t.Helper()
		g := geom.NodeLink(friends(), geom.From("who"), geom.To("with"))
		rec, f := nodeLinkFrame(t, g, w, h)
		if err := g.Build(rec, f); err != nil {
			t.Fatalf("Build: %v", err)
		}
		// The middles of the discs rather than their bounds: a disc's own size
		// is not part of how far apart the layout put things.
		lox, hix := float32(math.Inf(1)), float32(math.Inf(-1))
		loy, hiy := float32(math.Inf(1)), float32(math.Inf(-1))
		for _, c := range rec.Filter("FillPath") {
			for _, at := range subpathMiddles(c.Path) {
				lox, hix = min(lox, at.X), max(hix, at.X)
				loy, hiy = min(loy, at.Y), max(hiy, at.Y)
			}
		}
		return float64(hix - lox), float64(hiy - loy)
	}

	sw, sh := drawn(400, 400)
	ww, wh := drawn(900, 300)
	if ww > 300 {
		t.Errorf("the drawing is %v wide on a panel 300 tall, so it was stretched across it", ww)
	}
	if sw <= 0 || sh <= 0 || ww <= 0 || wh <= 0 {
		t.Fatalf("nothing was drawn: %v by %v and %v by %v", sw, sh, ww, wh)
	}
	square, wide := sw/sh, ww/wh
	if math.Abs(square-wide) > 0.02*square {
		t.Errorf("the drawing is %.3f wide for every unit tall on a square panel and %.3f on a wide one", square, wide)
	}
}

func TestNodeLinkRefusesAGraphItCannotDraw(t *testing.T) {
	n := stat.MaxStressNodes + 1
	who, with := make([]string, n-1), make([]string, n-1)
	for i := range who {
		who[i], with[i] = strconv.Itoa(i), strconv.Itoa(i+1)
	}
	g := geom.NodeLink(data.NewTable().String("who", who).String("with", with),
		geom.From("who"), geom.To("with"))
	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrTooManyNodes) {
		t.Errorf("a graph of %d nodes gave %v, want ErrTooManyNodes", n, err)
	}
}

func TestNodeLinkNeedsAContinuousAxis(t *testing.T) {
	g := geom.NodeLink(friends(), geom.From("who"), geom.To("with"))
	err := g.Train(geom.Training{X: scale.Ordinal(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrNotContinuous) {
		t.Errorf("an ordinal axis gave %v, want ErrNotContinuous", err)
	}
}

func TestNodeLinkNeedsBothEndsOfAnEdge(t *testing.T) {
	g := geom.NodeLink(friends(), geom.From("who"))
	err := g.Train(geom.Training{X: scale.Linear(), Y: scale.Linear()})
	if !errors.Is(err, geom.ErrNoColumn) {
		t.Errorf("a layer with one channel gave %v, want ErrNoColumn", err)
	}
}

// subpathMiddles is the centre of each subpath of a path, which for a run of
// circles is a run of node positions.
func subpathMiddles(p *ir.Path) []ir.Point {
	var out []ir.Point
	var lo, hi ir.Point
	flush := func() {
		if len(out) > 0 || lo != (ir.Point{}) || hi != (ir.Point{}) {
			out = append(out, ir.Point{X: (lo.X + hi.X) / 2, Y: (lo.Y + hi.Y) / 2})
		}
	}
	first := true
	p.Walk(func(op ir.PathOp, pts []ir.Point) {
		if op == ir.OpMoveTo {
			if !first {
				flush()
			}
			first = false
			lo, hi = pts[0], pts[0]
		}
		for _, q := range pts {
			lo.X, lo.Y = min(lo.X, q.X), min(lo.Y, q.Y)
			hi.X, hi.Y = max(hi.X, q.X), max(hi.Y, q.Y)
		}
	})
	if !first {
		flush()
	}
	return out
}

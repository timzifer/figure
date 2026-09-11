package three

import (
	"fmt"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// Surface draws z = f(x, y) over a regular grid.
//
// It is the chart the third dimension is for. A heatmap of the same grid gives
// the values and hides which way the ground falls; a surface gives the shape
// of the response between its samples — a ridge, a saddle, the edge of a
// plateau. See docs/adr/0058-what-3d-is-for.md.
//
//	three.Surface(src, geom.X("x"), geom.Y("y"), geom.Z("gain"))
//
// The grid is required rather than guessed. The rows must be the full product
// of the distinct x and y values with each cell present exactly once, and
// anything else is an error out of Train rather than a picture with holes in
// it: a surface drawn over a scattered sample is a surface over a
// triangulation nobody asked for.
//
// Shading is one directional light from the theme and nothing else — the
// face's normal against [github.com/timzifer/figure/theme.Theme.LightDir],
// mixed in linear light. [github.com/timzifer/figure/geom.ColorBy] paints from
// the height through a ramp instead, which is the terrain reading, and the two
// compose: the ramp gives the height and the shade gives the slope. There is
// no colourbar beside it and none is missing — the depth axis is the key, and
// it is drawn with ticks and a title.
//
// Decimation is off, and [github.com/timzifer/figure/geom.Decimate] is
// accepted and ignored. A reduction defined over pixel columns measures
// nothing in a projected scene, where one column of screen mixes values from
// everywhere along the view direction; a grid the caller chose the size of is
// the caller's to choose smaller.
func Surface(src data.Source, opts ...geom.Option) Layer {
	return &surface{base: newBase(src, "surface", opts)}
}

type surface struct {
	base

	// The resolved grid, rebuilt on every Train into buffers it keeps — which
	// is the whole of this layer's share of the allocation gate, since Train
	// runs on every frame.
	//
	// It is [stat.Lattice] rather than this layer's own, because a surface, a
	// contour and a raster of a measured field all need the same resolver: two
	// that agree today disagree at the first duplicated position, and the
	// symptom of that would be a surface and its own contours that do not line
	// up. The error messages stay here, because a useful one names a mark and
	// stat knows about numbers.
	grid stat.Lattice
	ok   bool

	ramp scale.ColorScale
}

func (g *surface) Train(t geom.Training) error {
	g.ok = false
	if err := g.resolve(t); err != nil {
		return err
	}
	t.X.Train(g.grid.Xs...)
	t.Y.Train(g.grid.Ys...)
	t.Z.Train(g.grid.V...)
	if g.ramp != nil {
		g.ramp.Train(g.grid.V...)
	}
	g.ok = true
	return nil
}

// resolve turns a long table of (x, y, z) rows into the lattice a surface is.
//
// It runs in Train rather than in Emit because it is where an error can still
// be reported, and because the answer is the same for every view: the lattice
// is a fact about the data and only the order it is walked in is a fact about
// the camera.
func (g *surface) resolve(t geom.Training) error {
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
	g.ramp = g.cfg.ColorScale

	switch f := g.grid.Reset(xs, ys, zs); f {
	case stat.LatticeOK:
		return nil
	case stat.LatticeRagged:
		return fmt.Errorf("figure/three: the surface's columns have %d, %d and %d rows",
			len(xs), len(ys), len(zs))
	case stat.LatticeTooSmall:
		return fmt.Errorf("figure/three: a surface needs at least two distinct values on each floor axis, got %d and %d",
			len(g.grid.Xs), len(g.grid.Ys))
	case stat.LatticeWrongCount:
		nx, ny := len(g.grid.Xs), len(g.grid.Ys)
		return fmt.Errorf("figure/three: a surface needs one row per cell of its grid: "+
			"%d by %d is %d cells and the table has %d rows",
			nx, ny, nx*ny, len(zs))
	case stat.LatticeBadPosition:
		r := g.grid.At
		return fmt.Errorf("figure/three: row %d of the surface is at (%v, %v), which is not a position",
			r, xs[r], ys[r])
	default:
		a, b := g.grid.At, g.grid.With
		return fmt.Errorf("figure/three: rows %d and %d are both at (%v, %v); a surface has one value per cell",
			a, b, xs[b], ys[b])
	}
}

func (g *surface) Emit(s *Sink, f Frame) error {
	if !g.ok {
		return nil
	}
	nx, ny := len(g.grid.Xs), len(g.grid.Ys)

	track := rowsWanted(f)

	// The traversal starts at the corner the view direction picks and works
	// outward, which costs a comparison per axis and nothing per quad. The
	// order it produces is already the order the sort will confirm, so the
	// keys reaching the sort are in sequence and pdqsort walks them without a
	// swap.
	fwd := f.Forward
	base := g.fillFor(f)
	stroke, width := g.outline()

	for jj := 0; jj < ny-1; jj++ {
		j := stepFrom(jj, ny-1, fwd.Y)
		for ii := 0; ii < nx-1; ii++ {
			i := stepFrom(ii, nx-1, fwd.X)
			if !g.cellDefined(f, i, j) {
				continue
			}
			c := g.quad(f, i, j)
			fill := base
			if g.ramp != nil {
				fill = g.ramp.Color(g.mid(i, j))
			}
			if track {
				s.Row(int(g.grid.Row[j*nx+i]))
			}
			s.Face(c[:], Style{
				Fill:   shade(f.Theme, fill, faceNormal(c[0], c[1], c[2])),
				Stroke: stroke,
				Width:  width,
			})
		}
	}
	return nil
}

// quad is one cell of the lattice, corners in order around it.
func (g *surface) quad(f Frame, i, j int) [4]Vec3 {
	nx := len(g.grid.Xs)
	x0, x1 := at(f.X, g.grid.Xs[i]), at(f.X, g.grid.Xs[i+1])
	y0, y1 := at(f.Y, g.grid.Ys[j]), at(f.Y, g.grid.Ys[j+1])
	return [4]Vec3{
		{x0, y0, at(f.Z, g.grid.V[j*nx+i])},
		{x1, y0, at(f.Z, g.grid.V[j*nx+i+1])},
		{x1, y1, at(f.Z, g.grid.V[(j+1)*nx+i+1])},
		{x0, y1, at(f.Z, g.grid.V[(j+1)*nx+i])},
	}
}

// mid is the mean height of a cell's four corners, which is what a colour ramp
// over the surface reads.
func (g *surface) mid(i, j int) float64 {
	nx := len(g.grid.Xs)
	return (g.grid.V[j*nx+i] + g.grid.V[j*nx+i+1] + g.grid.V[(j+1)*nx+i] + g.grid.V[(j+1)*nx+i+1]) / 4
}

// cellDefined reports whether all four corners of a cell have a position. A
// hole in the data is a hole in the surface rather than a quad drawn to
// nowhere.
func (g *surface) cellDefined(f Frame, i, j int) bool {
	nx := len(g.grid.Xs)
	for _, k := range [4]int{j*nx + i, j*nx + i + 1, (j+1)*nx + i, (j+1)*nx + i + 1} {
		if !defined(f.Z, g.grid.V[k]) {
			return false
		}
	}
	return defined(f.X, g.grid.Xs[i]) && defined(f.X, g.grid.Xs[i+1]) &&
		defined(f.Y, g.grid.Ys[j]) && defined(f.Y, g.grid.Ys[j+1])
}

func (g *surface) Legend(f Frame) (geom.LegendEntry, bool) {
	return g.base.legend(f, geom.SwatchBox)
}

// stepFrom turns a loop counter into an index that walks away from the camera:
// forward when the view direction points along the axis, backward when it
// points against it. It is the whole of the traversal ADR 0056 describes.
func stepFrom(k, n int, f float32) int {
	if f < 0 {
		return n - 1 - k
	}
	return k
}

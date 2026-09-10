package three

import (
	"fmt"
	"sort"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
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

	// The resolved grid, rebuilt on every Train into buffers the layer keeps.
	//
	// Keeping them rather than allocating them is the whole of this layer's
	// share of the allocation gate: Train runs on every frame, so a map or a
	// slice made here would be a cost per row of a chart redrawn per pointer
	// move. The maps are cleared rather than replaced so that their buckets
	// survive too.
	xs, ys []float64
	z      []float64 // len(xs)*len(ys), row-major in y
	row    []int32   // the source row behind each cell
	xi, yi map[float64]int
	seen   map[float64]struct{}
	ok     bool

	ramp scale.ColorScale
}

func (g *surface) Train(t geom.Training) error {
	g.ok = false
	if err := g.resolve(t); err != nil {
		return err
	}
	t.X.Train(g.xs...)
	t.Y.Train(g.ys...)
	t.Z.Train(g.z...)
	if g.ramp != nil {
		g.ramp.Train(g.z...)
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
	if len(xs) != len(ys) || len(ys) != len(zs) {
		return fmt.Errorf("figure/three: the surface's columns have %d, %d and %d rows",
			len(xs), len(ys), len(zs))
	}
	g.ramp = g.cfg.ColorScale

	g.xs = g.distinctInto(g.xs[:0], xs)
	g.ys = g.distinctInto(g.ys[:0], ys)
	nx, ny := len(g.xs), len(g.ys)
	if nx < 2 || ny < 2 {
		return fmt.Errorf("figure/three: a surface needs at least two distinct values on each floor axis, got %d and %d",
			nx, ny)
	}
	if nx*ny != len(zs) {
		return fmt.Errorf("figure/three: a surface needs one row per cell of its grid: "+
			"%d by %d is %d cells and the table has %d rows",
			nx, ny, nx*ny, len(zs))
	}

	g.z = grow(g.z, nx*ny)
	g.row = grow(g.row, nx*ny)
	for i := range g.row {
		g.row[i] = -1
	}
	g.xi, g.yi = indexInto(g.xi, g.xs), indexInto(g.yi, g.ys)
	xi, yi := g.xi, g.yi
	for r := range zs {
		i, okX := xi[xs[r]]
		j, okY := yi[ys[r]]
		if !okX || !okY {
			return fmt.Errorf("figure/three: row %d of the surface is at (%v, %v), which is not on the grid",
				r, xs[r], ys[r])
		}
		if g.row[j*nx+i] >= 0 {
			return fmt.Errorf("figure/three: rows %d and %d are both at (%v, %v); a surface has one value per cell",
				g.row[j*nx+i], r, xs[r], ys[r])
		}
		g.z[j*nx+i], g.row[j*nx+i] = zs[r], int32(r)
	}
	return nil
}

func (g *surface) Emit(s *Sink, f Frame) error {
	if !g.ok {
		return nil
	}
	nx, ny := len(g.xs), len(g.ys)

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
				s.Row(int(g.row[j*nx+i]))
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
	nx := len(g.xs)
	x0, x1 := at(f.X, g.xs[i]), at(f.X, g.xs[i+1])
	y0, y1 := at(f.Y, g.ys[j]), at(f.Y, g.ys[j+1])
	return [4]Vec3{
		{x0, y0, at(f.Z, g.z[j*nx+i])},
		{x1, y0, at(f.Z, g.z[j*nx+i+1])},
		{x1, y1, at(f.Z, g.z[(j+1)*nx+i+1])},
		{x0, y1, at(f.Z, g.z[(j+1)*nx+i])},
	}
}

// mid is the mean height of a cell's four corners, which is what a colour ramp
// over the surface reads.
func (g *surface) mid(i, j int) float64 {
	nx := len(g.xs)
	return (g.z[j*nx+i] + g.z[j*nx+i+1] + g.z[(j+1)*nx+i] + g.z[(j+1)*nx+i+1]) / 4
}

// cellDefined reports whether all four corners of a cell have a position. A
// hole in the data is a hole in the surface rather than a quad drawn to
// nowhere.
func (g *surface) cellDefined(f Frame, i, j int) bool {
	nx := len(g.xs)
	for _, k := range [4]int{j*nx + i, j*nx + i + 1, (j+1)*nx + i, (j+1)*nx + i + 1} {
		if !defined(f.Z, g.z[k]) {
			return false
		}
	}
	return defined(f.X, g.xs[i]) && defined(f.X, g.xs[i+1]) &&
		defined(f.Y, g.ys[j]) && defined(f.Y, g.ys[j+1])
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

// distinctInto appends the sorted distinct values of a column to dst, which is
// one axis of the lattice. It borrows the layer's own seen-map so that a frame
// costs nothing per row.
func (g *surface) distinctInto(dst, vs []float64) []float64 {
	if g.seen == nil {
		g.seen = make(map[float64]struct{}, len(vs))
	}
	clear(g.seen)
	for _, v := range vs {
		if _, ok := g.seen[v]; ok {
			continue
		}
		g.seen[v] = struct{}{}
		dst = append(dst, v)
	}
	sort.Float64s(dst)
	return dst
}

// indexInto fills m with the position of each value, clearing rather than
// replacing it so that its buckets survive between frames.
func indexInto(m map[float64]int, vs []float64) map[float64]int {
	if m == nil {
		m = make(map[float64]int, len(vs))
	}
	clear(m)
	for i, v := range vs {
		m[v] = i
	}
	return m
}

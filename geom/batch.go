package geom

import (
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// colorRun is a set of marks that share a colour.
type colorRun struct {
	color ir.Color
	pts   []ir.Point
}

// groupByColor batches marks by colour.
//
// The IR carries one style per drawing call and does not have a per-vertex
// colour channel; adding one would touch every backend for the sake of two
// geoms. Grouping instead costs one call per distinct colour, which for a
// continuous ramp over real data is far fewer calls than there are points, and
// for a handful of categories is a handful. See docs/adr/0007.
//
// Groups come out in order of first appearance, so a render is reproducible.
// Each group's point buffer is kept between frames — which is why the runs are
// held full-length in the scratch and only the used prefix is returned.
func (sc *scratch) groupByColor(pts []ir.Point, cols []ir.Color) []colorRun {
	if sc.at == nil {
		sc.at = make(map[ir.Color]int, 8)
	}
	clear(sc.at)
	n := 0
	for i, p := range pts {
		c := cols[i]
		j, ok := sc.at[c]
		if !ok {
			j = n
			n++
			if j == len(sc.runs) {
				sc.runs = append(sc.runs, colorRun{})
			}
			sc.runs[j].color = c
			sc.runs[j].pts = sc.runs[j].pts[:0]
			sc.at[c] = j
		}
		sc.runs[j].pts = append(sc.runs[j].pts, p)
	}
	return sc.runs[:n]
}

// indexRun is a set of marks that share a colour, listed by their position in
// the layer's own arrays rather than by their points.
//
// It is [colorRun] for a mark that carries more than a position. A bubble has a
// centre *and* a diameter, and a run of points alone would lose which diameter
// belongs to which centre — so the run carries the indices and the caller reads
// both arrays through them.
type indexRun struct {
	color ir.Color
	idx   []int
}

// groupByColorAt batches marks by colour, in order of first appearance,
// reporting each run as a list of indices into cols.
func (sc *scratch) groupByColorAt(cols []ir.Color) []indexRun {
	if sc.at == nil {
		sc.at = make(map[ir.Color]int, 8)
	}
	clear(sc.at)
	n := 0
	for i, c := range cols {
		j, ok := sc.at[c]
		if !ok {
			j = n
			n++
			if j == len(sc.iruns) {
				sc.iruns = append(sc.iruns, indexRun{})
			}
			sc.iruns[j].color = c
			sc.iruns[j].idx = sc.iruns[j].idx[:0]
			sc.at[c] = j
		}
		sc.iruns[j].idx = append(sc.iruns[j].idx, i)
	}
	return sc.iruns[:n]
}

// rectRun is a set of cells that share a colour. It is [colorRun] for a mark
// that is a box rather than a point, and it exists for the same reason: the IR
// carries one style per drawing call, so a layer of a thousand differently
// coloured cells is a call per distinct colour rather than a call per cell.
type rectRun struct {
	color ir.Color
	rects []ir.Rect
	// offs is how far each of the run's cells is broken out of the middle of
	// the coord, index for index with rects. It is empty for a layer that is
	// not broken out, which is every layer that never asked — read it through
	// [offsetAt], which is why that tests the length rather than nil.
	offs []ir.Point
}

// groupByRect batches cells by colour, in order of first appearance. offs is
// the displacement of each cell, or nil.
func (sc *scratch) groupByRect(rects []ir.Rect, cols []ir.Color, offs []ir.Point) []rectRun {
	if sc.at == nil {
		sc.at = make(map[ir.Color]int, 8)
	}
	clear(sc.at)
	n := 0
	for i, r := range rects {
		c := cols[i]
		j, ok := sc.at[c]
		if !ok {
			j = n
			n++
			if j == len(sc.rruns) {
				sc.rruns = append(sc.rruns, rectRun{})
			}
			sc.rruns[j].color = c
			sc.rruns[j].rects = sc.rruns[j].rects[:0]
			sc.rruns[j].offs = sc.rruns[j].offs[:0]
			sc.at[c] = j
		}
		sc.rruns[j].rects = append(sc.rruns[j].rects, r)
		if offs != nil {
			sc.rruns[j].offs = append(sc.rruns[j].offs, offs[i])
		}
	}
	return sc.rruns[:n]
}

// trainColors feeds the colour column into the colour scale. It is separate
// from training the positional scales because a colour scale is not a Scale:
// it maps to paint, not to a position.
func (c config) trainColors(s series) {
	if c.colorScale == nil || s.c == nil {
		return
	}
	c.colorScale.Train(s.c...)
	if bv, ok := scale.Bivariate(c.colorScale); ok && s.c2 != nil {
		bv.TrainSecond(s.c2...)
	}
}

// colorsFor resolves the per-mark colours for the rows in idx, or nil if this
// layer is a single colour. It writes into the scratch's buffer, so a chart
// redrawn every frame recolours the same memory.
func (sc *scratch) colorsFor(c config, s series, idx []int) []ir.Color {
	if c.colorScale == nil || s.c == nil {
		return nil
	}
	sc.cols = grow(sc.cols, len(idx))
	if bv, ok := scale.Bivariate(c.colorScale); ok && s.c2 != nil {
		for i, row := range idx {
			sc.cols[i] = bv.ColorAt(s.c[row], s.c2[row])
		}
		return sc.cols
	}
	for i, row := range idx {
		sc.cols[i] = c.colorScale.Color(s.c[row])
	}
	return sc.cols
}

// varying reports whether this layer paints each mark from a colour scale.
func (c config) varying(s series) bool { return c.colorScale != nil && s.c != nil }

// trainSizes feeds the size column into the size scale, for the same reason
// [config.trainColors] exists: a size scale is not a Scale, it maps to ink
// rather than to a position.
func (c config) trainSizes(s series) {
	if c.sizeScale == nil || s.sz == nil {
		return
	}
	c.sizeScale.Train(s.sz...)
}

// sizing reports whether this layer takes each mark's size from a column.
func (c config) sizing(s series) bool { return c.sizeScale != nil && s.sz != nil }

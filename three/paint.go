package three

import (
	"cmp"
	"slices"

	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/render"
)

// primKey is one primitive's place in the depth order.
//
// The sort runs over these rather than over a closure capturing the primitive
// list, so that the comparison is a top-level function and the hot path
// allocates nothing at all. [slices.SortFunc] rather than sort.Slice for the
// same reason: that one takes a closure and a reflect-based swapper and
// allocates on every call. ADR 0057 decided both.
type primKey struct {
	depth float32
	idx   int32
}

// byDepth orders the primitives of a whole scene, farthest first.
//
// This is the single depth order ADR 0056 asks for, over the primitives of
// every layer at once — because a point can be in front of one part of a
// surface and behind another, so no per-layer ordering can be right. **One
// order means one formula**: two layers keyed by two different measures of
// depth are two numbers that cannot be compared, and merging them would be
// arithmetic rather than geometry.
//
// The key is the depth of the primitive's centroid along the view direction,
// and nothing else — see [Sink.depthOf] for what that is a sample of and where
// sampling it is enough.
//
// Ties break by emission index, which is total and free: layers emit one after
// another, so the index is lexicographically (layer, the layer's own order),
// which is ADR 0012's rule without needing a second field. Nothing here
// depends on scheduling, and there is no hysteresis: two marks whose depths
// differ by a millionth swap legitimately as the camera turns, and that is
// what turning past each other looks like. A picture that depended on which
// frames preceded it could not be golden-tested.
func byDepth(a, b primKey) int {
	if c := cmp.Compare(b.depth, a.depth); c != 0 {
		return c
	}
	return cmp.Compare(a.idx, b.idx)
}

// paint projects, orders and draws everything the layers emitted.
//
// It is the one drawing order in this package, and it is why the package
// exists: render's order is per layer, and one per layer is exactly what a
// projected scene cannot have.
func (s *Sink) paint(b ir.Backend, pr projector, obs render.Observer, panel int, labels []string, rows geom.Rows) {
	if len(s.prims) == 0 {
		return
	}
	// An observer that wants to know how far away each mark is, for the reason
	// render.DepthObserver gives. Resolved once rather than per primitive.
	deep, _ := obs.(render.DepthObserver)

	s.pts = grow(s.pts, len(s.verts))[:0]
	for _, v := range s.verts {
		s.pts = append(s.pts, pr.point(v))
	}

	s.order = grow(s.order, len(s.prims))[:0]
	for i := range s.prims {
		p := &s.prims[i]
		s.order = append(s.order, primKey{depth: p.depth, idx: int32(i)})
	}
	slices.SortFunc(s.order, byDepth)

	open := -1
	for i := 0; i < len(s.order); {
		p := &s.prims[s.order[i].idx]
		if int(p.layer) != open {
			s.flushRows(rows)
			open = int(p.layer)
			if obs != nil {
				obs.Layer(render.LayerInfo{Index: open, Label: labelAt(labels, open)})
			}
		}
		switch p.kind {
		case kindText:
			run := s.runs[p.text]
			run.At = s.pts[p.lo]
			b.Text(run)
			i++
		case kindLine:
			s.line = grow(s.line, int(p.hi-p.lo))[:0]
			s.line = append(s.line, s.pts[p.lo:p.hi]...)
			if deep != nil {
				deep.Depth(float64(p.depth))
			}
			b.Polyline(s.line, ir.Stroke{Color: p.style.Stroke, Width: p.style.Width})
			s.noteRow(p)
			i++
		default:
			i = s.faces(b, deep, i)
		}
	}
	s.flushRows(rows)
}

// faces draws one face, or a run of adjacent faces that are provably the same
// picture drawn together, and returns where it got to.
//
// # When faces may be merged
//
// Batching is by *run* rather than by colour, and the difference is the point
// of this package. A flat chart batches every mark of one colour into one call
// because order within a layer does not matter; here order is the thing being
// computed, so only primitives already adjacent in it could be merged at all.
//
// Adjacency is not enough by itself, though, because two adjacent faces in the
// order can still overlap on screen — and then merging them is only the same
// picture under two further conditions:
//
//   - **The fill is opaque.** One path filled once covers its union in one
//     colour; several overlapping fills composite in the overlap. For an
//     opaque colour those are the same picture and for a translucent one they
//     are not.
//   - **Nothing is stroked.** A run draws all its fills and then all its
//     outlines, so a face early in the run — the farther one — would have its
//     outline drawn over a face later in it. That is the painter's order
//     undone by the optimisation meant to be invisible.
//
// So an outlined or translucent face is drawn on its own, in its place in the
// order, and the merge is kept for the case it is free in. Either way each
// face is its own subpath, so a hit index still sees one mark per quad.
func (s *Sink) faces(b ir.Backend, deep render.DepthObserver, i int) int {
	first := &s.prims[s.order[i].idx]

	j := i + 1
	if mergeable(first.style) {
		for j < len(s.order) {
			p := &s.prims[s.order[j].idx]
			if p.kind != kindFace || p.layer != first.layer || p.style != first.style {
				break
			}
			j++
		}
	}

	s.path.Reset()
	for k := i; k < j; k++ {
		p := &s.prims[s.order[k].idx]
		pts := s.pts[p.lo:p.hi]
		s.path.MoveTo(pts[0].X, pts[0].Y)
		for _, q := range pts[1:] {
			s.path.LineTo(q.X, q.Y)
		}
		s.path.Close()
		// Once per subpath, in the order the subpaths are appended, which is
		// the order an index walking them will see. A merged run is several
		// depths in one call, so saying it per call would be saying the
		// farthest one for all of them.
		if deep != nil {
			deep.Depth(float64(p.depth))
		}
		s.noteRow(p)
	}

	if first.style.Fill.A != 0 {
		b.FillPath(&s.path, ir.Solid(first.style.Fill), ir.NonZero)
	}
	// The outline is drawn only when a layer asked for one, and that is not a
	// default worth changing: a hit index ranks a stroked vertex above the
	// area it outlines, so an always-outlined mesh would report a corner on
	// every hover over a surface. It is geom.Rect's rule, for geom.Rect's
	// reason.
	if first.style.Stroke.A != 0 && first.style.Width > 0 {
		b.StrokePath(&s.path, ir.Stroke{Color: first.style.Stroke, Width: first.style.Width})
	}
	return j
}

// mergeable reports whether faces in this style may be drawn together without
// changing the picture. See [Sink.faces].
func mergeable(st Style) bool {
	return st.Fill.A == 255 && (st.Stroke.A == 0 || st.Width <= 0)
}

// noteRow records where a primitive that carries a source row landed, for the
// index to attribute a hit with. It costs nothing when nobody is listening.
func (s *Sink) noteRow(p *prim) {
	if p.row < 0 {
		return
	}
	var c ir.Point
	pts := s.pts[p.lo:p.hi]
	for _, q := range pts {
		c.X, c.Y = c.X+q.X, c.Y+q.Y
	}
	n := float32(len(pts))
	s.rowAt = append(s.rowAt, ir.Point{X: c.X / n, Y: c.Y / n})
	s.rowNo = append(s.rowNo, int(p.row))
	// The depth the painter already sorted by, so that a host can tell whether
	// the reader can actually see the row it was just told about.
	s.rowZ = append(s.rowZ, float64(p.depth))
}

// flushRows reports the rows collected since the last layer change.
//
// It has to be per layer rather than per view, because a hit index attributes
// what it is told to the layer that is open — and in a merged depth order the
// layers interleave, so a layer opens more than once per view.
func (s *Sink) flushRows(rows geom.Rows) {
	if rows != nil && len(s.rowAt) > 0 {
		rows.Marks(geom.MarkRows{At: s.rowAt, Rows: s.rowNo, Depth: s.rowZ})
	}
	s.rowAt, s.rowNo, s.rowZ = s.rowAt[:0], s.rowNo[:0], s.rowZ[:0]
}

func labelAt(labels []string, i int) string {
	if i < 0 || i >= len(labels) {
		return ""
	}
	return labels[i]
}

package three

import "github.com/timzifer/figure/ir"

// Style is the paint of one primitive.
//
// It carries no dash. A dash is a length on screen, and a projected run's
// length on screen is not its length in the data, so a dashed line in a scene
// says less than it appears to; when one is asked for, this gains a field.
// Being comparable is what lets adjacent primitives batch into one drawing
// call with ==.
type Style struct {
	// Fill is the interior colour, and is transparent for a line.
	Fill ir.Color
	// Stroke is the outline colour, and is transparent for a face drawn
	// without one.
	Stroke ir.Color
	// Width is the outline's width.
	Width float32
}

// primKind is what a primitive is. Three kinds ship; a marker is the fourth
// and arrives with the 3D scatter.
type primKind uint8

const (
	kindFace primKind = iota
	kindLine
	kindText
)

// prim is one primitive: a range in the sink's vertex arena, its ink, and the
// key it is ordered by.
type prim struct {
	kind   primKind
	lo, hi int32
	style  Style
	// depth is how far the primitive's centroid is from the camera along the
	// view direction. See [Sink.depthOf].
	//
	// It is computed in float64 and kept in float32 deliberately: the
	// narrowing turns a near-tie into an exact tie, and an exact tie is broken
	// by emission order, which is the same on every architecture. A float64
	// key would let two primitives a ulp apart order one way on arm64 and the
	// other on amd64, and the golden files tolerate exactly that much
	// coordinate difference — so the reorder would not be caught. AGENTS.md
	// records the same bug in its other two places.
	depth float32
	row   int32
	layer int32
	// text indexes the sink's run list for a kindText primitive, and is -1
	// for the others.
	text int32
}

// Sink collects the primitives of one view of a scene, in scene space.
//
// Vertices go into one flat arena and a primitive is a range in it, which is
// what makes a frame cost a handful of allocations over a surface of any size:
// the arena, the primitive list and the sort's key slice all come from a pool
// and are refilled rather than rebuilt. ADR 0057 pre-commits this package to
// that discipline and the allocation gate holds it to it.
//
// A Sink is handed to a layer's [Layer.Emit] and is not retained afterwards.
type Sink struct {
	verts []Vec3
	prims []prim
	runs  []ir.TextRun

	// pts is verts projected, filled in one pass before painting; the rest is
	// the painter's own scratch, kept here so that it is pooled with
	// everything else.
	pts   []ir.Point
	order []primKey
	path  ir.Path
	line  []ir.Point
	rowAt []ir.Point
	rowNo []int
	// boxes is the cube's label-collision scratch, kept here so that it is
	// pooled with everything else rather than made per view per frame.
	boxes []ir.Rect

	// The state a layer sets and this package resets between layers.
	layer int32
	row   int32
	fwd   Vec3
}

// Row attributes the primitives that follow to a source row, or to -1 for one
// that no row is behind — an interpolated point, a wall, a summary. It holds
// until it is changed and is reset to -1 for each layer.
//
// It costs nothing when nobody is listening: a scene rendered without row
// tracking never reads what a layer reported.
func (s *Sink) Row(i int) { s.row = int32(i) }

// Forward is the unit direction the camera looks along, which is the same
// vector [Frame.Forward] carries. It is here as well so that a helper handed
// only a sink can pick its traversal.
func (s *Sink) Forward() Vec3 { return s.fwd }

// Face appends a filled face: three or more corners, in order around it.
//
// The face is drawn as one closed subpath, which is what makes it one mark to
// a hit index — a quad of a surface is a thing the reader can point at.
func (s *Sink) Face(vs []Vec3, st Style) {
	if len(vs) < 3 {
		return
	}
	s.append(kindFace, vs, st, -1)
}

// Line appends a stroked run through vs.
//
// A layer that draws a path emits one Line per segment rather than one per
// path. A whole path is one primitive with one depth, and a curve that spans
// the scene has no single depth — so a trajectory that crossed a surface
// would be drawn wholly in front of it or wholly behind.
func (s *Sink) Line(vs []Vec3, st Style) {
	if len(vs) < 2 {
		return
	}
	s.append(kindLine, vs, st, -1)
}

// Text appends a run anchored at a scene point.
//
// Only the anchor is projected. A label lying in a projected plane needs a
// shear and [ir.TextRun] has none — deliberately, since a backend shapes its
// own runs — so the text stays upright, which is also easier to read than the
// sheared alternative every serious 3D tool ends up offering to turn off.
func (s *Sink) Text(at Vec3, run ir.TextRun) {
	s.runs = append(s.runs, run)
	s.append(kindText, []Vec3{at}, Style{}, int32(len(s.runs)-1))
}

func (s *Sink) append(kind primKind, vs []Vec3, st Style, text int32) {
	lo := int32(len(s.verts))
	s.verts = append(s.verts, vs...)
	hi := int32(len(s.verts))
	s.prims = append(s.prims, prim{
		kind: kind, lo: lo, hi: hi, style: st,
		depth: s.depthOf(lo, hi), row: s.row, layer: s.layer, text: text,
	})
}

// depthOf is how far a primitive's centroid is from the camera, along the view
// direction. Larger is farther.
//
// It is one number and the same number for every primitive of every layer,
// which is what makes a scene of several layers orderable at all: two layers
// keyed by two different measures of depth are two numbers on two scales, and
// merging them is arithmetic rather than geometry.
//
// # What a centroid is enough for, exactly
//
// A centroid is a *sample*, and a primitive covers a range of depths rather
// than one. So the promise is precise and it is smaller than "correct":
//
//	Two primitives are ordered correctly whenever their depth ranges are
//	disjoint — when a plane across the view direction separates them.
//
// That is what ADR 0056's "exact only when the pieces can be totally ordered"
// means once both pieces are extended rather than points, and it is what
// [TestPrimitivesSeparatedInDepthAreAlwaysOrderedCorrectly] pins. Where two
// primitives' depth ranges *interleave*, no per-primitive number can decide
// between them, and this package does not split them apart to find out:
// splitting is a BSP tree, which is a renderer, and ADR 0056 refuses one.
//
// What keeps real charts inside the promise is that the shapes emitted here
// are already small. A surface reaches the painter as one quad per cell, a
// field of bars as one face per side, a path as one primitive per segment —
// so a primitive's depth range is a cell wide rather than a scene wide. The
// case to know about is the one that leaves it: **several layers stacked over
// a grid coarse enough that one cell spans more depth than the layers are
// apart**. Draw that with a finer grid, or give each layer its own [View],
// which is what several cameras on one scene are for.
//
// # What this is not
//
// An earlier version keyed on the centroid dropped to the floor, on the
// argument that it varies less over a steep primitive and is monotone along
// every view ray. Both halves are true and the conclusion does not follow:
// monotone along *a* ray says nothing about two centroids, which lie on two
// different rays. Two sheets at different heights whose footprints overlap
// without coinciding come out backwards — see
// [TestASheetIsNotPaintedOverTheOneInFrontOfIt], which is that case with the
// numbers in it.
func (s *Sink) depthOf(lo, hi int32) float32 {
	var c Vec3
	for _, v := range s.verts[lo:hi] {
		c = c.Add(v)
	}
	c = c.Mul(1 / float32(hi-lo))
	return float32(c.Sub(centre).Dot(s.fwd))
}

// openLayer starts a layer's emission: the state a layer may set is reset, so
// that a layer that sets nothing gets the documented default rather than the
// previous layer's.
func (s *Sink) openLayer(i int, fwd Vec3) {
	s.layer, s.row, s.fwd = int32(i), -1, fwd
}

func (s *Sink) reset() {
	s.verts = s.verts[:0]
	s.prims = s.prims[:0]
	s.runs = s.runs[:0]
	s.pts = s.pts[:0]
	s.order = s.order[:0]
	s.path.Reset()
	s.line = s.line[:0]
	s.rowAt = s.rowAt[:0]
	s.rowNo = s.rowNo[:0]
	s.boxes = s.boxes[:0]
	s.layer, s.row, s.fwd = 0, -1, Vec3{}
}

package geom

import (
	"errors"
	"fmt"
	"math"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// What a schedule needs beyond a rectangle, and why it is one file.
//
// A gantt chart has been a [Rect] on a time axis against an ordinal one since
// there was a rect mark, and that draws every span in the plan correctly. What
// it does not draw is the two things a reader opens a schedule to find out:
// how far each task has got, and which task is waiting on which. Both are
// statements about a *span* rather than about a rectangle, so both live here:
// [ProgressBy], which a rect honours, and [Depends], which joins two of them.
//
// See docs/adr/0068-gantt-charts.md.

// unfinishedOpacity is the alpha the unfinished part of a cell is drawn at,
// as a fraction of the colour the cell would have had.
//
// It is a constant rather than an option because it is not a choice about this
// chart: it is the one relationship the mark asserts, that the pale part of a
// bar is the same task as the solid part and not a different colour. A caller
// who wants a different look draws two rect layers, which is what this saves
// them from.
const unfinishedOpacity = 0.3

// ErrProgressAxis reports a [ProgressBy] layer whose rows name both a far
// horizontal edge and a far vertical one.
//
// A fraction of a cell runs along one axis, and a cell bounded on both by its
// own row gives no reason to prefer either. It is the rule [ErrBothAxes]
// already draws for an error bar's orientation, and it is an error rather than
// a guess for the same reason: a guess that depends on the order the options
// were written is a chart that changes when somebody tidies a line.
var ErrProgressAxis = errors.New("figure/geom: a cell bounded on both axes has no single axis for its progress; name one pair of edges, or drop geom.ProgressBy")

// ErrNoKey reports a [Depends] layer with no column to resolve its links
// against. A dependency names two tasks, and a task with no name cannot be one
// of them.
var ErrNoKey = errors.New("figure/geom: this mark joins rows by name; give the task table an identity with geom.KeyBy")

// Linkage is which edges of two spans a dependency joins.
//
// The four are the schedule vocabulary, and each is the same sentence with two
// blanks filled in: the successor cannot *start or finish* until the
// predecessor has *started or finished*. [FinishToStart] is what "depends on"
// means when nobody said otherwise, and it is the zero value for that reason.
type Linkage uint8

// The four linkages, named from the predecessor's edge to the successor's.
const (
	// FinishToStart runs from the predecessor's far edge to the successor's
	// near one: the successor waits for it to be over. It is the default.
	FinishToStart Linkage = iota
	// StartToStart runs near edge to near edge: the two begin together, or the
	// successor begins no earlier.
	StartToStart
	// FinishToFinish runs far edge to far edge: the successor cannot be over
	// until the predecessor is.
	FinishToFinish
	// StartToFinish runs from the predecessor's near edge to the successor's
	// far one. It is the rare one, and it is here because a vocabulary with a
	// hole in it is one a reader has to check.
	StartToFinish
)

// String names the linkage as a schedule spells it: the two-letter form, which
// is what a document writes down and what [LinkBy] reads.
func (k Linkage) String() string {
	switch k {
	case StartToStart:
		return "ss"
	case FinishToFinish:
		return "ff"
	case StartToFinish:
		return "sf"
	}
	return "fs"
}

// LinkageNamed is the linkage a document names, and ok == false for a name
// this package does not have.
func LinkageNamed(s string) (Linkage, bool) {
	for _, k := range []Linkage{FinishToStart, StartToStart, FinishToFinish, StartToFinish} {
		if s == k.String() {
			return k, true
		}
	}
	return FinishToStart, false
}

// from reports whether the linkage leaves the predecessor's far edge.
func (k Linkage) from() bool { return k == FinishToStart || k == FinishToFinish }

// to reports whether the linkage arrives at the successor's far edge.
func (k Linkage) to() bool { return k == FinishToFinish || k == StartToFinish }

// Depends draws the arrows between spans that a schedule's constraints are.
//
// It reads two tables, because a dependency is a statement about two rows of
// one table and belongs in a table of its own: tasks carries the spans, named
// by [KeyBy] and placed by exactly the channels the [Rect] layer beside it was
// given, and links carries one row per constraint, naming its two ends with
// [From] and [To].
//
//	p.Add(geom.Rect(tasks, geom.X("start"), geom.X2("end"), geom.Y("task"),
//	    geom.ProgressBy("done")))
//	p.Add(geom.Depends(tasks, links, geom.X("start"), geom.X2("end"), geom.Y("task"),
//	    geom.KeyBy("id"), geom.From("before"), geom.To("after")))
//
// The positional channels are read from tasks and everything about the link —
// [From], [To], [LinkBy] and [ColorBy] — from links. That asymmetry is the
// whole shape of the mark and it is why the two sources are two arguments
// rather than one: a row of the link table has no position of its own, and a
// row of the task table has no other end.
//
// [ColorBy] over the link table is how a critical path is drawn, and it is
// worth saying out loud because it needs nothing else: colour the links whose
// slack is zero and the chart has a critical path in it.
//
//	p.Add(geom.Depends(tasks, links, …,
//	    geom.ColorBy("critical", scale.Qualitative(palette.Default))))
//
// A link naming a task the table does not hold is dropped rather than refused.
// That is what makes the mark survive a facet: faceting cuts the task table,
// and a constraint whose other end is in the next panel has nothing to point
// at there.
//
// A hit on an arrow names the panel and the layer and not the constraint. The
// row it would report belongs to the link table, and [Geom.Source] hands out
// the other one — so the mark reports no rows at all rather than row numbers
// into a table nobody holds.
//
// The route between two spans is orthogonal and is computed in device space,
// which is the one place in this package a geom does not work in mapped
// coordinates. The reason is that there is nothing in data space to work in:
// the finish of one task and the start of another are two positions, and the
// path a reader's eye takes between them is a reading aid rather than a
// statement about any value in between. The two ends go through the coord like
// every other mark, so they land on the bars wherever the coord put them; the
// elbow between them is drawn on the screen.
func Depends(tasks, links data.Source, opts ...Option) Geom {
	return &dependsGeom{src: tasks, links: links, cfg: newConfig(opts)}
}

type dependsGeom struct {
	src   data.Source
	links data.Source
	cfg   config

	s  series    // the tasks' near edge and lane
	x2 []float64 // the tasks' far edge on the horizontal axis
	at map[string]int

	from, to []int     // per link, the task row it names, or -1
	kinds    []Linkage // per link, resolved once in Train
	ls       series    // the link table's colour column, and its origins

	horizontal bool
	// halfX and halfY are the slot a row that named only one edge gets, in
	// data units. They are measured in Train rather than in Build for the
	// reason barGeom.gap is: smallestGap sorts, and asking it per frame is a
	// sort per frame over the whole column.
	halfX, halfY float64
	gaps         []float64
	err          error
}

func (g *dependsGeom) Train(t Training) error {
	g.err = g.train(t)
	return g.err
}

func (g *dependsGeom) train(t Training) error {
	x, y := t.X, t.Y
	if g.src == nil || g.links == nil {
		return errors.New("figure/geom: nil data source")
	}
	if g.cfg.keyCol == "" {
		return ErrNoKey
	}
	var err error
	if g.s.x, err = column(g.src, g.cfg.xcol, x); err != nil {
		return err
	}
	if g.s.y, err = column(g.src, g.cfg.ycol, y); err != nil {
		return err
	}
	if g.cfg.x2col != "" {
		if g.x2, err = column(g.src, g.cfg.x2col, x); err != nil {
			return err
		}
		if len(g.x2) != len(g.s.x) {
			return errLength(g.cfg.xcol, g.cfg.x2col, len(g.s.x), len(g.x2))
		}
	}
	if g.cfg.y2col != "" {
		if g.s.y2, err = column(g.src, g.cfg.y2col, y); err != nil {
			return err
		}
		if len(g.s.y2) != len(g.s.y) {
			return errLength(g.cfg.ycol, g.cfg.y2col, len(g.s.y), len(g.s.y2))
		}
	}
	if len(g.s.y) != len(g.s.x) {
		return errLength(g.cfg.xcol, g.cfg.ycol, len(g.s.x), len(g.s.y))
	}
	// Which axis the arrow runs along is the encoding, exactly as it is for an
	// error bar and for a rect's own slot: the axis whose edges the row named
	// is the axis the span lies on. Naming both is a gantt in two directions
	// at once.
	switch {
	case g.x2 != nil && g.s.y2 != nil:
		return ErrProgressAxis
	case g.s.y2 != nil:
		g.horizontal = false
	default:
		g.horizontal = true
	}

	labels, ok := data.Labels(g.src, g.cfg.keyCol)
	if !ok {
		return fmt.Errorf("%w: %q", ErrNoColumn, g.cfg.keyCol)
	}
	if len(labels) != len(g.s.x) {
		return errLength(g.cfg.xcol, g.cfg.keyCol, len(g.s.x), len(labels))
	}
	if g.at == nil {
		g.at = make(map[string]int, len(labels))
	}
	// The map is cleared rather than replaced between frames, so a chart
	// redrawn every frame keeps its buckets — the same reason the interner in
	// relational.go does it. The *first* row to claim a name keeps it: a
	// duplicate identity is a table that disagrees with itself, and taking the
	// first is the only answer that does not depend on how far the reader got.
	clear(g.at)
	for i, l := range labels {
		if _, seen := g.at[l]; !seen {
			g.at[l] = i
		}
	}

	if err := g.resolveLinks(); err != nil {
		return err
	}

	trainColumn(x, g.s.x)
	trainColumn(y, g.s.y)
	if g.x2 != nil {
		trainColumn(x, g.x2)
	}
	if g.s.y2 != nil {
		trainColumn(y, g.s.y2)
	}
	g.cfg.trainColors(g.ls)
	g.halfX, g.halfY = g.halfWidth(g.s.x), g.halfWidth(g.s.y)
	return nil
}

// resolveLinks turns the link table into a pair of task rows and a linkage per
// row. A link naming a task nothing in the table is called is left at -1 and
// dropped when it is drawn.
func (g *dependsGeom) resolveLinks() error {
	if g.cfg.fromCol == "" || g.cfg.toCol == "" {
		return fmt.Errorf("%w: this mark reads a link table; name both ends with geom.From and geom.To", ErrNoColumn)
	}
	fromLabels, ok := data.Labels(g.links, g.cfg.fromCol)
	if !ok {
		return fmt.Errorf("%w: %q", ErrNoColumn, g.cfg.fromCol)
	}
	toLabels, ok := data.Labels(g.links, g.cfg.toCol)
	if !ok {
		return fmt.Errorf("%w: %q", ErrNoColumn, g.cfg.toCol)
	}
	if len(fromLabels) != len(toLabels) {
		return errLength(g.cfg.fromCol, g.cfg.toCol, len(fromLabels), len(toLabels))
	}
	n := len(fromLabels)
	g.from, g.to = grow(g.from, n), grow(g.to, n)
	g.kinds = grow(g.kinds, n)

	var kinds []string
	if g.cfg.linkCol != "" {
		if kinds, ok = data.Labels(g.links, g.cfg.linkCol); !ok {
			return fmt.Errorf("%w: %q", ErrNoColumn, g.cfg.linkCol)
		}
		if len(kinds) != n {
			return errLength(g.cfg.fromCol, g.cfg.linkCol, n, len(kinds))
		}
	}
	row := func(name string) int {
		if i, seen := g.at[name]; seen {
			return i
		}
		return -1
	}
	for i := range n {
		g.from[i], g.to[i] = row(fromLabels[i]), row(toLabels[i])
		g.kinds[i] = g.cfg.linkage
		if kinds != nil {
			// A name this package does not have takes the layer's own
			// linkage rather than failing: a document from a later version
			// then draws a chart that is wrong in a place the reader can see
			// rather than one that does not draw. It is the rule a track's
			// edge name already follows.
			if k, known := LinkageNamed(kinds[i]); known {
				g.kinds[i] = k
			}
		}
	}

	g.ls = series{}
	if g.cfg.colorCol == "" || g.cfg.colorScale == nil {
		return nil
	}
	cs, err := colorColumn(g.links, g.cfg)
	if err != nil {
		return err
	}
	if len(cs) != n {
		return errLength(g.cfg.fromCol, g.cfg.colorCol, n, len(cs))
	}
	g.ls.c = cs
	return nil
}

// halfWidth is how far a slot-sized edge reaches on each side of its value, in
// data units. It is [rectGeom.halfWidth], measured the same way so that an
// arrow lands on the edge of the bar the rect layer beside it drew.
func (g *dependsGeom) halfWidth(vs []float64) float64 {
	gap, buf := smallestGap(g.gaps, vs)
	g.gaps = buf
	f := g.cfg.barWidth
	if f <= 0 || f > 1 {
		f = 1
	}
	return gap * f / 2
}

// DefaultArrowSize is the length of a dependency arrow's head, in
// device-independent pixels. [Size] replaces it.
const DefaultArrowSize = 7

// dependStub is how far an arrow leaves a span before it turns, in
// device-independent pixels. It is a length on the screen rather than in the
// data because it is there so that the corner can be seen: an elbow that turned
// on the edge of the bar would read as a line touching it.
const dependStub = 10

func (g *dependsGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	sc := acquire(f)
	defer sc.release()

	cd := f.Coords()

	// Both ends are resolved before anything is drawn, so that a link with one
	// end missing costs nothing but a comparison and the batching below sees
	// only links that will be painted.
	links := sc.links[:0]
	rows := sc.rows[:0]
	for i := range g.from {
		a, c := g.from[i], g.to[i]
		if a < 0 || c < 0 || a == c {
			continue
		}
		p, out, ok := g.anchor(f, cd, a, g.kinds[i].from())
		if !ok {
			continue
		}
		q, in, ok := g.anchor(f, cd, c, g.kinds[i].to())
		if !ok {
			continue
		}
		// Both anchors report the direction that leads *away* from their own
		// bar, so the arrow leaves along the first and arrives against the
		// second. That is what puts the head on the outside of the successor
		// whichever of its edges the linkage names, and it is measured rather
		// than assumed so that a reversed axis turns both of them together.
		links = append(links, link{a: p, b: q, da: out, db: -in})
		rows = append(rows, i)
	}
	sc.links, sc.rows = links, rows
	if len(links) == 0 {
		return nil
	}

	stroke := g.cfg.dependStroke(f)
	size := pick(g.cfg.size, DefaultArrowSize)
	cols := sc.colorsFor(g.cfg, g.ls, rows)
	if cols == nil {
		g.paint(b, sc, links, stroke, stroke.Color, size)
		return nil
	}
	// One path per distinct colour, and one subpath per link inside it, so
	// that a pointer lands on the arrow rather than on the sheet of them —
	// see docs/adr/0015-hit-testing.md.
	for _, run := range sc.groupByColorAt(cols) {
		if run.color.A == 0 {
			continue
		}
		sc.lrun = sc.lrun[:0]
		for _, i := range run.idx {
			sc.lrun = append(sc.lrun, links[i])
		}
		st := stroke
		st.Color = run.color
		g.paint(b, sc, sc.lrun, st, run.color, size)
	}
	return nil
}

// paint strokes a set of links and fills their heads, in two calls.
func (g *dependsGeom) paint(b ir.Backend, sc *scratch, links []link, st ir.Stroke, head ir.Color, size float32) {
	if st.Visible() {
		sc.line.Reset()
		for _, l := range links {
			sc.pts = route(sc.pts[:0], l, g.horizontal, dependStub)
			sc.line.Polyline(sc.pts)
		}
		b.StrokePath(&sc.line, st)
	}
	if head.A == 0 || size <= 0 {
		return
	}
	sc.fill.Reset()
	for _, l := range links {
		arrowhead(&sc.fill, l.b, l.db, g.horizontal, size)
	}
	b.FillPath(&sc.fill, ir.Solid(head), ir.NonZero)
}

// anchor is where one end of a link meets a task: the device point on the
// span's near or far edge, halfway across its lane, and which way *out* of the
// bar that edge faces.
//
// The direction is measured rather than assumed, because a reversed axis puts
// a task's finish on the left of its start and an arrow that left rightwards
// anyway would cross its own bar.
func (g *dependsGeom) anchor(f Frame, cd coord.Coord, i int, far bool) (ir.Point, float32, bool) {
	if !defined(f.X, g.s.x[i]) || !defined(f.Y, g.s.y[i]) {
		return ir.Point{}, 0, false
	}
	if g.x2 != nil && !defined(f.X, g.x2[i]) {
		return ir.Point{}, 0, false
	}
	if g.s.y2 != nil && !defined(f.Y, g.s.y2[i]) {
		return ir.Point{}, 0, false
	}
	var near, end, lo, hi float32
	if g.horizontal {
		near, end = edgesOn(f.X, g.s.x, g.x2, i, g.halfX)
		lo, hi = spanOn(f.Y, g.s.y, g.s.y2, i, g.halfY, false)
	} else {
		near, end = edgesOn(f.Y, g.s.y, g.s.y2, i, g.halfY)
		lo, hi = spanOn(f.X, g.s.x, g.x2, i, g.halfX, true)
	}
	at, across := near, (lo+hi)/2
	dir := float32(-1)
	if far {
		at = end
		dir = 1
	}
	if end < near {
		dir = -dir
	}
	if g.horizontal {
		return cd.Point(at, across), dir, true
	}
	return cd.Point(across, at), dir, true
}

// link is one resolved dependency: the two device points it joins, the way the
// predecessor's span runs at the first, and the way the arrow travels into the
// second.
type link struct {
	a, b   ir.Point
	da, db float32
}

// route builds the orthogonal elbow between a link's two ends.
//
// The arrow leaves a along its own span and arrives at b along b's, so the
// corner count depends only on whether there is room between the two stubs:
// one turn where the successor begins after the predecessor ends, and the
// three-turn detour round the back where it does not — which is the shape a
// reader already knows from every schedule that has ever drawn a link
// backwards.
func route(dst []ir.Point, l link, horizontal bool, stub float32) []ir.Point {
	a0, a1 := axisPair(l.a, horizontal)
	b0, b1 := axisPair(l.b, horizontal)
	out := a0 + l.da*stub
	in := b0 - l.db*stub

	dst = append(dst, l.a, axisPoint(out, a1, horizontal))
	if (b0-out)*l.db >= 0 {
		dst = append(dst, axisPoint(out, b1, horizontal))
	} else {
		mid := (a1 + b1) / 2
		dst = append(dst,
			axisPoint(out, mid, horizontal),
			axisPoint(in, mid, horizontal),
			axisPoint(in, b1, horizontal))
	}
	return append(dst, l.b)
}

// axisPair and axisPoint are the same point read and written in the frame the span runs
// in: the coordinate along the time axis first, the one across the lanes
// second. They are what keeps the routing one function rather than two that
// have to be kept in step — a gantt drawn down the page is the same chart
// turned a quarter turn, exactly as a left track is a bottom one turned.
func axisPair(p ir.Point, horizontal bool) (float32, float32) {
	if horizontal {
		return p.X, p.Y
	}
	return p.Y, p.X
}

func axisPoint(a, across float32, horizontal bool) ir.Point {
	if horizontal {
		return ir.Point{X: a, Y: across}
	}
	return ir.Point{X: across, Y: a}
}

// arrowhead appends the triangle that marks where a link arrives.
func arrowhead(p *ir.Path, tip ir.Point, dir float32, horizontal bool, size float32) {
	a, across := axisPair(tip, horizontal)
	back := a - dir*size
	half := size / 2
	p.MoveTo(tip.X, tip.Y)
	q := axisPoint(back, across-half, horizontal)
	p.LineTo(q.X, q.Y)
	q = axisPoint(back, across+half, horizontal)
	p.LineTo(q.X, q.Y)
	p.Close()
}

// dependStroke is the ink of a dependency arrow.
//
// It is the annotation colour and width rather than the next palette entry,
// for the reason [config.annotationColor] gives: a constraint is not a series,
// and an arrow in the colour of a measured thing reads as one more thing that
// was measured. The dash is the exception — an annotation is dashed by default
// and a dependency is not, because a dashed arrow already means something else
// in a schedule.
func (c config) dependStroke(f Frame) ir.Stroke {
	var dash []float32
	if c.dashSet {
		dash = c.dash
	}
	width := f.Theme.AnnotationWidth
	if width <= 0 {
		width = 1
	}
	return ir.Stroke{
		Color: c.annotationColor(f),
		Width: pick(c.width, width),
		Dash:  dash,
		Cap:   ir.CapButt,
		Join:  ir.JoinMiter,
	}
}

func (g *dependsGeom) Legends(f Frame) []LegendEntry {
	if g.err != nil {
		return nil
	}
	return LegendsOr(g, f, g.cfg.legends(f, nil, g.ls, SwatchLine))
}

func (g *dependsGeom) Legend(f Frame) (LegendEntry, bool) {
	if g.err != nil || g.cfg.varying(g.ls) {
		return LegendEntry{}, false
	}
	return g.cfg.annotationLegend(f, SwatchLine)
}

func (g *dependsGeom) ColorGuide() (ColorGuide, bool) { return g.cfg.colorGuide(g.ls, g.err) }

func (g *dependsGeom) Source() data.Source { return g.src }

// Subset cuts the *task* table and leaves the links alone. A constraint whose
// other end is in another panel then resolves to -1 and is dropped, which is
// the only reading a facet can give it: an arrow to a bar that is not there
// points at nothing.
func (g *dependsGeom) Subset(rows []int) Geom {
	return &dependsGeom{src: data.Rows(g.src, rows), links: g.links, cfg: g.cfg}
}

func (g *dependsGeom) Describe() Desc {
	d := g.cfg.describe(MarkDepends)
	d.Source = g.src
	d.Links = g.links
	d.Linkage = g.cfg.linkage
	d.LinkCol = g.cfg.linkCol
	return d
}

var (
	_ Describer = (*dependsGeom)(nil)
	_ Faceter   = (*dependsGeom)(nil)
	_ Guided    = (*dependsGeom)(nil)
	_ Legender  = (*dependsGeom)(nil)
)

// progress is a layer's per-row completion, resolved once in Train.
//
// It is a small type rather than a pair of fields because two marks read it —
// a rect today, and anything else that draws a span tomorrow — and because the
// axis it runs along is a decision that has to be made once and remembered:
// deriving it again in Build would be a second place for the rule to live.
type progress struct {
	vs         []float64
	horizontal bool
	on         bool
}

// trainProgress resolves [ProgressBy] against a layer's own edges.
func (c config) trainProgress(src data.Source, n int) (progress, error) {
	if c.progCol == "" {
		return progress{}, nil
	}
	if c.x2col != "" && c.y2col != "" {
		return progress{}, ErrProgressAxis
	}
	vs, err := column(src, c.progCol, nil)
	if err != nil {
		return progress{}, err
	}
	if len(vs) != n {
		return progress{}, errLength(c.xcol, c.progCol, n, len(vs))
	}
	return progress{vs: vs, horizontal: c.y2col == "", on: true}, nil
}

// at is how much of row i is finished, clamped to [0, 1]. A row with no
// answer is a cell whose progress is unknown, and an unknown fraction is a
// whole cell: a bar drawn empty would say the task has not started, which is a
// reading of the data rather than of its absence.
func (p progress) at(i int) float32 {
	if !p.on || i >= len(p.vs) {
		return 1
	}
	v := p.vs[i]
	if math.IsNaN(v) {
		return 1
	}
	return float32(clamp01(v))
}

// done is the part of a cell that is finished: the cell, with its far edge
// moved back along the axis the row named to wherever the fraction reaches.
//
// The fraction grows from the edge the row named first rather than from the
// left of the screen, which is what makes a reversed axis fill from the same
// end of the task. near and far are that pair in mapped space.
func (p progress) done(cell ir.Rect, near, far float32, f float32) ir.Rect {
	edge := near + f*(far-near)
	lo, hi := near, edge
	if hi < lo {
		lo, hi = hi, lo
	}
	if p.horizontal {
		cell.Min.X, cell.Max.X = lo, hi
		return cell
	}
	cell.Min.Y, cell.Max.Y = lo, hi
	return cell
}

// edgesOn returns a cell's two edges on one axis in the order the row named
// them — the near edge first — where [spanOn] returns them in the order the
// screen has them. The two are the same pair, and which one a mark wants is
// exactly whether it is asking about the data or about the ink.
func edgesOn(s scale.Scale, v, v2 []float64, i int, half float64) (near, far float32) {
	if v2 != nil {
		return s.Map(v[i]), s.Map(v2[i])
	}
	if band, ok := s.(scale.Band); ok {
		c, w := s.Map(v[i]), band.Bandwidth()
		return c - w/2, c + w/2
	}
	return s.Map(v[i] - half), s.Map(v[i] + half)
}

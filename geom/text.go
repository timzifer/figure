package geom

import (
	"fmt"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/theme"
)

// Text draws one label per row, read from a column.
//
// It is what [Note] is not: a note places one literal string at one literal
// position, so labelling rows with it costs a layer per row — and since a plot
// only ever gains layers, a chart whose rows change has to be rebuilt from
// scratch, which takes the reader's zoom with it. A text layer reads the same
// [data.Source] every other mark reads and needs no rebuild when the rows
// change.
//
// The label column is [TextBy], and any column will do: a text column is used
// as it is, a numeric or temporal one is formatted the way a category name is.
//
// Where the label goes follows from which channels the layer names.
//
// Naming neither [X2] nor [Y2] puts the label at the row's point, laid out by
// [Align] exactly as a note is — the name beside a scatter point, the value
// above a bar:
//
//	geom.Text(src, geom.X("t"), geom.Y("v"), geom.TextBy("name"),
//	    geom.Align(ir.AlignCenter, ir.AlignBottom))
//
// Naming either of them puts the label in the middle of the box the row spans,
// and that box is exactly the one [Rect] would draw for the same options — an
// edge the row does not name is the slot the axis implies, as it is there. So
// one set of options describes the rectangles and labels them:
//
//	opts := []geom.Option{geom.X("start"), geom.X2("end"), geom.Y("lo"), geom.Y2("hi"),
//	    geom.ColorBy("state", pal)}
//	p.Add(geom.Rect(src, opts...))
//	p.Add(geom.Text(src, append(opts, geom.TextBy("label"))...))
//
// Two things follow from the layer knowing the box, and both are why this is a
// mark rather than a recipe over [Note].
//
// A label is measured with the font it will be drawn in, and has to fit the
// shape its box has on screen — a wedge under a polar coord, not only the chord
// across its middle. One that does not is drawn smaller, down to
// [MinFontSize], and below that it is dropped — an overrunning label reads as
// belonging to the neighbour. [Callout] writes it outside the box with a
// leader instead, where the coord has an outside, and [Elide] truncates it.
// And the middle of the box is the
// middle of its *visible* part: a bar half scrolled off the edge carries its
// label in the middle of what is left rather than off-screen with the box's
// true centre.
//
// A layer given [ColorBy] takes each label's ink from the fill that scale
// gives the row, dark on light and light on dark, so that a qualitative
// palette does not leave half its categories unreadable. [Color] overrides it.
//
// AvoidOverlap opts into the renderer's panel-local label layout. Point labels
// may move; box labels remain anchored to their own box and are dropped if they
// collide. Without that option, neighbouring labels are not moved apart.
func Text(src data.Source, opts ...Option) Geom {
	return &textGeom{src: src, cfg: newConfig(opts)}
}

type textGeom struct {
	src    data.Source
	cfg    config
	s      series
	x2     []float64
	labels []string
	pull   []float64 // the break-out column, from ExplodeBy
	gaps   []float64 // the buffer a slot-sized edge is measured out of

	// callouts is the Build's own buffer of labels written outside their
	// boxes, kept for the next one as gaps is.
	callouts []callout

	// cut and elided cache the truncation: a chart redrawn every frame must
	// not build a string per row, so a row whose label is cut where it was cut
	// last frame reuses the string. They live on the layer rather than in the
	// frame's pool because they have to survive a Build — the same reason
	// barGeom.gaps does.
	cut    []int
	elided []string

	err error
}

// errNoText reports a text column the source does not have. A layer that says
// nothing is a layer with nothing to draw, so an absent column is an error
// rather than a silent no-op.
func errNoText(col string) error {
	if col == "" {
		return fmt.Errorf("%w: no label column selected (use geom.TextBy)", ErrNoColumn)
	}
	return fmt.Errorf("%w: %q", ErrNoColumn, col)
}

// ellipsis ends a truncated label. One character rather than three dots: it is
// what a font has a glyph for, and it costs a third of the width the box did
// not have in the first place.
const ellipsis = "…"

func (g *textGeom) Train(t Training) error {
	x, y := t.X, t.Y
	g.s, g.err = resolve(g.src, g.cfg, x, y)
	if g.err != nil {
		return g.err
	}
	if g.cfg.x2col != "" {
		g.x2, g.err = column(g.src, g.cfg.x2col, x)
		if g.err != nil {
			return g.err
		}
		if len(g.x2) != len(g.s.x) {
			g.err = errLength(g.cfg.xcol, g.cfg.x2col, len(g.s.x), len(g.x2))
			return g.err
		}
	}
	var ok bool
	if g.labels, ok = data.Labels(g.src, g.cfg.textCol); !ok {
		g.err = errNoText(g.cfg.textCol)
		return g.err
	}
	if len(g.labels) != len(g.s.x) {
		g.err = errLength(g.cfg.xcol, g.cfg.textCol, len(g.s.x), len(g.labels))
		return g.err
	}
	if err := g.s.checkMissing(g.cfg, x, y); err != nil {
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
	if g.pull, g.err = g.cfg.trainBreakOut(g.src, len(g.s.x)); g.err != nil {
		return g.err
	}
	g.cfg.trainColors(g.s)

	// No widen. A mark with width pads its axis so the outermost one is not
	// clipped in half; a label's ink is not a width in data space, and the
	// layer whose boxes these are has already padded the axis for both.
	return nil
}

// boxed reports whether this layer labels a box rather than a point. It is the
// encoding that decides: a row that names a far edge on either axis spans
// something, and a row that names neither is at a place.
func (g *textGeom) boxed() bool { return g.x2 != nil || g.s.y2 != nil }

func (g *textGeom) halfWidth(vs []float64) float64 {
	gap, buf := smallestGap(g.gaps, vs)
	g.gaps = buf
	frac := g.cfg.barWidth
	if frac <= 0 || frac > 1 {
		frac = 1
	}
	return gap * frac / 2
}

func (g *textGeom) Build(b ir.Backend, f Frame) error {
	if g.err != nil {
		return g.err
	}
	size := g.cfg.fontSize
	if size <= 0 {
		size = f.Theme.LabelSize
	}
	run := ir.TextRun{Font: f.Theme.Font(size), Rotation: g.cfg.rotation}

	sc := acquire(f)
	defer sc.release()

	cd := f.Coords()
	ok := sc.plottable(g.s, f.X, f.Y)
	boxed := g.boxed()
	run.H, run.V = g.align(boxed)

	var halfX, halfY float64
	if boxed {
		halfX, halfY = g.halfWidth(g.s.x), g.halfWidth(g.s.y)
	}
	x0e, x1e, y0e, y1e := cd.Extent()

	// The rows worth drawing, and where each one's label goes. A text layer
	// aggregates nothing and decimates nothing: a label dropped to save a
	// pixel is a row nobody can read, which is the opposite of the point.
	rows := sc.rows[:0]
	pts := sc.pts[:0]
	boxes := sc.lbox[:0]

	// A label on a slice broken out of the ring goes with its slice, by the
	// displacement the slice was given. It is fitted where the slice was,
	// because the coord answers for the unmoved box, and moved afterwards —
	// which is exactly what the slice's own path does.
	var brk breakOut
	if boxed {
		brk = g.cfg.breaking(cd, g.pull)
	}
	offs := sc.offs[:0]
	for i := range g.s.x {
		if !ok[i] || g.labels[i] == "" {
			continue
		}
		if g.x2 != nil && !defined(f.X, g.x2[i]) {
			continue
		}
		var at ir.Point
		var bx labelBox
		if boxed {
			x0, x1 := spanOn(f.X, g.s.x, g.x2, i, halfX, true)
			y0, y1 := spanOn(f.Y, g.s.y, g.s.y2, i, halfY, false)

			// The middle of the visible part of the box rather than the middle
			// of the box: a bar half scrolled off the edge carries its label in
			// what is left of it. The clamp is against the interval each scale
			// maps into rather than against the plot rectangle, because that is
			// what the coord answers for — under a polar coord it is an angle
			// and a radius rather than two edges.
			x0, x1 = clamp(x0, x0e, x1e), clamp(x1, x0e, x1e)
			y0, y1 = clamp(y0, y0e, y1e), clamp(y1, y0e, y1e)
			at = cd.Point((x0+x1)/2, (y0+y1)/2)
			bx = labelBox{x0, x1, y0, y1}
		} else {
			at = cd.Point(f.X.Map(g.s.x[i]), f.Y.Map(g.s.y[i]))
		}
		rows = append(rows, i)
		pts = append(pts, at)
		boxes = append(boxes, bx)
		if brk.on() {
			offs = append(offs, brk.at(ir.Rect{Min: ir.Point{X: bx.x0, Y: bx.y0}, Max: ir.Point{X: bx.x1, Y: bx.y1}}, i))
		}
	}
	sc.rows, sc.pts, sc.lbox, sc.offs = rows, pts, boxes, offs
	if !brk.on() {
		offs = nil
	}
	if len(rows) == 0 {
		return nil
	}

	cols := sc.colorsFor(g.cfg, g.s, rows)
	ink := g.ink(f)

	// A label that does not fit its box may be written outside it, and that
	// needs a middle to go out of. It is resolved once per Build, as a band
	// scale is.
	var out coord.Exploder
	slide := false
	if boxed && !g.cfg.pinned {
		_, slide = cd.(coord.Exploder)
	}
	if boxed && g.cfg.callout {
		out, _ = cd.(coord.Exploder)
	}
	callouts := g.callouts[:0]

	// One Text call per label. Colour batching does not apply here: the IR
	// carries one style per drawing call and a run is not a path, so a layer
	// of N labels is N calls whatever their colours are.
	drawn := sc.keep[:0]
	font := run.Font
	for i, row := range rows {
		text := g.labels[row]
		run.Font = font
		shape := wrapping{lines: 1}
		if boxed {
			var fits bool
			text, run.Font, pts[i], shape, fits = g.fitBox(b, cd, run, text, boxes[i], pts[i], row, out != nil, slide)
			if !fits {
				if out != nil {
					callouts = append(callouts, callout{at: i})
				}
				continue
			}
		}
		run.Color = ink
		if cols != nil {
			run.Color = contrast(f.Theme, cols[i])
		}
		if g.cfg.color != nil {
			run.Color = *g.cfg.color
		}
		if run.Color.A == 0 {
			continue
		}
		if d := offsetAt(offs, i); d != (ir.Point{}) {
			pts[i] = ir.Point{X: pts[i].X + d.X, Y: pts[i].Y + d.Y}
		}
		run.Text, run.At = text, pts[i]
		if shape.lines > 1 {
			// The placer answers for one run, and a box label is never moved,
			// so it is asked about the whole label on one line, which is
			// wider than the block and no taller than the placer measures;
			// a block it refuses is refused whole.
			if g.cfg.avoidLabels && f.Labels != nil {
				if _, keep := f.Labels.PlaceLabel(run, false); !keep {
					continue
				}
			}
			g.drawLines(b, run, text, shape)
			drawn = append(drawn, i)
			continue
		}
		if g.cfg.avoidLabels && f.Labels != nil {
			at, keep := f.Labels.PlaceLabel(run, !boxed)
			if !keep {
				continue
			}
			run.At, pts[i] = at, at
		}
		b.Text(run)
		drawn = append(drawn, i)
	}
	g.callouts = callouts
	if len(callouts) > 0 {
		run.Font = font
		drawn = g.callOut(b, f, cd, out, run, ink, rows, pts, boxes, offs, drawn)
		// The compaction below walks the drawn rows upwards, and the ones
		// written outside their boxes were appended after the rest.
		slices.Sort(drawn)
	}
	sc.keep = drawn

	// The rows behind the labels that were drawn, and only those: a label the
	// box had no room for is not a mark a pointer can land on.
	if f.tracking() {
		for i, e := range drawn {
			pts[i], rows[i] = pts[e], rows[e]
		}
		f.Marks(MarkRows{At: pts[:len(drawn)], Rows: sc.sourceRows(g.s, rows[:len(drawn)])})
	}
	return nil
}

// labelBox is the part of a row's box that is on the panel, in the space the
// scales map into.
type labelBox struct{ x0, x1, y0, y1 float32 }

// labelPad is how far a label's font box stays inside the edge of its box, in
// device units. Ink touching the edge of a slice reads as crossing it.
const labelPad = 1.5

// fitBox decides how a label is drawn in its box: at the layer's size on one
// line if it fits, broken over lines if [Wrap] was asked for and that fits,
// smaller if a size down to [MinFontSize] fits either way, and cut if [Elide]
// was asked for and the label is not to be written outside instead. It
// reports false for a label that has no place in its box at all.
//
// Fitting is decided on screen and by the coord, which is the point of it: the
// label's font box, laid out about its anchor as it will be drawn, has to lie
// inside the box, and whether a point on screen lies inside a box in the space
// the scales map into is a question [coord.Coord.Invert] answers for every
// coord. Under Cartesian that is a rectangle in a rectangle. Under a polar
// coord it is a level rectangle in a wedge of an annulus, where the chord
// through the middle of the slice — which is what used to be measured — claims
// the room at the middle for the whole height of the text, and the corners of
// a label in a thin slice cross into its neighbours.
func (g *textGeom) fitBox(m ir.Measurer, cd coord.Coord, run ir.TextRun, text string, bx labelBox, at ir.Point, row int, outside, slide bool) (string, ir.FontRef, ir.Point, wrapping, bool) {
	size := run.Font.Size
	tm := m.Measure(ir.TextRun{Text: text, Font: run.Font})
	w, h, asc := tm.Advance, tm.Height(), tm.Ascent
	if h <= 0 {
		h, asc = float32(size)*1.2, float32(size)*0.95
	}

	// Where in the box the label may go. The middle first, and under a coord
	// with a middle of its own, further along each of the box's two spans as
	// well: a long slice of a sunburst turns as it goes round, and a level
	// label that cannot sit across it where it runs diagonally sits in it
	// where it runs level. Under Cartesian a box is the same shape all the
	// way along, so there is nowhere better than its middle to look.
	var spots [9]ir.Point
	spots[0] = at
	n := 1
	if slide {
		xm, ym := (bx.x0+bx.x1)/2, (bx.y0+bx.y1)/2
		for _, t := range [4]float32{0.35, 0.65, 0.2, 0.8} {
			spots[n] = cd.Point(bx.x0+t*(bx.x1-bx.x0), ym)
			spots[n+1] = cd.Point(xm, bx.y0+t*(bx.y1-bx.y0))
			n += 2
		}
	}

	// The shapes the label may take: one line, and with Wrap the best break
	// into two lines and into three. Each is a width and a number of lines of
	// the font's height.
	shapes := [3]wrapping{{lines: 1, width: w}}
	k := 1
	if g.cfg.wrap {
		for lines := 2; lines <= maxLines; lines++ {
			if s, ok := balance(m, run.Font, text, lines); ok {
				shapes[k] = s
				k++
			}
		}
	}

	for _, s := range shapes[:k] {
		for _, p := range spots[:n] {
			if holds(cd, bx, p, run, s.width, h*float32(s.lines), asc) {
				return text, run.Font, p, s, true
			}
		}
	}

	// A run's metrics scale with its size, so a smaller size is tried by
	// scaling what was measured rather than by measuring again: the search is
	// the coord's arithmetic and not the shaper's. The shape and the spot that
	// hold the largest label win, and fewer lines win a tie.
	floor := g.minFontSize(size)
	best, where, shape := 0.0, at, shapes[0]
	for _, s := range shapes[:k] {
		for _, p := range spots[:n] {
			scaled := func(z float64) bool {
				f := float32(z / size)
				return holds(cd, bx, p, run, s.width*f, h*float32(s.lines)*f, asc*f)
			}
			lo := max(floor, best)
			if lo >= size || !scaled(lo) {
				continue
			}
			hi := size
			for range 10 {
				if mid := (lo + hi) / 2; scaled(mid) {
					lo = mid
				} else {
					hi = mid
				}
			}
			if lo > best {
				best, where, shape = lo, p, s
			}
		}
	}
	if best > 0 {
		// Rounded down to a quarter of a unit, so that a chart redrawn at the
		// same size asks for the same few fonts rather than one per label.
		font := run.Font
		font.Size = max(math.Floor(best*4)/4, floor)
		return text, font, where, shape, true
	}
	if !g.cfg.elide || outside {
		return "", run.Font, at, wrapping{}, false
	}

	// What is left is a cut, and the room it is cut to is the widest run at
	// the layer's size that the box holds — found the way a size is.
	if !holds(cd, bx, at, run, 0, h, asc) {
		return "", run.Font, at, wrapping{}, false
	}
	var fit, over float32 = 0, w
	for range 12 {
		if mid := (fit + over) / 2; holds(cd, bx, at, run, mid, h, asc) {
			fit = mid
		} else {
			over = mid
		}
	}
	cut, ok := g.fit(m, run.Font, text, fit, row)
	return cut, run.Font, at, wrapping{lines: 1}, ok
}

// maxLines is the most lines [Wrap] breaks a label into. A box label of four
// lines is a paragraph, which is not what a label is for.
const maxLines = 3

// wrapping is how a label is broken: into lines, at the byte offsets in cut —
// each a space, which the break swallows — and how wide the widest line is.
type wrapping struct {
	lines int
	cut   [maxLines - 1]int
	width float32
}

// line is the i'th line of s broken as w says.
func (w wrapping) line(s string, i int) string {
	lo, hi := 0, len(s)
	if i > 0 {
		lo = w.cut[i-1] + 1
	}
	if i < w.lines-1 {
		hi = w.cut[i]
	}
	return s[lo:hi]
}

// balance finds the break of s into the given number of lines, at spaces,
// whose widest line is narrowest — the break that makes the most compact
// block, which is what has the best chance in a box. It reports false for a
// label with too few spaces to break that often.
//
// The lines are cut out of the label rather than built, so a break costs
// measurements and no strings. Two lines are one pass over the spaces and
// three are a pass over the pairs of them; a label is short.
func balance(m ir.Measurer, font ir.FontRef, s string, lines int) (wrapping, bool) {
	width := func(t string) float32 { return m.Measure(ir.TextRun{Text: t, Font: font}).Advance }
	best := wrapping{lines: lines, width: float32(math.Inf(1))}
	found := false
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			continue
		}
		if lines == 2 {
			if w := max(width(s[:i]), width(s[i+1:])); w < best.width {
				best.width, best.cut[0], found = w, i, true
			}
			continue
		}
		head := width(s[:i])
		if head >= best.width {
			break
		}
		for j := i + 1; j < len(s); j++ {
			if s[j] != ' ' {
				continue
			}
			if w := max(head, width(s[i+1:j]), width(s[j+1:])); w < best.width {
				best.width, best.cut[0], best.cut[1], found = w, i, j, true
			}
		}
	}
	return best, found
}

// drawLines writes a label broken over lines as a block laid out about run's
// anchor the way one run would be: aligned by run.H along its own edge and by
// run.V as a whole, and turned with it. Each line is its own run, because a
// run is one line; the block is what was fitted.
func (g *textGeom) drawLines(b ir.Backend, run ir.TextRun, text string, shape wrapping) {
	m := b.Measure(ir.TextRun{Text: text, Font: run.Font})
	h := m.Height()
	if h <= 0 {
		h = float32(run.Font.Size) * 1.2
	}
	total := h * float32(shape.lines)
	var top float32
	switch run.V {
	case ir.AlignTop:
	case ir.AlignMiddle:
		top = -total / 2
	case ir.AlignBottom:
		top = -total
	default:
		top = -m.Ascent
	}
	sin, cos := float32(0), float32(1)
	if run.Rotation != 0 {
		s, c := math.Sincos(run.Rotation)
		sin, cos = float32(s), float32(c)
	}
	at := run.At
	run.V = ir.AlignMiddle
	for i := range shape.lines {
		v := top + (float32(i)+0.5)*h
		run.Text = shape.line(text, i)
		run.At = ir.Point{X: at.X - v*sin, Y: at.Y + v*cos}
		b.Text(run)
	}
}

// minFontSize is the smallest size a box label is shrunk to. See [MinFontSize].
func (g *textGeom) minFontSize(size float64) float64 {
	lo := g.cfg.minFont
	if lo <= 0 {
		lo = size * 0.75
	}
	return min(lo, size)
}

// holds reports whether a run whose font box is w by h, with its baseline asc
// below the top, laid out about at the way run is aligned and turned, lies
// inside the box bx.
//
// The font box is tested at points along each of its edges and not only at
// its corners, because the edge of a slice is not straight: the arc of a
// donut's hole bows out into a label whose corners are clear of it.
func holds(cd coord.Coord, bx labelBox, at ir.Point, run ir.TextRun, w, h, asc float32) bool {
	var x0, y0 float32
	switch run.H {
	case ir.AlignCenter:
		x0 = -w / 2
	case ir.AlignEnd:
		x0 = -w
	}
	switch run.V {
	case ir.AlignTop:
	case ir.AlignMiddle:
		y0 = -h / 2
	case ir.AlignBottom:
		y0 = -h
	default:
		y0 = -asc
	}
	x0, y0 = x0-labelPad, y0-labelPad
	x1, y1 := x0+w+2*labelPad, y0+h+2*labelPad

	sin, cos := float32(0), float32(1)
	if run.Rotation != 0 {
		s, c := math.Sincos(run.Rotation)
		sin, cos = float32(s), float32(c)
	}
	lx0, lx1 := min(bx.x0, bx.x1), max(bx.x0, bx.x1)
	ly0, ly1 := min(bx.y0, bx.y1), max(bx.y0, bx.y1)
	const eps = 1e-3
	inside := func(u, v float32) bool {
		mx, my := cd.Invert(ir.Point{X: at.X + u*cos - v*sin, Y: at.Y + u*sin + v*cos})
		return mx >= lx0-eps && mx <= lx1+eps && my >= ly0-eps && my <= ly1+eps
	}
	const n = 4
	for k := range n {
		t := float32(k) / n
		u, v := x0+t*(x1-x0), y0+t*(y1-y0)
		// Round the font box once, a quarter of an edge at a time.
		if !inside(u, y0) || !inside(x1+x0-u, y1) || !inside(x0, y1+y0-v) || !inside(x1, v) {
			return false
		}
	}
	return true
}

// callout is a label that did not fit its box and is written outside it.
type callout struct {
	at         int      // index into the Build's rows
	from, bend ir.Point // the leader's foot on the box, and where it turns level
	y          float32  // the height the label is written at, once stacked apart
	side       float32  // +1 written to the right, -1 to the left

	// How the label is written beside the chart: in which font, broken how,
	// and the block that makes — w wide and h high.
	font  ir.FontRef
	shape wrapping
	w, h  float32
}

// callOut writes the labels that did not fit their boxes outside them, each
// joined back to its box by a leader, and returns drawn with their rows
// appended.
//
// A label leaves its box through the middle of whichever edge is furthest out
// along the box's bisector, runs out along that bisector until it is past
// everything the coord draws — a label from the inner ring of a sunburst
// crosses the outer rings rather than stopping on top of them — and turns
// level. Labels on one side are then stacked apart in the order they left, so
// that a run of thin slices writes a column of labels rather than a pile.
//
// A label too wide for the room beside the chart on its side is broken over
// lines where [Wrap] allows it, then shrunk down to [MinFontSize], and only
// then dropped (see [textGeom.calloutShape]).
func (g *textGeom) callOut(b ir.Backend, f Frame, cd coord.Coord, out coord.Exploder, run ir.TextRun, ink ir.Color, rows []int, pts []ir.Point, boxes []labelBox, offs []ir.Point, drawn []int) []int {
	size := float32(run.Font.Size)
	run.Rotation = 0
	gap, arm := size, size
	x0e, x1e, y0e, y1e := cd.Extent()
	lx0, lx1 := min(x0e, x1e), max(x0e, x1e)
	ly0, ly1 := min(y0e, y1e), max(y0e, y1e)
	past := func(p ir.Point) bool {
		mx, my := cd.Invert(p)
		return mx < lx0 || mx > lx1 || my < ly0 || my > ly1
	}

	cs := g.callouts
	for k := range cs {
		c := &cs[k]
		bx := boxes[c.at]
		dx, dy := out.Explode(bx.x0, bx.y0, bx.x1, bx.y1, 1)
		if l := float32(math.Hypot(float64(dx), float64(dy))); l > 0 {
			dx, dy = dx/l, dy/l
		} else {
			dx, dy = 0, -1
		}

		// The foot is the middle of whichever edge of the box is furthest out.
		mid := pts[c.at]
		xm, ym := (bx.x0+bx.x1)/2, (bx.y0+bx.y1)/2
		c.from = mid
		best := float32(math.Inf(-1))
		for _, e := range [4]ir.Point{cd.Point(xm, bx.y0), cd.Point(xm, bx.y1), cd.Point(bx.x0, ym), cd.Point(bx.x1, ym)} {
			if d := (e.X-mid.X)*dx + (e.Y-mid.Y)*dy; d > best {
				best, c.from = d, e
			}
		}

		// Out along the bisector until the coord has nothing more to draw:
		// doubled until past its edge, then halved back onto it.
		along := func(t float32) ir.Point { return ir.Point{X: c.from.X + dx*t, Y: c.from.Y + dy*t} }
		var in, beyond float32 = 0, 1
		for beyond < 1<<14 && !past(along(beyond)) {
			in, beyond = beyond, beyond*2
		}
		if beyond < 1<<14 {
			for range 12 {
				if m := (in + beyond) / 2; past(along(m)) {
					beyond = m
				} else {
					in = m
				}
			}
		} else {
			beyond = 0
		}
		c.bend = along(beyond + gap)
		if d := offsetAt(offs, c.at); d != (ir.Point{}) {
			c.from = ir.Point{X: c.from.X + d.X, Y: c.from.Y + d.Y}
			c.bend = ir.Point{X: c.bend.X + d.X, Y: c.bend.Y + d.Y}
		}
		c.y = c.bend.Y
		c.side = 1
		if dx < -1e-3 {
			c.side = -1
		}
	}

	// How each label is written is settled before the labels are stacked,
	// because a label broken over two lines takes two lines of the column.
	// The room is what the drawing below allows: from where the label starts
	// at the end of a full arm to the edge of the panel, and the arm may give
	// up all but a quarter of the type to it.
	kept := cs[:0]
	for _, c := range cs {
		start := c.bend.X + c.side*(arm+gap/2)
		room := f.Area.Max.X - start
		if c.side < 0 {
			room = start - f.Area.Min.X
		}
		room += arm - size/4
		var ok bool
		c.font, c.shape, c.w, c.h, ok = g.calloutShape(b, run.Font, g.labels[rows[c.at]], room)
		if ok {
			kept = append(kept, c)
		}
	}
	cs = kept
	if len(cs) == 0 {
		return drawn
	}

	// Stacked apart a side at a time, top to bottom, inside the panel.
	slices.SortFunc(cs, func(a, b callout) int {
		if a.side != b.side {
			return cmpFloat(a.side, b.side)
		}
		return cmpFloat(a.bend.Y, b.bend.Y)
	})
	// Two neighbours stand half of each one's height apart and a pixel more,
	// which for two one-line labels is the line and a pixel it always was.
	step := func(a, b callout) float32 { return (a.h+b.h)/2 + 1 }
	for lo := 0; lo < len(cs); {
		hi := lo + 1
		for hi < len(cs) && cs[hi].side == cs[lo].side {
			hi++
		}
		side := cs[lo:hi]
		side[0].y = max(side[0].y, f.Area.Min.Y+side[0].h/2)
		for k := 1; k < len(side); k++ {
			side[k].y = max(side[k].y, side[k-1].y+step(side[k-1], side[k]))
		}
		last := &side[len(side)-1]
		if bottom := f.Area.Max.Y - last.h/2; last.y > bottom {
			last.y = bottom
			for k := len(side) - 2; k >= 0; k-- {
				side[k].y = min(side[k].y, side[k+1].y-step(side[k], side[k+1]))
			}
		}
		lo = hi
	}

	// The label stands on the background now rather than on its box, so it
	// is written in the theme's label ink: neither the contrast against a fill
	// that is no longer behind it nor a colour chosen to be read on that fill.
	// The leader is furniture of the label rather than of the data, so it is
	// drawn in the stroke an annotation is drawn in.
	ink = f.Theme.LabelColor
	if ink.A == 0 {
		ink = theme.Light.LabelColor
	}
	line := ir.Stroke{Color: f.Theme.AnnotationColor, Width: f.Theme.AnnotationWidth, Cap: ir.CapRound, Join: ir.JoinRound}
	if line.Color.A == 0 {
		line.Color = ink
	}
	if line.Width <= 0 {
		line.Width = 1
	}
	run.Color, run.V = ink, ir.AlignMiddle
	for _, c := range cs {
		end := ir.Point{X: c.bend.X + c.side*arm, Y: c.y}
		run.H = ir.AlignStart
		if c.side < 0 {
			run.H = ir.AlignEnd
		}
		run.Text = g.labels[rows[c.at]]
		run.Font = c.font
		run.At = ir.Point{X: end.X + c.side*gap/2, Y: c.y}

		// A label that runs past the edge of the panel is drawn in on a
		// shorter arm first, as far as an arm a quarter of the type long;
		// one that still has no room beside the chart is dropped rather than
		// cut by the edge, which is the rule every other label here keeps:
		// half a label names nothing.
		w := c.w
		over := max(f.Area.Min.X-(run.At.X-w), 0)
		if c.side > 0 {
			over = max(run.At.X+w-f.Area.Max.X, 0)
		}
		if over > arm-size/4 || c.y-c.h/2 < f.Area.Min.Y-0.01 || c.y+c.h/2 > f.Area.Max.Y+0.01 {
			continue
		}
		end.X -= c.side * over
		run.At.X -= c.side * over
		b.Polyline([]ir.Point{c.from, c.bend, end}, line)
		if c.shape.lines > 1 {
			g.drawLines(b, run, run.Text, c.shape)
		} else {
			b.Text(run)
		}
		pts[c.at] = run.At
		drawn = append(drawn, c.at)
	}
	return drawn
}

// calloutShape decides how a called-out label is written in the room beside
// the chart: at the layer's size on one line if it fits; broken over two or
// three lines if [Wrap] was asked for and that fits; smaller, down to
// [MinFontSize], on whichever of those shapes holds the largest type, fewer
// lines winning a tie. It reports false for a label that has no room even
// then, and returns the font, the break and the block it makes.
//
// It is the order a box label is fitted in, and for the same reason: two
// lines at the layer's size read better than one at three quarters of it. A
// label out of a slice at three o'clock has the margin beside the chart and
// nothing else, and a long name there is exactly the one that used to be
// dropped — the name a callout exists for.
//
// Only the width is searched. The column the labels are stacked in is as
// tall as the panel, and the stacking pushes a taller block along rather than
// refusing it.
func (g *textGeom) calloutShape(m ir.Measurer, font ir.FontRef, text string, room float32) (ir.FontRef, wrapping, float32, float32, bool) {
	size := font.Size
	tm := m.Measure(ir.TextRun{Text: text, Font: font})
	h := tm.Height()
	if h <= 0 {
		h = float32(size) * 1.2
	}
	if room <= 0 {
		return font, wrapping{}, 0, 0, false
	}

	shapes := [maxLines]wrapping{{lines: 1, width: tm.Advance}}
	k := 1
	if g.cfg.wrap {
		for lines := 2; lines <= maxLines; lines++ {
			if s, ok := balance(m, font, text, lines); ok {
				shapes[k] = s
				k++
			}
		}
	}
	for _, s := range shapes[:k] {
		if s.width <= room {
			return font, s, s.width, h * float32(s.lines), true
		}
	}

	// A run's advance scales with its size, so the largest size a shape
	// fits at is read off rather than searched for, and rounded down to a
	// quarter of a unit as a box label's is.
	floor := g.minFontSize(size)
	best, shape := 0.0, shapes[0]
	for _, s := range shapes[:k] {
		if s.width <= 0 {
			continue
		}
		z := math.Floor(size*float64(room/s.width)*4) / 4
		if z >= floor && z > best {
			best, shape = z, s
		}
	}
	if best <= 0 {
		return font, wrapping{}, 0, 0, false
	}
	f := float32(best / size)
	font.Size = best
	shape.width *= f
	return font, shape, shape.width, h * f * float32(shape.lines), true
}

func cmpFloat(a, b float32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// align is how a label sits about its anchor.
//
// A layer that was told nothing centres a label in its box and hangs it off
// its point, which is why the config records having been told: the start of a
// run on the baseline is a zero value as well as an alignment.
func (g *textGeom) align(boxed bool) (ir.HAlign, ir.VAlign) {
	if g.cfg.alignSet || !boxed {
		return g.cfg.halign, g.cfg.valign
	}
	return ir.AlignCenter, ir.AlignMiddle
}

// fit measures a label against the room it has and reports what to draw.
//
// A label that does not fit is dropped rather than allowed to overrun, because
// an overrunning label reads as belonging to the neighbouring row. With
// [Elide] it is truncated instead, and where it was cut is remembered: a chart
// redrawn at the same size cuts every label where it cut it last frame, so a
// steady-state frame builds no strings at all. That is the difference between
// a live chart that allocates per row and one that does not.
func (g *textGeom) fit(m ir.Measurer, font ir.FontRef, s string, room float32, row int) (string, bool) {
	if math.IsInf(float64(room), 1) {
		return s, true
	}
	if room <= 0 {
		return "", false
	}
	if m.Measure(ir.TextRun{Text: s, Font: font}).Advance <= room {
		return s, true
	}
	if !g.cfg.elide {
		return "", false
	}
	room -= m.Measure(ir.TextRun{Text: ellipsis, Font: font}).Advance
	if room <= 0 {
		return "", false
	}

	// The longest prefix that still leaves room for the ellipsis, found by
	// halving rather than by walking: a run's advance does not shrink as the
	// run grows, so the boundary can be searched for, and a long label in a
	// narrow box is exactly where a per-character walk would be felt. The cut
	// is on a rune boundary — half a character is not a shorter label.
	lo, hi := 0, len(s)
	for lo < hi {
		mid := lo + (hi-lo+1)/2
		for mid > lo && !utf8.RuneStart(s[mid]) {
			mid--
		}
		if mid == lo {
			break
		}
		if m.Measure(ir.TextRun{Text: s[:mid], Font: font}).Advance <= room {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return "", false
	}
	return g.remember(row, lo), true
}

// remember hands back this row's label cut at n bytes, out of the layer's own
// cache when the cut has not moved since the last frame.
func (g *textGeom) remember(row, n int) string {
	if len(g.cut) != len(g.labels) {
		g.cut = grow(g.cut, len(g.labels))
		g.elided = grow(g.elided, len(g.labels))
		for i := range g.cut {
			g.cut[i], g.elided[i] = -1, ""
		}
	}
	if g.cut[row] == n {
		return g.elided[row]
	}
	out := g.labels[row][:n] + ellipsis
	g.cut[row], g.elided[row] = n, out
	return out
}

// ink is the colour of a layer whose labels are not coloured per row.
func (g *textGeom) ink(f Frame) ir.Color {
	if g.cfg.color != nil {
		return *g.cfg.color
	}
	// A uniform fill is a background to read against exactly as a scale's is,
	// so a label inside one follows it too.
	if g.cfg.fill != nil {
		return contrast(f.Theme, *g.cfg.fill)
	}
	if f.Theme.LabelColor.A != 0 {
		return f.Theme.LabelColor
	}
	return theme.Light.LabelColor
}

// contrast picks the more readable of the theme's two inks against a fill.
//
// The pair is the theme's own — the colour it labels with and the colour
// behind the chart — rather than black and white, so that a dark theme's
// labels are drawn in its own colours and a chart still looks like one thing.
// The comparison is on luminance, which is why [palette.Luminance] is where it
// is: a fill comes from a colour scale, and how light a colour is is a fact
// about the colour rather than about the mark.
func contrast(th theme.Theme, bg ir.Color) ir.Color {
	label, back := th.LabelColor, th.Background
	if label.A == 0 {
		label = theme.Light.LabelColor
	}
	if back.A == 0 {
		back = theme.Light.Background
	}
	l := palette.Luminance(bg)
	if math.Abs(palette.Luminance(back)-l) > math.Abs(palette.Luminance(label)-l) {
		return back
	}
	return label
}

// clamp confines v to an interval given in either order, because the interval
// a scale maps into runs downwards on a vertical axis.
func clamp(v, lo, hi float32) float32 {
	if lo > hi {
		lo, hi = hi, lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// A text layer contributes no legend entry, for the reason an annotation does
// not: what it draws is the reading rather than a series to be named.
func (g *textGeom) Legend(f Frame) (LegendEntry, bool) { return LegendEntry{}, false }

func (g *textGeom) Describe() Desc {
	d := g.cfg.describe(MarkText)
	d.Source = g.src
	return d
}

func (g *textGeom) Source() data.Source { return g.src }

// AvoidsLabels reports whether this layer requests panel-local label layout.
func (g *textGeom) AvoidsLabels() bool { return g.cfg.avoidLabels }

// Overhangs reports whether this layer may write labels outside their boxes,
// and so outside what the coord clips to. See [Callout].
func (g *textGeom) Overhangs() bool { return g.cfg.callout }

func (g *textGeom) Subset(rows []int) Geom {
	return &textGeom{src: data.Rows(g.src, rows), cfg: g.cfg}
}

// A text layer holds rows, so it is faceted by splitting them. It contributes
// no colour guide: the layer whose boxes it labels carries the same ColorBy
// and has already contributed one, and a second would be the same guide twice.
var (
	_ Describer = (*textGeom)(nil)
	_ Faceter   = (*textGeom)(nil)
)

package three

import (
	"github.com/timzifer/figure/internal/layout"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/render"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// draw is this package's one drawing order, and the reason it is a package.
//
// It reads like render.Draw and diverges at exactly one point: where render
// walks a panel's layers and lets each of them stream into the backend, this
// collects every layer's primitives first and paints them in one depth order.
// The steps are: train the scenes, lay the views out, say what the chart is,
// paint the background and the title, then for each view its cube and its
// data.
func (p *Plot) draw(b ir.Backend) (areas []ir.Rect, err error) {
	th := p.theme()
	views := p.viewList()
	canvas := ir.R(0, 0, float32(p.width), float32(p.height))

	// 1. Train. Each distinct scene once, however many views look at it —
	//    a domain is a fact about the data and not about where it is looked
	//    at from, and a scale accumulates, so training per view would give a
	//    layer as many times its weight as there are cameras.
	trained := map[*Scene][3]scale.Scale{}
	order := make([]*Scene, 0, 1)
	for _, v := range views {
		sc := v.sceneOr(p.scene)
		if sc == nil {
			return nil, ErrNoScene
		}
		if _, done := trained[sc]; done {
			continue
		}
		scales := sc.scales()
		if err := sc.train(scales); err != nil {
			return nil, err
		}
		trained[sc] = scales
		order = append(order, sc)
	}

	// 2. Lay out. One solver, and it is called for what it is the solver for:
	//    the outer margin, the title, the strips and a grid of equal cells.
	//    Its tick-label gutters are left empty, because a projected cube's
	//    labels hang off its own edges inside the cell rather than in a
	//    four-sided gutter, and telling the solver otherwise would be lying to
	//    it.
	cols := p.cols
	if cols > len(views) {
		cols = len(views)
	}
	rows := (len(views) + cols - 1) / cols
	cells := make([]layout.Panel, len(views))
	for i, v := range views {
		cells[i] = layout.Panel{Row: i / cols, Col: i % cols, Strip: v.Label}
	}
	lay := layout.Panels(layout.Grid{
		Canvas: canvas,
		Theme:  th,
		Title:  p.title,
		Rows:   rows,
		Cols:   cols,
		Panels: cells,
	}, b)

	// 3. Say what the chart is, before any ink. A wrapper hides the optional
	//    interfaces of what it wraps, so this asks the backend it was handed.
	if s, ok := b.(ir.Semantics); ok {
		s.Describe(p.Description())
	}

	// 4. Background and title.
	if th.Background.A != 0 {
		var bg ir.Path
		bg.Rect(canvas)
		b.FillPath(&bg, ir.Solid(th.Background), ir.NonZero)
	}
	if p.title != "" {
		b.Text(ir.TextRun{
			Text: p.title, Font: th.Font(th.TitleSize), At: lay.Title,
			H: ir.AlignCenter, Color: th.TitleColor,
		})
	}

	// 5. The room a cube's own labels need, measured once for the whole
	//    figure and given to every cell — so two views of one scene get
	//    identically sized cubes, which is what makes them comparable.
	inset := p.labelRoom(b, th, order, trained)

	sink := acquire()
	defer release(sink)
	var path ir.Path

	for i, v := range views {
		area := lay.Areas[i]
		if area.Empty() {
			continue
		}
		drawStrip(b, lay.Strips[i], th, v.Label)

		sc := v.sceneOr(p.scene)
		scales, titles := trained[sc], sc.titles()
		inner := area.Inset(inset, inset, inset, inset)
		if inner.Empty() {
			inner = area
		}
		pr := project(v.Camera, inner)

		var clip ir.Path
		clip.Rect(area)
		b.Push(&clip, ir.Identity)

		newCube(th, pr, v.Camera, scales, titles).draw(b, &path)

		if p.obs != nil {
			// X and Y are nil deliberately. A projected scene has no screen
			// axes to invert a device point through, and a hit index checks
			// before it inverts — so a pointer reports which mark and which
			// row it landed on, and the third value is read from the row
			// rather than guessed from the geometry. ADR 0056's bargain.
			p.obs.Panel(render.PanelInfo{Index: i, Area: area, Z: scales[axisZ]})
		}

		f := Frame{
			X: scales[axisX], Y: scales[axisY], Z: scales[axisZ],
			Theme: th, View: i, Forward: v.Camera.Forward(), Rows: p.rows,
		}
		sink.reset()
		for k, l := range sc.layers {
			sink.openLayer(k, f.Forward)
			g := f
			g.Index = k
			if err := l.Emit(sink, g); err != nil {
				b.Pop()
				return nil, err
			}
		}
		sink.paint(b, pr, p.obs, i, sc.layerLabels(f), p.rows)

		b.Pop()
	}

	if p.obs != nil {
		p.obs.End()
	}
	return lay.Areas, nil
}

// labelRoom is how far a cube has to sit inside its cell so that its tick
// labels and axis titles fit beside it.
//
// It is one number for the whole figure rather than one per view, so that
// every cube is drawn at the same scale. Two views of one scene that were
// sized differently would be two charts that look comparable and are not.
func (p *Plot) labelRoom(m layout.Measurer, th theme.Theme, order []*Scene, trained map[*Scene][3]scale.Scale) float32 {
	font := th.Font(th.TickSize)
	widest := float32(0)
	titled := false
	want := [3]int{th.TickCountHintX, th.TickCountHintX, th.TickCountHintY}
	for _, sc := range order {
		for a, s := range trained[sc] {
			for _, t := range s.Ticks(scale.TickRequest{Want: want[a]}) {
				if t.Label == "" {
					continue
				}
				if w := m.Measure(ir.TextRun{Text: t.Label, Font: font}).Advance; w > widest {
					widest = w
				}
			}
		}
		for _, t := range sc.titles() {
			titled = titled || t != ""
		}
	}
	room := th.TickLength + th.TickLabelPad + widest
	if titled {
		room += th.AxisTitlePad + float32(th.LabelSize)*1.4
	}
	return room
}

// drawStrip writes a view's label in the band above it. It is render's strip,
// spelled again here rather than shared, because render's is unexported and
// this package draws in its own order — the two are the same picture and not
// the same drawing.
func drawStrip(b ir.Backend, box ir.Rect, th theme.Theme, label string) {
	if label == "" || box.Empty() {
		return
	}
	if th.StripBG.A != 0 {
		var p ir.Path
		p.Rect(box)
		b.FillPath(&p, ir.Solid(th.StripBG), ir.NonZero)
	}
	if th.StripBorder.A != 0 {
		var p ir.Path
		p.Rect(box)
		b.StrokePath(&p, ir.Stroke{Color: th.StripBorder, Width: pickWidth(th.AxisWidth)})
	}
	b.Text(ir.TextRun{
		Text:  label,
		Font:  th.Font(th.StripSize),
		At:    ir.Point{X: (box.Min.X + box.Max.X) / 2, Y: (box.Min.Y + box.Max.Y) / 2},
		H:     ir.AlignCenter,
		V:     ir.AlignMiddle,
		Color: th.StripColor,
	})
}

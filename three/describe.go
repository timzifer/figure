package three

import (
	"io"

	"github.com/timzifer/figure/a11y"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
)

// Describer is implemented by a [Layer] that can say what it plots.
//
// It is optional for the reason [github.com/timzifer/figure/geom.Describer] is
// optional: a layer nobody describes does not have to know what a screen
// reader is. A layer that implements none is named by its position rather than
// skipped, because a description that quietly omits a series is worse than one
// that admits to it.
type Describer interface {
	Describe() geom.Desc
}

// Describe reads the plot's scene and says what it shows.
//
// **It does not mention the camera, and that is a requirement rather than an
// omission.** A reader using the description gets the same chart as a reader
// dragging the scene; the description of a figure with four views is the
// description of what is plotted, once. A chart that must be turned to be read
// is a chart some readers cannot read, so the words and the data table have to
// be complete without it. See docs/adr/0057-orbiting-a-chart.md and
// docs/adr/0024-accessibility.md.
func (p *Plot) Describe() a11y.Summary { return a11y.Describe(p.a11yChart()) }

// Description is what a backend that can carry a description is told, before
// anything is drawn.
func (p *Plot) Description() ir.Description {
	if p.descSet {
		return p.desc
	}
	s := p.Describe()
	return ir.Description{Title: s.Title, Detail: s.Detail}
}

// DataTable writes the scene's rows as HTML tables — the third of the three
// channels a chart says what it is in, and the one a projected scene leans on
// hardest: a reader who cannot turn the picture still gets every number in it.
func (p *Plot) DataTable(w io.Writer) error { return a11y.WriteTable(w, p.a11yChart()) }

// a11yChart reduces the plot to the model a11y describes: the scene, its three
// scales and its three axis titles, with no camera and no rectangle anywhere.
func (p *Plot) a11yChart() a11y.Chart {
	sc := p.scene
	if sc == nil {
		for _, v := range p.views {
			if v.Scene != nil {
				sc = v.Scene
				break
			}
		}
	}
	if sc == nil {
		return a11y.Chart{Title: p.title}
	}
	scales := sc.scales()
	out := a11y.Chart{
		Title:  p.title,
		XTitle: sc.xTitle, YTitle: sc.yTitle, ZTitle: sc.zTitle,
		X: scales[axisX], Y: scales[axisY], Z: scales[axisZ],
	}
	for i, l := range sc.layers {
		d, ok := l.(Describer)
		if !ok {
			out.Descs = append(out.Descs, geom.Desc{Label: unnamed(i)})
			continue
		}
		out.Descs = append(out.Descs, d.Describe())
	}
	return out
}

func unnamed(i int) string {
	return "layer " + itoa(i+1)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[n:])
}

// describe is what every layer in this package answers with: the options it
// was configured with, plus its mark and its source.
func (b base) describe() geom.Desc {
	d := b.cfg
	d.Mark = b.mark
	d.Source = b.src
	return d
}

func (g *surface) Describe() geom.Desc { return g.base.describe() }
func (g *line3) Describe() geom.Desc   { return g.base.describe() }
func (g *bar3) Describe() geom.Desc    { return g.base.describe() }

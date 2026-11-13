package stilus

import (
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/go-opentype/fonts/goregular"
	"github.com/go-opentype/opentype"
	st "github.com/timzifer/stilus"
)

// face is one parsed font and what this backend derives from it: the outline
// of every glyph drawn so far, in design units, Y up.
//
// The outlines are read through an opentype.Face at the font's own em size
// with hinting off, which is the face that hands back design units unchanged;
// the size a glyph is drawn at is the matrix it is filled with.
type face struct {
	id   uint64 // the GlyphCache's name for this font
	font *opentype.Font
	of   *opentype.Face
	upem float64

	// asc and desc are the font box in design units, both positive — see
	// fontBox for which table they come from.
	asc, desc float64

	mu       sync.Mutex
	outlines map[opentype.GlyphIndex]*st.Path
}

var faceIDs atomic.Uint64

func parseFace(ttf []byte, what string) (*face, error) {
	f, err := opentype.Parse(ttf)
	if err != nil {
		return nil, fmt.Errorf("figure/backend/stilus: parsing the %s font: %w", what, err)
	}
	upem := f.UnitsPerEm()
	if upem <= 0 {
		return nil, fmt.Errorf("figure/backend/stilus: the %s font has no units per em", what)
	}
	asc, desc := fontBox(f)
	return &face{
		id:       faceIDs.Add(1),
		font:     f,
		of:       f.NewFace(upem),
		upem:     float64(upem),
		asc:      asc,
		desc:     desc,
		outlines: map[opentype.GlyphIndex]*st.Path{},
	}, nil
}

// fontBox is the ascent and descent the font box is measured with, in design
// units and both positive.
//
// It is gg's rule: OS/2 sTypoAscender and sTypoDescender, and hhea only when
// both are zero. The two disagree by about 15 % on the Go fonts (1579/395
// against 1935/432), which is the font-box drift backend/gg's parity test
// pins against the vector emitters — they read hhea. Taking gg's side keeps a
// chart laid out on the same rectangle by both raster backends.
func fontBox(f *opentype.Font) (asc, desc float64) {
	if os2, ok := f.Table("OS/2"); ok && len(os2) >= 74 {
		a := int16(binary.BigEndian.Uint16(os2[68:]))
		d := int16(binary.BigEndian.Uint16(os2[70:]))
		if a != 0 || d != 0 {
			return abs(float64(a)), abs(float64(d))
		}
	}
	return abs(float64(f.Ascent())), abs(float64(f.Descent()))
}

// glyph reports the glyph r maps to, if the font has one.
func (f *face) glyph(r rune) (opentype.GlyphIndex, bool) {
	g, ok := f.font.GlyphIndex(r)
	return g, ok && g != 0
}

// advance is g's advance in design units.
func (f *face) advance(g opentype.GlyphIndex) float64 {
	return f.of.AdvanceIndexUnits(g)
}

// outline is g's outline in ems, Y up, read on first use.
//
// Ems rather than design units because that is what stilus.GlyphCache keys
// on: it quantises the matrix's linear part to 1/64 and reads its scale as
// the em size in pixels. Handed design units, a 16 px label has a scale of
// 16/2048, which rounds to 1/64 — a glyph drawn at twice its size — and a
// 10 px one rounds to zero and is not drawn at all.
func (f *face) outline(g opentype.GlyphIndex) *st.Path {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.outlines[g]; ok {
		return p
	}
	p := new(st.Path)
	if segs, ok := f.of.GlyphOutline(g); ok {
		u := 1 / f.upem
		pt := func(q opentype.Point) (float32, float32) { return float32(q.X * u), float32(q.Y * u) }
		for _, s := range segs {
			switch s.Op {
			case opentype.SegMoveTo:
				p.MoveTo(pt(s.P[0]))
			case opentype.SegLineTo:
				p.LineTo(pt(s.P[0]))
			case opentype.SegQuadTo:
				cx, cy := pt(s.P[0])
				x, y := pt(s.P[1])
				p.QuadTo(cx, cy, x, y)
			case opentype.SegClose:
				p.Close()
			}
		}
	}
	f.outlines[g] = p
	return p
}

// ascent and descent are the font box in design units, both positive.
func (f *face) ascent() float64  { return f.asc }
func (f *face) descent() float64 { return f.desc }

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// fontSet is regular, bold and italic plus the fallbacks behind them: the
// same shape as backend/gg's, so the two are compared on the same fonts.
type fontSet struct {
	regular, bold, italic *face
	fallback              []*face
}

var (
	defaultOnce sync.Once
	defaultSet  *fontSet
	defaultErr  error
)

// defaultFonts are the Go fonts, which are what backend/gg embeds.
func defaultFonts() (*fontSet, error) {
	defaultOnce.Do(func() {
		var s fontSet
		if s.regular, defaultErr = parseFace(goregular.TTF, "embedded regular"); defaultErr != nil {
			return
		}
		if s.bold, defaultErr = parseFace(goregular.BoldTTF, "embedded bold"); defaultErr != nil {
			return
		}
		if s.italic, defaultErr = parseFace(goregular.ItalicTTF, "embedded italic"); defaultErr != nil {
			return
		}
		defaultSet = &s
	})
	return defaultSet, defaultErr
}

// pick is the face a run's font asks for. Bold wins over italic, as in gg.
func (s *fontSet) pick(weight int, italic bool) *face {
	switch {
	case weight >= 600:
		return s.bold
	case italic:
		return s.italic
	}
	return s.regular
}

// missing is what a rune no face has a glyph for is drawn as — backend/gg's
// and backend/pdf's rule.
const missing = '?'

// resolve finds the face and glyph r is drawn with: the run's own face, then
// each fallback, then `?` in the run's own face.
func (s *fontSet) resolve(primary *face, r rune) (*face, opentype.GlyphIndex) {
	if g, ok := primary.glyph(r); ok {
		return primary, g
	}
	for _, fb := range s.fallback {
		if g, ok := fb.glyph(r); ok {
			return fb, g
		}
	}
	g, _ := primary.glyph(missing)
	return primary, g
}

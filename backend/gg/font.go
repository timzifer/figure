package gg

import (
	"fmt"
	"sync"

	gogg "github.com/gogpu/gg"
	"github.com/gogpu/gg/text"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
)

// fontSet holds the sources a backend draws with: regular, bold and italic.
//
// Italic is there because figure asks for it. A variable a typesetter sets in
// notation is italic — see package mathtext — and both vector emitters honour
// [ir.FontRef.Italic] already, so a raster that ignored it would draw a
// different chart from the SVG beside it in the documentation.
type fontSet struct {
	regular *text.FontSource
	bold    *text.FontSource
	italic  *text.FontSource
	// fallback are the sources consulted, in order, for a rune the chosen
	// style has no glyph for. It is empty unless [WithFallbackFont] supplied
	// one, because this module embeds one typeface and a fallback has to come
	// from somewhere.
	fallback []*text.FontSource

	mu    sync.Mutex
	faces map[faceKey]text.Face
}

type faceKey struct {
	size   float64
	bold   bool
	italic bool
}

var (
	defaultOnce sync.Once
	defaultSet  *fontSet
	defaultErr  error
)

// defaultFonts parses the embedded Go fonts once per process.
//
// Parsing is not free and every chart needs the same two faces, so a package
// singleton is the right shape here. Faces derived from a source are safe for
// concurrent use, which is what makes sharing it across backends sound.
func defaultFonts() (*fontSet, error) {
	defaultOnce.Do(func() {
		reg, err := text.NewFontSource(goregular.TTF)
		if err != nil {
			defaultErr = fmt.Errorf("figure/backend/gg: parsing the embedded regular font: %w", err)
			return
		}
		bold, err := text.NewFontSource(gobold.TTF)
		if err != nil {
			defaultErr = fmt.Errorf("figure/backend/gg: parsing the embedded bold font: %w", err)
			return
		}
		italic, err := text.NewFontSource(goitalic.TTF)
		if err != nil {
			defaultErr = fmt.Errorf("figure/backend/gg: parsing the embedded italic font: %w", err)
			return
		}
		defaultSet = newFontSet(reg, bold, italic)
	})
	return defaultSet, defaultErr
}

// newFontSet builds a set from the sources it is given, falling back to the
// regular one for anything missing. A caller who supplies one font gets that
// font everywhere, which is a chart in one weight rather than an error.
func newFontSet(regular, bold, italic *text.FontSource) *fontSet {
	if bold == nil {
		bold = regular
	}
	if italic == nil {
		italic = regular
	}
	return &fontSet{
		regular: regular, bold: bold, italic: italic,
		faces: map[faceKey]text.Face{},
	}
}

// face returns a cached face for the given size, weight and style.
//
// Bold wins over italic when both are asked for, because there is no
// bold-italic source here and a bold label is more often a heading — but
// nothing in figure asks for both today: weights come from the theme and
// italics from notation.
func (f *fontSet) face(size float64, weight int, italic bool) text.Face {
	if size <= 0 {
		size = 12
	}
	k := faceKey{size: size, bold: weight >= 600, italic: italic}
	f.mu.Lock()
	defer f.mu.Unlock()
	if got, ok := f.faces[k]; ok {
		return got
	}
	src := f.regular
	switch {
	case k.bold:
		src = f.bold
	case k.italic:
		src = f.italic
	}
	fc := src.Face(size)
	if len(f.fallback) > 0 {
		faces := make([]text.Face, 0, 1+len(f.fallback))
		faces = append(faces, fc)
		for _, fb := range f.fallback {
			faces = append(faces, fb.Face(size))
		}
		// A MultiFace takes its metrics from the first face, which is what
		// keeps a row of labels on one baseline whether or not a fallback
		// glyph appeared in one of them.
		if multi, err := text.NewMultiFace(faces...); err == nil {
			fc = multi
		}
	}
	f.faces[k] = fc
	return fc
}

// missing is what a rune no face has a glyph for is drawn as.
//
// It is `?`, which is what backend/pdf writes for a rune outside its encoding,
// and it is drawn rather than dropped for that backend's reason: a label that
// quietly loses a character says something the data does not. The embedded Go
// fonts cover Latin, Greek, Cyrillic and the common punctuation and
// mathematical operators, and not the mathematical angle brackets U+27E8 and
// U+27E9 a Bloch sphere's |0⟩ is written with — [WithFallbackFont] is how a
// chart that needs them supplies them.
const missing = '?'

// drawable is s with every rune the face cannot draw replaced by [missing].
//
// It returns s itself when every rune is present, which is every label of
// every chart in the documentation, so the common path allocates nothing.
func drawable(face text.Face, s string) string {
	ok := true
	for _, r := range s {
		if !face.HasGlyph(r) {
			ok = false
			break
		}
	}
	if ok {
		return s
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if !face.HasGlyph(r) {
			r = missing
		}
		out = append(out, r)
	}
	return string(out)
}

// withFallback returns f with the given fallback sources behind it, or f
// itself where there are none.
//
// It copies rather than mutating, because the embedded set is a package
// singleton shared by every backend in the process: one chart asking for a
// fallback must not change what the next one draws. The copy gets its own face
// cache for the same reason.
func (f *fontSet) withFallback(fallback []*text.FontSource) *fontSet {
	if len(fallback) == 0 {
		return f
	}
	return &fontSet{
		regular: f.regular, bold: f.bold, italic: f.italic,
		fallback: append(append([]*text.FontSource(nil), f.fallback...), fallback...),
		faces:    map[faceKey]text.Face{},
	}
}

// apply sets the face on a context and returns it, so callers can measure with
// the very face they are about to draw with.
func (f *fontSet) apply(c *gogg.Context, size float64, weight int, italic bool) text.Face {
	fc := f.face(size, weight, italic)
	c.SetFont(fc)
	return fc
}

package stilus

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"

	"github.com/timzifer/figure/ir"
	st "github.com/timzifer/stilus"
)

// Format is an output image format.
type Format uint8

// The supported formats.
const (
	FormatPNG Format = iota
	FormatJPEG
)

// Option configures a target.
type Option func(*options)

type options struct {
	deferred    bool
	fonts       *fontSet
	fallback    []*face
	fontErr     error
	jpegQuality int
}

// WithFont replaces the embedded Go fonts with supplied TrueType or OpenType
// files.
//
// Pass bold or italic as nil to reuse regular for that style. Italic is worth
// supplying for a chart whose labels carry notation: a typesetter sets
// variables italic, and the vector emitters ask the viewer for an italic face
// whether or not one is given here.
func WithFont(regular, bold, italic []byte) Option {
	return func(o *options) {
		reg, err := parseFace(regular, "supplied regular")
		if err != nil {
			o.fontErr = err
			return
		}
		set := &fontSet{regular: reg, bold: reg, italic: reg}
		if len(bold) > 0 {
			if set.bold, err = parseFace(bold, "supplied bold"); err != nil {
				o.fontErr = err
				return
			}
		}
		if len(italic) > 0 {
			if set.italic, err = parseFace(italic, "supplied italic"); err != nil {
				o.fontErr = err
				return
			}
		}
		o.fonts = set
	}
}

// WithFallbackFont adds fonts consulted, in order, for a rune the chart's own
// font has no glyph for.
//
// The embedded Go fonts cover Latin, Greek, Cyrillic and the common
// punctuation and mathematical operators. They do not cover the mathematical
// angle brackets U+27E8 and U+27E9 — a Bloch sphere's |0⟩ — nor any CJK
// script. A rune no face can draw is written as `?`, which is what backend/pdf
// writes for a rune outside its encoding: a label that quietly loses a
// character says something the data does not.
//
// It composes with [WithFont] in either order, and the font box stays the
// chart's own font's, so a fallback glyph in one label does not move the
// baseline of the row it is in.
func WithFallbackFont(ttf ...[]byte) Option {
	return func(o *options) {
		for i, b := range ttf {
			if len(b) == 0 {
				continue
			}
			f, err := parseFace(b, fmt.Sprintf("fallback %d", i))
			if err != nil {
				o.fontErr = err
				return
			}
			o.fallback = append(o.fallback, f)
		}
	}
}

// JPEGQuality sets the JPEG encoder quality, 1 to 100. The default is 90.
func JPEGQuality(q int) Option {
	return func(o *options) {
		if q > 0 && q <= 100 {
			o.jpegQuality = q
		}
	}
}

func build(opts []Option) options {
	o := options{jpegQuality: 90}
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// resolve is the font set a target draws with: the supplied one or the
// embedded one, with any fallbacks behind it. It copies rather than editing,
// because the embedded set is shared by every backend in the process.
func (o options) resolve() (*fontSet, error) {
	if o.fontErr != nil {
		return nil, o.fontErr
	}
	fonts := o.fonts
	if fonts == nil {
		var err error
		if fonts, err = defaultFonts(); err != nil {
			return nil, err
		}
	}
	if len(o.fallback) == 0 {
		return fonts, nil
	}
	cp := *fonts
	cp.fallback = append(append([]*face(nil), fonts.fallback...), o.fallback...)
	return &cp, nil
}

// open makes a backend for s, drawing into spare if it is the right size and
// into a new buffer otherwise.
func (o options) open(s ir.Surface, glyphs *st.GlyphCache, spare *image.RGBA) (*backend, error) {
	fonts, err := o.resolve()
	if err != nil {
		return nil, err
	}
	img, dpr, err := buffer(s, spare)
	if err != nil {
		return nil, err
	}
	return newBackend(img, s.WidthPx, s.HeightPx, dpr, fonts, glyphs), nil
}

// buffer is the pixel buffer for a surface: logical size times the device
// pixel ratio, as gg's NewContextWithScale makes it. A spare buffer of exactly
// that size is cleared and handed back rather than allocated again.
func buffer(s ir.Surface, spare *image.RGBA) (*image.RGBA, float64, error) {
	if s.WidthPx <= 0 || s.HeightPx <= 0 {
		return nil, 0, errors.New("figure/backend/stilus: chart size must be positive")
	}
	dpr := s.DPR
	if dpr <= 0 {
		dpr = 1
	}
	w := int(float64(s.WidthPx)*dpr + 0.5)
	h := int(float64(s.HeightPx)*dpr + 0.5)
	r := image.Rect(0, 0, w, h)
	if spare != nil && spare.Rect == r {
		clear(spare.Pix)
		return spare, dpr, nil
	}
	return image.NewRGBA(r), dpr, nil
}

// PNG returns a target that writes a PNG file.
func PNG(path string, opts ...Option) ir.Target {
	return &target{path: path, format: FormatPNG, opts: build(opts)}
}

// JPEG returns a target that writes a JPEG file.
func JPEG(path string, opts ...Option) ir.Target {
	return &target{path: path, format: FormatJPEG, opts: build(opts)}
}

// Writer returns a target that encodes into w in the given format.
func Writer(w io.Writer, format Format, opts ...Option) ir.Target {
	return &target{w: w, format: format, opts: build(opts)}
}

type target struct {
	path   string
	w      io.Writer
	format Format
	opts   options
	b      *backend
}

func (t *target) Open(s ir.Surface) (ir.Backend, error) {
	b, err := t.opts.open(s, nil, nil)
	if err != nil {
		return nil, err
	}
	t.b = b
	return b, nil
}

func (t *target) Close() error {
	if t.b == nil {
		return nil
	}
	img := t.b.img
	t.b = nil
	if t.w != nil {
		return t.encode(t.w, img)
	}
	f, err := os.Create(t.path)
	if err != nil {
		return err
	}
	if err := t.encode(f, img); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (t *target) encode(w io.Writer, img *image.RGBA) error {
	if t.format == FormatJPEG {
		return jpeg.Encode(w, img, &jpeg.Options{Quality: t.opts.jpegQuality})
	}
	return png.Encode(w, img)
}

// Surface is a target that draws into memory rather than into a file.
//
// [PNG] and [JPEG] encode a chart and are done with it. A Surface keeps the
// pixels: it stays open, its image can be read after every frame, and it can be
// resized and repainted in place. That is what a window needs, and what any
// caller compositing a chart into a larger picture needs too.
//
//	s := stilus.NewSurface()
//	live, err := p.Live(s)
//	// ... draw, resize, draw again ...
//	img := s.Image()
//
// The glyph cache belongs to the surface rather than to a frame, so a chart
// opened again on the same surface finds its glyphs already rasterised; and so
// does the pixel buffer, which a chart rendered again at the same size draws
// into rather than allocating a megabyte per frame.
//
// A Surface draws one chart at a time and is not safe for concurrent use.
type Surface struct {
	opts   options
	glyphs st.GlyphCache
	b      *backend
	d      *deferred   // the backend of a [Deferred] surface, instead of b
	spare  *image.RGBA // the last buffer, reused by the next Open of its size
}

// NewSurface returns an in-memory target. It takes the same options as the file
// targets: the font is the one thing a raster backend has to be told about.
func NewSurface(opts ...Option) *Surface {
	return &Surface{opts: build(opts)}
}

// Open prepares the surface for a chart of the given size. Opening a surface
// that is already open replaces what it held.
func (s *Surface) Open(sf ir.Surface) (ir.Backend, error) {
	if s.opts.deferred {
		d, err := s.opts.openDeferred(sf, &s.glyphs)
		if err != nil {
			return nil, err
		}
		s.d = d
		return d, nil
	}
	if s.b != nil {
		s.spare = s.b.img
	}
	b, err := s.opts.open(sf, &s.glyphs, s.spare)
	if err != nil {
		return nil, err
	}
	s.b, s.spare = b, nil
	return b, nil
}

// Close ends the chart. The image is not available afterwards; the surface
// keeps the buffer for the next Open of the same size.
func (s *Surface) Close() error {
	if s.b != nil {
		s.spare = s.b.img
	}
	s.b, s.d = nil, nil
	return nil
}

// Composite rasterises the latest frame of a [Deferred] surface onto dst,
// which it clears first. dst is in device pixels and should be
// [Surface.PixelSize]; what lies outside that is not drawn.
//
// It is the one place a deferred surface touches pixels, so it is where a
// window calls it: in its draw callback, when the generation has moved since
// the last time or the destination is new.
func (s *Surface) Composite(dst Compositor) error {
	if s.d == nil {
		return errors.New("figure/backend/stilus: Composite needs an open deferred surface")
	}
	return s.d.composite(dst)
}

// PixelSize reports the size in device pixels of what the surface draws: its
// logical size times the device pixel ratio, rounded as its buffer is.
func (s *Surface) PixelSize() (w, h int) {
	switch {
	case s.d != nil:
		return s.d.pixelSize()
	case s.b != nil:
		return s.b.img.Rect.Dx(), s.b.img.Rect.Dy()
	}
	return 0, 0
}

// Image returns what has been drawn, or nil before the first [Surface.Open].
//
// The image is the surface's own buffer rather than a copy, so it is valid
// until the next frame overwrites it. A caller keeping one past that must copy
// it. Its pixels are premultiplied, as an [image.RGBA]'s are. A [Deferred]
// surface has no pixels and returns nil.
func (s *Surface) Image() image.Image {
	if s.b == nil {
		return nil
	}
	return s.b.img
}

// Generation reports a counter that changes whenever the pixels do.
//
// It answers "is this frame the same as the last one" without comparing two
// buffers, which is what lets a window skip re-uploading a chart nobody has
// touched. It is zero for a surface that has not been opened, and it is not a
// frame number: a frame that painted nothing does not advance it.
func (s *Surface) Generation() uint64 {
	switch {
	case s.d != nil:
		return s.d.gen
	case s.b != nil:
		return s.b.gen
	}
	return 0
}

// Size reports the surface's logical size, and the device pixel ratio its
// buffer is scaled by.
func (s *Surface) Size() (w, h int, dpr float64) {
	switch {
	case s.d != nil:
		return s.d.w, s.d.h, s.d.dpr
	case s.b != nil:
		return s.b.w, s.b.h, s.b.dpr
	}
	return 0, 0, 0
}

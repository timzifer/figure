package stilus

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"

	"github.com/timzifer/figure/ir"
)

// Option configures a target.
type Option func(*options)

type options struct {
	fallback [][]byte
}

// WithFallbackFont adds fonts consulted, in order, for a rune the chart's own
// font has no glyph for — backend/gg's option of the same name.
func WithFallbackFont(ttf ...[]byte) Option {
	return func(o *options) { o.fallback = append(o.fallback, ttf...) }
}

// PNG returns a target that writes a PNG file.
func PNG(path string, opts ...Option) ir.Target { return &target{path: path, opts: build(opts)} }

// Writer returns a target that encodes a PNG into w.
func Writer(w io.Writer, opts ...Option) ir.Target { return &target{w: w, opts: build(opts)} }

func build(opts []Option) options {
	var o options
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

type target struct {
	path string
	w    io.Writer
	opts options
	img  *image.RGBA
}

func (t *target) Open(s ir.Surface) (ir.Backend, error) {
	if s.WidthPx <= 0 || s.HeightPx <= 0 {
		return nil, errors.New("figure/backend/stilus: chart size must be positive")
	}
	fonts, err := defaultFonts()
	if err != nil {
		return nil, err
	}
	if len(t.opts.fallback) > 0 {
		cp := *fonts
		cp.fallback = nil
		for i, ttf := range t.opts.fallback {
			f, err := parseFace(ttf, fmt.Sprintf("fallback %d", i))
			if err != nil {
				return nil, err
			}
			cp.fallback = append(cp.fallback, f)
		}
		fonts = &cp
	}
	dpr := s.DPR
	if dpr <= 0 {
		dpr = 1
	}
	// The surface is in logical units and the buffer in device pixels, as
	// with gg: the backend's base transform is the device scale.
	w := int(float64(s.WidthPx)*dpr + 0.5)
	h := int(float64(s.HeightPx)*dpr + 0.5)
	t.img = image.NewRGBA(image.Rect(0, 0, w, h))
	return newBackend(t.img, dpr, fonts), nil
}

// Surface is a target that draws into memory and keeps the pixels, as
// backend/gg's Surface does. It keeps its glyph cache across opens, which is
// what a chart redrawn every frame gets — gg's face caches are process-wide
// for the same reason.
type Surface struct {
	opts options
	b    *backend
}

// NewSurface returns an in-memory target.
func NewSurface(opts ...Option) *Surface { return &Surface{opts: build(opts)} }

// Open prepares the surface for a chart of the given size.
func (s *Surface) Open(sf ir.Surface) (ir.Backend, error) {
	t := target{opts: s.opts}
	nb, err := t.Open(sf)
	if err != nil {
		return nil, err
	}
	b := nb.(*backend)
	if s.b != nil {
		b.glyphs = s.b.glyphs
	}
	s.b = b
	return b, nil
}

// Close keeps the pixels; Image stays valid.
func (s *Surface) Close() error { return nil }

// Image is what has been drawn, or nil before the first Open.
func (s *Surface) Image() image.Image {
	if s.b == nil {
		return nil
	}
	return s.b.img
}

func (t *target) Close() error {
	if t.img == nil {
		return nil
	}
	img := t.img
	t.img = nil
	if t.w != nil {
		return png.Encode(t.w, img)
	}
	f, err := os.Create(t.path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

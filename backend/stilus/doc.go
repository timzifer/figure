// Package stilus renders figure charts through github.com/timzifer/stilus, a
// sparse CPU rasterizer with no dependencies, and draws text through
// github.com/go-opentype/opentype.
//
// This is a separate Go module. Importing figure's core gets you a stdlib-only
// dependency graph and SVG output; importing this module adds stilus,
// opentype and the fonts it embeds, and gets you raster output with zero CGO.
// It is the proposed successor to backend/gg — see docs/adr/0094.
//
//	import stilusbackend "github.com/timzifer/figure/backend/stilus"
//
//	err := p.Render(stilusbackend.PNG("chart.png"))
//
// The module requires Go 1.27.1, because opentype does.
//
// # What is drawn how
//
// Every IR call maps onto stilus.Canvas calls. Transforms are composed here
// and handed to every call, because stilus keeps no transform state; clips go
// onto the canvas's own clip stack, and a rectangle reaches it as ClipRect,
// which costs nothing per pixel.
//
// The targets are [PNG], [JPEG] and [Writer], which encode a chart, and
// [Surface], which keeps its pixels for a window or a compositor. A Surface's
// backend implements ir.Partial and ir.Resizer, and stamps its buffer with a
// generation whenever a frame changes it.
//
// # Text
//
// Text is outlines. Each glyph's outline is read once per font out of
// opentype, scaled to one em, and filled through a stilus.GlyphCache, which
// rasterises it once per size and subpixel phase and composites it from then
// on. The outline is in ems rather than design units because the cache
// quantises the matrix to 1/64 and reads its scale as the em size in pixels.
//
// Measure answers from the font Text draws with. Advances are the font's own
// widths, unrounded — gg rounds each to a whole pixel, the vector emitters do
// not. The font box is OS/2's typographic ascender and descender, with hhea
// only when both are zero, which is gg's rule; see docs/adr/0094 for why it is
// gg's rather than the vector emitters'.
//
// The module embeds Go Regular, Go Bold and Go Italic, from
// github.com/go-opentype/fonts. They cover Latin, Greek, Cyrillic and the
// common punctuation and mathematical operators, and not CJK or the
// mathematical angle brackets U+27E8 and U+27E9. [WithFallbackFont] supplies
// the rest; a rune no face can draw is written as `?`.
//
// There is no shaping and no kerning yet. opentype has both; turning them on
// moves every label's width and is its own change.
//
// # Colour
//
// The buffer is premultiplied sRGB, which is what an [image.RGBA] holds.
// Gradients are sampled in linear light, as gg samples them.
package stilus

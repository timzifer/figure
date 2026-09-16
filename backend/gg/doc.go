// Package gg renders figure charts through github.com/gogpu/gg.
//
// This is a separate Go module. Importing figure's core gets you a
// stdlib-only dependency graph and SVG output; importing this module adds the
// GoGPU stack and gets you raster output — still with zero CGO, so
// CGO_ENABLED=0 builds and cross-compiles exactly as before.
//
//	import ggbackend "github.com/timzifer/figure/backend/gg"
//
//	err := p.Render(ggbackend.PNG("chart.png"))
//
// # What this backend uses, and what it deliberately does not
//
// It touches only the gg root package and gg/text. It does not import gg/gpu,
// gg/scene or gg/recording. That is a deliberate limit on the coupling surface
// (docs/adr/0006): gg is young and moves fast, so the narrower the adapter's
// contact with it, the cheaper it is to follow. It also means the CPU
// rasterizer is what runs — no GPU device is ever created, and CI needs no
// graphics hardware.
//
// The GPU tier, PDF output and a native window are later milestones. When they
// arrive they will arrive here, behind the same ir.Backend interface.
//
// # Text
//
// gg ships no default font, so this backend embeds Go Regular, Go Bold and Go
// Italic (golang.org/x/image/font/gofont, BSD-3-Clause) and uses them unless a
// font is supplied with [WithFont]. Measurement comes from the same face that
// draws the text, so metrics here are exact — unlike the built-in SVG backend,
// which approximates.
//
// # What the embedded fonts cover, and what to do about the rest
//
// The Go fonts cover Latin, Greek, Cyrillic, the common punctuation and the
// mathematical operators. They do not cover any CJK script, and they do not
// cover the mathematical angle brackets U+27E8 and U+27E9 — which is what a
// Bloch sphere's |0⟩ is written with, so a raster of that chart has a hole in
// its labels where a vector one drawn in the same code does not: the SVG and
// PDF viewers supply the glyph from the reader's own fonts, and a PNG has no
// reader to ask.
//
// [WithFallbackFont] is the answer: fonts consulted, in order, for a rune the
// chart's own font has no glyph for, with the metrics staying the chart's own
// so a fallback glyph does not move the baseline of the row it is in. A rune
// no face can draw is written as `?`, which is what backend/pdf writes for a
// rune outside its encoding and for the same reason — a label that quietly
// loses a character says something the data does not.
package gg

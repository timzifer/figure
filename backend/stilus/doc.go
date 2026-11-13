// Package stilus renders figure charts through github.com/timzifer/stilus, a
// sparse CPU rasterizer with no dependencies, and draws text through
// github.com/go-opentype/opentype.
//
// SPIKE. This package exists to measure whether stilus can replace gg as the
// raster backend: how far its pictures are from gg's, what it costs per frame,
// and what is missing. It is not part of a release and has no ADR yet.
//
//	err := p.Render(stilusbackend.PNG("chart.png"))
//
// # What is drawn how
//
// Every IR call maps onto one stilus.Canvas call. Transforms are composed here
// and handed to every call, because stilus keeps no transform state; clips go
// onto the canvas's own clip stack, and a rectangle reaches it as ClipRect —
// which costs nothing per pixel — for the reason backend/gg's Push gives.
//
// Text is outlines: each glyph's outline is read once per font out of
// opentype, kept as a stilus.Path in design units, and filled through a
// stilus.GlyphCache, which rasterises it once per size and subpixel position.
// Advances and vertical metrics come from the same fonts in design units,
// unrounded, so Measure answers with the width Text draws.
//
// # Not done in the spike
//
// No shaping (GSUB/GPOS) and no kerning, which keeps the advances comparable
// to gg's. No ir.Partial and no ir.Resizer, so no live window. An Image call
// converts its picture on every call.
package stilus

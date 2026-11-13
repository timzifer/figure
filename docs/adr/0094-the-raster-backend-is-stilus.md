# 0094 — The raster backend is stilus, and gg is deprecated before v1 freezes it

**Status:** Proposed · **Date:** 2026-11-13 · **Revisits:** [ADR 0006](0006-gg-coupling-surface.md), [ADR 0021](0021-native-window.md), [ADR 0022](0022-gpu-tier.md), [ADR 0005](0005-go-version.md)

## Context

`backend/gg` is figure's only rasterizer. Every PNG in `docs/images`, the
native window and the opt-in GPU tier go through it, and it is pinned to gg
v0.52.5 exactly. Three things have made that arrangement worth looking at
again.

**`timzifer/stilus` exists.** It is a sparse CPU rasterizer in pure Go with no
dependencies at all — not even on a font library — and it is what
`timzifer/cera`, a PDF renderer, draws with. Its cost per path is edge length
plus covered spans; after warm-up it draws without allocating; a rectangle clip
costs nothing per pixel; and a page drawn in bands is byte for byte the page
drawn whole. Each of those is a rule this repository already enforces on its
own code (the allocation gate, `ir.Path.AsRect`, ADR 0012).

**Text is solved next to it.** stilus knows no fonts, but cera shows the
arrangement: `github.com/go-opentype/opentype` parses the font and hands back
glyph outlines, and `stilus.GlyphCache` rasterises each glyph once per size and
subpixel phase and composites it from then on. opentype also does GSUB, GPOS
and kerning, which gg at this pin does not, and `go-opentype/fonts` carries the
Go fonts, Noto and DejaVu as importable packages.

**gg is the ceiling on the GPU stack.** `backend/window` holds gogpu at v0.53.1
because gg v0.52.5 does not build against wgpu v0.33 or later. `stilus/gpu`,
which composites stilus's spans onto a wgpu texture and now has a `Presenter`
that draws one into a caller's render pass, requires wgpu v0.34. One module
graph has one wgpu, so the two cannot meet in a module that still imports gg.

And **v1 is not tagged yet.** [ADR 0084](0084-what-v1-promises.md) freezes
`backend/gg` and `backend/window` together with the core. Whatever the raster
backend is on that day, its API is a promise for the life of v1.

### The spike

A spike on `spike/stilus-backend` implemented `ir.Backend` on a
`stilus.Canvas`, with text through opentype and `stilus.GlyphCache`, as the
nested module `backend/stilus`. It was compared against `backend/gg` on all 63
gallery figures, rendering into memory with no encoding.

*Picture.* Side by side the two are indistinguishable at a glance: italic,
rotation, typeset notation, images, dashes, hatching and projected scenes all
draw. The faint seams between a surface's quads are in gg's picture too; they
are conflation, not a stilus defect. Once the font box is taken from the same
table gg uses (below), the mean channel difference is about 1 in 255 on most
figures, and the channels more than 32 apart are around 1 %. What remains is
two things: gg hints glyph shapes and rounds every glyph advance to a whole
pixel, which moves tick labels and therefore the plot rectangle by a fraction
of a pixel, and the two antialias differently — `parallel`, sixty crossing
hairlines, is the worst figure at 7 % of channels more than 32 apart and looks
the same.

*Time.* Ten renders per figure, three runs, Apple Silicon:

| | gg | stilus | |
|---|---|---|---|
| all 63 figures | 1966 ms | 234 ms | 8.4× |
| a typical chart (bars, scatter, signal) | 5–8 ms | 0.2–0.5 ms | 15–30× |
| `map` | 562 ms | 6 ms | 90× |
| `hatched-area` | 179 ms, 645 MB | 2 ms, 2 MB | 85× |
| `market` | 300 ms | 1.4 ms | 209× |
| `density`, `market-live` | 65 / 125 ms | 55 / 115 ms | 1.1× |

Allocations per render fall from 7 000–50 000 to 200–2 000. With an empty glyph
cache on every render a typical chart costs 0.25–0.6 ms, so the margin is not
the cache. Where the factor is near one, the time is figure's own — a million
rows binned, a stream snapshotted — and the rasterizer is no longer what is
measured. The largest gaps sit on figures with non-rectangular clips, which gg
rasterises into a coverage mask consulted on every call inside them; that is
the cost [AGENTS.md](../../AGENTS.md)'s rectangular-clip trap describes, and it
was not measured further.

## Decision

**`backend/stilus` becomes figure's raster backend. `backend/gg` is deprecated
— not redirected — before v1, so that v1 freezes stilus's API rather than
gg's.**

- `backend/stilus` is a nested module, for the reason every backend with a
  dependency is ([ADR 0001](0001-module-layout.md)). Its graph is stilus,
  opentype, the fonts it embeds, brotli (opentype's, for WOFF2) and the core.
- **Text is outlines in ems.** Each glyph's outline is read once per font out
  of opentype, scaled to one em, and filled through `stilus.GlyphCache` with a
  matrix whose scale is the font size. Ems and not design units: the cache
  quantises the matrix's linear part to 1/64, so a 16 px label in a 2048-unit
  font has a scale of 16/2048, which rounds to 1/64 — twice the size — and a
  10 px label rounds to zero and is not drawn. The spike shipped that bug for
  one run.
- **Measure answers from the font that draws, unrounded.** Advances are the
  font's own `hmtx` widths in design units scaled to the size, without gg's
  per-glyph rounding to whole pixels — which is what the vector emitters do and
  what `Measure` promises. The **font box** is gg's: OS/2 `sTypoAscender` and
  `sTypoDescender`, with `hhea` only when both are zero. On the Go fonts the two
  tables disagree by about 15 % (1579/395 against 1935/432), which is the
  font-box drift `backend/gg`'s parity test pins against the vector emitters;
  taking gg's side keeps every chart laid out on the rectangle it had, so the
  switch moves pixels by antialiasing rather than by layout. Whether the raster
  path should instead move to `hhea` and agree with SVG and PDF is a separate
  decision, made once, with every PNG regenerated for it.
- A rune no face can draw is written as `?`, after the fallbacks, as in
  `backend/gg` and `backend/pdf`.
- **The gallery moves with it.** `backend/gg/cmd/gallery` becomes
  `backend/stilus/cmd/gallery`, and every PNG in `docs/images` is regenerated
  in the commit that moves it — the two cannot be separated without a commit
  whose figures check red. The SVGs do not change, and the PNGs check clean on
  arm64 and on amd64.
- **The window moves with it**, and that is what lifts the wgpu ceiling: with
  gg out of `backend/window`'s graph, gogpu follows wgpu to whatever release
  `stilus/gpu` requires. The first step presents a `backend/stilus` surface the
  way it presents gg's today, one texture per changed frame. Compositing spans
  straight onto a `stilus/gpu.Target` and presenting it with `Presenter` is a
  later step and needs no further record; it is ADR 0021's "Revisit if"
  arriving from another library.
- **`backend/gg` is deprecated, not redirected.** The module and package carry
  a `// Deprecated:` notice pointing at `backend/stilus`, keep building and
  keep their tests for at least one minor release, and are not tagged `v1`.
  `backend/gg/gpu` is deprecated with it: it is gg's GPU tier, and there is
  nothing of stilus's for it to switch on.

### Why not redirect `backend/gg` to stilus

It is the smaller diff for a caller, and it is wrong in three ways.

- **A patch upgrade would change every picture.** A caller who pinned
  `backend/gg` has PNGs of their own, and perhaps golden files; swapping the
  rasterizer under an unchanged import path moves every one of them without a
  word. A deprecation says it.
- **The name would lie.** A module called `gg` that does not contain gg is a
  trap for the next reader, and `WithFont`'s parse errors, `JPEGQuality` and the
  GPU tier all describe gg.
- **It would have to be frozen anyway.** Redirecting before v1 means v1 freezes
  a compatibility shim; deprecating means v1 freezes nothing of gg's at all.

## Consequences

- `backend/stilus` declares `go 1.27.1`, because opentype does.
  [ADR 0005](0005-go-version.md)'s "Revisit if" anticipated modules diverging;
  the core and every other module stay at 1.25. CI needs a 1.27 job for this
  module, and the committed `go.work` cannot simply list it without raising the
  whole workspace — that has to be decided when the module lands.
- Extending `internal/sfnt` instead of taking opentype would avoid that floor
  and the brotli dependency. It was rejected: that package says of itself that
  it is "deliberately not a font library" and does not rasterise, shape or
  kern, and a raster backend needs at least outlines and, soon, shaping.
- The raster backend gains kerning and shaping as opentype grows them, rather
  than when gg does. The spike draws neither, so that it measures the same
  advances gg does; turning them on moves layout and is its own change.
- [ADR 0006](0006-gg-coupling-surface.md)'s pinning discipline applies to
  stilus and opentype: exact versions, upgraded in their own commits.
- The allocation gate learns a second backend. The spike's numbers suggest the
  rasterizer is now well under the frame cost of the model, which makes
  `.github/scripts/allocgate.awk`'s per-row checks worth extending to a raster
  render rather than only to the recorder.
- `internal/release` gains `backend/stilus` and loses `backend/gg` from the
  modules that tag `v1` with the core.

### What has to exist before this is Accepted

The spike implemented drawing and nothing else. Since then the backend has
gained what `backend/gg` offers a caller:

- `ir.Partial` and `ir.Resizer`, and a `Surface` with gg's semantics — `Open`
  replaces, `Close` releases, `Image`, `Size`, and a `Generation` stamped only
  by a frame that touched the buffer, from one counter for the process so a
  replaced buffer is never mistaken for its predecessor. gg's surface tests
  are ported unchanged and pass.
- `WithFont(regular, bold, italic)`, `WithFallbackFont`, `JPEG`,
  `JPEGQuality`, and `Writer` with gg's signature.
- Gradients sampled in linear light into a 256-entry ramp, built once per set
  of stops: black to white is 188 at the middle, as gg and `palette.Lerp` make
  it. stilus interpolates its own knots in the encoded values, which is why the
  ramp is sampled here rather than handed over as knots.
- `Image` converts into a buffer the backend keeps rather than into a cache
  keyed by the picture. A density raster draws into the same image every
  frame, and a cache keyed by the pointer would show the first frame for ever;
  the conversion is one pass over the picture's pixels, which is what gg pays
  too.
- Tests in the module: a run aligned to end at its advance inks where one
  aligned to start does; advances agree with the SVG backend to 0.01 px and
  the font box within gg's stated drift; text from 6 px up is drawn at its
  size; `?` and fallbacks; rectangle and path clips; a mirrored transform; a
  hairline; device scale; the gradient midpoint; both encoders; and a redraw
  over 20 000 rows allocating what one over 200 does.

The pictures did not move and the renders did not slow down: the gallery's
channel differences against gg are what they were after the font box was
aligned, and the 63 figures render in 221 ms against 234 ms.

What remains before this is Accepted is the move itself:

- a CI job on Go 1.27 for this module, and its benchmarks in
  `.github/scripts/allocgate.awk`;
- `internal/release` learning the module;
- the gallery moved and `docs/images` regenerated, in one commit;
- `backend/window` on this backend, then the deprecation notices on
  `backend/gg` and `backend/gg/gpu`.

## Revisit if

stilus or opentype stops being maintained, or a rasterizer appears that draws
straight onto a GPU surface without a CPU pass at a comparable cost. The first
would put figure back where gg had it, with one young dependency carrying every
raster picture; the second would make the CPU pass the thing to remove.

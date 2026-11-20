# Working in the backends

Notes for changing the backends, moved here from the root
[AGENTS.md](../AGENTS.md), which holds the rules that apply everywhere — read
that one first. This file covers stilus and the deprecated gg, pins, PDF fonts,
embedded payloads in figures, rectangular clips, canvas and the window.

## Things that will bite

**`backend/gg` must not import `gg/gpu`.** Importing it activates the GPU tier,
pulls `wgpu` and `goffi` into the build, and makes CI need graphics hardware.
The CPU rasterizer is the supported path —
[ADR 0006](../docs/adr/0006-gg-coupling-surface.md).

**gg is pinned exactly.** Upgrading it is a deliberate change with its own
commit, not a side effect of `go get -u`.

**The raster backend is `backend/stilus`, and `backend/gg` is deprecated.**
New raster work goes into stilus; gg keeps building, unchanged, through
v0.19.0 and is removed in v0.20.0 with its GPU tier, and is never tagged v1
([ADR 0094](../docs/adr/0094-the-raster-backend-is-stilus.md)).
Four things there bite:

- *A glyph outline is in ems, not design units.* `stilus.GlyphCache`
  quantises the matrix's linear part to 1/64 and reads its scale as the em size
  in pixels. Handed design units, a 16 px label in a 2048-unit font has a scale
  of 16/2048, which rounds to 1/64 — twice its size — and a 10 px label rounds
  to zero and is laid out and never drawn. `TestSmallTextIsDrawnAtItsSize` is
  that bug.
- *The font box is OS/2's, advances are unrounded.* Ascent and descent come
  from `sTypoAscender`/`sTypoDescender`, hhea only when both are zero — gg's
  rule, which keeps every chart on the rectangle gg laid it out on. The vector
  emitters read hhea; the 15 % between the two is the drift
  `TestMeasurementAgreesWithTheVectorEmitter` allows. Advances are the font's
  own widths, not gg's whole pixels, and agree with the SVG backend to
  0.01 px.
- *The module declares Go 1.27.1*, because go-opentype does, and so does
  `backend/window`, which draws through it. The committed workspace says
  1.27.1 for that reason; every other module still declares 1.25, and CI's
  minimum-toolchain job builds them from a workspace that leaves those two out
  with `GOTOOLCHAIN=local`. Without that the toolchain switches itself up and
  the 1.25 claim is checked by nothing.
- *A module another one imports is tagged before the importer can require
  it.* A require on a version that does not exist breaks the workspace too,
  because the module graph is read before the workspace replaces it. So
  v0.15.0 went out in three steps — the core with gg and arrow, then
  `backend/stilus` once its require named that core, then `backend/window`
  once its require named stilus — which is CONTRIBUTING's "When a require
  line does have to move", and is what the next new module will need too.

**PDF is figure's own emitter, not `gg-pdf`.** That library cannot draw
geometry — its path operations reach a stub in `gxpdf` — so the roadmap's plan
of routing PDF through gg's recording API would have produced pages with tick
labels and nothing else. `gg/recording` therefore stays out of `backend/gg`,
and [ADR 0006](../docs/adr/0006-gg-coupling-surface.md)'s import rule is unchanged.
See [ADR 0009](../docs/adr/0009-pdf-backend.md) for the evidence.

**The PDF backend measures with the font it draws with, whichever font that
is.** `internal/fontmetrics` carries Helvetica's advance table and PDF's
base-14 Helvetica is that font; an *embedded* face measures through
`embeddedFace`, out of its own `hmtx` and `hhea`
([ADR 0038](../docs/adr/0038-embedded-fonts.md)). Every other backend
approximates; this one does not, and the promise has to survive the font
changing — measuring against Helvetica and drawing in Noto Sans sizes every
margin from a typeface the document does not contain. Do not "improve" either
half by measuring with something else.

**The raster backend's embedded fonts do not cover everything, and a rune they
cannot draw is written as `?`.** The Go fonts cover Latin, Greek, Cyrillic, the
common punctuation and the mathematical operators; they do not cover CJK, and
they do not cover the mathematical angle brackets U+27E8 and U+27E9 a Bloch
sphere's `|0⟩` is written with. So the SVG of a chart and the PNG of the same
chart can differ in their labels — the vector viewer asks the reader's own
fonts and a PNG has nobody to ask. `stilus.WithFallbackFont` (and the
deprecated `gg.WithFallbackFont`) supplies the coverage;
the substitution is `backend/pdf`'s rule for a rune outside WinAnsi, so a
missing glyph is visible rather than silent. `TestAMissingRuneIsAQuestionMarkUnlessAFallbackHasIt`
in backend/stilus and `TestTheEmbeddedFontHasNoMathematicalAngleBrackets` in backend/gg
pin the gap, so a font change that closed it would be noticed rather than
leaving the fallback guarding nothing. See
[ADR 0038](../docs/adr/0038-embedded-fonts.md)'s amendment.

**An embedded font's glyphs are numbered on first use, and that is what makes
writing a PDF one pass.** A content stream names glyphs by id and a subset's
ids depend on what the whole document drew, so a writer that waited would have
to buffer every text run in the file. Numbering on first use inverts it, and
`sfnt.Font.Subset` honours the order it is given — which is also the only order
that is a pure function of the drawing, per ADR 0012. Two things there are
load-bearing. A composite glyph's component ids are references into the same
font and are **rewritten** to the new numbering; copying one unrewritten draws
a plausible wrong letter, which is the worst kind of wrong. And the subset tag
comes from the face's index rather than from a hash of the glyphs, because a
hash would make the file depend on which labels the chart happened to draw and
the golden tests compare bytes.

**A density-raster figure embeds a PNG inside its SVG, and that payload is
compared as pixels.** `docs/images/density.svg` carries a base64 deflate stream,
and deflate output is the standard library's business rather than figure's —
the same chart on two Go releases produces two different streams, which is
exactly how this first went red. So the gallery lifts embedded payloads out of
both documents and compares them with the same per-channel tolerance the PNG
half already uses; the vector half, including the `<image>` element's own
position and size, is still compared exactly. See
`backend/stilus/cmd/gallery/embedded.go`, and note this is not a widened tolerance
but the same one applied to pixels rather than to an encoding of them. It is
also why there is no golden SVG for a density chart in `testdata/golden`:
pinning `compress/flate` is not a test of figure.

**A rectangular clip must reach a raster backend as a rectangle.** gg
rasterises a path clip into a coverage mask and then consults that mask on
every drawing call inside the clip, so a figure costs its drawing calls times
its area; a rectangle it can express as a scissor and the per-pixel cost goes
away. Every clip this library pushes is a rectangle — a panel, a facet cell,
one view of a scene — so `backend/gg` recognises one with
[`ir.Path.AsRect`](../ir/path.go) before falling back to the mask. The shape of
this bug is worth remembering: it is invisible at a hundred drawing calls and
it turned the gallery's ten-minute test budget into a failure at three
thousand, and the stack it fails in belongs to the rasteriser, not to the code
that caused it.

**The canvas backend builds one path string per drawing call.** Crossing the
WebAssembly boundary is what costs in a browser: a five-hundred-point line is
one `Path2D` and one `stroke`, not five hundred `lineTo` calls. There is a test
that fails if a chart over five hundred rows starts making more than a hundred
context calls, and one that fails if `beginPath` reappears.

**`backend/canvas` is empty except on js/wasm, and that is what the doc.go is
for.** Every implementation file carries `//go:build js && wasm`; `doc.go`
carries no constraint, so `go build ./...` on a server has a package to build
rather than an error. Its tests run under node in CI, against a recording
context — node has no canvas, and a test that needed one would be a test nobody
could run.

**`backend/gg` still must not import `gg/gpu`, and the GPU tier is a module
below it.** `backend/gg/gpu` is nested *inside* `backend/gg` so that the raster
backend's own module graph stays gg, `x/image` and the core; importing the tier
is the whole opt-in — [ADR 0022](../docs/adr/0022-gpu-tier.md). Moving the blank
import up one directory would put `wgpu`, `naga` and `goffi` into every program
that renders a PNG, and would cost the module its js/wasm target.

**gogpu is pinned exactly, and its pin is tied to gg's.** `backend/window`
requires gogpu v0.52.1 because that is the release whose `gputypes` and
`gpucontext` resolve to the versions gg v0.52.5 compiles against. A newer gogpu
pulls a `gputypes` that gg at this pin does not build with, and the failure is a
wall of "too many arguments" inside gg's internals rather than anything naming a
version. Upgrade the two together or not at all.

**The raster backend draws italic now, and `WithFont` takes three fonts.**
`ir.FontRef.Italic` was ignored by `backend/gg` once, which
was invisible until a typesetter started producing italic runs — at which point
the PNG and the SVG of the same chart would have disagreed in the
documentation. The default set parses `goitalic` alongside `goregular` and
`gobold`; a caller supplying their own passes nil for a style they do not have,
and gets the regular face for it.

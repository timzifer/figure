# Working in this repository

Notes for anyone — human or agent — making changes here. Read
[CONTRIBUTING.md](CONTRIBUTING.md) for the commands; this file is about the
constraints that are easy to break without noticing.

## The one rule

**The core module must never gain a dependency.** `go.mod` at the repository
root has no `require` block, and it stays that way. Anything that needs
`gogpu/gg`, `gogpu/gogpu`, `x/image`, Arrow, or anything else belongs in a
nested module — `backend/stilus`, `backend/gg`, `backend/gg/gpu`, `backend/window`
and `arrow/v18` are the five that exist. `syscall/js` is the standard library, which is why the
browser backend is in the core rather than beside them.

CI checks it:

```sh
go list -deps ./... | grep -v '^github.com/timzifer/figure' | grep '\.'
```

If that prints anything, the build fails. This is the promise the whole
positioning rests on — see [ADR 0001](docs/adr/0001-module-layout.md).

## Layer discipline

```
data, stat                                    →  rows in, rows out
geom, scale, coord, facet, layout, render     →  produce IR
                                                 (layout is internal/layout:
                                                 render and three call it)
three                                         →  produces IR, beside render
                                                 rather than through it
ir                                            →  the interface
interact                                      →  reads IR back
spec, a11y                                    →  write the model down
mathtext                                      →  places text, measures through IR
backend/svg, backend/pdf, backend/canvas,     →  consume IR
backend/gg, backend/window
```

- A geom must not import a backend, know about SVG, or measure text except
  through `ir.Backend.Measure`.
- `stat` knows about numbers and nothing else — no scales, no theme, no geoms.
  A reduction that needed one of those would be a geom in the wrong package.
- A backend must not import `geom`, `scale`, `theme` or `render`. That is why
  `Live.Bind` — which turns a wheel event into a zoom — is in the root package
  under a js build tag and not in `backend/canvas`: wiring input is not drawing.
- `render` is the only package that knows the drawing order of a *flat* chart,
  and `figure/three` is the only one that knows the drawing order of a
  projected scene. They are two orders because a layer is a paint unit in one
  and is not in the other; they are not two paths through one thing, and
  neither calls the other. See
  [ADR 0056](docs/adr/0056-three-dimensional-charts.md). Within `render`: a coord
  reports where a grid line, an axis line and a tick label go; `render` strokes
  them, in the order it always did. A coord that drew its own rings would be a
  second drawing order — see [ADR 0018](docs/adr/0018-coordinate-systems.md).
- `coord` knows about scales, points and paths, and nothing else. It must not
  import `theme`: the lengths it needs to place a tick mark travel as
  `coord.Metrics`.
- `spec` may read every model package and must never be read by one. A geom
  that knew about JSON would be a geom in the wrong package; `geom.Desc`,
  `scale.Desc` and `facet.Desc` are how a model type says what it is without
  knowing what will be done with the answer.
- `a11y` sits exactly where `spec` does, for the same reason: a geom that knew
  what a screen reader was would be a geom in the wrong package.
- `mathtext` neither draws nor knows about a chart. It is given a label, a font
  and a `Measure`, and it answers with positions — which is why it can be
  installed by wrapping the backend and reach every label at once.
- `interact` consumes IR and scales and produces neither. It wraps a backend
  rather than replacing one.

If a change needs to cross one of these lines, that is a signal the seam is in
the wrong place — say so rather than routing around it.

## Things that will bite

**Golden files are compared structurally, not byte for byte.** The SVG emitter
fixes attribute order and number formatting on purpose, so two runs on one
machine are identical — but arm64 and amd64 disagree in the last bit of a
float32, because Go contracts `a*b + c` into an FMA on one and not the other. So
`internal/svgdiff` compares everything but the numbers exactly and allows
coordinates a hundredth of a pixel. If output changes beyond that, the golden
tests fail; that is correct. Regenerate with `-update`, read the diff, and only
then commit. **Never widen a tolerance to make a failure go away.**

**Documentation figures are generated.** Everything in `docs/images/` comes from
`backend/stilus/cmd/gallery`. Never hand-edit them. A test and a CI job both check
they match the code.

**Scales place a value with one explicit rounding, and it is not redundant.**
`scale.place` writes `rlo + float32(float32(t)*(rhi-rlo))`. The inner
conversion looks like a no-op — `t` is already being converted — and it is not:
it is what stops Go contracting the multiply and the add into a fused
multiply-add. The spec permits that contraction "possibly across statements",
arm64 takes it and amd64 does not, and the result is a coordinate one float32
ulp apart on the two.

An ulp is invisible in a chart, which is why the golden files tolerate one. It
is not invisible to a *decision*: `stat.LTTB` picks the row forming the largest
triangle, so two candidates a hair apart swap places and a whole vertex moves.
That is how this was found — a documentation figure that differed between
architectures by a chosen sample rather than by a bit. The same rounding is
therefore forced in LTTB's own area computation. Do not "simplify" either one
away; nothing in the toolchain will tell you that you have.

**There is one layout solver, and every chart goes through it.**
`layout.Compute` is `layout.Panels` over a one-by-one grid, and `render.Draw`
resolves a single-panel chart into a one-panel grid. That is deliberate — see
[ADR 0010](docs/adr/0010-panel-layout.md). Adding a second path for "the simple
case" reintroduces exactly the divergence this arrangement exists to prevent,
and the golden files would only cover one of them.

**Colour ramps interpolate in linear light.** `palette.Lerp` decodes sRGB,
blends, and re-encodes. Averaging the encoded bytes instead is about 20% too
dark at the midpoint, which shows up as a band across a gradient. gg composites
in linear space for the same reason. There is a test that pins the midpoint of
black-to-white at 188, not 128.

**Everything sized by the data comes from a pool.** `geom.scratch` holds the
per-Build buffers and is returned at the end of the call, which is what makes a
redrawn chart allocate the same handful of times over a million rows as over a
thousand. Two consequences: a slice handed to a backend is **lent**, not given —
a backend that keeps one will show the next frame's data, which is why
`ir.Recorder` and `internal/irtest.Recorder` both copy — and a new allocation on
the data path will fail `TestARenderDoesNotAllocatePerPoint` rather than merely
slow things down. Find it with `-memprofile` and
`pprof -sample_index=alloc_objects`, do not widen the gate. The gate is behind
`//go:build !race` because the race detector allocates on figure's behalf, so
run it without `-race` when you are checking it — and it is checked a second
way, from real benchmark output, by `.github/scripts/allocgate.awk` in the
`Benchmarks and the allocation gate` CI job.

**The benchmark gate pins allocations, never times.** A shared CI runner cannot
measure a nanosecond usefully, and a gate that flakes is a gate people learn to
ignore. If you add a timing assertion there it will go red on an unlucky
Tuesday and be deleted, taking the allocation checks with it.

**A variadic interface method called per row allocates per row.** `Train(v)` on
a `scale.Scale` cannot be proved non-escaping by the compiler, so the argument
slice goes to the heap — a million times, on a million-row column. `Train` is
documented to ignore NaN and infinities, so the whole column goes in one call.
The same shape will appear again; look for it.

**A parallel render must be byte-identical to a serial one.** Panels record into
an `ir.Recorder` and replay **in panel order**, never in completion order —
[ADR 0012](docs/adr/0012-parallel-panels.md). That is what lets one set of
golden files cover both paths, and there is a test asserting the two produce
identical bytes. A change that makes the order depend on scheduling silently
halves the coverage of every golden file in the repository.

**Panels on separate goroutines need `scale.Snapshotter`, not `scale.Cloner`.**
Snapshot copies a scale *including* its trained domain; Clone deliberately does
not, because it is for a free facet axis. They are opposites and neither is a
substitute for the other. A scale implementing neither still works — the chart
is drawn serially.

**Do not compare device coordinates with `==` in a test.** `scale.place` now
removes the fusion that a scale itself introduced, but everything downstream of
it — a Catmull-Rom control point, a bar's half-width, a boxplot quantile — is
still ordinary float arithmetic the compiler may contract. `geom`'s annotation
tests carry `sameRect`/`samePoint` at `svgdiff.DefaultTolerance` for this, and
`TestTheCoordinateSlackIsTheRightWidth` pins that slack from both sides. Exact
comparisons were green on amd64 for three milestones and red on every macOS
run.

**A description costs a pass over the data, so a render must not take one.**
`Plot.Describe` is a method that does work rather than an option that sets a
flag: it reads every plotted column. The title alone is free and is always
written; the long description is written when someone asked for one. Wiring the
description into `chart()` so that every frame recomputes it would put a data
pass in the frame budget, and `TestARenderDoesNotAllocatePerPoint` would not
notice — it counts allocations, not passes.

**A gated benchmark measures on one processor, and that is what makes its count
reproducible.** `sync.Pool` keeps a private slot per P. A render Gets its
scratch and Puts it back on one goroutine — but a frame takes milliseconds, the
scheduler preempts asynchronously every ten of them, and a goroutine that
resumes on a different P finds its own scratch stranded in the old P's private
slot, which nothing can steal from. That frame then refills every buffer it
needs: about sixty allocations that have nothing to do with the data, landing
in one iteration out of a few dozen. Over ten iterations that is the difference
between 54 and 68 allocs/op for the same code, and it is why three gates in
`allocgate.awk` used to be budgets with an apology attached.

`onOnePGate` in `alloc_test.go` pins the measurement, and the counts are now
bit-identical across runs — `flat()` compares every pair again. Add it to any
new benchmark whose number the gate reads; leave it off the parallel ones,
which measure panels on several goroutines and are gated on nothing.
`testing.AllocsPerRun` pins the same way, which is why the test half of the
gate was steady all along while the benchmark half was not. A pool miss is a
real cost that a real chart pays occasionally; it is simply not the cost this
gate measures, which is whether a frame allocates *per row*.

**Responsive scaling multiplies lengths and must not mutate a shared theme.**
`theme.Scaled` copies every dash slice it touches rather than scaling in place —
`theme.Light` is a package variable, and scaling its grid dash would scale it
for every chart in the process, once per render. There is a reason that is a
sentence in the code as well as here.

**A transition's row set is the union of its two states, and it is fixed.** An
entering row exists at `f == 0` and an exiting one at `f == 1`. That is not
tidiness: it keeps the frame's *structure* identical between frames, which is
the condition `ir.Damage` needs to report the two comparable. A blend whose row
count changed partway through would make every frame a full repaint while the
picture stayed perfectly correct — so the test asserts on what the backend was
told to repaint (`irtest.Recorder.Whole`) rather than on anything visible. The
same trap is why `Transition.Rescale` pins the domains: a `Nice` axis
re-rounds, a changed tick *count* is a structural change, and that is the full
repaint again.

**Only numbers and times tween.** A string is a name rather than a quantity —
half of "ingest" is not a node — so a string column takes the end state's value
and anything a geom decides from one decides it abruptly: a categorical slot, a
discrete colour class, a `GroupBy` membership. `data.Hold` is how a *number*
that is really a name gets the same treatment. And a `data.Stream` cannot be
tweened at all: under a `Window` ring the row numbers slide, which is the whole
reason a key exists.

**A counting label needs `data.Round`, and that is not a formatting bug.**
`geom.Text` re-spells its column through `data.Labels` in `Train`, every frame,
so a *numeric* `geom.TextBy` column animates by itself — which is the useful
half of "animated text" and works with nothing added. But `data.FormatNumber`
spells a float at full precision, deliberately, because a facet panel key, a
categorical tick and a text label all go through it and must agree; so an
interpolated value reads `33.300000000000004`. Round the **value**, not the
spelling. A format option on `geom.Text` would put a second spelling of a
number into the model, which is what ADR 0035 exists to prevent. A *string*
label snaps and cannot be faded — a cross-fade needs per-row opacity, the IR
change ADR 0007 refuses — but its position still moves, which is usually the
shape wanted anyway.

## Package notes

Rules that belong to one part of the tree live next to it. An agent working in
a directory reads that directory's file as well as this one; a person changing
a package should too. A rule that turns out to bind more than one package
belongs here instead.

- [data/AGENTS.md](data/AGENTS.md) — nulls, `data.Stream` and the ring buffer.
- [stat/AGENTS.md](stat/AGENTS.md) — stress layout, evaluation grids, no
  sorting, histograms and the hexagonal lattice.
- [scale/AGENTS.md](scale/AGENTS.md) — domains, axis breaks, endpoint snapping,
  pinned domains, tick-label formats, locales and time origins.
- [coord/AGENTS.md](coord/AGENTS.md) — Smith charts, map projections, parallel
  coordinates, furniture, Cartesian as the identity, break-outs — and what the
  coords deliberately do not do.
- [geom/AGENTS.md](geom/AGENTS.md) — device-space exceptions, batching,
  decimation, row reporting, stacks and groups, the scratch pool, distribution
  marks, text layers, relational layouts — and what the marks deliberately do
  not do.
- [render/AGENTS.md](render/AGENTS.md) — shared scale objects across panels,
  tracks, the layout's row arithmetic, second axes, guides, watched and resized
  renders.
- [three/AGENTS.md](three/AGENTS.md) — the painter's order, the depth key,
  occlusion tests, batching faces, the fitted cube, scene scratch and projected
  axis labels.
- [ir/AGENTS.md](ir/AGENTS.md) — damage, comparing styles, and wrappers that
  hide optional interfaces.
- [interact/AGENTS.md](interact/AGENTS.md) — watched renders, hit-testing per
  subpath and against outlines, and keys read from the plot's layer.
- [spec/AGENTS.md](spec/AGENTS.md) — what a document writes for a mark,
  complete descriptions, and marks the encoding tells apart.
- [backend/AGENTS.md](backend/AGENTS.md) — stilus and the deprecated gg, pins,
  PDF fonts, embedded payloads in figures, rectangular clips, canvas and the
  window.

## Open questions

[CONCEPT.md §17](CONCEPT.md#17-the-decisions-this-design-rests-on) lists the
rules the design rests on. Every one has a record in [docs/adr](docs/adr) —
§17.3, the JSON dialect, is [ADR 0014](docs/adr/0014-json-spec.md); §17.7, the
third-party extension API, is
[ADR 0029](docs/adr/0029-extension-model.md). A question that opens from here
is answered the same way: with a record in `docs/adr`, never in passing.

Optional interfaces are how this codebase extends a type without breaking
everyone who implements it: `scale.Definite`, `scale.Categorical`,
`scale.Band`, `scale.Cloner`, `scale.Snapshotter`, `scale.Zoomer`,
`scale.Describer`, `scale.ColorDescriber`, `scale.DiscreteColorScale`,
`scale.SizeDescriber`, `scale.Temporal`, `scale.Breaker`, `geom.Faceter`, `geom.Guided`,
`geom.Sized`, `geom.Legender`, `geom.Describer`, `coord.Describer`,
`coord.Exploder`, `coord.Breakable`, `ir.Partial`, `ir.Semantics`, `ir.Resizer`,
`mathtext.Plainer`. Reach for one before adding a method to `Scale`, `Geom` or
`Backend`. `geom.Legender` is the newest and the argument is worth keeping in
view: a pie, a stack and a waffle contribute N legend entries from one layer,
and adding a second method to `Geom` for them would break every implementation
for a shape only some layers have.

`Cloner`, `Snapshotter` and `Zoomer` are three different things and none
substitutes for another: Clone hands back an *untrained* copy for a free facet
axis, Snapshot an *exact* copy for another goroutine, and SetDomain changes the
scale in place for a pan or a zoom.

## Scope

[CONCEPT.md §14](CONCEPT.md#14-what-is-built-and-what-is-next) says what exists
and what is next, in the order it is being done. Adding a stub for something on
that list is not progress towards it: the seams exist, that is enough. The
release order is in [CONTRIBUTING](CONTRIBUTING.md#releasing).

The deliberately-not-done lists for the marks and the coords are in
[geom/AGENTS.md](geom/AGENTS.md#deliberately-not-done) and
[coord/AGENTS.md](coord/AGENTS.md#deliberately-not-done); what follows is the
part that is not one package's.

Deliberately not done. The GPU tier is opt-in beta and stays that
way off — for server-side stills the CPU rasterizer and the vector
emitters are the supported path. The window is compiled by CI and never opened
by it, because a runner has no display; what is tested is everything that is not
the window, which is the same hole `backend/canvas` has about a browser. The
built-in typesetter is a deliberately small subset: no matrices, no growing
delimiters, no document-class macros, and a label needing those wants an engine
plugged into `mathtext.Typesetter`. Accessibility stops at the document: no
per-mark `<title>`, no tab order through the marks, no reduced-motion or
contrast handling — a chart with ten thousand points has no useful reading as
ten thousand elements, and the data table is the better answer to the same
question. And a description is a snapshot: data that changes afterwards leaves
it stale, which is why `Describe` is a call rather than a flag.

Deliberately not done. There was no native window and no GPU tier;
both needed GoGPU packages the raster backend does not touch.
A hit
reports data values rather than a row number, because carrying row identity
through decimation is bookkeeping the design avoids. Damage is per drawing
call, so moving one point of a line repaints the line's box. And the browser
path is canvas 2D, not WebGPU — gg has no `syscall/js` in it at the pinned
version, so there was no WebGPU path to take
([ADR 0017](docs/adr/0017-browser-backend.md)).

Deliberately not done, in case they look like oversights.
`stat` carries the decimation family and nothing else: smoothing, regression and
hexbin are stats rather than big-data machinery, and they belong with the geoms
that would draw them. There is no `StreamSource` and no snapshot/swap, because
the interesting half of streaming is damage-aware repaint and that needs the
interactive backends. Training the scales and splitting a facet's rows are still
serial — only the data pass is parallel — and making them parallel is a
different decision with a different shape. And the Arrow adapter does not handle
`float16`, decimals or extension types; nothing that plots produces them yet,
and an untested conversion is worse than an absent one.

One thing deliberately not done: a colourbar is vertical, in
the guide column; a horizontal one under the plot is a layout question, not a
drawing one. The other — "a PDF is one page with no embedded font: text outside
WinAnsi becomes `?`" — is half closed. It is still one page, and the *default*
is still Helvetica and WinAnsi, byte for byte; `pdf.WithFont` is the way past
it ([ADR 0038](docs/adr/0038-embedded-fonts.md)), and there is no per-glyph
fallback: a document draws every label in the face it was given, because a
fallback chain is a font-matching policy and a plotting library is the wrong
place to hold one.

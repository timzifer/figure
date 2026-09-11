# Architecture decision records

Short records of decisions that were genuinely open, why they went the way they
did, and what would make them worth revisiting.

They exist because `CONCEPT.md` §17 lists open decisions, and a design document
that never records how its own questions were answered stops being trustworthy.
Each record below closes one of those items or pins something the code now
depends on.

| # | Decision | Status | Closes |
|---|---|---|---|
| [0001](0001-module-layout.md) | The gg backend is a nested module in this repository | Accepted | §17.5 |
| [0002](0002-ir-and-backend.md) | The v0.1 IR and `Backend` interface | Accepted | — |
| [0003](0003-text-and-fonts.md) | Text measurement and the font strategy | Accepted | §17.4 |
| [0004](0004-svg-source-of-truth.md) | The built-in emitter is the only SVG path in v0.1 | Accepted | §17.2 |
| [0005](0005-go-version.md) | Go 1.25 is the minimum | Accepted | §17.6 |
| [0006](0006-gg-coupling-surface.md) | How much of gg the adapter is allowed to touch | Accepted | §17.1 |
| [0007](0007-per-mark-colour.md) | Colour varies per mark without changing the IR | Accepted | — |
| [0008](0008-categorical-axes.md) | Categorical axes ride the numeric `Scale` interface | Accepted | — |
| [0009](0009-pdf-backend.md) | PDF is a built-in emitter, not a gg recording | Accepted | — |
| [0010](0010-panel-layout.md) | One constraint solver for every chart shape | Accepted | — |
| [0011](0011-decimation.md) | Decimation at draw time, in device space, by default | Accepted | — |
| [0012](0012-parallel-panels.md) | Panels build in parallel by recording, replayed in order | Accepted | — |
| [0013](0013-arrow-adapter.md) | The Arrow adapter is its own module, and borrows only where it can | Accepted | — |
| [0014](0014-json-spec.md) | The JSON spec is Vega-Lite-shaped, not a Vega-Lite subset | Accepted | §17.3 |
| [0015](0015-hit-testing.md) | Hit-testing indexes what a render emitted, told apart by an observer | Accepted | — |
| [0016](0016-streaming-and-damage.md) | Streaming is a snapshot and a swap; damage is a diff of two recordings | Accepted | — |
| [0017](0017-browser-backend.md) | The browser backend is canvas 2D, in the core, and not gg | Accepted | — |
| [0018](0018-coordinate-systems.md) | Coordinates are a stage between the scales and the IR | Accepted | — |
| [0019](0019-position-adjustments.md) | Stacking is a position adjustment within a layer, derived in `Train` | Accepted | — |
| [0020](0020-discrete-colour-and-multi-entry-legends.md) | Discrete colour is a scale; a layer may contribute many legend entries | Accepted | — |
| [0021](0021-native-window.md) | The native window rasterizes on the CPU and presents one texture | Accepted | — |
| [0022](0022-gpu-tier.md) | The GPU tier is opted into by importing a module | Accepted | — |
| [0023](0023-math-typesetting.md) | Notation is typeset by a pluggable typesetter, installed by wrapping the backend | Accepted | — |
| [0024](0024-accessibility.md) | A chart says what it is in three channels: a name, a description, and its data | Accepted | — |
| [0025](0025-responsive-charts.md) | A chart follows its surface by scaling its theme | Accepted | — |
| [0026](0026-breaking-a-mark-out.md) | A mark is broken out by displacing it, and the coord answers how far | Accepted | — |
| [0027](0027-size-channel-and-the-guide-column.md) | A size channel maps by area, and the guide column is generalised once | Accepted | — |
| [0028](0028-distribution-stats.md) | A distribution stat runs in `Train`, and decides one of its own axes | Accepted | — |
| [0029](0029-extension-model.md) | A third party's geom, scale or coord is a first-class citizen of the spec | Accepted | §17.7 |
| [0030](0030-arrow-major-version.md) | The Arrow adapter's major version is its upstream's, and its import path says so | Accepted | — |
| [0031](0031-tracks.md) | A track is a panel with a fixed extent, on a scale it shares | Accepted | — |
| [0032](0032-text-as-a-mark.md) | Text is a mark that reads a column, and it labels the box a row spans | Accepted | — |
| [0033](0033-smith-charts.md) | A Smith chart is the polar-shaped coord seam, over normalised impedance | Accepted | — |
| [0034](0034-null-values.md) | A null is a missing value, and a column says so beside its values | Accepted | — |
| [0035](0035-label-format-and-locale.md) | A tick label is described rather than computed, and the description carries a language | Accepted | — |
| [0036](0036-error-bars.md) | An interval is a mark, and the encoding says which way it runs | Accepted | — |
| [0037](0037-secondary-axis.md) | A second axis is a scale on the chart and a binding on the layer | Accepted, amended | — |
| [0038](0038-embedded-fonts.md) | A PDF carries the font its labels need, subset to the glyphs it drew | Accepted | — |
| [0039](0039-relational-layouts.md) | A relational layout is a stat in the unit square, and the coord decides what it looks like | Accepted | — |
| [0040](0040-label-collision-avoidance.md) | Participating text layers share a deterministic panel-local label layout | Accepted | — |
| [0041](0041-qq-plots.md) | QQ plots rank the sample before it is drawn | Accepted | — |
| [0042](0042-colour-transforms-and-classes.md) | A colour ramp may compress its domain or cut it into classes | Accepted | — |
| [0043](0043-mark-identity.md) | Mark identity is a column the caller names, resolved outside the geom | Accepted | — |
| [0044](0044-transitions.md) | A transition is a keyed join blended in data space, driven by the host's clock | Accepted | — |
| [0045](0045-linked-views.md) | Linked views are the host's; figure supplies the two ends of the wire | Accepted | — |
| [0046](0046-overlay-layer.md) | The chart owns an overlay, drawn last and announced to nobody | Accepted | — |
| [0047](0047-clickable-legend.md) | A legend answers to a pointer; hiding is the chart's, toggling the caller's | Accepted | — |
| [0048](0048-clickable-colourbar-and-size-key.md) | A colourbar and a size key report a quantity, because neither is a series | Accepted | — |
| [0049](0049-paths-colour-in-classes.md) | A path colours in classes, and the scale decides where the colour changes | Accepted | — |
| [0050](0050-locus-annotations.md) | A locus is an annotation, and the coord draws it | Accepted, amended | — |
| [0051](0051-barycentric-coord.md) | A ternary chart is a barycentric coord, and its third grid family is not furniture | Proposed | — |
| [0052](0052-probability-scales.md) | A probability scale warps the axis, where a QQ plot warps the sample | Proposed | — |
| [0053](0053-tidy-tree-layout.md) | A tidy tree is a bounded deterministic layout, and it is not a force simulation | Proposed | — |
| [0054](0054-statistical-instruments.md) | A domain reduction belongs in `stat` when its output is the chart | Proposed | — |
| [0055](0055-depth-without-a-third-axis.md) | A mark gains volume before a chart gains a dimension | Planned | — |
| [0056](0056-three-dimensional-charts.md) | A third axis widens the seams that count scales, and the IR stays two-dimensional | Accepted, amended | — |
| [0057](0057-orbiting-a-chart.md) | The camera is a value, and turning it is the host's loop | Accepted, amended | — |
| [0058](0058-what-3d-is-for.md) | What the third dimension is for, and where it stops paying | Planned | — |
| [0059](0059-renaming-and-restarting-the-version.md) | The library is renamed, and the version restarts rather than doubling | Accepted | — |
| [0060](0060-parameter-structs-at-every-seam.md) | Every seam an outsider implements takes a struct, and the count of optional interfaces stops growing | Accepted | — |
| [0061](0061-columns-are-one-value.md) | A column is one value that grows, and identity is its spelling | Accepted | — |
| [0062](0062-a-scene-and-its-views.md) | A scene is the data, a camera is a way of looking at it, and one figure may hold several | Accepted | — |
| [0063](0063-an-overlay-over-a-scene.md) | A scene is painted over too, and the seam is `three`'s own | Accepted | — |
| [0064](0064-a-contour-and-its-lattice.md) | A contour is one tracing, and the lattice under it is one resolver | Accepted | — |
| [0065](0065-horizon-charts.md) | A horizon chart folds its own axis, and the colourbar is the ladder it gives up | Proposed | — |
| [0066](0066-a-raster-mark.md) | A field sampled on a grid is one image, and the lattice already knows its shape | Proposed | — |
| [0067](0067-a-bivariate-colour-channel.md) | A colour channel may carry two readings, and the second one takes resolution away | Proposed | — |

Nothing in §17 is open any more. **§17.7**, the third-party geom and backend
extension API, was the last, and it was held open on purpose until the
milestone that had to answer it: freezing it well is what makes the "last
plotting library" claim survivable. 0018 and 0020 were both shaped by that
deadline — the first widens `geom.Frame` because it had to be widened before
the freeze, the second declines to widen `Geom` because an optional interface
does the same work without spending it — and 0029 is the answer. A decision
that opens after v1.0 gets a record here before it gets code.

## Records not yet implemented

**0051 to 0054 are proposed, not accepted, and no code implements them.** They
are written down because the alternative is worse: each one answers a question
an earlier record left open — 0033's "Revisit if" for the first two, 0039's for
the fourth, 0041's serialisation rule for the third — and a question answered in
a conversation and not in the repository gets answered again, differently, later.

**0050 is the one of the five that has been built**, and it is *Accepted,
amended*: `geom.Locus` draws a family of curves given by a formula, `stat` names
four of them, and a Nichols diagram and a Smith chart's VSWR circles and
constant-Q arcs are the charts that came with it. Its amendment says where the
implementation sharpened it — chiefly that `Family` is `stat`'s type that `geom`
names, and that a Nichols curve is refined against the panel rather than walked
uniformly round its circle, because the chart is the log-polar view of that
circle and a uniform walk resolves the plunge to −∞ dB or the rest of the curve
and never both.

They are also deliberately written as a set, because four of the five lean on
each other. 0050 introduced the mark 0051 keeps as its escape hatch for a
ternary chart's third grid family and 0053 uses for a funnel plot's contours,
and it is now there to be leant on;
0051 and 0053 both cite 0041's rule that a named member of a closed family
serialises and an arbitrary Go function does not; 0053 narrows a category 0039
refused rather than reopening it. Reading any one of them alone will make it
look more expensive than it is.

A proposed record becomes accepted when it is implemented, or is deleted with a
sentence saying what it got wrong — 0050 is the worked example of the first.
Neither is urgent for the remaining four: nothing in v1.7 depends on any of
them, and each is additive by construction.

**0055 to 0058 are the same rule applied to 3D**, under the status *Planned*
rather than *Proposed*: the difference is that these four are meant to be built
rather than argued about first, and two of them now have been. **0056 and 0057
are Accepted**: `figure/three` draws a surface, a trajectory and a field of
bars, and turns them at a camera the host holds. Each carries an amendment
saying where the implementation sharpened the record — 0056 on what its single
depth order actually is, 0057 on the three things it left open — because a
record that no longer matches the code is worse than no record. **0055 and 0058
are still Planned**: the oblique coord is unbuilt, and 0058 is a catalogue
rather than a mechanism, so it becomes Accepted when the last form in it does.
0058's order of work is annotated with what landed.

They exist because 3D had been deferred as one
indivisible thing — `CONCEPT.md` §5 and §14 and
[the v1 audit](../v1-api-audit.md) each defer it in a single line, none of them
saying what "it" is — and that deferral stayed cheap only for as long as nobody
priced the parts. Split, the three have different costs, different blast radii
and different answers: volume is a coord and two geoms, a third axis is a
major version spent on three seams that leaves the IR alone, and turning the
scene is a pure function over a camera. Each earns its keep alone, and each is
the honest prerequisite for the next. 0058 is the fourth and says what the
other three are for: the charts 3D enables, ranked by what the third dimension
gives a reader that the flat chart of the same data does not — the only
ranking under which a 3D bar chart loses to a heatmap and a Smith sphere loses
to nothing. Implementation is a separate step; a Planned record becomes
Accepted when the code that proves it lands.

**0062 is the record 3D turned out to need and none of the four contained.**
0057 made the camera a value so that the host could own the drag; the
consequence nobody had written down is that a value can exist more than once,
so one scene can be looked at from several cameras at once — which is what a
static export, a printed page and an engineering drawing's plan-and-elevations
all need, none of which can be turned. It is a record of its own rather than a
paragraph in 0057 because [ADR 0059](0059-renaming-and-restarting-the-version.md)
asks for one: a change that is not on 0056's list needs a record here before it
needs code.

**0063 is what 0062 left unbuildable.** 0062 closed by saying that highlighting
the row a pointer landed on in every view is the host's, which is right — and a
host had nowhere to draw it, because ADR 0046's overlay stopped at the flat
chart. The record adds the paint and not the state: `three.OverlayView.At` hands
back the pixel the scene drew a value at, in every view, and which rows are
marked is still nobody's business but the program's.

**0064 is the row bucket F carried unbuilt.** ADR 0058 scheduled the contour
between the 3D scatter and the sphere with the argument that one function pays
for two charts, and that argument is the record. What needed writing down is
what *one* function means when two packages draw from it: one tracing, one
lattice resolver, and a saddle rule that is a function of four corner values, so
that a reading taken off the plan and one taken off the surface's floor cannot
disagree.

**0065 to 0067 came out of one sweep rather than out of the code**, which
makes them the first records here written from the outside in: the catalogue at
[xeno.graphics](https://xeno.graphics) was read against `docs/chart-types.md`,
and three of the forms on it turned out to want machinery this library does not
have while the rest turned out to be recipes, options or rows in a record that
already exists. That accounting is in
[chart-types.md](../chart-types.md#the-sweep-of-the-unusual-forms), so that the
ones declined stay declined for a reason rather than being rediscovered.

They are independent of each other and of 0050 to 0054. **0066 is the one with
a dependent**: a raster is what makes a Hovmöller diagram, a spectrogram and a
recurrence plot ordinary rather than impossible, and it is also the backdrop
0064's contours are usually drawn over — so a plan that builds one of them
builds it first. **0065 is the one that fits the examples already in the
repository**, which are a machine, a status board and a stream. **0067 is the
one that is a seam rather than a shape**, and it is written as a set of three
customers for that reason: a palette that suppresses a value's resolution, a
bin that says which class it holds, and the bivariate square, none of which
would justify the interface alone.

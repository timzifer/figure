# Working in `three`

Notes for changing `three`, moved here from the root [AGENTS.md](../AGENTS.md),
which holds the rules that apply everywhere — read that one first. This file
covers the painter's order, the depth key, occlusion tests, batching faces, the
fitted cube, scene scratch and projected axis labels.

## Things that will bite

**`figure/three` paints in its own order, and `render` did not gain a second
one.** A `three.Layer` is deliberately not a `geom.Geom`: a geom's `Build`
streams ink, which makes a layer a paint unit, and a projected scene has no
paint unit smaller than the view. So a layer emits primitives into a `Sink` and
one painter projects, orders and draws all of them at once. If a change here
starts wanting `render` to record layers and merge the recordings, that is the
thing [ADR 0010](../docs/adr/0010-panel-layout.md) exists to prevent and
[ADR 0056](../docs/adr/0056-three-dimensional-charts.md) refuses again.

**The depth key is the centroid's depth along the view direction, one formula
for every primitive of every layer, ties to the emission index.** Two things
that look like improvements are not, and both were tried:

*Letting a layer pick its own measure* is two numbers on two scales — they
differ by the height times the view direction's z — so one layer sorts wholly
before the other however the geometry runs.

*Keying on the centroid dropped to the floor* is the subtler one, because its
argument is half true: that key does vary less over a steep primitive, and it
is monotone along every view ray. It still does not follow, and this is the
sentence to remember — **monotone along a ray says nothing about two
centroids, which lie on two different rays.** Two sheets at different heights
whose footprints overlap without coinciding come out backwards.
`TestASheetIsNotPaintedOverTheOneInFrontOfIt` is that case with the numbers in
it; run it before believing any replacement key.

**What the key promises is smaller than "correct", and the difference is the
part to know.** Two primitives are ordered correctly when their depth *ranges*
are disjoint. Where the ranges interleave, no per-primitive number decides
between them and this package does not split them apart — splitting is a BSP
tree, which is a renderer. Real charts stay inside the promise because the
shapes emitted here are already small: one quad per cell, one face per bar
side, one primitive per path segment. The case that leaves it is several layers
stacked over a grid coarse enough that one cell spans more depth than the
layers are apart.

**Letting a layer pick its own depth measure is the same bug one level up.**
Two layers keyed by two different formulas are two numbers on two scales, and
merging them is arithmetic rather than geometry: one layer sorts wholly before
the other however the geometry runs. `TestASurfaceAndAPathAreOrderedAgainstEachOther`
is what catches it, and it is invisible in any picture with one layer in it.

**The occlusion tests cast rays; they do not restate the sort.** A test whose
expected answer recomputes the ordering rule proves only that the sort sorts.
`checkOcclusion` intersects the view ray through each sample point with the
plane of every primitive covering it and insists the nearest is painted last —
so it fails for any key that gets the visible picture wrong, including the one
the implementation happens to use. Keep it that way when the key changes.

**The depth key is computed in float64 and stored as float32, on purpose.** The
narrowing turns a near-tie into an exact tie, and an exact tie is broken by
emission index, which is the same on every architecture. A float64 key would
let two primitives a ulp apart order one way on arm64 and the other on amd64 —
and `internal/svgdiff` tolerates exactly that much coordinate difference, so
the reorder would *not* be caught. It is `scale.place`'s and `stat.LTTB`'s
lesson in a third place.

**Faces batch by run, never by colour — and only where the merge is provably
invisible.** `geom.groupByColor` merges every mark of one colour into one call
because order within a flat layer does not matter. In a scene the order *is*
the thing being computed, so only primitives already adjacent in it are
candidates at all. Adjacency is not sufficient either: two adjacent faces can
still overlap on screen, and a run that draws all its fills and then all its
outlines puts a farther face's outline over a nearer face's fill, while a union
filled once is not what several translucent fills compose to. So `mergeable`
takes the run only for an opaque, unoutlined style, where the colour is the
same everywhere in the union either way. Widening that condition is how the
painter's order gets undone by the optimisation meant to be invisible.

**A surface fills and outlines only when asked.** `interact` ranks a stroked
vertex above the area it outlines, so a mesh that always drew its own outline
would report a corner on every hover. It is `geom.Rect`'s rule and it is
load-bearing rather than cosmetic.

**`three` announces its panels with nil X and Y.** A projected scene has no
screen axes to invert a device position through, and `interact.Index.At`
already checks before it inverts — so a hit reports `Kind`, `Layer`, `Series`
and `Row`, and leaves `Hit.X` and `Hit.Y` at zero. Filling them in with the
scene's scales would name a value nothing was drawn at.

**The cube is fitted to its own circumscribed sphere, not to its projected
box.** The sphere's radius is a constant of the scene and the box's is not, so
the scene keeps its size while it turns. Fitting the box is the tighter fit and
makes the picture breathe under the reader's hand, and an instrument whose
scale moves while it is being read is not one. `three.Dolly` is how the reader
reclaims the room this costs.

**A `three.Live` keeps its own scratch; a one-shot render borrows from the
pool.** A scene large enough to be worth turning is megabytes of vertex arena,
so drawing one allocates enough between frames to run a collection — and
`sync.Pool` is emptied by one. A chart redrawn on every pointer move would find
an empty pool and rebuild its arenas every frame. `BenchmarkOrbit32` and
`BenchmarkOrbit96` are what catch it going back.

**A scene's scales are trained once per frame however many views look at it.**
`scale.Scale.Train` accumulates, so training per view gives every layer as many
times its weight as there are cameras — and the symptom is a domain that
changes when an author adds a picture, which looks entirely reasonable and is
wrong. There is a test that counts the calls.

**A shaded surface is one drawing call per quad, and a trajectory one per
segment.** The IR carries no per-mark colour
([ADR 0007](../docs/adr/0007-per-mark-colour.md)), so faces that differ in shade
cannot be merged, and a path with one depth would sort as one thing. That is a
cost model rather than a bug, but it is still a count, and
`backend/stilus/cmd/gallery`'s tests render every figure about five times over in
each of two formats. `TestNoFigureIsDrawnWithTooManyCalls` is the wall, and it
counts calls rather than seconds because a timing assertion on a shared runner
is a gate people learn to ignore.

**A projected axis labels itself only if at least two of its labels survive.**
Turn a scene until an axis points at the reader and it projects to a few
pixels: every value on it lands in the same place. A pile of overlapping
numbers there looks wrong at a glance and a reader discounts it; the single
number a greedy collision pass leaves behind looks like a label and reads like
one, while naming a position that cannot be told from any other on that axis.
So `three`'s label pass runs twice, once to count and once to draw, and an
axis that keeps fewer than two gives its boxes back to the next axis. Its
line, its tick marks and its title stay: a mark is a position, a number is a
claim about how much room there is.

**An axis title is placed past its own tick labels, and both the room and the
placement come from `furnitureReach`.** They were computed separately once: the
title was put a guessed distance out, its box overlapped the labels', it won
the collision because a title is placed first, and the axis quietly lost most
of its numbers. A guess and a measurement of the same distance are two numbers;
that function exists so there is one.

# Working in `scale`

Notes for changing `scale`, moved here from the root [AGENTS.md](../AGENTS.md),
which holds the rules that apply everywhere — read that one first. This file
covers domains, axis breaks, endpoint snapping, pinned domains, tick-label
formats, locales and time origins.

## Things that will bite

**A domain is ordered, and an axis's direction is `scale.Reverse`.** `Domain`,
`LogDomain`, `SymLogDomain` and `Zoomer.SetDomain` all order the bounds they are
given. Writing them backwards to flip an axis half-works, which is why it is not
allowed to: `Map` and `Invert` mirror, and then `Zero` and `Nice` reorder the
pair, the containment test in `Ticks` drops every tick there is, and the first
pan turns the axis back round. What reverses is the device range — see
[ADR 0075](../docs/adr/0075-an-axis-has-a-direction.md). Anything that walks ticks
across a panel walks them in **screen** order for the same reason: a tick
sequence is ascending by value, and on a reversed axis that is right to left.

**An axis break is off until the renderer switches it on, and a value inside
one is not missing.** `scale.Break`, `scale.Fold` and their time forms keep
`Map` continuous: a value in a cut maps linearly across the gap and the panel's
clip hides it, which is what keeps `Invert` exact and a bar across the break one
bar. Do not "fix" that by answering NaN or `Defined == false` — a point there
would vanish silently and the tall bar the break exists for would be cut. The
gap is a device length the scale cannot know, so `render.breakGaps` sets it
through `scale.Breaker` once per render, and sets it **off** under any coord
that is not `coord.Breakable` and on every second axis: a break drawn without
its mark is an axis whose ruler is wrong where nobody says so. The cuts are
resolved on the stack of every call rather than cached, so a pan needs nothing
invalidated and a snapshot shares nothing it writes; per piece, `place` and the
endpoint snap apply exactly as they do to a whole axis. With no active cut,
`framedCartesian.Clip` is the panel rectangle and its furniture is two-point
polylines — which is why no golden file moved, and why that branch must stay.
See [ADR 0083](../docs/adr/0083-an-axis-break-is-marked-or-not-drawn.md).

**Scales snap their endpoints.** `Map` returns the exact range bounds for the
exact domain bounds. Without that, a tick on the plot edge lands a float32 ulp
outside it and gets culled. There is a test; do not "simplify" it away. Every
scale does this.

**A pinned scale domain skips nicing, and that is not an oversight.**
`scale.Zoomer.SetDomain` sets `pinned` as well as `fixed`, and `effective()`
returns the pinned domain before nicing or zero-forcing get a chance. An axis
that snapped to round numbers after every wheel notch would not follow the
pointer, and on a log axis nicing rounds the view out to whole decades. `fixed`
alone stops *training*; `pinned` also stops *framing*.

**A tick label has two spellings and the Go one wins, but both are written
down.** `scale.Format` takes a function and `scale.NumberFormat` takes a spec;
`Desc` carries `Formatted` *and* `Format`, because a Desc that dropped the spec
when a function was present would silently change the chart the day somebody
deleted the Go code — [ADR 0035](../docs/adr/0035-label-format-and-locale.md). Two
things there are easy to break. A spec that names no precision must keep the
axis's own, which is derived from the tick *step* and is what keeps a column of
labels aligned: that is why `autoFor` returns a description rather than a
formatter, and why the two meet in `numberFormat.label`. And an unlocalised
scale must take the *old* path exactly — `punctuate` with English's separators
is the identity on `strconv`'s output, and a time scale with no locale calls
`time.Format` itself rather than the layout-splitting path — because that is
what leaves every golden file in the repository unchanged. There is a test
asserting that naming English changes nothing; do not "simplify" the default
branch away.

**A locale reaches a chart's axes by a walk, and the walk is the feature.**
`figure.Locale` localises the scales in the *chart description*, after it is
built, which is what reaches a track's own scale and a free facet axis's clone
— neither of which the caller holds. Setting it in `Plot.X` instead compiles
and misses both. It happens before the parallel path, and a locale is read
from then on and never written, so there is nothing for `Snapshotter` to fix.

**A time domain is nanoseconds since the scale's origin, not since 1970.**
`scale.Origin` moves it, `scale.ValueOf`/`InstantOf` convert across it, and
`geom.column` uses them — so a time column is read in the axis's own space.
The default origin is the Unix epoch, which is exactly `scale.Nanos`, so nothing
changes for a scale that never asked. What does change is that `scale.Nanos` is
the wrong conversion to reach for near a rebased axis: the domain values, the
`Invert` results and the `Desc` bounds are all measured from the origin, and the
JSON spec writes it out for that reason.

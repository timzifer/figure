# 0071 — A curve is a family chosen for the drawing, and a smoother is a fit chosen for the data

Status: Accepted

## Context

A line had exactly one smoothing knob. `geom.Tension(t)` fitted a uniform
Catmull-Rom spline through the device positions, `geom.Area` and `geom.Trend`
read the same field, and zero meant a polyline. One family, one parameter.

That is not enough, for two separate reasons, and the whole of this decision is
keeping the two apart.

**The family was wrong for some data.** A cardinal spline overshoots. Through a
series that only ever rises — a cumulative total, a queue depth, a reading that
physically cannot go backwards — it dips below the previous reading between two
samples. That dip is ink at a value nobody measured, on the one axis a reader
trusts most. There has been a family that cannot do this since Fritsch and
Carlson in 1980, and figure did not have it.

**The vocabulary slot was already there and empty.** The JSON dialect speaks
Vega-Lite's `mark.interpolate`. Encoding wrote the literal `"cardinal"` next to
the tension; decoding threw the string away and rebuilt the curve from the
number. A reader of the document could see a word that figure could not read
back, and the nine other words Vega-Lite has in that slot meant nothing at all.

Underneath both sits a question this repository had not had to answer: what is
a smoothed line a claim *about*? `catmullRom`'s own comment asserted one answer
— "a smoothing that misses the data would be drawing something that was never
measured" — and that answer rules out half of the families a chart library is
asked for.

## Decision

Two additions, in two layers, that must not be collapsed into one.

### A `Curve` is a drawing, fitted after the coord

`geom.Curve(CurveMonotone)` names a `CurveKind`. The fit happens in
`geom/curve.go`, on device points, inside the same `appendCurve` the tension
went through — which is now a method on `scratch`, because two of the families
need a workspace and a workspace in this library comes out of the pool.

It changes the path between the vertices and nothing else. No axis is trained
on it, no row is added, nothing in `stat` knows it happened. A curve family is
therefore never a reason for the extent of an axis to move, which is the
property that lets it be chosen last, by whoever is looking at the picture.

Ten families, spelled as Vega-Lite spells them: `linear`, `cardinal` and its
open and closed variants, `monotone`, `natural`, `basis` and its two variants,
and `bundle`. The names are not an aspiration to compatibility; they are the
names a reader of the document already knows.

### Interpolating and approximating are different claims, and both are allowed

`catmullRom`'s old comment was right about interpolating families and wrong as
a rule for all of them. The rule as it now stands:

- An **interpolating** family — linear, cardinal, monotone, natural — passes
  through every vertex. The curve between two rows is a claim about what the
  quantity did between them.
- An **approximating** family — basis, bundle — does not. The vertices are
  control points; the ink is nowhere the data was. It is a drawing of the
  sequence, for when the shape of a bundle of traces is the point and the
  individual readings are not.

Both ship, and `CurveKind.Interpolating` reports which is which, so the
distinction is a value a caller can branch on rather than a paragraph they have
to have read. The doc comments say the approximating ones miss the data in
those words.

What does **not** change with the family is what a row is:
`f.Marks(MarkRows{…})` reports the vertices under every one of the ten. A row is
where its value is regardless of where the ink went, so hit-testing, `interact`
and `a11y` see the same positions whatever was drawn through them. A basis
curve that sails past a peak still hands a tooltip the peak.

### A bent coord takes the edges over, from every family

`appendCurve` still hands a non-straight coord its own `appendEdges`. A spline
is fitted through *device* positions, and under a polar transform the tangent it
fits is not the tangent the data has: the curve would be smooth on the screen
and wrong about the data. An edge the coord already knows how to draw is the
better answer. Every new family inherits this, unchanged and for the unchanged
reason.

`geom.Horizon` also keeps its exemption ([ADR 0065](0065-horizon-charts.md)) and
it now names both options: a spline through a *clamped* fraction overshoots the
band it was clamped to, so a horizon chart draws straight edges whatever `Curve`
and `Tension` said.

### Monotone needs an axis to be a function of, and says so when there is none

The monotone family takes whichever device axis is strictly monotone — x for the
usual series, y for one plotted sideways — because a chart that runs down the
page is as ordinary as one that runs across it. When *neither* is, the run
doubles back on itself, no monotone fit is defined, and it falls back to the
polyline.

Falling back to cardinal would have been the smoother-looking choice and is the
wrong one: it would silently deliver the overshoot this family exists to
prevent, on exactly the data whose shape made it impossible to prevent.

### A smoother is a fit, and it lives in `stat`

`stat.MovingAverage` and `stat.SavitzkyGolay` are the other half of "line plots
have no smoothing", and they are not curves. They run in data space, in `Train`,
they produce points, and the axis is trained on what they produce — so a fit
that runs past the data is inside the panel rather than clipped at its edge,
which is the arrangement `geom.Trend` already had for `Loess`. They join
`geom.Smoothing` beside `Loess` and `LinearFit` and need no other plumbing.

They are two rather than one because they fail differently, and the difference
is the reason to have either: a running mean is the smoother a reader can check
by hand and it flattens every peak it passes; a Savitzky-Golay fit keeps the
height and width of a peak, because a polynomial of degree d reproduces a
polynomial of degree d exactly. The tests assert precisely that — a quadratic
survives an order-two fit untouched and does not survive the mean.

Both take `Span` as their width, converted to a count of rows. No new option:
the fraction is the same fraction, and a second knob spelling the same idea in
different units would be the thing to regret.

`stat` gained nothing else. It still knows about numbers and not about scales,
geoms or themes, and `AppendSavitzkyGolay` solves its normal equations in a
fixed-size array so a chart redrawn every frame allocates only `dst`.

### Why the two halves are not one option

Because they answer different questions and a reader of the chart needs to know
which was answered. "What did this quantity do between two readings" is a curve.
"What is this quantity doing, under the noise" is a fit. A single `Smooth`
option covering both would put a claim about the data and a choice about the ink
behind one word, and the one thing a chart library should never blur is which
marks are measurements.

## Consequences

- `Tension` keeps its meaning exactly. Naming a tension and no family is
  `CurveCardinal` at that tension, which is the whole of what the option meant
  before; every existing chart, golden file and example draws what it drew.
  `Tension` is now documented as the *parameter* of the families that have one
  — the cardinal tangent scale, and bundle's beta — rather than as a switch.
- `CurveLinear` is both the zero value and a family somebody may have asked for,
  so `Desc` carries `CurveSet` beside `Curve`, the way it already carries
  `DashSet` and `MarkerSet`. `Curve(CurveLinear)` beside a `Tension` is a
  straight line, and a round trip that dropped the flag would smooth it.
- Decoding now reads `mark.interpolate`. The one trap is that `geomMark` tells a
  staircase from a line by a `step` prefix on that same field, so **no curve
  family may be spelled with one**. There is a comment on the table and a test
  that decodes every name back to a line.
- Every family emits cubics where a polyline emitted line segments. That is more
  work for a rasterizer, and it is worth naming that `gg` v0.52.5 cannot
  currently flatten a cubic at coordinates beyond about 2²⁰ px without recursing
  for ever (`docs/gg-addcubic-infinite-recursion.md`). Nothing here works around
  it — the bug is a missing depth bound in the backend, and the coordinates that
  reach it are a caller mistake — but these options widen the surface it can be
  met through.
- The closed families close their own path, by wrapping the neighbour each end
  of the run was missing. That is a different shape from `Closed`'s straight
  join back to the first point, so the two must not both be applied; `closes`
  reports which families have already done it.

## Revisit if

- **A Savitzky-Golay order becomes worth exposing on a layer.** The `stat`
  function takes one and clamps it to three; `geom.Trend` fixes it at two,
  because a quadratic is the lowest degree with a curvature and curvature is the
  entire difference from a running mean. If somebody has real data where cubic
  is the right window shape, that is an option, not a rewrite.
- **A family is asked for whose fit is not a function of one axis.** Every one
  here is parameterised by x or by y. A closed shape smoothed by arc length — a
  Catmull-Rom over a genuine loop — is a different parameterisation, and it is
  the case `CurveCardinalClosed` currently approximates rather than solves.
- **A curve is wanted under a bent coord.** The refusal above is a real
  limitation, not a tidy-up: a polar radar with rounded corners is a reasonable
  thing to want. The answer is a spline fitted in *scale* space and mapped
  through the coord per sample, which is a different mechanism from this one and
  should be decided as such — `geom.stepColumns` is the precedent for doing the
  shape work before the coord.

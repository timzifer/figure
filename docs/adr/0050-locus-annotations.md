# 0050 — A locus is an annotation, and the coord draws it

**Status:** Accepted, amended · **Date:** 2026-09-08 · **Implemented:** 2026-09-11

## Context

[ADR 0033](0033-smith-charts.md) ends by naming what would reopen it:

> A second chart wants a grid family its axes have no tick for. That is the
> moment to argue a `Furniture` that carries families rather than sides — and
> it would reopen Γ-as-input, VSWR circles and a projection's graticule
> together, which is the right way to spend that seam.

A second chart has turned up. This record is that argument, and it comes out
somewhere other than where 0033 guessed.

### The chart that turned up

A **Nichols diagram** is how control engineers read an open-loop frequency
response L(jω): the open-loop phase in degrees along X, the open-loop gain in
decibels — 20·log₁₀|L| — along Y. Both axes are linear, both are ordinary, and
the response itself is a line over two columns.

**figure draws that today.** `geom.Line` over two `scale.Linear`, and nothing
in this record is needed for it.

What makes the picture a Nichols diagram is the grid printed underneath: two
families of curves describing the *closed* loop T = L/(1+L). The **M
contours** are the loci of constant |T| and are what the resonance peak M_r is
read off; the **N contours** are the loci of constant ∠T. They are the reason
the chart exists, and no general-purpose plotting library draws them —
MATLAB's Control System Toolbox does, `python-control` does, and outside those
two the form does not exist. Go has nothing.

### It is the same gap three more times

The families 0033 declined are the same shape of thing:

- **constant-|Γ| (VSWR) circles** on a Smith chart — the loci of impedances
  whose reflection has a given magnitude;
- **constant-Q arcs** — the loci of impedances with a given reactance-to-
  resistance ratio;
- **the combined ZY overlay** — the admittance grid drawn on the impedance
  chart, which is the impedance families read through y = 1/z.

Four families, two charts, one gap. The catalogue has no bucket for them
because the catalogue sorts by machinery and this machinery does not exist.

### Why the seam 0033 predicted is the wrong one

The obvious reading is that these are grid lines, that grid lines are
furniture, that furniture comes from a coord, and that `Furniture` therefore
has to carry families as well as sides. `coord.Furniture` is a struct filled
through a pointer, so adding a field to it does not add a method to `Coord`
and does not break the stability rule that interface is under — the widening
is affordable. It is still wrong, for two reasons that only become visible
once the four families are put side by side.

**A Nichols chart has no coord.** Its point mapping is phase to X and decibels
to Y, which is `Cartesian`, exactly. A `coord.Nichols` would be a type whose
`Point` is Cartesian's `Point`, whose `Frame` is Cartesian's `Frame`, and
whose `Furniture` ignores the tick lists it is handed. That is a coord in an
annotation's coat — the mirror image of the reason the beeswarm's offsets
stayed out of `stat`, and refused here for the same reason.

**None of the four families is a function of a tick.** A grid line is where an
axis says a value is; that is what makes it furniture and what makes
`render.drawAxes` able to walk it in tick order and take the label text from
`t.Label`. An M contour is not at a value of either axis. It is the set of
points satisfying a condition on a quantity neither axis carries. It is not
where the axis says something is; it is where the *chart* says something is —
which is the definition of an annotation, and figure has had annotations
since v0.1.

The word for the object is already in the Smith chart's own documentation:
a **locus**.

## Decision

**A locus is an annotation that draws a family of curves defined in data
space, and every coord already knows how to draw it.**

```go
p := figure.New()
p.X(scale.Linear(scale.Domain(-360, 0), scale.TickValues(-360, -270, -180, -90, 0)))
p.Y(scale.Linear(scale.Domain(-40, 40)))

p.Add(geom.Locus(stat.NicholsM, []float64{-12, -6, -3, -1, 0, 1, 3, 6, 12}))
p.Add(geom.Locus(stat.NicholsN, []float64{-1, -5, -10, -20, -45, -90, -150}))
p.Add(geom.Line(open, geom.X("phase"), geom.Y("gain")))
```

It goes in `geom/annotate.go`, beside `HLine`, `VBand`, `Region` and `Note`,
and it takes its literals positionally and its options after them the way each
of those does.

### Why this is the cheap answer and not merely the small one

`ruleGeom.Build` is already this mark with two points in it. It asks the coord
for the panel's extent, maps its one literal through the axis, builds device
points with `cd.Point`, and strokes them with the theme's annotation ink:

```go
cd := f.Coords()
x0, x1, y0, y1 := cd.Extent()
p := s.Map(g.at)
strokeRun(b, cd, &path, []ir.Point{cd.Point(x0, p), cd.Point(x1, p)}, stroke, false)
```

A locus is the same five lines with a curve where the two points were. `HLine`
is the degenerate member of this family already — the locus of constant y —
which is the strongest evidence available that the shape is right.

### The consequence that decides it: the coord draws the family for free

A locus is defined in **data space**, and so it goes through the coordinate
stage like every other mark. That is not a detail; it is the whole return on
the decision.

- A VSWR circle is *not* implemented as a circle on the disc. It is
  implemented as the set of impedances whose |Γ| is a given value, emitted as
  data-space points, and `coord.Smith` maps it into the circle it looks like.
  There is no Smith-specific drawing code anywhere in it.
- A constant-Q arc is the locus |x| = Q·r, which is two straight rays in
  impedance and therefore two arcs on the disc, by the same map, with no
  second implementation.
- The ZY overlay is the impedance families evaluated through y = 1/z. One
  family definition, read twice.

This is the v0.8 move made a fourth time: an icicle under a polar coord is a
sunburst, an arc diagram with its rail at the rim is a chord diagram, and a
locus of constant reflection magnitude under a Smith coord is a VSWR circle.
The mark supplies data-space geometry and the coordinate stage decides what it
looks like. Neither of the two knows the other's chart exists.

### The family is an interface, and the built-in ones are named

```go
// A Family is one indexed set of curves in data space. Given a level and the
// panel's extent, it appends the data-space points of that level's locus.
//
// A curve that leaves the extent and comes back, or that repeats — a Nichols
// family repeats every 360° of phase — appends NaN between its runs, which is
// the gap every mark in figure already understands.
type Family interface {
	Locus(xs, ys []float64, level float64, ext Extent) ([]float64, []float64)
}
```

The built-in families live in `stat`, because each of them is a pure function
of numbers and that is where those go: `stat.NicholsM`, `stat.NicholsN`,
`stat.SmithVSWR`, `stat.SmithQ`. They are values rather than functions so that
a name can be written down.

**Serialisation follows [ADR 0041](0041-qq-plots.md) exactly.** A named family
round-trips through `spec` as a string; a `Family` a caller wrote in Go does
not, and the escape hatch is the one 0041 already chose — materialise the
curve as data and draw it with `geom.Line`. No function serialisation, no
plugin registry beyond the one `geom.Register` already is, no dependency.

### The arithmetic is smaller than the picture suggests

Both Nichols families are **circles in the complex L-plane**, and the Nichols
plane is the log-polar view of that plane. With L = u + jv:

```
|T| = M      centre ( −M²/(M²−1), 0 ),      radius | M/(M²−1) |
             M = 1 degenerates to the line u = −1/2

∠T = α       centre ( −1/2, 1/(2N) ),       radius ½·√(1 + 1/N²),   N = tan α
```

and the chart is

```
x = arg(L) in degrees        y = 20·log₁₀|L|
```

So a family walks a circle and applies two lines of arithmetic. There is no
root finding, no iteration and no case analysis beyond the degenerate M = 1.
The whole of both families is a few dozen lines.

Two properties of these curves are worth recording because they are not bugs:

- Every N contour passes through L = 0 and through L = −1. The first is
  −∞ dB and the second is the critical point, which is why the contours all
  converge there. A point the scale cannot place is a NaN, and the missing-data
  policy has covered that since v0.1 — the test named
  `TestTheErrorPolicyCoversValuesTheScaleCannotPlace` already says so.
- The families repeat every 360° of phase, so a family is drawn once per 360°
  band the panel's extent covers. That is the reason `Locus` is handed the
  extent rather than only the level, and it is the only genuinely fiddly part
  of the implementation.

### A locus does not train the domain

`HLine` extends the axis by default, because an annotated threshold that is
out of view is not an annotation. A locus does the opposite and never trains,
because it is furniture in intent: it says what the region of the plane means,
and the region is whatever the axes already show. It is also the only defensible
answer arithmetically — the M = 0 dB locus runs to −∞ dB, and an axis trained
on it has no ticks left anywhere a reader is looking.

`geom.Extend(true)` is available for the caller who wants the other behaviour
on a bounded family, so the option is not a new one, only a default reversed.

### It is sampled in `Build`, in device space

How many points a curve is drawn with is a question about how wide the panel
is, so it is answered in `Build`, against the device rectangle, the way a
hexbin's lattice and a beeswarm's offsets are
([ADR 0028](0028-distribution-stats.md)). The rule that keeps decimation out of
`Train` — what a chart's axis reports must not depend on how wide the chart is
— does not bind here, because a locus reports nothing to an axis. The two
halves of that rule are consistent: it trains nothing, so it may measure the
screen.

### It announces nothing

A locus has no `data.Source` behind it, so there is nothing for hit-testing to
index, exactly as with every annotation since v0.1. This also happens to be
[ADR 0046](0046-overlay-layer.md)'s conclusion arrived at from the other side —
a piece of furniture a pointer can hit is furniture that flickers — and it is
why the reading a Nichols chart is for (find where the response is tangent to
the 3 dB contour) is the reader's, not the tooltip's.

## Consequences

| | |
|---|---|
| `render`, `ir`, `coord`, `scale`, `layout` | unchanged — **nothing** below `geom` is touched |
| `geom` | one constructor and one geom in `annotate.go`, one exported `Family` interface, one reversed default (`extend`) |
| `stat` | four family values, each a pure function with a determinism test, per CONTRIBUTING's rule for reductions |
| `spec` | a `"locus"` mark, a `family` string and a `levels` array; an unnamed Go family declines to serialise, and says so |
| `interact` | unchanged; a locus is not in the index because it has no rows |
| `a11y` | a labelled locus is a legend entry and is described as one; the curves themselves are furniture and are not enumerated |
| `theme` | unchanged — it draws with `annotationStroke`, which is where a reference line's ink already comes from |
| Charts unlocked | Nichols diagram, VSWR circles, constant-Q arcs, the ZY overlay, and the Hall chart, which is the same two circle families before the log-polar step rather than after it |

The Nichols chart specifically costs, beyond the shared mark: the two families,
an example, and a gallery figure. The coord package is not opened.

## Not in scope

- **A `Furniture` that carries families.** This record is the argument 0033
  asked for, and the answer is that the four families it was asked about are
  not furniture. `render` still walks two tick lists and still labels nothing
  a scale did not write. The one thing that genuinely does need the wider seam
  — a projection's graticule, which has no tick behind it *and* is not a
  data-space curve, because a projection has no linear interval underneath — is
  untouched by this and is still argued on its own evidence.
- **Γ as input to a Smith chart.** Unchanged, and for 0033's reason, which this
  record does not weaken: with Γ on the axes the *impedance* grid loses its
  ticks, and that grid is labelled. A locus is unlabelled-by-default furniture;
  an axis grid is not.
- **Labels written along a curve.** `geom.Locus` gets a legend entry from
  `Label` like any annotation. Writing "3 dB" on the contour itself, rotated to
  the tangent, is a second decision: it needs an anchor rule, a rotation, and a
  seat at the table [ADR 0040](0040-label-collision-avoidance.md) already sets
  for participating text layers. It is the part of this record most likely to
  need its own, and the chart is readable without it because the contours nest
  monotonically.
- **Automatic level selection.** Which contours a chart prints is a convention
  of its field — the Nichols levels above are the ones on printed paper — and
  is `scale.TickValues`'s situation exactly. The caller names them.
- **Contour plots.** `stat.Contour` over a field is marching squares over data
  and is bucket F's, unrelated to this and still open. The names are close
  enough to be worth keeping apart: a locus is a formula, a contour is a
  measurement.

## Amendment: what building it sharpened

Three things the record left open turned out to have one answer each, and a
fourth was wrong about where a type lives.

**`Family` is `stat`'s type, and `geom` names it.** The record puts the
interface in `geom` and the built-in families in `stat`, which cannot both be
true: `geom` imports `stat` and not the other way round, so a family written
there could not name a type declared here. `geom.Family` and `geom.Extent` are
aliases of `stat.Family` and `stat.Extent` — the same move `ir.Color` makes for
`color.NRGBA` — so the seam is named where a caller looks for it and defined
where it can be implemented.

**The extent carries a sample count, and a family is refined rather than
walked.** The record says a locus is sampled in `Build`, in device space, and
leaves how open. `Extent.Steps` is that count, and it is what the two Nichols
families set their *tolerance* from rather than their sample count: the chart is
the log-polar view of a circle, and the two are not evenly spaced against each
other. An N contour passes through L = 0, so the arc of its circle that is the
whole plunge to −∞ dB is a hair of the circle and the rest is a smooth curve a
few dozen samples describe. Walked uniformly, the −1° contour stops dead at
−6 dB, which draws a contour hanging in mid-air well above the bottom of the
panel. So a Nichols curve is bisected until its step is a few pixels, and the
refinement stops at a step that leaves the window — which is also what bounds
it, because the arc approaching the origin is self-similar and a rule that only
looked at the step would halve for ever.

**A curve that runs to infinity is cut a little outside the panel**, at a tenth
of its height beyond each edge, so that it leaves the picture at the edge rather
than short of it. Something has to cut it: a device coordinate is a float32, and
every sample past the edge is a point in a file nobody can see. The Smith
families' equivalent is `Extent.far`, which is why a constant-Q arc reaches the
rim of the disc — an infinite impedance is the rim — rather than stopping at the
panel's own `X1`.

**A branch is found by the step it takes, not by looking for it.** Every N
contour passes through the origin, where the phase turns by half a turn between
one sample and the next; a step above ninety degrees ends the run and starts
another. Without it the two branches are joined by a line drawn across the chart
at whatever depth the sampling happened to stop at. The same rule costs nothing
on a curve that does not have one, because no family here bends that far in one
step of a few hundred.

**"It announces nothing" means it announces no *row*.** A locus is a layer, and
a layer's drawing calls reach the hit index like every annotation's have since
v0.1 — one mark per run, so a family costs the index its curves and not its
samples. What it cannot report is a row, because there is no row: a hit on a
contour reports where the pointer is, with `Row == -1`, exactly as a hit on a
reference line does. Keeping the curves out of the index altogether would mean a
new optional interface and a change in `render`, which this record's consequences
table says it does not need; `interact`'s `TestALocusReportsNoRow` pins what it
does do instead.

One thing the record got exactly right and is worth recording as confirmed: the
coord really does draw the family for free. `stat.SmithVSWR` emits impedances
and `coord.Smith` makes them the circle they look like; there is no Smith-shaped
code in the family, in the mark, or in `render`, and `render`, `ir`, `coord`,
`scale` and `layout` are untouched by this record's implementation, exactly as
its consequences table says.

## Revisit if

- A third family wants to be labelled on itself. Then the anchor-and-rotation
  question above is real and gets its own record, and it is the same question
  a labelled graticule asks.
- A family turns out to want the panel's *scales* rather than its extent —
  something whose shape depends on a log axis, say. `Locus` is handed the
  extent deliberately; handing it the scales would put an interface call per
  point back on a hot path, which is the cost ADR 0018 property 3 exists to
  refuse.
- Someone asks for a psychrometric or Mollier chart. Constant-enthalpy,
  constant-wet-bulb and constant-relative-humidity lines are this mark with a
  different formula, and the answer should be "write three families", not
  "write a coord".

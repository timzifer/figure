# 0067 — A colour channel may carry two readings, and the second one takes resolution away

**Status:** Proposed · **Date:** 2026-09-11 · **Implemented:** —

## Context

Three charts that look unrelated want the same missing piece, and each of them
is individually too small to build it for.

**A value-suppressing uncertainty palette (VSUP)** colours a quantity and its
own uncertainty together, by giving the quantity *fewer distinguishable
colours* the less certain it is. Correll, Moritz and Heer introduced it in 2018
and measured what it fixes: readers of a conventional map read precision off
colours that do not carry any, and a palette that refuses to draw distinctions
the data cannot support stops them. The key is a tree — wide at the certain end,
collapsing to a single colour at the uncertain one.

**A multi-class hexbin** colours each bin by which class dominates it and how
purely — two readings per cell, one categorical and one continuous.

**A bivariate choropleth** is the 3×3 square: two ramps crossed, one colour per
pair of classes. It is the oldest of the three and the one most people have
seen.

### Where each of them stops today

`scale.ColorScale` is `Color(v float64) ir.Color` — one number in, one colour
out — and `geom.ColorBy(col string, s scale.ColorScale)` names one column.
[ADR 0061](0061-columns-are-one-value.md) has just finished making a column one
value; this asks for one *channel* to read two of them, which is a different
question and is not answered anywhere.

The guide side, by contrast, is ready and has been since
[ADR 0027](0027-size-channel-and-the-guide-column.md), which generalised the
guide column once rather than extending it twice and said what a fourth kind
would cost: "a constant and two functions". `layout.Guide` carries what the
solver needs of any kind and `GridResult.Guides` is the boxes.

### Why this is not an ornament for this library in particular

**The thesis is already in the tree, in a different channel.**
[ADR 0036](0036-error-bars.md) exists because "every chart of a mean, a
forecast or a tolerance has one number and a claim about how well it is known,
and the second half had nowhere to go". That is this record's sentence with the
channel changed. And the colour channel is where the omission does the most
damage, because a filled cell reads as a measurement whether or not anybody
measured it — which is precisely what VSUP was built to demonstrate.
[ADR 0061](0061-columns-are-one-value.md)'s own motivating example is an
adapter for a metrology library.

**And the multi-class bin closes a hole in the big-data story.** figure has
three answers to overplotting — decimation, the density raster, and the hexbin
— and all three answer *how many*. None answers *who*. `geom/decimate.go` says
so in code: `autoReduction` returns `NoDecimation` the moment `c.varying(s)` is
true, because a density raster of rows in eight colours has no colour to paint.
A binned answer that keeps the classes is the missing half, not an exotic.

## Decision

**A bivariate colour scale is an optional interface beside `ColorScale`, and
the second column is a second option on the layer.**

```go
type BivariateColorScale interface {
	ColorScale
	// ColorAt reads the pair: v is the quantity, u the second reading.
	ColorAt(v, u float64) ir.Color
	// SecondDomain is the trained extent of u, for the guide.
	SecondDomain() (lo, hi float64)
}
```

```go
p.Add(geom.Rect(src,
	geom.ColorBy("mean", scale.VSUP(palette.Viridis, 8, 3)),
	geom.UncertaintyBy("sd")))
```

### Riding the interface rather than widening it

This is `scale/classed.go`'s move, quoted from its own doc comment: a
`ClassedColorScale` "rides the `ColorScale` interface the way
`DiscreteColorScale` does". A bivariate scale is the third rider. `Color(v)`
stays, and answers the way the scale would at full certainty — so every mark
that paints through the existing interface keeps working, a bivariate scale
handed to a layer with no second column degrades to the univariate reading
rather than failing, and nothing in `render`, `geom` or `spec` changes shape.

`geom.UncertaintyBy(col)` is read only by a scale that implements the
interface. A second column named against a scale that does not is an error in
`Train`, named, rather than a column silently ignored — the rule
`geom.Hexbin`'s deliberately-unread `ColorBy` column is the *exception* to, and
it is an exception because there the quantity is the layer's own.

### VSUP is a classed scale whose class count is a function of the second reading

This is the sentence that makes the record cheap.
[ADR 0042](0042-colour-transforms-and-classes.md) already cuts a ramp into
classes and already reports the boundaries; `scale.Quantize(ramp, n)` is the
top row of a VSUP tree. What VSUP adds is that n halves as u rises, so the tree
is `layers` rows of `Quantize`, and `Breaks()` at a given u is still a list of
numbers the guide can print.

`scale.VSUP(ramp, classes, layers)` and `scale.BivariateMatrix(a, b, k)` are
the two constructors. The second is the 3×3 square, and it is also what a
multi-class bin wants with the class on one axis and the purity on the other.

### The guide is a square, and it is the fourth kind

`geom.BivariateGuide` carries the scale and two labels; `layout` gains a
`GuideKind` whose measured box is square-ish rather than a bar. VSUP's key is
drawn as its tree and the matrix's as its square, which is a difference in what
the guide *draws*, not in what the solver measures.

[ADR 0048](0048-clickable-colourbar-and-size-key.md)'s rule extends without
amendment: a colourbar reports a quantity rather than a series because it is not
one, and a two-dimensional key reports a pair. A drag on it is a two-dimensional
brush, which the record explicitly leaves to the host.

### The multi-class hexbin is not a new mark

With the scale and the guide in place it is `geom.Hexbin` plus `geom.GroupBy`:
`stat.Hex` gains a class dimension — a counter per class per cell instead of
one `uint32` — and the cell's colour is `ColorAt(dominant, 1 − purity)`.

Its guide is the one thing that is not symmetric with the rest, and the reason
is the one the hexbin already documents. The *counts* are not known until the
plot rectangle is, so a purity bar cannot be measured in time; the *classes*
are known in `Train`, because they are the groups. So a multi-class hexbin
contributes ordinary legend entries naming the classes, and says how pure a
cell is through a hit rather than through a key — which is exactly what
`geom.Hexbin` does today with its counts, and for the same reason.

## Consequences

| | |
|---|---|
| `scale` | one optional interface, two constructors, one option type; `ColorScale` unchanged |
| `geom` | `UncertaintyBy`, one guide type, one named error; a class dimension in the hexbin's binning |
| `stat` | `Hex` counts per class; `Cell` grows a class slice |
| `layout`, `render` | a fourth guide kind — a constant and two functions, per ADR 0027 |
| `ir`, `coord`, `three` | unchanged |
| `spec` | a `"vsup"` and a `"bivariate"` colour scale, and a second field on the colour encoding |
| `a11y` | a mark reads both numbers; the palette is a way of drawing them |
| Charts unlocked | VSUP maps and heatmaps, multi-class hexbin, bivariate choropleth, any mark whose colour carries a measurement and its error |

## Not in scope

- **Opacity as uncertainty.** It is the obvious cheap version and it is the one
  VSUP's authors measured as misleading: a faded mark confounds with the
  background, with overlapping marks and with a small mark, and the whole
  argument of the form is that taking *resolution* away is the honest move
  where taking contrast away is not.
- **An N-variate channel.** Two readings have a square; three have nothing a
  reader can decode, and the guide is the proof.
- **Blending two arbitrary ramps.** A mixed colour that corresponds to no
  entry in either ramp is a colour that names no value.
- **Changing `ColorScale`.** Every argument here is available without it, which
  is the test [ADR 0020](0020-discrete-colour-and-multi-entry-legends.md)
  applied when it declined to widen `Geom`.
- **Computing the uncertainty.** A standard error, a posterior width or a count
  of observations is the caller's column. `stat` reductions that produce one
  are [ADR 0054](0054-statistical-instruments.md)'s territory.

## Revisit if

- A second channel wants two columns — a size that carries an error, a position
  that carries one. Two channels pairing columns independently would be an
  argument for the pairing belonging to the *encoding* rather than to each
  channel, which is a bigger change than this and should not be made on one
  example.
- The multi-class bin turns out to want a purity key after all. That reopens
  the hexbin's guide-ordering problem rather than this record, and the fix
  there is to bin in data space, which would be its own decision.

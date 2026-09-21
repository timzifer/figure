# 0082 — A label fits the shape of its box on screen, is moved, broken or shrunk before it is dropped, and may be called out of it

**Status:** Accepted, amended · **Date:** 2026-09-21 · **Implemented:** 2026-09-21 · see [Amendment](#amendment-a-called-out-label-is-broken-or-shrunk-before-it-is-dropped)

## Context

`geom.Text` labels a box — a bar, a gantt cell, a slice of a pie or a ring of a
sunburst — and drops a label that does not fit, because an overrunning label
reads as belonging to the neighbour. What "fits" meant was one number: the
width of the box, measured under a polar coord as the chord across the middle
of the slice. That is a width, and a label is a width *and* a height, laid out
level, in a box that under a polar coord is a wedge of an annulus. A narrow
label in a thin slice near three o'clock fits the chord and overruns both
edges; the same number in a thin slice near twelve fits its middle and crosses
into its neighbours with its corners. Donut and sunburst labels were drawn over
the edges of their slices, which is exactly what dropping exists to prevent.

The other half of the complaint is what happens to a label that honestly does
not fit. Dropping it is right by default, but a pie's thin slices are where the
number is most wanted, and the usual answer is the one
[ADR 0040](0040-label-collision-avoidance.md) deferred in its last line:
*"Revisit when callouts need leader lines."*

## Decision

**Fitting is decided on screen, by the coord.** A box label's font box — laid
out about its anchor the way it will be drawn, aligned and turned, with a pixel
and a half of padding — has to lie inside the box. Whether a device point lies
inside a box in the space the scales map into is what `coord.Coord.Invert`
answers, for every coord, so the test walks the edges of the font box, four
points to a side, and inverts each one. Corners alone are not enough: the arc
of a donut's hole bows into a label whose corners are clear of it. Under
Cartesian this is a rectangle in a rectangle and the only change is that a
label's height is now counted as well as its width.

**A label that does not fit the middle of its box is looked for elsewhere in
it.** Under a coord with a middle — `coord.Exploder` again — the label is also
tried at a fifth, a third, two thirds and four fifths of the way along each of
the box's two spans. A long cell of a sunburst turns as it goes round, and a
level label that does not fit across it where it runs diagonally fits where it
runs level; a slice is wider at its rim than across its middle. Under
Cartesian a box is the same shape all the way along, so only its middle is
tried, and a chart of bars pays nothing for the search.

Sliding is on by default and `geom.Slide(false)` turns it off. A label moved to
the end of a long cell no longer sits where the eye looks for it, and on a
chart of many thin cells it can read as its neighbour's. With sliding off a
label is only ever in the middle of its box — broken or shrunk there — and one
the middle cannot hold is called out where `Callout` allows it and dropped
where it does not: a leader says which cell a label belongs to, and a label
that has slid does not. The spec writes `"slide": false`, and only then,
because a document that says nothing slides.

**With `geom.Wrap`, a label may be broken over two or three lines.** The break
is at spaces and is the one whose widest line is narrowest — the most compact
block the words make — and the block is fitted the way a line is, at every
spot. Breaking comes before shrinking: two lines at the layer's size read
better than one at three quarters of it. The lines are cut out of the label
rather than built, and a block is drawn as one run per line, because a run is
one line and the IR stays as it is. It is opt-in because it changes the shape
of a label as well as its size; a label called out of its box is written on
one line, because outside the box there is room for it — until there is not;
see the [amendment](#amendment-a-called-out-label-is-broken-or-shrunk-before-it-is-dropped).

**A label that does not fit is shrunk before anything else happens to it.**
It is drawn at the largest size that fits, down to `geom.MinFontSize`, whose
default is three quarters of the layer's size. The search scales the metrics
measured at the layer's size rather than measuring again, so it is the coord's
arithmetic and not the shaper's, and the size it lands on is rounded down to a
quarter of a unit so that a chart redrawn at one size asks for a few fonts
rather than one per label. Every shape — one line, two, three — at every spot
is tried, and the one that holds the largest type wins, fewer lines winning a
tie.

**Below the floor, `geom.Callout` writes the label outside its box.** The
label leaves the box through the middle of whichever edge is furthest out along
the box's bisector, runs out along the bisector until it is past everything the
coord draws, and turns level at the end of a short arm; labels on one side are
stacked apart top to bottom so that a run of thin slices writes a column rather
than a pile. The direction is `coord.Exploder`'s — the bisector is what
[ADR 0026](0026-breaking-a-mark-out.md) already answers — and "past everything
the coord draws" is `Invert` again, against `Extent`, which is why a label out
of the inner ring of a sunburst crosses the outer rings rather than stopping on
top of them. The label is written at the layer's size, in the theme's label
ink — neither the contrast against a fill it no longer stands on nor a colour
chosen to be read on that fill — and the leader is drawn in the theme's
annotation stroke. A called-out label that would run past the edge of the
panel is drawn in on a shorter arm first, down to an arm a quarter of the type
long; one that still has no room is dropped with its leader rather than cut by
the edge, which is the rule every other label here keeps. The first version
dropped it outright, and the gallery's donut lost its thinnest slice's label by
four pixels — the one label on that chart a callout exists for.

**A label goes with its slice.** A text layer given `geom.ExplodeBy` reads the
same break-out column a bar does; the label is fitted where the slice was,
because that is the box the coord answers for, and then moved by the slice's
displacement, leader and all.

Without `Callout`, `Elide` cuts the label as before, now to the widest run the
box holds at the layer's size; without either, the label is dropped.

**A layer that draws outside the coord's clip says so.** A polar panel clips
its data to the disc, and a called-out label is outside the disc and inside the
panel. `geom.Overhanger` is an optional interface beside `geom.LabelAvoider`;
`render` clips a layer that reports it to the panel rectangle and puts the
coord's clip back for the next layer. The rectangle is allocated only when a
layer asks for it, so a chart without one draws exactly what it drew.

## Consequences

- Box labels that used to overrun their slices are now shrunk, called out or
  dropped. A Cartesian label that fitted its box's width and not its height is
  now shrunk or dropped too; no golden file in the repository had one.
- `Callout` does nothing under Cartesian, for `Exploder`'s reason: a bar has no
  outside that is not the neighbouring bar's inside, and inventing a direction
  would be a claim the chart does not make.
- Called-out labels do not take part in `AvoidOverlap`. They are stacked
  against each other a side at a time, inside the panel; a label from another
  layer is not an obstacle to them. A wrapped block asks the placer about the
  label as one run, and is kept or refused whole.
- A leader out of an inner ring crosses the rings outside it. That is the
  price of leaving the chart by the shortest way, and it is paid only by a
  label that had no room in its own cell at any spot, in any shape.
- The spec writes `callout`, `minFontSize`, `wrap` and `slide` on a text mark.

## Revisit if

- Called-out labels need to avoid other layers' labels — that is the point to
  route them through the label placer rather than beside it.
- A sunburst wants its labels turned along the ring rather than level. That
  is a rotation chosen per label, and the fit already takes one; what is
  missing is the rule that picks it.
- A Cartesian chart wants callouts — a label above a short bar is a direction
  somebody chose, and it would be a named option rather than a default.

## Amendment: a called-out label is broken or shrunk before it is dropped

The decision above wrote a called-out label on one line at the layer's size,
and dropped one that had no room beside the chart even on a shorter arm. In a
sunburst whose panel is not much wider than the ring, that dropped the wrong
labels: a thin slice at nine or three o'clock has only the margin beside the
ring, and the labels that did not fit there were the long names — the ones a
reader could not have guessed from the colour, and the ones a callout exists
for. A slice near twelve, whose label turns level almost above the middle, had
room for the same name and kept it.

**A called-out label is fitted to the room beside the chart the way a box
label is fitted to its box, in the same order.** The room is measured on its
own side, from where the label starts at the end of a full arm to the edge of
the panel, plus the part of the arm it may give up. What fits it at the
layer's size on one line is written as before. With `geom.Wrap`, the label is
next tried broken over two lines and then three, at the break whose widest
line is narrowest — two lines at the layer's size read better than one at
three quarters of it, which is the reason breaking comes before shrinking in a
box. Then it is shrunk, down to `geom.MinFontSize`, on whichever shape holds
the largest type, fewer lines winning a tie. Only a label that fits none of
these is dropped with its leader.

Only the width is fitted. The column beside the chart is as tall as the panel,
and a label broken over lines is a taller block in it: labels on one side are
now stacked apart by half of each neighbour's height and a pixel, which for
two one-line labels is the line and a pixel it always was. The leader meets a
block in its middle. A block pushed past the bottom of the panel is dropped
there, as a one-line label was.

The shape is settled before the labels are stacked, because stacking needs the
heights, and that is why the decision is made per label from its own side's
room rather than while drawing.

Nothing changes for a label that fitted before: it is written on one line at
the layer's size, where it was. The documentation figures are the same.

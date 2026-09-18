# 0070 — A coord raises a labelled family of its own, and `Furniture` carries families beside its two sides

**Status:** Accepted · **Date:** 2026-09-18

## Context

Three records have now deferred the same seam, and the third one deferred it
with a deadline.

[ADR 0033](0033-smith-charts.md) chose impedance rather than Γ as a Smith
chart's input, and the argument was that `render.drawAxes` walks two tick lists
and takes label text from `t.Label`: *"a family with no tick behind it has
nowhere to come from and nothing to be labelled by."* Its **Not in scope**
declines the constant-|Γ| circles and the constant-Q arcs for that reason, and
its **Revisit if** names the moment — *"a second chart wants a grid family its
axes have no tick for"* — and says it would reopen Γ-as-input, the VSWR circles
and a projection's graticule together, *"which is the right way to spend that
seam."*

[ADR 0051](0051-barycentric-coord.md) was that second chart. A ternary panel
has **three** labelled ladders and two tick lists. Two of its three families
are the images of the axes' own ticks, and the third is drawn — as a second
subpath inside the X tick's shape, which is why `Furniture` gained nothing —
but it is drawn **unlabelled**, and the record says so in as many words:
*"What is genuinely missing is that family's labels, because `Furniture`
carries one `Label` per tick per side and there is no third side."* It declined
to spend the seam on two thirds of the problem and set its own trigger: *"The
third family's labels are asked for by name."*

[ADR 0050](0050-locus-annotations.md) sharpened the shape of the thing being
deferred rather than deferring it: `geom.Locus` draws an arbitrary family of
curves from a formula, so a caller who wants a fourth ladder has had one since
it landed. What a locus is not is **furniture**: it takes annotation ink, it is
a layer rather than a property of the panel, and 0051 says exactly why that
stops it being the default — *"the chart would come out with two of its three
families one colour and the third another."*

So the state of things is that the library can draw a third family and cannot
label one, in a package where labelling is the whole difference between a grid
and a ladder. A ternary chart's third edge carries the component a reader is
often after — the clay fraction, the quartz fraction, the third phase — and it
is the one edge of the three with no numbers on it.

## Decision

**`coord.Furniture` gains `Families`: grid families that belong to the coord
rather than to either axis, each carrying its own lines, its own label
positions and its own label text.**

```go
type Family struct {
	Name   string   // what the ladder reads
	Lines  []Shape  // one per level
	Labels []Label  // where each level's label sits, parallel to Lines
	Text   []string // what each says, parallel to Lines
}
```

`Furniture.Families` is a slice of them, reset and regrown with its buffers
like every other per-frame list on that struct, and a coord that raises none —
which is every coord in this package but one — leaves it empty and costs
nothing.

**A family carries its own text, and that is the whole of the widening.** The
constraint 0033 named was not that a family had nowhere to be *drawn*; 0051
proved a family can ride inside a tick's shape. It was that it had nothing to
be *labelled by*, because label text reached `render` only from
`scale.Tick.Label`. A family that says what its own levels are called removes
that constraint for every customer at once — a ternary's third edge, a
graticule's parallels, the VSWR ladder on a Smith chart drawn from Γ — which is
the together-or-not-at-all that 0033 asked for.

### The three rules `render` applies, and why each is the one it is

**Lines take grid ink**, and are governed by the two questions the grid as a
whole is governed by: `Panel.HideGrid`, and whether the theme's grid stroke is
visible at all. They are deliberately *not* governed by `ShowGridX` or
`ShowGridY`, because a family is neither axis: a theme that turns off the
horizontal grid has said something about the horizontal grid, and silently
taking the third family down with it would make the answer depend on which of
the three components the caller happened to put on X.

**Labels take tick ink and the tick font**, because a family is a ladder and a
ladder's numbers are tick labels wherever else they appear. They are drawn when
the panel writes tick labels at all — which in a facet is the outer panels — so
an inner panel that leaves its ladders to the edge of the grid leaves all three
of them.

**Labels are thinned last.** `render` already thins the labels of a panel whose
labels do not share a row against each other by their boxes, greedily, in axis
order. A family's labels join that pass after both axes, so where a family's
number would collide with an axis's number the axis keeps it. A family is a
third reading of the same chart and the axes are the first two; if something
has to go, it is the third one.

### The first customer, and the one visible change it makes

`coord.Ternary` raises the constant-c family: the same level sequence the X
ticks carry, labelled with the same strings, on the edge where the second
component is nothing.

Placing it forces the other two ladders into the arrangement every printed
ternary chart already uses, and this is the one thing about existing charts
that changes. **Each component is now read along its own edge**, cyclically:
the first along the base, the second along the a = 0 edge, the third along the
b = 0 edge. Before this record the first two ladders both ran out of the corner
where the third component is everything, which is fine while there are two of
them and leaves the third nowhere to go: the two edges it crosses were the two
already carrying numbers, and a third number at the same point is not a ladder,
it is a collision the thinning pass would resolve by deleting one of them.

The grid lines themselves do not move — the geometry of a constant-a line is
what it was — and the constant-c family moves out of `GridX`'s second subpath
into its own family, where it can be labelled. A chart's three ladders now each
read from nothing to everything along their own edge, which is what a soil
texture triangle, a QFL diagram and a phase diagram are all printed with.

### What this is not

**It is not a caller-facing seam.** `Families` is filled by a coord and read by
`render`. A caller who wants a fourth ladder, an unusual level sequence or a
differently-styled one still reaches for `geom.Locus`, exactly as 0051 said,
and that escape hatch is untouched and still the right one for a family that is
an annotation rather than a property of the coordinate system.

**It is not a spec field.** A family is derived from the coord and its ticks,
so a ternary chart serialises today exactly as it did: `{"type":"ternary"}`,
with no new key. A coord that wants its families configured configures the
coord.

**It is not `Furniture` gaining a third side.** A side has an axis line, tick
marks, an inside-the-panel flag, a shared-row question and a labels-first
preference; a family has lines, labels and text. Modelling it as a side would
have imported five answers a family does not have.

## Consequences

| | |
|---|---|
| `coord` | `Family`, `Furniture.Families`, and `Furniture.Reset` keeping their buffers; `ternary` raises one and moves its first ladder to the base |
| `render` | strokes family lines with the grid, writes family labels with the tick font, thins them after both axes |
| `ir`, `geom`, `scale`, `layout`, `spec`, `a11y` | unchanged |
| `interact` | unchanged; a family is furniture and furniture is not hit-tested |
| Golden files | unchanged, because no coord in the repository's golden charts raises a family and Cartesian is still the identity |
| Charts unlocked | a ternary chart with all three components readable — the soil texture triangle, QFL and QAP diagrams, three-phase and flammability diagrams |

### What it makes cheap next

Each of the three things 0033 said should be reopened together is now a coord's
own arithmetic and no further seam:

- **Constant-|Γ| circles and constant-Q arcs** as furniture rather than
  annotation: two families on `smith`, drawn in grid ink beside the two it has.
- **Γ as input**: a Smith coord whose axes carry Γ raises the impedance grid as
  families, which is precisely the thing 0033 costed and refused.
- **A projection's graticule**: meridians and parallels are families with their
  own levels and their own labels, which was the third customer and the reason
  the seam was worth spending once rather than three times.

None of them is built here. What is built is the field they each needed, and
the record 0033 asked for before any of them gets code.

## Not in scope

- **A family the caller declares.** `geom.Locus` is that, and it is a layer.
- **Hit-testing a family.** Furniture is not indexed; ADR 0015 indexes marks.
- **A family in the JSON spec**, per the argument above.
- **Minor levels inside a family.** A family's levels are the ones it raises;
  a coord that wants a finer ladder raises more of them. There is no
  major/minor question here because there is no tick generator behind it.

## Revisit if

- A family wants ink of its own — a graticule drawn lighter than the grid, a
  VSWR ladder drawn in the annotation colour. That is a theme question and the
  honest fix is a style hint on `Family`, not a second drawing order.
- A caller asks to turn one family off by name. Today the answer is the coord's
  options, which is where a ternary chart's `sum` already lives.

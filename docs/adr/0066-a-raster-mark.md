# 0066 — A field sampled on a grid is one image, and the lattice already knows its shape

**Status:** Accepted, amended · **Date:** 2026-09-11 · **Implemented:** 2026-09-15

## Context

`docs/chart-types.md` bucket A calls a heatmap a recipe: `geom.Rect` with
`ColorBy` over two band scales. That is true and it is enough for the heatmap
most people mean — a few dozen categories by a few dozen categories, one
labelled cell per reading.

It stops being true at the size a *measured field* comes in. A spectrogram of
one minute of audio is two thousand frames by five hundred bins. A Hovmöller
diagram of a season is a thousand timesteps by a few hundred positions. A
recurrence plot of ten thousand samples is ten thousand squared. Each of those
is one `Rect` per cell today: a million primitives through the IR, a million
paths in the SVG, and a file no browser will open — to produce a picture whose
every cell is smaller than a pixel.

### Three things in the tree already say what the answer is

**`ir.Backend.Image` exists and every backend implements it** — `backend/svg`,
`backend/gg`, `backend/canvas` and `backend/pdf` all have it, because
`geom.Scatter`'s density raster needs it. So the primitive this wants is not a
widening of the IR, which is the change CONTRIBUTING says needs the most
argument.

**`stat.Grid.Raster` already paints a grid of numbers into an `*image.NRGBA`**,
reusing the buffer across frames and leaving empty cells transparent so the
plot's own grid reads through.

**`stat.Lattice` already turns a long `(x, y, v)` table into a product grid**
and reports a fault code naming what is wrong with a table that is not one
([ADR 0064](0064-a-contour-and-its-lattice.md)). It was built so that
`three.Surface` and `geom.Contour` could not disagree about the same data.

A raster mark is those three things wired together. The record exists because
wiring them together forces four decisions that are easy to get quietly wrong.

## Decision

**`geom.Raster` draws a value sampled on a regular grid as one image, and it
reads the channels `geom.Contour` reads.**

```go
p.Add(geom.Raster(src, geom.X("t"), geom.Y("hz"), geom.Z("db"),
	geom.ColorBy("db", scale.Sequential(palette.Viridis))))
```

### It shares the contour's lattice, for the contour's own reason

[ADR 0064](0064-a-contour-and-its-lattice.md) put the resolver in `stat` so
that a surface and the contours on its floor could not disagree, and said why
in a sentence that decides this too: *two resolvers that agree today disagree
at the first duplicated position.* A raster with its isolines drawn over it is
the most common form this mark appears in, and they must come from one reading
of the table.

So `geom.Raster` and `geom.Contour` take the same three channels, run the same
`stat.Lattice`, and report the same faults with the same message.

### It has a colourbar, and that is the point

`geom.Hexbin` deliberately has none, and its doc comment records why: the counts
are not known until the plot rectangle is, and the guide column is measured
before it — the ordering [ADR 0011](0011-decimation.md) describes. The
consequence is a layer that shades from faded to full and asks the reader to
hover.

A raster does not have that problem. Its values are the data's, known in
`Train`, so `config.colorGuide` returns a guide with nothing added and the
reader gets a labelled bar in the units they measured. This is worth stating
because it closes a question rather than opening one: the hexbin's missing
colourbar stays missing, and the mark that *can* have one now exists beside it.

### The image is built at the panel's resolution, by the geom

The tempting version is to build one pixel per cell and let `Backend.Image`
scale to fit. It is rejected, because the backends do not agree about what
scaling means: a smooth upscale invents colours between two measurements, and
a chart that draws a value nobody recorded is the failure this library refuses
everywhere else — `geom.Contour` ends a run at a hole rather than routing round
it, and `data.Column.Nulls` exists so that an absent reading is not a zero.

So the geom expands the lattice into the panel's own pixels with **nearest
neighbour**, which costs one pass over the panel and makes every backend draw
the same picture. `stat.Grid.Raster`'s buffer-reuse discipline carries over
unchanged.

**Downsampling is named rather than assumed.** A lattice finer than the panel
has to drop something, and which thing depends on the field: a spectrum wants
the peak to survive, a temperature map wants the mean. `geom.Resample` takes a
named member of a closed family — `geom.Nearest`, `geom.Mean`, `geom.Max` —
which is the rule three records have now settled
([0041](0041-qq-plots.md), [0050](0050-locus-annotations.md),
[0052](0052-probability-scales.md)): a named member of a small closed family
serialises through `spec`, and an arbitrary Go function does not.

The default is `Nearest` in both directions, because it is the only one that
never shows a number that was not measured. It is also the one that aliases a
noisy spectrum, which is why `Max` exists and why the doc comment for a
spectrogram says to pass it.

### An unequally spaced lattice is refused, not approximated

An image is a grid of equal cells. A lattice whose x positions are unequal —
samples at 1 s, 2 s, then 10 s — cannot be blitted without either stretching
the wrong cells or inventing the missing ones.

`geom.Rect` draws that table correctly today, one box per row, and at the size
where the spacing is that irregular there are not a million rows. So the raster
reports a fault and names `Rect`, in the shape `stat.Lattice` already reports
its own: a code the geom turns into a message naming the column.

**A non-finite cell is transparent**, matching the density raster's empty cell
and [ADR 0064](0064-a-contour-and-its-lattice.md)'s hole: the panel's
background reads through, and a reader sees that nothing was measured there
rather than seeing the bottom of the ramp.

### Decimation does not apply

[ADR 0011](0011-decimation.md) reduces marks that would land on the same pixel
column. A raster's cell count *is* its resolution, and the reduction that
matters is `Resample` above, chosen in data terms rather than in device ones.
`geom.Decimate` on a raster layer is an error.

## Consequences

| | |
|---|---|
| `ir`, `render`, `layout`, `coord` | unchanged — `Backend.Image` is already in the interface and every backend implements it |
| `geom` | one mark, one option (`Resample`) with three named members, one fault message; `stat.Lattice` and `config.colorGuide` reused |
| `stat` | a resample helper beside `Grid.Raster`; no new layout |
| `spec` | a `"raster"` mark and a `resample` string |
| `interact` | a hit inverts the position through the lattice and reports the cell's value — the hexbin's shape, over data the geom kept |
| `a11y` | a cell is a `(x, y, v)` reading; the image is a drawing detail and the data channel does not mention it |
| Charts unlocked | spectrogram and waterfall, Hovmöller diagram, recurrence plot, dense heatmap of a measured field, thermal or line-scan sensor frame, occupancy and correlation matrices at size, and the backdrop a contour is drawn over |

## Amendment — what the implementation settled

`geom.Raster` is in `geom/raster.go`, `stat.Step`, `stat.Block`,
`stat.Lattice.Mean` and `stat.Lattice.Max` are the arithmetic beside
`Grid.Raster`, and `examples/spectrogram` is the chart the mark exists for:
1322 frames by 128 bins, one `<image>` in the SVG where a `Rect` per cell would
be 169,216 paths. Nine things the record did not settle:

**The same faults with the same message is one function, not a promise.** The
record asked that a raster and a contour refuse a table identically.
`geom.latticeError` is the switch over `stat.LatticeFault` that both call, and
the mark's name is its argument — so a caller who fixed the sentence a contour
gave them has fixed the raster drawn over it, and a test compares the two
sentences word for word. The three copies in `three` are left alone: they name
a *floor* axis, which is a different sentence about a different chart.

**The axes are trained to the cell edges, not to the samples.** A cell is
centred on its reading, so half of the first one lies below the first x. An
axis trained on the positions alone clips a half cell off each side of the
picture, which is the one part of a measured field a reader is most likely to
be looking at. Where a scale has no position for an edge — a log axis whose
first cell edge crosses zero — the sample stands.

**Nearest is one cell rather than a reduction, so `stat` gained two of them
and not three.** What the family chooses is how wide a pixel's block of cells
is and then what to do with it: `Nearest` narrows the span to the cell the
pixel's centre falls in and reads `Lattice.Value`, which was already there, and
only `Mean` and `Max` have arithmetic to do. The two agree at the boundary — a
block of one cell reduced either way is that cell, NaN included — so the three
cases are one loop with one branch.

**The pixel-to-cell mapping is inverted through the scale, per pixel.** The
record says nearest neighbour and one pass over the panel; doing that by
inverting the axis rather than by interpolating between the lattice's ends
costs the same and buys a property worth naming: a non-linear axis places the
cells where it places everything else, so a log frequency axis gives the low
bins more pixels than the high ones. The spans are resolved once per pixel
column and once per pixel row, so the cost of the mark is the panel's size and
not the field's — a recurrence plot of ten thousand samples is a hundred
million cells and still a few hundred thousand pixels.

**A row is reported per cell only while a cell is at least a pixel across.**
The record said a hit inverts the position through the lattice and reports the
cell's value. It does, through `geom.Rows`, one position per cell — but only
above that threshold. Below it the pixel is a reduction over several cells, and
under `Mean` its value is not any one of them, so there is no single row behind
it and naming one would name whichever cell the rounding reached. That is the
answer a hexbin's cell and the density raster already give, and the hit still
carries the position read back through the axes, which is the reading a field
has at a point.

**A non-Cartesian coord is an error rather than a wrong picture.** The record
files a polar raster under "Revisit if", which leaves open what the mark does
when it meets one today. It returns `ErrWarpedRaster` from `Build`: a blit is
axis-aligned in device space, and an image drawn square in a round panel is not
a degraded chart but a false one.

**A raster with no `ColorBy` still has a ramp and still has a bar.** The
record's examples all name one. Naming none takes `palette.DefaultRamp` over
the field's own extent, because a raster painted in one colour is not a
degraded raster — it is nothing at all — and the mark whose whole argument is
that it *can* have a labelled colourbar should not need to be asked for one.

**A ramp read per pixel is the mark's whole cost, so it is read into a table
first.** This is the one thing the record got materially wrong: "one pass over
the panel" is cheap only if a pixel is cheap, and `palette.Lerp` blends in
linear light — three transfer functions in and three out — which measured at
640 ns a colour, or 150 ms for a 600 by 400 panel. Every other mark asks for a
handful of colours and none of them noticed. `geom.ink` reads the scale once
per domain: one colour per class for a classed scale, taken at the middle of
the class exactly as `render`'s classed bar takes it so that the bar and the
picture cannot disagree, and 1024 samples along the ramp for a continuous one,
which is `render`'s own `colorbarStops` argument at a finer grain. A classed
scale is not sampled at all, for the reason `render` draws its bar in bands
rather than as a gradient: a class boundary is an edge, and an edge is what
sampling steps over. The panel above costs 3.3 ms and no allocations.

**The guide is the contour's `ColorGuide`, not `config.colorGuide`.** The
consequences table above says the shared one is reused, and it is not: that
helper answers only for a layer that resolved a *colour column* through a
scale, and a raster's quantity is `Z`. It is the same divergence `geom.Contour`
already has for the same reason, and the same four lines — which is the smaller
of the two costs. Widening the shared helper to mean "a layer that colours by a
number it has" would make every mark that does not answer it explain why.

## Not in scope

- **Interpolating a scattered field onto a grid.** Kriging, IDW and natural
  neighbour are estimators with their own failure modes, and the estimate is a
  claim about the world rather than a way of drawing one. A caller that has
  scattered points and wants a field computes it and hands over a lattice.
- **Smooth (bilinear) upscaling**, per the argument above. If it is ever wanted
  it is a fourth named member, not a default.
- **An image as data.** A caller with an `image.Image` already — a camera
  frame, a map tile — is asking for a backdrop, not a mark, and that is the
  overlay layer's territory ([ADR 0046](0046-overlay-layer.md)) or the caller's
  own `Backend.Image` call.
- **Per-cell borders.** A grid drawn over a raster is furniture the panel
  already has, and a stroke per cell is the million primitives this record
  exists to avoid.

## Revisit if

- A raster is wanted under a non-Cartesian coord. A blit is axis-aligned in
  device space, so a polar or ternary raster is a different mark — one
  quadrilateral per cell through the coordinate stage, which is what bucket J
  already calls a ternary density.
- Two layers want the same lattice. Today each resolves its own; a shared
  resolve is a cache keyed by the source and the three column names, and it is
  only worth it once a raster and its contours are drawn together often enough
  to measure.

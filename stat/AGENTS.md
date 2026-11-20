# Working in `stat`

Notes for changing `stat`, moved here from the root [AGENTS.md](../AGENTS.md),
which holds the rules that apply everywhere — read that one first. This file
covers stress layout, evaluation grids, no sorting, histograms and the
hexagonal lattice.

## Things that will bite

**A stress layout's row order decides which minimum it reaches, and that is a
bigger number than it sounds.** `stat.Stress` descends a non-convex objective,
and both the arrangement it starts from and the order a sweep visits the nodes
in come from the order the caller's rows interned them. So relabelling the same
edges is not a relabelling of the same picture: the collaboration graph in
`docs/images/network.png` settles at a stress of 3.47 in the order its rows
arrive in and at 2.30 under one relabelling — a third lower, from the same code
at the same budget. Picking the start rows more cleverly does not fix it; that
was measured and only moves which numbering is the unlucky one
([ADR 0077](../docs/adr/0077-a-node-link-layout.md)'s second amendment has the
table). And **`geom.Order` is not the way to ask for another one**: it reaches
the grouped marks and the set charts, `geom/relational.go`'s edge reader never
looks at it, and a `NodeLink` given one draws exactly what it drew. A caller
who wants a different node order re-orders the rows of the table. `stat.StressSweeps` is a *budget* and not a convergence point — about
four percent of stress is still on the table at fifty sweeps for a 127-node
tree — and a monotone descent on the stress is not a monotone improvement in
legibility, because nothing here counts an overlap or a crossing.

**A stat evaluated on a grid pins the grid's ends, and never truncates a
kernel.** Both halves of that are the same bug, and it shipped for exactly one
CI run — green on amd64, red on macOS. `lo + float64(i)*step` is a multiply and
an add, which Go may contract into an FMA "possibly across statements"; arm64
takes it and amd64 does not, so a grid of sixty-one points over [-3, 3] ended
at 3 on one machine and 3.0000000000000004 on the other. That is one ulp, and
one ulp is invisible — until it lands on the far side of a *decision*. `AppendKDE`
skipped any sample past four bandwidths, so the endpoint's kernel was summed on
one architecture and dropped on the other, and a density that was symmetric on
one was not on the other.

`stat.gridAt` now hands back the ends exactly rather than computing them, which
is the same "do not compute what is known" fix `scale.place` carries for the
same arithmetic; `AppendKDE` and `AppendLoess` both use it. And the kernel is
summed in full: a cutoff is a discontinuity, so the estimate depended on the
last bit of its argument, and it was not buying an order of growth either — the
loop visits every row either way and the cutoff saved an `exp`.
`TestADensityHasNoCliffInItsTail` and `TestAnEvaluationGridEndsExactlyOnItsBounds`
are what hold both, and the first fails on *every* architecture, which is the
point: a property that only one machine can check is a property nobody checks.

**`stat` does no sorting, and the geoms keep the buffers.** `Quantile`, `ECDF`
and `Loess` take ordered columns; `Silverman` takes two spread measures rather
than computing an IQR. Sorting inside a stat means either mutating the caller's
column or allocating a copy of it per frame, and a chart redrawn every frame
already keeps a buffer. Every distribution geom therefore carries its own sort
buffer on the layer, for the reason `barGeom.gaps` does: `Train` runs outside a
`Build`, where there is no scratch to take.

**`stat.Bin` is the 1-D histogram now.** The 2-D binner that used to be called
`Bin` is `stat.BinGrid`, beside `stat.BinHex`. The three are named after what
they fill, and "bin" without a qualifier means the histogram.

**A hexagonal lattice is not d3-hexbin's.** `Hex.Cell` follows its structure and
corrects its metric: both work in lattice coordinates where a column step is 1
and a row step is 1, but on screen those are √3·r and 1.5·r, so comparing
`px² + py²` there mixes units and misplaces points near the interlocking band.
The vertical term carries `(dy/dx)² = 3/4`, and
`TestEveryHexPointLandsInItsNearestCell` checks the defining property directly.
Without it the picture grows seams along every second row.

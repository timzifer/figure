# The gallery: layout, axes and scale

One chart split into panels or placed on a grid, axes broken or folded, and data far larger than the pixels it lands on.

Every figure here is rendered by
[`backend/gg/cmd/gallery`](../../backend/gg/cmd/gallery) and re-checked in CI, so a
picture here cannot drift away from the code that produced it.

[← The gallery](../gallery.md) · [Lines, points and bars](basics.md) · [Distributions](distributions.md) · [Parts of a whole](parts.md) · [Flows, networks and sets](relations.md) · [Coordinates and projections](coordinates.md) · [Markets](markets.md) · **Layout, axes and scale** · [Without colour](colour.md)

| | |
|---|---|
| ![Throughput faceted into one panel per region](../images/facets.png) | ![Four subplots on one dark canvas](../images/subplots.png) |
| ![A quarter of a million samples drawn as a clean line](../images/decimation.png) | ![A million points drawn as a density raster](../images/density.png) |
| ![Requests per second by site as bars on an axis broken between 12 and 85, marked with a double slash, so the one site thirty times the rest and the five small ones all read](../images/axis-break.png) | ![The same bars with the break marked by two parallel zigzags across the panel, the tall bar cut where they run](../images/axis-break-zigzag.png) |
| ![A machine's state log over two shifts with every idle period folded off the time axis, each fold marked by a small slash on the axis line](../images/machine-folds.png) |  |

---

**[README](../../README.md)** · **[CONCEPT](../../CONCEPT.md)** · **[ADRs](../adr)** · [The gallery](../gallery.md) · [Chart forms](../charts.md) · [Interaction](../interaction.md) · [A million rows](../scale-out.md) · [Reading a chart](../reading.md) · [JSON and Arrow](../spec.md) · [Features](../features.md) · [Chart-type catalogue](../chart-types.md) · [Benchmarks](../benchmarks.md) · [How it was built](../milestones.md)

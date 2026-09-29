# The gallery: distributions

How a sample is spread: binned, summarised, smoothed, or every observation placed.

Every figure here is rendered by
[`backend/gg/cmd/gallery`](../../backend/gg/cmd/gallery) and re-checked in CI, so a
picture here cannot drift away from the code that produced it.

[← The gallery](../gallery.md) · [Lines, points and bars](basics.md) · **Distributions** · [Parts of a whole](parts.md) · [Flows, networks and sets](relations.md) · [Coordinates and projections](coordinates.md) · [Markets](markets.md) · [Layout, axes and scale](scale.md) · [Without colour](colour.md)

| | |
|---|---|
| ![Request latency as a histogram](../images/histogram.png) | ![Latency distributions as boxplots](../images/boxplot.png) |
| ![Latency by service as violins, one per region within each service](../images/violin.png) | ![A year of daily maxima as a ridgeline, one density per month](../images/ridgeline.png) |
| ![Scores by cohort as a beeswarm, every observation placed](../images/beeswarm.png) | ![Scores by cohort as three empirical CDFs on one axis](../images/ecdf.png) |
| ![Ordered observations against normal quantiles](../images/qq.png) | ![Fifty thousand observations binned into hexagons with a loess trend through them](../images/hexbin.png) |

---

**[README](../../README.md)** · **[CONCEPT](../../CONCEPT.md)** · **[ADRs](../adr)** · [The gallery](../gallery.md) · [Chart forms](../charts.md) · [Interaction](../interaction.md) · [A million rows](../scale-out.md) · [Reading a chart](../reading.md) · [JSON and Arrow](../spec.md) · [Features](../features.md) · [Chart-type catalogue](../chart-types.md) · [Benchmarks](../benchmarks.md) · [How it was built](../milestones.md)

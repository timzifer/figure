# The gallery

Every figure the library draws, rendered from the code that draws it.

Every figure here is rendered by
[`backend/gg/cmd/gallery`](../backend/gg/cmd/gallery) and re-checked in CI, so a
picture here cannot drift away from the code that produced it.

| | |
|---|---|
| ![Three series with a legend](images/series.png) | ![Two groups of scattered points](images/scatter.png) |
| ![A response time histogram](images/bars.png) | ![A damped sine on a time axis](images/signal.png) |
| ![An estimate with a shaded interval](images/area.png) | ![A step chart of replica counts](images/steps.png) |
| ![Bars by region, coloured by value with a colourbar](images/categories.png) | ![Latency distributions as boxplots](images/boxplot.png) |
| ![Two growth curves on a log axis](images/logscale.png) | ![A series read against thresholds and a shaded window](images/annotations.png) |
| ![Throughput faceted into one panel per region](images/facets.png) | ![Four subplots on one dark canvas](images/subplots.png) |
| ![A quarter of a million samples drawn as a clean line](images/decimation.png) | ![A million points drawn as a density raster](images/density.png) |
| ![Standard error curves labelled with typeset notation](images/notation.png) | ![Revenue stacked by product, one layer over a long table](images/stacked.png) |
| ![Traffic by channel as a streamgraph](images/stream.png) | ![Calls per hour as a heatmap of coloured cells](images/heatmap.png) |
| ![Browser share as a donut](images/pie.png) | ![Two designs compared on five axes as a radar chart](images/radar.png) |
| ![Spend by team as a donut whose slices reach as far as each team used of its budget, with the team that went over broken out of the ring](images/donut.png) | ![Request latency as a histogram](images/histogram.png) |
| ![Latency by service as violins, one per region within each service](images/violin.png) | ![A year of daily maxima as a ridgeline, one density per month](images/ridgeline.png) |
| ![Scores by cohort as a beeswarm, every observation placed](images/beeswarm.png) | ![Scores by cohort as three empirical CDFs on one axis](images/ecdf.png) |
| ![Fifty thousand observations binned into hexagons with a loess trend through them](images/hexbin.png) | ![Income against life expectancy as bubbles sized by population, with a size key beside the legend](images/bubbles.png) |
| ![Mean latency per service with a 95 % interval drawn over each bar](images/errorbars.png) | ![Revenue as bars against a left axis and margin as a percentage line against a right one](images/twoaxes.png) |
| ![An oven temperature curve read against elapsed minutes along the bottom and cycle number along the top](images/twoextents.png) | ![A patch antenna's reflection swept across its band, on a Smith chart](images/smith.png) |
| ![Disk usage by directory as a treemap, one rectangle per file sized by its share](images/treemap.png) | ![The same directory tree as a sunburst, the root at the middle and the files at the rim](images/sunburst.png) |
| ![Requests per second through a service, drawn as a sankey diagram](images/sankey.png) | ![The same traffic as an arc diagram, each service a segment of the rail and each route a band arcing over it](images/arc.png) |
| ![The same traffic again as a chord diagram, each service an arc and each route a ribbon crossing the disc](images/chord.png) | |

---

**[README](../README.md)** · **[CONCEPT](../CONCEPT.md)** · **[ADRs](adr)** · [Chart forms](charts.md) · [Interaction](interaction.md) · [A million rows](scale-out.md) · [Reading a chart](reading.md) · [JSON and Arrow](spec.md) · [Features](features.md) · [Chart-type catalogue](chart-types.md) · [Benchmarks](benchmarks.md) · [How it was built](milestones.md)

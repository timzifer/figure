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
| ![The same traffic again as a chord diagram, each service an arc and each route a ribbon crossing the disc](images/chord.png) | ![A response surface drawn from three cameras at once: a three-quarter view, a plan and a front elevation of one scene](images/surface.png) |
| ![Thirty sweeps of a spectrum offset by sweep number, with a carrier drifting upward and a harmonic growing behind it](images/cascade.png) | ![An open loop at two gains on a Nichols diagram, drawn over the contours of constant closed-loop gain and phase](images/nichols.png) |
| ![A rebuild plan as a gantt chart: bars part filled by how far each task has got, arrows between them with the critical path in orange, milestone diamonds and a dashed line at today](images/gantt.png) | ![The same stacked revenue chart under redundant encoding: a different hatch pattern over each product, so the stack reads without colour](images/hatched.png) |
| ![The same stacked revenue chart with the colour taken out: three products as three greys, two of them almost the same one](images/greyscale-plain.png) | ![The same greyscale chart with redundant encoding on: the greys are unchanged and each product carries a pattern, so the stack reads again](images/greyscale-hatched.png) |
| ![Traffic by channel as stacked areas, each band hatched and its crest marked with points](images/hatched-area.png) | ![The fourteen hatch patterns as fourteen labelled samples: lines, dots, wavering lines and tilings](images/hatch-kinds.png) |
| ![Customers by which products they subscribe to, as an UpSet plot: a bar per combination over a matrix of dots saying which products that combination is](images/upset.png) | ![The same three products as a Venn diagram, each region carrying the customers who have exactly those products](images/venn.png) |
| ![Sixty cars as a parallel-coordinates plot: four vertical axes in four different units, one line per car crossing all of them, coloured by origin](images/parallel.png) | ![A quarter of support tickets as a parallel-sets diagram: three columns of category boxes for channel, urgency and outcome, with ribbons between them as thick as the tickets that answer both that way, split into the solved and escalated share](images/parallelsets.png) |
| ![Who worked with whom over a quarter, as a node-link diagram: three teams as three clusters, one person joining all of them, and a pair working on their own off to the side](images/network.png) | ![Last month's rainfall over twenty gauges, as the panel divided into the part nearest each gauge and filled with that gauge's reading, wettest in the north-west](images/voronoi.png) |
| ![Mean July cloud cover over the whole world on an equal-area projection: the globe as an ellipse, one coloured cell per ten degrees, cloudy along the equator and over the midlatitude oceans and clear through the subtropics](images/map.png) | ![Edinburgh to Tokyo on a globe seen from over the pole: the great circle running close to the ice and the rhumb line bowing far south of it, with the graticule and the rim of the visible hemisphere](images/globe.png) |
| ![Requests per second by site as bars on an axis broken between 12 and 85, marked with a double slash, so the one site thirty times the rest and the five small ones all read](images/axis-break.png) | ![The same bars with the break marked by two parallel zigzags across the panel, the tall bar cut where they run](images/axis-break-zigzag.png) |
| ![A machine's state log over two shifts with every idle period folded off the time axis, each fold marked by a small slash on the axis line](images/machine-folds.png) | |

---

**[README](../README.md)** · **[CONCEPT](../CONCEPT.md)** · **[ADRs](adr)** · [Chart forms](charts.md) · [Interaction](interaction.md) · [A million rows](scale-out.md) · [Reading a chart](reading.md) · [JSON and Arrow](spec.md) · [Features](features.md) · [Chart-type catalogue](chart-types.md) · [Benchmarks](benchmarks.md) · [How it was built](milestones.md)

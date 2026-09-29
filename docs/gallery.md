# The gallery

Every figure the library draws, rendered from the code that draws it, in
eight groups, 63 figures in all. Each picture below opens its group.

Every figure here is rendered by
[`backend/gg/cmd/gallery`](../backend/gg/cmd/gallery) and re-checked in CI, so a
picture here cannot drift away from the code that produced it.

| | |
|---|---|
| [![Three series with a legend](images/series.png)](gallery/basics.md)<br>**[Lines, points and bars](gallery/basics.md)** · 17 figures<br>The everyday forms: series over a continuous or time axis, scatter, bars, areas and steps, with intervals, thresholds, second axes and typeset labels. | [![Latency by service as violins, one per region within each service](images/violin.png)](gallery/distributions.md)<br>**[Distributions](gallery/distributions.md)** · 8 figures<br>How a sample is spread: binned, summarised, smoothed, or every observation placed. |
| [![Disk usage by directory as a treemap, one rectangle per file sized by its share](images/treemap.png)](gallery/parts.md)<br>**[Parts of a whole](gallery/parts.md)** · 6 figures<br>One total split into its shares, flat, stacked over time or nested. | [![The same traffic again as a chord diagram, each service an arc and each route a ribbon crossing the disc](images/chord.png)](gallery/relations.md)<br>**[Flows, networks and sets](gallery/relations.md)** · 8 figures<br>What connects to what: flows between stages, links between people, overlaps between sets and one line per record across many axes. |
| [![A patch antenna's reflection swept across its band, on a Smith chart](images/smith.png)](gallery/coordinates.md)<br>**[Coordinates and projections](gallery/coordinates.md)** · 8 figures<br>Charts whose frame is not a plain rectangle: polar, Smith and Nichols charts, map projections, the plane cut into cells and a 3-D scene from several cameras. | [![A daily price chart as a trading screen draws it: candles with weekends and a holiday folded out of the time axis, a Bollinger band, two moving averages and their crossings as buy and sell markers, the last close tagged on the price axis, volume and RSI in tracks underneath and the volume at each price in a track beside](images/market.png)](gallery/markets.md)<br>**[Markets](gallery/markets.md)** · 4 figures<br>What a trading screen draws: candles and OHLC bars on a calendar that folds out the closed days, indicators, volume tracks, the last price on the axis, and a live chart with a crosshair. |
| [![Throughput faceted into one panel per region](images/facets.png)](gallery/scale.md)<br>**[Layout, axes and scale](gallery/scale.md)** · 7 figures<br>One chart split into panels or placed on a grid, axes broken or folded, and data far larger than the pixels it lands on. | [![The same greyscale chart with redundant encoding on: the greys are unchanged and each product carries a pattern, so the stack reads again](images/greyscale-hatched.png)](gallery/colour.md)<br>**[Without colour](gallery/colour.md)** · 5 figures<br>Redundant encoding: hatch patterns that carry a series when the colour cannot, in print, in greyscale or for a reader who cannot tell the hues apart. |

---

**[README](../README.md)** · **[CONCEPT](../CONCEPT.md)** · **[ADRs](adr)** · [Chart forms](charts.md) · [Interaction](interaction.md) · [A million rows](scale-out.md) · [Reading a chart](reading.md) · [JSON and Arrow](spec.md) · [Features](features.md) · [Chart-type catalogue](chart-types.md) · [Benchmarks](benchmarks.md) · [How it was built](milestones.md)

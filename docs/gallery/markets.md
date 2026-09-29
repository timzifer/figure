# The gallery: markets

What a trading screen draws: candles and OHLC bars on a calendar that folds out the closed days, indicators, volume tracks, the last price on the axis, and a live chart with a crosshair.

Every figure here is rendered by
[`backend/gg/cmd/gallery`](../../backend/gg/cmd/gallery) and re-checked in CI, so a
picture here cannot drift away from the code that produced it.

[← The gallery](../gallery.md) · [Lines, points and bars](basics.md) · [Distributions](distributions.md) · [Parts of a whole](parts.md) · [Flows, networks and sets](relations.md) · [Coordinates and projections](coordinates.md) · **Markets** · [Layout, axes and scale](scale.md) · [Without colour](colour.md)

| | |
|---|---|
| ![A daily price chart as a trading screen draws it: candles with weekends and a holiday folded out of the time axis, a Bollinger band, two moving averages and their crossings as buy and sell markers, the last close tagged on the price axis, volume and RSI in tracks underneath and the volume at each price in a track beside](../images/market.png) | ![The frame a live chart ends on: one-minute candles zoomed into the last half hour, the price axis fitted to what is in view, the last price tagged in its direction and a crosshair snapped to a candle with its time and price tagged on the axes](../images/market-live.png) |
| ![The last thirty trading days as OHLC bars, a rule from low to high with the open ticked left and the close right, coloured by the close against the previous close](../images/market-bars.png) | ![The same thirty days as candles under redundant encoding: rising bodies drawn as outlines and falling bodies filled, so direction reads without colour](../images/market-hollow.png) |

---

**[README](../../README.md)** · **[CONCEPT](../../CONCEPT.md)** · **[ADRs](../adr)** · [The gallery](../gallery.md) · [Chart forms](../charts.md) · [Interaction](../interaction.md) · [A million rows](../scale-out.md) · [Reading a chart](../reading.md) · [JSON and Arrow](../spec.md) · [Features](../features.md) · [Chart-type catalogue](../chart-types.md) · [Benchmarks](../benchmarks.md) · [How it was built](../milestones.md)

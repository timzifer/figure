# The gallery: flows, networks and sets

What connects to what: flows between stages, links between people, overlaps between sets and one line per record across many axes.

Every figure here is rendered by
[`backend/gg/cmd/gallery`](../../backend/gg/cmd/gallery) and re-checked in CI, so a
picture here cannot drift away from the code that produced it.

[← The gallery](../gallery.md) · [Lines, points and bars](basics.md) · [Distributions](distributions.md) · [Parts of a whole](parts.md) · **Flows, networks and sets** · [Coordinates and projections](coordinates.md) · [Markets](markets.md) · [Layout, axes and scale](scale.md) · [Without colour](colour.md)

| | |
|---|---|
| ![Requests per second through a service, drawn as a sankey diagram](../images/sankey.png) | ![The same traffic as an arc diagram, each service a segment of the rail and each route a band arcing over it](../images/arc.png) |
| ![The same traffic again as a chord diagram, each service an arc and each route a ribbon crossing the disc](../images/chord.png) | ![Who worked with whom over a quarter, as a node-link diagram: three teams as three clusters, one person joining all of them, and a pair working on their own off to the side](../images/network.png) |
| ![Customers by which products they subscribe to, as an UpSet plot: a bar per combination over a matrix of dots saying which products that combination is](../images/upset.png) | ![The same three products as a Venn diagram, each region carrying the customers who have exactly those products](../images/venn.png) |
| ![Sixty cars as a parallel-coordinates plot: four vertical axes in four different units, one line per car crossing all of them, coloured by origin](../images/parallel.png) | ![A quarter of support tickets as a parallel-sets diagram: three columns of category boxes for channel, urgency and outcome, with ribbons between them as thick as the tickets that answer both that way, split into the solved and escalated share](../images/parallelsets.png) |

---

**[README](../../README.md)** · **[CONCEPT](../../CONCEPT.md)** · **[ADRs](../adr)** · [The gallery](../gallery.md) · [Chart forms](../charts.md) · [Interaction](../interaction.md) · [A million rows](../scale-out.md) · [Reading a chart](../reading.md) · [JSON and Arrow](../spec.md) · [Features](../features.md) · [Chart-type catalogue](../chart-types.md) · [Benchmarks](../benchmarks.md) · [How it was built](../milestones.md)

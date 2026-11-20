# Working in `render` and `internal/layout`

Notes for changing `render` and `internal/layout`, moved here from the root
[AGENTS.md](../AGENTS.md), which holds the rules that apply everywhere — read
that one first. This file covers shared scale objects across panels, tracks,
the layout's row arithmetic, second axes, guides, watched and resized renders.

## Things that will bite

**Panels sharing an axis share the scale object.** So the device range is set
per panel, twice: once before the furniture pass and once before the data pass.
Dropping the second call leaves every panel but the last drawing its data where
the last panel's axis is. There is a test.

**A track shares the panel's scale object, and that is the whole feature.**
`Plot.tracked` gives every track panel the chart's own scale for the axis it
runs along — `c.X` for a bottom or top band, `c.Y` for a left or right one. A zoom reaches a scale
through `scale.Zoomer.SetDomain`, so one object means the panel and its bands
move together by construction rather than by two handlers agreeing. Cloning it
there — which is what `freeScale` does for a *free facet axis*, and looks like
the tidier thing to do — compiles, draws a chart that looks right, and breaks
every acceptance criterion the feature has. The same trap is why
`scale.Cloner` and `scale.Snapshotter` are opposites; see
[ADR 0031](../docs/adr/0031-tracks.md). A track's own Y is unzoomable for free,
because `zoomAxis` no-ops on a scale that is not a `Zoomer` and `scale.Ordinal`
deliberately is not one. There is a test for each half.

**A fixed row's arithmetic is written to be bit-identical when nothing is
fixed.** `layout.rowHeights` divides once and multiplies once rather than
summing, because a float32 sum of n equal terms is not always their product,
and the guide column is measured against that total. Rewriting it as the
obvious loop moves every golden file in the repository by an ulp — which the
structural comparison will *not* catch, because it tolerates exactly that.

**A watched render is serial, on purpose.** `render.Chart.Observer` forces the
serial path in `concurrent`. The observer is told which layer is drawing so a
caller can attribute the calls that follow; two panels drawing at once have no
order to be told in. Do not "optimise" this by giving each panel its own
observer — the index would then depend on scheduling, and so would every
tooltip.

**A second axis is a scale on the chart and a binding on the layer, and five
places had to learn it.** `render.Panel.axesOf` asks the layer through
`geom.Describe` — not a method on `Geom`, which never gains one — so the chart
and the document agree by construction
([ADR 0037](../docs/adr/0037-secondary-axis.md)). The two directions are
independent: a layer may name `OnX2` and `OnY2` together. Four consequences are
load-bearing. `Panel.setRange` frames the coord against the **secondary** pair as
well as the primary one, or the layers on a second axis map through a scale
with no device range. `coord.Opposite` is an optional interface and `Polar` deliberately does
not implement it — a ring has no far side — so a chart that names a second axis
under a polar coord simply does not draw one. The second axis draws **no grid
lines**, and there is a test comparing the count against a one-axis chart:
two ladders of rules at different values are a moiré, and which scale a line
belongs to is unanswerable by looking. And the scales on `render.LayerInfo` are what make a
hit report the right number — an index that inverted every mark through the
panel's own scales would name 4200 on a chart whose right axis reads 12 %,
which is wrong by a *unit* on the feature whose whole purpose is that the units
differ. The layout's right and top gutters are zero for a column or row with no
second axis, which is what leaves every golden file unchanged; the guide column
is anchored past the right one, because the width was already reserved and an
anchor on the panel edge puts a legend on top of the labels. A facet's strip
sits **outside** the top gutter: a strip between a panel and its own tick
labels reads as though it named the axis.

Steering has to move both. `Live.Wheel`, `ZoomTo`, `PanBy` and `Autoscale` all
reach `Panel.Y2` as well as `Panel.Y`, or the two series slide apart under the
reader's hand — a chart with two axes is one chart, so a zoom is one zoom.
`interact.Panel` learns about the second axis from the scales on
`render.LayerInfo` rather than from `Observer.Panel`, which carries the panel's
own two.

**A typeset label is measured by laying it out.** `render`'s `mathBackend`
answers `Measure` from the typesetter and `Text` from the same layout, so the
margin a title is given is the width the title turns out to have. Measuring the
markup and drawing the notation would leave a fraction hanging out of the
canvas, and nothing would fail.

**A resized frame is not comparable with the last one.** `Live.Resize` clears
`drawn`, because every coordinate moved and there is no damage to compute. It
also keeps whatever the scales were zoomed to, on purpose: a reader who dragged
a view into place has not asked to leave it.

**A size scale's range is set by `render`, not by the geom.** A geom cannot set
it in `Build` — panels are built on separate goroutines and a shared scale
written from two of them is a data race with no `Snapshotter` to fix it — and
`Geom.Train` has no theme. So `sizeGuides` sets it once per scale on the serial
path, before the guides are measured and before any mark reads it. A range the
caller pinned with `scale.SizeRange` outranks it, exactly as a pinned domain
outranks training.

**The guide column is one list, and a fourth kind must not make it four.**
`layout.Guide` carries what the solver needs of any guide and `GridResult.Guides`
is the boxes, parallel and in order. It was generalised once rather than
extending it twice — see
[ADR 0027](../docs/adr/0027-size-channel-and-the-guide-column.md). Adding a kind is
a `GuideKind` constant, a measuring function and a drawing function; adding a
field to `Grid` is the thing that arrangement exists to refuse.

**Watching a render is serial, and animating a watched one is too.** An
`Observer` or a `RowSink` takes the serial path in `render/parallel.go`, so a
chart that is being hovered does not build its panels concurrently. That is a
property of watching, not of animating, but it is where someone will notice it.

# 0093 — Several symbols are a facet whose panels carry their own tracks, and a comparison divides each series by its value where the view begins

**Status:** Proposed · **Date:** 2026-09-29 · **Revisits:** `ErrTrackWithFacet`, and [ADR 0031](0031-tracks.md)

## Context

Everything [ADR 0085](0085-what-a-market-chart-needs.md) ranked is built, and
all of it is about one symbol. The two questions a screen with more than one
asks are not answerable yet.

**Several symbols, each with its volume.** A watchlist of four stocks is four
candle panels, each with a volume track under it, on one time axis, zoomed
together. The library has both halves and refuses the pair:

- `Plot.Facet` splits one plot by a column — the symbol — into panels that can
  share or free their axes, and a `Live` over a faceted plot zooms them
  together through the shared X scale object.
- `Plot.Track` gives a panel a band on a shared axis — the volume.
- A plot with both is `ErrTrackWithFacet`: *"A facet owns the grid's rows and
  columns … a track needs a row of that grid to live in. A band spanning a
  facet is a different feature with different questions to answer, so this is
  refused rather than guessed at."*

The refusal was right about the feature it named. A band *spanning* the facet
— one volume strip under all four symbols — is a different feature. But that
is not the one asked for. A watchlist wants the track **repeated**: each facet
panel with its own, exactly as each facet panel has its own copy of the
layers. The questions the refusal worried about are the ones repetition
answers already.

`Grid` is the other multi-panel tool, and it does not help: a grid takes its
members' layers and scales and drops their tracks, and it has no `Live`
(*"Interaction on stacked plots is what a track inside one plot is for"*).

**A comparison.** Four stocks on one axis are unreadable in their own units —
a share at 2,000 and one at 20 draw one line and one floor. Every platform's
comparison chart draws each series as its change since a common starting
point, in percent, and the starting point is the left edge of the view: pan to
last March, and every line starts at 0 % in last March. Nothing in the library
rebases a series, and because the starting point moves with the view, it
cannot be a column the caller computes once.

## Decision

**A track on a faceted plot is repeated in every facet panel, its layers
subset by the facet like the panel's own. A value layer can be drawn relative
— divided by its value where the view begins — so a comparison is a line per
symbol on one fitted axis written in percent.** Six claims.

### 1. A track is repeated per facet panel

```go
p := figure.New()
p.X(scale.Time(...)).Y(scale.Linear(scale.FitView()))
p.Add(geom.Candle(quotes, geom.X("start"), geom.X2("end"), geom.OHLC("open", "high", "low", "close")))
p.Facet(facet.Wrap("symbol", facet.Columns(2), facet.FreeY()))
p.Track(figure.Bottom, figure.TrackFraction(0.2)).
	Add(geom.Bar(quotes, geom.X("start"), geom.Y("volume"), geom.DirectionBy("open", "close")))
```

With a facet, every track is laid out once per facet panel, on the same edge,
in the same order, and its layers are subset by the facet's column exactly as
the panel's layers are — through `Faceter`. The volume under the AAPL panel is
AAPL's volume. A track layer that is not a `Faceter` is refused with an error
naming it, as a panel layer that is not one already is.

`ErrTrackWithFacet` is retired. The band spanning a whole facet that it
anticipated stays unbuilt, and is what *Not in scope* names.

### 2. A track's scales follow the facet's

The axis a track shares with its panel is that panel's — the same scale object,
so under a shared X one zoom moves every symbol and every volume strip, and
under `facet.FreeX` each block keeps its own. The axis a track owns follows the
facet's freedom on that direction: a bottom track's own Y is cloned per panel
when the facet frees Y, so each symbol's volume has its own scale, and shared
otherwise, so volumes are comparable across symbols. The rule is one sentence
because it is the one a reader expects: a track is free where its panel is.

A fitting axis ([ADR 0089](0089-a-value-axis-fits-what-its-time-axis-shows.md))
fits per panel, which is what that record predicted for a facet with free axes:
each symbol's price axis fits its own candles in the shared zoomed view.

### 3. The layout is a block per facet panel, and the panel order is fixed

Each facet cell becomes a block of rows and columns — the data panel with its
tracks round it, as a single tracked plot is laid out today (`Plot.tracked`) —
and the facet's grid is a grid of blocks. The strip that names a facet panel
sits above its block, not between the panel and a top track. Blocks in one row
of the facet have the same tracks, because the tracks are the plot's, so their
rows line up by construction.

The panel order a `Hit` names and an `Observer` is told is: for each facet
panel in the facet's order, its data panel, then its tracks in the order they
were added. It is fixed here, as `Plot.tracked` fixed its own, because a
tooltip that knew "panel 2 is AAPL's volume" must not see it renumbered by a
symbol added to the data.

### 4. A value layer can be drawn relative to where the view begins

`geom.Relative()` draws each series as its change since its first value in
view: `y / y₀ − 1`, where `y₀` is the series' first finite value at or after
the start of the view, per group when the layer is grouped. The start of the
view is `Training.Within`'s lower end when the value axis fits a pinned X
([ADR 0089](0089-a-value-axis-fits-what-its-time-axis-shows.md)) and the first
row otherwise, so an unzoomed comparison starts every line at 0 % on the left,
and panning moves the zero with the view — the behaviour every platform's
comparison chart has, derived every frame by the retraining that already
happens ([ADR 0090](0090-an-axis-describes-the-frame-it-is-drawn-in.md)).

```go
p.Y(scale.Linear(scale.FitView(), scale.NumberFormat("+#.1%")))
p.Add(geom.Line(quotes, geom.X("t"), geom.Y("close"), geom.GroupBy("symbol"), geom.Relative()))
```

It is a layer option rather than a scale because the base is a property of each
series, not of the axis: four symbols on one axis have four bases, and an axis
that divided by one of them would draw three lines wrong. It is read by the
marks that draw a value — `Line`, `Step`, `Area`, `Scatter` — and by `Candle`,
whose four values are divided by one base, the series' first close in view. A
series whose base is zero or missing has no relative value and is not drawn.

### 5. The axis says it is relative

A relative layer's values are fractions; the axis writes them with
`scale.NumberFormat`'s percent style, which the caller chooses. A comparison is
read by its sign, so the spec gains one flag: a leading `+` writes positive
values with a plus — `"+#.1%"` is `+12.5%`, `-3.0%` and `0.0%` — which the
spec's grammar has had no way to say, and which a zero leaves unsigned because
no change has no direction. The record does not pick a default format,
because an axis cannot know that every layer on it is relative, and a
comparison with an absolute line on the same axis — an index level, a
benchmark — is a mistake the axis should not paper over.

`geom.LastValue` over a relative layer tags the last value relative, in the
axis's own format, so each symbol's line ends in a tag reading its change.
That is `LastValue` reading `Relative` like any other mark — a claim about
composition, and a test.

### 6. The dialect

A facet with tracks writes both, as each is written today; a document that had
both was an error and is now a chart. `"relative": true` is a mark property,
figure's own.

## Consequences

- A watchlist is one plot: a facet by symbol, a volume track, a shared time
  axis zoomed by one wheel and a price axis fitted per symbol.
- A comparison chart is one layer option and one number format, and its zero
  follows the view.
- `ErrTrackWithFacet` is retired; `geom` gains `Relative`; `NumberFormat`
  gains a `+` flag.
- Panel indices of a faceted plot with tracks are the block order of claim 3.

## Not in scope

- **A band spanning a facet** — one strip under all its panels. The feature
  `ErrTrackWithFacet` was reserving; nobody has asked for it, and repetition is
  what a watchlist wants.
- **`Grid` interaction.** A grid of plots of different kinds — a price beside a
  macro series — stays a static render. The multi-symbol case is one plot, and
  a grid's `Live` is a record of its own when a dashboard asks for one.
- **A relative scale.** Claim 4.
- **Rebasing on a chosen date** rather than on the view — "since the IPO". The
  caller divides once; it is a column, because it does not move.
- **Correlation and spread charts** between symbols. They are computed series,
  drawn with the marks that exist.

## Revisit if

- A dashboard needs linked zoom across plots of different kinds, which is
  `Grid` with a `Live`.
- A comparison needs log-relative change (`ln(y / y₀)`) often enough to be an
  option of `Relative` rather than a caller's column.

## Order of work

1. Facet × track: the block layout, the per-panel track subsets, the scale rule
   of claim 2 and the panel order of claim 3; `ErrTrackWithFacet` removed; a
   test that a wheel zooms every block and each fitted price axis fits its own
   symbol.
2. `geom.Relative` on `Line`, `Step`, `Area`, `Scatter` and `Candle`, anchored
   at `Training.Within` or the first row; `LastValue` over it; the dialect's
   `"relative"`; the `+` flag of `NumberFormat`.
3. `examples/market` gains a watchlist — four symbols, faceted, with volume —
   and a comparison of the same four on one relative axis.

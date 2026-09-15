# 0054 — A domain reduction belongs in `stat` when its output is the chart

**Status:** Accepted, amended · **Date:** 2026-09-08 · **Implemented:** 2026-09-15

## Context

`docs/chart-types.md` sorts the missing charts by the machinery each needs, and
every bucket in it so far has been missing a *shape*: a rectangle, a position
adjustment, a coordinate system, a channel, a layout. Bucket H was the other
kind — things a chart says that no mark draws.

There is a third kind, and it has been invisible because it looks like nothing
is missing. These are charts whose **shape figure already draws** and whose
**arithmetic it does not have**:

| Chart | Marks it needs | All of which exist since |
|---|---|---|
| Kaplan–Meier survival curve | `Step`, `Area` with `Y2`, a `Track` | v0.10 |
| SPC control chart (X̄-R, I-MR, p, np, c, u) | `Line`, `Scatter`, `HLine` | v0.1 |
| ACF / PACF correlogram | `Bar`, `HLine` | v0.1 |
| ROC and precision–recall curve | `Line` | v0.1 |
| Lorenz curve | `Line`, `Segment` | v0.1 |

Nobody draws these in Go. Nobody draws most of them anywhere outside a
domain-specific package — R's `survminer`, Python's `lifelines`, `qcc`,
`statsmodels` — because a general plotting library has no reason to hold the
arithmetic and a domain package has no reason to hold a plotting library.
figure has an unusual position here: it already holds `Bin`, `KDE`, `Loess`,
`ECDF`, `Hex`, `QQ`, `Squarify`, `Sankey` and `Chord`, all under one rule.

The rule is CONTRIBUTING's: a reduction in `stat` is a pure function, numbers
in and numbers out, with an `Append` form and a determinism test, and none of
them ever sees a string. The question this record answers is not whether these
five are pure — they are — but **where the line is**, because "put the field's
arithmetic in `stat`" has no natural end and a plotting library that acquires a
statistics department has lost.

## Decision

### The admission test

**A reduction belongs in `stat` when its output is the chart's geometry, and
there is no reading of it that is not the chart.**

An ECDF is that: the staircase is the estimator. A KDE is that. A loess fit is
that. Squarify is that. By the same test:

- A **hierarchical clustering** is not, and [ADR 0053](0053-tidy-tree-layout.md)
  says so — its output is a tree you then use for other things, and drawing it
  is one of them.
- A **regression model** is not. `stat.Loess` is in because a loess curve has
  no life off the chart; a fitted GLM has coefficients, standard errors and a
  summary table, and none of that is geometry.
- A **meta-analysis** is not, per the same test, and that decides the forest
  plot below.

Applying it admits five and declines three.

### The five

**`stat.KaplanMeier`** — times and event indicators in, a step function and
Greenwood's variance out. The estimator *is* the curve; there is no other
object. It gets a mark, `geom.Survival`, which runs it in `Train` for
[ADR 0028](0028-distribution-stats.md)'s reason — its output is what the axis
describes — reading X as the time and a `geom.Event` channel as the indicator,
exactly as `geom.ECDF` reads its column. The confidence band and the censoring
ticks are opt-ins on the same layer, the way `geom.Outliers` is on a boxplot.
The numbers-at-risk table underneath is **not** the mark's: a table under the
panel on the shared time axis is a `Plot.Track`, a track is a panel, and a mark
cannot make one ([ADR 0031](0031-tracks.md)). The caller adds it, and that is
the correct division rather than a shortfall.

**`stat.ControlLimits` and the run rules** — a centre line, an upper and a
lower limit for each chart in the family, and the Nelson / Western Electric
rules as a pass returning the indices of the points they flag.

This one gets **no mark**, and the reason is domain-correct rather than
economical. Control limits are computed from a *baseline* period and then
frozen; new observations are judged against limits derived from data that is
not on the chart. That is the whole method — phase I establishes the limits,
phase II watches against them. A mark that recomputed its limits from the
points it was handed would be wrong for the principal use, silently, and would
be right only for the exploratory case. So the caller computes the limits once
and draws three `HLine`s, which is explicit about which data they came from.

The run rules return a **selection**, not geometry — a set of row indices —
which is a shape v1.7 already has a home for: flagged points are a
`geom.ColorBy` over a derived column, or a selection handed to the linked-views
machinery of [ADR 0045](0045-linked-views.md). No new channel.

The limits themselves got cheaper to show while this record was being written.
[ADR 0049](0049-paths-colour-in-classes.md) taught `Line` and `Step` to colour
in classes, and it interpolates the crossing rather than starting the new colour
at the next row — so `ColorBy` over `scale.Threshold` puts an out-of-limit run's
colour change on the limit, which is the reading the chart exists for. That is a
second customer for 0049 arriving before its first release, and it is why this
record needs no drawing machinery at all.

**`stat.ACF` and `stat.PACF`** — a column in, correlations per lag and a
confidence bound out. Stat only, no mark: the drawing convention genuinely
varies between bars, stems and points, and a mark would have to pick one
without there being a reading that distinguishes them.

**`stat.ROC`** — scores and labels in, the curve and its area out. It is two
ECDFs read against each other and sits beside `stat.ECDF` in the same file.
Precision–recall is the same walk with the other two ratios.

**`stat.Lorenz`** — cumulative share against cumulative population, with the
Gini coefficient as the area. ECDF family again.

All five take **sorted input and do not sort**, per `stat`'s existing rule and
for its existing reason: sorting means a buffer, and the geoms already keep
one.

### The three declined, and what they are instead

**A forest plot** is `geom.ErrorBar` plus `geom.Text` plus a `Track` for the
study labels, and it can be drawn today. What it also has is a pooled estimate,
and pooling is a meta-analysis — fixed or random effects, a heterogeneity
statistic, a choice of estimator — which fails the admission test on every
clause. It is an **example**, and a good one, because it demonstrates four
pieces of v1.3 and v0.10 machinery in one figure.

**A funnel plot** is a scatter of effect against standard error with triangular
pseudo-confidence contours. The scatter is a scatter; the contours are curves
defined by a formula in the plane, which is a `geom.Locus`
([ADR 0050](0050-locus-annotations.md)) and needs nothing from this record.

**A Pareto chart** is bars sorted descending with the cumulative percentage on
the secondary axis, and the secondary axis shipped in v1.3
([ADR 0037](0037-secondary-axis.md)). It is a recipe, and so is
**Bland–Altman**: a scatter and three reference lines.

Naming these is part of the decision. A catalogue that lists a chart as missing
when it is three lines of composition is the wish list `docs/chart-types.md`
was written to stop being.

## Consequences

| | |
|---|---|
| `render`, `ir`, `coord`, `scale`, `layout` | unchanged |
| `stat` | five reductions plus their `Append` forms, determinism tests, and no dependency — `math` only, as ever |
| `geom` | one mark (`geom.Survival`), one channel (`geom.Event`) |
| `spec` | a `"survival"` mark; the rest is composition and needs no vocabulary |
| `docs` | three recipes and two examples, which is most of the work |
| Charts unlocked | survival curves with a risk table, the SPC family, correlograms, ROC and PR curves, Lorenz curves, and — by composition — forest, funnel, Pareto and Bland–Altman plots |

This is the cheapest bucket in the catalogue and the one with the widest reach,
because it adds almost no architecture. That is also the argument for doing it
late rather than early: nothing else waits on it.

## Not in scope

- **Fitting, modelling and inference.** No regression beyond the loess that
  ships, no hypothesis tests, no distribution fitting. The admission test is
  the boundary and it is meant to be enforced.
- **Recomputed control limits**, per the argument above.
- **Censoring beyond right-censoring** in the survival estimator. Left- and
  interval-censoring change the estimator, not the chart, and would be argued
  on their own evidence.
- **A `stat` that sees a string.** Unchanged. Group labels are interned in the
  geom, where the order is decided.

## Revisit if

- A sixth reduction is proposed and the admission test does not settle it
  cleanly. The test is the thing this record is really for, and a case it
  cannot decide is evidence the test is wrong rather than that the case is.
- The run rules' selection turns out to want to be a first-class channel
  rather than a derived column. That is a question for
  [ADR 0045](0045-linked-views.md)'s machinery, not for `stat`.

## Amendment: what building it sharpened

The five are built, with `geom.Survival`, `geom.Event` and a `"survival"` mark
in `spec`, and the two examples the record promised: `examples/survival` is
Freireich's remission trial with its numbers-at-risk table in a track, and
`examples/spc` an individuals chart whose limits come from a baseline and whose
drift is caught by the run rules. Eight things came out sharper than the
record, and none of them moves the admission test.

**`stat.ControlLimits` is a family of functions, not one.** The charts of the
family do not share an input: an individuals chart reads a column, an X̄–R
chart a column cut into subgroups of a size, a p or u chart two columns whose
limits vary per subgroup. One function would have been a switch on a kind
argument with half its parameters meaningless for each kind. So `stat.Limits`
is the shared result — centre, lower, upper, and `Sigma` — and `LimitsIMR`,
`LimitsXbarR`, `LimitsNP` and `LimitsC` return it, while `AppendLimitsP` and
`AppendLimitsU` return a centre and a limit per subgroup. Attribute limits
clamp at zero, and a p chart's at one, because a limit nobody can reach is a
line that says nothing.

**The run rules are `stat.AppendRunRules`, and they flag every point that
completes a window.** Nelson's eight rules, selectable, ordered by row then
rule; a long run past a threshold flags each point beyond it rather than only
the first, which is what lets a derived column colour the whole run. A
non-finite reading breaks every run, and σ is read from the upper limit —
`(Upper − Centre)/3` — because the lower one may be clamped.

**`stat.KaplanMeier` returns points, and the band is a method on one.**
`stat.SurvivalPoint` carries the time, the estimate, the risk set, the events,
the censored count and the running Greenwood sum, and `Band(z)` turns that into
the log-log interval — the default in R's survival package and in lifelines,
chosen because the plain interval runs past 0 and 1 in the tails where a
survival curve is read hardest. A time at which only censoring happened is
still a point, with S unchanged: it is where the censoring tick goes. The event
indicator is a `[]bool` in `stat` and a numeric column in the geom — non-zero
for an event — because `data` has no boolean column and a 0/1 column is how
every survival dataset arrives.

**`geom.Survival` starts at time zero, except where zero is not a time.** A
survival curve is printed from zero with everybody alive, so the layer trains
its X axis to include it — unless that axis is a time scale, where zero is an
epoch, or has no position for it, where the curve starts at the first
observation instead. A layer that names no event column counts every row as an
event.

**`geom.Confidence` and `geom.CensorMarks` are the opt-ins.** Both are off by
default, as the record said, and both serialise. The band is painted at a
fifth of the curve's colour so that two arms' bands overlapping still show both
curves.

**The ACF is the biased estimator, and a gap poisons it.** Dividing by n rather
than by n − k keeps the autocorrelation sequence positive semi-definite, which
Durbin–Levinson needs for the PACF. A non-finite value makes every lag NaN
rather than being skipped, because skipping it would silently shift every lag
after it. `stat.CorrelationBound` is the large-sample white-noise band,
z/√n, without Bartlett's widening. It is the one reduction of the five that
does not take sorted input, because a time series' order is its content.

**ROC and precision–recall walk tied scores as one step.** A block of equal
scores moves the ROC curve diagonally, which is what makes the trapezoid area
equal the Mann–Whitney probability with ties counted half. Precision–recall
reports step-wise average precision rather than the trapezoid, which
over-states a PR curve. A curve with no positives — or, for ROC, no negatives —
is empty with a NaN area, not 0.5, because there is nothing to rank. `stat.Lorenz`
returns the Gini coefficient beside its curve. All three sit beside `stat.ECDF`
in `stat/ecdf.go`, as the record placed them.

**The control chart found two things wrong with the colourbar, and both are
fixed outside `stat`.** Its line colours in classes at the limits, so the
chart grew a classed colourbar — and the bar labelled its boundaries at the
precision of its own axis, which steps in whole grams, so a limit at 502.73
read "503", a boundary the chart does not have. A classed bar now writes each
boundary with the fewest decimals that write it exactly, or, for a limit
computed from data that has no short decimal, with enough that the rounding
is under a two-hundredth of the narrowest class. And the bar was not wanted at
all: the limits are on the chart as rules with their values beside them. So
`geom.Guide(false)` lets a layer decline its colourbar and size key, and
`spec` writes it as `"guide": false` on the mark; another layer on the same
scale still brings the bar, because guides merge by what they look like.

The recipes the record named — forest, funnel, Pareto, Bland–Altman — are
written out in `docs/charts.md` rather than as examples: each is a paragraph of
composition over marks that already have examples of their own.

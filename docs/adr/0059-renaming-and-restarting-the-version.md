# 0059 — The library is renamed, and the version restarts rather than doubling

**Status:** Accepted · **Date:** 2026-09-09 · **Implementation:** this commit

## Context

[ADR 0056](0056-three-dimensional-charts.md) decided that three seams are the
wrong shape and has to widen them:

| Seam | before | after |
|---|---|---|
| `geom.Geom` | `Train(x, y scale.Scale) error` | `Train(t geom.Training) error` |
| `render.Observer` | `Panel(i int, area ir.Rect, x, y scale.Scale, cd coord.Coord)` | `Panel(p render.PanelInfo)` |
| `coord.Coord` | `Frame(area ir.Rect, x, y scale.Scale) Coord` | `Frame(f coord.Framing) Coord` |

Under the old name that correction cost a major version, and in Go a major
version above one costs an import path: every file of every caller, every
nested module's `require`, the `$schema` URL, and a `/v2` directory suffix that
stays in the path for as long as the library lives. 0056 accepted that cost on
the grounds that a break is cheap while nobody has to migrate.

Between that record and this one, a cheaper route appeared, because the same
fact that makes the break affordable — **no users** — also makes the *name*
affordable. `refract` named a thesis about prisms that the library had already
outgrown: it is not the splitting that is the point, it is that a chart is
described once and comes out in whatever form is asked for. `figure` names the
thing the library makes.

So there were two changes on the table, each of which independently invalidates
every existing import line, and doing them separately would make callers pay
twice for one edit.

## Decision

**The repository is renamed to `github.com/timzifer/figure`, and the version
restarts at `v0.8.0` instead of continuing at `v2.0.0`.**

The module path changes, so nothing is inherited: the module proxy and the
checksum database index a path, and `github.com/timzifer/figure` is a path they
have never seen. Every tag `refract` published stays published under the old
path and reaches nobody here. That loss is the point rather than a side effect
— it is what lets the version number start where the API actually is.

### Why `v0.x`, and why `v0.8.0` specifically

A `v1` promises a frozen API. 0056's seams are not widened yet, so a `v1` under
this name would be a promise broken on purpose in the next release. `v0.x` says
what is true: the API is settled in every part except three named seams, and
those three are going to move.

The minor number continues the count rather than restarting it. This tree is
`refract v1.7.0`'s, an eighth minor release worth of work is in it, and calling
it `v0.1.0` would claim a newness the code does not have. `v0.8.0` reads as the
eighth release of a library whose API is not frozen yet, which is exactly the
claim.

`v1.0.0` under this name returns when 0056's seams are widened and the freeze
means something again. Until then the `v0.x` series carries the breaking
changes that are planned, and no others: **`v0.x` is not a licence to
churn.** A change that is not on 0056's list still needs a record here before it
needs code.

### What the nested modules do

`backend/gg` and `backend/window` track the core, so they restart with it at
`v0.8.0`. `backend/gg/gpu` was already `v0.x` and is unaffected — it stays at
`v0.3.0`, and the reason it is `v0` is still that the GPU tier is opt-in beta
rather than anything about the core's number. `arrow/v18` keeps its major,
which is Arrow's and never was ours ([ADR 0030](0030-arrow-major-version.md)).

`internal/release` is the single place that knows which major each module
publishes, and the three lockstep modules move from `Major: 1` to `Major: 0`
there. The release tool refuses a tag whose major does not match, which is what
keeps this decision from being re-litigated a tag at a time.

### What this costs, honestly

- **Every tagged release of `refract` is orphaned.** Anyone who did depend on
  the old path keeps working — the tags are still there — and gets nothing new.
  The README says so.
- **The gosumdb entries do not carry over**, so the first `go get` of this path
  publishes a fresh set. There is no way to prove continuity between the two
  paths, and no attempt is made to.
- **The pkg.go.dev history restarts.** Documentation, examples and the badge all
  point at a package with no release history behind it, for a while.
- **A rename is not free to write down.** Every doc that told the story in terms
  of `v1.x` had to be corrected, and this record exists so that the next reader
  does not have to reconstruct why a library at its eighth release calls itself
  `v0`.

## What this does not do

- **It does not implement 0056.** The seams are still positional in this commit.
  This record only removes the reason 0056 had to be spelled `/v2`.
- **It does not change any API.** The rename is mechanical: an import path, a
  package name, and the `figure-` prefix on the identifiers a chart writes into
  its SVG output.
- **It does not re-open the freeze in general.** The growth rule in CONCEPT §15
  holds for everything except the three seams 0056 names.

## Revisit if

Someone is found to be depending on `github.com/timzifer/refract`. The whole
argument rests on nobody being, and if that turns out to be wrong the answer is
not to undo the rename but to publish a `refract v1.8.0` whose package doc says
where the library went.

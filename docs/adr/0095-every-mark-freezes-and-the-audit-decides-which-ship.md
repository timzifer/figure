# 0095 — Every mark freezes with v1, and the audit decides which ones ship

**Status:** Accepted · **Date:** 2026-11-20 · **Revisits:** [0084](0084-what-v1-promises.md), [0029](0029-extension-model.md)

## Context

`geom` is one flat namespace, and it has grown faster than anything else in
the core. At `26f214a` — the rename that restarted the library at `v0.8.0`
([ADR 0059](0059-renaming-and-restarting-the-version.md)), the nearest commit
in this history to the first audit's tables — it held 28 marks, 60 option
functions and a `geom.Desc` of 67 field lines. Today it holds:

| | Then | Now |
|---|---|---|
| Marks (`Mark*` constants, constructors returning `Geom`) | 28 | 46 |
| Option functions | 60 | 102 |
| Exported types | 25 | 40 |
| `geom.Desc` field lines | 67 | 114 |
| Exported declarations (`go doc -short`) | — | 246 |

The eighteen marks are `Candle`, `Contour`, `Depends`, `Graph`, `Horizon`,
`Intersections`, `LastValue`, `Locus`, `NodeLink`, `Parallel`, `ParallelSets`,
`Raster`, `SetMatrix`, `SetSizes`, `Survival`, `Tree`, `Venn` and `Voronoi`.
The forty-two options are `BandHeight`, `Bands`, `Branches`, `Callout`,
`CandleStyle`, `CensorMarks`, `Confidence`, `Corner`, `Curve`, `Dims`,
`Direction`, `DirectionBy`, `Droplines`, `Event`, `Extrude`, `Falling`,
`Gradient`, `Guide`, `Hatch`, `HatchColor`, `HatchDensity`, `HatchWidth`,
`Hollow`, `Inset`, `LabelLevels`, `LevelCount`, `LevelFormat`, `Levels`,
`Link`, `LinkBy`, `MinFontSize`, `OHLC`, `Orient`, `ProgressBy`, `Resample`,
`Rising`, `Rule`, `Slide`, `Top`, `UncertaintyBy`, `Wrap` and `Z`. The new
types are `AxisTag`, `Branch`, `CandleDirection`, `CandleStyleKind`,
`CurveKind`, `Extent`, `Family`, `Linkage`, `MarkRows`, `Orientation`,
`Overhanger`, `Resampling`, `TagAxis`, `Tagger` and `Training`.

The rule that holds the namespace together — one `geom.Option` set, and a mark
accepts every option and ignores the ones it has no use for — was cheap at ten
marks. At forty-six it costs in two places.

**The reader cannot tell what an option does to a mark.** `geom.Tension(0.4)`
on a `Bar` compiles and draws exactly what it drew. pkg.go.dev groups the 102
options under `type Option`, alphabetically, and the package doc is seven lines
long. What each mark reads is known — `spec.writeMarkProps` is a switch over
the marks that decides which properties a document writes for each — but it is
known in the JSON encoder, in terms of the document's field names, and nowhere
a reader of the Go API would look.

**Everything here freezes at the tag.** [ADR 0084](0084-what-v1-promises.md)
makes every exported line of the core module part of the promise and a
machine check of it: Go tags a module, not a package, so `figure/geom` freezes
with the core whatever it holds, and a specialist mark's own option is as
frozen as `geom.Line`. The second audit 0084 §5 asks for has not been taken,
and the surface it would walk has grown by nine records since 0084 was written
(0085–0093, the market chart, the calendar axis, the candle and its direction
channel).

So the question this record answers is narrower than "is `geom` too big": it
is whether some of it may freeze less than the rest.

## Decision

### 1. Every mark in `geom` freezes with v1, and nothing is marked experimental

There is no experimental tier. Three ways to have one were weighed, and each is
worse than not having it:

- **A nested module for the specialist marks**, on `v0.x` beside a `v1` core.
  The marks are wired into `builtin`, `FromDesc`, `spec` and `a11y`; outside the
  core they would have to arrive through `geom.Register`, and 0084 §7 requires a
  registered name to contain a dot. `"sankey"` would become `"x.sankey"` in
  every document that ever named one, which is the dialect breaking in order to
  avoid breaking it. And a mark that can only be reached as an extension is a
  second-class mark for the life of v1.
- **A documented exclusion list** — "these are not covered by the promise".
  `apicheck` would need an allowlist to match it, and a promise with a hole cut
  in it by a text file is a promise a caller has to read the text file to
  trust. Go's own compatibility promise has no such list; what is not ready is
  in `golang.org/x/exp`, which is a separate module, which is the first option.
- **A sub-package of the core**, `geom/exp`. Go cannot tag it differently from
  the module that contains it, so it would freeze anyway, under a name that
  says it does not.

### 2. The lever is removal before the tag, not quarantine

What the second audit doubts, it removes. A mark, an option, a type or a
`Desc` field that the audit cannot defend is deleted or unexported before
`v1.0.0`, and comes back in v1.x if and when it is defended: a constructor, an
option and a field arriving later are additions, and a mark name arriving later
is a field value the dialect has never seen, which 0084's corpus allows. The
cost runs the other way round from quarantine — the surface that ships is the
surface that is promised, and nothing in it carries an asterisk.

### 3. The audit reads `geom` mark by mark, from the lists above

0084 §7 gains one CHANGE BEFORE V1 row: the geom pass of the second audit
starts from the eighteen marks, forty-two options, fifteen types and the
`Desc` growth listed in the Context, and asks of each the question 0084 already
asks of a constant — **is this a fact of the mark, or a knob?** A fact is what
the mark draws or reads (`OHLC` names a candle's four columns); a knob is a
convenience or a tuning choice that a caller could reach another way (a
default the theme already carries, a variant one release asked for). Knobs are
the ones removed.

An option that only one mark reads gets the hardest look, because that is
where the shared namespace pays without the sharing earning anything. It is not
forbidden — `OHLC` is one mark's and is right — but it has to be the mark's
fact rather than a configuration the mark happened to need first.

### 4. The accept-and-ignore rule stays, and there is no query for what was ignored

The rule stays because the alternative is per-mark option types, which is the
second option type the first audit refused
([v1-api-audit.md](../v1-api-audit.md), `geom`'s `Option` row): a third-party
geom could not take the shared options any more.

A function that reports which options a layer ignored was considered and is not
added. It would be one more exported declaration frozen for a question that is
answered at reading time rather than at run time, and the next section answers
it there.

### 5. Discoverability is documentation, kept honest by a test

- **One table of what each mark reads.** What `spec.writeMarkProps` decides in
  its switch is written down once more as data: a table, in
  `internal/markprops`, of the options each mark reads as a property of
  itself. `spec`'s tests derive the same answer from what the encoder actually
  writes for every mark and fail when the two differ, so the JSON encoder and
  the documentation cannot disagree about a mark. The encoder keeps its switch:
  turning a switch whose cases carry conditions of their own into a table
  would be a rewrite of the dialect's writer for no change in what it writes.
- **A package doc that is a map of the package.** `geom`'s doc comment grows
  from seven lines to a guide with headings — the marks by family (the basic
  marks, distributions, relational layouts, sets, time and markets,
  annotations), the options by group — and a **matrix of options against
  marks**. pkg.go.dev renders the headings as a table of contents. A test
  regenerates the matrix from the table and fails if the doc comment differs,
  the way the gallery test fails on a stale figure; the 102 options' own doc
  comments stay hand-written.
- **`geom/geom.go` is split by concern.** It is 2,254 lines: the options, the
  configuration they write, the shared traversals and the batching helpers are
  separate files, with no declaration added, removed or changed.

### Order of work

1. This record and the row in 0084 §7.
2. `geom/geom.go` split, a move and nothing else.
3. The table of what each mark reads, held to `spec.writeMarkProps` by a test,
   with every golden and round-trip test unchanged.
4. The package doc and the matrix test.
5. The geom pass of the second audit, under 0084 §5, from §3's lists. Its
   removals are CHANGE BEFORE V1 rows of that audit, each in its own commit.

## Consequences

- Every mark that is in `geom` on the day of the tag is promised for the life
  of v1. Shipping fewer marks is the price of the promise having no exceptions.
- The audit has a list to work from rather than a package to re-read, and the
  list is the part that has never been audited.
- A reader can see on pkg.go.dev which options a mark reads, and the answer is
  the one `spec` acts on.
- The silently ignored option stays silent at run time. A caller who wants to
  know reads the matrix.

## What this does not do

- **It removes nothing.** Which marks, options or fields go is the audit's
  verdict, not this record's.
- **It does not change the option API.** No per-mark option types, no
  `Option` interface, no validating constructor.
- **It does not touch `Geom`, `Desc`'s existing fields or the dialect.** The
  table in §5 is internal, and `spec` writes what it wrote.

## Revisit if

- A mark the audit keeps needs an option that no other mark could ever read,
  and three marks are in that position. Then the namespace is carrying several
  sub-languages, and a per-family option type is worth weighing again — as a
  v2 question.
- Somebody outside the module ships a mark through `geom.Register` that users
  want built in. That is the case for promoting an extension rather than
  quarantining a built-in, and the dotted name is the migration it costs.

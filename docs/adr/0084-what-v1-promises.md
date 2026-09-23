# 0084 — v1.0 is tagged when a machine can say the surface did not move, and a point counts as one value

**Status:** Proposed · **Date:** 2026-09-23 · **Revisits:** [0060](0060-parameter-structs-at-every-seam.md), the [v1 audit](../v1-api-audit.md)

## Context

The API was audited once, identifier by identifier, before the first attempt at
a freeze ([v1-api-audit.md](../v1-api-audit.md)). Every change that audit asked
for landed ([ADR 0029](0029-extension-model.md)), the library was renamed and
restarted at `v0.8.0` ([ADR 0059](0059-renaming-and-restarting-the-version.md)),
and the three seams that took positional scale arguments were widened into
parameter structs ([ADR 0060](0060-parameter-structs-at-every-seam.md)) so that
a third axis could arrive additively. It did: `figure/three`
([ADR 0056](0056-three-dimensional-charts.md)). The one question 0060 left open,
how `data.Source` grows, was settled by
[ADR 0061](0061-columns-are-one-value.md): a column is one value carrying a
`Kind`.

CONCEPT §15 says `v1.0.0` returns "when the surface settles". Nothing says what
settled means, and three things make that worth writing down now rather than at
the tag.

**The surface grew after the audit.** Twenty-seven records have been written
since 0056 — a third axis, a map, a coord with more axes than two, a dozen
marks, axis breaks and folds, callouts, hatching, a bivariate channel — and
none of them was read against the audit's verdicts. The core module now exports about a thousand
declarations (`go doc -short`, summed over the public packages: `geom` 226,
`scale` 137, `stat` 128, `.` 94, `coord` 73, `three` 68).

**The growth rule is stated and not checked.** 0060 wrote it as enforceable:
an interface a third party implements never gains a method, and a method on one
takes no more than one parameter beyond its output destination. Scanning every
exported interface in the core against that sentence finds it broken in places
nobody decided to break it:

| Interface | Method | Parameters beyond the destination |
|---|---|---|
| `stat.Family` | `Locus(xs, ys []float64, level float64, ext Extent)` | `level`, `ext` |
| `coord.Exploder` | `Explode(x0, y0, x1, y1 float32, by float64)` | an extent and `by` |
| `geom.LabelPlacer` | `PlaceLabel(run ir.TextRun, move bool)` | `run`, `move` |
| `scale.Breaker` | `SetBreakGap(brk, fold float32)` | `brk`, `fold` |

and in places where the literal count is wrong but the design is right:
`coord.Coord.Points(dst, xs, ys)`, `Edge(p, from, to)` and
`Area(p, x0, y0, x1, y1)`; `scale.Scale.SetRange(lo, hi)`,
`scale.Zoomer.SetDomain(lo, hi)`; `scale.BivariateColorScale.ColorAt(v, u)`.
Those take a point, a pair of points, an interval, a rectangle, a pair of
columns or a bivariate reading — values whose arity is fixed by what they are,
the way 0060 argued the tuple-returning coord calls are fixed. `ColorAt` is the
clearest case: it is `ColorScale.Color(v)` for a mark that carries a second
reading, called once per row, and two readings are the definition of the
interface rather than a count that could grow —
[ADR 0067](0067-a-bivariate-colour-channel.md) refuses a third, because three
readings have no key a reader can decode. A rule that a test cannot apply without
a judgement per method is a rule that erodes one reasonable exception at a
time.

**Nothing would notice the surface moving.** The golden files catch a chart
that changes; nothing catches an exported name that disappears, a signature
that changes shape, or a JSON field whose meaning drifts. Before v1 that is a
review burden; after it, it is a broken promise that ships.

## Decision

**v1.0.0 is tagged when four checks are green and a second audit has no open
CHANGE BEFORE V1 row — not when the surface feels settled.** Every check is
stdlib-only, because the core's one rule applies to its tools as much as to its
code.

### 1. The growth rule counts a value of fixed arity as one parameter

The rule in CONCEPT §15 is amended by one sentence: **a value whose arity is
fixed by what it is counts as one parameter** — a point, a pair of points, an
interval, a rectangle, a pair of parallel columns, a bivariate reading — because
its parts are the value's rather than a description that can grow. Everything
else counts as what it is. `ir.Backend`'s drawing calls
remain the exception the rule already names.

Under that reading `Points`, `Edge`, `Area`, `SetRange`, `SetDomain` and
`ColorAt` comply, and the four methods in the table above do not. Each is
changed before v1 to take a request struct — `stat.LocusRequest`,
`coord.ExplodeRequest`, `geom.LabelRequest`, `scale.BreakGaps` — named in the
style 0060 set (`FurnitureRequest`, `TickRequest`). All four are small: one
implementation or a handful in this repository, and the callers are `render`
and `geom`.

### 2. A test applies the rule

A test in the root package parses the public packages with `go/parser`, finds
every exported interface, counts each method's parameters beyond a leading
destination, and fails on any method with more than one it cannot account for.

Syntax cannot tell a fixed-arity value from a description: `SetRange(lo, hi
float32)` and `SetBreakGap(brk, fold float32)` are the same shape to a parser.
So the exceptions are an **allowlist of methods by name**, each with its reason
beside it — `ir.Backend`'s drawing calls, and the values §1 names — and §1 is
the test for whether a method may be added to it. An entry is a visible line in
a diff rather than a silence.

The test reads **every** exported interface, not only the ones §15 lists as
implemented by third parties. An optional interface such as `scale.Breaker` or
`geom.LabelPlacer` is asserted rather than required, but anything exported can
be implemented outside the module, and the day somebody does is not a day the
signature can still change. A new interface that breaks the
rule fails CI the day it is written, which is the only day it is free to fix.

### 3. The exported surface has a baseline, and CI diffs it

`internal/cmd/apicheck` writes one line per exported declaration of every
public package in each module — the shape of Go's own `api/*.txt` — into
`api/<module>.txt`, and CI fails when the file does not match the code. A
function, a type, a method, **each method of an exported interface, each
exported field of a struct and the value of each constant** is a line of its
own, because each is something a caller can depend on separately.

Before v1 the diff is a review aid: a change to the surface is a change to a
committed file, and a reviewer reads it. From v1 the check sorts a change into
three kinds:

- **A removed or changed line is refused.** That is a major version.
- **A method added to an exported interface is refused**, although it is an
  added line. Go has no default methods, so every implementation outside the
  module stops compiling whatever the method does; a new capability arrives as
  an optional interface beside it, as §15 already says.
- **Anything else added passes** — a function, a type, a constant, a field. A
  new field is where the check stops and the reviewer starts: it is compatible
  only if its zero value leaves the behaviour as it was, which is §15's rule for
  a struct and a question no text comparison can answer. It is a command of its own, beside `releasecheck`, rather than
`golang.org/x/exp/apidiff`: that would be the first dependency of the core's
tooling, and the check it performs is a sorted text comparison.

### 4. The JSON dialect has a corpus that is never edited

`spec/testdata/v1/` holds one document per mark, per coord, per positional
scale kind and per colour and size scale kind, written by the code at the tag.
A test parses each, checks that it marshals back to an equivalent document, and
checks that the chart it builds describes itself as the document says —
`geom.Desc`, `scale.Desc` and `coord.Desc` are what the comparison reads. After
v1 **the documents are only ever added to**: one that stops reading, or reads as
a different chart, is a broken promise that the dialect's comment in
`spec.Schema` makes and that nothing tests today.

What the corpus does not pin is the picture. Each document may also be rendered
against a golden file, and that file is regenerated with `-update` like any
other, because v1 does not promise pixels (below). The document is never
regenerated; that is the whole difference between the two.

### 5. A second audit, recorded beside the first

The audit is repeated over what arrived since it was taken, with the same three
verdicts and the same five rules, and its findings are appended to
[v1-api-audit.md](../v1-api-audit.md) as a dated section rather than a second
document — the two together are the reasoning behind one surface. It walks
the `api/` baseline rather than `go doc`, so that nothing it reads can be
missing from what CI later checks. Three questions it has to answer, because the
first audit could not ask them:

- **`figure/three` freezes with the core.** It is a package of the core module,
  and Go has no way to tag one package differently from its module. It was
  built after 0060 and follows the rule from birth; the audit confirms that
  rather than assumes it.
- **`stat` exports what a third party calls, and nothing only a geom calls.** It
  has grown with every instrument and layout since the first audit, and a
  helper exported so that `geom` could reach it across a package boundary is a
  declaration frozen for nobody. Those move under `internal/`.
- **Every registry reaches every kind.** 0029 made `FromDesc` a registry lookup;
  the audit checks that every coord, scale and mark added since round-trips
  through `spec` and is reachable by name — a kind that only its constructor can
  build is a chart that cannot be written down.

### Order of work

1. This record, and the amendment to CONCEPT §15.
2. The growth-rule test, failing on the four methods; the four request structs,
   making it pass.
3. `apicheck` and the committed baseline.
4. The second audit, working from the baseline; its CHANGE BEFORE V1 rows, each
   in its own commit.
5. The JSON corpus, written after the audit so that it records the dialect the
   audit left.
6. `releasecheck` over every module, the status lines in README and CONCEPT, and
   the tag — `v1.0.0` for the core, `backend/gg` and `backend/window` together,
   as §15 already says.

## Consequences

- Four interfaces change shape once more. Each is implemented outside the
  repository by approximately nobody, which is the argument for doing it now:
  after the tag, each would be a major version or a second method beside the
  first.
- A change to the exported surface touches a committed file. That is friction
  on purpose; it is the same friction a golden file puts on a changed chart.
- The rule's exception for fixed-arity values is written down, so the next
  `Area`-shaped method needs no argument — and a method that is *not* one of
  those values cannot borrow the exception by resembling one, because the test
  names the methods it excuses.
- The zero-value rule for a new field stays a human judgement. The check makes
  every new field a visible line; it cannot make the reviewer read it.

## What this does not do

- **It adds no feature.** The open items in CONCEPT §14 — a mark that fills a
  ring, a conic projection, labels that avoid other layers', a keyframe
  timeline — are additive and land in v1.x.
- **It does not touch `ir.Backend`.** Its drawing calls take the ink
  ([ADR 0002](0002-ir-and-backend.md)).
- **It does not reopen `data.Source`.** [ADR 0061](0061-columns-are-one-value.md)
  settled it.
- **It does not move `backend/gg/gpu` off `v0.x`.** The GPU tier is opt-in beta
  ([ADR 0022](0022-gpu-tier.md)).
- **It does not freeze the Go floor.** The `go` line in each `go.mod` follows
  [ADR 0005](0005-go-version.md) and may rise in a minor release, as it does in
  the Go project's own modules: it is a statement about the toolchain rather
  than about the API, and the promise here is to programs that build.
- **It does not promise output.** The golden files pin what a chart looks like
  within a release; v1 promises that a program compiles and a document reads,
  not that a later minor draws every pixel where an earlier one did.

## Revisit if

- The growth-rule test wants a third kind of exception. Two — the ink and a
  value of fixed arity — are a rule; three is a list, and the rule should be restated
  rather than extended.
- The baseline diff is routinely committed without being read. Then the check
  is a formality, and the answer is to make the pre-v1 diff fail too.

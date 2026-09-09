# 0061 — A column is one value that grows, and identity is its spelling

**Status:** Accepted · **Date:** 2026-09-09 · **Implementation:** this commit

## Context

[ADR 0060](0060-parameter-structs-at-every-seam.md) gave every seam an outsider
implements a growable parameter, and deliberately left one alone.
`data.Source` asked three questions —

```go
Float64Column(name string) ([]float64, bool)
TimeColumn(name string)    ([]time.Time, bool)
StringColumn(name string)  ([]string, bool)
```

— and its own documentation promised that a fourth kind of column would arrive
as an optional interface beside it. That promise works. What it does not do is
scale, and three things in the tree said so before any fourth kind existed.

**A reader asks the question three times.** `geom.column`, `data.Label`,
`data.Labels`, `a11y`'s cell reader and `spec`'s encoder each tried the three
in turn. A fourth kind is a fourth branch in five places, a fifth is a fifth,
and every one of them is a place to forget one.

**A wrapper has to forward every optional interface, and two did not.**
`data.Rows` forwarded `data.Nulls`; the stream's view and the transition's
blended view did not. So a source that marked a row absent while leaving a
value in it — which the interface explicitly permits — silently lost that mark
the moment it went through a transition, and the blend interpolated between two
numbers, one of which nobody had measured. Nothing failed to compile. Nothing
logged. The chart drew a value that did not exist.

**And one kind was already missing, with a bug attached.** The Arrow adapter
widens `Int64` into `float64`. Above 2^53 that stops being lossless, and the
column that most often holds large integers is the column that holds
*identifiers*. `geom.KeyBy` names the column that says which row is which, keys
are compared as their spelling ([ADR 0043](0043-mark-identity.md)), and the
spelling came from `strconv.FormatFloat`. So two ids one apart above 2^53 were
one key, one facet panel and one row — and a chart with a `KeyBy` on them
blended one into the other. `9007199254740993` also *printed* as
`9.007199254740992e+15`, in every tooltip, strip title and legend entry.

The prompt for settling it now is concrete: `github.com/timzifer/metrology` is a
candidate for an adapter. Its magnitudes are exact decimals rather than floats,
and a measurement's own text form is `"2.5 bar"` — the magnitude with every
digit it carries and the unit it is held in. A library whose data layer can only
hand over `[]float64` and `[]string` can plot that, and cannot label it.

## Decision

**`data.Source` has one column accessor, and it returns a growable value:**

```go
type Source interface {
	Len() int
	Columns() []string
	Column(name string) (Column, bool)
}
```

**`data.Column` carries the values, the nulls and the spelling.** A new kind of
column is a `Kind` and a field, handled in this package's own helpers. It is
not another method here, not an optional interface beside this one, and not a
branch every reader has to be taught.

### What a Column is

```go
type Column struct {
	Kind    Kind
	Floats  []float64
	Ints    []int64
	Strings []string
	Times   []time.Time
	Nulls   []bool
	Text    func(i int) string
}
```

The slices are borrowed exactly as they were, so `data.Float64Columns` is still
zero-copy and the allocation gates are unchanged.

### Position is a float; identity is a spelling

This is the rule the whole record turns on, and it settles the int64 question
without inventing an identity mechanism.

**A position may lose what a pixel cannot show.** `Column.Numbers` converts a
`KindInt64` column to `float64` for the axis, and above 2^53 that conversion is
lossy. It is also invisible: two ids that differ by one land on the same pixel
at any zoom a screen has.

**A spelling may lose nothing, because it is what identity is compared as.**
`Column.Spell` formats an exact integer with `strconv.FormatInt`. `data.Labels`
and `data.Label` go through it, `KeyBy` reads `data.Label`, faceting groups by
`data.Labels`, and a categorical axis encodes by it — so all four agree, and all
four are now exact.

`data.Table` gains `Int64`, and the Arrow adapter reads `Int64`, `Int32`,
`Int16`, `Int8` and the unsigned kinds that fit into an `int64` as exact
integers. A `uint64` is deliberately not claimed: its top half has no `int64` to
be exact in, and wrapping silently would be worse than the float it becomes.

### Nulls move onto the column

`data.Nulls` is deleted as an interface. Absence is `Column.Nulls`, and it
travels with the values it describes — which is what makes the transition bug
above structurally impossible rather than fixed once. `NullMask`, `IsNull` and
`AnyNull` stay, reading the field.

The transition's blended view now reports its own mask, because it has one to
report: a row neither end had is absent rather than reading as `""` or as the
year 1.

### The spelling is a function, and it is optional

`Column.Text` is `func(i int) string`, not `[]string`. Most columns are never
spelled — a scatter of a million points that nobody hovers over needs no labels
at all — and materialising a million strings for that is a cost with no reader.

`nil` means the default spelling of the kind, which is what every column built
in this package uses. A Source whose values mean more than their type fills it
in, and that is the whole of what an adapter needs to label a quantity: figure
asks for numbers to place the mark and for a spelling to name it, and knows
nothing about units, uncertainties or decimals. `data.Table.WithText` is the
same capability for a caller with a table in hand.

The spelling does not survive the JSON spec. A document records values; a
Source rebuilt from one spells them the default way. That is stated rather than
worked around: a Go function is not serialisable and
[ADR 0041](0041-qq-plots.md)'s rule — a named member of a closed family
serialises, an arbitrary function does not — already says so.

### An unknown kind is labelled rather than lost

The failure mode moves, and it is worth naming: instead of "this Source lacks a
method", it is now "this reader does not know this `Kind`". That is the same
shape as `ir.Marker`'s, and it gets the same answer as ADR 0060 gave that one.
**Every column of every kind can be spelled**, so the default branch that is
always available is `Column.Spell`. A reader that switches on `Kind` and has no
case for a later one still has a label; a reader that wants a *number* asks
`Column.Numbers` and is told no rather than being handed a wrong one.

### The kinds, and the ones deliberately absent

`KindFloat64`, `KindInt64`, `KindString`, `KindTime`. Each has a named use.

**Booleans and durations are not kinds.** Arrow already carries both into
`float64` — a bool as 0/1, a duration as a count of its unit — and neither has
a use case that the widening breaks today. When one appears, it is a constant
and a field and a case in this package's helpers, which is the whole point of
the shape. A caller who wants `true`/`false` on an ordinal axis now can put the
column in as strings, or give a numeric column a `Text`.

**There is no `Unit` field.** A spelling already carries the unit where the
Source has one, and a field would exist so that an axis could title *itself* —
which is a decision about who owns an axis title, not about how a column is
shaped. It gets its own record if it is wanted.

### `$schema` gains a parse type

`spec.ParseInteger` (`"integer"`) is written for a `KindInt64` column and read
back into one. The document's decoder asks `encoding/json` for `json.Number`,
because a JSON number read into an `any` becomes a `float64` and an id past 2^53
would come back a different id — losing on the round trip exactly what the kind
exists to keep. A document with no parse map has its integer columns inferred,
and one fractional value anywhere in a column demotes it to a number, so a table
whose first row happens to hold whole numbers is not truncated.

## What this does not do

- **It does not make figure arithmetic exact.** Binning, kernel density, LOESS,
  stacking and every other statistic run in `float64`, as they always have. The
  exactness is in identity and in text, which is where it is observable.
- **It does not decide where a metrology adapter lives.** The seam carries one;
  the placement comes with the adapter.
- **It does not touch `Subset`.** `data.Subset` stays an optional interface: a
  Source either is a cut of another or is not, which is a capability rather
  than a fact about a column. That is ADR 0060's test, applied again.
- **It does not reopen `ir.Backend`, the scales or the geoms.** A column is
  read into `[]float64` before it reaches any of them, exactly as before.

## Revisit if

A reader is found switching on `Kind` where `Spell` or `Numbers` would have
done. That is the sign that the fallback is not carrying the weight this record
gives it, and the fix is a helper here rather than a branch there.

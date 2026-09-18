# 0076 — The other half of the count is a mark, and the panel is still the caller's

**Status:** Accepted · **Date:** 2026-09-18 · **Implemented:** 2026-09-18

## Context

[ADR 0074](0074-sets-are-counted.md) built an UpSet plot out of one stat and two
marks, and refused a third thing in one line:

> - **Set-size bars as an option on the matrix mark.** They are a third panel,
>   and a mark cannot make one. `figure.Grid` is the answer and the example
>   shows it.

That is right about the panel and silent about the count, and the two are not
the same question. `stat.Intersections` already computes both halves of what a
membership table has to say:

```go
// Sizes has one entry per set: how many elements are in it, counting an
// element once per set it is in. These are the totals a set-size bar draws,
// and they do not add up to the number of elements.
Sizes []int
```

Nothing read it. `grep -rn "Sizes" geom/ spec/` found the size *channel* and
nothing else, and the third panel in `examples/sets` counted the table again by
hand:

```go
n := map[string]int{}
for _, s := range what {
	...
	n[s]++
}
```

That counts **rows**, and the stat beside it counts **elements**, on purpose:

> A membership named twice is one membership: a set holds an element or it does
> not, and counting the row again would make a duplicated row look like a
> bigger set.

A join hands back a row per match and the same (element, set) pair comes back
twice as a matter of course. One such row, and the panel on the left of that
figure disagreed with the two panels beside it — while the library held the
right number and offered no way to ask for it. [ADR 0074](0074-sets-are-counted.md)'s
own claim 2 is the argument against exactly that:

> a bar standing over the wrong column is the one way this form can lie, and two
> layers computing one thing separately from one table cannot do it.

## Decision

**`geom.SetSizes` is one bar per set, from the same table and the same count.**
Four claims, and the refusal above survives all of them.

### 1. A mark still cannot make a panel, and this one does not

[ADR 0054](0054-statistical-instruments.md)'s rule is untouched: the three-panel
UpSet is still a `figure.Grid` whose cells share two scale objects, built by the
caller, and `examples/sets` still builds it. What moved is the **arithmetic**,
which was never what that refusal was about. The line it replaces in the example
is a map and a loop, not a panel.

### 2. Three panels, one table, one count

0074's claim 2 extended by one: all three marks read the same membership table
and run `stat.Intersections` in their own `Train`. They cannot disagree about
what is in the data, because there is one piece of arithmetic and no second
copy of it anywhere.

The lanes line up for a second reason worth separating from the first. Both
`SetMatrix` and `SetSizes` encode set *names* into the ordinal scale object they
are given, in the order the table first names them — so a caller who hands both
the same scale gets rows that match by construction. Before, they matched
because two tables happened to be built in the same order, which is a property
of the example rather than of the library.

### 3. It reads no ranking

`Order` and `Top` decide which **columns** an UpSet has. A set's total is over
the whole table — a set with no drawn column still has elements in it, and
`SetMatrix` already draws a lane for one. So this mark accepts both options and
ignores them, which is [ADR 0039](0039-relational-layouts.md)'s precedent for
`SizeBy` on a relational mark: an option that means nothing here is ignored
rather than made to mean something else.

It is also why the counts here and the counts above are not comparable and are
not meant to be: a customer with two products is in two of these bars and in one
of those. One row of numbers says how the sets overlap; the other says how big
they are.

### 4. Which way the bars grow is the axis's business

The printed form has these bars growing *away* from the matrix they label.
That is `scale.Reverse` on the panel's X axis
([ADR 0075](0075-an-axis-has-a-direction.md)), and this mark has no orientation
option of its own — 0039's argument, which is that a mark places its geometry
and the scale and coord stages decide what that looks like. What the mark does
fix is which axis is which: the count is on X and the sets are on Y, because it
stands beside a matrix whose rows are the sets.

## Consequences

| | |
|---|---|
| `geom` | `SetSizes`, `MarkSetSizes`, `slotHalfHeight` (`slotHalfWidth`'s quarter turn, both now one helper) and `baselineAcross` |
| `spec` | the mark `"set-sizes"`, with no properties of its own beyond the fill |
| `stat`, `ir`, `render`, `coord`, `scale`, `internal/layout`, `figure` | unchanged |
| `examples/sets` | loses its hand-built size table, and its third panel now grows away from the matrix |

**No row is reported.** A bar is a set's total, which is what many rows have in
common — `Hit.Row` stays at −1, the rule 0074 set for every mark whose marks are
aggregates.

**The three-panel grid names its lanes twice,** once down the side panel and
once down the matrix. That is a `figure.Grid` property rather than a mark's: a
member plot's own theme is not used, so a panel in a grid cannot turn its own
tick labels off the way the single-plot UpSet turns off the bars' X ticks. It is
written down here because it is the first composition where it shows.

## Not in scope

- **An option on `SetMatrix`.** Refused in 0074 and still refused, for the
  reason given there: a mark cannot make a panel.
- **An orientation option.** Per claim 4 — the direction belongs to the scale.
- **Lane order by set size.** The common UpSet convention, and the same argument
  0074 made about a second column convention applies: it decides what two marks
  draw, so it would have to arrive on both at once, with its own record.
- **The elements themselves.** Still a different mark for a different reader,
  as 0074's revisit clause says.

## Revisit if

- **A fourth reading of the same table turns up** — a degree distribution, a
  Jaccard matrix. The question each time is whether the number already exists in
  `stat.Intersections`, in which case it is a mark over the count rather than a
  new count.
- **Lane ordering is asked for.** One option, two marks, one record.

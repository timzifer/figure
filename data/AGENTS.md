# Working in `data`

Notes for changing `data`, moved here from the root [AGENTS.md](../AGENTS.md),
which holds the rules that apply everywhere — read that one first. This file
covers nulls, `data.Stream` and the ring buffer.

## Things that will bite

**A null is a missing value, and only a numeric column can say so by itself.**
`""` is a string somebody may have measured and the zero time is an instant, so
absence in a text or temporal column is stated beside the values through
`data.Column.Nulls` rather than inside them — [ADR 0034](../docs/adr/0034-null-values.md),
[ADR 0061](../docs/adr/0061-columns-are-one-value.md).
It is read in exactly one place, `geom.column`, which writes NaN whatever the
column is stored as; every policy, traversal and mark downstream is the
machinery that already handled a NaN, and no geom knows about nulls. Three
things there are load-bearing. A column with **no** nulls must answer `ok ==
false`, because that answer is what every reader decides between the borrowed
column and a copy on — a mask of all false makes a chart copy a column to
change none of it. `masks` compares the mask against the values before copying,
so an Arrow numeric column, whose nulls are already NaN, is still handed on
untouched. And `series.detach` copies before writing, because `s.x` may be the
caller's own slice: a neighbouring column's null takes a row's *position* away,
and doing that in place edits the table rather than the chart. A genuine `""`
is still a category and there is a test for it — dropping every empty string
passes every null test and is a different feature.

**`data.Stream` is deliberately not a `data.Source`.** A Source is read column
by column over several calls, and a table appended to between two of them
disagrees with itself. `Snapshot` freezes and swaps; `Source()` reads whatever
the last snapshot froze. Making Stream implement Source would compile, and
would produce a chart with more timestamps than values under load. The two
buffers are reused, so a snapshot is valid only until the next one — that is
the price of the frame costing no allocations, and it is why Snapshot belongs
between frames rather than during one.

**Unwrap a ring buffer before rewriting it, not while.** `Stream.compact`
resolves every slot number first and rewrites the columns afterwards, and takes
the *old* window as a parameter. `at` answers from the ring's current length,
so rewriting the first column changes the answer for the second — a bug that
only appears on the second column, and only after the ring has wrapped. There
is a test.

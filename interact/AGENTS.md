# Working in `interact`

Notes for changing `interact`, moved here from the root
[AGENTS.md](../AGENTS.md), which holds the rules that apply everywhere — read
that one first. This file covers watched renders, hit-testing per subpath and
against outlines, and keys read from the plot's layer.

## Things that will bite

**A watched render must draw exactly what an unwatched one draws.**
`interact.Index.Watch` wraps a backend and indexes what it sees; every method
forwards first and indexes second. There is a test comparing the two traces
call for call, and `TestAWatchedRenderDrawsWhatAnUnwatchedOneDoes` compares the
SVG bytes. A probe that changed anything would silently halve the coverage of
every golden file, the same way a scheduling-dependent panel order would.

**Hit-testing indexes one mark per subpath, not one per call.** A layer draws
all its bars in a single path, because `geom.groupByColor` batches by colour. A
mark per call would make the row of bars one shape, so pointing at the fourth
bar would report whichever corner of whichever bar happened to be nearest.
There is a test.

**A hit on a filled mark is decided against the outline, not the box.**
`interact.inside` ray-casts the subpath the drawing call carried. Before the
coordinate stage
every filled mark figure drew was a rectangle, so its bounding box *was* its
shape; a pie's wedges have boxes that overlap almost completely, and a hit
decided on the box alone names whichever wedge was indexed last. The row behind
an area hit is then the nearest reported position *inside that shape's box*,
measured from the pointer rather than from the corner the hit reports — a corner
of a tall bar is nearer to the neighbouring bar's row than to its own, and every
slice of a pie shares the corner in the middle.

**A key is read from the *plot's* layer, never the panel's.** On a faceted
chart those are different objects: a panel holds a `geom.Faceter` `Subset` copy
whose `Source` is the cut, one panel's worth of rows, while a `Hit.Row` has
already been resolved back through `data.Subset` to the table the caller handed
in. Reading the cut with a handed-in row number indexes the wrong table — and
does it silently, with a plausible answer, which is the worst kind. The layer
index is the same in both because a facet preserves layer order.
`TestKeyIsReadFromTheHandedInTable` is the bug written down.

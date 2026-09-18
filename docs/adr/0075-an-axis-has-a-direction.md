# 0075 — An axis has a direction, and it belongs to the scale rather than to its domain

**Status:** Accepted · **Date:** 2026-09-18 · **Implemented:** 2026-09-18

## Context

[ADR 0039](0039-relational-layouts.md) left a clause open and named the shape of
its answer:

> - A third chart wants the rim/hub convention with the opposite orientation. A
>   positional `scale.Reverse` that survives the round trip would be the answer,
>   and it would be a change to `scale` and to the dialect rather than to these
>   marks.

and gave, in passing, the reason there was none:

> a reversed positional domain would not have survived the round trip anyway,
> because the JSON dialect's `scale.reverse` is a colour scale's.

The third chart has arrived. [ADR 0074](0074-sets-are-counted.md) put the
set-size bars of an UpSet plot in a panel of the caller's own, and the printed
form has them growing *away* from the matrix they label. `examples/sets` grows
them towards it, because there was no way to ask for the other direction.

Before writing the option, the refusal was measured rather than read, because
what it says would not have needed an option at all: pinning `Domain(hi, lo)`
mirrors `Map` and `Invert` exactly — both are one interpolation over the pair —
and `Describe` reports `min=12 max=0 fixed=true`, which the dialect writes as
`"domain": [12, 0]` and reads back unchanged. **The round trip was never what
stopped it.** Three other things did, and a fourth was waiting in `render`.

## Decision

**The direction is a property of the axis, and what reverses is the pixels
rather than the numbers.** Four claims.

### 1. A domain is an interval, and direction is not one of its two numbers

`Domain`, `LogDomain` and `SymLogDomain` order their bounds, which
[`Zoomer.SetDomain`](../../scale/zoom.go) already did — its `order` helper even
says why: "a drag that crosses itself, or one on an inverted axis, arrives here
backwards."

A descending pair does not fail; it half-works, which is worse. [Zero] and
[Nice] take a minimum and a maximum of it and hand back an ascending one. The
containment test in `Ticks` asks whether a value lies between `lo` and `hi`, and
on a descending pair nothing does — so an axis pinned `[12, 0]` drew its marks
mirrored and carried **no numbers at all**, including a sequence pinned by hand
with `TickValues`. And the first pan turned the axis back round, because a drag
arrives in the order the pointer met its ends and `SetDomain` orders it.

Three different parts of one file, each right on its own terms, and each written
against an invariant the descending pair broke. Ordering the bounds is how that
invariant is kept rather than patched in three places.

### 2. `scale.Reverse()` flips the range

One field on the linear scale and one helper:

```go
func (l *linear) device() (float32, float32) {
	rlo, rhi := l.rangeOf()
	if l.reverse {
		return rhi, rlo
	}
	return rlo, rhi
}
```

`Map`, `Map64` and `Invert` take their range from it and are otherwise
untouched. Everything that trains, frames, searches or filters a domain still
sees a low number and then a high one, so it keeps working with no knowledge of
this at all — which is why a reversed axis pans, zooms, autoscales, clones,
snapshots and describes itself without another line being written for any of
them. The device range was already allowed to run either way: `Scale.SetRange`
documents that `lo` may exceed `hi`, which is how a Cartesian panel puts larger
values higher up. A reversed axis composes with that rather than fighting it.

Two contracts are unchanged and are worth stating because they are what the rest
of the library rests on: `Domain()` reports its bounds ascending whatever the
axis does, and `Ticks` is still ordered ascending **by value** — it is the
*positions* that descend.

### 3. The dialect spells it `"reverse"`, which is the word already there

0039 expected a clash here, and there is one word for two ideas: `spec.Scale` is
one struct for positional and colour scales, and `reverse` already turns a
colour ramp around. It is not ambiguous, because the channel a scale hangs off
is what says which kind it is — under `encoding.x` it is an axis and under
`encoding.color` it is a ramp — and on both the word means the same thing in
English. What makes that safe rather than merely true is that it is **written
only by the kind that reads it back**: a linear scale writes `reverse` when it
is set, and no other kind writes it at all, so a document never carries the word
on a scale that would ignore it.

### 4. The label sweep had to learn screen order, and only a measurement found it

`render.selectXLabels` drops a tick label that would run into the one before it.
It swept in *tick* order, which is ascending by value — and on a reversed axis
that is right to left across the panel. So it kept the rightmost label, set its
watermark at the right-hand edge, and found every remaining label behind it: a
400-pixel chart came out with `0` on its X axis and nothing else.

The sweep now runs in screen order, outermost label first. This is the only
change outside `scale` and `spec`, and it is a repair rather than a feature —
the pass was always about which labels are adjacent *on screen*, and tick order
was standing in for that because until now the two could not disagree.

It is also the reason this record exists as a record: reading the code said the
work was a comparison in `Ticks`. Drawing the chart said otherwise.

## Consequences

| | |
|---|---|
| `scale` | `Reverse`, `Desc.Reverse`, `linear.device`; `Domain`, `LogDomain` and `SymLogDomain` order their bounds |
| `spec` | `"reverse"` on a linear scale, written only when it is set |
| `render` | the X label collision sweep runs in screen order |
| `geom`, `stat`, `coord`, `ir`, `internal/layout`, `figure` | unchanged |
| Charts unlocked | any reading whose small numbers belong at the far end: an UpSet plot's set-size panel, a depth below a surface, a rank where first is best, a countdown |

**A reversed axis costs nothing to draw.** No branch was added to any drawing
path; the one that existed in `Map` moved into `device`.

**`Domain(hi, lo)` is now an ordinary axis.** Nobody could have been relying on
the old behaviour — it drew the marks mirrored and the axis blank — but it did
change, and this is where that is written down.

## Not in scope

- **Log, symlog, time and probability axes.** The flip is `device()` and three
  call sites in each, and nothing about it is linear-specific; what is missing
  is a chart asking for one and the tests that go with it. Their pinned domains
  are ordered by claim 1 either way, so none of them half-works in the meantime.
- **An ordinal reverse.** A band scale's slots go to categories in the order it
  meets them, so turning the axis around is a question about the category order
  rather than about the axis. `geom.Order` is where that is asked, and
  [ADR 0074](0074-sets-are-counted.md) has the argument about asking it on two
  marks at once.
- **A reversed axis in a projected scene.** `figure/three` takes its axes from
  the same scales and so presumably inherits this. Presumably is not a claim:
  nothing here was tested against a scene.
- **Reversing an axis by writing its domain backwards.** Refused by claim 1, and
  the reason is measured rather than argued.

## Revisit if

- **A second scale kind wants it.** The shape is claim 2 and the word is already
  in the dialect; what a new kind needs is its own `device` and its own tests.
- **The rim/hub clause in 0039 comes due.** This record is the answer that clause
  named, so an icicle with its root at the top is now a question about the axis
  rather than about the mark. It has not been asked, and nothing here draws one.
- **A second collision pass turns up with the same assumption.** The Y labels
  have no equivalent sweep today because they sit on separate rows. One that
  arrives must be written against screen order from the start, per claim 4.

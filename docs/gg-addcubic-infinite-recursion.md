# `EdgeBuilder.addCubic` recurses forever on large coordinates → stack overflow

**Package:** `github.com/gogpu/gg` v0.52.5, `internal/raster/edge_builder.go`
**Severity:** crash (unrecoverable — `fatal error: stack overflow` cannot be recovered)
**Observed:** 2026-09-17, Windows/amd64, Go 1.27, via `github.com/timzifer/figure` v0.11.0 → `figure/backend/gg` v0.9.0

## Summary

`addCubic` subdivides a cubic until a deviation test is satisfied, but has **no
recursion depth limit**. Once the coordinates are large enough that one float32
ULP exceeds the flatness tolerance, de Casteljau's midpoints round back onto the
endpoints. The child curve is then bit-identical to its parent, the deviation
test yields the same answer, and the recursion never terminates.

Every other subdivision routine in the same file is bounded by `if depth > 10`.
`addCubic` — the only one that recurses on itself — is not.

## Evidence

Goroutine 1 of the crash dump, 55 consecutive frames with **byte-identical
arguments**:

```
fatal error: stack overflow
runtime: goroutine stack exceeds 1000000000-byte limit

goroutine 1 gp=0x31c09db16000 m=0 [running, locked to thread]:
raster.(*EdgeBuilder).addCubic(0x…, 0x4a0bba33, 0x43817f0a, 0x4a0bba34, 0x43817f0a, 0x4a0bba34, 0x43817f0a, 0x4a0bba34, 0x43817f0a)
	edge_builder.go:969 +0x2bf
raster.(*EdgeBuilder).addCubic(0x…, 0x4a0bba33, 0x43817f0a, 0x4a0bba34, 0x43817f0a, 0x4a0bba34, 0x43817f0a, 0x4a0bba34, 0x43817f0a)
	edge_builder.go:969 +0x2bf
…  (×55, identical)
```

Line 969 is the first of the two self-calls in the deviation branch.

Decoding the arguments:

| bits | value | note |
|---|---|---|
| `0x4a0bba33` | `2289292.75` | `x0` |
| `0x4a0bba34` | `2289293.00` | `c1x`, `c2x`, `x1` |
| `0x43817f0a` | `258.992493` | all four `y` |

`ULP(2289292.75) = 0.25`, and `x1 - x0` is exactly **one ULP**. All four control
points are collinear and all `y` are equal — this is a straight, axis-parallel
segment, a degenerate cubic.

The frames just above show the descent into the fixed point; `y` halves each
level until all four coincide, at which point the arguments stop changing:

```
…, 0x4a0bba34, 0x43817f5b, 0x4a0bba34, 0x43817fac, 0x4a0bba34, 0x43817ffe)
…, 0x4a0bba34, 0x43817f32, 0x4a0bba34, 0x43817f5b, 0x4a0bba34, 0x43817f84)
…, 0x4a0bba34, 0x43817f1e, 0x4a0bba34, 0x43817f32, 0x4a0bba34, 0x43817f46)
…, 0x4a0bba34, 0x43817f14, 0x4a0bba34, 0x43817f1e, 0x4a0bba34, 0x43817f28)
…, 0x4a0bba34, 0x43817f0f, 0x4a0bba34, 0x43817f14, 0x4a0bba34, 0x43817f19)
…, 0x4a0bba34, 0x43817f0c, 0x4a0bba34, 0x43817f0f, 0x4a0bba34, 0x43817f12)
…, 0x4a0bba34, 0x43817f0b, 0x4a0bba34, 0x43817f0c, 0x4a0bba34, 0x43817f0e)
…, 0x4a0bba34, 0x43817f0a, 0x4a0bba34, 0x43817f0b, 0x4a0bba34, 0x43817f0c)
…, 0x4a0bba34, 0x43817f0a, 0x4a0bba34, 0x43817f0a, 0x4a0bba34, 0x43817f0a)   ← fixed point
```

So the subdivision works correctly right up to the ULP floor, then stalls.

## Why it never terminates

`edge_builder.go:931-973`:

```go
func (eb *EdgeBuilder) addCubic(x0, y0, c1x, c1y, c2x, c2y, x1, y1 float32) {
	…
	d1x := c1x - (x0*2+x1)/3
	…
	const maxCubicDevSq = 0.1 * 0.1
	if devSq > maxCubicDevSq {
		m01x := (x0 + c1x) * 0.5
		…
		eb.addCubic(x0, y0, m01x, m01y, m012x, m012y, mx, my)      // :969
		eb.addCubic(mx, my, m123x, m123y, m23x, m23y, x1, y1)      // :970
		return
	}
```

With `x0 = 2289292.75`, `c1x = c2x = x1 = 2289293.0`:

1. `d1x = c1x - (x0*2 + x1)/3`. In float32 this evaluates to `0.25`, so
   `devSq = 0.0625 > 0.01` → **subdivide**.
2. `m01x = (x0 + c1x) * 0.5 = 4578585.75 * 0.5`. `4578585.75` is not
   representable (ULP is `0.5` at that magnitude); it rounds, and the halved
   result lands back on either `x0` or `x1`. Every other midpoint does the same.
3. The child therefore has the same `{x0, B, B, B}` shape as the parent, and
   step 1 gives the same answer. Fixed point.

The tolerance is an **absolute** `0.1 px`, but the arithmetic is float32. Once
`ULP(x) > 0.1` the deviation can no longer be reduced by subdivision. That
threshold is:

```
ULP(x) = 2^(floor(log2 x) − 23) > 0.1   ⟺   x ≥ 2^20 = 1 048 576
```

So **any coordinate at or beyond ~1.05 M px can hang**; ours was 2.29 M.

## Two things that would have stopped it

1. **No depth bound on `addCubic`.** Its four siblings in the same file all have
   one, and they all fall back to emitting a straight line:

   | function | line | guard |
   |---|---|---|
   | `flattenQuadToVelloLinesRecursive` | 834 | `if depth > 10` |
   | `flattenQuadRecursive` | 887 | `if depth > 10` |
   | `flattenCubicToVelloLinesRecursive` | 1027 | `if depth > 10` |
   | `flattenCubicRecursive` | 1083 | `if depth > 10` |
   | **`addCubic`** | **931** | **none** |

   This looks like an oversight rather than a decision.

2. **The existing overflow guard is unreachable in this path.** `addCubic` has

   ```go
   // Safety guard: … force-flatten curves that extend beyond clip bounds …
   // This prevents FDot6→FDot16 overflow in NewCubicEdge. (RAST-010)
   if eb.hasClipRect && !eb.curveBBoxInsideClip(…) {
   	eb.flattenCubicToLines(…)
   	return
   }
   ```

   but it sits **after** the deviation branch (`:975`, vs `:945`). A curve that
   is both out of clip bounds *and* too deviant never reaches it — it recurses
   first. Since a 2.29 M px coordinate is certainly outside any clip rect, RAST-010
   would have caught exactly this case had the check come first.

## Suggested fix

Minimal, matching the file's own convention — thread a depth through and bail to
a straight line, since at the ULP floor the curve *is* a straight line:

```go
func (eb *EdgeBuilder) addCubic(x0, y0, c1x, c1y, c2x, c2y, x1, y1 float32) {
	eb.addCubicDepth(x0, y0, c1x, c1y, c2x, c2y, x1, y1, 0)
}

func (eb *EdgeBuilder) addCubicDepth(…, depth int) {
	…
	if devSq > maxCubicDevSq && depth <= 10 {
		…
		eb.addCubicDepth(…, depth+1)
		eb.addCubicDepth(…, depth+1)
		return
	}
```

Worth considering in addition:

- **Move the clip-bbox check above the deviation branch.** It is cheaper than a
  subdivision and it is the guard that was written for out-of-range coordinates.
- **Make the tolerance relative, or reject non-finite/huge inputs at the entry
  point.** An absolute `0.1 px` tolerance is meaningless once the coordinate
  magnitude puts the ULP above it. A cheap entry check (`|x| > 1<<20` →
  flatten to lines, or clip first) removes the whole class.
- A depth bound alone is enough to turn the crash into a harmless straight line,
  which is all such a segment can render as anyway.

## How we got there (for the `figure` side)

The coordinates were device-space pixels. Our chart put a view domain of roughly
`0..1` on a panel whose data spanned `0..28800` (seconds of an 8-hour axis), so
a ~800 px wide plot mapped points out to millions of pixels.

That was our bug and it is fixed on our side. But it is worth noting what it
implies for `figure`:

- `figure` happily rendered a frame whose data/view ratio was ~30 000:1 and
  handed the resulting coordinates to the backend. A sanity clamp (or at least a
  cheap "is this frame drawable" check) in `Live.Draw`, or clipping in
  `backend/gg`'s `Polyline` before the points reach the rasterizer, would
  contain a whole class of caller mistakes.
- `figure.View` carries panel domains but exposes only `Empty()` and `Panels()`.
  From the outside there is no way to tell a usable view from a degenerate one,
  so a caller that synchronises views between charts (which is exactly what
  `Live.View`/`Live.SetView` invite) cannot defend itself. A predicate — or
  accessors for the axis domains — would let callers refuse a view instead of
  discovering the problem as a stack overflow.

The crash is in `gg`, but the missing guardrail is arguably in both places: `gg`
should not be able to hang on any input, and `figure` should make it hard to
hand it nonsense.

## Reproducer sketch

Not yet reduced to a standalone test; the following should be enough, since the
fixed point is reached directly:

```go
eb := raster.NewEdgeBuilder(…)     // flattenCurves = false, so addCubic is used
const a, b = 2289292.75, 2289293.0 // 1 ULP apart at this magnitude
eb.addCubic(a, 258.99, b, 258.99, b, 258.99, b, 258.99)
// expected: terminates
// actual:   fatal error: stack overflow
```

Any pair one ULP apart above `2^20` should behave the same; the `y` values only
need to be equal.

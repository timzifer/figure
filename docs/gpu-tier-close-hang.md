# A hang in the GPU tier's Close, after the tier is switched off and on

Status: open, watched rather than reported. Last looked at 2026-09-11, on
gg fork `v0.52.6-figure.4`, wgpu v0.32.1, NVIDIA GeForce RTX 3070 Laptop,
driver 546.30, Windows 11.

## What happens

`gpu.Close()` never returns. It blocks in the Vulkan driver, in
`vkDestroyInstance`, reached this way:

```
figure/backend/gg/gpu.Close
gg.CloseAccelerator
internal/gpu.(*SDFAccelerator).Close
internal/gpu.(*GPUShared).Close
wgpu.(*Instance).Release
wgpu/core.(*Instance).Destroy
wgpu/hal/vulkan.(*Instance).Destroy
wgpu/hal/vulkan/vk.(*Commands).DestroyInstance
goffi/ffi.CallFunction        <- the call that does not come back
```

It shows up as `TestTheTierPutsDownAsMuchInkAsTheCPU` hanging in
`backend/gg/gpu`, in roughly half of the runs, and only on a machine where the
tier actually took. CI has no GPU, so CI never sees it. What it costs in
practice is that `go test ./backend/gg/gpu` hangs locally about every second
run, and that a program offering a GPU on/off switch can hang when it exits.

## What triggers it

The smallest sequence that hangs:

1. draw a chart on the GPU tier,
2. `gpu.Disable()` — which releases the device and the instance,
3. a garbage collection,
4. `gpu.Enable()` — the probe draw alone is enough,
5. `gpu.Close()` — hangs.

The garbage collection is the part that decides, and so is where it falls
relative to the rest. Each row is four runs of the same sequence:

| Sequence | Result |
| --- | --- |
| The test's own order, GC left to itself | hangs in about half the runs |
| The same, with `GOGC=off` | 4 of 4 clean |
| GC forced right after `Disable`, then GC off | 4 of 4 hang |
| GC forced only after the second cycle has drawn | 4 of 4 clean |
| `Disable`/`Enable` with no draw before them | 4 of 4 clean |

So it takes a real draw on the first device, and a collection that happens
while the second device is being built rather than after it. That is the shape
of memory the driver still points at: the collection frees it, the second cycle
allocates over it, and the instance is torn down over the top of whatever ended
up there. It is not a Go-level deadlock — the process sits in the driver call
with nothing else running.

## What it is not

- **Not the stroke fix.** It reproduces with the convex fast path change
  (`v0.52.6-figure.4`) and without it, and the draw that precedes it can be any
  chart.
- **Not wgpu's GC cleanups.** `Buffer` and `BindGroup` register
  `runtime.AddCleanup` handlers that log when they fire. They never fire in a
  hanging run; every one of them is released explicitly.
- **Not gg's coverage filler.** `AdaptiveFiller` never touches the device; it is
  CPU rasterisation.
- **Not stale Vulkan entry points.** `vk.Commands` reloads every instance-level
  pointer per instance; only `vkGetInstanceProcAddr` is cached across them.
- **Not [gogpu/wgpu#356](https://github.com/gogpu/wgpu/issues/356).** That bug
  is real and present in v0.32.1 — `updateDescriptorSet` builds its
  `bufferInfos`/`imageInfos` slices with capacity 0, so each `append` leaves the
  pointers already stored in earlier writes dangling. Rebuilding wgpu with both
  slices preallocated does not stop the hang (3 of 4 runs still hang).
- **Not the memory `vkCreateInstance` is handed.** Keeping the application and
  engine names, the create infos and the extension and layer arrays alive for
  the life of the process does not stop it either.

## Where that leaves it

The remaining suspects are another dangling pointer of the same class elsewhere
in `hal/vulkan`, something in goffi's call path, or the driver itself holding a
pointer it should not across instances. Narrowing it further means bisecting
wgpu and goffi, and the driver on this machine is from late 2023, so a driver
update is worth trying before that.

Nothing in figure can work around it. A GPU on/off switch has to give the device
back, and gg has no way to unregister an accelerator without closing it:
`RegisterAccelerator` closes the one it replaces and `CloseAccelerator` is the
only way out.

## Reproducing it

In `backend/gg/gpu`, with the tier enabled:

```go
render(t)            // any chart, on the tier
gpu.Disable()
runtime.GC()
runtime.GC()
debug.SetGCPercent(-1)
if !gpu.Enable() {
	t.Fatal("Enable failed")
}
gpu.Close()          // does not return
```

Run it with `-timeout`, or the test binary sits there until it is killed. The
goroutine dump the timeout prints is the stack at the top of this note.

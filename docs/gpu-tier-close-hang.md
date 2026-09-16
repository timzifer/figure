# A hang in the GPU tier's Close, after the tier is switched off and on

Status: open in the driver, worked around in the tier — `backend/gg/gpu` now
runs every device call on one locked OS thread, and `gpu.Do` is how a program
puts its rendering there too (see "The workaround the tier now carries").
Watched rather than reported. Retested on 2026-09-14 with
gg fork `v0.52.6-figure.5`, wgpu v0.32.1, goffi v0.6.3 and Go 1.25.3,
NVIDIA GeForce RTX 3070 Laptop, driver 546.30, Windows 11. The original
2026-09-11 observations below used `v0.52.6-figure.4`. The newer native
stack and counter-experiments are recorded at the end of this note.

**The follow-up lives on a branch.** `gpu-close-hang-repro` carries the
2026-09-15 native isolation: `docs/repro/vulkan-thread-close`, a standalone
Windows x64 C diagnostic that reproduces the same teardown failure with no Go,
no renderer, no window, no surface and no submission — instance and device
created and destroyed across two native worker threads, over two cycles, with
a watchdog runner and the run's `results.json`. It settles that the Go bindings
are not needed to trigger the failure and that garbage collection is not
either, and it does not identify the invalid allocation. It is unmerged
because it is a diagnostic rather than a change to the library; this note's
amendments travel with it.

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

In these original runs, a real draw and the position of the collection were
strong triggers. A stale pointer becoming invalid after collection or reuse
is one explanation, but the matrix does not prove it: collection also changes
scheduling. The Go dump locates the blocked call; it does not account for
native threads or locks. The native stack collected on 2026-09-14 below is
more specific about where the call stops.

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

The native DLL teardown path described below is now the first lead. An earlier
invalid pointer or resource operation in the bindings is still possible:
finding the native failure site does not establish who first corrupted its
state. A driver comparison is still outstanding; this machine continues to
use driver 546.30 from late 2023.

The switch keeps its promise on this machine now, by the thread workaround
below, not by anything having been fixed underneath it. gg has no way to
unregister an accelerator without closing it: `RegisterAccelerator` closes
the one it replaces and `CloseAccelerator` is the only way out. That limits
the current API; it is not proof that no workaround can exist. The experiments
below also distinguish releasing a GPU device from unloading its driver DLL.

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

## Counter-experiments on 2026-09-14

These used a temporary external-package test added with `go test -overlay`.
The test requires the GPU probe to succeed, renders the existing `render(t)`
chart, calls `Disable`, forces two collections, disables automatic GC, runs
the second probe and closes the tier. Every sample is a separate process.
Production source and dependency pins were not changed.

The initial matrix used an 8-second Go test timeout and an additional
20-second process-tree watchdog. The first standalone reproduction used a
12-second Go timeout. Some native hangs survive the Go timeout, so the outer
watchdog is necessary. Later checks allowed 45 seconds overall and up to
20 seconds inside the test. Samples with no test-start output are inconclusive
and are not counted as successful counter-experiments.

| Change to the reproducer | Observed result |
| --- | --- |
| Reuse the accelerator, ordinary GC settings until the forced collections | 1 hang in `DestroyInstance`; 1 unsuccessful exit after entering final `Close`, without a stack |
| Same, `GODEBUG=clobberfree=1` | 1 hang in `DestroyInstance`, 1 clean exit |
| Fresh accelerator and fresh `GPUShared` for the second cycle | 2 of 2 hang in `DestroyInstance` |
| Fresh accelerator, `clobberfree=1` | 2 of 2 hang in `DestroyInstance` |
| Direct `syscall.SyscallN` for `vkDestroyInstance` only | 1 hang in `DestroyInstance`, 1 clean exit |
| Extra reference to `NvCameraAllowlisting64.dll`, released before final `Close` | 4 hangs in `DestroyInstance`, 2 clean exits across 3 ordinary and 3 clobber runs |
| Extra camera DLL reference, intended release after final `Close` | 2 of 2 hang inside `Close`, before the extra reference is released |
| Extra camera DLL reference retained for process lifetime | 5 watchdog terminations after entering final `Close`, 1 clean exit across 3 ordinary and 3 clobber runs; 4 terminations have `DestroyInstance` stacks |
| Pin `nvoglv64.dll` for the test process lifetime | All 7 started samples exit cleanly: 3 ordinary, 4 clobber. One additional attempt timed out without test-start output and is inconclusive |
| Lock the reproducer goroutine to one OS thread | 8 of 8 exit cleanly: 5 ordinary, 3 clobber |

The fresh accelerator is a new zero-valued instance of the registered concrete
accelerator type, created through reflection and initialized through
`gg.RegisterAccelerator`. Its 16-by-16 stroke probe is checked for ink and its
context is closed before tier shutdown. The direct-syscall experiment uses
a temporary copy of wgpu selected through a separate workspace, because Go
does not permit overlays beneath `GOMODCACHE`. It changes only the final
Vulkan wrapper, not earlier FFI calls.

These observations argue against incomplete accelerator reset and the last
goffi argument-marshalling step as sole causes. `clobberfree` did not turn the
hang into an earlier, useful Go diagnostic. A later unmodified control also
timed out in `vkDestroyInstance` during the first `Disable`, so the second
cycle is not a necessary condition in every run.

The final control uses the same test source/build as the pinning and thread
experiments, with both interventions disabled. Of four attempts, one has no
test-start output; the other three fail to finish (two have `DestroyInstance`
stacks, one stops after logging entry to final `Close`). The successful
counter-experiments therefore are not just a consequence of rebuilding the
test. These are small samples, not estimates of a failure rate.

### Two successful counter-experiments

Thread affinity was applied at the beginning of the test function, before
the first real chart, and retained through the final `Close`:

```go
runtime.LockOSThread()
defer runtime.UnlockOSThread()

render(t)
gpu.Disable()
runtime.GC()
runtime.GC()
debug.SetGCPercent(-1)
if !gpu.Enable() {
    t.Fatal("Enable failed")
}
gpu.Close()
```

The package-init GPU probe still ran normally before this test. This does not
show that every Vulkan call must share a thread, nor that locking just `Close`
would help. It does show that keeping this whole sequence on one thread is a
useful candidate workaround.

## The workaround the tier now carries

`backend/gg/gpu` keeps one goroutine locked to an OS thread for the life of the
program, and runs everything that touches the device on it: the startup probe,
`Enable`, `Disable` and `Close`. `gpu.Do(func())` is the door, exported because
the draws have to go through the same thread as the teardown — a program that
renders on one thread and releases the device on another is the case that hangs.
The thread is never unlocked and outlives `Close`, because it is where the
driver's DLLs are unloaded.

Measured on the machine at the top of this note, with the package tests
rendering through `gpu.Do`:

| Run | Result |
| --- | --- |
| `go test ./backend/gg/gpu`, 8 processes | 8 of 8 clean, 1.5–2.3 s each |
| The reproducer sequence above (draw, `Disable`, two collections, GC off, `Enable`, `Close`), 6 processes | 6 of 6 reach `Close returned` |

The comparable unmodified control is in the table above: three of four attempts
did not finish. This is a workaround inside one package, not a fix: nothing here
explains the `RtlFreeHeap` failure in the native stack, and a program that draws
outside `gpu.Do` can still meet the hang.

The separate DLL experiment called Windows `GetModuleHandleExW` with
`GET_MODULE_HANDLE_EX_FLAG_PIN` (1) for the already loaded `nvoglv64.dll`, using
the absolute driver path observed in CDB. It leaves the driver loaded until
the test process exits. Both device/instance teardown cycles still execute,
and success requires the entire test process to exit, not just `Close` to
return. This is process-local diagnostic pinning, not a driver installation
or global settings change. It is not a portable library fix.

Together these results shift the investigation toward native DLL/FLS
teardown and thread scheduling. They do not rule out an earlier memory
lifetime error whose manifestation changes with either intervention.

### Native stack

CDB attached after the second probe returned and captured all native thread
stacks. The blocked thread was in this path (outermost call first; routine
names with only NVIDIA export symbols are approximate):

```text
vulkan-1!vkDestroyInstance
  loader_unload_preloaded_icds
  loader_clear_scanned_icd_list
  FreeLibrary -> ntdll DLL detach
  nvoglv64!DllMain / NVIDIA teardown
  FreeLibrary -> ntdll DLL detach
  NvCameraAllowlisting64
  FlsFree -> RtlpFlsFree
  NvCameraAllowlisting64 FLS cleanup -> RtlFreeHeap
  exception dispatch / unwind
  NvCameraAllowlisting64 -> FlsFree -> RtlpFlsFree
  RtlAcquireSRWLockExclusive
  RtlpAcquireSRWLockExclusiveContended
  NtWaitForAlertByThreadId
```

In particular, `FlsFree` appears twice on the same stack, with exception
unwinding between them. This is evidence of a native teardown/lock problem,
not merely an opaque call into the Vulkan driver. The snapshot does not prove
which allocation was invalid, who owned the lock, or whether earlier binding
code caused the native state to become invalid. A first-chance exception
capture would be needed to inspect the initial `RtlFreeHeap` failure directly.

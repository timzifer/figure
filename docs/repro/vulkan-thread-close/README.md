# Native Vulkan thread/close reproducer

This Windows x64 diagnostic reproduces a native teardown failure seen on an
NVIDIA GeForce RTX 3070 Laptop GPU, driver 546.30 (`31.0.15.4630`), Windows 11,
with the system Vulkan loader `1.3.300.0`. It needs no Go code, chart renderer,
window, surface, command buffer, submission, or custom allocator.
See [the investigation](../../gpu-tier-close-hang.md#native-isolation-completed-on-2026-09-15)
for observations and limits. This is a reproducer, not a production workaround.

## Build

Use a Windows x64 C compiler and the official Khronos Vulkan headers, either
from a Vulkan SDK or [Vulkan-Headers](https://github.com/KhronosGroup/Vulkan-Headers).
The experiment used MinGW-w64 GCC 13.2.0, `-O0`, and the official
[v1.2.131 headers](https://github.com/KhronosGroup/Vulkan-Headers/tree/v1.2.131/include/vulkan)
(`vulkan_core.h` and `vk_platform.h`). Only Vulkan 1.0 APIs are requested.
Headers are not vendored and no Vulkan import library is needed: the program
loads `vulkan-1.dll` from the Windows system directory.

From the repository root, with `gcc` on PATH and an installed Vulkan SDK:

```powershell
gcc -std=c11 -O0 -g -Wall -Wextra -Wno-cast-function-type `
    -I "$env:VULKAN_SDK\Include" `
    .\docs\repro\vulkan-thread-close\vulkan-thread-close.c `
    -o .\gpu-close-c-repro.exe
```

For standalone headers, replace the include path with the directory containing
`vulkan/vulkan_core.h`. The diagnostic is outside all Go modules' package builds;
it adds no dependency to figure.

## Run with a watchdog

```powershell
.\docs\repro\vulkan-thread-close\run.ps1 -Binary .\gpu-close-c-repro.exe
```

Every sample is a fresh process. The runner hides its console, logs stdout and
stderr to a new directory under TEMP, and kills only its own diagnostic process
if it has not exited after 15 seconds. It writes `results.json`; inspect
`Success`, `TimedOut`, and `ExitCode`. The script itself does not treat an expected
hang as a PowerShell failure. The native program creates no child processes.

Four letters select native worker A or B for these four phases, in order:

1. create the instance;
2. enumerate physical devices, inspect queue families, and create the device;
3. wait for the device to become idle and destroy it;
4. destroy the instance.

All phases are serialized using Windows events. Both threads remain alive
through two complete cycles. The first physical device is selected; on the
machine above it is the NVIDIA GPU. `SUCCESS` is printed after both cycles;
a passing sample also requires exit code zero.

`AAAA` is the control. `AAAB` changes only the instance-destruction thread and
hangs during the second cycle on the affected machine. `AABA` changes only the
device-destruction thread and exits cleanly. `AABB` also hangs. These are
observations of this installation, not portable expected outcomes.

Reduced controls, without creating a logical device:

```powershell
.\docs\repro\vulkan-thread-close\run.ps1 -Binary .\gpu-close-c-repro.exe -Modes AAAA,AAAB -Level instance
.\docs\repro\vulkan-thread-close\run.ps1 -Binary .\gpu-close-c-repro.exe -Modes AAAA,AAAB -Level enumerate
```

Both reduced levels exit cleanly here. Phase names stay the same in the log,
but device creation/destruction are skipped. The enumeration level still
calls `vkEnumeratePhysicalDevices`.

To rule out implicit layers, set these variables only in the test shell and
restore their previous values afterward:

```powershell
$oldDisable = $env:VK_LOADER_LAYERS_DISABLE
$oldDebug = $env:VK_LOADER_DEBUG
try {
    $env:VK_LOADER_LAYERS_DISABLE = '~implicit~'
    $env:VK_LOADER_DEBUG = 'warn,layer'
    .\docs\repro\vulkan-thread-close\run.ps1 -Binary .\gpu-close-c-repro.exe -Modes AAAA,AAAB
} finally {
    $env:VK_LOADER_LAYERS_DISABLE = $oldDisable
    $env:VK_LOADER_DEBUG = $oldDebug
}
```

Check stderr to confirm the installed loader honors the setting. In this
experiment it disabled `VK_LAYER_NV_optimus`, showed Application -> Loader ->
Driver with no intermediate layer, and the failure remained. This setting does
not stop the driver from loading its camera/capture DLLs internally. No explicit
validation layer was installed, so these results do not include validation-layer
verification. The environment variable is documented in the
[Khronos loader debugging guide](https://github.com/KhronosGroup/Vulkan-Loader/blob/main/docs/LoaderDebugging.md).

## Catch the error before it turns into a hang

With Windows Debugging Tools installed, launch this executable in CDB/WinDbg
with argument `AAAB`. Use `capture-first.txt` as the debugger command file
(CDB `-cf`) and `-logo` to save a log. For example, from an x64 debugger shell:

```powershell
cdb -cf .\docs\repro\vulkan-thread-close\capture-first.txt -logo .\native-close.txt .\gpu-close-c-repro.exe AAAB
```

The commands stop at the first access violation or heap-corruption exception,
print registers and stacks, then quit and terminate the diagnostic. CDB's zero
exit code therefore does NOT mean the reproduction passed. `!address @r8` and
`!heap -x @r8` are useful specifically when stopped in `RtlFreeHeap` on x64;
interpret them differently if another function faults. Run with an external
watchdog if automating the debugger. The supplied `run.ps1` is for the native
executable, not for CDB.

`native-first-fault.txt` contains an excerpt captured from the checked-in C
source's build with implicit layers disabled. `results.json` retains the
measured matrices, distinguishing the earlier split harness from verification
of the cleaned source. Full transient logs remain at the local investigation
path recorded in the main note; no report was posted externally.

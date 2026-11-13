// Deprecated: gg's GPU tier, deprecated with backend/gg; figure's raster backend is github.com/timzifer/figure/backend/stilus; removed in v0.20.0 (docs/adr/0094).
module github.com/timzifer/figure/backend/gg/gpu

go 1.25.0

// The GPU tier is a module of its own so that importing the raster backend
// cannot pull a GPU stack in by accident: a nested module is excluded from its
// parent's module graph, so backend/gg's dependencies stay gg, x/image and the
// core. See docs/adr/0022.
//
// gg is pinned to exactly the version backend/gg pins, because this module
// enables a tier inside that build of it rather than a separate renderer.
require github.com/gogpu/gg v0.52.5

// wgpu is held below v0.33, which turned the render-pass calls into struct
// arguments; gg v0.52.5 above still makes the old calls, so a newer wgpu fails
// to build gg rather than this package. That is also why this module has a
// workspace of its own, go.work beside this file: the window is on wgpu v0.34
// now, and one workspace cannot hold both. See .github/dependabot.yml.
require (
	github.com/gogpu/wgpu v0.32.1
	github.com/timzifer/figure v0.9.0
	github.com/timzifer/figure/backend/gg v0.8.0
)

require (
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gpucontext v0.29.0 // indirect
	github.com/gogpu/gputypes v0.6.0 // indirect
	github.com/gogpu/naga v0.19.0 // indirect
	golang.org/x/image v0.45.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

// gg is replaced by a fork carrying GPU-tier fixes that are still open upstream
// (gogpu/gg#534, #537, #538, #539, #540). This is the only module that needs
// them: every one changes what gg's GPU accelerator draws, and this module is
// what registers that accelerator — the raster backend and the window backend
// draw on the CPU. This module's own go.work applies the replacement while it
// is built; the repository's workspace does not contain it.
//
// A replace does not reach anyone who imports this module, and the release
// check refuses one, so the tier is held out of releases (-skip backend/gg/gpu)
// until a gg release carries the fixes. Then this goes, and the require above
// moves to that release.
replace github.com/gogpu/gg => github.com/timzifer/gg v0.52.6-figure.5

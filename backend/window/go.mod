module github.com/timzifer/figure/backend/window

// backend/stilus requires Go 1.27.1, for its font library, and this module
// draws with it (docs/adr/0094).
go 1.27.1

// GoGPU is pinned to an exact version: the stack is young, and a release of
// this backend is validated against exactly one release of the window layer
// and says which.
//
// The pin used to have a ceiling that was gg's rather than the window's: gg
// v0.52.5 does not build against wgpu v0.33 or later, so gogpu could not move
// past v0.53.1. The chart is drawn by backend/stilus now and gg is not in this
// module's graph, so the ceiling is gone — but lifting the pin is a change of
// its own, validated on its own, and not a side effect of this one.
//
// backend/stilus has no tag yet, so it cannot be required: a require line
// naming a version that does not exist breaks the workspace as well, because
// the module graph is read before the workspace replaces it. Until its first
// release the workspace provides it, and this module's release check fails —
// which is what that check is for: it says to release backend/stilus first and
// then require it here.
require (
	github.com/gogpu/gogpu v0.53.1
	github.com/timzifer/figure v0.14.0
)

require github.com/gogpu/gpucontext v0.29.0

require (
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gputypes v0.6.0 // indirect
	github.com/gogpu/naga v0.19.0 // indirect
	github.com/gogpu/wgpu v0.32.1 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

module github.com/timzifer/figure/backend/window

// backend/stilus requires Go 1.27.1, for its font library, and this module
// draws with it (docs/adr/0094).
go 1.27.1

// GoGPU is pinned to an exact version: the stack is young, and a release of
// this backend is validated against exactly one release of the window layer
// and says which.
//
// v0.54.1 is the release whose wgpu, gputypes and gpucontext are the line
// stilus/gpu is built on (wgpu v0.34, gputypes v0.8.2). The pin used to sit at
// v0.53.1 under a ceiling that was gg's rather than the window's — gg v0.52.5
// does not build against wgpu v0.33 or later — and went when the chart moved to
// backend/stilus (docs/adr/0094). The deprecated GPU tier keeps that ceiling,
// which is why it has a workspace of its own.
//
// backend/stilus has no tag yet, so it cannot be required: a require line
// naming a version that does not exist breaks the workspace as well, because
// the module graph is read before the workspace replaces it. Until its first
// release the workspace provides it, and this module's release check fails —
// which is what that check is for: it says to release backend/stilus first and
// then require it here.
require (
	github.com/gogpu/gogpu v0.54.1
	github.com/gogpu/gpucontext v0.31.3
	github.com/timzifer/figure v0.14.0
)

require (
	github.com/go-webgpu/goffi v0.6.8 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gputypes v0.8.2 // indirect
	github.com/gogpu/naga v0.21.4 // indirect
	github.com/gogpu/wgpu v0.34.9 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

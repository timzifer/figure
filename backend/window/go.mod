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
require (
	github.com/gogpu/gogpu v0.54.1
	github.com/gogpu/gpucontext v0.32.0
	github.com/gogpu/gputypes v0.8.2
	github.com/gogpu/wgpu v0.34.14
	github.com/timzifer/figure v0.15.0
	github.com/timzifer/figure/backend/stilus v0.15.1
	github.com/timzifer/stilus/gpu v0.8.1
)

require (
	github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707 // indirect
	github.com/go-opentype/fonts v0.14.0 // indirect
	github.com/go-opentype/opentype v0.21.1 // indirect
	github.com/go-webgpu/goffi v0.6.8 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/naga v0.21.10 // indirect
	github.com/timzifer/stilus v0.19.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

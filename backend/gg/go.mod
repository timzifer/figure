// Deprecated: use github.com/timzifer/figure/backend/stilus; this module is removed in v0.20.0 (docs/adr/0094).
module github.com/timzifer/figure/backend/gg

go 1.25.0

// GoGPU is pinned to an exact version. The stack is young and moves fast, so
// each release of this backend is validated against exactly one gg release and
// says which.
require (
	github.com/gogpu/gg v0.52.7
	github.com/timzifer/figure v0.12.0
	golang.org/x/image v0.45.0
)

require (
	github.com/gogpu/gpucontext v0.31.3 // indirect
	github.com/gogpu/gputypes v0.8.2 // indirect
	golang.org/x/text v0.41.0 // indirect
)

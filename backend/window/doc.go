// Package window draws figure charts in a native window.
//
// It is the fourth surface, after the two vector emitters and the browser
// canvas: a chart on a desktop, panned and zoomed with a mouse rather than
// exported and opened. The window itself comes from gogpu — windowing, input
// and lifecycle across Windows, macOS, Linux/X11 and Linux/Wayland, with no
// cgo — and the chart is rasterized by the same backend that writes a PNG,
// composited onto a GPU texture and presented from there.
//
//	w := window.New(window.Title("Signal"), window.Size(900, 600))
//	live, _ := p.Live(w.Target())
//	defer live.Close()
//	in := live.Input()
//
//	err := w.Run(window.Handler{
//	    Frame:   live.Draw,
//	    Move:    func(x, y float64) { in.Move(x, y) },
//	    Press:   func(x, y float64) { in.Down(x, y) },
//	    Release: func(x, y float64) { in.Up(x, y) },
//	    Scroll:  func(x, y, d float64) { in.Wheel(x, y, d) },
//	    Resize:  func(w, h int) { in.Resize(w, h) },
//	})
//
// Package show is that same wiring in one call, and is what most callers want.
//
// # How the chart reaches the screen
//
// There is one implementation of every mark, and a window shows what a file
// would: the chart is drawn by backend/stilus, the rasterizer that writes the
// documentation's PNGs (docs/adr/0094). stilus rasterizes on the CPU — it
// computes which pixels a path covers and by how much — and stilus/gpu
// composites those spans onto a GPU texture, which a presenter then draws into
// the window's own render pass. No image is built in memory and none is
// uploaded; what crosses to the GPU is the spans.
//
// The window's surface is deferred. A chart draws in event handlers too — a
// resize, a pan, a wheel notch — and there is no GPU frame there to draw into,
// so the surface keeps the latest frame as IR and the window composites it in
// its draw callback. It does so only when the frame changed: figure repaints
// nothing when a frame is identical to the last, the surface stamps each frame
// that changed it with a generation, and the window compares two integers.
// Between that and an event-driven loop that blocks on the operating system's
// queue, a chart nobody is touching costs no CPU at all.
//
// A test composites a chart onto a GPU texture, reads it back and compares it
// with the same chart rasterized into memory. It runs where there is a GPU and
// skips where there is not, which is every CI runner.
//
// # A separate module
//
// Like backend/stilus, this is a nested module: importing figure's core still
// yields a dependency graph with nothing in it but the standard library, and a
// program that renders SVG on a server links no window layer. It depends on
// backend/stilus as well as on the core, because the rasterizer is where the
// marks are drawn — and so it declares the Go 1.27.1 that module does.
//
// # Status
//
// The window layer is young, and so is the GPU stack under it. A chart in a
// window is v0.6's newest surface and the least proven one; the vector
// emitters remain the supported path for output that has to be right.
package window

// Package three draws a chart whose x, y and z are all data.
//
// It is the second of the three features ADR 0055 split "3D" into: a surface
// over a grid, a trajectory through a volume, a field of bars over two
// categoricals — charts where the third dimension carries a reading the flat
// chart of the same table cannot. The camera that turns them is ADR 0057, and
// what the dimension is for is ADR 0058.
//
// # Shape of the API
//
// A [Scene] is what is plotted. A [View] is a camera on it. A [Plot] is the
// figure they are drawn into:
//
//	sc := three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("gain"))
//	sc.Z(scale.Linear(scale.Nice()))
//	sc.Add(three.Surface(src, geom.X("x"), geom.Y("y"), geom.Z("gain")))
//
//	p := three.New(three.Size(720, 520), three.Title("Response"))
//	p.Scene(sc)
//	err := p.Render(svg.File("surface.svg"))
//
// The split is the point. A scene holds no camera, so several views share one
// scene and the scales behind it are trained once however many angles are
// drawn from them — one data repository, several ways of looking at it:
//
//	p.Add(
//		three.View{Camera: three.Home(), Label: "three-quarter"},
//		three.View{Camera: three.LookAt(three.Azimuth(0)), Label: "front"},
//		three.View{Camera: three.LookAt(three.Elevation(math.Pi / 2)), Label: "top"},
//	)
//
// # Why this is its own package
//
// [github.com/timzifer/figure/render] walks a panel's layers and each layer's
// Build streams straight into the backend, so a layer is a paint unit. A
// projected scene has no paint unit smaller than the view: a point can be in
// front of one part of a surface and behind another, so correct occlusion is a
// single depth order over the primitives of every layer at once. Producing
// that inside render would need either a depth on every drawing call or two
// drawing orders, and both are refused. So the draw loop, the ordering, the
// cube's furniture and the camera live here, beside render rather than through
// it.
//
// A [Layer] is therefore not a
// [github.com/timzifer/figure/geom.Geom], and that is a guard as much as a
// consequence: a geom handed a depth would ignore it and draw a flat line
// inside a projected box — correct by its own lights, wrong by the chart's and
// silent either way. Here it cannot happen, because geom.Line does not compile
// into a Scene. Layers do read geom's option set, though, so a channel is
// spelled the way it is spelled everywhere else.
//
// # The IR gains nothing
//
// There is no ir.Point3, no 4x4 in the IR and no depth on a drawing call. This
// package owns its own [Vec3], its own matrix and its own [Camera], projects
// everything above the seam, and calls Polyline, FillPath and Text with the
// plain two-dimensional coordinates those have always taken. So every backend
// draws a surface on the day this package compiles: SVG, PDF, canvas, raster,
// the native window, the GPU tier. See docs/adr/0056-three-dimensional-charts.md.
//
// # Painting over a finished figure
//
// [Plot.Overlay] and [Live.Overlay] install something to paint after every view
// is drawn: a ring round the rows a reader picked, a leader line, a caption. It
// is where a host draws a selection, because there is no selection in here — a
// scene with four cameras shares its data by holding one pointer and nothing
// propagates between the views, which is the division
// docs/adr/0045-linked-views.md made for two charts and
// docs/adr/0062-a-scene-and-its-views.md restated for two cameras.
//
// An overlay is told where every view landed and, through [OverlayView.At], the
// pixel any value of the data was drawn at in it — which is the only direction
// that exists here, since a device point in a turned cube resolves to no triple
// of values. It is drawn last, clipped by nothing, and announced to no
// observer, so nothing it paints is hit-testable. See
// docs/adr/0063-an-overlay-over-a-scene.md.
//
// # What it does not do
//
// No perspective: under one, the same value is taller at the front of the
// scene than at the back, and a chart is a measuring instrument first. No
// lighting model beyond one directional shade per face. No arbitrary meshes —
// a painter's order is exact only over a set that can be totally ordered, and
// two triangles that interpenetrate have no correct order at all. No windows,
// no handlers, no event loop: a camera is a value the caller holds, and
// turning it is the host's loop.
package three

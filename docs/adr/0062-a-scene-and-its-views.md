# 0062 — A scene is the data, a camera is a way of looking at it, and one figure may hold several

**Status:** Accepted · **Date:** 2026-09-10 · **Implementation:** `three.Scene`, `three.View`, `three.Plot`

## Context

[ADR 0056](0056-three-dimensional-charts.md) builds the machinery for a chart
whose x, y and z are all data, and [ADR 0057](0057-orbiting-a-chart.md) makes
the camera an immutable value the caller holds. Both records describe **one**
scene seen from **one** camera throughout, and both are right about everything
they say.

What neither says is what happens when there are two.

The question is not hypothetical, and it is not really about 3D either. A
projected scene is genuinely hard to read from one fixed angle — 0057 opens by
saying so — and the answers to that are not only "let the reader turn it". A
static export cannot be turned. A printed page cannot be turned. A reader
comparing a designed part with a measured one wants both from the *same* angle,
not either from any angle. And a machinist reading a surface wants the plan and
the two elevations that an engineering drawing has had since before there were
computers, because three orthogonal views of one solid are a *measurement* in a
way that one three-quarter view is not.

So: several cameras, one scene. The obstacle is where the scene lives.

**On the plot, with a camera field.** `plot.Camera(cam)` is the smallest API
and the one 0057's sketch implies. It makes a second camera a second plot, and
a second plot has its own layers, its own scales and its own training — so two
views of "one" surface are two surfaces that happen to hold equal numbers, with
two domains that agree by luck. `scale.Scale.Train` accumulates, so even
*sharing* the scale objects between two plots would train them twice and move
the domain by however many cameras the author asked for. The bug is silent and
it is in the axis rather than in the picture.

**Two ends of a wire, as [ADR 0045](0045-linked-views.md) does for linked
views.** Give the caller everything needed to render one scene twice and let
the program hold it together. That is right for a *link* — a statement about
two charts — and wrong here, because this is not a statement about two charts.
It is one chart with four pictures of it, the way a facet grid is one chart
with nine.

**Split the value.** The data is one thing and the way of looking at it is
another, and they are already separate: 0057 made the camera a value precisely
so that it is not state of the scene. Taking that seriously costs one type.

## Decision

**A `Scene` is what is plotted. A `View` is a camera on a scene. A `Plot` is
the figure they are drawn into, and it may hold several views of one scene.**

```go
sc := three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("z")).
	Z(scale.Linear(scale.Nice())).
	Add(three.Surface(src, geom.X("x"), geom.Y("y"), geom.Z("z")))

three.New(three.Size(960, 720), three.Columns(2)).Scene(sc).Add(
	three.View{Camera: three.Home(), Label: "three-quarter"},
	three.View{Camera: three.LookAt(three.Elevation(1.45)), Label: "plan"},
	three.View{Camera: three.LookAt(three.Azimuth(0)), Label: "front"},
	three.View{Camera: three.LookAt(three.Azimuth(-math.Pi / 2)), Label: "side"},
).Render(svg.File("views.svg"))
```

The rule that decides which type a thing belongs to, and it is worth stating
because it answers the next question as well as this one:

> **A `Scene` answers *what is plotted*. A `View` answers *from where*. A
> `Plot` answers *how large and how it looks*.**

So the three axis titles are the scene's — an axis names the data, and every
view of one scene labels the same three. The theme and the chart title are the
plot's — a theme is how the figure looks and two views in one figure must share
it. The camera and the view's own label are the view's.

### Trained once, and that is a correctness property

**The scales behind a scene are trained once per frame, however many views are
drawn from them.** `scale.Scale.Train` accumulates by contract, so training per
view would give every layer as many times its weight as there are cameras — and
the visible symptom would be a domain that changes when an author adds a
picture. There is a test that counts the calls, because this is the kind of bug
that produces a chart which looks entirely reasonable and is wrong.

Emission is per view, and that is not an oversight. Which order a layer walks
its own geometry in is a fact about the camera — a surface starts at the corner
the view direction picks — so the geometry a view paints is the view's. The
cost is one data pass per view, and the alternative was an optional "my
emission does not depend on the camera" interface, which is a second path
through the painter that 0056 spends a version avoiding.

### A view may bring its own scene

`View.Scene` overrides the plot's, and is nil for the views that share it —
which is the ordinary case. It is the same mechanism read the other way round:
a view is a camera *and* what it is pointed at, and leaving the second half out
is the common case rather than the only one. That is what makes "the designed
part beside the measured one, from one angle" one figure rather than two.

### Placement is the plot's, not the view's

Views land in the order they were added, across the rows of a grid whose width
is `three.Columns`. A `Row`/`Col` pair on `View` was the alternative and is
worse: nought, nought is a real cell, so the struct would have no honest zero
value and every view would have to be placed explicitly. `figure.Grid` decided
this once already.

### The layout is the one solver

`internal/layout.Panels` lays the views out, and it is called for exactly what
it is the solver for: the outer margin, the chart title, the label strips and a
grid of equal cells. Its tick-label gutters are left empty, because a projected
cube's labels hang off its own edges *inside* the cell rather than in a
four-sided gutter, and telling the solver otherwise would be lying to it.

The room those labels need is measured once for the whole figure and given to
every cell. Two views of one scene drawn at two scales would be two pictures
that look comparable and are not, which is the same failure as two domains that
agree by luck, one level down.

### Turning several

`Live.Camera` points **every** view at one camera, because a synchronised orbit
of a front / plan / side figure is one statement about the chart;
`Live.SetCamera` points one. `Live.ViewAt` says which view a device position is
in, so a host that wants to turn the view under the pointer can. `Live.Home`
returns every view to the camera its author chose.

A turn damages the cells whose cameras moved rather than the whole surface,
which is what makes a four-view figure affordable to drag: three of them
genuinely did not change. 0057's amendment records that.

## Consequences

- **There is no scene registry and no shared selection.** Two views of one
  scene share the data because they hold the same pointer, and nothing
  propagates between them. Highlighting the row a pointer landed on in every
  view is the host's, exactly as ADR 0045 decided for two charts.
- **A `Scene` is not safe for concurrent modification**, which is the contract
  `figure.Plot` has. Rendering does not modify it, and there is a test.
- **`three.Live` copies the plot's views** so that turning a scene does not
  edit the specification the caller still holds.
- **Nothing about this appears in the JSON spec.** A document carrying a list
  of cameras would be a document about a picture rather than about a chart, and
  the dialect has no vocabulary for one — see
  [ADR 0056](0056-three-dimensional-charts.md) on `$schema` and
  [docs/spec.md](../spec.md).

## Revisit if

Two views of one scene need to *disagree* about something other than the
camera — a layer hidden in one, a sub-box of the cube in another. Both are
fields on `View` when they arrive rather than a second type, which is what the
struct is for.

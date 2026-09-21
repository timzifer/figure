package coord

import (
	"math"

	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// Geo reads a panel's two axes as a longitude and a latitude and places the
// pair by a map projection, which is the coordinate system every chart drawn
// on the world is in and the one this package deferred longest.
//
// X is the longitude in degrees east and Y the latitude in degrees north. The
// coord maps the pair through one of the projections [Projection] names — the
// equal-area one when the caller names none, see [DefaultProjection] — fits
// the image into the panel without stretching it, and draws the graticule out
// of the two axes' own ticks.
//
//	p := figure.New(figure.Coord(coord.Geo(coord.Mollweide)))
//	p.X(scale.Linear(scale.Domain(-180, 180), scale.TickStep(60)))
//	p.Y(scale.Linear(scale.Domain(-90, 90), scale.TickStep(30)))
//	p.Add(geom.Scatter(cities, geom.X("lon"), geom.Y("lat")))
//
// # Why this is a coord and not a mark
//
// A projection is a map of the sphere into the plane, and every mark drawn on
// it goes through that map: a dot for a city, a line for a route, a cell of a
// gridded field, the text of a place name. That is a stage between the scales
// and the IR, which is what a coord is — so a scatter is the mark that draws a
// point map, [github.com/timzifer/figure/geom.Rect] is the mark that draws a
// gridded one, and neither of them knows this chart exists.
//
// The graticule comes out of the same place a polar coord's rings do: a
// meridian is what a longitude tick looks like once the coord has had it, and
// a parallel is what a latitude tick looks like. Nothing in render changes, and
// the two tick lists a panel has are the two the map needs.
//
// # What the interval means
//
// [Geo.Frame] gives each scale the range its own domain already is — the move
// [Smith] makes, for the same reason and with more force: a projection is not
// separable, so the device X of a point depends on its latitude as well as on
// its longitude, and a pair that arrived as two independently mapped fractions
// could not be projected at all. What reaches [Geo.Point] is therefore the
// degrees themselves, which is also what makes the axes' ticks the graticule's
// levels rather than a second ladder that would have to be invented.
//
// It follows that an axis here wants a linear scale. A log or a symlog axis is
// not broken, it is a different chart — degrees measured on a warped rule —
// and the coord does not guess which one was meant.
//
// # What it does not do
//
// It ships no geography. A coastline, a border and a country's fill are rows
// in the caller's table like any other rows, and reading a shapefile or a
// GeoJSON document is a job for a package that is allowed dependencies. What
// this coord promises is that such rows land where the projection says.
//
// An edge between two rows is a straight line on screen by default, exactly as
// a measured sweep's is under a Smith chart: two samples of a track are what
// was measured, and the line between them is a convention. [GeoArc] asks for
// the other reading, where an edge is the image of the straight line in
// degrees — which is what the side of a graticule cell is, and what a boundary
// given by sparse vertices wants. Neither of them is a great circle: the
// shortest path over the sphere is a claim about the route rather than about
// the drawing, so it is rows a caller computes.
//
// A path that crosses the antimeridian is drawn across the whole map rather
// than wrapped round it, because a coord is handed a pair of numbers and two
// consecutive rows at 179° and −179° are two degrees apart or three hundred
// and fifty-eight, and nothing in the pair says which. Split such a track in
// the table, where the answer is known.
func Geo(p Projection, opts ...GeoOption) Coord {
	if p == "" {
		p = DefaultProjection
	}
	g := &geo{proj: p}
	for _, o := range opts {
		o(g)
	}
	return g
}

// Projection names the map projection a [Geo] coord draws, and the set is
// closed on purpose: a projection that travelled as a Go function could not be
// written down, and a chart that cannot be written down is a chart the spec
// cannot round-trip. It is [github.com/timzifer/figure/stat.Family]'s rule and
// [ADR 0041]'s, applied to the one stage of the pipeline that had not needed
// it yet.
//
// A projection this package does not define is a [Coord] of its own, built on
// the same seam and registered with [Register] — which is the extension model
// every other model type here has.
//
// [ADR 0041]: https://github.com/timzifer/figure/blob/main/docs/adr/0041-qq-plots.md
type Projection string

// DefaultProjection is what a map drawn without naming one is drawn in, and it
// is the equal-area projection rather than the simplest one.
//
// The other three all mislead about size — Mercator without bound, the
// equirectangular one by the cosine of the latitude, the globe by the
// foreshortening at its rim — and on a map, size reads as *quantity*: a reader
// comparing how much ground a thing covers is comparing ink. A projection that
// makes that comparison wrong is a fine thing to ask for by name, because the
// asking says the chart is about shape or about bearing instead. It is not a
// fine thing to get by default.
const DefaultProjection = Mollweide

// The projections.
const (
	// PlateCarree is the equirectangular projection: longitude and latitude
	// read straight off as x and y. It is the projection a table of degrees
	// is already in, it is the only one here that is affine — so an edge
	// straight in degrees is straight on screen — and it is neither conformal
	// nor equal-area, which is why it is a chart of where things are rather
	// than a chart of how large they are.
	PlateCarree Projection = "plate-carree"

	// Mercator is the conformal cylindrical projection: angles are preserved
	// everywhere, areas are not, and the exaggeration grows without bound
	// towards the poles. It is what a navigator's chart and every web map
	// tile are drawn in. Its latitudes are cut at ±85.051129°, which is where
	// the square tile of the web map convention ends and beyond which the
	// image runs off to infinity.
	Mercator Projection = "mercator"

	// Mollweide is the equal-area pseudocylindrical projection: every region
	// of the map covers the fraction of the page its share of the sphere,
	// bought with shapes that shear towards the corners. It is the projection
	// for a quantity measured per unit area — a field, a density, a count per
	// cell — because it is the one under which two cells of the same value
	// draw the same amount of ink.
	Mollweide Projection = "mollweide"

	// Orthographic is the view of the globe from infinitely far away: the
	// hemisphere facing the reader, drawn as a disc, with the far side not
	// drawn at all. It is neither conformal nor equal-area and it shows less
	// than half the world at once, which is the point — it is the projection
	// that reads as a planet rather than as a rectangle, and the one that
	// tells a reader where the near edge of the world is.
	Orthographic Projection = "orthographic"
)

// Known reports whether p is one of the projections this package draws.
//
// A projection it does not know draws no map at all rather than the nearest
// one it does: every position on a map is a projection's worth of wrong under
// another projection, and a chart that quietly drew the wrong one would be a
// chart nobody can tell is wrong. [FromDesc] refuses such a name outright,
// which is the same answer given where there is somewhere to put it.
func (p Projection) Known() bool {
	switch p {
	case PlateCarree, Mercator, Mollweide, Orthographic:
		return true
	}
	return false
}

// GeoOption configures a [Geo] coord. It is named for the coord it configures
// rather than for the package, as [SmithOption] and [PolarOption] are.
type GeoOption func(*geo)

// GeoCenter turns the globe under the projection: the meridian at lon runs
// down the middle of the map, and — for [Orthographic], which is the only
// projection here with a centre in both directions — the parallel at lat runs
// across it.
//
// A cylindrical or pseudocylindrical projection is centred on the equator by
// construction, so lat reaches only the orthographic one. Moving it would not
// re-project such a map, it would slide the picture up the page.
func GeoCenter(lon, lat float64) GeoOption {
	return func(g *geo) { g.lon0, g.lat0 = lon, lat }
}

// GeoArc draws an edge between two marks as the image of the straight line
// between their degrees, which under every projection here but [PlateCarree]
// is a curve.
//
// It is what the side of a cell wants, and what a boundary given by a handful
// of vertices wants: those rows describe a shape whose edges are the parallels
// and meridians between them, and a chord across a curved one encloses the
// wrong ground. Leave the default for a measured track, where the samples are
// what is known.
func GeoArc() GeoOption { return func(g *geo) { g.arc = true } }

// GeoChord is the default edge policy, spelled out for a chart that wants to
// say so: an edge between two marks is the straight line between them on
// screen. See [GeoArc] for the other one.
func GeoChord() GeoOption { return func(g *geo) { g.arc = false } }

// geo is the coord itself and — once [geo.Frame] has been called — the fitted
// map. Frame returns a copy rather than moving the receiver, so two panels
// drawn on two goroutines never share one fit.
type geo struct {
	proj       Projection
	lon0, lat0 float64
	arc        bool

	// The fit and the domains, set by Frame. A projected point at (px, py)
	// lands at (ox + k*px, oy − k*py): one scale for both directions, which is
	// what keeps a map's shape.
	ox, oy         float32
	k              float32
	x0, x1, y0, y1 float32
	// relW and relE are the domain's longitudes measured from the centre
	// meridian, which is the frame everything the coord draws for itself is
	// walked in. See [geo.cut].
	relW, relE float32
	// boxed reports that the map's edge is the image of the domain rather
	// than the projection's own boundary, which is everything but a globe
	// whose domain runs over the horizon. See [geo.Clip].
	boxed  bool
	area   ir.Rect
	framed bool
}

// mercatorMaxLat is where a Mercator map is cut: the latitude whose image is
// π, which makes the world square and is the edge of every web map tile.
const mercatorMaxLat = 85.05112877980659

// geoStep is how finely the coord walks a curve of its own — a graticule line,
// the side of a cell, the outline of the map — in degrees. Two degrees is
// under a pixel of chord error on a map a thousand pixels wide, and the walk
// is bounded by geoMaxSteps so that a domain in the thousands of degrees, or
// one a zoom has left inverted, cannot turn a grid line into a long loop.
const (
	geoStep     = 2.0
	geoMaxSteps = 512
)

func (g *geo) Frame(f Framing) Coord {
	q := *g
	q.area = f.Area
	q.x0, q.x1 = identityRange(f.X)
	q.y0, q.y1 = identityRange(f.Y)
	q.cut()
	q.fit()
	q.framed = true
	return &q
}

// cut brings the domain back to the part of the sphere the projection has an
// image for: a turn of longitude about the centre, and the latitudes the
// projection reaches.
//
// It is the coord's own interval and not the scale's — the scale keeps the
// range its domain is, because that is what makes [geo.Point] a projection
// rather than a fraction — and it is what [geo.Extent] reports, so that a mark
// asked to span the axis spans the map rather than running off to the infinity
// a Mercator's pole is.
func (g *geo) cut() {
	lim := float32(90.0)
	if g.proj == Mercator {
		// A hair inside the cut rather than on it: the domain travels as a
		// float32, the nearest float32 to the cut is outside it, and a
		// latitude outside the cut has no image at all — so a map clamped to
		// the cut itself would have no southern edge to draw or to number.
		lim = float32(mercatorMaxLat) - 1e-4
	}
	g.y0, g.y1 = clamp32(g.y0, -lim, lim), clamp32(g.y1, -lim, lim)
	// A domain wider than a turn covers some ground twice, and the second
	// coat lands on top of the first, because a longitude is measured from
	// the centre meridian.
	if g.x1-g.x0 > 360 {
		g.x0, g.x1 = float32(g.lon0-180), float32(g.lon0+180)
	}
	// And the same domain measured from the centre, which is the interval the
	// coord walks its own curves in: a graticule line, the side of a cell and
	// the outline of the map all run west to east *on the map*, and a walk in
	// the caller's longitudes would jump the width of the picture wherever
	// theirs crosses the map's cut. A Pacific-centred world is the ordinary
	// case: its domain is −180° to 180° and its map runs from −30° to 330°.
	span := g.x1 - g.x0
	g.relW = float32(wrap180(float64(g.x0) - g.lon0))
	g.relE = g.relW + span
	if g.relE > 180 {
		// The domain straddles the map's own cut, so it is drawn in two
		// pieces with the whole turn between them. A world map centred
		// anywhere but the prime meridian is this, and so is a regional map
		// whose centre was chosen outside it.
		g.relW, g.relE = -180, 180
	}
}

// fit chooses the one scale factor and the offset that put the projected
// domain in the middle of the panel at the largest size that fits.
//
// The scale is one number rather than two because a map that had been
// stretched to fill its panel would be a different projection from the one it
// says it is — the whole content of choosing between them is the shape they
// make — so a panel wider than the map leaves the room over at the sides.
func (g *geo) fit() {
	lo, hi := g.bounds()
	if lo.X > hi.X || lo.Y > hi.Y {
		// Nothing in the domain has an image: an empty map, drawn as no map
		// rather than as a division by zero.
		g.k, g.ox, g.oy = 0, (g.area.Min.X+g.area.Max.X)/2, (g.area.Min.Y+g.area.Max.Y)/2
		return
	}
	w, h := hi.X-lo.X, hi.Y-lo.Y
	k := float32(math.Inf(1))
	if w > 0 {
		k = g.area.Dx() / w
	}
	if h > 0 {
		if kh := g.area.Dy() / h; kh < k {
			k = kh
		}
	}
	if math.IsInf(float64(k), 1) || k < 0 {
		// A domain with no extent in either direction: one point, placed in
		// the middle of the panel at no magnification at all.
		k = 0
	}
	g.k = k
	cx, cy := (g.area.Min.X+g.area.Max.X)/2, (g.area.Min.Y+g.area.Max.Y)/2
	g.ox = cx - k*(lo.X+hi.X)/2
	g.oy = cy + k*(lo.Y+hi.Y)/2
	g.boxed = g.proj != Orthographic || g.domainIsVisible()
}

// domainIsVisible reports whether every point of the domain's boundary has an
// image, which is what decides whether the map's edge is that boundary or the
// projection's own. It is false for a globe showing the whole world, where the
// domain runs over the horizon and the edge is the rim.
func (g *geo) domainIsVisible() bool {
	const n = 64
	west, east := float64(g.relW), float64(g.relE)
	south, north := float64(g.y0), float64(g.y1)
	for i := 0; i <= n; i++ {
		t := float64(i) / n
		lon, lat := lerp(west, east, t), lerp(south, north, t)
		for _, pair := range [4][2]float64{{lon, south}, {lon, north}, {west, lat}, {east, lat}} {
			if _, _, ok := g.project(pair[0], pair[1]); !ok {
				return false
			}
		}
	}
	return true
}

// bounds is the box the domain's image occupies, in the projection's own
// units, found by walking the domain rather than by a formula: every
// projection here has its own shape, and four corners are not the extremes of
// any of the curved ones.
func (g *geo) bounds() (lo, hi ir.Point) {
	lo = ir.Point{X: float32(math.Inf(1)), Y: float32(math.Inf(1))}
	hi = ir.Point{X: float32(math.Inf(-1)), Y: float32(math.Inf(-1))}
	// An odd number of samples, so that the lattice holds the domain's own
	// middle as well as its corners: the widest point of a Mollweide world is
	// on the equator and the corners miss it.
	const n = 32
	west, east := float64(g.relW), float64(g.relE)
	lat0, lat1 := float64(g.y0), float64(g.y1)
	for i := 0; i <= n; i++ {
		dlon := lerp(west, east, float64(i)/n)
		for j := 0; j <= n; j++ {
			lat := lerp(lat0, lat1, float64(j)/n)
			if x, y, ok := g.project(dlon, lat); ok {
				lo, hi = stretch(lo, hi, x, y)
			}
		}
	}
	// The rim of an orthographic hemisphere is where the sphere turns away,
	// and no sample inside the domain lands on it: a grid dense enough to find
	// it would be a grid that is mostly wasted. It is a circle of known
	// radius, so it is walked rather than searched for, and only the part of
	// it the domain actually holds is allowed to widen the map.
	if g.proj == Orthographic {
		lo, hi = g.stretchToRim(lo, hi)
	}
	return lo, hi
}

// stretchToRim widens the box to take in the visible rim wherever the domain
// reaches it.
func (g *geo) stretchToRim(lo, hi ir.Point) (ir.Point, ir.Point) {
	const n = 360
	lat0, lat1 := float64(g.y0), float64(g.y1)
	if lat0 > lat1 {
		lat0, lat1 = lat1, lat0
	}
	clat := rad(g.lat0)
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / n
		// The point 90° from the centre in azimuth a, which is the rim.
		lat := deg(math.Asin(clamp(math.Cos(clat)*math.Cos(a), -1, 1)))
		dlon := deg(math.Atan2(math.Sin(a), -math.Sin(clat)*math.Cos(a)))
		if lat < lat0 || lat > lat1 || dlon < float64(g.relW) || dlon > float64(g.relE) {
			continue
		}
		lo, hi = stretch(lo, hi, math.Sin(a), math.Cos(a))
	}
	return lo, hi
}

func (g *geo) Extent() (x0, x1, y0, y1 float32) { return g.x0, g.x1, g.y0, g.y1 }

// Point places a longitude and a latitude. A degree the projection has no
// image for — the far side of a globe, a latitude past a Mercator's cut — is
// NaN, which every backend and every scale here already treats as nothing to
// draw.
func (g *geo) Point(x, y float32) ir.Point {
	return g.place(g.forward(float64(x), float64(y)))
}

// pointAt places a pair already measured from the centre meridian, which is
// what everything the coord draws for itself works in. See [geo.cut].
func (g *geo) pointAt(dlon, lat float64) ir.Point {
	return g.place(g.project(dlon, lat))
}

// place puts a point of the projection's own space on the canvas, or answers
// NaN where the projection had no image for it.
func (g *geo) place(px, py float64, ok bool) ir.Point {
	if !ok {
		nan := float32(math.NaN())
		return ir.Point{X: nan, Y: nan}
	}
	return ir.Point{X: g.ox + g.k*float32(px), Y: g.oy - g.k*float32(py)}
}

func (g *geo) Points(dst []ir.Point, xs, ys []float32) []ir.Point {
	for i := range xs {
		dst = append(dst, g.Point(xs[i], ys[i]))
	}
	return dst
}

// Invert reads a device point back as a longitude and a latitude. A point
// outside the map — beside the disc of a globe, outside the ellipse of a
// Mollweide — has no degrees behind it and comes back as NaN, which is what a
// tooltip needs in order to say nothing.
func (g *geo) Invert(pt ir.Point) (x, y float32) {
	if g.k == 0 {
		return float32(math.NaN()), float32(math.NaN())
	}
	dlon, lat, ok := g.degrees(pt)
	if !ok {
		return float32(math.NaN()), float32(math.NaN())
	}
	return float32(wrap180(dlon + g.lon0)), float32(lat)
}

// Straight reports whether an edge that is straight in degrees is straight on
// screen. It is true under [PlateCarree], whose map is affine, and true for
// every projection under the default edge policy, where an edge between two
// rows is the chord between them. See [GeoArc].
func (g *geo) Straight() bool { return !g.arc || g.proj == PlateCarree }

// Edge appends the image of the straight line in degrees between two device
// points, which is a curve under every projection here but [PlateCarree].
func (g *geo) Edge(p *ir.Path, from, to ir.Point) {
	if g.Straight() {
		p.LineTo(to.X, to.Y)
		return
	}
	dlon0, lat0, ok0 := g.degrees(from)
	dlon1, lat1, ok1 := g.degrees(to)
	if !ok0 || !ok1 {
		p.LineTo(to.X, to.Y)
		return
	}
	g.walk(p, dlon0, lat0, dlon1, lat1, false)
}

// degrees reads a device point back as a pair measured from the centre
// meridian, which is what a construction over two points the caller already
// placed needs — and what [geo.Invert] turns back into the caller's own
// longitudes.
func (g *geo) degrees(pt ir.Point) (dlon, lat float64, ok bool) {
	if g.k == 0 {
		return 0, 0, false
	}
	return g.unproject(float64((pt.X-g.ox)/g.k), float64((g.oy-pt.Y)/g.k))
}

// walk appends the image of the segment between two pairs of degrees, both
// measured from the centre meridian, moving to the first point rather than
// joining onto the path when start is set.
//
// The walk breaks where the image does: a segment that leaves the visible
// hemisphere stops and starts again where it comes back, which is why a
// graticule on a globe is one shape holding several subpaths rather than one
// line drawn through the middle of the earth.
func (g *geo) walk(p *ir.Path, dlon0, lat0, dlon1, lat1 float64, start bool) {
	n := steps(math.Max(math.Abs(dlon1-dlon0), math.Abs(lat1-lat0)))
	down := !start
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		pt := g.pointAt(lerp(dlon0, dlon1, t), lerp(lat0, lat1, t))
		if math.IsNaN(float64(pt.X)) {
			down = false
			continue
		}
		if down {
			p.LineTo(pt.X, pt.Y)
			continue
		}
		p.MoveTo(pt.X, pt.Y)
		down = true
	}
}

// steps is how many segments a span of degrees is walked in.
func steps(span float64) int {
	n := int(math.Ceil(math.Abs(span) / geoStep))
	if n < 1 {
		return 1
	}
	if n > geoMaxSteps {
		return geoMaxSteps
	}
	return n
}

// Area appends the closed image of a box in degrees, whose four sides are the
// meridians and parallels that bound it and are therefore curves.
//
// It uses the true image whatever the edge policy is, for the reason
// [smith.Area] does: a shape's outline is a claim about the region it
// encloses, and a chord across a curved side encloses the wrong ground. That
// is what makes a cell of a gridded field honest under an equal-area
// projection, which is the chart such a field is drawn as.
func (g *geo) Area(p *ir.Path, x0, y0, x1, y1 float32) {
	// The box measured from the centre meridian, keeping its own width: a
	// cell that straddles the map's cut is drawn past the edge of the map and
	// clipped there, rather than being turned inside out by wrapping each of
	// its sides on its own.
	a := wrap180(float64(x0) - g.lon0)
	b := a + float64(x1-x0)
	c, d := float64(y0), float64(y1)
	g.walk(p, a, c, b, c, true)
	g.walk(p, b, c, b, d, false)
	g.walk(p, b, d, a, d, false)
	g.walk(p, a, d, a, c, false)
	p.Close()
}

// Clip appends the outline of the map: the image of the domain's own box, or
// the rim where the domain runs over the horizon.
//
// A globe is the one map here whose edge is not the image of the domain at
// all: the visible hemisphere is bounded by the rim, which is where the sphere
// turns away rather than where the table stops. A globe showing a region that
// is wholly in view is clipped to that region like any other map, because
// there the table is the thing that stops.
func (g *geo) Clip(p *ir.Path, area ir.Rect) {
	if !g.framed || g.k == 0 {
		p.Rect(area)
		return
	}
	if !g.boxed {
		p.Circle(ir.Point{X: g.ox, Y: g.oy}, g.k)
		return
	}
	g.box(p)
}

// box appends the closed image of the domain, which is the outline of every
// map here but the globe: the ellipse of a Mollweide world, the rectangle of a
// Mercator, the four sides of a region.
func (g *geo) box(p *ir.Path) {
	west, east := float64(g.relW), float64(g.relE)
	south, north := float64(g.y0), float64(g.y1)
	g.walk(p, west, south, east, south, true)
	g.walk(p, east, south, east, north, false)
	g.walk(p, east, north, west, north, false)
	g.walk(p, west, north, west, south, false)
	p.Close()
}

// Decimates reports false: see [Coord.Decimates]. A bucket of equal width on
// screen is a bucket of equal longitude only under the cylindrical
// projections, and a reduction over pixel columns would be measuring something
// else under the other two — while a map of a million rows is a raster or a
// binned count rather than a decimated scatter, so saying no costs nothing
// this coord is for.
func (g *geo) Decimates() bool { return false }

func (g *geo) Describe() Desc {
	return Desc{
		Type:       TypeGeo,
		Projection: g.proj,
		CenterLon:  g.lon0,
		CenterLat:  g.lat0,
		Arc:        g.arc,
	}
}

// forward is the projection itself: degrees in, the projection's own units
// out, and ok == false where the map has no image for the pair.
//
// Every one of them is written for the unit sphere, so the numbers that come
// out are comparable between projections and the fit is the only thing that
// decides how large a map is drawn.
func (g *geo) forward(lon, lat float64) (x, y float64, ok bool) {
	return g.project(wrap180(lon-g.lon0), lat)
}

// project is the projection measured from the centre meridian: a longitude
// already taken about the centre, a latitude, and the projection's own units
// out. It is what the coord's own curves are walked in, because a walk in the
// caller's longitudes would jump the width of the picture wherever theirs
// crosses the map's cut.
func (g *geo) project(dlonDeg, lat float64) (x, y float64, ok bool) {
	if math.IsNaN(dlonDeg) || math.IsNaN(lat) || lat < -90 || lat > 90 {
		return 0, 0, false
	}
	dlon := rad(dlonDeg)
	phi := rad(lat)
	switch g.proj {
	case Mercator:
		if lat > mercatorMaxLat || lat < -mercatorMaxLat {
			return 0, 0, false
		}
		return dlon, math.Log(math.Tan(math.Pi/4 + phi/2)), true

	case Mollweide:
		theta := mollweideTheta(phi)
		return 2 * math.Sqrt2 / math.Pi * dlon * math.Cos(theta), math.Sqrt2 * math.Sin(theta), true

	case Orthographic:
		clat := rad(g.lat0)
		cosc := math.Sin(clat)*math.Sin(phi) + math.Cos(clat)*math.Cos(phi)*math.Cos(dlon)
		if cosc < 0 {
			// The far side of the globe, which is not drawn at all. The rim
			// itself — cosc exactly zero — is drawn, so that a graticule line
			// reaches the edge of the disc rather than stopping a pixel short.
			return 0, 0, false
		}
		return math.Cos(phi) * math.Sin(dlon),
			math.Cos(clat)*math.Sin(phi) - math.Sin(clat)*math.Cos(phi)*math.Cos(dlon), true

	case PlateCarree:
		return dlon, phi, true

	default:
		// A projection nobody defined: no map, rather than a map drawn in
		// some other projection. See [Projection.Known].
		return 0, 0, false
	}
}

// unproject undoes [geo.project]: the projection's own units in, a longitude
// measured from the centre meridian and a latitude out, or ok == false where
// the point is off the map.
func (g *geo) unproject(x, y float64) (dlon, lat float64, ok bool) {
	if math.IsNaN(x) || math.IsNaN(y) {
		return 0, 0, false
	}
	switch g.proj {
	case Mercator:
		return deg(x), deg(2*math.Atan(math.Exp(y)) - math.Pi/2), true

	case Mollweide:
		theta := math.Asin(clamp(y/math.Sqrt2, -1, 1))
		phi := math.Asin(clamp((2*theta+math.Sin(2*theta))/math.Pi, -1, 1))
		cos := math.Cos(theta)
		if cos == 0 {
			// A pole, where every meridian meets and the longitude a point
			// carries is not a reading of anything.
			return 0, deg(phi), true
		}
		lam := math.Pi * x / (2 * math.Sqrt2 * cos)
		if lam < -math.Pi || lam > math.Pi {
			// Outside the ellipse: the corners of the panel a world map
			// leaves empty.
			return 0, 0, false
		}
		return deg(lam), deg(phi), true

	case Orthographic:
		rho := math.Hypot(x, y)
		if rho > 1 {
			return 0, 0, false
		}
		clat := rad(g.lat0)
		if rho == 0 {
			return 0, g.lat0, true
		}
		c := math.Asin(rho)
		sinc, cosc := math.Sin(c), math.Cos(c)
		phi := math.Asin(clamp(cosc*math.Sin(clat)+y*sinc*math.Cos(clat)/rho, -1, 1))
		lam := math.Atan2(x*sinc, rho*cosc*math.Cos(clat)-y*sinc*math.Sin(clat))
		return deg(lam), deg(phi), true

	case PlateCarree:
		if y < -math.Pi/2 || y > math.Pi/2 {
			return 0, 0, false
		}
		return deg(x), deg(y), true

	default:
		return 0, 0, false
	}
}

// mollweideTheta solves 2θ + sin 2θ = π sin φ, which is the whole of what makes
// the projection equal-area and the only equation here without a closed form.
//
// Newton from θ = φ converges in a handful of steps everywhere but at the
// poles, where the derivative vanishes and the answer is known — so the poles
// are answered rather than iterated towards, and the loop is bounded rather
// than run to a tolerance, because a projection that took a different number
// of steps on two machines would draw two maps.
func mollweideTheta(phi float64) float64 {
	const iterations = 8
	if math.Abs(phi) >= math.Pi/2-1e-12 {
		return math.Copysign(math.Pi/2, phi)
	}
	target := math.Pi * math.Sin(phi)
	theta := phi
	for i := 0; i < iterations; i++ {
		d := 2 + 2*math.Cos(2*theta)
		if d == 0 {
			break
		}
		theta -= (2*theta + math.Sin(2*theta) - target) / d
	}
	return theta
}

func (g *geo) Furniture(dst *Furniture, req FurnitureRequest) {
	m := req.Metrics
	// Under the two cylindrical projections a parallel is a horizontal line,
	// so the labels along one really do share a row and render's overlap
	// filter should run over them. Under the other two they do not: two
	// meridian labels round the curve of a globe can share an x and be a
	// finger apart.
	dst.XLabelsShareARow = g.proj == PlateCarree || g.proj == Mercator

	// Where the two ladders are written. A map has no panel edge to hang its
	// numbers off — the edge of the picture is the map's own outline — so each
	// family is labelled along the line of the other family that carries it
	// best: the widest parallel and the tallest meridian, which is the south
	// edge and the west edge of a rectangular map and the equator and the
	// central meridian of a globe.
	labLat, edgeLat := g.labelLine(req.YTicks, true)
	labLon, edgeLon := g.labelLine(req.XTicks, false)
	// Numbers written inside the picture have to be stroked after the marks,
	// for the reason a polar coord's radial axis is.
	dst.AxesOverData = !edgeLat || !edgeLon

	// The edge of the map, which is the one piece of a map's furniture with no
	// tick behind it: a globe's rim is where the sphere turns away rather than
	// a meridian, and the boundary of a world map is where the table stops.
	// It is a family for exactly the reason [Family] exists — and not the
	// family ADR 0070 expected a projection to want, because the graticule
	// turned out to be the two tick lists a panel already has.
	g.outline(dst)

	west, east := float64(g.relW), float64(g.relE)
	south, north := float64(g.y0), float64(g.y1)

	x := dst.x()
	g.walk(&x.axis.Path, west, labLat, east, labLat, true)
	for _, t := range req.XTicks {
		grid, tick := x.next()
		// The meridian this tick stands for, measured from the centre. A tick
		// the domain reaches only by wrapping is one the map draws at the
		// other edge, which is where it is.
		dlon := wrap180(float64(t.Pos) - g.lon0)
		at := g.pointAt(dlon, labLat)
		if !g.onMap(at) {
			// A meridian the projection has no image for at the latitude the
			// numbers are written along: the far side of a globe, where a
			// label would be a number written over the ocean in front of it.
			x.mark(false, Label{})
			continue
		}
		if !t.Minor {
			g.walk(&grid.Path, dlon, south, dlon, north, true)
		}
		if l := m.tickLen(t); l > 0 {
			tick.line(at, ir.Point{X: at.X, Y: at.Y + l})
		}
		x.mark(true, Label{
			At: ir.Point{X: at.X, Y: at.Y + m.labelGap()},
			H:  ir.AlignCenter,
			V:  ir.AlignTop,
		})
	}

	y := dst.y()
	g.walk(&y.axis.Path, labLon, south, labLon, north, true)
	for _, t := range req.YTicks {
		grid, tick := y.next()
		lat := float64(t.Pos)
		at := g.pointAt(labLon, lat)
		if !g.onMap(at) {
			y.mark(false, Label{})
			continue
		}
		if !t.Minor {
			g.walk(&grid.Path, west, lat, east, lat, true)
		}
		if l := m.tickLen(t); l > 0 {
			tick.line(at, ir.Point{X: at.X - l, Y: at.Y})
		}
		y.mark(true, Label{
			At: ir.Point{X: at.X - m.labelGap(), Y: at.Y},
			H:  ir.AlignEnd,
			V:  ir.AlignMiddle,
		})
	}
}

// outline raises the map's own edge as an unlabelled family: the rim of a
// globe, or the boundary of the domain under a projection that has one.
//
// A world map usually draws its own edge twice over — the ±180° meridians and
// the polar parallels are grid lines too — and the second coat is invisible.
// A globe's rim is drawn nowhere else, and a map without it is a map with no
// edge at all.
func (g *geo) outline(dst *Furniture) {
	if g.k == 0 {
		return
	}
	fam := dst.family("outline")
	shape := fam.next()
	if !g.boxed {
		shape.Path.Circle(ir.Point{X: g.ox, Y: g.oy}, g.k)
		return
	}
	g.box(&shape.Path)
}

// labelLine chooses the line one family of numbers is written along, and
// reports whether that line is an edge of the domain.
//
// The candidates are the other family's own levels and the two edges, and the
// one chosen is the longest of them on the page: the widest parallel for the
// meridians' numbers, the tallest meridian for the parallels'. A rectangular
// map's candidates are all the same length and the edge wins the tie, which
// puts the numbers outside the picture where an atlas prints them; a
// projection whose meridians converge has a longest parallel in the middle,
// which is where the numbers can be read and the only place they do not pile
// up on each other.
func (g *geo) labelLine(ticks []scale.Tick, parallel bool) (at float64, edge bool) {
	lo, hi := float64(g.relW), float64(g.relE)
	if parallel {
		lo, hi = float64(g.y0), float64(g.y1)
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	best, bestLen := lo, float32(-1)
	for _, c := range candidates(g.levels(ticks, parallel), lo, hi) {
		if l := g.spanOf(c, parallel); l > bestLen+cullEps {
			best, bestLen = c, l
		}
	}
	return best, best <= lo+cullEps || best >= hi-cullEps
}

// levels is where a family's own lines fall in the frame the coord walks in:
// a latitude as it stands, a longitude measured from the centre meridian.
func (g *geo) levels(ticks []scale.Tick, parallel bool) []float64 {
	out := make([]float64, 0, len(ticks))
	for _, t := range ticks {
		v := float64(t.Pos)
		if !parallel {
			v = wrap180(v - g.lon0)
		}
		out = append(out, v)
	}
	return out
}

// candidates is the levels a family of numbers may be written along: the two
// edges first, so that a tie between an edge and an inner level goes to the
// edge, and then the other family's own levels.
func candidates(levels []float64, lo, hi float64) []float64 {
	out := make([]float64, 0, len(levels)+2)
	out = append(out, lo, hi)
	for _, v := range levels {
		if v > lo && v < hi {
			out = append(out, v)
		}
	}
	return out
}

// spanOf is how long one graticule line is on the page, measured as the box
// its drawn part occupies rather than as its arc length: the comparison it
// feeds only has to tell a line that crosses the map from one that is a point
// or is mostly over the horizon.
func (g *geo) spanOf(at float64, parallel bool) float32 {
	const n = 32
	lo, hi := float64(g.relW), float64(g.relE)
	if !parallel {
		lo, hi = float64(g.y0), float64(g.y1)
	}
	box := ir.Rect{
		Min: ir.Point{X: float32(math.Inf(1)), Y: float32(math.Inf(1))},
		Max: ir.Point{X: float32(math.Inf(-1)), Y: float32(math.Inf(-1))},
	}
	for i := 0; i <= n; i++ {
		v := lerp(lo, hi, float64(i)/n)
		dlon, lat := v, at
		if !parallel {
			dlon, lat = at, v
		}
		pt := g.pointAt(dlon, lat)
		if !g.onMap(pt) {
			continue
		}
		box.Min.X, box.Min.Y = min(box.Min.X, pt.X), min(box.Min.Y, pt.Y)
		box.Max.X, box.Max.Y = max(box.Max.X, pt.X), max(box.Max.Y, pt.Y)
	}
	if box.Max.X < box.Min.X {
		return -1
	}
	return max(box.Dx(), box.Dy())
}

// onMap reports whether a device point is one the projection drew — finite,
// and inside the panel it was fitted into.
func (g *geo) onMap(pt ir.Point) bool {
	if math.IsNaN(float64(pt.X)) || math.IsNaN(float64(pt.Y)) {
		return false
	}
	return inRange(pt.X, g.area.Min.X, g.area.Max.X) && inRange(pt.Y, g.area.Min.Y, g.area.Max.Y)
}

// stretch widens a box to take in one point of the projection's own space.
func stretch(lo, hi ir.Point, x, y float64) (ir.Point, ir.Point) {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return lo, hi
	}
	fx, fy := float32(x), float32(y)
	return ir.Point{X: min(lo.X, fx), Y: min(lo.Y, fy)}, ir.Point{X: max(hi.X, fx), Y: max(hi.Y, fy)}
}

// wrap180 brings a longitude into [−180, 180], the turn a map's centre
// meridian cuts the sphere into.
//
// A longitude already inside it is left exactly as it was, which matters at
// the two ends: a world map runs from −180° to 180° and those are its left and
// right edges, so a wrap that folded one onto the other would draw the western
// edge of the map on top of its eastern one and leave the fit a turn short.
func wrap180(lon float64) float64 {
	if math.IsNaN(lon) || math.IsInf(lon, 0) || (lon >= -180 && lon <= 180) {
		return lon
	}
	lon = math.Mod(lon, 360)
	if lon > 180 {
		return lon - 360
	}
	if lon < -180 {
		return lon + 360
	}
	return lon
}

func rad(deg float64) float64 { return deg * math.Pi / 180 }
func deg(rad float64) float64 { return rad * 180 / math.Pi }

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func clamp(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }

func clamp32(v, lo, hi float32) float32 { return min(max(v, lo), hi) }

package coord_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
)

// The panel every map below is drawn in: 360 by 360, so a world map that keeps
// its 2:1 shape is 360 wide and 180 tall with ninety points of air above and
// below it. Round numbers are what let a test say "the left edge" and mean a
// number rather than a formula.
var mapArea = ir.Rect{Min: ir.Point{X: 0, Y: 0}, Max: ir.Point{X: 360, Y: 360}}

const (
	mapCX, mapCY = float32(180), float32(180)
)

// world frames a map of the whole world, with a graticule every thirty
// degrees.
func world(p coord.Projection, opts ...coord.GeoOption) (coord.Coord, scale.Scale, scale.Scale) {
	x := scale.Linear(scale.Domain(-180, 180), scale.TickValues(-180, -120, -60, 0, 60, 120, 180))
	y := scale.Linear(scale.Domain(-90, 90), scale.TickValues(-90, -60, -30, 0, 30, 60, 90))
	c := coord.Geo(p, opts...).Frame(coord.Framing{Area: mapArea, X: x, Y: y})
	return c, x, y
}

// The projections all agree about one point and about the reader's compass:
// the centre of the map is the centre of the panel, north is up and east is
// right. A chart that got this wrong would be wrong in a way no amount of
// correct projection arithmetic could rescue.
func TestAMapIsCentredAndNorthIsUp(t *testing.T) {
	for _, p := range []coord.Projection{
		coord.PlateCarree, coord.Mercator, coord.Mollweide, coord.Orthographic,
	} {
		c, _, _ := world(p)
		mid := c.Point(0, 0)
		near(t, mid.X, mapCX, string(p)+": the centre of the map, x")
		near(t, mid.Y, mapCY, string(p)+": the centre of the map, y")

		north, east := c.Point(0, 10), c.Point(10, 0)
		if north.Y >= mid.Y {
			t.Errorf("%s: 10°N is at y = %v, which is not above the equator at %v", p, north.Y, mid.Y)
		}
		if east.X <= mid.X {
			t.Errorf("%s: 10°E is at x = %v, which is not right of the prime meridian at %v", p, east.X, mid.X)
		}
	}
}

// A map keeps its own shape rather than filling the panel it is given: the
// whole content of choosing a projection is the shape it makes, and one
// stretched to fit would be a different projection from the one it says it is.
//
// The numbers are the projections' own aspect ratios: 2:1 for the
// equirectangular and Mollweide worlds, square for a Mercator cut at ±85.05°
// and for a globe.
func TestAMapKeepsItsShape(t *testing.T) {
	for _, tc := range []struct {
		proj          coord.Projection
		width, height float32
	}{
		{coord.PlateCarree, 360, 180},
		{coord.Mollweide, 360, 180},
		{coord.Mercator, 360, 360},
		{coord.Orthographic, 360, 360},
	} {
		c, _, _ := world(tc.proj)
		var lo, hi ir.Point
		lo = ir.Point{X: float32(math.Inf(1)), Y: float32(math.Inf(1))}
		hi = ir.Point{X: float32(math.Inf(-1)), Y: float32(math.Inf(-1))}
		// The extent rather than the domain: a Mercator's world ends at
		// ±85.05°, which is the coord's own cut and what it reports.
		x0, x1, y0, y1 := c.Extent()
		for lon := float64(x0); lon <= float64(x1); lon += 2 {
			for lat := float64(y0); lat <= float64(y1); lat += 2 {
				pt := c.Point(float32(lon), float32(lat))
				if math.IsNaN(float64(pt.X)) {
					continue
				}
				lo.X, lo.Y = min(lo.X, pt.X), min(lo.Y, pt.Y)
				hi.X, hi.Y = max(hi.X, pt.X), max(hi.Y, pt.Y)
			}
		}
		// A sample every two degrees misses the very edge of a curved map by
		// a fraction of a degree's worth of pixels, which is what the loose
		// tolerance here is for: the claim is the shape, not the last pixel.
		if math.Abs(float64(hi.X-lo.X-tc.width)) > 2 {
			t.Errorf("%s: the world is %v wide, want %v", tc.proj, hi.X-lo.X, tc.width)
		}
		if math.Abs(float64(hi.Y-lo.Y-tc.height)) > 2 {
			t.Errorf("%s: the world is %v tall, want %v", tc.proj, hi.Y-lo.Y, tc.height)
		}
	}
}

// Every projection here is invertible where it draws anything, and that is
// what a tooltip over a map reads. The test is a round trip rather than a
// table because the inverse is the arithmetic most easily written down with a
// sign wrong, and a sign wrong in one hemisphere is invisible in a table of
// northern latitudes.
func TestEveryProjectionReadsBackTheDegreesItDrew(t *testing.T) {
	for _, p := range []coord.Projection{
		coord.PlateCarree, coord.Mercator, coord.Mollweide, coord.Orthographic,
	} {
		c, _, _ := world(p, coord.GeoCenter(20, 35))
		for lon := -170.0; lon <= 170; lon += 17 {
			for lat := -80.0; lat <= 80; lat += 11 {
				pt := c.Point(float32(lon), float32(lat))
				if math.IsNaN(float64(pt.X)) {
					continue // The far side of a globe, which is not drawn.
				}
				gotLon, gotLat := c.Invert(pt)
				near(t, gotLon, float32(lon), string(p)+": the longitude read back")
				near(t, gotLat, float32(lat), string(p)+": the latitude read back")
			}
		}
	}
}

// A device point that is not on the map has no degrees behind it. A tooltip
// asks about every pixel the pointer crosses, including the four corners a
// world map leaves empty, and a coord that answered with a plausible pair
// there would put a reading on the page for a place the reader is not pointing
// at.
func TestAPointOffTheMapReadsBackAsNothing(t *testing.T) {
	for _, p := range []coord.Projection{coord.Mollweide, coord.Orthographic} {
		c, _, _ := world(p)
		corner := ir.Point{X: mapArea.Min.X + 2, Y: mapArea.Min.Y + 2}
		lon, lat := c.Invert(corner)
		if !math.IsNaN(float64(lon)) || !math.IsNaN(float64(lat)) {
			t.Errorf("%s: the corner of the panel reads as %v, %v, want nothing", p, lon, lat)
		}
	}
}

// The far side of a globe is not drawn, and the near side is. That is the
// whole of what makes the orthographic projection a planet rather than a
// rectangle, and NaN is how a coord says "no image" to every backend here.
func TestTheFarSideOfAGlobeIsNotDrawn(t *testing.T) {
	c, _, _ := world(coord.Orthographic, coord.GeoCenter(0, 0))
	if pt := c.Point(170, 0); !math.IsNaN(float64(pt.X)) {
		t.Errorf("170°E is drawn at %v on a globe centred on the prime meridian", pt)
	}
	if pt := c.Point(80, 0); math.IsNaN(float64(pt.X)) {
		t.Error("80°E is not drawn on a globe centred on the prime meridian")
	}
	// The rim itself is drawn, so that a graticule reaches the edge of the
	// disc rather than stopping a pixel short of it.
	if pt := c.Point(90, 0); math.IsNaN(float64(pt.X)) {
		t.Error("the rim at 90°E is not drawn")
	}
}

// Mollweide is the equal-area projection, and this is the property it is here
// for: a cell ten degrees on a side at the equator and one at sixty degrees
// north cover the same fraction of the sphere once the latitude's cosine is
// taken into account, and they cover the same amount of the page. Under
// Mercator they do not, which is the comparison that makes the claim mean
// something.
func TestMollweideDrawsEqualAreasEqually(t *testing.T) {
	ratio := func(p coord.Projection) float64 {
		c, _, _ := world(p)
		at := func(lat float32) float64 {
			var path ir.Path
			c.Area(&path, -5, lat, 5, lat+10)
			return math.Abs(polygonArea(&path))
		}
		// The sphere's own ratio between the two cells: the integral of cos φ
		// over each band, which is what an equal-area map has to reproduce.
		sphere := (math.Sin(rad(70)) - math.Sin(rad(60))) / math.Sin(rad(10))
		return at(60) / at(0) / sphere
	}
	if got := ratio(coord.Mollweide); math.Abs(got-1) > 0.01 {
		t.Errorf("a Mollweide cell at 60°N draws %v times the ink its area deserves, want 1", got)
	}
	if got := ratio(coord.Mercator); got < 2 {
		t.Errorf("a Mercator cell at 60°N draws %v times the ink its area deserves, want the well-known exaggeration", got)
	}
}

// The side of a cell is the parallel or the meridian between its corners, not
// the chord across it — under a projection that bends them, a box drawn with
// straight sides encloses the wrong ground, and a gridded field drawn that way
// paints its value over ground that belongs to the next cell.
func TestACellsSidesFollowTheGraticule(t *testing.T) {
	c, _, _ := world(coord.Orthographic)
	var path ir.Path
	c.Area(&path, -40, 0, 40, 40)
	// The midpoint of the southern side is where the coord puts the equator
	// at 0°, and a straight side would fall short of it by the sagitta of the
	// arc.
	want := c.Point(0, 0)
	if !pathHasPointNear(&path, want, 0.5) {
		t.Errorf("the southern side of the cell misses (0°, 0°) at %v", want)
	}
}

// A map's numbers are written along the longest line of the other family,
// which is an edge of a rectangular map and the middle of a map whose
// meridians converge. Where they are written inside the picture the axes are
// stroked after the marks, for the reason a polar coord's radial axis is.
func TestTheNumbersAreWrittenWhereTheyCanBeRead(t *testing.T) {
	for _, tc := range []struct {
		proj      coord.Projection
		labelLat  float32
		overData  bool
		shareARow bool
	}{
		// A rectangular map is numbered along its south edge, outside the
		// picture, exactly as an atlas prints it.
		{coord.PlateCarree, -90, false, true},
		{coord.Mercator, -85.05, false, true},
		// A Mollweide's meridians all meet at the poles, so its numbers ride
		// the equator, which is the one parallel they do not pile up on.
		{coord.Mollweide, 0, true, false},
		{coord.Orthographic, 0, true, false},
	} {
		c, x, y := world(tc.proj)
		var fur coord.Furniture
		c.Furniture(&fur, coord.FurnitureRequest{
			Area:    mapArea,
			Metrics: coord.Metrics{TickLen: 4, LabelPad: 2},
			XTicks:  ticksOf(c, x),
			YTicks:  ticksOf(c, y),
		})
		// The label of the prime meridian sits under the point the coord
		// places the chosen parallel's crossing at.
		want := c.Point(0, tc.labelLat)
		var at ir.Point
		for i, l := range fur.LabelX {
			if fur.InX[i] && l.At.X > mapCX-1 && l.At.X < mapCX+1 {
				at = l.At
			}
		}
		if math.Abs(float64(at.Y-want.Y)) > 8 {
			t.Errorf("%s: the prime meridian is numbered at y = %v, want about %v", tc.proj, at.Y, want.Y)
		}
		if fur.AxesOverData != tc.overData {
			t.Errorf("%s: AxesOverData = %v, want %v", tc.proj, fur.AxesOverData, tc.overData)
		}
		if fur.XLabelsShareARow != tc.shareARow {
			t.Errorf("%s: XLabelsShareARow = %v, want %v", tc.proj, fur.XLabelsShareARow, tc.shareARow)
		}
	}
}

// A graticule line the projection has no image for is not drawn and not
// numbered: half a globe's meridians are behind it, and a number written for
// one of them would be a number over the ocean in front of it.
func TestAGlobeNumbersOnlyTheMeridiansItShows(t *testing.T) {
	c, x, y := world(coord.Orthographic)
	var fur coord.Furniture
	c.Furniture(&fur, coord.FurnitureRequest{
		Area:    mapArea,
		Metrics: coord.Metrics{TickLen: 4, LabelPad: 2},
		XTicks:  ticksOf(c, x),
		YTicks:  ticksOf(c, y),
	})
	var shown int
	for _, in := range fur.InX {
		if in {
			shown++
		}
	}
	if shown == 0 || shown >= len(fur.InX) {
		t.Errorf("a globe shows %d of its %d meridians; want some but not all", shown, len(fur.InX))
	}
}

// Every graticule line is drawn, and a meridian on a globe is drawn in the
// pieces that are visible rather than as one line through the middle of the
// earth.
func TestAGraticuleIsDrawnInThePiecesThatAreVisible(t *testing.T) {
	c, x, y := world(coord.Orthographic, coord.GeoCenter(0, 0))
	var fur coord.Furniture
	c.Furniture(&fur, coord.FurnitureRequest{
		Area:    mapArea,
		Metrics: coord.Metrics{TickLen: 4, LabelPad: 2},
		XTicks:  ticksOf(c, x),
		YTicks:  ticksOf(c, y),
	})
	// The equator crosses the visible hemisphere once: one subpath. The
	// ±180° meridian is the rim's own edge case and is split by the far side,
	// so the count is checked on a parallel, where the claim is sharp.
	var moves int
	for i, tick := range ticksOf(c, y) {
		if tick.Value != 0 {
			continue
		}
		fur.GridY[i].Path.Walk(func(op ir.PathOp, _ []ir.Point) {
			if op == ir.OpMoveTo {
				moves++
			}
		})
	}
	if moves != 1 {
		t.Errorf("the equator of a globe is drawn in %d pieces, want 1", moves)
	}
}

// A geo coord is a value like every other coord here: Frame hands back the
// map positioned in a panel and leaves the receiver alone, which is what makes
// two panels on two goroutines safe.
func TestFramingAMapDoesNotMoveTheCoord(t *testing.T) {
	c := coord.Geo(coord.Mollweide)
	a := c.Frame(coord.Framing{Area: mapArea, X: linear(-180, 180), Y: linear(-90, 90)})
	b := c.Frame(coord.Framing{
		Area: ir.Rect{Min: ir.Point{X: 0, Y: 0}, Max: ir.Point{X: 180, Y: 180}},
		X:    linear(-180, 180), Y: linear(-90, 90),
	})
	if pa, pb := a.Point(0, 0), b.Point(0, 0); math.Abs(float64(pa.X-pb.X)) < eps {
		t.Errorf("two panels put the centre of the world in the same place, %v", pa)
	}
	if pt := c.Point(0, 0); !math.IsNaN(float64(pt.X)) && pt.X != 0 {
		t.Errorf("the unframed coord was moved: it places the centre at %v", pt)
	}
}

// A map is steerable — a pan or a zoom narrows the domain and the projection
// refits to it — which is the one thing a Smith chart, the other coord that
// makes a scale's range its own domain, cannot do.
func TestAMapZoomsWhereASmithChartCannot(t *testing.T) {
	if !coord.Steerable(coord.Geo(coord.Mercator)) {
		t.Error("a map reports itself fixed, so an interactive chart will not zoom it")
	}
	c := coord.Geo(coord.Mercator).Frame(coord.Framing{
		Area: mapArea, X: linear(-10, 10), Y: linear(40, 60),
	})
	// Ten degrees of longitude now fill the panel rather than a twentieth of
	// it, so the two ends of the domain are the two ends of the map.
	west, east := c.Point(-10, 50), c.Point(10, 50)
	if east.X-west.X < 200 {
		t.Errorf("ten degrees each way are %v apart on a zoomed map, want most of a 360-wide panel", east.X-west.X)
	}
	// Twenty degrees of longitude are a shorter distance than twenty degrees
	// of latitude at fifty north, so it is the height that fills the panel and
	// the width that has room to spare. That is the fit doing its job.
	north, south := c.Point(0, 60), c.Point(0, 40)
	near(t, south.Y-north.Y, 360, "the height of a zoomed map")
}

// A domain with nothing in it draws nothing rather than dividing by zero: a
// chart built before its data arrived is a chart with one row in it, and a
// panic there would be a panic in a live chart's first frame.
func TestAnEmptyDomainDrawsNothing(t *testing.T) {
	c := coord.Geo(coord.Mollweide).Frame(coord.Framing{
		Area: mapArea, X: linear(5, 5), Y: linear(10, 10),
	})
	pt := c.Point(5, 10)
	if !mapArea.Contains(pt) {
		t.Errorf("the one place in the domain is drawn at %v, outside the panel", pt)
	}
	var path ir.Path
	c.Clip(&path, mapArea)
	if path.Empty() {
		t.Error("an empty map clips to nothing at all, which would hide the panel")
	}
}

// The clip is the outline of the map: the disc of a globe, and the image of
// the domain's own box for the rest. What it is for is the corners a world map
// leaves empty — a mark there is a mark outside the world.
func TestTheClipIsTheOutlineOfTheMap(t *testing.T) {
	c, _, _ := world(coord.Orthographic)
	var path ir.Path
	c.Clip(&path, mapArea)
	b := path.Bounds()
	near(t, b.Dx(), 360, "the clipped globe's width")
	near(t, b.Dy(), 360, "the clipped globe's height")

	var flat ir.Path
	cc, _, _ := world(coord.PlateCarree)
	cc.Clip(&flat, mapArea)
	fb := flat.Bounds()
	near(t, fb.Dx(), 360, "the clipped world's width")
	near(t, fb.Dy(), 180, "the clipped world's height")
}

// The centre turns the globe rather than sliding the picture: a map centred on
// 90°E shows the far side of the one centred on the prime meridian.
func TestACentredMapTurnsTheGlobe(t *testing.T) {
	c, _, _ := world(coord.Orthographic, coord.GeoCenter(90, 0))
	mid := c.Point(90, 0)
	near(t, mid.X, mapCX, "the centre of a turned globe, x")
	near(t, mid.Y, mapCY, "the centre of a turned globe, y")
	if pt := c.Point(0, 0); math.IsNaN(float64(pt.X)) {
		t.Error("the prime meridian is not drawn on a globe centred on 90°E, and it is on its rim")
	}
	if pt := c.Point(-90, 0); !math.IsNaN(float64(pt.X)) {
		t.Errorf("90°W is drawn at %v on a globe centred on 90°E, and it is behind it", pt)
	}
}

// A geo coord writes itself down and reads itself back, which is what keeps a
// map in a JSON document a map.
// The default is the projection that does not mislead about size. Every other
// one here draws a region larger or smaller than its share of the world, and
// on a map that reads as a quantity — so it is asked for by name.
func TestTheDefaultProjectionIsTheEqualAreaOne(t *testing.T) {
	if coord.DefaultProjection != coord.Mollweide {
		t.Errorf("the default projection is %q, want the equal-area one", coord.DefaultProjection)
	}
	d, _ := coord.Describe(coord.Geo(""))
	want, _ := coord.Describe(coord.Geo(coord.Mollweide))
	if !reflect.DeepEqual(d, want) {
		t.Errorf("a map drawn without naming a projection is\n %+v\nwant\n %+v", d, want)
	}
	// And it really is equal-area: a ten-degree cell at sixty north covers
	// the share of the page its share of the sphere, which is the property
	// the default is chosen for.
	c := coord.Geo("").Frame(coord.Framing{Area: mapArea, X: linear(-180, 180), Y: linear(-90, 90)})
	area := func(lat float32) float64 {
		var path ir.Path
		c.Area(&path, -5, lat, 5, lat+10)
		return math.Abs(polygonArea(&path))
	}
	sphere := (math.Sin(rad(70)) - math.Sin(rad(60))) / math.Sin(rad(10))
	if got := area(60) / area(0) / sphere; math.Abs(got-1) > 0.01 {
		t.Errorf("the default projection draws a cell at 60°N at %v times its share, want 1", got)
	}
}

func TestAMapDescribesItself(t *testing.T) {
	c := coord.Geo(coord.Mollweide, coord.GeoCenter(11, -5), coord.GeoArc())
	d, ok := coord.Describe(c)
	if !ok {
		t.Fatal("a geo coord does not describe itself")
	}
	want := coord.Desc{
		Type: coord.TypeGeo, Projection: coord.Mollweide,
		CenterLon: 11, CenterLat: -5, Arc: true,
	}
	if !reflect.DeepEqual(d, want) {
		t.Errorf("described as\n %+v\nwant\n %+v", d, want)
	}
	back, err := coord.FromDesc(d)
	if err != nil {
		t.Fatalf("rebuilding the coord: %v", err)
	}
	again, _ := coord.Describe(back)
	if !reflect.DeepEqual(again, d) {
		t.Errorf("the round trip is\n %+v\nwant\n %+v", again, d)
	}
}

// An edge between two rows is the straight line between them by default,
// because two samples of a track are what was measured; GeoArc is the other
// reading, and under the one affine projection the two are the same line.
func TestAnEdgeIsAChordUnlessAskedOtherwise(t *testing.T) {
	if c, _, _ := world(coord.Mollweide); !c.Straight() {
		t.Error("a map bends an edge between two rows by default")
	}
	if c, _, _ := world(coord.Mollweide, coord.GeoArc()); c.Straight() {
		t.Error("GeoArc did not ask for the curved edge")
	}
	if c, _, _ := world(coord.PlateCarree, coord.GeoArc()); !c.Straight() {
		t.Error("an affine projection bends a straight edge, which it cannot")
	}
}

// ticksOf asks a scale for its ticks with the range the coord gave it, which
// is what render hands a coord's Furniture.
func ticksOf(c coord.Coord, s scale.Scale) []scale.Tick {
	_ = c
	return s.Ticks(scale.TickRequest{Want: 12})
}

// polygonArea is the signed area of a path's first subpath, by the shoelace
// formula. Every path this file measures is a closed polyline.
func polygonArea(p *ir.Path) float64 {
	var pts []ir.Point
	p.Walk(func(op ir.PathOp, q []ir.Point) {
		switch op {
		case ir.OpMoveTo, ir.OpLineTo:
			pts = append(pts, q[len(q)-1])
		}
	})
	var a float64
	for i := range pts {
		j := (i + 1) % len(pts)
		a += float64(pts[i].X)*float64(pts[j].Y) - float64(pts[j].X)*float64(pts[i].Y)
	}
	return a / 2
}

// pathHasPointNear reports whether any of a path's points is within d of pt.
func pathHasPointNear(p *ir.Path, pt ir.Point, d float32) bool {
	var found bool
	p.Walk(func(_ ir.PathOp, q []ir.Point) {
		for _, v := range q {
			if math.Hypot(float64(v.X-pt.X), float64(v.Y-pt.Y)) <= float64(d) {
				found = true
			}
		}
	})
	return found
}

func rad(d float64) float64 { return d * math.Pi / 180 }

// A projection this package does not draw draws no map at all, and a document
// naming one is refused. Falling back to the nearest projection would put a
// chart on the page whose every position is wrong by a projection's worth,
// which is the one kind of wrong a reader cannot see.
func TestAnUnknownProjectionDrawsNoMap(t *testing.T) {
	c := coord.Geo("peirce-quincuncial").Frame(coord.Framing{
		Area: mapArea, X: linear(-180, 180), Y: linear(-90, 90),
	})
	if pt := c.Point(0, 0); !math.IsNaN(float64(pt.X)) {
		t.Errorf("a projection nobody defined placed the middle of the world at %v", pt)
	}
	if _, err := coord.FromDesc(coord.Desc{Type: coord.TypeGeo, Projection: "peirce-quincuncial"}); err == nil {
		t.Error("a Desc naming a projection this package does not draw was accepted")
	}
	// The absent projection is the equal-area one, so a document that names a
	// map and nothing else is a map that does not mislead about size.
	back, err := coord.FromDesc(coord.Desc{Type: coord.TypeGeo})
	if err != nil {
		t.Fatalf("a Desc naming no projection: %v", err)
	}
	if d, _ := coord.Describe(back); d.Projection != coord.Mollweide {
		t.Errorf("a Desc naming no projection came back as %q", d.Projection)
	}
}

// The edge of the map is the one piece of a map's furniture with no tick
// behind it, and it is the only reason this coord raises a family at all. A
// globe without it has no edge, because its rim is not a meridian.
func TestTheMapHasAnEdge(t *testing.T) {
	for _, tc := range []struct {
		proj          coord.Projection
		width, height float32
	}{
		{coord.PlateCarree, 360, 180},
		{coord.Orthographic, 360, 360},
	} {
		c, x, y := world(tc.proj)
		var fur coord.Furniture
		c.Furniture(&fur, coord.FurnitureRequest{
			Area: mapArea, Metrics: coord.Metrics{TickLen: 4, LabelPad: 2},
			XTicks: ticksOf(c, x), YTicks: ticksOf(c, y),
		})
		if len(fur.Families) != 1 {
			t.Fatalf("%s: %d families, want the outline and nothing else", tc.proj, len(fur.Families))
		}
		fam := fur.Families[0]
		if len(fam.Labels) != 0 || len(fam.Text) != 0 {
			t.Errorf("%s: the outline is labelled, and there is nothing to number it by", tc.proj)
		}
		b := fam.Lines[0].Path.Bounds()
		if math.Abs(float64(b.Dx()-tc.width)) > 2 || math.Abs(float64(b.Dy()-tc.height)) > 2 {
			t.Errorf("%s: the outline is %v by %v, want %v by %v", tc.proj, b.Dx(), b.Dy(), tc.width, tc.height)
		}
	}
}

// The batch form places every point exactly where the per-point one does. It
// exists so that a per-row interface call does not reappear on the hot path,
// and a batch form that disagreed would be a second projection.
func TestTheBatchFormPlacesTheSamePoints(t *testing.T) {
	c, _, _ := world(coord.Orthographic, coord.GeoCenter(30, 20))
	xs := []float32{-170, -90, -20, 0, 35, 120, 179}
	ys := []float32{-60, -10, 0, 25, 55, 80, 90}
	got := c.Points(nil, xs, ys)
	if len(got) != len(xs) {
		t.Fatalf("Points returned %d points for %d rows", len(got), len(xs))
	}
	for i := range xs {
		want := c.Point(xs[i], ys[i])
		if math.IsNaN(float64(want.X)) {
			if !math.IsNaN(float64(got[i].X)) {
				t.Errorf("row %d is drawn by the batch form and not by Point", i)
			}
			continue
		}
		near(t, got[i].X, want.X, "the batch form's x")
		near(t, got[i].Y, want.Y, "the batch form's y")
	}
}

// A map does not decimate. A bucket of equal width on screen is a bucket of
// equal longitude under the cylindrical projections and of nothing in
// particular under the other two, and a reduction defined over pixel columns
// would be measuring something else.
func TestAMapDoesNotDecimate(t *testing.T) {
	for _, p := range []coord.Projection{
		coord.PlateCarree, coord.Mercator, coord.Mollweide, coord.Orthographic,
	} {
		if c, _, _ := world(p); c.Decimates() {
			t.Errorf("%s reports that a reduction over pixel columns still measures what it was defined to", p)
		}
	}
}

// A map centred away from the prime meridian is drawn in one piece. Everything
// the coord draws for itself is walked in longitudes measured from the centre,
// because a walk in the caller's would jump the width of the picture wherever
// theirs crosses the map's own cut — which is a grid line drawn straight
// across the world.
func TestACentredWorldIsDrawnInOnePiece(t *testing.T) {
	c, x, y := world(coord.Mollweide, coord.GeoCenter(150, 0))
	var fur coord.Furniture
	c.Furniture(&fur, coord.FurnitureRequest{
		Area: mapArea, Metrics: coord.Metrics{TickLen: 4, LabelPad: 2},
		XTicks: ticksOf(c, x), YTicks: ticksOf(c, y),
	})
	// Every parallel crosses the map once, left to right. Two subpaths would
	// be the seam; a subpath that jumps would be the bug the seam causes.
	for i, tick := range ticksOf(c, y) {
		if tick.Value <= -90 || tick.Value >= 90 {
			continue // The poles, where a parallel is a point.
		}
		var moves int
		var prev ir.Point
		var jumped bool
		fur.GridY[i].Path.Walk(func(op ir.PathOp, pts []ir.Point) {
			at := pts[len(pts)-1]
			switch op {
			case ir.OpMoveTo:
				moves++
			case ir.OpLineTo:
				if math.Abs(float64(at.X-prev.X)) > 40 {
					jumped = true
				}
			}
			prev = at
		})
		if moves != 1 || jumped {
			t.Errorf("the parallel at %v° is drawn in %d pieces (jumped: %v)", tick.Value, moves, jumped)
		}
	}
	// And the whole turn is shown: the domain is a full turn, so the map is
	// as wide as a world map of the same size centred anywhere else.
	plain, _, _ := world(coord.Mollweide)
	got := c.Point(150, 0).X - c.Point(60, 0).X
	want := plain.Point(0, 0).X - plain.Point(-90, 0).X
	near(t, got, want, "ninety degrees of a Pacific-centred world")
}

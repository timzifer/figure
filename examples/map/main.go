// Command map renders the three charts a map projection draws, and the reason
// there is more than one of them.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. See
// docs/adr/0081-a-map-projection.md.
//
// The first chart is a measured field over the whole world on an equal-area
// projection, which is the projection a quantity per unit area has to be drawn
// on: under Mercator the cells near the poles are drawn several times the size
// of the cells at the equator, and a reader who compares how much ink a region
// covers would be reading the projection rather than the field.
//
// The second and third are the same two routes from Edinburgh to Tokyo, drawn
// first on a Mercator chart and then on a globe. The rhumb line — the one a
// ship holds a constant bearing along — is the straight line on the Mercator
// chart and is the longer of the two; the great circle is the shorter one and
// bows north of it. On the globe the comparison is the other way round: the
// great circle is visibly the direct route and the rhumb wanders. Neither map
// is lying, and that is the whole point of having more than one.
//
// Both routes are rows in the table rather than anything the library computes.
// A path between two places is a claim about the route — the shortest one over
// the sphere, the one at a constant bearing, the one that avoids the ice — and
// the coord's job is to put the rows where the projection says, not to decide
// which path was meant.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func main() {
	world := flag.String("world", "world.svg", "output path for the equal-area world map")
	mercator := flag.String("mercator", "mercator.svg", "output path for the Mercator chart")
	globe := flag.String("globe", "globe.svg", "output path for the globe")
	flag.Parse()
	if err := run(*world, *mercator, *globe); err != nil {
		fmt.Fprintln(os.Stderr, "map:", err)
		os.Exit(1)
	}
}

func run(world, mercator, globe string) error {
	if err := cloudCover(world); err != nil {
		return err
	}
	if err := routesOnMercator(mercator); err != nil {
		return err
	}
	return routesOnAGlobe(globe)
}

// cloudCover draws a gridded field on the equal-area projection: one rect per
// ten-degree cell, filled with the cell's reading.
//
// The cells are [geom.Rect] and nothing else — a box in degrees, which the
// coord draws as the shape the projection gives it, with its sides along the
// parallels and meridians that bound it rather than as the chords across
// them. A cell drawn with straight sides would cover ground belonging to its
// neighbour, which for a field is the difference between a reading and a
// reading painted over the wrong place.
func cloudCover(out string) error {
	lon0, lon1, lat0, lat1, value := cloudField()
	src := figure.NewTable().
		Float64("west", lon0).Float64("east", lon1).
		Float64("south", lat0).Float64("north", lat1).
		Float64("cover", value)

	p := figure.New(
		figure.Size(880, 500),
		figure.Title("Mean cloud cover in July, per cent"),
		figure.Theme(theme.Light),
		figure.Coord(coord.Geo(coord.Mollweide)),
	)
	graticule(p, -180, 180, 60, -90, 90, 30)
	p.Add(geom.Rect(src,
		geom.X("west"), geom.X2("east"),
		geom.Y("south"), geom.Y2("north"),
		geom.ColorBy("cover", scale.Sequential(palette.Viridis)),
		geom.Label("Cloud cover (%)"),
	))
	return p.Render(figure.SVG(out))
}

// routesOnMercator draws the two routes on the chart a navigator plots them
// on: the rhumb line is straight here, because that is what Mercator is for.
func routesOnMercator(out string) error {
	p := figure.New(
		figure.Size(880, 620),
		figure.Title("Edinburgh to Tokyo: the straight line on a Mercator chart is the long way"),
		figure.Theme(theme.Light),
		figure.Coord(coord.Geo(coord.Mercator)),
	)
	// The chart is cut to the ground the route crosses. A Mercator of the
	// whole world is square and two thirds of it is ocean nobody on this
	// flight sees; a domain is how a map is cropped, because a map's domain
	// is degrees like any other axis's.
	graticule(p, -20, 160, 30, 20, 80, 15)
	routes(p)
	return p.Render(figure.SVG(out))
}

// routesOnAGlobe draws the same two routes on the projection that shows why
// one of them is shorter. The globe is turned to look down on the route rather
// than at the equator, which is the centre a great circle over the north makes
// sense from.
func routesOnAGlobe(out string) error {
	p := figure.New(
		figure.Size(700, 700),
		figure.Title("The same two routes, from above the pole"),
		figure.Theme(theme.Light),
		figure.Coord(coord.Geo(coord.Orthographic, coord.GeoCenter(75, 55))),
	)
	graticule(p, -180, 180, 30, -90, 90, 30)
	routes(p)
	return p.Render(figure.SVG(out))
}

// graticule gives a map its two axes: degrees, at the spacing the chart wants
// its grid drawn at. Nothing about them is special — a longitude is a number
// on a linear scale — and the map is what the coord makes of the pair.
//
// The titles are turned off because a map's numbers say what they are: a
// reader who needs to be told that the numbers round the edge of a world map
// are degrees is not helped by being told it twice.
func graticule(p *figure.Plot, west, east, lonStep, south, north, latStep float64) {
	p.X(scale.Linear(scale.Domain(west, east), scale.TickValues(everyStep(west, east, lonStep)...)))
	p.Y(scale.Linear(scale.Domain(south, north), scale.TickValues(everyStep(south, north, latStep)...)))
}

// everyStep is the ladder of degrees a graticule is drawn at, from lo to hi
// inclusive. A map's ticks are stated rather than searched for: the levels a
// reader expects on a world map are the round ones, and an axis left to choose
// its own would pick a sequence that is correct and unfamiliar.
func everyStep(lo, hi, step float64) []float64 {
	var out []float64
	for v := lo; v <= hi+1e-9; v += step {
		out = append(out, v)
	}
	return out
}

// routes adds the two paths and the two cities to a map.
func routes(p *figure.Plot) {
	const (
		edinLon, edinLat   = -3.19, 55.95
		tokyoLon, tokyoLat = 139.69, 35.69
	)
	great := path(greatCircle(edinLon, edinLat, tokyoLon, tokyoLat, 128))
	rhumb := path(rhumbLine(edinLon, edinLat, tokyoLon, tokyoLat, 128))
	p.Add(geom.Line(great, geom.X("lon"), geom.Y("lat"),
		geom.Color(palette.SkyBlue), geom.Width(2.5), geom.Label("Great circle")))
	p.Add(geom.Line(rhumb, geom.X("lon"), geom.Y("lat"),
		geom.Color(ir.RGB(214, 96, 77)), geom.Width(2.5), geom.Label("Rhumb line")))

	cities := figure.NewTable().
		Float64("lon", []float64{edinLon, tokyoLon}).
		Float64("lat", []float64{edinLat, tokyoLat}).
		String("city", []string{"Edinburgh", "Tokyo"})
	p.Add(geom.Scatter(cities, geom.X("lon"), geom.Y("lat"),
		geom.Size(7), geom.Color(ir.RGB(34, 34, 34)), geom.Label("Cities")))
	p.Add(geom.Text(cities, geom.X("lon"), geom.Y("lat"), geom.TextBy("city"),
		geom.Align(ir.AlignStart, ir.AlignBottom), geom.AvoidOverlap(true)))
}

// path turns two slices of degrees into the table a line layer reads.
func path(lon, lat []float64) figure.Source {
	return figure.NewTable().Float64("lon", lon).Float64("lat", lat)
}

// greatCircle is the shortest path over the sphere, sampled at n+1 points.
//
// It is the spherical interpolation of the two end points read as vectors: the
// arc between them turned through at constant rate, which is the definition of
// the route rather than an approximation to it. The samples are the rows a
// line layer joins, and the only approximation is how many of them there are.
func greatCircle(lon0, lat0, lon1, lat1 float64, n int) (lon, lat []float64) {
	ax, ay, az := vector(lon0, lat0)
	bx, by, bz := vector(lon1, lat1)
	dot := math.Min(math.Max(ax*bx+ay*by+az*bz, -1), 1)
	omega := math.Acos(dot)
	lon, lat = make([]float64, 0, n+1), make([]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		var ka, kb float64
		if s := math.Sin(omega); s < 1e-9 {
			ka, kb = 1-t, t
		} else {
			ka, kb = math.Sin((1-t)*omega)/s, math.Sin(t*omega)/s
		}
		x, y, z := ka*ax+kb*bx, ka*ay+kb*by, ka*az+kb*bz
		lo, la := degrees(x, y, z)
		lon, lat = append(lon, lo), append(lat, la)
	}
	return lon, lat
}

// rhumbLine is the path of constant bearing, sampled at n+1 points.
//
// It is the straight line on a Mercator chart, which is the same statement:
// Mercator stretches the latitudes by exactly the amount that turns a constant
// bearing into a straight line, and that is the property it was constructed
// for. So the path is a linear interpolation in the projection's own
// coordinates, read back as degrees.
func rhumbLine(lon0, lat0, lon1, lat1 float64, n int) (lon, lat []float64) {
	y0, y1 := mercatorY(lat0), mercatorY(lat1)
	lon, lat = make([]float64, 0, n+1), make([]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		y := y0 + (y1-y0)*t
		lon = append(lon, lon0+(lon1-lon0)*t)
		lat = append(lat, 2*math.Atan(math.Exp(y))*180/math.Pi-90)
	}
	return lon, lat
}

func mercatorY(lat float64) float64 {
	return math.Log(math.Tan(math.Pi/4 + lat*math.Pi/360))
}

// vector is a place on the unit sphere, and degrees is the way back.
func vector(lon, lat float64) (x, y, z float64) {
	la, lo := lat*math.Pi/180, lon*math.Pi/180
	return math.Cos(la) * math.Cos(lo), math.Cos(la) * math.Sin(lo), math.Sin(la)
}

func degrees(x, y, z float64) (lon, lat float64) {
	return math.Atan2(y, x) * 180 / math.Pi, math.Atan2(z, math.Hypot(x, y)) * 180 / math.Pi
}

// cloudField is the figure's data: one reading per ten-degree cell of the
// whole world. The numbers are made up and the pattern in them is the ordinary
// one — the wet band along the equator, the clear subtropics either side of
// it, the cloudy storm tracks over the midlatitude oceans, and rather less of
// all of it over the continents than over the sea.
func cloudField() (west, east, south, north, cover []float64) {
	for lat := -90.0; lat < 90; lat += 10 {
		for lon := -180.0; lon < 180; lon += 10 {
			c, l := lat+5, lon+5
			// Wet along the equator, clear through the subtropics, cloudy
			// again over the midlatitude storm tracks: one cosine with a
			// sixty-degree period says all three.
			band := 52 + 26*math.Cos(c*math.Pi/30)
			// And rather less of it over the continents than over the sea,
			// which is what stops the picture being a set of stripes.
			land := 11*math.Cos((l+20)*math.Pi/60) + 7*math.Sin((l-60)*math.Pi/90)
			v := math.Min(math.Max(band-land*math.Cos(c*math.Pi/180), 5), 95)
			west, east = append(west, lon), append(east, lon+10)
			south, north = append(south, lat), append(north, lat+10)
			cover = append(cover, v)
		}
	}
	return west, east, south, north, cover
}

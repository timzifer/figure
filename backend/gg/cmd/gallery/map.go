package main

import (
	"math"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// mapFigure is a measured field over the whole world on the projection a
// quantity per unit area has to be drawn on.
//
// Every cell is one row of a table of degrees and one [geom.Rect]: a box in
// degrees, whose sides the coord draws along the parallels and meridians that
// bound it rather than as the chords across them. Under Mollweide two cells of
// equal area on the sphere cover equal amounts of the page, which is what lets
// a reader compare how much of the world is under cloud by how much of the
// picture is; under Mercator the polar cells would be drawn several times
// their share and the comparison would be with the projection.
//
// The graticule is the two axes' own ticks — a meridian is what a longitude
// tick looks like once the projection has had it — so nothing here draws it.
// See docs/adr/0081-a-map-projection.md.
func mapFigure() plate {
	return plate{
		name: "map", width: 860, high: 500, theme: theme.Light,
		title: "Mean cloud cover in July, on an equal-area projection",
		opts:  []figure.Option{figure.Coord(coord.Geo(coord.Mollweide))},
		build: func(p *figure.Plot) {
			graticule(p, -180, 180, 60, -90, 90, 30)
			p.Add(geom.Rect(cloudTable(),
				geom.X("west"), geom.X2("east"),
				geom.Y("south"), geom.Y2("north"),
				geom.ColorBy("cover", scale.Sequential(palette.Viridis)),
				geom.Label("Cloud cover (%)"),
			))
		},
	}
}

// globeFigure is the chart that says why the shortest way from Scotland to
// Japan goes over the ice: the great circle and the rhumb line between the
// same two cities, drawn on the hemisphere that faces them.
//
// Both routes are rows rather than anything the library computed. A path
// between two places is a claim about the route — the shortest over the
// sphere, the one at a constant bearing — and the coord's job is to put the
// rows where the projection says.
func globeFigure() plate {
	return plate{
		name: "globe", width: 720, high: 640, theme: theme.Light,
		title: "Edinburgh to Tokyo, two routes, seen from over the pole",
		opts: []figure.Option{
			figure.Coord(coord.Geo(coord.Orthographic, coord.GeoCenter(75, 55))),
		},
		build: func(p *figure.Plot) {
			graticule(p, -180, 180, 30, -90, 90, 30)
			const (
				edinLon, edinLat   = -3.19, 55.95
				tokyoLon, tokyoLat = 139.69, 35.69
			)
			p.Add(geom.Line(greatCircle(edinLon, edinLat, tokyoLon, tokyoLat),
				geom.X("lon"), geom.Y("lat"),
				geom.Color(palette.SkyBlue), geom.Width(2.5), geom.Label("Great circle")))
			p.Add(geom.Line(rhumbLine(edinLon, edinLat, tokyoLon, tokyoLat),
				geom.X("lon"), geom.Y("lat"),
				geom.Color(ir.RGB(214, 96, 77)), geom.Width(2.5), geom.Label("Rhumb line")))
			cities := data.NewTable().
				Float64("lon", []float64{edinLon, tokyoLon}).
				Float64("lat", []float64{edinLat, tokyoLat}).
				String("city", []string{"Edinburgh", "Tokyo"})
			p.Add(geom.Scatter(cities, geom.X("lon"), geom.Y("lat"),
				geom.Size(7), geom.Color(ir.RGB(34, 34, 34)), geom.Label("Cities")))
			p.Add(geom.Text(cities, geom.X("lon"), geom.Y("lat"), geom.TextBy("city"),
				geom.Align(ir.AlignStart, ir.AlignBottom)))
		},
	}
}

// graticule gives a map its two axes: degrees, at the spacing the chart draws
// its grid at. The levels are stated rather than searched for, because the
// ones a reader expects on a world map are the round ones.
func graticule(p *figure.Plot, west, east, lonStep, south, north, latStep float64) {
	p.X(scale.Linear(scale.Domain(west, east), scale.TickValues(ladder(west, east, lonStep)...)))
	p.Y(scale.Linear(scale.Domain(south, north), scale.TickValues(ladder(south, north, latStep)...)))
}

func ladder(lo, hi, step float64) []float64 {
	var out []float64
	for v := lo; v <= hi+1e-9; v += step {
		out = append(out, v)
	}
	return out
}

// cloudTable is the field: one reading per ten-degree cell. The numbers are
// made up and the pattern in them is the ordinary one — wet along the equator,
// clear through the subtropics, cloudy again over the midlatitude storm
// tracks, and less of all of it over the continents than over the sea.
func cloudTable() data.Source {
	var west, east, south, north, cover []float64
	for lat := -90.0; lat < 90; lat += 10 {
		for lon := -180.0; lon < 180; lon += 10 {
			c, l := lat+5, lon+5
			band := 52 + 26*math.Cos(c*math.Pi/30)
			land := 11*math.Cos((l+20)*math.Pi/60) + 7*math.Sin((l-60)*math.Pi/90)
			west, east = append(west, lon), append(east, lon+10)
			south, north = append(south, lat), append(north, lat+10)
			cover = append(cover, math.Min(math.Max(band-land*math.Cos(c*math.Pi/180), 5), 95))
		}
	}
	return data.NewTable().
		Float64("west", west).Float64("east", east).
		Float64("south", south).Float64("north", north).
		Float64("cover", cover)
}

// greatCircle is the shortest path over the sphere between two places,
// sampled: the spherical interpolation of the two ends read as vectors.
func greatCircle(lon0, lat0, lon1, lat1 float64) data.Source {
	const n = 128
	ax, ay, az := onSphere(lon0, lat0)
	bx, by, bz := onSphere(lon1, lat1)
	omega := math.Acos(math.Min(math.Max(ax*bx+ay*by+az*bz, -1), 1))
	lon, lat := make([]float64, 0, n+1), make([]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / n
		ka, kb := math.Sin((1-t)*omega), math.Sin(t*omega)
		if s := math.Sin(omega); s < 1e-9 {
			ka, kb = 1-t, t
		} else {
			ka, kb = ka/s, kb/s
		}
		x, y, z := ka*ax+kb*bx, ka*ay+kb*by, ka*az+kb*bz
		lon = append(lon, math.Atan2(y, x)*180/math.Pi)
		lat = append(lat, math.Atan2(z, math.Hypot(x, y))*180/math.Pi)
	}
	return data.NewTable().Float64("lon", lon).Float64("lat", lat)
}

// rhumbLine is the path of constant bearing, which is the straight line on a
// Mercator chart and a curve everywhere else — including here.
func rhumbLine(lon0, lat0, lon1, lat1 float64) data.Source {
	const n = 128
	y0, y1 := mercatorY(lat0), mercatorY(lat1)
	lon, lat := make([]float64, 0, n+1), make([]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / n
		lon = append(lon, lon0+(lon1-lon0)*t)
		lat = append(lat, 2*math.Atan(math.Exp(y0+(y1-y0)*t))*180/math.Pi-90)
	}
	return data.NewTable().Float64("lon", lon).Float64("lat", lat)
}

func mercatorY(lat float64) float64 { return math.Log(math.Tan(math.Pi/4 + lat*math.Pi/360)) }

func onSphere(lon, lat float64) (x, y, z float64) {
	la, lo := lat*math.Pi/180, lon*math.Pi/180
	return math.Cos(la) * math.Cos(lo), math.Cos(la) * math.Sin(lo), math.Sin(la)
}

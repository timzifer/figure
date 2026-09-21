package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleRuns executes the documented example, so that the three charts a
// map projection exists for cannot stop compiling or stop drawing without
// anyone noticing.
func TestExampleRuns(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world.svg")
	mercator := filepath.Join(dir, "mercator.svg")
	globe := filepath.Join(dir, "globe.svg")
	if err := run(world, mercator, globe); err != nil {
		t.Fatalf("run: %v", err)
	}
	for path, wants := range map[string][]string{
		world:    {"<svg", "Mean cloud cover", "Cloud cover (%)", "</svg>"},
		mercator: {"<svg", "Mercator chart", "Great circle", "Rhumb line", "Edinburgh", "Tokyo", "</svg>"},
		globe:    {"<svg", "from above the pole", "Great circle", "Tokyo", "</svg>"},
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		got := string(b)
		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Errorf("%s is missing %q", filepath.Base(path), want)
			}
		}
		// The far side of a globe has no image, and a coord says so with a
		// NaN. One reaching the document would be a coordinate no renderer
		// can draw — which is the failure mode a projection that hides half
		// the world has and no other coord here does.
		if strings.Contains(got, "NaN") {
			t.Errorf("%s: a NaN reached the SVG", filepath.Base(path))
		}
	}
}

// The great circle is the shorter route and the rhumb line is the one at a
// constant bearing. If the two were ever computed the same way the example's
// whole point would quietly disappear, and both curves would still draw.
func TestTheGreatCircleIsShorterThanTheRhumbLine(t *testing.T) {
	const (
		edinLon, edinLat   = -3.19, 55.95
		tokyoLon, tokyoLat = 139.69, 35.69
	)
	great := length(greatCircle(edinLon, edinLat, tokyoLon, tokyoLat, 256))
	rhumb := length(rhumbLine(edinLon, edinLat, tokyoLon, tokyoLat, 256))
	if great >= rhumb {
		t.Errorf("the great circle is %.0f km and the rhumb line %.0f km; the great circle is the shorter route by construction", great, rhumb)
	}
	// The great circle between these two cities runs north of both of them,
	// which is the fact the chart is drawn to show.
	_, lat := greatCircle(edinLon, edinLat, tokyoLon, tokyoLat, 64)
	var north float64
	for _, v := range lat {
		north = math.Max(north, v)
	}
	if north <= edinLat {
		t.Errorf("the great circle reaches %.1f°N, which is no further north than Edinburgh", north)
	}
}

// length is a path's length over the sphere, in kilometres, by the haversine
// between consecutive rows.
func length(lon, lat []float64) float64 {
	const earthKm = 6371.0
	var total float64
	for i := 1; i < len(lon); i++ {
		p1, p2 := rad(lat[i-1]), rad(lat[i])
		dp, dl := p2-p1, rad(lon[i]-lon[i-1])
		h := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
		total += 2 * earthKm * math.Asin(math.Min(1, math.Sqrt(h)))
	}
	return total
}

func rad(d float64) float64 { return d * math.Pi / 180 }

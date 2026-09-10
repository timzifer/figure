package three

import (
	"math"
	"strconv"
	"testing"

	"github.com/timzifer/figure/internal/irtest"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

func unitScales() [3]scale.Scale {
	var out [3]scale.Scale
	for i := range out {
		s := scale.Linear(scale.Domain(0, 10))
		s.SetRange(0, 1)
		out[i] = s
	}
	return out
}

// The three faces the cube shows are the three pointing away from the camera,
// and which three that is follows from the sign of the view direction alone.
// Getting it wrong draws the box in front of its own data.
func TestTheCubeShowsTheThreeFacesFacingAway(t *testing.T) {
	for _, cam := range octants() {
		pr := project(cam, ir.R(0, 0, 200, 200))
		c := newCube(theme.Light, pr, cam, ticksOf(theme.Light, unitScales()), [3]string{})

		for a := 0; a < 3; a++ {
			near := c.faceDepth(a, 1-c.far[a])
			far := c.faceDepth(a, c.far[a])
			if far <= near {
				t.Errorf("camera %+v: axis %d drew the face at %v (depth %v), "+
					"but the one at %v is farther (depth %v)",
					cam, a, c.far[a], far, 1-c.far[a], near)
			}
		}
	}
}

// faceDepth is how far the middle of one face of the cube is from the camera.
func (c cube) faceDepth(a int, side float32) float64 {
	return c.proj.depth(centre.with(a, side))
}

// With the camera above the floor plane, the floor is one of the three faces
// and the ceiling is not — which is 0056's "two back walls and the floor"
// falling out of the arithmetic rather than being asserted. Below it, the
// arithmetic correctly draws a ceiling instead.
func TestTheFloorIsDrawnFromAboveAndTheCeilingFromBelow(t *testing.T) {
	above := newCube(theme.Light, project(Home(), ir.R(0, 0, 100, 100)), Home(), ticksOf(theme.Light, unitScales()), [3]string{})
	if above.far[axisZ] != 0 {
		t.Errorf("looking down at the scene drew the face at z=%v, want the floor at 0", above.far[axisZ])
	}
	low := LookAt(Azimuth(-0.6), Elevation(-0.35))
	below := newCube(theme.Light, project(low, ir.R(0, 0, 100, 100)), low, ticksOf(theme.Light, unitScales()), [3]string{})
	if below.far[axisZ] != 1 {
		t.Errorf("looking up at the scene drew the face at z=%v, want the ceiling at 1", below.far[axisZ])
	}
}

// Every tick label is upright. A label lying in a projected plane would need a
// shear, ir.TextRun has none, and a sheared tick label is harder to read than
// an upright one anyway — so this is a decision to keep rather than a
// limitation to work around.
func TestTickLabelsAreUpright(t *testing.T) {
	cam := LookAt(Azimuth(-1.1), Elevation(0.5))
	c := newCube(theme.Light, project(cam, ir.R(0, 0, 300, 300)), cam, ticksOf(theme.Light, unitScales()),
		[3]string{"x", "y", "z"})

	rec := irtest.New()
	var path ir.Path
	var boxes []ir.Rect
	c.draw(rec, &path, &boxes)

	titles := map[string]bool{"x": true, "y": true, "z": true}
	upright := 0
	for _, call := range rec.Filter("Text") {
		if titles[call.Text.Text] {
			continue
		}
		if call.Text.Rotation != 0 {
			t.Errorf("the tick label %q is rotated by %v", call.Text.Text, call.Text.Rotation)
		}
		upright++
	}
	if upright == 0 {
		t.Fatal("no tick labels were drawn at all")
	}
}

// An axis title takes the screen angle of its own axis, which is a rotation
// about its anchor and therefore something the IR can express — and it never
// reads upside down, whichever way the scene has been turned.
func TestAnAxisTitleFollowsItsAxisAndNeverReadsUpsideDown(t *testing.T) {
	for _, cam := range octants() {
		c := newCube(theme.Light, project(cam, ir.R(0, 0, 300, 300)), cam, ticksOf(theme.Light, unitScales()),
			[3]string{"across", "into", "up"})
		rec := irtest.New()
		var path ir.Path
		var boxes []ir.Rect
		c.draw(rec, &path, &boxes)

		found := 0
		for _, call := range rec.Filter("Text") {
			switch call.Text.Text {
			case "across", "into", "up":
				found++
				if r := call.Text.Rotation; r > math.Pi/2+1e-9 || r <= -math.Pi/2-1e-9 {
					t.Errorf("camera %+v: the title %q is rotated by %v, outside the half turn that reads left to right",
						cam, call.Text.Text, r)
				}
			}
		}
		if found != 3 {
			t.Errorf("camera %+v: %d axis titles were drawn, want three", cam, found)
		}
	}
}

// A tick pinned with scale.TickValues reaches the depth axis exactly as it
// reaches a flat one. A third scale is a third scale; nothing about it is new.
func TestPinnedTicksReachTheDepthAxis(t *testing.T) {
	sc := unitScales()
	z := scale.Linear(scale.Domain(0, 100), scale.TickValues(0, 37, 100))
	z.SetRange(0, 1)
	sc[axisZ] = z

	cam := Home()
	c := newCube(theme.Light, project(cam, ir.R(0, 0, 300, 300)), cam, ticksOf(theme.Light, sc), [3]string{})
	rec := irtest.New()
	var path ir.Path
	var boxes []ir.Rect
	c.draw(rec, &path, &boxes)

	want := map[string]bool{"0": false, "37": false, "100": false}
	for _, call := range rec.Filter("Text") {
		if _, ok := want[call.Text.Text]; ok {
			want[call.Text.Text] = true
		}
	}
	for label, seen := range want {
		if !seen {
			t.Errorf("the pinned tick %q is not on the depth axis", label)
		}
	}
}

// The labels sit outside the box rather than across its walls, at every angle.
func TestLabelsSitOutsideTheProjectedBox(t *testing.T) {
	for _, cam := range octants() {
		pr := project(cam, ir.R(0, 0, 300, 300))
		c := newCube(theme.Light, pr, cam, ticksOf(theme.Light, unitScales()), [3]string{})
		box := cubeBounds(pr)

		rec := irtest.New()
		var path ir.Path
		var boxes []ir.Rect
		c.draw(rec, &path, &boxes)
		for _, call := range rec.Filter("Text") {
			at := call.Text.At
			// "Outside" means outside the box's own middle half: a label may
			// legitimately overhang a corner, but one in the middle of a wall
			// is written across the data.
			inner := box.Inset(box.Dx()/4, box.Dy()/4, box.Dx()/4, box.Dy()/4)
			if inner.Contains(at) {
				t.Errorf("camera %+v: the label %q is at %v, inside the box %v",
					cam, call.Text.Text, at, inner)
			}
		}
	}
}

func cubeBounds(pr projector) ir.Rect {
	first := true
	var r ir.Rect
	for _, x := range []float32{0, 1} {
		for _, y := range []float32{0, 1} {
			for _, z := range []float32{0, 1} {
				p := pr.point(Vec3{x, y, z})
				if first {
					r, first = ir.Rect{Min: p, Max: p}, false
					continue
				}
				r.Min.X = min32(r.Min.X, p.X)
				r.Min.Y = min32(r.Min.Y, p.Y)
				r.Max.X = max32(r.Max.X, p.X)
				r.Max.Y = max32(r.Max.Y, p.Y)
			}
		}
	}
	return r
}

// An axis seen nearly end-on projects its whole length into a few pixels, and
// without a collision pass every one of its ticks lands in the same place: a
// pile of numbers rather than an axis. The pass has to span all three axes,
// because they meet at the corners of the box — two of them looked at end-on
// put their first labels in exactly the same spot.
func TestNoTwoTickLabelsOverlap(t *testing.T) {
	sc := unitScales()
	// A camera looking almost straight down, which is the case that collapses
	// the depth axis, and one looking along a floor axis, which collapses that
	// one. Both are angles a reader reaches by dragging.
	for _, cam := range []Camera{
		LookAt(Azimuth(-0.6), Elevation(1.45)),
		LookAt(Azimuth(0), Elevation(0.02)),
		LookAt(Azimuth(-math.Pi/2), Elevation(0.02)),
		Home(),
	} {
		c := newCube(theme.Light, project(cam, ir.R(0, 0, 300, 300)), cam,
			ticksOf(theme.Light, sc), [3]string{"x", "y", "z"})
		rec := irtest.New()
		var path ir.Path
		var boxes []ir.Rect
		c.draw(rec, &path, &boxes)

		titles := map[string]bool{"x": true, "y": true, "z": true}
		var drawn []ir.Rect
		for _, call := range rec.Filter("Text") {
			if titles[call.Text.Text] {
				continue
			}
			box := labelBox(call.Text, rec.Measure(call.Text), 0)
			for _, k := range drawn {
				if box.Min.X < k.Max.X && k.Min.X < box.Max.X &&
					box.Min.Y < k.Max.Y && k.Min.Y < box.Max.Y {
					t.Errorf("camera %+v: the label %q at %v overlaps one already drawn at %v",
						cam, call.Text.Text, box, k)
				}
			}
			drawn = append(drawn, box)
		}
		if len(drawn) == 0 {
			t.Errorf("camera %+v: every tick label was dropped", cam)
		}
	}
}

// A tick mark is a position and a label is a claim about how much room there
// is, so dropping a label never drops its mark: an axis collapsed to a few
// pixels still shows where its ticks are.
func TestDroppingALabelKeepsItsTickMark(t *testing.T) {
	sc := unitScales()
	cam := LookAt(Azimuth(0), Elevation(0.02))
	c := newCube(theme.Light, project(cam, ir.R(0, 0, 300, 300)), cam,
		ticksOf(theme.Light, sc), [3]string{})
	rec := irtest.New()
	var path ir.Path
	var boxes []ir.Rect
	c.draw(rec, &path, &boxes)

	// Every axis strokes one path holding its line and every one of its tick
	// marks, whatever happened to the labels.
	if got := rec.Count("StrokePath"); got < 3 {
		t.Errorf("got %d stroked paths, want at least one axis each", got)
	}
	labels := 0
	for range rec.Filter("Text") {
		labels++
	}
	total := 0
	for _, axis := range c.ticks {
		for _, tk := range axis {
			if tk.Label != "" {
				total++
			}
		}
	}
	if labels >= total {
		t.Errorf("%d of %d labels were drawn at an angle that collapses an axis; "+
			"the collision pass did nothing", labels, total)
	}
}

// An axis that projects shorter than one of its own labels shows none. The
// collision pass alone would keep exactly one, and one number on an axis with
// no length names a position the reader cannot tell from any other on it —
// which is a claim the picture does not support. The line and the title stay,
// because those say what the axis is rather than where a value sits on it.
func TestACollapsedAxisShowsNoTickLabelsAtAll(t *testing.T) {
	sc := unitScales()
	// Straight down: the depth axis projects to a point.
	cam := LookAt(Azimuth(-0.6), Elevation(math.Pi/2))
	c := newCube(theme.Light, project(cam, ir.R(0, 0, 300, 300)), cam,
		ticksOf(theme.Light, sc), [3]string{"x", "y", "up"})

	rec := irtest.New()
	var path ir.Path
	var boxes []ir.Rect
	c.draw(rec, &path, &boxes)

	// The z ticks of unitScales run 0..10, and none of them may appear; the
	// floor axes are unaffected and still label themselves.
	texts := map[string]int{}
	for _, call := range rec.Filter("Text") {
		texts[call.Text.Text]++
	}
	if texts["up"] == 0 {
		t.Error("the collapsed axis lost its title; the title is what still says which axis it is")
	}
	// Every tick label that survives belongs to a floor axis, and there are
	// two of those, so no value appears more than twice.
	for label, n := range texts {
		if label == "x" || label == "y" || label == "up" {
			continue
		}
		if n > 2 {
			t.Errorf("the label %q appears %d times; the collapsed axis is labelling itself", label, n)
		}
	}
	if texts["0"] == 0 {
		t.Error("the floor axes lost their labels too; only the collapsed one should")
	}
}

// The rule is measured rather than chosen — it asks how many of an axis's own
// labels survive beside each other, with nothing to tune — so it has to be
// monotone in the angle: an axis that has lost its numbers because it is too
// foreshortened does not get them back by being foreshortened further.
func TestAnAxisDoesNotRegainItsLabelsAsItShrinks(t *testing.T) {
	// Only the depth axis carries ticks, so nothing else competes for room
	// and the sweep measures one thing.
	z := scale.Linear(scale.Domain(0, 10))
	z.SetRange(0, 1)
	ticks := ticksOf(theme.Light, [3]scale.Scale{nil, nil, z})

	bare := false
	for el := 0.0; el < math.Pi/2; el += 0.02 {
		cam := LookAt(Azimuth(-0.6), Elevation(el))
		c := newCube(theme.Light, project(cam, ir.R(0, 0, 300, 300)), cam, ticks, [3]string{})

		rec := irtest.New()
		var path ir.Path
		var boxes []ir.Rect
		c.draw(rec, &path, &boxes)

		n := len(rec.Filter("Text"))
		if n == 1 {
			t.Fatalf("at elevation %.2f the depth axis shows one number and no scale", el)
		}
		if n == 0 {
			bare = true
		} else if bare {
			t.Fatalf("the depth axis regained its labels at elevation %.2f after losing them", el)
		}
	}
	if !bare {
		t.Fatal("the depth axis never ran out of room, even looking straight down")
	}
}

// An axis title sits beyond its tick labels, not among them.
//
// It is placed first and wins collisions, which is right — a title names the
// axis and a label is one reading off it — but only because it is out of their
// way to begin with. Placing it a guessed distance out rather than past the
// widest label is quiet and expensive: the boxes overlap, the title wins, and
// the axis silently loses most of its numbers.
func TestAnAxisTitleClearsItsOwnTickLabels(t *testing.T) {
	sc := unitScales()
	// A long title and wide labels, which is the arrangement that catches it.
	z := scale.Linear(scale.Domain(-60, -20))
	z.SetRange(0, 1)
	sc[axisZ] = z

	for _, cam := range octants() {
		c := newCube(theme.Light, project(cam, ir.R(0, 0, 300, 300)), cam,
			ticksOf(theme.Light, sc), [3]string{"frequency (MHz)", "sweep", "power (dBm)"})
		rec := irtest.New()
		var path ir.Path
		var boxes []ir.Rect
		c.draw(rec, &path, &boxes)

		labels := 0
		for _, call := range rec.Filter("Text") {
			switch call.Text.Text {
			case "frequency (MHz)", "sweep", "power (dBm)":
			default:
				labels++
			}
		}
		// Three axes with several ticks each: if a title were eating its
		// axis's labels this would collapse toward one per axis.
		if labels < 8 {
			t.Errorf("camera %+v: only %d tick labels survived three titled axes; "+
				"a title is standing in its own labels' way", cam, labels)
		}
	}
}

// A projected axis shows a scale or nothing, and never exactly one number.
//
// This is the failure the length rule exists to stop, and it is the quiet
// kind. A pile of overlapping numbers on an axis pointing at the reader looks
// wrong at a glance; a single surviving number looks like a label and reads
// like one, while naming a place the reader cannot tell from any other place
// on that axis. It survived the first version of the rule, which asked
// whether one label fitted and then let the collision pass keep one.
func TestAnAxisNeverShowsExactlyOneTickLabel(t *testing.T) {
	// Three disjoint domains, so the text of a label says which axis drew it.
	var sc [3]scale.Scale
	for a := range sc {
		s := scale.Linear(scale.Domain(float64(1000*a), float64(1000*a+9)))
		s.SetRange(0, 1)
		sc[a] = s
	}
	ticks := ticksOf(theme.Light, sc)

	for _, el := range []float64{0, 0.2, 0.6, 1.0, 1.3, 1.5, 1.56} {
		for _, az := range []float64{0, 0.4, 0.8, 1.2, -0.6, math.Pi / 4, math.Pi / 2} {
			cam := LookAt(Azimuth(az), Elevation(el))
			c := newCube(theme.Light, project(cam, ir.R(0, 0, 300, 300)), cam,
				ticks, [3]string{})

			rec := irtest.New()
			var path ir.Path
			var boxes []ir.Rect
			c.draw(rec, &path, &boxes)

			var drawn [3]int
			for _, call := range rec.Filter("Text") {
				v, err := strconv.ParseFloat(call.Text.Text, 64)
				if err != nil {
					t.Fatalf("a tick label that is not a number: %q", call.Text.Text)
				}
				drawn[int(v)/1000]++
			}
			for a, n := range drawn {
				if n == 1 {
					t.Fatalf("at azimuth %.2f elevation %.2f axis %d shows one number and no scale",
						az, el, a)
				}
			}
		}
	}
}

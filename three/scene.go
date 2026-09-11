package three

import (
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
)

// Scene is what is plotted: the layers, and the three scales they train.
//
// It is the data half of a three-dimensional chart, and it holds no camera.
// That split is the whole arrangement: a [View] is a camera on a scene,
// several views share one scene, and the scales behind it are trained once
// however many angles are drawn from them. One data repository, several ways
// of looking at it.
//
// It holds no theme and no chart title either. A theme is how the *figure*
// looks and two views in one figure must share it; a chart title names the
// figure. Both belong to [Plot]. The three axis titles belong here, because an
// axis names the data and every view of one scene labels the same three.
//
// A *Scene is shareable and is not safe for concurrent modification, which is
// the contract figure.Plot has.
type Scene struct {
	x, y, z                scale.Scale
	xTitle, yTitle, zTitle string
	layers                 []Layer
}

// SceneOption configures a [Scene].
type SceneOption func(*Scene)

// XTitle, YTitle and ZTitle label the three axes.
func XTitle(s string) SceneOption { return func(sc *Scene) { sc.xTitle = s } }

// YTitle labels the axis running into the scene.
func YTitle(s string) SceneOption { return func(sc *Scene) { sc.yTitle = s } }

// ZTitle labels the axis running up.
func ZTitle(s string) SceneOption { return func(sc *Scene) { sc.zTitle = s } }

// NewScene returns an empty scene.
func NewScene(opts ...SceneOption) *Scene {
	s := &Scene{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// X sets the scale of the axis running across the floor.
func (s *Scene) X(sc scale.Scale) *Scene { s.x = sc; return s }

// Y sets the scale of the axis running into the floor.
func (s *Scene) Y(sc scale.Scale) *Scene { s.y = sc; return s }

// Z sets the scale of the axis running up, which is the one this package
// exists for.
func (s *Scene) Z(sc scale.Scale) *Scene { s.z = sc; return s }

// Add appends layers.
func (s *Scene) Add(ls ...Layer) *Scene { s.layers = append(s.layers, ls...); return s }

// SetLayers replaces the layers, which is what a scene that gains a highlight
// layer on every pointer move needs so that it does not gain one per move.
func (s *Scene) SetLayers(ls ...Layer) *Scene {
	s.layers = append(s.layers[:0:0], ls...)
	return s
}

// Layers returns a copy of the layers.
func (s *Scene) Layers() []Layer { return append([]Layer(nil), s.layers...) }

// scales returns the three, filling in the default for any the caller left
// out — the same default a flat plot's axis gets when nobody chose.
func (s *Scene) scales() [3]scale.Scale {
	return [3]scale.Scale{orLinear(s.x), orLinear(s.y), orLinear(s.z)}
}

func (s *Scene) titles() [3]string { return [3]string{s.xTitle, s.yTitle, s.zTitle} }

func orLinear(s scale.Scale) scale.Scale {
	if s == nil {
		return scale.Linear(scale.Nice())
	}
	return s
}

// train feeds every layer's data into the three scales and ranges them into
// the unit cube.
//
// It runs once per frame however many views are drawn from the scene, because
// a domain is a fact about the data and not about where the data is looked at
// from. What must not happen is training per view — a scale accumulates, so
// three views would give a layer three times the weight it has.
func (s *Scene) train(sc [3]scale.Scale) error {
	t := geom.Training{X: sc[axisX], Y: sc[axisY], Z: sc[axisZ]}
	for _, l := range s.layers {
		if err := l.Train(t); err != nil {
			return err
		}
	}
	// Every scale maps into an edge of the unit cube. Choosing what the
	// interval means is what this stage does, exactly as coord.Cartesian
	// chooses a distance along an edge of the panel and coord.Polar chooses
	// radians — so anything that assumes a scale's range is in pixels is wrong
	// under this package.
	//
	// The depth axis is not flipped. Larger is higher, and the one sign that
	// turns the scene's up into the screen's down lives in the projection.
	for _, x := range sc {
		x.SetRange(0, 1)
	}
	return nil
}

// layerLabels are the legend labels of the layers, which is what a hit index
// is told a layer is called.
func (s *Scene) layerLabels(f Frame) []string {
	out := make([]string, len(s.layers))
	for i, l := range s.layers {
		g := f
		g.Index = i
		if e, ok := l.Legend(g); ok {
			out[i] = e.Label
		}
	}
	return out
}

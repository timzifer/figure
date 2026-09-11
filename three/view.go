package three

// View is one camera on a scene.
//
// It is a struct with exported fields because it grows: a per-view layer mask,
// a clip to a sub-box of the cube, a per-view theme override are all fields
// when they arrive rather than a second View type. ADR 0060 is the rule.
//
// Where a view lands in the figure is [Plot]'s to say rather than the view's,
// because a row and a column on a struct have no honest zero value — nought,
// nought is a real cell. Views are placed in the order they were added, across
// the rows of a grid whose width is [Columns].
type View struct {
	// Camera is where this view looks from. The zero value is the front view;
	// [Home] is the angle a chart is designed at, and the one a static export
	// and a reader who cannot drag both get.
	Camera Camera

	// Label is written in a band above the view, or "" for none. It is what
	// names a cell of a front / top / side figure.
	Label string

	// Scene overrides the plot's scene for this view, and is nil for the views
	// that share it — which is the ordinary case and the one this whole
	// arrangement is for.
	Scene *Scene
}

// sceneOr resolves a view's scene against the plot's.
func (v View) sceneOr(def *Scene) *Scene {
	if v.Scene != nil {
		return v.Scene
	}
	return def
}

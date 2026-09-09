package ir

// markerKappa is the cubic Bézier constant that approximates a quarter circle.
const markerKappa = 0.5522847498307936

// MarkerPath appends the outline of a marker shape to p, centred on the origin
// and sized so that its nominal extent is size.
//
// It is exported because every backend has to draw the same diamond. A marker
// that is a slightly different shape in PDF than in SVG is the kind of
// difference nobody notices until it matters, and a backend written outside
// this module has no other way to get the shape right. A backend that draws
// markers itself calls this rather than reading [Marker] — which is also what
// keeps a marker added in a later release from silently becoming a circle
// everywhere but here.
//
// An unknown Marker appends a circle, because a sink for ink cannot refuse.
func MarkerPath(p *Path, m Marker, size float32) {
	r := size / 2
	switch m {
	case MarkerSquare:
		p.Rect(R(-r, -r, r, r))
	case MarkerDiamond:
		p.MoveTo(0, -r).LineTo(r, 0).LineTo(0, r).LineTo(-r, 0).Close()
	case MarkerTriangle:
		// Centroid-centred equilateral triangle pointing up.
		h := r * 1.5
		w := r * 1.2990381 // r * 1.5 * sqrt(3)/2
		p.MoveTo(0, -h*2/3).LineTo(w, h/3).LineTo(-w, h/3).Close()
	case MarkerCross:
		arm := r * 0.28
		d := (r - arm) * 0.7071068
		p.MoveTo(-d-arm, -d).LineTo(-d, -d-arm).LineTo(0, -arm*1.4142136).
			LineTo(d, -d-arm).LineTo(d+arm, -d).LineTo(arm*1.4142136, 0).
			LineTo(d+arm, d).LineTo(d, d+arm).LineTo(0, arm*1.4142136).
			LineTo(-d, d+arm).LineTo(-d-arm, d).LineTo(-arm*1.4142136, 0).Close()
	case MarkerPlus:
		arm := r * 0.28
		p.MoveTo(-arm, -r).LineTo(arm, -r).LineTo(arm, -arm).LineTo(r, -arm).
			LineTo(r, arm).LineTo(arm, arm).LineTo(arm, r).LineTo(-arm, r).
			LineTo(-arm, arm).LineTo(-r, arm).LineTo(-r, -arm).LineTo(-arm, -arm).Close()
	default: // MarkerCircle
		k := r * markerKappa
		p.MoveTo(r, 0).
			CubicTo(r, k, k, r, 0, r).
			CubicTo(-k, r, -r, k, -r, 0).
			CubicTo(-r, -k, -k, -r, 0, -r).
			CubicTo(k, -r, r, -k, r, 0).
			Close()
	}
}

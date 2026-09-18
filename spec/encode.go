package spec

import (
	"fmt"
	"time"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
)

// Of writes a chart down.
//
// It fails rather than guesses. A layer or a scale that cannot describe itself
// — a third-party one that implements [geom.Describer] or [scale.Describer]
// nowhere — is an error, because a spec missing a layer is a spec that draws a
// different chart, and finding that out at read time is worse than finding it
// out here.
func Of(c Chart) (Spec, error) {
	s := Spec{
		Schema: Schema,
		Width:  c.Width,
		Height: c.Height,
		Title:  c.Title,
	}

	enc := &Encoding{}
	var err error
	if enc.X, err = axisChannel(c.X, c.XTitle); err != nil {
		return Spec{}, fmt.Errorf("figure/spec: x axis: %w", err)
	}
	if enc.Y, err = axisChannel(c.Y, c.YTitle); err != nil {
		return Spec{}, fmt.Errorf("figure/spec: y axis: %w", err)
	}
	if enc.YSecondary, err = axisChannel(c.Y2, c.Y2Title); err != nil {
		return Spec{}, fmt.Errorf("figure/spec: secondary y axis: %w", err)
	}
	if enc.XSecondary, err = axisChannel(c.X2, c.X2Title); err != nil {
		return Spec{}, fmt.Errorf("figure/spec: secondary x axis: %w", err)
	}
	if enc.X != nil || enc.Y != nil || enc.YSecondary != nil || enc.XSecondary != nil {
		s.Encoding = enc
	}

	if s.Coord, err = encodeCoord(c.Coord); err != nil {
		return Spec{}, err
	}

	shared, hoist := commonSource(c.Layers)
	if hoist {
		if s.Data, err = encodeData(shared); err != nil {
			return Spec{}, err
		}
	}
	axes := axisKinds{x: kindOf(c.X), y: kindOf(c.Y), y2: kindOf(c.Y2), x2: kindOf(c.X2)}
	for i, g := range c.Layers {
		l, err := encodeLayer(g, hoist, axes)
		if err != nil {
			return Spec{}, fmt.Errorf("figure/spec: layer %d: %w", i, err)
		}
		s.Layer = append(s.Layer, l)
	}

	if c.Facet != nil {
		fd := c.Facet.Describe()
		if fd.Wrap {
			s.Facet = &Facet{Field: fd.Col, Type: "nominal"}
			s.Columns = fd.Columns
		} else {
			s.Facet = &Facet{
				Row:    &FacetField{Field: fd.Row, Type: "nominal"},
				Column: &FacetField{Field: fd.Col, Type: "nominal"},
			}
		}
		if fd.FreeX || fd.FreeY {
			s.Resolve = &Resolve{Scale: &ResolveScale{
				X: resolution(fd.FreeX),
				Y: resolution(fd.FreeY),
			}}
		}
	}

	for i, tr := range c.Tracks {
		d, err := encodeTrack(tr, kindOf(c.X), kindOf(c.Y))
		if err != nil {
			return Spec{}, fmt.Errorf("figure/spec: track %d: %w", i, err)
		}
		s.Tracks = append(s.Tracks, d)
	}

	cfg := &Config{Legend: c.Legend}
	if c.Theme.Name != "" {
		cfg.Theme = c.Theme.Name
	}
	if c.DPR != 0 && c.DPR != 1 {
		cfg.DevicePixelRatio = c.DPR
	}
	if *cfg != (Config{}) {
		s.Config = cfg
	}
	return s, nil
}

func resolution(free bool) string {
	if free {
		return Independent
	}
	return Shared
}

// axisChannel writes the plot's scale for one axis.
func axisChannel(s scale.Scale, title string) (*Channel, error) {
	if s == nil {
		if title == "" {
			return nil, nil
		}
		return &Channel{Title: title}, nil
	}
	d, ok := scale.Describe(s)
	if !ok {
		return nil, fmt.Errorf("%T cannot describe itself: it does not implement scale.Describer", s)
	}
	if d.Kind == scale.KindProbability && d.Link == "" {
		// A document that dropped the link would read back as probit paper,
		// which is a different axis rather than the same one labelled less.
		return nil, fmt.Errorf("a probability scale with a link of its own cannot be written down: only a link this library names has a name to write")
	}
	return &Channel{Type: channelType(d.Kind), Title: title, Scale: encodeScale(d)}, nil
}

// channelType is Vega-Lite's measurement type for a scale kind.
func channelType(k scale.Kind) string {
	switch k {
	case scale.KindTime:
		return "temporal"
	case scale.KindOrdinal:
		return "nominal"
	}
	return "quantitative"
}

func encodeScale(d scale.Desc) *Scale {
	out := &Scale{Nice: d.Nice, Zero: d.Zero, Format: d.Format, Locale: d.Locale}
	switch d.Kind {
	case scale.KindLinear:
		// Reverse is written only for the kind that reads it back. On a
		// positional scale the word means the axis runs the other way and on a
		// colour scale it means the ramp does, which is one word for two ideas
		// — tolerable because the channel a scale hangs off says which of them
		// is in question, and misleading the moment a kind that ignores it
		// writes it down anyway.
		out.Type, out.TickValues, out.Reverse = "linear", d.TickValues, d.Reverse
	case scale.KindLog:
		out.Type, out.Base = "log", d.Base
		out.MinorTicks = boolPtr(d.MinorTicks)
	case scale.KindSymLog:
		out.Type, out.Base, out.Constant = "symlog", d.Base, d.Threshold
		out.MinorTicks = boolPtr(d.MinorTicks)
	case scale.KindProbability:
		out.Type, out.Link = "probability", d.Link
		out.MinorTicks = boolPtr(d.MinorTicks)
	case scale.KindTime:
		out.Type, out.TimeZone = "time", d.Location
		// A time scale's declarative format is a layout, and it travels in
		// the same field a numeric scale's number format does: the type says
		// which of the two this is.
		out.Format = d.Layout
		if d.Origin != 0 {
			out.Origin = time.Unix(0, d.Origin).UTC().Format(timeLayout)
		}
	case scale.KindOrdinal:
		// "band" is Vega-Lite's name for a scale that gives each category a
		// slot of finite width, which is what this one does — see scale.Band.
		out.Type = "band"
		out.Padding = float64Ptr(d.Padding)
		for _, c := range d.Categories {
			out.Domain = append(out.Domain, c)
		}
	default:
		// A kind this package did not define — one built by [scale.Register] —
		// travels under its own name.
		out.Type = string(d.Kind)
	}
	if d.Fixed {
		switch d.Kind {
		case scale.KindTime:
			// The bounds are written as instants, so they are read back
			// against whatever origin the document carries rather than
			// against the one this axis happens to have.
			out.Domain = []any{
				time.Unix(0, d.Origin+int64(d.Min)).UTC().Format(timeLayout),
				time.Unix(0, d.Origin+int64(d.Max)).UTC().Format(timeLayout),
			}
		case scale.KindOrdinal:
			// The categories are already the domain.
		default:
			out.Domain = []any{d.Min, d.Max}
		}
	}
	return out
}

func encodeColorScale(cs scale.ColorScale) (*Scale, error) {
	d, ok := scale.DescribeColor(cs)
	if !ok {
		return nil, fmt.Errorf("%T cannot describe itself: it does not implement scale.ColorDescriber", cs)
	}
	out := &Scale{
		Type: string(d.Kind), Scheme: d.Ramp, Reverse: d.Reverse,
		Transform: string(d.Transform),
	}
	if d.Transform != scale.TransformLinear {
		// The defaults are left out: a document that does not say which base
		// gets the same scale back, and writing 10 into every log ramp would
		// suggest the number was a choice.
		if d.Base != 10 {
			out.Base = d.Base
		}
		if d.Transform == scale.TransformSymLog && d.Constant != 1 {
			out.Constant = d.Constant
		}
	}
	out.Breaks, out.Classes = d.Breaks, d.Classes
	out.Layers = d.Layers
	for _, c := range d.Colors {
		out.Range = append(out.Range, colorHex(c))
	}
	for _, label := range d.Labels {
		// A named scale's categories and their colours are a domain/range
		// pair, which is how Vega-Lite spells an explicit mapping from
		// categories to a discrete range. The scheme beside them is the
		// fallback rather than the mapping, which is the one thing this
		// document says that Vega-Lite's cannot.
		out.Domain = append(out.Domain, label)
	}
	for _, c := range d.Fallback {
		out.Fallback = append(out.Fallback, colorHex(c))
	}
	if d.Fixed && d.Kind != scale.KindQualitative && d.Kind != scale.KindNamed {
		// A discrete scale's domain is the labels it has been shown, and those
		// are the data rather than the scale — the same line an ordinal axis
		// draws between a fixed category set and a discovered one.
		out.Domain = []any{d.Min, d.Max}
	}
	if d.Kind == scale.KindDiverging {
		out.Center = float64Ptr(d.Center)
	}
	if d.Undefined.A != 0 {
		out.Undefined = colorHex(d.Undefined)
	}
	return out, nil
}

// encodeSizeScale writes a size scale down.
//
// Its type is spelled out — "size" — because the mapping is by area rather than
// by radius and a document that left the type off would read as a plain linear
// range. What is *not* written is a range the theme decided: that is a property
// of how the chart was drawn rather than of the chart, and pinning it here would
// stop a spec drawn at another size from scaling its bubbles.
func encodeSizeScale(ss scale.SizeScale) (*Scale, error) {
	d, ok := scale.DescribeSize(ss)
	if !ok {
		return nil, fmt.Errorf("%T cannot describe itself: it does not implement scale.SizeDescriber", ss)
	}
	out := &Scale{Type: "size"}
	if d.Fixed {
		out.Domain = []any{d.Min, d.Max}
	}
	if d.RangeSet {
		out.SizeRange = []float32{d.MinSize, d.MaxSize}
	}
	if d.ZeroSet {
		out.SizeZero = float64Ptr(d.Zero)
	}
	return out, nil
}

// colorChannelType is the measurement type of a colour encoding: a category
// for a discrete scale, a quantity for a ramp. Vega-Lite draws the same
// distinction with the same two words.
func colorChannelType(s *Scale) string {
	if s != nil && (s.Type == string(scale.KindQualitative) || s.Type == string(scale.KindNamed)) {
		return "nominal"
	}
	return "quantitative"
}

// commonSource reports the source every layer with data draws from, and
// whether there is exactly one.
//
// A chart usually has one table behind all its layers, and writing it once is
// the difference between a readable document and the same million rows three
// times over.
func commonSource(layers []geom.Geom) (data.Source, bool) {
	var first data.Source
	for _, g := range layers {
		d, ok := geom.Describe(g)
		if !ok || d.Source == nil {
			continue
		}
		if first == nil {
			first = d.Source
			continue
		}
		if !sameSource(first, d.Source) {
			return nil, false
		}
	}
	return first, first != nil
}

func encodeLayer(g geom.Geom, hoisted bool, axes axisKinds) (Layer, error) {
	d, ok := geom.Describe(g)
	if !ok {
		return Layer{}, fmt.Errorf("%T cannot describe itself: it does not implement geom.Describer", g)
	}
	typ, orient, err := markType(d.Mark)
	if err != nil {
		return Layer{}, err
	}

	m := Mark{Type: typ, Orient: orient, Extra: d.Extra}
	if d.Mark == geom.MarkLocus {
		// A named family round-trips as its name, and one a caller wrote in Go
		// does not round-trip at all. Saying so is the point: a document that
		// dropped the family would decode into a layer that draws nothing, and
		// the escape hatch is the one docs/adr/0041-qq-plots.md already chose —
		// materialise the curve as data and draw it with a line.
		name, ok := stat.FamilyName(d.Family)
		if !ok {
			return Layer{}, fmt.Errorf("a locus of %T cannot be written down: only a family this library names has a name to write", d.Family)
		}
		m.Family = name
	}
	if d.OnY2 {
		m.YAxis = axisSecondaryY
	}
	if d.OnX2 {
		m.XAxis = axisSecondaryX
	}
	if d.Color != nil {
		m.Color = colorHex(*d.Color)
	}
	writeMarkProps(&m, d)

	l := Layer{Name: d.Label, Mark: m}
	if l.Encoding, err = encodeLayerEncoding(d, axes); err != nil {
		return Layer{}, err
	}
	if d.Source != nil && !hoisted {
		if l.Data, err = encodeData(d.Source); err != nil {
			return Layer{}, err
		}
	}
	if d.Links != nil {
		if l.Links, err = encodeData(d.Links); err != nil {
			return Layer{}, err
		}
	}
	return l, nil
}

// writeMarkProps writes the properties the mark actually uses, and no others.
//
// A geom accepts every option and ignores the ones it has no use for, which is
// what keeps the option set one namespace instead of six — but a *document*
// listing a line's whisker extent and bar width is a document that reads as
// though those meant something. So the mark decides what is written, and the
// round trip is unaffected: an option the mark ignores draws nothing either
// way.
func writeMarkProps(m *Mark, d geom.Desc) {
	// Every mark that can have a guide can decline it, and the default is
	// left unwritten.
	if d.HideGuide {
		m.Guide = boolPtr(false)
	}
	stroke := func() {
		m.StrokeWidth = d.Width
		if d.DashSet {
			m.StrokeDash = d.Dash
			if m.StrokeDash == nil {
				// An explicit geom.Dash() with no pattern means solid, and an
				// absent field means "not set". Those are different, so the
				// empty list has to survive.
				m.StrokeDash = []float32{}
			}
		}
	}
	fill := func() {
		if d.Fill != nil {
			m.Fill = colorHex(*d.Fill)
		}
		if d.Opacity >= 0 {
			m.Opacity = float64Ptr(d.Opacity)
		}
		if d.HatchSet {
			m.Hatch = hatchName(d.Hatch)
		}
		if d.HatchDensity > 0 {
			m.HatchDensity = d.HatchDensity
		}
		if d.HatchWidth > 0 {
			m.HatchWidth = d.HatchWidth
		}
		if d.HatchColorSet {
			m.HatchColor = colorHex(d.HatchColor)
		}
		if d.Gradient != nil {
			m.Gradient = colorHex(*d.Gradient)
		}
		m.Corner, m.Inset = d.Corner, d.Inset
	}
	// The default policy and the default reduction are written only when they
	// are not the default: a document should say what was chosen, not repeat
	// what was not.
	rows := func() {
		if d.Missing != geom.Gap {
			m.Missing = missingName(d.Missing)
		}
		if d.Decimate != geom.AutoDecimation {
			m.Decimate = decimationName(d.Decimate)
		}
		m.Budget = d.Budget
	}
	// The adjustment is written by the marks that have one. The stack itself
	// goes on the positional channel, where Vega-Lite puts it and where this
	// function cannot reach; what is left here is the pair of choices that are
	// about the marks rather than about the axis.
	adjust := func() {
		if !d.Dodge {
			return
		}
		m.Dodge = float64Ptr(d.DodgePad)
	}
	group := func() {
		adjust()
		if d.Group != "" && d.Order != geom.OrderAppearance {
			m.Order = orderName(d.Order)
		}
	}
	// The distribution marks' own numbers. Each is written by the marks that
	// read it and by no others, the same rule the rest of this function follows:
	// a document listing a trend's bandwidth would read as though that meant
	// something.
	binning := func() {
		m.Bins = d.Bins
		if d.BinLo < d.BinHi {
			m.BinStart, m.BinEnd = float64Ptr(d.BinLo), float64Ptr(d.BinHi)
		}
	}
	density := func() {
		m.Bandwidth = d.Bandwidth
	}

	// A contour that joins back to its start is a property of the connected
	// marks and of nothing else, so it is written by those three and not by
	// the six that would ignore it.
	loop := func() {
		m.Closed = d.Closed
	}

	// The curve family and its parameter travel together, because a tension
	// with no family named is what "cardinal" always meant — which is why a
	// layer that only set a tension still writes that word.
	curve := func() {
		switch {
		case d.CurveSet:
			m.Interpolate = curveName(d.Curve)
		case d.Tension > 0:
			m.Interpolate = curveName(geom.CurveCardinal)
		}
		m.Tension = d.Tension
	}

	switch d.Mark {
	case geom.MarkLine:
		stroke()
		rows()
		group()
		loop()
		curve()
	case geom.MarkStep:
		stroke()
		rows()
		group()
		loop()
		m.Interpolate = stepName(d.Steps)
	case geom.MarkScatter:
		stroke()
		fill()
		rows()
		group()
		m.Size = d.Size
		// The shape is written when the layer chose one. Left out, the mark is
		// a circle unless the theme's redundant encoding has an opinion — and
		// a document that spelled "circle" out would pin it and lose that.
		if d.MarkerSet {
			m.Shape = shapeName(d.Marker)
		}
		m.DensityCells = d.CellSize
	case geom.MarkBar:
		stroke()
		fill()
		rows()
		group()
		m.BarWidth, m.Origin = float64Ptr(d.BarWidth), d.Baseline
		m.Explode = d.Explode
		m.Extrude = d.Extrude
	case geom.MarkRect:
		stroke()
		fill()
		m.Extrude = d.Extrude
		m.BarWidth = float64Ptr(d.BarWidth)
		m.Explode = d.Explode
	case geom.MarkDepends:
		// An arrow is a stroke and a head, so it writes a stroke, the head's
		// length as the mark's size, and which pair of edges it joins where
		// the row does not say. It writes no fill: the head takes the stroke's
		// colour, because an arrow with two colours in it is two marks.
		stroke()
		m.Size = d.Size
		m.BarWidth = float64Ptr(d.BarWidth)
		if d.Linkage != geom.FinishToStart {
			m.Link = d.Linkage.String()
		}
	case geom.MarkArea:
		stroke()
		fill()
		rows()
		group()
		loop()
		m.Origin = d.Baseline
		curve()
	case geom.MarkBoxplot:
		stroke()
		fill()
		rows()
		m.BarWidth = float64Ptr(d.BarWidth)
		m.Extent, m.Outliers = d.Whisker, boolPtr(d.Outliers)
	case geom.MarkHistogram:
		stroke()
		fill()
		binning()
		m.Origin = d.Baseline
	case geom.MarkViolin:
		stroke()
		fill()
		adjust()
		density()
		m.BarWidth = float64Ptr(d.BarWidth)
	case geom.MarkRidgeline:
		stroke()
		fill()
		density()
		m.Overlap = d.Overlap
	// The relational layouts. Each writes the gap it leaves between its shapes;
	// the two that place nodes also write how thick a node is, and the one
	// whose picture depends on where its rail sits writes that.
	case geom.MarkTreemap, geom.MarkIcicle:
		fill()
		m.Padding = d.Padding
	case geom.MarkSankey:
		fill()
		m.Padding, m.Thickness = d.Padding, d.Thickness
	case geom.MarkArc:
		fill()
		m.Padding, m.Thickness, m.Origin = d.Padding, d.Thickness, d.Baseline
	case geom.MarkTree:
		// A tree is its branches, so it writes a stroke and no fill; where its
		// root sits, and the branch shape when it is not the elbow.
		stroke()
		m.Origin = d.Baseline
		if d.Branch == geom.Straight {
			m.Branch = "straight"
		}
		if d.Orient == geom.Horizontal {
			m.Orientation = "horizontal"
		}
	case geom.MarkGraph:
		// A graph is boxes and arrows, so it writes both a fill and a stroke,
		// where rank zero sits, the edge shape when it is not the elbow, and
		// the head's length as the mark's size.
		fill()
		stroke()
		m.Origin = d.Baseline
		m.Size = d.Size
		if d.Branch == geom.Straight {
			m.Branch = "straight"
		}
	case geom.MarkHexbin:
		fill()
		m.DensityCells = d.CellSize
	case geom.MarkContour:
		stroke()
		// The levels the layer is actually tracing, so that a document reads
		// back as the same lines — which is the whole reason to pin them.
		m.Levels, m.LevelCount = d.Levels, d.LevelCount
		m.LabelLevels = d.LabelLevels
	case geom.MarkRaster:
		// The reduction and nothing else. A raster has neither a stroke nor a
		// fill of its own — every pixel takes its colour from the ramp — and
		// no group, because the table it reads is one value per cell.
		if d.Resample != geom.Nearest {
			m.Resample = resampleName(d.Resample)
		}
	case geom.MarkHorizon:
		fill()
		rows()
		// The fold and nothing else. A horizon has no stroke — every band is a
		// filled region whose only edge is the one the next band starts at —
		// and no group, because a grouped one is an error rather than a
		// picture.
		m.Bands, m.BandHeight, m.Origin = d.Bands, d.BandHeight, d.Baseline
	case geom.MarkBeeswarm:
		stroke()
		fill()
		group()
		m.Size, m.BarWidth = d.Size, float64Ptr(d.BarWidth)
		if d.MarkerSet {
			m.Shape = shapeName(d.Marker)
		}
	case geom.MarkErrorBar:
		stroke()
		rows()
		group()
		m.Size, m.BarWidth, m.Caps = d.Size, float64Ptr(d.BarWidth), boolPtr(d.Caps)
		if d.MarkerSet {
			m.Shape = shapeName(d.Marker)
		}
	case geom.MarkECDF:
		stroke()
		group()
	case geom.MarkIntersections:
		fill()
		// The ranking and nothing else: which columns there are is the whole
		// of this mark's configuration, and both halves of the chart carry it.
		m.Top, m.Order = d.Top, rankedBy(d)
	case geom.MarkSetMatrix:
		stroke()
		m.Size, m.Top, m.Order = d.Size, d.Top, rankedBy(d)
		if d.MarkerSet {
			m.Shape = shapeName(d.Marker)
		}
	case geom.MarkVenn:
		fill()
		stroke()
	case geom.MarkSurvival:
		stroke()
		group()
		m.Confidence, m.CensorMarks = d.Confidence, d.CensorMarks
	case geom.MarkQQ:
		stroke()
		fill()
		group()
		m.Size = d.Size
		if d.MarkerSet {
			m.Shape = shapeName(d.Marker)
		}
	case geom.MarkTrend:
		stroke()
		group()
		m.Span, m.Method = d.Span, smoothName(d.Smooth)
		curve()
	case geom.MarkLocus:
		stroke()
		// The levels and nothing else places this mark: a locus has no datum,
		// because it is not at a value of either axis. Its family is written by
		// the encoder rather than here, because a family with no name is a
		// layer that cannot be written down at all — see [encodeLayer].
		m.Levels, m.Extend = d.Levels, boolPtr(d.Extend)
		m.LabelLevels = d.LabelLevels
	case geom.MarkHLine, geom.MarkVLine, geom.MarkSegment:
		stroke()
		m.Extend = boolPtr(d.Extend)
	case geom.MarkHBand, geom.MarkVBand, geom.MarkRegion:
		stroke()
		fill()
		m.Extend = boolPtr(d.Extend)
	case geom.MarkNote:
		m.Text, m.FontSize, m.Angle = d.Text, d.FontSize, degrees(d.Rotation)
		m.Align, m.Baseline = hAlignName(d.HAlign), vAlignName(d.VAlign)
		m.Extend = boolPtr(d.Extend)
	case geom.MarkText:
		// The label is a column rather than a string, so it travels on the
		// encoding; what is left here is how the run is drawn. The alignment
		// is written only when the layer was told, because a text layer that
		// was not centres its labels in their boxes — writing "left" for that
		// would pin the default and change the chart.
		m.FontSize, m.Angle = d.FontSize, degrees(d.Rotation)
		if d.AlignSet {
			m.Align, m.Baseline = hAlignName(d.HAlign), vAlignName(d.VAlign)
		}
		m.Elide = d.Elide
		m.AvoidOverlap = d.AvoidOverlap
	default:
		// A mark this package did not define. Nobody here knows which of the
		// shared options it reads, so the ones a mark most plausibly honours
		// are written and the mark's own properties travel in Extra: a
		// document listing a property the mark ignores draws nothing either
		// way, which is the same bargain the option set makes.
		stroke()
		fill()
		rows()
		group()
		loop()
		m.Size = d.Size
		if d.MarkerSet {
			m.Shape = shapeName(d.Marker)
		}
		curve()
	}
}

func encodeLayerEncoding(d geom.Desc, axes axisKinds) (*Encoding, error) {
	enc := &Encoding{}
	// An annotation's literal values are written in the spelling of the axis
	// it is placed against, which is the second one where the layer asked for
	// it — a threshold on a temporal secondary axis is a timestamp even on a
	// chart whose first axis is a number line.
	axes.y, axes.x = axes.vertical(d.OnY2), axes.horizontal(d.OnX2)
	if d.Source != nil {
		if d.X != "" {
			enc.X = &Channel{Field: d.X}
		}
		if d.Y != "" {
			enc.Y = &Channel{Field: d.Y}
		}
		if d.X2 != "" {
			enc.X2 = &Channel{Field: d.X2}
		}
		if d.Y2 != "" {
			enc.Y2 = &Channel{Field: d.Y2}
		}
		if d.ColorCol != "" && d.ColorScale != nil {
			cs, err := encodeColorScale(d.ColorScale)
			if err != nil {
				return nil, err
			}
			enc.Color = &Channel{Field: d.ColorCol, Type: colorChannelType(cs), Scale: cs}
		}
		if d.Group != "" {
			enc.Detail = &Channel{Field: d.Group, Type: "nominal"}
		}
		if d.Z != "" {
			enc.Z = &Channel{Field: d.Z}
		}
		if d.WidthCol != "" {
			enc.Width = &Channel{Field: d.WidthCol}
		}
		if d.Key != "" {
			enc.Key = &Channel{Field: d.Key}
		}
		if d.ExplodeCol != "" {
			enc.Explode = &Channel{Field: d.ExplodeCol}
		}
		if d.TextCol != "" {
			enc.Text = &Channel{Field: d.TextCol}
		}
		if d.MidCol != "" {
			enc.Mid = &Channel{Field: d.MidCol}
		}
		if d.ErrorCol != "" {
			enc.Error = &Channel{Field: d.ErrorCol}
		}
		if d.ErrorXCol != "" {
			enc.ErrorX = &Channel{Field: d.ErrorXCol}
		}
		if d.From != "" {
			enc.From = &Channel{Field: d.From, Type: "nominal"}
		}
		if d.To != "" {
			enc.To = &Channel{Field: d.To, Type: "nominal"}
		}
		if d.ID != "" {
			enc.ID = &Channel{Field: d.ID, Type: "nominal"}
		}
		if d.ParentCol != "" {
			enc.Parent = &Channel{Field: d.ParentCol, Type: "nominal"}
		}
		if d.ValueCol != "" {
			enc.Value = &Channel{Field: d.ValueCol, Type: "quantitative"}
		}
		if d.EventCol != "" {
			enc.Event = &Channel{Field: d.EventCol, Type: "quantitative"}
		}
		if d.UncertaintyCol != "" {
			enc.Uncertainty = &Channel{Field: d.UncertaintyCol, Type: "quantitative"}
		}
		if d.ProgressCol != "" {
			enc.Progress = &Channel{Field: d.ProgressCol, Type: "quantitative"}
		}
		if d.LinkCol != "" {
			enc.Link = &Channel{Field: d.LinkCol, Type: "nominal"}
		}
		if d.SizeCol != "" && d.SizeScale != nil {
			ss, err := encodeSizeScale(d.SizeScale)
			if err != nil {
				return nil, err
			}
			enc.Size = &Channel{Field: d.SizeCol, Type: "quantitative", Scale: ss}
		}
		// The stack is a property of the axis the groups are stacked along,
		// which is the Y axis for every mark that has one — so it goes on the
		// Y channel, where Vega-Lite puts it and where a reader will look.
		if d.StackSet && enc.Y != nil {
			enc.Y.Stack = stackName(d.Stack)
		}
		if *enc == (Encoding{}) {
			return nil, nil
		}
		return enc, nil
	}

	// An annotation is placed by values rather than columns, which is what
	// Vega-Lite's `datum` is for.
	switch d.Mark {
	case geom.MarkHLine:
		enc.Y = axes.y.datum(d.Datum.Y0)
	case geom.MarkVLine:
		enc.X = axes.x.datum(d.Datum.X0)
	case geom.MarkHBand:
		enc.Y, enc.Y2 = axes.y.datum(d.Datum.Y0), axes.y.datum(d.Datum.Y1)
	case geom.MarkVBand:
		enc.X, enc.X2 = axes.x.datum(d.Datum.X0), axes.x.datum(d.Datum.X1)
	case geom.MarkSegment, geom.MarkRegion:
		enc.X, enc.Y = axes.x.datum(d.Datum.X0), axes.y.datum(d.Datum.Y0)
		enc.X2, enc.Y2 = axes.x.datum(d.Datum.X1), axes.y.datum(d.Datum.Y1)
	case geom.MarkNote:
		enc.X, enc.Y = axes.x.datum(d.Datum.X0), axes.y.datum(d.Datum.Y0)
	default:
		// A registered annotation carries whatever of its four values it set,
		// the way a region does.
		if d.Datum != (geom.Datum{}) {
			enc.X, enc.Y = axes.x.datum(d.Datum.X0), axes.y.datum(d.Datum.Y0)
			enc.X2, enc.Y2 = axes.x.datum(d.Datum.X1), axes.y.datum(d.Datum.Y1)
		}
	}
	if *enc == (Encoding{}) {
		return nil, nil
	}
	return enc, nil
}

// axisKinds remembers what each axis is, so that a value annotating a time
// axis is written as the timestamp it is rather than as a count of
// nanoseconds nobody can read.
type axisKinds struct{ x, y, y2, x2 axisKind }

// vertical and horizontal are the kinds of the axes a layer's values are read
// against, which are the secondary ones where the layer asked for them. They
// matter for an annotation rather than for a mark: a datum is written as a
// timestamp on a temporal axis and as a number everywhere else, so a threshold
// on a second axis of a different kind would otherwise be written in the first
// axis's spelling.
func (a axisKinds) vertical(onY2 bool) axisKind {
	if onY2 && a.y2 != "" {
		return a.y2
	}
	return a.y
}

func (a axisKinds) horizontal(onX2 bool) axisKind {
	if onX2 && a.x2 != "" {
		return a.x2
	}
	return a.x
}

type axisKind scale.Kind

func (k axisKind) datum(v float64) *Channel {
	if scale.Kind(k) == scale.KindTime {
		return &Channel{Datum: scale.FromNanos(v).UTC().Format(timeLayout)}
	}
	return &Channel{Datum: v}
}

func kindOf(s scale.Scale) axisKind {
	d, ok := scale.Describe(s)
	if !ok {
		return axisKind(scale.KindLinear)
	}
	return axisKind(d.Kind)
}

func boolPtr(b bool) *bool          { return &b }
func float64Ptr(v float64) *float64 { return &v }

// encodeTrack writes one band down.
//
// A track's layers are encoded against the axis kinds it is actually drawn
// with — one the chart's, one the track's — because a mark's channel type is
// read off the scale it is placed by, and a track's own scale is not the
// chart's.
func encodeTrack(t Track, x, y axisKind) (TrackDoc, error) {
	d := TrackDoc{
		Edge:     t.Edge,
		Size:     t.Size,
		Fraction: t.Fraction,
		NoAxis:   !t.Axis,
		Grid:     t.Grid,
	}
	var err error
	if d.Scale, err = axisChannel(t.Scale, ""); err != nil {
		return TrackDoc{}, fmt.Errorf("scale: %w", err)
	}
	// A band beside the panel owns its horizontal scale and shares the
	// chart's vertical one; a band under it is the other way round. The kinds
	// a mark's channels are written with follow that, because a channel's
	// type is read off the scale that places it.
	axes := axisKinds{x: x, y: kindOf(t.Scale)}
	if t.Edge == "left" || t.Edge == "right" {
		axes = axisKinds{x: kindOf(t.Scale), y: y}
	}
	for i, g := range t.Layers {
		l, err := encodeLayer(g, false, axes)
		if err != nil {
			return TrackDoc{}, fmt.Errorf("layer %d: %w", i, err)
		}
		d.Layer = append(d.Layer, l)
	}
	return d, nil
}

// rankedBy is the order a set chart writes down.
//
// It is written whenever the layer was told one, including "appearance" — which
// the grouped marks leave out, because for them it is the default. For a mark
// that ranks its own columns it is a choice, and a document that left it out
// would read back as the ranking rather than as the table's order.
func rankedBy(d geom.Desc) string {
	if !d.OrderSet {
		return ""
	}
	return orderName(d.Order)
}

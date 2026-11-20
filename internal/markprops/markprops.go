// Package markprops is the table of which options each built-in mark reads as
// a property of the mark — the options that are not a column and that the
// JSON spec writes on the mark object.
//
// It has two readers and is the only place the answer is written down. The
// spec package's tests derive the same answer from what its encoder actually
// writes for each mark and fail when the two differ; geom's tests render it
// into the option matrix in geom's package documentation and fail when that
// is stale. It is internal because it is a fact about this module's marks,
// not a promise to anybody outside it.
package markprops

// Reads maps a built-in mark's name to the options it reads as properties, by
// the Go name of the option function, in alphabetical order.
var Reads = map[string][]string{
	"line":          {"Budget", "Closed", "Curve", "Dash", "Decimate", "Dodge", "OnMissing", "Order", "Tension", "Width"},
	"scatter":       {"Budget", "Dash", "Decimate", "DensityCells", "Dodge", "Fill", "OnMissing", "Opacity", "Order", "Shape", "Size", "Width"},
	"bar":           {"BarWidth", "Baseline", "Budget", "Corner", "Dash", "Decimate", "Dodge", "Explode", "Extrude", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "OnMissing", "Order", "Orient", "Width"},
	"area":          {"Baseline", "Budget", "Closed", "Corner", "Curve", "Dash", "Decimate", "Dodge", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "OnMissing", "Opacity", "Order", "Tension", "Width"},
	"step":          {"Budget", "Closed", "Dash", "Decimate", "Dodge", "OnMissing", "Order", "Steps", "Width"},
	"boxplot":       {"BarWidth", "Budget", "Corner", "Dash", "Decimate", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "OnMissing", "Opacity", "Orient", "Outliers", "Whisker", "Width"},
	"rect":          {"BarWidth", "Corner", "Dash", "Explode", "Extrude", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Width"},
	"text":          {"Align", "AvoidOverlap", "Callout", "Elide", "Fill", "FontSize", "MinFontSize", "Rotate", "Slide", "Wrap"},
	"contour":       {"Dash", "LabelLevels", "LevelCount", "Levels", "Width"},
	"horizon":       {"BandHeight", "Bands", "Baseline", "Budget", "Decimate", "OnMissing"},
	"raster":        {"Resample"},
	"voronoi":       {"Corner", "Dash", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Width"},
	"histogram":     {"Baseline", "BinRange", "Bins", "Corner", "Dash", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Orient", "Width"},
	"violin":        {"Bandwidth", "BarWidth", "Corner", "Dash", "Dodge", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Width"},
	"ridgeline":     {"Bandwidth", "Corner", "Dash", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Overlap", "Width"},
	"hexbin":        {"DensityCells", "Fill"},
	"beeswarm":      {"BarWidth", "Dash", "Dodge", "Order", "Shape", "Size", "Width"},
	"ecdf":          {"Dash", "Dodge", "Order", "Width"},
	"qq":            {"Dash", "Dodge", "Fill", "Order", "Shape", "Size", "Width"},
	"trend":         {"Curve", "Dash", "Dodge", "Order", "Smooth", "Span", "Tension", "Width"},
	"treemap":       {"Corner", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Padding"},
	"icicle":        {"Corner", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Padding"},
	"sankey":        {"Corner", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Padding", "Thickness"},
	"arc-diagram":   {"Baseline", "Corner", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Padding", "Thickness"},
	"tree":          {"Baseline", "Branches", "Dash", "Opacity", "Orient", "Width"},
	"graph":         {"Baseline", "Branches", "Corner", "Dash", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Size", "Width"},
	"node-link":     {"Corner", "Dash", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Size", "Width"},
	"intersections": {"Corner", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Order", "Top"},
	"set-matrix":    {"Corner", "Dash", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Order", "Shape", "Size", "Top", "Width"},
	"set-sizes":     {"Corner", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity"},
	"venn":          {"Corner", "Dash", "Fill", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Opacity", "Width"},
	"parallel":      {"Dash", "Opacity", "Width"},
	"parallel-sets": {"Corner", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Padding", "Thickness"},
	"survival":      {"CensorMarks", "Confidence", "Dash", "Dodge", "Fill", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Inset", "Opacity", "Order", "Width"},
	"dependency":    {"BarWidth", "Dash", "Size", "Width"},
	"errorbar":      {"BarWidth", "Budget", "Caps", "Dash", "Decimate", "Dodge", "OnMissing", "Order", "Shape", "Size", "Width"},
	"candle":        {"BarWidth", "Budget", "CandleStyle", "Corner", "Dash", "Decimate", "Direction", "Falling", "Gradient", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Hollow", "Inset", "OnMissing", "Rising", "Width"},
	"last-value":    {"Color", "Dash", "Rule", "Width"},
	"locus":         {"Dash", "Extend", "LabelLevels", "Levels", "Width"},
	"hline":         {"Dash", "Extend", "Width"},
	"vline":         {"Dash", "Extend", "Width"},
	"hband":         {"Corner", "Dash", "Extend", "Fill", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Opacity", "Width"},
	"vband":         {"Corner", "Dash", "Extend", "Fill", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Opacity", "Width"},
	"segment":       {"Dash", "Extend", "Width"},
	"region":        {"Corner", "Dash", "Extend", "Fill", "Hatch", "HatchColor", "HatchDensity", "HatchWidth", "Opacity", "Width"},
	"note":          {"Align", "Extend", "FontSize", "Rotate"},
}

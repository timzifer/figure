// Command dendrogram renders the two charts a tidy tree unlocks.
//
// Like the other examples it is executed by a test, so it cannot silently stop
// compiling or stop producing a chart. The first chart is the clustered
// heatmap — the most-published figure shape in bioinformatics — and it is a
// geom.Rect over two ordinal axes with a geom.Tree in a track above it: a
// rectangle mark, a colour ramp, an ordinal axis and a band at a panel's edge,
// which figure has had since v0.10, and the tree that the band was missing.
// The second is a radial tree, which is the same mark under a polar coord.
// See docs/adr/0053-tidy-tree-layout.md.
//
// The clustering is done here, in a few lines of average linkage, because
// computing a clustering is an analysis rather than a reduction for drawing:
// figure reads the (node, parent, height) table however it was produced.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/coord"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/theme"
)

func main() {
	heat := flag.String("heatmap", "heatmap.svg", "output path for the clustered heatmap")
	radial := flag.String("radial", "radial.svg", "output path for the radial tree")
	flag.Parse()
	if err := run(*heat, *radial); err != nil {
		fmt.Fprintln(os.Stderr, "dendrogram:", err)
		os.Exit(1)
	}
}

func run(heat, radial string) error {
	if err := heatmap(heat); err != nil {
		return err
	}
	return radialTree(radial)
}

// The expression matrix: each sample is one of three conditions plus a little
// noise, so the clustering has a structure to find and the rows of the table
// are deliberately not in that structure's order.
var (
	genes     = []string{"ACT1", "GAL4", "HSP70", "CDC28", "RPL3", "PHO5", "SUC2", "ADH1"}
	samples   = []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9"}
	condition = []int{0, 2, 1, 0, 2, 1, 0, 1, 2}
	profiles  = [3][]float64{
		{2.1, -1.0, 0.3, 1.5, 0.2, -0.8, -1.2, 1.9},
		{-0.4, 1.8, 2.2, -0.9, 0.1, 1.1, 0.6, -1.5},
		{0.2, 0.4, -1.6, 0.3, 2.0, -0.2, 1.7, 0.1},
	}
)

func expression(s, g int) float64 {
	return profiles[condition[s]][g] + 0.35*math.Sin(float64(7*s+3*g))
}

func heatmap(out string) error {
	node, under, height, leaves := cluster()

	var sample, gene []string
	var value []float64
	for s := range samples {
		for g := range genes {
			sample, gene = append(sample, samples[s]), append(gene, genes[g])
			value = append(value, expression(s, g))
		}
	}

	p := figure.New(
		figure.Size(640, 560),
		figure.Title("Expression, clustered by sample"),
		figure.Theme(theme.Light),
	)
	// The sample axis is pinned to the tree's leaf order, so the heatmap's
	// columns stand under the leaves that name them. An axis left to discover
	// its categories would learn them from whichever layer trained first.
	p.X(scale.Ordinal(scale.Categories(leaves...), scale.OrdinalPadding(0)))
	p.Y(scale.Ordinal(scale.Categories(genes...), scale.OrdinalPadding(0)))
	p.Add(geom.Rect(figure.NewTable().
		String("sample", sample).String("gene", gene).Float64("expr", value),
		geom.X("sample"), geom.Y("gene"),
		geom.ColorBy("expr", scale.Diverging(palette.BlueOrange)),
	))
	p.Track(figure.Top, figure.TrackSize(110), figure.TrackScale(scale.Linear())).
		Add(geom.Tree(figure.NewTable().
			String("node", node).String("under", under).Float64("height", height),
			geom.ID("node"), geom.Parent("under"), geom.Value("height"),
			geom.Color(palette.Gray)))
	return p.Render(figure.SVG(out))
}

// cluster is average-linkage agglomerative clustering of the samples by
// Euclidean distance: the (node, parent, height) table a dendrogram reads,
// and the leaves in the order the tree lays them out.
func cluster() (node, under []string, height []float64, leaves []string) {
	type group struct {
		name    string
		members []int
	}
	dist := func(a, b int) float64 {
		d := 0.0
		for g := range genes {
			e := expression(a, g) - expression(b, g)
			d += e * e
		}
		return math.Sqrt(d)
	}

	var groups []group
	for s, name := range samples {
		groups = append(groups, group{name, []int{s}})
		node, height = append(node, name), append(height, 0)
	}
	parentOf := map[string]string{}
	for k := 1; len(groups) > 1; k++ {
		bi, bj, best := 0, 1, math.Inf(1)
		for i := range groups {
			for j := i + 1; j < len(groups); j++ {
				sum := 0.0
				for _, a := range groups[i].members {
					for _, b := range groups[j].members {
						sum += dist(a, b)
					}
				}
				if d := sum / float64(len(groups[i].members)*len(groups[j].members)); d < best {
					bi, bj, best = i, j, d
				}
			}
		}
		merged := group{fmt.Sprintf("m%d", k), append(append([]int(nil), groups[bi].members...), groups[bj].members...)}
		parentOf[groups[bi].name], parentOf[groups[bj].name] = merged.name, merged.name
		node, height = append(node, merged.name), append(height, best)
		groups = append(append(groups[:bi:bi], groups[bi+1:bj]...), groups[bj+1:]...)
		groups = append(groups, merged)
	}

	index := map[string]int{}
	for i, name := range node {
		index[name] = i
	}
	parent := make([]int, len(node))
	for i, name := range node {
		under = append(under, parentOf[name])
		parent[i] = stat.NoParent
		if p, ok := parentOf[name]; ok {
			parent[i] = index[p]
		}
	}
	var lay stat.Tidy
	lay.ResetLeaves(parent, stat.Depth(parent))
	for _, leaf := range lay.Leaves {
		leaves = append(leaves, node[leaf])
	}
	return node, under, height, leaves
}

// radialTree is a module's package tree, drawn round a circle with the root
// at the hub: the tidy tree on its depth, under a polar coord.
func radialTree(out string) error {
	node := []string{"figure", "geom", "scale", "stat", "render", "backend",
		"line", "bar", "tree", "linear", "log", "probability",
		"tidy", "kde", "bin", "svg", "pdf", "gg", "layout", "coord"}
	under := []string{"", "figure", "figure", "figure", "figure", "figure",
		"geom", "geom", "geom", "scale", "scale", "scale",
		"stat", "stat", "stat", "backend", "backend", "backend", "render", "render"}

	p := figure.New(
		figure.Size(520, 520),
		figure.Title("A module, from the root out"),
		figure.Theme(theme.Light.With(
			theme.Grid(false, false), theme.AxisLines(false, false), theme.Ticks(false, false))),
		figure.Coord(coord.Polar(coord.Hole(0.08))),
		figure.Legend(false),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(geom.Tree(figure.NewTable().String("node", node).String("under", under),
		geom.ID("node"), geom.Parent("under"), geom.Color(palette.Blue), geom.Width(1.5)))
	return p.Render(figure.SVG(out))
}

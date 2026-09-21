package stat

import "slices"

// Crosstab counts a table of categorical columns by the pairs of categories
// that neighbouring columns hold.
//
// It is the arithmetic under a parallel-sets diagram, and it is the only thing
// that chart needs which figure did not already have: the ribbons are these
// counts and the layout under them is [Sankey]'s. See
// docs/adr/0079-parallel-sets.md.
//
// It is a struct with a [Crosstab.Reset] rather than a function for the reason
// [Intersections] is one: the working state is the size of the data — a slot
// per distinct crossing — and a chart redrawn every frame should reuse it.
//
// # What a crossing is
//
// One crossing is a pair of categories in neighbouring columns, and the rows
// that hold both of them. A row is counted once per neighbouring pair, so a
// table of four columns contributes to three crossings, and every column's
// counts add up to the same total — which is the reading the chart rests on:
// each axis is the whole table, cut a different way.
//
// That total is why a row missing a category anywhere is counted nowhere. It
// is the one place in this library where an absent value costs a row rather
// than gapping what it is part of, and the reason is arithmetic: a line can
// break and resume because a line is one row, but a count that skipped only
// the crossings beside the gap would leave one column adding up to less than
// the next, and then no two thicknesses in the diagram would mean quite the
// same thing.
//
// A class subdivides it. Two rows that agree on both categories and disagree
// on the class are two crossings rather than one, because the colour that
// tells them apart has to have a ribbon of its own to paint. That is what
// makes a coloured parallel-sets diagram have more ribbons than an uncoloured
// one, rather than the same ribbons in a blend of colours nobody can read.
//
// # The order is not the table's, and here that is right
//
// Everything else in this package hands its answer back in the order the
// caller's rows gave it, because the order of a chart's own nodes has to come
// from the table rather than from a map (docs/adr/0012-parallel-panels.md).
// A crossing is not a row, though: it is a count over rows, and the table has
// no opinion about where it sits. So crossings come out ordered by the
// category they leave, then by the one they enter, then by class — the order
// that stacks the ribbons against each node in the order of the nodes at the
// other end of them, which is what keeps them from crossing each other for no
// reason. The keys are distinct, so nothing here depends on how a tie was
// broken, and interning the names is still the geom's business and still
// decides which category is which index.
//
// The zero Crosstab is unusable; call [Crosstab.Reset] first.
type Crosstab struct {
	// From and To are the two categories one crossing joins, as the indices
	// the caller gave them, and Value is what it carries: how many rows hold
	// both, or their total weight.
	From, To []int
	Value    []float64
	// Class is which class the crossing belongs to, or -1 where the caller
	// named none and for a row whose class is missing.
	Class []int

	at    map[crossing]int
	order []int
	from  []int
	to    []int
	class []int
	value []float64
}

// crossing is one distinct pair of categories and the class that subdivides
// them, which is the key the count is accumulated under.
type crossing struct {
	from, to, class int
}

// Reset counts the rows of cats, whose outer index is a column of the caller's
// table and whose inner index is a row, holding the index of the category that
// row has in that column, or a negative number where it has none.
//
// class is one class index per row, or nil for a table with no class column;
// weight is what each row contributes, or nil for one apiece. Both are read
// alongside cats, so a table of ten rows and a weight column of nine counts
// nine.
//
// A row missing a category in any column is counted nowhere, for the reason
// above: what it would be missing from is a partition. A row weighing zero or
// less is skipped for a plainer one — a ribbon of no thickness is not
// something a reader can be shown.
func (c *Crosstab) Reset(cats [][]int, class []int, weight []float64) {
	c.From, c.To, c.Class, c.Value = c.From[:0], c.To[:0], c.Class[:0], c.Value[:0]
	if len(cats) < 2 {
		return
	}
	n := len(cats[0])
	for _, col := range cats {
		n = min(n, len(col))
	}
	if class != nil {
		n = min(n, len(class))
	}
	if weight != nil {
		n = min(n, len(weight))
	}
	if c.at == nil {
		c.at = make(map[crossing]int, 16)
	}
	clear(c.at)

	for i := range n {
		if !complete(cats, i) {
			continue
		}
		w := 1.0
		if weight != nil {
			w = weight[i]
		}
		if !(w > 0) {
			continue
		}
		for d := 0; d+1 < len(cats); d++ {
			k := crossing{from: cats[d][i], to: cats[d+1][i], class: classAt(class, i)}
			j, seen := c.at[k]
			if !seen {
				j = len(c.From)
				c.From = append(c.From, k.from)
				c.To = append(c.To, k.to)
				c.Class = append(c.Class, k.class)
				c.Value = append(c.Value, 0)
				c.at[k] = j
			}
			c.Value[j] += w
		}
	}
	c.sort()
}

// complete reports whether row i has a category in every column. A row that
// does not is in no crossing at all — see [Crosstab].
func complete(cats [][]int, i int) bool {
	for _, col := range cats {
		if col[i] < 0 {
			return false
		}
	}
	return true
}

// classAt is the class of row i, and -1 for a table with no class column or a
// row whose class is missing. A row with no class is a crossing of its own
// rather than one of somebody else's: it is painted the colour a scale gives a
// value it has no category for, and a reader can see there is something there.
func classAt(class []int, i int) int {
	if class == nil || class[i] < 0 {
		return -1
	}
	return class[i]
}

// sort puts the crossings in (from, to, class) order.
//
// The keys are distinct — that is what the map is for — so the comparison is a
// total order and the result does not depend on the sort being stable.
func (c *Crosstab) sort() {
	c.order = grow(c.order, len(c.From))
	for i := range c.order {
		c.order[i] = i
	}
	slices.SortFunc(c.order, func(x, y int) int {
		if c.From[x] != c.From[y] {
			return c.From[x] - c.From[y]
		}
		if c.To[x] != c.To[y] {
			return c.To[x] - c.To[y]
		}
		return c.Class[x] - c.Class[y]
	})

	c.from = grow(c.from, len(c.order))
	c.to = grow(c.to, len(c.order))
	c.class = grow(c.class, len(c.order))
	c.value = growFloats(c.value, len(c.order))
	for i, j := range c.order {
		c.from[i], c.to[i], c.class[i], c.value[i] = c.From[j], c.To[j], c.Class[j], c.Value[j]
	}
	c.From, c.from = c.from, c.From
	c.To, c.to = c.to, c.To
	c.Class, c.class = c.class, c.Class
	c.Value, c.value = c.value, c.Value
}

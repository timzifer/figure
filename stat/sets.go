package stat

// MaxSets is how many sets an [Intersections] counts over.
//
// A combination of sets is a bit per set and the mask is a uint64, which is
// where the number comes from — but it is not a limit anybody meets. An UpSet
// plot of sixty-four sets has up to 2^64 columns and a reader has one screen;
// past a dozen or so the chart is a wall of dots whatever the arithmetic says.
const MaxSets = 64

// Combination is one distinct combination of sets, and how many elements are in
// exactly that combination and no other.
//
// Exactly, which is the whole reading of an UpSet plot and the opposite of a
// Venn diagram's usual mislabelling: an element in A and in B is counted once,
// under {A, B}, and is not counted again under {A}. So the counts partition the
// elements and add up to how many there are.
type Combination struct {
	// Sets is a bit per set: bit i is set when the set with index i is in this
	// combination.
	Sets uint64
	// Count is how many elements are in exactly these sets.
	Count int
	// Degree is how many sets that is, which is Sets's population count and is
	// what a reader calls the column's degree.
	Degree int
}

// Has reports whether set i is one of the combination's.
func (c Combination) Has(i int) bool {
	return i >= 0 && i < MaxSets && c.Sets&(1<<uint(i)) != 0
}

// Intersections counts the elements of a membership list by which sets they are
// in.
//
// It is the arithmetic under an UpSet plot and under a Venn diagram, and it is
// the only thing either of them needs that figure did not already draw: the
// bars are these counts, the dot matrix is these combinations, and a Venn's
// regions are the same counts placed in a fixed picture. See
// docs/adr/0074-sets-are-counted.md.
//
// It is a struct with a [Intersections.Reset] rather than a pair of functions
// for the reason [Sankey] is one: the working state is the size of the data —
// a mask per element, a slot per combination — and a chart redrawn every frame
// should reuse it.
//
// # The order is the caller's
//
// Combinations come out in the order their first element appears in the
// membership list, which is the order the caller's rows have, because the map
// that finds a combination again is only ever asked whether it has seen one and
// never what it holds (docs/adr/0012-parallel-panels.md). A mark that wants the
// biggest column first sorts this answer; the answer itself does not depend on
// how a tie was broken.
//
// The zero Intersections is unusable; call [Intersections.Reset] first.
type Intersections struct {
	// Combinations has one entry per distinct combination of sets, in order of
	// first appearance.
	Combinations []Combination
	// Sizes has one entry per set: how many elements are in it, counting an
	// element once per set it is in. These are the totals a set-size bar draws,
	// and they do not add up to the number of elements.
	Sizes []int
	// Elements is how many elements were counted, and Members how many of the
	// membership rows were used — a row naming a set or an element outside the
	// counts is skipped rather than guessed at.
	Elements int
	Members  int
	// TooMany reports a membership list naming more than [MaxSets] sets, in
	// which case nothing is counted.
	TooMany bool

	masks []uint64
	seen  []bool
	at    map[uint64]int
}

// Reset counts the membership list elem[i] ∈ set[i], over the given number of
// elements and sets.
//
// Both are indices rather than names, which is this package's rule: interning a
// string is where the order of everything downstream is decided, and that
// belongs to the geom that read the caller's table.
//
// A row naming an element outside [0, elements) or a set outside [0, sets) is
// skipped. An element named by no row is in no combination and is not counted:
// the membership list is the whole input, and an element nobody mentioned is
// not a fact about the data.
func (x *Intersections) Reset(elem, set []int, elements, sets int) {
	x.Combinations = x.Combinations[:0]
	x.Sizes = x.Sizes[:0]
	x.Elements, x.Members, x.TooMany = 0, 0, false
	if x.at == nil {
		x.at = make(map[uint64]int, 16)
	}
	clear(x.at)
	if elements <= 0 || sets <= 0 {
		return
	}
	if sets > MaxSets {
		x.TooMany = true
		return
	}

	x.masks = growUints(x.masks, elements)
	x.seen = growBools(x.seen, elements)
	for i := range elements {
		x.masks[i], x.seen[i] = 0, false
	}
	x.Sizes = growInts(x.Sizes, sets)
	for i := range sets {
		x.Sizes[i] = 0
	}

	n := min(len(elem), len(set))
	for i := range n {
		e, s := elem[i], set[i]
		if e < 0 || e >= elements || s < 0 || s >= sets {
			continue
		}
		x.Members++
		if x.masks[e]&(1<<uint(s)) != 0 {
			// A membership named twice is one membership: a set holds an
			// element or it does not, and counting the row again would make a
			// duplicated row look like a bigger set.
			continue
		}
		x.masks[e] |= 1 << uint(s)
		x.Sizes[s]++
		x.seen[e] = true
	}

	// Elements in index order, which is the order the geom interned them in,
	// which is the order they appear in the caller's table.
	for e := range elements {
		if !x.seen[e] {
			continue
		}
		x.Elements++
		mask := x.masks[e]
		if j, ok := x.at[mask]; ok {
			x.Combinations[j].Count++
			continue
		}
		x.at[mask] = len(x.Combinations)
		x.Combinations = append(x.Combinations, Combination{
			Sets: mask, Count: 1, Degree: popcount(mask),
		})
	}
}

// CountOf is how many elements are in exactly the given combination of sets,
// which is what a Venn diagram writes in one region.
//
// Zero for a combination nothing is in, so a region with nobody in it says so
// rather than being left out.
func (x *Intersections) CountOf(mask uint64) int {
	if j, ok := x.at[mask]; ok {
		return x.Combinations[j].Count
	}
	return 0
}

// popcount is how many bits a mask has set. It is a loop rather than
// math/bits.OnesCount64 for nothing but the reason the rest of this package is
// arithmetic: a degree is at most MaxSets and this is not a hot path.
func popcount(mask uint64) int {
	n := 0
	for mask != 0 {
		mask &= mask - 1
		n++
	}
	return n
}

func growUints(dst []uint64, n int) []uint64 {
	if cap(dst) < n {
		return make([]uint64, n)
	}
	return dst[:n]
}

func growBools(dst []bool, n int) []bool {
	if cap(dst) < n {
		return make([]bool, n)
	}
	return dst[:n]
}

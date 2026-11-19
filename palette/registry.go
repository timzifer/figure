package palette

import (
	"slices"
	"sort"
	"sync"

	"github.com/timzifer/figure/ir"
)

// registryMu guards ramps, qualitatives and matrices. A spec is encoded and
// decoded on whatever goroutine asks, and every encode reads these maps, so a
// registration beside one is a concurrent map write without it — the lock
// every other registry in the module already takes.
//
// Entries are copied on the way in and on the way out. A registry that handed
// back its own slice would let a caller who edited a lookup's result repaint
// Viridis for every chart in the process.
var registryMu sync.RWMutex

// Ramps are registered by name so that a chart can be written down and read
// back: a spec carries "viridis", not ten hex triples, and a reader who edits
// the file by hand has a word to type rather than a gradient to reproduce.
//
// The name is the scheme name Vega and matplotlib use for the same ramp, so
// that a figure spec and a Vega-Lite one say the same thing where they can.

var ramps = map[string]Ramp{
	"viridis":     Viridis,
	"cividis":     Cividis,
	"magma":       Magma,
	"blues":       Blues,
	"greys":       Greys,
	"blueorange":  BlueOrange,
	"purplegreen": PurpleGreen,
}

// RegisterRamp adds a ramp under a name, replacing any ramp already there. It
// is how a third-party ramp becomes reachable from a spec or a config file.
func RegisterRamp(name string, r Ramp) {
	if name == "" || len(r) == 0 {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	ramps[name] = slices.Clone(r)
}

// RampByName looks up a registered ramp. The result is a copy: editing it
// changes nothing that is registered.
func RampByName(name string) (Ramp, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	r, ok := ramps[name]
	return slices.Clone(r), ok
}

// Qualitative palettes are registered the same way and for the same reason: a
// discrete colour scale names "okabeito" in a spec rather than inlining eight
// hex triples, and a reader editing the document has a word to type.
var qualitatives = map[string]Qualitative{
	"okabeito": OkabeIto,
}

// RegisterQualitative adds a qualitative palette under a name, replacing any
// palette already there. It is how a third-party palette becomes reachable
// from a spec or a config file.
func RegisterQualitative(name string, q Qualitative) {
	if name == "" || len(q) == 0 {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	qualitatives[name] = slices.Clone(q)
}

// QualitativeByName looks up a registered qualitative palette. The result is a
// copy: editing it changes nothing that is registered.
func QualitativeByName(name string) (Qualitative, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	q, ok := qualitatives[name]
	return slices.Clone(q), ok
}

// QualitativeName reports the name a qualitative palette is registered under.
// Like [RampName] it compares colours rather than identity, so a copy is still
// recognised. ok is false for a palette nobody registered.
func QualitativeName(q Qualitative) (string, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	for _, name := range sortedNames(qualitatives) {
		if sameColors(qualitatives[name], q) {
			return name, true
		}
	}
	return "", false
}

// QualitativeNames lists the registered qualitative palettes, sorted.
func QualitativeNames() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return sortedNames(qualitatives)
}

// RampName reports the name a ramp is registered under.
//
// It compares colours rather than identity, so a ramp that was copied — which
// is what [Ramp.Reverse] and any slice expression produce — is still
// recognised. ok is false for a ramp nobody registered.
func RampName(r Ramp) (string, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	for _, name := range sortedNames(ramps) {
		if sameRamp(ramps[name], r) {
			return name, true
		}
	}
	return "", false
}

// RampNames lists the registered ramps, sorted, so that a caller iterating
// them gets the same order every time.
func RampNames() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return sortedNames(ramps)
}

// sortedNames is a registry's keys in order. The caller holds registryMu: the
// lookups that need the order already hold it, and a read lock taken twice on
// one goroutine deadlocks as soon as a writer is waiting between the two.
func sortedNames[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for name := range m {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func sameRamp(a, b Ramp) bool { return sameColors(a, b) }

func sameColors[A, B ~[]ir.Color](a A, b B) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

package gg

import (
	"testing"

	"github.com/gogpu/gg/text"
	"golang.org/x/image/font/gofont/goregular"
)

// The gap this backend has and the vector ones do not: the embedded Go fonts
// have no glyph for the mathematical angle brackets a Bloch sphere's |0⟩ is
// written with. It is pinned rather than merely documented, so that a font
// change that closed it would be noticed rather than leaving the fallback
// machinery guarding nothing.
func TestTheEmbeddedFontHasNoMathematicalAngleBrackets(t *testing.T) {
	fonts, err := defaultFonts()
	if err != nil {
		t.Fatal(err)
	}
	face := fonts.face(12, 400, false)
	for _, r := range []rune{'⟨', '⟩'} {
		if face.HasGlyph(r) {
			t.Errorf("the embedded font now has %q; the fallback note in doc.go is out of date", r)
		}
	}
	// And it does have the letters and the operators the note claims.
	for _, r := range []rune{'A', 'µ', 'Ω', '±', '°', '−'} {
		if !face.HasGlyph(r) {
			t.Errorf("the embedded font has no %q, which doc.go says it covers", r)
		}
	}
}

// A rune no face can draw is written as `?` rather than dropped, which is what
// backend/pdf writes for a rune outside its encoding: a label that quietly
// loses a character says something the data does not.
func TestARuneNoFaceCanDrawIsWrittenAsAQuestionMark(t *testing.T) {
	fonts, err := defaultFonts()
	if err != nil {
		t.Fatal(err)
	}
	face := fonts.face(12, 400, false)
	if got := drawable(face, "|0⟩"); got != "|0?" {
		t.Errorf("drawable(%q) = %q, want %q", "|0⟩", got, "|0?")
	}
	// A string the face can draw comes back as itself, which is every label of
	// every chart in the documentation.
	const plain = "p99 latency (ms)"
	if got := drawable(face, plain); got != plain {
		t.Errorf("drawable(%q) = %q, want it unchanged", plain, got)
	}
	// And the substitution is measurable, which is the point: the label is now
	// as wide as what is drawn rather than as wide as what survived.
	if face.Advance("|0?") <= face.Advance("|0") {
		t.Error("the substituted label is no wider than the one that lost a character")
	}
}

// A fallback is consulted for a rune the chart's own font has no glyph for,
// and the metrics stay the chart's own font's — so a fallback glyph in one
// label does not move the baseline of the row it is in.
func TestAFallbackFontSitsBehindTheChartsOwn(t *testing.T) {
	src, err := text.NewFontSource(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	base := newFontSet(src, nil, nil)
	with := base.withFallback([]*text.FontSource{src})
	if with == base {
		t.Fatal("withFallback returned the set it was given; the embedded set is shared by every backend in the process")
	}
	if len(base.fallback) != 0 {
		t.Error("withFallback changed the set it was given")
	}
	face := with.face(12, 400, false)
	if _, ok := face.(*text.MultiFace); !ok {
		t.Fatalf("a set with a fallback hands out a %T, want a MultiFace", face)
	}
	if got, want := face.Metrics(), base.face(12, 400, false).Metrics(); got != want {
		t.Errorf("metrics %+v with a fallback, want the chart's own font's %+v", got, want)
	}
	// With no fallback the set is handed back as it is, so the ordinary path
	// pays nothing for the feature.
	if base.withFallback(nil) != base {
		t.Error("a set with no fallback was copied anyway")
	}
}

// The option composes with WithFont in either order, and a font that will not
// parse is reported rather than ignored.
func TestWithFallbackFontComposesAndReportsABadFont(t *testing.T) {
	o := build([]Option{WithFallbackFont(goregular.TTF), WithFont(goregular.TTF, nil, nil)})
	if o.fontErr != nil {
		t.Fatal(o.fontErr)
	}
	fonts, err := o.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts.fallback) != 1 {
		t.Errorf("%d fallbacks, want the one that was supplied", len(fonts.fallback))
	}
	if bad := build([]Option{WithFallbackFont([]byte("not a font"))}); bad.fontErr == nil {
		t.Error("a fallback that will not parse was accepted")
	}
	// An empty fallback is no fallback rather than an error, which is what
	// WithFont's optional styles already do.
	if empty := build([]Option{WithFallbackFont(nil)}); empty.fontErr != nil || len(empty.fallback) != 0 {
		t.Errorf("an absent fallback gave %v and %d sources", empty.fontErr, len(empty.fallback))
	}
}

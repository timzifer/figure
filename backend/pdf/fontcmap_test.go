package pdf_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/timzifer/figure/backend/pdf"
	"github.com/timzifer/figure/internal/sfnt"
	"github.com/timzifer/figure/ir"
)

// noUnicodeTTF is a font that parses but cannot map a rune to a glyph: a
// subset, which addresses glyphs by id and carries no character map.
func noUnicodeTTF(t *testing.T) []byte {
	t.Helper()
	f, err := sfnt.Parse(testTTF())
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := f.Subset([]uint16{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := sfnt.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if sub.HasUnicodeMap() {
		t.Fatal("the subset has a Unicode map; this helper no longer builds the font it says it does")
	}
	return data
}

// Every face is refused for the reason the regular one is: no label drawn in
// it could be mapped to a glyph.
func TestEveryFaceWithoutAUnicodeMapIsRefused(t *testing.T) {
	bad := noUnicodeTTF(t)
	for _, tc := range []struct {
		which string
		opt   pdf.Option
	}{
		{"regular", pdf.WithFont(bad, nil, nil)},
		{"bold", pdf.WithFont(testTTF(), bad, nil)},
		{"italic", pdf.WithFont(testTTF(), nil, bad)},
	} {
		t.Run(tc.which, func(t *testing.T) {
			var buf bytes.Buffer
			_, err := pdf.Writer(&buf, tc.opt).Open(ir.Surface{WidthPx: 100, HeightPx: 100, DPR: 1})
			if err == nil {
				t.Fatalf("a %s face without a Unicode map was accepted", tc.which)
			}
			if !strings.Contains(err.Error(), tc.which+" font has no Unicode character map") {
				t.Errorf("error %q does not name the %s face", err, tc.which)
			}
		})
	}
}

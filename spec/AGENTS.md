# Working in `spec`

Notes for changing `spec`, moved here from the root [AGENTS.md](../AGENTS.md),
which holds the rules that apply everywhere — read that one first. This file
covers what a document writes for a mark, complete descriptions, and marks the
encoding tells apart.

## Things that will bite

**The JSON spec writes what a mark uses, not what it was given.** A geom
accepts every option and ignores the ones it has no use for, which is what
keeps the option set one namespace; a *document* listing a line's whisker
extent would read as though that meant something. `spec.writeMarkProps` decides
per mark. Adding an option means adding it there too, and the round-trip test
in `spec/` is what catches forgetting.

**`geom.Desc` and `scale.Desc` are complete, not partial.** Every field with a
non-zero default — `BarWidth`, `Whisker`, `Outliers`, `Extend`, `Opacity` —
carries the value the layer is actually using, so nothing downstream has to
know what the defaults are. `Opacity` is `-1` when unset, matching `config`.

**A rect and a region are both `("rect", "")` in the document, and the encoding
is what tells them apart.** `spec.geomMark` is passed the layer's encoding for
that one reason: a rect with a *field* is data, a rect with a *datum* is an
annotation. Dropping the parameter compiles and turns every heatmap into a
four-literal annotation.

**A note and a text layer are both `"text"` in the document, and the encoding is
what tells them apart.** `spec.geomMark` asks `hasField`, exactly as it does for
a rect against a region: a text mark with a `text` *field* is data, one placed at
literal values is an annotation. Adding a channel to `spec.Encoding` means adding
it to `hasField` too, or a layer encoded only by that channel reads back as an
annotation.

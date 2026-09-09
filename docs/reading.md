# Charts that can be read

Typeset notation in labels, and a chart that says what it is to something other than an eye.

## Labels that are notation

```go
p := figure.New(
	figure.Math(mathtext.TeX()),
	figure.YTitle(`flux $F_\nu$ ($\mathrm{W\,m^{-2}\,Hz^{-1}}$)`),
	figure.Title(`decay of $N_0e^{-\lambda t}$`),
)
```

![Standard error curves, with a fraction over a radical as the y title](images/notation.png)

`$…$` is set as notation and everything around it as text. The subset is the one
a chart label actually needs — scripts, `\frac`, `\sqrt`, `\bar`, `\mathrm`,
the spacing commands, and a table of symbols — and a single letter is set italic
because it is a variable while a run of letters is a name. Operators and
relations get TeX's own spacing, so `$\sigma = 1$` reads as an equation rather
than as a filename.

A typesetter is installed by wrapping the backend, so it reaches every label the
chart has: the title, the axis titles, the ticks, the legend, a facet's strip, a
geom's own note. That also means a label is *measured* as it will be drawn, so
the margin left for a fraction is the height of the fraction rather than the
width of its markup ([ADR 0023](adr/0023-math-typesetting.md)). Notation it
cannot parse is drawn exactly as written — a chart never fails to render because
of a label.

`mathtext.Typesetter` is the seam if you have a real engine to plug in.

## Charts that can be read without being seen

Three channels, because they fail for three different readers
([ADR 0024](adr/0024-accessibility.md)):

```go
p := figure.New(
	figure.Title("Signal against model"),
	figure.Theme(theme.Light.With(theme.Redundant(true))),  // dashes and shapes, not colour alone
)
p.Add(/* ... */)

p.Describe()                      // read the data; write a description
p.Render(figure.SVG("chart.svg"))
p.DataTable(w)                    // the same data as an HTML table
```

The title alone costs nothing and is always written: an SVG gets `<title>`,
`role="img"` and `aria-labelledby`, a PDF gets a document title, a canvas gets
`role` and `aria-label`. `Describe` costs a pass over the data — it reports how
many rows there are and over what range — so it is a call rather than something
every render pays for, and it fills in the `<desc>` a screen reader announces
next:

```
Signal against model. 3 layers with line marks. Axes: sample horizontally,
σ/√n (mV) vertically. measured, a line of 24 rows, sample from 0 to 23,
measured from 6.16 to 20.9. …
```

Notation in a title is read aloud rather than spelled out, because "dollar
backslash frac" is not a description of anything.

`theme.Redundant(true)` gives each layer a dash pattern and a marker shape
alongside its palette colour — the chart survives a greyscale printout and the
readers who cannot separate its first two colours — and it leaves a layer that
named its own `geom.Dash` or `geom.Shape` alone.

See [`examples/accessible`](../examples/accessible), which writes the picture, the
page and the description as three files.

---

**[README](../README.md)** · **[CONCEPT](../CONCEPT.md)** · **[ADRs](adr)** · [The gallery](gallery.md) · [Chart forms](charts.md) · [Interaction](interaction.md) · [A million rows](scale-out.md) · [JSON and Arrow](spec.md) · [Features](features.md) · [Chart-type catalogue](chart-types.md) · [Benchmarks](benchmarks.md) · [How it was built](milestones.md)

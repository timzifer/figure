# A chart as JSON, and data from elsewhere

The declarative dialect a chart writes itself down as, and the Arrow adapter that feeds one.

## A chart as JSON

```go
doc, err := p.MarshalJSON()      // indented; json.Marshal(p) compacts it
q, err := figure.ParseJSON(doc) // and reads back as the same chart
```

The document is Vega-Lite-*shaped*: `data.values`, `mark.type`,
`encoding.x.field`, `scale.type`, `facet` and `resolve` mean what they mean in
Vega-Lite, so anyone who knows that vocabulary can read one. It is not a
Vega-Lite subset and does not claim to be — figure has marks and options
Vega-Lite has no name for, and naming them plainly beats smuggling them through
a borrowed name. What is guaranteed is the round trip through figure, and there
is a test per mark and per scale that renders both and compares the primitives
([ADR 0014](adr/0014-json-spec.md)).

```json
{
  "$schema": "https://github.com/timzifer/figure/spec/v1",
  "width": 640,
  "height": 400,
  "title": "Throughput",
  "data": {
    "values": [{"x": 0, "y": 2}],
    "format": {"parse": {"x": "number", "y": "number"}}
  },
  "encoding": {
    "x": {"type": "quantitative", "scale": {"type": "linear", "nice": true}},
    "y": {"type": "quantitative", "scale": {"type": "linear", "nice": true}}
  },
  "layer": [
    {
      "mark": {"type": "line", "color": "#0072b2"},
      "encoding": {"x": {"field": "x"}, "y": {"field": "y"}}
    }
  ],
  "config": {"theme": "light"}
}
```

(shown with the objects folded up; the real output puts every field on its own
line)

## What the dialect does not carry

**A projected scene does not round-trip.** `figure/three` holds a scene and the
cameras on it rather than layers and a coord, and the dialect is Vega-Lite
shaped: a Vega-Lite chart has an `x`, a `y` and a coordinate system, and a
camera, a depth encoding and a list of views have no reading in that
vocabulary. `$schema` names the dialect and moves only with the module's major
version ([ADR 0056](adr/0056-three-dimensional-charts.md)), so carrying one
would mean inventing a dialect rather than extending this one — a decision of
its own, with a caller behind it.

What did land is `geom.Z`, the depth channel, beside `geom.X` and `geom.Y`. It
costs the document nothing: no flat mark reads it, and writing a line's depth
column into a document would read as though it meant something.

## Plotting Arrow data

```go
import "github.com/timzifer/figure/arrow/v18"

src := arrow.Source(rec)      // rec is an arrow.Record
p.Add(geom.Line(src, geom.X("t"), geom.Y("p99")))
```

A `float64` column with no nulls is Arrow's own buffer — no copy, no conversion.
Everything else (integers, `float32`, timestamps, dictionary-encoded strings)
converts once on first use and is cached, so a record with forty columns and a
chart that plots two pays for two. An Arrow null becomes `NaN`, which means the
missing-data policy you already set covers it
([ADR 0013](adr/0013-arrow-adapter.md)).

---

**[README](../README.md)** · **[CONCEPT](../CONCEPT.md)** · **[ADRs](adr)** · [The gallery](gallery.md) · [Chart forms](charts.md) · [Interaction](interaction.md) · [A million rows](scale-out.md) · [Reading a chart](reading.md) · [Features](features.md) · [Chart-type catalogue](chart-types.md) · [Benchmarks](benchmarks.md) · [How it was built](milestones.md)

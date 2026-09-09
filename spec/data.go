package spec

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/scale"
)

// timeLayout is how a time is written. RFC 3339 with nanoseconds is what
// Vega-Lite's "date" parse accepts and what a person reading the file can
// recognise without a decoder ring.
const timeLayout = time.RFC3339Nano

// encodeData writes a source as inline values plus the parse map that says
// what each column is.
//
// A NaN or an infinity becomes JSON null. That is not a loss: the missing-data
// policy already treats a NaN as a hole, null is the only thing JSON has to
// say "no value", and encoding/json refuses a NaN outright.
func encodeData(src data.Source) (*Data, error) {
	if src == nil {
		return nil, nil
	}
	n := src.Len()
	names := append([]string(nil), src.Columns()...)
	sort.Strings(names)

	parse := make(map[string]string, len(names))
	nums := map[string][]float64{}
	times := map[string][]time.Time{}
	strs := map[string][]string{}
	ints := map[string][]int64{}
	for _, name := range names {
		c, ok := src.Column(name)
		if !ok {
			return nil, fmt.Errorf("figure/spec: column %q vanished between being listed and being read", name)
		}
		switch c.Kind {
		case data.KindFloat64:
			nums[name], parse[name] = c.Floats, ParseNumber
		case data.KindInt64:
			// An exact integer is written as JSON's own number, which carries
			// every digit of an int64 as text. It comes back exact, which is
			// the whole reason the kind exists.
			ints[name], parse[name] = c.Ints, ParseInteger
		case data.KindTime:
			times[name], parse[name] = c.Times, ParseDate
		case data.KindString:
			strs[name], parse[name] = c.Strings, ParseString
		default:
			return nil, fmt.Errorf("figure/spec: column %q is of no kind this package can write", name)
		}
	}

	rows := make([]map[string]any, n)
	for i := range rows {
		row := make(map[string]any, len(names))
		for name, v := range nums {
			if i < len(v) && isFinite(v[i]) {
				row[name] = v[i]
			} else {
				row[name] = nil
			}
		}
		for name, v := range ints {
			if i < len(v) {
				row[name] = v[i]
			} else {
				row[name] = nil
			}
		}
		for name, v := range times {
			if i < len(v) {
				row[name] = v[i].Format(timeLayout)
			} else {
				row[name] = nil
			}
		}
		for name, v := range strs {
			if i < len(v) {
				row[name] = v[i]
			} else {
				row[name] = nil
			}
		}
		rows[i] = row
	}
	return &Data{Values: rows, Format: &Format{Parse: parse}}, nil
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// decodeData builds a table from inline values.
//
// The parse map is authoritative: it decides which columns exist and what type
// each is. Without one the types are inferred from the first value that is not
// null, which is what a hand-written spec will usually rely on.
func decodeData(d *Data) (data.Source, error) {
	if d == nil {
		return nil, nil
	}
	parse := d.Format.parseMap()
	if len(parse) == 0 {
		parse = infer(d.Values)
	}
	names := make([]string, 0, len(parse))
	for name := range parse {
		names = append(names, name)
	}
	sort.Strings(names)

	t := data.NewTable()
	n := len(d.Values)
	for _, name := range names {
		switch parse[name] {
		case ParseNumber:
			col := make([]float64, n)
			for i, row := range d.Values {
				col[i] = asNumber(row[name])
			}
			t.Float64(name, col)
		case ParseInteger:
			col := make([]int64, n)
			for i, row := range d.Values {
				col[i] = asInteger(row[name])
			}
			t.Int64(name, col)
		case ParseDate:
			col := make([]time.Time, n)
			for i, row := range d.Values {
				tv, err := asTime(row[name])
				if err != nil {
					return nil, fmt.Errorf("figure/spec: column %q row %d: %w", name, i, err)
				}
				col[i] = tv
			}
			t.Time(name, col)
		case ParseString:
			col := make([]string, n)
			for i, row := range d.Values {
				if s, ok := row[name].(string); ok {
					col[i] = s
				}
			}
			t.String(name, col)
		default:
			return nil, fmt.Errorf("figure/spec: column %q has unknown type %q", name, parse[name])
		}
	}
	return t, nil
}

func (f *Format) parseMap() map[string]string {
	if f == nil {
		return nil
	}
	return f.Parse
}

// infer types a table that arrived without a parse map: a number is a number,
// a string that reads as an RFC 3339 timestamp is a date, and everything else
// is a category.
//
// The timestamp rule is a guess, and it is the one Vega-Lite makes too. A spec
// figure wrote always carries the parse map, so this only ever runs on a
// document a person typed.
func infer(rows []map[string]any) map[string]string {
	out := map[string]string{}
	for _, row := range rows {
		for name, v := range row {
			if v == nil {
				continue
			}
			if kind, seen := out[name]; seen {
				// One fractional value demotes a column the rows before it
				// looked like integers: a table whose first row happens to
				// hold whole numbers is not a table of integers, and
				// truncating the rest of it would be a silent wrong answer.
				if n, isNum := v.(json.Number); isNum && kind == ParseInteger && !integral(n) {
					out[name] = ParseNumber
				}
				continue
			}
			switch x := v.(type) {
			case json.Number:
				// A document with no parse map says what it says: a column
				// whose every value is written without a fraction or an
				// exponent is a column of integers, and reading it as one
				// keeps a hand-written table of ids exact without anybody
				// having had to declare it.
				out[name] = ParseNumber
				if integral(x) {
					out[name] = ParseInteger
				}
			case float64, int, int64:
				out[name] = ParseNumber
			case string:
				if _, err := time.Parse(timeLayout, x); err == nil {
					out[name] = ParseDate
				} else {
					out[name] = ParseString
				}
			}
		}
	}
	// A column that was null in every row still exists; call it numeric,
	// which makes it a column of NaN rather than a column that vanished.
	for _, row := range rows {
		for name := range row {
			if _, seen := out[name]; !seen {
				out[name] = ParseNumber
			}
		}
	}
	return out
}

// integral reports whether a JSON number was written as a whole number that
// fits an int64 — no fraction, no exponent, no overflow.
func integral(n json.Number) bool {
	if strings.ContainsAny(n.String(), ".eE") {
		return false
	}
	_, err := n.Int64()
	return err == nil
}

// asInteger reads a value written by an exact-integer column.
//
// json.Number is the shape a document decoded through [Unmarshal] hands over,
// and it is the reason that decoder asks for one: an int64 past 2^53 read as a
// float64 comes back a different number, which would make the round trip lose
// exactly what the kind exists to keep. A value of any other shape is read as
// a number and truncated, because a document written by hand is under no
// obligation to have used integers.
func asInteger(v any) int64 {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		f, _ := x.Float64()
		return int64(f)
	case int64:
		return x
	case int:
		return int64(x)
	}
	return int64(asNumber(v))
}

func asNumber(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return math.NaN()
		}
		return f
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		// A number that arrived quoted. Vega-Lite's parse map exists to fix
		// exactly this, so honouring it rather than dropping the row is the
		// point of having one.
		if t, err := time.Parse(timeLayout, x); err == nil {
			return scale.Nanos(t)
		}
	}
	return math.NaN()
}

func asTime(v any) (time.Time, error) {
	switch x := v.(type) {
	case nil:
		return time.Time{}, nil
	case string:
		return time.Parse(timeLayout, x)
	case float64:
		// Unix nanoseconds, which is the domain a time scale actually maps.
		return scale.FromNanos(x), nil
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return time.Time{}, fmt.Errorf("%v is not a timestamp", v)
		}
		return scale.FromNanos(f), nil
	}
	return time.Time{}, fmt.Errorf("%v is not a timestamp", v)
}

// sameSource reports whether two layers draw from the same table.
//
// It compares identity, not contents: two tables holding equal rows are still
// two tables, and a chart that was built over one source should read back as
// one source rather than as several that happen to agree. Sources are
// interfaces over pointers in every implementation here, and comparing
// interface values directly would panic on an implementation whose dynamic
// type is not comparable — so the comparison goes through the pointer.
func sameSource(a, b data.Source) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	if va.Kind() != reflect.Pointer || vb.Kind() != reflect.Pointer {
		return false
	}
	return va.Type() == vb.Type() && va.Pointer() == vb.Pointer()
}

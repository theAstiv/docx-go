package template

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLexFormatters(t *testing.T) {
	cases := []struct {
		in    string
		path  string
		calls []call
	}{
		{"{{x|upper}}", "x", []call{{name: "upper"}}},
		{"{{ total | number 2 }}", "total", []call{{name: "number", args: []any{2.0}}}},
		{`{{d | date "2 Jan 2006"}}`, "d", []call{{name: "date", args: []any{"2 Jan 2006"}}}},
		{`{{d | date '2 Jan 2006'}}`, "d", []call{{name: "date", args: []any{"2 Jan 2006"}}}},
		// Word's smart quotes, with an apostrophe inside double quotes.
		{"{{d | date “Jan ’06”}}", "d", []call{{name: "date", args: []any{"Jan ’06"}}}},
		{"{{d | date ‘2006’}}", "d", []call{{name: "date", args: []any{"2006"}}}},
		// A pipe inside quotes is text.
		{`{{x | default "a | b"}}`, "x", []call{{name: "default", args: []any{"a | b"}}}},
		{`{{t | currency "€" 2 "." ","}}`, "t", []call{{name: "currency", args: []any{"€", 2.0, ".", ","}}}},
		{`{{x | number -1.5e2}}`, "x", []call{{name: "number", args: []any{-150.0}}}},
		{`{{x | upper | default "—"}}`, "x", []call{{name: "upper"}, {name: "default", args: []any{"—"}}}},
		{`{{x | default ""}}`, "x", []call{{name: "default", args: []any{""}}}},
	}
	for _, c := range cases {
		tags, bad := lexTags(c.in)
		if len(bad) != 0 || len(tags) != 1 {
			t.Errorf("lexTags(%q) = %+v, bad %+v", c.in, tags, bad)
			continue
		}
		got := tags[0]
		if got.kind != tagValue || got.path != c.path || !reflect.DeepEqual(got.calls, c.calls) {
			t.Errorf("lexTags(%q) = path %q calls %+v, want %q %+v", c.in, got.path, got.calls, c.path, c.calls)
		}
	}
}

func TestBuiltinFormatters(t *testing.T) {
	ts := time.Date(2026, 3, 4, 15, 6, 7, 0, time.UTC)
	cases := []struct {
		name string
		v    any
		args []any
		want string
	}{
		{"upper", "abc", nil, "ABC"},
		{"upper", 1.5, nil, "1.5"},
		{"lower", "ÄBC", nil, "äbc"},
		{"default", nil, []any{"—"}, "—"},
		{"default", "", []any{"—"}, "—"},
		{"default", "x", []any{"—"}, "x"},
		{"default", 0.0, []any{"—"}, "0"},
		{"number", 1234567.891, []any{2.0}, "1,234,567.89"},
		{"number", 2.675, []any{2.0}, "2.68"}, // exact decimal, not the float below it
		{"number", 2.5, []any{0.0}, "3"},
		{"number", -2.5, []any{0.0}, "-3"},
		{"number", -0.001, []any{2.0}, "0.00"}, // no "-0.00"
		{"number", 0.5, []any{3.0}, "0.500"},
		{"number", 999.995, []any{2.0}, "1,000.00"},
		{"number", 1e21, []any{0.0}, "1,000,000,000,000,000,000,000"},
		{"number", json.Number("12345.6785"), []any{3.0}, "12,345.679"},
		{"number", " 42 ", []any{1.0}, "42.0"},
		{"number", 1234, []any{0.0, ""}, "1234"},
		{"number", 1234567.5, []any{1.0, ".", ","}, "1.234.567,5"},
		{"number", int64(-7), []any{0.0}, "-7"},
		{"currency", 1234.5, []any{"€"}, "€1,234.50"},
		{"currency", -1234.5, []any{"$", 0.0}, "-$1,235"},
		{"currency", 1234.5, []any{"€ ", 2.0, ".", ","}, "€ 1.234,50"},
		{"date", "2026-03-04", []any{"2 Jan 2006"}, "4 Mar 2026"},
		{"date", "2026-03-04T15:06:07Z", []any{"2006-01-02 15:04"}, "2026-03-04 15:06"},
		{"date", "2026-03-04T15:06:07+05:30", []any{"15:04 -07:00"}, "15:06 +05:30"},
		{"date", "2026-03-04T15:06:07.123456Z", []any{"05.000"}, "07.123"},
		{"date", ts, []any{"Jan 2"}, "Mar 4"},
	}
	for _, c := range cases {
		b := builtins[c.name]
		if detail := b.args(c.args); detail != "" {
			t.Errorf("%s %v: args rejected: %s", c.name, c.args, detail)
			continue
		}
		out, err := b.fn(c.v, c.args)
		if err != nil {
			t.Errorf("%s(%#v, %v): %v", c.name, c.v, c.args, err)
			continue
		}
		if s, _ := format(out); s != c.want {
			t.Errorf("%s(%#v, %v) = %q, want %q", c.name, c.v, c.args, s, c.want)
		}
	}
}

func TestBuiltinFormatters_ValueErrors(t *testing.T) {
	cases := []struct {
		name string
		v    any
		args []any
		want string // substring of the error
	}{
		{"number", "abc", []any{2.0}, "not a number"},
		{"number", "0x10", []any{2.0}, "not a number"},
		{"number", "Inf", []any{2.0}, "not a number"},
		{"number", nil, []any{2.0}, "no value"},
		{"number", true, []any{2.0}, "not a number"},
		{"currency", []any{1.0}, []any{"€"}, "not a number"},
		{"date", "04/03/2026", []any{"2006"}, "not an RFC 3339"},
		{"date", 5.0, []any{"2006"}, "not a date"},
		{"upper", map[string]any{}, nil, "no text"},
	}
	for _, c := range cases {
		_, err := builtins[c.name].fn(c.v, c.args)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s(%#v) error = %v, want one containing %q", c.name, c.v, err, c.want)
		}
	}
}

func TestBuiltinFormatters_ArgErrors(t *testing.T) {
	cases := []struct {
		name string
		args []any
	}{
		{"upper", []any{1.0}},
		{"default", nil},
		{"default", []any{"a", "b"}},
		{"number", nil},
		{"number", []any{"2"}},
		{"number", []any{2.5}},
		{"number", []any{-1.0}},
		{"number", []any{21.0}},
		{"number", []any{2.0, 3.0}},
		{"number", []any{2.0, ",", ".", "x"}},
		{"currency", nil},
		{"currency", []any{2.0}},
		{"currency", []any{"€", "2"}},
		{"date", nil},
		{"date", []any{2006.0}},
	}
	for _, c := range cases {
		if detail := builtins[c.name].args(c.args); detail == "" {
			t.Errorf("%s %v: arguments accepted, want an error", c.name, c.args)
		}
	}
}

// A json.Number gets the checks a numeric string gets: an exponent that
// would expand to an enormous decimal is refused.
func TestDecimalString_JSONNumber(t *testing.T) {
	for _, bad := range []json.Number{"1e100000", "NaN", "0x10", "abc"} {
		if s, err := decimalString(bad); err == nil {
			t.Errorf("decimalString(%q) = %d chars, want an error", bad, len(s))
		}
	}
	if s, err := decimalString(json.Number("12.5")); err != nil || s != "12.5" {
		t.Errorf("decimalString(12.5) = %q, %v", s, err)
	}
}

type nilStringer struct{ s string }

func (n *nilStringer) String() string { return n.s }

// A Stringer whose String panics (a nil pointer) has no text form; it does
// not crash Execute.
func TestFormat_PanickingStringer(t *testing.T) {
	var p *nilStringer
	if s, ok := format(p); ok {
		t.Errorf("format(nil *T) = %q, true; want no text form", s)
	}
	if s, ok := format(&nilStringer{s: "x"}); !ok || s != "x" {
		t.Errorf("format(&T) = %q, %v", s, ok)
	}
}

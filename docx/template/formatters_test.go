package template_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

func TestFormatters_Body(t *testing.T) {
	doc := docx.New()
	paras(doc,
		`Total: {{total | currency "€" 2}}`,
		// Word's smart quotes.
		"Due: {{due | date “2 January 2006”}}",
		`Name: {{name | upper}}`,
		`Note: {{note | default "—"}}`,
		`Missing: {{absent | upper | default "n/a"}}`,
		`Chained: {{name | lower | default "x" | upper}}`,
		`Exact: {{price | number 2}}`,
	)
	var data map[string]any
	dec := json.NewDecoder(strings.NewReader(`{"total": 1234.5, "due": "2026-12-01", "name": "Acme",
		"note": null, "price": 2.675}`))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		t.Fatal(err)
	}
	mustExecute(t, doc, data)
	want := strings.Join([]string{
		"Total: €1,234.50",
		"Due: 1 December 2026",
		"Name: ACME",
		"Note: —",
		"Missing: n/a",
		"Chained: ACME",
		"Exact: 2.68",
	}, "\n")
	if got := bodyText(t, doc); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Word splits a tag across runs when part of it is formatted differently;
// the pipe and its quoted argument are rejoined like any tag.
func TestFormatters_SplitAcrossRuns(t *testing.T) {
	doc := docx.New()
	p := doc.Body().AddParagraph()
	p.AddRun("{{total | curr")
	r := p.AddRun("ency “€")
	r.Font().SetBold(docx.On())
	p.AddRun("” 0}}")
	mustExecute(t, doc, map[string]any{"total": 99.5})
	if got := bodyText(t, doc); got != "€100" {
		t.Errorf("got %q, want %q", got, "€100")
	}
}

func TestFormatters_InLoop(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#each items}}", `{{.name | upper}}: {{.qty | number 1}}`, "{{/each}}")
	mustExecute(t, doc, map[string]any{"items": []any{
		map[string]any{"name": "a", "qty": 1},
		map[string]any{"name": "b", "qty": 2.25},
	}})
	if got, want := bodyText(t, doc), "A: 1.0\nB: 2.3"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatters_MissingKey(t *testing.T) {
	for _, tag := range []string{"{{x | upper}}", `{{x | number 2}}`} {
		doc := docx.New()
		paras(doc, tag)
		if _, err := template.Execute(doc, map[string]any{}, template.Options{}); !errors.Is(err, template.ErrMissingKey) {
			t.Errorf("%s: err = %v, want ErrMissingKey", tag, err)
		}
	}
	// In Lenient mode a missing key is written as "", formatters or not.
	doc := docx.New()
	paras(doc, "[{{x | upper}}]")
	if _, err := template.Execute(doc, map[string]any{}, template.Options{Mode: template.Lenient}); err != nil {
		t.Fatal(err)
	}
	if got := bodyText(t, doc); got != "[]" {
		t.Errorf("got %q", got)
	}
}

func TestFormatters_UnknownFailsInBothModes(t *testing.T) {
	for _, mode := range []template.Mode{template.Strict, template.Lenient} {
		for _, x := range []bool{true, false} {
			doc := docx.New()
			// The tag is in a branch that x=false drops: still an error.
			paras(doc, "{{#if x}}", "{{v | shout}}", "{{/if}}")
			_, err := template.Execute(doc, map[string]any{"x": x, "v": "a"}, template.Options{Mode: mode})
			if !errors.Is(err, template.ErrUnknownFormatter) {
				t.Errorf("mode %v, x=%v: err = %v, want ErrUnknownFormatter", mode, x, err)
			}
		}
	}
}

// A wrong argument to a built-in is a template error, reported whatever the
// data, in both modes.
func TestFormatters_BadBuiltinArgs(t *testing.T) {
	for _, mode := range []template.Mode{template.Strict, template.Lenient} {
		doc := docx.New()
		paras(doc, "{{#if x}}", `{{v | number "2"}}`, "{{/if}}")
		_, err := template.Execute(doc, map[string]any{"x": false}, template.Options{Mode: mode})
		if !errors.Is(err, template.ErrMalformedTag) {
			t.Errorf("mode %v: err = %v, want ErrMalformedTag", mode, err)
		}
	}
}

func TestFormatters_ValueErrorStrictAndLenient(t *testing.T) {
	doc := docx.New()
	paras(doc, `[{{d | date "2006"}}]`)
	data := map[string]any{"d": "yesterday"}
	if _, err := template.Execute(doc, data, template.Options{}); !errors.Is(err, template.ErrFormat) {
		t.Errorf("Strict: err = %v, want ErrFormat", err)
	}
	warnings, err := template.Execute(doc, data, template.Options{Mode: template.Lenient})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !errors.Is(warnings[0].Err, template.ErrFormat) {
		t.Errorf("Lenient warnings = %v, want one ErrFormat", warnings)
	}
	if got := bodyText(t, doc); got != "[]" {
		t.Errorf("got %q", got)
	}
}

func TestFormatters_Funcs(t *testing.T) {
	lakh := func(v any, args []any) (any, error) {
		f, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("want a number, got %T", v)
		}
		return fmt.Sprintf("₹%.0f lakh", f/1e5), nil
	}
	shout := func(v any, args []any) (any, error) {
		return fmt.Sprint(v) + strings.Repeat("!", int(args[0].(float64))), nil
	}
	doc := docx.New()
	paras(doc, "{{amt | number 2}}", "{{word | shout 3}}")
	_, err := template.Execute(doc, map[string]any{"amt": 2500000.0, "word": "hi"}, template.Options{
		Funcs: map[string]template.Formatter{"number": lakh, "shout": shout},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := bodyText(t, doc), "₹25 lakh\nhi!!!"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatters_FuncsErrorAndPanic(t *testing.T) {
	fail := func(any, []any) (any, error) { return nil, errors.New("nope") }
	boom := func(any, []any) (any, error) { panic("boom") }
	for name, f := range map[string]template.Formatter{"fail": fail, "boom": boom} {
		doc := docx.New()
		paras(doc, "{{v | "+name+"}}")
		before := save(t, doc)
		_, err := template.Execute(doc, map[string]any{"v": 1}, template.Options{Funcs: map[string]template.Formatter{name: f}})
		if !errors.Is(err, template.ErrFormat) {
			t.Errorf("%s: err = %v, want ErrFormat", name, err)
		}
		if after := save(t, doc); string(after) != string(before) {
			t.Errorf("%s: a failed Execute changed the document", name)
		}
	}
}

func TestFormatters_DefaultCannotBeReplaced(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{v}}")
	_, err := template.Execute(doc, map[string]any{"v": 1}, template.Options{
		Funcs: map[string]template.Formatter{"default": func(v any, _ []any) (any, error) { return v, nil }},
	})
	var te *template.Error
	if err == nil || errors.As(err, &te) {
		t.Errorf("err = %v, want a plain configuration error", err)
	}
}

func TestFormatters_CoreProperties(t *testing.T) {
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("body")
	cp := doc.CoreProperties()
	cp.SetTitle(`{{title | upper}} – {{when | date "Jan 2006"}}`)
	if _, err := template.Execute(doc, map[string]any{"title": "audit", "when": "2026-11-19"}, template.Options{}); err != nil {
		t.Fatal(err)
	}
	if got, want := cp.Title(), "AUDIT – Nov 2026"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
}

func TestFormatters_Placeholders(t *testing.T) {
	doc := docx.New()
	paras(doc, `{{total | number 2 | default "—"}} {{name}}`)
	ps, err := template.Placeholders(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("got %d placeholders", len(ps))
	}
	if ps[0].Path != "total" || !reflect.DeepEqual(ps[0].Formatters, []string{"number", "default"}) {
		t.Errorf("first = %+v", ps[0])
	}
	if ps[1].Path != "name" || ps[1].Formatters != nil {
		t.Errorf("second = %+v", ps[1])
	}
}

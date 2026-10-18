package template

import (
	"errors"
	"testing"
)

func TestLexTags(t *testing.T) {
	cases := []struct {
		in   string
		want []tag // only kind, block, path and text are compared
	}{
		{"no tags", nil},
		{"{{name}}", []tag{{kind: tagValue, path: "name", text: "{{name}}"}}},
		{"Dear {{ client.name }},", []tag{{kind: tagValue, path: "client.name", text: "{{ client.name }}"}}},
		{"{{.}} and {{.field.sub}}", []tag{
			{kind: tagValue, path: ".", text: "{{.}}"},
			{kind: tagValue, path: ".field.sub", text: "{{.field.sub}}"},
		}},
		{"{{#each threats}}", []tag{{kind: tagOpen, block: "each", path: "threats", text: "{{#each threats}}"}}},
		{"{{#if  .has_fix }}{{/if}}", []tag{
			{kind: tagOpen, block: "if", path: ".has_fix", text: "{{#if  .has_fix }}"},
			{kind: tagClose, block: "if", text: "{{/if}}"},
		}},
		{"{{#unless x}}{{/unless}}", []tag{
			{kind: tagOpen, block: "unless", path: "x", text: "{{#unless x}}"},
			{kind: tagClose, block: "unless", text: "{{/unless}}"},
		}},
		{"{{image logo}}", []tag{{kind: tagImage, path: "logo", text: "{{image logo}}"}}},
		{"a }} b", nil},
		{"{{a}}}", []tag{{kind: tagValue, path: "a", text: "{{a}}"}}},
		{"{{kebab-case_2}}", []tag{{kind: tagValue, path: "kebab-case_2", text: "{{kebab-case_2}}"}}},
	}
	for _, c := range cases {
		got, bad := lexTags(c.in)
		if len(bad) != 0 {
			t.Errorf("lexTags(%q): unexpected bad tags %+v", c.in, bad)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("lexTags(%q) = %d tags, want %d: %+v", c.in, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			g, w := got[i], c.want[i]
			if g.kind != w.kind || g.block != w.block || g.path != w.path || g.text != w.text {
				t.Errorf("lexTags(%q)[%d] = %+v, want %+v", c.in, i, g, w)
			}
			if c.in[g.start:g.end] != g.text {
				t.Errorf("lexTags(%q)[%d]: offsets [%d,%d) give %q, want %q", c.in, i, g.start, g.end, c.in[g.start:g.end], g.text)
			}
		}
	}
}

func TestLexTags_Bad(t *testing.T) {
	cases := []struct {
		in   string
		want error
	}{
		{"{{name", ErrMalformedTag},
		{"{{}}", ErrMalformedTag},
		{"{{ }}", ErrMalformedTag},
		{"{{1abc}}", ErrMalformedTag},
		{"{{a..b}}", ErrMalformedTag},
		{"{{a.}}", ErrMalformedTag},
		{"{{#for x}}", ErrMalformedTag},
		{"{{#each}}", ErrMalformedTag},
		{"{{/while}}", ErrMalformedTag},
		{"{{a {{b}}", ErrMalformedTag},
		{"{{{x}}}", ErrMalformedTag},
		{"{{na" + string(barrierBreak) + "me}}", ErrMalformedTag},
		{"{{na" + string(barrierBoundary) + "me}}", ErrSplitAcrossBoundary},
	}
	for _, c := range cases {
		tags, bad := lexTags(c.in)
		if len(tags) != 0 || len(bad) != 1 {
			t.Errorf("lexTags(%q) = %d tags, %d bad; want 0 and 1", c.in, len(tags), len(bad))
			continue
		}
		if !errors.Is(bad[0].err, c.want) {
			t.Errorf("lexTags(%q) error = %v, want %v", c.in, bad[0].err, c.want)
		}
	}
}

// A malformed tag does not hide the well-formed tags after it.
func TestLexTags_ContinuesAfterBad(t *testing.T) {
	tags, bad := lexTags("{{1x}} then {{ok}}")
	if len(bad) != 1 || len(tags) != 1 || tags[0].path != "ok" {
		t.Errorf("got tags %+v, bad %+v", tags, bad)
	}
}

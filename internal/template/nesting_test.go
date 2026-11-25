package template

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// A row-edge tag split by an element the run merge does not cross (a
// bookmark) cannot be stripped from the row. It must be reported, not found
// again by every nested expansion until the stack overflows (v0.5.0 review
// H1).
func TestRowEdge_TagSplitByBookmark(t *testing.T) {
	bm := func(id string) string {
		return `<w:bookmarkStart w:id="` + id + `" w:name="b` + id + `"/><w:bookmarkEnd w:id="` + id + `"/>`
	}
	run := func(s string) string { return `<w:r><w:t>` + s + `</w:t></w:r>` }
	row := func(first, last string) string {
		return `<w:tbl><w:tr><w:tc><w:p>` + first + `</w:p></w:tc><w:tc><w:p>` + last + `</w:p></w:tc></w:tr></w:tbl><w:p/>`
	}
	cases := []struct{ name, body string }{
		{"open and close split", row(run("{{#each it")+bm("1")+run("ems}}x"), run("y{{/ea")+bm("2")+run("ch}}"))},
		{"open split", row(run("{{#each it")+bm("1")+run("ems}}x"), run("y{{/each}}"))},
		{"close split", row(run("{{#each items}}x"), run("y{{/ea")+bm("2")+run("ch}}"))},
		{"if split", row(run("{{#if i")+bm("1")+run("tems}}x"), run("y{{/if}}"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := executeBody(t, tc.body, map[string]any{"items": []any{"a"}})
			var te *Error
			if !errors.As(err, &te) {
				t.Fatalf("err = %v, want *Error", err)
			}
			found := false
			for _, is := range te.Issues {
				found = found || errors.Is(is.Err, ErrSplitAcrossBoundary)
			}
			if !found {
				t.Errorf("no ErrSplitAcrossBoundary issue:\n%v", err)
			}
		})
	}
}

// nestedParagraphs returns n nested paragraph ranges of the given block
// around middle paragraphs, each saying "x".
func nestedParagraphs(block string, n, middle int) string {
	open, close := "{{#if a}}", "{{/if}}"
	if block == "each" {
		open, close = "{{#each l}}", "{{/each}}"
	}
	p := func(s string) string { return `<w:p><w:r><w:t>` + s + `</w:t></w:r></w:p>` }
	var sb strings.Builder
	sb.WriteString(strings.Repeat(p(open), n))
	sb.WriteString(strings.Repeat(p("x"), middle))
	sb.WriteString(strings.Repeat(p(close), n))
	return sb.String()
}

// nestedRows returns a table of n nested row ranges, each opening one row
// later and closing one row earlier than the range around it, around middle
// plain rows.
func nestedRows(n, middle int) string {
	cell := func(s string) string { return `<w:tc><w:p><w:r><w:t>` + s + `</w:t></w:r></w:p></w:tc>` }
	var sb strings.Builder
	sb.WriteString(`<w:tbl>`)
	sb.WriteString(strings.Repeat(`<w:tr>`+cell("{{#if a}}o")+cell("p")+`</w:tr>`, n))
	sb.WriteString(strings.Repeat(`<w:tr>`+cell("x")+cell("y")+`</w:tr>`, middle))
	sb.WriteString(strings.Repeat(`<w:tr>`+cell("c")+cell("d{{/if}}")+`</w:tr>`, n))
	sb.WriteString(`</w:tbl><w:p/>`)
	return sb.String()
}

var nestData = map[string]any{"a": true, "l": []any{1}}

// nestTimeLimit bounds the nesting tests. They take well under a second
// without the race detector, which slows them about twentyfold; before
// v0.5.1 the deep cases took minutes or ran out of memory.
const nestTimeLimit = 30 * time.Second

// Block ranges nest at most maxNesting deep. Deeper nesting fails quickly
// with ErrOutputLimit instead of recursing (and, before v0.5.1, rescanning)
// once per level (v0.5.0 review H5).
func TestNesting_TooDeep(t *testing.T) {
	cases := []struct{ name, body string }{
		{"if", nestedParagraphs("if", 8000, 1)},
		{"each", nestedParagraphs("each", 8000, 1)},
		{"rows", nestedRows(3000, 1)},
		{"just over", nestedParagraphs("if", maxNesting+1, 1)},
		// Paragraph ranges in a cell count with the row ranges around it.
		{"rows and paragraphs", strings.Replace(nestedRows(maxNesting/2, 1), "<w:tc><w:p><w:r><w:t>x</w:t></w:r></w:p></w:tc>",
			"<w:tc>"+nestedParagraphs("if", maxNesting/2+1, 1)+"</w:tc>", 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			_, _, err := executeBody(t, tc.body, nestData)
			if !errors.Is(err, ErrOutputLimit) {
				t.Fatalf("err = %v, want ErrOutputLimit", err)
			}
			var te *Error
			if errors.As(err, &te) && len(te.Issues) != 1 {
				t.Errorf("%d issues, want 1:\n%v", len(te.Issues), err)
			}
			if d := time.Since(start); d > nestTimeLimit {
				t.Errorf("took %v", d)
			}
		})
	}
}

// Nesting exactly maxNesting deep, around a large body, works and stays
// fast: paragraph ranges are planned once per list, and row edges are
// examined once per row until the row is stripped.
func TestNesting_AtLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("large template")
	}
	for _, block := range []string{"if", "each"} {
		t.Run(block, func(t *testing.T) {
			const middle = 20000
			start := time.Now()
			text, _, err := executeBody(t, nestedParagraphs(block, maxNesting, middle), nestData)
			if err != nil {
				t.Fatal(err)
			}
			if want := strings.TrimSuffix(strings.Repeat("x\n", middle), "\n"); text != want {
				t.Errorf("got %d bytes of text, want %d", len(text), len(want))
			}
			if d := time.Since(start); d > nestTimeLimit {
				t.Errorf("took %v", d)
			}
		})
	}
	t.Run("rows", func(t *testing.T) {
		start := time.Now()
		if _, _, err := executeBody(t, nestedRows(maxNesting, 5000), nestData); err != nil {
			t.Fatal(err)
		}
		if d := time.Since(start); d > nestTimeLimit {
			t.Errorf("took %v", d)
		}
	})
}

// A field that starts in one branch and ends outside it is refused at the
// level where it crosses, however deep (the branch check is computed once
// per list from per-block summaries).
func TestNesting_FieldAcrossNestedBranch(t *testing.T) {
	p := func(s string) string { return `<w:p><w:r><w:t>` + s + `</w:t></w:r></w:p>` }
	begin := `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText>PAGE</w:instrText></w:r></w:p>`
	end := `<w:p><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`
	balanced := p("{{#if a}}") + p("{{#if b}}") + begin + end + p("{{/if}}") + p("{{/if}}")
	if _, _, err := executeBody(t, balanced, map[string]any{"a": true, "b": true}); err != nil {
		t.Fatalf("balanced: %v", err)
	}
	crossing := p("{{#if a}}") + p("{{#if b}}") + begin + p("{{/if}}") + end + p("{{/if}}")
	for _, b := range []bool{true, false} {
		_, _, err := executeBody(t, crossing, map[string]any{"a": true, "b": b})
		if !errors.Is(err, ErrSplitAcrossBoundary) {
			t.Errorf("b=%v: err = %v, want ErrSplitAcrossBoundary", b, err)
		}
	}
}

// Nested ranges of every kind still expand correctly once the structure is
// computed once per list.
func TestNesting_MixedRanges(t *testing.T) {
	p := func(s string) string { return `<w:p><w:r><w:t>` + s + `</w:t></w:r></w:p>` }
	body := p("{{#each outer}}") + p("o{{.n}}") +
		p("{{#if .show}}") + p("shown") + p("{{else}}") + p("hidden") + p("{{/if}}") +
		p("{{#each .inner}}") + p("i{{.}}") + p("{{/each}}") +
		p("{{/each}}") + p("end")
	data := map[string]any{"outer": []any{
		map[string]any{"n": 1, "show": true, "inner": []any{"a", "b"}},
		map[string]any{"n": 2, "show": false, "inner": []any{}},
	}}
	text, _, err := executeBody(t, body, data)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{"o1", "shown", "ia", "ib", "o2", "hidden", "end"}, "\n")
	if text != want {
		t.Errorf("text =\n%s\nwant\n%s", text, want)
	}
}

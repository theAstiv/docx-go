package template_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

func TestInline_Conditions(t *testing.T) {
	tests := []struct {
		text string
		data map[string]any
		want string
	}{
		{"Dear {{#if vip}}valued {{/if}}customer", map[string]any{"vip": true}, "Dear valued customer"},
		{"Dear {{#if vip}}valued {{/if}}customer", map[string]any{"vip": false}, "Dear customer"},
		{"Dear {{#if vip}}valued {{/if}}customer", map[string]any{}, "Dear customer"},
		{"Status: {{#if ok}}pass{{else}}fail{{/if}}.", map[string]any{"ok": true}, "Status: pass."},
		{"Status: {{#if ok}}pass{{else}}fail{{/if}}.", map[string]any{"ok": false}, "Status: fail."},
		{"{{#unless ok}}not {{/unless}}done", map[string]any{"ok": false}, "not done"},
		{"{{#unless ok}}not {{else}}all {{/unless}}done", map[string]any{"ok": true}, "all done"},
		// The kept branch's value tags are filled; the dropped branch's are
		// not required.
		{"{{#if who}}Hi {{who}}{{else}}Hi {{nobody}}{{/if}}!", map[string]any{"who": "Ana"}, "Hi Ana!"},
		// Two ranges in one paragraph.
		{"{{#if a}}A{{/if}}-{{#if b}}B{{/if}}", map[string]any{"a": false, "b": true}, "-B"},
		// A whole-paragraph inline range leaves an empty paragraph.
		{"{{#if a}}x{{/if}}{{#if b}}y{{/if}}", map[string]any{}, ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%v", tt.text, tt.data), func(t *testing.T) {
			doc := docx.New()
			paras(doc, tt.text)
			mustExecute(t, doc, tt.data)
			if got := bodyText(t, doc); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInline_Nested(t *testing.T) {
	const text = "[{{#if a}}A{{#if b}}B{{else}}b{{/if}}{{else}}-{{#unless b}}!{{/unless}}{{/if}}]"
	for _, tt := range []struct {
		a, b bool
		want string
	}{
		{true, true, "[AB]"}, {true, false, "[Ab]"}, {false, true, "[-]"}, {false, false, "[-!]"},
	} {
		doc := docx.New()
		paras(doc, text)
		mustExecute(t, doc, map[string]any{"a": tt.a, "b": tt.b})
		if got := bodyText(t, doc); got != tt.want {
			t.Errorf("a=%v b=%v: got %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}

// Inline ranges work inside paragraph and row ranges, with loop fields.
func TestInline_InsideBlocks(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#each threats}}", "{{.id}}{{#if .high}} (high){{/if}}: {{.title}}", "{{/each}}")
	table(doc, [][]string{
		{"{{#each threats}}{{#if .high}}!{{/if}}{{.id}}", "{{.title}}{{/each}}"},
	})
	mustExecute(t, doc, map[string]any{"threats": threats})
	re := reopen(t, doc)
	if got, want := strings.Join(texts(re.Body()), "\n"), "T1 (high): Spoofing\nT2: Tampering\nT3 (high): Repudiation"; !strings.HasPrefix(got, want) {
		t.Errorf("paragraphs:\n%s\nwant prefix:\n%s", got, want)
	}
	if got, want := strings.Join(tableTexts(re.Body())[0], "\n"), "!T1 | Spoofing\nT2 | Tampering\n!T3 | Repudiation"; got != want {
		t.Errorf("rows:\n%s\nwant:\n%s", got, want)
	}
}

// Formatting, tabs and line breaks inside a removed branch go with it; the
// formatting of the kept text is untouched.
func TestInline_RunContent(t *testing.T) {
	for _, on := range []bool{true, false} {
		doc := docx.New()
		p := doc.Body().AddParagraph()
		p.AddRun("a{{#if x}}")
		r := p.AddRun("bold")
		r.Font().SetBold(docx.On())
		r.AddTab()
		r.AddBreak(docx.BreakLine)
		p.AddRun("{{/if}}z")
		mustExecute(t, doc, map[string]any{"x": on})
		got := bodyText(t, doc)
		want := "az"
		if on {
			want = "abold\t\nz"
		}
		if got != want {
			t.Errorf("x=%v: got %q, want %q", on, got, want)
		}
	}
}

// A tag Word split across runs (here with a formatting change) is rejoined
// before the range is found.
func TestInline_SplitTags(t *testing.T) {
	doc := docx.New()
	p := doc.Body().AddParagraph()
	p.AddRun("x{{#if v")
	p.AddRun("ip}}VIP{{/i").Font().SetItalic(docx.On())
	p.AddRun("f}}y")
	mustExecute(t, doc, map[string]any{"vip": false})
	if got := bodyText(t, doc); got != "xy" {
		t.Errorf("got %q, want %q", got, "xy")
	}
}

func TestInline_InHyperlink(t *testing.T) {
	doc := docx.New()
	p := doc.Body().AddParagraph()
	p.AddRun("see ")
	h, err := p.AddHyperlink("https://example.com", docx.HyperlinkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h.AddRun("the {{#if full}}full {{/if}}report")
	mustExecute(t, doc, map[string]any{"full": false})
	if got := bodyText(t, doc); got != "see the report" {
		t.Errorf("got %q", got)
	}
}

// A paragraph bookmark sits outside the range, so it does not block it.
func TestInline_ParagraphBookmark(t *testing.T) {
	doc := docx.New()
	p := doc.Body().AddParagraph()
	p.AddRun("a {{#if x}}b {{/if}}c")
	if _, err := p.AddBookmark("sec"); err != nil {
		t.Fatal(err)
	}
	mustExecute(t, doc, map[string]any{"x": false})
	if got := bodyText(t, doc); got != "a c" {
		t.Errorf("got %q", got)
	}
	n := 0
	for range reopen(t, doc).Bookmarks() {
		n++
	}
	if n != 1 {
		t.Errorf("bookmarks = %d, want 1", n)
	}
}

// Errors do not depend on the data: each is checked for both outcomes.
func TestInline_Errors(t *testing.T) {
	tests := []struct {
		name  string
		build func(*docx.Document)
		want  error
	}{
		{"each inline", func(d *docx.Document) { paras(d, "items: {{#each items}}{{.}} {{/each}}") }, template.ErrMisplacedBlock},
		{"closes in another paragraph", func(d *docx.Document) { paras(d, "a {{#if x}} b", "c {{/if}} d") }, template.ErrMisplacedBlock},
		{"mismatched close", func(d *docx.Document) { paras(d, "a {{#if x}} b {{/unless}}") }, template.ErrMisplacedBlock},
		{"second else", func(d *docx.Document) { paras(d, "{{#if x}}a{{else}}b{{else}}c{{/if}}") }, template.ErrUnbalancedBlock},
		{"else outside", func(d *docx.Document) { paras(d, "a {{else}} b") }, template.ErrMisplacedBlock},
		{"half in a hyperlink", func(d *docx.Document) {
			p := d.Body().AddParagraph()
			p.AddRun("{{#if x}}a ")
			h, _ := p.AddHyperlink("https://example.com", docx.HyperlinkOptions{})
			h.AddRun("b{{/if}}")
		}, template.ErrMisplacedBlock},
		{"inline each in a dropped block", func(d *docx.Document) {
			paras(d, "{{#if x}}", "{{#each items}}{{.}}{{/each}}", "{{/if}}")
		}, template.ErrMisplacedBlock},
	}
	for _, tt := range tests {
		for _, x := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/x=%v", tt.name, x), func(t *testing.T) {
				doc := docx.New()
				tt.build(doc)
				before := save(t, doc)
				_, err := template.Execute(doc, map[string]any{"x": x, "items": []any{"i"}}, template.Options{})
				if !errors.Is(err, tt.want) {
					t.Errorf("err = %v, want %v", err, tt.want)
				}
				if after := save(t, doc); string(after) != string(before) {
					t.Error("a failed Execute changed the document")
				}
			})
		}
	}
}

// A range can remove whole hyperlinks and fields.
func TestInline_RemovesWholeElements(t *testing.T) {
	for _, x := range []bool{true, false} {
		doc := docx.New()
		p := doc.Body().AddParagraph()
		p.AddRun("Report{{#if x}}, see ")
		h, err := p.AddHyperlink("https://example.com", docx.HyperlinkOptions{})
		if err != nil {
			t.Fatal(err)
		}
		h.AddRun("the portal")
		p.AddRun(" (page ")
		p.AddPageNumber()
		p.AddRun("){{/if}}.")
		mustExecute(t, doc, map[string]any{"x": x})
		re := reopen(t, doc)
		got := bodyText(t, doc)
		links, fields := 0, 0
		for para := range re.Body().Paragraphs() {
			for range para.Hyperlinks() {
				links++
			}
			for range para.Fields() {
				fields++
			}
		}
		if x {
			if !strings.HasPrefix(got, "Report, see the portal (page ") || links != 1 || fields != 1 {
				t.Errorf("x=true: got %q, %d links, %d fields", got, links, fields)
			}
		} else if got != "Report." || links != 0 || fields != 0 {
			t.Errorf("x=false: got %q, %d links, %d fields", got, links, fields)
		}
	}
}

// A one-cell row whose range has an {{else}} is inline: a row range cannot
// hold an else in its opening row.
func TestInline_OneCellRowWithElse(t *testing.T) {
	for _, x := range []bool{true, false} {
		doc := docx.New()
		table(doc, [][]string{{"{{#if x}}A{{else}}B{{/if}}"}})
		mustExecute(t, doc, map[string]any{"x": x})
		got := strings.Join(tableTexts(reopen(t, doc).Body())[0], "\n")
		want := map[bool]string{true: "A", false: "B"}[x]
		if got != want {
			t.Errorf("x=%v: got %q, want %q", x, got, want)
		}
	}
}

// An inline range at the start of a row's first cell is not a row range
// (C13); the row and its other cells stay.
func TestInline_RowEdgeIsInline(t *testing.T) {
	for _, x := range []bool{true, false} {
		doc := docx.New()
		table(doc, [][]string{{"{{#if x}}yes{{/if}}", "b"}, {"c", "{{#if x}}d{{/if}}"}})
		mustExecute(t, doc, map[string]any{"x": x})
		got := strings.Join(tableTexts(reopen(t, doc).Body())[0], "\n")
		want := " | b\nc | "
		if x {
			want = "yes | b\nc | d"
		}
		if got != want {
			t.Errorf("x=%v: got %q, want %q", x, got, want)
		}
	}
}

// A range that fills a one-cell row on its own is still a row range: the
// row repeats or goes, as before inline ranges existed.
func TestInline_OneCellRowIsRowRange(t *testing.T) {
	doc := docx.New()
	table(doc, [][]string{{"head"}, {"{{#each items}}- {{.}}{{/each}}"}, {"{{#if x}}only when x{{/if}}"}, {"foot"}})
	mustExecute(t, doc, map[string]any{"items": []any{"a", "b"}, "x": false})
	got := strings.Join(tableTexts(reopen(t, doc).Body())[0], "\n")
	if want := "head\n- a\n- b\nfoot"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInline_Placeholders(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#if x}}a{{else}}b{{/if}}")
	ps, err := template.Placeholders(doc)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, p := range ps {
		kinds = append(kinds, p.Kind)
	}
	if got := strings.Join(kinds, " "); got != "if else end" {
		t.Errorf("kinds = %q", got)
	}
}

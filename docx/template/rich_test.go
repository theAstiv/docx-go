package template_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

// firstParagraph returns the body's first paragraph.
func firstParagraph(doc *docx.Document) *docx.Paragraph {
	for p := range doc.Body().Paragraphs() {
		return p
	}
	return nil
}

// runsByText maps each direct run's text to its run.
func runsByText(p *docx.Paragraph) map[string]*docx.Run {
	out := map[string]*docx.Run{}
	for r := range p.Runs() {
		out[r.Text()] = r
	}
	return out
}

func isOn(b *bool) bool { return b != nil && *b }

func TestRich_Runs(t *testing.T) {
	doc := docx.New()
	p := doc.Body().AddParagraph()
	r := p.AddRun("Status: {{status}} done.")
	r.Font().SetItalic(docx.On())
	mustExecute(t, doc, map[string]any{"status": template.Rich{Runs: []template.RichRun{
		{Text: "High", Bold: docx.On(), Color: "c00000"},
		{Text: " risk", Italic: docx.Off(), Underline: docx.On()},
	}}})
	re := reopen(t, doc)
	para := firstParagraph(re)
	if got := para.Text(); got != "Status: High risk done." {
		t.Fatalf("text = %q", got)
	}
	runs := runsByText(para)
	if len(runs) != 4 {
		t.Fatalf("runs = %v, want 4", slices.Collect(func(yield func(string) bool) {
			for k := range runs {
				if !yield(k) {
					return
				}
			}
		}))
	}
	high, risk := runs["High"], runs[" risk"]
	// "High": bold, red, and still italic from the tag run.
	if !isOn(high.Font().Bold()) || !isOn(high.Font().Italic()) || high.Font().Color() != "C00000" {
		t.Errorf("High: bold %v italic %v colour %q", high.Font().Bold(), high.Font().Italic(), high.Font().Color())
	}
	// " risk": italic switched off, underlined.
	if it := risk.Font().Italic(); it == nil || *it || risk.Font().Underline() != docx.UnderlineSingle {
		t.Errorf(" risk: italic %v underline %q", it, risk.Font().Underline())
	}
	// The text around the tag keeps the tag run's formatting.
	for _, s := range []string{"Status: ", " done."} {
		if !isOn(runs[s].Font().Italic()) {
			t.Errorf("%q lost the tag run's italic", s)
		}
	}
}

// Several rich tags in one run, a rich value at a run's start and end, and
// line breaks and tabs inside a rich run.
func TestRich_Positions(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{a}} and {{b}}", "{{c}}")
	rich := func(s string) template.Rich {
		return template.Rich{Runs: []template.RichRun{{Text: s, Bold: docx.On()}}}
	}
	mustExecute(t, doc, map[string]any{"a": rich("A"), "b": rich("B"), "c": rich("line1\nline2\tend")})
	if got, want := bodyText(t, doc), "A and B\nline1\nline2\tend"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRich_Link(t *testing.T) {
	doc := docx.New()
	paras(doc, "See {{site}}.")
	mustExecute(t, doc, map[string]any{"site": template.Rich{Runs: []template.RichRun{
		{Text: "our "}, {Text: "portal", Link: "https://example.com/portal", Bold: docx.On()},
	}}})
	re := reopen(t, doc)
	para := firstParagraph(re)
	if got := para.Text(); got != "See our portal." {
		t.Errorf("text = %q", got)
	}
	links := slices.Collect(para.Hyperlinks())
	if len(links) != 1 || links[0].URL() != "https://example.com/portal" || links[0].Text() != "portal" {
		t.Fatalf("links = %v", links)
	}
	for r := range links[0].Runs() {
		if r.Font().Style() != "Hyperlink" || !isOn(r.Font().Bold()) {
			t.Errorf("link run: style %q bold %v", r.Font().Style(), r.Font().Bold())
		}
	}
	if re.Styles().ByID("Hyperlink") == nil {
		t.Error("Hyperlink style not added")
	}
}

// A link in a header gets its relationship on the header part.
func TestRich_LinkInHeader(t *testing.T) {
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("body")
	doc.LastSection().SetHeader(docx.HeaderDefault).SetText("{{site}}")
	mustExecute(t, doc, map[string]any{"site": template.Rich{Runs: []template.RichRun{{Text: "home", Link: "https://example.com"}}}})
	out := save(t, doc)
	if !strings.Contains(strings.Join(zipNames(t, out), " "), "word/_rels/header1.xml.rels") {
		t.Error("no header relationships part")
	}
	h := reopen(t, doc).LastSection().Header(docx.HeaderDefault)
	n := 0
	for p := range h.Paragraphs() {
		for l := range p.Hyperlinks() {
			n++
			if l.URL() != "https://example.com" {
				t.Errorf("URL = %q", l.URL())
			}
		}
	}
	if n != 1 {
		t.Errorf("%d header links, want 1", n)
	}
}

// Inside an existing hyperlink, rich runs without links are spliced in;
// a run with a link is refused.
func TestRich_InsideHyperlink(t *testing.T) {
	build := func() *docx.Document {
		doc := docx.New()
		p := doc.Body().AddParagraph()
		h, err := p.AddHyperlink("https://example.com", docx.HyperlinkOptions{})
		if err != nil {
			t.Fatal(err)
		}
		h.AddRun("visit {{name}} now")
		return doc
	}
	doc := build()
	mustExecute(t, doc, map[string]any{"name": template.Rich{Runs: []template.RichRun{{Text: "Acme", Bold: docx.On()}}}})
	links := slices.Collect(firstParagraph(reopen(t, doc)).Hyperlinks())
	if len(links) != 1 || links[0].Text() != "visit Acme now" {
		t.Errorf("links = %v", links)
	}

	doc = build()
	_, err := template.Execute(doc, map[string]any{"name": template.Rich{Runs: []template.RichRun{{Text: "x", Link: "https://a.example"}}}}, template.Options{})
	if !errors.Is(err, template.ErrUnsupportedLocation) {
		t.Errorf("err = %v, want ErrUnsupportedLocation", err)
	}
}

func TestRich_Errors(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want error
	}{
		{"unsafe URL", template.Rich{Runs: []template.RichRun{{Text: "x", Link: "javascript:alert(1)"}}}, template.ErrUnsupportedValue},
		{"relative URL", template.Rich{Runs: []template.RichRun{{Text: "x", Link: "/path"}}}, template.ErrUnsupportedValue},
		{"handler scheme", template.Rich{Runs: []template.RichRun{{Text: "x", Link: "ms-msdt:/id PCWDiagnostic"}}}, template.ErrUnsupportedValue},
		{"file URL", template.Rich{Runs: []template.RichRun{{Text: "x", Link: "file://attacker/share/x"}}}, template.ErrUnsupportedValue},
		{"bad colour", template.Rich{Runs: []template.RichRun{{Text: "x", Color: "red"}}}, template.ErrUnsupportedValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := docx.New()
			paras(doc, "{{v}}")
			before := save(t, doc)
			if _, err := template.Execute(doc, map[string]any{"v": tt.v}, template.Options{}); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if after := save(t, doc); !bytes.Equal(after, before) {
				t.Error("a failed Execute changed the package")
			}
		})
	}
}

// A failed Execute after adding a link leaves no relationship and no
// Hyperlink style behind.
func TestRich_FailedExecuteIsAtomic(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{site}}", "{{missing}}")
	before := save(t, doc)
	data := map[string]any{"site": template.Rich{Runs: []template.RichRun{{Text: "home", Link: "https://example.com"}}}}
	if _, err := template.Execute(doc, data, template.Options{}); !errors.Is(err, template.ErrMissingKey) {
		t.Fatalf("err = %v, want ErrMissingKey", err)
	}
	if after := save(t, doc); !bytes.Equal(after, before) {
		t.Error("a failed Execute changed the package")
	}
	if doc.Styles().ByID("Hyperlink") != nil {
		t.Error("the Hyperlink style was left behind")
	}
}

// Where only text can go, a rich value writes its plain text with a
// warning.
func TestRich_PlainTextInProperties(t *testing.T) {
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("body")
	cp := doc.CoreProperties()
	cp.SetTitle("{{title}}")
	warnings, err := template.Execute(doc, map[string]any{"title": template.Rich{Runs: []template.RichRun{
		{Text: "Gap ", Bold: docx.On()}, {Text: "report", Link: "https://example.com"},
	}}}, template.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if cp.Title() != "Gap report" {
		t.Errorf("Title = %q", cp.Title())
	}
	if len(warnings) != 1 {
		t.Errorf("warnings = %v, want 1", warnings)
	}
}

func TestRich_Formatters(t *testing.T) {
	r := template.Rich{Runs: []template.RichRun{{Text: "x", Bold: docx.On()}}}
	doc := docx.New()
	paras(doc, `{{v | upper}}`)
	if _, err := template.Execute(doc, map[string]any{"v": r}, template.Options{}); !errors.Is(err, template.ErrFormat) {
		t.Errorf("upper on rich: err = %v, want ErrFormat", err)
	}
	// default passes a present rich value through, and fills a missing one.
	doc = docx.New()
	paras(doc, `{{v | default "-"}} {{w | default "-"}}`)
	mustExecute(t, doc, map[string]any{"v": r})
	if got := bodyText(t, doc); got != "x -" {
		t.Errorf("got %q", got)
	}
}

func TestRich_InLoopAndLenient(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#each items}}", "- {{.label}}{{.missing}}", "{{/each}}")
	items := []any{
		map[string]any{"label": template.Rich{Runs: []template.RichRun{{Text: "one", Bold: docx.On()}}}},
		map[string]any{"label": &template.Rich{Runs: []template.RichRun{{Text: "two", Italic: docx.On()}}}},
	}
	if _, err := template.Execute(doc, map[string]any{"items": items}, template.Options{Mode: template.Lenient}); err != nil {
		t.Fatal(err)
	}
	if got := bodyText(t, doc); got != "- one\n- two" {
		t.Errorf("got %q", got)
	}
}

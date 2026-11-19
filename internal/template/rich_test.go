package template

import (
	"encoding/xml"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// fakeLinkHooks hands out relationship ids and the Hyperlink style id.
var fakeLinkHooks = Hooks{
	AddImage: nil,
	AddLink:  func(string, string) (string, string, error) { return "rId99", "Hyperlink", nil },
}

// executeXML fills a body built from XML and returns it marshalled, the
// warnings and the error.
func executeXML(t *testing.T, inner string, data map[string]any) (string, []Issue, error) {
	t.Helper()
	var body wml.CT_Body
	if err := xml.Unmarshal([]byte(`<w:body `+testNS+`>`+inner+`</w:body>`), &body); err != nil {
		t.Fatal(err)
	}
	warnings, err := Execute([]Story{{Part: "word/document.xml", Blocks: &body.Children}}, data, Options{Hooks: fakeLinkHooks})
	out, merr := xml.Marshal(&body)
	if merr != nil {
		t.Fatal(merr)
	}
	return string(out), warnings, err
}

func on() *bool { b := true; return &b }

// A colour set on a rich run must win in Word, where a theme colour on the
// tag run would otherwise override the hex value.
func TestRich_ThemeColourDropped(t *testing.T) {
	inner := `<w:p><w:r><w:rPr><w:color w:val="1F3864" w:themeColor="accent1" w:themeShade="80"/></w:rPr><w:t>a {{v}} b</w:t></w:r></w:p>`
	out, _, err := executeXML(t, inner, map[string]any{"v": RichValue{Runs: []RichRun{{Text: "red", Color: "FF0000"}}}})
	if err != nil {
		t.Fatal(err)
	}
	// The red run has no theme colour; the runs around it keep theirs.
	if n := strings.Count(out, "themeColor"); n != 2 {
		t.Errorf("%d theme colours, want 2 (the runs around the tag):\n%s", n, out)
	}
	if !strings.Contains(out, `val="FF0000"`) {
		t.Errorf("red colour missing:\n%s", out)
	}
}

// A linked run drops the tag run's direct colour and underline, so the
// Hyperlink style shows, and a character style it replaces is a warning.
func TestRich_LinkLooksLikeALink(t *testing.T) {
	inner := `<w:p><w:r><w:rPr><w:rStyle w:val="Strong"/><w:color w:val="000080"/><w:u w:val="none"/></w:rPr><w:t>{{v}}</w:t></w:r></w:p>`
	out, warnings, err := executeXML(t, inner, map[string]any{"v": RichValue{Runs: []RichRun{{Text: "site", Link: "https://example.com"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "000080") || strings.Contains(out, `<u `) || strings.Contains(out, `:u `) {
		t.Errorf("the link kept the tag run's colour or underline:\n%s", out)
	}
	if !strings.Contains(out, `val="Hyperlink"`) {
		t.Errorf("no Hyperlink style:\n%s", out)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Err.Error(), "Strong") {
		t.Errorf("warnings = %v, want one about the Strong style", warnings)
	}
	// A colour the run sets itself is kept on the link.
	out, _, _ = executeXML(t, inner, map[string]any{"v": RichValue{Runs: []RichRun{{Text: "site", Link: "https://example.com", Color: "00AA00"}}}})
	if !strings.Contains(out, "00AA00") {
		t.Errorf("the run's own colour was dropped:\n%s", out)
	}
}

// A tracked formatting change on the tag run stays on the original run
// only: copies must not repeat its w:id.
func TestRich_TrackedChangeIDNotRepeated(t *testing.T) {
	inner := `<w:p><w:r><w:rPr><w:b/><w:rPrChange w:id="7" w:author="x" w:date="2026-01-01T00:00:00Z"><w:rPr/></w:rPrChange></w:rPr>` +
		`<w:t>a {{v}} b {{v}} c</w:t></w:r></w:p>`
	out, _, err := executeXML(t, inner, map[string]any{"v": RichValue{Runs: []RichRun{{Text: "X", Italic: on()}}}})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, "rPrChange"); n != 2 { // one start tag, one end tag
		t.Errorf("rPrChange appears %d times, want once (start and end tag = 2):\n%s", n, out)
	}
}

// A tag in the result of a HYPERLINK field is inside a link: a rich run
// with a link is refused there, and one without is fine.
func TestRich_HyperlinkField(t *testing.T) {
	field := func(text string) string {
		return `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> HYPER</w:instrText></w:r>` +
			`<w:r><w:instrText xml:space="preserve">LINK "https://example.com" </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
			`<w:r><w:t>` + text + `</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r><w:r><w:t>{{v}}</w:t></w:r></w:p>`
	}
	link := RichValue{Runs: []RichRun{{Text: "x", Link: "https://a.example"}}}
	if _, _, err := executeXML(t, field("{{v}}"), map[string]any{"v": link}); !errors.Is(err, ErrUnsupportedLocation) {
		t.Errorf("link in a HYPERLINK field: err = %v, want ErrUnsupportedLocation", err)
	}
	// After the field ends, the same value may make a link.
	if _, _, err := executeXML(t, field("text"), map[string]any{"v": link}); err != nil {
		t.Errorf("link after the field: %v", err)
	}
	if _, _, err := executeXML(t, field("{{v}}"), map[string]any{"v": RichValue{Runs: []RichRun{{Text: "x", Bold: on()}}}}); err != nil {
		t.Errorf("rich text without a link in the field: %v", err)
	}
}

// runTag matches a run's start tag however the encoder prefixed it.
var runTag = regexp.MustCompile(`<(?:w:)?r[ >]`)

// An empty rich value inserts nothing and leaves the run whole.
func TestRich_EmptyValueDoesNotSplit(t *testing.T) {
	for _, v := range []any{RichValue{Runs: nil}, RichValue{Runs: []RichRun{{Text: "", Bold: on()}}}} {
		out, _, err := executeXML(t, `<w:p><w:r><w:t xml:space="preserve">a {{v}} b</w:t></w:r></w:p>`, map[string]any{"v": v})
		if err != nil {
			t.Fatal(err)
		}
		if n := len(runTag.FindAllString(out, -1)); n != 1 {
			t.Errorf("%v: %d runs, want 1:\n%s", v, n, out)
		}
	}
}

package template

import (
	"encoding/xml"
	"errors"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Row-range edge tags are found with the paragraph's full text, which
// includes tracked insertions and inline content controls, but only typed
// runs can be edited. A tag held there must be reported, not re-detected
// forever (the range used to re-open until the stack overflowed). The tag
// held in raw content is the
// one reported as unsupported, not only its unpaired partner.
func TestRowEdge_TagInRawContent(t *testing.T) {
	ins := func(s string) string {
		return `<w:ins w:id="9" w:author="a" w:date="2026-01-01T00:00:00Z"><w:r><w:t>` + s + `</w:t></w:r></w:ins>`
	}
	sdt := func(s string) string {
		return `<w:sdt><w:sdtContent><w:r><w:t>` + s + `</w:t></w:r></w:sdtContent></w:sdt>`
	}
	run := func(s string) string { return `<w:r><w:t>` + s + `</w:t></w:r>` }
	row := func(first, last string) string {
		return `<w:tbl><w:tr><w:tc><w:p>` + first + `</w:p></w:tc><w:tc><w:p>` + last + `</w:p></w:tc></w:tr></w:tbl><w:p/>`
	}
	cases := []struct{ name, body string }{
		{"open in ins", row(ins("{{#each items}}")+run("{{.}}"), run("x{{/each}}"))},
		{"close in ins", row(run("{{#each items}}{{.}}"), ins("x{{/each}}"))},
		{"both in ins", row(ins("{{#each items}}")+run("{{.}}"), ins("{{/each}}"))},
		{"open in sdt", row(sdt("{{#each items}}")+run("{{.}}"), run("{{/each}}"))},
		{"close in sdt", row(run("{{#each items}}{{.}}"), run("x")+sdt("{{/each}}"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := executeBody(t, tc.body, map[string]any{"items": []any{"a", "b"}})
			var te *Error
			if !errors.As(err, &te) {
				t.Fatalf("err = %v, want *Error", err)
			}
			found := false
			for _, is := range te.Issues {
				found = found || errors.Is(is.Err, ErrUnsupportedLocation)
			}
			if !found {
				t.Errorf("no ErrUnsupportedLocation issue for the tag in raw content:\n%v", err)
			}
		})
	}
}

// Elements that show no text (a bookmark, an empty content control) before a
// row-edge tag do not stop it being a row edge.
func TestRowEdge_TextlessElementsBeforeTag(t *testing.T) {
	body := `<w:tbl><w:tr><w:tc><w:p><w:bookmarkStart w:id="1" w:name="b"/><w:bookmarkEnd w:id="1"/>` +
		`<w:sdt><w:sdtContent/></w:sdt><w:r><w:t>{{#each items}}{{.}}</w:t></w:r></w:p></w:tc>` +
		`<w:tc><w:p><w:r><w:t>x{{/each}}</w:t></w:r></w:p></w:tc></w:tr></w:tbl><w:p/>`
	var b wml.CT_Body
	if err := xml.Unmarshal([]byte(`<w:body `+testNS+`>`+body+`</w:body>`), &b); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute([]Story{{Part: "word/document.xml", Blocks: &b.Children}}, map[string]any{"items": []any{"a", "b", "c"}}, Options{}); err != nil {
		t.Fatal(err)
	}
	tbl, _ := b.Children[0].(*wml.CT_Tbl)
	if tbl == nil || len(tbl.Children) != 3 {
		t.Fatalf("got %v, want a table with 3 rows", b.Children[0])
	}
}

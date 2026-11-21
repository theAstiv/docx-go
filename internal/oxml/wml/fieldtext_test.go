package wml_test

import (
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func fc(typ string) string { return `<w:r><w:fldChar w:fldCharType="` + typ + `"/></w:r>` }

func instr(s string) string { return `<w:r><w:instrText xml:space="preserve">` + s + `</w:instrText></w:r>` }

func run(s string) string { return `<w:r><w:t xml:space="preserve">` + s + `</w:t></w:r>` }

// Paragraph text follows nested complex fields: a field without a separate
// (XE, TC) hides nothing after its end, a field nested in another field's
// instruction contributes nothing, and a field nested in a result shows.
// The raw path (inside w:ins) follows the same rules.
func TestParagraphText_Fields(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"index entry without separate",
			run("before ") + fc("begin") + instr(` XE "Term" `) + fc("end") + run("after"),
			"before after"},
		{"field nested in an instruction",
			fc("begin") + instr(" IF ") + fc("begin") + instr(" MERGEFIELD x ") + fc("separate") + run("inner") + fc("end") +
				instr(` = "a" "yes" "no" `) + fc("separate") + run("yes") + fc("end") + run("!"),
			"yes!"},
		{"field nested in a result",
			fc("begin") + instr(" TOC ") + fc("separate") + run("Intro ") + fc("begin") + instr(" PAGEREF _Toc1 ") +
				fc("separate") + run("3") + fc("end") + fc("end") + run(" end"),
			"Intro 3 end"},
		{"stray end and separate are ignored",
			fc("end") + run("a") + fc("separate") + run("b"),
			"ab"},
		{"raw runs inside w:ins",
			`<w:ins w:id="1" w:author="x">` + run("x ") + fc("begin") + instr(` XE "y" `) + fc("end") + run("z") + `</w:ins>`,
			"x z"},
	}
	for _, tc := range cases {
		var p wml.CT_P
		if err := xml.Unmarshal([]byte(`<w:p `+textNS+`>`+tc.in+`</w:p>`), &p); err != nil {
			t.Fatal(err)
		}
		if got := p.Text(); got != tc.want {
			t.Errorf("%s: Text = %q, want %q", tc.name, got, tc.want)
		}
	}
}

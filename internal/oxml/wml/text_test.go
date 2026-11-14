package wml_test

import (
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const textNS = `xmlns:w="` + ns.W + `" xmlns:mc="` + ns.MC + `"`

func bodyFrom(t *testing.T, inner string) *wml.CT_Body {
	t.Helper()
	var b wml.CT_Body
	if err := xml.Unmarshal([]byte(`<w:body `+textNS+`>`+inner+`</w:body>`), &b); err != nil {
		t.Fatal(err)
	}
	return &b
}

func para(text string) string {
	return `<w:p><w:r><w:t xml:space="preserve">` + text + `</w:t></w:r></w:p>`
}

func cell(inner string) string { return `<w:tc>` + inner + `</w:tc>` }

func TestBlocksText_Layout(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"paragraphs end with newline",
			para("one") + para("") + para("three"),
			"one\n\nthree\n"},
		{"table: tab between cells, newline per row",
			para("before") +
				`<w:tbl><w:tblPr/><w:tblGrid/>` +
				`<w:tr>` + cell(para("a")) + cell(para("b")) + `</w:tr>` +
				`<w:tr>` + cell(para("c")) + cell(para("d")) + `</w:tr></w:tbl>` +
				para("after"),
			"before\na\tb\nc\td\nafter\n"},
		{"cell paragraphs joined, trailing empty ones dropped",
			`<w:tbl><w:tr>` + cell(para("x")+para("y")+para("")) + cell(para("")) + `</w:tr></w:tbl>`,
			"x\ny\t\n"},
		{"nested table inside a cell",
			`<w:tbl><w:tr>` + cell(`<w:tbl><w:tr>`+cell(para("n1"))+cell(para("n2"))+`</w:tr></w:tbl>`+para("")) + cell(para("z")) + `</w:tr></w:tbl>`,
			"n1\tn2\tz\n"},
		{"block content control",
			`<w:sdt><w:sdtPr><w:alias w:val="Cover"/><w:placeholder><w:docPart w:val="x"/></w:placeholder></w:sdtPr>` +
				`<w:sdtContent>` + para("Title") + para("Sub") + `</w:sdtContent></w:sdt>` + para("body"),
			"Title\nSub\nbody\n"},
		{"table inside a content control",
			`<w:sdt><w:sdtContent><w:tbl><w:tr>` + cell(para("p")) + cell(para("q")) + `</w:tr></w:tbl></w:sdtContent></w:sdt>`,
			"p\tq\n"},
		{"row-level and cell-level content controls",
			`<w:tbl><w:sdt><w:sdtContent><w:tr>` + cell(para("r1")) +
				`<w:sdt><w:sdtContent>` + cell(para("c2")) + `</w:sdtContent></w:sdt></w:tr></w:sdtContent></w:sdt></w:tbl>`,
			"r1\tc2\n"},
		{"alternate content: choice only",
			`<mc:AlternateContent><mc:Choice Requires="w14">` + para("new") + `</mc:Choice><mc:Fallback>` + para("old") + `</mc:Fallback></mc:AlternateContent>`,
			"new\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := wml.BlocksText(bodyFrom(t, c.in).Children); got != c.want {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestParagraphText_RawInline(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"tracked insertion included, deletion excluded",
			`<w:r><w:t xml:space="preserve">a </w:t></w:r>` +
				`<w:ins w:id="1" w:author="x"><w:r><w:t>new</w:t></w:r></w:ins>` +
				`<w:del w:id="2" w:author="x"><w:r><w:delText>old</w:delText><w:tab/></w:r></w:del>` +
				`<w:moveFrom w:id="3" w:author="x"><w:r><w:t>moved</w:t></w:r></w:moveFrom>`,
			"a new"},
		{"smart tag and inline content control",
			`<w:smartTag w:uri="u" w:element="e"><w:smartTagPr/><w:r><w:t>Paris</w:t></w:r></w:smartTag>` +
				`<w:sdt><w:sdtPr><w:alias w:val="A"/></w:sdtPr><w:sdtContent><w:r><w:t xml:space="preserve"> rocks</w:t></w:r></w:sdtContent></w:sdt>`,
			"Paris rocks"},
		{"simple field result included",
			`<w:fldSimple w:instr=" PAGE "><w:r><w:t>7</w:t></w:r></w:fldSimple>`,
			"7"},
		{"complex field inside an insertion: instruction excluded, result included",
			`<w:ins w:id="1" w:author="x"><w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
				`<w:r><w:instrText> DATE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
				`<w:r><w:t>today</w:t><w:br/></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:ins>`,
			"today\n"},
		{"field state carries from raw to typed runs",
			`<w:ins w:id="1" w:author="x"><w:r><w:fldChar w:fldCharType="begin"/></w:r></w:ins>` +
				`<w:r><w:instrText> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
				`<w:r><w:t>3</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>`,
			"3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var p wml.CT_P
			if err := xml.Unmarshal([]byte(`<w:p `+textNS+`>`+c.in+`</w:p>`), &p); err != nil {
				t.Fatal(err)
			}
			if got := p.Text(); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

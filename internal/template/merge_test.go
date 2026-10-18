package template

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const testNS = `xmlns:w="` + ns.W + `"`

func parseP(t *testing.T, inner string) *wml.CT_P {
	t.Helper()
	var p wml.CT_P
	if err := xml.Unmarshal([]byte(`<w:p `+testNS+`>`+inner+`</w:p>`), &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

// describe renders inline content compactly: runs as [text|text], with a
// "b:" prefix when bold; hyperlinks as H(...); other elements by local name.
func describe(children []wml.InlineElement) string {
	var parts []string
	for _, c := range children {
		switch el := c.(type) {
		case *wml.CT_R:
			var texts []string
			for _, rc := range el.Children {
				switch r := rc.(type) {
				case *wml.CT_Text:
					texts = append(texts, r.Content)
				case *wml.RawRunContent:
					texts = append(texts, "<"+r.E.QName.Local+">")
				default:
					texts = append(texts, "<other>")
				}
			}
			prefix := ""
			if el.RPr != nil {
				if b := el.RPr.Bold(); b != nil && *b {
					prefix = "b:"
				}
			}
			parts = append(parts, prefix+"["+strings.Join(texts, "|")+"]")
		case *wml.CT_Hyperlink:
			parts = append(parts, "H("+describe(el.Children)+")")
		case *wml.CT_BookmarkStart:
			parts = append(parts, "bs:"+el.Name)
		case *wml.CT_BookmarkEnd:
			parts = append(parts, "be")
		case *wml.RawInline:
			parts = append(parts, el.E.QName.Local)
		}
	}
	return strings.Join(parts, " ")
}

func TestNormalize_Merges(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"already whole",
			`<w:r><w:t>Dear {{name}},</w:t></w:r>`,
			`[Dear {{name}},]`},
		{"split over three runs, first run's formatting wins",
			`<w:r><w:rPr><w:b/></w:rPr><w:t>{{cli</w:t></w:r><w:r><w:t>ent.na</w:t></w:r><w:r><w:t>me}}</w:t></w:r>`,
			`b:[{{client.name}}]`},
		{"surrounding text stays in its own run",
			`<w:r><w:t xml:space="preserve">Dear {{</w:t></w:r><w:r><w:rPr><w:b/></w:rPr><w:t>name}}, hello</w:t></w:r>`,
			`[Dear {{name}}] b:[, hello]`},
		{"spell-check marks between the pieces are dropped",
			`<w:r><w:t>{{</w:t></w:r><w:proofErr w:type="spellStart"/><w:r><w:t>clnt</w:t></w:r>` +
				`<w:proofErr w:type="spellEnd"/><w:r><w:t>}}</w:t></w:r>`,
			`[{{clnt}}]`},
		{"spell-check marks outside the tag are kept",
			`<w:proofErr w:type="spellStart"/><w:r><w:t>Lorm</w:t></w:r><w:proofErr w:type="spellEnd"/><w:r><w:t> {{a}}</w:t></w:r>`,
			`proofErr [Lorm] proofErr [ {{a}}]`},
		{"_GoBack bookmark moves before the tag",
			`<w:r><w:t>x {{na</w:t></w:r><w:bookmarkStart w:id="0" w:name="_GoBack"/><w:bookmarkEnd w:id="0"/><w:r><w:t>me}}</w:t></w:r>`,
			`bs:_GoBack be [x {{name}}]`},
		{"lastRenderedPageBreak inside a tag is dropped",
			`<w:r><w:t>{{na</w:t></w:r><w:r><w:lastRenderedPageBreak/><w:t>me}}</w:t></w:r>`,
			`[{{name}}]`},
		{"two split tags in one paragraph",
			`<w:r><w:t>{{a</w:t></w:r><w:r><w:t>}} and {{</w:t></w:r><w:r><w:t>b}}</w:t></w:r>`,
			`[{{a}}] [ and {{b}}]`},
		{"split inside a hyperlink",
			`<w:hyperlink w:anchor="x"><w:r><w:t>{{li</w:t></w:r><w:r><w:t>nk}}</w:t></w:r></w:hyperlink>`,
			`H([{{link}}])`},
		{"several w:t in one run",
			`<w:r><w:t>{{a</w:t><w:t>b}}</w:t></w:r>`,
			`[{{ab}}]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := parseP(t, c.in)
			before := p.Text()
			var bad []badTag
			p.Children, bad = normalizeInlines(p.Children, nil)
			if len(bad) != 0 {
				t.Fatalf("unexpected bad tags: %+v", bad)
			}
			if got := describe(p.Children); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
			if after := p.Text(); after != before {
				t.Errorf("paragraph text changed: %q -> %q", before, after)
			}
		})
	}
}

func TestNormalize_Boundaries(t *testing.T) {
	cases := []struct {
		name, in string
		want     error
	}{
		{"across a field",
			`<w:r><w:t>{{na</w:t></w:r><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:t>me}}</w:t></w:r>`,
			ErrSplitAcrossBoundary},
		{"across a bookmark",
			`<w:r><w:t>{{na</w:t></w:r><w:bookmarkStart w:id="1" w:name="sec"/><w:r><w:t>me}}</w:t></w:r>`,
			ErrSplitAcrossBoundary},
		{"across a hyperlink edge",
			`<w:r><w:t>{{na</w:t></w:r><w:hyperlink w:anchor="x"><w:r><w:t>me}}</w:t></w:r></w:hyperlink>`,
			ErrSplitAcrossBoundary}, // the outer "{{na" never closes; "me}}" alone is text
		{"across a tracked insertion",
			`<w:r><w:t>{{na</w:t></w:r><w:ins w:id="1" w:author="a"><w:r><w:t>m</w:t></w:r></w:ins><w:r><w:t>e}}</w:t></w:r>`,
			ErrSplitAcrossBoundary},
		{"with a tab inside",
			`<w:r><w:t>{{na</w:t><w:tab/><w:t>me}}</w:t></w:r>`,
			ErrMalformedTag},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := parseP(t, c.in)
			want := describe(p.Children)
			var bad []badTag
			p.Children, bad = normalizeInlines(p.Children, nil)
			if len(bad) != 1 || !errors.Is(bad[0].err, c.want) {
				t.Fatalf("bad tags = %+v, want one %v", bad, c.want)
			}
			if got := describe(p.Children); got != want {
				t.Errorf("paragraph was modified:\n got  %s\nwant %s", got, want)
			}
		})
	}
}

// Whitespace at the edge of a merged w:t must be preserved.
func TestNormalize_PreservesSpace(t *testing.T) {
	p := parseP(t, `<w:r><w:t>x</w:t><w:t>{{a</w:t></w:r><w:r><w:t xml:space="preserve">}} y</w:t></w:r>`)
	p.Children, _ = normalizeInlines(p.Children, nil)
	r, _ := p.Children[1].(*wml.CT_R)
	tx, _ := r.Children[0].(*wml.CT_Text)
	if tx.Content != " y" || tx.Space != "preserve" {
		t.Errorf("suffix = %q (space %q), want %q preserved", tx.Content, tx.Space, " y")
	}
}

func TestNormalizeBlocks_Locations(t *testing.T) {
	var body wml.CT_Body
	src := `<w:body ` + testNS + `><w:p/><w:tbl><w:tblPr/><w:tblGrid/>` +
		`<w:tr><w:tc><w:p/></w:tc><w:tc><w:p/><w:p><w:r><w:t>{{bad</w:t></w:r></w:p></w:tc></w:tr></w:tbl>` +
		`<w:p><w:r><w:t>{{1}}</w:t></w:r></w:p></w:body>`
	if err := xml.Unmarshal([]byte(src), &body); err != nil {
		t.Fatal(err)
	}
	issues := normalizeBlocks(body.Children, Location{Part: "word/document.xml"}, nil)
	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2: %v", len(issues), issues)
	}
	wantPaths := []string{"tbl[0]/tr[0]/tc[1]/p[1]", "p[1]"}
	for i, is := range issues {
		if is.Location.Path != wantPaths[i] {
			t.Errorf("issue %d at %q, want %q", i, is.Location.Path, wantPaths[i])
		}
	}
}

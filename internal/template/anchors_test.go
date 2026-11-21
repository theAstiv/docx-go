package template

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func tp(s string) string { return `<w:p><w:r><w:t>` + s + `</w:t></w:r></w:p>` }

func bodyOf(t *testing.T, inner string) *wml.CT_Body {
	t.Helper()
	var b wml.CT_Body
	if err := xml.Unmarshal([]byte(`<w:body `+testNS+`>`+inner+`</w:body>`), &b); err != nil {
		t.Fatal(err)
	}
	return &b
}

func marshalBlocks(t *testing.T, blocks []wml.BlockElement) string {
	t.Helper()
	var sb strings.Builder
	for _, b := range blocks {
		out, err := xml.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(out)
	}
	return sb.String()
}

func issuesWith(err error, target error) int {
	var te *Error
	if !errors.As(err, &te) {
		return 0
	}
	n := 0
	for _, is := range te.Issues {
		if errors.Is(is.Err, target) {
			n++
		}
	}
	return n
}

// A dropped branch takes the partner of an anchor outside it with it, as
// docx Remove does (C3); anchors the template already had alone stay.
func TestBlockDrop_RemovesOrphanedPartners(t *testing.T) {
	inner := tp("{{#if show}}") +
		`<w:p><w:bookmarkStart w:id="5" w:name="Gone"/><w:commentRangeStart w:id="2"/><w:r><w:t>hidden</w:t></w:r></w:p>` +
		tp("{{/if}}") +
		`<w:p><w:r><w:t>kept</w:t></w:r><w:bookmarkEnd w:id="5"/><w:commentRangeEnd w:id="2"/><w:bookmarkEnd w:id="77"/></w:p>`
	b := bodyOf(t, inner)
	if _, err := Execute([]Story{{Part: "word/document.xml", Blocks: &b.Children}}, map[string]any{"show": false}, Options{}); err != nil {
		t.Fatal(err)
	}
	out := marshalBlocks(t, b.Children)
	if strings.Contains(out, `id="5"`) || strings.Contains(out, `id="2"`) {
		t.Errorf("orphaned partner left behind:\n%s", out)
	}
	if !strings.Contains(out, `id="77"`) {
		t.Errorf("a pre-existing lone anchor was removed:\n%s", out)
	}
}

// A bookmark a dropped branch removed is reported when a cross-reference
// points at it.
func TestBlockDrop_WarnsReferencedBookmark(t *testing.T) {
	inner := tp("{{#if show}}") +
		`<w:p><w:bookmarkStart w:id="5" w:name="Target"/><w:r><w:t>t</w:t></w:r><w:bookmarkEnd w:id="5"/></w:p>` +
		tp("{{/if}}") +
		`<w:p><w:hyperlink w:anchor="Target"><w:r><w:t>see</w:t></w:r></w:hyperlink></w:p>`
	_, warnings, err := executeBody(t, inner, map[string]any{"show": false})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range warnings {
		found = found || strings.Contains(w.Err.Error(), `"Target"`)
	}
	if !found {
		t.Errorf("no warning for the referenced bookmark: %v", warnings)
	}
}

// A field that starts in a branch and ends outside it, or a marker
// paragraph that holds a drawing, is refused whatever the data.
func TestBlockDrop_Refusals(t *testing.T) {
	fld := func(typ string) string { return `<w:r><w:fldChar w:fldCharType="` + typ + `"/></w:r>` }
	cases := []struct {
		name, inner string
		want        error
	}{
		{"field leaves the branch",
			tp("{{#if show}}") + `<w:p>` + fld("begin") + `<w:r><w:instrText> TOC </w:instrText></w:r>` + fld("separate") + `</w:p>` +
				tp("{{/if}}") + `<w:p><w:r><w:t>x</w:t></w:r>` + fld("end") + `</w:p>`,
			ErrSplitAcrossBoundary},
		{"field crosses else",
			tp("{{#if show}}") + `<w:p>` + fld("begin") + `</w:p>` + tp("{{else}}") + `<w:p>` + fld("end") + `</w:p>` + tp("{{/if}}"),
			ErrSplitAcrossBoundary},
		{"drawing in marker paragraph",
			`<w:p><w:r><w:t>{{#if show}}</w:t></w:r><w:r><w:pict/></w:r></w:p>` + tp("a") + tp("{{/if}}"),
			ErrMisplacedBlock},
		{"row branch holds part of a field",
			`<w:tbl><w:tr><w:tc><w:p><w:r><w:t>{{#if show}}a</w:t></w:r>` + fld("begin") + `</w:p></w:tc><w:tc>` + tp("b{{/if}}") + `</w:tc></w:tr>` +
				`<w:tr><w:tc><w:p>` + fld("end") + `</w:p></w:tc><w:tc>` + tp("c") + `</w:tc></w:tr></w:tbl><w:p/>`,
			ErrSplitAcrossBoundary},
	}
	for _, tc := range cases {
		for _, show := range []bool{true, false} {
			_, _, err := executeBody(t, tc.inner, map[string]any{"show": show})
			if issuesWith(err, tc.want) == 0 {
				t.Errorf("%s (show=%v): err = %v, want %v", tc.name, show, err, tc.want)
			}
		}
	}
}

// A header emptied by a range still ends with a paragraph.
func TestFinalParagraph_EmptiedHeader(t *testing.T) {
	b := bodyOf(t, tp("{{#if show}}")+tp("logo")+tp("{{/if}}"))
	if _, err := Execute([]Story{{Part: "word/header1.xml", Blocks: &b.Children, FinalParagraph: true}},
		map[string]any{"show": false}, Options{}); err != nil {
		t.Fatal(err)
	}
	if len(b.Children) != 1 {
		t.Fatalf("header has %d blocks, want one empty paragraph", len(b.Children))
	}
	if _, ok := b.Children[0].(*wml.CT_P); !ok {
		t.Errorf("header block is %T, want *wml.CT_P", b.Children[0])
	}
}

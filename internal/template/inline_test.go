package template

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// executeBody runs Execute on a body built from XML and returns the body's
// text, the warnings and the error.
func executeBody(t *testing.T, inner string, data map[string]any) (string, []Issue, error) {
	t.Helper()
	var body wml.CT_Body
	if err := xml.Unmarshal([]byte(`<w:body `+testNS+`>`+inner+`</w:body>`), &body); err != nil {
		t.Fatal(err)
	}
	warnings, err := Execute([]Story{{Part: "word/document.xml", Blocks: &body.Children}}, data, Options{})
	var lines []string
	for _, b := range body.Children {
		if p, ok := b.(*wml.CT_P); ok {
			lines = append(lines, p.Text())
		}
	}
	return strings.Join(lines, "\n"), warnings, err
}

// Every refused range in a paragraph is reported, not just the first.
func TestInline_AllRefusalsReported(t *testing.T) {
	ref := `<w:r><w:footnoteReference w:id="1"/></w:r>`
	inner := `<w:p><w:r><w:t>{{#if x}}a</w:t></w:r>` + ref + `<w:r><w:t>{{/if}} {{#unless x}}b</w:t></w:r>` + ref + `<w:r><w:t>{{/unless}}</w:t></w:r></w:p>`
	for _, x := range []bool{true, false} {
		_, _, err := executeBody(t, inner, map[string]any{"x": x})
		var te *Error
		if !errors.As(err, &te) {
			t.Fatalf("x=%v: err = %v, want *Error", x, err)
		}
		n := 0
		for _, is := range te.Issues {
			if errors.Is(is.Err, ErrSplitAcrossBoundary) {
				n++
			}
		}
		if n != 2 {
			t.Errorf("x=%v: %d boundary issues, want 2:\n%v", x, n, err)
		}
	}
}

// Removing a bookmark that a link or a REF field points at is a warning;
// removing one nothing points at is not.
func TestInline_RemovedBookmarkReferences(t *testing.T) {
	target := `<w:p><w:r><w:t>{{#if x}}a </w:t></w:r><w:bookmarkStart w:id="1" w:name="tgt"/><w:r><w:t>b</w:t></w:r>` +
		`<w:bookmarkEnd w:id="1"/><w:r><w:t>{{/if}}c</w:t></w:r></w:p>`
	cases := []struct {
		name, ref string
		warn      bool
	}{
		{"internal link", `<w:p><w:hyperlink w:anchor="tgt"><w:r><w:t>see</w:t></w:r></w:hyperlink></w:p>`, true},
		// Word splits field instructions across runs.
		{"REF field", `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> PAGE</w:instrText></w:r>` +
			`<w:r><w:instrText>REF t</w:instrText></w:r><w:r><w:instrText xml:space="preserve">gt \h </w:instrText></w:r>` +
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>b</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`, true},
		{"simple PAGEREF", `<w:p><w:fldSimple w:instr=" PAGEREF tgt \h "><w:r><w:t>1</w:t></w:r></w:fldSimple></w:p>`, true},
		{"no reference", `<w:p><w:hyperlink w:anchor="other"><w:r><w:t>see</w:t></w:r></w:hyperlink></w:p>`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text, warnings, err := executeBody(t, target+c.ref, map[string]any{"x": false})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(text, "c\n") {
				t.Errorf("text = %q", text)
			}
			got := false
			for _, w := range warnings {
				if strings.Contains(w.Err.Error(), `"tgt"`) {
					got = true
				}
			}
			if got != c.warn {
				t.Errorf("warned = %v, want %v: %v", got, c.warn, warnings)
			}
			// When the branch is kept nothing is removed, so no warning.
			if _, warnings, _ := executeBody(t, target+c.ref, map[string]any{"x": true}); len(warnings) != 0 {
				t.Errorf("x=true: warnings %v", warnings)
			}
		})
	}
}

package template

import (
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Drawings copied by {{#each}} take ids above the ones held outside the
// stories (Options.UsedDocPr: footnotes, endnotes, comments), so a copy
// cannot reuse a note's drawing id (review M1). The template drawing is 49,
// so without UsedDocPr the first copy would take 50.
func TestExecute_CopiesAvoidUsedDocPr(t *testing.T) {
	drawing := `<w:r><w:drawing><wp:inline><wp:extent cx="1" cy="1"/><wp:docPr id="49" name="p"/></wp:inline></w:drawing></w:r>`
	inner := `<w:p><w:r><w:t>{{#each items}}</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>{{.}}</w:t></w:r>` + drawing + `</w:p>` +
		`<w:p><w:r><w:t>{{/each}}</w:t></w:r></w:p>`
	var body wml.CT_Body
	src := `<w:body ` + testNS + ` xmlns:wp="` + ns.WP + `">` + inner + `</w:body>`
	if err := xml.Unmarshal([]byte(src), &body); err != nil {
		t.Fatal(err)
	}
	opts := Options{UsedDocPr: []int64{50}}
	data := map[string]any{"items": []any{"a", "b", "c"}}
	if _, err := Execute([]Story{{Part: "word/document.xml", Blocks: &body.Children}}, data, opts); err != nil {
		t.Fatal(err)
	}
	scan := new(wml.IDScan)
	wml.ScanBlocks(body.Children, scan)
	if len(scan.DocPr) != 3 {
		t.Fatalf("%d drawings, want 3: %v", len(scan.DocPr), scan.DocPr)
	}
	seen := map[int64]bool{}
	for _, id := range scan.DocPr {
		if id == 50 || seen[id] {
			t.Errorf("drawing ids %v reuse 50 or repeat", scan.DocPr)
		}
		seen[id] = true
	}
}

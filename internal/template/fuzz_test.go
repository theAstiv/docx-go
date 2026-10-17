package template

import (
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// FuzzTemplateScan feeds arbitrary paragraph content to the normalizer.
// Whatever the input, normalizing must not panic, must not change the
// paragraph's text, and must leave a paragraph that marshals and parses.
func FuzzTemplateScan(f *testing.F) {
	seeds := []string{
		`<w:r><w:t>{{a}}</w:t></w:r>`,
		`<w:r><w:rPr><w:b/></w:rPr><w:t>{{cli</w:t></w:r><w:proofErr w:type="spellStart"/><w:r><w:t>ent}}</w:t></w:r>`,
		`<w:r><w:t>{{na</w:t></w:r><w:bookmarkStart w:id="0" w:name="_GoBack"/><w:bookmarkEnd w:id="0"/><w:r><w:t>me}}</w:t></w:r>`,
		`<w:hyperlink w:anchor="x"><w:r><w:t>{{l</w:t></w:r><w:r><w:t>}}</w:t></w:r></w:hyperlink><w:r><w:t>}}{{</w:t></w:r>`,
		`<w:r><w:t>{{#each x}}</w:t><w:tab/><w:t>{{/each}}</w:t></w:r>`,
		`<w:r><w:t>{{</w:t></w:r><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:t>}}</w:t></w:r>`,
		`<w:r><w:t>{{{{}}}}</w:t></w:r><w:r><w:lastRenderedPageBreak/><w:t>}}</w:t></w:r>`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, inner string) {
		var p wml.CT_P
		if err := xml.Unmarshal([]byte(`<w:p `+testNS+`>`+inner+`</w:p>`), &p); err != nil {
			return // not a paragraph; nothing to test
		}
		// Skip input the element layer cannot round-trip by itself (e.g. a
		// prefixed name like A:00 that is written back as the invalid <00>);
		// an unchanged save would fail the same way, so it is not the
		// normalizer's concern.
		orig, err := xml.Marshal(&p)
		if err != nil {
			return
		}
		var probe wml.CT_P
		if xml.Unmarshal(orig, &probe) != nil {
			return
		}
		before := p.Text()
		p.Children, _ = normalizeInlines(p.Children)
		if after := p.Text(); after != before {
			t.Fatalf("text changed: %q -> %q", before, after)
		}
		out, err := xml.Marshal(&p)
		if err != nil {
			t.Fatalf("marshal after normalize: %v", err)
		}
		var again wml.CT_P
		if err := xml.Unmarshal(out, &again); err != nil {
			t.Fatalf("reparse after normalize: %v\n%s", err, out)
		}
	})
}

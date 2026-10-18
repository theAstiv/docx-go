package template

import (
	"bytes"
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// fuzzData is the data FuzzTemplateExecute fills templates from.
var fuzzData = map[string]any{
	"a":    "x",
	"n":    3.0,
	"t":    true,
	"text": "line\nbreak\tand tab",
	"obj":  map[string]any{"k": "v", "list": []any{"p", "q"}},
	"rows": []any{
		map[string]any{"k": "1", "on": true, "sub": []any{"s"}},
		map[string]any{"k": "2", "on": false, "sub": []any{}},
	},
}

// FuzzTemplateExecute runs arbitrary body content through the whole engine.
// It must not panic; on success the result must marshal and parse again;
// on failure the content must be unchanged (Execute is atomic).
func FuzzTemplateExecute(f *testing.F) {
	seeds := []string{
		`<w:p><w:r><w:t>{{a}} {{obj.k}}</w:t></w:r></w:p>`,
		`<w:p><w:r><w:t>{{#each rows}}</w:t></w:r></w:p><w:p><w:r><w:t>{{.k}}</w:t></w:r></w:p><w:p><w:r><w:t>{{/each}}</w:t></w:r></w:p>`,
		`<w:tbl><w:tblPr/><w:tblGrid/><w:tr><w:tc><w:p><w:r><w:t>{{#each rows}}{{.k}}</w:t></w:r></w:p></w:tc>` +
			`<w:tc><w:p><w:r><w:t>{{#if .on}}on{{/if}}{{/each}}</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`,
		`<w:p><w:r><w:t>{{#if t}}</w:t></w:r></w:p><w:p><w:r><w:t>{{text}}</w:t></w:r></w:p><w:p><w:r><w:t>{{/if}}</w:t></w:r></w:p>`,
		`<w:sdt><w:sdtContent><w:p><w:r><w:t>{{a}}{{</w:t></w:r></w:p></w:sdtContent></w:sdt>`,
		`<w:p><w:r><w:t>{{#each obj.list}}</w:t></w:r></w:p><w:p><w:r><w:t>{{.}}</w:t></w:r></w:p>`,
	}
	for _, s := range seeds {
		f.Add(s, false)
	}
	f.Fuzz(func(t *testing.T, inner string, lenient bool) {
		src := []byte(`<w:body ` + testNS + `>` + inner + `</w:body>`)
		var body wml.CT_Body
		if err := xml.Unmarshal(src, &body); err != nil {
			return
		}
		before, err := xml.Marshal(&body)
		if err != nil {
			return
		}
		var probe wml.CT_Body
		if xml.Unmarshal(before, &probe) != nil {
			return // the element layer cannot round-trip this input by itself
		}
		mode := Strict
		if lenient {
			mode = Lenient
		}
		_, execErr := Execute([]Story{{Part: "word/document.xml", Blocks: &body.Children}}, fuzzData, Options{Mode: mode})
		after, err := xml.Marshal(&body)
		if err != nil {
			t.Fatalf("marshal after Execute: %v", err)
		}
		if execErr != nil {
			if !bytes.Equal(before, after) {
				t.Fatalf("failed Execute changed the content: %v", execErr)
			}
			return
		}
		var again wml.CT_Body
		if err := xml.Unmarshal(after, &again); err != nil {
			t.Fatalf("reparse after Execute: %v\n%s", err, after)
		}
	})
}

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
		p.Children, _ = normalizeInlines(p.Children, nil)
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

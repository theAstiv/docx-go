package wml_test

import (
	"bytes"
	"encoding/xml"
	"strconv"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const cloneNS = `xmlns:w="` + ns.W + `" xmlns:w14="` + ns.W14 + `" xmlns:mc="` + ns.MC + `"`

// marshalOne returns the encoder output for el.
func marshalOne(t *testing.T, el wml.Cloneable) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := el.MarshalXML(enc, xml.StartElement{}); err != nil {
		t.Fatal(err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// bodyChildren parses src as the content of a w:body.
func bodyChildren(t *testing.T, src string) []wml.BlockElement {
	t.Helper()
	var b wml.CT_Body
	if err := xml.Unmarshal([]byte(`<w:body `+cloneNS+`>`+src+`</w:body>`), &b); err != nil {
		t.Fatal(err)
	}
	return b.Children
}

func TestClone_BlockKinds(t *testing.T) {
	cases := map[string]string{
		"paragraph": `<w:p w14:paraId="1A2B3C4D" w:rsidR="00467858"><w:pPr><w:pStyle w:val="Heading1"/></w:pPr>` +
			`<w:r w:rsidRPr="00467858"><w:rPr><w:b/></w:rPr><w:t xml:space="preserve"> a &amp; b </w:t><w:tab/></w:r>` +
			`<w:proofErr w:type="spellStart"/><w:bookmarkStart w:id="0" w:name="x"/><w:bookmarkEnd w:id="0"/>` +
			`<w:hyperlink w:anchor="x"><w:r><w:t>link</w:t></w:r></w:hyperlink></w:p>`,
		"table": `<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/></w:tblPr><w:tblGrid><w:gridCol w:w="2000"/></w:tblGrid>` +
			`<w:tr w14:paraId="00000001" w:rsidR="00AB"><w:trPr><w:tblHeader/></w:trPr>` +
			`<w:tc><w:tcPr><w:tcW w:w="2000" w:type="dxa"/></w:tcPr><w:p><w:r><w:t>cell</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`,
		"content control": `<w:sdt><w:sdtPr><w:alias w:val="Cover"/></w:sdtPr><w:sdtContent>` +
			`<w:p><w:r><w:t>{{client.name}}</w:t></w:r></w:p></w:sdtContent></w:sdt>`,
		"alternate content": `<w:p><w:r><mc:AlternateContent><mc:Choice Requires="w14"><w:t>new</w:t></mc:Choice>` +
			`<mc:Fallback><w:t>old</w:t></mc:Fallback></mc:AlternateContent></w:r></w:p>`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			children := bodyChildren(t, src)
			if len(children) != 1 {
				t.Fatalf("parsed %d children, want 1", len(children))
			}
			orig := children[0]
			cp, err := wml.Clone(orig)
			if err != nil {
				t.Fatal(err)
			}
			if cp == orig {
				t.Fatal("Clone returned the same pointer")
			}
			if want, got := marshalOne(t, orig), marshalOne(t, cp); !bytes.Equal(got, want) {
				t.Errorf("clone marshals differently\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestClone_Independent(t *testing.T) {
	orig, ok := bodyChildren(t, `<w:p w:rsidR="00AB"><w:r><w:t>before</w:t></w:r></w:p>`)[0].(*wml.CT_P)
	if !ok {
		t.Fatal("not a paragraph")
	}
	cp, err := wml.Clone(orig)
	if err != nil {
		t.Fatal(err)
	}
	cp.Attrs[0].Value = "FFFF"
	cpRun, _ := cp.Children[0].(*wml.CT_R)
	cpText, _ := cpRun.Children[0].(*wml.CT_Text)
	cpText.Content = "after"
	cp.Children = append(cp.Children, &wml.CT_R{})

	if got := orig.Text(); got != "before" {
		t.Errorf("original text changed to %q", got)
	}
	if orig.Attrs[0].Value != "00AB" {
		t.Errorf("original attribute changed to %q", orig.Attrs[0].Value)
	}
	if len(orig.Children) != 1 {
		t.Errorf("original has %d children, want 1", len(orig.Children))
	}
}

func TestClone_Row(t *testing.T) {
	tbl, ok := bodyChildren(t, `<w:tbl><w:tblPr/><w:tblGrid><w:gridCol w:w="1"/></w:tblGrid>`+
		`<w:tr><w:tblPrEx><w:tblBorders/></w:tblPrEx><w:trPr><w:cantSplit/></w:trPr><w:tc><w:p/></w:tc></w:tr></w:tbl>`)[0].(*wml.CT_Tbl)
	if !ok {
		t.Fatal("not a table")
	}
	row := tbl.Children[0]
	cp, err := wml.Clone(row)
	if err != nil {
		t.Fatal(err)
	}
	if want, got := marshalOne(t, row), marshalOne(t, cp); !bytes.Equal(got, want) {
		t.Errorf("row clone marshals differently\n got: %s\nwant: %s", got, want)
	}
}

func TestCloneWith_Fixups(t *testing.T) {
	// A row holding everything a copy must not duplicate, in typed content
	// and inside a content control (raw), plus a footnote reference.
	src := `<w:tbl><w:tblPr/><w:tblGrid><w:gridCol w:w="1"/></w:tblGrid>` +
		`<w:tr w14:paraId="00000001" w:rsidR="00AB"><w:tc>` +
		`<w:p w14:paraId="00000002" w14:textId="77777777" w:rsidR="00AB">` +
		`<w:bookmarkStart w:id="3" w:name="row"/><w:commentRangeStart w:id="0"/>` +
		`<w:r><w:t>typed</w:t></w:r><w:commentRangeEnd w:id="0"/>` +
		`<w:r><w:commentReference w:id="0"/></w:r><w:bookmarkEnd w:id="3"/>` +
		`<w:r><w:footnoteReference w:id="1"/></w:r>` +
		`<w:r><w:drawing><wp:inline xmlns:wp="` + ns.WP + `"><wp:extent cx="1" cy="1"/><wp:docPr id="7" name="Picture 7"/></wp:inline></w:drawing></w:r></w:p>` +
		`<w:sdt><w:sdtContent><w:p w14:paraId="00000003"><w:bookmarkStart w:id="4" w:name="inner"/>` +
		`<w:r><w:t>raw</w:t></w:r><w:bookmarkEnd w:id="4"/></w:p></w:sdtContent></w:sdt>` +
		`</w:tc></w:tr></w:tbl>`
	tbl, ok := bodyChildren(t, src)[0].(*wml.CT_Tbl)
	if !ok {
		t.Fatal("not a table")
	}
	ids := (&wml.IDScan{DocPr: []int64{7, 41}}).DocPrAllocator()
	fx := &wml.CloneFixup{StripParaIDs: true, DropBookmarks: true, DropCommentAnchors: true, NextDocPrID: ids}
	cp, err := wml.CloneWith(tbl.Children[0], fx)
	if err != nil {
		t.Fatal(err)
	}
	out := string(marshalOne(t, cp))

	for _, gone := range []string{"paraId", "textId", "bookmarkStart", "bookmarkEnd", "commentRange", "commentReference"} {
		if strings.Contains(out, gone) {
			t.Errorf("copy still contains %s:\n%s", gone, out)
		}
	}
	for _, kept := range []string{"rsidR", ">typed<", ">raw<", "footnoteReference", `id="42"`} {
		if !strings.Contains(out, kept) {
			t.Errorf("copy lost %s:\n%s", kept, out)
		}
	}
	if fx.DroppedComments != 1 || fx.NoteRefs != 1 {
		t.Errorf("DroppedComments=%d NoteRefs=%d, want 1 and 1", fx.DroppedComments, fx.NoteRefs)
	}
	if next := ids(); next != 43 {
		t.Errorf("allocator continued at %d, want 43", next)
	}

	// The original row is untouched.
	orig := string(marshalOne(t, tbl.Children[0]))
	for _, kept := range []string{"paraId", "bookmarkStart", "commentReference", `id="7"`} {
		if !strings.Contains(orig, kept) {
			t.Errorf("original lost %s", kept)
		}
	}
}

func TestClone_NilPointer(t *testing.T) {
	var p *wml.CT_P
	if _, err := wml.Clone(p); err == nil {
		t.Error("Clone(nil) returned no error")
	}
}

// BenchmarkClone_Table50 measures the clone-by-round-trip cost a 50-item
// row loop pays: one table row cloned 50 times.
func BenchmarkClone_Table50(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`<w:body ` + cloneNS + `><w:tbl><w:tblPr/><w:tblGrid><w:gridCol w:w="1"/><w:gridCol w:w="1"/><w:gridCol w:w="1"/></w:tblGrid>`)
	sb.WriteString(`<w:tr w14:paraId="00000001" w:rsidR="00AB">`)
	for c := 0; c < 3; c++ {
		sb.WriteString(`<w:tc><w:tcPr><w:tcW w:w="2000" w:type="dxa"/></w:tcPr><w:p w:rsidR="00AB"><w:r w:rsidRPr="00AB"><w:rPr><w:b/><w:sz w:val="20"/></w:rPr><w:t>{{.field` + strconv.Itoa(c) + `}}</w:t></w:r></w:p></w:tc>`)
	}
	sb.WriteString(`</w:tr></w:tbl></w:body>`)
	var body wml.CT_Body
	if err := xml.Unmarshal([]byte(sb.String()), &body); err != nil {
		b.Fatal(err)
	}
	tbl, _ := body.Children[0].(*wml.CT_Tbl)
	row := tbl.Children[0]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 50; j++ {
			if _, err := wml.Clone(row); err != nil {
				b.Fatal(err)
			}
		}
	}
}

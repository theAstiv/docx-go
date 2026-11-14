package textmerge

import (
	"encoding/xml"
	"slices"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const testNS = `xmlns:w="` + ns.W + `"`

func parseP(t testing.TB, inner string) *wml.CT_P {
	t.Helper()
	var p wml.CT_P
	if err := xml.Unmarshal([]byte(`<w:p `+testNS+`>`+inner+`</w:p>`), &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

// runTexts renders each run's w:t contents as [a|b], other inlines by name.
func runTexts(children []wml.InlineElement) string {
	var parts []string
	for _, c := range children {
		switch el := c.(type) {
		case *wml.CT_R:
			var texts []string
			for _, rc := range el.Children {
				if tx, ok := rc.(*wml.CT_Text); ok {
					texts = append(texts, tx.Content)
				} else {
					texts = append(texts, "<>")
				}
			}
			parts = append(parts, "["+strings.Join(texts, "|")+"]")
		case *wml.CT_BookmarkStart:
			parts = append(parts, "bs:"+el.Name)
		case *wml.CT_BookmarkEnd:
			parts = append(parts, "be")
		case *wml.RawInline:
			parts = append(parts, el.E.QName.Local)
		case *wml.CT_Hyperlink:
			parts = append(parts, "H")
		}
	}
	return strings.Join(parts, " ")
}

func TestFlatten_Kinds(t *testing.T) {
	p := parseP(t, `<w:r><w:t>ab</w:t><w:tab/><w:t>c</w:t></w:r>`+
		`<w:proofErr w:type="spellStart"/>`+
		`<w:bookmarkStart w:id="0" w:name="_GoBack"/><w:bookmarkEnd w:id="0"/>`+
		`<w:bookmarkStart w:id="1" w:name="sec"/>`+
		`<w:r><w:lastRenderedPageBreak/><w:fldChar w:fldCharType="begin"/></w:r>`+
		`<w:hyperlink w:anchor="x"><w:r><w:t>in link</w:t></w:r></w:hyperlink>`)
	flat, pieces := Flatten(p.Children, nil)
	want := "ab" + string(BarrierBreak) + "c" + string(BarrierBoundary) + string(BarrierBoundary) + string(BarrierBoundary)
	if flat != want {
		t.Fatalf("flat = %q, want %q", flat, want)
	}
	kinds := make([]PieceKind, len(pieces))
	for i, pc := range pieces {
		kinds[i] = pc.Kind
	}
	wantKinds := []PieceKind{
		PieceText, PieceBarrier, PieceText, // run 0
		PieceDrop,            // proofErr
		PieceMove, PieceMove, // _GoBack start and end
		PieceBarrier,            // other bookmark
		PieceDrop, PieceBarrier, // lastRenderedPageBreak, fldChar
		PieceBarrier, // hyperlink
	}
	if len(kinds) != len(wantKinds) {
		t.Fatalf("kinds = %v, want %v", kinds, wantKinds)
	}
	for i := range kinds {
		if kinds[i] != wantKinds[i] {
			t.Errorf("piece %d kind = %v, want %v", i, kinds[i], wantKinds[i])
		}
	}
	// Offsets tile the flattened string.
	end := 0
	for _, pc := range pieces {
		if pc.Start != end {
			t.Errorf("piece %+v starts at %d, want %d", pc, pc.Start, end)
		}
		end = pc.End
	}
	if end != len(flat) {
		t.Errorf("pieces end at %d, want %d", end, len(flat))
	}
}

func TestFlatten_SkipBecomesBoundary(t *testing.T) {
	p := parseP(t, `<w:r><w:t>keep</w:t><w:t>skip</w:t></w:r>`)
	r, _ := p.Children[0].(*wml.CT_R)
	skipped, _ := r.Children[1].(*wml.CT_Text)
	flat, _ := Flatten(p.Children, SkipSet{skipped: true})
	if flat != "keep"+string(BarrierBoundary) {
		t.Errorf("flat = %q", flat)
	}
}

func TestCollapse(t *testing.T) {
	cases := []struct {
		name, in, find, want string
	}{
		{"already in one w:t",
			`<w:r><w:t>say hello</w:t></w:r>`, "hello",
			`[say hello]`},
		{"across three runs; first run keeps the text",
			`<w:r><w:t>he</w:t></w:r><w:r><w:t>l</w:t></w:r><w:r><w:t>lo world</w:t></w:r>`, "hello",
			`[hello] [ world]`},
		{"prefix and suffix stay where they were",
			`<w:r><w:t>say he</w:t></w:r><w:r><w:t>llo now</w:t></w:r>`, "hello",
			`[say hello] [ now]`},
		{"spell-check marks inside are dropped, outside kept",
			`<w:proofErr w:type="spellStart"/><w:r><w:t>he</w:t></w:r><w:proofErr w:type="spellEnd"/><w:r><w:t>llo</w:t></w:r>`, "hello",
			`proofErr [hello]`},
		{"_GoBack moves before the run holding the range",
			`<w:r><w:t>x he</w:t></w:r><w:bookmarkStart w:id="0" w:name="_GoBack"/><w:bookmarkEnd w:id="0"/><w:r><w:t>llo</w:t></w:r>`, "hello",
			`bs:_GoBack be [x hello]`},
		{"lastRenderedPageBreak inside is dropped",
			`<w:r><w:t>he</w:t></w:r><w:r><w:lastRenderedPageBreak/><w:t>llo</w:t></w:r>`, "hello",
			`[hello]`},
		{"several w:t in one run",
			`<w:r><w:t>he</w:t><w:t>llo</w:t></w:r>`, "hello",
			`[hello]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := parseP(t, c.in)
			before := p.Text()
			flat, pieces := Flatten(p.Children, nil)
			start := strings.Index(flat, c.find)
			if start < 0 {
				t.Fatalf("%q not in %q", c.find, flat)
			}
			var ok bool
			p.Children, ok = Collapse(p.Children, pieces, flat, start, start+len(c.find))
			if !ok {
				t.Fatal("Collapse refused")
			}
			if got := runTexts(p.Children); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
			if after := p.Text(); after != before {
				t.Errorf("text changed: %q -> %q", before, after)
			}
		})
	}
}

func TestCollapse_RefusesBarriers(t *testing.T) {
	cases := []struct{ name, in string }{
		{"tab", `<w:r><w:t>he</w:t><w:tab/><w:t>llo</w:t></w:r>`},
		{"field", `<w:r><w:t>he</w:t></w:r><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:t>llo</w:t></w:r>`},
		{"bookmark", `<w:r><w:t>he</w:t></w:r><w:bookmarkStart w:id="1" w:name="b"/><w:r><w:t>llo</w:t></w:r>`},
		{"tracked change", `<w:r><w:t>he</w:t></w:r><w:ins w:id="1" w:author="a"><w:r><w:t>l</w:t></w:r></w:ins><w:r><w:t>lo</w:t></w:r>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := parseP(t, c.in)
			want := runTexts(p.Children)
			flat, pieces := Flatten(p.Children, nil)
			out, ok := Collapse(p.Children, pieces, flat, 0, len(flat))
			if ok {
				t.Fatal("Collapse accepted a range with a barrier")
			}
			if got := runTexts(out); got != want {
				t.Errorf("children changed:\n got  %s\nwant %s", got, want)
			}
		})
	}
}

func TestPreserveSpace(t *testing.T) {
	for _, c := range []struct {
		in, want string
	}{{"a", ""}, {" a", "preserve"}, {"a ", "preserve"}, {"", ""}} {
		tx := &wml.CT_Text{Content: c.in}
		PreserveSpace(tx)
		if tx.Space != c.want {
			t.Errorf("%q: Space = %q, want %q", c.in, tx.Space, c.want)
		}
	}
}

func TestTOCSkips_SpansParagraphs(t *testing.T) {
	var body wml.CT_Body
	src := `<w:body ` + testNS + `>` +
		`<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> TOC \o "1-3" </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>Intro</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>Scope</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>` +
		`<w:p><w:r><w:t>Body</w:t></w:r></w:p>` +
		`<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText>PAGE</w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>1</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>` +
		`</w:body>`
	if err := xml.Unmarshal([]byte(src), &body); err != nil {
		t.Fatal(err)
	}
	var got []string
	for tx := range TOCSkips(body.Children) {
		got = append(got, tx.Content)
	}
	if len(got) != 2 || !slices.Contains(got, "Intro") || !slices.Contains(got, "Scope") {
		t.Errorf("skipped %v, want Intro and Scope only", got)
	}
}

func TestIsTOCInstr(t *testing.T) {
	for in, want := range map[string]bool{
		` TOC \o "1-3" \h`: true, "toc": true, "INDEX \\c 2": true, "TOA \\c 1": true,
		"PAGE": false, "TOCX": false, "": false, " REF _Toc1 ": false,
	} {
		if got := IsTOCInstr(in); got != want {
			t.Errorf("IsTOCInstr(%q) = %v, want %v", in, got, want)
		}
	}
}

// FuzzCollapse checks that collapsing any barrier-free range keeps the
// paragraph's text, and leaves the range inside one w:t.
func FuzzCollapse(f *testing.F) {
	f.Add(`<w:r><w:t>he</w:t></w:r><w:r><w:t>llo</w:t></w:r>`, 0, 5)
	f.Add(`<w:proofErr/><w:r><w:t>a b</w:t></w:r><w:bookmarkStart w:id="0" w:name="_GoBack"/><w:bookmarkEnd w:id="0"/><w:r><w:t> c</w:t></w:r>`, 1, 4)
	f.Add(`<w:r><w:t>x</w:t><w:tab/><w:t>y</w:t></w:r>`, 0, 3)
	f.Fuzz(func(t *testing.T, inner string, start, end int) {
		var p wml.CT_P
		if err := xml.Unmarshal([]byte(`<w:p `+testNS+`>`+inner+`</w:p>`), &p); err != nil {
			return
		}
		flat, pieces := Flatten(p.Children, nil)
		if start < 0 || end > len(flat) || start >= end {
			return
		}
		before := p.Text()
		out, ok := Collapse(p.Children, pieces, flat, start, end)
		p.Children = out
		if after := p.Text(); after != before {
			t.Fatalf("text changed: %q -> %q", before, after)
		}
		if !ok {
			return
		}
		flat2, pieces2 := Flatten(p.Children, nil)
		if flat2 != flat {
			t.Fatalf("flattened text changed: %q -> %q", flat, flat2)
		}
		holders := 0
		for _, pc := range pieces2 {
			if pc.Kind == PieceText && pc.Start < end && pc.End > start {
				holders++
			}
		}
		if holders > 1 {
			t.Fatalf("range [%d,%d) still spans %d w:t in %q", start, end, holders, flat)
		}
	})
}

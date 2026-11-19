package textmerge

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// deleteBetween deletes the flattened text from the first "<" to the first
// ">" inclusive and returns the result and whether it was allowed.
func deleteBetween(t *testing.T, inner string) (string, []wml.InlineElement, bool) {
	t.Helper()
	p := parseP(t, inner)
	flat, pieces := Flatten(p.Children, nil)
	start, end := strings.Index(flat, "<"), strings.Index(flat, ">")+1
	if start < 0 || end <= start {
		t.Fatalf("no <…> in %q", flat)
	}
	can := CanDelete(p.Children, pieces, start, end)
	out, ok := DeleteRange(p.Children, pieces, start, end)
	if can != ok {
		t.Errorf("CanDelete = %v, DeleteRange = %v", can, ok)
	}
	text, _ := Flatten(out, nil)
	return text, out, ok
}

func TestDeleteRange(t *testing.T) {
	cases := []struct {
		name, inner string
		want        string // flattened text after deletion
		runs        string // runTexts after deletion; "" to skip
	}{
		{"one run", `<w:r><w:t>ab&lt;cd&gt;ef</w:t></w:r>`, "abef", "[abef]"},
		{"across runs, middle run dropped",
			`<w:r><w:t>a&lt;b</w:t></w:r><w:r><w:rPr><w:b/></w:rPr><w:t>mid</w:t></w:r><w:r><w:t>c&gt;d</w:t></w:r>`,
			"ad", "[a] [d]"},
		{"tab, break, symbol and hyphens go",
			`<w:r><w:t>a&lt;</w:t><w:tab/><w:br/><w:sym w:font="Wingdings" w:char="F0E0"/><w:noBreakHyphen/><w:softHyphen/><w:cr/><w:t>&gt;b</w:t></w:r>`,
			"ab", "[a|b]"},
		{"drawing goes",
			`<w:r><w:t>a&lt;</w:t></w:r><w:r><w:drawing><wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"/></w:drawing></w:r><w:r><w:t>&gt;b</w:t></w:r>`,
			"ab", "[a] [b]"},
		{"spell-check mark inside goes",
			`<w:r><w:t>a&lt;x</w:t></w:r><w:proofErr w:type="spellStart"/><w:r><w:t>y&gt;b</w:t></w:r>`,
			"ab", "[a] [b]"},
		{"spell-check mark at the edge stays",
			`<w:r><w:t>a</w:t></w:r><w:proofErr w:type="spellStart"/><w:r><w:t>&lt;y&gt;b</w:t></w:r>`,
			"ab", "[a] proofErr [b]"},
		{"bookmark pair inside goes",
			`<w:r><w:t>a&lt;</w:t></w:r><w:bookmarkStart w:id="3" w:name="x"/><w:r><w:t>in</w:t></w:r><w:bookmarkEnd w:id="3"/><w:r><w:t>&gt;b</w:t></w:r>`,
			"ab", "[a] [b]"},
		{"_GoBack stays",
			`<w:r><w:t>a&lt;</w:t></w:r><w:bookmarkStart w:id="0" w:name="_GoBack"/><w:bookmarkEnd w:id="0"/><w:r><w:t>&gt;b</w:t></w:r>`,
			"ab", "[a] bs:_GoBack be [b]"},
		{"run with other content keeps it",
			`<w:r><w:t>a&lt;x&gt;</w:t><w:tab/><w:t>b</w:t></w:r>`,
			"a" + string(BarrierBreak) + "b", "[a|<>|b]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text, out, ok := deleteBetween(t, c.inner)
			if !ok {
				t.Fatal("refused")
			}
			if text != c.want {
				t.Errorf("text = %q, want %q", text, c.want)
			}
			if c.runs != "" {
				if got := runTexts(out); got != c.runs {
					t.Errorf("runs = %s, want %s", got, c.runs)
				}
			}
		})
	}
}

func TestDeleteRange_Refused(t *testing.T) {
	cases := []struct{ name, inner string }{
		{"field character", `<w:r><w:t>a&lt;</w:t></w:r><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:t>&gt;b</w:t></w:r>`},
		{"field instruction", `<w:r><w:t>a&lt;</w:t><w:instrText>PAGE</w:instrText><w:t>&gt;b</w:t></w:r>`},
		{"hyperlink", `<w:r><w:t>a&lt;</w:t></w:r><w:hyperlink w:anchor="x"><w:r><w:t>l</w:t></w:r></w:hyperlink><w:r><w:t>&gt;b</w:t></w:r>`},
		{"content control", `<w:r><w:t>a&lt;</w:t></w:r><w:sdt><w:sdtContent><w:r><w:t>s</w:t></w:r></w:sdtContent></w:sdt><w:r><w:t>&gt;b</w:t></w:r>`},
		{"tracked insertion", `<w:r><w:t>a&lt;</w:t></w:r><w:ins w:id="1" w:author="x"><w:r><w:t>i</w:t></w:r></w:ins><w:r><w:t>&gt;b</w:t></w:r>`},
		{"bookmark start only", `<w:r><w:t>a&lt;</w:t></w:r><w:bookmarkStart w:id="3" w:name="x"/><w:r><w:t>&gt;b</w:t></w:r><w:bookmarkEnd w:id="3"/>`},
		{"bookmark end only", `<w:bookmarkStart w:id="3" w:name="x"/><w:r><w:t>a&lt;</w:t></w:r><w:bookmarkEnd w:id="3"/><w:r><w:t>&gt;b</w:t></w:r>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := parseP(t, c.inner)
			before := runTexts(p.Children)
			_, out, ok := deleteBetween(t, c.inner)
			if ok {
				t.Fatal("allowed, want refused")
			}
			if got := runTexts(out); got != before {
				t.Errorf("children changed: %s, want %s", got, before)
			}
		})
	}
}

// Text Flatten treats as skipped (a table of contents) cannot be deleted.
func TestDeleteRange_SkippedText(t *testing.T) {
	p := parseP(t, `<w:r><w:t>a&lt;</w:t><w:t>toc</w:t><w:t>&gt;b</w:t></w:r>`)
	run := p.Children[0].(*wml.CT_R)
	skip := SkipSet{run.Children[1].(*wml.CT_Text): true}
	flat, pieces := Flatten(p.Children, skip)
	start, end := strings.Index(flat, "<"), strings.Index(flat, ">")+1
	if _, ok := DeleteRange(p.Children, pieces, start, end); ok {
		t.Error("deleted skipped text")
	}
}

// Deleting right to left keeps earlier offsets valid.
func TestDeleteRange_RightToLeft(t *testing.T) {
	p := parseP(t, `<w:r><w:t>keep1 </w:t></w:r><w:r><w:t>DEL1 keep2 </w:t></w:r><w:r><w:t>DEL2 keep3</w:t></w:r>`)
	flat, _ := Flatten(p.Children, nil)
	children := p.Children
	for _, word := range []string{"DEL2 ", "DEL1 "} {
		i := strings.Index(flat, word)
		var pieces []Piece
		_, pieces = Flatten(children, nil)
		var ok bool
		children, ok = DeleteRange(children, pieces, i, i+len(word))
		if !ok {
			t.Fatalf("refused %q", word)
		}
	}
	if got, _ := Flatten(children, nil); got != "keep1 keep2 keep3" {
		t.Errorf("got %q", got)
	}
}

func FuzzDeleteRange(f *testing.F) {
	f.Add(`<w:r><w:t>ab</w:t><w:tab/><w:t>cd</w:t></w:r><w:proofErr w:type="spellStart"/><w:r><w:t>ef</w:t></w:r>`, 1, 5)
	f.Add(`<w:r><w:t>a</w:t></w:r><w:bookmarkStart w:id="1" w:name="x"/><w:r><w:t>b</w:t></w:r><w:bookmarkEnd w:id="1"/>`, 0, 3)
	f.Add(`<w:r><w:t>a</w:t><w:fldChar w:fldCharType="begin"/><w:t>b</w:t></w:r>`, 0, 3)
	f.Fuzz(func(t *testing.T, inner string, start, end int) {
		var p wml.CT_P
		if err := xml.Unmarshal([]byte(`<w:p `+testNS+`>`+inner+`</w:p>`), &p); err != nil {
			return
		}
		flat, pieces := Flatten(p.Children, nil)
		if start < 0 || end > len(flat) || start > end {
			return
		}
		out, ok := DeleteRange(p.Children, pieces, start, end)
		got, _ := Flatten(out, nil)
		if !ok {
			if got != flat {
				t.Fatalf("refused deletion changed the text: %q → %q", flat, got)
			}
			return
		}
		if want := flat[:start] + flat[end:]; got != want {
			t.Fatalf("text after deleting [%d,%d) of %q = %q, want %q", start, end, flat, got, want)
		}
	})
}

package wml_test

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

var pName = ns.Qn("w:p")

func TestCT_P_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_P](t, &wml.CT_P{}, pName)
}

func TestCT_P_RoundTrip_WithText(t *testing.T) {
	p := &wml.CT_P{}
	r := &wml.CT_R{}
	r.Children = append(r.Children, &wml.CT_Text{Content: "hello", Space: "preserve"})
	p.Children = append(p.Children, r)
	roundtrip.Verify[*wml.CT_P](t, p, pName)
}

func TestCT_P_RoundTrip_WithPPr(t *testing.T) {
	p := &wml.CT_P{}
	p.PPr = &wml.CT_PPr{}
	p.PPr.SetStyleID("Heading1")
	p.PPr.SetAlignment("center")
	r := &wml.CT_R{}
	r.Children = append(r.Children, &wml.CT_Text{Content: "Title"})
	p.Children = append(p.Children, r)
	roundtrip.Verify[*wml.CT_P](t, p, pName)
}

func TestCT_P_RoundTrip_WithBookmarks(t *testing.T) {
	p := &wml.CT_P{}
	p.Children = append(p.Children,
		&wml.CT_BookmarkStart{ID: 1, Name: "ref1"},
		&wml.CT_R{},
		&wml.CT_BookmarkEnd{ID: 1},
	)
	roundtrip.Verify[*wml.CT_P](t, p, pName)
}

func TestCT_P_RoundTrip_WithHyperlink(t *testing.T) {
	p := &wml.CT_P{}
	hl := &wml.CT_Hyperlink{
		RID:    "rId1",
		Anchor: "",
	}
	r := &wml.CT_R{}
	r.Children = append(r.Children, &wml.CT_Text{Content: "click"})
	hl.Children = append(hl.Children, r)
	p.Children = append(p.Children, hl)
	roundtrip.Verify[*wml.CT_P](t, p, pName)
}

func TestCT_P_Text_Simple(t *testing.T) {
	p := &wml.CT_P{}
	r := &wml.CT_R{}
	r.Children = append(r.Children, &wml.CT_Text{Content: "hello world"})
	p.Children = append(p.Children, r)
	if got := p.Text(); got != "hello world" {
		t.Errorf("Text() = %q, want %q", got, "hello world")
	}
}

func TestCT_P_Text_TabAndBreak(t *testing.T) {
	p := &wml.CT_P{}
	r := &wml.CT_R{}
	r.Children = append(r.Children,
		&wml.CT_Text{Content: "a"},
		&wml.CT_Tab{},
		&wml.CT_Text{Content: "b"},
		&wml.CT_Br{},
		&wml.CT_Text{Content: "c"},
	)
	p.Children = append(p.Children, r)
	if got := p.Text(); got != "a\tb\nc" {
		t.Errorf("Text() = %q, want %q", got, "a\tb\nc")
	}
}

func TestCT_P_Text_HyperlinkIncluded(t *testing.T) {
	p := &wml.CT_P{}
	hl := &wml.CT_Hyperlink{}
	r := &wml.CT_R{}
	r.Children = append(r.Children, &wml.CT_Text{Content: "link"})
	hl.Children = append(hl.Children, r)
	p.Children = append(p.Children, hl)
	if got := p.Text(); got != "link" {
		t.Errorf("Text() = %q, want %q", got, "link")
	}
}

func TestCT_P_Text_FieldResult(t *testing.T) {
	// Field: begin ... instrText ... separate ... result ... end
	p := &wml.CT_P{}
	r := &wml.CT_R{}
	r.Children = append(r.Children,
		&wml.CT_FldChar{FldCharType: "begin"},
		&wml.CT_InstrText{Content: " DATE "},
		&wml.CT_FldChar{FldCharType: "separate"},
	)
	p.Children = append(p.Children, r)
	r2 := &wml.CT_R{}
	r2.Children = append(r2.Children, &wml.CT_Text{Content: "1/1/2026"})
	p.Children = append(p.Children, r2)
	r3 := &wml.CT_R{}
	r3.Children = append(r3.Children, &wml.CT_FldChar{FldCharType: "end"})
	p.Children = append(p.Children, r3)

	if got := p.Text(); got != "1/1/2026" {
		t.Errorf("Text() = %q, want field result %q", got, "1/1/2026")
	}
}

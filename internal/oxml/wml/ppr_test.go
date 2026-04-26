package wml_test

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

var pprName = ns.Qn("w:pPr")

func TestCT_PPr_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_PPr](t, &wml.CT_PPr{}, pprName)
}

func TestCT_PPr_RoundTrip_StyleAndAlignment(t *testing.T) {
	p := &wml.CT_PPr{}
	p.SetStyleID("Heading1")
	p.SetAlignment("center")
	roundtrip.Verify[*wml.CT_PPr](t, p, pprName)
}

func TestCT_PPr_RoundTrip_SpacingAndIndent(t *testing.T) {
	p := &wml.CT_PPr{}
	before := int64(240)
	after := int64(120)
	line := int64(360)
	p.SetSpacingBefore(&before)
	p.SetSpacingAfter(&after)
	p.SetSpacingLine(&line)
	p.SetSpacingLineRule("auto")
	start := int64(720)
	p.SetIndStart(&start)
	roundtrip.Verify[*wml.CT_PPr](t, p, pprName)
}

func TestCT_PPr_RoundTrip_OnOffFlags(t *testing.T) {
	p := &wml.CT_PPr{}
	p.SetKeepNext(boolPtr(true))
	p.SetKeepLines(boolPtr(true))
	p.SetPageBreakBefore(boolPtr(false))
	p.SetContextualSpacing(boolPtr(true))
	roundtrip.Verify[*wml.CT_PPr](t, p, pprName)
}

func TestCT_PPr_StyleID_Accessors(t *testing.T) {
	p := &wml.CT_PPr{}
	if got := p.StyleID(); got != "" {
		t.Errorf("StyleID() on empty = %q, want empty", got)
	}
	p.SetStyleID("Normal")
	if got := p.StyleID(); got != "Normal" {
		t.Errorf("StyleID() = %q, want %q", got, "Normal")
	}
	p.SetStyleID("")
	if got := p.StyleID(); got != "" {
		t.Errorf("StyleID() after clear = %q, want empty", got)
	}
}

func TestCT_PPr_Spacing_Accessors(t *testing.T) {
	p := &wml.CT_PPr{}
	if p.SpacingBefore() != nil {
		t.Error("SpacingBefore() on empty should be nil")
	}
	v := int64(200)
	p.SetSpacingBefore(&v)
	if got := p.SpacingBefore(); got == nil || *got != 200 {
		t.Errorf("SpacingBefore() = %v, want 200", got)
	}
	p.SetSpacingBefore(nil)
	if p.SpacingBefore() != nil {
		t.Error("SpacingBefore() after nil set should be nil")
	}
}

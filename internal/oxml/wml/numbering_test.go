package wml_test

import (
	"bytes"
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

var (
	numName           = ns.Qn("w:num")
	abstractNumName   = ns.Qn("w:abstractNum")
	lvlName           = ns.Qn("w:lvl")
	lvlOverrideName   = ns.Qn("w:lvlOverride")
	numberingRootName = ns.Qn("w:numbering")
)

// ── CT_Lvl ────────────────────────────────────────────────────────────────────

func TestCT_Lvl_RoundTrip_Empty(t *testing.T) {
	l := &wml.CT_Lvl{}
	roundtrip.Verify[*wml.CT_Lvl](t, l, lvlName)
}

func TestCT_Lvl_RoundTrip_WithBody(t *testing.T) {
	// Build a CT_Lvl by round-tripping XML.
	src := []byte(`<w:lvl xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:lvlJc w:val="left"/></w:lvl>`)
	dec := xml.NewDecoder(bytes.NewReader(src))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	start := tok.(xml.StartElement)
	l := new(wml.CT_Lvl)
	if err := l.UnmarshalXML(dec, start); err != nil {
		t.Fatalf("UnmarshalXML: %v", err)
	}
	if l.ILvl != 0 {
		t.Errorf("ILvl = %d, want 0", l.ILvl)
	}
	if got := l.NumFmt(); got != "decimal" {
		t.Errorf("NumFmt() = %q, want %q", got, "decimal")
	}
	if got := l.LvlText(); got != "%1." {
		t.Errorf("LvlText() = %q, want %q", got, "%1.")
	}
	if got := l.Start(); got == nil || *got != 1 {
		t.Errorf("Start() = %v, want 1", got)
	}
	roundtrip.Verify[*wml.CT_Lvl](t, l, lvlName)
}

// ── CT_LvlOverride ────────────────────────────────────────────────────────────

func TestCT_LvlOverride_RoundTrip_Empty(t *testing.T) {
	o := &wml.CT_LvlOverride{ILvl: 0}
	roundtrip.Verify[*wml.CT_LvlOverride](t, o, lvlOverrideName)
}

func TestCT_LvlOverride_RoundTrip_WithStartOverride(t *testing.T) {
	n := int64(1)
	o := &wml.CT_LvlOverride{ILvl: 2, StartOverride: &n}
	roundtrip.Verify[*wml.CT_LvlOverride](t, o, lvlOverrideName)
}

func TestCT_LvlOverride_RoundTrip_WithLvl(t *testing.T) {
	n := int64(1)
	o := &wml.CT_LvlOverride{
		ILvl:          0,
		StartOverride: &n,
		Lvl:           &wml.CT_Lvl{ILvl: 0},
	}
	roundtrip.Verify[*wml.CT_LvlOverride](t, o, lvlOverrideName)
}

// ── CT_Num ────────────────────────────────────────────────────────────────────

func TestCT_Num_RoundTrip_Simple(t *testing.T) {
	n := &wml.CT_Num{NumID: 1, AbstractNumID: 0}
	roundtrip.Verify[*wml.CT_Num](t, n, numName)
}

func TestCT_Num_RoundTrip_WithOverride(t *testing.T) {
	start := int64(1)
	n := &wml.CT_Num{
		NumID:         2,
		AbstractNumID: 0,
		LvlOverrides: []*wml.CT_LvlOverride{
			{ILvl: 0, StartOverride: &start},
		},
	}
	roundtrip.Verify[*wml.CT_Num](t, n, numName)
}

// ── CT_AbstractNum ────────────────────────────────────────────────────────────

func TestCT_AbstractNum_RoundTrip_Empty(t *testing.T) {
	a := &wml.CT_AbstractNum{AbstractNumID: 0}
	roundtrip.Verify[*wml.CT_AbstractNum](t, a, abstractNumName)
}

func TestCT_AbstractNum_RoundTrip_WithLvls(t *testing.T) {
	a := &wml.CT_AbstractNum{
		AbstractNumID: 1,
		Lvls: []*wml.CT_Lvl{
			{ILvl: 0},
			{ILvl: 1},
		},
	}
	roundtrip.Verify[*wml.CT_AbstractNum](t, a, abstractNumName)
}

// ── CT_Numbering ─────────────────────────────────────────────────────────────

func TestCT_Numbering_RoundTrip_Empty(t *testing.T) {
	n := &wml.CT_Numbering{}
	roundtrip.Verify[*wml.CT_Numbering](t, n, numberingRootName)
}

func TestCT_Numbering_RoundTrip_Simple(t *testing.T) {
	a := &wml.CT_AbstractNum{
		AbstractNumID: 0,
		Lvls:          []*wml.CT_Lvl{{ILvl: 0}, {ILvl: 1}},
	}
	num := &wml.CT_Num{NumID: 1, AbstractNumID: 0}
	n := &wml.CT_Numbering{
		AbstractNums: []*wml.CT_AbstractNum{a},
		Nums:         []*wml.CT_Num{num},
	}
	roundtrip.Verify[*wml.CT_Numbering](t, n, numberingRootName)
}

func TestCT_Numbering_NextIDs(t *testing.T) {
	n := &wml.CT_Numbering{
		AbstractNums: []*wml.CT_AbstractNum{{AbstractNumID: 0}, {AbstractNumID: 2}},
		Nums:         []*wml.CT_Num{{NumID: 1}, {NumID: 3}},
	}
	if got := n.NextAbstractNumID(); got != 3 {
		t.Errorf("NextAbstractNumID() = %d, want 3", got)
	}
	if got := n.NextNumID(); got != 4 {
		t.Errorf("NextNumID() = %d, want 4", got)
	}
}

func TestCT_Numbering_RoundTrip_FromXML(t *testing.T) {
	// A minimal numbering.xml as Word would generate it.
	src := []byte(`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:abstractNum w:abstractNumId="0">` +
		`<w:multiLevelType w:val="hybridMultilevel"/>` +
		`<w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:lvlJc w:val="left"/></w:lvl>` +
		`</w:abstractNum>` +
		`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>` +
		`</w:numbering>`)

	dec := xml.NewDecoder(bytes.NewReader(src))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	start := tok.(xml.StartElement)
	n := new(wml.CT_Numbering)
	if err := n.UnmarshalXML(dec, start); err != nil {
		t.Fatalf("UnmarshalXML: %v", err)
	}

	if len(n.AbstractNums) != 1 {
		t.Fatalf("AbstractNums count = %d, want 1", len(n.AbstractNums))
	}
	if n.AbstractNums[0].AbstractNumID != 0 {
		t.Errorf("AbstractNumID = %d, want 0", n.AbstractNums[0].AbstractNumID)
	}
	if len(n.Nums) != 1 || n.Nums[0].NumID != 1 {
		t.Errorf("Nums = %v, want [{NumID:1}]", n.Nums)
	}
	if a := n.AbstractNumByID(0); a == nil {
		t.Error("AbstractNumByID(0) returned nil")
	}
	if num := n.NumByID(1); num == nil {
		t.Error("NumByID(1) returned nil")
	}
	if lvl := n.AbstractNums[0].LvlByIlvl(0); lvl == nil {
		t.Error("LvlByIlvl(0) returned nil")
	} else {
		if got := lvl.NumFmt(); got != "decimal" {
			t.Errorf("LvlByIlvl(0).NumFmt() = %q, want %q", got, "decimal")
		}
	}

	roundtrip.Verify[*wml.CT_Numbering](t, n, numberingRootName)
}

// ── SetNumPr / ClearNumPr on CT_PPr ──────────────────────────────────────────

func TestCT_PPr_SetNumPr(t *testing.T) {
	p := new(wml.CT_PPr)
	if p.NumID() != nil {
		t.Error("NumID() on fresh CT_PPr should be nil")
	}
	p.SetNumPr(3, 1)
	if got := p.NumID(); got == nil || *got != 3 {
		t.Errorf("NumID() = %v, want 3", got)
	}
	if got := p.NumILvl(); got == nil || *got != 1 {
		t.Errorf("NumILvl() = %v, want 1", got)
	}
}

func TestCT_PPr_SetNumPr_Overwrite(t *testing.T) {
	p := new(wml.CT_PPr)
	p.SetNumPr(1, 0)
	p.SetNumPr(2, 1)
	if got := p.NumID(); got == nil || *got != 2 {
		t.Errorf("NumID() after overwrite = %v, want 2", got)
	}
	if got := p.NumILvl(); got == nil || *got != 1 {
		t.Errorf("NumILvl() after overwrite = %v, want 1", got)
	}
}

func TestCT_PPr_ClearNumPr(t *testing.T) {
	p := new(wml.CT_PPr)
	p.SetNumPr(1, 0)
	p.ClearNumPr()
	if p.NumID() != nil {
		t.Error("NumID() after ClearNumPr should be nil")
	}
}

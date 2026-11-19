package wml_test

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Attributes and children Word writes on numbering and field characters
// must survive a round-trip (review 2026-11-19, P0 #4-#6). VerifyBytes
// compares the output with the input, so a lossy UnmarshalXML fails here.
const (
	wDecl   = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	w15NS   = `xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml"`
	w16cid  = `xmlns:w16cid="http://schemas.microsoft.com/office/word/2016/wordml/cid"`
	extNS   = `xmlns:x="urn:example:ext"`
	allNS   = wDecl + ` ` + w15NS + ` ` + w16cid + ` ` + extNS
	lvlBody = `<w:lvl w:ilvl="0" w:tplc="04090001"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/></w:lvl>`
)

func TestWordAttrs_AbstractNum(t *testing.T) {
	src := `<w:abstractNum ` + allNS + ` w:abstractNumId="0" w15:restartNumberingAfterBreak="0" w16cid:durableId="1234567">` +
		`<w:nsid w:val="1A2B3C4D"/><w:multiLevelType w:val="hybridMultilevel"/>` + lvlBody + `</w:abstractNum>`
	roundtrip.VerifyBytes[*wml.CT_AbstractNum](t, []byte(src), ns.Qn("w:abstractNum"))
}

func TestWordAttrs_Num(t *testing.T) {
	src := `<w:num ` + allNS + ` w:numId="3" w16cid:durableId="98765"><w:abstractNumId w:val="0"/>` +
		`<w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/><x:ext x:a="1"/></w:lvlOverride><x:ext x:b="2"/></w:num>`
	roundtrip.VerifyBytes[*wml.CT_Num](t, []byte(src), ns.Qn("w:num"))
}

func TestWordAttrs_LvlOverride(t *testing.T) {
	src := `<w:lvlOverride ` + allNS + ` w:ilvl="1" x:flag="1"><w:startOverride w:val="5"/>` + lvlBody + `<x:ext/></w:lvlOverride>`
	roundtrip.VerifyBytes[*wml.CT_LvlOverride](t, []byte(src), ns.Qn("w:lvlOverride"))
}

func TestWordAttrs_FldChar(t *testing.T) {
	for _, src := range []string{
		`<w:fldChar ` + wDecl + ` w:fldCharType="begin" w:dirty="true"/>`,
		`<w:fldChar ` + wDecl + ` w:fldCharType="begin" w:fldLock="1"/>`,
		`<w:fldChar ` + wDecl + ` w:fldCharType="begin" w:dirty="1" w:fldLock="0"><w:ffData><w:name w:val="Text1"/></w:ffData></w:fldChar>`,
	} {
		roundtrip.VerifyBytes[*wml.CT_FldChar](t, []byte(src), ns.Qn("w:fldChar"))
	}
}

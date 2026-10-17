package wml

import (
	"encoding/xml"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// Built-in style IDs that NewBuiltinStyle can construct. They match the
// styleId values Word writes for the English UI.
const (
	StyleIDNormal               = "Normal"
	StyleIDDefaultParagraphFont = "DefaultParagraphFont"
	StyleIDTableNormal          = "TableNormal"
	StyleIDTitle                = "Title"
	StyleIDHeading1             = "Heading1"
	StyleIDHeading2             = "Heading2"
	StyleIDHeading3             = "Heading3"
	StyleIDHyperlink            = "Hyperlink"
	StyleIDTableGrid            = "TableGrid"
	StyleIDTOC1                 = "TOC1"
	StyleIDTOC2                 = "TOC2"
	StyleIDTOC3                 = "TOC3"
	StyleIDCaption              = "Caption"
)

// helper constructors for the small raw trees used by table-style tblPr.

func rawEl(local string, attrs ...string) *raw.RawElement {
	re := new(raw.RawElement)
	re.QName = ns.Qn("w:" + local)
	for i := 0; i+1 < len(attrs); i += 2 {
		re.Attrs = append(re.Attrs, xml.Attr{Name: ns.Qn("w:" + attrs[i]), Value: attrs[i+1]})
	}
	return re
}

func rawParent(local string, kids ...*raw.RawElement) *raw.RawElement {
	re := rawEl(local)
	for _, k := range kids {
		re.Children = append(re.Children, k)
	}
	return re
}

func i64(v int64) *int64 { return &v }

func bptr(v bool) *bool { return &v }

// BuiltinStyleDeps returns the style IDs that the built-in style id refers to
// through basedOn, so callers can create them first.
func BuiltinStyleDeps(id string) []string {
	switch id {
	case StyleIDTitle, StyleIDHeading1, StyleIDHeading2, StyleIDHeading3,
		StyleIDTOC1, StyleIDTOC2, StyleIDTOC3, StyleIDCaption:
		return []string{StyleIDNormal}
	case StyleIDHyperlink:
		return []string{StyleIDDefaultParagraphFont}
	case StyleIDTableGrid:
		return []string{StyleIDTableNormal}
	default:
		return nil
	}
}

// NewBuiltinStyle returns a Word-compatible definition of the built-in style
// id, or nil when id is not a known built-in. Definitions follow the Word 2016
// default template, without theme references or linked character styles.
func NewBuiltinStyle(id string) *CT_Style {
	s := new(CT_Style)
	s.StyleID = id
	switch id {
	case StyleIDNormal:
		s.Type, s.Default, s.Name = "paragraph", true, "Normal"
		s.SetQFormat(bptr(true))
	case StyleIDDefaultParagraphFont:
		s.Type, s.Default, s.Name = "character", true, "Default Paragraph Font"
		s.SetUIPriority(i64(1))
		s.SetSemiHidden(bptr(true))
		s.SetUnhideWhenUsed(bptr(true))
	case StyleIDTableNormal:
		s.Type, s.Default, s.Name = "table", true, "Normal Table"
		s.SetUIPriority(i64(99))
		s.SetSemiHidden(bptr(true))
		s.SetUnhideWhenUsed(bptr(true))
		s.rawRestPost = append(s.rawRestPost, rawParent("tblPr",
			rawEl("tblInd", "w", "0", "type", "dxa"),
			rawParent("tblCellMar",
				rawEl("top", "w", "0", "type", "dxa"),
				rawEl("left", "w", "108", "type", "dxa"),
				rawEl("bottom", "w", "0", "type", "dxa"),
				rawEl("right", "w", "108", "type", "dxa"),
			)))
	case StyleIDTitle:
		s.Type, s.Name = "paragraph", "Title"
		s.SetBasedOn(StyleIDNormal)
		s.SetNext(StyleIDNormal)
		s.SetUIPriority(i64(10))
		s.SetQFormat(bptr(true))
		s.PPr = new(CT_PPr)
		s.PPr.SetSpacingAfter(i64(0))
		s.PPr.SetSpacingLine(i64(240))
		s.PPr.SetSpacingLineRule("auto")
		s.PPr.SetContextualSpacing(bptr(true))
		s.RPr = new(CT_RPr)
		s.RPr.SetFontASCII("Calibri Light")
		s.RPr.SetFontHAnsi("Calibri Light")
		s.RPr.SetSzHalfPt(i64(56))
	case StyleIDHeading1, StyleIDHeading2, StyleIDHeading3:
		s.Type = "paragraph"
		s.SetBasedOn(StyleIDNormal)
		s.SetNext(StyleIDNormal)
		s.SetUIPriority(i64(9))
		s.SetUnhideWhenUsed(bptr(true))
		s.SetQFormat(bptr(true))
		s.PPr = new(CT_PPr)
		s.PPr.SetKeepNext(bptr(true))
		s.PPr.SetKeepLines(bptr(true))
		s.RPr = new(CT_RPr)
		s.RPr.SetFontASCII("Calibri Light")
		s.RPr.SetFontHAnsi("Calibri Light")
		switch id {
		case StyleIDHeading1:
			s.Name = "heading 1"
			s.PPr.SetSpacingBefore(i64(240))
			s.PPr.SetOutlineLevel(i64(0))
			s.RPr.SetColorVal("2F5496")
			s.RPr.SetSzHalfPt(i64(32))
		case StyleIDHeading2:
			s.Name = "heading 2"
			s.PPr.SetSpacingBefore(i64(40))
			s.PPr.SetOutlineLevel(i64(1))
			s.RPr.SetColorVal("2F5496")
			s.RPr.SetSzHalfPt(i64(26))
		default:
			s.Name = "heading 3"
			s.PPr.SetSpacingBefore(i64(40))
			s.PPr.SetOutlineLevel(i64(2))
			s.RPr.SetColorVal("1F3763")
			s.RPr.SetSzHalfPt(i64(24))
		}
		s.PPr.SetSpacingAfter(i64(0))
	case StyleIDHyperlink:
		s.Type, s.Name = "character", "Hyperlink"
		s.SetBasedOn(StyleIDDefaultParagraphFont)
		s.SetUIPriority(i64(99))
		s.SetUnhideWhenUsed(bptr(true))
		s.RPr = new(CT_RPr)
		s.RPr.SetColorVal("0563C1")
		s.RPr.SetUnderlineStyle("single")
	case StyleIDTableGrid:
		s.Type, s.Name = "table", "Table Grid"
		s.SetBasedOn(StyleIDTableNormal)
		s.SetUIPriority(i64(39))
		s.PPr = new(CT_PPr)
		s.PPr.SetSpacingAfter(i64(0))
		s.PPr.SetSpacingLine(i64(240))
		s.PPr.SetSpacingLineRule("auto")
		border := func(side string) *raw.RawElement {
			return rawEl(side, "val", "single", "sz", "4", "space", "0", "color", "auto")
		}
		s.rawRestPost = append(s.rawRestPost, rawParent("tblPr",
			rawParent("tblBorders",
				border("top"), border("left"), border("bottom"),
				border("right"), border("insideH"), border("insideV"))))
	case StyleIDTOC1, StyleIDTOC2, StyleIDTOC3:
		s.Type = "paragraph"
		s.SetBasedOn(StyleIDNormal)
		s.SetNext(StyleIDNormal)
		s.SetUIPriority(i64(39))
		s.SetUnhideWhenUsed(bptr(true))
		s.PPr = new(CT_PPr)
		s.PPr.SetSpacingAfter(i64(100))
		switch id {
		case StyleIDTOC1:
			s.Name = "toc 1"
		case StyleIDTOC2:
			s.Name = "toc 2"
			s.PPr.SetIndStart(i64(220))
		default:
			s.Name = "toc 3"
			s.PPr.SetIndStart(i64(440))
		}
	case StyleIDCaption:
		s.Type, s.Name = "paragraph", "caption"
		s.SetBasedOn(StyleIDNormal)
		s.SetNext(StyleIDNormal)
		s.SetUIPriority(i64(35))
		s.SetUnhideWhenUsed(bptr(true))
		s.SetQFormat(bptr(true))
		s.PPr = new(CT_PPr)
		s.PPr.SetSpacingAfter(i64(200))
		s.PPr.SetSpacingLine(i64(240))
		s.PPr.SetSpacingLineRule("auto")
		s.RPr = new(CT_RPr)
		s.RPr.SetItalic(bptr(true))
		s.RPr.SetItalicCS(bptr(true))
		s.RPr.SetColorVal("44546A")
		s.RPr.SetSzHalfPt(i64(18))
	default:
		return nil
	}
	return s
}

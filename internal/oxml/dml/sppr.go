package dml

import (
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_ShapeProperties is the pic:spPr element: geometry and appearance of a
// picture. Children stay raw in document order; only the transform's extent
// is exposed, because it must track the drawing's wp:extent.
type CT_ShapeProperties struct {
	// Attrs preserves bwMode.
	Attrs    []xml.Attr
	Children []*raw.RawElement
}

// xfrmExt returns the a:ext element inside a:xfrm, or nil.
func (p *CT_ShapeProperties) xfrmExt() *raw.RawElement {
	for _, c := range p.Children {
		if c.QName != ns.Qn("a:xfrm") {
			continue
		}
		for _, k := range c.Children {
			if re, ok := k.(*raw.RawElement); ok && re.QName == ns.Qn("a:ext") {
				return re
			}
		}
	}
	return nil
}

// ExtentEMU returns the transform's size in EMU; ok is false when the shape
// has no a:xfrm/a:ext.
func (p *CT_ShapeProperties) ExtentEMU() (cx, cy int64, ok bool) {
	ext := p.xfrmExt()
	if ext == nil {
		return 0, 0, false
	}
	cxs, _ := attrStr(ext.Attrs, "", "cx")
	cys, _ := attrStr(ext.Attrs, "", "cy")
	return attrInt64(cxs), attrInt64(cys), true
}

// SetExtentEMU updates the transform's size in place. It does nothing when
// the shape has no a:xfrm/a:ext (a picture without an explicit transform
// takes its size from the drawing extent).
func (p *CT_ShapeProperties) SetExtentEMU(cx, cy int64) {
	ext := p.xfrmExt()
	if ext == nil {
		return
	}
	setAttr(&ext.Attrs, "cx", strconv.FormatInt(cx, 10))
	setAttr(&ext.Attrs, "cy", strconv.FormatInt(cy, 10))
}

// setAttr sets an unqualified attribute, appending it when absent.
func setAttr(attrs *[]xml.Attr, local, value string) {
	for i, a := range *attrs {
		if a.Name.Space == "" && a.Name.Local == local {
			(*attrs)[i].Value = value
			return
		}
	}
	*attrs = append(*attrs, xml.Attr{Name: xml.Name{Local: local}, Value: value})
}

// MarshalXML emits <pic:spPr>.
func (p *CT_ShapeProperties) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("pic:spPr")
	start.Attr = append(start.Attr, p.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("dml: spPr start: %w", err)
	}
	if err := marshalRaw(e, p.Children); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a pic:spPr element.
func (p *CT_ShapeProperties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.Attrs, p.Children = nil, nil
	for _, a := range start.Attr {
		if !isNSDecl(a) {
			p.Attrs = append(p.Attrs, a)
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("dml: spPr: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			re, err := readRaw(d, t)
			if err != nil {
				return err
			}
			p.Children = append(p.Children, re)
		case xml.EndElement:
			return nil
		}
	}
}

func init() {
	registry.Register(ns.Qn("pic:spPr"), func() registry.Element { return new(CT_ShapeProperties) })
}

package wml

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_R is the w:r element: a text run.
//
// RPr holds optional run properties.
// Children holds the run's content elements (text, tabs, breaks, etc.)
// in document order.
type CT_R struct {
	RPr      *CT_RPr
	Children []RunContentElement
}

func (*CT_R) isInline() {}

// Text returns the visible text of this run using the same rendering rules as
// CT_P.Text(): tabs as \t, breaks as \n, field results included, instructions excluded.
func (r *CT_R) Text() string {
	var sb strings.Builder
	textFromRunContent(r.Children, &sb, false)
	return sb.String()
}

// MarshalXML emits <w:r>.
func (r *CT_R) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:r")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_R start: %w", err)
	}
	if r.RPr != nil {
		if err := r.RPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	for _, child := range r.Children {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:r element.
func (r *CT_R) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	r.RPr = nil
	r.Children = r.Children[:0]
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_R: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name {
			case ns.Qn("w:rPr"):
				r.RPr = new(CT_RPr)
				if err := r.RPr.UnmarshalXML(d, t); err != nil {
					return err
				}
			case ns.Qn("w:t"):
				rc := new(CT_Text)
				if err := rc.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			case ns.Qn("w:tab"):
				rc := new(CT_Tab)
				if err := rc.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			case ns.Qn("w:br"):
				rc := new(CT_Br)
				if err := rc.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			case ns.Qn("w:cr"):
				rc := new(CT_Cr)
				if err := rc.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			case ns.Qn("w:noBreakHyphen"):
				rc := new(CT_NoBreakHyphen)
				if err := rc.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			case ns.Qn("w:softHyphen"):
				rc := new(CT_SoftHyphen)
				if err := rc.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			case ns.Qn("w:sym"):
				rc := new(CT_Sym)
				if err := rc.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			case ns.Qn("w:fldChar"):
				rc := new(CT_FldChar)
				if err := rc.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			case ns.Qn("w:instrText"):
				rc := new(CT_InstrText)
				if err := rc.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			default:
				rc := new(RawRunContent)
				if err := rc.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			}
		case xml.EndElement:
			return nil
		}
	}
}

func init() {
	registry.Register(ns.Qn("w:r"), func() registry.Element { return new(CT_R) })
}

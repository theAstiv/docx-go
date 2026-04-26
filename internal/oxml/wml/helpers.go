package wml

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// wAttr extracts an attribute value by local name, ignoring namespace.
// OOXML uses the same local name for w:-namespaced attributes, so
// matching on Local alone is correct for well-formed documents.
func wAttr(attrs []xml.Attr, localName string) string {
	for _, a := range attrs {
		if a.Name.Local == localName {
			return a.Value
		}
	}
	return ""
}

// setAttrInSlice sets or appends an attribute in a slice.
func setAttrInSlice(attrs *[]xml.Attr, name xml.Name, value string) {
	for i, a := range *attrs {
		if a.Name.Local == name.Local {
			(*attrs)[i].Value = value
			return
		}
	}
	*attrs = append(*attrs, xml.Attr{Name: name, Value: value})
}

// removeAttrFromSlice removes an attribute by local name.
func removeAttrFromSlice(attrs *[]xml.Attr, localName string) {
	for i, a := range *attrs {
		if a.Name.Local == localName {
			*attrs = append((*attrs)[:i], (*attrs)[i+1:]...)
			return
		}
	}
}

// parseOnOffVal parses an OnOff val attribute.
// Absent val or values in {1, true, on} (case-insensitive) → true.
// Values in {0, false, off} → false.
func parseOnOffVal(val string) bool {
	switch strings.ToLower(val) {
	case "0", "false", "off":
		return false
	default:
		return true
	}
}

// rawGetOnOff returns the tri-state *bool for an OnOff child element.
// Returns nil when the element is absent.
func rawGetOnOff(children []*raw.RawElement, qname xml.Name) *bool {
	for _, child := range children {
		if child.QName == qname {
			v := parseOnOffVal(wAttr(child.Attrs, "val"))
			return &v
		}
	}
	return nil
}

// rawSetOnOff sets or removes an OnOff child element.
func rawSetOnOff(children *[]*raw.RawElement, qname xml.Name, b *bool) {
	if b == nil {
		rawRemoveChild(children, qname)
		return
	}
	for _, child := range *children {
		if child.QName == qname {
			if *b {
				removeAttrFromSlice(&child.Attrs, "val")
			} else {
				setAttrInSlice(&child.Attrs, ns.Qn("w:val"), "false")
			}
			return
		}
	}
	re := new(raw.RawElement)
	re.QName = qname
	if !*b {
		re.Attrs = []xml.Attr{{Name: ns.Qn("w:val"), Value: "false"}}
	}
	*children = append(*children, re)
}

// rawGetStr returns the w:val attribute of a child element, or "".
func rawGetStr(children []*raw.RawElement, qname xml.Name) string {
	for _, child := range children {
		if child.QName == qname {
			return wAttr(child.Attrs, "val")
		}
	}
	return ""
}

// rawSetStr sets or removes a child element with a single w:val attribute.
// An empty value removes the element.
func rawSetStr(children *[]*raw.RawElement, qname xml.Name, value string) {
	if value == "" {
		rawRemoveChild(children, qname)
		return
	}
	for _, child := range *children {
		if child.QName == qname {
			setAttrInSlice(&child.Attrs, ns.Qn("w:val"), value)
			return
		}
	}
	re := new(raw.RawElement)
	re.QName = qname
	re.Attrs = []xml.Attr{{Name: ns.Qn("w:val"), Value: value}}
	*children = append(*children, re)
}

// rawGetInt64Attr returns an integer attribute from a named child element.
func rawGetInt64Attr(children []*raw.RawElement, qname xml.Name, attrLocal string) *int64 {
	for _, child := range children {
		if child.QName == qname {
			s := wAttr(child.Attrs, attrLocal)
			if s == "" {
				return nil
			}
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return nil
			}
			return &n
		}
	}
	return nil
}

// rawSetInt64Attr sets an integer attribute on a child element,
// creating the element if it does not exist.
func rawSetInt64Attr(children *[]*raw.RawElement, qname xml.Name, attrName xml.Name, v *int64) {
	if v == nil {
		rawRemoveAttrFromElement(children, qname, attrName.Local)
		return
	}
	rawSetAttrOnElement(children, qname, attrName, strconv.FormatInt(*v, 10))
}

// rawSetAttrOnElement sets an attribute on a child element,
// creating the element if it does not exist.
func rawSetAttrOnElement(children *[]*raw.RawElement, qname xml.Name, attrName xml.Name, value string) {
	for _, child := range *children {
		if child.QName == qname {
			setAttrInSlice(&child.Attrs, attrName, value)
			return
		}
	}
	re := new(raw.RawElement)
	re.QName = qname
	re.Attrs = []xml.Attr{{Name: attrName, Value: value}}
	*children = append(*children, re)
}

// rawRemoveAttrFromElement removes an attribute from a child element.
// Removes the element itself if it becomes attribute-free.
func rawRemoveAttrFromElement(children *[]*raw.RawElement, qname xml.Name, attrLocal string) {
	for i, child := range *children {
		if child.QName == qname {
			removeAttrFromSlice(&child.Attrs, attrLocal)
			if len(child.Attrs) == 0 && len(child.Children) == 0 {
				*children = append((*children)[:i], (*children)[i+1:]...)
			}
			return
		}
	}
}

// rawRemoveChild removes the first child element with the given QName.
func rawRemoveChild(children *[]*raw.RawElement, qname xml.Name) {
	for i, child := range *children {
		if child.QName == qname {
			*children = append((*children)[:i], (*children)[i+1:]...)
			return
		}
	}
}

// rawUnmarshalChildren reads StartElement tokens from d into a RawElement slice.
// Stops and returns at the matching EndElement of the parent.
func rawUnmarshalChildren(d *xml.Decoder) ([]*raw.RawElement, error) {
	var children []*raw.RawElement
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("wml: rawUnmarshalChildren: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			re := new(raw.RawElement)
			if err := re.UnmarshalXML(d, t); err != nil {
				return nil, err
			}
			children = append(children, re)
		case xml.EndElement:
			return children, nil
		}
	}
}

// rawMarshalChildren encodes a []*raw.RawElement slice to enc.
func rawMarshalChildren(enc *xml.Encoder, children []*raw.RawElement) error {
	for _, child := range children {
		if err := child.MarshalXML(enc, xml.StartElement{}); err != nil {
			return err
		}
	}
	return nil
}

package common

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// OnOffValue represents a tri-state boolean as used by OOXML CT_OnOff elements.
//
// States:
//   - !Set          → element omitted (inherit from style)
//   - Set && True   → <w:b/> (empty element)
//   - Set && !True  → <w:b w:val="false"/>
type OnOffValue struct {
	Set  bool
	True bool
}

// OnOffTrue is the "explicitly on" state.
var OnOffTrue = OnOffValue{Set: true, True: true}

// OnOffFalse is the "explicitly off" state.
var OnOffFalse = OnOffValue{Set: true, True: false}

// OnOffUnset is the "inherit" state.
var OnOffUnset = OnOffValue{} //nolint:exhaustruct_v5 // intentional zero-value sentinel; Set==false means unset

// FromBool converts a *bool proxy value to OnOffValue.
func FromBool(b *bool) OnOffValue {
	if b == nil {
		return OnOffUnset
	}
	return OnOffValue{Set: true, True: *b}
}

// ToBool converts an OnOffValue to a *bool for use in the proxy layer.
// Returns nil when not set.
func (o OnOffValue) ToBool() *bool {
	if !o.Set {
		return nil
	}
	b := o.True
	return &b
}

// UnmarshalXML reads a CT_OnOff element.  The element is considered "true"
// when: the element is present with no val attribute, or val ∈ {1, true, on,
// True, On}.  It is "false" when val ∈ {0, false, off, False, Off}.
func (o *OnOffValue) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	o.Set = true
	o.True = true // default: presence alone means true

	val := attrVal(start.Attr, "val")
	if val != "" {
		switch strings.ToLower(val) {
		case "1", "true", "on":
			o.True = true
		case "0", "false", "off":
			o.True = false
		default:
			return fmt.Errorf("onoff: unknown val %q on <%s>", val, start.Name.Local)
		}
	}
	// Consume the (empty) element.
	return d.Skip()
}

// MarshalXML emits the CT_OnOff element.  If not Set, it emits nothing.
func (o OnOffValue) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if !o.Set {
		return nil
	}
	if o.True {
		// Emit self-closing element: <w:b/>
		if err := e.EncodeToken(start); err != nil {
			return err
		}
		return e.EncodeToken(xml.EndElement{Name: start.Name})
	}
	// Emit with val="false": <w:b w:val="false"/>
	wNS := "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	start.Attr = append(start.Attr, xml.Attr{
		Name:  xml.Name{Space: wNS, Local: "val"},
		Value: "false",
	})
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func attrVal(attrs []xml.Attr, localName string) string {
	for _, a := range attrs {
		if a.Name.Local == localName {
			return a.Value
		}
	}
	return ""
}

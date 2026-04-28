package common

import (
	"encoding/xml"
	"fmt"
	"strconv"
)

// DecimalNumber wraps an integer stored in a w:val attribute.
//
// Example XML: <w:numId w:val="3"/>.
type DecimalNumber struct {
	Val int64
}

func (d *DecimalNumber) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	raw := attrVal(start.Attr, "val")
	if raw == "" {
		return fmt.Errorf("decimalNumber <%s>: missing w:val attribute", start.Name.Local)
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("decimalNumber <%s>: %w", start.Name.Local, err)
	}
	d.Val = n
	return dec.Skip()
}

func (d DecimalNumber) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	wNS := "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	start.Attr = append(start.Attr, xml.Attr{
		Name:  xml.Name{Space: wNS, Local: "val"},
		Value: strconv.FormatInt(d.Val, 10),
	})
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// LongHexNumber wraps a hex string stored in a w:val attribute.
//
// Example XML: <w:rsid w:val="00AB1234"/>.
type LongHexNumber struct {
	Val uint32
}

func (h *LongHexNumber) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	raw := attrVal(start.Attr, "val")
	if raw == "" {
		return fmt.Errorf("longHexNumber <%s>: missing w:val attribute", start.Name.Local)
	}
	n, err := strconv.ParseUint(raw, 16, 32)
	if err != nil {
		return fmt.Errorf("longHexNumber <%s>: %w", start.Name.Local, err)
	}
	h.Val = uint32(n)
	return dec.Skip()
}

func (h LongHexNumber) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	wNS := "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	start.Attr = append(start.Attr, xml.Attr{
		Name:  xml.Name{Space: wNS, Local: "val"},
		Value: fmt.Sprintf("%08X", h.Val),
	})
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// TwipsMeasure wraps a length stored as an integer number of twips in w:val.
//
// Example XML: <w:ind w:left="720"/>.
type TwipsMeasure struct {
	Val int64
}

func (t *TwipsMeasure) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	raw := attrVal(start.Attr, "val")
	if raw == "" {
		return fmt.Errorf("twipsMeasure <%s>: missing w:val attribute", start.Name.Local)
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("twipsMeasure <%s>: %w", start.Name.Local, err)
	}
	t.Val = n
	return dec.Skip()
}

func (t TwipsMeasure) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	wNS := "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	start.Attr = append(start.Attr, xml.Attr{
		Name:  xml.Name{Space: wNS, Local: "val"},
		Value: strconv.FormatInt(t.Val, 10),
	})
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

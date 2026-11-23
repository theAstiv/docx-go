package docprops

import (
	"encoding/xml"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// Name is the type of a core property's element name. The alias lets the
// docx layer name it without importing encoding/xml.
type Name = xml.Name

// Core property element names. Title, Subject, Creator and Description are
// Dublin Core elements; Created and Modified are Dublin Core terms written
// with xsi:type="dcterms:W3CDTF"; the rest belong to the OPC core-properties
// namespace.
var (
	Title          = ns.Qn("dc:title")
	Subject        = ns.Qn("dc:subject")
	Creator        = ns.Qn("dc:creator")
	Description    = ns.Qn("dc:description")
	Language       = ns.Qn("dc:language")
	Identifier     = ns.Qn("dc:identifier")
	Keywords       = ns.Qn("cp:keywords")
	LastModifiedBy = ns.Qn("cp:lastModifiedBy")
	Revision       = ns.Qn("cp:revision")
	Category       = ns.Qn("cp:category")
	ContentStatus  = ns.Qn("cp:contentStatus")
	Version        = ns.Qn("cp:version")
	LastPrinted    = ns.Qn("cp:lastPrinted")
	Created        = ns.Qn("dcterms:created")
	Modified       = ns.Qn("dcterms:modified")
)

// rootName is the root element, cp:coreProperties.
var rootName = ns.Qn("cp:coreProperties")

// w3cdtf is the xsi:type value that dcterms:created and dcterms:modified
// must carry.
const w3cdtf = "dcterms:W3CDTF"

// CT_CoreProperties is the root element of docProps/core.xml.
//
// Children are kept as raw elements in document order, with typed
// accessors on top, so properties this package does not name (and any
// extension elements) survive a round-trip.
//
// MarshalXML writes elements with their canonical prefixes (cp, dc,
// dcterms, dcmitype, xsi) declared once on the root, as Word does. It does
// not go through nsfix: encoding/xml cannot otherwise keep the dcterms
// prefix that the xsi:type value "dcterms:W3CDTF" refers to.
type CT_CoreProperties struct {
	// Attrs holds the root's attributes other than namespace declarations.
	Attrs    []xml.Attr
	children []*raw.RawElement
}

// Text returns the text of the first child named name, and whether such a
// child exists.
func (c *CT_CoreProperties) Text(name xml.Name) (string, bool) {
	el := c.find(name)
	if el == nil {
		return "", false
	}
	var sb strings.Builder
	for _, tok := range el.Children {
		if cd, ok := tok.(xml.CharData); ok {
			sb.Write(cd)
		}
	}
	return sb.String(), true
}

// SetText sets the text of the child named name, creating it at the end
// when absent. Its attributes are kept.
func (c *CT_CoreProperties) SetText(name xml.Name, value string) {
	el := c.find(name)
	if el == nil {
		el = &raw.RawElement{QName: name, Attrs: nil, Children: nil}
		c.children = append(c.children, el)
	}
	el.Children = []xml.Token{xml.CharData(value)}
}

// Remove deletes every child named name.
func (c *CT_CoreProperties) Remove(name xml.Name) {
	c.children = slices.DeleteFunc(c.children, func(el *raw.RawElement) bool { return el.QName == name })
}

// Time returns a W3CDTF date-time child (Created, Modified, LastPrinted)
// parsed as a time, and false when it is absent or not a valid W3CDTF value.
func (c *CT_CoreProperties) Time(name xml.Name) (time.Time, bool) {
	s, ok := c.Text(name)
	if !ok {
		return time.Time{}, false
	}
	return ParseW3CDTF(strings.TrimSpace(s))
}

// SetTime writes t as a W3CDTF date-time in UTC with second precision. The
// dcterms elements also get xsi:type="dcterms:W3CDTF", which the schema
// requires.
func (c *CT_CoreProperties) SetTime(name xml.Name, t time.Time) {
	c.SetText(name, t.UTC().Format("2006-01-02T15:04:05Z"))
	if name.Space != ns.DCTerms {
		return
	}
	el := c.find(name)
	typ := ns.Qn("xsi:type")
	for i, a := range el.Attrs {
		if a.Name == typ {
			el.Attrs[i].Value = w3cdtf
			return
		}
	}
	el.Attrs = append(el.Attrs, xml.Attr{Name: typ, Value: w3cdtf})
}

// w3cdtfLayouts are the W3CDTF profiles of ISO 8601, most precise first.
var w3cdtfLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04Z07:00",
	"2006-01-02",
	"2006-01",
	"2006",
}

// ParseW3CDTF parses a W3CDTF date or date-time (the profile of ISO 8601
// core properties use).
func ParseW3CDTF(s string) (time.Time, bool) {
	for _, layout := range w3cdtfLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (c *CT_CoreProperties) find(name xml.Name) *raw.RawElement {
	for _, el := range c.children {
		if el.QName == name {
			return el
		}
	}
	return nil
}

// ── marshalling ───────────────────────────────────────────────────────────

// basePrefixes are declared on every root, in this order, as Word does.
var basePrefixes = []string{ns.CP, ns.DC, ns.DCTerms, ns.DCMIType, ns.XSI}

// prefixes assigns a prefix to every namespace the tree uses: the canonical
// one when known and free, else ns1, ns2, … It returns the map and the
// declaration order.
func (c *CT_CoreProperties) prefixes() (map[string]string, []string) {
	pfx := map[string]string{}
	taken := map[string]bool{}
	var order []string
	add := func(uri string) {
		if uri == "" || uri == ns.XML {
			return
		}
		if _, ok := pfx[uri]; ok {
			return
		}
		p := ns.Prefix(uri)
		for n := 1; p == "" || taken[p]; n++ {
			p = fmt.Sprintf("ns%d", n)
		}
		pfx[uri] = p
		taken[p] = true
		order = append(order, uri)
	}
	for _, uri := range basePrefixes {
		add(uri)
	}
	for _, a := range c.Attrs {
		add(a.Name.Space)
	}
	var walk func(el *raw.RawElement)
	walk = func(el *raw.RawElement) {
		add(el.QName.Space)
		for _, a := range el.Attrs {
			add(a.Name.Space)
		}
		for _, tok := range el.Children {
			if k, ok := tok.(*raw.RawElement); ok {
				walk(k)
			}
		}
	}
	for _, el := range c.children {
		walk(el)
	}
	return pfx, order
}

// qualified returns name as a literal "prefix:local" name. encoding/xml
// writes a Name with an empty Space verbatim, so the output keeps the
// prefixes declared on the root.
func qualified(name xml.Name, pfx map[string]string) xml.Name {
	switch name.Space {
	case "":
		return xml.Name{Space: "", Local: name.Local}
	case ns.XML:
		return xml.Name{Space: "", Local: "xml:" + name.Local}
	}
	return xml.Name{Space: "", Local: pfx[name.Space] + ":" + name.Local}
}

func qualifiedAttrs(attrs []xml.Attr, pfx map[string]string) []xml.Attr {
	out := make([]xml.Attr, 0, len(attrs))
	for _, a := range attrs {
		out = append(out, xml.Attr{Name: qualified(a.Name, pfx), Value: a.Value})
	}
	return out
}

// MarshalXML writes cp:coreProperties with canonical prefixes.
func (c *CT_CoreProperties) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	pfx, order := c.prefixes()
	attrs := make([]xml.Attr, 0, len(order)+len(c.Attrs))
	for _, uri := range order {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Space: "", Local: "xmlns:" + pfx[uri]}, Value: uri})
	}
	attrs = append(attrs, qualifiedAttrs(c.Attrs, pfx)...)
	start := xml.StartElement{Name: qualified(rootName, pfx), Attr: attrs}
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("docprops: coreProperties start: %w", err)
	}
	for _, el := range c.children {
		if err := writeRaw(e, el, pfx); err != nil {
			return err
		}
	}
	if err := e.EncodeToken(start.End()); err != nil {
		return fmt.Errorf("docprops: coreProperties end: %w", err)
	}
	return nil
}

func writeRaw(e *xml.Encoder, el *raw.RawElement, pfx map[string]string) error {
	start := xml.StartElement{Name: qualified(el.QName, pfx), Attr: qualifiedAttrs(el.Attrs, pfx)}
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("docprops: %s: %w", el.QName.Local, err)
	}
	for _, tok := range el.Children {
		var err error
		switch t := tok.(type) {
		case *raw.RawElement:
			err = writeRaw(e, t, pfx)
		case xml.CharData, xml.Comment, xml.ProcInst:
			err = e.EncodeToken(t)
		}
		if err != nil {
			return fmt.Errorf("docprops: %s: %w", el.QName.Local, err)
		}
	}
	if err := e.EncodeToken(start.End()); err != nil {
		return fmt.Errorf("docprops: %s: %w", el.QName.Local, err)
	}
	return nil
}

// UnmarshalXML reads cp:coreProperties. Namespace declarations are dropped
// (MarshalXML declares what it uses); whitespace and comments between
// properties are not kept.
func (c *CT_CoreProperties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	if start.Name != rootName {
		return fmt.Errorf("docprops: root is %s:%s, want coreProperties", start.Name.Space, start.Name.Local)
	}
	c.Attrs = c.Attrs[:0]
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		c.Attrs = append(c.Attrs, a)
	}
	c.children = c.children[:0]
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("docprops: coreProperties: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			el := new(raw.RawElement)
			if err := el.UnmarshalXML(d, t); err != nil {
				return fmt.Errorf("docprops: coreProperties: %w", err)
			}
			c.children = append(c.children, el)
		case xml.EndElement:
			return nil
		}
	}
}

func init() {
	registry.Register(ns.Qn("cp:coreProperties"), func() registry.Element { return new(CT_CoreProperties) })
}

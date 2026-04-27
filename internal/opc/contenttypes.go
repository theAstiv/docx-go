package opc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
)

const contentTypesName = "[Content_Types].xml"

// contentTypesXMLNS is the namespace for [Content_Types].xml.
const contentTypesXMLNS = "http://schemas.openxmlformats.org/package/2006/content-types"

// ContentTypes holds the Default (extension-based) and Override (per-part)
// content type entries from [Content_Types].xml.
type ContentTypes struct {
	Defaults  []CTDefault
	Overrides []CTOverride
}

// CTDefault maps a file extension to a content type.
type CTDefault struct {
	Extension   string
	ContentType string
}

// CTOverride maps an absolute part name to a content type.
type CTOverride struct {
	PartName    string
	ContentType string
}

// parseContentTypes parses the raw XML bytes of [Content_Types].xml.
func parseContentTypes(data []byte) (*ContentTypes, error) {
	type xmlDefault struct {
		Extension   string `xml:"Extension,attr"`
		ContentType string `xml:"ContentType,attr"`
	}
	type xmlOverride struct {
		PartName    string `xml:"PartName,attr"`
		ContentType string `xml:"ContentType,attr"`
	}
	type xmlTypes struct {
		XMLName   xml.Name      `xml:"Types"`
		Defaults  []xmlDefault  `xml:"Default"`
		Overrides []xmlOverride `xml:"Override"`
	}

	var xt xmlTypes
	if err := xml.Unmarshal(data, &xt); err != nil {
		return nil, fmt.Errorf("opc/contenttypes: parse: %w", err)
	}
	ct := &ContentTypes{}
	for _, d := range xt.Defaults {
		ct.Defaults = append(ct.Defaults, CTDefault(d))
	}
	for _, o := range xt.Overrides {
		ct.Overrides = append(ct.Overrides, CTOverride(o))
	}
	return ct, nil
}

// marshal serialises ContentTypes to XML bytes, with entries sorted for
// deterministic output.
func (ct *ContentTypes) marshal() ([]byte, error) {
	// Stable sort for deterministic output.
	defaults := make([]CTDefault, len(ct.Defaults))
	copy(defaults, ct.Defaults)
	sort.Slice(defaults, func(i, j int) bool {
		return defaults[i].Extension < defaults[j].Extension
	})

	overrides := make([]CTOverride, len(ct.Overrides))
	copy(overrides, ct.Overrides)
	sort.Slice(overrides, func(i, j int) bool {
		return overrides[i].PartName < overrides[j].PartName
	})

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")

	start := xml.StartElement{
		Name: xml.Name{Local: "Types"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "xmlns"}, Value: contentTypesXMLNS},
		},
	}
	if err := enc.EncodeToken(start); err != nil {
		return nil, err
	}
	for _, d := range defaults {
		elem := xml.StartElement{
			Name: xml.Name{Local: "Default"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "Extension"}, Value: d.Extension},
				{Name: xml.Name{Local: "ContentType"}, Value: d.ContentType},
			},
		}
		if err := enc.EncodeToken(elem); err != nil {
			return nil, err
		}
		if err := enc.EncodeToken(xml.EndElement{Name: elem.Name}); err != nil {
			return nil, err
		}
	}
	for _, o := range overrides {
		elem := xml.StartElement{
			Name: xml.Name{Local: "Override"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "PartName"}, Value: o.PartName},
				{Name: xml.Name{Local: "ContentType"}, Value: o.ContentType},
			},
		}
		if err := enc.EncodeToken(elem); err != nil {
			return nil, err
		}
		if err := enc.EncodeToken(xml.EndElement{Name: elem.Name}); err != nil {
			return nil, err
		}
	}
	if err := enc.EncodeToken(xml.EndElement{Name: start.Name}); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SetOverride adds or replaces the Override entry for partName.
func (ct *ContentTypes) SetOverride(partName, contentType string) {
	for i, o := range ct.Overrides {
		if o.PartName == partName {
			ct.Overrides[i].ContentType = contentType
			return
		}
	}
	ct.Overrides = append(ct.Overrides, CTOverride{PartName: partName, ContentType: contentType})
}

// SetDefault adds or replaces the Default entry for extension.
func (ct *ContentTypes) SetDefault(extension, contentType string) {
	for i, d := range ct.Defaults {
		if d.Extension == extension {
			ct.Defaults[i].ContentType = contentType
			return
		}
	}
	ct.Defaults = append(ct.Defaults, CTDefault{Extension: extension, ContentType: contentType})
}

// ContentTypeFor returns the effective content type for a part name (checking
// Override first, then Default by extension).  Returns "" if not found.
func (ct *ContentTypes) ContentTypeFor(partName string) string {
	for _, o := range ct.Overrides {
		if o.PartName == partName {
			return o.ContentType
		}
	}
	// Fall back to Default by extension.
	ext := extension(partName)
	for _, d := range ct.Defaults {
		if d.Extension == ext {
			return d.ContentType
		}
	}
	return ""
}

// extension extracts the lower-case file extension from a part name.
func extension(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i+1:]
		}
		if name[i] == '/' {
			break
		}
	}
	return ""
}

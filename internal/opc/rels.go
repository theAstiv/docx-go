package opc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Relationship type URI constants for known OPC part kinds.
const (
	RelTypeOfficeDocument = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	RelTypeStyles         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
	RelTypeSettings       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings"
	RelTypeWebSettings    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/webSettings"
	RelTypeFontTable      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/fontTable"
	RelTypeTheme          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"
	RelTypeImage          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
	RelTypeHyperlink      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
	RelTypeHeader         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header"
	RelTypeFooter         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer"
	RelTypeNumbering      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering"
	RelTypeComments       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	RelTypeFootnotes      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes"
	RelTypeEndnotes       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes"
	RelTypeThumbnail      = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail"
	RelTypeCoreProps      = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties"
	RelTypeExtendedProps  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties"
)

const relsXMLNS = "http://schemas.openxmlformats.org/package/2006/relationships"

// Relationship is a single OPC relationship entry.
type Relationship struct {
	ID         string
	Type       string
	Target     string
	TargetMode string // "External" or "" (internal)
}

// Rels holds the relationships defined in a .rels file.
type Rels struct {
	entries []Relationship
	nextID  int
}

// Add appends a relationship, auto-assigning the next rId if id == "".
// Returns the assigned ID.
func (r *Rels) Add(relType, target, targetMode string) string {
	id := r.nextRID()
	r.entries = append(r.entries, Relationship{
		ID:         id,
		Type:       relType,
		Target:     target,
		TargetMode: targetMode,
	})
	return id
}

// AddWithID adds a relationship with an explicit ID (used when round-tripping
// an existing document).
func (r *Rels) AddWithID(id, relType, target, targetMode string) {
	r.entries = append(r.entries, Relationship{
		ID:         id,
		Type:       relType,
		Target:     target,
		TargetMode: targetMode,
	})
	// Keep nextID ahead of any numeric rIds already present.
	if n := parseRIDNum(id); n >= r.nextID {
		r.nextID = n + 1
	}
}

// ByType returns all relationships whose Type matches relType.
func (r *Rels) ByType(relType string) []Relationship {
	var out []Relationship
	for _, e := range r.entries {
		if e.Type == relType {
			out = append(out, e)
		}
	}
	return out
}

// ByID returns the relationship with the given ID, or the zero value if not found.
func (r *Rels) ByID(id string) (Relationship, bool) {
	for _, e := range r.entries {
		if e.ID == id {
			return e, true
		}
	}
	return Relationship{}, false
}

// All returns all relationships.
func (r *Rels) All() []Relationship {
	out := make([]Relationship, len(r.entries))
	copy(out, r.entries)
	return out
}

// parseRels parses the raw XML bytes of a .rels file.
func parseRels(data []byte) (*Rels, error) {
	type xmlRel struct {
		ID         string `xml:"Id,attr"`
		Type       string `xml:"Type,attr"`
		Target     string `xml:"Target,attr"`
		TargetMode string `xml:"TargetMode,attr"`
	}
	type xmlRels struct {
		XMLName xml.Name  `xml:"Relationships"`
		Rels    []xmlRel  `xml:"Relationship"`
	}
	var xr xmlRels
	if err := xml.Unmarshal(data, &xr); err != nil {
		return nil, fmt.Errorf("opc/rels: parse: %w", err)
	}
	r := &Rels{}
	for _, rel := range xr.Rels {
		r.AddWithID(rel.ID, rel.Type, rel.Target, rel.TargetMode)
	}
	return r, nil
}

// marshal serialises the Rels to XML bytes, entries sorted by ID.
func (r *Rels) marshal() ([]byte, error) {
	sorted := make([]Relationship, len(r.entries))
	copy(sorted, r.entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")

	start := xml.StartElement{
		Name: xml.Name{Local: "Relationships"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "xmlns"}, Value: relsXMLNS},
		},
	}
	if err := enc.EncodeToken(start); err != nil {
		return nil, err
	}
	for _, rel := range sorted {
		attrs := []xml.Attr{
			{Name: xml.Name{Local: "Id"}, Value: rel.ID},
			{Name: xml.Name{Local: "Type"}, Value: rel.Type},
			{Name: xml.Name{Local: "Target"}, Value: rel.Target},
		}
		if rel.TargetMode != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "TargetMode"}, Value: rel.TargetMode})
		}
		elem := xml.StartElement{Name: xml.Name{Local: "Relationship"}, Attr: attrs}
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

// relsPath returns the .rels path for a given part path.
// e.g. "word/document.xml" → "word/_rels/document.xml.rels"
//      ""                  → "_rels/.rels"
func relsPath(partPath string) string {
	if partPath == "" {
		return "_rels/.rels"
	}
	dir := path.Dir(partPath)
	base := path.Base(partPath)
	return path.Join(dir, "_rels", base+".rels")
}

func (r *Rels) nextRID() string {
	id := "rId" + strconv.Itoa(r.nextID)
	r.nextID++
	return id
}

func parseRIDNum(id string) int {
	if !strings.HasPrefix(id, "rId") {
		return 0
	}
	n, _ := strconv.Atoi(id[3:])
	return n
}
